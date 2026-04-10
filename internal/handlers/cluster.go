package handlers

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

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
			nodeCount, readyNodes := h.getClusterNodeInfo(cluster)
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

// ImportCluster 匯入叢集
//
// @Summary     匯入叢集（平台管理員）
// @Tags        clusters
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} models.Cluster
// @Failure     400 {object} response.ErrorBody
// @Failure     403 {object} response.ErrorBody
// @Router      /clusters/import [post]
func (h *ClusterHandler) ImportCluster(c *gin.Context) {
	logger.Info("匯入叢集")

	// 獲取請求參數
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		ApiServer   string `json:"apiServer"`
		Kubeconfig  string `json:"kubeconfig"`
		Token       string `json:"token"`
		CaCert      string `json:"caCert"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	logger.Info("叢集匯入請求", "name", req.Name, "apiServer", maskURL(req.ApiServer))

	// 驗證參數
	if req.Kubeconfig == "" && (req.ApiServer == "" || req.Token == "") {
		response.BadRequest(c, "請提供kubeconfig或者API Server地址和訪問令牌")
		return
	}

	var k8sClient *services.K8sClient
	var err error

	// 根據提供的參數建立Kubernetes客戶端
	if req.Kubeconfig != "" {
		k8sClient, err = services.NewK8sClientFromKubeconfig(req.Kubeconfig)
		if err != nil {
			logger.Error("從kubeconfig建立客戶端失敗", "error", err)
			response.BadRequest(c, fmt.Sprintf("kubeconfig格式錯誤: %v", err))
			return
		}
	} else {
		k8sClient, err = services.NewK8sClientFromToken(req.ApiServer, req.Token, req.CaCert)
		if err != nil {
			logger.Error("從Token建立客戶端失敗", "error", err)
			response.BadRequest(c, fmt.Sprintf("連線配置錯誤: %v", err))
			return
		}
	}

	// 測試連線
	clusterInfo, err := k8sClient.TestConnection()
	if err != nil {
		logger.Error("連線測試失敗", "error", err)
		response.BadRequest(c, fmt.Sprintf("連線測試失敗: %v", err))
		return
	}

	// 獲取 API Server 地址：如果使用 kubeconfig，從配置中解析
	apiServer := req.ApiServer
	if apiServer == "" && req.Kubeconfig != "" {
		// 從 kubeconfig 解析出的配置中獲取 API Server 地址
		restConfig := k8sClient.GetRestConfig()
		if restConfig != nil && restConfig.Host != "" {
			apiServer = restConfig.Host
			logger.Info("從 kubeconfig 中解析出 API Server", "apiServer", maskURL(apiServer))
		}
	}

	// P2-2：RBAC 危險程度評估（非阻塞，失敗不影響匯入）
	rbacCtx, rbacCancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer rbacCancel()
	rbacSummary := k8sClient.CheckRBACSummary(rbacCtx)

	// P2-1：取得 API Server 憑證到期日（非阻塞，失敗不影響匯入）
	var certExpireAt *time.Time
	if expiry, err := k8sClient.GetAPIServerCertExpiry(); err == nil {
		certExpireAt = expiry
	} else {
		logger.Warn("無法取得 API Server 憑證到期日", "error", err)
	}

	// 建立叢集模型
	cluster := &models.Cluster{
		Name:               req.Name,
		APIServer:          apiServer,
		KubeconfigEnc:      req.Kubeconfig,
		SATokenEnc:         req.Token,
		CAEnc:              req.CaCert,
		Version:            clusterInfo.Version,
		Status:             clusterInfo.Status,
		Labels:             "{}",
		MonitoringConfig:   "{}",
		AlertManagerConfig: "{}",
		CertExpireAt:       certExpireAt,
		CreatedBy:          1, // 臨時設定為1，後續需要從JWT中獲取使用者ID
	}

	// 儲存到資料庫
	err = h.clusterService.CreateCluster(c.Request.Context(), cluster)
	if err != nil {
		logger.Error("儲存叢集資訊失敗", "error", err)
		response.InternalError(c, "儲存叢集資訊失敗: "+err.Error())
		return
	}

	// 返回新建立的叢集資訊（含 RBAC 警告供前端提示）
	newCluster := gin.H{
		"id":           cluster.ID,
		"name":         cluster.Name,
		"apiServer":    cluster.APIServer,
		"version":      cluster.Version,
		"status":       cluster.Status,
		"createdAt":    cluster.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"rbacWarnings": rbacSummary,
	}
	if certExpireAt != nil {
		newCluster["certExpireAt"] = certExpireAt.Format("2006-01-02T15:04:05Z")
	}

	response.OK(c, newCluster)
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
	nodeCount, readyNodes := h.getClusterNodeInfo(cluster)

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

// GetClusterOverview 獲取叢集概覽資訊
func (h *ClusterHandler) GetClusterOverview(c *gin.Context) {
	clusterID := c.Param("clusterID")
	id, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(uint(id))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 優先使用 Informer 快取（方案C）
	// 確保本叢集的 informer 已初始化並啟動
	if _, err := h.k8sMgr.EnsureForCluster(cluster); err == nil {
		if snap, err := h.k8sMgr.GetOverviewSnapshot(c.Request.Context(), cluster.ID); err == nil {
			// 獲取容器子網IP資訊
			containerSubnetIPs, err := h.getContainerSubnetIPs(c.Request.Context(), cluster)
			if err != nil {
				logger.Error("獲取容器子網IP資訊失敗", "error", err)
				// 不返回錯誤，只是不顯示容器子網資訊
			} else {
				// 轉換型別
				snap.ContainerSubnetIPs = &k8s.ContainerSubnetIPs{
					TotalIPs:     containerSubnetIPs.TotalIPs,
					UsedIPs:      containerSubnetIPs.UsedIPs,
					AvailableIPs: containerSubnetIPs.AvailableIPs,
				}
			}

			response.OK(c, snap)
			return
		}
	}

	// 如果 informer 方式失敗，返回錯誤
	response.ServiceUnavailable(c, "叢集資訊獲取失敗")
}

// getContainerSubnetIPs 獲取容器子網IP資訊
func (h *ClusterHandler) getContainerSubnetIPs(ctx context.Context, cluster *models.Cluster) (*models.ContainerSubnetIPs, error) {
	// 獲取監控配置（使用注入的服務，不再從 h.db 重新構造）
	config, err := h.monitoringCfgSvc.GetMonitoringConfig(cluster.ID)
	if err != nil {
		return nil, fmt.Errorf("獲取監控配置失敗: %w", err)
	}

	// 如果監控功能被禁用，返回空資訊
	if config.Type == "disabled" {
		return nil, fmt.Errorf("監控功能已禁用")
	}

	// 查詢容器子網IP資訊（使用注入的 Prometheus 服務）
	return h.promService.QueryContainerSubnetIPs(ctx, config)
}

/*
*
GetClusterEvents 獲取叢集 K8s 事件列表
GET /api/v1/clusters/:clusterID/events?search=xxx&type=Normal|Warning
返回前端定義的 K8sEvent 陣列（不分頁）
*/
func (h *ClusterHandler) GetClusterEvents(c *gin.Context) {
	idStr := c.Param("clusterID")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(id))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err)
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	cs := k8sClient.GetClientset()

	// 拉取所有命名空間的 core/v1 Event
	evList, err := cs.CoreV1().Events("").List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取K8s事件失敗", "error", err)
		response.InternalError(c, "獲取K8s事件失敗: "+err.Error())
		return
	}

	search := strings.TrimSpace(c.Query("search"))
	ftype := strings.TrimSpace(c.Query("type"))

	out := make([]gin.H, 0, len(evList.Items))
	for _, e := range evList.Items {
		// 型別過濾
		if ftype != "" && !strings.EqualFold(e.Type, ftype) {
			continue
		}
		// 關鍵字過濾（物件kind/name/ns、reason、message）
		if search != "" {
			s := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(e.InvolvedObject.Kind), s) &&
				!strings.Contains(strings.ToLower(e.InvolvedObject.Name), s) &&
				!strings.Contains(strings.ToLower(e.InvolvedObject.Namespace), s) &&
				!strings.Contains(strings.ToLower(e.Reason), s) &&
				!strings.Contains(strings.ToLower(e.Message), s) {
				continue
			}
		}

		// 發生時間優先順序：lastTimestamp > eventTime > firstTimestamp > metadata.creationTimestamp
		var lastTS string
		if !e.LastTimestamp.IsZero() {
			lastTS = e.LastTimestamp.Time.UTC().Format(time.RFC3339)
		} else if !e.EventTime.IsZero() {
			lastTS = e.EventTime.Time.UTC().Format(time.RFC3339)
		} else if !e.FirstTimestamp.IsZero() {
			lastTS = e.FirstTimestamp.Time.UTC().Format(time.RFC3339)
		} else if !e.CreationTimestamp.IsZero() {
			lastTS = e.ObjectMeta.CreationTimestamp.Time.UTC().Format(time.RFC3339)
		}

		out = append(out, gin.H{
			"metadata": gin.H{
				"uid":       string(e.UID),
				"name":      e.Name,
				"namespace": e.Namespace,
				"creationTimestamp": func() string {
					if e.CreationTimestamp.IsZero() {
						return ""
					}
					return e.CreationTimestamp.Time.UTC().Format(time.RFC3339)
				}(),
			},
			"involvedObject": gin.H{
				"kind":       e.InvolvedObject.Kind,
				"name":       e.InvolvedObject.Name,
				"namespace":  e.InvolvedObject.Namespace,
				"uid":        string(e.InvolvedObject.UID),
				"apiVersion": e.InvolvedObject.APIVersion,
				"fieldPath":  e.InvolvedObject.FieldPath,
			},
			"type":    e.Type,
			"reason":  e.Reason,
			"message": e.Message,
			"source":  gin.H{"component": e.Source.Component, "host": e.Source.Host},
			"firstTimestamp": func() string {
				if e.FirstTimestamp.IsZero() {
					return ""
				}
				return e.FirstTimestamp.Time.UTC().Format(time.RFC3339)
			}(),
			"lastTimestamp": lastTS,
			"eventTime": func() string {
				if e.EventTime.IsZero() {
					return ""
				}
				return e.EventTime.Time.UTC().Format(time.RFC3339)
			}(),
			"count": e.Count,
		})
	}

	response.OK(c, out)
}

// GetClusterMetrics 獲取叢集監控資料
func (h *ClusterHandler) GetClusterMetrics(c *gin.Context) {
	id := c.Param("clusterID")
	logger.Info("獲取叢集監控資料: %s", id)

	// 獲取請求參數
	rangeParam := c.DefaultQuery("range", "1h")
	step := c.DefaultQuery("step", "1m")

	// 從資料庫獲取叢集
	clusterID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(clusterID))
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err)
		response.InternalError(c, "獲取叢集監控資料失敗: "+err.Error())
		return
	}

	// 獲取叢集監控資料
	metrics, err := k8sClient.GetClusterMetrics(rangeParam, step)
	if err != nil {
		logger.Error("獲取叢集監控資料失敗", "error", err)
		response.InternalError(c, "獲取叢集監控資料失敗: "+err.Error())
		return
	}

	response.OK(c, metrics)
}

// TestConnection 測試叢集連線
//
// @Summary     測試叢集 API Server 連線
// @Tags        clusters
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200
// @Failure     400 {object} response.ErrorBody
// @Router      /clusters/test-connection [post]
func (h *ClusterHandler) TestConnection(c *gin.Context) {
	logger.Info("測試叢集連線")

	// 獲取請求參數
	var req struct {
		ApiServer  string `json:"apiServer"`
		Kubeconfig string `json:"kubeconfig"`
		Token      string `json:"token"`
		CaCert     string `json:"caCert"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("參數繫結錯誤: %v", err)
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	// 列印接收到的參數用於除錯

	// 驗證參數
	if req.Kubeconfig == "" && (req.ApiServer == "" || req.Token == "") {
		response.BadRequest(c, "請提供kubeconfig或者API Server地址和訪問令牌")
		return
	}

	var k8sClient *services.K8sClient
	var err error

	// 根據提供的參數建立Kubernetes客戶端
	if req.Kubeconfig != "" {
		// 使用kubeconfig建立客戶端
		k8sClient, err = services.NewK8sClientFromKubeconfig(req.Kubeconfig)
		if err != nil {
			logger.Error("從kubeconfig建立客戶端失敗", "error", err)
			response.BadRequest(c, fmt.Sprintf("kubeconfig格式錯誤: %v", err))
			return
		}
	} else {
		// 使用API Server和Token建立客戶端
		k8sClient, err = services.NewK8sClientFromToken(req.ApiServer, req.Token, req.CaCert)
		if err != nil {
			logger.Error("從Token建立客戶端失敗", "error", err)
			response.BadRequest(c, fmt.Sprintf("連線配置錯誤: %v", err))
			return
		}
	}

	// 測試連線並獲取叢集資訊
	clusterInfo, err := k8sClient.TestConnection()
	if err != nil {
		logger.Error("連線測試失敗", "error", err)
		response.BadRequest(c, fmt.Sprintf("連線測試失敗: %v", err))
		return
	}

	testResult := gin.H{
		"version":    clusterInfo.Version,
		"nodeCount":  clusterInfo.NodeCount,
		"readyNodes": clusterInfo.ReadyNodes,
		"status":     clusterInfo.Status,
	}

	response.OK(c, testResult)
}

// getClusterNodeInfo 獲取叢集節點資訊
func (h *ClusterHandler) getClusterNodeInfo(cluster *models.Cluster) (int, int) {
	// 使用 informer+lister 讀取節點並統計（不直連 API）
	if _, err := h.k8sMgr.EnsureAndWait(context.Background(), cluster, 5*time.Second); err != nil {
		logger.Error("informer 未就緒", "error", err)
		return 0, 0
	}
	nodes, err := h.k8sMgr.NodesLister(cluster.ID).List(labels.Everything())
	if err != nil {
		logger.Error("讀取節點快取失敗", "error", err)
		return 0, 0
	}
	nodeCount := len(nodes)
	readyNodes := 0
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				readyNodes++
				break
			}
		}
	}
	return nodeCount, readyNodes
}

// getClusterResourceUsage 獲取叢集 CPU 和記憶體使用率
func (h *ClusterHandler) getClusterResourceUsage(ctx context.Context, cluster *models.Cluster) (float64, float64) {
	if h.promService == nil || h.monitoringCfgSvc == nil {
		return 0, 0
	}

	// 獲取叢集的監控配置
	config, err := h.monitoringCfgSvc.GetMonitoringConfig(cluster.ID)
	if err != nil || config.Type == "disabled" {
		return 0, 0
	}

	// 設定時間範圍（最近 5 分鐘）
	now := time.Now().Unix()
	start := now - 300
	step := "1m"

	var cpuUsage, memoryUsage float64

	// 查詢 CPU 使用率
	cpuQuery := &models.MetricsQuery{
		Query: "(1 - avg(rate(node_cpu_seconds_total{mode=\"idle\"}[5m]))) * 100",
		Start: start,
		End:   now,
		Step:  step,
	}
	if resp, err := h.promService.QueryPrometheus(ctx, config, cpuQuery); err == nil {
		if val := extractLatestValueFromResponse(resp); val >= 0 {
			cpuUsage = val
		}
	}

	// 查詢記憶體使用率
	memQuery := &models.MetricsQuery{
		Query: "(1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)) * 100",
		Start: start,
		End:   now,
		Step:  step,
	}
	if resp, err := h.promService.QueryPrometheus(ctx, config, memQuery); err == nil {
		if val := extractLatestValueFromResponse(resp); val >= 0 {
			memoryUsage = val
		}
	}

	return cpuUsage, memoryUsage
}

// extractLatestValueFromResponse 從 Prometheus range query 響應中提取最新值
func extractLatestValueFromResponse(resp *models.MetricsResponse) float64 {
	if resp == nil || len(resp.Data.Result) == 0 {
		return -1
	}
	result := resp.Data.Result[0]
	// 優先從 Values (range query) 中獲取最後一個值
	if len(result.Values) > 0 {
		lastValue := result.Values[len(result.Values)-1]
		if len(lastValue) >= 2 {
			if strVal, ok := lastValue[1].(string); ok {
				var f float64
				if _, err := fmt.Sscanf(strVal, "%f", &f); err == nil {
				return f
				}
			}
		}
	}
	// 相容 instant query 的 Value 格式
	if len(result.Value) >= 2 {
		if val, ok := result.Value[1].(string); ok {
			var f float64
			if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
			return f
			}
		}
	}
	return -1
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
//nolint:unused // 保留用於未來使用
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
