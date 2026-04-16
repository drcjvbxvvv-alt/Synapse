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

func TestCancel_InvalidRunID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"owner":"o","repo":"r"}`)
	if err := a.Cancel(context.Background(), ""); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestCancel_PlaceholderRunID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"owner":"o","repo":"r"}`)
	err := a.Cancel(context.Background(), "dispatch:main@1700000000")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestCancel_MissingOwnerRepo(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	if err := a.Cancel(context.Background(), "42"); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestCancel_Success(t *testing.T) {
	var seenPath string
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusAccepted)
	}, `{"owner":"o","repo":"r"}`)
	if err := a.Cancel(context.Background(), "42"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !strings.HasSuffix(seenPath, "/repos/o/r/actions/runs/42/cancel") {
		t.Fatalf("path = %q", seenPath)
	}
}

func TestCancel_Conflict_MapsToAlreadyTerminal(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"message":"Cannot cancel a run that has already completed."}`)
	}, `{"owner":"o","repo":"r"}`)
	err := a.Cancel(context.Background(), "42")
	if !errors.Is(err, engine.ErrAlreadyTerminal) {
		t.Fatalf("got %v", err)
	}
}

func TestCancel_NotFound(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, `{"owner":"o","repo":"r"}`)
	if err := a.Cancel(context.Background(), "42"); !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestContainsStatusMarker(t *testing.T) {
	// Positive matches
	if !containsStatusMarker("github returned 409: Cannot cancel", "409") {
		t.Fatal("should match")
	}
	// Negative: 409 appears in body but not after "returned"
	if containsStatusMarker("body contained 409 somewhere but http was 500", "409") {
		t.Fatal("should not match without 'returned' prefix")
	}
}

func TestIndexOf(t *testing.T) {
	if indexOf("hello world", "world") != 6 {
		t.Fatal("substring not found correctly")
	}
	if indexOf("hello", "") != 0 {
		t.Fatal("empty substr should match start")
	}
	if indexOf("abc", "xyz") != -1 {
		t.Fatal("missing substr should return -1")
	}
}
