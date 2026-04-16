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

func TestGetArtifacts_EmptyRunID(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	_, err := a.GetArtifacts(context.Background(), "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetArtifacts_QueueRunID_ReturnsEmpty(t *testing.T) {
	// A queued build has no artifacts; the adapter returns empty slice
	// (not error) so the UI can render a stable empty state.
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("should not hit backend for queue runID")
	}, `{"job_path":"foo"}`)
	arts, err := a.GetArtifacts(context.Background(), "queue:42")
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if len(arts) != 0 || arts == nil {
		t.Fatalf("expected non-nil empty slice, got %+v", arts)
	}
}

func TestGetArtifacts_MissingJobPath(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	_, err := a.GetArtifacts(context.Background(), "42")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetArtifacts_NonNumeric(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	_, err := a.GetArtifacts(context.Background(), "abc")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetArtifacts_InlinesMetadata(t *testing.T) {
	startMs := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC).UnixMilli()
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/job/foo/42/api/json") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(jenkinsBuild{
			Number:    42,
			Result:    "SUCCESS",
			Timestamp: startMs,
			Duration:  1000,
			Artifacts: []jenkinsBuildArtifact{
				{FileName: "app.jar", RelativePath: "build/libs/app.jar", DisplayPath: "app.jar"},
				{FileName: "report.xml", RelativePath: "build/reports/report.xml"},
			},
		})
	}, `{"job_path":"foo"}`)

	arts, err := a.GetArtifacts(context.Background(), "42")
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(arts))
	}
	// URL points at the download route and includes relativePath.
	if !strings.HasSuffix(arts[0].URL, "/job/foo/42/artifact/build/libs/app.jar") {
		t.Fatalf("artifact[0].URL wrong: %q", arts[0].URL)
	}
	if arts[0].Name != "app.jar" {
		t.Fatalf("artifact[0].Name wrong: %q", arts[0].Name)
	}
	if arts[0].Kind != "file" {
		t.Fatalf("Kind = %q", arts[0].Kind)
	}
	// CreatedAt should be the build Timestamp.
	if arts[0].CreatedAt.UnixMilli() != startMs {
		t.Fatalf("CreatedAt not build timestamp")
	}
}

func TestGetArtifacts_NoArtifacts_ReturnsEmptySlice(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jenkinsBuild{Number: 42, Result: "SUCCESS"})
	}, `{"job_path":"foo"}`)
	arts, err := a.GetArtifacts(context.Background(), "42")
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if arts == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(arts) != 0 {
		t.Fatalf("expected empty, got %+v", arts)
	}
}

func TestGetArtifacts_NotFound(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, `{"job_path":"foo"}`)
	_, err := a.GetArtifacts(context.Background(), "42")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
