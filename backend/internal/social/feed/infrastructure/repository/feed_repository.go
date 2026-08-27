package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/feed/entity"
)

// FeedRepository defines the interface for feed read operations.
type FeedRepository interface {
	// GetFeed retrieves content for the user's feed.
	//
	// Returns content from followed users, excluding:
	// - Blocked users (both directions)
	// - Deleted content
	// - Non-active content
	//
	// Uses cursor-based pagination (no OFFSET) over the canonical
	// (created_at DESC, id DESC) ordering. `cursor` may be nil for the
	// first page; subsequent pages pass the FeedCursor materialised
	// from the prior result.
	//
	// `limit` is capped at 50; values <= 0 default to 20. The
	// repository internally fetches limit+1 rows so the resulting
	// FeedResult.HasMore is precise (never a boundary-equality false
	// positive).
	GetFeed(ctx context.Context, tx interface{}, viewerID uuid.UUID, cursor *entity.FeedCursor, limit int) (*entity.FeedResult, error)
}


