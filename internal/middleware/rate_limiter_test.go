package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── MemoryRateLimiter tests ───────────────────────────────────────────────

func TestMemoryRateLimiter_NotLockedInitially(t *testing.T) {
	m := NewMemoryRateLimiter()
	assert.False(t, m.IsLocked("192.168.1.1"))
}

func TestMemoryRateLimiter_LocksAfterThreshold(t *testing.T) {
	m := NewMemoryRateLimiter()
	key := "user@example.com"

	for i := 0; i < loginMaxAttempts; i++ {
		assert.False(t, m.IsLocked(key), "should not be locked before threshold")
		m.RecordFailure(key)
	}
	assert.True(t, m.IsLocked(key), "should be locked after threshold")
}

func TestMemoryRateLimiter_ResetClearsLock(t *testing.T) {
	m := NewMemoryRateLimiter()
	key := "10.0.0.1"

	for i := 0; i < loginMaxAttempts; i++ {
		m.RecordFailure(key)
	}
	require.True(t, m.IsLocked(key))

	m.Reset(key)
	assert.False(t, m.IsLocked(key))
}

func TestMemoryRateLimiter_LockExpires(t *testing.T) {
	// Patch constants for test speed — we restore them after.
	origDuration := loginLockDuration
	// Cannot reassign const; we test the expiry logic by manipulating time in
	// a white-box fashion: insert a record with a lockedAt in the past.
	_ = origDuration // keep reference to avoid unused import warnings

	m := NewMemoryRateLimiter()
	key := "expiry-test"

	// Manually insert an expired lock record.
	m.mu.Lock()
	m.records[key] = &memAttempt{
		count:    loginMaxAttempts,
		lockedAt: time.Now().Add(-loginLockDuration - time.Second),
		windowAt: time.Now().Add(-loginLockDuration - time.Second),
	}
	m.mu.Unlock()

	// IsLocked should clean up the expired record and return false.
	assert.False(t, m.IsLocked(key), "expired lock should not block")

	m.mu.Lock()
	_, exists := m.records[key]
	m.mu.Unlock()
	assert.False(t, exists, "expired record should be cleaned up")
}

func TestMemoryRateLimiter_WindowReset(t *testing.T) {
	m := NewMemoryRateLimiter()
	key := "window-reset"

	// Record loginMaxAttempts-1 failures.
	for i := 0; i < loginMaxAttempts-1; i++ {
		m.RecordFailure(key)
	}
	assert.False(t, m.IsLocked(key))

	// Simulate window expiry by backdating windowAt.
	m.mu.Lock()
	m.records[key].windowAt = time.Now().Add(-loginWindow - time.Second)
	m.mu.Unlock()

	// Another failure should reset the window counter (count=1, not locked).
	m.RecordFailure(key)
	assert.False(t, m.IsLocked(key))
}

// ─── LoginRateLimit middleware integration tests ───────────────────────────

func TestLoginRateLimit_BlocksLockedIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := NewMemoryRateLimiter()
	// Pre-lock the IP.
	for i := 0; i < loginMaxAttempts; i++ {
		m.RecordFailure("192.0.2.1")
	}

	w := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(w)
	engine.POST("/login", LoginRateLimit(m), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"username":"alice","password":"pw"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.1:1234"
	ctx.Request = req

	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestLoginRateLimit_RecordsFailureOn401(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := NewMemoryRateLimiter()

	engine := gin.New()
	engine.POST("/login", LoginRateLimit(m), func(c *gin.Context) {
		c.Status(http.StatusUnauthorized)
	})

	for i := 0; i < loginMaxAttempts; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login",
			strings.NewReader(`{"username":"bob","password":"bad"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.0.0.2:9999"
		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}

	// Next attempt should be rate-limited.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"username":"bob","password":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.2:9999"
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestLoginRateLimit_ResetsOnSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := NewMemoryRateLimiter()

	engine := gin.New()
	engine.POST("/login", LoginRateLimit(m), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Record failures up to threshold - 1.
	for i := 0; i < loginMaxAttempts-1; i++ {
		m.RecordFailure("10.1.1.1")
	}

	// Successful login should reset.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"username":"carol","password":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.1.1.1:1111"
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	assert.False(t, m.IsLocked("10.1.1.1"))
	assert.False(t, m.IsLocked("carol"))
}
