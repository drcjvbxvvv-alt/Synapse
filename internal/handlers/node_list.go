package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

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
