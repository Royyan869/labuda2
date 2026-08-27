package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
)

// LoggerMiddleware returns a gin middleware for logging HTTP requests
func LoggerMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log after processing
		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		// Get request ID if available
		requestID := GetRequestID(c)

		// Build log fields
		fields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("client_ip", clientIP),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
		}

		if query != "" {
			fields = append(fields, zap.String("query", query))
		}

		// Log based on status code
		switch {
		case status >= 500:
			log.Error("HTTP request", fields...)
		case status >= 400:
			log.Warn("HTTP request", fields...)
		default:
			log.Info("HTTP request", fields...)
		}
	}
}


