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

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// newClient — input validation
// ---------------------------------------------------------------------------

func TestNewClient_RejectsEmptyEndpoint(t *testing.T) {
	_, err := newClient(clientConfig{})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewClient_RejectsNonAbsoluteURL(t *testing.T) {
	_, err := newClient(clientConfig{Endpoint: "gitlab.example.com"})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewClient_AppendsAPIV4Suffix(t *testing.T) {
	c, err := newClient(clientConfig{Endpoint: "https://gitlab.example.com"})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	if c.baseURL.Path != "/api/v4" {
		t.Fatalf("baseURL.Path = %q, want /api/v4", c.baseURL.Path)
	}
}

func TestNewClient_IdempotentAPIV4Suffix(t *testing.T) {
	c, err := newClient(clientConfig{Endpoint: "https://gitlab.example.com/api/v4"})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	// Must not result in /api/v4/api/v4.
	if c.baseURL.Path != "/api/v4" {
		t.Fatalf("baseURL.Path = %q, want /api/v4", c.baseURL.Path)
	}
}

func TestNewClient_InvalidCABundle(t *testing.T) {
	_, err := newClient(clientConfig{
		Endpoint:    "https://gitlab.example.com",
		CABundlePEM: "not-a-certificate",
	})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// newRequest — authentication + header shape
// ---------------------------------------------------------------------------

func TestNewRequest_SetsPrivateTokenAndAcceptHeaders(t *testing.T) {
	c, err := newClient(clientConfig{Endpoint: "https://gitlab.example.com", Token: "glpat-abc"})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	req, err := c.newRequest(context.Background(), http.MethodGet, "/version", nil)
	if err != nil {
		t.Fatalf("newRequest: %v", err)
	}
	if got := req.Header.Get("PRIVATE-TOKEN"); got != "glpat-abc" {
		t.Fatalf("PRIVATE-TOKEN = %q", got)
	}
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Fatalf("Accept = %q", got)
	}
	wantURL := "https://gitlab.example.com/api/v4/version"
	if req.URL.String() != wantURL {
		t.Fatalf("url = %q, want %q", req.URL.String(), wantURL)
	}
}

func TestNewRequest_OmitsTokenHeaderWhenEmpty(t *testing.T) {
	c, _ := newClient(clientConfig{Endpoint: "https://gitlab.example.com"})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/version", nil)
	if got := req.Header.Get("PRIVATE-TOKEN"); got != "" {
		t.Fatalf("empty token should not set header, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// doJSON — happy path + error mapping
// ---------------------------------------------------------------------------

// startTestServer returns an httptest.Server that behaves like a GitLab API
// endpoint for a single request. The handler is responsible for writing both
// status and body.
func startTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := newClient(clientConfig{Endpoint: srv.URL, Token: "test-token"})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	return srv, c
}

func TestDoJSON_DecodesSuccessResponse(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify we really hit /api/v4/version.
		if !strings.HasSuffix(r.URL.Path, "/api/v4/version") {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			t.Errorf("missing PRIVATE-TOKEN")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gitlabVersion{Version: "16.10.0"})
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/version", nil)
	var out gitlabVersion
	if err := c.doJSON(req, &out); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if out.Version != "16.10.0" {
		t.Fatalf("Version = %q", out.Version)
	}
}

func TestDoJSON_NilOut_DrainsResponse(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"id":1}`)
	})
	req, _ := c.newRequest(context.Background(), http.MethodPost, "/projects/1/pipelines/2/cancel", nil)
	if err := c.doJSON(req, nil); err != nil {
		t.Fatalf("doJSON with nil out: %v", err)
	}
}

func TestDoJSON_401_MapsToUnauthorized(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"401 Unauthorized"}`)
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/version", nil)
	err := c.doJSON(req, nil)
	if !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestDoJSON_403_MapsToUnauthorized(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/version", nil)
	err := c.doJSON(req, nil)
	if !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for 403, got %v", err)
	}
}

func TestDoJSON_404_MapsToNotFound(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/projects/999/pipelines/1", nil)
	err := c.doJSON(req, nil)
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDoJSON_422_MapsToInvalidInput(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"message":"Reference not found"}`)
	})
	req, _ := c.newRequest(context.Background(), http.MethodPost, "/projects/1/pipeline", nil)
	err := c.doJSON(req, nil)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestDoJSON_5xx_MapsToUnavailable(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/version", nil)
	err := c.doJSON(req, nil)
	if !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestDoJSON_ContextCancelled(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Block forever; client ctx cancellation should interrupt.
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req, _ := c.newRequest(ctx, http.MethodGet, "/version", nil)
	err := c.doJSON(req, nil)
	if err == nil {
		t.Fatalf("expected error on cancelled ctx")
	}
}

// ---------------------------------------------------------------------------
// doRaw — streaming path
// ---------------------------------------------------------------------------

func TestDoRaw_ReturnsBody(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "line1\nline2\n")
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/projects/1/jobs/2/trace", nil)
	rc, err := c.doRaw(req)
	if err != nil {
		t.Fatalf("doRaw: %v", err)
	}
	defer func() { _ = rc.Close() }()

	buf, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(buf) != "line1\nline2\n" {
		t.Fatalf("body = %q", buf)
	}
}

func TestDoRaw_404_MapsToNotFound(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/projects/1/jobs/2/trace", nil)
	_, err := c.doRaw(req)
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// mapHTTPStatus — exhaustive branch coverage
// ---------------------------------------------------------------------------

func TestMapHTTPStatus_AllBranches(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, engine.ErrUnauthorized},
		{http.StatusForbidden, engine.ErrUnauthorized},
		{http.StatusNotFound, engine.ErrNotFound},
		{http.StatusBadRequest, engine.ErrInvalidInput},
		{http.StatusUnprocessableEntity, engine.ErrInvalidInput},
		{http.StatusInternalServerError, engine.ErrUnavailable},
		{http.StatusBadGateway, engine.ErrUnavailable},
		{http.StatusServiceUnavailable, engine.ErrUnavailable},
		{http.StatusConflict, engine.ErrUnavailable},     // default branch
		{http.StatusTooManyRequests, engine.ErrUnavailable}, // default branch
	}
	for _, tc := range cases {
		err := mapHTTPStatus(tc.status, "preview")
		if !errors.Is(err, tc.want) {
			t.Fatalf("status %d: got %v, want %v", tc.status, err, tc.want)
		}
	}
}
