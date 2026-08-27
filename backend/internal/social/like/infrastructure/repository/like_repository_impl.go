package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/like"
	"github.com/labuda/backend/pkg/db"
)

// LikeRepositoryImpl handles like persistence using pgx-based DB layer.
type LikeRepositoryImpl struct{}

// NewLikeRepository creates a new LikeRepository.
func NewLikeRepository() like.Repository {
	return &LikeRepositoryImpl{}
}

// InsertLike creates a new like on content.
// Uses ON CONFLICT DO NOTHING for idempotent behavior.
func (r *LikeRepositoryImpl) InsertLike(
	ctx context.Context,
	tx interface{},
	contentID, userID uuid.UUID,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		INSERT INTO content_likes (content_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (content_id, user_id) DO NOTHING
	`, contentID, userID)

	if err != nil {
		return fmt.Errorf("insert like failed: %w", err)
	}

	return nil
}

// DeleteLike removes a like on content.
// Returns nil even if like doesn't exist (idempotent).
func (r *LikeRepositoryImpl) DeleteLike(
	ctx context.Context,
	tx interface{},
	contentID, userID uuid.UUID,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		DELETE FROM content_likes
		WHERE content_id = $1 AND user_id = $2
	`, contentID, userID)

	if err != nil {
		return fmt.Errorf("delete like failed: %w", err)
	}

	return nil
}

// ExistsLike checks if a user has liked a content.
func (r *LikeRepositoryImpl) ExistsLike(
	ctx context.Context,
	tx interface{},
	contentID, userID uuid.UUID,
) (bool, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return false, fmt.Errorf("invalid transaction type")
	}

	var exists bool
	err := dbTx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM content_likes
			WHERE content_id = $1 AND user_id = $2
		)
	`, contentID, userID).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("check like exists failed: %w", err)
	}

	return exists, nil
}

// CountLikes returns the number of likes for a content.
func (r *LikeRepositoryImpl) CountLikes(
	ctx context.Context,
	tx interface{},
	contentID uuid.UUID,
) (int, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return 0, fmt.Errorf("invalid transaction type")
	}

	var count int
	err := dbTx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM content_likes
		WHERE content_id = $1
	`, contentID).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("count likes failed: %w", err)
	}

	return count, nil
}

// GetLikeCreatedAt returns the created_at of an existing (content, user) like.
func (r *LikeRepositoryImpl) GetLikeCreatedAt(
	ctx context.Context,
	tx interface{},
	contentID, userID uuid.UUID,
) (time.Time, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid transaction type")
	}

	var createdAt time.Time
	err := dbTx.QueryRow(ctx, `
		SELECT created_at
		FROM content_likes
		WHERE content_id = $1 AND user_id = $2
	`, contentID, userID).Scan(&createdAt)

	if err != nil {
		return time.Time{}, fmt.Errorf("get like created_at failed: %w", err)
	}

	return createdAt, nil
}
