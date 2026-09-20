// Package application provides the refund service for refund lifecycle management.
// Refund = buyer <-> seller negotiation (before dispute escalation).
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	orderRepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	escrowApp "github.com/labuda/backend/internal/core/escrow/application"
	escrowEntity "github.com/labuda/backend/internal/core/escrow/entity"
	"github.com/labuda/backend/internal/finance/refund/entity"
	"github.com/labuda/backend/internal/finance/refund/infrastructure/repository"
	refundRepo "github.com/labuda/backend/internal/finance/refund/repository"
	"github.com/labuda/backend/internal/identity/auth"
	coinsentity "github.com/labuda/backend/internal/incentive/coins/entity"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// CreateRefundInput contains the input for creating a refund request.
type CreateRefundInput struct {
	IdempotencyKey  string
	Reason          string
	Description     *string
	EvidenceURLs    []string
	RequestedAmount int64 // Buyer's claimed amount (advisory only — not used for seller approval)
}

// ApproveRefundInput contains the input for a seller approving a refund.
type ApproveRefundInput struct {
	Notes *string
}

// RejectRefundInput contains the input for a seller rejecting a refund.
type RejectRefundInput struct {
	Notes *string
}

// RefundCursor aliases the canonical order-refund cursor authority.
type RefundCursor = refundRepo.OrderRefundCursor

func DecodeRefundCursor(raw string) (RefundCursor, error) {
	return refundRepo.DecodeOrderRefundCursor(raw)
}

// RefundHistoryPage is the canonical order-scoped refund history page.
type RefundHistoryPage struct {
	Items      []*entity.Refund
	NextCursor *RefundCursor
	HasMore    bool
}

// RefundService handles refund lifecycle operations.
//
// Refund flow (canonical decision model — see entity.Refund):
// 1. Buyer creates refund request (pending_seller_review)
// 2. Seller ACCEPTS (final decision → seller_approved) or REJECTS (not final → seller_rejected)
// 3. If rejected: buyer may escalate to dispute → escalated_to_admin (admin is the final authority)
// 4. Admin decides → admin_refunded (buyer wins) or admin_released (seller wins)
// 5. The order's own refund window closing ends an open process through the
//    normal lifecycle; an undecided refund never freezes the order once the
//    window has closed (see entity.Refund.BlocksOrderRelease).
//
// Gateway settlement of the money is a SEPARATE axis (gateway_status): it never
// rewrites the refund decision and the decision never fakes settlement.
//
// CRITICAL HARDENING: Uses live escrow state for validation (not cached Order.EscrowStatus).
type RefundService struct {
	refundRepo              refundRepo.RefundRepository
	escrowService           *escrowApp.EscrowService // CRITICAL: Used for live escrow validation
	orderRepo               orderrepository.OrderRepository
	outboxRepo              *outboxRepo.OutboxRepository
	orderRefundStatusSyncer OrderRefundStatusSyncer

	// Gateway-aware refund pipeline (TASK 33 / Phase 1).
	// Wired via SetGatewayClient. Both fields may be nil; in that case
	// InitiateGatewayRefund returns ErrGatewayClientNotConfigured and
	// HandleGatewayRefundAck logs to a Nop logger but still functions.
	gatewayClient GatewayRefundClient
	gatewayLogger *zap.Logger

	// Refund reversal ledger booking (TASK 41 / Phase 2B). Wired via
	// SetFinanceReverser. Nil-safe: if unset, HandleGatewayRefundAck
	// falls back to gateway-only state transition (Phase 1 behavior)
	// and emits refund_reversal_unwired so the gap is observable.
	financeReverser FinanceReverser

	// coinsSpendReader resolves the canonical coins spent (K) for an order
	// from the coins domain (coins_transactions). The order persists no coins
	// snapshot at all, so the coins domain is the only place K can come from.
	// Wired via SetCoinsSpendReader; nil-safe (K=0).
	coinsSpendReader CoinsSpendReader

	// db is the transaction runner used by the REC-6 Slice-2 dispatch executor
	// (DispatchPendingRec6Refunds) to claim intents and persist dispatch
	// outcomes in short, independent transactions. Wired via SetTxRunner;
	// dispatch returns an error if unset. Existing call paths are unaffected:
	// they always receive their tx from the caller.
	db db.Transactor
}

// CoinsSpendReader is the minimal coins-domain surface the refund pipeline
// needs to resolve K (coins actually redeemed for an order). The canonical
// implementation is coinsRepo.FindSpendByReference against coins_transactions
// (reference_type='order_spend', reference_id=order_id), matching the worker's
// CoinsRefundRequiredHandler.findSpendTransaction. K is a coin-subsystem
// concern: the order carries no coins snapshot to read it from.
type CoinsSpendReader interface {
	FindSpendByReference(ctx context.Context, tx db.Tx, userID uuid.UUID, referenceID uuid.UUID) (*coinsentity.CoinsTransaction, error)
}

// SetCoinsSpendReader wires the canonical K reader. Optional: when unset,
// the refund pipeline treats K as 0 (no coins redeemed), which is safe for
// orders paid without coins and matches the tracked test fixtures.
func (s *RefundService) SetCoinsSpendReader(r CoinsSpendReader) {
	s.coinsSpendReader = r
}

// coinsSpendForOrder returns the canonical coins redeemed (K) for an order by
// reading the coins-domain spend transaction. Returns 0 when the reader is
// unset or no spend exists (order paid without coins). Errors are returned so
// a real DB failure is not silently treated as "no coins".
func (s *RefundService) coinsSpendForOrder(ctx context.Context, tx db.Tx, userID, orderID uuid.UUID) (int64, error) {
	if s.coinsSpendReader == nil {
		return 0, nil
	}
	spend, err := s.coinsSpendReader.FindSpendByReference(ctx, tx, userID, orderID)
	if err != nil {
		return 0, err
	}
	if spend == nil {
		return 0, nil
	}
	return spend.Amount, nil
}

// NewRefundService creates a new RefundService.
func NewRefundService(
	escrowService *escrowApp.EscrowService,
	outboxRepo *outboxRepo.OutboxRepository,
) *RefundService {
	return &RefundService{
		refundRepo:    repository.NewRefundRepository(),
		orderRepo:     orderRepo.NewOrderRepository(),
		escrowService: escrowService, // CRITICAL: For live escrow validation
		outboxRepo:    outboxRepo,
	}
}

// SetTxRunner wires the transaction runner used by the REC-6 Slice-2 dispatch
// executor. Required for DispatchPendingRec6Refunds; every other RefundService
// method is unaffected (they take their tx from the caller).
func (s *RefundService) SetTxRunner(t db.Transactor) { s.db = t }

// CreateRefund creates a new refund request for an order.
//
// IDEMPOTENCY: Uses idempotencyKey to ensure safe retries.
//
// Transaction flow:
// 1. Lock and get order for update
// 2. Validate order status (paid, shipped, or delivered)
// 3. Check for existing refund with same idempotency key
// 4. Create refund record
// 5. Create evidence attachments if provided
// 6. Emit outbox event
//
// Caller must provide an active transaction.
func (s *RefundService) CreateRefund(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	buyerID uuid.UUID,
	input CreateRefundInput,
) (*entity.Refund, error) {
	// Get order with lock
	order, err := s.orderRepo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// Validate caller is buyer
	if order.BuyerID != buyerID {
		return nil, fmt.Errorf("only buyer can request refund")
	}

	// B4A: Refund request available from SHIPPED only (before buyer accepts).
	// Once buyer taps "Terima Barang" → Complete(), refund request path is closed.
	if order.Status != orderEntity.StatusShipped {
		return nil, fmt.Errorf("cannot request refund: order must be shipped, current status: %s", order.Status)
	}

	// C1B: Coexistence guard — block refund if order has an active dispute.
	if order.HasDispute {
		return nil, fmt.Errorf("cannot request refund: order has an active dispute")
	}

	// CRITICAL HARDENING: Validate escrow status using LIVE escrow state (not cached Order.EscrowStatus)
	liveEscrow, err := s.escrowService.GetEscrowForOrder(ctx, tx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify escrow state: %w", err)
	}
	if liveEscrow == nil || liveEscrow.Status != escrowEntity.EscrowStatusHolding {
		return nil, fmt.Errorf("cannot request refund: escrow must be in holding state, escrow status: %s",
			func() string {
				if liveEscrow == nil {
					return "none"
				}
				return string(liveEscrow.Status)
			}())
	}

	// ONE REFUND PROCESS PER ORDER: a new request is only allowed once the
	// previous process is finished. It is blocked while the previous refund's
	// decision is still open, or while money owed to the buyer is still settling
	// at the gateway (a second refund would double-refund the same order).
	existingRefund, _ := s.refundRepo.GetByOrderID(ctx, tx, orderID)
	if existingRefund != nil && (existingRefund.AwaitsDecision() || existingRefund.IsSettlementPending()) {
		// Same buyer retrying an open request is idempotent.
		if existingRefund.BuyerID == buyerID {
			return existingRefund, nil
		}
		return nil, fmt.Errorf("cannot create refund: order already has an active refund request")
	}

	// Parse reason
	reason := entity.RefundReason(input.Reason)
	if !isValidRefundReason(reason) {
		return nil, fmt.Errorf("invalid refund reason: %s", input.Reason)
	}

	// Validate requested amount (cannot exceed PD+S, excludes C).
	// CANONICAL: the buyer-funded base is total_before_coins_amount = (P-D)+S.
	// orders.discount_amount / orders.escrow_amount are NOT authoritative.
	escrowAmount := order.TotalBeforeCoinsAmount.Int64()
	if !order.HasCanonicalMoneyBase() {
		// FAIL CLOSED: no fallback to the undiscounted P + S cap. An order
		// without the persisted buyer-funded base (PD + S) has no refundable
		// canonical amount.
		return nil, fmt.Errorf("cannot create refund: canonical buyer-funded base invalid (total_before_coins=%d shipping=%d)", escrowAmount, order.ShippingTotal.Int64())
	}
	if input.RequestedAmount <= 0 {
		return nil, fmt.Errorf("requested amount must be positive")
	}
	if input.RequestedAmount > escrowAmount {
		return nil, fmt.Errorf("requested amount cannot exceed escrow amount")
	}

	// Create refund entity
	refund := entity.NewRefund(
		orderID,
		order.BuyerID,
		order.SellerID,
		reason,
		input.Description,
		input.RequestedAmount,
	)

	// Persist refund
	if err := s.refundRepo.Create(ctx, tx, refund); err != nil {
		return nil, fmt.Errorf("failed to create refund: %w", err)
	}

	// Create evidence attachments if provided
	for _, evidenceURL := range input.EvidenceURLs {
		if err := s.refundRepo.CreateEvidence(ctx, tx, refund.ID, evidenceURL); err != nil {
			return nil, fmt.Errorf("failed to create refund evidence: %w", err)
		}
	}

	// Emit outbox event
	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id": refund.ID,
		"order_id":  orderID,
		"buyer_id":  order.BuyerID,
		"seller_id": order.SellerID,
		"reason":    string(refund.Reason),
		"status":    string(refund.Status),
		"amount":    refund.RequestedAmount,
		"opened_at": refund.OpenedAt,
	})

	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"refund.opened",
		refund.ID,
		payload,
	); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return refund, nil
}

// GetRefund retrieves a refund by ID.
func (s *RefundService) GetRefund(
	ctx context.Context,
	tx db.Tx,
	refundID uuid.UUID,
) (*entity.Refund, error) {
	return s.refundRepo.GetByID(ctx, tx, refundID)
}

// GetRefundByOrderID retrieves a refund by order ID.
func (s *RefundService) GetRefundByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*entity.Refund, error) {
	return s.refundRepo.GetByOrderID(ctx, tx, orderID)
}

// ListRefundHistoryByOrderID retrieves the canonical refund history for an order.
func (s *RefundService) ListRefundHistoryByOrderID(
	ctx context.Context, tx db.Tx, orderID uuid.UUID, limit int, cursor *RefundCursor,
) (*RefundHistoryPage, error) {
	if limit <= 0 {
		limit = 20
	}
	refunds, err := s.refundRepo.ListByOrderID(ctx, tx, orderID, limit+1, cursor)
	if err != nil {
		return nil, err
	}
	hasMore := len(refunds) > limit
	if hasMore {
		refunds = refunds[:limit]
	}
	var nextCursor *RefundCursor
	if hasMore && len(refunds) > 0 {
		last := refunds[len(refunds)-1]
		nextCursor = &RefundCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return &RefundHistoryPage{Items: refunds, NextCursor: nextCursor, HasMore: hasMore}, nil
}

// EscalateToDispute escalates a rejected refund to a dispute.
//
// This is called when the buyer opens a dispute after seller rejection.
// The refund status changes to escalated_to_admin.
//
// OWNERSHIP: callerID must be the buyer of the order associated with the refund.
//
// Transaction flow:
// 1. Lock and get refund for update
// 2. Validate buyer ownership
// 3. Update refund status to escalated_to_admin
// 4. Load evidence for dispute creation
// 5. Emit outbox event
//
// Returns the refund (with EvidenceURLs populated) so the caller can
// create the linked dispute using the refund's context.
func (s *RefundService) EscalateToDispute(
	ctx context.Context,
	tx db.Tx,
	refundID uuid.UUID,
	callerID uuid.UUID,
) (*entity.Refund, error) {
	// Get refund with lock
	refund, err := s.refundRepo.GetForUpdate(ctx, tx, refundID)
	if err != nil {
		return nil, fmt.Errorf("refund not found: %w", err)
	}

	// Buyer ownership check
	if refund.BuyerID != callerID {
		return nil, fmt.Errorf("only the buyer can escalate this refund")
	}

	// Escalate the refund
	now := time.Now()
	if err := refund.EscalateToAdmin(now); err != nil {
		return nil, fmt.Errorf("failed to escalate refund: %w", err)
	}

	// Update refund record
	if err := s.refundRepo.Update(ctx, tx, refund); err != nil {
		return nil, fmt.Errorf("failed to update refund: %w", err)
	}

	// Load evidence so the handler can forward it to dispute creation
	evidence, _ := s.refundRepo.ListEvidence(ctx, tx, refund.ID)
	refund.EvidenceURLs = evidence

	// Emit outbox event (D1A: buyer_id/seller_id enrichment for notification handler)
	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id":    refund.ID,
		"order_id":     refund.OrderID,
		"buyer_id":     refund.BuyerID,
		"seller_id":    refund.SellerID,
		"status":       string(refund.Status),
		"escalated_at": now,
	})

	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"refund.escalated",
		refund.ID,
		payload,
	); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return refund, nil
}

// AdminResolveRefundDecision records the ADMIN's FINAL decision on the order's
// existing refund process, and for a buyer-wins decision dispatches the gateway
// refund on that SAME refund row.
//
// CANONICAL B1: Admin may ONLY decide on refunds that were explicitly escalated
// to admin (escalated_to_admin). The escalation is the buyer's explicit action
// after seller rejection — via POST /refunds/:id/escalate — which transitions
// seller_rejected → escalated_to_admin AND opens a linked dispute in a single
// atomic flow. A generic dispute opening does NOT grant admin authority over a
// refund that is still seller_rejected or pending_seller_review.
//
// CANONICAL: one refund process = one refund row. The admin decision is written
// back onto the row that was escalated, so the original request can never be
// left looking unresolved while the money moves. Financial settlement stays on
// the separate gateway axis and never rewrites the decision.
//
// Returns recorded=false when the order has NO refund process (a direct dispute
// that never went through a refund request). There is then no decision to record
// and no negotiation row to attach it to; the caller creates the platform refund
// record (system_refunded) instead.
//
// FAILS CLOSED when the refund is not escalated_to_admin: the admin must not
// be able to decide on a refund that the buyer has not explicitly escalated.
//
// Idempotent: when the decision is already final this is a no-op and returns
// recorded=true.
func (s *RefundService) AdminResolveRefundDecision(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	adminID uuid.UUID,
	buyerWins bool,
	amount int64,
	notes *string,
) (bool, error) {
	existing, err := s.refundRepo.GetByOrderID(ctx, tx, orderID)
	if err != nil {
		return false, fmt.Errorf("admin refund decision: load refund process: %w", err)
	}
	if existing == nil {
		return false, nil
	}
	if existing.IsDecisionFinal() {
		return true, nil
	}

	refund, err := s.refundRepo.GetForUpdate(ctx, tx, existing.ID)
	if err != nil {
		return false, fmt.Errorf("admin refund decision: lock refund: %w", err)
	}
	if refund.IsDecisionFinal() {
		return true, nil
	}

	now := time.Now()

	// CANONICAL B1: Admin may ONLY decide on refunds that were explicitly
	// escalated to admin (escalated_to_admin). The escalation is the buyer's
	// explicit action after seller rejection — via POST /refunds/:id/escalate —
	// which transitions seller_rejected → escalated_to_admin AND opens a linked
	// dispute in a single atomic flow.
	//
	// A generic dispute opening (support, seller, or buyer via OpenDispute)
	// does NOT grant admin authority over a refund that is still
	// seller_rejected or pending_seller_review. The admin decision must be
	// blocked (fail closed) when the refund has not been explicitly escalated.
	if refund.Status != entity.RefundStatusEscalatedToAdmin {
		return false, fmt.Errorf("admin refund decision: refund must be escalated_to_admin, current status: %s", refund.Status)
	}

	if buyerWins {
		if err := refund.AdminRefund(adminID, amount, notes, now); err != nil {
			return false, fmt.Errorf("admin refund decision: %w", err)
		}
	} else {
		if err := refund.AdminRelease(adminID, notes, now); err != nil {
			return false, fmt.Errorf("admin release decision: %w", err)
		}
	}

	if err := s.refundRepo.Update(ctx, tx, refund); err != nil {
		return false, fmt.Errorf("admin refund decision: persist: %w", err)
	}
	if err := s.emitAdminDecisionOutbox(ctx, tx, refund); err != nil {
		return false, err
	}

	if !buyerWins {
		// Seller wins: the decision is final and no money moves to the buyer.
		// The order side releases escrow inside the same transaction.
		return true, nil
	}

	// Dispatch the gateway refund on the SAME row that carries the decision.
	callerType := GatewayRefundCallerTypeAdmin
	if auth.IsSystemCaller(adminID) {
		callerType = GatewayRefundCallerTypeSystem
	}
	if _, err := s.InitiateGatewayRefund(ctx, tx, InitiateGatewayRefundInput{
		RefundID:       refund.ID,
		Amount:         amount,
		Reason:         string(refund.Reason),
		IdempotencyKey: fmt.Sprintf("admin_decision_%s", refund.ID.String()),
		CallerID:       adminID,
		CallerType:     callerType,
	}); err != nil {
		return false, fmt.Errorf("admin refund decision: gateway dispatch: %w", err)
	}

	return true, nil
}

// HasFinalRefundDecisionOwedToBuyer reports whether the order's refund process
// already carries a FINAL decision that owes the buyer money — seller ACCEPT,
// admin buyer-wins, or a platform-initiated refund.
//
// This is the refund domain's answer to the order side's question "may escrow
// still be released to the seller?". Once such a decision exists it is FINAL
// (locked business truth), so paying the seller as well would move the same
// money twice and contradict a decision the buyer is entitled to rely on.
//
// Settlement state is deliberately NOT part of this predicate: the decision is
// final the moment it is recorded, whether or not the gateway has landed it yet.
func (s *RefundService) HasFinalRefundDecisionOwedToBuyer(ctx context.Context, tx db.Tx, orderID uuid.UUID) (bool, error) {
	existing, err := s.refundRepo.GetByOrderID(ctx, tx, orderID)
	if err != nil {
		return false, fmt.Errorf("load refund process: %w", err)
	}
	return existing != nil && existing.IsDecisionFinal() && existing.OwesBuyerRefund(), nil
}

// emitAdminDecisionOutbox emits the canonical audit event for a final admin
// decision on an escalated refund process.
func (s *RefundService) emitAdminDecisionOutbox(ctx context.Context, tx db.Tx, refund *entity.Refund) error {
	eventType := "refund.admin_released"
	if refund.Status == entity.RefundStatusAdminRefunded {
		eventType = "refund.admin_refunded"
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id":       refund.ID,
		"order_id":        refund.OrderID,
		"buyer_id":        refund.BuyerID,
		"seller_id":       refund.SellerID,
		"status":          string(refund.Status),
		"reviewed_by":     refund.ReviewedBy,
		"admin_reviewed_at": refund.AdminReviewedAt,
	})
	if err := s.outboxRepo.InsertEvent(ctx, tx, eventType, refund.ID, payload); err != nil {
		return fmt.Errorf("failed to insert %s outbox event: %w", eventType, err)
	}
	return nil
}

// ApproveRefund processes a seller's approval of a refund request.
//
// OWNERSHIP: Actor must be the seller of the order associated with the refund.
// STATUS GUARD: Refund must be in pending_seller_review status.
// POLICY GUARD: Refund reason must be seller-approvable per canonical policy.
//
//	Reasons requiring admin review (item_not_as_described, delivery_delay,
//	change_of_mind, other) return ErrAdminReviewRequired.
//
// On approval:
//   - Refund amount is SYSTEM-COMPUTED from policy (not buyer's requested_amount)
//   - Refund status transitions to seller_approved
//   - Seller decision fields are populated (amount, notes, reviewed_at)
//   - Gateway refund is dispatched via the canonical gateway refund path
//   - "refund.approved" outbox event is emitted
//
// Policy table:
//   - item_damaged / defective_item → product_only (DiscountedProduct = PD)
//   - item_not_received / wrong_item → full (PD + S = total_before_coins_amount)
//   - item_not_as_described / delivery_delay / change_of_mind / other → blocked
//
// Caller must provide an active transaction.
func (s *RefundService) ApproveRefund(
	ctx context.Context,
	tx db.Tx,
	refundID uuid.UUID,
	sellerID uuid.UUID,
	input ApproveRefundInput,
) (*entity.Refund, error) {
	// CANONICAL LOCK ORDER: ORDER → REFUND (never REFUND → ORDER).
	// Lock order first for policy resolution, then lock refund for state transition.
	// This prevents deadlock with RefundFromDispute/ReleaseFromDispute which lock ORDER → REFUND.

	// Read refund without lock to get orderID for canonical lock ordering
	refundRead, err := s.refundRepo.GetByID(ctx, tx, refundID)
	if err != nil {
		return nil, fmt.Errorf("refund not found: %w", err)
	}
	if refundRead == nil {
		return nil, fmt.Errorf("refund not found: %w", err)
	}

	// Ownership check: caller must be the seller of the order
	if refundRead.SellerID != sellerID {
		return nil, fmt.Errorf("only the seller of this order can approve the refund")
	}

	// Lock order first (canonical ORDER → REFUND order)
	order, err := s.orderRepo.GetForUpdate(ctx, tx, refundRead.OrderID)
	if err != nil {
		return nil, fmt.Errorf("order not found for policy resolution: %w", err)
	}

	// Now lock refund for state transition (after order lock acquired)
	refund, err := s.refundRepo.GetForUpdate(ctx, tx, refundID)
	if err != nil {
		return nil, fmt.Errorf("refund not found: %w", err)
	}

	// Resolve canonical refund policy from reason + order snapshot.
	//
	// CANONICAL PD: OrderSnapshot.DiscountedProduct is the DISCOUNTED product
	// value (PD), never the order's undiscounted Subtotal (P). PD is derived
	// from the persisted buyer-funded base (total_before_coins_amount - S).
	// FAIL CLOSED: no fallback to P.
	if !order.HasCanonicalMoneyBase() {
		return nil, fmt.Errorf("cannot approve refund: canonical money base invalid (total_before_coins=%d shipping=%d) — no P fallback", order.TotalBeforeCoinsAmount.Int64(), order.ShippingTotal.Int64())
	}
	orderSnap := entity.OrderSnapshot{
		DiscountedProduct: order.DiscountedProductAmount().Int64(),
		ShippingTotal:    order.ShippingTotal.Int64(),
		CommissionAmount: order.CommissionAmount.Int64(),
	}
	policy := entity.ResolveRefundPolicy(refund.Reason, orderSnap)

	// POLICY GUARD: block seller auto-approve for admin-review reasons
	if !policy.IsSellerApprovable() {
		return nil, &entity.ErrAdminReviewRequired{Reason: refund.Reason}
	}

	// System-computed amounts from policy — seller cannot influence these.
	// S2C2: CashRefund = Rpd+Rs, excludes C and F. Product/shipping split stamped on refund row.
	approvedAmount := policy.CashRefund
	rpd := policy.ProductAmount
	rs := policy.ShippingAmount

	// Transition status via entity state machine
	now := time.Now()
	if err := refund.SellerApprove(approvedAmount, input.Notes, now); err != nil {
		return nil, fmt.Errorf("failed to approve refund: %w", err)
	}

	// Stamp canonical product/shipping split.
	refund.RefundedProductAmount = &rpd
	refund.RefundedShippingAmount = &rs

	// Persist the status change
	if err := s.refundRepo.Update(ctx, tx, refund); err != nil {
		return nil, fmt.Errorf("failed to update refund: %w", err)
	}

	// Emit refund.approved outbox event
	orderGross := orderSnap.ProductGross() // PD+S, excludes C
	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id":          refund.ID,
		"order_id":           refund.OrderID,
		"buyer_id":           refund.BuyerID,
		"seller_id":          refund.SellerID,
		"status":             string(refund.Status),
		"approved_amount":    approvedAmount,
		"policy_type":        string(policy.PolicyType),
		"seller_reviewed_at": now,
	})
	if err := s.outboxRepo.InsertEvent(ctx, tx, "refund.approved", refund.ID, payload); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	// Dispatch gateway refund using the canonical path.
	// The amount dispatched is the policy-computed amount, NOT buyer's requested_amount.
	idempotencyKey := fmt.Sprintf("seller_approve_%s", refund.ID)
	_, dispatchErr := s.CreateAndDispatchSystemRefundFromApproval(ctx, tx, refund, orderGross, idempotencyKey)
	if dispatchErr != nil {
		return nil, fmt.Errorf("gateway refund dispatch after seller approval: %w", dispatchErr)
	}

	// Reload to capture gateway state mutations
	refreshed, err := s.refundRepo.GetByID(ctx, tx, refund.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload refund: %w", err)
	}
	if refreshed != nil {
		refund = refreshed
	}

	return refund, nil
}

// CreateAndDispatchSystemRefundFromApproval dispatches a gateway refund for a
// seller-approved refund. Unlike CreateAndDispatchSystemRefund (which creates
// a NEW refund row in admin_refunded state), this reuses the existing refund
// row — the row was created by the buyer and approved by the seller.
//
// CRITICAL: The amount dispatched is SellerApprovedAmount (policy-computed),
// NOT the buyer's RequestedAmount. This ensures the refund amount is always
// determined by the canonical refund policy resolver.
func (s *RefundService) CreateAndDispatchSystemRefundFromApproval(
	ctx context.Context,
	tx db.Tx,
	refund *entity.Refund,
	orderGross int64,
	idempotencyKey string,
) (*entity.Refund, error) {
	if s.gatewayClient == nil {
		return nil, ErrGatewayClientNotConfigured
	}

	// Use policy-computed amount (SellerApprovedAmount), not buyer's claim
	dispatchAmount := refund.RequestedAmount // fallback for legacy
	if refund.SellerApprovedAmount != nil {
		dispatchAmount = *refund.SellerApprovedAmount
	}

	return s.InitiateGatewayRefund(ctx, tx, InitiateGatewayRefundInput{
		RefundID:       refund.ID,
		Amount:         dispatchAmount,
		Reason:         string(refund.Reason),
		IdempotencyKey: idempotencyKey,
		CallerID:       auth.SystemCallerID,
		CallerType:     GatewayRefundCallerTypeSystem,
	})
}

// RejectRefund processes a seller's rejection of a refund request.
//
// OWNERSHIP: Actor must be the seller of the order associated with the refund.
// STATUS GUARD: Refund must be in pending_seller_review status.
//
// On rejection:
//   - Refund status transitions to seller_rejected
//   - Seller decision fields are populated (notes, reviewed_at)
//   - NO money movement occurs
//   - "refund.rejected" outbox event is emitted
//   - Buyer may then escalate to dispute via EscalateToDispute
//
// Caller must provide an active transaction.
func (s *RefundService) RejectRefund(
	ctx context.Context,
	tx db.Tx,
	refundID uuid.UUID,
	sellerID uuid.UUID,
	input RejectRefundInput,
) (*entity.Refund, error) {
	// Lock refund row
	refund, err := s.refundRepo.GetForUpdate(ctx, tx, refundID)
	if err != nil {
		return nil, fmt.Errorf("refund not found: %w", err)
	}

	// Ownership check: caller must be the seller of the order
	if refund.SellerID != sellerID {
		return nil, fmt.Errorf("only the seller of this order can reject the refund")
	}

	// Transition status via entity state machine
	now := time.Now()
	if err := refund.SellerReject(input.Notes, now); err != nil {
		return nil, fmt.Errorf("failed to reject refund: %w", err)
	}

	// Persist the status change
	if err := s.refundRepo.Update(ctx, tx, refund); err != nil {
		return nil, fmt.Errorf("failed to update refund: %w", err)
	}

	// Emit refund.rejected outbox event
	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id":          refund.ID,
		"order_id":           refund.OrderID,
		"buyer_id":           refund.BuyerID,
		"seller_id":          refund.SellerID,
		"status":             string(refund.Status),
		"seller_reviewed_at": now,
	})
	if err := s.outboxRepo.InsertEvent(ctx, tx, "refund.rejected", refund.ID, payload); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return refund, nil
}

// isValidRefundReason checks if the reason is valid.
func isValidRefundReason(reason entity.RefundReason) bool {
	switch reason {
	case entity.RefundReasonItemNotReceived,
		entity.RefundReasonItemNotAsDescribed,
		entity.RefundReasonItemDamaged,
		entity.RefundReasonDefectiveItem,
		entity.RefundReasonWrongItem,
		entity.RefundReasonChangeOfMind,
		entity.RefundReasonDeliveryDelay,
		entity.RefundReasonOther:
		return true
	default:
		return false
	}
}
