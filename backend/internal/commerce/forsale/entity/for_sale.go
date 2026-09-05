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

// ForSale represents the selling surface owned by a seller.
// Product content (title, description, media, koi attributes, farm address,
// preparation) is owned exclusively by Product — ForSale surface owns ONLY price/stock/visibility/status.
//
// Deprecated alias fields below (Title, Description, MediaURLs, Variety, SizeCM, AgeMonths, Gender, Breeder, Bloodline,
// Certificates, FarmAddressID, PreparationTime/Note) are kept ONLY for Social/legacy read compatibility until
// Fase 2.2 Social convergence removes them. They are ALWAYS synced from Product during hydration and writes —
// Product is the sole persistence authority. Do NOT read/write these alias fields in new code; use Product directly.
type ForSale struct {
	ID        uuid.UUID
	ProductID uuid.UUID
	SellerID  uuid.UUID

	// Deprecated aliases — Product is authority. Kept for Social compatibility (revert per closure scope integrity).
	Title       string          `json:"-"` // Deprecated: use Product.Title
	Description string          `json:"-"` // Deprecated: use Product.Description
	MediaURLs   json.RawMessage `json:"-"` // Deprecated: use Product.MediaURLs
	Variety     string          `json:"-"` // Deprecated: use Product.Variety
	SizeCM      *int            `json:"-"` // Deprecated: use Product.SizeCm
	AgeMonths   *int            `json:"-"` // Deprecated: use Product.AgeMonths
	Gender      *string         `json:"-"` // Deprecated: use Product.Gender
	Breeder     *string         `json:"-"` // Deprecated: use Product.Breeder
	Bloodline   *string         `json:"-"` // Deprecated: use Product.Bloodline
	Certificates []string       `json:"-"` // Deprecated: use Product.Certificates
	FarmAddressID *uuid.UUID    `json:"-"` // Deprecated: use Product.FarmAddressID
	PreparationTime PreparationTime `json:"-"` // Deprecated: use Product.PreparationTime
	PreparationNote *string         `json:"-"` // Deprecated: use Product.PreparationNote

	// Pricing and inventory — ForSale surface authority
	ForSaleType       ForSaleType
	PricePerUnit      money.Money
	QuantityAvailable int

	// Feature flags
	NegotiationEnabled bool
	Visibility         ForSaleVisibility

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
	Reason    string
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
// GOVERNANCE BYPASS: This method intentionally bypasses the ordinary transition
// graph (CanTransition) because moderation restoration is a governance authority
// override, not a seller action. This is the ONLY path that can transition
// withdrawn → active.
//
// GUARD: Only applies when status == withdrawn. Sold for_sales are rejected
// (stock was claimed by buyer). Draft for_sales are rejected (never active).
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
// This is the ONLY path that can transition sold → active.

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

// NewForSaleSurface creates a ForSale surface-only entity (no Product content).
// Use this in service mint path where Product is created explicitly via ProductRepository.
// This is the canonical constructor for production service code — no hidden Product creation.
func NewForSaleSurface(
	sellerID uuid.UUID,
	for_saleType ForSaleType,
	pricePerUnit money.Money,
	quantityAvailable int,
	negotiationEnabled bool,
	visibility ForSaleVisibility,
) (*ForSale, error) {
	if for_saleType == ForSaleTypeFixedPrice && quantityAvailable < 1 {
		return nil, &InvalidQuantityError{Amount: quantityAvailable}
	}
	if pricePerUnit.IsNegative() {
		return nil, fmt.Errorf("price cannot be negative: %d", pricePerUnit.Int64())
	}
	now := time.Now()
	return &ForSale{
		ID:                 uuid.New(),
		SellerID:           sellerID,
		ForSaleType:        for_saleType,
		PricePerUnit:       pricePerUnit,
		QuantityAvailable:  quantityAvailable,
		NegotiationEnabled: negotiationEnabled,
		Visibility:         visibility,
		Status:             ForSaleStatusDraft,
		PublishedAt:        nil,
		SoldAt:             nil,
		WithdrawnAt:        nil,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}
