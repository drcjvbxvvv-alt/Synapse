package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestRegister_NilFactory(t *testing.T) {
	if err := Register(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRegister_RegistersGitHubBuilder(t *testing.T) {
	f := engine.NewFactory()
	if err := Register(f); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !f.IsRegistered(engine.EngineGitHub) {
		t.Fatal("EngineGitHub should be registered")
	}
}

func TestRegister_Duplicate(t *testing.T) {
	f := engine.NewFactory()
	if err := Register(f); err != nil {
		t.Fatal(err)
	}
	if err := Register(f); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestRegister_BuildAdapterFromFactory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()

	f := engine.NewFactory()
	if err := Register(f); err != nil {
		t.Fatal(err)
	}
	adapter, err := f.Build(&models.CIEngineConfig{
		EngineType: "github",
		Name:       "gh",
		Endpoint:   srv.URL,
		Token:      "t",
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if adapter.Type() != engine.EngineGitHub {
		t.Fatal("type mismatch")
	}
	v, err := adapter.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != apiVersion {
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
	MustRegister(f)
	MustRegister(f)
}
