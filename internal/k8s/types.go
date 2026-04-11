package k8s

import "time"

// InformerHealth 描述單一叢集 Informer 的健康狀態（純記憶體讀取，無網路呼叫）。
type InformerHealth struct {
	ClusterID  uint      `json:"cluster_id"`
	Started    bool      `json:"started"`
	Synced     bool      `json:"synced"`
	StartedAt  time.Time `json:"started_at"`
	LastAccess time.Time `json:"last_access"`
}

// OverviewSnapshot 統一的叢集概覽快照（由本地 Informer 快取即時彙總）
type OverviewSnapshot struct {
	ClusterID uint `json:"clusterID"`

	Nodes int `json:"nodes"`

	Namespace int `json:"namespace"`

	Pods int `json:"pods"`

	Deployments  int `json:"deployments"`
	StatefulSets int `json:"statefulsets"`
	DaemonSets   int `json:"daemonsets"`
	Jobs         int `json:"jobs"`
	Rollouts     int `json:"rollouts"`

	// 容器子網IP資訊
	ContainerSubnetIPs *ContainerSubnetIPs `json:"containerSubnetIPs,omitempty"`
}

// ContainerSubnetIPs 容器子網IP資訊
type ContainerSubnetIPs struct {
	TotalIPs     int `json:"total_ips"`
	UsedIPs      int `json:"used_ips"`
	AvailableIPs int `json:"available_ips"`
}
