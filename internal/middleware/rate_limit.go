package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// LoginRateLimit returns a Gin middleware that enforces per-IP and per-username
// rate limiting on the login endpoint using the provided RateLimiter backend.
//
// Rules:
//   - More than 5 failed attempts within a 1-minute window → 429 Too Many Requests
//   - Account locked for 15 minutes after exceeding the threshold
//
// On successful login the limiter resets counters for both IP and username.
func LoginRateLimit(limiter RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		// Peek at the username in the JSON body without consuming the stream.
		var username string
		if c.Request.Method == http.MethodPost && c.Request.Body != nil {
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

		if limiter.IsLocked(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "登入嘗試次數過多，請 15 分鐘後再試",
			})
			return
		}
		if username != "" && limiter.IsLocked(username) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "帳號已被暫時鎖定，請 15 分鐘後再試",
			})
			return
		}

		// Store keys so other middleware/handlers can reference them if needed.
		c.Set("rate_limit_ip", ip)
		c.Set("rate_limit_username", username)

		c.Next()

		// Record failure on 401/403; reset on 200.
		status := c.Writer.Status()
		switch {
		case status == http.StatusUnauthorized || status == http.StatusForbidden:
			limiter.RecordFailure(ip)
			if username != "" {
				limiter.RecordFailure(username)
			}
		case status == http.StatusOK:
			limiter.Reset(ip)
			if username != "" {
				limiter.Reset(username)
			}
		}
	}
}
