package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/response"
)

// Common auth errors
var (
	ErrNoToken      = errors.New("no authorization token provided")
	ErrInvalidToken = errors.New("invalid or expired token")
)

// GetUserIDFromContext extracts user ID (UUID) from the gin context
// This is the canonical way to get user ID - use this instead of duplicating the logic
// Slice 3: Firebase UID via UserClaims is purged. Canonical identity is
// Labuda JWT user_id → DB.
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	// First try to get from user_id key (set by LabudaAuthMiddleware)
	if userIDVal, exists := c.Get("user_id"); exists {
		switch v := userIDVal.(type) {
		case uuid.UUID:
			if v != uuid.Nil {
				return v, nil
			}
		case string:
			if id, err := uuid.Parse(v); err == nil {
				return id, nil
			}
		}
	}

	// Try to get from userID key (alternative key)
	if userIDVal, exists := c.Get("userID"); exists {
		switch v := userIDVal.(type) {
		case uuid.UUID:
			if v != uuid.Nil {
				return v, nil
			}
		case string:
			if id, err := uuid.Parse(v); err == nil {
				return id, nil
			}
		}
	}

	return uuid.Nil, errors.New("user not authenticated")
}

// GetOptionalUserIDFromContext extracts user ID from context, returns nil if not present
func GetOptionalUserIDFromContext(c *gin.Context) *uuid.UUID {
	id, err := GetUserIDFromContext(c)
	if err != nil {
		return nil
	}
	return &id
}

// MustGetUserIDFromContext extracts user ID and returns error response if not found
// Use this in handlers that require authentication
func MustGetUserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	id, err := GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return uuid.Nil, false
	}
	return id, true
}

// GetUserID is an alias for GetUserIDFromContext for backward compatibility
func GetUserID(c *gin.Context) (uuid.UUID, error) {
	return GetUserIDFromContext(c)
}

// GetUUIDParam parses a UUID from a path parameter
// This should be called after the middleware has validated the request
func GetUUIDParam(c *gin.Context, paramName string) (uuid.UUID, error) {
	paramValue := c.Param(paramName)
	return uuid.Parse(paramValue)
}



