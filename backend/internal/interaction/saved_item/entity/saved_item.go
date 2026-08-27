package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
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

// SavedItem represents a user's saved item (unified shortlist + auction watch)
// This is a SINGLE SOURCE OF TRUTH for all user-saved items
type SavedItem struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TargetType TargetType
	TargetID   uuid.UUID
	IntentType IntentType // Semantic intent: bookmark (forSales) or watch (auctions)
	SellerID   *uuid.UUID // Nullable: Only for forSales, nil for auctions
	CreatedAt  time.Time
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
	ForSaleTitle      string
	ForSalePrice      int64
	ForSaleType       string
	QuantityAvailable int
	ForSaleStatus     string
	ForSaleVisibility string
	ForSaleMediaURLs  []byte // JSONB array of image URLs
}

// SavedItemWithAuction represents a saved auction with its details
type SavedItemWithAuction struct {
	SavedItem

	// Auction snapshot
	AuctionTitle   string
	AuctionStatus  string
	StartPrice     *int64
	CurrentBid     *int64
	EndAt          *time.Time
}

// SavedItemList represents a user's saved items with pagination
type SavedItemList struct {
	UserID  uuid.UUID
	Items   []*SavedItemWithForSale
	Auctions []*SavedItemWithAuction
	Total   int
	Page    int
	PerPage int
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


