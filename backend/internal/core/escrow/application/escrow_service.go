// Package application contains the canonical EscrowService — the single
// authority for escrow lifecycle operations in the gateway-funded model.
//
// This package contains NO Wallet concepts. Wallet is forbidden legacy;
// escrow is canonical operational state. All money movement is double-entry
// in the finance ledger, written by the caller via FinanceService in the
// same transaction — this service only flips the escrow row's status
// (replay-safe).
package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/core/escrow/entity"
	infraEscrowRepo "github.com/labuda/backend/internal/core/escrow/infrastructure/repository"
	escrowrepo "github.com/labuda/backend/internal/core/escrow/repository"
	disputeRepo "github.com/labuda/backend/internal/governance/dispute/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// EscrowService handles escrow business logic under the canonical
// gateway-funded escrow model.
//
// OPERATIONS:
//   - CreateEscrowFromGatewaySettlement: insert escrow at gateway settlement
//   - GetEscrowForOrder: read escrow state for an order
//   - ReleaseGatewayEscrow: holding → released (status flip only)
//   - RefundGatewayEscrow: holding → refunded (status flip only)
//   - PartialRefundGatewayEscrow: holding → released (split between buyer/seller)
//
// DISPUTE INTEGRATION:
//   - Blocks release/refund when an active under-review dispute exists
//   - Dispute resolution triggers escrow flips via OrderCompletionService
type EscrowService struct {
	escrowRepo  escrowrepo.EscrowRepository
	disputeRepo disputeRepo.DisputeRepository
	db          *db.DB
	logger      *zap.Logger
}

// NewEscrowService creates a new EscrowService.
func NewEscrowService(database *db.DB, logger *zap.Logger) *EscrowService {
	return &EscrowService{
		escrowRepo:  infraEscrowRepo.NewEscrowRepository(),
		disputeRepo: nil, // Set via SetDisputeRepository to avoid circular dependency
		db:          database,
		logger:      logger,
	}
}

// SetDisputeRepository sets the dispute repository.
// This is done separately to avoid circular dependency issues during initialization.
func (s *EscrowService) SetDisputeRepository(repo disputeRepo.DisputeRepository) {
	s.disputeRepo = repo
}

// SetEscrowRepository replaces the escrow repository.
// Used by tests that need to inject a spy/mock without a real DB.
func (s *EscrowService) SetEscrowRepository(repo escrowrepo.EscrowRepository) {
	s.escrowRepo = repo
}

// ============================================================================
// DISPUTE GUARD
// ============================================================================

// ErrEscrowLockedByDispute is returned when an escrow operation is blocked
// due to an active dispute on the order.
type ErrEscrowLockedByDispute struct {
	OrderID       uuid.UUID
	DisputeID     uuid.UUID
	DisputeStatus string
}

func (e *ErrEscrowLockedByDispute) Error() string {
	return fmt.Sprintf("escrow locked due to active dispute on order %s (dispute_id=%s, status=%s)",
		e.OrderID, e.DisputeID, e.DisputeStatus)
}

// ErrDisputeRepositoryNotConfigured is returned when the dispute repository
// is not configured but an escrow operation requires dispute checking.
// This is a FAIL-CLOSED safety mechanism to prevent bypassing dispute checks.
type ErrDisputeRepositoryNotConfigured struct {
	Operation string
	OrderID   uuid.UUID
}

func (e *ErrDisputeRepositoryNotConfigured) Error() string {
	return fmt.Sprintf("dispute repository not configured for operation %s on order %s: BLOCKING for financial safety - dispute integration must be enabled",
		e.Operation, e.OrderID)
}

// ErrDisputeCheckFailed is returned when the dispute check fails due to
// a database error or other system error. This is a FAIL-CLOSED safety mechanism.
type ErrDisputeCheckFailed struct {
	OrderID uuid.UUID
	Err     error
}

func (e *ErrDisputeCheckFailed) Error() string {
	return fmt.Sprintf("dispute check failed for order %s: BLOCKING for financial safety - %v", e.OrderID, e.Err)
}

// checkActiveDispute checks if there's an active dispute on the order.
// Returns an error if a dispute exists and is in under_review status.
//
// FAIL-CLOSED BEHAVIOR:
//   - If disputeRepo is not configured: BLOCKS operation (returns error)
//   - If dispute check fails with DB error: BLOCKS operation (returns error)
//   - Financial safety > availability
//
// This is called BEFORE any escrow release/refund operation to ensure
// disputes control escrow outcome.
func (s *EscrowService) checkActiveDispute(ctx context.Context, tx db.Tx, orderID uuid.UUID) error {
	// FAIL-CLOSED: If dispute repository is not set, BLOCK the operation
	// This prevents bypassing dispute checks when dispute integration is not properly configured.
	if s.disputeRepo == nil {
		s.logger.Error("escrow_dispute_repo_missing_blocked",
			zap.String("order_id", orderID.String()),
			zap.String("severity", "critical"),
			zap.String("action", "operation_blocked"),
			zap.String("reason", "dispute_repository_not_configured"),
		)
		return &ErrDisputeRepositoryNotConfigured{
			Operation: "escrow_operation",
			OrderID:   orderID,
		}
	}

	// Query dispute by order ID
	dispute, err := s.disputeRepo.GetByOrderID(ctx, tx, orderID)
	if err != nil {
		// FAIL-CLOSED: Block on database error to prevent unsafe operations
		// Financial correctness is more important than availability
		s.logger.Error("escrow_dispute_check_failed_blocked",
			zap.String("order_id", orderID.String()),
			zap.String("severity", "critical"),
			zap.String("action", "operation_blocked"),
			zap.String("reason", "dispute_check_database_error"),
			zap.Error(err),
		)
		return &ErrDisputeCheckFailed{
			OrderID: orderID,
			Err:     err,
		}
	}

	// No dispute found - allow operation
	if dispute == nil {
		return nil
	}

	// Check if dispute is active (under_review)
	// Using the IsUnderReview() method from the dispute entity for consistency
	if dispute.IsUnderReview() {
		s.logger.Warn("escrow_blocked_by_dispute",
			zap.String("order_id", orderID.String()),
			zap.String("dispute_id", dispute.ID.String()),
			zap.String("dispute_status", string(dispute.Status)),
			zap.String("action", "escrow_operation_blocked"),
			zap.String("severity", "blocked"),
		)
		return &ErrEscrowLockedByDispute{
			OrderID:       orderID,
			DisputeID:     dispute.ID,
			DisputeStatus: string(dispute.Status),
		}
	}

	// Dispute is resolved (resolved_refund or resolved_release)
	// Allow operation to proceed for idempotency
	s.logger.Info("escrow_dispute_resolved",
		zap.String("order_id", orderID.String()),
		zap.String("dispute_id", dispute.ID.String()),
		zap.String("dispute_status", string(dispute.Status)),
	)
	return nil
}

// ============================================================================
// GATEWAY-FUNDED ESCROW (PAYMENT SETTLEMENT)
// ============================================================================

// CreateEscrowFromGatewaySettlementInput contains parameters for creating a
// gateway-funded escrow at payment settlement time.
type CreateEscrowFromGatewaySettlementInput struct {
	OrderID   uuid.UUID
	BuyerID   uuid.UUID
	SellerID  uuid.UUID
	Amount    int64     // payment.gross_amount (already validated upstream)
	PaymentID uuid.UUID // for log/audit only
}

// CreateEscrowFromGatewaySettlement records an internal escrow representing
// platform-held obligation for an order whose payment has settled at the
// external gateway (Midtrans).
//
// MODEL: This is NOT a buyer wallet hold. Buyer has no internal balance.
// Money lives at the gateway. The escrow row represents the platform's
// obligation to either release to seller (Complete) or refund (Cancel/Dispute).
//
// INVARIANTS:
//   - DOES NOT mutate any user balance
//   - DOES NOT create ledger entries
//   - Idempotent on existing escrow for same order_id (returns existing row)
//
// IMPORTANT: Must be called within the same transaction as the payment
// settlement and the order MarkPaid call.
func (s *EscrowService) CreateEscrowFromGatewaySettlement(
	ctx context.Context,
	tx db.Tx,
	input CreateEscrowFromGatewaySettlementInput,
) (*entity.Escrow, error) {
	if input.OrderID == uuid.Nil {
		return nil, fmt.Errorf("order_id cannot be nil")
	}
	if input.BuyerID == uuid.Nil {
		return nil, fmt.Errorf("buyer_id cannot be nil")
	}
	if input.SellerID == uuid.Nil {
		return nil, fmt.Errorf("seller_id cannot be nil")
	}
	if input.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive: got %d", input.Amount)
	}

	// IDEMPOTENCY GUARD: existing escrow → return as success.
	existing, err := s.escrowRepo.GetByOrderID(ctx, tx, input.OrderID)
	if err != nil {
		s.logger.Error("escrow_gateway_create_check_failed",
			zap.String("order_id", input.OrderID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to check existing escrow: %w", err)
	}
	if existing != nil {
		s.logger.Info("escrow_gateway_created_idempotent",
			zap.String("order_id", input.OrderID.String()),
			zap.String("escrow_id", existing.ID.String()),
		)
		return existing, nil
	}

	// Build escrow row.
	escrow, err := entity.NewEscrow(input.OrderID, input.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to build escrow entity: %w", err)
	}

	if err := s.escrowRepo.Create(ctx, tx, escrow); err != nil {
		// Race: another concurrent webhook may have inserted between our
		// idempotency read and this insert. Read back and return.
		var alreadyExists *entity.ErrEscrowAlreadyExists
		if errors.As(err, &alreadyExists) {
			row, getErr := s.escrowRepo.GetByOrderID(ctx, tx, input.OrderID)
			if getErr != nil {
				return nil, fmt.Errorf("escrow already exists but read-back failed: %w", getErr)
			}
			s.logger.Info("escrow_gateway_race_idempotent",
				zap.String("order_id", input.OrderID.String()),
				zap.String("escrow_id", row.ID.String()),
			)
			return row, nil
		}
		s.logger.Error("escrow_gateway_create_failed",
			zap.String("order_id", input.OrderID.String()),
			zap.String("payment_id", input.PaymentID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create gateway escrow: %w", err)
	}

	s.logger.Info("escrow_gateway_created",
		zap.String("order_id", input.OrderID.String()),
		zap.String("escrow_id", escrow.ID.String()),
		zap.Int64("amount", input.Amount),
		zap.String("payment_id", input.PaymentID.String()),
	)

	return escrow, nil
}

// GetEscrowForOrder retrieves the escrow for an order.
//
// This is a READ-ONLY operation used to derive Order.EscrowStatus from escrow state.
// Returns nil if escrow not found (order hasn't been paid yet).
//
// CRITICAL: This method does NOT modify any state.
// It is ONLY used to check the current escrow state for deriving Order.EscrowStatus.
//
// IMPORTANT: This method MUST be called within a transaction.
// The caller is responsible for beginning and committing the transaction.
func (s *EscrowService) GetEscrowForOrder(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Escrow, error) {
	if orderID == uuid.Nil {
		return nil, fmt.Errorf("order_id cannot be nil")
	}

	escrow, err := s.escrowRepo.GetByOrderID(ctx, tx, orderID)
	if err != nil {
		s.logger.Error("escrow_get_failed",
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get escrow for order: %w", err)
	}

	// Return nil if escrow not found (order hasn't been paid yet)
	// This is expected behavior for unpaid orders
	return escrow, nil
}

// ============================================================================
// GATEWAY-FUNDED ESCROW LIFECYCLE — Release / Refund / PartialRefund
//
// Under the gateway-funded model, escrow funds physically live at the platform
// clearing account at the payment gateway. Buyer has NO internal balance to
// debit at settlement or credit at refund. The primitives below only flip the
// escrow row's status (replay-safe); the canonical money movement is
// double-entry in the finance ledger, written by the caller via
// FinanceService.RecordOrderRelease / RecordRefundReversal in the same tx.
// ============================================================================

// ReleaseGatewayEscrow transitions a holding escrow to "released" without any
// balance mutation. Caller is responsible for the matching ledger entry via
// FinanceService.RecordOrderRelease in the same tx.
//
// PRECONDITIONS:
//   - escrow.status == "holding"
//   - escrow.amount == expectedGross (defends against amount drift between
//     order pricing snapshot and escrow row)
//   - no active dispute on the order
//
// IDEMPOTENCY:
//   - if escrow already released, returns (escrow, false, nil)
//   - if escrow already refunded or in any other terminal state, returns error
//
// RETURNS:
//   - escrow: the escrow row (post-update on success)
//   - newlyReleased: true if this call performed the release, false if it was
//     already released (idempotent)
func (s *EscrowService) ReleaseGatewayEscrow(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	expectedGross int64,
) (*entity.Escrow, bool, error) {
	if orderID == uuid.Nil {
		return nil, false, fmt.Errorf("order_id cannot be nil")
	}

	escrow, err := s.escrowRepo.GetByOrderIDForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load escrow for update: %w", err)
	}
	if escrow == nil {
		return nil, false, fmt.Errorf("escrow not found for order: %s", orderID.String())
	}

	if escrow.Status == entity.EscrowStatusReleased {
		s.logger.Info("escrow_release_gateway_idempotent",
			zap.String("order_id", orderID.String()),
			zap.String("escrow_id", escrow.ID.String()),
		)
		return escrow, false, nil
	}
	if escrow.Status == entity.EscrowStatusRefunded {
		return nil, false, fmt.Errorf("cannot release escrow: already refunded (order_id=%s)", orderID.String())
	}
	if escrow.Status != entity.EscrowStatusHolding {
		return nil, false, fmt.Errorf("cannot release escrow: invalid status (current=%s, required=holding)", escrow.Status)
	}

	if escrow.Amount != expectedGross {
		return nil, false, fmt.Errorf("escrow amount mismatch (escrow=%d expected=%d) for order %s", escrow.Amount, expectedGross, orderID.String())
	}

	if err := s.checkActiveDispute(ctx, tx, orderID); err != nil {
		return nil, false, err
	}

	if err := escrow.Release(); err != nil {
		return nil, false, fmt.Errorf("failed to mark escrow released: %w", err)
	}
	if err := s.escrowRepo.Update(ctx, tx, escrow); err != nil {
		return nil, false, fmt.Errorf("failed to persist released escrow: %w", err)
	}

	s.logger.Info("escrow_release_gateway_success",
		zap.String("order_id", orderID.String()),
		zap.String("escrow_id", escrow.ID.String()),
		zap.Int64("amount", escrow.Amount),
	)
	return escrow, true, nil
}

// RefundGatewayEscrow transitions a holding escrow to "refunded" without any
// balance mutation. Caller is responsible for the matching ledger entry
// (FinanceService.RecordRefundReversal) and for orchestrating any gateway-side
// refund issuance (RefundService.InitiateGatewayRefund) where applicable.
//
// PRECONDITIONS:
//   - escrow.status == "holding"
//   - no active dispute on the order (callers driven by dispute resolution
//     enter with dispute already moved out of under_review)
//
// IDEMPOTENCY:
//   - if escrow already refunded, returns (escrow, false, nil)
//   - if escrow already released or in any other terminal state, returns error
//
// RETURNS:
//   - escrow: the escrow row (post-update on success)
//   - newlyRefunded: true if this call performed the refund, false if it was
//     already refunded (idempotent)
func (s *EscrowService) RefundGatewayEscrow(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*entity.Escrow, bool, error) {
	if orderID == uuid.Nil {
		return nil, false, fmt.Errorf("order_id cannot be nil")
	}

	escrow, err := s.escrowRepo.GetByOrderIDForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load escrow for update: %w", err)
	}
	if escrow == nil {
		return nil, false, fmt.Errorf("escrow not found for order: %s", orderID.String())
	}

	if escrow.Status == entity.EscrowStatusRefunded {
		s.logger.Info("escrow_refund_gateway_idempotent",
			zap.String("order_id", orderID.String()),
			zap.String("escrow_id", escrow.ID.String()),
		)
		return escrow, false, nil
	}
	if escrow.Status == entity.EscrowStatusReleased {
		return nil, false, fmt.Errorf("cannot refund escrow: already released (order_id=%s)", orderID.String())
	}
	if escrow.Status != entity.EscrowStatusHolding {
		return nil, false, fmt.Errorf("cannot refund escrow: invalid status (current=%s, required=holding)", escrow.Status)
	}

	if err := s.checkActiveDispute(ctx, tx, orderID); err != nil {
		return nil, false, err
	}

	if err := escrow.Refund(); err != nil {
		return nil, false, fmt.Errorf("failed to mark escrow refunded: %w", err)
	}
	if err := s.escrowRepo.Update(ctx, tx, escrow); err != nil {
		return nil, false, fmt.Errorf("failed to persist refunded escrow: %w", err)
	}

	s.logger.Info("escrow_refund_gateway_success",
		zap.String("order_id", orderID.String()),
		zap.String("escrow_id", escrow.ID.String()),
		zap.Int64("amount", escrow.Amount),
	)
	return escrow, true, nil
}

// PartialRefundGatewayEscrow transitions a holding escrow to "released"
// (terminal) when a portion of the escrow is refunded to the buyer and the
// remainder released to the seller (e.g., partial dispute resolution where
// the buyer keeps shipping and refunds the item). No balance mutation; caller
// is responsible for both ledger entries (refund reversal for the buyer
// portion and order release for the seller portion) in the same tx.
//
// The escrow terminates in RELEASED state because the seller portion is the
// remaining live obligation; the buyer-refund portion is accounted for in the
// ledger reversal.
//
// PRECONDITIONS:
//   - escrow.status == "holding"
//   - escrow.amount > 0 (sanity)
//   - refundAmount in (0, escrow.Amount) — strict partial
//   - no active dispute on the order
//
// IDEMPOTENCY: if escrow already released/refunded, returns (escrow, false, nil)
// or an error depending on the prior terminal state.
//
// RETURNS:
//   - escrow: the escrow row (post-update on success)
//   - newlyResolved: true if this call performed the resolution
func (s *EscrowService) PartialRefundGatewayEscrow(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	refundAmount int64,
) (*entity.Escrow, bool, error) {
	if orderID == uuid.Nil {
		return nil, false, fmt.Errorf("order_id cannot be nil")
	}
	if refundAmount <= 0 {
		return nil, false, fmt.Errorf("refund amount must be positive: got %d", refundAmount)
	}

	escrow, err := s.escrowRepo.GetByOrderIDForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load escrow for update: %w", err)
	}
	if escrow == nil {
		return nil, false, fmt.Errorf("escrow not found for order: %s", orderID.String())
	}

	if escrow.Status == entity.EscrowStatusReleased {
		s.logger.Info("escrow_partial_refund_gateway_idempotent",
			zap.String("order_id", orderID.String()),
			zap.String("escrow_id", escrow.ID.String()),
		)
		return escrow, false, nil
	}
	if escrow.Status == entity.EscrowStatusRefunded {
		return nil, false, fmt.Errorf("cannot partial refund escrow: already refunded (order_id=%s)", orderID.String())
	}
	if escrow.Status != entity.EscrowStatusHolding {
		return nil, false, fmt.Errorf("cannot partial refund escrow: invalid status (current=%s, required=holding)", escrow.Status)
	}
	if refundAmount >= escrow.Amount {
		return nil, false, fmt.Errorf("partial refund amount %d must be strictly less than escrow amount %d", refundAmount, escrow.Amount)
	}

	if err := s.checkActiveDispute(ctx, tx, orderID); err != nil {
		return nil, false, err
	}

	if err := escrow.Release(); err != nil {
		return nil, false, fmt.Errorf("failed to mark escrow released: %w", err)
	}
	if err := s.escrowRepo.Update(ctx, tx, escrow); err != nil {
		return nil, false, fmt.Errorf("failed to persist partially-refunded escrow: %w", err)
	}

	s.logger.Info("escrow_partial_refund_gateway_success",
		zap.String("order_id", orderID.String()),
		zap.String("escrow_id", escrow.ID.String()),
		zap.Int64("escrow_amount", escrow.Amount),
		zap.Int64("refund_amount", refundAmount),
		zap.Int64("seller_remainder", escrow.Amount-refundAmount),
	)
	return escrow, true, nil
}

// ============================================================================
// QUERY METHODS
// ============================================================================

// GetEscrowByOrderID retrieves the escrow for an order.
func (s *EscrowService) GetEscrowByOrderID(ctx context.Context, orderID uuid.UUID) (*entity.Escrow, error) {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	escrow, err := s.escrowRepo.GetByOrderID(ctx, tx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get escrow for update: %w", err)
	}

	return escrow, nil
}
