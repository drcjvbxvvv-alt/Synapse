package metrics

import "github.com/prometheus/client_golang/prometheus"

// WSMetrics holds Prometheus metrics for WebSocket connections.
// Label "type" values: pod-exec, kubectl, ssh, log-stream.
type WSMetrics struct {
	Active      *prometheus.GaugeVec
	Total       *prometheus.CounterVec
	ErrorsTotal *prometheus.CounterVec
}

func newWSMetrics(reg prometheus.Registerer) *WSMetrics {
	m := &WSMetrics{
		Active: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "synapse",
			Name:      "websocket_connections_active",
			Help:      "Current number of active WebSocket connections.",
		}, []string{"type"}),

		Total: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "synapse",
			Name:      "websocket_connections_total",
			Help:      "Total number of WebSocket connections established.",
		}, []string{"type"}),

		ErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "synapse",
			Name:      "websocket_errors_total",
			Help:      "Total number of WebSocket connection errors.",
		}, []string{"type"}),
	}
	reg.MustRegister(m.Active, m.Total, m.ErrorsTotal)
	return m
}

// Connect records a new WebSocket connection of the given type.
func (m *WSMetrics) Connect(connType string) {
	m.Total.WithLabelValues(connType).Inc()
	m.Active.WithLabelValues(connType).Inc()
}

// Disconnect records a closed WebSocket connection.
func (m *WSMetrics) Disconnect(connType string) {
	m.Active.WithLabelValues(connType).Dec()
}

// Error records a WebSocket error.
func (m *WSMetrics) Error(connType string) {
	m.ErrorsTotal.WithLabelValues(connType).Inc()
}
