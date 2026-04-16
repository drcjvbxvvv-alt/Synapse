package argo

import (
	"context"
	"testing"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestRegister_NilFactory(t *testing.T) {
	if err := Register(nil, newResolverArgoInstalled(t)); err == nil {
		t.Fatal("expected error")
	}
}

func TestRegister_NilResolver(t *testing.T) {
	if err := Register(engine.NewFactory(), nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRegister_RegistersArgoBuilder(t *testing.T) {
	f := engine.NewFactory()
	if err := Register(f, newResolverArgoInstalled(t)); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !f.IsRegistered(engine.EngineArgo) {
		t.Fatal("EngineArgo should be registered")
	}
}

func TestRegister_Duplicate(t *testing.T) {
	f := engine.NewFactory()
	r := newResolverArgoInstalled(t)
	if err := Register(f, r); err != nil {
		t.Fatal(err)
	}
	if err := Register(f, r); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestRegister_BuildAdapterFromFactory(t *testing.T) {
	f := engine.NewFactory()
	r := newResolverArgoInstalled(t)
	if err := Register(f, r); err != nil {
		t.Fatal(err)
	}
	id := uint(1)
	cfg := &models.CIEngineConfig{
		EngineType: "argo",
		ClusterID:  &id,
		ExtraJSON:  `{"workflow_template_name":"wt","namespace":"ci"}`,
	}
	adapter, err := f.Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if adapter.Type() != engine.EngineArgo {
		t.Fatal("type mismatch")
	}
	v, err := adapter.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != "argoproj.io/v1alpha1" {
		t.Fatalf("version = %q", v)
	}
}

func TestMustRegister_PanicsOnDuplicate(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	f := engine.NewFactory()
	r := newResolverArgoInstalled(t)
	MustRegister(f, r)
	MustRegister(f, r)
}
