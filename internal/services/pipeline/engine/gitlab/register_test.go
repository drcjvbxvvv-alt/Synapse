package gitlab

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
		t.Fatal("expected error for nil factory")
	}
}

func TestRegister_RegistersGitLabBuilder(t *testing.T) {
	f := engine.NewFactory()
	if err := Register(f); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !f.IsRegistered(engine.EngineGitLab) {
		t.Fatal("engine.EngineGitLab should be registered")
	}
}

func TestRegister_DoubleRegistration_Errors(t *testing.T) {
	f := engine.NewFactory()
	if err := Register(f); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := Register(f); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}

// TestRegister_BuildAdapterFromFactory exercises the full round trip:
// register → Build(cfg) → call Version through the returned adapter.
func TestRegister_BuildAdapterFromFactory(t *testing.T) {
	// Stand up a mock GitLab.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(gitlabVersion{Version: "16.11.0"})
	}))
	defer srv.Close()

	f := engine.NewFactory()
	if err := Register(f); err != nil {
		t.Fatalf("Register: %v", err)
	}

	cfg := &models.CIEngineConfig{
		EngineType: "gitlab",
		Name:       "gl",
		Endpoint:   srv.URL,
		Token:      "t",
	}
	adapter, err := f.Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if adapter.Type() != engine.EngineGitLab {
		t.Fatalf("type mismatch")
	}
	v, err := adapter.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "16.11.0" {
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
	MustRegister(f)
	MustRegister(f) // should panic
}
