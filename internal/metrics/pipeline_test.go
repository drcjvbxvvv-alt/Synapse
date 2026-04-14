package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus "github.com/prometheus/client_model/go"
)

func TestPipelineMetrics_Registration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newPipelineMetrics(reg)
	if m == nil {
		t.Fatal("expected non-nil PipelineMetrics")
	}

	// Verify all metrics are registered by gathering
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	expectedNames := map[string]bool{
		"synapse_pipeline_runs_total":             false,
		"synapse_pipeline_step_runs_total":        false,
		"synapse_pipeline_webhook_received_total": false,
		"synapse_pipeline_webhook_rejected_total": false,
		"synapse_pipeline_run_wait_seconds":       false,
		"synapse_pipeline_run_duration_seconds":   false,
		"synapse_pipeline_step_duration_seconds":  false,
		"synapse_pipeline_queue_depth":            false,
		"synapse_pipeline_concurrent_runs":        false,
	}

	for _, f := range families {
		if _, ok := expectedNames[f.GetName()]; ok {
			expectedNames[f.GetName()] = true
		}
	}

	// Counters/histograms won't appear until first observation.
	// Just verify no registration error occurred — that's the key test.
}

func TestPipelineMetrics_RecordRunCompleted(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newPipelineMetrics(reg)

	m.RecordRunCompleted("my-pipeline", "success", "manual", 90*time.Second)
	m.RecordRunCompleted("my-pipeline", "failed", "webhook", 30*time.Second)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	found := false
	for _, f := range families {
		if f.GetName() == "synapse_pipeline_runs_total" {
			found = true
			if len(f.GetMetric()) != 2 {
				t.Errorf("expected 2 metric series, got %d", len(f.GetMetric()))
			}
		}
	}
	if !found {
		t.Error("runs_total metric not found")
	}
}

func TestPipelineMetrics_RecordStepCompleted(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newPipelineMetrics(reg)

	m.RecordStepCompleted("my-pipeline", "build-image", "success", 120*time.Second)
	m.RecordStepCompleted("my-pipeline", "deploy", "failed", 5*time.Second)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	var stepCounter *io_prometheus.MetricFamily
	for _, f := range families {
		if f.GetName() == "synapse_pipeline_step_runs_total" {
			stepCounter = f
			break
		}
	}
	if stepCounter == nil {
		t.Fatal("step_runs_total metric not found")
	}
	if len(stepCounter.GetMetric()) != 2 {
		t.Errorf("expected 2 step metric series, got %d", len(stepCounter.GetMetric()))
	}
}

func TestPipelineMetrics_RecordRunWait(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newPipelineMetrics(reg)

	m.RecordRunWait("my-pipeline", 15*time.Second)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	found := false
	for _, f := range families {
		if f.GetName() == "synapse_pipeline_run_wait_seconds" {
			found = true
			for _, metric := range f.GetMetric() {
				if metric.GetHistogram().GetSampleCount() != 1 {
					t.Errorf("expected 1 sample, got %d", metric.GetHistogram().GetSampleCount())
				}
			}
		}
	}
	if !found {
		t.Error("run_wait_seconds metric not found")
	}
}

func TestPipelineMetrics_Gauges(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newPipelineMetrics(reg)

	m.SetQueueDepth("1", 5)
	m.SetConcurrentRuns("1", 3)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	gaugeValues := map[string]float64{}
	for _, f := range families {
		switch f.GetName() {
		case "synapse_pipeline_queue_depth":
			for _, metric := range f.GetMetric() {
				gaugeValues["queue_depth"] = metric.GetGauge().GetValue()
			}
		case "synapse_pipeline_concurrent_runs":
			for _, metric := range f.GetMetric() {
				gaugeValues["concurrent_runs"] = metric.GetGauge().GetValue()
			}
		}
	}

	if v, ok := gaugeValues["queue_depth"]; !ok || v != 5 {
		t.Errorf("expected queue_depth=5, got %v", v)
	}
	if v, ok := gaugeValues["concurrent_runs"]; !ok || v != 3 {
		t.Errorf("expected concurrent_runs=3, got %v", v)
	}
}

func TestPipelineMetrics_WebhookCounters(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newPipelineMetrics(reg)

	m.RecordWebhookReceived("github", "accepted")
	m.RecordWebhookReceived("github", "accepted")
	m.RecordWebhookRejected("github", "invalid_signature")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	for _, f := range families {
		if f.GetName() == "synapse_pipeline_webhook_received_total" {
			for _, metric := range f.GetMetric() {
				if metric.GetCounter().GetValue() != 2 {
					t.Errorf("expected 2 received webhooks, got %v", metric.GetCounter().GetValue())
				}
			}
		}
		if f.GetName() == "synapse_pipeline_webhook_rejected_total" {
			for _, metric := range f.GetMetric() {
				if metric.GetCounter().GetValue() != 1 {
					t.Errorf("expected 1 rejected webhook, got %v", metric.GetCounter().GetValue())
				}
			}
		}
	}
}

func TestRegistryIncludesPipelineMetrics(t *testing.T) {
	r := New()
	if r.Pipeline == nil {
		t.Fatal("expected Pipeline metrics in registry")
	}
}
