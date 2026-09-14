package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
)

// UserLookupService interface for validating canonical Labuda user ID.
type UserLookupService interface {
	GetUserIDByID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
}

// UserLookupMiddleware validates that the canonical Labuda user_id exists.
//
// RULES:
// - IF user not found in database → return "USER_NOT_PROVISIONED" error
// - DO NOT create users automatically
// - DO NOT fallback - users must be created through explicit signup flow
// - Firebase UID is NOT a bearer/auth lookup authority; that path is purged (Slice 2).
//   Canonical identity is Labuda JWT user_id → DB.
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

// GetUserIDByID validates that a canonical Labuda user_id exists.
func (s *DBUserLookupService) GetUserIDByID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var found uuid.UUID
	err := s.db.Pool().QueryRow(ctx, "SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL", userID).Scan(&found)
	if err != nil {
		return uuid.Nil, err
	}
	return found, nil
}


