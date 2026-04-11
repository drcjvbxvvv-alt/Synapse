package middleware

import "time"

// RateLimiter abstracts login-attempt tracking so the implementation
// can be swapped between a single-process in-memory store and a
// cross-pod Redis store without changing the middleware logic.
//
// Fail-open policy: all implementations MUST allow the request through
// when the backend is unavailable, and log a warning instead.
type RateLimiter interface {
	// IsLocked returns true when the key is currently in lock-out.
	IsLocked(key string) bool
	// RecordFailure increments the failure counter for key and
	// sets the lock when the threshold is exceeded.
	RecordFailure(key string)
	// Reset clears all state for key (called on successful login).
	Reset(key string)
}

// Shared constants used by both implementations.
const (
	loginMaxAttempts  = 5
	loginWindow       = time.Minute
	loginLockDuration = 15 * time.Minute
)
