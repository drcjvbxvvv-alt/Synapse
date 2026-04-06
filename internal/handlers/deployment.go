package handlers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/config"
	"github.com/clay-wangzhi/Synapse/internal/k8s"
	"github.com/clay-wangzhi/Synapse/internal/middleware"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	sigsyaml "sigs.k8s.io/yaml"
)

// DeploymentHandler Deployment處理器
type DeploymentHandler struct {
	db             *gorm.DB
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewDeploymentHandler 建立Deployment處理器
func NewDeploymentHandler(db *gorm.DB, cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *DeploymentHandler {
	return &DeploymentHandler{
		db:             db,
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// DeploymentInfo Deployment資訊
type DeploymentInfo struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Type              string            `json:"type"`
	Status            string            `json:"status"`
	Replicas          int32             `json:"replicas"`
	ReadyReplicas     int32             `json:"readyReplicas"`
	AvailableReplicas int32             `json:"availableReplicas"`
	UpdatedReplicas   int32             `json:"updatedReplicas"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreatedAt         time.Time         `json:"createdAt"`
	Images            []string          `json:"images"`
	Selector          map[string]string `json:"selector"`
	Strategy          string            `json:"strategy"`
	CPULimit          string            `json:"cpuLimit"`
	CPURequest        string            `json:"cpuRequest"`
	MemoryLimit       string            `json:"memoryLimit"`
	MemoryRequest     string            `json:"memoryRequest"`
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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

// ApplyYAML 應用Deployment YAML
func (h *DeploymentHandler) ApplyYAML(c *gin.Context) {
	clusterId := c.Param("clusterID")

	var req YAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	logger.Info("應用Deployment YAML: cluster=%s, dryRun=%v", clusterId, req.DryRun)

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 解析YAML
	var objMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.YAML), &objMap); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	// 驗證必要欄位
	if objMap["apiVersion"] == nil || objMap["kind"] == nil {
		response.BadRequest(c, "YAML缺少必要欄位: apiVersion 或 kind")
		return
	}

	kind := objMap["kind"].(string)
	if kind != "Deployment" {
		response.BadRequest(c, "YAML型別錯誤，期望Deployment，實際為: "+kind)
		return
	}

	// 獲取metadata
	metadata, ok := objMap["metadata"].(map[string]interface{})
	if !ok {
		response.BadRequest(c, "YAML缺少 metadata 欄位")
		return
	}

	namespace, _ := metadata["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	// 應用YAML
	result, err := h.applyYAML(ctx, k8sClient, req.YAML, namespace, req.DryRun)
	if err != nil {
		response.InternalError(c, "YAML應用失敗: "+err.Error())
		return
	}

	response.OK(c, result)
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	err = clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		response.InternalError(c, "刪除失敗: "+err.Error())
		return
	}

	response.NoContent(c)
}

// 輔助方法：轉換Deployment到DeploymentInfo
func (h *DeploymentHandler) convertToDeploymentInfo(d *appsv1.Deployment) DeploymentInfo {
	status := "Running"
	if d.Status.Replicas == 0 {
		status = "Stopped"
	} else if d.Status.AvailableReplicas < d.Status.Replicas {
		status = "Degraded"
	}

	// 提取映像列表和資源資訊
	var images []string
	var cpuLimits, cpuRequests []string
	var memoryLimits, memoryRequests []string

	for _, container := range d.Spec.Template.Spec.Containers {
		images = append(images, container.Image)

		// CPU 限制
		if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
			cpuLimits = append(cpuLimits, cpu.String())
		}

		// CPU 申請
		if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
			cpuRequests = append(cpuRequests, cpu.String())
		}

		// 記憶體 限制
		if memory := container.Resources.Limits.Memory(); memory != nil && !memory.IsZero() {
			memoryLimits = append(memoryLimits, memory.String())
		}

		// 記憶體 申請
		if memory := container.Resources.Requests.Memory(); memory != nil && !memory.IsZero() {
			memoryRequests = append(memoryRequests, memory.String())
		}
	}

	// 策略
	strategy := string(d.Spec.Strategy.Type)

	// 格式化資源值
	cpuLimit := "-"
	if len(cpuLimits) > 0 {
		cpuLimit = strings.Join(cpuLimits, " + ")
	}

	cpuRequest := "-"
	if len(cpuRequests) > 0 {
		cpuRequest = strings.Join(cpuRequests, " + ")
	}

	memoryLimit := "-"
	if len(memoryLimits) > 0 {
		memoryLimit = strings.Join(memoryLimits, " + ")
	}

	memoryRequest := "-"
	if len(memoryRequests) > 0 {
		memoryRequest = strings.Join(memoryRequests, " + ")
	}

	return DeploymentInfo{
		ID:                fmt.Sprintf("%s/%s", d.Namespace, d.Name),
		Name:              d.Name,
		Namespace:         d.Namespace,
		Type:              "Deployment",
		Status:            status,
		Replicas:          *d.Spec.Replicas,
		ReadyReplicas:     d.Status.ReadyReplicas,
		AvailableReplicas: d.Status.AvailableReplicas,
		UpdatedReplicas:   d.Status.UpdatedReplicas,
		Labels:            d.Labels,
		Annotations:       d.Annotations,
		CreatedAt:         d.CreationTimestamp.Time,
		Images:            images,
		Selector:          d.Spec.Selector.MatchLabels,
		Strategy:          strategy,
		CPULimit:          cpuLimit,
		CPURequest:        cpuRequest,
		MemoryLimit:       memoryLimit,
		MemoryRequest:     memoryRequest,
	}
}

// 輔助方法：應用YAML
func (h *DeploymentHandler) applyYAML(ctx context.Context, k8sClient *services.K8sClient, yamlContent string, namespace string, dryRun bool) (interface{}, error) {
	// 建立解碼器
	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil, fmt.Errorf("無法轉換為Deployment型別")
	}

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if dryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	// 嘗試獲取現有資源
	existing, err := clientset.AppsV1().Deployments(deployment.Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
	if err == nil {
		// 資源存在，執行更新
		deployment.ResourceVersion = existing.ResourceVersion
		result, err := clientset.AppsV1().Deployments(deployment.Namespace).Update(ctx, deployment, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// 資源不存在，執行建立
	result, err := clientset.AppsV1().Deployments(deployment.Namespace).Create(ctx, deployment, metav1.CreateOptions{DryRun: dryRunOpt})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetDeploymentPods 獲取Deployment關聯的Pods
func (h *DeploymentHandler) GetDeploymentPods(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment Pods: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

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

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 獲取Deployment
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Deployment不存在")
		return
	}

	// 使用selector查詢Pods
	selector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		response.InternalError(c, "獲取Pod列表失敗: "+err.Error())
		return
	}

	// 轉換Pod資訊
	pods := make([]map[string]interface{}, 0, len(podList.Items))
	for _, pod := range podList.Items {
		podInfo := map[string]interface{}{
			"name":         pod.Name,
			"namespace":    pod.Namespace,
			"phase":        string(pod.Status.Phase),
			"nodeName":     pod.Spec.NodeName,
			"nodeIP":       pod.Status.HostIP,
			"podIP":        pod.Status.PodIP,
			"restartCount": 0,
			"createdAt":    pod.CreationTimestamp.Time,
		}

		// 計算重啟次數和提取資源限制
		var totalRestarts int32
		var cpuRequest, cpuLimit, memoryRequest, memoryLimit string
		for _, container := range pod.Spec.Containers {
			// 資源限制
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests["cpu"]; ok {
					cpuRequest = cpu.String()
				}
				if mem, ok := container.Resources.Requests["memory"]; ok {
					memoryRequest = mem.String()
				}
			}
			if container.Resources.Limits != nil {
				if cpu, ok := container.Resources.Limits["cpu"]; ok {
					cpuLimit = cpu.String()
				}
				if mem, ok := container.Resources.Limits["memory"]; ok {
					memoryLimit = mem.String()
				}
			}
		}

		// 統計重啟次數
		for _, containerStatus := range pod.Status.ContainerStatuses {
			totalRestarts += containerStatus.RestartCount
		}

		podInfo["restartCount"] = totalRestarts
		podInfo["cpuRequest"] = cpuRequest
		podInfo["cpuLimit"] = cpuLimit
		podInfo["memoryRequest"] = memoryRequest
		podInfo["memoryLimit"] = memoryLimit

		pods = append(pods, podInfo)
	}

	response.List(c, pods, int64(len(pods)))
}

// GetDeploymentServices 獲取Deployment關聯的Services
func (h *DeploymentHandler) GetDeploymentServices(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment Services: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

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

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 獲取Deployment
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Deployment不存在")
		return
	}

	// 獲取Services
	serviceList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取Service列表失敗: "+err.Error())
		return
	}

	// 篩選匹配的Services
	deploymentLabels := deployment.Spec.Selector.MatchLabels
	matchedServices := make([]map[string]interface{}, 0)
	for _, svc := range serviceList.Items {
		// 檢查Service的selector是否匹配Deployment的labels
		matches := true
		for key, value := range svc.Spec.Selector {
			if deploymentLabels[key] != value {
				matches = false
				break
			}
		}

		if matches {
			ports := make([]map[string]interface{}, 0, len(svc.Spec.Ports))
			for _, port := range svc.Spec.Ports {
				ports = append(ports, map[string]interface{}{
					"name":       port.Name,
					"protocol":   port.Protocol,
					"port":       port.Port,
					"targetPort": port.TargetPort.String(),
					"nodePort":   port.NodePort,
				})
			}

			serviceInfo := map[string]interface{}{
				"name":        svc.Name,
				"namespace":   svc.Namespace,
				"type":        string(svc.Spec.Type),
				"clusterIP":   svc.Spec.ClusterIP,
				"externalIPs": svc.Spec.ExternalIPs,
				"ports":       ports,
				"selector":    svc.Spec.Selector,
				"createdAt":   svc.CreationTimestamp.Time,
			}
			matchedServices = append(matchedServices, serviceInfo)
		}
	}

	response.List(c, matchedServices, int64(len(matchedServices)))
}

// GetDeploymentIngresses 獲取Deployment關聯的Ingresses
func (h *DeploymentHandler) GetDeploymentIngresses(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment Ingresses: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

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

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 獲取Ingresses
	ingressList, err := clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取Ingress列表失敗: "+err.Error())
		return
	}

	// 轉換Ingress資訊
	ingresses := make([]map[string]interface{}, 0, len(ingressList.Items))
	for _, ingress := range ingressList.Items {
		rules := make([]map[string]interface{}, 0, len(ingress.Spec.Rules))
		for _, rule := range ingress.Spec.Rules {
			paths := make([]map[string]interface{}, 0)
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					paths = append(paths, map[string]interface{}{
						"path":     path.Path,
						"pathType": string(*path.PathType),
						"backend": map[string]interface{}{
							"serviceName": path.Backend.Service.Name,
							"servicePort": path.Backend.Service.Port.Number,
						},
					})
				}
			}
			rules = append(rules, map[string]interface{}{
				"host":  rule.Host,
				"paths": paths,
			})
		}

		ingressInfo := map[string]interface{}{
			"name":             ingress.Name,
			"namespace":        ingress.Namespace,
			"ingressClassName": ingress.Spec.IngressClassName,
			"rules":            rules,
			"createdAt":        ingress.CreationTimestamp.Time,
		}
		ingresses = append(ingresses, ingressInfo)
	}

	response.List(c, ingresses, int64(len(ingresses)))
}

// GetDeploymentHPA 獲取Deployment的HPA
func (h *DeploymentHandler) GetDeploymentHPA(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment HPA: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

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

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 獲取HPA列表
	hpaList, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取HPA列表失敗: "+err.Error())
		return
	}

	// 查詢匹配的HPA
	for _, hpa := range hpaList.Items {
		if hpa.Spec.ScaleTargetRef.Kind == "Deployment" && hpa.Spec.ScaleTargetRef.Name == name {
			metrics := make([]map[string]interface{}, 0, len(hpa.Spec.Metrics))
			for _, metric := range hpa.Spec.Metrics {
				metricInfo := map[string]interface{}{
					"type": string(metric.Type),
				}
				if metric.Resource != nil {
					metricInfo["resource"] = map[string]interface{}{
						"name":   metric.Resource.Name,
						"target": metric.Resource.Target,
					}
				}
				metrics = append(metrics, metricInfo)
			}

			conditions := make([]map[string]interface{}, 0, len(hpa.Status.Conditions))
			for _, condition := range hpa.Status.Conditions {
				conditions = append(conditions, map[string]interface{}{
					"type":    string(condition.Type),
					"status":  string(condition.Status),
					"reason":  condition.Reason,
					"message": condition.Message,
				})
			}

			hpaInfo := map[string]interface{}{
				"name":            hpa.Name,
				"namespace":       hpa.Namespace,
				"minReplicas":     *hpa.Spec.MinReplicas,
				"maxReplicas":     hpa.Spec.MaxReplicas,
				"currentReplicas": hpa.Status.CurrentReplicas,
				"desiredReplicas": hpa.Status.DesiredReplicas,
				"metrics":         metrics,
				"conditions":      conditions,
			}

			response.OK(c, hpaInfo)
			return
		}
	}

	// 未找到HPA
	response.NotFound(c, "未找到HPA")
}

// GetDeploymentReplicaSets 獲取Deployment的ReplicaSets
func (h *DeploymentHandler) GetDeploymentReplicaSets(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment ReplicaSets: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

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

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 檢查Deployment是否存在
	_, err = clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Deployment不存在")
		return
	}

	// 獲取ReplicaSets
	rsList, err := clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取ReplicaSet列表失敗: "+err.Error())
		return
	}

	// 篩選匹配的ReplicaSets
	matchedReplicaSets := make([]map[string]interface{}, 0)
	for _, rs := range rsList.Items {
		// 檢查owner reference
		isOwned := false
		for _, owner := range rs.OwnerReferences {
			if owner.Kind == "Deployment" && owner.Name == name {
				isOwned = true
				break
			}
		}

		if isOwned {
			// 提取映像列表
			images := make([]string, 0)
			for _, container := range rs.Spec.Template.Spec.Containers {
				images = append(images, container.Image)
			}

			// 獲取revision號
			revision := rs.Annotations["deployment.kubernetes.io/revision"]

			rsInfo := map[string]interface{}{
				"name":              rs.Name,
				"namespace":         rs.Namespace,
				"replicas":          *rs.Spec.Replicas,
				"readyReplicas":     rs.Status.ReadyReplicas,
				"availableReplicas": rs.Status.AvailableReplicas,
				"revision":          revision,
				"images":            images,
				"createdAt":         rs.CreationTimestamp.Time,
			}
			matchedReplicaSets = append(matchedReplicaSets, rsInfo)
		}
	}

	response.List(c, matchedReplicaSets, int64(len(matchedReplicaSets)))
}

// GetDeploymentEvents 獲取Deployment的Events
func (h *DeploymentHandler) GetDeploymentEvents(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment Events: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

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

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 獲取Events
	eventList, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Deployment", name),
	})
	if err != nil {
		response.InternalError(c, "獲取Events失敗: "+err.Error())
		return
	}

	// 轉換Event資訊
	events := make([]map[string]interface{}, 0, len(eventList.Items))
	for _, event := range eventList.Items {
		eventInfo := map[string]interface{}{
			"type":           event.Type,
			"reason":         event.Reason,
			"message":        event.Message,
			"source":         event.Source,
			"count":          event.Count,
			"firstTimestamp": event.FirstTimestamp.Time,
			"lastTimestamp":  event.LastTimestamp.Time,
			"involvedObject": map[string]interface{}{
				"kind":      event.InvolvedObject.Kind,
				"name":      event.InvolvedObject.Name,
				"namespace": event.InvolvedObject.Namespace,
			},
		}
		events = append(events, eventInfo)
	}

	response.List(c, events, int64(len(events)))
}
