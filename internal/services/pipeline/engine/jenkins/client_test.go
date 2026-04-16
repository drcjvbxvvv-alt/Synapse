package jenkins

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// newClient — validation
// ---------------------------------------------------------------------------

func TestNewClient_RejectsEmptyEndpoint(t *testing.T) {
	if _, err := newClient(clientConfig{}); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewClient_RejectsNonAbsoluteURL(t *testing.T) {
	_, err := newClient(clientConfig{Endpoint: "jenkins.example.com"})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewClient_InvalidCABundle(t *testing.T) {
	_, err := newClient(clientConfig{
		Endpoint:    "https://jenkins.example.com",
		CABundlePEM: "not-a-cert",
	})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// newRequest — Basic Auth + headers
// ---------------------------------------------------------------------------

func TestNewRequest_SetsBasicAuth(t *testing.T) {
	c, _ := newClient(clientConfig{
		Endpoint: "https://jenkins.example.com",
		Username: "bot",
		APIToken: "tok",
	})
	req, err := c.newRequest(context.Background(), http.MethodGet, "/api/json", nil)
	if err != nil {
		t.Fatalf("newRequest: %v", err)
	}
	user, pass, ok := req.BasicAuth()
	if !ok || user != "bot" || pass != "tok" {
		t.Fatalf("basic auth mismatch: user=%q pass=%q ok=%v", user, pass, ok)
	}
}

func TestNewRequest_OmitsBasicAuthWhenNoCreds(t *testing.T) {
	c, _ := newClient(clientConfig{Endpoint: "https://jenkins.example.com"})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/api/json", nil)
	if _, _, ok := req.BasicAuth(); ok {
		t.Fatal("BasicAuth should not be set when creds empty")
	}
}

func TestNewRequest_URLComposition(t *testing.T) {
	c, _ := newClient(clientConfig{Endpoint: "https://jenkins.example.com"})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/job/foo/api/json", nil)
	if req.URL.String() != "https://jenkins.example.com/job/foo/api/json" {
		t.Fatalf("url = %q", req.URL.String())
	}
}

func TestEnsureLeadingSlash(t *testing.T) {
	if ensureLeadingSlash("foo") != "/foo" {
		t.Fatal("missing slash not added")
	}
	if ensureLeadingSlash("/bar") != "/bar" {
		t.Fatal("existing slash duplicated")
	}
}

// ---------------------------------------------------------------------------
// doJSON happy path + error mapping
// ---------------------------------------------------------------------------

func startTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := newClient(clientConfig{
		Endpoint: srv.URL,
		Username: "bot",
		APIToken: "t",
	})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	return srv, c
}

func TestDoJSON_DecodesSuccess(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "NORMAL"})
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/api/json", nil)
	var out map[string]string
	if err := c.doJSON(req, &out); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if out["mode"] != "NORMAL" {
		t.Fatalf("mode = %q", out["mode"])
	}
}

func TestDoJSON_401_MapsToUnauthorized(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/api/json", nil)
	if err := c.doJSON(req, nil); !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestDoJSON_403_MapsToUnauthorized(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/api/json", nil)
	if err := c.doJSON(req, nil); !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestDoJSON_404_MapsToNotFound(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/job/missing/api/json", nil)
	if err := c.doJSON(req, nil); !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDoJSON_5xx_MapsToUnavailable(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/api/json", nil)
	if err := c.doJSON(req, nil); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Crumb cache + mutation retry
// ---------------------------------------------------------------------------

func TestCrumbCache_FetchThenCache(t *testing.T) {
	var fetches int32
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/crumbIssuer/api/json") {
			atomic.AddInt32(&fetches, 1)
			_ = json.NewEncoder(w).Encode(crumbResponse{
				Crumb: "crumb-1", CrumbRequestField: "Jenkins-Crumb",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	// First mutation → should fetch crumb.
	if err := c.doMutation(context.Background(), http.MethodPost, "/job/foo/build", nil, nil); err != nil {
		t.Fatalf("doMutation #1: %v", err)
	}
	// Second mutation → should reuse cached crumb (no new fetch).
	if err := c.doMutation(context.Background(), http.MethodPost, "/job/foo/build", nil, nil); err != nil {
		t.Fatalf("doMutation #2: %v", err)
	}
	if atomic.LoadInt32(&fetches) != 1 {
		t.Fatalf("expected crumbIssuer called once, got %d", fetches)
	}
}

func TestCrumbCache_403_DropsAndRetries(t *testing.T) {
	var crumbFetches int32
	var mutAttempts int32
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/crumbIssuer/api/json"):
			atomic.AddInt32(&crumbFetches, 1)
			val := "crumb-" + (map[bool]string{true: "stale", false: "fresh"})[atomic.LoadInt32(&crumbFetches) == 1]
			_ = json.NewEncoder(w).Encode(crumbResponse{Crumb: val, CrumbRequestField: "Jenkins-Crumb"})
		case strings.HasSuffix(r.URL.Path, "/job/foo/build"):
			n := atomic.AddInt32(&mutAttempts, 1)
			// First attempt gets 403 → client should drop & retry.
			if n == 1 {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			// Second attempt succeeds.
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	if err := c.doMutation(context.Background(), http.MethodPost, "/job/foo/build", nil, nil); err != nil {
		t.Fatalf("doMutation: %v", err)
	}
	if atomic.LoadInt32(&crumbFetches) != 2 {
		t.Fatalf("expected 2 crumb fetches (initial + post-403), got %d", crumbFetches)
	}
	if atomic.LoadInt32(&mutAttempts) != 2 {
		t.Fatalf("expected 2 mutation attempts, got %d", mutAttempts)
	}
}

func TestCrumbCache_403_SingleRetryOnly(t *testing.T) {
	// If Jenkins keeps returning 403 even after fresh crumb, client must
	// NOT loop forever — it returns the error after the single retry.
	var mutAttempts int32
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/crumbIssuer/api/json") {
			_ = json.NewEncoder(w).Encode(crumbResponse{Crumb: "x", CrumbRequestField: "Jenkins-Crumb"})
			return
		}
		atomic.AddInt32(&mutAttempts, 1)
		w.WriteHeader(http.StatusForbidden)
	})
	err := c.doMutation(context.Background(), http.MethodPost, "/job/foo/build", nil, nil)
	if !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized after retry, got %v", err)
	}
	if atomic.LoadInt32(&mutAttempts) != 2 {
		t.Fatalf("expected exactly 2 attempts, got %d", mutAttempts)
	}
}

func TestCrumbCache_DisabledOnJenkins_NoCrumbSent(t *testing.T) {
	// Some Jenkins instances have CSRF disabled; /crumbIssuer returns 404.
	// Adapter should proceed without a crumb, not fail.
	var crumbHeader string
	_, c := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/crumbIssuer/api/json"):
			w.WriteHeader(http.StatusNotFound)
		case strings.HasSuffix(r.URL.Path, "/job/foo/build"):
			crumbHeader = r.Header.Get("Jenkins-Crumb")
			w.WriteHeader(http.StatusOK)
		}
	})
	if err := c.doMutation(context.Background(), http.MethodPost, "/job/foo/build", nil, nil); err != nil {
		t.Fatalf("doMutation: %v", err)
	}
	if crumbHeader != "" {
		t.Fatalf("Jenkins-Crumb header was set to %q despite issuer returning 404", crumbHeader)
	}
}

func TestCrumbCache_Invalidate(t *testing.T) {
	k := newCrumbCache()
	// Peek returns empty on zero value.
	if c, f := k.peek(); c != "" || f != "" {
		t.Fatalf("empty cache should peek to empty: %q %q", c, f)
	}
	// Manually simulate a fetched crumb (bypass the network).
	k.crumb = "x"
	k.field = "Jenkins-Crumb"
	k.invalidate()
	if c, f := k.peek(); c != "" || f != "" {
		t.Fatalf("invalidate did not clear cache: %q %q", c, f)
	}
}

// ---------------------------------------------------------------------------
// doRaw
// ---------------------------------------------------------------------------

func TestDoRaw_StreamsBody(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "log line 1\nlog line 2\n")
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/job/foo/42/consoleText", nil)
	rc, err := c.doRaw(req)
	if err != nil {
		t.Fatalf("doRaw: %v", err)
	}
	defer func() { _ = rc.Close() }()
	buf, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(buf) != "log line 1\nlog line 2\n" {
		t.Fatalf("body = %q", buf)
	}
}

func TestDoRaw_404(t *testing.T) {
	_, c := startTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/x", nil)
	_, err := c.doRaw(req)
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// mapHTTPStatus branch coverage
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
		{http.StatusInternalServerError, engine.ErrUnavailable},
		{http.StatusBadGateway, engine.ErrUnavailable},
		{http.StatusServiceUnavailable, engine.ErrUnavailable},
		{http.StatusMethodNotAllowed, engine.ErrUnavailable}, // default
		{http.StatusConflict, engine.ErrUnavailable},         // default
	}
	for _, tc := range cases {
		err := mapHTTPStatus(tc.status, "preview")
		if !errors.Is(err, tc.want) {
			t.Fatalf("status %d: got %v want %v", tc.status, err, tc.want)
		}
	}
}

func TestIsCSRFError(t *testing.T) {
	if !isCSRFError(http.StatusForbidden) {
		t.Fatal("403 should be CSRF")
	}
	if isCSRFError(http.StatusUnauthorized) {
		t.Fatal("401 should not be CSRF")
	}
}
