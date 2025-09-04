package k8s

// OverviewSnapshot 统一的集群概览快照（由本地 Informer 缓存即时汇总）
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
}
