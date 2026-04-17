package services

// k8s_retry.go — Exponential backoff retry for transient K8s API errors.
//
// Only operations in the pipeline path (Job/Secret create) use this.
// Handler-level K8s calls remain direct (user-facing latency must stay low).

import (
	"context"
	"errors"
	"net"
	"time"

	backoff "github.com/cenkalti/backoff/v5"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/shaia/Synapse/pkg/logger"
)

const (
	k8sRetryMaxTries        = 4               // initial attempt + 3 retries
	k8sRetryMaxElapsed      = 30 * time.Second // hard cap on total retry window
	k8sRetryInitialInterval = 200 * time.Millisecond
	k8sRetryMaxInterval     = 5 * time.Second
)

// isRetryableK8sError returns true for transient errors that are safe to retry.
// Permanent errors (auth, validation, conflict, not-found) return false.
func isRetryableK8sError(err error) bool {
	if err == nil {
		return false
	}
	if k8serrors.IsServerTimeout(err) ||
		k8serrors.IsTooManyRequests(err) ||
		k8serrors.IsServiceUnavailable(err) ||
		k8serrors.IsInternalError(err) ||
		k8serrors.IsTimeout(err) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

// k8sRetry executes operation with exponential backoff, retrying only on
// transient K8s API errors. Non-retryable errors are wrapped as
// backoff.Permanent so the caller gets the original error immediately.
func k8sRetry[T any](ctx context.Context, opName string, operation func() (T, error)) (T, error) {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = k8sRetryInitialInterval
	bo.MaxInterval = k8sRetryMaxInterval
	bo.Multiplier = 2.0

	return backoff.Retry(ctx,
		func() (T, error) {
			result, err := operation()
			if err == nil {
				return result, nil
			}
			if !isRetryableK8sError(err) {
				var zero T
				return zero, backoff.Permanent(err)
			}
			return result, err
		},
		backoff.WithBackOff(bo),
		backoff.WithMaxTries(k8sRetryMaxTries),
		backoff.WithMaxElapsedTime(k8sRetryMaxElapsed),
		backoff.WithNotify(func(err error, wait time.Duration) {
			logger.Warn("k8s API transient error, retrying",
				"operation", opName,
				"error", err,
				"retry_after", wait,
			)
		}),
	)
}
