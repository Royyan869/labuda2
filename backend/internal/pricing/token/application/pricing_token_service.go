// ============================================================================
// PRICING TOKEN SERVICE - SOURCE OF TRUTH
// ============================================================================
//
// SOURCE OF TRUTH: All pricing calculation must go through this service.
//
// DOMAIN RULE: This service is the SINGLE SOURCE OF TRUTH for pricing.
// - Values are computed once at token generation and NEVER recomputed
// - Order creation uses snapshot values directly (NO recalculation)
// - This eliminates duplicate logic and ensures pricing consistency
//
// BUSINESS RULES:
// - Commission calculated on (subtotal - discount) net amount
// - Commission safety: final_order_value >= commission_amount
// - Economic safety: discount percentage capped at 50% maximum
//
// NO PRICING CALCULATION SHALL HAPPEN OUTSIDE THIS SERVICE.
// ============================================================================
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	auctionentity "github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepoImpl "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	forSaleentity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forSaleRepoImpl "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	forSalerepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationRepoImpl "github.com/labuda/backend/internal/commerce/negotiation/infrastructure/repository"
	negotiationrepo "github.com/labuda/backend/internal/commerce/negotiation/repository"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	shippingRepoImpl "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	shippingrepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	shippingquoteRepoImpl "github.com/labuda/backend/internal/commerce/shipping/quote/infrastructure/repository"
	shippingquoteRepo "github.com/labuda/backend/internal/commerce/shipping/quote/repository"
	addressRepoImpl "github.com/labuda/backend/internal/identity/address/infrastructure/repository"
	addressrepo "github.com/labuda/backend/internal/identity/address/repository"
	coinsapp "github.com/labuda/backend/internal/incentive/coins/application"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	discountApp "github.com/labuda/backend/internal/pricing/discount/application"
	discountentity "github.com/labuda/backend/internal/pricing/discount/entity"
	pricingtokenentity "github.com/labuda/backend/internal/pricing/token/entity"
	pricingtokenrepoimpl "github.com/labuda/backend/internal/pricing/token/infrastructure/repository"
	pricingtokenrepo "github.com/labuda/backend/internal/pricing/token/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/shopspring/decimal"
)

// PricingTokenService handles pricing token generation and validation.
//
// ============================================================================
// PRICING TOKEN IS AUTHORITATIVE (PURE PRICING LAYER)
// ============================================================================
// CRITICAL ARCHITECTURAL RULE:
// - PricingToken is the SINGLE SOURCE OF TRUTH for pricing
// - Values are computed once at token generation and NEVER recomputed
// - Order creation uses snapshot values directly (NO recalculation)
// - This eliminates duplicate logic and ensures pricing consistency
// - Coins are NOT handled at pricing layer - they are calculated at order layer
// ============================================================================
// - Pricing is ALWAYS computed by backend at token generation time
// - Pricing is NEVER provided by client or computed at order creation
// - Token stores immutable snapshot for validation during order creation
//
// This prevents:
// - Client-side price manipulation
// - Race conditions between preview and order creation
// - Stale pricing data being used for transactions
//
// FLOW:
// 1. GenerateForForSale/Negotiation/Auction → Token (PREVIEW)
// 2. Client displays preview to user
// 3. ValidateAndConsume → validates snapshot, marks token used, returns snapshot
// 4. Order creation uses the validated snapshot from token
// 5. Coins are calculated and applied AT ORDER LAYER (not pricing)
//
// CHAT COMMERCE EXTENSION:
// - Supports token generation from accepted negotiations
// - Private agreement prices are used instead of forSale prices
//
// AUCTION EXTENSION:
// - Supports token generation from auctions (buy-now and winner claim)
// - Auction prices are authoritative (buy-now price or winning bid)
type PricingTokenService struct {
	tokenRepo       pricingtokenrepo.PricingTokenRepository
	forSaleRepo     forSalerepo.ForSaleRepository
	shippingRepo    shippingrepo.ShippingOptionRepository
	coverageRepo    shippingrepo.ShippingCoverageRepository
	addressRepo     addressrepo.AddressRepository
	configService   *platformconfigApp.ConfigService
	auctionRepo     *auctionRepoImpl.AuctionRepository
	discountService *discountApp.DiscountService
	// Chat commerce dependencies (added for safety closure)
	negotiationRepo negotiationrepo.Repository
	// Shipping quote dependency (added for manual shipping quotes)
	shippingQuoteRepo shippingquoteRepo.ShippingQuoteRepository
}

// NewPricingTokenService creates a new PricingTokenService.
func NewPricingTokenService(
	configService *platformconfigApp.ConfigService,
) *PricingTokenService {
	return &PricingTokenService{
		tokenRepo:         pricingtokenrepoimpl.NewPricingTokenRepository(),
		forSaleRepo:       forSaleRepoImpl.NewForSaleRepository(),
		shippingRepo:      shippingRepoImpl.NewShippingOptionRepository(),
		coverageRepo:      shippingRepoImpl.NewShippingCoverageRepository(),
		addressRepo:       addressRepoImpl.NewAddressRepository(),
		configService:     configService,
		auctionRepo:       auctionRepoImpl.NewAuctionRepository(),
		discountService:   discountApp.NewDiscountService(),
		negotiationRepo:   negotiationRepoImpl.NewNegotiationRepository(),
		shippingQuoteRepo: shippingquoteRepoImpl.NewShippingQuoteRepository(),
	}
}

// GenerateForForSaleRequest contains the parameters for generating a pricing token.
//
// SHIPPING SOURCE (REPLACE MODE):
// - Exactly one of ShippingQuoteID or ShippingOptionID must be provided
// - If ShippingQuoteID is set, shipping cost comes from the manual quote
// - If ShippingOptionID is set, shipping cost comes from forSale shipping options
// - Providing both or neither is a validation error
type GenerateForForSaleRequest struct {
	UserID           uuid.UUID
	ProductID        uuid.UUID
	SourceType       string
	SourceID         uuid.UUID
	Quantity         int
	ShippingOptionID *uuid.UUID // Optional: Pointer to allow nil when using ShippingQuote
	ShippingQuoteID  *uuid.UUID // Optional: When set, uses manual shipping quote
	AddressID        uuid.UUID
	DiscountCode     *string
}

// GenerateForForSaleResponse contains the generated pricing token and its snapshot.
type GenerateForForSaleResponse struct {
	Token           uuid.UUID
	ExpiresAt       string
	PricingSnapshot PricingSnapshot
}

// PricingSnapshot contains the complete pricing breakdown.
type PricingSnapshot struct {
	UnitPrice          money.Money      `json:"unit_price"`
	Quantity           int              `json:"quantity"`
	Subtotal           money.Money      `json:"subtotal"`
	ShippingTotal      money.Money      `json:"shipping_total"`
	CommissionPercent  int64            `json:"commission_percent"`
	CommissionAmount   money.Money      `json:"commission_amount"`
	DiscountAmount     money.Money      `json:"discount_amount"`
	ServiceFeeAmount   money.Money      `json:"service_fee_amount"`
	TotalPayableAmount money.Money      `json:"total_payable_amount"`
	DiscountCode       *string          `json:"discount_code,omitempty"`
	DiscountType       *string          `json:"discount_type,omitempty"`
	DiscountValue      *decimal.Decimal `json:"discount_value,omitempty"`
	EscrowAmount       money.Money      `json:"escrow_amount"`

	// ============================================================================
	// SHIPPING MODE INDICATOR (UI CONTRACT FIX)
	// ============================================================================
	// ShippingMode indicates the shipping source for UI to properly display:
	// - "quote": Manual shipping quote from seller (no shipping option selection)
	// - "standard": Standard forSale shipping options (user selects shipping)
	//
	// This allows UI to:
	// - Hide shipping option dropdown when mode is "quote"
	// - Show appropriate labels ("Pengiriman (Hasil Negosiasi)" vs "Pilih Pengiriman")
	// - Prevent dual source confusion
	ShippingMode string `json:"shipping_mode"` // "standard" | "quote"

	// ============================================================================
	// COINS PREVIEW (NON-BINDING)
	// ============================================================================
	// This field provides a PREVIEW of how many coins the user COULD apply.
	// It is NOT stored in the token and NOT used for order creation.
	//
	// The actual coins used is determined at ORDER CREATION time based on:
	// 1. User's choice (use_coins boolean flag)
	// 2. User's current active balance
	// 3. 20% max discount rule
	// 4. Commission safety constraint
	//
	// This preview is for UI display ONLY - the backend decides the final amount.
	CoinsPreview *CoinsPreview `json:"coins_preview,omitempty"`
}

// CoinsPreview contains non-binding coins information for UI display.
type CoinsPreview struct {
	// MaxApplicable is the maximum coins that can be applied based on:
	// - 20% of order value rule
	// - Commission safety constraint
	// Does NOT consider user balance (checked at order creation)
	MaxApplicable int64 `json:"max_applicable"`
}

// GenerateForForSale generates a pricing token for a forSale purchase.
// It calculates the complete pricing snapshot and stores it for later validation.
//
// SHIPPING SOURCE (REPLACE MODE):
// - Exactly one of ShippingQuoteID or ShippingOptionID must be provided
// - If ShippingQuoteID is set, shipping cost comes from the manual quote
// - If ShippingOptionID is set, shipping cost comes from forSale shipping options
// - Providing both or neither is a validation error
func (s *PricingTokenService) GenerateForForSale(
	ctx context.Context,
	tx db.Tx,
	req *GenerateForForSaleRequest,
) (*GenerateForForSaleResponse, error) {
	// ============================================================================
	// STEP 0: VALIDATE SHIPPING SOURCE (EXACTLY ONE REQUIRED)
	// ============================================================================
	hasShippingQuote := req.ShippingQuoteID != nil && *req.ShippingQuoteID != uuid.Nil
	hasShippingOption := req.ShippingOptionID != nil && *req.ShippingOptionID != uuid.Nil

	if hasShippingQuote && hasShippingOption {
		return nil, fmt.Errorf("invalid shipping source: both shipping_quote_id and shipping_option_id cannot be provided")
	}
	if !hasShippingQuote && !hasShippingOption {
		return nil, fmt.Errorf("invalid shipping source: either shipping_quote_id or shipping_option_id must be provided")
	}

	// Validate quantity
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("invalid quantity: %d", req.Quantity)
	}

	// Fetch product authority using the ForSale UUID (source_id)
	forSale, err := s.forSaleRepo.GetByID(ctx, tx, req.SourceID)
	if err != nil {
		return nil, fmt.Errorf("forSale not found: %w", err)
	}

	// Validate forSale is active
	if forSale.Status != forSaleentity.ForSaleStatusActive {
		return nil, fmt.Errorf("forSale is not active: %s", forSale.Status)
	}

	// Validate sufficient stock
	if forSale.QuantityAvailable < req.Quantity {
		return nil, fmt.Errorf("insufficient stock: requested=%d, available=%d", req.Quantity, forSale.QuantityAvailable)
	}

	// Prevent self-purchase
	if forSale.SellerID == req.UserID {
		return nil, errors.New("cannot purchase own forSale")
	}

	// ============================================================================
	// STEP 1: DETERMINE SHIPPING COST AND DETAILS
	// ============================================================================
	var shippingTotal money.Money
	var shippingOptionID uuid.UUID
	var shippingOptionName string
	var shippingTransportType string
	var shippingExpeditionName *string
	var estimatedDays *string

	if hasShippingQuote {
		// ========================================================================
		// SHIPPING QUOTE MODE: Use manual shipping quote
		// ========================================================================
		quote, err := s.shippingQuoteRepo.GetByID(ctx, tx, *req.ShippingQuoteID)
		if err != nil {
			return nil, fmt.Errorf("shipping quote not found: %w", err)
		}

		// Validate quote belongs to the same forSale
		if quote.ProductID != req.ProductID {
			return nil, fmt.Errorf("shipping quote product mismatch: quote_product=%s, request_product=%s",
				quote.ProductID, req.ProductID)
		}
		if quote.SourceType == nil || quote.SourceID == nil || *quote.SourceType != req.SourceType || *quote.SourceID != req.SourceID {
			quoteSourceType := "<nil>"
			quoteSourceID := "<nil>"
			if quote.SourceType != nil {
				quoteSourceType = *quote.SourceType
			}
			if quote.SourceID != nil {
				quoteSourceID = quote.SourceID.String()
			}
			return nil, fmt.Errorf("shipping quote source mismatch: quote=%s:%s request=%s:%s",
				quoteSourceType, quoteSourceID, req.SourceType, req.SourceID)
		}

		// Validate quote is for the same buyer
		if quote.BuyerID != req.UserID {
			return nil, fmt.Errorf("shipping quote buyer mismatch: quote_buyer=%s, requester=%s",
				quote.BuyerID, req.UserID)
		}

		// Use quote cost as shipping total
		shippingTotal = quote.Cost

		// For shipping quotes, use empty/placeholder values for shipping option details
		// The actual shipping method will be determined by seller during fulfillment
		shippingOptionID = uuid.Nil
		shippingOptionName = "Manual Quote"
		shippingTransportType = "manual"
		shippingExpeditionName = quote.Note // Store quote note as expedition name for reference
		estimatedDays = nil                 // No ETA for manual quotes
	} else {
		// ========================================================================
		// SHIPPING OPTION MODE: Use forSale shipping options
		// ========================================================================
		// Fetch shipping option
		shippingOption, err := s.shippingRepo.GetByID(ctx, tx, *req.ShippingOptionID)
		if err != nil {
			return nil, fmt.Errorf("shipping option not found: %w", err)
		}

		// Get buyer province for shipping coverage lookup
		provinceCode, _, err := s.getAddressWithProvince(ctx, tx, req.AddressID)
		if err != nil {
			return nil, fmt.Errorf("failed to get buyer province: %w", err)
		}

		// Get province-based shipping cost and ETA from ShippingCoverage
		shippingTotal, estimatedDays, err = s.getShippingCostAndETA(ctx, tx, *req.ShippingOptionID, provinceCode)
		if err != nil {
			return nil, fmt.Errorf("failed to get shipping cost for province %s: %w", provinceCode, err)
		}

		// Store shipping option details
		shippingOptionID = shippingOption.ID
		shippingOptionName = shippingOption.Name
		shippingTransportType = string(shippingOption.TransportType)
		shippingExpeditionName = shippingOption.ExpeditionName
	}

	// Calculate base pricing
	unitPrice := forSale.PricePerUnit
	subtotal := money.New(int64(req.Quantity) * unitPrice.Int64())

	// Initialize discount values
	var discountID *uuid.UUID
	var discountCode *string
	var discountType *discountentity.DiscountType
	var discountValue *decimal.Decimal
	var discountAmount money.Money = money.Zero()

	// Apply discount if provided
	// DISCOUNT TRUTH: Validate and calculate discount using canonical discount domain
	// No placeholder acceptance - either valid discount or error returned
	if req.DiscountCode != nil && *req.DiscountCode != "" {
		result, err := s.discountService.ApplyDiscountAtCheckout(
			ctx,
			tx,
			req.UserID,
			*req.DiscountCode,
			subtotal.Int64(),
			discountentity.DiscountContextForSale,
			&forSale.SellerID,
		)
		if err != nil {
			// Return validation error to user - honest failure instead of silent ignore
			return nil, fmt.Errorf("discount validation failed: %w", err)
		}

		// Store validated discount details in pricing token
		// CRITICAL: Store DiscountID for atomic usage recording during token consumption
		discountID = &result.DiscountID
		discountCode = &result.Code
		dt := result.Type
		discountType = &dt
		discountValue = &result.Value
		discountAmount = money.New(result.DiscountAmount.IntPart())
	}

	// ============================================================================
	// CANONICAL POST-DISCOUNT MONEY FLOW
	// ============================================================================
	commissionPercent := s.configService.GetForSaleCommission(ctx, tx)
	postDiscount, err := calculatePostDiscountMoneyFlow(subtotal, discountAmount, shippingTotal, commissionPercent)
	if err != nil {
		return nil, err
	}
	coinsUsed := int64(0) // Initially 0, set when user confirms order

	// Fetch address snapshot
	addressSnapshot, err := s.getAddressSnapshot(ctx, tx, req.AddressID)
	if err != nil {
		return nil, fmt.Errorf("failed to get address snapshot: %w", err)
	}

	// Create pricing token with shipping quote ID if applicable
	token := pricingtokenentity.NewPricingToken(
		req.UserID,
		req.ProductID,
		req.SourceType,
		req.SourceID,
		req.Quantity,
		unitPrice,
		shippingTotal,
		commissionPercent.IntPart(),
		postDiscount.CommissionAmount,
		postDiscount.EscrowAmount,
		money.Zero(),
		shippingOptionID,
		shippingOptionName,
		shippingTransportType,
		shippingExpeditionName,
		estimatedDays,
		req.AddressID,
		addressSnapshot,
		discountID, // Added for atomic discount usage recording
		discountCode,
		discountType,
		discountValue,
		discountAmount,
		req.ShippingQuoteID, // Optional: Pass shipping quote ID
		coinsUsed,           // Coins applied (0 for new tokens)
		postDiscount.MaxCoinsAllowed,     // Max coins allowed based on canonical 20% of PD
		postDiscount.OrderValueForCoins,  // Pre-calculated for coins service: discounted product value (PD)
	)

	// Store token
	if err := s.tokenRepo.CreateTx(ctx, tx, token); err != nil {
		return nil, fmt.Errorf("failed to create pricing token: %w", err)
	}

	// Prepare response
	var discountTypeStr *string
	if discountType != nil {
		dt := string(*discountType)
		discountTypeStr = &dt
	}

	// Calculate coins preview (non-binding, for UI display only)
	var coinsPreview *CoinsPreview
	if postDiscount.MaxCoinsAllowed > 0 {
		coinsPreview = &CoinsPreview{
			MaxApplicable: postDiscount.MaxCoinsAllowed,
		}
	}

	// Determine shipping mode based on whether we're using a shipping quote
	// This allows the UI to properly display shipping options vs fixed quote
	shippingMode := "standard"
	if hasShippingQuote {
		shippingMode = "quote"
	}

	return &GenerateForForSaleResponse{
		Token:     token.Token,
		ExpiresAt: token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		PricingSnapshot: PricingSnapshot{
			UnitPrice:          unitPrice,
			Quantity:           req.Quantity,
			Subtotal:           subtotal,
			ShippingTotal:      shippingTotal,
			CommissionPercent:  commissionPercent.IntPart(),
			CommissionAmount:   postDiscount.CommissionAmount,
			DiscountAmount:     discountAmount,
			ServiceFeeAmount:   money.Zero(),
			TotalPayableAmount: postDiscount.TotalPayableAmount,
			DiscountCode:       discountCode,
			DiscountType:       discountTypeStr,
			DiscountValue:      discountValue,
			EscrowAmount:       postDiscount.EscrowAmount,
			ShippingMode:       shippingMode,
			CoinsPreview:       coinsPreview,
		},
	}, nil
}

// ValidateForOrderLocked validates a pricing token for order creation without consuming it.
//
// ============================================================================
// PREVIEW-ONLY SEMANTICS ENFORCEMENT
// ============================================================================
// CRITICAL: This method enforces that pricing is always computed by the backend.
//
// The pricing token was created at preview time with a complete pricing snapshot.
// This method validates that the snapshot hasn't been tampered with and that
// all parameters match exactly what was previewed to the user.
//
// Returned snapshot is IMMUTABLE and used directly for order creation.
// No pricing recalculation happens at order creation time - the snapshot
// from the validated token is the source of truth.
//
// This should be called atomically with order creation (within same transaction).
// The token row is locked FOR UPDATE so no concurrent transaction can consume it
// before this transaction either finalizes or rolls back.
func (s *PricingTokenService) ValidateForOrderLocked(
	ctx context.Context,
	tx db.Tx,
	token uuid.UUID,
	requesterID uuid.UUID,
	productID uuid.UUID,
	sourceType string,
	sourceID uuid.UUID,
	quantity int,
	addressID uuid.UUID,
	shippingOptionID uuid.UUID,
) (*pricingtokenentity.PricingToken, error) {
	// Fetch token with FOR UPDATE lock to prevent concurrent use
	pricingToken, err := s.tokenRepo.GetByTokenForUpdate(ctx, tx, token)
	if err != nil {
		if errors.Is(err, pricingtokenentity.ErrTokenNotFound) {
			return nil, pricingtokenentity.NewValidationError(
				pricingtokenentity.CodeTokenNotFound,
				"pricing token not found",
			)
		}
		return nil, fmt.Errorf("failed to fetch pricing token: %w", err)
	}

	quantityToValidate := quantity
	if quantityToValidate <= 0 {
		quantityToValidate = pricingToken.Quantity
	}

	// Validate token for order creation
	if err := pricingToken.ValidateForOrder(
		requesterID,
		productID,
		sourceType,
		sourceID,
		quantityToValidate,
		addressID,
		shippingOptionID,
	); err != nil {
		return nil, err
	}

	return pricingToken, nil
}

// FinalizeOrderConsumption marks a previously validated token as used and links it to the real order.
//
// This MUST be called in the same transaction as ValidateForOrder and order creation.
// If the transaction rolls back, both the order row and token usage rollback together.
func (s *PricingTokenService) FinalizeOrderConsumption(
	ctx context.Context,
	tx db.Tx,
	pricingToken *pricingtokenentity.PricingToken,
	orderID uuid.UUID,
) error {
	// ============================================================
	// ATOMIC DISCOUNT USAGE RECORDING (STEP 6)
	// ============================================================
	// Record discount usage atomically with token consumption.
	// This ensures discount cannot be used twice in concurrent requests.
	//
	// If pricingToken.DiscountID is set, the discount is validated
	// and its usage is recorded in the same transaction as token marking.
	// This prevents race conditions where discount could be used multiple times.
	// ============================================================
	if pricingToken.DiscountID != nil {
		if err := s.discountService.RecordUsage(
			ctx,
			tx,
			*pricingToken.DiscountID,
			pricingToken.UserID,
			orderID,
		); err != nil {
			return fmt.Errorf("failed to record discount usage: %w", err)
		}
	}

	// Mark token as used
	if err := s.tokenRepo.MarkAsUsedTx(ctx, tx, pricingToken.ID, orderID); err != nil {
		return fmt.Errorf("failed to mark token as used: %w", err)
	}

	return nil
}

// ValidateAndConsume validates a pricing token and marks it as used.
//
// Deprecated for order creation flows that need the real order ID. Prefer:
// 1. ValidateForOrderLocked
// 2. Create order
// 3. FinalizeOrderConsumption
func (s *PricingTokenService) ValidateAndConsume(
	ctx context.Context,
	tx db.Tx,
	token uuid.UUID,
	requesterID uuid.UUID,
	productID uuid.UUID,
	sourceType string,
	sourceID uuid.UUID,
	quantity int,
	addressID uuid.UUID,
	shippingOptionID uuid.UUID,
	orderID uuid.UUID,
) (*pricingtokenentity.PricingToken, error) {
	pricingToken, err := s.ValidateForOrderLocked(
		ctx,
		tx,
		token,
		requesterID,
		productID,
		sourceType,
		sourceID,
		quantity,
		addressID,
		shippingOptionID,
	)
	if err != nil {
		return nil, err
	}

	if err := s.FinalizeOrderConsumption(ctx, tx, pricingToken, orderID); err != nil {
		return nil, err
	}

	return pricingToken, nil
}

// GetSnapshot retrieves a pricing token's snapshot without consuming it.
// Useful for re-displaying order details before final confirmation.
func (s *PricingTokenService) GetSnapshot(
	ctx context.Context,
	tx db.Tx,
	token uuid.UUID,
) (*pricingtokenentity.PricingToken, error) {
	pricingToken, err := s.tokenRepo.GetByToken(ctx, tx, token)
	if err != nil {
		return nil, err
	}
	return pricingToken, nil
}

// Helper functions

// calculateCommission calculates the commission amount.
func calculateCommission(subtotal money.Money, percent decimal.Decimal) money.Money {
	commission := subtotal.Int64() * percent.IntPart() / 100
	return money.New(commission)
}

// ============================================================================
// CANONICAL POST-DISCOUNT MONEY FLOW
// ============================================================================
//
// Every pricing model (For Sale, Negotiation, Auction) converges into this
// single canonical money-flow calculation after discount is applied.
//
// Input:
//   P    = final product transaction price (subtotal before discount)
//   D    = discount amount (from DiscountService)
//   S    = shipping cost
//   C%   = commission percent (rate varies by selling surface)
//
// Output:
//   PD             = P - D (discounted product value)
//   Commission     = f(PD, C%)
//   CommissionSafe = PD + S >= Commission (rejection if false)
//   Escrow         = PD + S
//   CoinCap        = 20% × PD
//   OrderValueForCoins = PD
//
// This function is the SINGLE authority for post-discount money calculations.
// No path-specific money logic may exist outside this function.
type PostDiscountMoneyFlow struct {
	DiscountedProduct int64
	CommissionAmount  money.Money
	EscrowAmount      money.Money
	TotalPayableAmount money.Money
	MaxCoinsAllowed   int64
	OrderValueForCoins int64
}

func calculatePostDiscountMoneyFlow(
	subtotal money.Money,
	discountAmount money.Money,
	shippingTotal money.Money,
	commissionPercent decimal.Decimal,
) (*PostDiscountMoneyFlow, error) {
	// PD = P - D
	netSubtotal := subtotal.Sub(discountAmount)
	PD := subtotal.Int64() - discountAmount.Int64()

	// Commission = f(PD, C%)
	commissionAmount := calculateCommission(netSubtotal, commissionPercent)

	// Commission safety: PD + S >= Commission
	finalOrderValue := PD + shippingTotal.Int64()
	if finalOrderValue < commissionAmount.Int64() {
		return nil, fmt.Errorf(
			"discount rejected: final_order_value (%d) < commission_amount (%d): discount cannot reduce order value below safe commission threshold",
			finalOrderValue, commissionAmount.Int64())
	}

	// Escrow = PD + S
	escrowAmount := money.New(PD).Add(shippingTotal)

	// Total payable = Escrow + ServiceFee (fee unknown at preview time)
	serviceFeeAmount := money.Zero()
	totalPayableAmount := escrowAmount.Add(serviceFeeAmount)

	// Coins = 20% × PD
	orderValueForCoins := PD
	maxCoinsAllowed := coinsapp.MaxCoinsAllowedForDiscountedProduct(orderValueForCoins)

	return &PostDiscountMoneyFlow{
		DiscountedProduct:  PD,
		CommissionAmount:   commissionAmount,
		EscrowAmount:       escrowAmount,
		TotalPayableAmount: totalPayableAmount,
		MaxCoinsAllowed:    maxCoinsAllowed,
		OrderValueForCoins: orderValueForCoins,
	}, nil
}

// getAddressSnapshot retrieves an address as a JSON snapshot.
func (s *PricingTokenService) getAddressSnapshot(
	ctx context.Context,
	tx db.Tx,
	addressID uuid.UUID,
) ([]byte, error) {
	address, err := s.addressRepo.GetByID(ctx, tx, addressID)
	if err != nil {
		return nil, err
	}
	// Convert to JSON snapshot
	return json.Marshal(map[string]interface{}{
		"id":             address.ID,
		"recipient_name": address.RecipientName,
		"phone":          address.Phone,
		"province_id":    address.ProvinceID,
		"province_name":  address.ProvinceName,
		"city_id":        address.CityID,
		"city_name":      address.CityName,
		"district_id":    address.DistrictID,
		"district_name":  address.DistrictName,
		"village_id":     address.VillageID,
		"village_name":   address.VillageName,
		"street_address": address.StreetAddress,
		"postal_code":    address.PostalCode,
		"latitude":       address.Latitude,
		"longitude":      address.Longitude,
	})
}

// getAddressWithProvince retrieves an address with province information.
// Returns the province code for shipping coverage lookup.
func (s *PricingTokenService) getAddressWithProvince(
	ctx context.Context,
	tx db.Tx,
	addressID uuid.UUID,
) (provinceCode string, provinceName string, err error) {
	address, err := s.addressRepo.GetByID(ctx, tx, addressID)
	if err != nil {
		return "", "", fmt.Errorf("address not found: %w", err)
	}
	if address.ProvinceID == "" {
		return "", "", fmt.Errorf("address missing province_id: %s", addressID)
	}
	return address.ProvinceID, address.ProvinceName, nil
}

// getShippingCostAndETA retrieves the province-based shipping cost and ETA.
// Uses ShippingCoverage as the canonical source for pricing.
// Returns explicit error if coverage is not found for the province.
func (s *PricingTokenService) getShippingCostAndETA(
	ctx context.Context,
	tx db.Tx,
	shippingOptionID uuid.UUID,
	provinceCode string,
) (shippingCost money.Money, estimatedDays *string, err error) {
	coverage, err := s.coverageRepo.GetByOptionAndProvince(ctx, tx, shippingOptionID, provinceCode)
	if err != nil {
		return money.Zero(), nil, fmt.Errorf("shipping coverage not found for province %s: %w", provinceCode, err)
	}
	if !coverage.IsAvailable {
		return money.Zero(), nil, fmt.Errorf("shipping not available for province %s", provinceCode)
	}
	return coverage.ProvinceRate, coverage.EstimatedDays, nil
}

// ValidateForOrderRequest represents the validation request parameters.
//
// SHIPPING SOURCE (REPLACE MODE):
// - Exactly one of ShippingQuoteID or ShippingOptionID must be provided
// - If ShippingQuoteID is set, shipping comes from manual quote
// - If ShippingOptionID is set, shipping comes from forSale options
type ValidateForOrderRequest struct {
	Token            uuid.UUID
	RequesterID      uuid.UUID
	ProductID        uuid.UUID
	SourceType       string
	SourceID         uuid.UUID
	Quantity         int
	AddressID        uuid.UUID
	ShippingOptionID *uuid.UUID // Optional: Pointer to allow nil when using ShippingQuote
	ShippingQuoteID  *uuid.UUID // Optional: When set, uses manual shipping quote
}

// ValidateForOrder validates a pricing token without consuming it.
// Returns the pricing snapshot if valid, or an error if validation fails.
func (s *PricingTokenService) ValidateForOrder(
	ctx context.Context,
	tx db.Tx,
	req *ValidateForOrderRequest,
) (*pricingtokenentity.PricingToken, error) {
	// Fetch token (without lock for read-only validation)
	pricingToken, err := s.tokenRepo.GetByToken(ctx, tx, req.Token)
	if err != nil {
		if errors.Is(err, pricingtokenentity.ErrTokenNotFound) {
			return nil, pricingtokenentity.NewValidationError(
				pricingtokenentity.CodeTokenNotFound,
				"pricing token not found",
			)
		}
		return nil, fmt.Errorf("failed to fetch pricing token: %w", err)
	}

	// Determine shipping option ID for validation
	// If token has ShippingQuoteID, pass uuid.Nil for shipping option validation
	shippingOptionID := uuid.Nil
	if req.ShippingOptionID != nil {
		shippingOptionID = *req.ShippingOptionID
	}

	// Validate token for order creation
	if err := pricingToken.ValidateForOrder(
		req.RequesterID,
		req.ProductID,
		req.SourceType,
		req.SourceID,
		req.Quantity,
		req.AddressID,
		shippingOptionID,
	); err != nil {
		return nil, err
	}

	return pricingToken, nil
}

// ============================================================================
// CHAT COMMERCE: GENERATE TOKEN FROM NEGOTIATION
// ============================================================================

// GenerateForNegotiationRequest contains parameters for generating a pricing token from an accepted negotiation.
type GenerateForNegotiationRequest struct {
	UserID           uuid.UUID
	NegotiationID    uuid.UUID
	AddressID        uuid.UUID
	ShippingOptionID uuid.UUID
	DiscountCode     *string
}

// GenerateForNegotiationResponse contains the generated pricing token and its snapshot.
type GenerateForNegotiationResponse struct {
	Token           uuid.UUID
	ExpiresAt       string
	PricingSnapshot PricingSnapshot
}

// GenerateForNegotiation generates a pricing token from an accepted negotiation.
//
// CRITICAL INVARIANT:
// Order price MUST come from negotiation_sessions.accepted_price in the database.
// NEVER from external input or parameters. This prevents price manipulation attacks.
//
// CHAT COMMERCE SAFETY:
// - Validates negotiation is accepted and belongs to the buyer
// - Uses negotiated price (NOT forSale price) as unit price
// - Token is linked to negotiation_id for validation
// - Token consumption creates order with negotiated price
//
// DISCOUNT: Negotiation supports promo discounts.
// - Discount is calculated against the final negotiated price (P).
// - The negotiated price is the final transaction price, not the forSale price.
func (s *PricingTokenService) GenerateForNegotiation(
	ctx context.Context,
	tx db.Tx,
	req *GenerateForNegotiationRequest,
) (*GenerateForNegotiationResponse, error) {
	// ============================================================
	// DISCOUNT: Negotiation discounts use the final negotiated price (P).
	// The negotiated price is the final transaction price, not the forSale price.
	// Discount is applied during the subsequent checkout/pricing-token flow.
	// ============================================================
	// STEP 1: VALIDATE NEGOTIATION
	// ============================================================
	session, err := s.negotiationRepo.GetSession(ctx, tx, req.NegotiationID)
	if err != nil {
		return nil, fmt.Errorf("negotiation session not found: %w", err)
	}

	// Guard: Negotiation must be accepted
	if session.Status != negotiationEntity.NegotiationStatusAccepted {
		return nil, fmt.Errorf("negotiation is not accepted: current_status=%s", session.Status)
	}

	// Guard: Negotiation must not be expired
	// NEGOTIATION EXPIRY CONSISTENCY: Prevent pricing token generation for expired negotiations
	// Even if status is "accepted", an expired negotiation should not be settleable
	if session.IsExpired() {
		return nil, fmt.Errorf("negotiation expired: session_id=%s, expired_at=%v",
			session.ID, session.ExpiresAt)
	}

	// Guard: User must be the buyer
	if session.BuyerID != req.UserID {
		return nil, fmt.Errorf("negotiation buyer mismatch: session_buyer=%s, requester=%s",
			session.BuyerID, req.UserID)
	}

	// CRITICAL: Validate accepted_price is set
	// This is the ONLY source of truth for order unit price
	if session.AcceptedPrice == nil {
		return nil, fmt.Errorf("negotiation accepted_price not set: session_id=%s, status=%s",
			session.ID, session.Status)
	}

	// Guard: Not already settled (duplicate order prevention)
	if session.OrderID != nil {
		return nil, fmt.Errorf("negotiation already settled: order_id=%s", *session.OrderID)
	}

	// ============================================================
	// STEP 3: VALIDATE SALE SURFACE / PRODUCT
	// ============================================================
	if session.ForSaleID == uuid.Nil {
		return nil, fmt.Errorf("negotiation session has no for_sale_id")
	}
	forSale, err := s.forSaleRepo.GetByID(ctx, tx, session.ForSaleID)
	if err != nil {
		return nil, fmt.Errorf("fixed price sale not found: %w", err)
	}

	if forSale.Status != forSaleentity.ForSaleStatusActive {
		return nil, fmt.Errorf("forSale is not active: %s", forSale.Status)
	}

	if forSale.QuantityAvailable < 1 {
		return nil, fmt.Errorf("insufficient stock: available=%d", forSale.QuantityAvailable)
	}

	// Prevent self-purchase
	if forSale.SellerID == req.UserID {
		return nil, errors.New("cannot purchase own forSale")
	}

	// ============================================================
	// STEP 4: VALIDATE SHIPPING OPTION AND GET PROVINCE-BASED PRICING
	// ============================================================
	shippingOption, err := s.shippingRepo.GetByID(ctx, tx, req.ShippingOptionID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}

	// Get buyer province for shipping coverage lookup
	provinceCode, _, err := s.getAddressWithProvince(ctx, tx, req.AddressID)
	if err != nil {
		return nil, fmt.Errorf("failed to get buyer province: %w", err)
	}

	// Get province-based shipping cost and ETA from ShippingCoverage
	shippingTotal, estimatedDays, err := s.getShippingCostAndETA(ctx, tx, req.ShippingOptionID, provinceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get shipping cost for province %s: %w", provinceCode, err)
	}

	// ============================================================
	// STEP 4: CALCULATE PRICING (using negotiated price)
	// ============================================================
	// CRITICAL: Use accepted price from negotiation session (NOT forSale price)
	// Price is validated above to be non-nil
	unitPrice := money.New(*session.AcceptedPrice)
	subtotal := unitPrice // Negotiation is always for 1 item

	// Get commission rate
	commissionPercent := s.configService.GetForSaleCommission(ctx, tx)

	// Initialize discount values
	var discountCode *string
	var discountType *discountentity.DiscountType
	var discountValue *decimal.Decimal
	var discountAmount money.Money = money.Zero()

	// Apply discount if provided and valid for the for_sale context.
	// DISCOUNT TRUTH: Negotiation uses the final negotiated price as P.
	// Discount is calculated from the negotiated price, not the original forSale price.
	var discountID *uuid.UUID
	if req.DiscountCode != nil && *req.DiscountCode != "" {
		result, err := s.discountService.ApplyDiscountAtCheckout(
			ctx,
			tx,
			req.UserID,
			*req.DiscountCode,
			subtotal.Int64(),
			discountentity.DiscountContextForSale,
			&forSale.SellerID,
		)
		if err != nil {
			return nil, fmt.Errorf("discount validation failed: %w", err)
		}
		discountID = &result.DiscountID
		discountCode = &result.Code
		dt := result.Type
		discountType = &dt
		discountValue = &result.Value
		discountAmount = money.New(result.DiscountAmount.IntPart())
	}

	// ============================================================================
	// CANONICAL POST-DISCOUNT MONEY FLOW
	// ============================================================================
	postDiscount, err := calculatePostDiscountMoneyFlow(subtotal, discountAmount, shippingTotal, commissionPercent)
	if err != nil {
		return nil, err
	}
	coinsUsed := int64(0) // Initially 0, set when user confirms order

	// Fetch address snapshot
	addressSnapshot, err := s.getAddressSnapshot(ctx, tx, req.AddressID)
	if err != nil {
		return nil, fmt.Errorf("failed to get address snapshot: %w", err)
	}

	// ============================================================
	// STEP 5: CREATE PRICING TOKEN WITH NEGOTIATION CONTEXT
	// ============================================================
	token := pricingtokenentity.NewPricingTokenFromNegotiation(
		req.UserID,
		forSale.ProductID,
		session.ID,
		1,         // Negotiation is always for 1 item
		unitPrice, // Negotiated price, NOT forSale price
		shippingTotal,
		commissionPercent.IntPart(),
		postDiscount.CommissionAmount,
		postDiscount.EscrowAmount,
		money.Zero(),
		req.ShippingOptionID,
		shippingOption.Name,
		string(shippingOption.TransportType),
		shippingOption.ExpeditionName,
		estimatedDays,
		req.AddressID,
		addressSnapshot,
		discountID, // Discount may be applied for negotiation checkout
		discountCode,
		discountType,
		discountValue,
		discountAmount,
		coinsUsed,          // Coins applied (0 for new tokens)
		postDiscount.MaxCoinsAllowed,    // Max coins allowed
		postDiscount.OrderValueForCoins, // Pre-calculated for coins service: discounted product value (PD)
	)

	if err := s.tokenRepo.CreateTx(ctx, tx, token); err != nil {
		return nil, fmt.Errorf("failed to create pricing token: %w", err)
	}

	// Prepare response
	var discountTypeStr *string
	if discountType != nil {
		dt := string(*discountType)
		discountTypeStr = &dt
	}

	// Calculate coins preview (non-binding, for UI display only)
	var coinsPreview *CoinsPreview
	if postDiscount.MaxCoinsAllowed > 0 {
		coinsPreview = &CoinsPreview{
			MaxApplicable: postDiscount.MaxCoinsAllowed,
		}
	}

	return &GenerateForNegotiationResponse{
		Token:     token.Token,
		ExpiresAt: token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		PricingSnapshot: PricingSnapshot{
			UnitPrice:          unitPrice,
			Quantity:           1,
			Subtotal:           subtotal,
			ShippingTotal:      shippingTotal,
			CommissionPercent:  commissionPercent.IntPart(),
			CommissionAmount:   postDiscount.CommissionAmount,
			DiscountAmount:     discountAmount,
			ServiceFeeAmount:   money.Zero(),
			TotalPayableAmount: postDiscount.TotalPayableAmount,
			DiscountCode:       discountCode,
			DiscountType:       discountTypeStr,
			DiscountValue:      discountValue,
			EscrowAmount:       postDiscount.EscrowAmount,
			ShippingMode:       "standard", // Negotiations use standard shipping options
			CoinsPreview:       coinsPreview,
		},
	}, nil
}

// ============================================================================
// AUCTION: GENERATE TOKEN FROM AUCTION
// ============================================================================

// GenerateForAuctionRequest contains parameters for generating a pricing token from an auction.
type GenerateForAuctionRequest struct {
	UserID           uuid.UUID
	AuctionID        uuid.UUID
	AddressID        uuid.UUID
	ShippingOptionID uuid.UUID
	DiscountCode     *string
	UseCoins         bool // Whether buyer wants to apply coins (backend decides actual amount)
}

// GenerateForAuctionResponse contains the generated pricing token and its snapshot.
type GenerateForAuctionResponse struct {
	Token                 uuid.UUID
	ExpiresAt             string
	PricingSnapshot       PricingSnapshot
	AuctionSettlementType string // "buy_now" or "bid_win"
}

// GenerateForAuction generates a pricing token from an auction.
//
// AUCTION CHECKOUT SAFETY:
// - Validates auction exists and is in valid state for checkout
// - Uses auction price (buy-now price or winning bid, NOT forSale price)
// - Token consumption creates order with auction source
//
// AUCTION PRICING GUARD:
// - Buy-now: Treated as fixed-price checkout, promo discounts and coins ALLOWED
// - Bid-win: Competitive final price, promo discounts and coins ALLOWED (owner canonical 2026-06-16)
func (s *PricingTokenService) GenerateForAuction(
	ctx context.Context,
	tx db.Tx,
	req *GenerateForAuctionRequest,
) (*GenerateForAuctionResponse, error) {
	// ============================================================
	// STEP 1: VALIDATE AUCTION EXISTS AND GET CONTEXT
	// ============================================================
	auction, err := s.auctionRepo.GetByID(ctx, tx, req.AuctionID)
	if err != nil {
		return nil, fmt.Errorf("auction not found: %w", err)
	}

	// ============================================================
	// STEP 2: DETERMINE SETTLEMENT TYPE
	// ============================================================
	// Auction checkout can happen via two paths:
	// - Buy-now: User clicks buy-now while auction is active
	// - Winner claim: Auction ended, winner claims their item
	//
	// For buy-now: auction must be active, have buy_now_price set
	// For winner claim: auction must be ended, caller must be the winner
	var settlementType orderentity.AuctionSettlementType
	var unitPrice int64

	if auction.Status == auctionentity.StatusActive && auction.BuyNowPrice != nil {
		// BUY-NOW FLOW
		settlementType = orderentity.AuctionSettlementBuyNow
		unitPrice = *auction.BuyNowPrice

		// Validate caller is not the seller
		if req.UserID == auction.SellerID {
			return nil, fmt.Errorf("cannot buy own auction")
		}

		// Buy-now flow: discounts and coins are allowed

	} else if (auction.Status == auctionentity.StatusEnded || auction.Status == auctionentity.StatusWaitingSettlement) && auction.HasWinner() {
		// WINNER CLAIM FLOW
		settlementType = orderentity.AuctionSettlementBidWin

		// Validate caller is the winner
		if auction.WinnerID() == nil || *auction.WinnerID() != req.UserID {
			return nil, fmt.Errorf("caller is not the auction winner")
		}

		// Get winning bid amount
		winningBid := auction.WinningBid()
		if winningBid == nil {
			return nil, fmt.Errorf("auction has no winning bid amount")
		}
		unitPrice = *winningBid

	} else {
		// Invalid state for checkout
		if auction.Status == auctionentity.StatusActive {
			return nil, fmt.Errorf("auction does not have buy-now option available")
		}
		return nil, fmt.Errorf("auction is not eligible for checkout: status=%s", auction.Status)
	}

	// ============================================================
	// STEP 3: CHECK FOR DUPLICATE SETTLEMENT
	// ============================================================
	// Guard: If auction.order_id is already set, prevent double settlement
	if auction.OrderID != nil {
		return nil, fmt.Errorf("auction already settled: order_id=%s", *auction.OrderID)
	}

	// ============================================================
	// STEP 4: VALIDATE AUCTION ELIGIBILITY
	// ============================================================
	// Auction products do not have a corresponding ForSale record.
	// For bid-win claims, validate directly against the auction entity:
	// - Prevent self-purchase (buyer == seller)
	// Quantity and active status are guaranteed by the auction lifecycle (step 2).

	// Prevent self-purchase
	if auction.SellerID == req.UserID {
		return nil, errors.New("cannot purchase own forSale")
	}

	// ============================================================
	// STEP 5: VALIDATE SHIPPING OPTION AND GET PROVINCE-BASED PRICING
	// ============================================================
	shippingOption, err := s.shippingRepo.GetByID(ctx, tx, req.ShippingOptionID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}

	// Get buyer province for shipping coverage lookup
	provinceCode, _, err := s.getAddressWithProvince(ctx, tx, req.AddressID)
	if err != nil {
		return nil, fmt.Errorf("failed to get buyer province: %w", err)
	}

	// Get province-based shipping cost and ETA from ShippingCoverage
	shippingTotal, estimatedDays, err := s.getShippingCostAndETA(ctx, tx, req.ShippingOptionID, provinceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get shipping cost for province %s: %w", provinceCode, err)
	}

	// ============================================================
	// STEP 6: CALCULATE PRICING
	// ============================================================
	subtotal := money.New(unitPrice) // Auction is always for 1 item

	// Initialize discount values
	var discountID *uuid.UUID
	var discountCode *string
	var discountType *discountentity.DiscountType
	var discountValue *decimal.Decimal
	var discountAmount money.Money = money.Zero()

	// Apply discount if provided and valid for the auction context.
	// DISCOUNT TRUTH: Validate and calculate discount using canonical discount domain
	if req.DiscountCode != nil && *req.DiscountCode != "" {
		result, err := s.discountService.ApplyDiscountAtCheckout(
			ctx,
			tx,
			req.UserID,
			*req.DiscountCode,
			subtotal.Int64(),
			discountentity.DiscountContextAuction,
			&auction.SellerID,
		)
		if err != nil {
			// Return validation error to user - honest failure instead of silent ignore
			return nil, fmt.Errorf("discount validation failed: %w", err)
		}

		// Store validated discount details in pricing token
		// CRITICAL: Store DiscountID for atomic usage recording during token consumption
		discountID = &result.DiscountID
		discountCode = &result.Code
		dt := result.Type
		discountType = &dt
		discountValue = &result.Value
		discountAmount = money.New(result.DiscountAmount.IntPart())
	}

	// ============================================================================
	// CANONICAL POST-DISCOUNT MONEY FLOW
	// ============================================================================
	commissionPercent := s.configService.GetAuctionCommission(ctx, tx)
	postDiscount, err := calculatePostDiscountMoneyFlow(subtotal, discountAmount, shippingTotal, commissionPercent)
	if err != nil {
		return nil, err
	}
	coinsUsed := int64(0) // Initially 0, set when user confirms order

	// Fetch address snapshot
	addressSnapshot, err := s.getAddressSnapshot(ctx, tx, req.AddressID)
	if err != nil {
		return nil, fmt.Errorf("failed to get address snapshot: %w", err)
	}

	// ============================================================
	// STEP 7: CREATE PRICING TOKEN WITH AUCTION CONTEXT
	// ============================================================
	token := pricingtokenentity.NewPricingTokenFromAuction(
		req.UserID,
		auction.ProductID,
		auction.ID,
		1,                    // Auction is always for 1 item
		money.New(unitPrice), // Auction price (buy-now or winning bid)
		shippingTotal,
		commissionPercent.IntPart(),
		postDiscount.CommissionAmount,
		postDiscount.EscrowAmount,
		money.Zero(),
		req.ShippingOptionID,
		shippingOption.Name,
		string(shippingOption.TransportType),
		shippingOption.ExpeditionName,
		estimatedDays,
		req.AddressID,
		addressSnapshot,
		discountID, // Added for atomic discount usage recording
		discountCode,
		discountType,
		discountValue,
		discountAmount,
		coinsUsed,          // Coins applied (0 for new tokens)
		postDiscount.MaxCoinsAllowed,    // Max coins allowed based on canonical 20% of PD
		postDiscount.OrderValueForCoins, // Pre-calculated for coins service: discounted product value (PD)
	)

	if err := s.tokenRepo.CreateTx(ctx, tx, token); err != nil {
		return nil, fmt.Errorf("failed to create pricing token: %w", err)
	}

	// Prepare response
	var discountTypeStr *string
	if discountType != nil {
		dt := string(*discountType)
		discountTypeStr = &dt
	}

	// Map settlement type to string
	var settlementTypeStr string
	switch settlementType {
	case orderentity.AuctionSettlementBuyNow:
		settlementTypeStr = "buy_now"
	case orderentity.AuctionSettlementBidWin:
		settlementTypeStr = "bid_win"
	}

	// Calculate coins preview (non-binding, for UI display only)
	// Both buy-now and bid-win emit coins preview (owner canonical 2026-06-16).
	var coinsPreview *CoinsPreview
	if postDiscount.MaxCoinsAllowed > 0 {
		coinsPreview = &CoinsPreview{
			MaxApplicable: postDiscount.MaxCoinsAllowed,
		}
	}

	return &GenerateForAuctionResponse{
		Token:     token.Token,
		ExpiresAt: token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		PricingSnapshot: PricingSnapshot{
			UnitPrice:          money.New(unitPrice),
			Quantity:           1,
			Subtotal:           subtotal,
			ShippingTotal:      shippingTotal,
			CommissionPercent:  commissionPercent.IntPart(),
			CommissionAmount:   postDiscount.CommissionAmount,
			DiscountAmount:     discountAmount,
			ServiceFeeAmount:   money.Zero(),
			TotalPayableAmount: postDiscount.TotalPayableAmount,
			DiscountCode:       discountCode,
			DiscountType:       discountTypeStr,
			DiscountValue:      discountValue,
			EscrowAmount:       postDiscount.EscrowAmount,
			ShippingMode:       "standard", // Auctions use standard shipping options
			CoinsPreview:       coinsPreview,
		},
		AuctionSettlementType: settlementTypeStr,
	}, nil
}
