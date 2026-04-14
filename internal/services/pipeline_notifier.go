package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// PipelineNotifier — Pipeline 事件 → NotifyChannel 路由
//
// 設計原則（CICD_ARCHITECTURE §15）：
//   - Pipeline 定義 notify_on_success / notify_on_failure / notify_on_scan
//   - 每個欄位是 JSON 陣列，存放 NotifyChannel ID
//   - 事件觸發時查詢 channel → 格式化 → 發送
//   - 整合 NotifyDedup 防止通知風暴
// ---------------------------------------------------------------------------

// PipelineEvent 通知事件。
type PipelineEvent struct {
	Type         string // "run_success", "run_failed", "run_cancelled", "scan_critical"
	PipelineName string
	PipelineID   uint
	RunID        uint
	ClusterName  string
	Namespace    string
	TriggerType  string
	Error        string // 失敗原因（僅 run_failed）
	Duration     time.Duration
}

// PipelineNotifier 負責將 Pipeline 事件路由到 NotifyChannel。
type PipelineNotifier struct {
	db     *gorm.DB
	dedup  *NotifyDedup
	client *http.Client
}

// NewPipelineNotifier 建立 PipelineNotifier。
func NewPipelineNotifier(db *gorm.DB, dedup *NotifyDedup) *PipelineNotifier {
	return &PipelineNotifier{
		db:    db,
		dedup: dedup,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Notify 發送 Pipeline 事件通知到配置的 channels。
func (n *PipelineNotifier) Notify(ctx context.Context, event *PipelineEvent) {
	// 查詢 Pipeline 通知配置
	var pipeline models.Pipeline
	if err := n.db.WithContext(ctx).
		Select("id, name, notify_on_success, notify_on_failure, notify_on_scan").
		First(&pipeline, event.PipelineID).Error; err != nil {
		logger.Warn("pipeline notifier: failed to load pipeline",
			"pipeline_id", event.PipelineID, "error", err)
		return
	}

	// 根據事件類型選擇 channel ID 列表
	channelIDs := n.resolveChannelIDs(&pipeline, event.Type)
	if len(channelIDs) == 0 {
		return
	}

	// 查詢 channels
	var channels []models.NotifyChannel
	if err := n.db.WithContext(ctx).
		Where("id IN ? AND enabled = ?", channelIDs, true).
		Find(&channels).Error; err != nil {
		logger.Error("pipeline notifier: failed to load channels",
			"channel_ids", channelIDs, "error", err)
		return
	}

	// 發送到每個 channel
	for i := range channels {
		ch := &channels[i]

		// Dedup 檢查
		if n.dedup != nil && !n.dedup.ShouldNotify(event.PipelineID, event.Type, fmt.Sprintf("ch:%d", ch.ID), event.RunID) {
			logger.Debug("pipeline notifier: suppressed duplicate",
				"pipeline_id", event.PipelineID,
				"event", event.Type,
				"channel_id", ch.ID,
			)
			continue
		}

		go n.sendToChannel(ch, event)
	}
}

// resolveChannelIDs 從 Pipeline 的 notify 欄位解析 channel ID。
func (n *PipelineNotifier) resolveChannelIDs(pipeline *models.Pipeline, eventType string) []uint {
	var raw string
	switch eventType {
	case "run_success":
		raw = pipeline.NotifyOnSuccess
	case "run_failed":
		raw = pipeline.NotifyOnFailure
	case "scan_critical":
		raw = pipeline.NotifyOnScan
	default:
		// run_cancelled 等其他事件不通知
		return nil
	}

	if raw == "" || raw == "null" || raw == "[]" {
		return nil
	}

	var ids []uint
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		logger.Warn("pipeline notifier: invalid channel IDs JSON",
			"pipeline_id", pipeline.ID,
			"event_type", eventType,
			"raw", raw,
		)
		return nil
	}
	return ids
}

// sendToChannel 發送通知到指定 channel。
func (n *PipelineNotifier) sendToChannel(ch *models.NotifyChannel, event *PipelineEvent) {
	payload := n.formatPayload(ch.Type, event)

	data, err := json.Marshal(payload)
	if err != nil {
		logger.Error("pipeline notifier: marshal payload failed",
			"channel_id", ch.ID, "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", ch.WebhookURL, bytes.NewReader(data))
	if err != nil {
		logger.Error("pipeline notifier: create request failed",
			"channel_id", ch.ID, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		logger.Error("pipeline notifier: send failed",
			"channel_id", ch.ID,
			"channel_name", ch.Name,
			"event", event.Type,
			"error", err,
		)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		logger.Warn("pipeline notifier: non-2xx response",
			"channel_id", ch.ID,
			"status", resp.StatusCode,
			"event", event.Type,
		)
	}
}

// formatPayload 根據 channel 類型格式化通知內容。
func (n *PipelineNotifier) formatPayload(channelType string, event *PipelineEvent) map[string]interface{} {
	title := formatEventTitle(event)
	body := formatEventBody(event)

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
			"event":    event.Type,
			"pipeline": event.PipelineName,
			"run_id":   event.RunID,
			"cluster":  event.ClusterName,
			"duration": event.Duration.String(),
			"error":    event.Error,
		}
	}
}

func formatEventTitle(event *PipelineEvent) string {
	switch event.Type {
	case "run_success":
		return fmt.Sprintf("[Synapse] Pipeline `%s` succeeded", event.PipelineName)
	case "run_failed":
		return fmt.Sprintf("[Synapse] Pipeline `%s` failed", event.PipelineName)
	case "scan_critical":
		return fmt.Sprintf("[Synapse] Pipeline `%s` scan found critical vulnerabilities", event.PipelineName)
	default:
		return fmt.Sprintf("[Synapse] Pipeline `%s` — %s", event.PipelineName, event.Type)
	}
}

func formatEventBody(event *PipelineEvent) string {
	parts := fmt.Sprintf("Run #%d | Cluster: %s | Namespace: %s | Trigger: %s | Duration: %s",
		event.RunID, event.ClusterName, event.Namespace, event.TriggerType, event.Duration.Round(time.Second))
	if event.Error != "" {
		parts += fmt.Sprintf("\nError: %s", event.Error)
	}
	return parts
}
