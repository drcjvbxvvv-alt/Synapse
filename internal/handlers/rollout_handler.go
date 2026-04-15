package handlers

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	rollouts "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	sigsyaml "sigs.k8s.io/yaml"
)

func init() {
	// 註冊Argo Rollouts型別到scheme
	_ = rollouts.AddToScheme(scheme.Scheme)
}

// RolloutHandler Rollout處理器
type RolloutHandler struct {
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewRolloutHandler 建立Rollout處理器
func NewRolloutHandler(cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *RolloutHandler {
	return &RolloutHandler{
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// CheckRolloutCRD 檢查叢集是否安裝了 Argo Rollouts CRD
func (h *RolloutHandler) CheckRolloutCRD(c *gin.Context) {
	clusterId := c.Param("clusterID")

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "failed to get K8s client: "+err.Error())
		return
	}

	// 清除 Discovery 快取，強制重新偵測
	h.k8sMgr.InvalidateDiscoveryCache(cluster.ID)

	clientset := k8sClient.GetClientset()
	installed := false
	version := ""
	for _, v := range []string{"v1alpha1"} {
		_, discErr := clientset.Discovery().ServerResourcesForGroupVersion("argoproj.io/" + v)
		if discErr == nil {
			installed = true
			version = v
			break
		}
	}

	response.OK(c, gin.H{"installed": installed, "version": version})
}

// ListRollouts 獲取Rollout列表
func (h *RolloutHandler) ListRollouts(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Query("namespace")
	searchName := c.Query("search")
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	logger.Info("獲取Rollout列表: cluster=%s, namespace=%s, search=%s", clusterId, namespace, searchName)

	// 獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 確保 informer 快取就緒
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	var rolloutList []RolloutInfo
	sel := labels.Everything()

	// 檢查 Argo Rollouts CRD 是否存在
	lister := h.k8sMgr.RolloutsLister(cluster.ID)
	if lister == nil {
		// 叢集未安裝 Argo Rollouts CRD，返回空列表
		response.OK(c, gin.H{
			"items":           []RolloutInfo{},
			"total":           0,
			"page":            page,
			"pageSize":        pageSize,
			"rolloutEnabled":  false,
			"rolloutDisabled": true,
		})
		return
	}

	// 檢查命名空間權限
	nsInfo, hasAccess := middleware.CheckNamespacePermission(c, namespace)
	if !hasAccess {
		middleware.ForbiddenNS(c, nsInfo)
		return
	}

	// 從Informer快取讀取
	if namespace != "" {
		rs, err := lister.Rollouts(namespace).List(sel)
		if err != nil {
			logger.Error("讀取Rollout快取失敗", "error", err)
		} else {
			for _, r := range rs {
				rolloutList = append(rolloutList, convertToRolloutInfo(r))
			}
		}
	} else {
		rs, err := lister.List(sel)
		if err != nil {
			logger.Error("讀取Rollout快取失敗", "error", err)
		} else {
			for _, r := range rs {
				rolloutList = append(rolloutList, convertToRolloutInfo(r))
			}
		}
	}

	// 根據命名空間權限過濾
	if !nsInfo.HasAllAccess && namespace == "" {
		rolloutList = middleware.FilterResourcesByNamespace(c, rolloutList, func(ro RolloutInfo) string {
			return ro.Namespace
		})
	}

	// 搜尋過濾
	if searchName != "" {
		var filtered []RolloutInfo
		searchLower := strings.ToLower(searchName)
		for _, ro := range rolloutList {
			if strings.Contains(strings.ToLower(ro.Name), searchLower) {
				filtered = append(filtered, ro)
			}
		}
		rolloutList = filtered
	}

	// 排序
	sort.Slice(rolloutList, func(i, j int) bool {
		return rolloutList[i].CreatedAt.After(rolloutList[j].CreatedAt)
	})

	// 分頁
	total := len(rolloutList)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedRollouts := rolloutList[start:end]

	response.PagedList(c, pagedRollouts, int64(total), page, pageSize)
}

// GetRollout 獲取Rollout詳情
func (h *RolloutHandler) GetRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout詳情: %s/%s/%s", clusterId, namespace, name)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	rollout, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Rollout不存在: "+err.Error())
		return
	}

	// 獲取關聯的Pods
	clientset := k8sClient.GetClientset()
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(rollout.Spec.Selector),
	})
	if err != nil {
		logger.Error("獲取Rollout關聯Pods失敗", "error", err)
	}

	// 清理 managed fields 以生成更乾淨的 YAML
	cleanRollout := rollout.DeepCopy()
	cleanRollout.ManagedFields = nil
	cleanRollout.Annotations = filterAnnotations(cleanRollout.Annotations)
	// 設定 TypeMeta（client-go 返回的物件預設不包含 apiVersion 和 kind）
	cleanRollout.APIVersion = "argoproj.io/v1alpha1"
	cleanRollout.Kind = "Rollout"
	// 將 Rollout 物件轉換為 YAML 字串
	yamlBytes, yamlErr := sigsyaml.Marshal(cleanRollout)
	var yamlString string
	if yamlErr == nil {
		yamlString = string(yamlBytes)
	} else {
		logger.Error("轉換Rollout為YAML失敗", "error", yamlErr)
		yamlString = ""
	}

	response.OK(c, gin.H{
		"workload": convertToRolloutInfo(rollout),
		"raw":      cleanRollout,
		"yaml":     yamlString,
		"pods":     pods,
	})
}

// GetRolloutNamespaces 獲取包含Rollout的命名空間列表
func (h *RolloutHandler) GetRolloutNamespaces(c *gin.Context) {
	clusterId := c.Param("clusterID")

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 確保 informer 快取就緒
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	// 從Informer讀取所有Rollouts並統計命名空間
	sel := labels.Everything()
	lister := h.k8sMgr.RolloutsLister(cluster.ID)
	if lister == nil {
		// 叢集未安裝 Argo Rollouts CRD，返回空列表
		response.OK(c, []interface{}{})
		return
	}
	rs, err := lister.List(sel)
	if err != nil {
		response.InternalError(c, "讀取Rollout快取失敗: "+err.Error())
		return
	}

	// 統計每個命名空間的Rollout數量
	nsCount := make(map[string]int)
	for _, ro := range rs {
		nsCount[ro.Namespace]++
	}

	// 轉換為列表格式
	type NamespaceInfo struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	var namespaces []NamespaceInfo
	for ns, count := range nsCount {
		namespaces = append(namespaces, NamespaceInfo{
			Name:  ns,
			Count: count,
		})
	}

	// 按名稱排序
	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})

	response.OK(c, namespaces)
}

// ScaleRollout 擴縮容Rollout
func (h *RolloutHandler) ScaleRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req ScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	logger.Info("擴縮容Rollout: %s/%s/%s to %d", clusterId, namespace, name, req.Replicas)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	// 獲取Rollout
	rollout, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.InternalError(c, "獲取Rollout失敗: "+err.Error())
		return
	}

	// 更新副本數
	rollout.Spec.Replicas = &req.Replicas
	_, err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Update(ctx, rollout, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "擴縮容失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "擴縮容成功"})
}

// DeleteRollout 刪除Rollout
func (h *RolloutHandler) DeleteRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("刪除Rollout: %s/%s/%s", clusterId, namespace, name)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		response.InternalError(c, "刪除失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "刪除成功"})
}
