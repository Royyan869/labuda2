// DOMAIN: PRICING
// NOTE: Seller-owned promo code system for checkout discounts

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
	DiscountTypePercentage   DiscountType = "percentage"
	DiscountTypeFlatAmount   DiscountType = "flat_amount"
	DiscountTypeFreeShipping DiscountType = "free_shipping"
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
	case DiscountTypePercentage, DiscountTypeFlatAmount, DiscountTypeFreeShipping:
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

// DiscountTargetMode represents whether a discount applies seller-wide or to selected items.
type DiscountTargetMode string

const (
	DiscountTargetModeSellerWide    DiscountTargetMode = "seller_wide"
	DiscountTargetModeSelectedItems DiscountTargetMode = "selected_items"
)

// String returns the string representation of DiscountTargetMode.
func (dm DiscountTargetMode) String() string {
	return string(dm)
}

// IsValid checks if the DiscountTargetMode is valid.
func (dm DiscountTargetMode) IsValid() bool {
	switch dm {
	case DiscountTargetModeSellerWide, DiscountTargetModeSelectedItems:
		return true
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
// - Discount does NOT modify forSale.price
// - Discount is ONLY calculated at checkout time
// - Discount does NOT touch ledger directly
// - OrderService remains the only creator of orders
//
// MODEL BEHAVIOR:
// - applies_to: forSale, auction, or both
// - target_mode: seller_wide or selected_items
// - seller_id must always be set (seller-owned discounts only)
// - selected items are stored as specific forSale and/or auction IDs
type Discount struct {
	ID                uuid.UUID          `json:"id"`
	Code              string             `json:"code"`
	Type              DiscountType       `json:"type"`
	Value             decimal.Decimal    `json:"value"`
	MinPurchase       decimal.Decimal    `json:"min_purchase"`
	MaxDiscount       *decimal.Decimal   `json:"max_discount,omitempty"`
	AppliesTo         DiscountAppliesTo  `json:"applies_to"`
	TargetMode        DiscountTargetMode `json:"target_mode"`
	SellerID          *uuid.UUID         `json:"seller_id,omitempty"`
	ForSaleIDs        []uuid.UUID        `json:"applicable_for_sale_ids,omitempty"`
	AuctionIDs        []uuid.UUID        `json:"applicable_auction_ids,omitempty"`
	ValidFrom         time.Time          `json:"valid_from"`
	ValidUntil        time.Time          `json:"valid_until"`
	MaxUsagePerUser   int                `json:"max_usage_per_user"`
	TotalUsageLimit   int                `json:"total_usage_limit"`
	CurrentUsageCount int                `json:"current_usage_count"`
	IsActive          bool               `json:"is_active"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
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
	Code            string
	CurrentUsage    int
	UsageLimit      int
	UserUsageCount  int
	MaxUsagePerUser int
	IsUserLimit     bool
}

func (e *DiscountUsageLimitExceededError) Error() string {
	if e.IsUserLimit {
		return fmt.Sprintf("discount '%s': user usage limit exceeded (used: %d, limit: %d)",
			e.Code, e.UserUsageCount, e.MaxUsagePerUser)
	}
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
func NewDiscount(
	code string,
	discountType DiscountType,
	value decimal.Decimal,
	minPurchase decimal.Decimal,
	maxDiscount *decimal.Decimal,
	appliesTo DiscountAppliesTo,
	targetMode DiscountTargetMode,
	sellerID *uuid.UUID,
	forSaleIDs []uuid.UUID,
	auctionIDs []uuid.UUID,
	validFrom, validUntil time.Time,
	maxUsagePerUser, totalUsageLimit int,
) (*Discount, error) {
	if !discountType.IsValid() {
		return nil, fmt.Errorf("invalid discount type: %s", discountType)
	}
	if !appliesTo.IsValid() {
		return nil, fmt.Errorf("invalid discount applies_to: %s", appliesTo)
	}
	if !targetMode.IsValid() {
		return nil, fmt.Errorf("invalid discount target_mode: %s", targetMode)
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
		return nil, fmt.Errorf("min purchase cannot be negative: got %s", minPurchase.String())
	}
	if maxDiscount != nil && maxDiscount.LessThan(decimal.Zero) {
		return nil, fmt.Errorf("max discount cannot be negative: got %s", maxDiscount.String())
	}
	if validUntil.Before(validFrom) {
		return nil, fmt.Errorf("valid_until cannot be before valid_from")
	}
	if maxUsagePerUser < 0 {
		return nil, fmt.Errorf("max_usage_per_user cannot be negative: got %d", maxUsagePerUser)
	}
	if totalUsageLimit < 0 {
		return nil, fmt.Errorf("total_usage_limit cannot be negative: got %d", totalUsageLimit)
	}

	if targetMode == DiscountTargetModeSellerWide {
		if len(forSaleIDs) > 0 || len(auctionIDs) > 0 {
			return nil, fmt.Errorf("for_sale_ids and auction_ids should not be set for seller_wide discounts")
		}
	}

	if targetMode == DiscountTargetModeSelectedItems {
		if len(forSaleIDs) == 0 && len(auctionIDs) == 0 {
			return nil, fmt.Errorf("for_sale_ids or auction_ids are required for selected_items discounts")
		}
	}

	if appliesTo == DiscountAppliesToForSale && len(auctionIDs) > 0 {
		return nil, fmt.Errorf("auction_ids should not be set for forSale-only discounts")
	}
	if appliesTo == DiscountAppliesToAuction && len(forSaleIDs) > 0 {
		return nil, fmt.Errorf("for_sale_ids should not be set for auction-only discounts")
	}

	now := time.Now()
	return &Discount{
		ID:                uuid.New(),
		Code:              code,
		Type:              discountType,
		Value:             value,
		MinPurchase:       minPurchase,
		MaxDiscount:       maxDiscount,
		AppliesTo:         appliesTo,
		TargetMode:        targetMode,
		SellerID:          sellerID,
		ForSaleIDs:        forSaleIDs,
		AuctionIDs:        auctionIDs,
		ValidFrom:         validFrom,
		ValidUntil:        validUntil,
		MaxUsagePerUser:   maxUsagePerUser,
		TotalUsageLimit:   totalUsageLimit,
		CurrentUsageCount: 0,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// IsActiveNow checks if the discount is currently active based on:
// - IsActive flag
// - Current time is within ValidFrom and ValidUntil
func (d *Discount) IsActiveNow() bool {
	if !d.IsActive {
		return false
	}

	now := time.Now()
	if now.Before(d.ValidFrom) || now.After(d.ValidUntil) {
		return false
	}

	return true
}

// CanBeUsedBy checks if a user can use this discount based on active state and usage limits.
func (d *Discount) CanBeUsedBy(userUsageCount int) error {
	if !d.IsActive {
		return &DiscountNotActiveError{Code: d.Code}
	}

	now := time.Now()
	if now.Before(d.ValidFrom) {
		return &DiscountValidationError{Code: d.Code, Reason: "discount not yet valid"}
	}
	if now.After(d.ValidUntil) {
		return &DiscountExpiredError{Code: d.Code, ValidUntil: d.ValidUntil}
	}

	if d.TotalUsageLimit > 0 && d.CurrentUsageCount >= d.TotalUsageLimit {
		return &DiscountUsageLimitExceededError{
			Code:         d.Code,
			UsageLimit:   d.TotalUsageLimit,
			CurrentUsage: d.CurrentUsageCount,
			IsUserLimit:  false,
		}
	}

	if d.MaxUsagePerUser > 0 && userUsageCount >= d.MaxUsagePerUser {
		return &DiscountUsageLimitExceededError{
			Code:            d.Code,
			MaxUsagePerUser: d.MaxUsagePerUser,
			UserUsageCount:  userUsageCount,
			IsUserLimit:     true,
		}
	}

	return nil
}

// MeetsMinPurchase checks if the subtotal meets the minimum purchase requirement.
func (d *Discount) MeetsMinPurchase(subtotal decimal.Decimal) error {
	if subtotal.LessThan(d.MinPurchase) {
		return &MinPurchaseNotMetError{
			Code:        d.Code,
			MinPurchase: d.MinPurchase,
			Subtotal:    subtotal,
		}
	}
	return nil
}

// ============================================================================
// ECONOMIC SAFETY VALIDATION (P0)
// ============================================================================

// ValidateEconomicSafety checks if the discount meets economic safety requirements.
func (d *Discount) ValidateEconomicSafety() error {
	if d.Type != DiscountTypePercentage {
		return nil
	}

	if d.Value.GreaterThan(decimal.NewFromInt(MaxDiscountPercentage)) {
		return &DiscountValidationError{
			Code: d.Code,
			Reason: fmt.Sprintf("discount percentage %.2f%% exceeds maximum allowed %d%% - economic safety violation",
				d.Value.InexactFloat64(), MaxDiscountPercentage),
		}
	}

	return nil
}

// CalculateDiscountAmount calculates the discount amount for a given subtotal.
func (d *Discount) CalculateDiscountAmount(subtotal decimal.Decimal) decimal.Decimal {
	var discountAmount decimal.Decimal

	switch d.Type {
	case DiscountTypePercentage:
		discountAmount = subtotal.Mul(d.Value).Div(decimal.NewFromInt(100))
	case DiscountTypeFlatAmount:
		discountAmount = d.Value
	case DiscountTypeFreeShipping:
		return decimal.Zero
	default:
		return decimal.Zero
	}

	if d.MaxDiscount != nil && discountAmount.GreaterThan(*d.MaxDiscount) {
		discountAmount = *d.MaxDiscount
	}

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
	TargetMode      DiscountTargetMode
	DiscountAmount  decimal.Decimal
	Subtotal        decimal.Decimal
	DiscountedTotal decimal.Decimal
	IsFreeShipping  bool
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
		TargetMode:      discount.TargetMode,
		DiscountAmount:  discountAmount,
		Subtotal:        subtotal,
		DiscountedTotal: subtotal.Sub(discountAmount),
		IsFreeShipping:  discount.Type == DiscountTypeFreeShipping,
	}
}

// MatchPriority returns a deterministic priority for "best discount" selection.
// Higher number = higher priority.
//
// Priority order:
// - selected_items exact context
// - seller_wide exact context
// - selected_items both-context
// - seller_wide both-context
func (d *Discount) MatchPriority(ctx DiscountContextType) int {
	if !d.AppliesTo.AllowsContext(ctx) {
		return 0
	}

	switch d.TargetMode {
	case DiscountTargetModeSelectedItems:
		if d.AppliesTo == DiscountAppliesToBoth {
			return 2
		}
		return 4
	case DiscountTargetModeSellerWide:
		if d.AppliesTo == DiscountAppliesToBoth {
			return 1
		}
		return 3
	default:
		return 0
	}
}

// IsBetterThan returns true if this discount is "better" than another discount.
func (d *Discount) IsBetterThan(other *Discount, subtotal decimal.Decimal, ctx DiscountContextType) bool {
	thisPriority := d.MatchPriority(ctx)
	otherPriority := other.MatchPriority(ctx)
	if thisPriority != otherPriority {
		return thisPriority > otherPriority
	}

	thisAmount := d.CalculateDiscountAmount(subtotal)
	otherAmount := other.CalculateDiscountAmount(subtotal)
	if !thisAmount.Equal(otherAmount) {
		return thisAmount.GreaterThan(otherAmount)
	}

	if d.Type == DiscountTypePercentage && other.Type != DiscountTypePercentage {
		return true
	}

	return false
}


