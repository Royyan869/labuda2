package response

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Response represents a standard API response
type Response struct {
	Success   bool        `json:"success" example:"true"`
	Message   string      `json:"message,omitempty" example:"Operation completed successfully"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty"`
	Meta      *Meta       `json:"meta,omitempty"`
	Timestamp string      `json:"timestamp" example:"2024-01-15T10:30:00Z"`
}

// ErrorInfo contains error details
type ErrorInfo struct {
	Code    string      `json:"code" example:"VALIDATION_ERROR"`
	Message string      `json:"message" example:"The request validation failed"`
	Details interface{} `json:"details,omitempty"`
}

// Meta contains pagination and other metadata
// Note: omitempty removed from Total/TotalPages so zero values are sent
type Meta struct {
	Page       int `json:"page,omitempty" example:"1"`
	PerPage    int `json:"per_page,omitempty" example:"20"`
	Total      int `json:"total" example:"100"`
	TotalPages int `json:"total_pages" example:"5"`
}

// Success sends a successful response
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// SuccessWithMessage sends a successful response with a message
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// SuccessWithMeta sends a successful response with metadata (for pagination)
func SuccessWithMeta(c *gin.Context, data interface{}, meta *Meta) {
	c.JSON(http.StatusOK, Response{
		Success:   true,
		Data:      data,
		Meta:      meta,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Created sends a 201 Created response
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// NoContent sends a 204 No Content response
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error sends an error response
func Error(c *gin.Context, statusCode int, code, message string) {
	c.JSON(statusCode, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// ErrorWithDetails sends an error response with details
func ErrorWithDetails(c *gin.Context, statusCode int, code, message string, details interface{}) {
	c.JSON(statusCode, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// BadRequest sends a 400 Bad Request response
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, "BAD_REQUEST", message)
}

// Unauthorized sends a 401 Unauthorized response
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// Forbidden sends a 403 Forbidden response
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, "FORBIDDEN", message)
}

// NotFound sends a 404 Not Found response
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, "NOT_FOUND", message)
}

// InternalServerError sends a 500 Internal Server Error response
func InternalServerError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", message)
}

// FeatureDisabled sends a 503 Service Unavailable response for disabled features
// Used when a feature is temporarily or permanently disabled via feature flags
func FeatureDisabled(c *gin.Context, feature string) {
	Error(c, http.StatusServiceUnavailable, "FEATURE_DISABLED", feature+" feature is currently disabled")
}

// ValidationError sends a 422 Unprocessable Entity response
func ValidationError(c *gin.Context, details interface{}) {
	ErrorWithDetails(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Validation failed", details)
}

// ============================================================================
// Error Code Constants - Use these for consistent error codes across handlers
// ============================================================================

const (
	ErrCodeInvalidInput       = "INVALID_INPUT"
	ErrCodeEmailExists        = "EMAIL_ALREADY_EXISTS"
	ErrCodeFirebaseUIDExists  = "FIREBASE_UID_EXISTS"
	ErrCodeUsernameExists     = "USERNAME_ALREADY_EXISTS"
	ErrCodeUsernameReserved   = "USERNAME_RESERVED"
	ErrCodeInvalidEmail       = "INVALID_EMAIL"
	ErrCodeInvalidPhone       = "INVALID_PHONE"
	ErrCodeInvalidUsername    = "INVALID_USERNAME"
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeUserNotFound       = "USER_NOT_FOUND"
	ErrCodeProfileNotFound    = "PROFILE_NOT_FOUND"
	ErrCodeDatabaseError      = "DATABASE_ERROR"
	ErrCodeTransactionFailed  = "TRANSACTION_FAILED"
	ErrCodeUnauthorized       = "UNAUTHORIZED"
	ErrCodeForbidden          = "FORBIDDEN"
	ErrCodeConflict           = "CONFLICT"
)

// ============================================================================
// Enhanced Response Functions with Logging
// ============================================================================

// ErrorWithLog logs and sends error response with full context
func ErrorWithLog(c *gin.Context, log *zap.Logger, statusCode int, code, message string, err error) {
	if log != nil && err != nil {
		log.Error("API error",
			zap.Int("status", statusCode),
			zap.String("code", code),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.Error(err),
		)
	}
	Error(c, statusCode, code, message)
}

// ConflictWithLog sends a 409 Conflict error with logging (for duplicate/constraint violations)
func ConflictWithLog(c *gin.Context, log *zap.Logger, code, message string, err error) {
	ErrorWithLog(c, log, http.StatusConflict, code, message, err)
}

// NotFoundWithLog sends a 404 error with logging
func NotFoundWithLog(c *gin.Context, log *zap.Logger, resource string) {
	if log != nil {
		log.Debug("Resource not found",
			zap.String("resource", resource),
			zap.String("path", c.Request.URL.Path),
		)
	}
	Error(c, http.StatusNotFound, ErrCodeNotFound, resource+" not found")
}

// IsError checks if err matches target error
func IsError(err, target error) bool {
	return errors.Is(err, target)
}


