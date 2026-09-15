package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// StructuredLogger returns a Gin middleware that logs incoming HTTP requests using slog.
func StructuredLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		// Process request
		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if rawQuery != "" {
			path = path + "?" + rawQuery
		}

		attrs := []slog.Attr{
			slog.Int("status", statusCode),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("ip", clientIP),
			slog.Duration("latency", latency),
		}

		if len(c.Errors) > 0 {
			slog.Error("HTTP request with errors",
				"status", statusCode,
				"method", method,
				"path", path,
				"errors", c.Errors.String(),
			)
			return
		}

		if statusCode >= 500 {
			slog.LogAttrs(c.Request.Context(), slog.LevelError, "HTTP Server Error", attrs...)
		} else if statusCode >= 400 {
			slog.LogAttrs(c.Request.Context(), slog.LevelWarn, "HTTP Client Warning", attrs...)
		} else {
			slog.LogAttrs(c.Request.Context(), slog.LevelInfo, "HTTP Request Handled", attrs...)
		}
	}
}
