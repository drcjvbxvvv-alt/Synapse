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

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	sigsyaml "sigs.k8s.io/yaml"
)

// StatefulSetHandler StatefulSet處理器
type StatefulSetHandler struct {
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewStatefulSetHandler 建立StatefulSet處理器
func NewStatefulSetHandler(cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *StatefulSetHandler {
	return &StatefulSetHandler{
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// StatefulSetInfo StatefulSet資訊
type StatefulSetInfo struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Type            string            `json:"type"`
	Status          string            `json:"status"`
	Replicas        int32             `json:"replicas"`
	ReadyReplicas   int32             `json:"readyReplicas"`
	CurrentReplicas int32             `json:"currentReplicas"`
	UpdatedReplicas int32             `json:"updatedReplicas"`
	Labels          map[string]string `json:"labels"`
	Annotations     map[string]string `json:"annotations"`
	CreatedAt       time.Time         `json:"createdAt"`
	Images          []string          `json:"images"`
	Selector        map[string]string `json:"selector"`
	ServiceName     string            `json:"serviceName"`
}

// ListStatefulSets 獲取StatefulSet列表
func (h *StatefulSetHandler) ListStatefulSets(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Query("namespace")
	searchName := c.Query("search")
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	logger.Info("獲取StatefulSet列表: cluster=%s, namespace=%s, search=%s", clusterId, namespace, searchName)

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

	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	// 檢查命名空間權限
	nsInfo, hasAccess := middleware.CheckNamespacePermission(c, namespace)
	if !hasAccess {
		middleware.ForbiddenNS(c, nsInfo)
		return
	}

	var statefulSets []StatefulSetInfo
	sel := labels.Everything()

	if namespace != "" {
		sss, err := h.k8sMgr.StatefulSetsLister(cluster.ID).StatefulSets(namespace).List(sel)
		if err != nil {
			logger.Error("讀取StatefulSet快取失敗", "error", err)
		} else {
			for _, ss := range sss {
				statefulSets = append(statefulSets, h.convertToStatefulSetInfo(ss))
			}
		}
	} else {
		sss, err := h.k8sMgr.StatefulSetsLister(cluster.ID).List(sel)
		if err != nil {
			logger.Error("讀取StatefulSet快取失敗", "error", err)
		} else {
			for _, ss := range sss {
				statefulSets = append(statefulSets, h.convertToStatefulSetInfo(ss))
			}
		}
	}

	// 根據命名空間權限過濾
	if !nsInfo.HasAllAccess && namespace == "" {
		statefulSets = middleware.FilterResourcesByNamespace(c, statefulSets, func(ss StatefulSetInfo) string {
			return ss.Namespace
		})
	}

	if searchName != "" {
		var filtered []StatefulSetInfo
		searchLower := strings.ToLower(searchName)
		for _, ss := range statefulSets {
			if strings.Contains(strings.ToLower(ss.Name), searchLower) {
				filtered = append(filtered, ss)
			}
		}
		statefulSets = filtered
	}

	sort.Slice(statefulSets, func(i, j int) bool {
		return statefulSets[i].CreatedAt.After(statefulSets[j].CreatedAt)
	})

	total := len(statefulSets)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedStatefulSets := statefulSets[start:end]

	response.PagedList(c, pagedStatefulSets, int64(total), page, pageSize)
}

// GetStatefulSet 獲取StatefulSet詳情
func (h *StatefulSetHandler) GetStatefulSet(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取StatefulSet詳情: %s/%s/%s", clusterId, namespace, name)

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
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	statefulSet, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "StatefulSet不存在: "+err.Error())
		return
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(statefulSet.Spec.Selector),
	})
	if err != nil {
		logger.Error("獲取StatefulSet關聯Pods失敗", "error", err)
	}

	// 清理 managed fields 以生成更乾淨的 YAML
	cleanStatefulSet := statefulSet.DeepCopy()
	cleanStatefulSet.ManagedFields = nil
	cleanStatefulSet.Annotations = filterAnnotations(cleanStatefulSet.Annotations)
	// 設定 TypeMeta（client-go 返回的物件預設不包含 apiVersion 和 kind）
	cleanStatefulSet.APIVersion = "apps/v1"
	cleanStatefulSet.Kind = "StatefulSet"
	// 將 StatefulSet 物件轉換為 YAML 字串
	yamlBytes, yamlErr := sigsyaml.Marshal(cleanStatefulSet)
	var yamlString string
	if yamlErr == nil {
		yamlString = string(yamlBytes)
	} else {
		logger.Error("轉換StatefulSet為YAML失敗", "error", yamlErr)
		yamlString = ""
	}

	response.OK(c, gin.H{
		"workload": h.convertToStatefulSetInfo(statefulSet),
		"raw":      statefulSet,
		"yaml":     yamlString,
		"pods":     pods,
	})
}

// GetStatefulSetNamespaces 獲取包含StatefulSet的命名空間列表
func (h *StatefulSetHandler) GetStatefulSetNamespaces(c *gin.Context) {
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

	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	sel := labels.Everything()
	sss, err := h.k8sMgr.StatefulSetsLister(cluster.ID).List(sel)
	if err != nil {
		response.InternalError(c, "讀取StatefulSet快取失敗: "+err.Error())
		return
	}

	nsCount := make(map[string]int)
	for _, ss := range sss {
		nsCount[ss.Namespace]++
	}

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

	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})

	response.OK(c, namespaces)
}
