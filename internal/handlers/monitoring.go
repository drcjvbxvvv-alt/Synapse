package handlers

import (
	"strconv"

	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
)

// MonitoringHandler 監控處理器
type MonitoringHandler struct {
	monitoringConfigService *services.MonitoringConfigService
	prometheusService       *services.PrometheusService
}

// NewMonitoringHandler 建立監控處理器
func NewMonitoringHandler(monitoringConfigService *services.MonitoringConfigService, prometheusService *services.PrometheusService) *MonitoringHandler {
	return &MonitoringHandler{
		monitoringConfigService: monitoringConfigService,
		prometheusService:       prometheusService,
	}
}

// GetMonitoringConfig 獲取叢集監控配置
func (h *MonitoringHandler) GetMonitoringConfig(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	config, err := h.monitoringConfigService.GetMonitoringConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取監控配置失敗", "error", err)
		response.InternalError(c, "獲取監控配置失敗: "+err.Error())
		return
	}

	response.OK(c, config)
}

// UpdateMonitoringConfig 更新叢集監控配置
func (h *MonitoringHandler) UpdateMonitoringConfig(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	var config models.MonitoringConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	// 更新配置
	if err := h.monitoringConfigService.UpdateMonitoringConfig(uint(clusterID), &config); err != nil {
		logger.Error("更新監控配置失敗", "error", err)
		response.InternalError(c, "更新監控配置失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "更新成功"})
}

// TestMonitoringConnection 測試監控連線
func (h *MonitoringHandler) TestMonitoringConnection(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	_, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	var config models.MonitoringConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	// 測試連線
	if err := h.prometheusService.TestConnection(c.Request.Context(), &config); err != nil {
		logger.Error("測試監控連線失敗", "error", err)
		response.BadRequest(c, "連線測試失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "連線測試成功"})
}

// GetClusterMetrics 獲取叢集監控指標
func (h *MonitoringHandler) GetClusterMetrics(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取監控配置
	config, err := h.monitoringConfigService.GetMonitoringConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取監控配置失敗", "error", err)
		response.InternalError(c, "獲取監控配置失敗: "+err.Error())
		return
	}

	if config.Type == "disabled" {
		response.OK(c, &models.ClusterMetricsData{})
		return
	}

	// 獲取查詢參數
	timeRange := c.DefaultQuery("range", "1h")
	step := c.DefaultQuery("step", "1m")
	clusterName := c.Query("clusterName")

	// 查詢監控指標
	metrics, err := h.prometheusService.QueryClusterMetrics(c.Request.Context(), config, clusterName, timeRange, step)
	if err != nil {
		logger.Error("查詢叢集監控指標失敗", "error", err)
		response.InternalError(c, "查詢監控指標失敗: "+err.Error())
		return
	}

	response.OK(c, metrics)
}

// GetNodeMetrics 獲取節點監控指標
func (h *MonitoringHandler) GetNodeMetrics(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	nodeName := c.Param("nodeName")
	if nodeName == "" {
		response.BadRequest(c, "節點名稱不能為空")
		return
	}

	// 獲取監控配置
	config, err := h.monitoringConfigService.GetMonitoringConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取監控配置失敗", "error", err)
		response.InternalError(c, "獲取監控配置失敗: "+err.Error())
		return
	}

	if config.Type == "disabled" {
		response.OK(c, &models.ClusterMetricsData{})
		return
	}

	// 獲取查詢參數
	timeRange := c.DefaultQuery("range", "1h")
	step := c.DefaultQuery("step", "1m")
	clusterName := c.Query("clusterName")

	// 查詢節點監控指標
	metrics, err := h.prometheusService.QueryNodeMetrics(c.Request.Context(), config, clusterName, nodeName, timeRange, step)
	if err != nil {
		logger.Error("查詢節點監控指標失敗", "error", err)
		response.InternalError(c, "查詢監控指標失敗: "+err.Error())
		return
	}

	response.OK(c, metrics)
}

// GetPodMetrics 獲取 Pod 監控指標
func (h *MonitoringHandler) GetPodMetrics(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	namespace := c.Param("namespace")
	podName := c.Param("name")
	if namespace == "" || podName == "" {
		response.BadRequest(c, "命名空間和Pod名稱不能為空")
		return
	}

	// 獲取監控配置
	config, err := h.monitoringConfigService.GetMonitoringConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取監控配置失敗", "error", err)
		response.InternalError(c, "獲取監控配置失敗: "+err.Error())
		return
	}

	if config.Type == "disabled" {
		response.OK(c, &models.ClusterMetricsData{})
		return
	}

	// 獲取查詢參數
	timeRange := c.DefaultQuery("range", "1h")
	step := c.DefaultQuery("step", "1m")
	clusterName := c.Query("clusterName")

	// 查詢 Pod 監控指標
	metrics, err := h.prometheusService.QueryPodMetrics(c.Request.Context(), config, clusterName, namespace, podName, timeRange, step)
	if err != nil {
		logger.Error("查詢Pod監控指標失敗", "error", err)
		response.InternalError(c, "查詢監控指標失敗: "+err.Error())
		return
	}

	response.OK(c, metrics)
}

// GetWorkloadMetrics 獲取工作負載監控指標
func (h *MonitoringHandler) GetWorkloadMetrics(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	namespace := c.Param("namespace")
	workloadName := c.Param("name")
	if namespace == "" || workloadName == "" {
		response.BadRequest(c, "命名空間和工作負載名稱不能為空")
		return
	}

	// 獲取監控配置
	config, err := h.monitoringConfigService.GetMonitoringConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取監控配置失敗", "error", err)
		response.InternalError(c, "獲取監控配置失敗: "+err.Error())
		return
	}

	if config.Type == "disabled" {
		response.OK(c, &models.ClusterMetricsData{})
		return
	}

	// 獲取查詢參數
	timeRange := c.DefaultQuery("range", "1h")
	step := c.DefaultQuery("step", "1m")
	clusterName := c.Query("clusterName")

	// 查詢工作負載監控指標
	metrics, err := h.prometheusService.QueryWorkloadMetrics(c.Request.Context(), config, clusterName, namespace, workloadName, timeRange, step)
	if err != nil {
		logger.Error("查詢工作負載監控指標失敗", "error", err)
		response.InternalError(c, "查詢監控指標失敗: "+err.Error())
		return
	}

	response.OK(c, metrics)
}

// GetMonitoringTemplates 獲取監控配置模板
func (h *MonitoringHandler) GetMonitoringTemplates(c *gin.Context) {
	templates := gin.H{
		"disabled":        h.monitoringConfigService.GetDefaultConfig(),
		"prometheus":      h.monitoringConfigService.GetPrometheusConfig(),
		"victoriametrics": h.monitoringConfigService.GetVictoriaMetricsConfig(),
	}

	response.OK(c, templates)
}
