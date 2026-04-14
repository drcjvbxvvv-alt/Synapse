package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/pkg/logger"
)

// apiWindowEntry tracks request count in the current sliding window.
type apiWindowEntry struct {
	mu          sync.Mutex
	count       int
	windowStart time.Time
}

// apiRateLimitStore is a process-local store for API rate limit counters.
// For multi-replica deployments, replace with a Redis-backed implementation.
var apiRateLimitStore sync.Map

// APIRateLimit returns a middleware that limits requests to maxPerMin per
// user (authenticated by JWT user_id) or per IP (unauthenticated requests)
// within a sliding 1-minute window. Returns 429 when the limit is exceeded.
//
// name differentiates independent rate limit buckets (e.g. "api", "ai_chat")
// so multiple middlewares can be stacked without sharing counters.
func APIRateLimit(name string, maxPerMin int) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := name + ":" + apiRateLimitKey(c)
		now := time.Now()

		val, _ := apiRateLimitStore.LoadOrStore(key, &apiWindowEntry{windowStart: now})
		entry := val.(*apiWindowEntry)

		entry.mu.Lock()
		if now.Sub(entry.windowStart) >= time.Minute {
			entry.count = 0
			entry.windowStart = now
		}
		entry.count++
		count := entry.count
		entry.mu.Unlock()

		if count > maxPerMin {
			logger.Warn("API rate limit exceeded",
				"bucket", name,
				"key", key,
				"count", count,
				"limit", maxPerMin,
			)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": fmt.Sprintf("請求過於頻繁，請稍後再試（上限 %d 次/分鐘）", maxPerMin),
			})
			return
		}
		c.Next()
	}
}

// apiRateLimitKey returns "user:{id}" for authenticated requests, "ip:{addr}" otherwise.
func apiRateLimitKey(c *gin.Context) string {
	if uid, exists := c.Get("user_id"); exists {
		return fmt.Sprintf("user:%v", uid)
	}
	return "ip:" + c.ClientIP()
}
