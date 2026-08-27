package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

// CommentRepository defines the interface for comment persistence.
type CommentRepository interface {
	// Create persists a new comment within a transaction.
	Create(ctx context.Context, tx db.Tx, comment *entity.Comment) error

	// ListByTarget retrieves comments with cursor-based pagination (flat rows;
	// replies are rows with parent_id in the same list).
	ListByTarget(ctx context.Context, tx db.Tx, targetType entity.CommentTargetType, targetID uuid.UUID, limit int, cursor string) ([]*entity.Comment, string, error)

	// FindTargetIDByCommerceReference finds the target_id for a commerce_reference comment.
	FindTargetIDByCommerceReference(ctx context.Context, tx db.Tx, resourceID uuid.UUID) (uuid.UUID, error)

	// GetByID retrieves a comment by ID (without lock).
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Comment, error)

	// CountTopLevelCommentsByContent returns the count of non-deleted top-level
	// comments (parent_id IS NULL) on a content row. Used by the content detail
	// handler to populate EngagementResponse.CommentCount (C7C). Deleted
	// comments (deleted_at IS NOT NULL) are excluded — governance soft-delete
	// is the authority, not application-level filtering.
	CountTopLevelCommentsByContent(ctx context.Context, tx db.Tx, contentID uuid.UUID) (int, error)

	// SoftDelete marks a comment as deleted.
	SoftDelete(ctx context.Context, tx db.Tx, id uuid.UUID, deletedAt time.Time) error

	// Restore restores a comment that was soft-deleted (sets deleted_at to NULL).
	Restore(ctx context.Context, tx db.Tx, id uuid.UUID) error
}


