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
type ContentMedia struct {
	ID        uuid.UUID
	ContentID uuid.UUID
	MediaURL  string
	MediaType MediaType
	Position  int
	CreatedAt time.Time
}

// NewContentMedia creates a new media attachment.
func NewContentMedia(contentID uuid.UUID, mediaURL string, mediaType MediaType, position int) *ContentMedia {
	return &ContentMedia{
		ID:        uuid.New(),
		ContentID: contentID,
		MediaURL:  mediaURL,
		MediaType: mediaType,
		Position:  position,
		CreatedAt: time.Now(),
	}
}


