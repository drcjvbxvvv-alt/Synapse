package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "kubepolaris",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "kubepolaris",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// ClusterInformerStatus allows the k8s manager to expose per-cluster sync state.
	ClusterInformerSynced = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "kubepolaris",
			Name:      "cluster_informer_synced",
			Help:      "1 if the cluster Informer cache is fully synced, 0 otherwise.",
		},
		[]string{"cluster_id"},
	)
)

// PrometheusMetrics records HTTP method, path, status code, and latency for
// every request.  WebSocket upgrade paths (/ws/) are excluded to avoid
// label-cardinality explosion.
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip WebSocket paths — they are long-lived and don't produce normal status codes.
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/ws/" {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		// Use the matched route pattern (not the raw URL) to avoid high cardinality.
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}
