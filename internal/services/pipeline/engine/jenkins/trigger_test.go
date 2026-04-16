package jenkins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// newTriggerServer returns a test server + adapter with short queue poll
// settings so tests don't actually wait the default 10 s.
//
// The handler is called for every non-crumb path; the crumb issuer is
// stubbed uniformly inside this helper.
func newTriggerServer(t *testing.T, handler http.HandlerFunc, extraJSON string) (*Adapter, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/crumbIssuer/api/json") {
			_ = json.NewEncoder(w).Encode(crumbResponse{Crumb: "c", CrumbRequestField: "Jenkins-Crumb"})
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	cfg := &models.CIEngineConfig{
		Name:       "jt",
		EngineType: "jenkins",
		Endpoint:   srv.URL,
		Username:   "bot",
		Token:      "t",
		ExtraJSON:  extraJSON,
	}
	a, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	return a, srv
}

// ---------------------------------------------------------------------------
// extractQueueID
// ---------------------------------------------------------------------------

func TestExtractQueueID(t *testing.T) {
	cases := map[string]int64{
		"http://example.com/queue/item/42/":          42,
		"https://jenkins.example.com/queue/item/7":   7,
		"/queue/item/123/":                           123,
	}
	for in, want := range cases {
		got, err := extractQueueID(in)
		if err != nil {
			t.Fatalf("extractQueueID(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("extractQueueID(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestExtractQueueID_Invalid(t *testing.T) {
	for _, in := range []string{"", "/queue/item/", "/queue/item/abc/", "bogus"} {
		if _, err := extractQueueID(in); !errors.Is(err, engine.ErrUnavailable) {
			t.Fatalf("extractQueueID(%q): expected ErrUnavailable, got %v", in, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Trigger — validation
// ---------------------------------------------------------------------------

func TestTrigger_NilRequest(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, `{"job_path":"foo"}`)
	_, err := a.Trigger(context.Background(), nil)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTrigger_MissingJobPath(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, _ *http.Request) {}, "")
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Trigger — happy paths
// ---------------------------------------------------------------------------

func TestTrigger_Success_ImmediateBuildNumber(t *testing.T) {
	// Mock: 201 with Location, then queue item returns executable immediately.
	var seenParams string
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/job/foo/buildWithParameters"):
			body, _ := io.ReadAll(r.Body)
			seenParams = string(body)
			w.Header().Set("Location", "http://"+r.Host+"/queue/item/42/")
			w.WriteHeader(http.StatusCreated)
		case strings.HasSuffix(r.URL.Path, "/queue/item/42/api/json"):
			_ = json.NewEncoder(w).Encode(queueItem{
				Executable: &queueExecutable{Number: 100, URL: "/job/foo/100/"},
			})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}, `{"job_path":"foo"}`)

	res, err := a.Trigger(context.Background(), &engine.TriggerRequest{
		Variables: map[string]string{"ENV": "staging"},
	})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if res.RunID != "100" || res.ExternalID != "100" {
		t.Fatalf("RunID/ExternalID mismatch: %+v", res)
	}
	if !strings.Contains(res.URL, "/job/foo/100/") {
		t.Fatalf("URL should point to build page, got %q", res.URL)
	}
	if !strings.Contains(seenParams, "ENV=staging") {
		t.Fatalf("variables not propagated, body=%q", seenParams)
	}
}

func TestTrigger_FolderedJobPath(t *testing.T) {
	var seenPath string
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			seenPath = r.URL.Path
			w.Header().Set("Location", "http://"+r.Host+"/queue/item/1/")
			w.WriteHeader(http.StatusCreated)
		default:
			_ = json.NewEncoder(w).Encode(queueItem{
				Executable: &queueExecutable{Number: 1},
			})
		}
	}, `{"job_path":"folder1/folder2/myjob"}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if !strings.HasSuffix(seenPath, "/job/folder1/job/folder2/job/myjob/buildWithParameters") {
		t.Fatalf("job path not URL-encoded into folder segments: %q", seenPath)
	}
}

func TestTrigger_TimeoutFallsBackToQueueID(t *testing.T) {
	// Queue never produces an executable → we expect queue:<id> RunID.
	// Lower queuePollTimeout by cancelling the caller context early is
	// tricky; instead we rely on the queuePollTimeout constant being
	// short enough for testing (10 s) — keep the handler returning fast
	// (never scheduled) and assert the queue prefix after a shortened
	// context.
	// We use ctx.WithTimeout to bound the test quickly.
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			w.Header().Set("Location", "http://"+r.Host+"/queue/item/77/")
			w.WriteHeader(http.StatusCreated)
		default:
			// Queue item never gets an executable.
			_ = json.NewEncoder(w).Encode(queueItem{})
		}
	}, `{"job_path":"foo"}`)

	// Shorten the poll budget by cancelling the outer ctx; waitForBuildNumber
	// treats outer ctx cancellation as a timeout (returns 0, nil).
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = ctx // silence unused if code path short-circuits

	res, err := a.Trigger(ctx, &engine.TriggerRequest{})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if !strings.HasPrefix(res.RunID, queueRunIDPrefix) {
		t.Fatalf("expected queue prefix, got RunID=%q", res.RunID)
	}
	if !strings.HasSuffix(res.URL, "/queue/item/77/") {
		t.Fatalf("URL should point at queue item, got %q", res.URL)
	}
}

func TestTrigger_QueueCancelled(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Location", "http://"+r.Host+"/queue/item/55/")
			w.WriteHeader(http.StatusCreated)
			return
		}
		_ = json.NewEncoder(w).Encode(queueItem{Cancelled: true})
	}, `{"job_path":"foo"}`)

	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if !errors.Is(err, engine.ErrAlreadyTerminal) {
		t.Fatalf("expected ErrAlreadyTerminal, got %v", err)
	}
}

func TestTrigger_MissingLocationHeader(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// 201 but no Location header — defensive edge case.
			w.WriteHeader(http.StatusCreated)
			return
		}
	}, `{"job_path":"foo"}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestTrigger_Unauthorized(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}, `{"job_path":"foo"}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestTrigger_BuildPageURLConstruction(t *testing.T) {
	a, _ := newTriggerServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Location", fmt.Sprintf("http://%s/queue/item/1/", r.Host))
			w.WriteHeader(http.StatusCreated)
			return
		}
		_ = json.NewEncoder(w).Encode(queueItem{Executable: &queueExecutable{Number: 7}})
	}, `{"job_path":"parent/child"}`)
	res, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if !strings.HasSuffix(res.URL, "/job/parent/job/child/7/") {
		t.Fatalf("build page URL wrong: %q", res.URL)
	}
}
