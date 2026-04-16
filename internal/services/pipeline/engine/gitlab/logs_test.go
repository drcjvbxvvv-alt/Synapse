package gitlab

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestStreamLogs_EmptyStepID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"project_id":42}`)
	_, err := a.StreamLogs(context.Background(), "77", "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty stepID, got %v", err)
	}
}

func TestStreamLogs_MissingProjectID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	_, err := a.StreamLogs(context.Background(), "77", "1")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStreamLogs_NonNumericStepID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"project_id":42}`)
	_, err := a.StreamLogs(context.Background(), "77", "abc")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for non-numeric stepID, got %v", err)
	}
}

func TestStreamLogs_Success(t *testing.T) {
	seenPath := ""
	seenAccept := ""
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "line1\nline2\n")
	}, `{"project_id":42}`)

	rc, err := a.StreamLogs(context.Background(), "77", "5")
	if err != nil {
		t.Fatalf("StreamLogs: %v", err)
	}
	defer func() { _ = rc.Close() }()

	buf, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(buf) != "line1\nline2\n" {
		t.Fatalf("body = %q", buf)
	}
	if !strings.HasSuffix(seenPath, "/api/v4/projects/42/jobs/5/trace") {
		t.Fatalf("path = %q", seenPath)
	}
	if seenAccept != "text/plain" {
		t.Fatalf("Accept = %q, want text/plain", seenAccept)
	}
}

func TestStreamLogs_NotFound(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, `{"project_id":42}`)

	_, err := a.StreamLogs(context.Background(), "77", "5")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStreamLogs_Unauthorized(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}, `{"project_id":42}`)

	_, err := a.StreamLogs(context.Background(), "77", "5")
	if !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}
