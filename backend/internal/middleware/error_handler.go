package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/platform/response"
	"go.uber.org/zap"
)

// ErrorHandler is a global error recovery middleware that:
// 1. Recovers from panics
// 2. Catches any unhandled errors
// 3. Maps them to appropriate HTTP responses
// 4. Logs errors appropriately
//
// This ensures no raw error messages leak to clients and all errors
// are handled consistently across the API.
func ErrorHandler(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Defer panic recovery
		defer response.RecoverFromPanic(c, log)

		// Process request
		c.Next()

		// Check for errors set during handler execution
		// Handlers can use c.Error() to pass errors to this middleware
		if len(c.Errors) > 0 {
			// Get the last error (most recent)
			err := c.Errors.Last().Err

			// If no response has been written yet, send error response
			if !c.Writer.Written() {
				response.RespondWithError(c, log, err)
			}
			c.Abort()
		}
	}
}

// TypedErrorHandler allows handlers to return errors instead of manually handling them.
// Usage:
//
//	r.GET("/resource", TypedErrorHandler(log, func(c *gin.Context) error {
//	    if err := doSomething(); err != nil {
//	        return err
//	    }
//	    response.Success(c, data)
//	    return nil
//	}))
func TypedErrorHandler(log *zap.Logger, handler func(*gin.Context) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Recover from panics
		defer response.RecoverFromPanic(c, log)

		// Execute handler
		if err := handler(c); err != nil {
			// If handler returned an error and no response written yet
			if !c.Writer.Written() {
				response.RespondWithError(c, log, err)
			}
			c.Abort()
		}
	}
}

// ValidateRequest is a helper for validating JSON requests
// Returns true if validation passed, false otherwise
func ValidateRequest(c *gin.Context, log *zap.Logger, req interface{}) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "INVALID_JSON", "Invalid request body", gin.H{
			"details": err.Error(),
		})
		return false
	}
	return true
}

// ValidateQuery is a helper for validating query parameters
// Returns true if validation passed, false otherwise
func ValidateQuery(c *gin.Context, log *zap.Logger, req interface{}) bool {
	if err := c.ShouldBindQuery(req); err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "INVALID_QUERY", "Invalid query parameters", gin.H{
			"details": err.Error(),
		})
		return false
	}
	return true
}

// ValidateURI is a helper for validating URI parameters
// Returns true if validation passed, false otherwise
func ValidateURI(c *gin.Context, log *zap.Logger, req interface{}) bool {
	if err := c.ShouldBindUri(req); err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "INVALID_PARAMS", "Invalid URL parameters", gin.H{
			"details": err.Error(),
		})
		return false
	}
	return true
}


