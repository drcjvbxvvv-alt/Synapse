package jenkins

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// crumbCache — lazy, concurrency-safe cache of the CSRF crumb.
// ---------------------------------------------------------------------------
//
// Jenkins' CSRF protection issues a short-lived crumb via
// GET /crumbIssuer/api/json. Every mutating HTTP call must include it in
// the `Jenkins-Crumb` header (the exact header name is returned alongside
// the crumb so installations with customised field names still work).
//
// The cache semantics are intentionally simple:
//   - Zero-value cache has no crumb; the first get() fetches one.
//   - invalidate() clears the cache (used when a mutation gets 403).
//   - get() is serialised via a sync.Mutex to avoid the thundering-herd
//     problem where a dozen parallel requests all fetch a crumb on cold
//     start.
//
// The cache is per-*client — two different Jenkins endpoints (e.g. two
// separate CIEngineConfig rows) each carry their own crumb state.

// crumbCache holds a single active crumb + its header name.
type crumbCache struct {
	mu    sync.Mutex
	crumb string
	field string // header name, usually "Jenkins-Crumb"
}

// newCrumbCache constructs an empty cache.
func newCrumbCache() *crumbCache { return &crumbCache{} }

// get returns the current crumb; on first call (or after invalidate) it
// fetches a fresh one from Jenkins. The `c` parameter breaks the circular
// type dependency (crumbCache doesn't import the client type directly).
func (k *crumbCache) get(ctx context.Context, c *client) (crumb, field string, err error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.crumb != "" && k.field != "" {
		return k.crumb, k.field, nil
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/crumbIssuer/api/json", nil)
	if err != nil {
		return "", "", fmt.Errorf("jenkins: build crumb request: %w", err)
	}
	var resp crumbResponse
	if err := c.doJSON(req, &resp); err != nil {
		return "", "", err
	}
	if resp.Crumb == "" || resp.CrumbRequestField == "" {
		// Some mis-configured / stubbed Jenkins instances return 200 with
		// an empty body. Treat as "no crumb needed" so mutations proceed
		// without a header.
		return "", "", fmt.Errorf("jenkins: crumb issuer returned empty crumb: %w", engine.ErrUnavailable)
	}
	k.crumb = resp.Crumb
	k.field = resp.CrumbRequestField
	return k.crumb, k.field, nil
}

// invalidate clears the cache so the next get() re-fetches. Used by the
// client when it receives a 403 on a mutation call.
func (k *crumbCache) invalidate() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.crumb = ""
	k.field = ""
}

// peek returns the current cached crumb (empty if none) without
// triggering a fetch. Primarily useful for tests and diagnostics.
func (k *crumbCache) peek() (string, string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.crumb, k.field
}
