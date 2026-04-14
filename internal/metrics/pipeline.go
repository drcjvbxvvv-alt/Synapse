package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ---------------------------------------------------------------------------
// PipelineMetrics — Pipeline 子系統 Prometheus 指標
//
// 對齊 CICD_ARCHITECTURE §16.1 設計：
//   - Counters: runs_total, step_runs_total, webhook_received_total, webhook_rejected_total
//   - Histograms: run_wait_seconds, run_duration_seconds, step_duration_seconds
//   - Gauges: queue_depth, concurrent_runs
//
// Label cardinality 控制：
//   - pipeline_id 使用 pipeline name（不超過 ~200），不使用 numeric ID
//   - step_name 限制在 pipeline 定義內（通常 < 20 per pipeline）
//   - status 值域固定（queued/running/success/failed/cancelled）
// ---------------------------------------------------------------------------

// PipelineMetrics holds all Pipeline-related Prometheus metrics.
type PipelineMetrics struct {
	// Counters
	RunsTotal            *prometheus.CounterVec
	StepRunsTotal        *prometheus.CounterVec
	WebhookReceivedTotal *prometheus.CounterVec
	WebhookRejectedTotal *prometheus.CounterVec

	// Histograms
	RunWaitSeconds      *prometheus.HistogramVec
	RunDurationSeconds  *prometheus.HistogramVec
	StepDurationSeconds *prometheus.HistogramVec

	// Gauges
	QueueDepth     *prometheus.GaugeVec
	ConcurrentRuns *prometheus.GaugeVec
}

func newPipelineMetrics(reg prometheus.Registerer) *PipelineMetrics {
	m := &PipelineMetrics{
		// ── Counters ──────────────────────────────────────────────────────

		RunsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "synapse",
			Subsystem: "pipeline",
			Name:      "runs_total",
			Help:      "Total number of pipeline runs by status and trigger type.",
		}, []string{"pipeline", "status", "trigger_type"}),

		StepRunsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "synapse",
			Subsystem: "pipeline",
			Name:      "step_runs_total",
			Help:      "Total number of step runs by step type and status.",
		}, []string{"pipeline", "step_type", "status"}),

		WebhookReceivedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "synapse",
			Subsystem: "pipeline",
			Name:      "webhook_received_total",
			Help:      "Total number of webhook events received by provider.",
		}, []string{"provider", "outcome"}),

		WebhookRejectedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "synapse",
			Subsystem: "pipeline",
			Name:      "webhook_rejected_total",
			Help:      "Total number of webhook events rejected by reason.",
		}, []string{"provider", "reason"}),

		// ── Histograms ───────────────────────────────────────────────────

		RunWaitSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "synapse",
			Subsystem: "pipeline",
			Name:      "run_wait_seconds",
			Help:      "Time from queued to running (queue wait time).",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
		}, []string{"pipeline"}),

		RunDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "synapse",
			Subsystem: "pipeline",
			Name:      "run_duration_seconds",
			Help:      "Total pipeline run duration from running to finished.",
			Buckets:   []float64{10, 30, 60, 120, 300, 600, 1200, 1800, 3600},
		}, []string{"pipeline", "status"}),

		StepDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "synapse",
			Subsystem: "pipeline",
			Name:      "step_duration_seconds",
			Help:      "Step execution duration by step type.",
			Buckets:   []float64{5, 10, 30, 60, 120, 300, 600, 1200},
		}, []string{"step_type", "status"}),

		// ── Gauges ────────────────────────────────────────────────────────

		QueueDepth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "synapse",
			Subsystem: "pipeline",
			Name:      "queue_depth",
			Help:      "Number of pipeline runs currently in queued state.",
		}, []string{"cluster_id"}),

		ConcurrentRuns: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "synapse",
			Subsystem: "pipeline",
			Name:      "concurrent_runs",
			Help:      "Number of pipeline runs currently executing.",
		}, []string{"cluster_id"}),
	}

	reg.MustRegister(
		m.RunsTotal,
		m.StepRunsTotal,
		m.WebhookReceivedTotal,
		m.WebhookRejectedTotal,
		m.RunWaitSeconds,
		m.RunDurationSeconds,
		m.StepDurationSeconds,
		m.QueueDepth,
		m.ConcurrentRuns,
	)

	return m
}

// ---------------------------------------------------------------------------
// Convenience helpers
// ---------------------------------------------------------------------------

// RecordRunCompleted increments the runs_total counter and observes duration.
func (m *PipelineMetrics) RecordRunCompleted(pipeline, status, triggerType string, duration time.Duration) {
	m.RunsTotal.WithLabelValues(pipeline, status, triggerType).Inc()
	m.RunDurationSeconds.WithLabelValues(pipeline, status).Observe(duration.Seconds())
}

// RecordRunWait observes the queue wait time (queued → running).
func (m *PipelineMetrics) RecordRunWait(pipeline string, waitTime time.Duration) {
	m.RunWaitSeconds.WithLabelValues(pipeline).Observe(waitTime.Seconds())
}

// RecordStepCompleted increments the step_runs_total counter and observes duration.
func (m *PipelineMetrics) RecordStepCompleted(pipeline, stepType, status string, duration time.Duration) {
	m.StepRunsTotal.WithLabelValues(pipeline, stepType, status).Inc()
	m.StepDurationSeconds.WithLabelValues(stepType, status).Observe(duration.Seconds())
}

// RecordWebhookReceived increments the webhook_received_total counter.
func (m *PipelineMetrics) RecordWebhookReceived(provider, outcome string) {
	m.WebhookReceivedTotal.WithLabelValues(provider, outcome).Inc()
}

// RecordWebhookRejected increments the webhook_rejected_total counter.
func (m *PipelineMetrics) RecordWebhookRejected(provider, reason string) {
	m.WebhookRejectedTotal.WithLabelValues(provider, reason).Inc()
}

// SetQueueDepth sets the current queue depth gauge for a cluster.
func (m *PipelineMetrics) SetQueueDepth(clusterID string, depth float64) {
	m.QueueDepth.WithLabelValues(clusterID).Set(depth)
}

// SetConcurrentRuns sets the current concurrent runs gauge for a cluster.
func (m *PipelineMetrics) SetConcurrentRuns(clusterID string, count float64) {
	m.ConcurrentRuns.WithLabelValues(clusterID).Set(count)
}
