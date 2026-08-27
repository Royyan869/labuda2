// ⚠️ FINANCIAL RULE:
// All money operations MUST go through WalletService.
// Direct balance mutation is forbidden.
//
// Order domain is a PRICING SNAPSHOT only.
// Wallet domain is the SINGLE SOURCE OF TRUTH for all money operations.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	auctionRepoImpl "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	forSaleRepoImpl "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	forSalerepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	"github.com/labuda/backend/internal/commerce/order/entity"
	orderRepoImpl "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	ratingApp "github.com/labuda/backend/internal/commerce/order/rating/application"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	walletApp "github.com/labuda/backend/internal/core/wallet/application"
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

// ErrActiveRefundCheckerNotConfigured is returned when Complete() is called
// without a wired ActiveRefundChecker. Fail-closed by design: escrow must
// not be released if the refund guard is absent.
var ErrActiveRefundCheckerNotConfigured = fmt.Errorf(
	"order: active refund checker not configured; cannot complete order safely")

// ============================================================================
// WALLET ESCROW DERIVATION
// ============================================================================

// mapWalletEscrowToOrderEscrow maps Wallet.Escrow.Status to Order.EscrowStatus.
//
// CRITICAL: This is the ONLY valid way to set Order.EscrowStatus.
// Order.EscrowStatus MUST always be derived from Wallet.Escrow.Status.
//
// Wallet.Escrow.Status values (from wallet/entity/escrow.go):
// - "holding": Funds held for pending order
// - "released": Released to seller (order complete)
// - "refunded": Refunded to buyer (order cancelled)
//
// This function ensures Order.EscrowStatus is a READ-ONLY projection of Wallet state.
func mapWalletEscrowToOrderEscrow(walletEscrowStatus string) entity.EscrowStatus {
	switch walletEscrowStatus {
	case "holding":
		return entity.EscrowStatusHolding
	case "released":
		return entity.EscrowStatusReleased
	case "refunded":
		return entity.EscrowStatusRefunded
	default:
		// If wallet has no escrow or unknown state, default to holding
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
// WALLET INTEGRATION:
// - Uses WalletService to fetch escrow state for deriving Order.EscrowStatus
// - Order.EscrowStatus is ALWAYS derived from Wallet.Escrow.Status
type OrderCompletionService struct {
	repo                 orderrepository.OrderRepository
	forSaleRepo          forSalerepo.ForSaleRepository
	auctionRepo          *auctionRepoImpl.AuctionRepository // PASS_20B: auction order-binding release on cancel/expire
	ownership            *auth.OwnershipValidator
	accountStatusChecker auth.AccountStatusChecker
	outboxRepo           *outboxRepo.OutboxRepository
	idempotencyRepo      *idempotencyRepo.Repository
	paymentService       *OrderPaymentService
	paymentRepo          *paymentRepo.PaymentRepository
	coinsService         *coinsApp.CoinsService  // Used for earning points on completion (NOT for refunds)
	ratingMutator        ratingApp.RatingMutator // Interface-based access (write-only)
	supportRepo          supportrepo.Repository
	shippingQuoteService ShippingQuoteService
	disputeRepo          disputerepo.DisputeRepository // Entry point guard: check dispute status
	walletService        *walletApp.WalletService      // Used to derive Order.EscrowStatus from Wallet state
	activeRefundChecker  ActiveRefundChecker           // H2-F2a: Block completion if refund is active
	logger               *zap.Logger
}

// ShippingQuoteService defines the interface for shipping quote operations.
type ShippingQuoteService interface {
	ReactivateQuoteIfEligible(ctx context.Context, tx db.Tx, quoteID uuid.UUID) error
}

// ActiveRefundChecker checks whether an order has an active (non-terminal) refund.
// Used by Complete() to prevent auto-releasing escrow while a refund is being
// negotiated or awaiting gateway settlement.
//
// Terminal statuses that do NOT block: refunded, admin_released.
// All other statuses block: pending_seller_review, seller_approved,
// seller_rejected, escalated_to_admin, admin_refunded.
type ActiveRefundChecker interface {
	HasActiveRefundByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) (bool, error)
}

// NewOrderCompletionService creates a new OrderCompletionService.
func NewOrderCompletionService(
	accountStatusChecker auth.AccountStatusChecker,
	outboxRepo *outboxRepo.OutboxRepository,
	paymentService *OrderPaymentService,
	coinsService *coinsApp.CoinsService,
	shippingQuoteService ShippingQuoteService,
	disputeRepo disputerepo.DisputeRepository,
	walletService *walletApp.WalletService,
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
		walletService:        walletService, // Used to derive Order.EscrowStatus from Wallet state
		logger:               logger,
	}
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
// CRITICAL: Order.EscrowStatus is DERIVED from Wallet.Escrow.Status
// This ensures Order.EscrowStatus is ALWAYS a projection of Wallet state.
//
// IDEMPOTENCY: If order is already in paid status, returns success immediately.
// This prevents duplicate state transitions on retry.
//
// CRITICAL ORDER:
// 1. Lock order and validate transition
// 2. CRITICAL: Check if order is expired (PHASE 6 DEFENSIVE GUARD)
// 3. Check idempotency (already paid -> return success)
// 4. Fetch wallet escrow state
// 5. Derive Order.EscrowStatus from wallet state
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

	// Step 4: CRITICAL - Fetch wallet escrow state to derive Order.EscrowStatus
	// This ensures Order.EscrowStatus is ALWAYS a projection of Wallet state
	walletEscrow, err := s.walletService.GetEscrowForOrder(ctx, tx, orderID)
	if err != nil {
		s.logger.Error("failed_to_fetch_wallet_escrow",
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to fetch wallet escrow for order: %w", err)
	}

	// Step 5: Derive Order.EscrowStatus from Wallet state
	// If escrow doesn't exist yet (edge case), default to holding
	// This should not happen in normal flow since SettlePaymentByID creates escrow first
	var derivedEscrowStatus entity.EscrowStatus
	if walletEscrow == nil {
		s.logger.Warn("wallet_escrow_not_found_for_paid_order",
			zap.String("order_id", orderID.String()),
			zap.String("reason", "escrow_should_exist_after_payment"),
		)
		derivedEscrowStatus = entity.EscrowStatusHolding // Default for paid orders
	} else {
		derivedEscrowStatus = mapWalletEscrowToOrderEscrow(walletEscrow.Status.String())
	}

	// Step 6: Update order state
	if err := order.MarkPaid(); err != nil {
		return err
	}

	// CRITICAL: Set EscrowStatus from Wallet, not from business logic
	order.EscrowStatus = derivedEscrowStatus

	// No ledger entries here - escrow already funded by PaymentSettlementService
	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return err
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

	// REFUND DOMAIN GUARD (H2-F2a): Block completion if refund is active.
	// Prevents releasing escrow while buyer/seller refund negotiation or gateway
	// settlement is in progress. Without this, auto-complete can release funds
	// to seller, creating a post-release money gap if the refund later succeeds.
	if s.activeRefundChecker == nil {
		return ErrActiveRefundCheckerNotConfigured
	}
	hasActiveRefund, err := s.activeRefundChecker.HasActiveRefundByOrderID(ctx, tx, order.ID)
	if err != nil {
		return fmt.Errorf("failed to check active refund: %w", err)
	}
	if hasActiveRefund {
		return fmt.Errorf("cannot complete order: active refund exists (order_id=%s)", order.ID)
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
	// No buyer/seller wallet balance is touched — the canonical seller payable
	// surface is financial_accounts[SELLER_PAYABLE].
	releaseSummary, err := s.paymentService.ReleaseGatewayEscrowToSeller(ctx, tx, order)
	if err != nil {
		return err
	}

	// ============================================================
	// STEP 3: UPDATE ORDER STATE (reflect financial state)
	// ============================================================
	// NOW update Order.EscrowStatus to match Wallet state
	// Order domain follows Wallet domain (financial-first architecture)
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
	// - Uses final paid amount (Subtotal + Shipping), NOT forSale price
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
		order.Subtotal.Int64()+order.ShippingTotal.Int64(), // Final paid amount (excluding commission)
	); err != nil {
		// ====================================================================
		// OPERATIONAL VISIBILITY: LOG FAILED EARN ATTEMPTS
		// ====================================================================
		// Log the failure for operational visibility.
		// Order completion succeeds regardless — the loyalty reward is secondary.
		// Reconciliation: query coins_transactions for missing order_reward entries.
		finalPaidAmount := order.Subtotal.Int64() + order.ShippingTotal.Int64()
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
	// STEP 5.5: REACTIVATE SHIPPING QUOTE IF ELIGIBLE
	// ============================================================
	// Quote is marked USED at order creation (checkout), before payment.
	// If buyer cancels from pending, the quote was consumed for an order
	// that was never paid — reactivate so buyer can retry.
	if order.ShippingQuoteID != nil && *order.ShippingQuoteID != uuid.Nil {
		if err := s.shippingQuoteService.ReactivateQuoteIfEligible(ctx, tx, *order.ShippingQuoteID); err != nil {
			s.logger.Warn("Failed to reactivate shipping quote after order cancel",
				zap.String("order_id", order.ID.String()),
				zap.String("shipping_quote_id", order.ShippingQuoteID.String()),
				zap.Error(err),
			)
		}
	}

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
	refundAmount := order.Subtotal.Int64() + order.ShippingTotal.Int64()
	if err := s.paymentService.InitiateGatewayRefundForOrder(
		ctx, tx, order, auth.SystemCallerID,
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

	walletEscrow, err := s.walletService.GetEscrowForOrder(ctx, tx, order.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch wallet escrow after refund: %w", err)
	}
	derivedEscrowStatus := mapWalletEscrowToOrderEscrow(walletEscrow.Status.String())

	// ============================================================
	// STEP 7: UPDATE ORDER STATE (reflect financial state)
	// ============================================================
	// NOW update Order.EscrowStatus to match Wallet state
	// Order domain follows Wallet domain (financial-first architecture)
	order.Status = entity.StatusCancelledTimeout
	// CRITICAL: Set Order.EscrowStatus from Wallet state (not independent)
	order.EscrowStatus = derivedEscrowStatus
	order.UpdatedAt = time.Now()

	// ============================================================
	// STEP 8: EMIT COINS REFUND REQUIRED EVENT
	// ============================================================
	// STRICT_EVENT_ATOMIC: Coins must be refunded when order is cancelled due
	// to shipment timeout. Outbox failure rolls back the entire transaction so
	// the cancel can be retried by the worker — buyer coins are never silently lost.
	if order.CoinsUsed > 0 {
		payload := map[string]interface{}{
			"order_id": order.ID.String(),
			"user_id":  order.BuyerID.String(),
			"reason":   "order_cancelled_overdue",
			"source":   "buyer_overdue_cancel",
		}
		payloadBytes, _ := json.Marshal(payload)
		if err := s.outboxRepo.InsertEvent(ctx, tx, "coins.refund_required", order.ID, payloadBytes); err != nil {
			return fmt.Errorf("outbox coins.refund_required (overdue cancel): %w", err)
		}
	}

	// ============================================================
	// STEP 9: RESTORE LISTING STOCK
	// ============================================================
	// Restore stock for all forSales in this order
	// This must happen BEFORE updating order status to ensure atomicity
	if err := s.restoreForSaleStock(ctx, tx, order); err != nil {
		return err
	}

	// ============================================================
	// STEP 9.5: REACTIVATE SHIPPING QUOTE IF ELIGIBLE
	// ============================================================
	// Seller did not ship, buyer force-cancelled with full refund.
	// Quote was consumed at checkout — reactivate so buyer can retry.
	if order.ShippingQuoteID != nil && *order.ShippingQuoteID != uuid.Nil {
		if err := s.shippingQuoteService.ReactivateQuoteIfEligible(ctx, tx, *order.ShippingQuoteID); err != nil {
			s.logger.Warn("Failed to reactivate shipping quote after overdue cancel",
				zap.String("order_id", order.ID.String()),
				zap.String("shipping_quote_id", order.ShippingQuoteID.String()),
				zap.Error(err),
			)
		}
	}

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
// COINS REFUND:
//   - Emits coins.refund_required event when payment expires
//   - CoinsRefundRequiredHandler processes the refund asynchronously
//   - Uses INSERT-FIRST pattern for idempotent refund
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
	// - This calls WalletService.RefundEscrow which moves:
	//   buyer.held_balance → buyer.available_balance (full refund)
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
	// WALLET SAFETY: WalletService.RefundEscrow is idempotent
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
	escrowForExpiry, escrowErr := s.walletService.GetEscrowForOrder(ctx, tx, order.ID)
	if escrowErr != nil {
		return fmt.Errorf("CRITICAL: failed to load escrow for expiry: order_id=%s, error=%w", orderID, escrowErr)
	}
	if escrowForExpiry != nil {
		refundAmount := order.Subtotal.Int64() + order.ShippingTotal.Int64()
		if err := s.paymentService.InitiateGatewayRefundForOrder(
			ctx, tx, order, auth.SystemCallerID,
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

	// ============================================================
	// STEP 4: EMIT COINS REFUND REQUIRED EVENT (SINGLE ENTRY POINT)
	// ============================================================
	// Coins must be refunded when order expires due to payment failure.
	//
	// CRITICAL: We NO LONGER call RefundCoinsInternal() directly.
	// Instead, we emit a coins.refund_required event that will be
	// processed by CoinsRefundRequiredHandler - the SINGLE SOURCE OF TRUTH.
	//
	// This ensures:
	// 1. All refunds flow through one handler (idempotent)
	// 2. Refunds are based on transactions, not order snapshot
	// 3. Failures are recoverable via coins.refund_failed event
	//
	// NOTE: We check order.CoinsUsed > 0 as an optimization to avoid
	// emitting unnecessary events. The handler will still verify
	// the actual spend transaction exists.
	// STRICT_EVENT_ATOMIC: Coins must be refunded when order expires.
	// Outbox failure rolls back the entire transaction so the expire can be
	// retried by the worker — buyer coins are never silently lost.
	if order.CoinsUsed > 0 {
		payload := map[string]interface{}{
			"order_id": order.ID.String(),
			"user_id":  order.BuyerID.String(),
			"reason":   "order_expired",
			"source":   "order_expire_worker",
		}
		payloadBytes, _ := json.Marshal(payload)
		if err := s.outboxRepo.InsertEvent(ctx, tx, "coins.refund_required", order.ID, payloadBytes); err != nil {
			return fmt.Errorf("outbox coins.refund_required (expire): %w", err)
		}
	}

	// ============================================================
	// STEP 4.5: REACTIVATE SHIPPING QUOTE IF ELIGIBLE
	// ============================================================
	// HARD FIX - SHIPPING QUOTE REUSE
	// When order expires, reactivate the shipping quote if it was used.
	// This allows the buyer to reuse the quote for a new order attempt.
	//
	// TRIGGER: order_expired (this method)
	// VALIDATION: Done by ReactivateQuoteIfEligible
	// - Quote must be in USED status
	// - Order using quote must be in EXPIRED status
	// - No other valid orders should be using this quote
	if order.ShippingQuoteID != nil && *order.ShippingQuoteID != uuid.Nil {
		if err := s.shippingQuoteService.ReactivateQuoteIfEligible(ctx, tx, *order.ShippingQuoteID); err != nil {
			// Log error but don't fail the expiry operation
			// The order expiry is more critical than the quote reactivation
			s.logger.Warn("Failed to reactivate shipping quote after order expiry",
				zap.String("order_id", order.ID.String()),
				zap.String("shipping_quote_id", order.ShippingQuoteID.String()),
				zap.Error(err),
			)
		}
	}

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

// MarkHasDisputePostRelease marks has_dispute=true on a completed order WITHOUT
// changing the order status. Used exclusively by DisputeService when a dispute
// is opened on an already-completed (post-release) order.
//
// Pre-release disputes use MarkDisputeOpen (status → dispute_open).
// Post-release disputes leave status = completed and rely on a dispute_freeze
// record in the finance layer to block withdrawal.
//
// LOCK: caller must hold order FOR UPDATE (via GetForUpdate) in the same tx.
func (s *OrderCompletionService) MarkHasDisputePostRelease(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	order, err := s.repo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}
	if order.Status != entity.StatusCompleted {
		return fmt.Errorf("MarkHasDisputePostRelease: expected completed order, got %s", order.Status)
	}
	if order.HasDispute {
		return nil // idempotent
	}
	order.HasDispute = true
	order.UpdatedAt = time.Now()
	if err := s.repo.UpdateStatusTx(ctx, tx, order); err != nil {
		return fmt.Errorf("MarkHasDisputePostRelease: persist failed: %w", err)
	}
	return nil
}

// RefundOrder processes a full refund to buyer.
// Creates ledger transaction first, then transitions escrow_status to "refunded".
//
// UNIFIED SETTLEMENT MODEL V2:
//   - Refund amount is the canonical buyer base (subtotal + shipping = PD + S).
//     Commission C is seller/platform-side and NEVER part of buyer refund cash.
//   - No platform revenue is recognized when refund occurs before release
//   - Commission is never reversed (it was never recognized in the first place)
//
// GOVERNANCE BOUNDARY:
// - ONLY allows escrow_status = "holding"
// - EXPLICITLY REJECTS orders with active disputes (status = dispute_open)
// - For dispute resolution refunds, use RefundFromDispute instead
//
// CRITICAL: This method is idempotent via ledger idempotency key.
// Even if called multiple times, the refund will only execute once.
//
// LEDGER ENTRIES (atomic, before state update):
// - Debit: ESCROW (system account) - decreases by escrow_amount (canonical buyer base)
// - Credit: buyer's BUYER_REFUNDABLE - increases by escrow_amount (canonical buyer base)
//
// STATE UPDATES (after ledger success):
// - escrow_status = "refunded"
// - status = "refunded"
// - refunded_amount = escrow_amount
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
	// "frozen" state requires dispute resolution path via RefundFromDispute
	if order.EscrowStatus != entity.EscrowStatusHolding {
		return &entity.InvalidEscrowStatusError{
			CurrentStatus:  order.EscrowStatus,
			RequiredStatus: entity.EscrowStatusHolding,
		}
	}

	// CANONICAL REFUND: dispatch gateway refund FIRST, then flip local escrow.
	refundAmount := order.Subtotal.Int64() + order.ShippingTotal.Int64()
	if err := s.paymentService.InitiateGatewayRefundForOrder(
		ctx, tx, order, auth.SystemCallerID,
		refundAmount, "other",
		fmt.Sprintf("system_refund_manual_%s", order.ID.String()),
	); err != nil {
		return fmt.Errorf("gateway refund initiation failed: %w", err)
	}
	if err := s.paymentService.RefundToBuyer(ctx, tx, order); err != nil {
		return err
	}

	walletEscrow, err := s.walletService.GetEscrowForOrder(ctx, tx, order.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch wallet escrow after refund: %w", err)
	}
	derivedEscrowStatus := mapWalletEscrowToOrderEscrow(walletEscrow.Status.String())

	// ============================================================
	// EMIT MONEY REFUNDED EVENT (triggers coins refund)
	// ============================================================
	if order.CoinsUsed > 0 {
		payload := map[string]interface{}{
			"order_id":    order.ID.String(),
			"buyer_id":    order.BuyerID.String(),
			"refund_type": "full",
			"coins_used":  order.CoinsUsed,
		}
		payloadBytes, _ := json.Marshal(payload)
		_ = s.outboxRepo.InsertEvent(ctx, tx, "money.refunded", order.ID, payloadBytes)
	}

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
	// REACTIVATE SHIPPING QUOTE IF ELIGIBLE
	// ============================================================
	// HARD FIX - SHIPPING QUOTE REUSE
	// When order is refunded, reactivate the shipping quote if it was used.
	// This allows the buyer to reuse the quote for a new order attempt.
	//
	// TRIGGER: payment_failed (order refund)
	// VALIDATION: Done by ReactivateQuoteIfEligible
	// - Quote must be in USED status
	// - Order using quote must be in REFUNDED status
	// - No other valid orders should be using this quote
	if order.ShippingQuoteID != nil && *order.ShippingQuoteID != uuid.Nil {
		if err := s.shippingQuoteService.ReactivateQuoteIfEligible(ctx, tx, *order.ShippingQuoteID); err != nil {
			// Log error but don't fail the refund operation
			// The refund is more critical than the quote reactivation
			s.logger.Warn("Failed to reactivate shipping quote after order refund",
				zap.String("order_id", order.ID.String()),
				zap.String("shipping_quote_id", order.ShippingQuoteID.String()),
				zap.Error(err),
			)
		}
	}

	// CRITICAL: Set Order.EscrowStatus from Wallet state (not independent)
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
// This method handles dispute-approved refunds.
// It enforces escrow_status = "frozen" to ensure dispute path exclusivity.
//
// UNIFIED SETTLEMENT MODEL V2:
//   - Refund amount is the canonical buyer base (subtotal + shipping = PD + S).
//     Commission C is seller/platform-side and NEVER part of buyer refund cash.
//   - No platform revenue is recognized when refund occurs before release
//   - Commission is never reversed (it was never recognized in the first place)
//
// GOVERNANCE: This is the ONLY path that can refund an order with status = dispute_open.
//
// LEDGER ENTRIES (atomic, before state update):
// - Debit: ESCROW (system account) - decreases by escrow_amount (canonical buyer base)
// - Credit: buyer's BUYER_REFUNDABLE - increases by escrow_amount (canonical buyer base)
//
// STATE UPDATES (after ledger success):
// - escrow_status = "refunded"
// - status = "refunded"
// - refunded_amount = escrow_amount
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

	// DISPUTE → WALLET INTEGRATION: Log that dispute resolution is triggering wallet refund
	s.logger.Info("wallet_dispute_refund_triggered",
		zap.String("order_id", order.ID.String()),
		zap.String("buyer_id", order.BuyerID.String()),
		zap.String("seller_id", order.SellerID.String()),
		zap.String("escrow_status", string(order.EscrowStatus)),
		zap.String("trigger", "dispute_resolution"),
	)

	// CANONICAL REFUND: dispatch gateway refund FIRST (creates Refund row
	// in admin_refunded + admin_refunded gateway dispatch), then flip the
	// local escrow state. Ledger reversal happens at webhook ack.
	refundAmount := order.Subtotal.Int64() + order.ShippingTotal.Int64()
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

	walletEscrow, _, err := s.walletService.RefundGatewayEscrow(ctx, tx, orderID)
	if err != nil {
		s.logger.Error("wallet_refund_escrow_failed",
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to refund escrow via wallet service: %w", err)
	}

	// Derive Order.EscrowStatus from Wallet state (CRITICAL: no independent state)
	derivedEscrowStatus := mapWalletEscrowToOrderEscrow(walletEscrow.Status.String())

	// ============================================================
	// EMIT MONEY REFUNDED EVENT
	// ============================================================
	// Event consumed as NoHandlerAuditOnly (coins refund moved to ack-time).
	if order.CoinsUsed > 0 {
		payload := map[string]interface{}{
			"order_id":    order.ID.String(),
			"buyer_id":    order.BuyerID.String(),
			"refund_type": "full",
			"coins_used":  order.CoinsUsed,
		}
		payloadBytes, _ := json.Marshal(payload)
		_ = s.outboxRepo.InsertEvent(ctx, tx, "money.refunded", order.ID, payloadBytes)
	}

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
	// REACTIVATE SHIPPING QUOTE IF ELIGIBLE
	// ============================================================
	// HARD FIX - SHIPPING QUOTE REUSE
	// When order is refunded via dispute, reactivate the shipping quote if it was used.
	// This allows the buyer to reuse the quote for a new order attempt.
	//
	// TRIGGER: payment_failed (order refund via dispute)
	// VALIDATION: Done by ReactivateQuoteIfEligible
	// - Quote must be in USED status
	// - Order using quote must be in REFUNDED status
	// - No other valid orders should be using this quote
	if order.ShippingQuoteID != nil && *order.ShippingQuoteID != uuid.Nil {
		if err := s.shippingQuoteService.ReactivateQuoteIfEligible(ctx, tx, *order.ShippingQuoteID); err != nil {
			// Log error but don't fail the refund operation
			// The refund is more critical than the quote reactivation
			s.logger.Warn("Failed to reactivate shipping quote after dispute refund",
				zap.String("order_id", order.ID.String()),
				zap.String("shipping_quote_id", order.ShippingQuoteID.String()),
				zap.Error(err),
			)
		}
	}

	// CRITICAL: Set Order.EscrowStatus from Wallet state (not independent)
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

// refundFromDispute is an alias for RefundFromDispute for backward compatibility.
// Deprecated: Use RefundFromDispute directly.
func (s *OrderCompletionService) refundFromDispute(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	return s.RefundFromDispute(ctx, tx, orderID, auth.SystemCallerID)
}

// ReleaseFromDispute processes an escrow release to seller from dispute resolution.
// PUBLIC API: Called by DisputeService for dispute resolution in favor of seller.
//
// Gateway-aware release: mirrors the canonical Complete() release semantics
// (paymentService.ReleaseGatewayEscrowToSeller). Buyer wallet balances are NOT
// touched — the seller's withdrawable surface is financial_accounts[SELLER_PAYABLE].
//
// FLOW (single tx, caller-owned):
//  1. Lock order and enforce dispute guards (HasDispute, status == dispute_open).
//  2. Call paymentService.ReleaseGatewayEscrowToSeller (wallet escrow flip +
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

	// DISPUTE → WALLET INTEGRATION: Log that dispute resolution is triggering wallet release
	s.logger.Info("wallet_dispute_release_triggered",
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

	// Update order status (Order.EscrowStatus mirrors Wallet.Escrow.Status).
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
// - Buyer gets refund for item price (subtotal)
// - Seller gets release for shipping fee (shipping_total)
//
// STRICT RULES:
// - ADMIN ONLY (no user-triggered)
// - MUST use walletService.PartialRefundEscrow (atomic single transaction)
// - MUST be from dispute_open status with frozen escrow
// - Refund amount MUST equal order.Subtotal (item price only)
// - Shipping fee is released to seller (remainder)
//
// FINANCIAL FLOW:
// 1. Validate order.Subtotal + order.ShippingTotal == escrow_amount
// 2. Call walletService.PartialRefundEscrow with refund_amount = order.Subtotal
// 3. Wallet service handles:
//   - Transfer subtotal to buyer wallet
//   - Transfer shipping_total to seller wallet
//   - Create 3 ledger entries (debit buyer held, credit buyer, credit seller)
//
// 4. Update order status to partially_refunded
// 5. Update escrow status to released
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

	// DATA VALIDATION: Ensure order has subtotal, shipping_total, and commission_amount.
	itemPrice := order.Subtotal
	shippingFee := order.ShippingTotal
	commissionFee := order.CommissionAmount

	// CANONICAL ESCROW VALIDATION: escrow = total_before_coins_amount = PD + S.
	// The buyer-funded escrow excludes commission (C is a seller/platform-side
	// allocation, not buyer cash). The rejected model (P+S+C via
	// CalculateGrossEscrowFromSnapshot) must not gate the partial dispute path.
	escrowAmount := order.TotalBeforeCoinsAmount
	calculatedTotal := itemPrice.Add(shippingFee)

	if !calculatedTotal.Equal(escrowAmount) {
		return fmt.Errorf("partial dispute validation failed: item_price + shipping_fee != escrow_amount (item=%d, shipping=%d, escrow=%d)",
			itemPrice.Int64(), shippingFee.Int64(), escrowAmount.Int64())
	}

	// LOG: Dispute resolution triggering wallet partial refund
	s.logger.Info("wallet_dispute_partial_refund_triggered",
		zap.String("order_id", order.ID.String()),
		zap.String("buyer_id", order.BuyerID.String()),
		zap.String("seller_id", order.SellerID.String()),
		zap.Int64("item_price", itemPrice.Int64()),
		zap.Int64("shipping_fee", shippingFee.Int64()),
		zap.Int64("commission_fee", commissionFee.Int64()),
		zap.Int64("escrow_amount", escrowAmount.Int64()),
		zap.String("trigger", "dispute_partial_split_resolution"),
	)

	// CANONICAL PARTIAL REFUND: dispatch gateway refund for the BUYER
	// portion (item price) ONLY. Local escrow.status DOES NOT flip here —
	// the wallet primitive (PartialRefundGatewayEscrow) is now invoked
	// from RefundService.HandleGatewayRefundAck after the canonical ledger
	// reversal commits. Escrow stays HOLDING until the gateway ack arrives.
	if err := s.paymentService.InitiateGatewayRefundForOrder(
		ctx, tx, order, adminID,
		itemPrice.Int64(), "other",
		fmt.Sprintf("system_refund_partial_dispute_%s", order.ID.String()),
	); err != nil {
		return fmt.Errorf("partial gateway refund initiation failed: %w", err)
	}

	// ============================================================
	// EMIT PARTIAL REFUND AND RELEASE EVENTS (eliminates ambiguity)
	// ============================================================
	// STRICT MODE: Separate events for partial operations
	// - money.partial_refund: Buyer gets item price refund (triggers coin refund)
	// - money.partial_release: Seller gets shipping fee release (audit only)
	if order.CoinsUsed > 0 {
		// Emit money.partial_refund event (triggers proportional coin refund)
		partialRefundPayload := map[string]interface{}{
			"order_id":     order.ID.String(),
			"buyer_id":     order.BuyerID.String(),
			"item_price":   itemPrice.Int64(),
			"shipping_fee": shippingFee.Int64(),
			"total_escrow": escrowAmount.Int64(),
			"coins_used":   order.CoinsUsed,
		}
		partialRefundBytes, _ := json.Marshal(partialRefundPayload)
		_ = s.outboxRepo.InsertEvent(ctx, tx, "money.partial_refund", order.ID, partialRefundBytes)
	}

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

	s.logger.Info("wallet_dispute_partial_refund_success",
		zap.String("order_id", order.ID.String()),
		zap.String("buyer_id", order.BuyerID.String()),
		zap.String("seller_id", order.SellerID.String()),
		zap.Int64("buyer_refund", itemPrice.Int64()),
		zap.Int64("seller_payout", shippingFee.Int64()),
		zap.String("resolution_type", "partial_split"),
	)

	return nil
}

// ============================================================================
// PARKED POST-RELEASE DISPUTE REFUND (H2-F2b)
// ============================================================================
//
// These methods are parked under the owner finality rule. Post-release buyer
// objections are handled outside the app, so the live runtime no longer calls
// into this refund path.
//
// Historical differences from pre-release RefundFromDispute/PartialRefundFromDispute:
//   - Order status stayed "completed" (terminal — no valid transitions)
//   - Escrow status stayed "released" (terminal — no flip)
//   - Dispute freeze stayed ACTIVE until gateway ack succeeded
//   - Ledger reversal happened at gateway webhook ack time
// ============================================================================

// RefundFromDisputePostRelease is parked under the owner finality rule.
// Post-release buyer objections are handled outside the app and this method
// now returns an explicit error instead of dispatching a refund.
func (s *OrderCompletionService) RefundFromDisputePostRelease(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	adminID uuid.UUID,
) error {
	return errors.New("post-release dispute refunds are disabled; handle objections outside the app")
}

// PartialRefundFromDisputePostRelease is parked under the owner finality rule.
// Post-release buyer objections are handled outside the app and this method
// now returns an explicit error instead of dispatching a refund.
func (s *OrderCompletionService) PartialRefundFromDisputePostRelease(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	adminID uuid.UUID,
) error {
	return errors.New("post-release dispute refunds are disabled; handle objections outside the app")
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

// releaseAuctionOrderBinding releases the auction<->order binding for a
// cancelled/expired auction order (PASS_20B — see D2 in the PASS_20B
// report). Locks the auction row, then clears OrderID via
// Auction.ReleaseUnpaidOrder (idempotent, mismatch-safe — see that method's
// doc comment for why the auction's own Status stays Ended rather than
// reopening). Product carries no selling lifecycle, so no Product row is
// touched here.
func (s *OrderCompletionService) releaseAuctionOrderBinding(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
) error {
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, order.SourceID)
	if err != nil {
		return fmt.Errorf("failed to lock auction for order release: %w", err)
	}

	if err := auction.ReleaseUnpaidOrder(order.ID); err != nil {
		return fmt.Errorf("failed to release auction order binding: %w", err)
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
