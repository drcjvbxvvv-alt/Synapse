package github

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Adapter implements engine.CIEngineAdapter for GitHub Actions.
type Adapter struct {
	c     *client
	extra *ExtraConfig
	name  string
}

var _ engine.CIEngineAdapter = (*Adapter)(nil)

const availabilityProbeTimeout = 5 * time.Second

// NewAdapter constructs an Adapter from a stored CIEngineConfig.
//
// When cfg.Endpoint is empty the adapter targets the public github.com API
// (https://api.github.com). GHE customers supply their server URL; the
// client adds the /api/v3 suffix automatically.
func NewAdapter(cfg *models.CIEngineConfig) (*Adapter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("github: nil CIEngineConfig: %w", engine.ErrInvalidInput)
	}
	extra, err := parseExtra(cfg.ExtraJSON)
	if err != nil {
		return nil, err
	}
	c, err := newClient(clientConfig{
		Endpoint:           cfg.Endpoint,
		Token:              cfg.Token,
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

// Type returns engine.EngineGitHub.
func (a *Adapter) Type() engine.EngineType { return engine.EngineGitHub }

// IsAvailable probes GET /meta which does not require authentication. It
// only proves we can reach the API; authentication is exercised in later
// methods. Never returns an error (CLAUDE §8).
func (a *Adapter) IsAvailable(ctx context.Context) bool {
	probeCtx, cancel := context.WithTimeout(ctx, availabilityProbeTimeout)
	defer cancel()
	req, err := a.c.newRequest(probeCtx, http.MethodGet, "/meta", nil)
	if err != nil {
		return false
	}
	// Discard the response body; we only care about status.
	return a.c.doJSON(req, nil) == nil
}

// Version returns the API version string the client speaks against. GitHub
// does not surface an X-API-Version header, so we report the constant that
// governs our request headers.
func (a *Adapter) Version(context.Context) (string, error) {
	return apiVersion, nil
}

// Capabilities describes what the adapter exposes. GitHub Actions supports
// more than we currently wire (matrix builds, caching via actions/cache,
// approval gates via environments); flags stay conservative.
func (a *Adapter) Capabilities() engine.EngineCapabilities {
	return engine.EngineCapabilities{
		SupportsDAG:          true,  // jobs[].needs
		SupportsMatrix:       true,  // strategy.matrix
		SupportsArtifacts:    true,  // actions/upload-artifact
		SupportsSecrets:      true,  // repo / org / env secrets
		SupportsCaching:      true,  // actions/cache
		SupportsApprovals:    true,  // environments + required reviewers
		SupportsNotification: false, // GitHub notifies via email/UI; Synapse bridges for consolidated alerting
		SupportsLiveLog:      true,  // /jobs/{id}/logs returns text
	}
}

// ---------------------------------------------------------------------------
// Execution methods (Stage 2-3 stubs)
// ---------------------------------------------------------------------------

// Execution methods live in their own files: trigger.go, runs.go,
// cancel.go, logs.go, artifacts.go.
