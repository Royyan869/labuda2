package entity

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	productentity "github.com/labuda/backend/internal/commerce/product/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
)

// TargetType defines the type of saved item
type TargetType string

const (
	TargetTypeForSale TargetType = "for_sale"
	TargetTypeAuction TargetType = "auction"
)

// IsValid checks if the target type is valid
func (t TargetType) IsValid() bool {
	return t == TargetTypeForSale || t == TargetTypeAuction
}

// IntentType defines the semantic intent of saving an item
type IntentType string

const (
	IntentTypeBookmark IntentType = "bookmark" // For forSales: interest parking for later
	IntentTypeWatch    IntentType = "watch"    // For auctions: engagement tracking
)

// IsValid checks if the intent type is valid
func (i IntentType) IsValid() bool {
	return i == IntentTypeBookmark || i == IntentTypeWatch
}

// GetIntentTypeForTarget returns the appropriate intent type for a given target type
func GetIntentTypeForTarget(targetType TargetType) IntentType {
	switch targetType {
	case TargetTypeForSale:
		return IntentTypeBookmark
	case TargetTypeAuction:
		return IntentTypeWatch
	default:
		return ""
	}
}

// SavedItem represents a user's saved item (for_sale with bookmark intent,
// auction with watch intent)
// This is a SINGLE SOURCE OF TRUTH for all user-saved items
type SavedItem struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	TargetType TargetType `json:"target_type"`
	TargetID   uuid.UUID  `json:"target_id"`
	IntentType IntentType `json:"intent_type"`         // Semantic intent: bookmark (forSales) or watch (auctions)
	SellerID   *uuid.UUID `json:"seller_id,omitempty"` // Nullable: Only for forSales, nil for auctions
	CreatedAt  time.Time  `json:"created_at"`
}

// NewSavedItem creates a new saved item with automatic intent type detection
func NewSavedItem(userID uuid.UUID, targetType TargetType, targetID uuid.UUID, sellerID *uuid.UUID) *SavedItem {
	return &SavedItem{
		ID:         uuid.New(),
		UserID:     userID,
		TargetType: targetType,
		TargetID:   targetID,
		IntentType: GetIntentTypeForTarget(targetType),
		SellerID:   sellerID,
		CreatedAt:  time.Now(),
	}
}

// IsForSale checks if this is a forSale
func (s *SavedItem) IsForSale() bool {
	return s.TargetType == TargetTypeForSale
}

// IsAuction checks if this is an auction
func (s *SavedItem) IsAuction() bool {
	return s.TargetType == TargetTypeAuction
}

// SavedItemWithForSale represents a saved forSale with its details
type SavedItemWithForSale struct {
	SavedItem

	// ForSale snapshot (immutable at time of saving)
	ForSaleTitle      string `json:"for_sale_title,omitempty"`
	ForSalePrice      int64  `json:"for_sale_price,omitempty"`
	ForSaleType       string `json:"for_sale_type,omitempty"`
	QuantityAvailable int    `json:"quantity_available,omitempty"`
	ForSaleStatus     string `json:"for_sale_status,omitempty"`
	ForSaleVisibility string `json:"for_sale_visibility,omitempty"`
	ForSaleMediaURLs  []byte `json:"-"` // JSONB snapshot; projected by MarshalJSON
}

// MarshalJSON projects the persisted JSONB media snapshot onto the canonical
// readable URL array. The stored bytes are raw JSONB; the default marshaler
// would emit them base64-encoded — a wire shape no consumer can read.
func (s SavedItemWithForSale) MarshalJSON() ([]byte, error) {
	type Alias SavedItemWithForSale
	return json.Marshal(struct {
		Alias
		ForSaleMediaURLs []string `json:"for_sale_media_urls,omitempty"`
	}{
		Alias:            Alias(s),
		ForSaleMediaURLs: forSaleMediaURLs(s.ForSaleMediaURLs),
	})
}

// forSaleMediaURLs decodes the persisted products.media_urls snapshot through
// the canonical Product media projection: tolerant to {url, blurhash?} objects
// and legacy bare strings, resolved to readable URLs.
func forSaleMediaURLs(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var media []productentity.ProductMedia
	if err := json.Unmarshal(raw, &media); err != nil {
		return nil
	}
	return commerceshared.ResolveReadableMediaReferences(productentity.URLs(media))
}

// SavedItemWithAuction represents a saved auction with its details
type SavedItemWithAuction struct {
	SavedItem

	// Auction snapshot
	AuctionTitle  string     `json:"auction_title,omitempty"`
	AuctionStatus string     `json:"auction_status,omitempty"`
	StartPrice    *int64     `json:"start_price,omitempty"`
	CurrentBid    *int64     `json:"current_bid,omitempty"`
	EndAt         *time.Time `json:"end_at,omitempty"`
}

// SavedItemList represents a user's saved items with pagination
type SavedItemList struct {
	UserID   uuid.UUID               `json:"user_id"`
	Items    []*SavedItemWithForSale `json:"items"`
	Auctions []*SavedItemWithAuction `json:"auctions"`
	Total    int                     `json:"total"`
	Page     int                     `json:"page"`
	PerPage  int                     `json:"per_page"`
}

// ErrInvalidTargetType is returned when target type is invalid
type ErrInvalidTargetType struct {
	TargetType TargetType
}

func (e *ErrInvalidTargetType) Error() string {
	return fmt.Sprintf("invalid target type: %s", e.TargetType)
}

// ErrDuplicateSavedItem is returned when trying to save an item that's already saved
type ErrDuplicateSavedItem struct {
	UserID     uuid.UUID
	TargetType TargetType
	TargetID   uuid.UUID
}

func (e *ErrDuplicateSavedItem) Error() string {
	return fmt.Sprintf("item already saved: user_id=%s, target_type=%s, target_id=%s", e.UserID, e.TargetType, e.TargetID)
}

// ErrInvalidIntentType is returned when intent type is invalid
type ErrInvalidIntentType struct {
	IntentType IntentType
}

func (e *ErrInvalidIntentType) Error() string {
	return fmt.Sprintf("invalid intent type: %s", e.IntentType)
}


