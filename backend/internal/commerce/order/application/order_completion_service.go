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
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepoImpl "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	forSaleRepoImpl "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	forSalerepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	"github.com/labuda/backend/internal/commerce/governance/commercegov"
	"github.com/labuda/backend/internal/commerce/order/entity"
	orderRepoImpl "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	ratingApp "github.com/labuda/backend/internal/commerce/order/rating/application"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	escrowApp "github.com/labuda/backend/internal/core/escrow/application"
	disputeEntity "github.com/labuda/backend/internal/governance/dispute/entity"
	disputerepo "github.com/labuda/backend/internal/governance/dispute/repository"
	supportRepoImpl "github.com/labuda/backend/internal/governance/support/infrastructure/repository"
	supportrepo "github.com/labuda/backend/internal/governance/support/repository"
	"github.com/labuda/backend/internal/identity/auth"
	coinsApp "github.com/labuda/backend/internal/incentive/coins/application"
	paymentRepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/events"
	idempotencyRepo "github.com/labuda/backend/internal/platform/idempotency/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ErrFinalRefundDecisionOwesBuyer is returned when escrow is asked to be
// released to the seller while the order's refund process already carries a
// FINAL decision that owes the buyer money (seller ACCEPT, admin buyer-wins, or
// a platform-initiated refund). That decision is final, so the escrow belongs to
// the buyer: paying the seller as well would move the same money twice.
var ErrFinalRefundDecisionOwesBuyer = errors.New(
	"cannot release escrow: the refund decision to pay the buyer is final",
)

// ErrRefundReleaseGuardNotConfigured is returned when Complete() is called
// without a wired RefundReleaseGuard. Fail-closed by design: escrow must not be
// released if the refund guard is absent.
var ErrRefundReleaseGuardNotConfigured = fmt.Errorf(
	"order: refund release guard not configured; cannot complete order safely")

// ============================================================================
// ESCROW STATUS DERIVATION
// ============================================================================

// mapEscrowToOrderEscrow maps Escrow.Status to Order.EscrowStatus.
//
// CRITICAL: This is the ONLY valid way to set Order.EscrowStatus.
// Order.EscrowStatus MUST always be derived from Escrow.Status.
//
// Escrow.Status values (from escrow/entity/escrow.go):
// - "holding": Funds held for pending order
// - "released": Released to seller (order complete)
// - "refunded": Refunded to buyer (order cancelled)
//
// This function ensures Order.EscrowStatus is a READ-ONLY projection of Escrow state.
func mapEscrowToOrderEscrow(escrowStatus string) entity.EscrowStatus {
	switch escrowStatus {
	case "holding":
		return entity.EscrowStatusHolding
	case "released":
		return entity.EscrowStatusReleased
	case "refunded":
		return entity.EscrowStatusRefunded
	default:
		// If no escrow row exists or unknown state, default to holding
		// This should not happen in practice, but provides safe fallback
		return entity.EscrowStatusHolding
	}
}

// OrderCompletionService handles order state transitions and completion operations.
//
// RATING DOMAIN BOUNDARY:
// - Uses RatingMutator interface for rating invalidation operations
// - CANNOT access rating repository directly
// - Enforces clear separation between order and rating domains
//
// ESCROW INTEGRATION:
// - Uses EscrowService to fetch escrow state for deriving Order.EscrowStatus
// - Order.EscrowStatus is ALWAYS derived from Escrow.Status
type OrderCompletionService struct {
	repo                  orderrepository.OrderRepository
	forSaleRepo           forSalerepo.ForSaleRepository
	auctionRepo           *auctionRepoImpl.AuctionRepository // PASS_20B: auction order-binding release on cancel/expire
	commerceViolationRepo commercegov.Repository             // Canonical violation/restriction authority for settlement failure
	ownership             *auth.OwnershipValidator
	accountStatusChecker  auth.AccountStatusChecker
	outboxRepo            *outboxRepo.OutboxRepository
	idempotencyRepo       *idempotencyRepo.Repository
	paymentService        *OrderPaymentService
	paymentRepo           *paymentRepo.PaymentRepository
	coinsService          *coinsApp.CoinsService  // Used for earning points on completion (NOT for refunds)
	ratingMutator         ratingApp.RatingMutator // Interface-based access (write-only)
	supportRepo           supportrepo.Repository
	shippingQuoteService  ShippingQuoteService
	disputeRepo           disputerepo.DisputeRepository // Entry point guard: check dispute status
	escrowService         *escrowApp.EscrowService      // Used to derive Order.EscrowStatus from Escrow state
	refundReleaseGuard    RefundReleaseGuard            // Canonical: block release while a refund must be respected
	refundDecisionAuth    RefundDecisionAuthority       // Canonical: record the admin's final refund decision on the refund process row
	logger                *zap.Logger
}

// ShippingQuoteService defines the interface for shipping quote operations.
type ShippingQuoteService interface {
	ReactivateQuoteIfEligible(ctx context.Context, tx db.Tx, quoteID uuid.UUID) error
	// InvalidateQuotesByProduct marks all ACTIVE unsuperseded quotes for a
	// product as INVALID. Called during auction settlement failure to prevent
	// stale quotes from being usable in the next settlement lifecycle.
	InvalidateQuotesByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) error
}

// RefundReleaseGuard answers the ONE question the order lifecycle must ask
// before releasing money to the seller: does this order have a refund that still
// has to be respected?
//
// It blocks when money owed to the buyer has not settled at the gateway yet, or
// when the refund decision is still open while the order's own refund window is
// still open (refundWindowOpen — owned by the order domain via
// Order.IsRefundWindowOpen). Implemented by the refund repository as the SQL
// mirror of refundEntity.Refund.BlocksOrderRelease.
type RefundReleaseGuard interface {
	HasRefundBlockingRelease(ctx context.Context, tx db.Tx, orderID uuid.UUID, refundWindowOpen bool) (bool, error)
}

// RefundDecisionAuthority lets the dispute/order side record the ADMIN's final
// refund decision on the order's refund process WITHOUT holding refund authority
// itself. Implemented by the refund domain (RefundService).
//
// buyerWins=true records a final buyer-wins decision and dispatches the gateway
// refund on the same refund row. buyerWins=false records a final seller-wins
// decision (no money to the buyer).
//
// recorded=false means the order has NO refund process (a direct dispute); the
// caller then creates the platform refund record instead.
type RefundDecisionAuthority interface {
	AdminResolveRefundDecision(ctx context.Context, tx db.Tx, orderID, adminID uuid.UUID, buyerWins bool, amount int64, notes *string) (recorded bool, err error)
	// HasFinalRefundDecisionOwedToBuyer reports whether the order's refund
	// process already carries a FINAL decision that owes the buyer money
	// (seller ACCEPT, admin buyer-wins, platform-initiated refund). A final
	// decision may not be paid over to the seller as well.
	HasFinalRefundDecisionOwedToBuyer(ctx context.Context, tx db.Tx, orderID uuid.UUID) (bool, error)
}

// reactivateShippingQuoteIfEligible reactivates a USED shipping quote after an
// order failure — but ONLY for fixed-price (for_sale) orders.
//
// QUOTE ISOLATION (canonical auction settlement): auction-sourced orders must
// NEVER reactivate their quote on expiry/cancel/refund. A relist reuses the
// same auction record (source_type=auction, source_id=auction.id), so a
// reactivated old quote would become the current settlement authority for the
// relist. The quote stays a historical record; the next settlement must obtain
// a fresh quote (or normal shipping).
func (s *OrderCompletionService) reactivateShippingQuoteIfEligible(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
) {
	if order.SourceType == entity.OrderSourceAuction {
		return
	}
	if order.ShippingQuoteID != nil && *order.ShippingQuoteID != uuid.Nil {
		if err := s.shippingQuoteService.ReactivateQuoteIfEligible(ctx, tx, *order.ShippingQuoteID); err != nil {
			s.logger.Warn("Failed to reactivate shipping quote after order failure",
				zap.String("order_id", order.ID.String()),
				zap.String("shipping_quote_id", order.ShippingQuoteID.String()),
				zap.Error(err),
			)
		}
	}
}

// SetCommerceViolationRepo wires the canonical commerce violation/restriction
// repository used when an auction order expires unpaid and its auction returns
// to DRAFT. Called post-construction from serverboot.
func (s *OrderCompletionService) SetCommerceViolationRepo(repo commercegov.Repository) {
	s.commerceViolationRepo = repo
}

// NewOrderCompletionService creates a new OrderCompletionService.
func NewOrderCompletionService(
	accountStatusChecker auth.AccountStatusChecker,
	outboxRepo *outboxRepo.OutboxRepository,
	paymentService *OrderPaymentService,
	coinsService *coinsApp.CoinsService,
	shippingQuoteService ShippingQuoteService,
	disputeRepo disputerepo.DisputeRepository,
	escrowService *escrowApp.EscrowService,
	logger *zap.Logger,
) *OrderCompletionService {
	// RATING DOMAIN BOUNDARY: Use factory to get rating mutator interface
	ratingFactory := ratingApp.NewRatingDomainFactory()

	return &OrderCompletionService{
		repo:                 orderRepoImpl.NewOrderRepository(),
		forSaleRepo:          forSaleRepoImpl.NewForSaleRepository(),
		auctionRepo:          auctionRepoImpl.NewAuctionRepository(),
		ownership:            auth.NewOwnershipValidator(),
		accountStatusChecker: accountStatusChecker,
		outboxRepo:           outboxRepo,
		idempotencyRepo:      idempotencyRepo.NewRepository(),
		paymentService:       paymentService,
		paymentRepo:          paymentRepo.NewPaymentRepository(),
		coinsService:         coinsService,               // Used for earning points on completion (NOT for refunds)
		ratingMutator:        ratingFactory.GetMutator(), // Interface-based: write-only access for invalidation
		supportRepo:          supportRepoImpl.NewSupportRepository(),
		shippingQuoteService: shippingQuoteService,
		disputeRepo:          disputeRepo,   // Entry point guard: check dispute status before resolution
		escrowService:        escrowService, // Used to derive Order.EscrowStatus from Escrow state
		logger:               logger,
	}
}

// SetRefundDecisionAuthority wires the refund domain's admin-decision write-back
// used by the dispute resolution paths. Without it, admin decisions on escalated
// refunds fail closed (the refund process would otherwise stay unresolved).
func (s *OrderCompletionService) SetRefundDecisionAuthority(authority RefundDecisionAuthority) {
	s.refundDecisionAuth = authority
}

// SetCoinsService wires the coins service after construction.
//
// CoinsService is built after the order services in the bootstrap, so this
// setter exists for post-construction injection (mirrors the
// SetShippingQuoteService / SetDisputeRepository pattern). Loyalty points are
// a NON-CANONICAL side effect; Complete() guards against a nil coinsService
// and skips the reward rather than panicking, so a missing setter call only
// loses loyalty points — it never blocks the canonical release path.
func (s *OrderCompletionService) SetCoinsService(coinsService *coinsApp.CoinsService) {
	s.coinsService = coinsService
}

// MarkPaid transitions an order from pending to paid.
// Locks the row, validates transition, and persists the status update.
//
// UNIFIED SETTLEMENT MODEL V2:
// - ESCROW funding is handled by PaymentSettlementService (payment layer)
// - This method ONLY manages order state transitions
// - No ledger entries created here (prevents double escrow posting)
//
// CRITICAL: Order.EscrowStatus is DERIVED from Escrow.Status
// This ensures Order.EscrowStatus is ALWAYS a projection of Escrow state.
//
// IDEMPOTENCY: If order is already in paid status, returns success immediately.
// This prevents duplicate state transitions on retry.
//
// CRITICAL ORDER:
// 1. Lock order and validate transition
// 2. CRITICAL: Check if order is expired (PHASE 6 DEFENSIVE GUARD)
// 3. Check idempotency (already paid -> return success)
// 4. Fetch escrow state
// 5. Derive Order.EscrowStatus from escrow state
// 6. Update order status (pending -> paid)
// 7. Emit outbox event
//
// This ensures that if the state update succeeds but outbox fails,
// the outbox worker can still reconcile from the order status.
//
// LEDGER ENTRIES: None (created by PaymentSettlementService.SettlePaymentByID)
func (s *OrderCompletionService) MarkPaid(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	// Step 1: Lock and validate
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// Step 2: CRITICAL EXPIRY CHECK (PHASE 6 DEFENSIVE GUARD)
	// HARD BLOCK: Cannot mark paid if order is expired
	// This prevents payment after expiry window
	if order.IsExpired() {
		return errors.New("cannot mark paid - order expired")
	}

	// Step 3: IDEMPOTENCY CHECK - If already paid, return success
	if order.Status == entity.StatusPaid {
		// Already in target state - idempotent operation
		return nil
	}

	// Step 4: CRITICAL - Fetch escrow state to derive Order.EscrowStatus
	// This ensures Order.EscrowStatus is ALWAYS a projection of Escrow state
	escrowRow, err := s.escrowService.GetEscrowForOrder(ctx, tx, orderID)
	if err != nil {
		s.logger.Error("failed_to_fetch_escrow",
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to fetch escrow for order: %w", err)
	}

	// Step 5: Derive Order.EscrowStatus from Escrow state
	// If escrow doesn't exist yet (edge case), default to holding
	// This should not happen in normal flow since SettlePaymentByID creates escrow first
	var derivedEscrowStatus entity.EscrowStatus
	if escrowRow == nil {
		s.logger.Warn("escrow_not_found_for_paid_order",
			zap.String("order_id", orderID.String()),
			zap.String("reason", "escrow_should_exist_after_payment"),
		)
		derivedEscrowStatus = entity.EscrowStatusHolding // Default for paid orders
	} else {
		derivedEscrowStatus = mapEscrowToOrderEscrow(escrowRow.Status.String())
	}

	// Step 6: Update order state
	if err := order.MarkPaid(); err != nil {
		return err
	}

	// CRITICAL: Set EscrowStatus from escrow row, not from business logic
	order.EscrowStatus = derivedEscrowStatus

	// No ledger entries here - escrow already funded by PaymentSettlementService
	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// Settlement success: when this is an auction-sourced order and the auction
	// is still in waiting_settlement (bid-win claim flow), settle the auction
	// to ended atomically with payment success. Buy-now auctions are already
	// ended at order creation; no-winner/for-sale orders are untouched.
	if order.SourceType == entity.OrderSourceAuction {
		if err := s.settleAuctionOnPaymentSuccess(ctx, tx, order); err != nil {
			return err
		}
	}

	// Step 5: Emit outbox event
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		events.EventOrderPaid,
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	return nil
}

// settleAuctionOnPaymentSuccess settles a bid-win auction (waiting_settlement)
// to ended once its bound order is paid. Idempotent: an auction already ended
// (buy-now, or a retried MarkPaid) is a no-op.
func (s *OrderCompletionService) settleAuctionOnPaymentSuccess(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
) error {
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, order.SourceID)
	if err != nil {
		return fmt.Errorf("failed to lock auction for payment settlement: %w", err)
	}
	if auction.Status != auctionEntity.StatusWaitingSettlement {
		// Already ended (buy-now) or returned to draft via a concurrent expiry —
		// nothing to settle.
		return nil
	}
	if err := auction.Settle(); err != nil {
		return fmt.Errorf("failed to settle auction on payment success: %w", err)
	}
	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return fmt.Errorf("failed to persist auction settlement: %w", err)
	}
	return nil
}

// MarkShipped transitions an order from paid to shipped.
// AUTHORIZATION: Only the seller can mark an order as shipped, or system caller.
//
// SHIPPING PROOF REQUIREMENTS (STRICT - NO FAKE SHIPMENT):
// - proofType: REQUIRED - "tracking" | "phone" | "manual"
// - shippingReference: REQUIRED for tracking/phone types
// - shippingProofMedia: REQUIRED for manual type
// - note: Optional shipping note
//
// BUSINESS RULE: Auto-complete timer starts when seller marks order as shipped.
//
// IDEMPOTENCY: Uses idempotencyKey to ensure safe retries.
// If a record with the same idempotency key exists, returns nil (operation already performed).
// The idempotency check is performed inside the transaction for atomicity.
func (s *OrderCompletionService) MarkShipped(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	orderID uuid.UUID,
	idempotencyKey string,
	proofType *string,
	shippingReference *string,
	shippingProofMedia *string,
	note *string,
) error {
	// ============================================================
	// STEP 1: IDEMPOTENCY CHECK (inside transaction)
	// ============================================================
	operation := fmt.Sprintf("order.shipped.%s", orderID.String())
	if err := s.idempotencyRepo.TryInsert(ctx, tx, idempotencyKey, operation, orderID); err != nil {
		if errors.Is(err, idempotencyRepo.ErrAlreadyExists) {
			// Idempotent - operation already performed
			return nil
		}
		return err
	}

	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return err
	}

	// Get order with row lock FIRST (for ban check context)
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// AUTHORIZATION: Only seller can mark as shipped (or system caller)
	if !auth.IsSystemCaller(callerID) && !s.ownership.IsSeller(callerID, order.SellerID) {
		return auth.ErrSellerRequired
	}

	// MODERATION DOMAIN HARD CHECK (STEP 1 & 2):
	// - banned users CANNOT mark shipped
	// - banned seller cannot control order after ban
	// - only system can proceed
	if !auth.IsSystemCaller(callerID) {
		if err := s.accountStatusChecker.EnsureActive(ctx, callerID); err != nil {
			return fmt.Errorf("seller cannot mark shipped: %w", err)
		}
	}

	if err := order.MarkShipped(proofType, shippingReference, shippingProofMedia, note); err != nil {
		return err
	}

	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// Emit outbox event for order.shipped
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"order.shipped",
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	return nil
}

// Complete transitions an order from shipped (or delivered) to completed.
// B4A: This is the canonical "Terima Barang" path — buyer's single-click final acceptance.
//
// AUTHORIZATION: Only the buyer can complete an order (releases escrow to seller), or system caller.
//
// CRITICAL IDEMPOTENCY (defense-in-depth):
// 1. LAYER 1 - Idempotency repo: Uses key "order.complete.<order_id>" (if caller provides key)
// 2. LAYER 2 - Service check: Returns success if already completed (no-op)
// 3. LAYER 3 - Ledger idempotency: Uses key "order_release_<order_id>"
// 4. LAYER 4 - Service guard: HasDispute check immediately after GetForUpdate (RACE PREVENTION)
// 5. LAYER 5 - Entity guards: order.ValidateComplete() checks HasDispute and EscrowStatus
//
// MULTI-LAYER SAFETY (prevents auto-completing disputed orders):
// - Query layer: has_dispute = false excludes disputed orders from worker
// - Service layer: HasDispute check immediately after GetForUpdate (THIS METHOD)
// - Entity layer: order.ValidateComplete() returns DisputeActiveError if HasDispute = true
func (s *OrderCompletionService) Complete(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	orderID uuid.UUID,
	idempotencyKey string,
) error {
	// ============================================================
	// STEP 0: IDEMPOTENCY CHECK (inside transaction)
	// ============================================================
	// B4A: Complete() is now buyer-facing with Idempotency-Key header.
	// Empty key = system/worker caller (uses ledger-level idempotency only).
	if idempotencyKey != "" {
		operation := fmt.Sprintf("order.complete.%s", orderID.String())
		if err := s.idempotencyRepo.TryInsert(ctx, tx, idempotencyKey, operation, orderID); err != nil {
			if errors.Is(err, idempotencyRepo.ErrAlreadyExists) {
				return nil
			}
			return err
		}
	}

	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return err
	}

	// Get order with row lock FIRST (for ban check context)
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// IDEMPOTENCY CHECK: If already completed, return success immediately
	// This makes the completion operation safe to retry without side effects
	if order.Status == entity.StatusCompleted && order.EscrowStatus == entity.EscrowStatusReleased {
		// Already in target state - idempotent operation
		return nil
	}

	// AUTHORIZATION: Only buyer can complete order (or system caller)
	if !auth.IsSystemCaller(callerID) && !s.ownership.IsBuyer(callerID, order.BuyerID) {
		return auth.ErrBuyerRequired
	}

	// MODERATION DOMAIN HARD CHECK (STEP 1 & 2):
	// - banned buyers CANNOT complete orders (release escrow)
	// - banned sellers: system handles completion flow
	// - check BOTH parties for safe completion
	if !auth.IsSystemCaller(callerID) {
		// Check buyer (actor) status
		if err := s.accountStatusChecker.EnsureActive(ctx, callerID); err != nil {
			return fmt.Errorf("buyer cannot complete order: %w", err)
		}

		// CRITICAL: Also check seller status
		// If seller is banned, system should handle completion, not buyer
		sellerStatus, err := s.accountStatusChecker.GetStatus(ctx, order.SellerID)
		if err == nil && sellerStatus == "banned" {
			return fmt.Errorf("cannot complete order: seller is banned, system must handle completion")
		}
	}

	// CRITICAL: Check HasDispute IMMEDIATELY after GetForUpdate (RACE CONDITION PREVENTION)
	// This check must happen BEFORE any other validation to prevent the race condition:
	// 1. Worker queries orders WHERE has_dispute = false
	// 2. Worker calls GetForUpdate
	// 3. [DISPUTE OPENED HERE - RACE WINDOW]
	// 4. order.ValidateComplete() checks HasDispute (TOO LATE)
	//
	// By checking HasDispute immediately after GetForUpdate, we close this race window.
	// This is the THIRD LAYER of defense (after query guard + entity guard).
	if order.HasDispute {
		return &entity.DisputeActiveError{OrderID: order.ID}
	}

	// REFUND DOMAIN GUARD: block release while a refund still has to be
	// respected. Two independent reasons (see RefundReleaseGuard): money owed to
	// the buyer has not settled at the gateway yet (releasing now would pay the
	// same money twice), or the refund decision is still open inside the order's
	// own refund window. Once that window closes the order lifecycle owns the
	// outcome, so an undecided refund no longer freezes the order forever.
	if s.refundReleaseGuard == nil {
		return ErrRefundReleaseGuardNotConfigured
	}
	refundBlocksRelease, err := s.refundReleaseGuard.HasRefundBlockingRelease(
		ctx, tx, order.ID, order.IsRefundWindowOpen(),
	)
	if err != nil {
		return fmt.Errorf("failed to check refund release guard: %w", err)
	}
	if refundBlocksRelease {
		return fmt.Errorf("cannot complete order: refund still blocking release (order_id=%s)", order.ID)
	}

	// SUPPORT DOMAIN GUARD: Block completion if active support ticket exists
	// This prevents order completion while support is actively investigating an issue
	activeTicketCount, err := s.supportRepo.CountActiveTicketsByOrderID(ctx, tx, orderID)
	if err != nil {
		return fmt.Errorf("failed to check for active support tickets: %w", err)
	}
	if activeTicketCount > 0 {
		return errors.New("cannot complete order: active support ticket exists")
	}

	// PAYMENT STATUS GUARD: Cannot complete order if payment is not confirmed
	// Only allow completion when payment.status is 'settlement' or 'capture'
	payment, err := s.paymentRepo.GetPaymentByReference(ctx, tx, paymentRepo.ReferenceTypeOrder, orderID)
	if err != nil {
		return fmt.Errorf("cannot complete order: payment not found")
	}
	if payment.Status != paymentRepo.PaymentStatusSettlement && payment.Status != paymentRepo.PaymentStatusCapture {
		return fmt.Errorf("cannot complete order: payment not confirmed (current status: %s)", payment.Status)
	}

	// ============================================================
	// STEP 1: VALIDATE ORDER STATE (before any money movement)
	// ============================================================
	// Check that order can be completed (status, escrow, disputes, etc.)
	// This is validation only - no state changes yet
	if err := order.ValidateComplete(); err != nil {
		return err
	}

	// ============================================================
	// STEP 2: GATEWAY-AWARE RELEASE
	// ============================================================
	// Locks escrow, validates state and amount, flips
	// escrow.status to 'released', and writes the finance ledger:
	//   GATEWAY_CLEARING -= gross, SELLER_PAYABLE += sellerNet, PLATFORM_REVENUE += commission.
	// Idempotency key (finance ledger): "order_release_<order_id>".
	// No user balance is touched — the canonical seller payable
	// surface is financial_accounts[SELLER_PAYABLE].
	releaseSummary, err := s.paymentService.ReleaseGatewayEscrowToSeller(ctx, tx, order)
	if err != nil {
		return err
	}

	// ============================================================
	// STEP 3: UPDATE ORDER STATE (reflect financial state)
	// ============================================================
	// NOW update Order.EscrowStatus to match escrow state
	// Order domain follows escrow domain (escrow-first operational state)
	order.Status = entity.StatusCompleted
	now := time.Now()
	order.CompletedAt = &now
	order.EscrowStatus = entity.EscrowStatusReleased
	order.UpdatedAt = now

	// ========================================================================
	// ORDER COMPLETION REWARD (LOYALTY POINTS)
	// ========================================================================
	// Grant loyalty points to buyer when order is completed.
	// This is the ONLY place where order completion rewards are granted.
	//
	// REWARD FORMULA: 1 point per Rp1.000 of final paid amount (floor division)
	// IDEMPOTENCY: Checks for existing order_reward transaction
	//
	// IMPORTANT:
	// - ONLY granted when order status = "completed"
	// - NOT granted for cancelled, refunded, or disputed orders
	// - Uses the canonical buyer-funded base PD + S (total_before_coins_amount),
	//   NOT forSale price and NOT the undiscounted P + S. The payment fee F and
	//   the commission C are not part of the reward base.
	//
	// Note: Error is logged but does not fail the completion.
	// The order completion is more important than the points reward.
	//
	// DEFENSIVE: coinsService is a non-canonical side-effect dependency. If it
	// was not wired (post-construction setter call missed), skip the loyalty
	// reward with a warn log rather than panic on a nil receiver. Constitutional
	// release path must NEVER be blocked by an optional loyalty side effect.
	if s.coinsService == nil {
		s.logger.Warn("coins_service_not_wired_skipping_loyalty_reward",
			zap.String("order_id", order.ID.String()),
			zap.String("buyer_id", order.BuyerID.String()),
		)
	} else if err := s.coinsService.EarnPointsForOrderCompletion(
		ctx,
		tx,
		order.BuyerID,
		order.ID,
		order.TotalBeforeCoinsAmount.Int64(), // CANONICAL buyer-funded base PD + S (excludes F and C)
	); err != nil {
		// ====================================================================
		// OPERATIONAL VISIBILITY: LOG FAILED EARN ATTEMPTS
		// ====================================================================
		// Log the failure for operational visibility.
		// Order completion succeeds regardless — the loyalty reward is secondary.
		// Reconciliation: query coins_transactions for missing order_reward entries.
		finalPaidAmount := order.TotalBeforeCoinsAmount.Int64()
		expectedPoints := finalPaidAmount / 1000
		s.logger.Error("coins_earn_failed_order_completion",
			zap.String("order_id", order.ID.String()),
			zap.String("buyer_id", order.BuyerID.String()),
			zap.Int64("attempted_amount", expectedPoints),
			zap.Int64("final_paid_amount", finalPaidAmount),
			zap.Error(err),
		)
	}

	// Persist order status changes
	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// Emit outbox event for order.completed
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		events.EventOrderCompleted,
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	// Emit money.released describing the financial split for downstream
	// consumers (analytics, seller earnings, reconcilers). Outbox layer dedups
	// on idempotency_key="money.released.<order_id>" via ON CONFLICT, so
	// re-emits on retried Complete() calls are safe no-ops.
	moneyReleasedPayload, mrErr := json.Marshal(map[string]interface{}{
		"order_id":       order.ID.String(),
		"seller_id":      releaseSummary.SellerID.String(),
		"gross":          releaseSummary.Gross,
		"commission":     releaseSummary.Commission,
		"seller_net":     releaseSummary.SellerNet,
		"newly_released": releaseSummary.NewlyReleased,
		"released_at":    now.UTC().Format(time.RFC3339Nano),
	})
	if mrErr != nil {
		return fmt.Errorf("marshal money.released payload: %w", mrErr)
	}
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		events.EventMoneyReleased,
		order.ID,
		moneyReleasedPayload,
	); err != nil {
		return err
	}

	return nil
}

// ExtendConfirmation extends the buyer confirmation period by 3 days.
// AUTHORIZATION: Only the buyer can extend confirmation, or system caller.
//
// BUSINESS RULES:
// - status must be 'shipped'
// - status must not be 'dispute_open'
// - confirmation_extension_used must be false
// - auto_release_at must be in the future (now < auto_release_at)
//
// ACTION:
// - auto_release_at += 3 days
// - confirmation_extension_used = true
// - confirmation_extended_at = now()
//
// IDEMPOTENCY: Uses idempotencyKey to ensure safe retries.
func (s *OrderCompletionService) ExtendConfirmation(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	orderID uuid.UUID,
	idempotencyKey string,
) error {
	// ============================================================
	// STEP 1: IDEMPOTENCY CHECK (inside transaction)
	// ============================================================
	operation := fmt.Sprintf("order.extend_confirmation.%s", orderID.String())
	if err := s.idempotencyRepo.TryInsert(ctx, tx, idempotencyKey, operation, orderID); err != nil {
		if errors.Is(err, idempotencyRepo.ErrAlreadyExists) {
			// Idempotent - operation already performed
			return nil
		}
		return err
	}

	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return err
	}

	// ACCOUNT STATUS: Check if buyer's account is active (system caller bypasses)
	if !auth.IsSystemCaller(callerID) {
		if err := s.accountStatusChecker.EnsureActive(ctx, callerID); err != nil {
			return err
		}
	}

	// ============================================================
	// STEP 2: LOCK ORDER (FOR UPDATE)
	// ============================================================
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// ============================================================
	// STEP 3: AUTHORIZATION - Only buyer can extend confirmation
	// ============================================================
	if !auth.IsSystemCaller(callerID) && !s.ownership.IsBuyer(callerID, order.BuyerID) {
		return auth.ErrBuyerRequired
	}

	// ============================================================
	// STEP 4: EXTEND CONFIRMATION PERIOD
	// ============================================================
	// This validates all business rules and extends auto_release_at by 3 days
	if err := order.ExtendConfirmationPeriod(); err != nil {
		return err
	}

	// ============================================================
	// STEP 5: UPDATE ORDER IN DATABASE
	// ============================================================
	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// ============================================================
	// STEP 6: EMIT OUTBOX EVENT
	// ============================================================
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"order.confirmation_extended",
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	return nil
}

// Cancel transitions an order from pending to cancelled.
//
// IDEMPOTENCY: Uses idempotencyKey to ensure safe retries.
// If a record with the same idempotency key exists, returns nil (operation already performed).
// The idempotency check is performed inside the transaction for atomicity.
//
// LOCK HIERARCHY (strict order - MUST NOT reverse):
// 1. Try insert idempotency record (UNIQUE constraint)
// 2. Lock Order (FOR UPDATE)
//
// STOCK RESTORATION:
//   - Restores forSale quantities when order is cancelled
//   - Handles multi-item orders
//   - All operations happen inside same transaction for atomicity
//
// AUTHORIZATION: Only buyer can cancel (or system caller for payment expiry)
func (s *OrderCompletionService) Cancel(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	idempotencyKey string,
	callerID uuid.UUID,
) error {
	// ============================================================
	// STEP 1: IDEMPOTENCY CHECK (inside transaction)
	// ============================================================
	operation := fmt.Sprintf("order.cancel.%s", orderID.String())
	if err := s.idempotencyRepo.TryInsert(ctx, tx, idempotencyKey, operation, orderID); err != nil {
		if errors.Is(err, idempotencyRepo.ErrAlreadyExists) {
			// Idempotent - operation already performed
			return nil
		}
		return err
	}

	// ============================================================
	// STEP 2: LOCK ORDER (FOR UPDATE)
	// ============================================================
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// ============================================================
	// STEP 3: AUTHORIZATION - Only buyer can cancel order (or system caller)
	// ============================================================
	if !auth.IsSystemCaller(callerID) && !s.ownership.IsBuyer(callerID, order.BuyerID) {
		return auth.ErrBuyerRequired
	}

	// ============================================================
	// STEP 3.5: CANCEL GUARD (EXPLICIT SAFETY CHECK)
	// ============================================================
	// Cannot cancel after order has been shipped
	// This provides an explicit guard on top of the state machine transition check
	if order.Status == entity.StatusShipped || order.Status == entity.StatusDelivered {
		return errors.New("cannot cancel order: order already shipped")
	}

	// ============================================================
	// STEP 4: VALIDATE ORDER STATUS TRANSITION
	// ============================================================
	// order.Cancel() enforces: only pending -> cancelled is allowed
	if err := order.Cancel(); err != nil {
		return err
	}

	// ============================================================
	// STEP 5: RESTORE LISTING STOCK
	// ============================================================
	// Restore stock for all forSales in this order
	// This must happen BEFORE updating order status to ensure atomicity
	if err := s.restoreForSaleStock(ctx, tx, order); err != nil {
		return err
	}

	// ============================================================
	// STEP 5.5: REACTIVATE SHIPPING QUOTE IF ELIGIBLE (fixed-price only)
	// ============================================================
	// Quote is marked USED at order creation (checkout), before payment.
	// If buyer cancels from pending, the quote was consumed for an order
	// that was never paid — reactivate so buyer can retry. Auction-sourced
	// orders are excluded (quote isolation: an old auction quote must never
	// become the current settlement authority for a relist).
	s.reactivateShippingQuoteIfEligible(ctx, tx, order)

	// ============================================================
	// STEP 6: UPDATE ORDER STATUS
	// ============================================================
	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// ============================================================
	// STEP 7: EMIT OUTBOX EVENT FOR ORDER.CANCELLED
	// ============================================================
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"order.cancelled",
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	return nil
}

// CancelOverdue allows buyer to cancel an order that is overdue for shipment.
//
// 🔥 PHASE 3: BUYER FORCE ACTION
//
// This method allows buyers to cancel orders when the seller has not shipped
// within the ReadyToShipBy + grace period deadline.
//
// AUTHORIZATION: Only buyer can cancel overdue orders (or system caller)
//
// VALIDATION:
// - Order must be in 'paid' status
// - Order must be overdue (ready_to_ship_by + grace_period < NOW())
// - Caller must be the buyer
//
// ATOMICITY:
// - Idempotent cancellation
// - Restores forSale stock
// - Refunds escrow to buyer (via paymentService)
// - Emits outbox event
func (s *OrderCompletionService) CancelOverdue(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	idempotencyKey string,
	callerID uuid.UUID,
) error {
	// ============================================================
	// STEP 1: IDEMPOTENCY CHECK (inside transaction)
	// ============================================================
	operation := fmt.Sprintf("order.cancel_overdue.%s", orderID.String())
	if err := s.idempotencyRepo.TryInsert(ctx, tx, idempotencyKey, operation, orderID); err != nil {
		if errors.Is(err, idempotencyRepo.ErrAlreadyExists) {
			// Idempotent - operation already performed
			return nil
		}
		return err
	}

	// ============================================================
	// STEP 2: LOCK ORDER (FOR UPDATE)
	// ============================================================
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// ============================================================
	// STEP 3: AUTHORIZATION - Only buyer can cancel overdue order
	// ============================================================
	if !auth.IsSystemCaller(callerID) && !s.ownership.IsBuyer(callerID, order.BuyerID) {
		return auth.ErrBuyerRequired
	}

	// ============================================================
	// STEP 4: VALIDATE ORDER IS OVERDUE
	// ============================================================
	// Check if order is eligible for overdue cancellation
	if !order.IsBuyerEligibleForCancel() {
		return &entity.ErrBuyerNotEligibleForCancel{
			OrderID:        order.ID,
			ReadyToShipBy:  *order.ReadyToShipBy,
			GracePeriodEnd: *order.GetGracePeriodEnd(),
		}
	}

	// ============================================================
	// STEP 5: VALIDATE ORDER STATE (before any money movement)
	// ============================================================
	// Check that order can be cancelled due to timeout
	// This is validation only - no state changes yet
	if err := order.ValidateCancelTimeout(); err != nil {
		return err
	}

	// ============================================================
	// STEP 6: CANONICAL REFUND (gateway dispatch FIRST, escrow flip locally)
	// ============================================================
	// Order was paid (otherwise it would not be eligible for buyer overdue
	// cancel), so escrow exists in holding. Dispatch the canonical gateway
	// refund BEFORE the local escrow flip so we never advance local state
	// without the gateway-side reversal in flight. Ledger reversal happens
	// at webhook ack (FinanceService.RecordRefundReversal).
	// CANONICAL FULL REFUND: buyer-funded base PD+S = total_before_coins_amount.
	// P+S is WRONG when D>0. No fallback — 0 is invalid/corrupt, fail closed.
	refundAmount := order.TotalBeforeCoinsAmount.Int64()
	if refundAmount <= 0 {
		return fmt.Errorf("refund failed: total_before_coins_amount invalid (order_id=%s total_before_coins=%d)", order.ID.String(), refundAmount)
	}
	if err := s.paymentService.InitiateGatewayRefundForOrder(
		ctx, tx, order, uuid.Nil, // automatic system refund — reviewed_by persists as NULL
		refundAmount, "other",
		fmt.Sprintf("system_refund_buyer_overdue_%s", order.ID.String()),
	); err != nil {
		return fmt.Errorf("gateway refund initiation failed: %w", err)
	}

	// Flip local escrow status to refunded. The canonical money movement is
	// the ledger reversal booked at webhook ack; this flip records intent.
	if err := s.paymentService.RefundToBuyer(ctx, tx, order); err != nil {
		return fmt.Errorf("failed to refund escrow: %w", err)
	}

	escrowRow, err := s.escrowService.GetEscrowForOrder(ctx, tx, order.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch escrow after refund: %w", err)
	}
	derivedEscrowStatus := mapEscrowToOrderEscrow(escrowRow.Status.String())

	// ============================================================
	// STEP 7: UPDATE ORDER STATE (reflect financial state)
	// ============================================================
	// NOW update Order.EscrowStatus to match escrow state
	// Order domain follows escrow domain (escrow-first operational state)
	order.Status = entity.StatusCancelledTimeout
	// CRITICAL: Set Order.EscrowStatus from escrow state (not independent)
	order.EscrowStatus = derivedEscrowStatus
	order.UpdatedAt = time.Now()

	// Coins are NOT refunded from the order domain here. A paid order cancelled
	// for shipment timeout is refunded through the canonical gateway refund path
	// above (InitiateGatewayRefundForOrder + RefundToBuyer); when the gateway ack
	// lands, the refund pipeline emits coins.refund_required with the computed
	// coin_delta and the coins domain restores K from coins_transactions.

	// ============================================================
	// STEP 9: RESTORE LISTING STOCK
	// ============================================================
	// Restore stock for all forSales in this order
	// This must happen BEFORE updating order status to ensure atomicity
	if err := s.restoreForSaleStock(ctx, tx, order); err != nil {
		return err
	}

	// ============================================================
	// STEP 9.5: REACTIVATE SHIPPING QUOTE IF ELIGIBLE (fixed-price only)
	// ============================================================
	// Seller did not ship, buyer force-cancelled with full refund.
	// Quote was consumed at checkout — reactivate so buyer can retry
	// (auction orders excluded — quote isolation).
	s.reactivateShippingQuoteIfEligible(ctx, tx, order)

	// ============================================================
	// STEP 10: PERSIST ORDER STATE TO DATABASE
	// ============================================================
	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// ============================================================
	// STEP 11: EMIT OUTBOX EVENT FOR ORDER.CANCELLED_TIMEOUT
	// ============================================================
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"order.cancelled_timeout",
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	return nil
}

// Expire transitions an order from pending to expired.
// Called by PaymentExpiryWorker when payment expires.
//
// LOCK HIERARCHY (strict order - MUST NOT reverse):
// 1. Lock Order (FOR UPDATE)
//
// STOCK RESTORATION:
//   - Restores forSale quantities when order expires
//   - Handles multi-item orders
//   - All operations happen inside same transaction for atomicity
//
// COINS:
//   - An unpaid order has no coin spend transaction (coins are redeemed at
//     payment settlement via ConsumeAndSpendForOrder), so expiry has nothing
//     to restore and this path emits no coins event.
//
// IMPORTANT: This does NOT trigger any ledger operations.
// Expired orders have no funds held, so no escrow movement is needed.
func (s *OrderCompletionService) Expire(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	// ============================================================
	// STEP 1: LOCK ORDER (FOR UPDATE)
	// ============================================================
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// ============================================================
	// STEP 2: VALIDATE ORDER STATUS TRANSITION
	// ============================================================
	// order.MarkExpired() enforces: only pending -> expired is allowed
	if err := order.MarkExpired(); err != nil {
		return err
	}

	// ============================================================
	// STEP 3: RESTORE LISTING STOCK
	// ============================================================
	// Restore stock for all forSales in this order
	// This must happen BEFORE updating order status to ensure atomicity
	if err := s.restoreForSaleStock(ctx, tx, order); err != nil {
		return err
	}

	// ============================================================
	// STEP 3.5: RELEASE ESCROW IF ANY WAS HELD (PHASE 1: BLOCKING REFUND)
	// ============================================================
	// 🔥 CRITICAL: ESCROW REFUND MUST BE BLOCKING
	// - Orders now hold escrow on creation (WALLET PHASE 1)
	// - When orders expire, we MUST refund the escrow to the buyer
	// - This calls RefundService which reverses:
	//   Escrow refund via gateway refund pipeline
	//
	// ❌ OLD BEHAVIOR (NON-BLOCKING):
	//   - Log error and continue with expiry
	//   - Result: order expires + escrow stuck (MONEY LEAK!)
	//
	// ✔️ NEW BEHAVIOR (BLOCKING):
	//   - If refund fails → entire expiry transaction FAILS
	//   - No state: expired + escrow held
	//   - Transaction rollback ensures atomicity
	//
	// SAFETY: RefundGatewayEscrow is idempotent
	// - Safe to call multiple times (only succeeds once)
	// - If no escrow exists, returns success (no-op)
	//
	// 🔥 ZERO LOOPHOLE: Expiry cannot complete without refund success
	//
	// CANONICAL REFUND CONVERGENCE: distinguish unpaid expiry from paid
	// expiry-with-escrow. Unpaid orders never funded the gateway clearing
	// account, so there is no gateway refund and no escrow to flip. Paid
	// orders (escrow exists in holding) must dispatch the canonical gateway
	// refund before the local escrow flip, mirroring the buyer-overdue path.
	escrowForExpiry, escrowErr := s.escrowService.GetEscrowForOrder(ctx, tx, order.ID)
	if escrowErr != nil {
		return fmt.Errorf("CRITICAL: failed to load escrow for expiry: order_id=%s, error=%w", orderID, escrowErr)
	}
	if escrowForExpiry != nil {
		// CANONICAL FULL REFUND: PD+S = total_before_coins_amount. No P+S fallback — 0 is invalid.
		refundAmount := order.TotalBeforeCoinsAmount.Int64()
		if refundAmount <= 0 {
			return fmt.Errorf("refund failed: total_before_coins_amount invalid (order_id=%s total_before_coins=%d)", order.ID.String(), refundAmount)
		}
		if err := s.paymentService.InitiateGatewayRefundForOrder(
			ctx, tx, order, uuid.Nil, // automatic system refund — reviewed_by persists as NULL
			refundAmount, "other",
			fmt.Sprintf("system_refund_payment_expired_%s", order.ID.String()),
		); err != nil {
			return fmt.Errorf("CRITICAL: gateway refund initiation failed during expiry: order_id=%s, error=%w", orderID, err)
		}
		if err := s.paymentService.RefundToBuyer(ctx, tx, order); err != nil {
			return fmt.Errorf("CRITICAL: escrow refund failed, abort expiry: order_id=%s, error=%w", orderID, err)
		}
	} else {
		s.logger.Info("expiry_no_escrow_canonical_skip",
			zap.String("order_id", orderID.String()),
			zap.String("reason", "payment never settled; no gateway refund needed"),
		)
	}

	// An order that expires without payment never reached payment settlement, so
	// no coin spend transaction was ever created for it (ConsumeAndSpendForOrder
	// runs in the payment finalization path) and there is nothing to restore.

	// ============================================================
	// STEP 4.5: REACTIVATE SHIPPING QUOTE IF ELIGIBLE (fixed-price only)
	// ============================================================
	// HARD FIX - SHIPPING QUOTE REUSE
	// When order expires, reactivate the shipping quote if it was used.
	// This allows the buyer to reuse the quote for a new order attempt.
	// Auction orders excluded — quote isolation (an expired auction order's
	// quote must not become the current settlement authority for a relist).
	s.reactivateShippingQuoteIfEligible(ctx, tx, order)

	// ============================================================
	// STEP 5: UPDATE ORDER STATUS
	// ============================================================
	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// ============================================================
	// STEP 6: EMIT OUTBOX EVENT FOR ORDER.EXPIRED
	// ============================================================
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"order.expired",
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	return nil
}

// MarkDisputeOpen marks the order as dispute_open and freezes escrow.
// This is called when a dispute is opened for the order.
// Transitions both order.status to dispute_open and escrow_status to frozen.
func (s *OrderCompletionService) MarkDisputeOpen(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// Use entity method for state transition
	if err := order.MarkDisputeOpen(); err != nil {
		return err
	}

	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// Emit outbox event for notification
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"order.dispute_open",
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	return nil
}

// RefundOrder processes a full refund to buyer.
//
// CANONICAL: the gateway refund is dispatched FIRST (via the refund domain's
// RefundService), then escrow flips locally. The ledger reversal is booked at
// gateway webhook ack (FinanceService.RecordRefundReversal), never here.
//
// Refund amount is the canonical buyer-funded base
// (total_before_coins_amount = (P − D) + S).
// Commission C is seller/platform-side and NEVER part of buyer refund cash.
//
// GOVERNANCE BOUNDARY:
// - ONLY allows escrow_status = "holding"
// - EXPLICITLY REJECTS orders with active disputes (status = dispute_open)
// - For dispute resolution refunds, use RefundFromDispute instead
//
// CRITICAL: This method is idempotent via the gateway refund idempotency key.
// Even if called multiple times, the refund will only execute once.
//
// STATE UPDATES:
// - escrow_status = derived from the escrow row after the refund
// - status = "refunded"
//
// The refund amount is never stored on the order; the refund row and the
// ledger own that truth.
func (s *OrderCompletionService) RefundOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// GOVERNANCE GUARD 1: Cannot refund directly when dispute exists
	// Must use DisputeService.ResolveApproved() for dispute-related refunds
	if order.Status == entity.StatusDisputeOpen {
		return errors.New("cannot refund directly: active dispute exists, use DisputeService.ResolveApproved()")
	}

	// GOVERNANCE GUARD 2: Only allow refund from "holding" state
	if order.EscrowStatus != entity.EscrowStatusHolding {
		return &entity.InvalidEscrowStatusError{
			CurrentStatus:  order.EscrowStatus,
			RequiredStatus: entity.EscrowStatusHolding,
		}
	}

	// CANONICAL FULL REFUND: buyer-funded base PD+S = total_before_coins_amount.
	// P+S is WRONG when D>0. No P+S fallback — TotalBeforeCoins must be >0;
	// legacy 0 would be corrupt state, fail closed per existing canonical error handling.
	refundAmount := order.TotalBeforeCoinsAmount.Int64()
	if refundAmount <= 0 {
		return fmt.Errorf("refund failed: total_before_coins_amount invalid (order_id=%s, total_before_coins=%d subtotal=%d shipping=%d)", order.ID.String(), refundAmount, order.Subtotal.Int64(), order.ShippingTotal.Int64())
	}
	if err := s.paymentService.InitiateGatewayRefundForOrder(
		ctx, tx, order, uuid.Nil, // automatic system refund — reviewed_by persists as NULL
		refundAmount, "other",
		fmt.Sprintf("system_refund_manual_%s", order.ID.String()),
	); err != nil {
		return fmt.Errorf("gateway refund initiation failed: %w", err)
	}
	if err := s.paymentService.RefundToBuyer(ctx, tx, order); err != nil {
		return err
	}

	escrowRow, err := s.escrowService.GetEscrowForOrder(ctx, tx, order.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch escrow after refund: %w", err)
	}
	derivedEscrowStatus := mapEscrowToOrderEscrow(escrowRow.Status.String())

	// ============================================================
	// RATING INVALIDATION - EVENTUAL CONSISTENCY
	// ============================================================
	// IMPORTANT: Rating invalidation is SECONDARY to financial refund.
	// - Refund MUST complete regardless of rating invalidation success
	// - Rating data is eventually consistent (can be fixed later)
	// - Financial data is primary (cannot be easily reversed)
	//
	// If rating invalidation fails:
	// 1. Log CRITICAL error for monitoring
	// 2. Continue with refund (money flow is primary)
	// 3. Background job will retry rating invalidation
	//
	// RATIONALE: Blocking refund for rating issues would create:
	// - Financial customer harm (money stuck)
	// - Support burden (manual intervention required)
	// - System unreliability (secondary data blocking primary flow)
	if err := s.invalidateRatingForOrder(ctx, tx, order.ID); err != nil {
		// Log CRITICAL error but don't fail the refund
		// This will be retried by a background cleanup job
		s.logger.Error("CRITICAL: rating invalidation failed - will retry via background job",
			zap.String("order_id", order.ID.String()),
			zap.String("seller_id", order.SellerID.String()),
			zap.Error(err),
			zap.String("impact", "valid rating may survive refund temporarily - background job will fix"),
		)
	}

	// ============================================================
	// REACTIVATE SHIPPING QUOTE IF ELIGIBLE (fixed-price only)
	// ============================================================
	// When order is refunded, reactivate the shipping quote if it was used.
	// Auction orders excluded — quote isolation.
	s.reactivateShippingQuoteIfEligible(ctx, tx, order)

	// CRITICAL: Set Order.EscrowStatus from escrow state (not independent)
	order.EscrowStatus = derivedEscrowStatus
	order.Status = entity.StatusRefunded
	// Note: Refund amount is tracked in Ledger, not in Order
	order.UpdatedAt = time.Now()

	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// Emit outbox event for order.refunded
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"order.refunded",
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	return nil
}

// RefundFromDispute processes a full refund to buyer from dispute resolution.
// PUBLIC API: Called by DisputeService for dispute resolution.
//
// Refund amount is the canonical buyer-funded base
// (total_before_coins_amount = (P − D) + S).
// Commission C is seller/platform-side and NEVER part of buyer refund cash.
//
// GOVERNANCE: This is the ONLY path that can refund an order with status = dispute_open.
//
// CANONICAL ADMIN DECISION: the admin's final buyer-wins decision is recorded
// on the order's own refund process row (RefundService.AdminResolveRefundDecision)
// and the gateway refund is dispatched on that same row. The ledger reversal is
// booked at gateway webhook ack, never here.
//
// STATE UPDATES:
// - escrow_status = derived from the escrow row after the refund
// - status = "refunded"
func (s *OrderCompletionService) RefundFromDispute(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	adminID uuid.UUID,
) error {
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// GOVERNANCE GUARD: Only allow from dispute state (dispute path exclusive)
	// This prevents bypassing dispute resolution for regular refunds
	// CRITICAL: Use Order.HasDispute instead of checking removed "frozen" state
	if !order.HasDispute {
		return &entity.InvalidEscrowStatusError{
			CurrentStatus:  order.EscrowStatus,
			RequiredStatus: entity.EscrowStatusHolding, // Disputes can only be resolved from holding state
		}
	}

	// DISPUTE STATE GUARD: Only allow from "dispute_open" status
	if order.Status != entity.StatusDisputeOpen {
		return errors.New("invalid state for dispute resolution")
	}

	// DISPUTE → ESCROW INTEGRATION: Log that dispute resolution is triggering escrow refund
	s.logger.Info("escrow_dispute_refund_triggered",
		zap.String("order_id", order.ID.String()),
		zap.String("buyer_id", order.BuyerID.String()),
		zap.String("seller_id", order.SellerID.String()),
		zap.String("escrow_status", string(order.EscrowStatus)),
		zap.String("trigger", "dispute_resolution"),
	)

	// CANONICAL ADMIN DECISION + SETTLEMENT: the admin's final buyer-wins
	// decision is recorded on the ORDER'S OWN refund process row (the escalated
	// one), and the gateway refund is dispatched on that same row. Ledger
	// reversal happens at webhook ack, after the local escrow flip.
	//
	// A direct dispute that never went through a refund request has no process
	// row: there is no decision to record, so a platform refund record
	// (system_refunded) is created instead.
	// CANONICAL FULL REFUND: PD+S = total_before_coins_amount, not P+S.
	refundAmount := order.TotalBeforeCoinsAmount.Int64()
	if refundAmount <= 0 {
		return fmt.Errorf("refund failed: total_before_coins_amount invalid (order_id=%s, total_before_coins=%d)", orderID.String(), refundAmount)
	}
	decisionRecorded, err := s.recordAdminRefundDecision(ctx, tx, order, adminID, true, refundAmount)
	if err != nil {
		s.logger.Error("dispute_refund_decision_failed",
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("admin refund decision failed: %w", err)
	}
	if !decisionRecorded {
		if err := s.paymentService.InitiateGatewayRefundForOrder(
			ctx, tx, order, adminID,
			refundAmount, "other",
			fmt.Sprintf("system_refund_dispute_%s", order.ID.String()),
		); err != nil {
			s.logger.Error("dispute_refund_gateway_dispatch_failed",
				zap.String("order_id", orderID.String()),
				zap.Error(err),
			)
			return fmt.Errorf("gateway refund initiation failed: %w", err)
		}
	}

	escrowRow, _, err := s.escrowService.RefundGatewayEscrow(ctx, tx, orderID)
	if err != nil {
		s.logger.Error("escrow_refund_failed",
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to refund escrow via escrow service: %w", err)
	}

	// Derive Order.EscrowStatus from escrow state (CRITICAL: no independent state)
	derivedEscrowStatus := mapEscrowToOrderEscrow(escrowRow.Status.String())

	// ============================================================
	// RATING INVALIDATION - EVENTUAL CONSISTENCY
	// ============================================================
	// IMPORTANT: Rating invalidation is SECONDARY to financial refund.
	// - Refund MUST complete regardless of rating invalidation success
	// - Rating data is eventually consistent (can be fixed later)
	// - Financial data is primary (cannot be easily reversed)
	if err := s.invalidateRatingForOrder(ctx, tx, order.ID); err != nil {
		// Log CRITICAL error but don't fail the refund
		// This will be retried by a background cleanup job
		s.logger.Error("CRITICAL: rating invalidation failed - will retry via background job",
			zap.String("order_id", order.ID.String()),
			zap.String("seller_id", order.SellerID.String()),
			zap.String("context", "dispute refund"),
			zap.Error(err),
			zap.String("impact", "valid rating may survive refund temporarily - background job will fix"),
		)
	}

	// ============================================================
	// REACTIVATE SHIPPING QUOTE IF ELIGIBLE (fixed-price only)
	// ============================================================
	// When order is refunded via dispute, reactivate the shipping quote if it
	// was used. Auction orders excluded — quote isolation.
	s.reactivateShippingQuoteIfEligible(ctx, tx, order)

	// CRITICAL: Set Order.EscrowStatus from escrow state (not independent)
	order.EscrowStatus = derivedEscrowStatus
	order.Status = entity.StatusRefunded
	// Note: Refund amount is tracked in Ledger, not in Order
	order.UpdatedAt = time.Now()

	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// Emit outbox event for order.refunded
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"order.refunded",
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	return nil
}

// recordAdminRefundDecision delegates the admin's FINAL refund decision to the
// refund domain (which owns refund state). Returns recorded=false when the order
// has no refund process row — the caller then creates the platform refund
// record. Fails closed when the authority is not wired.
func (s *OrderCompletionService) recordAdminRefundDecision(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
	adminID uuid.UUID,
	buyerWins bool,
	amount int64,
) (bool, error) {
	if s.refundDecisionAuth == nil {
		return false, errors.New("refund decision authority not configured; refusing to resolve dispute without recording the final refund decision")
	}
	return s.refundDecisionAuth.AdminResolveRefundDecision(ctx, tx, order.ID, adminID, buyerWins, amount, nil)
}

// ReleaseFromDispute processes an escrow release to seller from dispute resolution.
// PUBLIC API: Called by DisputeService for dispute resolution in favor of seller.
//
// Gateway-aware release: mirrors the canonical Complete() release semantics
// (paymentService.ReleaseGatewayEscrowToSeller). Buyer balances are NOT
// touched — the seller's withdrawable surface is financial_accounts[SELLER_PAYABLE].
//
// FLOW (single tx, caller-owned):
//  1. Lock order and enforce dispute guards (HasDispute, status == dispute_open).
//  2. Call paymentService.ReleaseGatewayEscrowToSeller (escrow flip +
//     finance ledger via idempotency_key="order_release_<order_id>").
//  3. Update order: status=completed, escrow_status=released, completed_at=now.
//  4. Emit order.completed.
//  5. Emit money.released describing the financial split.
//
// Differences from Complete():
//   - No loyalty points granted (dispute resolution is not a reward path).
//   - No request fulfillment side effects.
//   - Dispute guards (HasDispute + dispute_open) instead of the Complete guards.
func (s *OrderCompletionService) ReleaseFromDispute(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	adminID uuid.UUID,
) error {
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// GOVERNANCE GUARD: Only allow from dispute context (dispute path exclusive)
	if !order.HasDispute {
		return errors.New("order must have an open dispute to release from dispute")
	}

	// DISPUTE STATE GUARD: Only allow from "dispute_open" status
	if order.Status != entity.StatusDisputeOpen {
		return errors.New("invalid state for dispute resolution")
	}

	// CANONICAL FINALITY GUARD: a refund decision that owes the buyer money is
	// FINAL (seller ACCEPT / admin buyer-wins / platform-initiated refund), so
	// the order's escrow belongs to the buyer. Releasing it to the seller as
	// well would move the same money twice and contradict a decision the buyer
	// is entitled to rely on. Fail closed and let the refund settlement land.
	if s.refundDecisionAuth == nil {
		return errors.New("refund decision authority not configured; refusing to release escrow without checking the refund decision")
	}
	owedToBuyer, err := s.refundDecisionAuth.HasFinalRefundDecisionOwedToBuyer(ctx, tx, order.ID)
	if err != nil {
		return fmt.Errorf("failed to check final refund decision: %w", err)
	}
	if owedToBuyer {
		return fmt.Errorf("%w (order_id=%s)", ErrFinalRefundDecisionOwesBuyer, order.ID)
	}

	// CANONICAL ADMIN DECISION: record the final seller-wins decision on the
	// order's refund process row (if any) BEFORE releasing the money. The refund
	// process must not be left looking unresolved once the admin has decided.
	if _, err := s.recordAdminRefundDecision(ctx, tx, order, adminID, false, 0); err != nil {
		return err
	}

	// DISPUTE → ESCROW INTEGRATION: Log that dispute resolution is triggering escrow release
	s.logger.Info("escrow_dispute_release_triggered",
		zap.String("order_id", order.ID.String()),
		zap.String("buyer_id", order.BuyerID.String()),
		zap.String("seller_id", order.SellerID.String()),
		zap.String("escrow_status", string(order.EscrowStatus)),
		zap.String("trigger", "dispute_resolution"),
	)

	// Gateway-aware release: locks escrow, validates status and amount, flips
	// escrow.status to 'released', and writes the finance ledger:
	//   GATEWAY_CLEARING -= gross, SELLER_PAYABLE += sellerNet, PLATFORM_REVENUE += commission.
	// Idempotent via finance idempotency_key="order_release_<order_id>".
	releaseSummary, err := s.paymentService.ReleaseGatewayEscrowToSeller(ctx, tx, order)
	if err != nil {
		return err
	}

	// Update order status (Order.EscrowStatus mirrors Escrow.Status).
	now := time.Now()
	order.Status = entity.StatusCompleted
	order.EscrowStatus = entity.EscrowStatusReleased
	order.CompletedAt = &now
	order.UpdatedAt = now

	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// Emit outbox event for order.completed
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		events.EventOrderCompleted,
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	// Emit money.released describing the financial split. Outbox dedup on
	// idempotency_key="money.released.<order_id>" makes re-emits a no-op.
	moneyReleasedPayload, mrErr := json.Marshal(map[string]interface{}{
		"order_id":       order.ID.String(),
		"seller_id":      releaseSummary.SellerID.String(),
		"gross":          releaseSummary.Gross,
		"commission":     releaseSummary.Commission,
		"seller_net":     releaseSummary.SellerNet,
		"newly_released": releaseSummary.NewlyReleased,
		"released_at":    now.UTC().Format(time.RFC3339Nano),
	})
	if mrErr != nil {
		return fmt.Errorf("marshal money.released payload: %w", mrErr)
	}
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		events.EventMoneyReleased,
		order.ID,
		moneyReleasedPayload,
	); err != nil {
		return err
	}

	return nil
}

// PartialRefundFromDispute resolves a dispute with partial split:
// - Buyer gets refund for discounted product value PD (total_before_coins - S)
// - Seller gets release for shipping fee (shipping_total)
//
// STRICT RULES:
// - ADMIN ONLY (no user-triggered)
// - MUST be from dispute_open status with escrow in holding
// - Refund amount MUST equal PD (discounted product value, not undiscounted subtotal)
// - Shipping fee is released to seller (remainder)
//
// CANONICAL: escrow = total_before_coins_amount = PD+S. P+S is NOT canonical when D>0.
//
// CANONICAL FINANCIAL FLOW:
// 1. Validate PD + S == escrow_amount where PD = total_before_coins - S, escrow = total_before_coins
// 2. Record the admin's buyer-wins decision on the order's refund process row
//    with the item-price amount (RefundService.AdminResolveRefundDecision),
//    which also dispatches the gateway refund on that same row.
// 3. The escrow primitive (PartialRefundGatewayEscrow) is invoked from
//    RefundService.HandleGatewayRefundAck after the canonical ledger reversal
//    commits — escrow stays holding until the gateway ack arrives.
// 4. Update order status to partially_refunded
//
// CRITICAL: This method is idempotent and atomic.
func (s *OrderCompletionService) PartialRefundFromDispute(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	adminID uuid.UUID,
) error {
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// GOVERNANCE GUARD: Only allow from dispute context (dispute path exclusive)
	if !order.HasDispute {
		return errors.New("order must have an open dispute for partial refund from dispute")
	}

	// DISPUTE STATE GUARD: Only allow from "dispute_open" status
	if order.Status != entity.StatusDisputeOpen {
		return errors.New("invalid state for partial dispute resolution: must be dispute_open")
	}

	// STRICT MODE ENTRY POINT GUARD: Check dispute status is "under_review"
	// This prevents gray area - only allows partial resolution when dispute is actively being reviewed
	dispute, err := s.disputeRepo.GetByOrderID(ctx, tx, orderID)
	if err != nil {
		return fmt.Errorf("failed to fetch dispute for entry point guard: %w", err)
	}
	if dispute == nil {
		return errors.New("entry point guard violation: no dispute found for order")
	}
	if dispute.Status != disputeEntity.DisputeStatusUnderReview {
		return fmt.Errorf("entry point guard violation: dispute status must be 'under_review', got '%s'", dispute.Status)
	}

	// DATA VALIDATION: Ensure order has canonical buyer-funded escrow base.
	// CANONICAL: escrow = total_before_coins_amount = PD + S = (P-D)+S.
	// itemPrice for partial refund is the discounted product value PD,
	// derived from the persisted canonical base (total_before_coins - S),
	// NOT the undiscounted subtotal P. Shipping commission C is never buyer cash.
	escrowAmount := order.TotalBeforeCoinsAmount
	if !order.HasCanonicalMoneyBase() {
		return fmt.Errorf("partial dispute validation failed: canonical money base invalid (total_before_coins=%d shipping=%d) — no P or P+S fallback", escrowAmount.Int64(), order.ShippingTotal.Int64())
	}
	// PD = (P-D) = total_before_coins - S — the ONE canonical PD derivation.
	itemPrice := order.DiscountedProductAmount()
	shippingFee := order.ShippingTotal
	commissionFee := order.CommissionAmount
	if itemPrice.IsNegative() {
		return fmt.Errorf("partial dispute validation failed: discounted product value negative (total_before_coins=%d shipping=%d)", escrowAmount.Int64(), shippingFee.Int64())
	}
	// Invariant: PD + S == total_before_coins (by construction) — sanity check that derived PD reconstructs canonical base.
	if !itemPrice.Add(shippingFee).Equal(escrowAmount) {
		return fmt.Errorf("partial dispute validation failed: item_price(PD) + shipping_fee != total_before_coins_amount (item(PD)=%d shipping=%d total_before_coins(PD+S)=%d)",
			itemPrice.Int64(), shippingFee.Int64(), escrowAmount.Int64())
	}

	// LOG: Dispute resolution triggering escrow partial refund
	s.logger.Info("escrow_dispute_partial_refund_triggered",
		zap.String("order_id", order.ID.String()),
		zap.String("buyer_id", order.BuyerID.String()),
		zap.String("seller_id", order.SellerID.String()),
		zap.Int64("item_price", itemPrice.Int64()),
		zap.Int64("shipping_fee", shippingFee.Int64()),
		zap.Int64("commission_fee", commissionFee.Int64()),
		zap.Int64("total_before_coins_amount", escrowAmount.Int64()),
		zap.String("trigger", "dispute_partial_split_resolution"),
	)

	// CANONICAL PARTIAL REFUND: the admin's final buyer-wins decision is recorded
	// on the order's OWN refund process row and the gateway refund for the BUYER
	// portion (item price) ONLY is dispatched on that same row. Local escrow.status
	// DOES NOT flip here — the escrow primitive (PartialRefundGatewayEscrow) is
	// invoked from RefundService.HandleGatewayRefundAck after the canonical ledger
	// reversal commits. Escrow stays HOLDING until the gateway ack arrives.
	//
	// A direct dispute with no refund process has nothing to record, so a platform
	// refund record (system_refunded) is created instead.
	partialRecorded, err := s.recordAdminRefundDecision(ctx, tx, order, adminID, true, itemPrice.Int64())
	if err != nil {
		return fmt.Errorf("partial admin refund decision failed: %w", err)
	}
	if !partialRecorded {
		if err := s.paymentService.InitiateGatewayRefundForOrder(
			ctx, tx, order, adminID,
			itemPrice.Int64(), "other",
			fmt.Sprintf("system_refund_partial_dispute_%s", order.ID.String()),
		); err != nil {
			return fmt.Errorf("partial gateway refund initiation failed: %w", err)
		}
	}

	// ============================================================
	// EMIT PARTIAL RELEASE EVENT
	// ============================================================
	// The proportional coin restoration for the buyer-owed portion is emitted by
	// the gateway refund ack pipeline (coins.refund_required with coin_delta); the
	// order domain does not compute it.
	// Emit money.partial_release event (audit only - no coin refund)
	partialReleasePayload := map[string]interface{}{
		"order_id":     order.ID.String(),
		"seller_id":    order.SellerID.String(),
		"item_price":   itemPrice.Int64(),
		"shipping_fee": shippingFee.Int64(),
		"total_escrow": escrowAmount.Int64(),
	}
	partialReleaseBytes, _ := json.Marshal(partialReleasePayload)
	_ = s.outboxRepo.InsertEvent(ctx, tx, "money.partial_release", order.ID, partialReleaseBytes)
	// ============================================================
	// RATING INVALIDATION - EVENTUAL CONSISTENCY
	// ============================================================
	// IMPORTANT: Rating invalidation is SECONDARY to financial refund.
	// - Even partial refunds invalidate entire rating (cannot have "partial" rating)
	// - Refund MUST complete regardless of rating invalidation success
	// - Rating data is eventually consistent (can be fixed later)
	if err := s.invalidateRatingForOrder(ctx, tx, order.ID); err != nil {
		// Log CRITICAL error but don't fail the partial refund
		// This will be retried by a background cleanup job
		s.logger.Error("CRITICAL: rating invalidation failed - will retry via background job",
			zap.String("order_id", order.ID.String()),
			zap.String("seller_id", order.SellerID.String()),
			zap.String("context", "partial dispute refund"),
			zap.Error(err),
			zap.String("impact", "valid rating may survive partial refund temporarily - background job will fix"),
		)
	}

	// Update order state using entity method
	if err := order.MarkPartiallyRefunded(); err != nil {
		return err
	}

	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
	}

	// Emit outbox event for order.partially_refunded
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"order.partially_refunded",
		order.ID,
		buildOrderPayload(order),
	); err != nil {
		return err
	}

	s.logger.Info("escrow_dispute_partial_refund_success",
		zap.String("order_id", order.ID.String()),
		zap.String("buyer_id", order.BuyerID.String()),
		zap.String("seller_id", order.SellerID.String()),
		zap.Int64("buyer_refund", itemPrice.Int64()),
		zap.Int64("seller_payout", shippingFee.Int64()),
		zap.String("resolution_type", "partial_split"),
	)

	return nil
}

// SyncRefundSettlementFromGatewayAck syncs order terminal refund status after
// gateway ack has been accepted and reversal booked.
//
// Authority: order domain owns order.status and order.escrow_status mutation.
// Behavior parity with legacy finance-side SQL:
// - fullyRefunded=true  -> status=refunded, escrow_status=refunded
// - fullyRefunded=false -> status=partially_refunded, escrow_status=released
// - idempotent on already-target state
// - no outbox emission (matches previous behavior)
func (s *OrderCompletionService) SyncRefundSettlementFromGatewayAck(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	fullyRefunded bool,
	occurredAt time.Time,
) error {
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}

	targetStatus := entity.StatusPartiallyRefunded
	targetEscrow := entity.EscrowStatusReleased
	if fullyRefunded {
		targetStatus = entity.StatusRefunded
		targetEscrow = entity.EscrowStatusRefunded
	}

	if order.Status == targetStatus && order.EscrowStatus == targetEscrow {
		return nil
	}

	if targetStatus == entity.StatusPartiallyRefunded {
		if err := order.MarkPartiallyRefunded(); err != nil {
			return err
		}
	} else {
		order.Status = entity.StatusRefunded
		order.UpdatedAt = occurredAt
	}
	order.EscrowStatus = targetEscrow
	order.UpdatedAt = occurredAt

	return s.repo.UpdateStatusTx(ctx, tx, order)
}

// restoreForSaleStock restores stock/inventory binding for an order that is
// being cancelled or expired.
//
// PASS_20B: branches by order.SourceType. Auction orders have no
// for_sales row at all (CreateFromAuction builds a purely in-memory
// ForSale surface for validation, never persists one) — routing them
// through the fixed-price restore path below made forSaleRepo.GetForUpdate
// look up a nonexistent for_sales.id and fail the whole
// Cancel/CancelOverdue/Expire transaction.
//
// Stage 5 (identity convergence): the selling surface is resolved from
// orders.source_type + orders.source_id — never from order_items.product_id,
// which is the Product identity only. order_items.product_id MUST be
// products.id for every order path (FPS, negotiation, auction).
func (s *OrderCompletionService) restoreForSaleStock(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
) error {
	if order.SourceType == entity.OrderSourceAuction {
		return s.releaseAuctionOrderBinding(ctx, tx, order)
	}
	return s.restoreFixedPriceForSaleStock(ctx, tx, order)
}

// restoreFixedPriceForSaleStock restores stock for the fixed-price forSale
// that sourced an order. This is called when a fixed-price/negotiation order
// is cancelled or expired.
//
// SOURCE RESOLUTION (Stage 5): the forSale is locked via order.SourceID
// (order.source_type = 'for_sale' -> for_sales.id). It is a
// hard error for order_items.product_id to be treated as a selling-surface
// ID — a fixed-price/negotiation order maps to exactly one surface, and that
// surface is orders.source_id. item.Quantity is summed because every item of
// a single-source order belongs to that same surface.
//
// LOCK HIERARCHY:
// - Order is already locked (FOR UPDATE) by caller
// - The sourcing forSale is locked (FOR UPDATE) before restoration
//
// IDEMPOTENCY: Safe to call multiple times - RestoreQuantity only increments,
// and if this method is called again after a partial rollback, it will correct
// the stock level.
func (s *OrderCompletionService) restoreFixedPriceForSaleStock(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
) error {
	// Sum quantities across order items and restore them against the sourcing
	// surface. order_items.product_id is NOT consulted here (Product identity only).
	orderItems, err := s.repo.GetOrderItems(ctx, tx, order.ID)
	if err != nil {
		return err
	}
	// No items means no stock was claimed on this order — nothing to restore
	// (also keeps the shortcut for synthetic/empty orders).
	if len(orderItems) == 0 {
		return nil
	}

	// The sourcing forSale is order.source_id (order.source_type =
	// 'for_sale' -> for_sales.id). Never treat
	// order_items.product_id as a selling-surface ID.
	if order.SourceID == uuid.Nil {
		return fmt.Errorf("fixed-price order has no source_id: order=%s", order.ID)
	}

	// Lock the sourcing forSale (FOR UPDATE).
	forSale, err := s.forSaleRepo.GetForUpdate(ctx, tx, order.SourceID)
	if err != nil {
		return err
	}

	totalQuantity := 0
	for _, item := range orderItems {
		totalQuantity += item.Quantity
	}

	// Restore quantity (this handles status reversion from sold to active if needed)
	if err := forSale.RestoreQuantity(totalQuantity); err != nil {
		return err
	}

	// Persist the stock restoration
	if err := s.forSaleRepo.UpdateStock(ctx, tx, forSale); err != nil {
		return err
	}

	return nil
}

// releaseAuctionOrderBinding handles an auction-sourced order that is being
// cancelled or expired before payment succeeded.
//
// Bid-win claim flow: the auction stays in waiting_settlement with OrderID
// bound until payment succeeds. When the bound order is cancelled/expired
// unpaid, the settlement has FAILED: release the binding, record the buyer's
// commerce violation (buyer_bnr), apply/extend the buyer restriction, and
// return the auction to DRAFT so the seller can relist the same auction
// record. All in the caller's transaction.
//
// Buy-now flow: the auction was ended at order creation; cancelling/expiring
// the unpaid order only releases the binding (the auction stays ended —
// a settled-by-buy-now auction never reopens).
//
// Idempotent: an auction with OrderID already nil or already returned to
// DRAFT is a no-op.
func (s *OrderCompletionService) releaseAuctionOrderBinding(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
) error {
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, order.SourceID)
	if err != nil {
		return fmt.Errorf("failed to lock auction for order release: %w", err)
	}

	if auction.OrderID == nil {
		// Already released (idempotent retry).
		return nil
	}

	// Release the binding first (validates the order actually belongs to this
	// auction).
	if err := auction.ReleaseUnpaidOrder(order.ID); err != nil {
		return fmt.Errorf("failed to release auction order binding: %w", err)
	}

	if auction.Status == auctionEntity.StatusWaitingSettlement {
		// Bid-win settlement failure: record the buyer violation + restriction
		// and return the auction to DRAFT.
		if s.commerceViolationRepo == nil {
			return fmt.Errorf("commerce violation repo not wired; cannot return auction %s to draft safely", auction.ID)
		}
		if _, _, err := commercegov.RecordViolationAndRestrict(
			ctx, tx, s.commerceViolationRepo, commercegov.RecordInput{
				UserID:        order.BuyerID,
				ViolationType: commercegov.ViolationBuyerBNR,
				SourceType:    commercegov.SourceTypeAuction,
				SourceID:      auction.ID,
				Reason:        "buyer failed to pay after shipping was resolved (payment window elapsed)",
				Metadata: map[string]any{
					"order_id":             order.ID.String(),
					"shipping_resolved_at": auction.ShippingResolvedAt,
				},
			},
		); err != nil {
			return fmt.Errorf("failed to record buyer violation for expired auction order: %w", err)
		}
		if err := auction.TransitionToDraftOnSettlementFailure(); err != nil {
			return fmt.Errorf("failed to return auction to draft on payment failure: %w", err)
		}
		// CROSS-LIFECYCLE ISOLATION: invalidate all ACTIVE shipping quotes
		// for this product so no stale quote from the previous settlement
		// lifecycle can be used in the next lifecycle.
		if s.shippingQuoteService != nil {
			if err := s.shippingQuoteService.InvalidateQuotesByProduct(ctx, tx, auction.ProductID); err != nil {
				return fmt.Errorf("failed to invalidate shipping quotes on settlement failure: %w", err)
			}
		}
	}

	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return fmt.Errorf("failed to persist auction order release: %w", err)
	}

	return nil
}

// invalidateRatingForOrder marks the rating for an order as invalid.
// This is called when an order is refunded to prevent rating abuse.
//
// RATING INVALIDATION:
// - Sets invalidated_at = NOW() on the order's rating
// - Prevents the rating from being counted in aggregations
// - Does not fail the refund if rating invalidation fails
//
// Use cases:
// - Full refund (RefundOrder)
// - Dispute refund (RefundFromDispute)
// - Partial refund (PartialRefund)
//
// RATING DOMAIN BOUNDARY: This method enforces the rating domain boundary
// by delegating to RatingMutator interface instead of accessing order_ratings table directly.
// Only rating invalidation is allowed - no read operations through this boundary.
func (s *OrderCompletionService) invalidateRatingForOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	return s.ratingMutator.InvalidateForOrder(ctx, tx, orderID)
}
