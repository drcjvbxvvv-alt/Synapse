package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Adapter implements engine.CIEngineAdapter for GitLab CI.
//
// The adapter is stateless beyond its http client and parsed config; it is
// safe to share across goroutines and across many Pipeline runs. One Adapter
// instance is created per CIEngineConfig (see RegisterGitLab in register.go).
type Adapter struct {
	c     *client
	extra *ExtraConfig
	name  string // for logging / diagnostics (equal to CIEngineConfig.Name)
}

// Compile-time proof Adapter satisfies the engine interface.
var _ engine.CIEngineAdapter = (*Adapter)(nil)

// availabilityProbeTimeout bounds IsAvailable() so a slow GitLab does not
// stall the engine-status page. 5 s matches CIEngineService's per-engine
// budget.
const availabilityProbeTimeout = 5 * time.Second

// NewAdapter builds an Adapter from a stored CIEngineConfig.
//
// Errors from this function mean the config is unusable (bad endpoint,
// invalid CA bundle, malformed ExtraJSON). Transient network failures are
// surfaced later via IsAvailable() / operation methods.
func NewAdapter(cfg *models.CIEngineConfig) (*Adapter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("gitlab: nil CIEngineConfig: %w", engine.ErrInvalidInput)
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

// Type returns engine.EngineGitLab.
func (a *Adapter) Type() engine.EngineType { return engine.EngineGitLab }

// IsAvailable calls GET /api/v4/version with a short timeout. Contract
// (CLAUDE §8): must never return an error; a failure is reported as false.
func (a *Adapter) IsAvailable(ctx context.Context) bool {
	probeCtx, cancel := context.WithTimeout(ctx, availabilityProbeTimeout)
	defer cancel()
	_, err := a.fetchVersion(probeCtx)
	return err == nil
}

// Version returns the GitLab server version (e.g. "16.10.0"). The underlying
// endpoint does not require admin privileges but does require a valid token.
func (a *Adapter) Version(ctx context.Context) (string, error) {
	v, err := a.fetchVersion(ctx)
	if err != nil {
		return "", err
	}
	return v.Version, nil
}

// Capabilities describes what GitLab CI can do. Flags are conservative;
// future adapters may flip individual bits on as their implementation
// matures.
func (a *Adapter) Capabilities() engine.EngineCapabilities {
	return engine.EngineCapabilities{
		SupportsDAG:          true, // GitLab 12.2+ DAG via `needs:`
		SupportsMatrix:       true, // GitLab 13.10+ parallel:matrix
		SupportsArtifacts:    true, // native artifact storage
		SupportsSecrets:      true, // CI/CD variables + masked vars
		SupportsCaching:      true, // cache: key/paths
		SupportsApprovals:    true, // `when: manual` + protected envs
		SupportsNotification: true, // GitLab notifications integrations
		SupportsLiveLog:      true, // /trace streaming
	}
}

// fetchVersion is a shared helper used by IsAvailable() + Version().
func (a *Adapter) fetchVersion(ctx context.Context) (*gitlabVersion, error) {
	req, err := a.c.newRequest(ctx, http.MethodGet, "/version", nil)
	if err != nil {
		return nil, err
	}
	var v gitlabVersion
	if err := a.c.doJSON(req, &v); err != nil {
		return nil, fmt.Errorf("gitlab: get version: %w", err)
	}
	return &v, nil
}

// ---------------------------------------------------------------------------
// Execution methods live in their own files:
//   trigger.go    — Trigger
//   runs.go       — GetRun
//   cancel.go     — Cancel
//   logs.go       — StreamLogs
//   artifacts.go  — GetArtifacts
// ---------------------------------------------------------------------------
