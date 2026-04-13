package handlers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// IngressHandler Ingress處理器
type IngressHandler struct {
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewIngressHandler 建立Ingress處理器
func NewIngressHandler(cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *IngressHandler {
	return &IngressHandler{
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// ListIngresses 獲取Ingress列表
func (h *IngressHandler) ListIngresses(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取查詢參數
	namespace := c.DefaultQuery("namespace", "")
	ingressClass := c.DefaultQuery("ingressClass", "")
	search := c.DefaultQuery("search", "")
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	// 檢查命名空間權限
	nsInfo, hasAccess := middleware.CheckNamespacePermission(c, namespace)
	if !hasAccess {
		middleware.ForbiddenNS(c, nsInfo)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// 獲取Ingresses
	ingresses, err := h.getIngresses(ctx, clientset, namespace)
	if err != nil {
		logger.Error("獲取Ingresses失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取Ingresses失敗: %v", err))
		return
	}

	// 根據命名空間權限過濾
	if !nsInfo.HasAllAccess && namespace == "" {
		ingresses = middleware.FilterResourcesByNamespace(c, ingresses, func(i IngressInfo) string {
			return i.Namespace
		})
	}

	// 過濾和搜尋
	filteredIngresses := h.filterIngresses(ingresses, ingressClass, search)

	// 排序
	sort.Slice(filteredIngresses, func(i, j int) bool {
		return filteredIngresses[i].CreatedAt.After(filteredIngresses[j].CreatedAt)
	})

	// 分頁
	total := len(filteredIngresses)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedIngresses := filteredIngresses[start:end]

	response.PagedList(c, pagedIngresses, int64(total), page, pageSize)
}

// GetIngress 獲取單個Ingress詳情
func (h *IngressHandler) GetIngress(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Ingress
	ingress, err := clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Ingress失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取Ingress失敗: %v", err))
		return
	}

	ingressInfo := h.convertToIngressInfo(ingress)

	response.OK(c, ingressInfo)
}

// GetIngressYAML 獲取Ingress的YAML
func (h *IngressHandler) GetIngressYAML(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Ingress
	ingress, err := clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Ingress失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取Ingress失敗: %v", err))
		return
	}

	// 設定 apiVersion 和 kind（API 返回的物件不包含這些欄位）
	cleanIng := ingress.DeepCopy()
	cleanIng.APIVersion = "networking.k8s.io/v1"
	cleanIng.Kind = "Ingress"
	cleanIng.ManagedFields = nil
	cleanIng.Annotations = filterAnnotations(cleanIng.Annotations)

	// 轉換為YAML
	yamlData, err := yaml.Marshal(cleanIng)
	if err != nil {
		logger.Error("轉換YAML失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("轉換YAML失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": string(yamlData)})
}

// DeleteIngress 刪除Ingress
func (h *IngressHandler) DeleteIngress(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 刪除Ingress
	err = clientset.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error("刪除Ingress失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("刪除Ingress失敗: %v", err))
		return
	}

	logger.Info("Ingress刪除成功", "clusterId", clusterID, "namespace", namespace, "name", name)
	response.NoContent(c)
}

// GetIngressNamespaces 獲取Ingress所在的命名空間列表
func (h *IngressHandler) GetIngressNamespaces(c *gin.Context) {
	clusterID := c.Param("clusterID")

	// 獲取叢集
	id, err := strconv.ParseUint(clusterID, 10, 32)
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
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}
	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// 獲取所有Ingresses
	ingressList, err := clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取Ingress列表失敗", "cluster", cluster.Name, "error", err)
		response.InternalError(c, fmt.Sprintf("獲取Ingress列表失敗: %v", err))
		return
	}

	// 統計每個命名空間的Ingress數量
	nsMap := make(map[string]int)
	for _, ing := range ingressList.Items {
		nsMap[ing.Namespace]++
	}

	type NamespaceItem struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	var namespaces []NamespaceItem
	for ns, count := range nsMap {
		namespaces = append(namespaces, NamespaceItem{
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
