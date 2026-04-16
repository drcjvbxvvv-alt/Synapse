package tekton

import (
	"errors"
	"testing"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestParseExtra_Empty(t *testing.T) {
	cfg, err := parseExtra("")
	if err != nil {
		t.Fatalf("parseExtra: %v", err)
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}
	if cfg.PipelineName != "" || cfg.Namespace != "" {
		t.Fatalf("expected zero ExtraConfig, got %+v", cfg)
	}
}

func TestParseExtra_Valid(t *testing.T) {
	cfg, err := parseExtra(`{"pipeline_name":"build-x","namespace":"ci","service_account_name":"runner"}`)
	if err != nil {
		t.Fatalf("parseExtra: %v", err)
	}
	if cfg.PipelineName != "build-x" {
		t.Fatalf("PipelineName = %q", cfg.PipelineName)
	}
	if cfg.Namespace != "ci" {
		t.Fatalf("Namespace = %q", cfg.Namespace)
	}
	if cfg.ServiceAccountName != "runner" {
		t.Fatalf("ServiceAccountName = %q", cfg.ServiceAccountName)
	}
}

func TestParseExtra_Malformed(t *testing.T) {
	_, err := parseExtra(`{bad`)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestExtraConfig_RequireTargets(t *testing.T) {
	// nil receiver
	if _, _, err := (*ExtraConfig)(nil).requireTargets(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("nil: want ErrInvalidInput, got %v", err)
	}
	// both empty
	if _, _, err := (&ExtraConfig{}).requireTargets(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("empty: want ErrInvalidInput, got %v", err)
	}
	// missing pipeline_name
	if _, _, err := (&ExtraConfig{Namespace: "ci"}).requireTargets(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("missing pipeline: want ErrInvalidInput, got %v", err)
	}
	// missing namespace
	if _, _, err := (&ExtraConfig{PipelineName: "x"}).requireTargets(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("missing namespace: want ErrInvalidInput, got %v", err)
	}
	// whitespace trimming
	pn, ns, err := (&ExtraConfig{PipelineName: " x ", Namespace: " ci "}).requireTargets()
	if err != nil {
		t.Fatalf("valid: %v", err)
	}
	if pn != "x" || ns != "ci" {
		t.Fatalf("trim failed: %q %q", pn, ns)
	}
}

func TestExtraConfig_RequireNamespace(t *testing.T) {
	if _, err := (*ExtraConfig)(nil).requireNamespace(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("nil: want ErrInvalidInput, got %v", err)
	}
	if _, err := (&ExtraConfig{}).requireNamespace(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("empty: want ErrInvalidInput, got %v", err)
	}
	if ns, err := (&ExtraConfig{Namespace: " ci "}).requireNamespace(); err != nil || ns != "ci" {
		t.Fatalf("got ns=%q err=%v", ns, err)
	}
}
