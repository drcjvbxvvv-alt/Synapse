package jenkins

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// newTestAdapter wires up an Adapter whose HTTP client talks to the supplied
// mock server. Extra JSON is passed through for tests that need specific
// job_path values.
func newTestAdapter(t *testing.T, h http.HandlerFunc, extraJSON string) (*Adapter, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	cfg := &models.CIEngineConfig{
		Name:       "jenkins-test",
		EngineType: "jenkins",
		Endpoint:   srv.URL,
		Username:   "bot",
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
		EngineType: "jenkins",
		Endpoint:   "https://jenkins.example.com",
		ExtraJSON:  `{bad json`,
	})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewAdapter_EmptyExtraJSON_OK(t *testing.T) {
	a, err := NewAdapter(&models.CIEngineConfig{
		EngineType: "jenkins",
		Endpoint:   "https://jenkins.example.com",
	})
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	if a.extra == nil {
		t.Fatal("extra must not be nil after parseExtra('')")
	}
	if a.extra.JobPath != "" {
		t.Fatalf("expected empty JobPath, got %q", a.extra.JobPath)
	}
}

// ---------------------------------------------------------------------------
// Type / Capabilities
// ---------------------------------------------------------------------------

func TestAdapter_Type(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	if a.Type() != engine.EngineJenkins {
		t.Fatalf("Type = %q", a.Type())
	}
}

func TestAdapter_Capabilities_Contract(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	caps := a.Capabilities()
	want := engine.EngineCapabilities{
		SupportsDAG:          true,
		SupportsMatrix:       true,
		SupportsArtifacts:    true,
		SupportsSecrets:      true,
		SupportsCaching:      false,
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
		if !strings.HasSuffix(r.URL.Path, "/api/json") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("X-Jenkins", "2.426.1")
		_, _ = w.Write([]byte(`{}`))
	}, "")
	if !a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable = false, want true")
	}
}

func TestAdapter_IsAvailable_NeverErrors_OnFailure(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}, "")
	if a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable should be false on 500")
	}
}

func TestAdapter_IsAvailable_Unauthorized(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}, "")
	if a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable should be false on 401")
	}
}

func TestAdapter_Version_ReadsXJenkinsHeader(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Jenkins", "2.426.1")
		_, _ = w.Write([]byte(`{}`))
	}, "")
	v, err := a.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "2.426.1" {
		t.Fatalf("Version = %q, want 2.426.1", v)
	}
}

func TestAdapter_Version_SentinelWhenHeaderMissing(t *testing.T) {
	// Some proxies strip X-Jenkins. We return the sentinel "unknown" so
	// IsAvailable() still succeeds and the UI can show a yellow badge.
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}, "")
	v, err := a.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "unknown" {
		t.Fatalf("Version = %q, want 'unknown'", v)
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
// parseExtra / buildJobURLPath
// ---------------------------------------------------------------------------

func TestParseExtra_Empty(t *testing.T) {
	cfg, err := parseExtra("")
	if err != nil || cfg == nil || cfg.JobPath != "" {
		t.Fatalf("parseExtra empty: cfg=%+v err=%v", cfg, err)
	}
}

func TestParseExtra_Malformed(t *testing.T) {
	_, err := parseExtra(`{not-json`)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestExtraConfig_RequireJobPath(t *testing.T) {
	if _, err := (*ExtraConfig)(nil).requireJobPath(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("nil receiver: want ErrInvalidInput, got %v", err)
	}
	if _, err := (&ExtraConfig{}).requireJobPath(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("empty path: want ErrInvalidInput, got %v", err)
	}
	if p, err := (&ExtraConfig{JobPath: " /foo/bar/ "}).requireJobPath(); err != nil || p != "foo/bar" {
		t.Fatalf("normalised path: got %q err %v", p, err)
	}
}

func TestBuildJobURLPath(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"my-job":       "/job/my-job",
		"foo/bar":      "/job/foo/job/bar",
		"foo/bar/baz":  "/job/foo/job/bar/job/baz",
		"foo//bar":     "/job/foo/job/bar", // dedup empty segments
	}
	for in, want := range cases {
		if got := buildJobURLPath(in); got != want {
			t.Fatalf("buildJobURLPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// All execution-method tests live in dedicated files: trigger_test.go,
// runs_test.go, cancel_test.go, logs_test.go, artifacts_test.go.
// The adapter-level test file focuses on construction + metadata methods.
