package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// APIResponse represents a standard API response envelope
// This provides a consistent response structure across all user endpoints
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     interface{} `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Success sends a successful response with data
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC(),
	})
}

// Created sends a 201 Created response with data
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC(),
	})
}

// Error sends an error response
func Error(c *gin.Context, status int, code, message string) {
	c.JSON(status, APIResponse{
		Success: false,
		Error: map[string]interface{}{
			"code":    code,
			"message": message,
		},
		Timestamp: time.Now().UTC(),
	})
}

// ErrorWithDetails sends an error response with additional details
func ErrorWithDetails(c *gin.Context, status int, code, message string, details interface{}) {
	c.JSON(status, APIResponse{
		Success: false,
		Error: map[string]interface{}{
			"code":    code,
			"message": message,
			"details": details,
		},
		Timestamp: time.Now().UTC(),
	})
}

// BadRequest sends a 400 Bad Request error
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, "BAD_REQUEST", message)
}

// Unauthorized sends a 401 Unauthorized error
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// NotFound sends a 404 Not Found error
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, "NOT_FOUND", message)
}

// Conflict sends a 409 Conflict error
func Conflict(c *gin.Context, code, message string) {
	Error(c, http.StatusConflict, code, message)
}

// InternalServerError sends a 500 Internal Server Error
func InternalServerError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", message)
}


