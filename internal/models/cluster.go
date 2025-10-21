package models

import (
	"time"

	"gorm.io/gorm"
)

// Cluster 集群模型
type Cluster struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	Name          string         `json:"name" gorm:"uniqueIndex;not null;size:100"`
	APIServer     string         `json:"api_server" gorm:"not null;size:255"`
	KubeconfigEnc string         `json:"-" gorm:"type:text"` // 加密存储的 kubeconfig
	CAEnc         string         `json:"-" gorm:"type:text"` // 加密存储的 CA 证书
	SATokenEnc    string         `json:"-" gorm:"type:text"` // 加密存储的 SA Token
	Version       string         `json:"version" gorm:"size:50"`
	Status        string         `json:"status" gorm:"default:unknown;size:20"` // healthy, unhealthy, unknown
	Labels        string         `json:"labels" gorm:"type:json"`               // JSON 格式存储标签
	CertExpireAt  *time.Time     `json:"cert_expire_at"`
	LastHeartbeat *time.Time     `json:"last_heartbeat"`
	CreatedBy     uint           `json:"created_by"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

	// 监控配置
	MonitoringConfig string `json:"monitoring_config" gorm:"type:json"` // JSON 格式存储监控配置

	// 关联关系
	Creator         User              `json:"creator" gorm:"foreignKey:CreatedBy"`
	TerminalSession []TerminalSession `json:"terminal_sessions" gorm:"foreignKey:ClusterID"`
}

// ClusterStats 集群统计信息
type ClusterStats struct {
	TotalClusters     int `json:"total_clusters"`
	HealthyClusters   int `json:"healthy_clusters"`
	UnhealthyClusters int `json:"unhealthy_clusters"`
	TotalNodes        int `json:"total_nodes"`
	ReadyNodes        int `json:"ready_nodes"`
	TotalPods         int `json:"total_pods"`
	RunningPods       int `json:"running_pods"`
}

// ClusterMetrics 集群实时指标
type ClusterMetrics struct {
	ClusterID    uint      `json:"cluster_id" gorm:"primaryKey"`
	NodeCount    int       `json:"node_count"`
	ReadyNodes   int       `json:"ready_nodes"`
	PodCount     int       `json:"pod_count"`
	RunningPods  int       `json:"running_pods"`
	CPUUsage     float64   `json:"cpu_usage"`
	MemoryUsage  float64   `json:"memory_usage"`
	StorageUsage float64   `json:"storage_usage"`
	UpdatedAt    time.Time `json:"updated_at"`

	// 关联关系
	Cluster Cluster `json:"cluster" gorm:"foreignKey:ClusterID"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Type     string                 `json:"type"`     // prometheus, victoriametrics, disabled
	Endpoint string                 `json:"endpoint"` // 监控数据源地址
	Auth     *MonitoringAuth        `json:"auth,omitempty"`
	Labels   map[string]string      `json:"labels,omitempty"` // 用于统一数据源的集群标签
	Options  map[string]interface{} `json:"options,omitempty"`
}

// MonitoringAuth 监控认证配置
type MonitoringAuth struct {
	Type     string `json:"type"` // none, basic, bearer, mtls
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
	CertFile string `json:"cert_file,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`
	CAFile   string `json:"ca_file,omitempty"`
}

// MetricsQuery 监控查询参数
type MetricsQuery struct {
	Query   string            `json:"query"`
	Start   int64             `json:"start"`
	End     int64             `json:"end"`
	Step    string            `json:"step"`
	Timeout string            `json:"timeout,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
}

// MetricsResponse 监控查询响应
type MetricsResponse struct {
	Status string      `json:"status"`
	Data   MetricsData `json:"data"`
}

// MetricsData 监控数据
type MetricsData struct {
	ResultType string          `json:"resultType"`
	Result     []MetricsResult `json:"result"`
}

// MetricsResult 监控结果
type MetricsResult struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values,omitempty"`
	Value  []interface{}     `json:"value,omitempty"`
}

// ClusterMetricsData 集群监控数据
type ClusterMetricsData struct {
	CPU     *MetricSeries   `json:"cpu,omitempty"`
	Memory  *MetricSeries   `json:"memory,omitempty"`
	Network *NetworkMetrics `json:"network,omitempty"`
	Storage *MetricSeries   `json:"storage,omitempty"`
	Pods    *PodMetrics     `json:"pods,omitempty"`
}

// MetricSeries 指标时间序列
type MetricSeries struct {
	Current float64     `json:"current"`
	Series  []DataPoint `json:"series"`
}

// DataPoint 数据点
type DataPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// NetworkMetrics 网络指标
type NetworkMetrics struct {
	In  *MetricSeries `json:"in"`
	Out *MetricSeries `json:"out"`
}

// PodMetrics Pod指标
type PodMetrics struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Pending int `json:"pending"`
	Failed  int `json:"failed"`
}

// ContainerSubnetIPs 容器子网IP信息
type ContainerSubnetIPs struct {
	TotalIPs     int `json:"total_ips"`
	UsedIPs      int `json:"used_ips"`
	AvailableIPs int `json:"available_ips"`
}
