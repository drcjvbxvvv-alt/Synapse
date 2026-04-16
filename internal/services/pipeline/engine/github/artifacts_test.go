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

func TestGetArtifacts_InvalidRunID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"owner":"o","repo":"r"}`)
	if _, err := a.GetArtifacts(context.Background(), ""); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestGetArtifacts_Placeholder_EmptySlice(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("should not hit backend for placeholder")
	}, `{"owner":"o","repo":"r"}`)
	arts, err := a.GetArtifacts(context.Background(), "dispatch:main@1700000000")
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if arts == nil || len(arts) != 0 {
		t.Fatalf("expected non-nil empty slice, got %+v", arts)
	}
}

func TestGetArtifacts_MissingOwnerRepo(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	if _, err := a.GetArtifacts(context.Background(), "42"); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestGetArtifacts_NotFound(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, `{"owner":"o","repo":"r"}`)
	if _, err := a.GetArtifacts(context.Background(), "42"); !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestGetArtifacts_Success(t *testing.T) {
	created := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/repos/o/r/actions/runs/42/artifacts") {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(artifactsList{
			TotalCount: 2,
			Artifacts: []artifactEntry{
				{ID: 100, Name: "app.jar", SizeInBytes: 1024, ArchiveDownloadURL: "https://api.example.com/zip/100", CreatedAt: &created},
				{ID: 101, Name: "test-report", SizeInBytes: 512, ArchiveDownloadURL: "https://api.example.com/zip/101", CreatedAt: &created},
			},
		})
	}, `{"owner":"o","repo":"r"}`)

	arts, err := a.GetArtifacts(context.Background(), "42")
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2, got %d", len(arts))
	}
	for _, a := range arts {
		if a.Kind != "file" {
			t.Fatalf("Kind = %q", a.Kind)
		}
	}
	if arts[0].Name != "app.jar" || arts[0].SizeBytes != 1024 || arts[0].Digest != "100" {
		t.Fatalf("artifact[0] = %+v", arts[0])
	}
	if arts[0].URL != "https://api.example.com/zip/100" {
		t.Fatalf("URL = %q", arts[0].URL)
	}
}

func TestGetArtifacts_Empty(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(artifactsList{})
	}, `{"owner":"o","repo":"r"}`)
	arts, err := a.GetArtifacts(context.Background(), "42")
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if arts == nil {
		t.Fatal("expected non-nil")
	}
	if len(arts) != 0 {
		t.Fatalf("got %+v", arts)
	}
}
