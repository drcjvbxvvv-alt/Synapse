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
// Trigger — validation
// ---------------------------------------------------------------------------

func TestTrigger_NilRequest(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"owner":"o","repo":"r","workflow_id":"w"}`)
	if _, err := a.Trigger(context.Background(), nil); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestTrigger_MissingTargets(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	if _, err := a.Trigger(context.Background(), &engine.TriggerRequest{}); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestTrigger_MissingRef(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"owner":"o","repo":"r","workflow_id":"w"}`)
	if _, err := a.Trigger(context.Background(), &engine.TriggerRequest{}); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Trigger — happy paths
// ---------------------------------------------------------------------------

func TestTrigger_Success_ImmediateRunDiscovery(t *testing.T) {
	var dispatchBody dispatchRequest
	seenDispatch := false
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/dispatches"):
			seenDispatch = true
			if err := json.NewDecoder(r.Body).Decode(&dispatchBody); err != nil {
				t.Errorf("decode: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/workflows/"):
			// Return a newly-created workflow_dispatch run.
			now := time.Now().UTC()
			_ = json.NewEncoder(w).Encode(workflowRunList{
				TotalCount: 1,
				WorkflowRuns: []workflowRun{{
					ID:        987654321,
					Event:     "workflow_dispatch",
					Status:    "queued",
					CreatedAt: &now,
					HTMLURL:   "https://github.com/o/r/actions/runs/987654321",
				}},
			})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}, `{"owner":"o","repo":"r","workflow_id":"build.yml","default_ref":"main"}`)

	res, err := a.Trigger(context.Background(), &engine.TriggerRequest{
		Variables: map[string]string{"ENV": "staging"},
	})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if !seenDispatch {
		t.Fatal("dispatch endpoint not called")
	}
	if dispatchBody.Ref != "main" {
		t.Fatalf("ref = %q (should fall back to default_ref)", dispatchBody.Ref)
	}
	if dispatchBody.Inputs["ENV"] != "staging" {
		t.Fatalf("inputs not forwarded: %+v", dispatchBody.Inputs)
	}
	if res.RunID != "987654321" {
		t.Fatalf("RunID = %q", res.RunID)
	}
	if res.URL != "https://github.com/o/r/actions/runs/987654321" {
		t.Fatalf("URL = %q", res.URL)
	}
}

func TestTrigger_Success_OverrideRef(t *testing.T) {
	var body dispatchRequest
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		now := time.Now().UTC()
		_ = json.NewEncoder(w).Encode(workflowRunList{
			WorkflowRuns: []workflowRun{{ID: 1, Event: "workflow_dispatch", CreatedAt: &now}},
		})
	}, `{"owner":"o","repo":"r","workflow_id":"w","default_ref":"main"}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{Ref: "feature/x"})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if body.Ref != "feature/x" {
		t.Fatalf("ref = %q (TriggerRequest.Ref should win over default_ref)", body.Ref)
	}
}

func TestTrigger_TimeoutFallsBackToPlaceholder(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// GET /runs: always return empty (no matching run yet).
		_ = json.NewEncoder(w).Encode(workflowRunList{})
	}, `{"owner":"o","repo":"r","workflow_id":"w"}`)

	// Shorten the outer ctx so waiter exits quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	res, err := a.Trigger(ctx, &engine.TriggerRequest{Ref: "main"})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if !strings.HasPrefix(res.RunID, dispatchPrefix) {
		t.Fatalf("expected placeholder, got %q", res.RunID)
	}
}

func TestTrigger_422_InvalidInput(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
	}, `{"owner":"o","repo":"r","workflow_id":"w"}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{Ref: "main"})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestTrigger_Unauthorized(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}, `{"owner":"o","repo":"r","workflow_id":"w"}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{Ref: "main"})
	if !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("got %v", err)
	}
}

// ---------------------------------------------------------------------------
// parseRunID
// ---------------------------------------------------------------------------

func TestParseRunID_Numeric(t *testing.T) {
	id, ref, _, err := parseRunID("987")
	if err != nil || id != 987 || ref != "" {
		t.Fatalf("got id=%d ref=%q err=%v", id, ref, err)
	}
}

func TestParseRunID_Placeholder(t *testing.T) {
	id, ref, cutoff, err := parseRunID("dispatch:main@1700000000")
	if err != nil || id != 0 || ref != "main" {
		t.Fatalf("got id=%d ref=%q err=%v", id, ref, err)
	}
	if cutoff.Unix() != 1700000000 {
		t.Fatalf("cutoff epoch = %d", cutoff.Unix())
	}
}

func TestParseRunID_Invalid(t *testing.T) {
	cases := []string{"", "abc", "dispatch:nothing", "dispatch:x@not-a-number"}
	for _, in := range cases {
		if _, _, _, err := parseRunID(in); !errors.Is(err, engine.ErrInvalidInput) {
			t.Fatalf("%q: got %v", in, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Status mapping
// ---------------------------------------------------------------------------

func TestMapGitHubStatus_AllValues(t *testing.T) {
	cases := []struct {
		status, conclusion string
		want               engine.RunPhase
	}{
		{"queued", "", engine.RunPhasePending},
		{"requested", "", engine.RunPhasePending},
		{"waiting", "", engine.RunPhasePending},
		{"pending", "", engine.RunPhasePending},
		{"in_progress", "", engine.RunPhaseRunning},
		{"completed", "success", engine.RunPhaseSuccess},
		{"completed", "failure", engine.RunPhaseFailed},
		{"completed", "timed_out", engine.RunPhaseFailed},
		{"completed", "startup_failure", engine.RunPhaseFailed},
		{"completed", "cancelled", engine.RunPhaseCancelled},
		{"completed", "skipped", engine.RunPhaseCancelled},
		{"completed", "action_required", engine.RunPhasePending},
		{"completed", "neutral", engine.RunPhaseUnknown},
		{"completed", "stale", engine.RunPhaseUnknown},
		{"completed", "", engine.RunPhaseUnknown},
		{"", "", engine.RunPhasePending},
		{"bogus", "", engine.RunPhaseUnknown},
	}
	for _, tc := range cases {
		if got := mapGitHubStatus(tc.status, tc.conclusion); got != tc.want {
			t.Fatalf("mapGitHubStatus(%q, %q) = %q, want %q", tc.status, tc.conclusion, got, tc.want)
		}
	}
}
