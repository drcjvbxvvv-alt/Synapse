package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// GetRun — validation
// ---------------------------------------------------------------------------

func TestGetRun_EmptyRunID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"owner":"o","repo":"r"}`)
	if _, err := a.GetRun(context.Background(), ""); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestGetRun_MissingOwnerRepo(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	if _, err := a.GetRun(context.Background(), "1"); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestGetRun_InvalidNumeric(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"owner":"o","repo":"r"}`)
	if _, err := a.GetRun(context.Background(), "abc"); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetRun — happy paths
// ---------------------------------------------------------------------------

func TestGetRun_Success(t *testing.T) {
	start := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 16, 12, 5, 0, 0, time.UTC)
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/actions/runs/42"):
			_ = json.NewEncoder(w).Encode(workflowRun{
				ID:           42,
				Status:       "completed",
				Conclusion:   "success",
				RunStartedAt: &start,
				UpdatedAt:    &end,
			})
		case strings.Contains(r.URL.Path, "/runs/42/jobs"):
			_ = json.NewEncoder(w).Encode(workflowJobList{
				TotalCount: 1,
				Jobs: []workflowJob{{
					ID: 1, Name: "build",
					Status: "completed", Conclusion: "success",
					StartedAt:   &start,
					CompletedAt: &end,
				}},
			})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}, `{"owner":"o","repo":"r"}`)

	rs, err := a.GetRun(context.Background(), "42")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.Phase != engine.RunPhaseSuccess {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if rs.Raw != "completed/success" {
		t.Fatalf("Raw = %q", rs.Raw)
	}
	if rs.StartedAt == nil || !rs.StartedAt.Equal(start) {
		t.Fatalf("StartedAt = %v", rs.StartedAt)
	}
	if rs.FinishedAt == nil || !rs.FinishedAt.Equal(end) {
		t.Fatalf("FinishedAt = %v", rs.FinishedAt)
	}
	if len(rs.Steps) != 1 || rs.Steps[0].Name != "build" {
		t.Fatalf("Steps = %+v", rs.Steps)
	}
}

func TestGetRun_JobsFailure_StillReturnsRun(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/actions/runs/42") {
			_ = json.NewEncoder(w).Encode(workflowRun{ID: 42, Status: "in_progress"})
			return
		}
		if strings.Contains(r.URL.Path, "/jobs") {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}, `{"owner":"o","repo":"r"}`)

	rs, err := a.GetRun(context.Background(), "42")
	if err != nil {
		t.Fatalf("GetRun should not fail when only jobs errors: %v", err)
	}
	if rs.Phase != engine.RunPhaseRunning {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if !strings.Contains(rs.Raw, "jobs unavailable") {
		t.Fatalf("Raw should carry diagnostic: %q", rs.Raw)
	}
}

func TestGetRun_NotFound(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, `{"owner":"o","repo":"r"}`)
	if _, err := a.GetRun(context.Background(), "42"); !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetRun — placeholder resolution
// ---------------------------------------------------------------------------

func TestGetRun_Placeholder_Resolves(t *testing.T) {
	// First call: /actions/workflows/w/runs (polling) returns the new run.
	// Second call: /actions/runs/7 returns details.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/workflows/w/runs"):
			now := time.Now().UTC()
			_ = json.NewEncoder(w).Encode(workflowRunList{
				WorkflowRuns: []workflowRun{{ID: 7, Event: "workflow_dispatch", Status: "queued", CreatedAt: &now}},
			})
		case strings.HasSuffix(r.URL.Path, "/actions/runs/7"):
			_ = json.NewEncoder(w).Encode(workflowRun{ID: 7, Status: "in_progress"})
		case strings.Contains(r.URL.Path, "/jobs"):
			_ = json.NewEncoder(w).Encode(workflowJobList{})
		default:
			t.Errorf("unexpected %q", r.URL.Path)
		}
	}, `{"owner":"o","repo":"r","workflow_id":"w"}`)

	// Use a cutoff that allows the "now" run to match.
	rs, err := a.GetRun(context.Background(), "dispatch:main@1700000000")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.Phase != engine.RunPhaseRunning {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if rs.RunID != "7" {
		t.Fatalf("should upgrade to numeric RunID, got %q", rs.RunID)
	}
}

func TestGetRun_Placeholder_StillPending(t *testing.T) {
	// Polling returns no runs → adapter reports RunPhasePending with the
	// dispatch-pending Raw marker.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(workflowRunList{})
	}, `{"owner":"o","repo":"r","workflow_id":"w"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	rs, err := a.GetRun(ctx, "dispatch:main@1700000000")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.Phase != engine.RunPhasePending {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if rs.Raw != "dispatch-pending" {
		t.Fatalf("Raw = %q", rs.Raw)
	}
}

func TestGetRun_Placeholder_MissingWorkflowID(t *testing.T) {
	// Placeholder RunID but config has no workflow_id → ErrInvalidInput.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"owner":"o","repo":"r"}`)
	_, err := a.GetRun(context.Background(), "dispatch:main@1700000000")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestStepsFromJobs_Empty(t *testing.T) {
	if got := stepsFromJobs(nil); got == nil || len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestStepsFromJobs_Shape(t *testing.T) {
	jobs := []workflowJob{{ID: 1, Name: "test", Status: "completed", Conclusion: "success"}}
	got := stepsFromJobs(jobs)
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Name != "test" || got[0].Phase != engine.RunPhaseSuccess || got[0].Raw != "completed/success" {
		t.Fatalf("step = %+v", got[0])
	}
}
