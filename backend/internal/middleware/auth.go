package middleware

import (
	"context"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/firebase"
)

// normalizeEmail normalizes email for comparison and storage
// Ensures lowercase and trimmed whitespace for database invariant
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

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

// AuthMiddleware creates a middleware that validates Firebase ID tokens
func AuthMiddleware(firebaseClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "Authorization header required")
			c.Abort()
			return
		}

		// Parse Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "Invalid authorization header format")
			c.Abort()
			return
		}

		idToken := parts[1]

		// Verify token with Firebase
		ctx := context.Background()

		// Firebase client MUST be initialized
		if firebaseClient == nil {
			response.InternalServerError(c, "Authentication not configured properly")
			c.Abort()
			return
		}

		token, err := firebaseClient.VerifyIDToken(ctx, idToken)
		if err != nil {
			response.Unauthorized(c, "Invalid or expired token")
			c.Abort()
			return
		}

		// Extract user claims with safe type assertions
		// Use comma-ok idiom to prevent panic on malformed tokens
		claims := &UserClaims{
			UID:          token.UID,
			CustomClaims: token.Claims,
		}

		// 🔐 SECURITY HARDENING: Normalize email from token
		// Ensures lowercase and trimmed for database invariant
		if email, ok := token.Claims["email"].(string); ok {
			claims.Email = normalizeEmail(email)
		}

		// Safe type assertion for email_verified (may not exist in all tokens)
		if emailVerified, ok := token.Claims["email_verified"].(bool); ok {
			claims.EmailVerified = emailVerified
		}

		// Extract ban status from custom claims (for Ban Enforcement)
		if banned, ok := token.Claims["banned"].(bool); ok {
			claims.Banned = banned
		}
		if banReason, ok := token.Claims["ban_reason"].(string); ok {
			claims.BanReason = banReason
		}

		// 🔐 EMAIL VERIFICATION: Extract sign-in provider for email verification enforcement
		// Provider is required to determine if email verification is needed:
		// - "password" (email signup): REQUIRES email verification
		// - "google.com" (OAuth): Google has already verified the email
		if firebaseClaims, ok := token.Claims["firebase"].(map[string]interface{}); ok {
			if provider, ok := firebaseClaims["sign_in_provider"].(string); ok {
				claims.Provider = provider
			}
		}

		// Check if user is banned (reject banned users)
		if claims.Banned {
			response.Forbidden(c, "Account is banned")
			c.Abort()
			return
		}

		// Store user claims in context
		c.Set(AuthContextKey, claims)

		c.Next()
	}
}

// OptionalAuthMiddleware is like AuthMiddleware but doesn't require authentication
func OptionalAuthMiddleware(firebaseClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		idToken := parts[1]
		ctx := context.Background()
		token, err := firebaseClient.VerifyIDToken(ctx, idToken)
		if err != nil {
			c.Next()
			return
		}

		// Extract user claims with safe type assertions
		// Use comma-ok idiom to prevent panic on malformed tokens
		claims := &UserClaims{
			UID:          token.UID,
			CustomClaims: token.Claims,
		}

		// 🔐 SECURITY HARDENING: Normalize email from token
		// Ensures lowercase and trimmed for database invariant
		if email, ok := token.Claims["email"].(string); ok {
			claims.Email = normalizeEmail(email)
		}

		// Safe type assertion for email_verified (may not exist in all tokens)
		if emailVerified, ok := token.Claims["email_verified"].(bool); ok {
			claims.EmailVerified = emailVerified
		}

		// Extract ban status from custom claims (for Ban Enforcement)
		if banned, ok := token.Claims["banned"].(bool); ok {
			claims.Banned = banned
		}
		if banReason, ok := token.Claims["ban_reason"].(string); ok {
			claims.BanReason = banReason
		}

		// 🔐 EMAIL VERIFICATION: Extract sign-in provider for email verification enforcement
		// Provider is required to determine if email verification is needed:
		// - "password" (email signup): REQUIRES email verification
		// - "google.com" (OAuth): Google has already verified the email
		if firebaseClaims, ok := token.Claims["firebase"].(map[string]interface{}); ok {
			if provider, ok := firebaseClaims["sign_in_provider"].(string); ok {
				claims.Provider = provider
			}
		}

		// Check if user is banned (reject banned users)
		if claims.Banned {
			response.Forbidden(c, "Account is banned")
			c.Abort()
			return
		}

		c.Set(AuthContextKey, claims)
		c.Next()
	}
}

// StrictBrowseAuthMiddleware is like AuthMiddleware but allows anonymous access.
//
// Behavior:
//  1. No Authorization header → pass through (anonymous, no claims set)
//  2. Authorization header present but not "Bearer <token>" format → 401
//  3. Bearer token invalid/expired → 401
//  4. Bearer token valid but user is banned → 403
//  5. Bearer token valid → inject UserClaims into context, pass through
//
// Use this for public browse routes (GET listings/auctions/search/content/users)
// where unauthenticated readers must be permitted but invalid tokens must be rejected
// so the client knows to clear stale credentials rather than silently proceeding.
func StrictBrowseAuthMiddleware(firebaseClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Case 1: No token at all → anonymous, pass through
		if authHeader == "" {
			c.Next()
			return
		}

		// Case 2: Authorization header present but malformed
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "Invalid authorization header format")
			c.Abort()
			return
		}

		idToken := parts[1]

		// Firebase client MUST be initialized
		if firebaseClient == nil {
			response.InternalServerError(c, "Authentication not configured properly")
			c.Abort()
			return
		}

		// Case 3: Bearer token invalid/expired
		ctx := context.Background()
		token, err := firebaseClient.VerifyIDToken(ctx, idToken)
		if err != nil {
			response.Unauthorized(c, "Invalid or expired token")
			c.Abort()
			return
		}

		// Extract user claims with safe type assertions
		claims := &UserClaims{
			UID:          token.UID,
			CustomClaims: token.Claims,
		}

		if email, ok := token.Claims["email"].(string); ok {
			claims.Email = normalizeEmail(email)
		}

		if emailVerified, ok := token.Claims["email_verified"].(bool); ok {
			claims.EmailVerified = emailVerified
		}

		if banned, ok := token.Claims["banned"].(bool); ok {
			claims.Banned = banned
		}
		if banReason, ok := token.Claims["ban_reason"].(string); ok {
			claims.BanReason = banReason
		}

		if firebaseClaims, ok := token.Claims["firebase"].(map[string]interface{}); ok {
			if provider, ok := firebaseClaims["sign_in_provider"].(string); ok {
				claims.Provider = provider
			}
		}

		// Case 4: Valid token but banned user
		if claims.Banned {
			response.Forbidden(c, "Account is banned")
			c.Abort()
			return
		}

		// Case 5: Valid token → inject claims
		c.Set(AuthContextKey, claims)
		c.Next()
	}
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

