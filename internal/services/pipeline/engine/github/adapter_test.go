package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// newTestAdapter returns an Adapter whose HTTP client points at the given
// test server. An empty extraJSON is legal at this stage; methods that
// require owner/repo/workflow_id enforce that separately.
func newTestAdapter(t *testing.T, h http.HandlerFunc, extraJSON string) (*Adapter, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cfg := &models.CIEngineConfig{
		Name:       "gh",
		EngineType: "github",
		Endpoint:   srv.URL,
		Token:      "pat-test",
		ExtraJSON:  extraJSON,
	}
	a, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	return a, srv
}

// ---------------------------------------------------------------------------
// Client-level plumbing
// ---------------------------------------------------------------------------

func TestNewClient_DefaultsToPublicGithub(t *testing.T) {
	c, err := newClient(clientConfig{})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	if c.baseURL.String() != "https://api.github.com" {
		t.Fatalf("baseURL = %q", c.baseURL.String())
	}
}

func TestNewClient_GHE_AppendsAPIv3(t *testing.T) {
	c, err := newClient(clientConfig{Endpoint: "https://github.example.com"})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	if c.baseURL.Path != "/api/v3" {
		t.Fatalf("baseURL.Path = %q", c.baseURL.Path)
	}
}

func TestNewClient_GHE_Idempotent(t *testing.T) {
	c, err := newClient(clientConfig{Endpoint: "https://github.example.com/api/v3"})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	if c.baseURL.Path != "/api/v3" {
		t.Fatalf("duplicated: %q", c.baseURL.Path)
	}
}

func TestNewClient_PublicAPI_NoPathAppend(t *testing.T) {
	c, err := newClient(clientConfig{Endpoint: "https://api.github.com"})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	if c.baseURL.Path != "" {
		t.Fatalf("public api should have empty path, got %q", c.baseURL.Path)
	}
}

func TestNewClient_InvalidCABundle(t *testing.T) {
	_, err := newClient(clientConfig{
		Endpoint:    "https://api.github.com",
		CABundlePEM: "not-a-cert",
	})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestNewClient_NonAbsoluteURL(t *testing.T) {
	_, err := newClient(clientConfig{Endpoint: "github.com"})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestNewRequest_HeadersAndAuth(t *testing.T) {
	c, _ := newClient(clientConfig{Endpoint: "https://api.github.com", Token: "abc"})
	req, err := c.newRequest(context.Background(), http.MethodGet, "/meta", nil)
	if err != nil {
		t.Fatalf("newRequest: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer abc" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := req.Header.Get("Accept"); got != "application/vnd.github+json" {
		t.Fatalf("Accept = %q", got)
	}
	if got := req.Header.Get("X-GitHub-Api-Version"); got != apiVersion {
		t.Fatalf("X-GitHub-Api-Version = %q", got)
	}
}

func TestNewRequest_OmitsAuthWhenEmpty(t *testing.T) {
	c, _ := newClient(clientConfig{Endpoint: "https://api.github.com"})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/meta", nil)
	if req.Header.Get("Authorization") != "" {
		t.Fatal("Authorization should be empty")
	}
}

// ---------------------------------------------------------------------------
// doJSON happy path + error mapping
// ---------------------------------------------------------------------------

func TestDoJSON_DecodesSuccess(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"verifiable_password_authentication": true})
	}, "")
	req, _ := a.c.newRequest(context.Background(), http.MethodGet, "/meta", nil)
	var out map[string]bool
	if err := a.c.doJSON(req, &out); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if !out["verifiable_password_authentication"] {
		t.Fatalf("body not decoded")
	}
}

func TestDoJSON_HonoursHTTPStatusMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, engine.ErrUnauthorized},
		{http.StatusForbidden, engine.ErrUnauthorized},
		{http.StatusNotFound, engine.ErrNotFound},
		{http.StatusUnprocessableEntity, engine.ErrInvalidInput},
		{http.StatusBadRequest, engine.ErrInvalidInput},
		{http.StatusBadGateway, engine.ErrUnavailable},
	}
	for _, tc := range cases {
		code := tc.status // capture
		a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}, "")
		req, _ := a.c.newRequest(context.Background(), http.MethodGet, "/meta", nil)
		err := a.c.doJSON(req, nil)
		if !errors.Is(err, tc.want) {
			t.Fatalf("status %d: got %v want %v", tc.status, err, tc.want)
		}
	}
}

func TestDoJSON_204_NoContent_IgnoresBody(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, "")
	req, _ := a.c.newRequest(context.Background(), http.MethodPost, "/x/dispatches", strings.NewReader("{}"))
	// Pass a pointer to ensure the 204 path doesn't try to decode.
	var out map[string]any
	if err := a.c.doJSON(req, &out); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if out != nil {
		t.Fatalf("out should remain nil for 204, got %+v", out)
	}
}

// ---------------------------------------------------------------------------
// Adapter metadata methods
// ---------------------------------------------------------------------------

func TestAdapter_Type(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	if a.Type() != engine.EngineGitHub {
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
		SupportsCaching:      true,
		SupportsApprovals:    true,
		SupportsNotification: false,
		SupportsLiveLog:      true,
	}
	if caps != want {
		t.Fatalf("Capabilities mismatch\n got: %+v\nwant: %+v", caps, want)
	}
}

func TestAdapter_IsAvailable_Happy(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/meta") {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}, "")
	if !a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable = false")
	}
}

func TestAdapter_IsAvailable_NeverErrors_On5xx(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}, "")
	if a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable should be false")
	}
}

func TestAdapter_Version_ReportsAPIVersion(t *testing.T) {
	a, _ := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	v, err := a.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != apiVersion {
		t.Fatalf("Version = %q", v)
	}
}

// ---------------------------------------------------------------------------
// ExtraConfig helpers
// ---------------------------------------------------------------------------

func TestParseExtra_Empty(t *testing.T) {
	cfg, err := parseExtra("")
	if err != nil || cfg == nil || cfg.Owner != "" {
		t.Fatalf("got cfg=%+v err=%v", cfg, err)
	}
}

func TestParseExtra_Valid(t *testing.T) {
	cfg, err := parseExtra(`{"owner":"o","repo":"r","workflow_id":"build.yml","default_ref":"main"}`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Owner != "o" || cfg.Repo != "r" || cfg.WorkflowID != "build.yml" || cfg.DefaultRef != "main" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestParseExtra_Malformed(t *testing.T) {
	if _, err := parseExtra(`{bad`); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestRequireTargets(t *testing.T) {
	if _, _, _, err := (*ExtraConfig)(nil).requireTargets(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("nil: %v", err)
	}
	if _, _, _, err := (&ExtraConfig{}).requireTargets(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("empty: %v", err)
	}
	if _, _, _, err := (&ExtraConfig{Owner: "o"}).requireTargets(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("no repo: %v", err)
	}
	if _, _, _, err := (&ExtraConfig{Owner: "o", Repo: "r"}).requireTargets(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("no workflow: %v", err)
	}
	o, r, w, err := (&ExtraConfig{Owner: " o ", Repo: " r ", WorkflowID: " w "}).requireTargets()
	if err != nil || o != "o" || r != "r" || w != "w" {
		t.Fatalf("trim failed: %q %q %q %v", o, r, w, err)
	}
}

func TestRequireOwnerRepo(t *testing.T) {
	if _, _, err := (*ExtraConfig)(nil).requireOwnerRepo(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("nil: %v", err)
	}
	if _, _, err := (&ExtraConfig{Owner: "o"}).requireOwnerRepo(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("no repo: %v", err)
	}
	o, r, err := (&ExtraConfig{Owner: "o", Repo: "r"}).requireOwnerRepo()
	if err != nil || o != "o" || r != "r" {
		t.Fatalf("got %q %q %v", o, r, err)
	}
}

// ---------------------------------------------------------------------------
// Stage 2-3 stub verification
// ---------------------------------------------------------------------------

// Tests for Cancel / StreamLogs / GetArtifacts live in their dedicated files.

// ---------------------------------------------------------------------------
// mapHTTPStatus branch coverage
// ---------------------------------------------------------------------------

func TestMapHTTPStatus_Branches(t *testing.T) {
	cases := []struct {
		code int
		want error
	}{
		{401, engine.ErrUnauthorized},
		{403, engine.ErrUnauthorized},
		{404, engine.ErrNotFound},
		{400, engine.ErrInvalidInput},
		{422, engine.ErrInvalidInput},
		{500, engine.ErrUnavailable},
		{502, engine.ErrUnavailable},
		{429, engine.ErrUnavailable}, // default branch
		{409, engine.ErrUnavailable}, // default branch
	}
	for _, tc := range cases {
		err := mapHTTPStatus(tc.code, "preview")
		if !errors.Is(err, tc.want) {
			t.Fatalf("%d: got %v want %v", tc.code, err, tc.want)
		}
	}
}
