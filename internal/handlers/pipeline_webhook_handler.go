package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// PipelineWebhookHandler — Webhook 觸發端點
//
// 安全設計（CICD_ARCHITECTURE §7.7, P0-7）：
//   - HMAC-SHA256 驗簽（X-Synapse-Signature header）
//   - Timestamp replay protection（X-Synapse-Timestamp，±5 分鐘）
//   - Nonce 去重（X-Synapse-Nonce，LRU 快取 10 分鐘）
//   - Rate limit 由 middleware.APIRateLimit 在路由層套用
// ---------------------------------------------------------------------------

const (
	webhookMaxBodySize      = 1 << 20 // 1MB
	webhookTimestampMaxSkew = 5 * time.Minute
	webhookNonceTTL         = 10 * time.Minute
)

// PipelineWebhookHandler 處理外部 Webhook 觸發 Pipeline。
type PipelineWebhookHandler struct {
	pipelineSvc *services.PipelineService
	scheduler   *services.PipelineScheduler
	secretSvc   *services.PipelineSecretService
	nonceCache  *nonceCache
}

// NewPipelineWebhookHandler 建立 Webhook handler。
func NewPipelineWebhookHandler(
	pipelineSvc *services.PipelineService,
	scheduler *services.PipelineScheduler,
	secretSvc *services.PipelineSecretService,
) *PipelineWebhookHandler {
	return &PipelineWebhookHandler{
		pipelineSvc: pipelineSvc,
		scheduler:   scheduler,
		secretSvc:   secretSvc,
		nonceCache:  newNonceCache(webhookNonceTTL),
	}
}

// TriggerWebhook 處理 Webhook 觸發請求。
// POST /webhooks/pipelines/:pipelineID/trigger
func (h *PipelineWebhookHandler) TriggerWebhook(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pipeline ID"})
		return
	}

	// ── 讀取 body（限制大小） ─────────────────────────────────────
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, webhookMaxBodySize+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	if len(body) > webhookMaxBodySize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "body too large"})
		return
	}

	// ── 載入 Pipeline + 取得 webhook secret ──────────────────────
	pipeline, err := h.pipelineSvc.GetPipeline(c.Request.Context(), pipelineID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pipeline not found"})
		return
	}

	// 從 PipelineSecretService 查詢 Webhook Secret
	webhookSecret, err := h.lookupWebhookSecret(c.Request.Context(), pipeline.ID)
	if err != nil {
		logger.Warn("webhook secret not configured",
			"pipeline_id", pipelineID, "error", err)
		c.JSON(http.StatusForbidden, gin.H{"error": "webhook secret not configured for this pipeline"})
		return
	}

	// ── HMAC 驗簽 ───────────────────────────────────────────────
	signature := c.GetHeader("X-Synapse-Signature")
	if signature == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing signature header"})
		return
	}
	if !verifyHMAC(body, signature, webhookSecret) {
		logger.Warn("webhook HMAC verification failed",
			"pipeline_id", pipelineID,
			"remote_addr", c.ClientIP(),
		)
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
		return
	}

	// ── Timestamp replay protection ─────────────────────────────
	tsStr := c.GetHeader("X-Synapse-Timestamp")
	if tsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing timestamp header"})
		return
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid timestamp"})
		return
	}
	requestTime := time.Unix(ts, 0)
	skew := time.Since(requestTime)
	if skew < 0 {
		skew = -skew
	}
	if skew > webhookTimestampMaxSkew {
		logger.Warn("webhook timestamp out of range",
			"pipeline_id", pipelineID,
			"skew", skew,
		)
		c.JSON(http.StatusForbidden, gin.H{"error": "timestamp expired or too far in future"})
		return
	}

	// ── Nonce 去重（強制要求） ──────────────────────────────────
	nonce := c.GetHeader("X-Synapse-Nonce")
	if nonce == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing nonce header"})
		return
	}
	if h.nonceCache.seen(nonce) {
		logger.Warn("webhook nonce replay detected",
			"pipeline_id", pipelineID,
			"nonce", nonce,
		)
		c.JSON(http.StatusConflict, gin.H{"error": "duplicate request (nonce already used)"})
		return
	}
	h.nonceCache.add(nonce)

	// ── 建立 Pipeline Run ───────────────────────────────────────
	run, err := h.createWebhookRun(c, pipeline, body)
	if err != nil {
		logger.Error("webhook trigger failed",
			"pipeline_id", pipelineID, "error", err)
		response.InternalError(c, "failed to trigger pipeline: "+err.Error())
		return
	}

	logger.Info("pipeline triggered by webhook",
		"pipeline_id", pipelineID,
		"run_id", run.ID,
		"remote_addr", c.ClientIP(),
	)

	// Webhook 要求快速回應（202 Accepted）
	c.JSON(http.StatusAccepted, gin.H{
		"run_id":  run.ID,
		"status":  run.Status,
		"message": "pipeline triggered",
	})
}

// ---------------------------------------------------------------------------
// HMAC 驗簽
// ---------------------------------------------------------------------------

// verifyHMAC 驗證 HMAC-SHA256 簽名。
// signature 格式: sha256=<hex>
func verifyHMAC(body []byte, signature, secret string) bool {
	// 支援 "sha256=" prefix（GitHub/GitLab 格式）
	if len(signature) > 7 && signature[:7] == "sha256=" {
		signature = signature[7:]
	}

	expectedMAC, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	calculatedMAC := mac.Sum(nil)

	return hmac.Equal(expectedMAC, calculatedMAC)
}

// ---------------------------------------------------------------------------
// Nonce cache（LRU with TTL）
// ---------------------------------------------------------------------------

type nonceEntry struct {
	addedAt time.Time
}

type nonceCache struct {
	mu      sync.Mutex
	entries map[string]nonceEntry
	ttl     time.Duration
}

func newNonceCache(ttl time.Duration) *nonceCache {
	nc := &nonceCache{
		entries: make(map[string]nonceEntry),
		ttl:     ttl,
	}
	// 背景清理過期 nonce
	go nc.cleanupLoop()
	return nc
}

func (nc *nonceCache) seen(nonce string) bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	entry, ok := nc.entries[nonce]
	if !ok {
		return false
	}
	return time.Since(entry.addedAt) < nc.ttl
}

func (nc *nonceCache) add(nonce string) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.entries[nonce] = nonceEntry{addedAt: time.Now()}
}

func (nc *nonceCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		nc.mu.Lock()
		now := time.Now()
		for k, v := range nc.entries {
			if now.Sub(v.addedAt) > nc.ttl {
				delete(nc.entries, k)
			}
		}
		nc.mu.Unlock()
	}
}

// ---------------------------------------------------------------------------
// Run 建立
// ---------------------------------------------------------------------------

// lookupWebhookSecret 從 PipelineSecretService 查詢 Webhook Secret。
// 查詢順序：scope=pipeline → scope=global，name=WEBHOOK_SECRET。
func (h *PipelineWebhookHandler) lookupWebhookSecret(ctx context.Context, pipelineID uint) (string, error) {
	// 先查 pipeline scope
	secrets, err := h.secretSvc.ListSecrets(ctx, "pipeline", &pipelineID)
	if err != nil {
		return "", fmt.Errorf("list pipeline secrets: %w", err)
	}
	for _, sec := range secrets {
		if sec.Name == "WEBHOOK_SECRET" {
			full, err := h.secretSvc.GetSecret(ctx, sec.ID)
			if err != nil {
				return "", err
			}
			return full.ValueEnc, nil // AfterFind 已解密
		}
	}

	// 再查 global scope
	secrets, err = h.secretSvc.ListSecrets(ctx, "global", nil)
	if err != nil {
		return "", fmt.Errorf("list global secrets: %w", err)
	}
	for _, sec := range secrets {
		if sec.Name == "WEBHOOK_SECRET" {
			full, err := h.secretSvc.GetSecret(ctx, sec.ID)
			if err != nil {
				return "", err
			}
			return full.ValueEnc, nil
		}
	}

	return "", fmt.Errorf("WEBHOOK_SECRET not found for pipeline %d", pipelineID)
}

func (h *PipelineWebhookHandler) createWebhookRun(
	c *gin.Context,
	pipeline *models.Pipeline,
	payload []byte,
) (*models.PipelineRun, error) {
	// 確認有 current version
	if pipeline.CurrentVersionID == nil {
		return nil, fmt.Errorf("pipeline %d has no active version", pipeline.ID)
	}

	// 計算 payload hash（避免儲存完整 payload）
	payloadHash := sha256.Sum256(payload)
	payloadRef := hex.EncodeToString(payloadHash[:16]) // 前 16 bytes 作為參考

	run := &models.PipelineRun{
		PipelineID:       pipeline.ID,
		SnapshotID:       *pipeline.CurrentVersionID,
		ClusterID:        pipeline.ClusterID,
		Namespace:        pipeline.Namespace,
		TriggerType:      models.TriggerTypeWebhook,
		TriggerPayload:   payloadRef,
		TriggeredByUser:  math.MaxUint32, // system user for webhook triggers
		ConcurrencyGroup: pipeline.ConcurrencyGroup,
	}

	if err := h.scheduler.EnqueueRun(c.Request.Context(), run); err != nil {
		return nil, fmt.Errorf("enqueue webhook run: %w", err)
	}

	return run, nil
}
