package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDContextKey is the key for request ID in gin context
	RequestIDContextKey = "request_id"
	// TraceIDContextKey is the key for trace ID in gin context
	TraceIDContextKey = "trace_id"
	// RequestIDHeader is the header name for request ID
	RequestIDHeader = "X-Request-ID"
	// TraceIDHeader is the header name for trace ID
	TraceIDHeader = "X-Trace-ID"
)

// RequestIDMiddleware generates/extracts request ID and trace ID for each request
//
// P9 Observability & Incident Readiness:
// - Generates UUID v4 for request_id if not present in header
// - Generates UUID v4 for trace_id if not present in header
// - Stores both in gin context for downstream use
// - Returns request_id and trace_id in response headers
//
// Request Flow:
// 1. Check X-Request-ID header - use if present, otherwise generate new UUID
// 2. Check X-Trace-ID header - use if present, otherwise generate new UUID
// 3. Store both in context for handlers and logging
// 4. Set headers in response for client-side correlation
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract or generate Request ID
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Extract or generate Trace ID
		// Trace ID spans multiple requests (e.g., microservice calls)
		traceID := c.GetHeader(TraceIDHeader)
		if traceID == "" {
			// If no trace ID, use request ID as trace ID (root request)
			traceID = requestID
		}

		// Store in context for downstream use
		c.Set(RequestIDContextKey, requestID)
		c.Set(TraceIDContextKey, traceID)

		// Set response headers for client-side correlation
		c.Header(RequestIDHeader, requestID)
		c.Header(TraceIDHeader, traceID)

		c.Next()
	}
}

// GetRequestID extracts request ID from gin context
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get(RequestIDContextKey); exists {
		if requestID, ok := id.(string); ok {
			return requestID
		}
	}
	return ""
}

// GetTraceID extracts trace ID from gin context
func GetTraceID(c *gin.Context) string {
	if id, exists := c.Get(TraceIDContextKey); exists {
		if traceID, ok := id.(string); ok {
			return traceID
		}
	}
	return ""
}

// GetRequestTraceIDs returns both request ID and trace ID
func GetRequestTraceIDs(c *gin.Context) (requestID, traceID string) {
	return GetRequestID(c), GetTraceID(c)
}


