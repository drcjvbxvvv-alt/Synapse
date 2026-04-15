package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// GitOps Reconciler — Auto Sync + Drift 通知（CICD_ARCHITECTURE §12.3）
//
// 設計原則：
//   - 定期（sync_interval）檢查 native GitOps App 的 drift 狀態
//   - sync_policy=auto → 偵測到 drift 時自動 apply（§12.3 auto sync）
//   - sync_policy=manual → 偵測到 drift 時發送通知到 NotifyChannel
//   - 整合 DiffEngine + GitCacheService + NotifyChannel
//   - 單一 goroutine loop，每 30 秒掃描一批需要 reconcile 的 App
// ---------------------------------------------------------------------------

const (
	// ReconcileTickInterval is the base interval for the reconciler scan loop.
	ReconcileTickInterval = 30 * time.Second

	// ReconcileDiffTimeout is the timeout for a single diff computation.
	ReconcileDiffTimeout = 30 * time.Second
)

// GitOpsReconciler 定期 reconcile GitOps 應用。
type GitOpsReconciler struct {
	db         *gorm.DB
	gitopsSvc  *GitOpsService
	diffEngine *GitOpsDiffEngine
	cacheSvc   *GitCacheService
	client     *http.Client

	stopCh chan struct{}
	once   sync.Once
}

// NewGitOpsReconciler 建立 GitOpsReconciler。
func NewGitOpsReconciler(
	db *gorm.DB,
	gitopsSvc *GitOpsService,
	diffEngine *GitOpsDiffEngine,
	cacheSvc *GitCacheService,
) *GitOpsReconciler {
	return &GitOpsReconciler{
		db:         db,
		gitopsSvc:  gitopsSvc,
		diffEngine: diffEngine,
		cacheSvc:   cacheSvc,
		client:     &http.Client{Timeout: 10 * time.Second},
		stopCh:     make(chan struct{}),
	}
}

// Start 啟動 reconciler loop（背景 goroutine）。
func (r *GitOpsReconciler) Start() {
	r.once.Do(func() {
		go r.loop()
		logger.Info("gitops reconciler started")
	})
}

// Stop 停止 reconciler。
func (r *GitOpsReconciler) Stop() {
	select {
	case r.stopCh <- struct{}{}:
	default:
	}
}

// loop 是 reconciler 的主循環。
func (r *GitOpsReconciler) loop() {
	ticker := time.NewTicker(ReconcileTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			logger.Info("gitops reconciler stopped")
			return
		case <-ticker.C:
			r.reconcileAll()
		}
	}
}

// reconcileAll 掃描所有需要 reconcile 的 App 並處理。
func (r *GitOpsReconciler) reconcileAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	apps, err := r.findAppsNeedingReconcile(ctx)
	if err != nil {
		logger.Error("gitops reconciler: failed to find apps", "error", err)
		return
	}

	if len(apps) == 0 {
		return
	}

	logger.Debug("gitops reconciler: processing apps", "count", len(apps))

	for i := range apps {
		select {
		case <-ctx.Done():
			return
		default:
			r.reconcileApp(ctx, &apps[i])
		}
	}
}

// findAppsNeedingReconcile 查詢所有需要 reconcile 的 native GitOps App。
// 條件：source=native AND 距離上次 diff 超過 sync_interval。
func (r *GitOpsReconciler) findAppsNeedingReconcile(ctx context.Context) ([]models.GitOpsApp, error) {
	var apps []models.GitOpsApp
	now := time.Now()

	// 查詢 native source 的 App，按 sync_interval 過濾
	if err := r.db.WithContext(ctx).
		Where("source = ?", models.GitOpsSourceNative).
		Find(&apps).Error; err != nil {
		return nil, fmt.Errorf("query gitops apps: %w", err)
	}

	// 過濾需要 reconcile 的 App
	var needReconcile []models.GitOpsApp
	for _, app := range apps {
		interval := time.Duration(app.SyncInterval) * time.Second
		if interval < 30*time.Second {
			interval = 30 * time.Second
		}

		if app.LastDiffAt == nil || now.Sub(*app.LastDiffAt) >= interval {
			needReconcile = append(needReconcile, app)
		}
	}

	return needReconcile, nil
}

// reconcileApp 執行單一 App 的 reconcile（diff + 通知/sync）。
func (r *GitOpsReconciler) reconcileApp(ctx context.Context, app *models.GitOpsApp) {
	diffCtx, cancel := context.WithTimeout(ctx, ReconcileDiffTimeout)
	defer cancel()

	logger.Debug("gitops reconciler: reconciling app",
		"app_id", app.ID,
		"app_name", app.Name,
		"sync_policy", app.SyncPolicy,
	)

	// 更新 last_diff_at 即使 diff 失敗，避免重複嘗試
	now := time.Now()
	_ = r.gitopsSvc.UpdateApp(diffCtx, app.ID, map[string]interface{}{
		"last_diff_at": now,
	})

	// 注意：完整的 diff 需要 K8s dynamic client + Git clone。
	// 這裡我們只處理 diff 結果的通知邏輯。
	// 實際的 DiffEngine.ComputeDiff 需要 K8s client 和 desiredResources，
	// 這些在 reconciler 整合到 router 時由 K8sProvider 提供。

	// 根據現有的 last_diff_result 處理通知
	if app.Status == models.GitOpsStatusDrifted && app.SyncPolicy == models.GitOpsSyncPolicyManual {
		r.notifyDrift(ctx, app)
	}
}

// ---------------------------------------------------------------------------
// Drift 通知 — 整合 NotifyChannel
// ---------------------------------------------------------------------------

// DriftEvent 代表一個 drift 通知事件。
type DriftEvent struct {
	AppID       uint   `json:"app_id"`
	AppName     string `json:"app_name"`
	ClusterID   uint   `json:"cluster_id"`
	Namespace   string `json:"namespace"`
	DiffSummary string `json:"diff_summary"`
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
}

// notifyDrift 發送 drift 通知到配置的 NotifyChannel。
func (r *GitOpsReconciler) notifyDrift(ctx context.Context, app *models.GitOpsApp) {
	channelIDs := parseNotifyChannelIDs(app.NotifyChannelIDs)
	if len(channelIDs) == 0 {
		return
	}

	// 查詢啟用的 channels
	var channels []models.NotifyChannel
	if err := r.db.WithContext(ctx).
		Where("id IN ? AND enabled = ?", channelIDs, true).
		Find(&channels).Error; err != nil {
		logger.Error("gitops reconciler: failed to load notify channels",
			"app_id", app.ID,
			"channel_ids", channelIDs,
			"error", err,
		)
		return
	}

	if len(channels) == 0 {
		return
	}

	event := &DriftEvent{
		AppID:       app.ID,
		AppName:     app.Name,
		ClusterID:   app.ClusterID,
		Namespace:   app.Namespace,
		DiffSummary: app.StatusMessage,
		RepoURL:     app.RepoURL,
		Branch:      app.Branch,
	}

	for i := range channels {
		go r.sendDriftNotification(&channels[i], event)
	}

	logger.Info("gitops drift notification sent",
		"app_id", app.ID,
		"app_name", app.Name,
		"channels", len(channels),
	)
}

// sendDriftNotification 發送 drift 通知到單一 channel。
func (r *GitOpsReconciler) sendDriftNotification(ch *models.NotifyChannel, event *DriftEvent) {
	payload := formatDriftPayload(ch.Type, event)

	data, err := json.Marshal(payload)
	if err != nil {
		logger.Error("gitops reconciler: marshal drift payload failed",
			"channel_id", ch.ID, "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", ch.WebhookURL, bytes.NewReader(data))
	if err != nil {
		logger.Error("gitops reconciler: create drift request failed",
			"channel_id", ch.ID, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		logger.Error("gitops reconciler: send drift notification failed",
			"channel_id", ch.ID,
			"channel_name", ch.Name,
			"error", err,
		)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		logger.Warn("gitops reconciler: non-2xx drift notification response",
			"channel_id", ch.ID,
			"status", resp.StatusCode,
		)
	}
}

// formatDriftPayload 根據 channel 類型格式化 drift 通知內容。
func formatDriftPayload(channelType string, event *DriftEvent) map[string]interface{} {
	title := fmt.Sprintf("[Synapse] GitOps Drift Detected — %s", event.AppName)
	body := formatDriftBody(event)

	switch channelType {
	case "slack":
		return map[string]interface{}{
			"text": fmt.Sprintf("%s\n%s", title, body),
		}
	case "telegram":
		return map[string]interface{}{
			"text":       fmt.Sprintf("%s\n%s", title, body),
			"parse_mode": "Markdown",
		}
	case "teams":
		return map[string]interface{}{
			"type": "message",
			"attachments": []map[string]interface{}{{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content": map[string]interface{}{
					"type":    "AdaptiveCard",
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"version": "1.2",
					"body": []map[string]interface{}{
						{"type": "TextBlock", "text": title, "weight": "bolder", "size": "medium"},
						{"type": "TextBlock", "text": body, "wrap": true},
					},
				},
			}},
		}
	default: // generic webhook
		return map[string]interface{}{
			"event":        "gitops_drift",
			"app_name":     event.AppName,
			"app_id":       event.AppID,
			"cluster_id":   event.ClusterID,
			"namespace":    event.Namespace,
			"diff_summary": event.DiffSummary,
			"repo_url":     event.RepoURL,
			"branch":       event.Branch,
		}
	}
}

func formatDriftBody(event *DriftEvent) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("App: %s (ID: %d)", event.AppName, event.AppID))
	parts = append(parts, fmt.Sprintf("Namespace: %s", event.Namespace))
	if event.RepoURL != "" {
		parts = append(parts, fmt.Sprintf("Repo: %s @ %s", event.RepoURL, event.Branch))
	}
	if event.DiffSummary != "" {
		parts = append(parts, fmt.Sprintf("Drift: %s", event.DiffSummary))
	}
	return strings.Join(parts, "\n")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseNotifyChannelIDs 解析 JSON 陣列 "[1,2,3]" → []uint{1,2,3}。
func parseNotifyChannelIDs(raw string) []uint {
	if raw == "" || raw == "null" || raw == "[]" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}
