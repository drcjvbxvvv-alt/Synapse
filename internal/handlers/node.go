package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// NodeHandler 節點處理器
type NodeHandler struct {
	cfg              *config.Config
	clusterService   *services.ClusterService
	k8sMgr           *k8s.ClusterInformerManager
	promService      *services.PrometheusService
	monitoringCfgSvc *services.MonitoringConfigService
}

// NewNodeHandler 建立節點處理器
func NewNodeHandler(cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager, promService *services.PrometheusService, monitoringCfgSvc *services.MonitoringConfigService) *NodeHandler {
	return &NodeHandler{
		cfg:              cfg,
		clusterService:   clusterService,
		k8sMgr:           k8sMgr,
		promService:      promService,
		monitoringCfgSvc: monitoringCfgSvc,
	}
}

// GetNodes 獲取節點列表
func (h *NodeHandler) GetNodes(c *gin.Context) {
	clusterId := c.Param("clusterID")
	logger.Info("獲取節點列表: %s", clusterId)

	// 從叢集服務獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	if clusterID == 0 {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 使用 informer+lister 獲取節點列表
	if h.k8sMgr == nil {
		response.ServiceUnavailable(c, "K8s informer 管理器未初始化")
		return
	}
	if _, err := h.k8sMgr.EnsureAndWait(context.Background(), cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}
	nodeObjs, err := h.k8sMgr.NodesLister(cluster.ID).List(labels.Everything())
	if err != nil {
		response.InternalError(c, "讀取節點快取失敗: "+err.Error())
		return
	}
	// 轉為值型別以複用原有處理邏輯
	items := make([]corev1.Node, 0, len(nodeObjs))
	for _, n := range nodeObjs {
		items = append(items, *n)
	}

	// 獲取所有 Pod，統計每個節點上的 Pod 數量
	nodePodCounts := make(map[string]int)
	podObjs, err := h.k8sMgr.PodsLister(cluster.ID).List(labels.Everything())
	if err != nil {
		logger.Error("讀取 Pod 快取失敗: %v", err)
		// 繼續執行，podCount 將為 0
	} else {
		for _, pod := range podObjs {
			// 只統計非 Succeeded/Failed 狀態的 Pod
			if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
				nodePodCounts[pod.Spec.NodeName]++
			}
		}
	}

	// 獲取所有節點的資源使用率
	nodeResourceUsage := h.getNodesResourceUsage(c.Request.Context(), cluster.ID)

	// 轉換為API響應格式
	result := make([]map[string]interface{}, 0, len(items))
	for _, node := range items {
		// 獲取節點狀態
		status := "NotReady"
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" {
				if condition.Status == "True" {
					status = "Ready"
				}
				break
			}
		}

		// 獲取節點角色
		roles := []string{}
		for label := range node.Labels {
			if label == "node-role.kubernetes.io/control-plane" || label == "node-role.kubernetes.io/master" {
				roles = append(roles, "master")
			} else if strings.HasPrefix(label, "node-role.kubernetes.io/") {
				role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
				roles = append(roles, role)
			}
		}
		if len(roles) == 0 {
			roles = append(roles, "worker")
		}

		// 獲取節點汙點
		taints := []map[string]string{}
		for _, taint := range node.Spec.Taints {
			taints = append(taints, map[string]string{
				"key":    taint.Key,
				"value":  taint.Value,
				"effect": string(taint.Effect),
			})
		}

		// 獲取節點資源資訊
		cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
		memoryCapacity := node.Status.Capacity.Memory().Value() / (1024 * 1024) // 轉換為MB
		podCapacity := node.Status.Capacity.Pods().Value()

		// 獲取節點的 CPU 和記憶體使用率
		cpuUsage := 0.0
		memoryUsage := 0.0
		// 嘗試透過節點名稱匹配
		if usage, exists := nodeResourceUsage[node.Name]; exists {
			cpuUsage = usage["cpuUsage"]
			memoryUsage = usage["memoryUsage"]
		} else {
			// 嘗試透過內部 IP 匹配（Prometheus 的 instance 標籤可能是 IP 地址）
			internalIP := getNodeInternalIP(node)
			if internalIP != "" {
				if usage, exists := nodeResourceUsage[internalIP]; exists {
					cpuUsage = usage["cpuUsage"]
					memoryUsage = usage["memoryUsage"]
				}
			}
		}

		result = append(result, map[string]interface{}{
			"id":               node.Name, // 使用節點名作為ID
			"name":             node.Name,
			"status":           status,
			"roles":            roles,
			"version":          node.Status.NodeInfo.KubeletVersion,
			"osImage":          node.Status.NodeInfo.OSImage,
			"kernelVersion":    node.Status.NodeInfo.KernelVersion,
			"containerRuntime": node.Status.NodeInfo.ContainerRuntimeVersion,
			"cpuUsage":         cpuUsage,
			"memoryUsage":      memoryUsage,
			"podCount":         nodePodCounts[node.Name],
			"maxPods":          podCapacity,
			"taints":           taints,
			"unschedulable":    node.Spec.Unschedulable,
			"createdAt":        node.CreationTimestamp.Time,
			"internalIP":       getNodeInternalIP(node),
			"externalIP":       getNodeExternalIP(node),
			"resources": map[string]interface{}{
				"cpu":    cpuCapacity,
				"memory": memoryCapacity,
				"pods":   podCapacity,
			},
		})
	}

	response.PagedList(c, result, int64(len(result)), 1, 50)
}

// GetNodeOverview 獲取節點概覽資訊
func (h *NodeHandler) GetNodeOverview(c *gin.Context) {
	clusterId := c.Param("clusterID")
	logger.Info("獲取節點概覽: %s", clusterId)

	// 從叢集服務獲取叢集資訊
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

	// 使用 informer+lister 讀取節點並統計
	if _, err := h.k8sMgr.EnsureAndWait(context.Background(), cluster, 5*time.Second); err != nil {
		logger.Error("informer 未就緒", "error", err)
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}
	nodeObjs, err := h.k8sMgr.NodesLister(cluster.ID).List(labels.Everything())
	if err != nil {
		logger.Error("讀取節點快取失敗", "error", err)
		response.InternalError(c, "讀取節點快取失敗: "+err.Error())
		return
	}
	totalNodes := len(nodeObjs)
	readyNodes := 0
	notReadyNodes := 0
	maintenanceNodes := 0

	for _, node := range nodeObjs {
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" {
				if condition.Status == "True" {
					readyNodes++
				} else {
					notReadyNodes++
				}
				break
			}
		}

		// 檢查是否處於維護狀態（有NoSchedule汙點）
		if node.Spec.Unschedulable {
			maintenanceNodes++
		}
	}

	ctx := c.Request.Context()
	nodeResourceUsage := h.getNodesResourceUsage(ctx, cluster.ID)
	var totalCPU, totalMemory float64
	usageCount := 0
	for _, usage := range nodeResourceUsage {
		totalCPU += usage["cpuUsage"]
		totalMemory += usage["memoryUsage"]
		usageCount++
	}
	var cpuUsage, memoryUsage float64
	if usageCount > 0 {
		cpuUsage = totalCPU / float64(usageCount)
		memoryUsage = totalMemory / float64(usageCount)
	}

	overview := gin.H{
		"totalNodes":       totalNodes,
		"readyNodes":       readyNodes,
		"notReadyNodes":    notReadyNodes,
		"maintenanceNodes": maintenanceNodes,
		"cpuUsage":         cpuUsage,
		"memoryUsage":      memoryUsage,
		"storageUsage":     0.0,
	}

	response.OK(c, overview)
}

// GetNode 獲取節點詳情
func (h *NodeHandler) GetNode(c *gin.Context) {
	clusterId := c.Param("clusterID")
	name := c.Param("name")
	logger.Info("獲取節點詳情: %s/%s", clusterId, name)

	// 從叢集服務獲取叢集資訊
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

	// 使用 informer+lister 獲取節點詳情
	if h.k8sMgr == nil {
		response.ServiceUnavailable(c, "K8s informer 管理器未初始化")
		return
	}
	if _, err := h.k8sMgr.EnsureAndWait(context.Background(), cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}
	node, err := h.k8sMgr.NodesLister(cluster.ID).Get(name)
	if err != nil {
		response.InternalError(c, "讀取節點快取失敗: "+err.Error())
		return
	}

	// 獲取節點狀態
	status := "NotReady"
	conditions := []map[string]interface{}{}
	for _, condition := range node.Status.Conditions {
		conditions = append(conditions, map[string]interface{}{
			"type":    string(condition.Type),
			"status":  string(condition.Status),
			"reason":  condition.Reason,
			"message": condition.Message,
		})

		if condition.Type == "Ready" {
			if condition.Status == "True" {
				status = "Ready"
			}
		}
	}

	// 獲取節點角色
	roles := []string{}
	for label := range node.Labels {
		if label == "node-role.kubernetes.io/control-plane" || label == "node-role.kubernetes.io/master" {
			roles = append(roles, "master")
		} else if strings.HasPrefix(label, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		roles = append(roles, "worker")
	}

	// 獲取節點汙點
	taints := []map[string]string{}
	for _, taint := range node.Spec.Taints {
		taints = append(taints, map[string]string{
			"key":    taint.Key,
			"value":  taint.Value,
			"effect": string(taint.Effect),
		})
	}

	// 獲取節點標籤
	nodeLabels := []map[string]string{}
	for key, value := range node.Labels {
		nodeLabels = append(nodeLabels, map[string]string{
			"key":   key,
			"value": value,
		})
	}

	// 獲取節點資源資訊
	cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
	memoryCapacity := node.Status.Capacity.Memory().Value() / (1024 * 1024) // 轉換為MB
	podCapacity := node.Status.Capacity.Pods().Value()

	// 獲取節點地址
	addresses := []map[string]string{}
	for _, address := range node.Status.Addresses {
		addresses = append(addresses, map[string]string{
			"type":    string(address.Type),
			"address": address.Address,
		})
	}

	// 獲取節點的實際資源使用情況（透過快取讀取路徑暫不直連 API，保留預設值）
	cpuUsage := 0.0
	memoryUsage := 0.0
	podCount := 0

	// 統計該節點上的 Pod 數量
	podObjs, err := h.k8sMgr.PodsLister(cluster.ID).List(labels.Everything())
	if err != nil {
		logger.Error("讀取 Pod 快取失敗: %v", err)
	} else {
		for _, pod := range podObjs {
			// 只統計執行在該節點上且非 Succeeded/Failed 狀態的 Pod
			if pod.Spec.NodeName == name && pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
				podCount++
			}
		}
	}

	result := map[string]interface{}{
		"name":              node.Name,
		"status":            status,
		"roles":             roles,
		"addresses":         addresses,
		"conditions":        conditions,
		"osImage":           node.Status.NodeInfo.OSImage,
		"kernelVersion":     node.Status.NodeInfo.KernelVersion,
		"kubeletVersion":    node.Status.NodeInfo.KubeletVersion,
		"containerRuntime":  node.Status.NodeInfo.ContainerRuntimeVersion,
		"architecture":      node.Status.NodeInfo.Architecture,
		"taints":            taints,
		"labels":            nodeLabels,
		"unschedulable":     node.Spec.Unschedulable,
		"creationTimestamp": node.CreationTimestamp.Time,
		"cpuUsage":          cpuUsage,
		"memoryUsage":       memoryUsage,
		"podCount":          podCount,
		"resources": map[string]interface{}{
			"cpu":    cpuCapacity,
			"memory": memoryCapacity,
			"pods":   podCapacity,
		},
	}

	response.OK(c, result)
}

// CordonNode 封鎖節點
func (h *NodeHandler) CordonNode(c *gin.Context) {
	clusterId := c.Param("clusterID")
	name := c.Param("name")
	logger.Info("封鎖節點: %s/%s", clusterId, name)

	// 從叢集服務獲取叢集資訊
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

	// 封鎖節點
	err = k8sClient.CordonNode(name)
	if err != nil {
		response.InternalError(c, "封鎖節點失敗: "+err.Error())
		return
	}

	response.NoContent(c)
}

// UncordonNode 解封節點
func (h *NodeHandler) UncordonNode(c *gin.Context) {
	clusterId := c.Param("clusterID")
	name := c.Param("name")
	logger.Info("解封節點: %s/%s", clusterId, name)

	// 從叢集服務獲取叢集資訊
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

	// 解封節點
	err = k8sClient.UncordonNode(name)
	if err != nil {
		response.InternalError(c, "解封節點失敗: "+err.Error())
		return
	}

	response.NoContent(c)
}

// DrainNode 驅逐節點
func (h *NodeHandler) DrainNode(c *gin.Context) {
	clusterId := c.Param("clusterID")
	name := c.Param("name")
	logger.Info("驅逐節點: %s/%s", clusterId, name)

	// 解析請求參數
	var options map[string]interface{}
	if err := c.ShouldBindJSON(&options); err != nil {
		response.BadRequest(c, "參數解析失敗: "+err.Error())
		return
	}

	// 從叢集服務獲取叢集資訊
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

	// 驅逐節點
	err = k8sClient.DrainNode(name, options)
	if err != nil {
		response.InternalError(c, "驅逐節點失敗: "+err.Error())
		return
	}

	response.NoContent(c)
}

// PatchNodeLabels 新增或更新節點標籤
func (h *NodeHandler) PatchNodeLabels(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	name := c.Param("name")

	var req struct {
		Labels map[string]string `json:"labels" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
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

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": req.Labels,
		},
	}
	patchBytes, _ := json.Marshal(patch)

	_, err = k8sClient.GetClientset().CoreV1().Nodes().Patch(
		context.Background(), name, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		response.InternalError(c, "更新標籤失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"name": name, "labels": req.Labels})
}

// 獲取節點內部IP
func getNodeInternalIP(node corev1.Node) string {
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.Address
		}
	}
	return ""
}

// 獲取節點外部IP
func getNodeExternalIP(node corev1.Node) string {
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeExternalIP {
			return address.Address
		}
	}
	return ""
}

// getNodesResourceUsage 獲取所有節點的 CPU 和記憶體使用率
// 返回一個 map，key 是節點名稱，value 包含 cpuUsage 和 memoryUsage
func (h *NodeHandler) getNodesResourceUsage(ctx context.Context, clusterID uint) map[string]map[string]float64 {
	result := make(map[string]map[string]float64)

	if h.promService == nil || h.monitoringCfgSvc == nil {
		return result
	}

	// 獲取叢集的監控配置
	config, err := h.monitoringCfgSvc.GetMonitoringConfig(clusterID)
	if err != nil || config.Type == "disabled" {
		return result
	}

	// 呼叫 PrometheusService 的 queryNodeListMetrics 獲取節點指標
	nodeList, err := h.promService.QueryNodeListMetrics(ctx, config, "")
	if err != nil {
		logger.Error("獲取節點資源使用率失敗", "error", err)
		return result
	}

	// 將結果轉換為 map
	for _, node := range nodeList {
		result[node.NodeName] = map[string]float64{
			"cpuUsage":    node.CPUUsageRate,
			"memoryUsage": node.MemoryUsageRate,
		}
	}

	return result
}
