package entity

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ExternalProductMedia represents an uploaded asset attached to an external product.
type ExternalProductMedia struct {
	ID                uuid.UUID
	ExternalProductID uuid.UUID
	MediaType         ExternalProductMediaType
	StorageKey        string
	URL               string
	ThumbnailURL      *string
	SortOrder         int
	Metadata          json.RawMessage
	CreatedAt         time.Time
	DeletedAt         *time.Time
}

// NewExternalProductMedia creates a validated media record.
func NewExternalProductMedia(
	externalProductID uuid.UUID,
	mediaType ExternalProductMediaType,
	storageKey string,
	url string,
	thumbnailURL *string,
	sortOrder int,
	metadata json.RawMessage,
	dbTime time.Time,
) (*ExternalProductMedia, error) {
	if externalProductID == uuid.Nil {
		return nil, fmt.Errorf("external_product_id is required")
	}
	if err := ValidateMediaType(mediaType); err != nil {
		return nil, err
	}
	if storageKey == "" {
		return nil, fmt.Errorf("storage_key is required")
	}
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}

	return &ExternalProductMedia{
		ID:                uuid.New(),
		ExternalProductID: externalProductID,
		MediaType:         mediaType,
		StorageKey:        storageKey,
		URL:               url,
		ThumbnailURL:      thumbnailURL,
		SortOrder:         sortOrder,
		Metadata:          metadata,
		CreatedAt:         dbTime,
	}, nil
}

// ValidateMediaType validates the canonical media type.
func ValidateMediaType(mediaType ExternalProductMediaType) error {
	if !mediaType.IsValid() {
		return fmt.Errorf("invalid media type: %s", mediaType)
	}
	return nil
}
