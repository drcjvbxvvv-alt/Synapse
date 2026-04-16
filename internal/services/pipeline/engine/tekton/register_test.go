package tekton

import (
	"context"
	"testing"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestRegister_NilFactory(t *testing.T) {
	if err := Register(nil, newResolverWithTektonInstalled(t)); err == nil {
		t.Fatal("expected error for nil factory")
	}
}

func TestRegister_NilResolver(t *testing.T) {
	if err := Register(engine.NewFactory(), nil); err == nil {
		t.Fatal("expected error for nil resolver")
	}
}

func TestRegister_RegistersTektonBuilder(t *testing.T) {
	f := engine.NewFactory()
	if err := Register(f, newResolverWithTektonInstalled(t)); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !f.IsRegistered(engine.EngineTekton) {
		t.Fatal("engine.EngineTekton should be registered")
	}
}

func TestRegister_Duplicate(t *testing.T) {
	f := engine.NewFactory()
	r := newResolverWithTektonInstalled(t)
	if err := Register(f, r); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := Register(f, r); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}

// Full round trip: register → Build(cfg) → call Version via the returned
// adapter.
func TestRegister_BuildAdapterFromFactory(t *testing.T) {
	f := engine.NewFactory()
	r := newResolverWithTektonInstalled(t)
	if err := Register(f, r); err != nil {
		t.Fatalf("Register: %v", err)
	}
	id := uint(1)
	cfg := &models.CIEngineConfig{
		EngineType: "tekton",
		Name:       "tekton-main",
		ClusterID:  &id,
		ExtraJSON:  `{"pipeline_name":"p","namespace":"ci"}`,
	}
	adapter, err := f.Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if adapter.Type() != engine.EngineTekton {
		t.Fatalf("type mismatch")
	}
	v, err := adapter.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "tekton.dev/v1" {
		t.Fatalf("version = %q", v)
	}
}

func TestMustRegister_PanicsOnDuplicate(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate MustRegister")
		}
	}()
	f := engine.NewFactory()
	r := newResolverWithTektonInstalled(t)
	MustRegister(f, r)
	MustRegister(f, r)
}
