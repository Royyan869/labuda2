package social

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SocialRepository defines the interface for social graph persistence.
//
// DESIGN PRINCIPLES:
// - All write operations must use explicit transaction (tx interface{})
// - No business logic in repository
// - No cross-domain calls
// - Use ON CONFLICT DO NOTHING for duplicate-safe inserts
type SocialRepository interface {
	// InsertFollow creates a new follow relationship.
	// Returns nil on success, including if follow already exists (idempotent).
	InsertFollow(ctx context.Context, tx interface{}, followerID, followingID uuid.UUID) error

	// DeleteFollow removes a follow relationship.
	// Returns nil even if follow doesn't exist (idempotent).
	DeleteFollow(ctx context.Context, tx interface{}, followerID, followingID uuid.UUID) error

	// DeleteFollowBothDirections removes follow relationships in both directions.
	// Used when block is created to clean up follows atomically.
	DeleteFollowBothDirections(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error

	// ExistsFollow checks if a follow relationship exists.
	ExistsFollow(ctx context.Context, tx interface{}, followerID, followingID uuid.UUID) (bool, error)

	// ListFollowers retrieves user IDs who follow the given user.
	// Ordered by created_at DESC (newest first).
	// If cursor is provided, paginates from that timestamp.
	ListFollowers(ctx context.Context, tx interface{}, userID uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error)

	// ListFollowing retrieves user IDs that the given user follows.
	// Ordered by created_at DESC (newest first).
	// If cursor is provided, paginates from that timestamp.
	ListFollowing(ctx context.Context, tx interface{}, userID uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error)

	// InsertBlock creates a new block relationship.
	// Returns nil on success, including if block already exists (idempotent).
	InsertBlock(ctx context.Context, tx interface{}, blockerID, blockedID uuid.UUID) error

	// DeleteBlock removes a block relationship.
	// Returns nil even if block doesn't exist (idempotent).
	DeleteBlock(ctx context.Context, tx interface{}, blockerID, blockedID uuid.UUID) error

	// ExistsBlock checks if a block relationship exists in either direction.
	// Returns true if userA blocked userB OR userB blocked userA.
	ExistsBlock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)

	// AcquireFollowLock acquires an advisory lock for the relationship between two users.
	// Uses pg_advisory_xact_lock which is automatically released at transaction end.
	// The lock is bidirectional - AcquireFollowLock(A,B) and AcquireFollowLock(B,A) acquire the same lock.
	// This MUST be called before ExistsBlock in follow operations to prevent race conditions.
	AcquireFollowLock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error

	// IsBlockedBy checks if blockerID has blocked targetID (directional check).
	// Returns true if blockerID -> targetID block exists.
	IsBlockedBy(ctx context.Context, tx interface{}, blockerID, targetID uuid.UUID) (bool, error)

	// InsertMute creates a new mute relationship.
	// Returns nil on success, including if mute already exists (idempotent).
	InsertMute(ctx context.Context, tx interface{}, muterID, mutedID uuid.UUID) error

	// DeleteMute removes a mute relationship.
	// Returns nil even if mute doesn't exist (idempotent).
	DeleteMute(ctx context.Context, tx interface{}, muterID, mutedID uuid.UUID) error

	// ExistsMute checks if a mute relationship exists.
	ExistsMute(ctx context.Context, tx interface{}, muterID, mutedID uuid.UUID) (bool, error)

	// ListMuted retrieves user IDs that the given user has muted.
	// Ordered by created_at DESC (newest first).
	// If cursor is provided, paginates from that timestamp.
	ListMuted(ctx context.Context, tx interface{}, userID uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error)

	// ListBlocked retrieves user IDs that the given user has blocked.
	// Ordered by created_at DESC (newest first).
	// If cursor is provided, paginates from that timestamp.
	ListBlocked(ctx context.Context, tx interface{}, userID uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error)
}


