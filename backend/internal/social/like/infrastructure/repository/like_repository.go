package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/like/entity"
	"github.com/labuda/backend/pkg/db"
)

// TargetLikeRepository handles like persistence for target types.
// Uses separate tables for each target type for optimal query performance.
type TargetLikeRepository struct{}

// NewTargetLikeRepository creates a new TargetLikeRepository.
func NewTargetLikeRepository() *TargetLikeRepository {
	return &TargetLikeRepository{}
}

// InsertLike creates a new like on a target type.
// Uses ON CONFLICT DO NOTHING for idempotent behavior.
func (r *TargetLikeRepository) InsertLike(
	ctx context.Context,
	tx db.Tx,
	targetID uuid.UUID,
	targetType entity.TargetType,
	userID uuid.UUID,
) error {
	tableName, columnName := r.getTableMapping(targetType)

	query := fmt.Sprintf(`
		INSERT INTO %s (%s, user_id)
		VALUES ($1, $2)
		ON CONFLICT (%s, user_id) DO NOTHING
	`, tableName, columnName, columnName)

	_, err := tx.Exec(ctx, query, targetID, userID)
	if err != nil {
		return fmt.Errorf("insert like failed: %w", err)
	}

	return nil
}

// DeleteLike removes a like on a target type.
// Returns nil even if like doesn't exist (idempotent).
func (r *TargetLikeRepository) DeleteLike(
	ctx context.Context,
	tx db.Tx,
	targetID uuid.UUID,
	targetType entity.TargetType,
	userID uuid.UUID,
) error {
	tableName, columnName := r.getTableMapping(targetType)

	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE %s = $1 AND user_id = $2
	`, tableName, columnName)

	_, err := tx.Exec(ctx, query, targetID, userID)
	if err != nil {
		return fmt.Errorf("delete like failed: %w", err)
	}

	return nil
}

// ExistsLike checks if a user has liked a specific target.
func (r *TargetLikeRepository) ExistsLike(
	ctx context.Context,
	tx db.Tx,
	targetID uuid.UUID,
	targetType entity.TargetType,
	userID uuid.UUID,
) (bool, error) {
	tableName, columnName := r.getTableMapping(targetType)

	query := fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s
			WHERE %s = $1 AND user_id = $2
		)
	`, tableName, columnName)

	var exists bool
	err := tx.QueryRow(ctx, query, targetID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check like exists failed: %w", err)
	}

	return exists, nil
}

// CountLikes returns the number of likes for a target.
func (r *TargetLikeRepository) CountLikes(
	ctx context.Context,
	tx db.Tx,
	targetID uuid.UUID,
	targetType entity.TargetType,
) (int, error) {
	tableName, columnName := r.getTableMapping(targetType)

	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s
		WHERE %s = $1
	`, tableName, columnName)

	var count int
	err := tx.QueryRow(ctx, query, targetID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count likes failed: %w", err)
	}

	return count, nil
}

// getTableMapping returns the table name and column name for a given target type.
func (r *TargetLikeRepository) getTableMapping(targetType entity.TargetType) (string, string) {
	switch targetType {
	case entity.TargetTypeContent:
		return "content_likes", "content_id"
	case entity.TargetTypeComment:
		return "comment_likes", "comment_id"
	default:
		// Default to content_likes for unknown types (will fail at query level)
		return "content_likes", "content_id"
	}
}
