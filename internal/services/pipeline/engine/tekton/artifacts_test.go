package tekton

import (
	"context"
	"errors"
	"testing"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestGetArtifacts_EmptyRunID(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"namespace":"ci"}`)
	if _, err := a.GetArtifacts(context.Background(), ""); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetArtifacts_MissingNamespace(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), "")
	if _, err := a.GetArtifacts(context.Background(), "pr-1"); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetArtifacts_NilResolver(t *testing.T) {
	a := newAdapter(t, nil, `{"namespace":"ci"}`)
	if _, err := a.GetArtifacts(context.Background(), "pr-1"); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestGetArtifacts_NotFound(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"namespace":"ci"}`)
	if _, err := a.GetArtifacts(context.Background(), "missing"); !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetArtifacts_NoResults_EmptySlice(t *testing.T) {
	pr := newPipelineRun("pr-1", "ci", map[string]any{
		"conditions": []any{
			map[string]any{"type": "Succeeded", "status": "True"},
		},
	})
	a := newAdapter(t, newDynamicResolver(t, pr), `{"namespace":"ci"}`)
	arts, err := a.GetArtifacts(context.Background(), "pr-1")
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

func TestGetArtifacts_PipelineResults(t *testing.T) {
	pr := newPipelineRun("pr-1", "ci", map[string]any{
		"completionTime": "2026-04-16T12:05:00Z",
		"pipelineResults": []any{
			map[string]any{"name": "image-digest", "value": "sha256:abc123"},
			map[string]any{"name": "git-sha", "value": "deadbeef"},
		},
	})
	a := newAdapter(t, newDynamicResolver(t, pr), `{"namespace":"ci"}`)
	arts, err := a.GetArtifacts(context.Background(), "pr-1")
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(arts))
	}
	names := map[string]string{}
	for _, a := range arts {
		names[a.Name] = a.Digest
		if a.Kind != "result" {
			t.Fatalf("Kind = %q", a.Kind)
		}
	}
	if names["image-digest"] != "sha256:abc123" || names["git-sha"] != "deadbeef" {
		t.Fatalf("values lost: %+v", names)
	}
}

func TestGetArtifacts_ResultsKeyAlias(t *testing.T) {
	// Some Tekton revisions renamed `pipelineResults` to just `results`;
	// the adapter accepts both for forward-compat.
	pr := newPipelineRun("pr-1", "ci", map[string]any{
		"results": []any{
			map[string]any{"name": "x", "value": "1"},
		},
	})
	a := newAdapter(t, newDynamicResolver(t, pr), `{"namespace":"ci"}`)
	arts, err := a.GetArtifacts(context.Background(), "pr-1")
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if len(arts) != 1 || arts[0].Name != "x" {
		t.Fatalf("got %+v", arts)
	}
}

func TestGetArtifacts_SkipsMalformedEntries(t *testing.T) {
	// Entries without a name field are dropped.
	pr := newPipelineRun("pr-1", "ci", map[string]any{
		"pipelineResults": []any{
			map[string]any{"value": "no-name"},
			map[string]any{"name": "ok", "value": "1"},
			"not-an-object",
		},
	})
	a := newAdapter(t, newDynamicResolver(t, pr), `{"namespace":"ci"}`)
	arts, err := a.GetArtifacts(context.Background(), "pr-1")
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if len(arts) != 1 || arts[0].Name != "ok" {
		t.Fatalf("got %+v", arts)
	}
}

func TestExtractPipelineResults_NilStatus(t *testing.T) {
	if got := extractPipelineResults(map[string]any{}); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
