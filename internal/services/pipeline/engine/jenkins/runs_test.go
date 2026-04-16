package jenkins

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
// mapJenkinsStatus
// ---------------------------------------------------------------------------

func TestMapJenkinsStatus_AllKnownValues(t *testing.T) {
	cases := []struct {
		result   string
		building bool
		want     engine.RunPhase
	}{
		{"", true, engine.RunPhaseRunning},   // building=true dominates
		{"SUCCESS", false, engine.RunPhaseSuccess},
		{"FAILURE", false, engine.RunPhaseFailed},
		{"UNSTABLE", false, engine.RunPhaseFailed},
		{"ABORTED", false, engine.RunPhaseCancelled},
		{"", false, engine.RunPhasePending},
		{"NOT_BUILT", false, engine.RunPhaseUnknown},
	}
	for _, tc := range cases {
		if got := mapJenkinsStatus(tc.result, tc.building); got != tc.want {
			t.Fatalf("mapJenkinsStatus(%q, %v) = %q, want %q", tc.result, tc.building, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// GetRun — validation
// ---------------------------------------------------------------------------

func TestGetRun_EmptyRunID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	_, err := a.GetRun(context.Background(), "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetRun_MissingJobPath(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	_, err := a.GetRun(context.Background(), "42")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetRun_NonNumericRunID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	_, err := a.GetRun(context.Background(), "abc")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetRun_InvalidQueueID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	_, err := a.GetRun(context.Background(), "queue:not-a-number")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetRun — by build number
// ---------------------------------------------------------------------------

func TestGetRun_Success(t *testing.T) {
	startMs := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC).UnixMilli()
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/job/foo/100/api/json") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(jenkinsBuild{
			Number:    100,
			Result:    "SUCCESS",
			Building:  false,
			Timestamp: startMs,
			Duration:  60 * 1000,
		})
	}, `{"job_path":"foo"}`)

	rs, err := a.GetRun(context.Background(), "100")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.RunID != "100" || rs.ExternalID != "100" {
		t.Fatalf("ids mismatch: %+v", rs)
	}
	if rs.Phase != engine.RunPhaseSuccess {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if rs.Raw != "SUCCESS" {
		t.Fatalf("Raw = %q", rs.Raw)
	}
	if rs.StartedAt == nil || rs.FinishedAt == nil {
		t.Fatal("expected both timestamps")
	}
	if fin := rs.FinishedAt.Sub(*rs.StartedAt); fin != time.Minute {
		t.Fatalf("duration computation wrong: %v", fin)
	}
}

func TestGetRun_Building_NoFinishTime(t *testing.T) {
	startMs := time.Now().UnixMilli()
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jenkinsBuild{
			Number:    5,
			Building:  true,
			Timestamp: startMs,
			Duration:  0,
		})
	}, `{"job_path":"foo"}`)

	rs, err := a.GetRun(context.Background(), "5")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.Phase != engine.RunPhaseRunning {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if rs.FinishedAt != nil {
		t.Fatalf("FinishedAt should be nil while building")
	}
	if rs.Raw != "BUILDING" {
		t.Fatalf("Raw = %q, expected BUILDING synthetic value", rs.Raw)
	}
}

func TestGetRun_NotFound(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, `{"job_path":"foo"}`)
	_, err := a.GetRun(context.Background(), "999")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetRun — queue path
// ---------------------------------------------------------------------------

func TestGetRun_QueuePending(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/queue/item/42/api/json") {
			_ = json.NewEncoder(w).Encode(queueItem{Why: "Waiting for next executor"})
			return
		}
		t.Errorf("unexpected path %q", r.URL.Path)
	}, `{"job_path":"foo"}`)

	rs, err := a.GetRun(context.Background(), "queue:42")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.Phase != engine.RunPhasePending {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if rs.Message != "Waiting for next executor" {
		t.Fatalf("Message = %q", rs.Message)
	}
}

func TestGetRun_QueueCancelled(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(queueItem{Cancelled: true, Why: "canceled by user"})
	}, `{"job_path":"foo"}`)
	rs, err := a.GetRun(context.Background(), "queue:42")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.Phase != engine.RunPhaseCancelled {
		t.Fatalf("Phase = %q", rs.Phase)
	}
	if rs.Raw != "queue-cancelled" {
		t.Fatalf("Raw = %q", rs.Raw)
	}
}

func TestGetRun_QueueResolvesToBuild(t *testing.T) {
	// Queue item has since been scheduled; GetRun should auto-follow.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/queue/item/42/api/json"):
			_ = json.NewEncoder(w).Encode(queueItem{
				Executable: &queueExecutable{Number: 77},
			})
		case strings.HasSuffix(r.URL.Path, "/job/foo/77/api/json"):
			_ = json.NewEncoder(w).Encode(jenkinsBuild{Number: 77, Result: "SUCCESS"})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}, `{"job_path":"foo"}`)

	rs, err := a.GetRun(context.Background(), "queue:42")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rs.RunID != "77" {
		t.Fatalf("should upgrade RunID to build number, got %q", rs.RunID)
	}
	if rs.Phase != engine.RunPhaseSuccess {
		t.Fatalf("Phase = %q", rs.Phase)
	}
}
