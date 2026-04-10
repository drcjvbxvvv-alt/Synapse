package handlers

import (
	"context"
	"fmt"
	"sort"
	"strconv"

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

// ServiceHandler Service處理器
type ServiceHandler struct {
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewServiceHandler 建立Service處理器
func NewServiceHandler(cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *ServiceHandler {
	return &ServiceHandler{
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// ListServices 獲取Service列表
func (h *ServiceHandler) ListServices(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取查詢參數
	namespace := c.DefaultQuery("namespace", "")
	serviceType := c.DefaultQuery("type", "")
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

	// 獲取Services
	svcList, err := h.getServices(clientset, namespace)
	if err != nil {
		logger.Error("獲取Services失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取Services失敗: %v", err))
		return
	}

	// 根據命名空間權限過濾
	if !nsInfo.HasAllAccess && namespace == "" {
		svcList = middleware.FilterResourcesByNamespace(c, svcList, func(s ServiceInfo) string {
			return s.Namespace
		})
	}

	// 過濾和搜尋
	filteredServices := h.filterServices(svcList, serviceType, search)

	// 排序
	sort.Slice(filteredServices, func(i, j int) bool {
		return filteredServices[i].CreatedAt.After(filteredServices[j].CreatedAt)
	})

	// 分頁
	total := len(filteredServices)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedServices := filteredServices[start:end]

	response.PagedList(c, pagedServices, int64(total), page, pageSize)
}

// GetService 獲取單個Service詳情
func (h *ServiceHandler) GetService(c *gin.Context) {
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

	// 獲取Service
	service, err := clientset.CoreV1().Services(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Service失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取Service失敗: %v", err))
		return
	}

	serviceInfo := h.convertToServiceInfo(service)

	response.OK(c, serviceInfo)
}

// GetServiceYAML 獲取Service的YAML
func (h *ServiceHandler) GetServiceYAML(c *gin.Context) {
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

	// 獲取Service
	service, err := clientset.CoreV1().Services(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Service失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取Service失敗: %v", err))
		return
	}

	// 設定 apiVersion 和 kind（API 返回的物件不包含這些欄位）
	cleanSvc := service.DeepCopy()
	cleanSvc.APIVersion = "v1"
	cleanSvc.Kind = "Service"
	cleanSvc.ManagedFields = nil // 移除 managedFields 簡化 YAML

	// 轉換為YAML
	yamlData, err := yaml.Marshal(cleanSvc)
	if err != nil {
		logger.Error("轉換YAML失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("轉換YAML失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": string(yamlData)})
}

// DeleteService 刪除Service
func (h *ServiceHandler) DeleteService(c *gin.Context) {
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

	// 刪除Service
	err = clientset.CoreV1().Services(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error("刪除Service失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("刪除Service失敗: %v", err))
		return
	}

	logger.Info("Service刪除成功", "clusterId", clusterID, "namespace", namespace, "name", name)
	response.NoContent(c)
}

// GetServiceEndpoints 獲取Service的Endpoints
func (h *ServiceHandler) GetServiceEndpoints(c *gin.Context) {
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

	// 獲取Endpoints
	endpoints, err := clientset.CoreV1().Endpoints(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Endpoints失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取Endpoints失敗: %v", err))
		return
	}

	// 轉換Endpoints資訊
	endpointInfo := h.convertEndpointsInfo(endpoints)

	response.OK(c, endpointInfo)
}

// GetServiceNamespaces 獲取Service所在的命名空間列表
func (h *ServiceHandler) GetServiceNamespaces(c *gin.Context) {
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

	// 獲取所有Services
	serviceList, err := clientset.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取Service列表失敗", "cluster", cluster.Name, "error", err)
		response.InternalError(c, fmt.Sprintf("獲取Service列表失敗: %v", err))
		return
	}

	// 統計每個命名空間的Service數量
	nsMap := make(map[string]int)
	for _, svc := range serviceList.Items {
		nsMap[svc.Namespace]++
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
