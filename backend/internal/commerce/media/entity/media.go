package entity

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MediaType identifies the canonical type of a commerce media asset.
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
)

// IsValid reports whether the media type is canonical.
func (t MediaType) IsValid() bool {
	switch t {
	case MediaTypeImage, MediaTypeVideo:
		return true
	default:
		return false
	}
}

// String returns the raw string representation.
func (t MediaType) String() string {
	return string(t)
}

// InferMediaType derives a canonical media type from a reference string.
// We default to image because the current commerce create surfaces only
// submit image URLs. Video is inferred when the path extension is a
// well-known video suffix.
func InferMediaType(reference string) MediaType {
	trimmed := strings.TrimSpace(reference)
	if trimmed == "" {
		return MediaTypeImage
	}

	pathValue := trimmed
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" {
		pathValue = parsed.Path
	}

	switch strings.ToLower(filepath.Ext(pathValue)) {
	case ".mp4", ".mov", ".webm", ".m4v", ".avi", ".mkv", ".3gp", ".wmv":
		return MediaTypeVideo
	default:
		return MediaTypeImage
	}
}

// Media represents an ordered commerce media asset.
type Media struct {
	ID           uuid.UUID
	URL          string
	Type         MediaType
	Position     int
	CreatedAt    time.Time
	ThumbnailURL *string
	Width        *int
	Height       *int
	Duration     *int
}

// NewMedia creates a validated media value object.
func NewMedia(reference string, mediaType MediaType, position int, createdAt time.Time) (*Media, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return nil, fmt.Errorf("media url is required")
	}
	if mediaType == "" {
		mediaType = InferMediaType(reference)
	}
	if !mediaType.IsValid() {
		return nil, fmt.Errorf("invalid media type: %s", mediaType)
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	return &Media{
		ID:        uuid.New(),
		URL:       reference,
		Type:      mediaType,
		Position:  position,
		CreatedAt: createdAt,
	}, nil
}

// NewListFromReferences converts a reference list into an ordered typed list.
func NewListFromReferences(references []string, createdAt time.Time) ([]Media, error) {
	if len(references) == 0 {
		return nil, nil
	}

	items := make([]Media, 0, len(references))
	for i, reference := range references {
		item, err := NewMedia(reference, InferMediaType(reference), i, createdAt)
		if err != nil {
			return nil, fmt.Errorf("media[%d]: %w", i, err)
		}
		items = append(items, *item)
	}
	return items, nil
}

// NewLegacyImageListFromReferences converts a legacy media_urls list into an
// ordered image-only list. Legacy commerce media never infers video authority
// from URL text.
func NewLegacyImageListFromReferences(references []string, createdAt time.Time) ([]Media, error) {
	if len(references) == 0 {
		return nil, nil
	}

	items := make([]Media, 0, len(references))
	for i, reference := range references {
		item, err := NewMedia(reference, MediaTypeImage, i, createdAt)
		if err != nil {
			return nil, fmt.Errorf("media[%d]: %w", i, err)
		}
		items = append(items, *item)
	}
	return items, nil
}

// FlattenURLs returns the canonical URL list in the provided order.
func FlattenURLs(items []Media) []string {
	if len(items) == 0 {
		return nil
	}

	urls := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item.URL); trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	return urls
}

// CloneList returns a shallow copy of the media slice.
func CloneList(items []Media) []Media {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]Media, len(items))
	copy(cloned, items)
	return cloned
}
