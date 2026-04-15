package metrics

import "github.com/prometheus/client_golang/prometheus"

// K8sMetrics holds Prometheus metrics for the Kubernetes layer.
type K8sMetrics struct {
	// ClustersActive is the current number of clusters with an active Informer.
	ClustersActive prometheus.Gauge
	// InformerSynced tracks per-cluster Informer sync state (1=synced, 0=not).
	InformerSynced *prometheus.GaugeVec
	// APIRequestsTotal counts K8s API calls by cluster and resource kind.
	APIRequestsTotal *prometheus.CounterVec
	// APIErrorsTotal counts K8s API errors by cluster and resource kind.
	APIErrorsTotal *prometheus.CounterVec
}

// ---------------------------------------------------------------------------
// InformerAgeCollector — custom Prometheus Collector（ARCHITECTURE_REVIEW P1-9）
//
// 以 Collect() 呼叫時動態計算各叢集距最後 Informer 同步的秒數，確保數值永遠是最新的。
// 未曾同步的叢集回傳 -1。
// ---------------------------------------------------------------------------

// InformerAgeCollector 是一個自定義 prometheus.Collector，
// 在每次 scrape 時即時計算 informer_last_sync_age_seconds。
type InformerAgeCollector struct {
	desc    *prometheus.Desc
	getAges func() map[string]float64 // cluster_id → age in seconds (-1 if never synced)
}

// NewInformerAgeCollector 建立 InformerAgeCollector。
// getAges 是 ClusterInformerManager.GetSyncAges() 的函數引用。
func NewInformerAgeCollector(getAges func() map[string]float64) *InformerAgeCollector {
	return &InformerAgeCollector{
		desc: prometheus.NewDesc(
			"synapse_informer_last_sync_age_seconds",
			"Seconds since the last successful Informer cache sync per cluster. -1 if never synced.",
			[]string{"cluster_id"},
			nil,
		),
		getAges: getAges,
	}
}

// Describe implements prometheus.Collector.
func (c *InformerAgeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

// Collect implements prometheus.Collector — called on every Prometheus scrape.
func (c *InformerAgeCollector) Collect(ch chan<- prometheus.Metric) {
	for clusterID, age := range c.getAges() {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, age, clusterID)
	}
}

func newK8sMetrics(reg prometheus.Registerer) *K8sMetrics {
	m := &K8sMetrics{
		ClustersActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "synapse",
			Name:      "k8s_clusters_active",
			Help:      "Number of clusters with an active Informer cache.",
		}),

		InformerSynced: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "synapse",
			Name:      "cluster_informer_synced",
			Help:      "1 if the cluster Informer cache is fully synced, 0 otherwise.",
		}, []string{"cluster_id"}),

		APIRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "synapse",
			Name:      "k8s_api_requests_total",
			Help:      "Total number of Kubernetes API calls.",
		}, []string{"cluster_id", "resource"}),

		APIErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "synapse",
			Name:      "k8s_api_errors_total",
			Help:      "Total number of Kubernetes API errors.",
		}, []string{"cluster_id", "resource"}),
	}
	reg.MustRegister(m.ClustersActive, m.InformerSynced, m.APIRequestsTotal, m.APIErrorsTotal)
	return m
}
