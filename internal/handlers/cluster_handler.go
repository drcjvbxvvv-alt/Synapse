package handlers

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ClusterHandler 叢集處理器
//
// P0-4b: the raw *gorm.DB field was removed. All persistence goes through
// the injected services (ClusterService, MonitoringConfigService, …). If a
// handler method needs a new query, add it to the right service instead of
// re-adding h.db.
type ClusterHandler struct {
	cfg              *config.Config
	clusterService   *services.ClusterService
	k8sMgr           *k8s.ClusterInformerManager
	promService      *services.PrometheusService
	monitoringCfgSvc *services.MonitoringConfigService
	permissionSvc    *services.PermissionService
}

// NewClusterHandler 建立叢集處理器
//
// P0-4b: accepts a shared *services.ClusterService instead of building its
// own. Previously the handler instantiated NewClusterService(db) directly,
// which defeated dependency injection and, post-P0-4b, would have required
// the handler to know about the Repository layer. Now the router passes
// d.clusterSvc so Repository/flag wiring stays in one place.
func NewClusterHandler(
	cfg *config.Config,
	mgr *k8s.ClusterInformerManager,
	clusterSvc *services.ClusterService,
	promService *services.PrometheusService,
	monitoringCfgSvc *services.MonitoringConfigService,
	permSvc *services.PermissionService,
) *ClusterHandler {
	return &ClusterHandler{
		cfg:              cfg,
		clusterService:   clusterSvc,
		k8sMgr:           mgr,
		promService:      promService,
		monitoringCfgSvc: monitoringCfgSvc,
		permissionSvc:    permSvc,
	}
}

// maskURL 只保留 scheme + host，截斷路徑與 query string，防止含 token 的 URL 洩入日誌。
// 若解析失敗則返回原字串（截斷至 64 字元）。
func maskURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		if len(raw) > 64 {
			return raw[:64] + "…"
		}
		return raw
	}
	return u.Scheme + "://" + u.Host
}

// GetClusters 獲取叢集列表（按使用者權限過濾，支援分頁 page/pageSize）
//
// @Summary     取得叢集列表
// @Tags        clusters
// @Produce     json
// @Security    BearerAuth
// @Param       page     query int    false "頁碼"
// @Param       pageSize query int    false "每頁筆數（最大 200）"
// @Param       search   query string false "叢集名稱搜尋"
// @Success     200 {object} response.PagedListResult
// @Failure     401 {object} response.ErrorBody
// @Router      /clusters [get]
func (h *ClusterHandler) GetClusters(c *gin.Context) {
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	userID := c.GetUint("user_id")

	// 獲取使用者可訪問的叢集
	clusters, err := h.getAccessibleClusters(c.Request.Context(), userID)
	if err != nil {
		logger.Error("獲取叢集列表失敗", "error", err)
		response.InternalError(c, "獲取叢集列表失敗: "+err.Error())
		return
	}

	// 轉換為響應格式
	clusterList := make([]gin.H, 0, len(clusters))
	for _, cluster := range clusters {
		clusterData := gin.H{
			"id":        cluster.ID,
			"name":      cluster.Name,
			"apiServer": cluster.APIServer,
			"version":   cluster.Version,
			"status":    cluster.Status,
			"createdAt": cluster.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"updatedAt": cluster.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}

		if cluster.LastHeartbeat != nil {
			clusterData["lastHeartbeat"] = cluster.LastHeartbeat.Format("2006-01-02T15:04:05Z")
		}

		// 獲取實時節點資訊和指標
		if h.k8sMgr != nil {
			nodeCount, readyNodes := h.getClusterNodeInfo(c.Request.Context(), cluster)
			clusterData["nodeCount"] = nodeCount
			clusterData["readyNodes"] = readyNodes
		} else {
			clusterData["nodeCount"] = 0
			clusterData["readyNodes"] = 0
		}

		// 獲取叢集 CPU、記憶體使用率
		cpuUsage, memoryUsage := h.getClusterResourceUsage(c.Request.Context(), cluster)
		clusterData["cpuUsage"] = cpuUsage
		clusterData["memoryUsage"] = memoryUsage

		clusterList = append(clusterList, clusterData)
	}

	total := int64(len(clusterList))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > len(clusterList) {
		start = len(clusterList)
	}
	if end > len(clusterList) {
		end = len(clusterList)
	}
	response.PagedList(c, clusterList[start:end], total, page, pageSize)
}

// GetCluster 獲取叢集詳情
//
// @Summary     取得叢集詳情
// @Tags        clusters
// @Produce     json
// @Security    BearerAuth
// @Param       clusterID path int true "叢集 ID"
// @Success     200 {object} models.Cluster
// @Failure     401 {object} response.ErrorBody
// @Failure     404 {object} response.ErrorBody
// @Router      /clusters/{clusterID} [get]
func (h *ClusterHandler) GetCluster(c *gin.Context) {
	idStr := c.Param("clusterID")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(id))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	clusterData := gin.H{
		"id":        cluster.ID,
		"name":      cluster.Name,
		"apiServer": cluster.APIServer,
		"version":   cluster.Version,
		"status":    cluster.Status,
		"createdAt": cluster.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updatedAt": cluster.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if cluster.LastHeartbeat != nil {
		clusterData["lastHeartbeat"] = cluster.LastHeartbeat.Format("2006-01-02T15:04:05Z")
	}

	response.OK(c, clusterData)
}

// DeleteCluster 刪除叢集
//
// @Summary     刪除叢集（平台管理員）
// @Tags        clusters
// @Produce     json
// @Security    BearerAuth
// @Param       clusterID path int true "叢集 ID"
// @Success     200
// @Failure     403 {object} response.ErrorBody
// @Failure     404 {object} response.ErrorBody
// @Router      /clusters/{clusterID} [delete]
func (h *ClusterHandler) DeleteCluster(c *gin.Context) {
	idStr := c.Param("clusterID")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	clusterID := uint(id)

	// 先停止叢集的 informer/watch，避免刪除後繼續 watch 導致錯誤
	if h.k8sMgr != nil {
		h.k8sMgr.StopForCluster(clusterID)
	}

	err = h.clusterService.DeleteCluster(c.Request.Context(), clusterID)
	if err != nil {
		// 檢查是否是叢集不存在的錯誤
		if strings.Contains(err.Error(), "叢集不存在") {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, nil)
}

// GetClusterStats 獲取叢集統計
//
// @Summary     取得叢集統計摘要（總叢集數、節點數、Pod 數等）
// @Tags        clusters
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} models.ClusterStats
// @Failure     401 {object} response.ErrorBody
// @Router      /clusters/stats [get]
func (h *ClusterHandler) GetClusterStats(c *gin.Context) {
	logger.Info("獲取叢集統計")

	stats, err := h.clusterService.GetClusterStats(c.Request.Context())
	if err != nil {
		logger.Error("獲取叢集統計失敗", "error", err)
		response.InternalError(c, "獲取叢集統計失敗: "+err.Error())
		return
	}

	response.OK(c, stats)
}

// GetClusterStatus 獲取叢集實時狀態
func (h *ClusterHandler) GetClusterStatus(c *gin.Context) {
	idStr := c.Param("clusterID")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(id))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	// 獲取實時節點資訊
	nodeCount, readyNodes := h.getClusterNodeInfo(c.Request.Context(), cluster)

	statusData := gin.H{
		"id":         cluster.ID,
		"name":       cluster.Name,
		"status":     cluster.Status,
		"nodeCount":  nodeCount,
		"readyNodes": readyNodes,
		"version":    cluster.Version,
	}

	response.OK(c, statusData)
}

// getAccessibleClusters 獲取使用者可訪問的叢集列表
//
// P0-4b: the "filter by IDs" branch used to run a raw h.db.Where("id IN ?")
// query straight from the handler. That has been pushed down into
// ClusterService.GetClustersByIDs so the handler no longer touches the DB
// directly and the Repository layer can own the IN-clause translation.
func (h *ClusterHandler) getAccessibleClusters(ctx context.Context, userID uint) ([]*models.Cluster, error) {
	clusterIDs, isAll, err := h.permissionSvc.GetUserAccessibleClusterIDs(userID)
	if err != nil {
		return nil, err
	}
	if isAll {
		return h.clusterService.GetAllClusters(ctx)
	}
	if len(clusterIDs) == 0 {
		return []*models.Cluster{}, nil
	}
	return h.clusterService.GetClustersByIDs(ctx, clusterIDs)
}

// maxInt 返回較大的整數，避免出現負數（例如 worker = total - 1）
//
//nolint:unused // 保留用於未來使用
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
