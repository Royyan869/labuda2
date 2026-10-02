package entity

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SellingSurface represents the exclusive selling surface attached to a Product.
// A Product may belong to exactly one surface: for_sale OR auction, never both.
type SellingSurface string

const (
	SellingSurfaceNone    SellingSurface = ""      // Unattached: Product has no selling surface
	SellingSurfaceForSale SellingSurface = "for_sale" // Attached to a ForSale
	SellingSurfaceAuction SellingSurface = "auction"  // Attached to an Auction
)

// ProductMedia is one ordered product media slot. Stored as a JSON object
// {url, blurhash?, thumbnail_url?, width?, height?, duration_ms?} inside
// products.media_urls. Bare-string elements are legacy/test residue and
// read tolerantly (see UnmarshalJSON). Duration is milliseconds; dimension
// and duration are client-provisional until the video worker canonicalizes
// them — the wire emits whatever is persisted, never derived guesses,
// except the poster/thumbnail fallback in shared.MediaWireItem.
type ProductMedia struct {
	URL          string  `json:"url"`
	Blurhash     *string `json:"blurhash,omitempty"`
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
	Width        *int    `json:"width,omitempty"`
	Height       *int    `json:"height,omitempty"`
	DurationMs   *int    `json:"duration_ms,omitempty"`
	// Processing state: processing|ready|failed (see
	// internal/pkg/mediaref/media_status.go). Absent = ready
	// (mediaref.NormalizeStatus); omitempty keeps legacy rows clean.
	Status string `json:"status,omitempty"`
}

// UnmarshalJSON accepts the canonical object shape and legacy bare strings.
func (m *ProductMedia) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*m = ProductMedia{}
		return nil
	}
	if strings.HasPrefix(trimmed, "{") {
		type alias ProductMedia
		var a alias
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		*m = ProductMedia(a)
		return nil
	}
	var url string
	if err := json.Unmarshal(data, &url); err != nil {
		return fmt.Errorf("product media must be a URL string or object: %w", err)
	}
	*m = ProductMedia{URL: url}
	return nil
}

// URLs flattens ordered media URLs — the position authority stays the slice.
func URLs(media []ProductMedia) []string {
	out := make([]string, 0, len(media))
	for _, m := range media {
		if strings.TrimSpace(m.URL) == "" {
			continue
		}
		out = append(out, m.URL)
	}
	return out
}

// Product is the internal physical item authority.
// selling_surface tracks exclusive surface ownership (for_sale | auction | null).
type Product struct {
	ID              uuid.UUID
	SellerID        uuid.UUID
	Title           string
	Description     string
	MediaURLs       []ProductMedia
	Variety         string
	SizeCm          *int
	AgeMonths       *int
	Gender          *string
	Breeder         *string
	Bloodline       *string
	Certificates    []string
	FarmAddressID   *uuid.UUID
	PreparationTime string
	SellingSurface  SellingSurface // Exclusive surface ownership: NULL | 'for_sale' | 'auction'
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
