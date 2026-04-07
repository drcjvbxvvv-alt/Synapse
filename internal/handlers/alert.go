package handlers

import (
	"fmt"
	"strconv"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
	"k8s.io/client-go/kubernetes"

	"github.com/gin-gonic/gin"
)

// AlertHandler 告警處理器
type AlertHandler struct {
	alertManagerConfigService *services.AlertManagerConfigService
	alertManagerService       *services.AlertManagerService
	k8sMgr                    *k8s.ClusterInformerManager
	clusterSvc                *services.ClusterService
}

// NewAlertHandler 建立告警處理器
func NewAlertHandler(
	alertManagerConfigService *services.AlertManagerConfigService,
	alertManagerService *services.AlertManagerService,
	k8sMgr *k8s.ClusterInformerManager,
	clusterSvc *services.ClusterService,
) *AlertHandler {
	return &AlertHandler{
		alertManagerConfigService: alertManagerConfigService,
		alertManagerService:       alertManagerService,
		k8sMgr:                    k8sMgr,
		clusterSvc:                clusterSvc,
	}
}

// GetAlertManagerConfig 獲取叢集 Alertmanager 配置
func (h *AlertHandler) GetAlertManagerConfig(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	config, err := h.alertManagerConfigService.GetAlertManagerConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取 Alertmanager 配置失敗", "error", err)
		response.InternalError(c, "獲取 Alertmanager 配置失敗: "+err.Error())
		return
	}

	response.OK(c, config)
}

// UpdateAlertManagerConfig 更新叢集 Alertmanager 配置
func (h *AlertHandler) UpdateAlertManagerConfig(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	var config models.AlertManagerConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	// 更新配置
	if err := h.alertManagerConfigService.UpdateAlertManagerConfig(uint(clusterID), &config); err != nil {
		logger.Error("更新 Alertmanager 配置失敗", "error", err)
		response.InternalError(c, "更新 Alertmanager 配置失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "更新成功"})
}

// TestAlertManagerConnection 測試 Alertmanager 連線
func (h *AlertHandler) TestAlertManagerConnection(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	_, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	var config models.AlertManagerConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	// 測試連線
	if err := h.alertManagerService.TestConnection(c.Request.Context(), &config); err != nil {
		logger.Error("測試 Alertmanager 連線失敗", "error", err)
		response.BadRequest(c, "連線測試失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "連線測試成功"})
}

// GetAlertManagerStatus 獲取 Alertmanager 狀態
func (h *AlertHandler) GetAlertManagerStatus(c *gin.Context) {
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
	response.OK(c, gin.H{"message": "Alertmanager 未啟用"})
		return
	}

	// 獲取狀態
	status, err := h.alertManagerService.GetStatus(c.Request.Context(), config)
	if err != nil {
		logger.Error("獲取 Alertmanager 狀態失敗", "error", err)
		response.InternalError(c, "獲取 Alertmanager 狀態失敗: "+err.Error())
		return
	}

	response.OK(c, status)
}

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

// GetReceivers 獲取接收器列表
func (h *AlertHandler) GetReceivers(c *gin.Context) {
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
		response.OK(c, []models.Receiver{})
		return
	}

	// 獲取接收器列表
	receivers, err := h.alertManagerService.GetReceivers(c.Request.Context(), config)
	if err != nil {
		logger.Error("獲取接收器列表失敗", "error", err)
		response.InternalError(c, "獲取接收器列表失敗: "+err.Error())
		return
	}

	response.OK(c, receivers)
}

// GetAlertManagerConfigTemplate 獲取 Alertmanager 配置模板
func (h *AlertHandler) GetAlertManagerConfigTemplate(c *gin.Context) {
	template := h.alertManagerConfigService.GetAlertManagerConfigTemplate()
	response.OK(c, template)
}

// GetFullReceivers 取得完整 Receiver 設定（含各渠道詳細參數）
func (h *AlertHandler) GetFullReceivers(c *gin.Context) {
	_, config, ok := h.getAlertConfig(c)
	if !ok {
		return
	}
	if !config.Enabled {
		response.OK(c, []models.ReceiverConfig{})
		return
	}
	receivers, err := h.alertManagerService.GetFullReceivers(c.Request.Context(), config)
	if err != nil {
		logger.Error("取得完整 Receiver 列表失敗", "error", err)
		response.InternalError(c, "取得 Receiver 列表失敗: "+err.Error())
		return
	}
	response.OK(c, receivers)
}

// CreateReceiver 新增 Receiver
func (h *AlertHandler) CreateReceiver(c *gin.Context) {
	clusterID, config, ok := h.getAlertConfig(c)
	if !ok {
		return
	}
	if !config.Enabled {
		response.BadRequest(c, "Alertmanager 未啟用")
		return
	}
	var req models.CreateReceiverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}
	clientset, err := h.getClientset(clusterID)
	if err != nil {
		response.InternalError(c, "取得叢集客戶端失敗: "+err.Error())
		return
	}
	if err := h.alertManagerService.CreateReceiver(c.Request.Context(), config, clientset, &req); err != nil {
		logger.Error("新增 Receiver 失敗", "error", err)
		response.InternalError(c, "新增 Receiver 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "新增成功"})
}

// UpdateReceiver 更新 Receiver
func (h *AlertHandler) UpdateReceiver(c *gin.Context) {
	clusterID, config, ok := h.getAlertConfig(c)
	if !ok {
		return
	}
	if !config.Enabled {
		response.BadRequest(c, "Alertmanager 未啟用")
		return
	}
	name := c.Param("name")
	if name == "" {
		response.BadRequest(c, "Receiver 名稱不能為空")
		return
	}
	var req models.UpdateReceiverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}
	clientset, err := h.getClientset(clusterID)
	if err != nil {
		response.InternalError(c, "取得叢集客戶端失敗: "+err.Error())
		return
	}
	if err := h.alertManagerService.UpdateReceiver(c.Request.Context(), config, clientset, name, &req); err != nil {
		logger.Error("更新 Receiver 失敗", "error", err)
		response.InternalError(c, "更新 Receiver 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "更新成功"})
}

// DeleteReceiver 刪除 Receiver
func (h *AlertHandler) DeleteReceiver(c *gin.Context) {
	clusterID, config, ok := h.getAlertConfig(c)
	if !ok {
		return
	}
	if !config.Enabled {
		response.BadRequest(c, "Alertmanager 未啟用")
		return
	}
	name := c.Param("name")
	if name == "" {
		response.BadRequest(c, "Receiver 名稱不能為空")
		return
	}
	clientset, err := h.getClientset(clusterID)
	if err != nil {
		response.InternalError(c, "取得叢集客戶端失敗: "+err.Error())
		return
	}
	if err := h.alertManagerService.DeleteReceiver(c.Request.Context(), config, clientset, name); err != nil {
		logger.Error("刪除 Receiver 失敗", "error", err)
		response.InternalError(c, "刪除 Receiver 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "刪除成功"})
}

// TestReceiver 傳送測試告警至指定 Receiver
func (h *AlertHandler) TestReceiver(c *gin.Context) {
	_, config, ok := h.getAlertConfig(c)
	if !ok {
		return
	}
	if !config.Enabled {
		response.BadRequest(c, "Alertmanager 未啟用")
		return
	}
	name := c.Param("name")
	if name == "" {
		response.BadRequest(c, "Receiver 名稱不能為空")
		return
	}
	var req models.TestReceiverRequest
	_ = c.ShouldBindJSON(&req) // 可選 body
	if err := h.alertManagerService.TestReceiver(c.Request.Context(), config, name, &req); err != nil {
		logger.Error("測試 Receiver 失敗", "error", err)
		response.InternalError(c, "測試告警傳送失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "測試告警已傳送"})
}

// -------- 共用輔助方法 --------

// getAlertConfig 解析 clusterID 並取得 AlertManagerConfig（失敗時自動回傳錯誤）
func (h *AlertHandler) getAlertConfig(c *gin.Context) (uint, *models.AlertManagerConfig, bool) {
	clusterIDStr := c.Param("clusterID")
	clusterID64, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return 0, nil, false
	}
	clusterID := uint(clusterID64)
	config, err := h.alertManagerConfigService.GetAlertManagerConfig(clusterID)
	if err != nil {
		logger.Error("取得 Alertmanager 配置失敗", "error", err)
		response.InternalError(c, "取得 Alertmanager 配置失敗: "+err.Error())
		return 0, nil, false
	}
	return clusterID, config, true
}

// getClientset 取得叢集的 kubernetes.Clientset
func (h *AlertHandler) getClientset(clusterID uint) (*kubernetes.Clientset, error) {
	if h.clusterSvc == nil || h.k8sMgr == nil {
		return nil, fmt.Errorf("K8s 管理器未初始化")
	}
	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		return nil, fmt.Errorf("取得叢集失敗: %w", err)
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		return nil, fmt.Errorf("取得 K8s 客戶端失敗: %w", err)
	}
	return k8sClient.GetClientset(), nil
}
