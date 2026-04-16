package jenkins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Adapter implements engine.CIEngineAdapter for Jenkins.
//
// One adapter is built per CIEngineConfig and may serve many concurrent
// requests. Jenkins returns per-request crumbs; the embedded *client caches
// the crumb across invocations (see crumb.go).
type Adapter struct {
	c     *client
	extra *ExtraConfig
	name  string
}

// Compile-time assertion: Adapter satisfies CIEngineAdapter.
var _ engine.CIEngineAdapter = (*Adapter)(nil)

// availabilityProbeTimeout bounds IsAvailable() so a slow Jenkins does not
// stall the engine-status page. Matches the GitLab adapter.
const availabilityProbeTimeout = 5 * time.Second

// NewAdapter constructs an Adapter from a stored CIEngineConfig.
//
// Returns engine.ErrInvalidInput for obvious misconfiguration (bad URL,
// invalid CA bundle, malformed ExtraJSON). Transient network failures
// surface later via IsAvailable() / per-method errors.
func NewAdapter(cfg *models.CIEngineConfig) (*Adapter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("jenkins: nil CIEngineConfig: %w", engine.ErrInvalidInput)
	}
	extra, err := parseExtra(cfg.ExtraJSON)
	if err != nil {
		return nil, err
	}
	// Jenkins Basic Auth = username + api token. CIEngineConfig holds the
	// API token in `Token` and the Jenkins username in `Username`.
	c, err := newClient(clientConfig{
		Endpoint:           cfg.Endpoint,
		Username:           cfg.Username,
		APIToken:           cfg.Token,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		CABundlePEM:        cfg.CABundle,
	})
	if err != nil {
		return nil, err
	}
	return &Adapter{c: c, extra: extra, name: cfg.Name}, nil
}

// ---------------------------------------------------------------------------
// Capability probing
// ---------------------------------------------------------------------------

// Type returns engine.EngineJenkins.
func (a *Adapter) Type() engine.EngineType { return engine.EngineJenkins }

// IsAvailable calls GET /api/json with a short timeout. Contract: never
// returns an error; a failure is reported as false.
func (a *Adapter) IsAvailable(ctx context.Context) bool {
	probeCtx, cancel := context.WithTimeout(ctx, availabilityProbeTimeout)
	defer cancel()
	_, err := a.fetchVersion(probeCtx)
	return err == nil
}

// Version returns the Jenkins controller version. Jenkins publishes it via
// the X-Jenkins response header, not a JSON body field, so we extract it
// there. Installations behind a proxy that strips X-Jenkins get the
// sentinel value "unknown" rather than an error.
func (a *Adapter) Version(ctx context.Context) (string, error) {
	return a.fetchVersion(ctx)
}

// Capabilities describes what Jenkins can do. Flags reflect what this
// adapter currently exposes — Jenkins itself supports more (Blue Ocean
// artifacts download, pipeline visualisation …) which M18c does not wire
// through.
func (a *Adapter) Capabilities() engine.EngineCapabilities {
	return engine.EngineCapabilities{
		SupportsDAG:          true,  // Jenkins declarative pipelines + parallel stages
		SupportsMatrix:       true,  // `matrix` directive
		SupportsArtifacts:    true,  // archiveArtifacts
		SupportsSecrets:      true,  // credentials plugin + withCredentials
		SupportsCaching:      false, // not natively; plugins vary too widely
		SupportsApprovals:    true,  // input step / milestone
		SupportsNotification: true,  // Jenkins notifications + plugins
		SupportsLiveLog:      true,  // progressiveText streaming
	}
}

// fetchVersion does a single GET /api/json and extracts the X-Jenkins
// header. Returns the sentinel "unknown" when the header is missing so
// availability checks don't fail for stripped-header proxies.
func (a *Adapter) fetchVersion(ctx context.Context) (string, error) {
	req, err := a.c.newRequest(ctx, http.MethodGet, "/api/json", nil)
	if err != nil {
		return "", err
	}
	resp, err := a.c.httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("jenkins: http: %w: %w", err, engine.ErrUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := readPreview(resp.Body, 256)
		return "", mapHTTPStatus(resp.StatusCode, preview)
	}
	// Drain the body so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBodyBytes))

	v := resp.Header.Get("X-Jenkins")
	if v == "" {
		return "unknown", nil
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// Execution methods (Stage 3-4 replace these stubs with real files).
// Retaining the stubs here means the interface compile-time assertion
// holds from Stage 2 onwards.
// ---------------------------------------------------------------------------

// Trigger is implemented in trigger.go.
// GetRun is implemented in runs.go.
// Cancel is implemented in cancel.go.
// StreamLogs is implemented in logs.go.
// GetArtifacts is implemented in artifacts.go.
