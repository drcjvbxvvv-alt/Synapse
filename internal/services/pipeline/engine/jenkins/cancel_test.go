package jenkins

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestCancel_EmptyRunID(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	err := a.Cancel(context.Background(), "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCancel_MissingJobPath(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	err := a.Cancel(context.Background(), "42")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCancel_NonNumericRunID(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	err := a.Cancel(context.Background(), "abc")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCancel_QueueItem_Success(t *testing.T) {
	var seenPath string
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path + "?" + r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}, `{"job_path":"foo"}`)
	if err := a.Cancel(context.Background(), "queue:55"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !strings.Contains(seenPath, "/queue/cancelItem?id=55") {
		t.Fatalf("path = %q", seenPath)
	}
}

func TestCancel_InvalidQueueID(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	err := a.Cancel(context.Background(), "queue:not-a-number")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCancel_BuildStillRunning_CallsStop(t *testing.T) {
	var stopCalled atomic.Bool
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/job/foo/100/api/json"):
			_ = json.NewEncoder(w).Encode(jenkinsBuild{Number: 100, Building: true})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/job/foo/100/stop"):
			stopCalled.Store(true)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}, `{"job_path":"foo"}`)

	if err := a.Cancel(context.Background(), "100"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !stopCalled.Load() {
		t.Fatal("stop endpoint not called")
	}
}

func TestCancel_BuildAlreadyTerminal(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(jenkinsBuild{
				Number: 100, Result: "SUCCESS", Building: false,
			})
			return
		}
		t.Errorf("stop endpoint should NOT be called for terminal build, got %s %s", r.Method, r.URL.Path)
	}, `{"job_path":"foo"}`)

	err := a.Cancel(context.Background(), "100")
	if !errors.Is(err, engine.ErrAlreadyTerminal) {
		t.Fatalf("expected ErrAlreadyTerminal, got %v", err)
	}
}

func TestCancel_BuildNotFound(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, `{"job_path":"foo"}`)
	err := a.Cancel(context.Background(), "999")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
