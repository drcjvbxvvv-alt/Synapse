package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// OMHandler 運維中心處理器
type OMHandler struct {
	clusterSvc *services.ClusterService
	omSvc      *services.OMService
	k8sMgr     *k8s.ClusterInformerManager
}

// NewOMHandler 建立運維中心處理器
func NewOMHandler(clusterSvc *services.ClusterService, omSvc *services.OMService, k8sMgr *k8s.ClusterInformerManager) *OMHandler {
	return &OMHandler{
		clusterSvc: clusterSvc,
		omSvc:      omSvc,
		k8sMgr:     k8sMgr,
	}
}

// GetHealthDiagnosis 獲取叢集健康診斷
// @Summary 獲取叢集健康診斷
// @Description 對叢集進行全面健康診斷，返回健康評分、風險項和診斷建議
// @Tags O&M
// @Accept json
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Success 200 {object} models.HealthDiagnosisResponse
// @Router /api/v1/clusters/{clusterID}/om/health-diagnosis [get]
func (h *OMHandler) GetHealthDiagnosis(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取叢集資訊
	cluster, err := h.clusterSvc.GetCluster(uint(clusterID))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err)
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	if clientset == nil {
		response.InternalError(c, "獲取K8s客戶端失敗")
		return
	}

	// 執行健康診斷
	result, err := h.omSvc.GetHealthDiagnosis(c.Request.Context(), clientset, uint(clusterID))
	if err != nil {
		logger.Error("執行健康診斷失敗", "error", err)
		response.InternalError(c, "執行健康診斷失敗: "+err.Error())
		return
	}

	response.OK(c, result)
}

// GetResourceTop 獲取資源消耗 Top N
// @Summary 獲取資源消耗 Top N
// @Description 獲取指定資源型別的消耗排行榜
// @Tags O&M
// @Accept json
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Param type query string true "資源型別" Enums(cpu, memory, disk, network)
// @Param level query string true "統計級別" Enums(namespace, workload, pod)
// @Param limit query int false "返回數量" default(10)
// @Success 200 {object} models.ResourceTopResponse
// @Router /api/v1/clusters/{clusterID}/om/resource-top [get]
func (h *OMHandler) GetResourceTop(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 解析請求參數
	var req models.ResourceTopRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	// 設定預設值
	if req.Limit <= 0 {
		req.Limit = 10
	}

	// 獲取叢集資訊
	cluster, err := h.clusterSvc.GetCluster(uint(clusterID))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err)
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	if clientset == nil {
		response.InternalError(c, "獲取K8s客戶端失敗")
		return
	}

	// 獲取資源 Top N
	result, err := h.omSvc.GetResourceTop(c.Request.Context(), clientset, uint(clusterID), &req)
	if err != nil {
		logger.Error("獲取資源Top N失敗", "error", err)
		response.InternalError(c, "獲取資源Top N失敗: "+err.Error())
		return
	}

	response.OK(c, result)
}

// GetControlPlaneStatus 獲取控制面元件狀態
// @Summary 獲取控制面元件狀態
// @Description 獲取叢集控制面元件（apiserver, scheduler, controller-manager, etcd）的狀態
// @Tags O&M
// @Accept json
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Success 200 {object} models.ControlPlaneStatusResponse
// @Router /api/v1/clusters/{clusterID}/om/control-plane-status [get]
func (h *OMHandler) GetControlPlaneStatus(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取叢集資訊
	cluster, err := h.clusterSvc.GetCluster(uint(clusterID))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err)
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	if clientset == nil {
		response.InternalError(c, "獲取K8s客戶端失敗")
		return
	}

	// 獲取控制面狀態
	result, err := h.omSvc.GetControlPlaneStatus(c.Request.Context(), clientset, uint(clusterID))
	if err != nil {
		logger.Error("獲取控制面狀態失敗", "error", err)
		response.InternalError(c, "獲取控制面狀態失敗: "+err.Error())
		return
	}

	response.OK(c, result)
}
