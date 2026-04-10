package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// ========== Alerts ==========

// GetAlerts 獲取告警列表
func (h *AlertHandler) GetAlerts(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取配置
	config, err := h.alertManagerConfigService.GetAlertManagerConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取 Alertmanager 配置失敗", "error", err)
		response.InternalError(c, "獲取 Alertmanager 配置失敗: "+err.Error())
		return
	}

	if !config.Enabled {
		response.OK(c, []models.Alert{})
		return
	}

	// 獲取過濾參數
	filter := make(map[string]string)
	if severity := c.Query("severity"); severity != "" {
		filter["severity"] = severity
	}
	if alertname := c.Query("alertname"); alertname != "" {
		filter["alertname"] = alertname
	}

	// 獲取告警列表
	alerts, err := h.alertManagerService.GetAlerts(c.Request.Context(), config, filter)
	if err != nil {
		logger.Error("獲取告警列表失敗", "error", err)
		response.InternalError(c, "獲取告警列表失敗: "+err.Error())
		return
	}

	response.OK(c, alerts)
}

// GetAlertGroups 獲取告警分組
func (h *AlertHandler) GetAlertGroups(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取配置
	config, err := h.alertManagerConfigService.GetAlertManagerConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取 Alertmanager 配置失敗", "error", err)
		response.InternalError(c, "獲取 Alertmanager 配置失敗: "+err.Error())
		return
	}

	if !config.Enabled {
		response.OK(c, []models.AlertGroup{})
		return
	}

	// 獲取告警分組
	groups, err := h.alertManagerService.GetAlertGroups(c.Request.Context(), config)
	if err != nil {
		logger.Error("獲取告警分組失敗", "error", err)
		response.InternalError(c, "獲取告警分組失敗: "+err.Error())
		return
	}

	response.OK(c, groups)
}

// GetAlertStats 獲取告警統計
func (h *AlertHandler) GetAlertStats(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取配置
	config, err := h.alertManagerConfigService.GetAlertManagerConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取 Alertmanager 配置失敗", "error", err)
		response.InternalError(c, "獲取 Alertmanager 配置失敗: "+err.Error())
		return
	}

	if !config.Enabled {
		response.OK(c, &models.AlertStats{
			Total:      0,
			Firing:     0,
			Pending:    0,
			Resolved:   0,
			Suppressed: 0,
			BySeverity: make(map[string]int),
		})
		return
	}

	// 獲取告警統計
	stats, err := h.alertManagerService.GetAlertStats(c.Request.Context(), config)
	if err != nil {
		logger.Error("獲取告警統計失敗", "error", err)
		response.InternalError(c, "獲取告警統計失敗: "+err.Error())
		return
	}

	response.OK(c, stats)
}

// ========== Silences ==========

// GetSilences 獲取靜默規則列表
func (h *AlertHandler) GetSilences(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取配置
	config, err := h.alertManagerConfigService.GetAlertManagerConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取 Alertmanager 配置失敗", "error", err)
		response.InternalError(c, "獲取 Alertmanager 配置失敗: "+err.Error())
		return
	}

	if !config.Enabled {
		response.OK(c, []models.Silence{})
		return
	}

	// 獲取靜默規則
	silences, err := h.alertManagerService.GetSilences(c.Request.Context(), config)
	if err != nil {
		logger.Error("獲取靜默規則失敗", "error", err)
		response.InternalError(c, "獲取靜默規則失敗: "+err.Error())
		return
	}

	response.OK(c, silences)
}

// CreateSilence 建立靜默規則
func (h *AlertHandler) CreateSilence(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取配置
	config, err := h.alertManagerConfigService.GetAlertManagerConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取 Alertmanager 配置失敗", "error", err)
		response.InternalError(c, "獲取 Alertmanager 配置失敗: "+err.Error())
		return
	}

	if !config.Enabled {
		response.BadRequest(c, "Alertmanager 未啟用")
		return
	}

	var req models.CreateSilenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	// 建立靜默規則
	silence, err := h.alertManagerService.CreateSilence(c.Request.Context(), config, &req)
	if err != nil {
		logger.Error("建立靜默規則失敗", "error", err)
		response.InternalError(c, "建立靜默規則失敗: "+err.Error())
		return
	}

	response.OK(c, silence)
}

// DeleteSilence 刪除靜默規則
func (h *AlertHandler) DeleteSilence(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	silenceID := c.Param("silenceId")
	if silenceID == "" {
		response.BadRequest(c, "靜默規則ID不能為空")
		return
	}

	// 獲取配置
	config, err := h.alertManagerConfigService.GetAlertManagerConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取 Alertmanager 配置失敗", "error", err)
		response.InternalError(c, "獲取 Alertmanager 配置失敗: "+err.Error())
		return
	}

	if !config.Enabled {
		response.BadRequest(c, "Alertmanager 未啟用")
		return
	}

	// 刪除靜默規則
	if err := h.alertManagerService.DeleteSilence(c.Request.Context(), config, silenceID); err != nil {
		logger.Error("刪除靜默規則失敗", "error", err)
		response.InternalError(c, "刪除靜默規則失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "刪除成功"})
}
