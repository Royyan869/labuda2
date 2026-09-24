package entity

import (
	"time"

	"github.com/google/uuid"
)

// CommentMedia represents a foto/video attachment for a comment.
// Mirrors content_media but scoped to comments, max 5 (4 image + 1 video) via service validation.
type CommentMedia struct {
	ID         uuid.UUID
	CommentID  uuid.UUID
	StorageKey string
	MediaURL   string
	MediaType  MediaType
	Position   int
	ByteSize   *int64
	CreatedAt  time.Time
}

// NewCommentMedia creates a validated comment media row.
func NewCommentMedia(commentID uuid.UUID, storageKey, mediaURL string, mediaType MediaType, position int, byteSize *int64) *CommentMedia {
	return &CommentMedia{
		ID:         uuid.New(),
		CommentID:  commentID,
		StorageKey: storageKey,
		MediaURL:   mediaURL,
		MediaType:  mediaType,
		Position:   position,
		ByteSize:   byteSize,
		CreatedAt:  time.Now(),
	}
}
