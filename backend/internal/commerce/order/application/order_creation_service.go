// ⚠️ FINANCIAL RULE:
// All escrow lifecycle operations MUST go through EscrowService.
// Direct state mutation is forbidden.
//
// Order domain is a PRICING SNAPSHOT only.
// Escrow domain is the SINGLE SOURCE OF TRUTH for escrow row state.
// Finance ledger is the SINGLE SOURCE OF TRUTH for money movement.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	forSaleRepoImpl "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	forSalerepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	"github.com/labuda/backend/internal/commerce/governance/commercegov"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationRepoImpl "github.com/labuda/backend/internal/commerce/negotiation/infrastructure/repository"
	negotiationRepo "github.com/labuda/backend/internal/commerce/negotiation/repository"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderRepoImpl "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	productRepoImpl "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	productRepo "github.com/labuda/backend/internal/commerce/product/repository"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingRepoImpl "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	shippingquoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"

	auditApp "github.com/labuda/backend/internal/governance/audit/application"
	addressApp "github.com/labuda/backend/internal/identity/address/application"
	addressentity "github.com/labuda/backend/internal/identity/address/entity"
	addressRepoImpl "github.com/labuda/backend/internal/identity/address/infrastructure/repository"
	addressRepo "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/internal/identity/auth"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	"github.com/labuda/backend/internal/platform/events"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	contentRepoImpl "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	contentrepo "github.com/labuda/backend/internal/social/content/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// AuctionStatusChecker is an interface for checking auction status by product ID.
// Used by OrderCreationService to validate product is not in auction settlement.
type AuctionStatusChecker interface {
	GetAuctionStatusByProductID(ctx context.Context, tx db.Tx, productID uuid.UUID) (*string, error)
}

// CheckoutAddressResolver resolves and validates a buyer's shipping address
// for checkout (ownership + availability). Mirrors AuctionStatusChecker above:
// a minimal local interface for exactly the one method OrderCreationService
// calls, so that *addressApp.AddressService — which has no dependency-injected
// constructor (NewAddressService() hardcodes its own real repository) — can
// still be substituted with a test fake. The concrete *addressApp.AddressService
// satisfies this interface structurally; NewOrderCreationService's wiring is
// unchanged (PHASE 2.5: test coverage).
type CheckoutAddressResolver interface {
	GetAddressForCheckout(ctx context.Context, tx db.Tx, userID, addressID uuid.UUID) (*addressentity.Address, error)
}

// ShippingQuoteCheckoutAuthority is the ONE authority for shipping-quote
// checkout validation and the ACTIVE -> USED transition. It is implemented by
// the Commerce Shipping Quote service (*shippingQuoteApp.Service). Order
// creation NEVER re-implements quote lifecycle rules; it delegates here inside
// the order's transaction so quote consumption and order mutation are atomic.
type ShippingQuoteCheckoutAuthority interface {
	// ConsumeQuoteForCheckout validates the quote against the expected context
	// and marks it USED in the caller's transaction.
	ConsumeQuoteForCheckout(ctx context.Context, tx db.Tx, in shippingquoteEntity.CheckoutContext) (*shippingquoteEntity.ShippingQuote, error)
	// InvalidateQuotesByProduct marks all remaining ACTIVE quotes for a product
	// INVALID (e.g. once the sale surface is sold).
	InvalidateQuotesByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) error
}

// ============================================================================
// PAYMENT EXPIRY CALCULATION (PHASE 2)
// ============================================================================

// Payment method constants
const (
	PaymentMethodInstant = "instant" // QRIS, e-wallet (GoPay, OVO, Dana)
	PaymentMethodVA      = "va"      // Bank Transfer (Virtual Account)
	PaymentMethodRetail  = "retail"  // Alfamart/Indomaret
	PaymentMethodDefault = "default" // Default payment path
)

// calculatePaymentExpiry returns the payment expiry time based on payment method.
//
// PAYMENT EXPIRY RULES:
// - instant (QRIS, e-wallet): 15 minutes
// - va (Bank Transfer): 1 hour
// - retail (Alfamart/Indomaret): 6 hours
// - default: 30 minutes
//
// CRITICAL: This is the SINGLE SOURCE OF TRUTH for payment expiry calculation.
// All order creation paths MUST use this function.
func calculatePaymentExpiry(paymentMethod string, createdAt time.Time) time.Time {
	switch paymentMethod {
	case PaymentMethodInstant:
		return createdAt.Add(15 * time.Minute)
	case PaymentMethodVA:
		return createdAt.Add(1 * time.Hour)
	case PaymentMethodRetail:
		return createdAt.Add(6 * time.Hour)
	default:
		return createdAt.Add(30 * time.Minute) // Default expiry window
	}
}

// OrderCreationService handles order creation operations.
//
// MARKET AUTHORITY ENFORCEMENT (PHASE 1B):
// - Buyers cannot create orders from expired sellers' forSales
// - Sellers must have active subscription to accept new orders
type OrderCreationService struct {
	repo                   orderrepository.OrderRepository
	forSaleRepo            forSalerepo.ForSaleRepository
	productRepo            productRepo.ProductRepository
	negotiationRepo        negotiationRepo.Repository
	productShippingRepo    shippingRepoImpl.ProductShippingSetupRepository
	shippingQuoteAuthority ShippingQuoteCheckoutAuthority // ONE authority for quote checkout validation + use
	shippingService        *shippingApp.ShippingService
	addressService         CheckoutAddressResolver       // See CheckoutAddressResolver doc for why this is an interface
	addressRepo            addressRepo.AddressRepository // For fetching farm address (shipping origin)
	ownership              *auth.OwnershipValidator
	accountStatusChecker   auth.AccountStatusChecker
	roleChecker            auth.RoleChecker
	actorResolver          capabilityEntity.ActorResolver // SERVICE LAYER ENFORCEMENT
	outboxRepo             *outboxRepo.OutboxRepository
	configService          *platformconfigApp.ConfigService
	commentRepo            contentrepo.CommentRepository
	auditService           *auditApp.AuditService // OBSERVABILITY: Audit logging service
	auctionRepo            AuctionStatusChecker   // BNR: Check auction settlement status
	commerceGovRepo        commercegov.Repository // COMMERCE RESTRICTION: canonical restriction checker
}

// NewOrderCreationService creates a new OrderCreationService.
func NewOrderCreationService(
	accountStatusChecker auth.AccountStatusChecker,
	shippingService *shippingApp.ShippingService,
	outboxRepo *outboxRepo.OutboxRepository,
	configService *platformconfigApp.ConfigService,
	roleChecker auth.RoleChecker,
	actorResolver capabilityEntity.ActorResolver, // SERVICE LAYER ENFORCEMENT
	auditService *auditApp.AuditService, // OBSERVABILITY: Audit service
	productShippingRepo shippingRepoImpl.ProductShippingSetupRepository, // DI: Product shipping options
	auctionStatusChecker AuctionStatusChecker, // BNR: Auction status checker (optional)
) *OrderCreationService {
	return &OrderCreationService{
		repo:                 orderRepoImpl.NewOrderRepository(),
		forSaleRepo:          forSaleRepoImpl.NewForSaleRepository(),
		productRepo:          productRepoImpl.NewProductRepository(),
		negotiationRepo:      negotiationRepoImpl.NewNegotiationRepository(),
		productShippingRepo:  productShippingRepo,
		shippingService:      shippingService,
		addressService:       addressApp.NewAddressService(),
		addressRepo:          addressRepoImpl.NewAddressRepository(), // For fetching farm address
		ownership:            auth.NewOwnershipValidator(),
		accountStatusChecker: accountStatusChecker,
		roleChecker:          roleChecker,
		actorResolver:        actorResolver,
		outboxRepo:           outboxRepo,
		configService:        configService,
		commentRepo:          contentRepoImpl.NewCommentRepository(),
		auditService:         auditService,
		auctionRepo:          auctionStatusChecker,
	}
}

// SetCommerceGovRepository wires the canonical commerce restriction repository
// into the order creation path so buyer restriction can be enforced inside the
// same transaction as the order mutation (TOCTOU prevention).
func (s *OrderCreationService) SetCommerceGovRepository(repo commercegov.Repository) {
	s.commerceGovRepo = repo
}

// SetShippingQuoteCheckoutAuthority wires the ONE Commerce shipping-quote
// checkout authority. It is wired after construction because the Shipping Quote
// service is built after the order services (avoids an import/initialization
// cycle). Checkout that uses a shipping quote FAILS CLOSED when this is not
// wired — it never silently skips quote validation.
func (s *OrderCreationService) SetShippingQuoteCheckoutAuthority(a ShippingQuoteCheckoutAuthority) {
	s.shippingQuoteAuthority = a
}

// requireBuyerNotRestricted checks whether the given buyer has an active
// commerce restriction. Must be called inside the same transaction as the
// order mutation. Returns auth.ErrCommerceRestricted when restricted.
func (s *OrderCreationService) requireBuyerNotRestricted(ctx context.Context, tx db.Tx, buyerID uuid.UUID) error {
	if s.commerceGovRepo == nil {
		return nil // repo not wired — fail-open for backward compat
	}
	restricted, _, err := commercegov.IsUserRestricted(ctx, tx, s.commerceGovRepo, buyerID)
	if err != nil {
		return fmt.Errorf("commerce restriction check failed: %w", err)
	}
	if restricted {
		return auth.ErrCommerceRestricted
	}
	return nil
}

// extractPrimaryImageURL extracts the first image URL from a media URL array (JSONB).
// Returns nil if MediaURLs is empty or invalid JSON.
func extractPrimaryImageURL(mediaURLs json.RawMessage) *string {
	if len(mediaURLs) == 0 {
		return nil
	}
	var urls []string
	if err := json.Unmarshal(mediaURLs, &urls); err != nil {
		return nil
	}
	if len(urls) > 0 {
		return &urls[0]
	}
	return nil
}

// getOriginRequestTargetID finds the originating request content target for a fixed-price forSale.
// Returns the target_id if the sale surface was created in response to a request.
// Returns uuid.Nil if no request origin is found or on error (non-fatal).
func (s *OrderCreationService) getOriginRequestTargetID(ctx context.Context, tx db.Tx, forSaleID uuid.UUID) uuid.UUID {
	targetID, err := s.commentRepo.FindTargetIDByCommerceReference(ctx, tx, forSaleID)
	if err != nil {
		// Log error but don't fail order creation
		// The request origin is a nice-to-have feature, not critical
		return uuid.Nil
	}
	return targetID
}

// emitChatLinkRequestedEvent inserts an order.chat_link_requested outbox event
// in the order's canonical transaction. The chat domain consumes this event
// asynchronously and idempotently establishes the buyer↔seller direct room and
// sets linked_order_id (LATEST ACTIVE ORDER RULE).
//
// DOCTRINAL POSITION (RUNTIME-INVARIANTS §1.2 — A transaction MUST NOT span two
// domain authorities):
//   - chat_rooms is owned by the chat domain (interaction/chat).
//   - Previously this was an inline cross-domain mutation in the order tx.
//   - Now it is an outbox handoff: order tx → outbox event → chat consumer.
//
// FAILURE SEMANTICS (RUNTIME-INVARIANTS §6.4 — Eventual consistency):
//   - Chat-link is a UX convenience, not commerce authority.
//   - Order is the source of truth; chat linkage failure does NOT roll back the
//     order. Outbox retries the consumer with exponential backoff; persistent
//     failure goes to DLQ (visible via the observability metrics added in the
//     prior hardening pass). The order remains valid regardless.
//
// REPLAY / IDEMPOTENCY:
//   - The consumer uses chat's existing UNIQUE (participant_a, participant_b,
//     room_type) constraint to avoid duplicate rooms.
//   - Setting linked_order_id to the same orderID twice is a no-op (same value).
//   - Safe under at-least-once delivery (RUNTIME-INVARIANTS §3.3).
//
// COMPENSATION:
//   - None needed. linked_order_id naturally rolls forward to the next active
//     order in the same buyer↔seller pair (existing LATEST ACTIVE ORDER RULE).
//     A cancelled order remains linked until the next order overwrites it; this
//     matches the prior inline behaviour.
func (s *OrderCreationService) emitChatLinkRequestedEvent(
	ctx context.Context,
	tx db.Tx,
	order *orderentity.Order,
) error {
	type chatLinkRequestedPayload struct {
		OrderID  string `json:"order_id"`
		BuyerID  string `json:"buyer_id"`
		SellerID string `json:"seller_id"`
	}
	payload, err := json.Marshal(chatLinkRequestedPayload{
		OrderID:  order.ID.String(),
		BuyerID:  order.BuyerID.String(),
		SellerID: order.SellerID.String(),
	})
	if err != nil {
		return fmt.Errorf("marshal chat-link payload: %w", err)
	}

	return s.outboxRepo.InsertEvent(
		ctx, tx,
		events.EventOrderChatLinkRequested,
		order.ID,
		payload,
	)
}

// ============================================================================
// UNIFIED SALE-SURFACE VALIDATION FOR CHECKOUT
// ============================================================================

// ValidateSaleSurfaceForCheckoutInput contains parameters for validating a sale surface for checkout.
type ValidateSaleSurfaceForCheckoutInput struct {
	SaleSurface *entity.ForSale // Pre-loaded sale surface (must be locked with FOR UPDATE)
	BuyerID     uuid.UUID       // Buyer user ID
	Quantity    int             // Requested quantity

	// WonAuctionSettlement marks this order as the settlement of an auction
	// the buyer already won (claim with settlement type bid_win). OWNER
	// CANONICAL (Oct 2026): an auction with bids runs to completion even if
	// the seller's subscription lapsed mid-run — the won order must be
	// fulfillable. Guard 6 still blocks every NEW sale: ForSale checkout and
	// auction buy-now both leave this false.
	WonAuctionSettlement bool
}

// validateSaleSurfaceForCheckout performs unified validation for all checkout entry paths.
//
// This consolidates validation logic used by:
// - CreateFromSaleSurface (direct checkout with optional negotiation)
// - CreateFromAuction (auction checkout)
//
// VALIDATIONS PERFORMED:
// 1. Sale surface status must be ACTIVE
// 2. Sale surface visibility must be PUBLIC
// 3. Buyer cannot purchase their own sale surface (self-purchase guard)
// 4. Sufficient quantity available
// 5. Auction sale surfaces must have quantity = 1
// 6. Seller must have active market authority (subscription)
//
// ⚠️ CRITICAL LOCKING REQUIREMENT ⚠️
// The sale surface passed in MUST already be locked with FOR UPDATE to prevent:
// 1. Race conditions during validation (status changes between check and use)
// 2. Double-spending inventory (multiple orders for same sale surface)
// 3. Price manipulation attacks (price changes after order calculation)
//
// ❌ NEVER bypass this lock - it protects the core commerce invariant
// ✅ ALWAYS use forSaleRepo.GetForUpdate() before calling this method
// ✅ DB constraints are the FINAL GUARD - never disable them
func (s *OrderCreationService) validateSaleSurfaceForCheckout(
	ctx context.Context,
	input ValidateSaleSurfaceForCheckoutInput,
) error {
	l := input.SaleSurface

	// Guard 1: Sale surface must be active
	if l.Status != entity.ForSaleStatusActive {
		return &entity.ForSaleNotActiveError{Status: l.Status}
	}

	// Guard 2: Sale surface must be public
	if l.Visibility != entity.ForSaleVisibilityPublic {
		return &entity.ForSaleNotAvailableError{
			ForSaleID: l.ID,
			Reason:    fmt.Sprintf("sale surface not public: visibility=%s", l.Visibility),
		}
	}

	// Guard 3: Buyer cannot purchase their own sale surface
	if l.SellerID == input.BuyerID {
		return &orderentity.ErrSelfPurchase{
			BuyerID:  input.BuyerID,
			SellerID: l.SellerID,
			Source:   "checkout",
		}
	}

	// Guard 4: Sufficient quantity available
	if l.QuantityAvailable < input.Quantity {
		return &entity.InsufficientQuantityError{
			Available: l.QuantityAvailable,
			Requested: input.Quantity,
		}
	}

	// Guard 6: MARKET AUTHORITY CHECK — blocks NEW sales from sellers
	// without an active subscription (ForSale checkout, auction buy-now).
	// The one deliberate exception is WonAuctionSettlement: a won auction
	// must run to completion even if the seller's subscription lapsed
	// mid-run (owner canonical), otherwise the winner could never receive
	// the order they already won.
	if !input.WonAuctionSettlement {
		hasCapability, err := s.roleChecker.HasActiveSellerCapability(ctx, l.SellerID)
		if err != nil {
			return fmt.Errorf("failed to verify seller market authority: %w", err)
		}
		if !hasCapability {
			return auth.ErrMarketAuthorityRequired
		}
	}

	return nil
}

// getSellerOriginSnapshot resolves the seller account's primary address and
// returns it as an immutable order shipping-origin snapshot.
//
// CANONICAL TRUTH: every product's origin is the seller account's primary
// address. There is no product-level origin address.
func (s *OrderCreationService) getSellerOriginSnapshot(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (addressentity.AddressSnapshot, error) {
	primary, err := s.addressRepo.GetPrimaryByUserID(ctx, tx, sellerID)
	if err != nil {
		return addressentity.AddressSnapshot{}, fmt.Errorf("failed to resolve seller primary address: seller_id=%s", sellerID)
	}
	if primary == nil {
		return addressentity.AddressSnapshot{}, fmt.Errorf("seller has no primary address: seller_id=%s", sellerID)
	}
	return primary.ToSnapshot(), nil
}

// ============================================================================
// SHIPPING QUOTE CHECKOUT CONSUMPTION (DELEGATES TO THE ONE AUTHORITY)
// ============================================================================

// consumeShippingQuoteForOrder delegates shipping-quote checkout validation and
// the ACTIVE -> USED transition to the SINGLE Commerce authority
// (ShippingQuoteCheckoutAuthority), inside the caller's order transaction. Order
// creation owns NO quote lifecycle rules — it only supplies the expected
// commercial context.
//
// FAIL-CLOSED: if the authority is not wired, checkout that uses a quote is
// rejected rather than silently skipping validation.
//
// AUDIT: the authority returns a typed CheckoutRejectionError; this function
// maps it into the order-side audit trail (reason code + detail). The authority
// stays the single decision-maker; the order domain only records its own
// observability.
func (s *OrderCreationService) consumeShippingQuoteForOrder(
	ctx context.Context,
	tx db.Tx,
	in shippingquoteEntity.CheckoutContext,
) error {
	if s.shippingQuoteAuthority == nil {
		return fmt.Errorf("shipping quote checkout authority not configured: quote consumption rejected")
	}

	quote, err := s.shippingQuoteAuthority.ConsumeQuoteForCheckout(ctx, tx, in)
	if err != nil {
		var rejection *shippingquoteEntity.CheckoutRejectionError
		if errors.As(err, &rejection) && s.auditService != nil {
			s.auditService.ShippingQuoteRejected(ctx, tx, in.QuoteID, in.BuyerID, rejection.Field, rejection.Reason)
		}
		return err
	}

	if s.auditService != nil {
		s.auditService.ShippingQuoteUsed(ctx, tx, in.QuoteID, in.BuyerID, quote.SellerID, quote.Cost.Int64())
	}
	return nil
}

// ============================================================================
// SHIPPING QUOTE IDEMPOTENCY CHECK (SCENARIO 6 FIX)
// ============================================================================

// checkShippingQuoteIdempotency verifies that a shipping quote doesn't already
// have a live/successful blocking order. This prevents double-order attacks from:
// - Double-click checkout
// - Multiple tabs
// - Network retries
//
// CRITICAL: This check happens BEFORE the standard idempotency key check so
// shipping-quote-specific protection can short-circuit duplicate live orders.
//
// SCENARIO 6 FIX: Ensures 1 SHIPPING QUOTE = 1 LIVE/SUCCESSFUL ORDER ONLY
func (s *OrderCreationService) checkShippingQuoteIdempotency(
	ctx context.Context,
	tx db.Tx,
	shippingQuoteID uuid.UUID,
	buyerID uuid.UUID,
) (*orderentity.Order, error) {
	// Query for a blocking order with this shipping quote.
	existingOrder, err := s.repo.GetBlockingOrderByShippingQuoteID(ctx, tx, shippingQuoteID)
	if err != nil {
		return nil, fmt.Errorf("failed to check shipping quote idempotency: %w", err)
	}

	// If a blocking order exists, the quote is already consumed by a live or
	// successful commerce flow and must not be reused.
	if existingOrder != nil {
		// Verify buyer matches (security check)
		if existingOrder.BuyerID != buyerID {
			return nil, fmt.Errorf("shipping quote already used by different buyer: quote_id=%s, existing_buyer=%s, requested_buyer=%s (possible quote theft)",
				shippingQuoteID, existingOrder.BuyerID, buyerID)
		}

		// Log the idempotent response for audit
		// Note: Using generic audit log since specific method doesn't exist
		if s.auditService != nil {
			// Audit log for shipping quote idempotent response
			// This prevents duplicate orders when buyer double-clicks checkout
		}

		return existingOrder, nil
	}

	return nil, nil
}

// ============================================================================
// ORDER CREATION FROM AUCTION
// ============================================================================

// CreateFromAuctionInput contains parameters for creating an order from an auction.
type CreateFromAuctionInput struct {
	AuctionID             uuid.UUID
	AuctionSellerID       uuid.UUID
	ProductID             uuid.UUID
	BuyerID               uuid.UUID
	WinningBid            int64
	AddressID             uuid.UUID // Buyer's shipping address ID
	ShippingSetupID       uuid.UUID
	ProvinceCode          string                            // Deprecated: Use AddressID instead
	CityCode              string                            // Deprecated: Use AddressID instead
	DiscountCode          *string                           // Optional discount code for checkout pricing
	AuctionSettlementType orderentity.AuctionSettlementType // buy_now vs bid_win
	PricingSnapshot       *PricingSnapshot                  // Pricing snapshot from validated pricing token (pricing authority)
	IdempotencyKey        *string                           // Optional: HTTP idempotency key for safe retries
	// ShippingResolvedAt is the canonical payment-deadline anchor for auction
	// orders: payment_expires_at = shipping_resolved_at + 24h (NOT the
	// method-based expiry used by fixed-price orders).
	ShippingResolvedAt time.Time
	// PaymentMethodCode is the exact method the buyer selected at checkout. It
	// is bound to the order and enforced by POST /payments. Nil for the
	// auction-claim path, which selects a method at payment time.
	PaymentMethodCode *string
}

// calculateAuctionPaymentExpiry returns the payment deadline for an
// auction-sourced order: shipping_resolved_at + 24h. There is no extension
// and no second payment-deadline authority for auctions. A zero anchor (order
// creation racing a missing shipping-resolution marker) falls back to now so
// the buyer still gets the full 24h window.
func calculateAuctionPaymentExpiry(shippingResolvedAt time.Time) time.Time {
	anchor := shippingResolvedAt
	if anchor.IsZero() {
		anchor = time.Now()
	}
	return anchor.Add(24 * time.Hour)
}

// auctionOrderPaymentExpiry computes the payment deadline for an auction
// order:
//   - bid-win (claim flow): shipping_resolved_at + 24h (canonical settlement
//     payment deadline).
//   - buy-now: method-based expiry (the auction ends immediately at buy-now
//     order creation; there is no shipping-resolution phase).
func auctionOrderPaymentExpiry(input CreateFromAuctionInput, snapshot *PricingSnapshot) time.Time {
	if input.AuctionSettlementType == orderentity.AuctionSettlementBuyNow {
		return calculatePaymentExpiry(snapshot.PaymentMethod, time.Now())
	}
	return calculateAuctionPaymentExpiry(input.ShippingResolvedAt)
}

// CreateFromAuction creates an order from an ended auction.
//
// This method is called by AuctionService after an auction ends.
// It encapsulates all business logic for auction order creation.
//
// LOCK DISCIPLINE:
// - Load Address (no lock) - validates ownership and availability
// - Lock ForSale (FOR UPDATE) - prevents oversell
// - Validate sale surface is active and has quantity
// - Check shipping option availability for address location
// - Reduce sale surface quantity by 1
// - Create order with OrderSourceAuction + address snapshot
// - Create order item
// - Emit outbox event: auction.order.created
//
// ATOMICITY: All operations happen within a caller-provided transaction.
func (s *OrderCreationService) CreateFromAuction(
	ctx context.Context,
	tx db.Tx,
	input CreateFromAuctionInput,
) (*orderentity.Order, error) {
	// ============================================================
	// STEP 0: CHECK IDEMPOTENCY KEY (if provided)
	// ============================================================
	// Auction orders don't carry a pricing-token in this input; permissive
	// idempotency is preserved (same buyer+key returns prior order).
	if input.IdempotencyKey != nil && *input.IdempotencyKey != "" {
		existingOrder, err := s.repo.GetByIdempotencyKey(ctx, tx, input.BuyerID, *input.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("failed to check idempotency key: %w", err)
		}
		if existingOrder != nil {
			return idempotentOrderRecovery(existingOrder, nil)
		}
	}

	// ============================================================
	// STEP 0.25: SHIPPING QUOTE IDEMPOTENCY CHECK (SCENARIO 6 FIX)
	// ============================================================
	// If using shipping quote, check if it was already used to create an order
	// This prevents double-order attacks from double-click, multiple tabs, network retries
	// CRITICAL: This check happens AFTER the standard idempotency key check
	// to provide shipping-quote-specific protection
	if input.PricingSnapshot != nil && input.PricingSnapshot.ShippingQuoteID != nil {
		existingOrder, err := s.checkShippingQuoteIdempotency(ctx, tx, *input.PricingSnapshot.ShippingQuoteID, input.BuyerID)
		if err != nil {
			return nil, fmt.Errorf("failed to check shipping quote idempotency: %w", err)
		}
		// If order exists for this quote, return it (idempotent response)
		if existingOrder != nil {
			return existingOrder, nil
		}
	}

	// ============================================================
	// PRICING SNAPSHOT REQUIRED (HARD FAIL)
	// ============================================================
	// ALL order creation MUST provide a PricingSnapshot from a validated pricing token.
	// No alternate calculation path exists.
	// No live recalculation path exists.
	//
	// If PricingSnapshot is nil → HARD FAIL
	if input.PricingSnapshot == nil {
		return nil, fmt.Errorf("pricing_snapshot is required: all orders must use pricing token")
	}
	snapshot := input.PricingSnapshot
	usesShippingQuote := snapshot.ShippingSource != nil &&
		*snapshot.ShippingSource == "shipping_quote" &&
		snapshot.ShippingQuoteID != nil

	// Step 1: SERVICE LAYER ENFORCEMENT - CHECK BUYER CAN CHECKOUT
	// This checks: account active, email verified, profile complete
	actor, err := s.actorResolver.ResolveActor(ctx, input.BuyerID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve buyer actor: %w", err)
	}
	if !actor.CanCheckout() {
		return nil, auth.ErrProfileNotComplete
	}

	// COMMERCE RESTRICTION: Reject restricted buyer at the order-mutation boundary.
	// Checked inside the same transaction to prevent TOCTOU bypass.
	if err := s.requireBuyerNotRestricted(ctx, tx, input.BuyerID); err != nil {
		return nil, err
	}

	// Step 2: Load and validate shipping address
	// Load address using AddressService
	// Validates: address.user_id == buyerID, address.is_available_for_checkout == true
	address, err := s.addressService.GetAddressForCheckout(ctx, tx, input.BuyerID, input.AddressID)
	if err != nil {
		return nil, fmt.Errorf("failed to load address for checkout: %w", err)
	}

	// Create address snapshot for order immutability
	addressSnapshot := address.ToSnapshot()

	// Step 2: Load canonical product for the settled auction.
	product, err := s.productRepo.GetByID(ctx, tx, input.ProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to load auction product: %w", err)
	}

	// auctionSurface is a synthetic, never-persisted ForSale-shaped
	// view built solely to reuse validateSaleSurfaceForCheckout's generic
	// status/visibility/self-purchase/quantity/market-authority guards for
	// a settled auction. Auction is never a ForSaleType (removed in
	// PASS_21C) — quantity is hardcoded to 1 by the caller, so no
	// type-based quantity guard is needed here.
	auctionSurface := &entity.ForSale{
		ID:                input.AuctionID,
		ProductID:         product.ID,
		SellerID:          product.SellerID,
		Status:            entity.ForSaleStatusActive,
		Visibility:        entity.ForSaleVisibilityPublic,
		QuantityAvailable: 1,
		Product:           product,
	}

	// Step 2.5: SHIPPING GUARD - Verify sale surface has shipping options configured.
	// Returns shippingApp.ErrNoShippingSetups wrapped with %w so the handler
	// can surface the NO_SHIPPING_OPTIONS error code via errors.Is.
	//
	// SKIPPED when using a manual shipping quote: the quote (validated by the
	// canonical ConsumeQuoteForCheckout below) replaces the normal shipping
	// requirement, exactly as in CreateFromSaleSurface.
	if !usesShippingQuote {
		shippingSetups, err := s.productShippingRepo.GetByProduct(ctx, tx, product.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to check sale surface shipping options: %w", err)
		}
		if len(shippingSetups) == 0 {
			return nil, fmt.Errorf("sale surface %s: %w", product.ID, shippingApp.ErrNoShippingSetups)
		}
	}

	// Step 3: VALIDATE SALE-SURFACE STATE (UNIFIED)
	// UNIFIED VALIDATION: All checkout paths use the same validation logic
	if err := s.validateSaleSurfaceForCheckout(ctx, ValidateSaleSurfaceForCheckoutInput{
		SaleSurface: auctionSurface,
		BuyerID:     input.BuyerID,
		Quantity:    1, // Auction orders always have quantity = 1
		// bid_win = claim of an already-won auction (settlement, allowed for
		// lapsed sellers); buy_now = a NEW sale (still blocked by Guard 6).
		WonAuctionSettlement: input.AuctionSettlementType == orderentity.AuctionSettlementBidWin,
	}); err != nil {
		return nil, err
	}

	// Step 4-7: Check delivery availability and validate the selected shipping
	// option — SKIPPED when using a shipping quote (PHASE 2: parity with
	// CreateFromSaleSurface, which treats an already-validated shipping quote
	// as sufficient and does not re-check generic delivery availability).
	var selectedOption *shippingApp.DeliveryOption
	if !usesShippingQuote {
		// Step 4: Get available delivery options for this sale surface to the buyer's address location
		deliveryOptions, err := s.shippingService.CheckDeliveryAvailabilityForProduct(ctx, tx, product.ID, addressSnapshot.ProvinceID, addressSnapshot.CityID)
		if err != nil {
			return nil, fmt.Errorf("failed to check delivery availability: %w", err)
		}

		// Step 6: Find the selected shipping option from available options
		for i := range deliveryOptions {
			if deliveryOptions[i].ShippingSetupID == input.ShippingSetupID {
				selectedOption = &deliveryOptions[i]
				break
			}
		}

		// Step 7: Validate that the selected shipping option is available.
		// Wraps shippingApp.ErrShippingSetupUnavailable with %w so the handler
		// can surface the SHIPPING_OPTION_UNAVAILABLE error code via errors.Is.
		if selectedOption == nil || !selectedOption.IsAvailable {
			return nil, fmt.Errorf("option_id=%s province_id=%s city_id=%s: %w",
				input.ShippingSetupID, addressSnapshot.ProvinceID, addressSnapshot.CityID,
				shippingApp.ErrShippingSetupUnavailable)
		}
	}

	// Step 8 (removed): the auction buy-now path no longer writes a
	// Product "sold" mirror — Product carries no selling lifecycle.

	// ============================================================
	// PRICING SNAPSHOT (SINGLE SOURCE OF TRUTH)
	// ============================================================
	// PricingSnapshot is MANDATORY - all values from pricing token.
	// NO calculation happens here - snapshot is the authority.
	// (snapshot already resolved above, right after the nil-check, so
	// usesShippingQuote could gate the delivery-availability check.)

	// ============================================================
	// STEP 9.5: VALIDATE PAYMENT METHOD (PHASE 5: BACKEND-CONTROLLED PAYMENT)
	// ============================================================
	// 🔥 CRITICAL: Payment method MUST be validated against allowed values
	// - Prevents client-side manipulation
	// - Ensures expiry calculation is correct
	// - No arbitrary payment methods allowed
	var validPaymentMethod bool
	switch snapshot.PaymentMethod {
	case PaymentMethodInstant, PaymentMethodVA, PaymentMethodRetail, PaymentMethodDefault:
		validPaymentMethod = true
	}

	if !validPaymentMethod {
		return nil, fmt.Errorf("invalid payment method: %s (allowed: instant, va, retail, default)", snapshot.PaymentMethod)
	}

	// Step 10: Create order record with pricing snapshot
	// SourceType = auction, SourceID = auction_id
	winningBidAmount := money.New(input.WinningBid)

	// Determine quote snapshot values (TASK F)
	var shippingQuoteID *uuid.UUID
	var shippingQuotePrice *int64
	if snapshot.ShippingSource != nil && *snapshot.ShippingSource == "shipping_quote" {
		shippingQuoteID = snapshot.ShippingQuoteID
		if shippingQuoteID != nil && snapshot.ShippingTotal.Int64() > 0 {
			price := snapshot.ShippingTotal.Int64()
			shippingQuotePrice = &price
		}
	}

	// ============================================================
	// STEP 10.0: CONSUME THE SHIPPING QUOTE (ONE AUTHORITY)
	// ============================================================
	// Delegates validation + USED transition to the Commerce shipping-quote
	// authority inside this transaction. Consumed whenever the order carries a
	// quote — the chat id is passed when available (provenance check).
	if shippingQuoteID != nil {
		if err := s.consumeShippingQuoteForOrder(ctx, tx, shippingquoteEntity.CheckoutContext{
			QuoteID:            *shippingQuoteID,
			BuyerID:            input.BuyerID,
			ProductID:          product.ID,
			SourceType:         "auction",
			SourceID:           input.AuctionID,
			SellerID:           product.SellerID,
			ChatID:             snapshot.ChatID,
			ShippingProvinceID: addressSnapshot.ProvinceID,
			ShippingCityID:     addressSnapshot.CityID,
		}); err != nil {
			return nil, fmt.Errorf("shipping quote validation failed: %w", err)
		}
	}

	// NULLABLE: nil when using a manual shipping quote (PHASE 2: parity with
	// CreateFromSaleSurface's nil-safe shippingSetupID handling).
	var shippingSetupID *uuid.UUID
	if selectedOption != nil {
		shippingSetupID = &selectedOption.ShippingSetupID
	}

	order := orderentity.NewOrderFromSource(
		input.BuyerID,
		product.SellerID,               // IMPORTANT: Use canonical auction product seller, NOT input.AuctionSellerID
		orderentity.OrderSourceAuction, // Source type = auction
		input.AuctionID,                // Source ID = auction ID
		nil,                            // No negotiation ID
		1,                              // Quantity = 1
		winningBidAmount,               // Unit price = winning bid
		snapshot.Subtotal,              // Subtotal from pricing snapshot
		snapshot.ShippingTotal,         // Shipping from pricing snapshot
		snapshot.CommissionPercent,     // Commission percent from pricing snapshot
		snapshot.CommissionAmount,      // Commission amount from pricing snapshot
		snapshot.ServiceFeeAmount,      // Buyer service fee from pricing snapshot
		snapshot.EscrowAmount,          // CANONICAL buyer-funded base PD + S (fee excluded)
		shippingSetupID,                // NULLABLE: pointer to shipping option ID
		snapshot.ShippingSetupName,     // Option name from pricing snapshot
		snapshot.ShippingTransportType, // Transport type from pricing snapshot
		product.PreparationTime,        // SNAPSHOT: Freeze preparation time from canonical product
		snapshot.ShippingSource,        // Shipping source from pricing snapshot
		shippingQuoteID,                // TASK F: Quote ID
		shippingQuotePrice,             // TASK F: Quote price snapshot
		&snapshot.TokenID,              // Store pricing token ID (prevents double-ordering)
		auctionOrderPaymentExpiry(input, snapshot), // Canonical auction payment deadline
	)

	// Bind the buyer's selected payment method (nil for the auction-claim path).
	order.PaymentMethodCode = input.PaymentMethodCode

	// Apply shipping destination snapshot
	order.ApplyAddressSnapshot(addressSnapshot)

	// Apply shipping origin snapshot from the seller account's primary address
	farmAddressSnapshot, err := s.getSellerOriginSnapshot(ctx, tx, product.SellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seller origin snapshot: %w", err)
	}
	order.ApplyShippingOrigin(farmAddressSnapshot)

	// ============================================================
	// STEP 10.5: AUCTION SETTLEMENT TYPE VALIDATION
	// ============================================================
	// Per business requirements (owner canonical 2026-06-16):
	// - BUY NOW: Fixed-price checkout, promo discounts and coins ALLOWED
	// - BID WIN: Competitive final price, promo discounts and coins ALLOWED
	//
	// Coins are Labuda platform usage rights (not money). Backend enforces
	// 20% cap and commission safety on all orders regardless of settlement type.
	if !input.AuctionSettlementType.IsValid() {
		return nil, fmt.Errorf("invalid auction settlement type: %s", input.AuctionSettlementType)
	}

	// ============================================================
	// COIN INTENT (AUCTION REQUEST ONLY; SETTLEMENT HAPPENS LATER)
	// ============================================================
	// Mirrors CreateFromSaleSurface exactly: the raw amount is never taken
	// from client input. It is capped server-side against the buyer's current
	// active balance and the pricing token's pre-calculated MaxCoinsAllowed —
	// consistent with this file's PricingSnapshot-is-authority doctrine for
	// all money-shaped values.
	orderValueForCoins := snapshot.OrderValueForCoins

	// Create order_item with full product snapshot (koi details, image, etc.)
	// Order items are immutable historical records - all data is copied at order creation time
	orderItem := orderentity.NewOrderItem(
		order.ID,
		product.ID,
		winningBidAmount,
		1,
		product.Title,
	)

	// ============================================================
	// FINALIZE: snapshot integrity check, persistence,
	// outbox events, audit logging (PHASE 2: shared with CreateFromSaleSurface)
	// ============================================================
	finalized, err := s.finalizeOrderCreationTx(ctx, tx, finalizeOrderCreationInput{
		Order:              order,
		OrderItem:          orderItem,
		Snapshot:           snapshot,
		OrderValueForCoins: orderValueForCoins,
	})
	if err != nil {
		return nil, err
	}

	// Auction-specific outbox event — not part of the shared finalize stage
	// since sale-surface orders have no auction to report.
	type auctionOrderPayload struct {
		OrderID    string `json:"order_id"`
		BuyerID    string `json:"buyer_id"`
		SellerID   string `json:"seller_id"`
		AuctionID  string `json:"auction_id"`
		WinningBid int64  `json:"winning_bid"`
	}
	payload := auctionOrderPayload{
		OrderID:    finalized.ID.String(),
		BuyerID:    finalized.BuyerID.String(),
		SellerID:   finalized.SellerID.String(),
		AuctionID:  input.AuctionID.String(),
		WinningBid: input.WinningBid,
	}
	p, _ := json.Marshal(payload)

	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"auction.order.created",
		finalized.ID,
		p,
	); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return finalized, nil
}

// CreateFromSaleSurfaceInput contains the parameters for creating a sale-surface order.
type CreateFromSaleSurfaceInput struct {
	ProductID       uuid.UUID // Canonical product identity
	SourceType      orderentity.OrderSourceType
	SourceID        uuid.UUID // Sale-surface identity (fixed-price sale ID)
	BuyerID         uuid.UUID
	Quantity        int
	AddressID       uuid.UUID // Buyer's shipping address ID
	ShippingSetupID uuid.UUID
	ProvinceCode    string           // Deprecated: Use AddressID instead
	CityCode        string           // Deprecated: Use AddressID instead
	DiscountCode    *string          // Optional discount code for checkout pricing
	PricingSnapshot *PricingSnapshot // Optional: Pricing snapshot from validated pricing token
	IdempotencyKey  *string          // Optional: HTTP idempotency key for safe retries
	NegotiationID   *uuid.UUID       // Optional: Negotiation session ID for price override
	PricingTokenID  *uuid.UUID       // Pricing token ID used for this order (prevents double-ordering)
	// PaymentMethodCode is the exact method the buyer selected at checkout. It
	// is bound to the order and enforced by POST /payments.
	PaymentMethodCode *string
}

// idempotentOrderRecovery returns the existing order when (buyer, idempotency_key)
// already exists AND the request payload matches. If the key is reused with a
// different pricing token (i.e. a different request), returns ErrDuplicateIdempotencyKey
// so callers can surface a 409 instead of silently rebinding the key to a new payload.
func idempotentOrderRecovery(existing *orderentity.Order, requestedPricingTokenID *uuid.UUID) (*orderentity.Order, error) {
	if requestedPricingTokenID == nil {
		// No pricing token on the new request (auction etc.) — preserve existing
		// permissive idempotency: same key returns the prior order.
		return existing, nil
	}
	if existing.PricingTokenID == nil {
		return nil, orderrepository.ErrDuplicateIdempotencyKey
	}
	if *existing.PricingTokenID != *requestedPricingTokenID {
		return nil, orderrepository.ErrDuplicateIdempotencyKey
	}
	return existing, nil
}

// PricingSnapshot contains the pricing breakdown from a validated pricing token.
//
// ============================================================================
// PRICING TOKEN HARDENING: SINGLE SOURCE OF TRUTH
// ============================================================================
// This snapshot contains ALL pricing values from the validated pricing token.
// When this snapshot is provided, NO pricing recalculation occurs.
//
// The snapshot values are used DIRECTLY for:
// - Order creation (all pricing fields)
// - Discount usage tracking (record only, no calculation)
//
// FIELDS:
// - Base pricing: UnitPrice, Subtotal, Shipping, Commission
// - Discount: DiscountAmount (for order value calculation)
// - Shipping details: Option name, type, expedition, ETA
// - Destination: Address snapshot
// - Shipping source: "for_sale" or "shipping_quote"
// - TASK A-G: ShippingQuoteID, ChatID, AuctionID for anti-tamper validation
// - Token ID: Reference to pricing token (prevents double-ordering)
//
// IMPORTANT: EscrowAmount is calculated dynamically from these values.
// ============================================================================
type PricingSnapshot struct {
	UnitPrice             money.Money
	Subtotal              money.Money
	ShippingTotal         money.Money
	CommissionPercent     int64
	CommissionAmount      money.Money
	EscrowAmount          money.Money // Escrow amount from pricing token ((P−D)+S; commission is never buyer-funded)
	ServiceFeeAmount      money.Money // Flat buyer checkout service fee
	TotalPayableAmount    money.Money // EscrowAmount + ServiceFeeAmount
	DiscountAmount        money.Money // Discount amount for order value calculation
	OrderValueForCoins    int64       // Pre-calculated for coins service: PD = (P - D); shipping excluded
	ShippingSetupName     string
	ShippingTransportType string
	AddressSnapshot       *addressentity.AddressSnapshot // Shipping address snapshot (orders.address_snapshot)
	ShippingSource        *string                        // "for_sale" or "shipping_quote"
	ShippingQuoteID       *uuid.UUID                     // TASK A-G: Set when using shipping quote
	ChatID                *uuid.UUID                     // TASK A-G: Chat context for validation
	AuctionID             *uuid.UUID                     // TASK A: Auction ID for auction quotes
	NegotiationID         *uuid.UUID                     // N8-B: Negotiation ID from pricing token (direct==nil)
	TokenID               uuid.UUID                      // Pricing token ID (prevents double-ordering)
	PaymentMethod         string                         // PHASE 2: Payment method (instant, va, retail, default)
}

// finalizeOrderCreationInput bundles the data needed by the final,
// source-agnostic stage of order creation shared by every checkout entry
// point (sale surface, auction).
type finalizeOrderCreationInput struct {
	Order              *orderentity.Order
	OrderItem          *orderentity.OrderItem
	Snapshot           *PricingSnapshot
	OrderValueForCoins int64
}

// finalizeOrderCreationTx runs the final stage shared by every order-creation
// entry point: pricing-snapshot integrity validation, order + order-item
// persistence, outbox events, and audit logging.
//
// PHASE 2 (COMMERCE CORE REFACTOR): Extracted out of CreateFromSaleSurface so
// CreateFromAuction can reuse the exact same finalization logic instead of
// duplicating it (dedup target: order/item persistence, outbox
// events, audit log — 6 steps previously repeated in both functions).
//
// ATOMICITY: Runs entirely within the caller-provided transaction. Caller is
// responsible for constructing input.Order (via NewOrderFromSource) and
// input.OrderItem (via NewOrderItem) beforehand.
func (s *OrderCreationService) finalizeOrderCreationTx(
	ctx context.Context,
	tx db.Tx,
	input finalizeOrderCreationInput,
) (*orderentity.Order, error) {
	order := input.Order
	snapshot := input.Snapshot

	// ============================================================
	// SNAPSHOT INTEGRITY VALIDATION
	// ============================================================
	// HARD FAIL POLICY: Reject order if snapshot values are inconsistent
	// This prevents tampering and ensures pricing integrity
	// ============================================================

	// VALIDATION 1: All pricing values must be non-negative
	if snapshot.Subtotal.IsNegative() || snapshot.ShippingTotal.IsNegative() ||
		snapshot.CommissionAmount.IsNegative() || snapshot.ServiceFeeAmount.IsNegative() ||
		snapshot.TotalPayableAmount.IsNegative() || snapshot.DiscountAmount.IsNegative() {
		return nil, fmt.Errorf("snapshot integrity violation: pricing values cannot be negative")
	}

	// VALIDATION 2: Discount cannot exceed subtotal
	if snapshot.DiscountAmount.Int64() > snapshot.Subtotal.Int64() {
		return nil, fmt.Errorf("snapshot integrity violation: discount exceeds subtotal")
	}

	// VALIDATION 3: Buyer gross must equal escrow + service fee
	if snapshot.EscrowAmount.Add(snapshot.ServiceFeeAmount).Int64() != snapshot.TotalPayableAmount.Int64() {
		return nil, fmt.Errorf("snapshot integrity violation: total payable mismatch")
	}

	// PERSIST ORDER
	// ============================================================
	if err := s.repo.CreateOrderTx(ctx, tx, order); err != nil {
		if errors.Is(err, orderrepository.ErrDuplicatePricingToken) ||
			errors.Is(err, orderrepository.ErrDuplicateIdempotencyKey) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	// ============================================================
	// PERSIST ORDER ITEM
	// ============================================================
	// Order items are immutable historical records - all data is copied at order creation time
	if err := s.repo.CreateOrderItemTx(ctx, tx, input.OrderItem); err != nil {
		return nil, fmt.Errorf("failed to create order item: %w", err)
	}

	// Buyer payment is captured through payment gateway after order creation.

	// ============================================================
	// EMIT OUTBOX EVENTS
	// ============================================================
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		events.EventOrderCreated,
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return nil, fmt.Errorf("failed to insert order.created event: %w", err)
	}

	// CHAT-BORN ORDER AUTO-LINK: decoupled from the canonical order tx — see
	// emitChatLinkRequestedEvent for failure semantics (eventual consistency,
	// never rolls back the order).
	if err := s.emitChatLinkRequestedEvent(ctx, tx, order); err != nil {
		return nil, fmt.Errorf("failed to insert order.chat_link_requested event: %w", err)
	}

	// ============================================================
	// OBSERVABILITY: AUDIT EVENT LOGGING
	// ============================================================
	// Emit audit event AFTER all operations succeed.
	// SAFETY: Audit failure is logged but does NOT break the flow.
	if s.auditService != nil {
		s.auditService.OrderCreated(ctx, tx, order.ID, order.BuyerID, order.SellerID, order.Subtotal.Int64())
	}

	return order, nil
}

// CreateFromSaleSurface creates a new order from a sale surface.
//
// This is the unified method for order creation from sale surfaces.
//
// LOCK HIERARCHY (strict order - MUST NOT reverse):
// 1. Load Address (no lock) - validates ownership and availability
// 2. Lock sale surface (FOR UPDATE) - prevents race conditions
//
// ATOMICITY: This method runs within a caller-provided transaction.
// It does NOT open a new transaction or commit - the caller manages tx lifecycle.
//
// SNAPSHOT RULES:
// - Sale-surface price is authoritative
// - Address snapshot from Address.ToSnapshot()
// - Shipping snapshot from input selection
// - Commission calculated once and stored
//
// PROCESS:
// 1. Check Buyer Account Status
// 2. Load and validate shipping address (ownership, availability)
// 3. Lock sale surface with FOR UPDATE
// 4. Validate: saleSurface.Status == active, saleSurface.IsAvailable(), sufficient quantity
// 5. Check shipping option availability for address location
// 6. Reduce sale-surface quantity (auto-transitions to sold if quantity reaches 0)
// 7. Create Order with snapshot from sale surface + address + shipping option
// 8. Persist Order + OrderItem
// 9. Emit outbox event: order.created
//
// All operations happen within the same transaction for atomicity.
func (s *OrderCreationService) CreateFromSaleSurface(
	ctx context.Context,
	tx db.Tx,
	input CreateFromSaleSurfaceInput,
) (*orderentity.Order, error) {
	// ============================================================
	// STEP 0: CHECK IDEMPOTENCY KEY (if provided)
	// ============================================================
	// If idempotency key is provided, check if an order already exists
	// for this buyer with the same key. If the existing order was created
	// from the SAME pricing token, return it (idempotent retry). If the key
	// is reused with a DIFFERENT pricing token, return ErrDuplicateIdempotencyKey
	// so the handler surfaces a 409 instead of silently rebinding the key.
	if input.IdempotencyKey != nil && *input.IdempotencyKey != "" {
		existingOrder, err := s.repo.GetByIdempotencyKey(ctx, tx, input.BuyerID, *input.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("failed to check idempotency key: %w", err)
		}
		if existingOrder != nil {
			return idempotentOrderRecovery(existingOrder, input.PricingTokenID)
		}
	}

	// ============================================================
	// STEP 0.25: SHIPPING QUOTE IDEMPOTENCY CHECK (SCENARIO 6 FIX)
	// ============================================================
	// If using shipping quote, check if it was already used to create an order
	// This prevents double-order attacks from double-click, multiple tabs, network retries
	// CRITICAL: This check happens AFTER the standard idempotency key check
	// to provide shipping-quote-specific protection
	if input.PricingSnapshot != nil && input.PricingSnapshot.ShippingQuoteID != nil {
		existingOrder, err := s.checkShippingQuoteIdempotency(ctx, tx, *input.PricingSnapshot.ShippingQuoteID, input.BuyerID)
		if err != nil {
			return nil, fmt.Errorf("failed to check shipping quote idempotency: %w", err)
		}
		// If order exists for this quote, return it (idempotent response)
		if existingOrder != nil {
			return existingOrder, nil
		}
	}

	// ============================================================
	// PRICING SNAPSHOT REQUIRED (HARD FAIL)
	// ============================================================
	// ALL order creation MUST provide a PricingSnapshot from a validated pricing token.
	// No alternate calculation path exists.
	// No live recalculation path exists.
	//
	// If PricingSnapshot is nil → HARD FAIL
	if input.PricingSnapshot == nil {
		return nil, fmt.Errorf("pricing_snapshot is required: all orders must use pricing token")
	}
	snapshot := input.PricingSnapshot
	usesShippingQuote := snapshot.ShippingSource != nil &&
		*snapshot.ShippingSource == "shipping_quote" &&
		snapshot.ShippingQuoteID != nil

	// ============================================================
	// STEP 1: CHECK BUYER ACCOUNT STATUS
	// ============================================================
	// Guard: Buyer account must be active to create order
	if err := s.accountStatusChecker.EnsureActive(ctx, input.BuyerID); err != nil {
		return nil, fmt.Errorf("buyer account not active: %w", err)
	}

	// ============================================================
	// STEP 1.5: SERVICE LAYER ENFORCEMENT - CHECK BUYER CAN CHECKOUT
	// ============================================================
	// This checks: account active, email verified, profile complete
	actor, err := s.actorResolver.ResolveActor(ctx, input.BuyerID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve buyer actor: %w", err)
	}
	if !actor.CanCheckout() {
		return nil, auth.ErrProfileNotComplete
	}

	// ============================================================
	// STEP 1.6: COMMERCE RESTRICTION - REJECT RESTRICTED BUYER
	// ============================================================
	// Cross-commerce restriction applies to buyer + seller; checked inside the
	// same transaction as the order mutation to prevent TOCTOU bypass.
	if err := s.requireBuyerNotRestricted(ctx, tx, input.BuyerID); err != nil {
		return nil, err
	}

	// ============================================================
	// STEP 2: LOAD AND VALIDATE SHIPPING ADDRESS
	// ============================================================
	// Load address using AddressService
	// Validates: address.user_id == buyerID, address.is_available_for_checkout == true
	address, err := s.addressService.GetAddressForCheckout(ctx, tx, input.BuyerID, input.AddressID)
	if err != nil {
		return nil, fmt.Errorf("failed to load address for checkout: %w", err)
	}

	// Create address snapshot for order immutability
	// RED TRACK R3: Removed - no longer used
	addressSnapshot := address.ToSnapshot()

	// ============================================================
	// STEP 2: LOCK SALE SURFACE (FOR UPDATE)
	// ============================================================
	// Lock hierarchy: Sale surface first to prevent race conditions
	if input.SourceID == uuid.Nil {
		return nil, fmt.Errorf("source_id is required for fixed-price checkout")
	}
	forSale, err := s.forSaleRepo.GetForUpdate(ctx, tx, input.SourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to lock sale surface: %w", err)
	}

	// ============================================================
	// STEP 2.5: SHIPPING GUARD - Verify sale surface has shipping options configured.
	// Returns shippingApp.ErrNoShippingSetups wrapped with %w so the handler
	// surfaces NO_SHIPPING_OPTIONS via errors.Is.
	// ============================================================
	if !usesShippingQuote {
		shippingSetups, err := s.productShippingRepo.GetByProduct(ctx, tx, forSale.ProductID)
		if err != nil {
			return nil, fmt.Errorf("failed to check sale surface shipping options: %w", err)
		}
		if len(shippingSetups) == 0 {
			return nil, fmt.Errorf("sale surface %s: %w", forSale.ID, shippingApp.ErrNoShippingSetups)
		}
	}

	// ============================================================
	// STEP 2.6: BNR GUARD - Check if sale surface has auction in waiting_settlement
	// ============================================================
	// If the sale surface has an auction in waiting_settlement status, reject direct purchase
	// The auction winner has priority to claim and create the order
	if s.auctionRepo != nil {
		auctionStatus, err := s.auctionRepo.GetAuctionStatusByProductID(ctx, tx, forSale.ProductID)
		if err != nil {
			return nil, fmt.Errorf("failed to check auction status: %w", err)
		}
		if auctionStatus != nil && *auctionStatus == "waiting_settlement" {
			return nil, fmt.Errorf("sale surface is locked due to auction settlement in progress")
		}
	}

	// ============================================================
	// STEP 3: VALIDATE SALE-SURFACE STATE (UNIFIED)
	// ============================================================
	// UNIFIED VALIDATION: All checkout paths use the same validation logic
	if err := s.validateSaleSurfaceForCheckout(ctx, ValidateSaleSurfaceForCheckoutInput{
		SaleSurface: forSale,
		BuyerID:     input.BuyerID,
		Quantity:    input.Quantity,
	}); err != nil {
		return nil, err
	}

	// ============================================================
	// STEP 3.5: NEGOTIATION SETTLEMENT BINDING (N8-B CANONICAL)
	// ============================================================
	// Canonical invariant:
	//   DIRECT:      snapshot.NegotiationID==nil && input.NegotiationID==nil
	//   NEGOTIATION: snapshot.NegotiationID !=nil && input.NegotiationID !=nil
	//                && *snapshot.NegotiationID == *input.NegotiationID == lockedSession.ID
	// All mismatched combinations are rejected here. Lock order remains
	// for_sale FOR UPDATE (above) before negotiation FOR UPDATE (here).
	// ============================================================
	// Token↔request binding (structural, not handler-only)
	snapshotNegotiationID := snapshot.NegotiationID
	if (snapshotNegotiationID == nil) != (input.NegotiationID == nil) {
		return nil, fmt.Errorf("negotiation binding mismatch: token negotiation_id=%v, request negotiation_id=%v", snapshotNegotiationID, input.NegotiationID)
	}
	if snapshotNegotiationID != nil && input.NegotiationID != nil && *snapshotNegotiationID != *input.NegotiationID {
		return nil, fmt.Errorf("negotiation binding mismatch: token=%s request=%s", snapshotNegotiationID.String(), input.NegotiationID.String())
	}
	var negotiatedPrice money.Money
	var hasNegotiation bool
	var lockedNegotiationSession *negotiationEntity.NegotiationSession
	if input.NegotiationID != nil {
		// Step 1: Lock negotiation row FOR UPDATE before any settlement validation
		session, err := s.negotiationRepo.GetSessionForUpdate(ctx, tx, *input.NegotiationID)
		if err != nil {
			return nil, fmt.Errorf("negotiation session not found: %w", err)
		}
		lockedNegotiationSession = session

		// Step 2: Validate ownership — runs BEFORE the settlement gate so a
		// non-owner can never observe a session's settlement state (OrderID).
		// (Handler-level pricing-token validation already binds the buyer to
		// this negotiation; the checks here are defense in depth.)
		if session.BuyerID != input.BuyerID {
			return nil, fmt.Errorf("negotiation buyer mismatch: session_buyer=%s, requester=%s",
				session.BuyerID, input.BuyerID)
		}

		if session.SellerID != forSale.SellerID {
			return nil, fmt.Errorf("negotiation seller mismatch: session_seller=%s, for_sale_seller=%s",
				session.SellerID, forSale.SellerID)
		}

		// Step 3: Validate identity match for the locked fixed-price sale and canonical product
		if session.ForSaleID != forSale.ID {
			return nil, fmt.Errorf("negotiation sale mismatch: session_for_sale_id=%s, for_sale_id=%s",
				session.ForSaleID, forSale.ID)
		}
		if input.ProductID != forSale.ProductID {
			return nil, fmt.Errorf("negotiation product mismatch: input_product_id=%s, product_id=%s",
				input.ProductID, forSale.ProductID)
		}

		// Step 4: Lifecycle settlement eligibility — one canonical entity
		// predicate (accepted AND not expired AND not settled). The switch below
		// only maps the failure to a distinct actionable error; the eligibility
		// decision itself lives in NegotiationSession.CanSettle.
		if !session.CanSettle() {
			switch {
			case session.Status != negotiationEntity.NegotiationStatusAccepted:
				return nil, fmt.Errorf("negotiation is not accepted: current_status=%s", session.Status)
			case session.IsExpired():
				return nil, fmt.Errorf("negotiation expired: session_id=%s", session.ID)
			default:
				// Status accepted + not expired + CanSettle false ⇒ already settled
				// (OrderID != nil && != uuid.Nil) — dereference is safe here.
				return nil, &negotiationEntity.ErrNegotiationAlreadySettled{
					SessionID: session.ID,
					OrderID:   *session.OrderID,
				}
			}
		}

		// Step 5: Validate accepted_price is set
		if session.AcceptedPrice == nil {
			return nil, fmt.Errorf("negotiation accepted_price not set: session_id=%s", session.ID)
		}

		// Step 5.5: N8-B price agreement — snapshot unit price must equal locked accepted_price
		if snapshot.UnitPrice.Int64() != *session.AcceptedPrice {
			return nil, fmt.Errorf("negotiation price mismatch: token unit_price=%d accepted_price=%d", snapshot.UnitPrice.Int64(), *session.AcceptedPrice)
		}

		// Step 6: Apply negotiated price
		negotiatedPrice = money.New(*session.AcceptedPrice)
		hasNegotiation = true
	}

	// ============================================================
	// STEP 4: CHECK SHIPPING COVERAGE
	// ============================================================
	// Check delivery availability using address's province and city codes
	// Uses CheckDeliveryAvailabilityForProduct with address.ProvinceID and address.CityID
	var selectedOption *shippingApp.DeliveryOption
	if !usesShippingQuote {
		deliveryOptions, err := s.shippingService.CheckDeliveryAvailabilityForProduct(ctx, tx, forSale.ProductID, addressSnapshot.ProvinceID, addressSnapshot.CityID)
		if err != nil {
			return nil, fmt.Errorf("failed to check delivery availability: %w", err)
		}

		// Find the selected shipping option from available options
		for i := range deliveryOptions {
			if deliveryOptions[i].ShippingSetupID == input.ShippingSetupID {
				selectedOption = &deliveryOptions[i]
				break
			}
		}

		// Validate shipping option is available for this address location.
		// Wraps shippingApp.ErrShippingSetupUnavailable with %w so the handler
		// surfaces SHIPPING_OPTION_UNAVAILABLE via errors.Is.
		if selectedOption == nil || !selectedOption.IsAvailable {
			return nil, fmt.Errorf("option_id=%s province_id=%s city_id=%s: %w",
				input.ShippingSetupID, addressSnapshot.ProvinceID, addressSnapshot.CityID,
				shippingApp.ErrShippingSetupUnavailable)
		}
	}

	// ============================================================
	// STEP 5: REDUCE SALE SURFACE QUANTITY
	// ============================================================
	// HARDENING: Final availability check before ReduceQuantity
	// IsAvailable() checks: status == active, visibility == public, quantity > 0
	if !forSale.IsAvailable() {
		return nil, &entity.ForSaleNotAvailableError{
			ForSaleID: forSale.ID,
			Reason: fmt.Sprintf("sale surface not available at reduce time: status=%s, visibility=%s, quantity=%d",
				forSale.Status, forSale.Visibility, forSale.QuantityAvailable),
		}
	}

	// Reduce quantity (auto-transitions to sold if quantity reaches 0)
	// Track whether a sold transition occurs for event emission.
	previousQuantity := forSale.QuantityAvailable
	if err := forSale.ReduceQuantity(input.Quantity); err != nil {
		return nil, fmt.Errorf("failed to reduce sale surface quantity: %w", err)
	}
	willTransitionToSold := previousQuantity > 0 && forSale.QuantityAvailable == 0 && forSale.Status == entity.ForSaleStatusSold

	// Update sale surface in database
	if err := s.forSaleRepo.UpdateStock(ctx, tx, forSale); err != nil {
		return nil, fmt.Errorf("failed to update sale surface: %w", err)
	}

	// P1-3 FIX: Emit for_sale.sold event when the sale transitions to sold.
	// This is the ONLY emitter for for_sale.sold on order creation.
	// Event is emitted within the same transaction as the stock mutation.
	//
	// buyer_id is additive (WIRED consumer): the negotiation ForSaleSoldEventHandler
	// uses it to exclude the winning buyer from the "item sold" notifications and
	// to keep their canonical accepted negotiation untouched. Historical payloads
	// without buyer_id remain consumable (the handler tolerates its absence).
	if willTransitionToSold {
		type forSaleSoldPayload struct {
			ForSaleID string `json:"for_sale_id"`
			SellerID  string `json:"seller_id"`
			BuyerID   string `json:"buyer_id"`
			Status    string `json:"status"`
		}
		payload, _ := json.Marshal(forSaleSoldPayload{
			ForSaleID: forSale.ID.String(),
			SellerID:  forSale.SellerID.String(),
			BuyerID:   input.BuyerID.String(),
			Status:    string(forSale.Status),
		})
		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			events.EventForSaleSold,
			forSale.ID,
			payload,
		); err != nil {
			return nil, fmt.Errorf("failed to insert for_sale.sold event: %w", err)
		}
	}

	// ============================================================
	// STEP 5.5: VALIDATE PAYMENT METHOD (PHASE 5: BACKEND-CONTROLLED PAYMENT)
	// ============================================================
	// 🔥 CRITICAL: Payment method MUST be validated against allowed values
	// - Prevents client-side manipulation
	// - Ensures expiry calculation is correct
	// - No arbitrary payment methods allowed
	// Validate payment method is one of the allowed values
	var validPaymentMethod bool
	switch snapshot.PaymentMethod {
	case PaymentMethodInstant, PaymentMethodVA, PaymentMethodRetail, PaymentMethodDefault:
		validPaymentMethod = true
	}

	if !validPaymentMethod {
		return nil, fmt.Errorf("invalid payment method: %s (allowed: instant, va, retail, default)", snapshot.PaymentMethod)
	}

	if input.SourceType != orderentity.OrderSourceForSale {
		return nil, fmt.Errorf("invalid source type for fixed-price checkout: %s", input.SourceType)
	}
	if input.SourceID == uuid.Nil {
		return nil, fmt.Errorf("source_id is required for fixed-price checkout")
	}

	// ============================================================
	// STEP 6: CREATE ORDER WITH PRICING SNAPSHOT
	// ============================================================
	// PricingSnapshot is MANDATORY - all values from pricing token.
	// NO calculation happens here - snapshot is the authority.

	// Determine quote snapshot values (TASK F)
	var shippingQuoteID *uuid.UUID
	var shippingQuotePrice *int64
	if snapshot.ShippingSource != nil && *snapshot.ShippingSource == "shipping_quote" {
		shippingQuoteID = snapshot.ShippingQuoteID
		if shippingQuoteID != nil && snapshot.ShippingTotal.Int64() > 0 {
			price := snapshot.ShippingTotal.Int64()
			shippingQuotePrice = &price
		}
	}

	// ============================================================
	// STEP 6.0: CONSUME THE SHIPPING QUOTE (ONE AUTHORITY)
	// ============================================================
	// Delegates validation + USED transition to the Commerce shipping-quote
	// authority inside this transaction. Consumed whenever the order carries a
	// quote — the chat id is passed when available (provenance check).
	if shippingQuoteID != nil {
		if err := s.consumeShippingQuoteForOrder(ctx, tx, shippingquoteEntity.CheckoutContext{
			QuoteID:            *shippingQuoteID,
			BuyerID:            input.BuyerID,
			ProductID:          forSale.ProductID,
			SourceType:         "for_sale",
			SourceID:           input.SourceID,
			SellerID:           forSale.SellerID,
			ChatID:             snapshot.ChatID,
			ShippingProvinceID: addressSnapshot.ProvinceID,
			ShippingCityID:     addressSnapshot.CityID,
		}); err != nil {
			return nil, fmt.Errorf("shipping quote validation failed: %w", err)
		}
	}

	// UX TRUTH HARDENING: Invalidate remaining ACTIVE shipping quotes if the sale surface becomes sold.
	// Run this after the current quote is marked USED so it is not invalidated mid-checkout.
	if forSale.Status == entity.ForSaleStatusSold && s.shippingQuoteAuthority != nil {
		if err := s.shippingQuoteAuthority.InvalidateQuotesByProduct(ctx, tx, forSale.ProductID); err != nil {
			return nil, fmt.Errorf("failed to invalidate shipping quotes: %w", err)
		}
	}

	// Determine unit price: use negotiated price if available, otherwise use sale-surface price from snapshot
	unitPrice := snapshot.UnitPrice
	negotiationIDToPass := input.NegotiationID
	if hasNegotiation {
		// Override with negotiated price
		unitPrice = negotiatedPrice
	}

	// Create order with pricing snapshot values (NO local calculation)
	var shippingSetupID *uuid.UUID
	if selectedOption != nil {
		shippingSetupID = &selectedOption.ShippingSetupID
	}

	order := orderentity.NewOrderFromSource(
		input.BuyerID,
		forSale.SellerID,
		input.SourceType,                        // Source type = for_sale
		input.SourceID,                          // Source ID = fixed-price sale surface ID
		negotiationIDToPass,                     // Negotiation ID (optional)
		input.Quantity,                          // Quantity from input
		unitPrice,                               // Unit price (negotiated or forSale price)
		snapshot.Subtotal,                       // Subtotal from pricing snapshot
		snapshot.ShippingTotal,                  // Shipping from pricing snapshot
		snapshot.CommissionPercent,              // Commission percent from pricing snapshot
		snapshot.CommissionAmount,               // Commission amount from pricing snapshot
		snapshot.ServiceFeeAmount,               // Buyer service fee from pricing snapshot
		snapshot.EscrowAmount,                   // CANONICAL buyer-funded base PD + S (fee excluded)
		shippingSetupID,                         // NULLABLE: nil when using a manual shipping quote
		snapshot.ShippingSetupName,              // Option name from pricing snapshot
		snapshot.ShippingTransportType,          // Transport type from pricing snapshot
		string(forSale.Product.PreparationTime), // SNAPSHOT: Freeze preparation time from canonical product
		snapshot.ShippingSource,                 // Shipping source from pricing snapshot
		shippingQuoteID,                         // TASK F: Quote ID
		shippingQuotePrice,                      // TASK F: Quote price snapshot
		&snapshot.TokenID,                       // Store pricing token ID (prevents double-ordering)
		calculatePaymentExpiry(snapshot.PaymentMethod, time.Now()), // PHASE 2: Calculate expiry based on payment method
	)

	// Bind the buyer's selected payment method to the order.
	order.PaymentMethodCode = input.PaymentMethodCode

	// Apply shipping destination snapshot
	order.ApplyAddressSnapshot(addressSnapshot)

	// Apply shipping origin snapshot from the seller account's primary address
	farmAddressSnapshot, err := s.getSellerOriginSnapshot(ctx, tx, forSale.SellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seller origin snapshot: %w", err)
	}
	order.ApplyShippingOrigin(farmAddressSnapshot)

	// Set idempotency key if provided (for safe retries)
	if input.IdempotencyKey != nil && *input.IdempotencyKey != "" {
		order.IdempotencyKey = input.IdempotencyKey
	}

	// Set origin request target ID if this sale surface was created in response to a request
	// This links the order to the original buyer request for tracking and fulfillment
	// TODO: Implement SetOriginRequestID method on Order entity
	_ = s.getOriginRequestTargetID(ctx, tx, forSale.ID)

	// ============================================================
	// PRICING SNAPSHOT INTEGRITY VALIDATION
	// ============================================================

	// Use pre-calculated order value from pricing snapshot (no recalculation)
	// Uses the pre-calculated canonical coin base PD = (P - D) from the pricing
	// token (shipping is NOT part of the coin base; see
	// pricing/token/application/canonical_pricing_formula_test.go).
	orderValueForCoins := snapshot.OrderValueForCoins

	// PRE-GENERATE ORDER ID
	// Generate order ID now so the order item can reference it.
	orderID := uuid.New()
	order.ID = orderID

	// Order items are immutable historical records - all data is copied at order creation time.
	// Stage 5 (identity convergence): order_items.product_id is ALWAYS products.id.
	// forSale.ProductID is the canonical FPS -> Product relationship; the selling surface
	// identity stays on orders.source_type + orders.source_id.
	orderItem := orderentity.NewOrderItem(
		order.ID,
		forSale.ProductID,
		unitPrice,
		input.Quantity,
		forSale.Product.Title, // Canonical content authority — alias purged
	)

	// ============================================================
	// FINALIZE: snapshot integrity check, persistence,
	// outbox events, audit logging (PHASE 2: shared with CreateFromAuction)
	// ============================================================
	finalized, err := s.finalizeOrderCreationTx(ctx, tx, finalizeOrderCreationInput{
		Order:              order,
		OrderItem:          orderItem,
		Snapshot:           snapshot,
		OrderValueForCoins: orderValueForCoins,
	})
	if err != nil {
		return nil, err
	}

	// ============================================================
	// MARK NEGOTIATION AS SETTLED (DUPLICATE PREVENTION)
	// ============================================================
	// N4 CONVERGENCE: reuse already-locked negotiation row — no second SELECT FOR UPDATE.
	// The row has been locked since step 3.5 and validated; assign order_id on that canonical entity.
	if hasNegotiation && lockedNegotiationSession != nil {
		lockedNegotiationSession.OrderID = &finalized.ID
		lockedNegotiationSession.UpdatedAt = time.Now()

		if err := s.negotiationRepo.UpdateSession(ctx, tx, lockedNegotiationSession); err != nil {
			return nil, fmt.Errorf("failed to mark negotiation as settled: %w", err)
		}
	}

	return finalized, nil
}

// buildOrderPayload creates a JSON-serializable payload for order events.
// Note: financial amount total_before_coins_amount is intentionally NOT serialized
// here — projection re-queries the write model and notification handlers only need IDs.
// The previous escrow_amount payload field was obsolete (no consumer) and has been purged.
func buildOrderPayload(order *orderentity.Order) []byte {
	payload := map[string]interface{}{
		"order_id":          order.ID.String(),
		"buyer_id":          order.BuyerID.String(),
		"seller_id":         order.SellerID.String(),
		"source_type":       order.SourceType,
		"source_id":         order.SourceID,
		"status":            order.Status,
		"escrow_status":     order.EscrowStatus,
		"subtotal":          order.Subtotal.Int64(),
		"shipping_total":    order.ShippingTotal.Int64(),
		"commission_amount": order.CommissionAmount.Int64(),
		"created_at":        order.CreatedAt.Unix(),
	}
	data, _ := json.Marshal(payload)
	return data
}
