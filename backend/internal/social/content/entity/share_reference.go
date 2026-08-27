package entity

import (
	"fmt"
)

// SHARE CONTRACT ALIGNMENT V1:
//
// Share = Reference Distribution Layer
// - Share does NOT copy objects
// - Share does NOT mutate source
// - Share creates a reference with cached preview data
//
// Reference Types:
// - content: Social posts and requests
// - for_sale: Fixed-price sale surfaces
// - auction: Active auctions
// - profile: User profiles
//
// SEMANTIC RULES:
// - targetType + targetId is the CANONICAL reference
// - Preview data (title, imageUrl) is CACHE for UI only
// - Business logic MUST resolve through backend using canonical IDs
// - Preview data can be stale - never use for business decisions

// ShareTargetType defines the type of entity being shared.
type ShareTargetType string

const (
	ShareTargetTypeContent        ShareTargetType = "content"
	ShareTargetTypeForSale ShareTargetType = "for_sale"
	ShareTargetTypeAuction        ShareTargetType = "auction"
	ShareTargetTypeProfile        ShareTargetType = "profile"
)

// SharePreview contains cached preview data for display.
// This data can be stale - use only for UI, never for business logic.
//
// SNAPSHOT HONESTY: Live status fields indicate current state of shared target.
// UI should use these fields to show appropriate indicators (sold, closed, unavailable).
type SharePreview struct {
	Title    string `json:"title"`
	ImageURL string `json:"imageUrl,omitempty"`

	// SNAPSHOT HONESTY FIELDS: Live status at time of share creation
	// These enable honest UI rendering without additional API calls
	IsAvailable bool `json:"isAvailable"` // true if fixed-price sale/auction is available
	IsSold      bool `json:"isSold"`      // true if fixed-price sale is sold
	IsClosed    bool `json:"isClosed"`    // true if auction is closed
	IsDeleted   bool `json:"isDeleted"`   // true if target is deleted
}

// ShareReference represents a reference to shared content.
// It contains the canonical reference and cached preview data.
type ShareReference struct {
	TargetType ShareTargetType `json:"targetType"`
	TargetID   string          `json:"targetId"`
	Preview    SharePreview    `json:"preview"`
}

// NewShareReference creates a new share reference.
func NewShareReference(targetType ShareTargetType, targetID string, preview SharePreview) *ShareReference {
	return &ShareReference{
		TargetType: targetType,
		TargetID:   targetID,
		Preview:    preview,
	}
}

// NewShareReferenceFromContent creates a share reference from content.
//
// LIVE SHARE TRUTH V1: Accepts live status parameters for honest UI rendering.
// Callers MUST provide actual current status from content entity, not defaults.
func NewShareReferenceFromContent(
	contentID string,
	title string,
	imageURL string,
	isDeleted bool,
) *ShareReference {
	return &ShareReference{
		TargetType: ShareTargetTypeContent,
		TargetID:   contentID,
		Preview: SharePreview{
			Title:       title,
			ImageURL:    imageURL,
			IsAvailable: !isDeleted, // Content is available if not deleted
			IsSold:      false,      // Content doesn't have sold state
			IsClosed:    false,      // Content doesn't have closed state
			IsDeleted:   isDeleted,
		},
	}
}

// NewShareReferenceFromForSale creates a share reference from a fixed-price sale.
//
// LIVE SHARE TRUTH V1: Accepts live status parameters for honest UI rendering.
// Callers MUST provide actual current status from the fixed-price sale entity, not defaults.
//
// Expected source of truth: fixed-price sale availability and status fields.
func NewShareReferenceFromForSale(
	forSaleID string,
	title string,
	imageURL string,
	isAvailable bool,
	isSold bool,
	isDeleted bool,
) *ShareReference {
	return &ShareReference{
		TargetType: ShareTargetTypeForSale,
		TargetID:   forSaleID,
		Preview: SharePreview{
			Title:       title,
			ImageURL:    imageURL,
			IsAvailable: isAvailable,
			IsSold:      isSold,
			IsClosed:    false, // Fixed-price sales don't have closed state
			IsDeleted:   isDeleted,
		},
	}
}

// NewShareReferenceFromAuction creates a share reference from an auction.
//
// LIVE SHARE TRUTH V1: Accepts live status parameters for honest UI rendering.
// Callers MUST provide actual current status from auction entity, not defaults.
//
// Expected source of truth: Auction.Status field.
// - StatusActive -> IsAvailable: true, IsClosed: false
// - StatusEnded -> IsAvailable: false, IsClosed: true
// - StatusCancelled -> IsAvailable: false, IsClosed: true
func NewShareReferenceFromAuction(
	auctionID string,
	title string,
	imageURL string,
	isAvailable bool,
	isClosed bool,
	isDeleted bool,
) *ShareReference {
	return &ShareReference{
		TargetType: ShareTargetTypeAuction,
		TargetID:   auctionID,
		Preview: SharePreview{
			Title:       title,
			ImageURL:    imageURL,
			IsAvailable: isAvailable,
			IsSold:      false, // Auctions don't have sold state (they close)
			IsClosed:    isClosed,
			IsDeleted:   isDeleted,
		},
	}
}

// NewShareReferenceFromProfile creates a share reference from a profile.
//
// LIVE SHARE TRUTH V1: Accepts live status parameters for honest UI rendering.
// Callers MUST provide actual current status from profile/user entity.
func NewShareReferenceFromProfile(
	profileID string,
	name string,
	avatarURL string,
	isDeleted bool,
) *ShareReference {
	return &ShareReference{
		TargetType: ShareTargetTypeProfile,
		TargetID:   profileID,
		Preview: SharePreview{
			Title:       name,
			ImageURL:    avatarURL,
			IsAvailable: !isDeleted, // Profile is available if not deleted
			IsSold:      false,      // Profiles don't have sold state
			IsClosed:    false,      // Profiles don't have closed state
			IsDeleted:   isDeleted,
		},
	}
}

// IsValid returns true if the share reference has valid target ID.
func (s *ShareReference) IsValid() bool {
	return s.TargetID != ""
}

// DeepLink returns the deep link path for this share reference.
func (s *ShareReference) DeepLink() string {
	switch s.TargetType {
	case ShareTargetTypeContent:
		return fmt.Sprintf("/content/%s", s.TargetID)
	case ShareTargetTypeForSale:
		return fmt.Sprintf("/forSale/%s", s.TargetID)
	case ShareTargetTypeAuction:
		return fmt.Sprintf("/auction/%s", s.TargetID)
	case ShareTargetTypeProfile:
		return fmt.Sprintf("/profile/%s", s.TargetID)
	default:
		return ""
	}
}

// DisplayName returns the display name for the target type.
func (s ShareTargetType) DisplayName() string {
	switch s {
	case ShareTargetTypeContent:
		return "Post"
	case ShareTargetTypeForSale:
		return "Produk Dijual"
	case ShareTargetTypeAuction:
		return "Lelang"
	case ShareTargetTypeProfile:
		return "Profil"
	default:
		return string(s)
	}
}

// IsValid returns true if the target type is valid.
func (s ShareTargetType) IsValid() bool {
	switch s {
	case ShareTargetTypeContent, ShareTargetTypeForSale, ShareTargetTypeAuction,
		ShareTargetTypeProfile:
		return true
	default:
		return false
	}
}

// ParseShareTargetType parses a string into ShareTargetType.
func ParseShareTargetType(s string) (ShareTargetType, error) {
	t := ShareTargetType(s)
	if !t.IsValid() {
		return "", fmt.Errorf("invalid share target type: %s", s)
	}
	return t, nil
}
