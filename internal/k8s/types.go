package k8s

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
