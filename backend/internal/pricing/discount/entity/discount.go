// DOMAIN: PRICING
// NOTE: Seller-owned promo code system for checkout discounts
//
// CANONICAL MODEL (DISCOUNT-003):
// - Discount is seller-funded and seller-created
// - Discount applicability is by SELLING SURFACE ONLY (for_sale / auction / both)
// - No specific item/surface targeting — discount applies to ALL surfaces of the seller's chosen type
// - Discount types: percentage, flat_amount
// - Validity: expiry-only (valid_until). Discount becomes active on creation.
// - Usage: optional total_usage_limit (0 = unlimited). No per-user limit.
// - Minimum purchase: optional min_purchase against the final transaction price P
// - Anyone who knows the code may attempt to use it
// - PricingToken is the sole transaction-pricing authority
// - Discount applies to the FINAL TRANSACTION PRICE (P), not starting/reference price

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// DiscountType represents the type of discount.
type DiscountType string

const (
	DiscountTypePercentage DiscountType = "percentage"
	DiscountTypeFlatAmount DiscountType = "flat_amount"
)

// ============================================================================
// ECONOMIC SAFETY CONSTANTS (P0)
// ============================================================================

// MaxDiscountPercentage is the maximum allowed discount percentage.
// This prevents excessive margin erosion and ensures business sustainability.
const MaxDiscountPercentage = 50 // 50% maximum discount

// String returns the string representation of DiscountType.
func (dt DiscountType) String() string {
	return string(dt)
}

// IsValid checks if the DiscountType is valid.
func (dt DiscountType) IsValid() bool {
	switch dt {
	case DiscountTypePercentage, DiscountTypeFlatAmount:
		return true
	default:
		return false
	}
}

// DiscountAppliesTo represents what commerce context a discount can be used in.
type DiscountAppliesTo string

const (
	DiscountAppliesToForSale DiscountAppliesTo = "for_sale"
	DiscountAppliesToAuction DiscountAppliesTo = "auction"
	DiscountAppliesToBoth    DiscountAppliesTo = "both"
)

// String returns the string representation of DiscountAppliesTo.
func (da DiscountAppliesTo) String() string {
	return string(da)
}

// IsValid checks if the DiscountAppliesTo value is valid.
func (da DiscountAppliesTo) IsValid() bool {
	switch da {
	case DiscountAppliesToForSale, DiscountAppliesToAuction, DiscountAppliesToBoth:
		return true
	default:
		return false
	}
}

// AllowsContext checks if the discount can be used in the provided checkout context.
func (da DiscountAppliesTo) AllowsContext(ctx DiscountContextType) bool {
	switch da {
	case DiscountAppliesToBoth:
		return true
	case DiscountAppliesToForSale:
		return ctx == DiscountContextForSale
	case DiscountAppliesToAuction:
		return ctx == DiscountContextAuction
	default:
		return false
	}
}

// DiscountContextType represents the checkout context used to validate a code.
type DiscountContextType string

const (
	DiscountContextForSale DiscountContextType = "for_sale"
	DiscountContextAuction DiscountContextType = "auction"
)

// String returns the string representation of DiscountContextType.
func (ct DiscountContextType) String() string {
	return string(ct)
}

// IsValid checks if the DiscountContextType is valid.
func (ct DiscountContextType) IsValid() bool {
	switch ct {
	case DiscountContextForSale, DiscountContextAuction:
		return true
	default:
		return false
	}
}

// Discount represents a seller-owned promo code that can be applied at checkout.
//
// DOMAIN BOUNDARY:
// - Discount does NOT modify forSale.price or auction price
// - Discount is ONLY calculated at checkout time, server-side
// - Discount does NOT touch ledger directly
// - OrderService remains the only creator of orders
//
// CANONICAL MODEL:
// - applies_to: for_sale, auction, or both (surface-level, NOT item-level)
// - seller_id must always be set (seller-owned discounts only)
// - valid_until is the only time boundary; discount is active from creation
// - min_purchase is evaluated against the final transaction price P
type Discount struct {
	ID                uuid.UUID         `json:"id"`
	Code              string            `json:"code"`
	Type              DiscountType      `json:"type"`
	Value             decimal.Decimal   `json:"value"`
	MinPurchase       decimal.Decimal   `json:"min_purchase"`
	AppliesTo         DiscountAppliesTo `json:"applies_to"`
	SellerID          *uuid.UUID        `json:"seller_id,omitempty"`
	ValidUntil        time.Time         `json:"valid_until"`
	TotalUsageLimit   int               `json:"total_usage_limit"`
	CurrentUsageCount int               `json:"current_usage_count"`
	IsActive          bool              `json:"is_active"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// DiscountValidationError is returned when discount validation fails.
type DiscountValidationError struct {
	Code   string
	Reason string
}

func (e *DiscountValidationError) Error() string {
	return fmt.Sprintf("discount validation failed for code '%s': %s", e.Code, e.Reason)
}

// DiscountExpiredError is returned when a discount has expired.
type DiscountExpiredError struct {
	Code       string
	ValidUntil time.Time
}

func (e *DiscountExpiredError) Error() string {
	return fmt.Sprintf("discount '%s' expired at: %s", e.Code, e.ValidUntil.Format(time.RFC3339))
}

// DiscountNotActiveError is returned when a discount is not active.
type DiscountNotActiveError struct {
	Code string
}

func (e *DiscountNotActiveError) Error() string {
	return fmt.Sprintf("discount '%s' is not active", e.Code)
}

// DiscountUsageLimitExceededError is returned when usage limits are exceeded.
type DiscountUsageLimitExceededError struct {
	Code         string
	CurrentUsage int
	UsageLimit   int
}

func (e *DiscountUsageLimitExceededError) Error() string {
	return fmt.Sprintf("discount '%s': total usage limit exceeded (used: %d, limit: %d)",
		e.Code, e.CurrentUsage, e.UsageLimit)
}

// MinPurchaseNotMetError is returned when minimum purchase requirement is not met.
type MinPurchaseNotMetError struct {
	Code        string
	MinPurchase decimal.Decimal
	Subtotal    decimal.Decimal
}

func (e *MinPurchaseNotMetError) Error() string {
	return fmt.Sprintf("discount '%s': minimum purchase not met (required: %s, actual: %s)",
		e.Code, e.MinPurchase.String(), e.Subtotal.String())
}

// NewDiscount creates a new discount with validation.
//
// CANONICAL CONSTRUCTOR: code, type, value, minPurchase, appliesTo,
// sellerID, validUntil, totalUsageLimit.
func NewDiscount(
	code string,
	discountType DiscountType,
	value decimal.Decimal,
	minPurchase decimal.Decimal,
	appliesTo DiscountAppliesTo,
	sellerID *uuid.UUID,
	validUntil time.Time,
	totalUsageLimit int,
) (*Discount, error) {
	if !discountType.IsValid() {
		return nil, fmt.Errorf("invalid discount type: %s", discountType)
	}
	if !appliesTo.IsValid() {
		return nil, fmt.Errorf("invalid discount applies_to: %s", appliesTo)
	}
	if sellerID == nil {
		return nil, fmt.Errorf("seller_id is required for seller-owned discounts")
	}

	if value.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("discount value must be positive: got %s", value.String())
	}
	if discountType == DiscountTypePercentage && value.GreaterThan(decimal.NewFromInt(100)) {
		return nil, fmt.Errorf("percentage discount cannot exceed 100%%: got %s", value.String())
	}

	if minPurchase.LessThan(decimal.Zero) {
		return nil, fmt.Errorf("min_purchase cannot be negative: got %s", minPurchase.String())
	}

	if validUntil.Before(time.Now()) {
		return nil, fmt.Errorf("valid_until cannot be in the past")
	}
	if totalUsageLimit < 0 {
		return nil, fmt.Errorf("total_usage_limit cannot be negative: got %d", totalUsageLimit)
	}

	now := time.Now()
	return &Discount{
		ID:                uuid.New(),
		Code:              code,
		Type:              discountType,
		Value:             value,
		MinPurchase:       minPurchase,
		AppliesTo:         appliesTo,
		SellerID:          sellerID,
		ValidUntil:        validUntil,
		TotalUsageLimit:   totalUsageLimit,
		CurrentUsageCount: 0,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// IsActiveNow checks if the discount is currently active based on:
// - IsActive flag
// - Current time is before ValidUntil
func (d *Discount) IsActiveNow() bool {
	if !d.IsActive {
		return false
	}
	if time.Now().After(d.ValidUntil) {
		return false
	}
	return true
}

// CanBeUsedBy checks if the discount can be used based on active state and usage limits.
func (d *Discount) CanBeUsedBy() error {
	if !d.IsActive {
		return &DiscountNotActiveError{Code: d.Code}
	}

	now := time.Now()
	if now.After(d.ValidUntil) {
		return &DiscountExpiredError{Code: d.Code, ValidUntil: d.ValidUntil}
	}

	if d.TotalUsageLimit > 0 && d.CurrentUsageCount >= d.TotalUsageLimit {
		return &DiscountUsageLimitExceededError{
			Code:         d.Code,
			UsageLimit:   d.TotalUsageLimit,
			CurrentUsage: d.CurrentUsageCount,
		}
	}

	return nil
}

// MeetsMinPurchase checks if the subtotal meets the minimum purchase requirement.
// MinPurchase is evaluated against the final transaction product price P.
func (d *Discount) MeetsMinPurchase(subtotal decimal.Decimal) error {
	if d.MinPurchase.GreaterThan(decimal.Zero) && subtotal.LessThan(d.MinPurchase) {
		return &MinPurchaseNotMetError{
			Code:        d.Code,
			MinPurchase: d.MinPurchase,
			Subtotal:    subtotal,
		}
	}
	return nil
}

// ValidateEconomicSafety checks if the discount meets economic safety requirements.
// Enforces the 50% maximum percentage discount cap.
func (d *Discount) ValidateEconomicSafety() error {
	if d.Type != DiscountTypePercentage {
		return nil
	}

	if d.Value.GreaterThan(decimal.NewFromInt(MaxDiscountPercentage)) {
		return &DiscountValidationError{
			Code:   d.Code,
			Reason: fmt.Sprintf("discount percentage %.2f%% exceeds maximum allowed %d%% - economic safety violation",
				d.Value.InexactFloat64(), MaxDiscountPercentage),
		}
	}

	return nil
}

// CalculateDiscountAmount calculates the discount amount for a given subtotal (P).
//
// Percentage: D = P × percentage / 100
// Flat: D = min(flat_amount, P)
//
// Caller is responsible for checking MeetsMinPurchase before calling this.
func (d *Discount) CalculateDiscountAmount(subtotal decimal.Decimal) decimal.Decimal {
	var discountAmount decimal.Decimal

	switch d.Type {
	case DiscountTypePercentage:
		discountAmount = subtotal.Mul(d.Value).Div(decimal.NewFromInt(100))
	case DiscountTypeFlatAmount:
		discountAmount = d.Value
	default:
		return decimal.Zero
	}

	// Discount cannot exceed subtotal
	if discountAmount.GreaterThan(subtotal) {
		discountAmount = subtotal
	}

	return discountAmount
}

// Deactivate marks the discount as inactive.
func (d *Discount) Deactivate() {
	d.IsActive = false
	d.UpdatedAt = time.Now()
}

// IncrementUsage increments the current usage count.
func (d *Discount) IncrementUsage() {
	d.CurrentUsageCount++
	d.UpdatedAt = time.Now()
}

// DiscountUsage represents a single usage of a discount code.
type DiscountUsage struct {
	ID         uuid.UUID
	DiscountID uuid.UUID
	UserID     uuid.UUID
	OrderID    uuid.UUID
	UsedAt     time.Time
}

// NewDiscountUsage creates a new discount usage record.
func NewDiscountUsage(discountID, userID, orderID uuid.UUID) *DiscountUsage {
	return &DiscountUsage{
		ID:         uuid.New(),
		DiscountID: discountID,
		UserID:     userID,
		OrderID:    orderID,
		UsedAt:     time.Now(),
	}
}

// DiscountApplicationResult contains the result of applying a discount.
type DiscountApplicationResult struct {
	DiscountID      uuid.UUID
	Code            string
	Type            DiscountType
	Value           decimal.Decimal
	AppliesTo       DiscountAppliesTo
	DiscountAmount  decimal.Decimal
	Subtotal        decimal.Decimal
	DiscountedTotal decimal.Decimal
}

// NewDiscountApplicationResult creates a new discount application result.
func NewDiscountApplicationResult(
	discount *Discount,
	discountAmount, subtotal decimal.Decimal,
) *DiscountApplicationResult {
	return &DiscountApplicationResult{
		DiscountID:      discount.ID,
		Code:            discount.Code,
		Type:            discount.Type,
		Value:           discount.Value,
		AppliesTo:       discount.AppliesTo,
		DiscountAmount:  discountAmount,
		Subtotal:        subtotal,
		DiscountedTotal: subtotal.Sub(discountAmount),
	}
}
