package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

// CommentMediaRepositoryImpl persists comment_media rows.
type CommentMediaRepositoryImpl struct{}

func NewCommentMediaRepository() *CommentMediaRepositoryImpl { return &CommentMediaRepositoryImpl{} }

func (r *CommentMediaRepositoryImpl) CreateBatch(ctx context.Context, tx db.Tx, items []*entity.CommentMedia) error {
	if len(items) == 0 {
		return nil
	}
	for _, m := range items {
		if m == nil {
			return fmt.Errorf("comment media is required")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO comment_media (id, comment_id, storage_key, media_url, media_type, position, byte_size, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			m.ID, m.CommentID, m.StorageKey, m.MediaURL, string(m.MediaType), m.Position, m.ByteSize, m.CreatedAt,
		); err != nil {
			return fmt.Errorf("failed to insert comment_media: %w", err)
		}
	}
	return nil
}

func (r *CommentMediaRepositoryImpl) GetByCommentIDs(ctx context.Context, tx db.Tx, commentIDs []uuid.UUID) (map[uuid.UUID][]*entity.CommentMedia, error) {
	if len(commentIDs) == 0 {
		return map[uuid.UUID][]*entity.CommentMedia{}, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT id, comment_id, storage_key, media_url, media_type, position, byte_size, created_at
		FROM comment_media WHERE comment_id = ANY($1) ORDER BY comment_id, position`, commentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query comment_media: %w", err)
	}
	defer rows.Close()
	out := map[uuid.UUID][]*entity.CommentMedia{}
	for rows.Next() {
		var m entity.CommentMedia
		var mediaType string
		if err := rows.Scan(&m.ID, &m.CommentID, &m.StorageKey, &m.MediaURL, &mediaType, &m.Position, &m.ByteSize, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.MediaType = entity.MediaType(mediaType)
		out[m.CommentID] = append(out[m.CommentID], &m)
	}
	return out, rows.Err()
}

func (r *CommentMediaRepositoryImpl) DeleteByCommentID(ctx context.Context, tx db.Tx, commentID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM comment_media WHERE comment_id=$1`, commentID)
	return err
}
