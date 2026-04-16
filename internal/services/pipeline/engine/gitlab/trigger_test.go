package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// convertVariables
// ---------------------------------------------------------------------------

func TestConvertVariables_Empty(t *testing.T) {
	if got := convertVariables(nil); got != nil {
		t.Fatalf("nil map → %v, want nil", got)
	}
	if got := convertVariables(map[string]string{}); got != nil {
		t.Fatalf("empty map → %v, want nil", got)
	}
}

func TestConvertVariables_PassThrough(t *testing.T) {
	got := convertVariables(map[string]string{"ENV": "staging", "DEBUG": "1"})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	seen := map[string]string{}
	for _, v := range got {
		seen[v.Key] = v.Value
	}
	if seen["ENV"] != "staging" || seen["DEBUG"] != "1" {
		t.Fatalf("variables not propagated: %+v", seen)
	}
}

// ---------------------------------------------------------------------------
// Trigger — validation
// ---------------------------------------------------------------------------

func TestTrigger_NilRequest(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"project_id":42,"default_ref":"main"}`)
	_, err := a.Trigger(context.Background(), nil)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTrigger_MissingProjectID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "") // no ExtraJSON
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{Ref: "main"})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput when project_id missing, got %v", err)
	}
}

func TestTrigger_MissingRef(t *testing.T) {
	// project_id set, but TriggerRequest.Ref empty AND default_ref empty.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"project_id":42}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput when ref missing, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Trigger — happy path
// ---------------------------------------------------------------------------

func TestTrigger_Success_UsesRequestRef(t *testing.T) {
	// Capture body for assertions.
	var seenBody triggerRequest
	seenPath := ""

	created := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)

	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(gitlabPipeline{
			ID:        77,
			ProjectID: 42,
			Status:    "pending",
			Ref:       seenBody.Ref,
			SHA:       "abc123",
			WebURL:    "https://gitlab.example.com/foo/bar/pipelines/77",
			CreatedAt: &created,
		})
	}, `{"project_id":42,"default_ref":"main"}`)

	res, err := a.Trigger(context.Background(), &engine.TriggerRequest{
		PipelineID:      1,
		SnapshotID:      1,
		Ref:             "feature/x",
		Variables:       map[string]string{"ENV": "staging"},
		TriggerType:     "manual",
		TriggeredByUser: 42,
	})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}

	if !strings.HasSuffix(seenPath, "/api/v4/projects/42/pipeline") {
		t.Fatalf("path = %q", seenPath)
	}
	if seenBody.Ref != "feature/x" {
		t.Fatalf("ref sent to server = %q, want feature/x (TriggerRequest.Ref wins over default_ref)", seenBody.Ref)
	}
	if len(seenBody.Variables) != 1 || seenBody.Variables[0].Key != "ENV" || seenBody.Variables[0].Value != "staging" {
		t.Fatalf("variables not forwarded: %+v", seenBody.Variables)
	}
	if res.ExternalID != "77" {
		t.Fatalf("ExternalID = %q, want 77", res.ExternalID)
	}
	if res.URL == "" {
		t.Fatalf("URL should be populated from web_url")
	}
	if !res.QueuedAt.Equal(created) {
		t.Fatalf("QueuedAt = %v, want %v", res.QueuedAt, created)
	}
}

func TestTrigger_Success_FallsBackToDefaultRef(t *testing.T) {
	var seen triggerRequest
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&seen)
		_ = json.NewEncoder(w).Encode(gitlabPipeline{ID: 1, Status: "pending"})
	}, `{"project_id":42,"default_ref":"main"}`)

	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{}) // no Ref
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if seen.Ref != "main" {
		t.Fatalf("ref = %q, want main (fallback to default_ref)", seen.Ref)
	}
}

func TestTrigger_QueuedAt_DefaultsToNowWhenMissing(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(gitlabPipeline{
			ID:     1,
			Status: "pending",
			// CreatedAt is nil → Trigger should default to time.Now()
		})
	}, `{"project_id":42,"default_ref":"main"}`)

	before := time.Now().UTC()
	res, err := a.Trigger(context.Background(), &engine.TriggerRequest{Ref: "main"})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if res.QueuedAt.Before(before) {
		t.Fatalf("QueuedAt should not be earlier than test start: %v < %v", res.QueuedAt, before)
	}
}

// ---------------------------------------------------------------------------
// Trigger — server errors
// ---------------------------------------------------------------------------

func TestTrigger_422_MapsToInvalidInput(t *testing.T) {
	// GitLab returns 422 when the ref doesn't exist.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"message":"Reference not found"}`)
	}, `{"project_id":42,"default_ref":"main"}`)

	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{Ref: "nonexistent"})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTrigger_Unauthorized(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}, `{"project_id":42,"default_ref":"main"}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{Ref: "main"})
	if !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}
