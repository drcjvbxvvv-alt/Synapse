package services

// ========== 響應結構體 ==========

// OverviewStatsResponse 總覽統計響應
type OverviewStatsResponse struct {
	ClusterStats        ClusterStatsData      `json:"clusterStats"`
	NodeStats           NodeStatsData         `json:"nodeStats"`
	PodStats            PodStatsData          `json:"podStats"`
	VersionDistribution []VersionDistribution `json:"versionDistribution"`
}

// ClusterStatsData 叢集統計
type ClusterStatsData struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Unknown   int `json:"unknown"`
}

// NodeStatsData 節點統計
type NodeStatsData struct {
	Total    int `json:"total"`
	Ready    int `json:"ready"`
	NotReady int `json:"notReady"`
}

// PodStatsData Pod 統計
type PodStatsData struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Pending   int `json:"pending"`
	Failed    int `json:"failed"`
	Succeeded int `json:"succeeded"`
}

// VersionDistribution 版本分佈
type VersionDistribution struct {
	Version  string   `json:"version"`
	Count    int      `json:"count"`
	Clusters []string `json:"clusters"`
}

// ResourceUsageResponse 資源使用率響應
type ResourceUsageResponse struct {
	CPU     ResourceUsageData `json:"cpu"`
	Memory  ResourceUsageData `json:"memory"`
	Storage ResourceUsageData `json:"storage"`
}

// ResourceUsageData 資源使用資料
type ResourceUsageData struct {
	UsagePercent float64 `json:"usagePercent"`
	Used         float64 `json:"used"`
	Total        float64 `json:"total"`
	Unit         string  `json:"unit"`
}

// ResourceDistributionResponse 資源分佈響應
type ResourceDistributionResponse struct {
	PodDistribution    []ClusterResourceCount `json:"podDistribution"`
	NodeDistribution   []ClusterResourceCount `json:"nodeDistribution"`
	CPUDistribution    []ClusterResourceCount `json:"cpuDistribution"`
	MemoryDistribution []ClusterResourceCount `json:"memoryDistribution"`
}

// ClusterResourceCount 叢集資源計數
type ClusterResourceCount struct {
	ClusterID   uint    `json:"clusterId"`
	ClusterName string  `json:"clusterName"`
	Value       float64 `json:"value"`
}

// TrendResponse 趨勢資料響應
type TrendResponse struct {
	PodTrends  []ClusterTrendSeries `json:"podTrends"`
	NodeTrends []ClusterTrendSeries `json:"nodeTrends"`
}

// ClusterTrendSeries 叢集趨勢序列
type ClusterTrendSeries struct {
	ClusterID   uint             `json:"clusterId"`
	ClusterName string           `json:"clusterName"`
	DataPoints  []TrendDataPoint `json:"dataPoints"`
}

// TrendDataPoint 趨勢資料點
type TrendDataPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// AbnormalWorkload 異常工作負載
type AbnormalWorkload struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	ClusterID   uint   `json:"clusterId"`
	ClusterName string `json:"clusterName"`
	Type        string `json:"type"`
	Reason      string `json:"reason"`
	Message     string `json:"message"`
	Duration    string `json:"duration"`
	Severity    string `json:"severity"`
}

// GlobalAlertStats 全域性告警統計
type GlobalAlertStats struct {
	Total        int                 `json:"total"`        // 告警總數
	Firing       int                 `json:"firing"`       // 觸發中
	Pending      int                 `json:"pending"`      // 等待中
	Resolved     int                 `json:"resolved"`     // 已解決
	Suppressed   int                 `json:"suppressed"`   // 已抑制
	BySeverity   map[string]int      `json:"bySeverity"`   // 按嚴重程度統計
	ByCluster    []ClusterAlertCount `json:"byCluster"`    // 按叢集統計
	EnabledCount int                 `json:"enabledCount"` // 已啟用告警的叢集數
}

// ClusterAlertCount 叢集告警計數
type ClusterAlertCount struct {
	ClusterID   uint   `json:"clusterId"`
	ClusterName string `json:"clusterName"`
	Total       int    `json:"total"`
	Firing      int    `json:"firing"`
}
