package argo

import (
	"context"
	"fmt"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Adapter implements engine.CIEngineAdapter for Argo Workflows.
//
// Each adapter instance is bound to one CIEngineConfig; the ClusterResolver
// is injected at construction time so dynamic/discovery clients can be
// obtained lazily per request without the adapter owning long-lived client
// handles (matching the Tekton adapter's design).
type Adapter struct {
	clusterID uint
	extra     *ExtraConfig
	resolver  ClusterResolver
	name      string
}

// Compile-time assertion.
var _ engine.CIEngineAdapter = (*Adapter)(nil)

// availabilityProbeTimeout bounds IsAvailable() per-engine.
const availabilityProbeTimeout = 5 * time.Second

// NewAdapter constructs an Adapter from a stored CIEngineConfig.
func NewAdapter(cfg *models.CIEngineConfig, resolver ClusterResolver) (*Adapter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("argo: nil CIEngineConfig: %w", engine.ErrInvalidInput)
	}
	if cfg.ClusterID == nil || *cfg.ClusterID == 0 {
		return nil, fmt.Errorf("argo: cluster_id is required on CIEngineConfig: %w", engine.ErrInvalidInput)
	}
	extra, err := parseExtra(cfg.ExtraJSON)
	if err != nil {
		return nil, err
	}
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

// Type returns engine.EngineArgo.
func (a *Adapter) Type() engine.EngineType { return engine.EngineArgo }

// IsAvailable probes the cluster for argoproj.io/v1alpha1 CRDs.
// Contract: never returns an error; reports availability as false on any
// failure path (CLAUDE §8).
func (a *Adapter) IsAvailable(ctx context.Context) bool {
	if err := requireResolver(a.resolver); err != nil {
		return false
	}
	probeCtx, cancel := context.WithTimeout(ctx, availabilityProbeTimeout)
	defer cancel()
	_ = probeCtx

	disc, err := a.resolver.Discovery(a.clusterID)
	if err != nil || disc == nil {
		return false
	}
	_, err = disc.ServerResourcesForGroupVersion(probeGroupVersion)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return false
		}
		return false
	}
	return true
}

// Version returns the GroupVersion string Argo is registered under. Argo
// itself does not expose controller versions via Discovery, so this serves
// as a "which API surface we're using" marker rather than a release number.
func (a *Adapter) Version(ctx context.Context) (string, error) {
	if err := requireResolver(a.resolver); err != nil {
		return "", err
	}
	disc, err := a.resolver.Discovery(a.clusterID)
	if err != nil {
		return "", fmt.Errorf("argo: discovery: %w", engine.ErrUnavailable)
	}
	list, err := disc.ServerResourcesForGroupVersion(probeGroupVersion)
	if err != nil {
		return "", mapK8sError(err)
	}
	if list != nil && list.GroupVersion != "" {
		return list.GroupVersion, nil
	}
	return "unknown", nil
}

// Capabilities describes what the adapter exposes. Conservative flags —
// Argo supports much more (DAG with dependencies, suspend/resume,
// parallelism), but the adapter's contract is limited to CRUD for now.
func (a *Adapter) Capabilities() engine.EngineCapabilities {
	return engine.EngineCapabilities{
		SupportsDAG:          true,  // Workflow DAG + dependencies
		SupportsMatrix:       false, // requires "withItems"/"withParam" — not surfaced
		SupportsArtifacts:    true,  // outputs.artifacts[]
		SupportsSecrets:      true,  // spec.volumes secret + withCredentials
		SupportsCaching:      false, // Argo has no native cache; "memoize" plugin varies
		SupportsApprovals:    true,  // spec.suspend + resume
		SupportsNotification: false, // relies on Argo Events / external; Synapse bridges
		SupportsLiveLog:      true,  // Pod log streaming via ClusterResolver.Kubernetes()
	}
}

// ---------------------------------------------------------------------------
// Execution methods live in their own files:
//   trigger.go   — Trigger
//   runs.go      — GetRun
//   cancel.go    — Cancel
//   logs.go      — StreamLogs (ErrUnsupported in M18e)
//   artifacts.go — GetArtifacts
//
// Stage 2-3 stubs follow so the compile-time interface assertion holds
// throughout the staged rollout.
// ---------------------------------------------------------------------------

// Trigger / GetRun / Cancel / StreamLogs / GetArtifacts all live in their
// own files (trigger.go, runs.go, cancel.go, logs.go, artifacts.go).
