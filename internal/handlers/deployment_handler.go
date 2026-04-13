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

// DeploymentHandler Deployment處理器
type DeploymentHandler struct {
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewDeploymentHandler 建立Deployment處理器
func NewDeploymentHandler(cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *DeploymentHandler {
	return &DeploymentHandler{
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// fetchDeploymentsFromCache 從 Informer 快取讀取 Deployment 列表
func (h *DeploymentHandler) fetchDeploymentsFromCache(clusterID uint, namespace string) []DeploymentInfo {
	sel := labels.Everything()
	var deployments []DeploymentInfo
	if namespace != "" {
		deps, err := h.k8sMgr.DeploymentsLister(clusterID).Deployments(namespace).List(sel)
		if err != nil {
			logger.Error("讀取Deployment快取失敗", "error", err)
			return deployments
		}
		for _, d := range deps {
			deployments = append(deployments, h.convertToDeploymentInfo(d))
		}
	} else {
		deps, err := h.k8sMgr.DeploymentsLister(clusterID).List(sel)
		if err != nil {
			logger.Error("讀取Deployment快取失敗", "error", err)
			return deployments
		}
		for _, d := range deps {
			deployments = append(deployments, h.convertToDeploymentInfo(d))
		}
	}
	return deployments
}

// filterDeploymentsByName 按名稱過濾 Deployment 列表
func filterDeploymentsByName(deployments []DeploymentInfo, search string) []DeploymentInfo {
	if search == "" {
		return deployments
	}
	searchLower := strings.ToLower(search)
	var filtered []DeploymentInfo
	for _, dep := range deployments {
		if strings.Contains(strings.ToLower(dep.Name), searchLower) {
			filtered = append(filtered, dep)
		}
	}
	return filtered
}

// ListDeployments 獲取Deployment列表
func (h *DeploymentHandler) ListDeployments(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Query("namespace")
	searchName := c.Query("search")
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	logger.Info("獲取Deployment列表: cluster=%s, namespace=%s, search=%s", clusterId, namespace, searchName)

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

	nsInfo, hasAccess := middleware.CheckNamespacePermission(c, namespace)
	if !hasAccess {
		middleware.ForbiddenNS(c, nsInfo)
		return
	}

	deployments := h.fetchDeploymentsFromCache(cluster.ID, namespace)

	if !nsInfo.HasAllAccess && namespace == "" {
		deployments = middleware.FilterResourcesByNamespace(c, deployments, func(d DeploymentInfo) string {
			return d.Namespace
		})
	}

	deployments = filterDeploymentsByName(deployments, searchName)

	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].CreatedAt.After(deployments[j].CreatedAt)
	})

	total := len(deployments)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	response.PagedList(c, deployments[start:end], int64(total), page, pageSize)
}

// GetDeployment 獲取Deployment詳情
func (h *DeploymentHandler) GetDeployment(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment詳情: %s/%s/%s", clusterId, namespace, name)

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

	clientset := k8sClient.GetClientset()
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Deployment不存在: "+err.Error())
		return
	}

	// 獲取關聯的Pods
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector),
	})
	if err != nil {
		logger.Error("獲取Deployment關聯Pods失敗", "error", err)
	}

	// 清理 managed fields 以生成更乾淨的 YAML
	cleanDeployment := deployment.DeepCopy()
	cleanDeployment.ManagedFields = nil
	cleanDeployment.Annotations = filterAnnotations(cleanDeployment.Annotations)
	// 設定 TypeMeta（client-go 返回的物件預設不包含 apiVersion 和 kind）
	cleanDeployment.APIVersion = "apps/v1"
	cleanDeployment.Kind = "Deployment"
	// 將 Deployment 物件轉換為 YAML 字串
	yamlBytes, yamlErr := sigsyaml.Marshal(cleanDeployment)
	var yamlString string
	if yamlErr == nil {
		yamlString = string(yamlBytes)
	} else {
		logger.Error("轉換Deployment為YAML失敗", "error", yamlErr)
		yamlString = ""
	}

	response.OK(c, gin.H{
		"workload": h.convertToDeploymentInfo(deployment),
		"raw":      deployment,
		"yaml":     yamlString,
		"pods":     pods,
	})
}

// GetDeploymentNamespaces 獲取包含Deployment的命名空間列表
func (h *DeploymentHandler) GetDeploymentNamespaces(c *gin.Context) {
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

	// 從Informer讀取所有Deployments並統計命名空間
	sel := labels.Everything()
	deps, err := h.k8sMgr.DeploymentsLister(cluster.ID).List(sel)
	if err != nil {
		response.InternalError(c, "讀取Deployment快取失敗: "+err.Error())
		return
	}

	// 統計每個命名空間的Deployment數量
	nsCount := make(map[string]int)
	for _, dep := range deps {
		nsCount[dep.Namespace]++
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

// ScaleDeployment 擴縮容Deployment
func (h *DeploymentHandler) ScaleDeployment(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req ScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	logger.Info("擴縮容Deployment: %s/%s/%s to %d", clusterId, namespace, name, req.Replicas)

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

	clientset := k8sClient.GetClientset()
	scale, err := clientset.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.InternalError(c, "獲取Deployment Scale失敗: "+err.Error())
		return
	}

	scale.Spec.Replicas = req.Replicas
	_, err = clientset.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "擴縮容失敗: "+err.Error())
		return
	}

	response.NoContent(c)
}

// DeleteDeployment 刪除Deployment
func (h *DeploymentHandler) DeleteDeployment(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("刪除Deployment: %s/%s/%s", clusterId, namespace, name)

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

	clientset := k8sClient.GetClientset()
	err = clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		response.InternalError(c, "刪除失敗: "+err.Error())
		return
	}

	response.NoContent(c)
}
