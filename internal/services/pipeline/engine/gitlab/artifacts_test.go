package gitlab

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
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"project_id":42}`)
	_, err := a.GetArtifacts(context.Background(), "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetArtifacts_MissingProjectID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	_, err := a.GetArtifacts(context.Background(), "77")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetArtifacts_NonNumericRunID(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"project_id":42}`)
	_, err := a.GetArtifacts(context.Background(), "abc")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetArtifacts_FiltersJobsWithoutArtifacts(t *testing.T) {
	finished := time.Date(2026, 4, 16, 12, 5, 0, 0, time.UTC)
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/v4/projects/42/pipelines/77/jobs") {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]gitlabJob{
			{ID: 1, Name: "build", Status: "success", FinishedAt: &finished,
				ArtifactsFile: &gitlabArtifactMeta{Filename: "app.jar", Size: 1024}},
			{ID: 2, Name: "test", Status: "success", FinishedAt: &finished}, // no artifacts
			{ID: 3, Name: "report", Status: "success", FinishedAt: &finished,
				ArtifactsFile: &gitlabArtifactMeta{Filename: "report.xml", Size: 256}},
		})
	}, `{"project_id":42}`)

	arts, err := a.GetArtifacts(context.Background(), "77")
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if len(arts) != 2 {
		t.Fatalf("len = %d, want 2 (test job should be filtered)", len(arts))
	}
	if arts[0].Name != "app.jar" || arts[0].SizeBytes != 1024 {
		t.Fatalf("unexpected artifact[0]: %+v", arts[0])
	}
	if arts[1].Name != "report.xml" || arts[1].SizeBytes != 256 {
		t.Fatalf("unexpected artifact[1]: %+v", arts[1])
	}
	for i, a := range arts {
		if a.Kind != "file" {
			t.Fatalf("artifacts[%d].Kind = %q", i, a.Kind)
		}
		if a.URL == "" {
			t.Fatalf("artifacts[%d].URL should not be empty", i)
		}
	}
}

func TestGetArtifacts_EmptyListWhenNoArtifacts(t *testing.T) {
	// Contract requires empty slice (not nil, not error) when nothing to
	// return — callers write `if len(arts) == 0`.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]gitlabJob{{ID: 1, Name: "build", Status: "success"}})
	}, `{"project_id":42}`)

	arts, err := a.GetArtifacts(context.Background(), "77")
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

func TestGetArtifacts_JobListFailure(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}, `{"project_id":42}`)

	_, err := a.GetArtifacts(context.Background(), "77")
	if !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestArtifactJobURL(t *testing.T) {
	got := artifactJobURL("https", "gitlab.example.com", 42, 77)
	if got != "https://gitlab.example.com/-/jobs/77" {
		t.Fatalf("URL = %q", got)
	}
}
