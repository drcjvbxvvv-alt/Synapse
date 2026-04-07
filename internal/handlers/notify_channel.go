package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NotifyChannelHandler 通知渠道處理器
type NotifyChannelHandler struct {
	db *gorm.DB
}

// NewNotifyChannelHandler 建立通知渠道處理器
func NewNotifyChannelHandler(db *gorm.DB) *NotifyChannelHandler {
	return &NotifyChannelHandler{db: db}
}

// ListNotifyChannels GET /system/notify-channels
func (h *NotifyChannelHandler) ListNotifyChannels(c *gin.Context) {
	var channels []models.NotifyChannel
	if err := h.db.Order("created_at DESC").Find(&channels).Error; err != nil {
		response.InternalError(c, "查詢通知渠道失敗")
		return
	}
	// 遮蔽 Telegram Bot Token
	for i := range channels {
		if channels[i].TelegramChatID != "" {
			channels[i].TelegramChatID = "******"
		}
	}
	response.OK(c, channels)
}

// CreateNotifyChannel POST /system/notify-channels
func (h *NotifyChannelHandler) CreateNotifyChannel(c *gin.Context) {
	var req models.NotifyChannel
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數無效: "+err.Error())
		return
	}
	if req.Name == "" || req.Type == "" || req.WebhookURL == "" {
		response.BadRequest(c, "名稱、型別和 Webhook URL 為必填")
		return
	}
	if err := h.db.Create(&req).Error; err != nil {
		response.InternalError(c, "建立通知渠道失敗")
		return
	}
	if req.TelegramChatID != "" {
		req.TelegramChatID = "******"
	}
	response.OK(c, req)
}

// UpdateNotifyChannel PUT /system/notify-channels/:id
func (h *NotifyChannelHandler) UpdateNotifyChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的 ID")
		return
	}

	var channel models.NotifyChannel
	if err := h.db.First(&channel, id).Error; err != nil {
		response.NotFound(c, "通知渠道不存在")
		return
	}

	var req models.NotifyChannel
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數無效: "+err.Error())
		return
	}

	// 若 TelegramChatID 為遮蔽值則保留原值
	if req.TelegramChatID == "******" {
		req.TelegramChatID = channel.TelegramChatID
	}

	channel.Name = req.Name
	channel.Type = req.Type
	channel.WebhookURL = req.WebhookURL
	channel.TelegramChatID = req.TelegramChatID
	channel.Description = req.Description
	channel.Enabled = req.Enabled

	if err := h.db.Save(&channel).Error; err != nil {
		response.InternalError(c, "更新通知渠道失敗")
		return
	}
	if channel.TelegramChatID != "" {
		channel.TelegramChatID = "******"
	}
	response.OK(c, channel)
}

// DeleteNotifyChannel DELETE /system/notify-channels/:id
func (h *NotifyChannelHandler) DeleteNotifyChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的 ID")
		return
	}
	if err := h.db.Delete(&models.NotifyChannel{}, id).Error; err != nil {
		response.InternalError(c, "刪除通知渠道失敗")
		return
	}
	response.OK(c, nil)
}

// TestNotifyChannelRequest 測試通知請求
type TestNotifyChannelRequest struct {
	ChannelID uint `json:"channelId"`
}

// TestNotifyChannel POST /system/notify-channels/:id/test
func (h *NotifyChannelHandler) TestNotifyChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的 ID")
		return
	}

	var channel models.NotifyChannel
	if err := h.db.First(&channel, id).Error; err != nil {
		response.NotFound(c, "通知渠道不存在")
		return
	}

	if err := sendTestNotification(&channel); err != nil {
		logger.Error("測試通知失敗", "channel", channel.Name, "error", err)
		response.BadRequest(c, "測試通知傳送失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "測試通知已傳送"})
}

// sendTestNotification 傳送測試通知
func sendTestNotification(ch *models.NotifyChannel) error {
	var payload interface{}

	testMsg := fmt.Sprintf("[Synapse] 通知渠道「%s」測試訊息，傳送時間：%s", ch.Name, time.Now().Format("2006-01-02 15:04:05"))

	switch ch.Type {
	case "telegram":
		payload = map[string]interface{}{
			"chat_id": ch.TelegramChatID,
			"text":    testMsg,
		}
	case "slack":
		payload = map[string]interface{}{"text": testMsg}
	case "teams":
		payload = map[string]interface{}{
			"type": "message",
			"attachments": []map[string]interface{}{{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content": map[string]interface{}{
					"type":    "AdaptiveCard",
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"version": "1.2",
					"body":    []map[string]interface{}{{"type": "TextBlock", "text": testMsg, "wrap": true}},
				},
			}},
		}
	default: // webhook
		payload = map[string]string{"message": testMsg}
	}

	data, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", ch.WebhookURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// SendToChannel 透過指定渠道傳送通知（供 EventAlertWorker 使用）
func SendToChannel(ch *models.NotifyChannel, title, content string) error {
	var payload interface{}

	switch ch.Type {
	case "telegram":
		payload = map[string]interface{}{
			"chat_id": ch.TelegramChatID,
			"text":    fmt.Sprintf("*%s*\n%s", title, content),
			"parse_mode": "Markdown",
		}
	case "slack":
		payload = map[string]interface{}{"text": content}
	case "teams":
		payload = map[string]interface{}{
			"type": "message",
			"attachments": []map[string]interface{}{{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content": map[string]interface{}{
					"type":    "AdaptiveCard",
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"version": "1.2",
					"body": []map[string]interface{}{
						{"type": "TextBlock", "text": title, "weight": "bolder"},
						{"type": "TextBlock", "text": content, "wrap": true},
					},
				},
			}},
		}
	default: // webhook
		payload = map[string]interface{}{"title": title, "content": content}
	}

	data, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", ch.WebhookURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
