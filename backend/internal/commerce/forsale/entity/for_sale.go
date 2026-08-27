// DOMAIN: COMMERCE
// NOTE: Core commerce entity representing sellable items

package entity

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/pkg/money"
)

// ForSale represents a sellable item owned by a seller.
// This is the new commerce entry object, replacing Asset and Collection domains.
type ForSale struct {
	ID          uuid.UUID
	ProductID   uuid.UUID
	SellerID    uuid.UUID
	Title       string
	Description string

	// MediaURLs is a JSONB array of media URLs (images/videos)
	MediaURLs json.RawMessage

	// Koi fish specific attributes
	Variety      string   // Koi variety (e.g., Kohaku, Showa)
	SizeCM       *int     // Size in centimeters (nullable)
	AgeMonths    *int     // Age in months (nullable)
	Gender       *string  // Gender (nullable)
	Breeder      *string  // Breeder name (nullable)
	Bloodline    *string  // Bloodline information (nullable)
	Certificates []string // Array of certificate URLs/IDs

	// Pricing and inventory
	ForSaleType ForSaleType
	PricePerUnit       money.Money
	QuantityAvailable  int

	// Feature flags
	NegotiationEnabled bool
	Visibility         ForSaleVisibility

	// Origin - Source/context of how for_sale was created (tracking only, NOT for business logic)
	Origin ForSaleOrigin

	// Shipping preferences - how this item can be shipped
	// Shipping options are now managed through product_shipping_options table
	// FarmAddressID is the source of truth for shipping origin (references address with purpose="sender")
	FarmAddressID *uuid.UUID // Farm/warehouse address for shipping origin (REQUIRED for published for_sales)

	// Shipping Readiness - preparation time before item can be shipped
	// BUYER EXPECTATION: This is what buyers see before purchasing
	// ORDER SNAPSHOT: When order is created, this value is frozen as preparation_time_snapshot
	PreparationTime PreparationTime // How long seller needs to prepare item for shipping
	PreparationNote *string         // Optional: Additional context (e.g., "Butuh puasa 2 hari sebelum packing")

	// Status
	Status ForSaleStatus

	// Timestamps
	PublishedAt *time.Time
	SoldAt      *time.Time
	WithdrawnAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Canonical joined product snapshot.
	Product *productEntity.Product
}

// ============================================================================
// BUSINESS INVARiants
// ============================================================================

// InsufficientQuantityError is returned when quantity is not available.
type InsufficientQuantityError struct {
	Available int
	Requested int
}

func (e *InsufficientQuantityError) Error() string {
	return fmt.Sprintf("insufficient quantity: available=%d, requested=%d", e.Available, e.Requested)
}

// InvalidQuantityError is returned when amount is invalid (<= 0).
type InvalidQuantityError struct {
	Amount int
}

func (e *InvalidQuantityError) Error() string {
	return fmt.Sprintf("invalid quantity: must be positive, got=%d", e.Amount)
}

// ForSaleNotActiveError is returned when attempting operation on non-active for_sale.
type ForSaleNotActiveError struct {
	Status ForSaleStatus
}

func (e *ForSaleNotActiveError) Error() string {
	return fmt.Sprintf("for_sale not active: current status=%s", e.Status)
}

// ForSaleNotAvailableError is returned when for_sale is not available for purchase.
type ForSaleNotAvailableError struct {
	ForSaleID uuid.UUID
	Reason           string
}

func (e *ForSaleNotAvailableError) Error() string {
	return fmt.Sprintf("for_sale not available: id=%s, reason=%s", e.ForSaleID, e.Reason)
}

// ============================================================================
// STATE TRANSITIONS
// ============================================================================

// Publish transitions the for_sale from draft to active (published to market).
// This is the EXPLICIT publish boundary - for_sales do NOT auto-publish.
//
// MARKET AUTHORITY: Requires active seller subscription (checked at service layer).
//
// HARD RULE: ACTIVE = PUBLIC ONLY
// When a for_sale is published to active, it MUST be public.
// Private for_sales are workspace-only (draft state).
func (l *ForSale) Publish() error {
	if !CanTransition(l.Status, ForSaleStatusActive) {
		return &InvalidTransitionError{
			CurrentStatus: l.Status,
			TargetStatus:  ForSaleStatusActive,
		}
	}

	// HARD RULE: ACTIVE = PUBLIC ONLY
	l.Status = ForSaleStatusActive
	l.Visibility = ForSaleVisibilityPublic
	now := time.Now()
	l.PublishedAt = &now
	l.UpdatedAt = time.Now()
	return nil
}

// MarkSold transitions the for_sale to sold status.
func (l *ForSale) MarkSold() error {
	if !CanTransition(l.Status, ForSaleStatusSold) {
		return &InvalidTransitionError{
			CurrentStatus: l.Status,
			TargetStatus:  ForSaleStatusSold,
		}
	}
	l.Status = ForSaleStatusSold
	now := time.Now()
	l.SoldAt = &now
	l.UpdatedAt = time.Now()
	return nil
}

// MarkWithdrawn transitions the for_sale to withdrawn status.
func (l *ForSale) MarkWithdrawn() error {
	if !CanTransition(l.Status, ForSaleStatusWithdrawn) {
		return &InvalidTransitionError{
			CurrentStatus: l.Status,
			TargetStatus:  ForSaleStatusWithdrawn,
		}
	}
	l.Status = ForSaleStatusWithdrawn
	now := time.Now()
	l.WithdrawnAt = &now
	l.UpdatedAt = time.Now()
	return nil
}

// MarkActiveFromModeration restores a moderation-withdrawn for_sale to active.
//
// MODERATION AUTHORITY BYPASS: This method intentionally bypasses the normal
// state machine (which treats withdrawn as terminal) because moderation
// restoration is a governance authority override, not a seller action.
//
// GUARD: Only applies when status == withdrawn. Selling inventory (sold) and
// draft for_sales are not affected — sold inventory cannot be restored, and
// a draft was never active.
//
// IDEMPOTENT: If already active, this is a no-op.
func (l *ForSale) MarkActiveFromModeration() error {
	switch l.Status {
	case ForSaleStatusActive:
		// Already active — idempotent no-op.
		return nil
	case ForSaleStatusWithdrawn:
		// Restore: transition withdrawn → active (governance bypass).
		l.Status = ForSaleStatusActive
		l.Visibility = ForSaleVisibilityPublic
		now := time.Now()
		l.PublishedAt = &now
		l.UpdatedAt = time.Now()
		return nil
	case ForSaleStatusSold:
		// Cannot restore sold inventory — stock was claimed.
		return fmt.Errorf("cannot restore for_sale from moderation: status is sold (id=%s)", l.ID)
	default:
		// Draft or unknown — not a valid restoration target.
		return fmt.Errorf("cannot restore for_sale from moderation: unexpected status %q (id=%s)", l.Status, l.ID)
	}
}

// IsDraft returns true if the for_sale is in draft state (not yet published).
func (l *ForSale) IsDraft() bool {
	return l.Status == ForSaleStatusDraft
}

// IsPublished returns true if the for_sale has been published (active state).
func (l *ForSale) IsPublished() bool {
	return l.Status == ForSaleStatusActive
}

// ============================================================================
// STOCK MANAGEMENT
// ============================================================================
//
// Stock restoration is ONLY allowed through:
// - OrderService.Cancel() - when order is cancelled
// - OrderService.Expire() - when payment expires
//
// Direct calls to RestoreQuantity from ForSaleService are prohibited.
// This ensures stock restoration is always paired with order lifecycle events.

// ReduceQuantity reduces available quantity by specified amount.
// Enforces:
// - ForSale status must be active
// - Amount must be positive
// - Sufficient quantity available
// - Auto marks as sold when quantity reaches 0
func (l *ForSale) ReduceQuantity(amount int) error {
	// Guard: ForSale must be active
	if l.Status != ForSaleStatusActive {
		return &ForSaleNotActiveError{Status: l.Status}
	}

	// Guard: Amount must be positive
	if amount <= 0 {
		return &InvalidQuantityError{Amount: amount}
	}

	// Guard: Sufficient quantity available
	if l.QuantityAvailable < amount {
		return &InsufficientQuantityError{
			Available: l.QuantityAvailable,
			Requested: amount,
		}
	}

	l.QuantityAvailable -= amount
	l.UpdatedAt = time.Now()

	// Auto transition to sold when quantity exhausted
	if l.QuantityAvailable == 0 {
		l.Status = ForSaleStatusSold
		l.UpdatedAt = time.Now()
	}

	return nil
}

// RestoreQuantity restores quantity after order cancellation/expiration.
// This method should only be called from OrderService.Cancel() or OrderService.Expire().
//
// INTERNAL USE ONLY: Not exposed through ForSaleService public API.
// This is the inverse of ReduceQuantity.
func (l *ForSale) RestoreQuantity(amount int) error {
	// Guard: Amount must be positive
	if amount <= 0 {
		return &InvalidQuantityError{Amount: amount}
	}

	l.QuantityAvailable += amount
	l.UpdatedAt = time.Now()

	// If we were in sold state and now have quantity, revert to active
	if l.Status == ForSaleStatusSold && l.QuantityAvailable > 0 {
		l.Status = ForSaleStatusActive
		l.UpdatedAt = time.Now()
	}

	return nil
}

// IsAvailable checks if the for_sale is available for purchase.
//
// A for_sale is available when ALL conditions are met:
// 1. Status is ACTIVE (published) - draft for_sales are NOT available
// 2. Quantity available > 0
//
// NOTE: Visibility check is redundant because ACTIVE = PUBLIC ONLY (enforced in Publish())
// This is the authoritative buyability check used by:
// - Order creation
// - Shortlist operations
// - Purchase validation
func (l *ForSale) IsAvailable() bool {
	return l.Status == ForSaleStatusActive && l.QuantityAvailable > 0
}

// ============================================================================
// FACTORY
// ============================================================================

// NewForSale creates a new for_sale in DRAFT state.
// The for_sale must be explicitly published via Publish() before it becomes market-visible.
//
// Validates:
// - Auction for_sales must have quantity = 1
// - Fixed price for_sales must have quantity >= 1
// - Price must be non-negative
//
// NOTE: Visibility defaults to private for draft for_sales.
// Publishing with visibility=public requires active seller subscription.
// Shipping options are managed through product_shipping_options table.
func NewForSale(
	sellerID uuid.UUID,
	title string,
	description string,
	mediaURLs json.RawMessage,
	variety string,
	sizeCM *int,
	ageMonths *int,
	gender *string,
	breeder *string,
	bloodline *string,
	certificates []string,
	for_saleType ForSaleType,
	pricePerUnit money.Money,
	quantityAvailable int,
	negotiationEnabled bool,
	visibility ForSaleVisibility,
	origin ForSaleOrigin,
	// Shipping preferences
	farmAddressID *uuid.UUID,
	// Shipping Readiness
	preparationTime PreparationTime,
	preparationNote *string,
) (*ForSale, error) {
	// Guard: Fixed price for_sales must have quantity >= 1
	if for_saleType == ForSaleTypeFixedPrice && quantityAvailable < 1 {
		return nil, &InvalidQuantityError{Amount: quantityAvailable}
	}

	// Guard: Price must be non-negative
	if pricePerUnit.IsNegative() {
		return nil, fmt.Errorf("price cannot be negative: %d", pricePerUnit.Int64())
	}

	now := time.Now()
	return &ForSale{
		ID:                 uuid.New(),
		SellerID:           sellerID,
		Title:              title,
		Description:        description,
		MediaURLs:          mediaURLs,
		Variety:            variety,
		SizeCM:             sizeCM,
		AgeMonths:          ageMonths,
		Gender:             gender,
		Breeder:            breeder,
		Bloodline:          bloodline,
		Certificates:       certificates,
		ForSaleType: for_saleType,
		PricePerUnit:       pricePerUnit,
		QuantityAvailable:  quantityAvailable,
		NegotiationEnabled: negotiationEnabled,
		Visibility:         visibility,
		Origin:             origin,
		FarmAddressID:      farmAddressID,
		PreparationTime:    preparationTime,
		PreparationNote:    preparationNote,
		Status:             ForSaleStatusDraft,
		PublishedAt:        nil,
		SoldAt:             nil,
		WithdrawnAt:        nil,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}
