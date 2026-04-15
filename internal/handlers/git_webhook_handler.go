package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// GitWebhookHandler — Git Provider Webhook 接收端點
//
// 設計（CICD_ARCHITECTURE §10, M14）：
//   - POST /webhooks/git/:token — 公開端點，不需 JWT
//   - 透過 WebhookToken 查找 GitProvider
//   - 依 Provider 類型驗證 HMAC 簽名（GitHub / GitLab / Gitea）
//   - 解析 payload → WebhookEvent → 比對所有 Pipeline 的觸發規則
//   - 符合條件的 Pipeline 自動排入執行佇列
// ---------------------------------------------------------------------------

const gitWebhookMaxBodySize = 1 << 20 // 1MB

// GitWebhookHandler 處理 Git Provider 的 Webhook 事件。
type GitWebhookHandler struct {
	providerSvc *services.GitProviderService
	pipelineSvc *services.PipelineService
	scheduler   *services.PipelineScheduler
}

// NewGitWebhookHandler 建立 GitWebhookHandler。
func NewGitWebhookHandler(
	providerSvc *services.GitProviderService,
	pipelineSvc *services.PipelineService,
	scheduler *services.PipelineScheduler,
) *GitWebhookHandler {
	return &GitWebhookHandler{
		providerSvc: providerSvc,
		pipelineSvc: pipelineSvc,
		scheduler:   scheduler,
	}
}

// IngestWebhook 接收 Git Provider 的 Webhook 事件。
// POST /webhooks/git/:token
func (h *GitWebhookHandler) IngestWebhook(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing webhook token"})
		return
	}

	// ── 1. 查找 Provider ──────────────────────────────────────────
	ctx := c.Request.Context()
	provider, err := h.providerSvc.GetProviderByWebhookToken(ctx, token)
	if err != nil {
		// 不透露 token 無效的具體原因
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown webhook"})
		return
	}

	// ── 2. 讀取 body ─────────────────────────────────────────────
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, gitWebhookMaxBodySize+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	if len(body) > gitWebhookMaxBodySize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "body too large"})
		return
	}

	// ── 3. 驗證 Provider 專屬簽名 ────────────────────────────────
	if provider.WebhookSecretEnc != "" {
		if !h.verifyProviderSignature(c, provider, body) {
			logger.Warn("git webhook signature verification failed",
				"provider_id", provider.ID,
				"provider_type", provider.Type,
				"remote_addr", c.ClientIP(),
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
			return
		}
	}

	// ── 4. 偵測事件類型 + 解析 payload ───────────────────────────
	parser, err := services.NewWebhookPayloadParser(provider.Type)
	if err != nil {
		logger.Error("unsupported provider type", "type", provider.Type)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unsupported provider type"})
		return
	}

	eventHeader := h.getEventHeader(c, provider.Type)
	eventType := parser.DetectEventType(eventHeader)

	var event *services.WebhookEvent
	switch {
	case eventType == "push" || eventType == "tag_push":
		event, err = parser.ParsePushEvent(body)
	case eventType == "pull_request" || eventType == "merge_request":
		event, err = parser.ParseMergeRequestEvent(body)
	default:
		logger.Debug("git webhook event type not handled, skipping",
			"provider_id", provider.ID,
			"event_type", eventType,
		)
		c.JSON(http.StatusOK, gin.H{"message": "event type not handled", "event": eventType})
		return
	}
	if err != nil {
		logger.Warn("failed to parse git webhook payload",
			"provider_id", provider.ID,
			"event_type", eventType,
			"error", err,
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse webhook payload"})
		return
	}

	// ── 5. 比對所有 Pipeline 的觸發規則 ──────────────────────────
	pipelines, err := h.pipelineSvc.ListPipelinesWithTriggers(ctx)
	if err != nil {
		logger.Error("failed to list pipelines with triggers", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var triggered []gin.H
	for _, pt := range pipelines {
		rules, parseErr := services.ParseTriggerRules(pt.TriggersJSON)
		if parseErr != nil {
			logger.Warn("invalid trigger rules, skipping pipeline",
				"pipeline_id", pt.Pipeline.ID,
				"error", parseErr,
			)
			continue
		}

		result := services.EvaluateWebhookTriggers(rules, event)
		if !result.Matched {
			continue
		}

		// 建立 PipelineRun
		run, enqueueErr := h.enqueueRun(c, &pt.Pipeline, event)
		if enqueueErr != nil {
			logger.Error("failed to enqueue pipeline run from git webhook",
				"pipeline_id", pt.Pipeline.ID,
				"error", enqueueErr,
			)
			continue
		}

		logger.Info("pipeline triggered by git webhook",
			"pipeline_id", pt.Pipeline.ID,
			"run_id", run.ID,
			"provider", provider.Name,
			"repo", event.Repo,
			"branch", event.Branch,
			"event", event.EventType,
			"reason", result.Reason,
		)

		triggered = append(triggered, gin.H{
			"pipeline_id": pt.Pipeline.ID,
			"run_id":      run.ID,
			"reason":      result.Reason,
		})
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":   "webhook processed",
		"triggered": len(triggered),
		"runs":      triggered,
	})
}

// ---------------------------------------------------------------------------
// Provider-specific signature verification
// ---------------------------------------------------------------------------

func (h *GitWebhookHandler) verifyProviderSignature(c *gin.Context, provider *models.GitProvider, body []byte) bool {
	secret := provider.WebhookSecretEnc // AfterFind 已解密

	switch provider.Type {
	case models.GitProviderTypeGitHub:
		// GitHub: X-Hub-Signature-256 = sha256=<hex>
		sig := c.GetHeader("X-Hub-Signature-256")
		if sig == "" {
			return false
		}
		return verifyHMACSHA256(body, sig, secret)

	case models.GitProviderTypeGitLab:
		// GitLab: X-Gitlab-Token = <secret> (direct comparison)
		token := c.GetHeader("X-Gitlab-Token")
		return hmac.Equal([]byte(token), []byte(secret))

	case models.GitProviderTypeGitea:
		// Gitea: X-Gitea-Signature = <hex> (HMAC-SHA256, no prefix)
		sig := c.GetHeader("X-Gitea-Signature")
		if sig == "" {
			return false
		}
		return verifyHMACSHA256(body, sig, secret)

	default:
		return false
	}
}

// verifyHMACSHA256 驗證 HMAC-SHA256 簽名，支援 "sha256=" 前綴。
func verifyHMACSHA256(body []byte, signature, secret string) bool {
	signature = strings.TrimPrefix(signature, "sha256=")

	expectedMAC, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), expectedMAC)
}

// ---------------------------------------------------------------------------
// Event header extraction
// ---------------------------------------------------------------------------

func (h *GitWebhookHandler) getEventHeader(c *gin.Context, providerType string) string {
	switch providerType {
	case models.GitProviderTypeGitHub:
		return c.GetHeader("X-GitHub-Event")
	case models.GitProviderTypeGitLab:
		return c.GetHeader("X-Gitlab-Event")
	case models.GitProviderTypeGitea:
		return c.GetHeader("X-Gitea-Event")
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// Run creation
// ---------------------------------------------------------------------------

func (h *GitWebhookHandler) enqueueRun(
	c *gin.Context,
	pipeline *models.Pipeline,
	event *services.WebhookEvent,
) (*models.PipelineRun, error) {
	if pipeline.CurrentVersionID == nil {
		return nil, nil
	}

	run := &models.PipelineRun{
		PipelineID:       pipeline.ID,
		SnapshotID:       *pipeline.CurrentVersionID,
		ClusterID:        pipeline.ClusterID,
		Namespace:        pipeline.Namespace,
		TriggerType:      models.TriggerTypeWebhook,
		TriggerPayload:   event.Provider + ":" + event.Repo + "@" + event.Branch,
		TriggeredByUser:  math.MaxUint32, // system user
		ConcurrencyGroup: pipeline.ConcurrencyGroup,
	}

	if err := h.scheduler.EnqueueRun(c.Request.Context(), run); err != nil {
		return nil, err
	}
	return run, nil
}
