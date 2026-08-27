package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/pkg/db"
)

// UserRepository defines the interface for user and user profile persistence.
// This is the SINGLE SOURCE OF TRUTH for all user-related data access.
// NO direct SQL access to users/user_profiles should exist outside this repository.
type UserRepository interface {
	// ============================================================================
	// USER QUERIES
	// ============================================================================

	// GetByID retrieves a user by ID without locking (for read-only operations).
	GetByID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.User, error)

	// GetByIDForUpdate retrieves a user with FOR UPDATE lock.
	// This prevents concurrent modifications and must be used within a transaction.
	GetByIDForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.User, error)

	// GetByFirebaseUID retrieves a user by Firebase UID.
	// Used in auth flow for user lookup and account linking.
	GetByFirebaseUID(ctx context.Context, tx db.Tx, firebaseUID string) (*entity.User, error)

	// GetByEmail retrieves a user by email (case-insensitive).
	// Used for account linking when same email exists across Firebase accounts.
	// Returns nil if not found.
	GetByEmail(ctx context.Context, tx db.Tx, email string) (*entity.User, error)

	// GetMultipleByIDs retrieves multiple users by their IDs in a single query.
	// Returns a map of userID -> User for efficient lookup.
	GetMultipleByIDs(ctx context.Context, tx db.Tx, userIDs []uuid.UUID) (map[uuid.UUID]*entity.User, error)

	// ============================================================================
	// USER PROFILE QUERIES
	// ============================================================================

	// GetProfileByID retrieves a user profile by user ID.
	// Returns nil if profile doesn't exist (user may not have completed profile).
	GetProfileByID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.UserProfile, error)

	// GetPublicInfo retrieves publicly viewable user information.
	// This includes user profile + roles + seller state for public display.
	GetPublicInfo(ctx context.Context, tx db.Tx, userID uuid.UUID, isOwnProfile bool) (*entity.UserPublicInfo, error)

	// GetPublicInfoMultiple retrieves public info for multiple users in a single query.
	// Returns a map of userID -> UserPublicInfo for efficient lookup.
	GetPublicInfoMultiple(ctx context.Context, tx db.Tx, userIDs []uuid.UUID) (map[uuid.UUID]*entity.UserPublicInfo, error)

	// GetMyProfile retrieves complete profile data for the authenticated user.
	// This includes user + profile + roles + seller state.
	GetMyProfile(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.MyProfileResponse, error)

	// ============================================================================
	// USER MUTATIONS
	// ============================================================================

	// Create creates a new user within a transaction.
	Create(ctx context.Context, tx db.Tx, user *entity.User) error

	// Update updates user fields within a transaction.
	Update(ctx context.Context, tx db.Tx, user *entity.User) error

	// SoftDeleteUser sets deleted_at = NOW() for the given user.
	// Returns alreadyDeleted=true (and nil error) when deleted_at was already
	// set — callers treat this as idempotent success.
	SoftDeleteUser(ctx context.Context, tx db.Tx, userID uuid.UUID) (alreadyDeleted bool, err error)

	// ============================================================================
	// USER PROFILE MUTATIONS
	// ============================================================================

	// UpdateProfile updates user profile fields within a transaction.
	// Only updates non-nil fields from the input.
	UpdateProfile(ctx context.Context, tx db.Tx, userID uuid.UUID, input *entity.UpdateProfileInput) (*entity.UserProfile, error)

	// ============================================================================
	// ROLE QUERIES
	// ============================================================================

	// GetRoles retrieves all roles for a user.
	GetRoles(ctx context.Context, tx db.Tx, userID uuid.UUID) ([]string, error)

	// HasRole checks if a user has a specific role.
	HasRole(ctx context.Context, tx db.Tx, userID uuid.UUID, role string) (bool, error)

	// ============================================================================
	// USERNAME OPERATIONS
	// ============================================================================

	// GetUsername retrieves the username for a user.
	// Returns empty string if not found.
	GetUsername(ctx context.Context, tx db.Tx, userID uuid.UUID) (string, error)

	// IsUsernameTaken checks if a username is already taken.
	// excludeUserID is used to allow users to keep their current username.
	IsUsernameTaken(ctx context.Context, tx db.Tx, username string, excludeUserID uuid.UUID) (bool, error)
}
