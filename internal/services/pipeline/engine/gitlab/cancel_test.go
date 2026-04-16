package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestCancel_EmptyRunID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"project_id":42}`)
	err := a.Cancel(context.Background(), "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCancel_MissingProjectID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	err := a.Cancel(context.Background(), "1")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCancel_NonNumericRunID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"project_id":42}`)
	err := a.Cancel(context.Background(), "abc")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCancel_Success_ReturnsCanceled(t *testing.T) {
	seenPath := ""
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(gitlabPipeline{ID: 77, Status: "canceled"})
	}, `{"project_id":42}`)

	if err := a.Cancel(context.Background(), "77"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !strings.HasSuffix(seenPath, "/api/v4/projects/42/pipelines/77/cancel") {
		t.Fatalf("path = %q", seenPath)
	}
}

func TestCancel_AlreadyTerminal_Success(t *testing.T) {
	// GitLab may report success/failed after the cancel request if the
	// pipeline actually finished before cancellation could take effect.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(gitlabPipeline{ID: 77, Status: "success"})
	}, `{"project_id":42}`)

	err := a.Cancel(context.Background(), "77")
	if !errors.Is(err, engine.ErrAlreadyTerminal) {
		t.Fatalf("expected ErrAlreadyTerminal, got %v", err)
	}
}

func TestCancel_AlreadyTerminal_Failed(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(gitlabPipeline{ID: 77, Status: "failed"})
	}, `{"project_id":42}`)

	err := a.Cancel(context.Background(), "77")
	if !errors.Is(err, engine.ErrAlreadyTerminal) {
		t.Fatalf("expected ErrAlreadyTerminal, got %v", err)
	}
}

func TestCancel_GitLab400_MapsToAlreadyTerminal(t *testing.T) {
	// Some GitLab versions return 400 "Pipeline cannot be canceled"
	// instead of updating the status. Adapter should not surface this as
	// ErrInvalidInput but rather ErrAlreadyTerminal.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"message":"Pipeline cannot be canceled"}`)
	}, `{"project_id":42}`)

	err := a.Cancel(context.Background(), "77")
	if !errors.Is(err, engine.ErrAlreadyTerminal) {
		t.Fatalf("expected ErrAlreadyTerminal, got %v", err)
	}
}

func TestCancel_NotFound(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, `{"project_id":42}`)

	err := a.Cancel(context.Background(), "77")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
