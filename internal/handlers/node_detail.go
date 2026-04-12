package handlers

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

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
	if _, err := h.k8sMgr.EnsureAndWait(c.Request.Context(), cluster, 5*time.Second); err != nil {
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
