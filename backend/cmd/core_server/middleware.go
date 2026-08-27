package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
)

// LoggerMiddleware logs all HTTP requests
func LoggerMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		// Get tracing IDs from context
		requestID := middleware.GetRequestID(c)
		traceID := middleware.GetTraceID(c)

		// Build log fields with tracing info
		fields := []zap.Field{
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("duration", duration),
			zap.String("client_ip", c.ClientIP()),
		}

		// Add tracing fields if available
		if requestID != "" {
			fields = append(fields, zap.String("request_id", requestID))
		}
		if traceID != "" {
			fields = append(fields, zap.String("trace_id", traceID))
		}

		// Log request with tracing info
		log.Info("HTTP request", fields...)
	}
}

// CORSMiddleware handles CORS with proper origin validation
func CORSMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check against allowed origins from config
		allowed := false
		for _, allowedOrigin := range cfg.CORS.AllowedOrigins {
			if allowedOrigin == "*" {
				// Only allow wildcard in non-production environments
				if cfg.Server.Env != "production" {
					c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
					allowed = true
					break
				}
				// In production, wildcard in config means we need explicit origin match
			} else if allowedOrigin == origin {
				// Exact match - set the specific origin
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				allowed = true
				break
			}
		}

		// If origin not allowed in production, reject the request
		if !allowed && cfg.Server.Env == "production" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Origin not allowed",
			})
			return
		}

		// If not allowed but in development, allow anyway (fallback for local dev)
		if !allowed && cfg.Server.Env != "production" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}

		// Use config for other CORS headers
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		// Join allowed headers from config
		if len(cfg.CORS.AllowedHeaders) > 0 {
			headers := ""
			for i, h := range cfg.CORS.AllowedHeaders {
				if i > 0 {
					headers += ", "
				}
				headers += h
			}
			c.Writer.Header().Set("Access-Control-Allow-Headers", headers)
		}

		// Join allowed methods from config
		if len(cfg.CORS.AllowedMethods) > 0 {
			methods := ""
			for i, m := range cfg.CORS.AllowedMethods {
				if i > 0 {
					methods += ", "
				}
				methods += m
			}
			c.Writer.Header().Set("Access-Control-Allow-Methods", methods)
		}

		// Set max age from config
		if cfg.CORS.MaxAge > 0 {
			c.Writer.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.CORS.MaxAge))
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
