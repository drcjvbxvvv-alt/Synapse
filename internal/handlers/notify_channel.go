package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
	// 遮蔽 DingTalk 簽名密鑰
	for i := range channels {
		if channels[i].DingTalkSecret != "" {
			channels[i].DingTalkSecret = "******"
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
		response.BadRequest(c, "名稱、類型和 Webhook URL 為必填")
		return
	}
	if err := h.db.Create(&req).Error; err != nil {
		response.InternalError(c, "建立通知渠道失敗")
		return
	}
	if req.DingTalkSecret != "" {
		req.DingTalkSecret = "******"
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

	// 若 DingTalkSecret 為遮蔽值則保留原值
	if req.DingTalkSecret == "******" {
		req.DingTalkSecret = channel.DingTalkSecret
	}

	channel.Name = req.Name
	channel.Type = req.Type
	channel.WebhookURL = req.WebhookURL
	channel.DingTalkSecret = req.DingTalkSecret
	channel.Description = req.Description
	channel.Enabled = req.Enabled

	if err := h.db.Save(&channel).Error; err != nil {
		response.InternalError(c, "更新通知渠道失敗")
		return
	}
	if channel.DingTalkSecret != "" {
		channel.DingTalkSecret = "******"
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
		response.BadRequest(c, "測試通知發送失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "測試通知已發送"})
}

// sendTestNotification 發送測試通知
func sendTestNotification(ch *models.NotifyChannel) error {
	var payload interface{}

	testMsg := fmt.Sprintf("[Synapse] 通知渠道「%s」測試訊息，發送時間：%s", ch.Name, time.Now().Format("2006-01-02 15:04:05"))

	switch ch.Type {
	case "dingtalk":
		payload = map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]string{"content": testMsg},
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
	webhookURL := ch.WebhookURL

	// DingTalk HMAC-SHA256 簽名
	if ch.Type == "dingtalk" && ch.DingTalkSecret != "" {
		timestamp := time.Now().UnixMilli()
		sign := dingTalkSign(ch.DingTalkSecret, timestamp)
		webhookURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhookURL, timestamp, sign)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(data))
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

// dingTalkSign 產生 DingTalk HMAC-SHA256 簽名
func dingTalkSign(secret string, timestamp int64) string {
	strToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(strToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// SendToChannel 透過指定渠道發送通知（供 EventAlertWorker 使用）
func SendToChannel(ch *models.NotifyChannel, title, content string) error {
	var payload interface{}

	switch ch.Type {
	case "dingtalk":
		payload = map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]string{"content": content},
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
	webhookURL := ch.WebhookURL

	if ch.Type == "dingtalk" && ch.DingTalkSecret != "" {
		timestamp := time.Now().UnixMilli()
		sign := dingTalkSign(ch.DingTalkSecret, timestamp)
		webhookURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhookURL, timestamp, sign)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(data))
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
