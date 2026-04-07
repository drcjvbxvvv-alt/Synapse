package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// WorkerMetrics holds Prometheus metrics for background workers.
// The "worker" label values are: cost, event_alert, log_retention.
type WorkerMetrics struct {
	LastRunTimestamp *prometheus.GaugeVec
	RunDuration      *prometheus.GaugeVec
	ErrorsTotal      *prometheus.CounterVec
}

func newWorkerMetrics(reg prometheus.Registerer) *WorkerMetrics {
	m := &WorkerMetrics{
		LastRunTimestamp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "synapse",
			Name:      "worker_last_run_timestamp",
			Help:      "Unix timestamp of the last successful worker run.",
		}, []string{"worker"}),

		RunDuration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "synapse",
			Name:      "worker_run_duration_seconds",
			Help:      "Duration in seconds of the last worker run.",
		}, []string{"worker"}),

		ErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "synapse",
			Name:      "worker_errors_total",
			Help:      "Total number of worker run errors.",
		}, []string{"worker"}),
	}
	reg.MustRegister(m.LastRunTimestamp, m.RunDuration, m.ErrorsTotal)
	return m
}

// WorkerRun is a helper used to time a single worker execution.
// Obtain one via WorkerMetrics.Start(), call Done() when finished.
type WorkerRun struct {
	m       *WorkerMetrics
	name    string
	startAt time.Time
}

// Start records the beginning of a worker run and returns a handle.
func (m *WorkerMetrics) Start(name string) *WorkerRun {
	return &WorkerRun{m: m, name: name, startAt: time.Now()}
}

// Done records the completion of a worker run. Pass a non-nil err to
// increment the error counter; pass nil on success.
func (r *WorkerRun) Done(err error) {
	dur := time.Since(r.startAt).Seconds()
	r.m.RunDuration.WithLabelValues(r.name).Set(dur)
	r.m.LastRunTimestamp.WithLabelValues(r.name).Set(float64(time.Now().Unix()))
	if err != nil {
		r.m.ErrorsTotal.WithLabelValues(r.name).Inc()
	}
}
