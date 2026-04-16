package gitlab

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
// mapGitLabStatus — lookup table coverage
// ---------------------------------------------------------------------------

func TestMapGitLabStatus_AllKnownValues(t *testing.T) {
	cases := map[string]engine.RunPhase{
		"created":              engine.RunPhasePending,
		"waiting_for_resource": engine.RunPhasePending,
		"preparing":            engine.RunPhasePending,
		"pending":              engine.RunPhasePending,
		"scheduled":            engine.RunPhasePending,
		"manual":               engine.RunPhasePending,
		"running":              engine.RunPhaseRunning,
		"success":              engine.RunPhaseSuccess,
		"failed":               engine.RunPhaseFailed,
		"canceled":             engine.RunPhaseCancelled,
		"skipped":              engine.RunPhaseCancelled,
	}
	for raw, want := range cases {
		if got := mapGitLabStatus(raw); got != want {
			t.Fatalf("mapGitLabStatus(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestMapGitLabStatus_Unknown(t *testing.T) {
	if mapGitLabStatus("paused-for-external-approval") != engine.RunPhaseUnknown {
		t.Fatal("unknown status should map to RunPhaseUnknown")
	}
	if mapGitLabStatus("") != engine.RunPhaseUnknown {
		t.Fatal("empty status should map to RunPhaseUnknown")
	}
}

// ---------------------------------------------------------------------------
// GetRun — validation
// ---------------------------------------------------------------------------

func TestGetRun_EmptyRunID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"project_id":42}`)
	_, err := a.GetRun(context.Background(), "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetRun_MissingProjectID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	_, err := a.GetRun(context.Background(), "1")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetRun_NonNumericRunID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"project_id":42}`)
	_, err := a.GetRun(context.Background(), "abc")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetRun — happy path
// ---------------------------------------------------------------------------

func TestGetRun_AggregatesPipelineAndJobs(t *testing.T) {
	started := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	finished := time.Date(2026, 4, 16, 12, 5, 0, 0, time.UTC)

	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/v4/projects/42/pipelines/77"):
			_ = json.NewEncoder(w).Encode(gitlabPipeline{
				ID:         77,
				ProjectID:  42,
				Status:     "success",
				Ref:        "main",
				StartedAt:  &started,
				FinishedAt: &finished,
			})
		case strings.HasSuffix(r.URL.Path, "/api/v4/projects/42/pipelines/77/jobs"):
			_ = json.NewEncoder(w).Encode([]gitlabJob{
				{ID: 1, Name: "build", Stage: "build", Status: "success", StartedAt: &started, FinishedAt: &finished},
				{ID: 2, Name: "test", Stage: "test", Status: "success", StartedAt: &started, FinishedAt: &finished},
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}, `{"project_id":42}`)

	rs, err := a.GetRun(context.Background(), "77")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.RunID != "77" || rs.ExternalID != "77" {
		t.Fatalf("run id mismatch: %+v", rs)
	}
	if rs.Phase != engine.RunPhaseSuccess {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if rs.Raw != "success" {
		t.Fatalf("Raw = %q", rs.Raw)
	}
	if !rs.StartedAt.Equal(started) || !rs.FinishedAt.Equal(finished) {
		t.Fatalf("time fields not propagated: %+v", rs)
	}
	if len(rs.Steps) != 2 {
		t.Fatalf("Steps len = %d", len(rs.Steps))
	}
	if rs.Steps[0].Name != "build" || rs.Steps[0].Phase != engine.RunPhaseSuccess {
		t.Fatalf("unexpected step[0]: %+v", rs.Steps[0])
	}
}

func TestGetRun_JobsFailure_StillReturnsPipelineStatus(t *testing.T) {
	// If the /jobs endpoint fails, GetRun should still succeed and encode
	// the error in RunStatus.Raw — the UI can show "jobs temporarily
	// unavailable" instead of the whole page erroring out.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/pipelines/77") {
			_ = json.NewEncoder(w).Encode(gitlabPipeline{
				ID: 77, ProjectID: 42, Status: "running",
			})
			return
		}
		if strings.Contains(r.URL.Path, "/jobs") {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}, `{"project_id":42}`)

	rs, err := a.GetRun(context.Background(), "77")
	if err != nil {
		t.Fatalf("GetRun should not fail when only jobs endpoint errors: %v", err)
	}
	if rs.Phase != engine.RunPhaseRunning {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if !strings.Contains(rs.Raw, "jobs unavailable") {
		t.Fatalf("Raw should include jobs-unavailable diagnostic, got %q", rs.Raw)
	}
	if len(rs.Steps) != 0 {
		t.Fatalf("Steps len = %d, want 0", len(rs.Steps))
	}
}

func TestGetRun_PipelineNotFound(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, `{"project_id":42}`)

	_, err := a.GetRun(context.Background(), "77")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
