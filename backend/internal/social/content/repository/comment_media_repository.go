package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

// CommentMediaRepository persists foto+video attachments for comments.
type CommentMediaRepository interface {
	CreateBatch(ctx context.Context, tx db.Tx, items []*entity.CommentMedia) error
	GetByCommentIDs(ctx context.Context, tx db.Tx, commentIDs []uuid.UUID) (map[uuid.UUID][]*entity.CommentMedia, error)
	DeleteByCommentID(ctx context.Context, tx db.Tx, commentID uuid.UUID) error
}
