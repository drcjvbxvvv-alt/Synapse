package jenkins

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestStreamLogs_EmptyRunID(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	_, err := a.StreamLogs(context.Background(), "", "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStreamLogs_QueueRunID_Rejected(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	_, err := a.StreamLogs(context.Background(), "queue:42", "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStreamLogs_MissingJobPath(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	_, err := a.StreamLogs(context.Background(), "42", "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStreamLogs_NonNumeric(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	_, err := a.StreamLogs(context.Background(), "abc", "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStreamLogs_Success(t *testing.T) {
	var seenPath, seenAccept string
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path + "?" + r.URL.RawQuery
		seenAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "step one\nstep two\n")
	}, `{"job_path":"foo"}`)

	rc, err := a.StreamLogs(context.Background(), "42", "")
	if err != nil {
		t.Fatalf("StreamLogs: %v", err)
	}
	defer func() { _ = rc.Close() }()
	buf, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(buf) != "step one\nstep two\n" {
		t.Fatalf("body = %q", buf)
	}
	if !strings.Contains(seenPath, "/job/foo/42/logText/progressiveText?start=0") {
		t.Fatalf("path = %q", seenPath)
	}
	if seenAccept != "text/plain" {
		t.Fatalf("Accept = %q", seenAccept)
	}
}

func TestStreamLogs_NotFound(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, `{"job_path":"foo"}`)
	_, err := a.StreamLogs(context.Background(), "42", "")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStreamLogs_StepIDIgnored(t *testing.T) {
	// Non-empty stepID should not change behaviour (M18c-level log).
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}, `{"job_path":"foo"}`)
	rc, err := a.StreamLogs(context.Background(), "42", "some-stage")
	if err != nil {
		t.Fatalf("StreamLogs: %v", err)
	}
	_, _ = io.Copy(io.Discard, rc)
	_ = rc.Close()
}
