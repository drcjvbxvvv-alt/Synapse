package tekton

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// mapTektonStatus — lookup coverage
// ---------------------------------------------------------------------------

func TestMapTektonStatus_AllKnownValues(t *testing.T) {
	cases := []struct {
		in   succeededCondition
		want engine.RunPhase
	}{
		{succeededCondition{Status: "True"}, engine.RunPhaseSuccess},
		{succeededCondition{Status: "False", Reason: reasonCancelled}, engine.RunPhaseCancelled},
		{succeededCondition{Status: "False", Reason: reasonPipelineCancelled}, engine.RunPhaseCancelled},
		{succeededCondition{Status: "False", Reason: "Failed"}, engine.RunPhaseFailed},
		{succeededCondition{Status: "False"}, engine.RunPhaseFailed},
		{succeededCondition{Status: "Unknown", Reason: "Pending"}, engine.RunPhasePending},
		{succeededCondition{Status: "Unknown", Reason: "PipelineRunPending"}, engine.RunPhasePending},
		{succeededCondition{Status: "Unknown", Reason: "Running"}, engine.RunPhaseRunning},
		{succeededCondition{Status: "Unknown"}, engine.RunPhaseRunning},
		{succeededCondition{}, engine.RunPhasePending},
		{succeededCondition{Status: "WhatIsThis"}, engine.RunPhaseUnknown},
	}
	for _, tc := range cases {
		if got := mapTektonStatus(tc.in); got != tc.want {
			t.Fatalf("mapTektonStatus(%+v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// GetRun — validation
// ---------------------------------------------------------------------------

func TestGetRun_EmptyRunID(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"namespace":"ns"}`)
	_, err := a.GetRun(context.Background(), "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetRun_MissingNamespace(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), "")
	_, err := a.GetRun(context.Background(), "pr-1")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetRun_NilResolver(t *testing.T) {
	a := newAdapter(t, nil, `{"namespace":"ns"}`)
	_, err := a.GetRun(context.Background(), "pr-1")
	if !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetRun — happy paths
// ---------------------------------------------------------------------------

func TestGetRun_Success_WithTaskRuns(t *testing.T) {
	pr := newPipelineRun("pr-1", "ci", map[string]any{
		"startTime":      "2026-04-16T12:00:00Z",
		"completionTime": "2026-04-16T12:05:00Z",
		"conditions": []any{
			map[string]any{
				"type":    "Succeeded",
				"status":  "True",
				"reason":  "Succeeded",
				"message": "all good",
			},
		},
	})
	tr1 := newTaskRun("pr-1-build", "ci", "pr-1", "build", map[string]any{
		"conditions": []any{
			map[string]any{"type": "Succeeded", "status": "True"},
		},
	})
	tr2 := newTaskRun("pr-1-test", "ci", "pr-1", "test", map[string]any{
		"conditions": []any{
			map[string]any{"type": "Succeeded", "status": "True"},
		},
	})

	a := newAdapter(t, newDynamicResolver(t, pr, tr1, tr2), `{"namespace":"ci"}`)
	rs, err := a.GetRun(context.Background(), "pr-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.RunID != "pr-1" || rs.ExternalID != "pr-1" {
		t.Fatalf("id mismatch: %+v", rs)
	}
	if rs.Phase != engine.RunPhaseSuccess {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if rs.Message != "all good" {
		t.Fatalf("Message = %q", rs.Message)
	}
	if rs.StartedAt == nil || !rs.StartedAt.Equal(time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("StartedAt = %v", rs.StartedAt)
	}
	if rs.FinishedAt == nil || !rs.FinishedAt.Equal(time.Date(2026, 4, 16, 12, 5, 0, 0, time.UTC)) {
		t.Fatalf("FinishedAt = %v", rs.FinishedAt)
	}
	if len(rs.Steps) != 2 {
		t.Fatalf("Steps len = %d, want 2", len(rs.Steps))
	}
	stepNames := map[string]bool{}
	for _, s := range rs.Steps {
		stepNames[s.Name] = true
		if s.Phase != engine.RunPhaseSuccess {
			t.Fatalf("step %q phase = %q", s.Name, s.Phase)
		}
	}
	if !stepNames["build"] || !stepNames["test"] {
		t.Fatalf("expected steps build+test, got %+v", stepNames)
	}
}

func TestGetRun_NoTaskRuns(t *testing.T) {
	pr := newPipelineRun("pr-1", "ci", map[string]any{
		"conditions": []any{
			map[string]any{"type": "Succeeded", "status": "Unknown", "reason": "Running"},
		},
	})
	a := newAdapter(t, newDynamicResolver(t, pr), `{"namespace":"ci"}`)
	rs, err := a.GetRun(context.Background(), "pr-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.Phase != engine.RunPhaseRunning {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if len(rs.Steps) != 0 {
		t.Fatalf("Steps len = %d", len(rs.Steps))
	}
}

func TestGetRun_NoStatusYet(t *testing.T) {
	// Fresh PipelineRun without status block at all — should be pending.
	pr := newPipelineRun("pr-1", "ci", nil)
	a := newAdapter(t, newDynamicResolver(t, pr), `{"namespace":"ci"}`)
	rs, err := a.GetRun(context.Background(), "pr-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.Phase != engine.RunPhasePending {
		t.Fatalf("Phase = %q", rs.Phase)
	}
}

func TestGetRun_CancelledStatus(t *testing.T) {
	pr := newPipelineRun("pr-1", "ci", map[string]any{
		"conditions": []any{
			map[string]any{"type": "Succeeded", "status": "False", "reason": reasonPipelineCancelled},
		},
	})
	a := newAdapter(t, newDynamicResolver(t, pr), `{"namespace":"ci"}`)
	rs, err := a.GetRun(context.Background(), "pr-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.Phase != engine.RunPhaseCancelled {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if rs.Raw != "False/PipelineRunCancelled" {
		t.Fatalf("Raw = %q", rs.Raw)
	}
}

func TestGetRun_NotFound(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"namespace":"ci"}`)
	_, err := a.GetRun(context.Background(), "missing-pr")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

func TestReadSucceededCondition_Absent(t *testing.T) {
	obj := map[string]any{"status": map[string]any{"conditions": []any{}}}
	if c := readSucceededCondition(obj, "status"); c.Status != "" {
		t.Fatalf("expected zero value, got %+v", c)
	}
}

func TestReadSucceededCondition_WrongType(t *testing.T) {
	obj := map[string]any{"status": map[string]any{"conditions": []any{
		map[string]any{"type": "SomeOtherCondition", "status": "True"},
	}}}
	if c := readSucceededCondition(obj, "status"); c.Status != "" {
		t.Fatalf("should only match Succeeded, got %+v", c)
	}
}

func TestReadTimes_Malformed(t *testing.T) {
	obj := map[string]any{"status": map[string]any{"startTime": "not-a-time"}}
	if s, f := readTimes(obj); s != nil || f != nil {
		t.Fatalf("malformed should yield nils, got %v %v", s, f)
	}
}

func TestReadStepName_PrefersLabel(t *testing.T) {
	tr := newTaskRun("pr-1-build", "ci", "pr-1", "build-stage", nil)
	if got := readStepName(tr); got != "build-stage" {
		t.Fatalf("got %q, want build-stage", got)
	}
}

func TestReadStepName_FallsBackToName(t *testing.T) {
	tr := newTaskRun("fallback-tr", "ci", "pr-1", "", nil)
	// Remove the pipelineTask label so fallback kicks in.
	labels := tr.GetLabels()
	delete(labels, "tekton.dev/pipelineTask")
	tr.SetLabels(labels)
	if got := readStepName(tr); got != "fallback-tr" {
		t.Fatalf("got %q, want fallback-tr", got)
	}
}
