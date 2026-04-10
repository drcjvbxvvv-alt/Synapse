package handlers

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
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

// ========== AlertManager Config ==========

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

// GetAlertManagerConfigTemplate 獲取 Alertmanager 配置模板
func (h *AlertHandler) GetAlertManagerConfigTemplate(c *gin.Context) {
	template := h.alertManagerConfigService.GetAlertManagerConfigTemplate()
	response.OK(c, template)
}

// ========== 共用輔助方法 ==========

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
