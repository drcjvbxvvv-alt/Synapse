package models

import (
	"time"

	"github.com/shaia/Synapse/pkg/crypto"
	"gorm.io/gorm"
)

// Cluster 叢集模型
type Cluster struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	Name          string         `json:"name" gorm:"uniqueIndex;not null;size:100"`
	APIServer     string         `json:"api_server" gorm:"not null;size:255"`
	KubeconfigEnc string         `json:"-" gorm:"type:text"` // 加密儲存的 kubeconfig
	CAEnc         string         `json:"-" gorm:"type:text"` // 加密儲存的 CA 證書
	SATokenEnc    string         `json:"-" gorm:"type:text"` // 加密儲存的 SA Token
	Version       string         `json:"version" gorm:"size:50"`
	Status        string         `json:"status" gorm:"default:unknown;size:20"` // healthy, unhealthy, unknown
	Labels        string         `json:"labels" gorm:"type:json"`               // JSON 格式儲存標籤
	CertExpireAt  *time.Time     `json:"cert_expire_at"`
	LastHeartbeat *time.Time     `json:"last_heartbeat"`
	CreatedBy     uint           `json:"created_by"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

	// 監控配置
	MonitoringConfig string `json:"monitoring_config" gorm:"type:json"` // JSON 格式儲存監控配置

	// Alertmanager 配置
	AlertManagerConfig string `json:"alertmanager_config" gorm:"type:json"` // JSON 格式儲存 Alertmanager 配置

	// 關聯關係
	Creator         User              `json:"creator" gorm:"foreignKey:CreatedBy"`
	TerminalSession []TerminalSession `json:"terminal_sessions" gorm:"foreignKey:ClusterID"`
}

// ---------------------------------------------------------------------------
// GORM hooks — transparent AES-256-GCM encryption for sensitive fields.
// Encryption is enabled when crypto.Init() is called with a non-empty key.
// When no key is configured the hooks are no-ops (plaintext passthrough).
// ---------------------------------------------------------------------------

// BeforeSave encrypts sensitive fields before INSERT or UPDATE.
func (c *Cluster) BeforeSave(_ *gorm.DB) error {
	// MySQL JSON 欄位不接受空字串，統一用 "null" 作為空值
	if c.MonitoringConfig == "" {
		c.MonitoringConfig = "null"
	}
	if c.AlertManagerConfig == "" {
		c.AlertManagerConfig = "null"
	}

	if !crypto.IsEnabled() {
		return nil
	}
	var err error
	if c.KubeconfigEnc, err = crypto.Encrypt(c.KubeconfigEnc); err != nil {
		return err
	}
	if c.CAEnc, err = crypto.Encrypt(c.CAEnc); err != nil {
		return err
	}
	if c.SATokenEnc, err = crypto.Encrypt(c.SATokenEnc); err != nil {
		return err
	}
	return nil
}

// afterSave decrypts sensitive fields back into memory after a successful save
// so the caller always sees plaintext values.
func (c *Cluster) afterSave() {
	if !crypto.IsEnabled() {
		return
	}
	c.KubeconfigEnc, _ = crypto.Decrypt(c.KubeconfigEnc)
	c.CAEnc, _ = crypto.Decrypt(c.CAEnc)
	c.SATokenEnc, _ = crypto.Decrypt(c.SATokenEnc)
}

// AfterCreate decrypts the fields back to plaintext after a CREATE operation.
func (c *Cluster) AfterCreate(_ *gorm.DB) error {
	c.afterSave()
	return nil
}

// AfterUpdate decrypts the fields back to plaintext after an UPDATE operation.
func (c *Cluster) AfterUpdate(_ *gorm.DB) error {
	c.afterSave()
	return nil
}

// AfterFind decrypts sensitive fields after loading from the database.
func (c *Cluster) AfterFind(_ *gorm.DB) error {
	if !crypto.IsEnabled() {
		return nil
	}
	var err error
	if c.KubeconfigEnc, err = crypto.Decrypt(c.KubeconfigEnc); err != nil {
		return err
	}
	if c.CAEnc, err = crypto.Decrypt(c.CAEnc); err != nil {
		return err
	}
	if c.SATokenEnc, err = crypto.Decrypt(c.SATokenEnc); err != nil {
		return err
	}
	return nil
}

// ClusterStats 叢集統計資訊
type ClusterStats struct {
	TotalClusters     int `json:"total_clusters"`
	HealthyClusters   int `json:"healthy_clusters"`
	UnhealthyClusters int `json:"unhealthy_clusters"`
	TotalNodes        int `json:"total_nodes"`
	ReadyNodes        int `json:"ready_nodes"`
	TotalPods         int `json:"total_pods"`
	RunningPods       int `json:"running_pods"`
}

// ClusterMetrics 叢集實時指標
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

	// 關聯關係
	Cluster Cluster `json:"cluster" gorm:"foreignKey:ClusterID"`
}

// MonitoringConfig 監控配置
type MonitoringConfig struct {
	Type     string                 `json:"type"`     // prometheus, victoriametrics, disabled
	Endpoint string                 `json:"endpoint"` // 監控資料來源地址
	Auth     *MonitoringAuth        `json:"auth,omitempty"`
	Labels   map[string]string      `json:"labels,omitempty"` // 用於統一資料來源的叢集標籤
	Options  map[string]interface{} `json:"options,omitempty"`
}

// MonitoringAuth 監控認證配置
type MonitoringAuth struct {
	Type     string `json:"type"` // none, basic, bearer, mtls
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
	CertFile string `json:"cert_file,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`
	CAFile   string `json:"ca_file,omitempty"`
}

// MetricsQuery 監控查詢參數
type MetricsQuery struct {
	Query   string            `json:"query"`
	Start   int64             `json:"start"`
	End     int64             `json:"end"`
	Step    string            `json:"step"`
	Timeout string            `json:"timeout,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
}

// MetricsResponse 監控查詢響應
type MetricsResponse struct {
	Status string      `json:"status"`
	Data   MetricsData `json:"data"`
}

// MetricsData 監控資料
type MetricsData struct {
	ResultType string          `json:"resultType"`
	Result     []MetricsResult `json:"result"`
}

// MetricsResult 監控結果
type MetricsResult struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values,omitempty"`
	Value  []interface{}     `json:"value,omitempty"`
}

// ClusterMetricsData 叢集監控資料
type ClusterMetricsData struct {
	CPU     *MetricSeries   `json:"cpu,omitempty"`
	Memory  *MetricSeries   `json:"memory,omitempty"`
	Network *NetworkMetrics `json:"network,omitempty"`
	Storage *MetricSeries   `json:"storage,omitempty"`
	Pods    *PodMetrics     `json:"pods,omitempty"`
	// Pod 級別的擴充套件監控指標
	CPURequest        *MetricSeries   `json:"cpu_request,omitempty"`         // CPU 請求值（固定）
	CPULimit          *MetricSeries   `json:"cpu_limit,omitempty"`           // CPU 限制值（固定）
	MemoryRequest     *MetricSeries   `json:"memory_request,omitempty"`      // 記憶體請求值（固定）
	MemoryLimit       *MetricSeries   `json:"memory_limit,omitempty"`        // 記憶體限制值（固定）
	ProbeFailures     *MetricSeries   `json:"probe_failures,omitempty"`      // 健康檢查失敗次數
	ContainerRestarts *MetricSeries   `json:"container_restarts,omitempty"`  // 容器重啟次數
	NetworkPPS        *NetworkPPS     `json:"network_pps,omitempty"`         // 網路PPS（包/秒）
	Threads           *MetricSeries   `json:"threads,omitempty"`             // 執行緒數
	NetworkDrops      *NetworkDrops   `json:"network_drops,omitempty"`       // 網絡卡丟包情況
	CPUThrottling     *MetricSeries   `json:"cpu_throttling,omitempty"`      // CPU 限流比例
	CPUThrottlingTime *MetricSeries   `json:"cpu_throttling_time,omitempty"` // CPU 限流時間
	DiskIOPS          *DiskIOPS       `json:"disk_iops,omitempty"`           // 磁碟 IOPS
	DiskThroughput    *DiskThroughput `json:"disk_throughput,omitempty"`     // 磁碟吞吐量
	CPUUsageAbsolute  *MetricSeries   `json:"cpu_usage_absolute,omitempty"`  // CPU 實際使用量（cores）
	MemoryUsageBytes  *MetricSeries   `json:"memory_usage_bytes,omitempty"`  // 記憶體實際使用量（bytes）
	OOMKills          *MetricSeries   `json:"oom_kills,omitempty"`           // OOM Kill 次數

	// 叢集級別監控指標
	ClusterOverview *ClusterOverview `json:"cluster_overview,omitempty"` // 叢集概覽
	NodeList        []NodeMetricItem `json:"node_list,omitempty"`        // Node列表指標

	// 工作負載多Pod監控指標（顯示多條曲線）
	CPUMulti               *MultiSeriesMetric `json:"cpu_multi,omitempty"`                 // CPU使用率（多Pod）
	MemoryMulti            *MultiSeriesMetric `json:"memory_multi,omitempty"`              // 記憶體使用率（多Pod）
	ContainerRestartsMulti *MultiSeriesMetric `json:"container_restarts_multi,omitempty"`  // 容器重啟次數（多Pod）
	OOMKillsMulti          *MultiSeriesMetric `json:"oom_kills_multi,omitempty"`           // OOM Kill 次數（多Pod）
	ProbeFailuresMulti     *MultiSeriesMetric `json:"probe_failures_multi,omitempty"`      // 健康檢查失敗次數（多Pod）
	NetworkPPSMulti        *MultiSeriesMetric `json:"network_pps_multi,omitempty"`         // 網路PPS（多Pod）
	ThreadsMulti           *MultiSeriesMetric `json:"threads_multi,omitempty"`             // 執行緒數（多Pod）
	NetworkDropsMulti      *MultiSeriesMetric `json:"network_drops_multi,omitempty"`       // 網絡卡丟包情況（多Pod）
	CPUThrottlingMulti     *MultiSeriesMetric `json:"cpu_throttling_multi,omitempty"`      // CPU 限流比例（多Pod）
	CPUThrottlingTimeMulti *MultiSeriesMetric `json:"cpu_throttling_time_multi,omitempty"` // CPU 限流時間（多Pod）
	DiskIOPSMulti          *MultiSeriesMetric `json:"disk_iops_multi,omitempty"`           // 磁碟 IOPS（多Pod）
	DiskThroughputMulti    *MultiSeriesMetric `json:"disk_throughput_multi,omitempty"`     // 磁碟吞吐量（多Pod）
}

// MetricSeries 指標時間序列
type MetricSeries struct {
	Current float64     `json:"current"`
	Series  []DataPoint `json:"series"`
}

// DataPoint 資料點
type DataPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// NetworkMetrics 網路指標
type NetworkMetrics struct {
	In  *MetricSeries `json:"in"`
	Out *MetricSeries `json:"out"`
}

// PodMetrics Pod指標
type PodMetrics struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Pending int `json:"pending"`
	Failed  int `json:"failed"`
}

// ContainerSubnetIPs 容器子網IP資訊
type ContainerSubnetIPs struct {
	TotalIPs     int `json:"total_ips"`
	UsedIPs      int `json:"used_ips"`
	AvailableIPs int `json:"available_ips"`
}

// NetworkPPS 網路PPS指標
type NetworkPPS struct {
	In  *MetricSeries `json:"in"`  // 入站PPS
	Out *MetricSeries `json:"out"` // 出站PPS
}

// NetworkDrops 網絡卡丟包指標
type NetworkDrops struct {
	Receive  *MetricSeries `json:"receive"`  // 接收丟包
	Transmit *MetricSeries `json:"transmit"` // 傳送丟包
}

// DiskIOPS 磁碟IOPS指標
type DiskIOPS struct {
	Read  *MetricSeries `json:"read"`  // 讀IOPS
	Write *MetricSeries `json:"write"` // 寫IOPS
}

// DiskThroughput 磁碟吞吐量指標
type DiskThroughput struct {
	Read  *MetricSeries `json:"read"`  // 讀吞吐量（bytes/s）
	Write *MetricSeries `json:"write"` // 寫吞吐量（bytes/s）
}

// MultiSeriesDataPoint 多時間序列資料點（支援多個Pod/例項）
type MultiSeriesDataPoint struct {
	Timestamp int64              `json:"timestamp"`
	Values    map[string]float64 `json:"values"` // key為pod名稱，value為對應值
}

// MultiSeriesMetric 多時間序列指標（用於展示多個Pod的資料）
type MultiSeriesMetric struct {
	Series []MultiSeriesDataPoint `json:"series"` // 時間序列資料
}

// ClusterOverview 叢集概覽監控指標
type ClusterOverview struct {
	// 資源總量
	TotalCPUCores float64 `json:"total_cpu_cores"` // CPU 總核數
	TotalMemory   float64 `json:"total_memory"`    // 記憶體總數（bytes）

	// 資源使用
	CPUUsageRate    *MetricSeries `json:"cpu_usage_rate,omitempty"`    // CPU 使用率
	MemoryUsageRate *MetricSeries `json:"memory_usage_rate,omitempty"` // 記憶體使用率

	// Pod 相關
	MaxPods       int     `json:"max_pods"`       // Pod 最大可建立數
	CreatedPods   int     `json:"created_pods"`   // Pod 已建立數
	AvailablePods int     `json:"available_pods"` // Pod 可建立數
	PodUsageRate  float64 `json:"pod_usage_rate"` // Pod 使用率

	// 叢集狀態
	EtcdHasLeader         bool    `json:"etcd_has_leader"`        // Etcd 是否有 leader
	ApiServerAvailability float64 `json:"apiserver_availability"` // ApiServer 近30天可用率

	// 資源配額
	CPURequestRatio *MetricSeries `json:"cpu_request_ratio,omitempty"` // CPU Request 比值
	CPULimitRatio   *MetricSeries `json:"cpu_limit_ratio,omitempty"`   // CPU Limit 比值
	MemRequestRatio *MetricSeries `json:"mem_request_ratio,omitempty"` // 記憶體 Request 比值
	MemLimitRatio   *MetricSeries `json:"mem_limit_ratio,omitempty"`   // 記憶體 Limit 比值

	// ApiServer 請求量
	ApiServerRequestRate *MetricSeries `json:"apiserver_request_rate,omitempty"` // ApiServer 總請求量
}

// NodeMetricItem Node 監控指標項
type NodeMetricItem struct {
	NodeName        string  `json:"node_name"`         // 節點名稱
	CPUUsageRate    float64 `json:"cpu_usage_rate"`    // CPU 使用率
	MemoryUsageRate float64 `json:"memory_usage_rate"` // 記憶體使用率
	CPUCores        float64 `json:"cpu_cores"`         // CPU 核數
	TotalMemory     float64 `json:"total_memory"`      // 總記憶體（bytes）
	Status          string  `json:"status"`            // 節點狀態
}
