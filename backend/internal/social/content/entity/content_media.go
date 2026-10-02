package entity

import (
	"time"

	"github.com/google/uuid"
)

// MediaType represents the type of media attachment.
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
)

// ContentMedia represents a media attachment for content.
//
// DurationMs/Width/Height are client-provisional video metadata persisted at
// upload (NULL = unknown). The video worker canonicalizes them later; the
// wire emits whatever is stored, never derived guesses.
type ContentMedia struct {
	ID         uuid.UUID
	ContentID  uuid.UUID
	MediaURL   string
	MediaType  MediaType
	Position   int
	Blurhash   *string
	DurationMs *int
	Width      *int
	Height     *int
	// Processing state: processing|ready|failed (see
	// internal/pkg/mediaref/media_status.go). Plain string to keep the
	// entity layer dependency-free; writers use mediaref constants.
	Status    string
	CreatedAt time.Time
}

// NewContentMedia creates a new media attachment.
//
// Status defaults to ready; writers override with processing for video
// (see mediaref.StatusForNewRow). The default keeps the DB vocabulary
// CHECK satisfied for image fast-paths that never touch status.
func NewContentMedia(contentID uuid.UUID, mediaURL string, mediaType MediaType, position int) *ContentMedia {
	return &ContentMedia{
		ID:        uuid.New(),
		ContentID: contentID,
		MediaURL:  mediaURL,
		MediaType: mediaType,
		Position:  position,
		Status:    "ready",
		CreatedAt: time.Now(),
	}
}


