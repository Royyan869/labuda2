package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/like/entity"
	"github.com/labuda/backend/pkg/db"
)

// ErrUnsupportedTargetType is returned when a mutation is attempted on an
// unsupported target type. Only TargetTypeComment is permitted for mutations
// through this repository; content likes use LikeRepositoryImpl exclusively.
var ErrUnsupportedTargetType = fmt.Errorf("unsupported target type for like mutation")

// TargetLikeRepository handles like persistence for comment target types.
// Content likes are written exclusively through LikeRepositoryImpl.
// This repository supports reads for both content and comment targets
// (used by GetLikeStats) but mutations only for comments.
type TargetLikeRepository struct{}

// NewTargetLikeRepository creates a new TargetLikeRepository.
func NewTargetLikeRepository() *TargetLikeRepository {
	return &TargetLikeRepository{}
}

// InsertLike creates a new like on a comment target.
// Uses ON CONFLICT DO NOTHING for idempotent behavior.
// Content likes must NOT be written through this method — use LikeRepositoryImpl.
func (r *TargetLikeRepository) InsertLike(
	ctx context.Context,
	tx db.Tx,
	targetID uuid.UUID,
	targetType entity.TargetType,
	userID uuid.UUID,
) error {
	if targetType != entity.TargetTypeComment {
		return fmt.Errorf("%w: InsertLike only supports comment targets, got %s", ErrUnsupportedTargetType, targetType)
	}

	tableName, columnName, err := r.getTableMapping(targetType)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (%s, user_id)
		VALUES ($1, $2)
		ON CONFLICT (%s, user_id) DO NOTHING
	`, tableName, columnName, columnName)

	_, err = tx.Exec(ctx, query, targetID, userID)
	if err != nil {
		return fmt.Errorf("insert like failed: %w", err)
	}

	return nil
}

// DeleteLike removes a like on a comment target.
// Returns nil even if like doesn't exist (idempotent).
// Content likes must NOT be deleted through this method — use LikeRepositoryImpl.
func (r *TargetLikeRepository) DeleteLike(
	ctx context.Context,
	tx db.Tx,
	targetID uuid.UUID,
	targetType entity.TargetType,
	userID uuid.UUID,
) error {
	if targetType != entity.TargetTypeComment {
		return fmt.Errorf("%w: DeleteLike only supports comment targets, got %s", ErrUnsupportedTargetType, targetType)
	}

	tableName, columnName, err := r.getTableMapping(targetType)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE %s = $1 AND user_id = $2
	`, tableName, columnName)

	_, err = tx.Exec(ctx, query, targetID, userID)
	if err != nil {
		return fmt.Errorf("delete like failed: %w", err)
	}

	return nil
}

// ExistsLike checks if a user has liked a specific target.
// Supports both content and comment targets (read-only, used by GetLikeStats).
func (r *TargetLikeRepository) ExistsLike(
	ctx context.Context,
	tx db.Tx,
	targetID uuid.UUID,
	targetType entity.TargetType,
	userID uuid.UUID,
) (bool, error) {
	tableName, columnName, err := r.getTableMapping(targetType)
	if err != nil {
		return false, err
	}

	query := fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s
			WHERE %s = $1 AND user_id = $2
		)
	`, tableName, columnName)

	var exists bool
	err = tx.QueryRow(ctx, query, targetID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check like exists failed: %w", err)
	}

	return exists, nil
}

// CountLikes returns the number of likes for a target.
// Supports both content and comment targets (read-only, used by GetLikeStats).
func (r *TargetLikeRepository) CountLikes(
	ctx context.Context,
	tx db.Tx,
	targetID uuid.UUID,
	targetType entity.TargetType,
) (int, error) {
	tableName, columnName, err := r.getTableMapping(targetType)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s
		WHERE %s = $1
	`, tableName, columnName)

	var count int
	err = tx.QueryRow(ctx, query, targetID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count likes failed: %w", err)
	}

	return count, nil
}

// getTableMapping returns the table name and column name for a given target type.
// Returns an error for unsupported types instead of silently falling back.
func (r *TargetLikeRepository) getTableMapping(targetType entity.TargetType) (string, string, error) {
	switch targetType {
	case entity.TargetTypeContent:
		return "content_likes", "content_id", nil
	case entity.TargetTypeComment:
		return "comment_likes", "comment_id", nil
	default:
		return "", "", fmt.Errorf("%w: %s", ErrUnsupportedTargetType, targetType)
	}
}
