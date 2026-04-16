package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// newTestAdapter wires up an Adapter whose HTTP client talks to the supplied
// mock server. Returns both the adapter and the server so tests can inspect
// / close it.
func newTestAdapter(t *testing.T, h http.HandlerFunc, extraJSON string) (*Adapter, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	cfg := &models.CIEngineConfig{
		Name:       "gl-test",
		EngineType: "gitlab",
		Endpoint:   srv.URL,
		Token:      "test-token",
		ExtraJSON:  extraJSON,
	}
	a, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	return a, srv
}

// ---------------------------------------------------------------------------
// NewAdapter
// ---------------------------------------------------------------------------

func TestNewAdapter_NilConfig(t *testing.T) {
	_, err := NewAdapter(nil)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewAdapter_InvalidExtraJSON(t *testing.T) {
	_, err := NewAdapter(&models.CIEngineConfig{
		EngineType: "gitlab",
		Endpoint:   "https://gitlab.example.com",
		ExtraJSON:  `{"project_id":`, // malformed
	})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewAdapter_EmptyExtraJSON_OK(t *testing.T) {
	// An empty ExtraJSON is legal at construction time; Trigger etc. will
	// later enforce project_id presence when required.
	a, err := NewAdapter(&models.CIEngineConfig{
		EngineType: "gitlab",
		Endpoint:   "https://gitlab.example.com",
	})
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	if a.extra == nil {
		t.Fatal("extra should not be nil after parseExtra('')")
	}
	if a.extra.ProjectID != 0 {
		t.Fatalf("ProjectID = %d, want 0", a.extra.ProjectID)
	}
}

// ---------------------------------------------------------------------------
// Type / Capabilities
// ---------------------------------------------------------------------------

func TestAdapter_Type(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	if a.Type() != engine.EngineGitLab {
		t.Fatalf("Type = %q, want gitlab", a.Type())
	}
}

func TestAdapter_Capabilities_Contract(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	caps := a.Capabilities()
	// Lock in the stage-2 contract. Future capability toggles must update
	// this test in lock-step so the change is visible in review.
	want := engine.EngineCapabilities{
		SupportsDAG:          true,
		SupportsMatrix:       true,
		SupportsArtifacts:    true,
		SupportsSecrets:      true,
		SupportsCaching:      true,
		SupportsApprovals:    true,
		SupportsNotification: true,
		SupportsLiveLog:      true,
	}
	if caps != want {
		t.Fatalf("Capabilities mismatch\n got: %+v\nwant: %+v", caps, want)
	}
}

// ---------------------------------------------------------------------------
// IsAvailable / Version
// ---------------------------------------------------------------------------

func TestAdapter_IsAvailable_Happy(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/v4/version") {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(gitlabVersion{Version: "16.10.0"})
	}, "")
	if !a.IsAvailable(context.Background()) {
		t.Fatalf("IsAvailable = false, want true")
	}
}

func TestAdapter_IsAvailable_NeverErrors_OnFailure(t *testing.T) {
	// Contract (CLAUDE §8): IsAvailable() must never return an error; a
	// failure is reported as false.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}, "")
	// Should not panic, should not error; just returns false.
	if a.IsAvailable(context.Background()) {
		t.Fatalf("IsAvailable = true despite 500")
	}
}

func TestAdapter_IsAvailable_Unauthorized_ReturnsFalse(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}, "")
	if a.IsAvailable(context.Background()) {
		t.Fatalf("IsAvailable should be false on 401")
	}
}

func TestAdapter_Version_Success(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(gitlabVersion{Version: "16.10.0", Revision: "abc"})
	}, "")
	v, err := a.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "16.10.0" {
		t.Fatalf("Version = %q", v)
	}
}

func TestAdapter_Version_UnauthorizedMapped(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}, "")
	_, err := a.Version(context.Background())
	if !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// parseExtra / ExtraConfig
// ---------------------------------------------------------------------------

func TestParseExtra_Empty(t *testing.T) {
	cfg, err := parseExtra("")
	if err != nil {
		t.Fatalf("parseExtra: %v", err)
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}
	if cfg.ProjectID != 0 {
		t.Fatalf("ProjectID = %d", cfg.ProjectID)
	}
}

func TestParseExtra_Valid(t *testing.T) {
	cfg, err := parseExtra(`{"project_id": 42, "default_ref": "main"}`)
	if err != nil {
		t.Fatalf("parseExtra: %v", err)
	}
	if cfg.ProjectID != 42 {
		t.Fatalf("ProjectID = %d", cfg.ProjectID)
	}
	if cfg.DefaultRef != "main" {
		t.Fatalf("DefaultRef = %q", cfg.DefaultRef)
	}
}

func TestParseExtra_Malformed(t *testing.T) {
	_, err := parseExtra(`{not-json`)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestExtraConfig_RequireProjectID(t *testing.T) {
	var cfg *ExtraConfig
	if _, err := cfg.requireProjectID(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("nil receiver: expected ErrInvalidInput, got %v", err)
	}
	cfg = &ExtraConfig{}
	if _, err := cfg.requireProjectID(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("zero ProjectID: expected ErrInvalidInput, got %v", err)
	}
	cfg.ProjectID = 42
	if id, err := cfg.requireProjectID(); err != nil || id != 42 {
		t.Fatalf("valid: id=%d err=%v", id, err)
	}
}

// Tests for Cancel / StreamLogs / GetArtifacts live in the dedicated
// *_test.go files for each method (cancel_test.go, logs_test.go,
// artifacts_test.go). The adapter-level test file focuses on construction
// and metadata methods.

// keep io reachable so future test refactors don't need to re-add the import
var _ = io.EOF
