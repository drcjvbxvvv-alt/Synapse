package middleware

import (
	"sync"
	"time"
)

// MemoryRateLimiter is the default single-process implementation.
// It is NOT safe for use across multiple pods — use RedisRateLimiter instead.
type MemoryRateLimiter struct {
	mu      sync.Mutex
	records map[string]*memAttempt
}

type memAttempt struct {
	count    int
	lockedAt time.Time
	windowAt time.Time
}

// NewMemoryRateLimiter returns an initialised MemoryRateLimiter.
func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{
		records: make(map[string]*memAttempt),
	}
}

// Compile-time interface check.
var _ RateLimiter = (*MemoryRateLimiter)(nil)

func (m *MemoryRateLimiter) IsLocked(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[key]
	if !ok {
		return false
	}
	if rec.count >= loginMaxAttempts {
		if time.Since(rec.lockedAt) < loginLockDuration {
			return true
		}
		// Lock expired — reset.
		delete(m.records, key)
	}
	return false
}

func (m *MemoryRateLimiter) RecordFailure(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[key]
	if !ok {
		rec = &memAttempt{windowAt: time.Now()}
		m.records[key] = rec
	}
	// Reset window if older than loginWindow.
	if time.Since(rec.windowAt) > loginWindow {
		rec.count = 0
		rec.windowAt = time.Now()
	}
	rec.count++
	if rec.count >= loginMaxAttempts {
		rec.lockedAt = time.Now()
	}
}

func (m *MemoryRateLimiter) Reset(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.records, key)
}
