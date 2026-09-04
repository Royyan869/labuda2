package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
)

// UserLookupService interface for looking up user by Firebase UID or canonical Labuda ID.
type UserLookupService interface {
	GetUserIDByFirebaseUID(ctx context.Context, firebaseUID string) (uuid.UUID, error)
	GetUserIDByID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
}

// UserLookupMiddleware creates middleware that looks up user ID from Firebase UID
// and stores it in context for use by handlers
//
// RULES:
// - IF user not found in database → return "USER_NOT_PROVISIONED" error
// - DO NOT create users automatically
// - DO NOT fallback - users must be created through explicit signup flow
func UserLookupMiddleware(userLookup UserLookupService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If canonical user_id is already present (e.g., from LabudaAuthMiddleware), validate existence.
		if uidVal, hasUserID := c.Get("user_id"); hasUserID {
			if uid, ok := uidVal.(uuid.UUID); ok && uid != uuid.Nil {
				if _, err := userLookup.GetUserIDByID(c.Request.Context(), uid); err != nil {
					response.Unauthorized(c, "USER_NOT_PROVISIONED: User not found")
					c.Abort()
					return
				}
			}
			c.Next()
			return
		}

		// Get claims from auth middleware
		claims, exists := GetUserFromContext(c)
		if !exists {
			// No auth claims, skip user lookup
			c.Next()
			return
		}

		// Look up user ID from Firebase UID
		userID, err := userLookup.GetUserIDByFirebaseUID(c.Request.Context(), claims.UID)
		if err != nil {
			// User not found in database - return error
			// Users must be created through explicit signup flow (FirebaseAuth handler → createUser())
			response.Unauthorized(c, "USER_NOT_PROVISIONED: User must complete signup first")
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Set("userID", userID)
		c.Set("firebase_uid", claims.UID)

		c.Next()
	}
}

// RequireUserMiddleware ensures user exists in database
func RequireUserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.Unauthorized(c, "User not authenticated")
			c.Abort()
			return
		}

		if id, ok := userID.(uuid.UUID); !ok || id == uuid.Nil {
			response.Unauthorized(c, "User not found in database")
			c.Abort()
			return
		}

		c.Next()
	}
}

// DBUserLookupService implements UserLookupService using raw SQL via pkg/db
type DBUserLookupService struct {
	db *db.DB
}

// NewDBUserLookupService creates a new DBUserLookupService
func NewDBUserLookupService(database *db.DB) *DBUserLookupService {
	return &DBUserLookupService{db: database}
}

// GetUserIDByFirebaseUID looks up a user ID by Firebase UID using raw SQL
func (s *DBUserLookupService) GetUserIDByFirebaseUID(ctx context.Context, firebaseUID string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := s.db.Pool().QueryRow(ctx, "SELECT id FROM users WHERE firebase_uid = $1 AND deleted_at IS NULL", firebaseUID).Scan(&userID)
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// GetUserIDByID validates that a canonical Labuda user_id exists.
func (s *DBUserLookupService) GetUserIDByID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var found uuid.UUID
	err := s.db.Pool().QueryRow(ctx, "SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL", userID).Scan(&found)
	if err != nil {
		return uuid.Nil, err
	}
	return found, nil
}

// RolesLookupMiddleware fetches user role from database and injects into UserClaims.
// This middleware must be placed AFTER UserLookupMiddlewareWithProvisioning
// so that user_id is available in context.
//
// DATABASE-BASED ROLE REFRESH: Role is queried from PostgreSQL on every request.
// This allows immediate role revocation without waiting for Firebase token refresh.
//
// **AUTH ALIGNMENT CLARIFICATION:**
// The roles injected into context are for CONVENIENCE ONLY (UI rendering, logging).
// They are NOT authoritative for authorization decisions.
//
// **CANONICAL AUTHORITY PATH:**
// For authoritative authorization, use the appropriate middleware:
// - RequireSellerMiddleware with RoleChecker.HasActiveSellerCapability()
// - RequireAdminMiddleware with RoleChecker.IsAdmin()
//
// The context roles and explicit market-authority/profile keys should only
// be used for non-critical decisions like UI element visibility.
func RolesLookupMiddleware(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user_id from context (set by UserLookupMiddleware)
		userIDVal, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		userID, ok := userIDVal.(uuid.UUID)
		if !ok || userID == uuid.Nil {
			c.Next()
			return
		}

		// Fetch role from database using raw SQL
		// DATABASE-BASED: Query PostgreSQL for authoritative role value
		var role string
		err := database.Pool().QueryRow(c.Request.Context(), "SELECT role FROM users WHERE id = $1 AND deleted_at IS NULL", userID).Scan(&role)
		if err != nil {
			// Log error but don't fail request - roles may not be critical for all endpoints
			c.Next()
			return
		}

		// Convert single role to roles slice for compatibility with existing code
		roles := []string{role}

		// Get existing UserClaims from context and update with roles
		claims, exists := GetUserFromContext(c)
		if exists {
			claims.Roles = roles
			c.Set(AuthContextKey, claims)
		}

		c.Next()
	}
}
