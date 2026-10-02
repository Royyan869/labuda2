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
	Blurhash   *string
	DurationMs *int
	Width      *int
	Height     *int
	// Processing state: processing|ready|failed (see
	// internal/pkg/mediaref/media_status.go).
	Status    string
	CreatedAt time.Time
}

// NewCommentMedia creates a validated comment media row.
//
// Status defaults to ready; writers override with processing for video
// (see mediaref.StatusForNewRow).
func NewCommentMedia(commentID uuid.UUID, storageKey, mediaURL string, mediaType MediaType, position int, byteSize *int64) *CommentMedia {
	return &CommentMedia{
		ID:         uuid.New(),
		CommentID:  commentID,
		StorageKey: storageKey,
		MediaURL:   mediaURL,
		MediaType:  mediaType,
		Position:   position,
		ByteSize:   byteSize,
		Status:     "ready",
		CreatedAt:  time.Now(),
	}
}
