package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/content/entity"
)

var ErrDuplicateContentResourceOccurrence = errors.New("duplicate content resource occurrence")

// ContentRepository defines the interface for content persistence.
type ContentRepository interface {
	// Create persists a new content within a transaction.
	Create(ctx context.Context, tx interface{}, content *entity.Content) error

	// CreateMedia persists media attachments within a transaction.
	CreateMedia(ctx context.Context, tx interface{}, media []*entity.ContentMedia) error

	// GetByID retrieves content without locking (for read-only operations).
	GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error)

	// GetForUpdate retrieves content with FOR UPDATE lock.
	// This prevents concurrent modifications and must be used within a transaction.
	GetForUpdate(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error)

	// Update persists content changes within a transaction.
	Update(ctx context.Context, tx interface{}, content *entity.Content) error

	// ListByAuthor retrieves content by author ID with cursor-based pagination.
	// Returns active, non-deleted content by the author.
	ListByAuthor(ctx context.Context, tx interface{}, authorID uuid.UUID, limit int, cursor string) ([]*entity.Content, string, error)

	// GetMedia retrieves all media for a content.
	GetMedia(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]*entity.ContentMedia, error)

	// GetTagsByContentID retrieves hashtags for a content item.
	// Returns an empty (non-nil) slice when the content has no tags.
	GetTagsByContentID(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]string, error)

	// InsertTags persists hashtags for a content item within a transaction.
	// Each tag string is stored as-is (caller responsible for normalisation).
	// Idempotent: duplicate (content_id, hashtag) rows are ignored via ON CONFLICT DO NOTHING.
	InsertTags(ctx context.Context, tx interface{}, contentID uuid.UUID, tags []string) error
}
