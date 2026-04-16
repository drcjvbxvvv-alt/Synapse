package argo

import (
	"context"
	"errors"
	"testing"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestGetArtifacts_EmptyRunID(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), `{"namespace":"ci"}`)
	if _, err := a.GetArtifacts(context.Background(), ""); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestGetArtifacts_MissingNamespace(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), "")
	if _, err := a.GetArtifacts(context.Background(), "wf-1"); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestGetArtifacts_NilResolver(t *testing.T) {
	a := newAdapter(t, nil, `{"namespace":"ci"}`)
	if _, err := a.GetArtifacts(context.Background(), "wf-1"); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestGetArtifacts_NotFound(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), `{"namespace":"ci"}`)
	if _, err := a.GetArtifacts(context.Background(), "missing"); !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestGetArtifacts_NoArtifacts_EmptySlice(t *testing.T) {
	wf := newWorkflow("wf-1", "ci", map[string]any{"phase": "Succeeded"})
	a := newAdapter(t, newResolverArgoInstalled(t, wf), `{"namespace":"ci"}`)
	arts, err := a.GetArtifacts(context.Background(), "wf-1")
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if arts == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(arts) != 0 {
		t.Fatalf("got %+v", arts)
	}
}

func TestGetArtifacts_FlattensNodeOutputs(t *testing.T) {
	wf := newWorkflow("wf-1", "ci", map[string]any{
		"phase":      "Succeeded",
		"finishedAt": "2026-04-16T12:05:00Z",
		"nodes": map[string]any{
			"wf-1-build": map[string]any{
				"displayName": "build",
				"phase":       "Succeeded",
				"outputs": map[string]any{
					"artifacts": []any{
						map[string]any{
							"name": "app-jar",
							"s3":   map[string]any{"key": "artifacts/build/app.jar"},
						},
					},
				},
			},
			"wf-1-report": map[string]any{
				"displayName": "report",
				"phase":       "Succeeded",
				"outputs": map[string]any{
					"artifacts": []any{
						map[string]any{
							"name": "coverage-report",
							"http": map[string]any{"url": "https://example.com/report.xml"},
						},
					},
				},
			},
		},
	})
	a := newAdapter(t, newResolverArgoInstalled(t, wf), `{"namespace":"ci"}`)
	arts, err := a.GetArtifacts(context.Background(), "wf-1")
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(arts))
	}
	byName := map[string]string{}
	for _, a := range arts {
		byName[a.Name] = a.URL
		if a.Kind != "file" {
			t.Fatalf("Kind = %q", a.Kind)
		}
	}
	if byName["app-jar"] != "s3://artifacts/build/app.jar" {
		t.Fatalf("s3 url wrong: %q", byName["app-jar"])
	}
	if byName["coverage-report"] != "https://example.com/report.xml" {
		t.Fatalf("http url wrong: %q", byName["coverage-report"])
	}
}

func TestGetArtifacts_SkipsMalformed(t *testing.T) {
	wf := newWorkflow("wf-1", "ci", map[string]any{
		"nodes": map[string]any{
			"n1": map[string]any{
				"outputs": map[string]any{
					"artifacts": []any{
						map[string]any{}, // no name → dropped
						"not-a-map",       // wrong type → dropped
						map[string]any{"name": "ok"},
					},
				},
			},
		},
	})
	a := newAdapter(t, newResolverArgoInstalled(t, wf), `{"namespace":"ci"}`)
	arts, err := a.GetArtifacts(context.Background(), "wf-1")
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if len(arts) != 1 || arts[0].Name != "ok" {
		t.Fatalf("got %+v", arts)
	}
}

// ---------------------------------------------------------------------------
// extractArtifactURL branch coverage
// ---------------------------------------------------------------------------

func TestExtractArtifactURL_Priorities(t *testing.T) {
	cases := []struct {
		art  map[string]any
		want string
	}{
		{map[string]any{"http": map[string]any{"url": "https://h"}, "s3": map[string]any{"key": "x"}}, "https://h"}, // http wins
		{map[string]any{"s3": map[string]any{"key": "bucket/key"}}, "s3://bucket/key"},
		{map[string]any{"gcs": map[string]any{"key": "bucket/key"}}, "gcs://bucket/key"},
		{map[string]any{"oss": map[string]any{"key": "bucket/key"}}, "oss://bucket/key"},
		{map[string]any{"azure": map[string]any{"key": "bucket/key"}}, "azure://bucket/key"},
		{map[string]any{"raw": map[string]any{"data": "hello"}}, "raw://"},
		{map[string]any{}, ""},
	}
	for _, tc := range cases {
		if got := extractArtifactURL(tc.art); got != tc.want {
			t.Fatalf("artifact %+v → %q, want %q", tc.art, got, tc.want)
		}
	}
}
