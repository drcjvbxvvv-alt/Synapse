package tekton

import (
	"errors"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// mapK8sError converts a Kubernetes-client error into one of the engine
// package's sentinel errors. Adapter methods wrap all dynamic.Interface /
// discovery.DiscoveryInterface calls through this helper so that handler
// code can errors.Is() uniformly.
//
// Never call this with a nil err — it's meant for the error path only.
func mapK8sError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case k8serrors.IsNotFound(err):
		return fmt.Errorf("tekton: %w: %w", err, engine.ErrNotFound)
	case k8serrors.IsUnauthorized(err):
		return fmt.Errorf("tekton: %w: %w", err, engine.ErrUnauthorized)
	case k8serrors.IsForbidden(err):
		return fmt.Errorf("tekton: %w: %w", err, engine.ErrUnauthorized)
	case k8serrors.IsBadRequest(err), k8serrors.IsInvalid(err):
		return fmt.Errorf("tekton: %w: %w", err, engine.ErrInvalidInput)
	case k8serrors.IsConflict(err), k8serrors.IsAlreadyExists(err):
		return fmt.Errorf("tekton: %w: %w", err, engine.ErrInvalidInput)
	case k8serrors.IsServerTimeout(err), k8serrors.IsServiceUnavailable(err),
		k8serrors.IsInternalError(err), k8serrors.IsTooManyRequests(err):
		return fmt.Errorf("tekton: %w: %w", err, engine.ErrUnavailable)
	}
	// Transient network / dial failures that don't carry a k8s Status — fall
	// through to ErrUnavailable so callers can retry uniformly.
	return fmt.Errorf("tekton: %w: %w", err, engine.ErrUnavailable)
}

// Sanity check that k8serrors is still importable with the expected
// predicates; keeps the error path obvious to readers new to client-go.
var _ = errors.Is
