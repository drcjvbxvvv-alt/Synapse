package github

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
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"owner":"o","repo":"r"}`)
	_, err := a.StreamLogs(context.Background(), "42", "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestStreamLogs_NonNumericStepID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"owner":"o","repo":"r"}`)
	_, err := a.StreamLogs(context.Background(), "42", "abc")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestStreamLogs_MissingOwnerRepo(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	_, err := a.StreamLogs(context.Background(), "42", "7")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestStreamLogs_Success(t *testing.T) {
	var seenPath string
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "line 1\nline 2\n")
	}, `{"owner":"o","repo":"r"}`)

	rc, err := a.StreamLogs(context.Background(), "42", "7")
	if err != nil {
		t.Fatalf("StreamLogs: %v", err)
	}
	defer func() { _ = rc.Close() }()
	buf, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(buf) != "line 1\nline 2\n" {
		t.Fatalf("body = %q", buf)
	}
	if !strings.HasSuffix(seenPath, "/repos/o/r/actions/jobs/7/logs") {
		t.Fatalf("path = %q", seenPath)
	}
}

func TestStreamLogs_NotFound(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, `{"owner":"o","repo":"r"}`)
	_, err := a.StreamLogs(context.Background(), "42", "7")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
