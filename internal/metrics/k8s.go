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
