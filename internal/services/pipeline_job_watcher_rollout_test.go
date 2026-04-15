package services

import (
	"encoding/json"
	"testing"

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// parseRolloutTarget tests
// ---------------------------------------------------------------------------

func TestParseRolloutTarget_DeployRollout(t *testing.T) {
	w := &JobWatcher{}
	cfg, _ := json.Marshal(DeployRolloutConfig{
		RolloutName: "my-app",
		Namespace:   "production",
		Image:       "my-app:v2",
	})
	sr := &models.StepRun{
		StepType:   "deploy-rollout",
		ConfigJSON: string(cfg),
	}
	name, ns := w.parseRolloutTarget(sr)
	if name != "my-app" {
		t.Errorf("expected name=my-app, got %q", name)
	}
	if ns != "production" {
		t.Errorf("expected namespace=production, got %q", ns)
	}
}

func TestParseRolloutTarget_RolloutStatus(t *testing.T) {
	w := &JobWatcher{}
	cfg, _ := json.Marshal(RolloutStatusConfig{
		RolloutName:    "canary-svc",
		Namespace:      "staging",
		ExpectedStatus: "healthy",
	})
	sr := &models.StepRun{
		StepType:   "rollout-status",
		ConfigJSON: string(cfg),
	}
	name, ns := w.parseRolloutTarget(sr)
	if name != "canary-svc" {
		t.Errorf("expected name=canary-svc, got %q", name)
	}
	if ns != "staging" {
		t.Errorf("expected namespace=staging, got %q", ns)
	}
}

func TestParseRolloutTarget_EmptyConfig(t *testing.T) {
	w := &JobWatcher{}
	sr := &models.StepRun{
		StepType:   "deploy-rollout",
		ConfigJSON: "",
	}
	name, ns := w.parseRolloutTarget(sr)
	if name != "" || ns != "" {
		t.Errorf("expected empty for empty config, got %q/%q", name, ns)
	}
}

func TestParseRolloutTarget_UnsupportedType(t *testing.T) {
	w := &JobWatcher{}
	sr := &models.StepRun{
		StepType:   "run-script",
		ConfigJSON: `{"rollout_name":"x","namespace":"y"}`,
	}
	name, ns := w.parseRolloutTarget(sr)
	if name != "" || ns != "" {
		t.Errorf("expected empty for unsupported type, got %q/%q", name, ns)
	}
}

func TestParseRolloutTarget_InvalidJSON(t *testing.T) {
	w := &JobWatcher{}
	sr := &models.StepRun{
		StepType:   "deploy-rollout",
		ConfigJSON: "not-json",
	}
	name, ns := w.parseRolloutTarget(sr)
	if name != "" || ns != "" {
		t.Errorf("expected empty for invalid JSON, got %q/%q", name, ns)
	}
}

// ---------------------------------------------------------------------------
// enrichRolloutFields nil-safety tests
// ---------------------------------------------------------------------------

func TestEnrichRolloutFields_NilRolloutSvc(t *testing.T) {
	w := &JobWatcher{rolloutSvc: nil}
	sr := &models.StepRun{
		StepType:   "deploy-rollout",
		ConfigJSON: `{"rollout_name":"x","namespace":"y","image":"z"}`,
	}
	// Should not panic
	w.enrichRolloutFields(nil, sr, 1)
	if sr.RolloutStatus != "" {
		t.Errorf("expected empty rollout_status, got %q", sr.RolloutStatus)
	}
}

func TestEnrichRolloutFields_EmptyTarget(t *testing.T) {
	w := &JobWatcher{rolloutSvc: NewRolloutService()}
	sr := &models.StepRun{
		StepType:   "deploy-rollout",
		ConfigJSON: "",
	}
	// Should not panic — returns early since no rollout name
	w.enrichRolloutFields(nil, sr, 1)
	if sr.RolloutStatus != "" {
		t.Errorf("expected empty rollout_status, got %q", sr.RolloutStatus)
	}
}
