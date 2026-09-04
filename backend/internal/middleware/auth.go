package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/response"
)

// AuthContextKey is the key for user info in context
const AuthContextKey = "user"

// Common auth errors
var (
	ErrNoToken      = errors.New("no authorization token provided")
	ErrInvalidToken = errors.New("invalid or expired token")
)

// UserClaims represents authenticated user information.
//
// **AUTH ALIGNMENT CLARIFICATION:**
// The `Roles` field is INFORMATIONAL ONLY, refreshed from PostgreSQL by RolesLookupMiddleware.
// Do NOT use these roles for authorization decisions - they are for UI rendering hints only.
//
// **CANONICAL AUTHORITY:**
// For authoritative authorization, use:
// - RequireSellerMiddleware with RoleChecker.HasActiveSellerCapability()
// - RequireAdminMiddleware with RoleChecker.IsAdmin()
type UserClaims struct {
	UID           string
	UserID        uint // Database user ID (populated after lookup)
	Email         string
	EmailVerified bool
	Banned        bool     // Ban status from Firebase custom claims
	BanReason     string   // Ban reason from Firebase custom claims
	Provider      string   // Firebase sign-in provider (password, google.com, etc.)
	Roles         []string // **NON-AUTHORITATIVE** - Use RoleChecker for authorization
	CustomClaims  map[string]interface{}
}

// GetUserFromContext extracts user claims from context
func GetUserFromContext(c *gin.Context) (*UserClaims, bool) {
	user, exists := c.Get(AuthContextKey)
	if !exists {
		return nil, false
	}

	claims, ok := user.(*UserClaims)
	return claims, ok
}

// GetUserIDFromContext extracts user ID (UUID) from the gin context
// This is the canonical way to get user ID - use this instead of duplicating the logic
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	// First try to get from user_id key (set by UserLookupMiddleware)
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

	// Try to get from auth middleware claims
	claims, exists := GetUserFromContext(c)
	if !exists {
		return uuid.Nil, errors.New("user not authenticated")
	}

	// If UID is set in claims, try to parse as UUID
	if claims.UID != "" {
		if id, err := uuid.Parse(claims.UID); err == nil {
			return id, nil
		}
	}

	return uuid.Nil, errors.New("user_id not found in context")
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

// IsUserBannedFromContext checks if the current user is banned
func IsUserBannedFromContext(c *gin.Context) bool {
	claims, exists := GetUserFromContext(c)
	if !exists {
		return false
	}
	return claims.Banned
}

// GetBanReasonFromContext returns the ban reason for the current user
func GetBanReasonFromContext(c *gin.Context) string {
	claims, exists := GetUserFromContext(c)
	if !exists {
		return ""
	}
	return claims.BanReason
}

