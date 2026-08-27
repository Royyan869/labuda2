package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// COMMENT → COMMERCE REFERENCE CONTRACT:
//
// Canonical commerce linkage lives in comment_commerce_references (migration
// 000031); the legacy comments.share_reference/for_sale_id/type
// columns are gone. The comment domain treats the reference as provenance —
// it mutates no commerce row.
//
// Data flow:
// - Seller shares a commerce resource on content → canonical commerce-reference write path
// - To find original content: FindTargetIDByCommerceReference(resourceID) → targetID

// FindTargetIDByCommerceReference finds the target_id for a commerce-reference comment.
// Returns the target_id of the content associated with the commerce reference comment.
//
// Canonical V2: Queries comment_commerce_references instead of the
// removed comments.share_reference JSONB field.
func (r *CommentRepositoryImpl) FindTargetIDByCommerceReference(
	ctx context.Context,
	tx db.Tx,
	resourceID uuid.UUID,
) (uuid.UUID, error) {
	var targetID uuid.UUID

	query := `
		SELECT c.target_id
		FROM comments c
		JOIN comment_commerce_references ccr ON ccr.comment_id = c.id
		WHERE ccr.for_sale_id = $1
		LIMIT 1
	`

	err := tx.QueryRow(ctx, query, resourceID).Scan(&targetID)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return uuid.Nil, fmt.Errorf("commerce reference comment not found: %s", resourceID)
		}
		return uuid.Nil, fmt.Errorf("find target id by commerce reference failed: %w", err)
	}

	return targetID, nil
}

// SoftDelete marks a comment as deleted.
func (r *CommentRepositoryImpl) SoftDelete(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	deletedAt time.Time,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE comments
		SET deleted_at = $1, updated_at = $1
		WHERE id = $2
	`, deletedAt, id)
	if err != nil {
		return fmt.Errorf("soft delete comment failed: %w", err)
	}
	return nil
}

// Restore restores a comment that was soft-deleted (sets deleted_at to NULL).
func (r *CommentRepositoryImpl) Restore(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE comments
		SET deleted_at = NULL, updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("restore comment failed: %w", err)
	}
	return nil
}

// CountTopLevelCommentsByContent returns the count of non-deleted top-level
// comments for a content row.
//
// C7C — canonical CommentCount source. Excludes:
//   - soft-deleted comments (deleted_at IS NOT NULL)
//   - replies (parent_id IS NOT NULL)
//
// Only top-level comments are counted; replies are nested UI detail. This
// matches the user-visible "comment count" on the content detail surface.
func (r *CommentRepositoryImpl) CountTopLevelCommentsByContent(
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
) (int, error) {
	var count int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM comments
		WHERE target_type = 'content'
		  AND target_id = $1
		  AND deleted_at IS NULL
		  AND parent_id IS NULL
	`, contentID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count top-level comments failed: %w", err)
	}
	return count, nil
}