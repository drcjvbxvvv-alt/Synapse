package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
)

// SIEMHandler 稽核日誌 SIEM 匯出處理器
type SIEMHandler struct {
	svc *services.SIEMService
}

func NewSIEMHandler(svc *services.SIEMService) *SIEMHandler {
	return &SIEMHandler{svc: svc}
}

// GetSIEMConfig 取得 SIEM Webhook 設定
// GET /api/v1/system/siem/config
func (h *SIEMHandler) GetSIEMConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if cfg.ID == 0 {
		response.OK(c, gin.H{"enabled": false, "webhookURL": "", "secretHeader": "", "secretValue": ""})
		return
	}
	response.OK(c, gin.H{
		"id":           cfg.ID,
		"enabled":      cfg.Enabled,
		"webhookURL":   cfg.WebhookURL,
		"secretHeader": cfg.SecretHeader,
		"secretValue":  cfg.SecretValue,
		"batchSize":    cfg.BatchSize,
	})
}

// UpdateSIEMConfig 更新 SIEM Webhook 設定
// PUT /api/v1/system/siem/config
func (h *SIEMHandler) UpdateSIEMConfig(c *gin.Context) {
	var req struct {
		Enabled      bool   `json:"enabled"`
		WebhookURL   string `json:"webhookURL"`
		SecretHeader string `json:"secretHeader"`
		SecretValue  string `json:"secretValue"`
		BatchSize    int    `json:"batchSize"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}
	if req.Enabled && req.WebhookURL == "" {
		response.BadRequest(c, "啟用 SIEM 匯出時必須填寫 Webhook URL")
		return
	}
	if req.BatchSize <= 0 {
		req.BatchSize = 100
	}

	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	cfg.Enabled = req.Enabled
	cfg.WebhookURL = req.WebhookURL
	cfg.SecretHeader = req.SecretHeader
	cfg.SecretValue = req.SecretValue
	cfg.BatchSize = req.BatchSize

	if err := h.svc.SaveConfig(c.Request.Context(), cfg); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "SIEM 設定已更新"})
}

// TestSIEMWebhook 測試 SIEM Webhook 連線
// POST /api/v1/system/siem/test
func (h *SIEMHandler) TestSIEMWebhook(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil || cfg.WebhookURL == "" {
		response.BadRequest(c, "請先設定 Webhook URL")
		return
	}

	payload := map[string]interface{}{
		"source":    "synapse",
		"eventType": "test",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"message":   "Synapse SIEM Webhook 測試",
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", cfg.WebhookURL, bytes.NewBuffer(b))
	if err != nil {
		response.InternalError(c, "建立請求失敗: "+err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.SecretHeader != "" {
		req.Header.Set(cfg.SecretHeader, cfg.SecretValue)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		response.InternalError(c, "Webhook 連線失敗: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		response.InternalError(c, fmt.Sprintf("Webhook 回傳錯誤狀態碼: %d", resp.StatusCode))
		return
	}
	response.OK(c, gin.H{"message": "Webhook 測試成功", "statusCode": resp.StatusCode})
}

// ExportAuditLogs 批次匯出稽核日誌（JSON）
// GET /api/v1/audit/export?start=2026-01-01&end=2026-12-31&format=json
func (h *SIEMHandler) ExportAuditLogs(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")

	var startT, endT *time.Time
	if startStr != "" {
		t, err := time.Parse("2006-01-02", startStr)
		if err == nil { startT = &t }
	}
	if endStr != "" {
		t, err := time.Parse("2006-01-02", endStr)
		if err == nil { endT = &t }
	}

	logs, err := h.svc.ExportAuditLogs(c.Request.Context(), startT, endT)
	if err != nil {
		response.InternalError(c, "查詢失敗: "+err.Error())
		return
	}

	b, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		response.InternalError(c, "序列化失敗: "+err.Error())
		return
	}

	filename := fmt.Sprintf("synapse-audit-%s.json", time.Now().Format("20060102"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")
	c.Data(200, "application/json", b)
}

