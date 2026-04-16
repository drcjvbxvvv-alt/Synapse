package argo

import (
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// mapK8sError maps k8s client-go errors to engine sentinels. Mirrors the
// tekton adapter's implementation; kept separate so neither package
// becomes the "shared util" for the other.
func mapK8sError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case k8serrors.IsNotFound(err):
		return fmt.Errorf("argo: %w: %w", err, engine.ErrNotFound)
	case k8serrors.IsUnauthorized(err):
		return fmt.Errorf("argo: %w: %w", err, engine.ErrUnauthorized)
	case k8serrors.IsForbidden(err):
		return fmt.Errorf("argo: %w: %w", err, engine.ErrUnauthorized)
	case k8serrors.IsBadRequest(err), k8serrors.IsInvalid(err):
		return fmt.Errorf("argo: %w: %w", err, engine.ErrInvalidInput)
	case k8serrors.IsConflict(err), k8serrors.IsAlreadyExists(err):
		return fmt.Errorf("argo: %w: %w", err, engine.ErrInvalidInput)
	case k8serrors.IsServerTimeout(err), k8serrors.IsServiceUnavailable(err),
		k8serrors.IsInternalError(err), k8serrors.IsTooManyRequests(err):
		return fmt.Errorf("argo: %w: %w", err, engine.ErrUnavailable)
	}
	return fmt.Errorf("argo: %w: %w", err, engine.ErrUnavailable)
}
