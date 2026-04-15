package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// PromotionNotifier — Production Gate 通知（CICD_ARCHITECTURE §13, M17）
//
// 設計原則：
//   - 當 PromotionService 建立需要人工審核的 ApprovalRequest 時觸發通知
//   - 通知發送到目標環境的 notify_channel_ids 設定的 NotifyChannel
//   - 支援 Slack、Telegram、Teams、generic webhook 四種格式
//   - 通知內容包含：Pipeline 名稱、來源/目標環境、請求者、審批連結
// ---------------------------------------------------------------------------

// PromotionNotifier 負責發送 Production Gate 審批通知。
type PromotionNotifier struct {
	db     *gorm.DB
	client *http.Client
}

// NewPromotionNotifier 建立 PromotionNotifier。
func NewPromotionNotifier(db *gorm.DB) *PromotionNotifier {
	return &PromotionNotifier{
		db:     db,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GateEvent 代表一個 Production Gate 通知事件。
type GateEvent struct {
	PipelineID      uint   `json:"pipeline_id"`
	PipelineName    string `json:"pipeline_name"`
	PipelineRunID   uint   `json:"pipeline_run_id"`
	FromEnvironment string `json:"from_environment"`
	ToEnvironment   string `json:"to_environment"`
	RequesterName   string `json:"requester_name"`
	ApprovalID      uint   `json:"approval_id"`
	Reason          string `json:"reason"`
}

// NotifyProductionGate 在 production gate 觸發審批時發送通知。
// notifyChannelIDsJSON 是 JSON 陣列格式的 notify channel ID 清單。
func (n *PromotionNotifier) NotifyProductionGate(ctx context.Context, event *GateEvent, notifyChannelIDsJSON string) {
	channelIDs := parseGateChannelIDs(notifyChannelIDsJSON)
	if len(channelIDs) == 0 {
		return
	}

	// 查詢啟用的 channels
	var channels []models.NotifyChannel
	if err := n.db.WithContext(ctx).
		Where("id IN ? AND enabled = ?", channelIDs, true).
		Find(&channels).Error; err != nil {
		logger.Error("promotion notifier: failed to load channels",
			"channel_ids", channelIDs,
			"error", err,
		)
		return
	}

	if len(channels) == 0 {
		return
	}

	for i := range channels {
		go n.sendGateNotification(&channels[i], event)
	}

	logger.Info("production gate notification sent",
		"pipeline_id", event.PipelineID,
		"from", event.FromEnvironment,
		"to", event.ToEnvironment,
		"approval_id", event.ApprovalID,
		"channels", len(channels),
	)
}

// sendGateNotification 發送 production gate 通知到單一 channel。
func (n *PromotionNotifier) sendGateNotification(ch *models.NotifyChannel, event *GateEvent) {
	payload := formatGatePayload(ch.Type, event)

	data, err := json.Marshal(payload)
	if err != nil {
		logger.Error("promotion notifier: marshal gate payload failed",
			"channel_id", ch.ID, "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", ch.WebhookURL, bytes.NewReader(data))
	if err != nil {
		logger.Error("promotion notifier: create gate request failed",
			"channel_id", ch.ID, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		logger.Error("promotion notifier: send gate notification failed",
			"channel_id", ch.ID,
			"channel_name", ch.Name,
			"error", err,
		)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		logger.Warn("promotion notifier: non-2xx gate notification response",
			"channel_id", ch.ID,
			"status", resp.StatusCode,
		)
	}
}

// formatGatePayload 根據 channel 類型格式化 production gate 通知。
func formatGatePayload(channelType string, event *GateEvent) map[string]interface{} {
	title := fmt.Sprintf("[Synapse] Production Gate — %s → %s", event.FromEnvironment, event.ToEnvironment)
	body := formatGateBody(event)

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
			"event":            "production_gate",
			"pipeline_name":    event.PipelineName,
			"pipeline_id":      event.PipelineID,
			"pipeline_run_id":  event.PipelineRunID,
			"from_environment": event.FromEnvironment,
			"to_environment":   event.ToEnvironment,
			"requester":        event.RequesterName,
			"approval_id":      event.ApprovalID,
			"reason":           event.Reason,
		}
	}
}

func formatGateBody(event *GateEvent) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Pipeline: %s (Run #%d)", event.PipelineName, event.PipelineRunID))
	parts = append(parts, fmt.Sprintf("Promotion: %s → %s", event.FromEnvironment, event.ToEnvironment))
	if event.RequesterName != "" {
		parts = append(parts, fmt.Sprintf("Requested by: %s", event.RequesterName))
	}
	parts = append(parts, fmt.Sprintf("Approval ID: %d", event.ApprovalID))
	if event.Reason != "" {
		parts = append(parts, fmt.Sprintf("Reason: %s", event.Reason))
	}
	parts = append(parts, "Action required: approve or reject in Synapse")
	return strings.Join(parts, "\n")
}

// parseGateChannelIDs 解析 JSON 陣列 "[1,2,3]" → []uint{1,2,3}。
func parseGateChannelIDs(raw string) []uint {
	if raw == "" || raw == "null" || raw == "[]" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}
