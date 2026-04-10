package handlers

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ==================== Grafana 配置相關介面 ====================

// UpdateGrafanaConfigRequest Grafana 配置更新請求
type UpdateGrafanaConfigRequest struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

// GetGrafanaConfig 獲取 Grafana 配置
func (h *SystemSettingHandler) GetGrafanaConfig(c *gin.Context) {
	config, err := h.grafanaSettingService.GetGrafanaConfig()
	if err != nil {
		logger.Error("獲取 Grafana 配置失敗: %v", err)
		response.InternalError(c, "獲取 Grafana 配置失敗")
		return
	}

	safeConfig := *config
	if safeConfig.APIKey != "" {
		safeConfig.APIKey = "******"
	}

	response.OK(c, safeConfig)
}

// UpdateGrafanaConfig 更新 Grafana 配置
func (h *SystemSettingHandler) UpdateGrafanaConfig(c *gin.Context) {
	var req UpdateGrafanaConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	existingConfig, err := h.grafanaSettingService.GetGrafanaConfig()
	if err != nil {
		logger.Error("獲取現有 Grafana 配置失敗: %v", err)
		response.InternalError(c, "更新 Grafana 配置失敗")
		return
	}

	config := &models.GrafanaSettingConfig{
		URL: req.URL,
	}

	if req.APIKey != "" && req.APIKey != "******" {
		config.APIKey = req.APIKey
	} else {
		config.APIKey = existingConfig.APIKey
	}

	if err := h.grafanaSettingService.SaveGrafanaConfig(config); err != nil {
		logger.Error("儲存 Grafana 配置失敗: %v", err)
		response.InternalError(c, "儲存 Grafana 配置失敗")
		return
	}

	// 配置更新後，重新整理 GrafanaService 的連線參數
	if h.grafanaService != nil {
		h.grafanaService.UpdateConfig(config.URL, config.APIKey)
		logger.Info("Grafana 服務配置已熱更新", "url", config.URL)
	}

	logger.Info("Grafana 配置更新成功")

	response.OK(c, gin.H{"message": "Grafana 配置更新成功"})
}

// TestGrafanaConnection 測試 Grafana 連線
func (h *SystemSettingHandler) TestGrafanaConnection(c *gin.Context) {
	var req UpdateGrafanaConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	existingConfig, _ := h.grafanaSettingService.GetGrafanaConfig()

	apiKey := req.APIKey
	if (apiKey == "" || apiKey == "******") && existingConfig != nil {
		apiKey = existingConfig.APIKey
	}

	// 建立臨時 GrafanaService 用於測試連線
	testSvc := services.NewGrafanaService(req.URL, apiKey)
	if err := testSvc.TestConnection(); err != nil {
		logger.Warn("Grafana 連線測試失敗: %v", err)
		response.OK(c, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.Info("Grafana 連線測試成功")

	response.OK(c, gin.H{
		"success": true,
	})
}

// GetGrafanaDashboardStatus 獲取 Dashboard 同步狀態
func (h *SystemSettingHandler) GetGrafanaDashboardStatus(c *gin.Context) {
	if h.grafanaService == nil || !h.grafanaService.IsEnabled() {
		response.OK(c, gin.H{
			"folder_exists": false,
			"dashboards":    []interface{}{},
			"all_synced":    false,
		})
		return
	}

	status, err := h.grafanaService.GetDashboardSyncStatus()
	if err != nil {
		logger.Error("獲取 Dashboard 同步狀態失敗: %v", err)
		response.InternalError(c, "獲取 Dashboard 同步狀態失敗: "+err.Error())
		return
	}

	response.OK(c, status)
}

// GetGrafanaDataSourceStatus 獲取資料來源同步狀態
func (h *SystemSettingHandler) GetGrafanaDataSourceStatus(c *gin.Context) {
	if h.grafanaService == nil || !h.grafanaService.IsEnabled() {
		response.OK(c, gin.H{
			"datasources": []interface{}{},
			"all_synced":  false,
		})
		return
	}

	clusters := h.getMonitoringClusters()
	status, err := h.grafanaService.GetDataSourceSyncStatus(clusters)
	if err != nil {
		response.InternalError(c, "獲取資料來源同步狀態失敗: "+err.Error())
		return
	}

	response.OK(c, status)
}

// SyncGrafanaDataSources 同步所有資料來源到 Grafana
func (h *SystemSettingHandler) SyncGrafanaDataSources(c *gin.Context) {
	if h.grafanaService == nil || !h.grafanaService.IsEnabled() {
		response.BadRequest(c, "請先配置 Grafana 連線資訊")
		return
	}

	clusters := h.getMonitoringClusters()
	if len(clusters) == 0 {
		response.OK(c, gin.H{
			"datasources": []interface{}{},
			"all_synced":  false,
		})
		return
	}

	status, err := h.grafanaService.SyncAllDataSources(clusters)
	if err != nil {
		response.InternalError(c, "同步資料來源失敗: "+err.Error())
		return
	}

	response.OK(c, status)
}

// getMonitoringClusters 獲取所有啟用了監控的叢集資訊
func (h *SystemSettingHandler) getMonitoringClusters() []services.DataSourceClusterInfo {
	return h.clusterService.ListMonitoringClusters(context.Background())
}

// SyncGrafanaDashboards 同步 Dashboard 到 Grafana
func (h *SystemSettingHandler) SyncGrafanaDashboards(c *gin.Context) {
	if h.grafanaService == nil || !h.grafanaService.IsEnabled() {
		response.BadRequest(c, "請先配置 Grafana 連線資訊")
		return
	}

	status, err := h.grafanaService.EnsureDashboards()
	if err != nil {
		logger.Error("同步 Dashboard 失敗: %v", err)
		response.InternalError(c, "同步 Dashboard 失敗: "+err.Error())
		return
	}

	logger.Info("Dashboard 同步完成", "all_synced", status.AllSynced)

	response.OK(c, status)
}
