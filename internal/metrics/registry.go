package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry holds a custom Prometheus registry and all sub-metric groups.
type Registry struct {
	reg       *prometheus.Registry
	HTTP      *HTTPMetrics
	WebSocket *WSMetrics
	DB        *DBMetrics
	Worker    *WorkerMetrics
	K8s       *K8sMetrics
	Pipeline  *PipelineMetrics
}

// New creates a Registry with all metric groups registered. It includes
// Go runtime and process collectors automatically.
func New() *Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return &Registry{
		reg:       reg,
		HTTP:      newHTTPMetrics(reg),
		WebSocket: newWSMetrics(reg),
		DB:        newDBMetrics(reg),
		Worker:    newWorkerMetrics(reg),
		K8s:       newK8sMetrics(reg),
		Pipeline:  newPipelineMetrics(reg),
	}
}

// Handler returns an http.Handler that serves the custom registry.
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{EnableOpenMetrics: false})
}
