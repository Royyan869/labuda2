// DOMAIN: PRICING
// NOTE: Single-use pricing snapshot tokens for checkout validation

package entity

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	discountentity "github.com/labuda/backend/internal/pricing/discount/entity"
	"github.com/labuda/backend/pkg/money"
	"github.com/shopspring/decimal"
)

// PricingToken represents a single-use, expiring token that captures
// the complete pricing snapshot at preview time.
//
// SECURITY PROPERTIES:
// 1. Tokens are generated server-side with cryptographic randomness
// 2. Tokens expire after a short window (default 10 minutes)
// 3. Tokens are single-use (marked as used after order creation)
// 4. Tokens are bound to specific user, product, sale surface, and quantity
// 5. Pricing is immutable once token is created
type PricingToken struct {
	ID    uuid.UUID
	Token uuid.UUID

	// User and sale-surface context
	UserID     uuid.UUID
	ProductID  uuid.UUID
	SourceType string
	SourceID   uuid.UUID
	Quantity   int

	// Chat commerce context (optional, mutually exclusive)
	// Exactly one of these may be set for private commerce flows
	NegotiationID *uuid.UUID // Set when token is generated from an accepted negotiation
	AuctionID     *uuid.UUID // Set when token is generated from an auction (buy-now or winner claim)

	// Shipping context (optional, mutually exclusive with ShippingOptionID)
	// When set, shipping cost comes from ShippingQuote instead of ShippingOption
	ShippingQuoteID *uuid.UUID // Set when using manual shipping quote from seller

	// Pricing snapshot (immutable, calculated at token creation)
	UnitPrice          money.Money
	Subtotal           money.Money // quantity × unit_price
	ShippingTotal      money.Money
	CommissionPercent  int64
	CommissionAmount   money.Money
	EscrowAmount       money.Money // buyer gross escrow snapshot after discounts and before service fee
	ServiceFeeAmount   money.Money // Flat buyer checkout service fee (platform revenue)
	TotalPayableAmount money.Money // EscrowAmount + ServiceFeeAmount

	// Discount snapshot (optional)
	// DiscountID is stored for atomic usage recording during token consumption
	DiscountID     *uuid.UUID
	DiscountCode   *string
	DiscountType   *discountentity.DiscountType
	DiscountValue  *decimal.Decimal
	DiscountAmount money.Money

	// Coins snapshot (calculated at token generation)
	// MaxCoinsAllowed: maximum coins that can be applied based on canonical 20% of PD
	// CoinsUsed: actual coins applied (initially 0, set when user confirms order)
	// OrderValueForCoins: coin-cap basis (PD = subtotal - discount)
	CoinsUsed          int64
	MaxCoinsAllowed    int64
	OrderValueForCoins int64 // Pre-calculated for coins service: discounted product value (PD)

	// Shipping option snapshot
	ShippingOptionID       uuid.UUID
	ShippingOptionName     string
	ShippingTransportType  string
	ShippingExpeditionName *string
	ShippingEstimatedDays  *string

	// Address snapshot
	AddressID       uuid.UUID
	AddressSnapshot []byte // JSONB encoded address

	// Token state
	IsUsed  bool
	UsedAt  *time.Time
	OrderID *uuid.UUID

	// Expiration
	ExpiresAt time.Time

	// Audit
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DefaultTokenExpiration is the default time-to-live for pricing tokens.
const DefaultTokenExpiration = 10 * time.Minute

var (
	// ErrTokenNotFound is returned when a pricing token is not found.
	ErrTokenNotFound = errors.New("pricing token not found")

	// ErrTokenExpired is returned when a pricing token has expired.
	ErrTokenExpired = errors.New("pricing token has expired")

	// ErrTokenAlreadyUsed is returned when a pricing token has already been used.
	ErrTokenAlreadyUsed = errors.New("pricing token has already been used")

	// ErrTokenUserMismatch is returned when the token's user_id doesn't match the requester.
	ErrTokenUserMismatch = errors.New("pricing token user_id mismatch")

	// ErrTokenProductMismatch is returned when the token's product_id doesn't match the request.
	ErrTokenProductMismatch = errors.New("pricing token product_id mismatch")

	// ErrTokenQuantityMismatch is returned when the token's quantity doesn't match the request.
	ErrTokenQuantityMismatch = errors.New("pricing token quantity mismatch")

	// ErrTokenAddressMismatch is returned when the token's address_id doesn't match the request.
	ErrTokenAddressMismatch = errors.New("pricing token address_id mismatch")

	// ErrTokenShippingOptionMismatch is returned when the token's shipping_option_id doesn't match.
	ErrTokenShippingOptionMismatch = errors.New("pricing token shipping_option_id mismatch")
)

// ValidationErrorCode represents a specific validation error.
type ValidationErrorCode string

const (
	CodeTokenNotFound    ValidationErrorCode = "TOKEN_NOT_FOUND"
	CodeTokenExpired     ValidationErrorCode = "TOKEN_EXPIRED"
	CodeTokenAlreadyUsed ValidationErrorCode = "TOKEN_ALREADY_USED"
	CodeUserMismatch     ValidationErrorCode = "USER_MISMATCH"
	CodeProductMismatch  ValidationErrorCode = "PRODUCT_MISMATCH"
	CodeQuantityMismatch ValidationErrorCode = "QUANTITY_MISMATCH"
	CodeAddressMismatch  ValidationErrorCode = "ADDRESS_MISMATCH"
	CodeShippingMismatch ValidationErrorCode = "SHIPPING_MISMATCH"
)

// ValidationError represents a pricing token validation error with details.
type ValidationError struct {
	Code    ValidationErrorCode
	Message string
	OrderID *uuid.UUID
	UsedAt  *time.Time
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a new ValidationError with the given code and message.
func NewValidationError(code ValidationErrorCode, msg string) *ValidationError {
	return &ValidationError{
		Code:    code,
		Message: msg,
	}
}

// NewTokenAlreadyUsedValidationError creates a token-used validation error that preserves
// the linked order identity for safe retry recovery.
func NewTokenAlreadyUsedValidationError(usedAt *time.Time, orderID *uuid.UUID) *ValidationError {
	msg := "pricing token already used"
	if usedAt != nil {
		msg = fmt.Sprintf("pricing token already used at %s", usedAt.Format(time.RFC3339))
	}
	if orderID != nil && *orderID != uuid.Nil {
		msg = fmt.Sprintf("%s for order %s", msg, orderID.String())
	}

	return &ValidationError{
		Code:    CodeTokenAlreadyUsed,
		Message: msg,
		OrderID: orderID,
		UsedAt:  usedAt,
	}
}

// NewPricingToken creates a new PricingToken with the given parameters.
// The token is initialized as unused (is_used=false) and expires after DefaultTokenExpiration.
//
// SHIPPING SOURCE (REPLACE MODE):
// - Exactly one of shippingQuoteID or shippingOptionID must be provided
// - If shippingQuoteID is set, shipping cost comes from the manual quote
// - If shippingOptionID is set, shipping cost comes from for_sale shipping options
//
// COINS SNAPSHOT:
// - coinsUsed should be 0 for new tokens (set at order confirmation time)
// - maxCoinsAllowed is calculated at token generation using the same logic as preview
func NewPricingToken(
	userID uuid.UUID,
	productID uuid.UUID,
	sourceType string,
	sourceID uuid.UUID,
	quantity int,
	unitPrice money.Money,
	shippingTotal money.Money,
	commissionPercent int64,
	commissionAmount money.Money,
	escrowAmount money.Money,
	serviceFeeAmount money.Money,
	shippingOptionID uuid.UUID,
	shippingOptionName string,
	shippingTransportType string,
	shippingExpeditionName *string,
	shippingEstimatedDays *string,
	addressID uuid.UUID,
	addressSnapshot []byte,
	discountID *uuid.UUID, // Added for atomic discount usage recording
	discountCode *string,
	discountType *discountentity.DiscountType,
	discountValue *decimal.Decimal,
	discountAmount money.Money,
	shippingQuoteID *uuid.UUID, // Optional, when using manual shipping quote
	coinsUsed int64, // Coins applied (0 for new tokens)
	maxCoinsAllowed int64, // Max coins allowed based on canonical 20% of PD
	orderValueForCoins int64, // Pre-calculated for coins service: discounted product value (PD)
) *PricingToken {
	now := time.Now()

	return &PricingToken{
		ID:                     uuid.New(),
		Token:                  uuid.New(),
		UserID:                 userID,
		ProductID:              productID,
		SourceType:             sourceType,
		SourceID:               sourceID,
		Quantity:               quantity,
		UnitPrice:              unitPrice,
		Subtotal:               money.New(int64(quantity) * unitPrice.Int64()),
		ShippingTotal:          shippingTotal,
		CommissionPercent:      commissionPercent,
		CommissionAmount:       commissionAmount,
		EscrowAmount:           escrowAmount,
		ServiceFeeAmount:       serviceFeeAmount,
		TotalPayableAmount:     escrowAmount.Add(serviceFeeAmount),
		ShippingOptionID:       shippingOptionID,
		ShippingOptionName:     shippingOptionName,
		ShippingTransportType:  shippingTransportType,
		ShippingExpeditionName: shippingExpeditionName,
		ShippingEstimatedDays:  shippingEstimatedDays,
		AddressID:              addressID,
		AddressSnapshot:        addressSnapshot,
		DiscountID:             discountID,
		DiscountCode:           discountCode,
		DiscountType:           discountType,
		DiscountValue:          discountValue,
		DiscountAmount:         discountAmount,
		ShippingQuoteID:        shippingQuoteID,    // Store shipping quote reference
		CoinsUsed:              coinsUsed,          // Coins applied (0 for new tokens)
		MaxCoinsAllowed:        maxCoinsAllowed,    // Max coins allowed
		OrderValueForCoins:     orderValueForCoins, // Pre-calculated for coins service
		IsUsed:                 false,
		UsedAt:                 nil,
		OrderID:                nil,
		ExpiresAt:              now.Add(DefaultTokenExpiration),
		CreatedAt:              now,
		UpdatedAt:              now,
	}
}

// NewPricingTokenFromNegotiation creates a new PricingToken from an accepted negotiation.
//
// CHAT COMMERCE SAFETY:
// - The negotiation_id links the token to the private agreement
// - The unit price comes from the final negotiation offer (NOT the for_sale price)
// - Token validation ensures the negotiation is accepted and belongs to the buyer
// - Token consumption creates the order with the negotiated price
//
// COINS SNAPSHOT:
// - coinsUsed should be 0 for new tokens (set at order confirmation time)
// - maxCoinsAllowed is calculated at token generation using the same logic as preview
func NewPricingTokenFromNegotiation(
	userID uuid.UUID,
	productID uuid.UUID,
	negotiationID uuid.UUID,
	quantity int,
	unitPrice money.Money, // Negotiated price, NOT for_sale price
	shippingTotal money.Money,
	commissionPercent int64,
	commissionAmount money.Money,
	escrowAmount money.Money,
	serviceFeeAmount money.Money,
	shippingOptionID uuid.UUID,
	shippingOptionName string,
	shippingTransportType string,
	shippingExpeditionName *string,
	shippingEstimatedDays *string,
	addressID uuid.UUID,
	addressSnapshot []byte,
	discountID *uuid.UUID, // Added for atomic discount usage recording
	discountCode *string,
	discountType *discountentity.DiscountType,
	discountValue *decimal.Decimal,
	discountAmount money.Money,
	coinsUsed int64, // Coins applied (0 for new tokens)
	maxCoinsAllowed int64, // Max coins allowed based on canonical 20% of PD
	orderValueForCoins int64, // Pre-calculated for coins service: discounted product value (PD)
) *PricingToken {
	now := time.Now()

	return &PricingToken{
		ID:                     uuid.New(),
		Token:                  uuid.New(),
		UserID:                 userID,
		ProductID:              productID,
		SourceType:             "negotiation",
		SourceID:               negotiationID,
		NegotiationID:          &negotiationID,
		AuctionID:              nil,
		Quantity:               quantity,
		UnitPrice:              unitPrice,
		Subtotal:               money.New(int64(quantity) * unitPrice.Int64()),
		ShippingTotal:          shippingTotal,
		CommissionPercent:      commissionPercent,
		CommissionAmount:       commissionAmount,
		EscrowAmount:           escrowAmount,
		ServiceFeeAmount:       serviceFeeAmount,
		TotalPayableAmount:     escrowAmount.Add(serviceFeeAmount),
		ShippingOptionID:       shippingOptionID,
		ShippingOptionName:     shippingOptionName,
		ShippingTransportType:  shippingTransportType,
		ShippingExpeditionName: shippingExpeditionName,
		ShippingEstimatedDays:  shippingEstimatedDays,
		AddressID:              addressID,
		AddressSnapshot:        addressSnapshot,
		DiscountID:             discountID,
		DiscountCode:           discountCode,
		DiscountType:           discountType,
		DiscountValue:          discountValue,
		DiscountAmount:         discountAmount,
		CoinsUsed:              coinsUsed,          // Coins applied (0 for new tokens)
		MaxCoinsAllowed:        maxCoinsAllowed,    // Max coins allowed
		OrderValueForCoins:     orderValueForCoins, // Pre-calculated for coins service
		IsUsed:                 false,
		UsedAt:                 nil,
		OrderID:                nil,
		ExpiresAt:              now.Add(DefaultTokenExpiration),
		CreatedAt:              now,
		UpdatedAt:              now,
	}
}

// NewPricingTokenFromAuction creates a new PricingToken from an auction.
//
// AUCTION CHECKOUT SAFETY:
// - The auction_id links the token to the auction (buy-now or winner claim)
// - The unit price comes from the auction (buy-now price or winning bid)
// - Token validation ensures the auction is in a valid state for checkout
// - Token consumption creates the order with the auction price
//
// AUCTION PRICING GUARD:
//   - Buy-now: Treated as fixed-price checkout, promo discounts and coins ALLOWED
//   - Bid-win: Treated as competitive final price, promo discounts and coins ALLOWED
//     (Owner canonical 2026-06-16: both settlement types go through the same backend
//     pricing authority — 20% cap, commission safety, balance check — so coins are
//     permitted on bid-win claims.)
//
// This is enforced by the service layer during token generation.
//
// COINS SNAPSHOT:
// - coinsUsed should be 0 for new tokens (set at order confirmation time)
// - maxCoinsAllowed is calculated at token generation (calculated for both buy-now and bid-win)
func NewPricingTokenFromAuction(
	userID uuid.UUID,
	productID uuid.UUID,
	auctionID uuid.UUID,
	quantity int,
	unitPrice money.Money, // Buy-now price or winning bid, NOT for_sale price
	shippingTotal money.Money,
	commissionPercent int64,
	commissionAmount money.Money,
	escrowAmount money.Money,
	serviceFeeAmount money.Money,
	shippingOptionID uuid.UUID,
	shippingOptionName string,
	shippingTransportType string,
	shippingExpeditionName *string,
	shippingEstimatedDays *string,
	addressID uuid.UUID,
	addressSnapshot []byte,
	discountID *uuid.UUID, // Added for atomic discount usage recording
	discountCode *string,
	discountType *discountentity.DiscountType,
	discountValue *decimal.Decimal,
	discountAmount money.Money,
	coinsUsed int64, // Coins applied (0 for new tokens)
	maxCoinsAllowed int64, // Max coins allowed (0 for bid-win, canonical PD-based helper for buy-now)
	orderValueForCoins int64, // Pre-calculated for coins service: discounted product value (PD)
) *PricingToken {
	now := time.Now()

	return &PricingToken{
		ID:                     uuid.New(),
		Token:                  uuid.New(),
		UserID:                 userID,
		ProductID:              productID,
		SourceType:             "auction",
		SourceID:               auctionID,
		NegotiationID:          nil,
		AuctionID:              &auctionID,
		Quantity:               quantity,
		UnitPrice:              unitPrice,
		Subtotal:               money.New(int64(quantity) * unitPrice.Int64()),
		ShippingTotal:          shippingTotal,
		CommissionPercent:      commissionPercent,
		CommissionAmount:       commissionAmount,
		EscrowAmount:           escrowAmount,
		ServiceFeeAmount:       serviceFeeAmount,
		TotalPayableAmount:     escrowAmount.Add(serviceFeeAmount),
		ShippingOptionID:       shippingOptionID,
		ShippingOptionName:     shippingOptionName,
		ShippingTransportType:  shippingTransportType,
		ShippingExpeditionName: shippingExpeditionName,
		ShippingEstimatedDays:  shippingEstimatedDays,
		AddressID:              addressID,
		AddressSnapshot:        addressSnapshot,
		DiscountID:             discountID,
		DiscountCode:           discountCode,
		DiscountType:           discountType,
		DiscountValue:          discountValue,
		DiscountAmount:         discountAmount,
		CoinsUsed:              coinsUsed,          // Coins applied (0 for new tokens)
		MaxCoinsAllowed:        maxCoinsAllowed,    // Max coins allowed
		OrderValueForCoins:     orderValueForCoins, // Pre-calculated for coins service
		IsUsed:                 false,
		UsedAt:                 nil,
		OrderID:                nil,
		ExpiresAt:              now.Add(DefaultTokenExpiration),
		CreatedAt:              now,
		UpdatedAt:              now,
	}
}

// ValidateForOrder validates the pricing token for order creation.
// It checks all security constraints and returns an error if any validation fails.
//
// SHIPPING SOURCE VALIDATION:
// - If ShippingQuoteID is set, validates that the provided shippingQuoteID matches
// - If ShippingQuoteID is NOT set, validates that shippingOptionID matches
func (t *PricingToken) ValidateForOrder(
	requesterID uuid.UUID,
	productID uuid.UUID,
	sourceType string,
	sourceID uuid.UUID,
	quantity int,
	addressID uuid.UUID,
	shippingOptionID uuid.UUID,
) error {
	now := time.Now()

	// Check expiration
	if now.After(t.ExpiresAt) {
		return &ValidationError{
			Code:    CodeTokenExpired,
			Message: fmt.Sprintf("pricing token expired at %s", t.ExpiresAt.Format(time.RFC3339)),
		}
	}

	// Check if already used
	if t.IsUsed {
		return NewTokenAlreadyUsedValidationError(t.UsedAt, t.OrderID)
	}

	// Check user ID match
	if t.UserID != requesterID {
		return &ValidationError{
			Code: CodeUserMismatch,
			Message: fmt.Sprintf("pricing token user_id mismatch: token=%s, requester=%s",
				t.UserID, requesterID),
		}
	}

	// Check product ID match
	if t.ProductID != productID {
		return &ValidationError{
			Code: CodeProductMismatch,
			Message: fmt.Sprintf("pricing token product_id mismatch: token=%s, request=%s",
				t.ProductID, productID),
		}
	}

	// Check sale surface binding
	if t.SourceType != sourceType || t.SourceID != sourceID {
		return &ValidationError{
			Code: CodeShippingMismatch,
			Message: fmt.Sprintf("pricing token source mismatch: token=%s:%s, request=%s:%s",
				t.SourceType, t.SourceID, sourceType, sourceID),
		}
	}

	// Check quantity match
	if t.Quantity != quantity {
		return &ValidationError{
			Code: CodeQuantityMismatch,
			Message: fmt.Sprintf("pricing token quantity mismatch: token=%d, request=%d",
				t.Quantity, quantity),
		}
	}

	// Check address ID match
	if t.AddressID != addressID {
		return &ValidationError{
			Code: CodeAddressMismatch,
			Message: fmt.Sprintf("pricing token address_id mismatch: token=%s, request=%s",
				t.AddressID, addressID),
		}
	}

	// ============================================================================
	// SHIPPING SOURCE VALIDATION (REPLACE MODE)
	// ============================================================================
	// If ShippingQuoteID is set, we're using manual shipping quote
	// - shippingOptionID should be uuid.Nil (not used)
	// - No shipping option validation needed
	//
	// If ShippingQuoteID is NOT set, validate shippingOptionID matches
	// - This is the normal for_sale checkout flow
	if t.ShippingQuoteID != nil {
		// Shipping quote mode: shippingOptionID should be nil/empty
		if shippingOptionID != uuid.Nil {
			return &ValidationError{
				Code:    CodeShippingMismatch,
				Message: "shipping_option_id must be empty when using shipping_quote",
			}
		}
	} else {
		// Normal mode: validate shipping option ID match
		if t.ShippingOptionID != shippingOptionID {
			return &ValidationError{
				Code: CodeShippingMismatch,
				Message: fmt.Sprintf("pricing token shipping_option_id mismatch: token=%s, request=%s",
					t.ShippingOptionID, shippingOptionID),
			}
		}
	}

	return nil
}

// MarkAsUsed marks the token as used and links it to the given order ID.
// This should be called atomically with order creation.
func (t *PricingToken) MarkAsUsed(orderID uuid.UUID) error {
	if t.IsUsed {
		return ErrTokenAlreadyUsed
	}

	now := time.Now()
	t.IsUsed = true
	t.UsedAt = &now
	t.OrderID = &orderID
	t.UpdatedAt = now

	return nil
}

// IsExpired returns true if the token has expired.
func (t *PricingToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid returns true if the token is valid (not expired, not used).
func (t *PricingToken) IsValid() bool {
	return !t.IsExpired() && !t.IsUsed
}
