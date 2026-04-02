package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// loginAttempt tracks failed login attempts for a given key (IP or username).
type loginAttempt struct {
	count     int
	lockedAt  time.Time
	windowAt  time.Time // start of the current 1-minute sliding window
}

var (
	loginMu      sync.Mutex
	loginRecords = make(map[string]*loginAttempt)
)

const (
	loginMaxAttempts  = 5              // max failures before lockout
	loginWindow       = time.Minute    // sliding window for attempt counting
	loginLockDuration = 15 * time.Minute
)

// isLocked returns true when the key is currently locked out.
func isLocked(key string) bool {
	loginMu.Lock()
	defer loginMu.Unlock()
	rec, ok := loginRecords[key]
	if !ok {
		return false
	}
	if rec.count >= loginMaxAttempts {
		if time.Since(rec.lockedAt) < loginLockDuration {
			return true
		}
		// Lock expired — reset
		delete(loginRecords, key)
	}
	return false
}

// recordFailure increments the failure counter for key.
func recordFailure(key string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	rec, ok := loginRecords[key]
	if !ok {
		rec = &loginAttempt{windowAt: time.Now()}
		loginRecords[key] = rec
	}
	// Reset window if older than loginWindow
	if time.Since(rec.windowAt) > loginWindow {
		rec.count = 0
		rec.windowAt = time.Now()
	}
	rec.count++
	if rec.count >= loginMaxAttempts {
		rec.lockedAt = time.Now()
	}
}

// ResetLoginAttempts clears the failure record for a key (called on successful login).
func ResetLoginAttempts(key string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginRecords, key)
}

// LoginRateLimit is a Gin middleware that enforces per-IP and per-username
// rate limiting on the login endpoint.
//
// Rules:
//   - More than 5 failed attempts within a 1-minute window → 429 Too Many Requests
//   - Account locked for 15 minutes after exceeding the threshold
//
// Call ResetLoginAttempts(ip) and ResetLoginAttempts(username) on successful login.
func LoginRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		// Parse username from JSON body without consuming the body stream.
		// We peek only when the content-type is JSON.
		var username string
		if c.Request.Method == http.MethodPost && c.Request.Body != nil {
			// Peek at username without consuming the body — read, unmarshal, then restore.
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				var peek struct {
					Username string `json:"username"`
				}
				_ = json.Unmarshal(bodyBytes, &peek)
				username = peek.Username
			}
		}

		if isLocked(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "登入嘗試次數過多，請 15 分鐘後再試",
			})
			return
		}
		if username != "" && isLocked(username) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "帳號已被暫時鎖定，請 15 分鐘後再試",
			})
			return
		}

		// Store keys in context so the handler can call RecordLoginFailure / Reset.
		c.Set("rate_limit_ip", ip)
		c.Set("rate_limit_username", username)

		c.Next()

		// If the handler responded with 401/403, record a failure.
		status := c.Writer.Status()
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			recordFailure(ip)
			if username != "" {
				recordFailure(username)
			}
		} else if status == http.StatusOK {
			// Successful login — reset counters
			ResetLoginAttempts(ip)
			if username != "" {
				ResetLoginAttempts(username)
			}
		}
	}
}
