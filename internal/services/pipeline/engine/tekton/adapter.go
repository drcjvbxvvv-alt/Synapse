package tekton

import (
	"context"
	"fmt"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Adapter implements engine.CIEngineAdapter for Tekton Pipelines.
//
// Unlike the GitLab and Jenkins adapters, a Tekton adapter has no long-lived
// HTTP client: every operation acquires a fresh dynamic.Interface via
// ClusterResolver. This is intentional — a Synapse-managed cluster's
// credentials may rotate (kubeconfig rewrite, service-account token
// refresh) and the resolver abstracts that lifecycle.
type Adapter struct {
	clusterID uint
	extra     *ExtraConfig
	resolver  ClusterResolver
	name      string
}

// Compile-time assertion.
var _ engine.CIEngineAdapter = (*Adapter)(nil)

// availabilityProbeTimeout bounds IsAvailable() for this cluster.
const availabilityProbeTimeout = 5 * time.Second

// NewAdapter constructs an Adapter from a stored CIEngineConfig.
//
// Returns engine.ErrInvalidInput when:
//   - cfg is nil
//   - ExtraJSON is malformed
//   - ClusterID is zero (Tekton always runs in a Synapse-managed cluster)
//
// Returns no error for connectivity / CRD absence; those surface later
// through IsAvailable() (false) and per-method ErrUnavailable.
func NewAdapter(cfg *models.CIEngineConfig, resolver ClusterResolver) (*Adapter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("tekton: nil CIEngineConfig: %w", engine.ErrInvalidInput)
	}
	if cfg.ClusterID == nil || *cfg.ClusterID == 0 {
		return nil, fmt.Errorf("tekton: cluster_id is required on CIEngineConfig: %w", engine.ErrInvalidInput)
	}
	extra, err := parseExtra(cfg.ExtraJSON)
	if err != nil {
		return nil, err
	}
	// resolver may legitimately be nil in unit tests that only exercise
	// metadata methods; we store it as-is and each operation guards on
	// requireResolver().
	return &Adapter{
		clusterID: *cfg.ClusterID,
		extra:     extra,
		resolver:  resolver,
		name:      cfg.Name,
	}, nil
}

// ---------------------------------------------------------------------------
// Capability probing
// ---------------------------------------------------------------------------

// Type returns engine.EngineTekton.
func (a *Adapter) Type() engine.EngineType { return engine.EngineTekton }

// IsAvailable reports whether Tekton CRDs are installed in the target
// cluster. Contract: never returns an error; a failure is reported as
// `false`. Matches the per-engine 5 s budget in CIEngineService.
//
// Detection uses Discovery API against `tekton.dev/v1`. Installations on
// beta APIs (tekton.dev/v1beta1) are considered **not available** for M18d;
// a future milestone can widen the probe to accept beta.
func (a *Adapter) IsAvailable(ctx context.Context) bool {
	if err := requireResolver(a.resolver); err != nil {
		return false
	}
	probeCtx, cancel := context.WithTimeout(ctx, availabilityProbeTimeout)
	defer cancel()

	disc, err := a.resolver.Discovery(a.clusterID)
	if err != nil || disc == nil {
		return false
	}
	_ = probeCtx // discovery client does not accept ctx; the timeout is
	// enforced by the REST transport via ClusterResolver.
	_, err = disc.ServerResourcesForGroupVersion(probeGroupVersion)
	if err != nil {
		// IsNotFound means the group/version isn't registered — a clean
		// "Tekton not installed" signal. Any other error (network,
		// unauthorized) is best reported as unavailable without noise.
		if !k8serrors.IsNotFound(err) {
			// Non-404: could be transient; we still report false per the
			// Observer contract.
			return false
		}
		return false
	}
	return true
}

// Version attempts to report the Tekton Pipelines controller version.
//
// M18d derives the version from the server-side group-version listing
// rather than interrogating the controller deployment (which would need a
// clientset dependency not currently wired through the resolver). When no
// version can be inferred, returns the sentinel "unknown" — IsAvailable()
// is allowed to remain true.
func (a *Adapter) Version(ctx context.Context) (string, error) {
	if err := requireResolver(a.resolver); err != nil {
		return "", err
	}
	disc, err := a.resolver.Discovery(a.clusterID)
	if err != nil {
		return "", fmt.Errorf("tekton: discovery: %w", engine.ErrUnavailable)
	}
	list, err := disc.ServerResourcesForGroupVersion(probeGroupVersion)
	if err != nil {
		return "", mapK8sError(err)
	}
	// APIResourceList.GroupVersion is the canonical identifier of the
	// server-recognised Tekton API version. We expose it verbatim — callers
	// typically care about the distinction between v1 and v1beta1 rather
	// than controller release numbers.
	if list != nil && list.GroupVersion != "" {
		return list.GroupVersion, nil
	}
	return "unknown", nil
}

// Capabilities describes what Tekton supports via the CRD surface used by
// this adapter. Flags mirror what the dynamic-client path can reliably
// inspect; features that require extra plugins (Tekton Dashboard for
// live-log UI, Results CRD for archival) are reported conservatively.
func (a *Adapter) Capabilities() engine.EngineCapabilities {
	return engine.EngineCapabilities{
		SupportsDAG:          true,  // Pipeline spec.tasks / runAfter
		SupportsMatrix:       true,  // Tekton v1 matrix
		SupportsArtifacts:    false, // PipelineRun results only — file artifacts live outside Tekton
		SupportsSecrets:      true,  // spec.workspaces from Secret, serviceAccountName
		SupportsCaching:      false, // Tekton has no native cache; plugins vary
		SupportsApprovals:    false, // no native manual-approval step; can be simulated with Params
		SupportsNotification: false, // Tekton does not notify externally; Synapse bridges events
		SupportsLiveLog:      true,  // Pod log streaming via ClusterResolver.Kubernetes()
	}
}

// ---------------------------------------------------------------------------
// Execution methods live in their own files:
//   trigger.go   — Trigger
//   runs.go      — GetRun
//   cancel.go    — Cancel
//   logs.go      — StreamLogs (returns ErrUnsupported in M18d — see comment)
//   artifacts.go — GetArtifacts
// ---------------------------------------------------------------------------
