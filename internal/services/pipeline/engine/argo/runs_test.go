package argo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// GetRun — validation
// ---------------------------------------------------------------------------

func TestGetRun_EmptyRunID(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), `{"namespace":"ci"}`)
	if _, err := a.GetRun(context.Background(), ""); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestGetRun_MissingNamespace(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), "")
	if _, err := a.GetRun(context.Background(), "wf-1"); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestGetRun_NilResolver(t *testing.T) {
	a := newAdapter(t, nil, `{"namespace":"ci"}`)
	if _, err := a.GetRun(context.Background(), "wf-1"); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestGetRun_NotFound(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), `{"namespace":"ci"}`)
	if _, err := a.GetRun(context.Background(), "missing"); !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetRun — happy paths
// ---------------------------------------------------------------------------

func TestGetRun_Success_WithNodes(t *testing.T) {
	wf := newWorkflow("wf-1", "ci", map[string]any{
		"phase":      "Succeeded",
		"startedAt":  "2026-04-16T12:00:00Z",
		"finishedAt": "2026-04-16T12:05:00Z",
		"message":    "ok",
		"nodes": map[string]any{
			"wf-1-build": map[string]any{
				"id":          "wf-1-build",
				"displayName": "build",
				"phase":       "Succeeded",
				"startedAt":   "2026-04-16T12:00:00Z",
				"finishedAt":  "2026-04-16T12:02:00Z",
			},
			"wf-1-test": map[string]any{
				"id":          "wf-1-test",
				"displayName": "test",
				"phase":       "Succeeded",
			},
		},
	})
	a := newAdapter(t, newResolverArgoInstalled(t, wf), `{"namespace":"ci"}`)
	rs, err := a.GetRun(context.Background(), "wf-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.Phase != engine.RunPhaseSuccess {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if rs.Message != "ok" {
		t.Fatalf("Message = %q", rs.Message)
	}
	if rs.StartedAt == nil || !rs.StartedAt.Equal(time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("StartedAt = %v", rs.StartedAt)
	}
	if rs.FinishedAt == nil || !rs.FinishedAt.Equal(time.Date(2026, 4, 16, 12, 5, 0, 0, time.UTC)) {
		t.Fatalf("FinishedAt = %v", rs.FinishedAt)
	}
	if len(rs.Steps) != 2 {
		t.Fatalf("Steps len = %d", len(rs.Steps))
	}
	names := map[string]bool{}
	for _, s := range rs.Steps {
		names[s.Name] = true
		if s.Phase != engine.RunPhaseSuccess {
			t.Fatalf("step %q phase = %q", s.Name, s.Phase)
		}
	}
	if !names["build"] || !names["test"] {
		t.Fatalf("missing step names: %+v", names)
	}
}

func TestGetRun_NoStatus_Pending(t *testing.T) {
	wf := newWorkflow("wf-1", "ci", nil)
	a := newAdapter(t, newResolverArgoInstalled(t, wf), `{"namespace":"ci"}`)
	rs, err := a.GetRun(context.Background(), "wf-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.Phase != engine.RunPhasePending {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if len(rs.Steps) != 0 {
		t.Fatalf("Steps should be empty")
	}
}

func TestGetRun_Failed_Error_MapsToFailed(t *testing.T) {
	for _, p := range []string{"Failed", "Error"} {
		wf := newWorkflow("wf-1", "ci", map[string]any{"phase": p})
		a := newAdapter(t, newResolverArgoInstalled(t, wf), `{"namespace":"ci"}`)
		rs, err := a.GetRun(context.Background(), "wf-1")
		if err != nil {
			t.Fatalf("GetRun %q: %v", p, err)
		}
		if rs.Phase != engine.RunPhaseFailed {
			t.Fatalf("%q → %q, want Failed", p, rs.Phase)
		}
	}
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

func TestStepNameFromNode_Priority(t *testing.T) {
	// displayName wins over name over id.
	full := map[string]any{"id": "id1", "name": "name1", "displayName": "disp1"}
	if got := stepNameFromNode(full); got != "disp1" {
		t.Fatalf("got %q", got)
	}
	nameOnly := map[string]any{"id": "id1", "name": "name1"}
	if got := stepNameFromNode(nameOnly); got != "name1" {
		t.Fatalf("got %q", got)
	}
	idOnly := map[string]any{"id": "id1"}
	if got := stepNameFromNode(idOnly); got != "id1" {
		t.Fatalf("got %q", got)
	}
	empty := map[string]any{}
	if got := stepNameFromNode(empty); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestParseTimeField_Malformed(t *testing.T) {
	m := map[string]any{"t": "not-a-time"}
	if v := parseTimeField(m, "t"); v != nil {
		t.Fatalf("got %v, want nil", v)
	}
}

func TestParseTimeField_Absent(t *testing.T) {
	if v := parseTimeField(map[string]any{}, "t"); v != nil {
		t.Fatalf("got %v, want nil", v)
	}
}
