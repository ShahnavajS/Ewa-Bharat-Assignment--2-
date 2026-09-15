package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a middleware that handles Cross-Origin Resource Sharing (CORS).
// It verifies the Origin header against the configured allowed origins list.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	// Fast lookup map for configured origins
	originMap := make(map[string]bool, len(allowedOrigins))
	hasWildcard := false
	for _, o := range allowedOrigins {
		o = strings.TrimSuffix(strings.TrimSpace(o), "/")
		originMap[o] = true
		if o == "*" {
			hasWildcard = true
		}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSuffix(c.Request.Header.Get("Origin"), "/")

		// If the request origin is in our allowed list, set the header explicitly
		if origin != "" && (hasWildcard || originMap[origin]) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		}

		// Handle preflight requests
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
