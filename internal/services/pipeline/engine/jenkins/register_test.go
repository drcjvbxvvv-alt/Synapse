package jenkins

import (
	"context"
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

func TestRegister_RegistersJenkinsBuilder(t *testing.T) {
	f := engine.NewFactory()
	if err := Register(f); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !f.IsRegistered(engine.EngineJenkins) {
		t.Fatal("engine.EngineJenkins should be registered")
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

// TestRegister_BuildAdapterFromFactory exercises the full wiring:
// register → Build(cfg) → call Version via the returned adapter.
func TestRegister_BuildAdapterFromFactory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Jenkins", "2.426.1")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	f := engine.NewFactory()
	if err := Register(f); err != nil {
		t.Fatalf("Register: %v", err)
	}

	cfg := &models.CIEngineConfig{
		EngineType: "jenkins",
		Name:       "jenkins-main",
		Endpoint:   srv.URL,
		Username:   "bot",
		Token:      "t",
	}
	adapter, err := f.Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if adapter.Type() != engine.EngineJenkins {
		t.Fatalf("type mismatch")
	}
	v, err := adapter.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "2.426.1" {
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
	MustRegister(f)
}
