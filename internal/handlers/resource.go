package handlers

import (
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
	"github.com/gin-gonic/gin"
)

// ResourceHandler 資源治理處理器（Phase 1：K8s API 即時計算）
type ResourceHandler struct {
	svc        *services.ResourceService
	clusterSvc *services.ClusterService
}

// NewResourceHandler 建立處理器
func NewResourceHandler(svc *services.ResourceService, clusterSvc *services.ClusterService) *ResourceHandler {
	return &ResourceHandler{svc: svc, clusterSvc: clusterSvc}
}

// GetSnapshot 取得叢集即時資源佔用快照
// GET /api/v1/clusters/:clusterID/resources/snapshot
func (h *ResourceHandler) GetSnapshot(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	snap, err := h.svc.GetSnapshot(cluster)
	if err != nil {
		logger.Warn("資源快照取得失敗（Informer 未就緒）", "cluster_id", clusterID, "error", err)
		response.ServiceUnavailable(c, "叢集連線中，請稍後再試")
		return
	}
	response.OK(c, snap)
}

// GetNamespaceOccupancy 取得各命名空間資源佔用明細
// GET /api/v1/clusters/:clusterID/resources/namespaces
func (h *ResourceHandler) GetNamespaceOccupancy(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	items, err := h.svc.GetNamespaceOccupancy(cluster)
	if err != nil {
		logger.Warn("命名空間佔用查詢失敗", "cluster_id", clusterID, "error", err)
		response.ServiceUnavailable(c, "叢集連線中，請稍後再試")
		return
	}
	response.OK(c, items)
}

// GetGlobalOverview 取得跨叢集全平台資源彙總
// GET /api/v1/resources/global/overview
func (h *ResourceHandler) GetGlobalOverview(c *gin.Context) {
	overview, err := h.svc.GetGlobalOverview()
	if err != nil {
		logger.Error("全局資源彙總失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, overview)
}

// GetNamespaceEfficiency 取得命名空間效率分析（佔用率 + Prometheus 使用量）
// GET /api/v1/clusters/:clusterID/resources/efficiency
func (h *ResourceHandler) GetNamespaceEfficiency(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	items, err := h.svc.GetNamespaceEfficiency(cluster)
	if err != nil {
		logger.Warn("命名空間效率查詢失敗", "cluster_id", clusterID, "error", err)
		response.ServiceUnavailable(c, "叢集連線中，請稍後再試")
		return
	}
	response.OK(c, items)
}

// GetWorkloadEfficiency 取得工作負載效率列表（分頁）
// GET /api/v1/clusters/:clusterID/resources/workloads?namespace=&page=&pageSize=
func (h *ResourceHandler) GetWorkloadEfficiency(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	namespace := c.Query("namespace")
	page := parseIntQuery(c, "page", 1)
	pageSize := parseIntQuery(c, "pageSize", 20)

	result, err := h.svc.GetWorkloadEfficiency(cluster, namespace, page, pageSize)
	if err != nil {
		logger.Warn("工作負載效率查詢失敗", "cluster_id", clusterID, "error", err)
		response.ServiceUnavailable(c, "叢集連線中，請稍後再試")
		return
	}
	response.OK(c, result)
}

// GetWasteWorkloads 取得低效工作負載列表
// GET /api/v1/clusters/:clusterID/resources/waste?cpu_threshold=0.2
func (h *ResourceHandler) GetWasteWorkloads(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	threshold := parseFloatQuery(c, "cpu_threshold", 0.2)
	items, err := h.svc.GetWasteWorkloads(cluster, threshold)
	if err != nil {
		logger.Warn("低效工作負載查詢失敗", "cluster_id", clusterID, "error", err)
		response.ServiceUnavailable(c, "叢集連線中，請稍後再試")
		return
	}
	response.OK(c, items)
}
