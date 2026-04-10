package handlers

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/shaia/Synapse/pkg/logger"
)

// getNodeInternalIP 獲取節點內部IP
func getNodeInternalIP(node corev1.Node) string {
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.Address
		}
	}
	return ""
}

// getNodeExternalIP 獲取節點外部IP
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
