package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	smetrics "github.com/shaia/Synapse/internal/metrics"
)

// PrometheusMetrics records HTTP method, path (template), status code, and
// latency for every request. WebSocket upgrade paths (/ws/) are excluded to
// avoid label-cardinality explosion.
//
// Pass a nil *smetrics.HTTPMetrics to disable (no-op middleware).
func PrometheusMetrics(m *smetrics.HTTPMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m == nil || len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/ws/" {
			c.Next()
			return
		}

		m.RequestsInFlight.Inc()
		defer m.RequestsInFlight.Dec()

		start := time.Now()
		c.Next()

		dur := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		m.RequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		m.RequestDuration.WithLabelValues(c.Request.Method, path).Observe(dur)
	}
}
