package like

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines the interface for like persistence.
//
// DESIGN PRINCIPLES:
// - All write operations must use explicit transaction (tx interface{})
// - No business logic in repository
// - No cross-domain calls
// - Use ON CONFLICT DO NOTHING for duplicate-safe inserts
// - No OFFSET - use cursor pagination only
type Repository interface {
	// InsertLike creates a new like on content.
	// Uses ON CONFLICT DO NOTHING for idempotent behavior.
	InsertLike(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) error

	// DeleteLike removes a like on content.
	// Returns nil even if like doesn't exist (idempotent).
	DeleteLike(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) error

	// ExistsLike checks if a user has liked a content.
	ExistsLike(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) (bool, error)

	// CountLikes returns the number of likes for a content.
	CountLikes(ctx context.Context, tx interface{}, contentID uuid.UUID) (int, error)

	// GetLikeCreatedAt returns the created_at of an existing (content, user) like.
	// The caller inserts the like first (ON CONFLICT DO NOTHING); the value read
	// back is the single canonical occurrence identity used to scope the outbox
	// idempotency key so a LIKE after an UNLIKE re-emits a fresh event.
	GetLikeCreatedAt(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) (time.Time, error)
}
