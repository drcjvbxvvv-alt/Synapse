package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

// RequestID injects a unique request identifier into every HTTP request.
// If the client already sends an X-Request-ID header its value is reused,
// otherwise a new UUID v4 is generated.  The ID is exposed via:
//   - Response header X-Request-ID
//   - Gin context key "request_id" (accessible with c.GetString("request_id"))
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader(RequestIDHeader)
		if reqID == "" {
			reqID = uuid.New().String()
		}
		c.Set("request_id", reqID)
		c.Header(RequestIDHeader, reqID)
		c.Next()
	}
}
