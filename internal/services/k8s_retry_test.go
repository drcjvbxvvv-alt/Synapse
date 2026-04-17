package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ── isRetryableK8sError ──────────────────────────────────────────────────────

func TestIsRetryableK8sError_Nil(t *testing.T) {
	assert.False(t, isRetryableK8sError(nil))
}

func TestIsRetryableK8sError_ServerTimeout(t *testing.T) {
	err := k8serrors.NewServerTimeout(schema.GroupResource{}, "list", 5)
	assert.True(t, isRetryableK8sError(err))
}

func TestIsRetryableK8sError_TooManyRequests(t *testing.T) {
	err := k8serrors.NewTooManyRequestsError("throttled")
	assert.True(t, isRetryableK8sError(err))
}

func TestIsRetryableK8sError_ServiceUnavailable(t *testing.T) {
	err := k8serrors.NewServiceUnavailable("unavailable")
	assert.True(t, isRetryableK8sError(err))
}

func TestIsRetryableK8sError_NetworkTimeout(t *testing.T) {
	err := &fakeNetError{timeout: true}
	assert.True(t, isRetryableK8sError(err))
}

func TestIsRetryableK8sError_AlreadyExists_Permanent(t *testing.T) {
	err := k8serrors.NewAlreadyExists(schema.GroupResource{}, "foo")
	assert.False(t, isRetryableK8sError(err))
}

func TestIsRetryableK8sError_NotFound_Permanent(t *testing.T) {
	err := k8serrors.NewNotFound(schema.GroupResource{}, "foo")
	assert.False(t, isRetryableK8sError(err))
}

func TestIsRetryableK8sError_Forbidden_Permanent(t *testing.T) {
	err := k8serrors.NewForbidden(schema.GroupResource{}, "foo", fmt.Errorf("no access"))
	assert.False(t, isRetryableK8sError(err))
}

func TestIsRetryableK8sError_Unauthorized_Permanent(t *testing.T) {
	err := k8serrors.NewUnauthorized("bad token")
	assert.False(t, isRetryableK8sError(err))
}

func TestIsRetryableK8sError_PlainError_Permanent(t *testing.T) {
	assert.False(t, isRetryableK8sError(errors.New("some non-k8s error")))
}

// ── k8sRetry ─────────────────────────────────────────────────────────────────

func TestK8sRetry_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	result, err := k8sRetry(context.Background(), "test-op", func() (string, error) {
		calls++
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
	assert.Equal(t, 1, calls)
}

func TestK8sRetry_RetriesOnTransientError(t *testing.T) {
	calls := 0
	transient := k8serrors.NewServerTimeout(schema.GroupResource{}, "create", 1)

	result, err := k8sRetry(context.Background(), "test-op", func() (string, error) {
		calls++
		if calls < 3 {
			return "", transient
		}
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
	assert.Equal(t, 3, calls)
}

func TestK8sRetry_StopsOnPermanentError(t *testing.T) {
	calls := 0
	permanent := k8serrors.NewAlreadyExists(schema.GroupResource{Resource: "jobs"}, "my-job")

	_, err := k8sRetry(context.Background(), "test-op", func() (string, error) {
		calls++
		return "", permanent
	})
	require.Error(t, err)
	assert.Equal(t, 1, calls, "must not retry permanent errors")
	assert.True(t, k8serrors.IsAlreadyExists(err), "original error must be unwrapped")
}

func TestK8sRetry_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	calls := 0
	transient := k8serrors.NewServiceUnavailable("down")

	_, err := k8sRetry(ctx, "test-op", func() (string, error) {
		calls++
		return "", transient
	})
	require.Error(t, err)
	// At most 1 attempt before context check aborts
	assert.LessOrEqual(t, calls, 2)
}

func TestK8sRetry_ExhaustsMaxTries(t *testing.T) {
	transient := k8serrors.NewTooManyRequestsError("throttled")
	calls := 0

	_, err := k8sRetry(context.Background(), "test-op", func() (string, error) {
		calls++
		return "", transient
	})
	require.Error(t, err)
	assert.Equal(t, int(k8sRetryMaxTries), calls,
		"should attempt exactly k8sRetryMaxTries times")
}

func TestK8sRetry_MaxElapsedTime(t *testing.T) {
	// Use a very short elapsed time to trigger the limit quickly.
	// We can't easily override the const, so just verify the behaviour
	// is bounded: with maxTries=4 and tiny context timeout, stops fast.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	transient := k8serrors.NewServerTimeout(schema.GroupResource{}, "list", 1)
	calls := 0

	_, err := k8sRetry(ctx, "test-op", func() (string, error) {
		calls++
		return "", transient
	})
	require.Error(t, err)
	// Should not run more than maxTries regardless of time
	assert.LessOrEqual(t, calls, int(k8sRetryMaxTries))
}

// ── helpers ──────────────────────────────────────────────────────────────────

type fakeNetError struct{ timeout bool }

func (e *fakeNetError) Error() string   { return "fake net error" }
func (e *fakeNetError) Timeout() bool   { return e.timeout }
func (e *fakeNetError) Temporary() bool { return false }

var _ net.Error = (*fakeNetError)(nil)
