package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/social/feed/entity"
	feedrepo "github.com/labuda/backend/internal/social/feed/infrastructure/repository"
)

// FeedService handles feed read operations.
//
// Feed is a read model that shows content from followed users.
// No mutations, no transactions required.
type FeedService struct {
	feedRepo feedrepo.FeedRepository
}

// NewFeedService creates a new FeedService.
func NewFeedService(feedRepo feedrepo.FeedRepository) *FeedService {
	return &FeedService{
		feedRepo: feedRepo,
	}
}

// GetFeed retrieves content for the user's feed.
//
// AUTHORIZATION: Any active user can view their feed.
//
// Returns:
// - Content from followed users
// - Excludes blocked users (both directions)
// - Excludes deleted/non-active content
// - Cursor-based pagination over (created_at DESC, id DESC)
// - Limit capped at 50
//
// `cursor` is the decoded FeedCursor from the previous page; nil for
// the first page. Decoding (and 400-on-malformed) is handled at the
// HTTP boundary so the service stays cursor-shape agnostic.
//
// No transaction required (read-only).
func (s *FeedService) GetFeed(
	ctx context.Context,
	tx interface{},
	callerID uuid.UUID,
	cursor *entity.FeedCursor,
	limit int,
) (*entity.FeedResult, error) {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return nil, err
	}

	// Validate and sanitize limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	// Get feed from repository
	result, err := s.feedRepo.GetFeed(ctx, tx, callerID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("get feed: %w", err)
	}

	return result, nil
}


