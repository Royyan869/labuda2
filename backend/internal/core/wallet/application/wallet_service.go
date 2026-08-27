package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/core/wallet/entity"
	infraWalletRepo "github.com/labuda/backend/internal/core/wallet/infrastructure/repository"
	walletrepo "github.com/labuda/backend/internal/core/wallet/repository"
	disputeRepo "github.com/labuda/backend/internal/governance/dispute/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

const (
	// WalletOrderReleaseDescription is the description for escrow release ledger entries.
	WalletOrderReleaseDescription = "Escrow release to seller"
	// WalletOrderRefundDescription is the description for escrow refund ledger entries.
	WalletOrderRefundDescription = "Escrow refund to buyer"

	// EnablePartialRefund enables partial refund escrow functionality.
	//
	// PARTIAL DISPUTE RESOLUTION ENABLED:
	// - Admin-only access control (via capability.CapFinanceDisputeResolve)
	// - Clear audit trail (admin ID and notes stored in dispute)
	// - Atomic operation (single transaction with wallet transfers)
	// - Idempotent (via wallet service idempotency)
	// - Proper notification (outbox events for all parties)
	//
	// USE CASE: Partial dispute resolution where:
	// - Buyer gets refund for item price (subtotal)
	// - Seller gets release for shipping fee (shipping_total)
	EnablePartialRefund = true

	// EnableInvariantChecks enables wallet-finance consistency assertions.
	// When true, critical operations will verify wallet and finance ledger are in sync.
	// If inconsistency is detected, the operation will fail with a critical error.
	//
	// PHASE 1: Start with false (monitoring only)
	// PHASE 2: Enable for staging environment
	// PHASE 3: Enable for production after validation
	EnableInvariantChecks = false
)

// ============================================================================
// FINANCE LEDGER INTEGRATION (INVARIANT ENFORCEMENT)
// ============================================================================

// FinanceLedgerRepository is a minimal interface for checking finance ledger balances.
// This allows WalletService to verify consistency without creating a hard dependency.
type FinanceLedgerRepository interface {
	// GetUserAccountID retrieves a user account by type and owner ID.
	GetUserAccountID(ctx context.Context, tx db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error)

	// GetAccountBalance retrieves the current balance of an account.
	GetAccountBalance(ctx context.Context, tx db.Tx, accountID uuid.UUID) (money.Money, error)

	// GetSystemAccountID retrieves a system account by type.
	GetSystemAccountID(ctx context.Context, tx db.Tx, accountType string) (uuid.UUID, error)
}

// Account type constants for finance ledger (mirror from finance domain)
const (
	FinanceAccountSellerPayable = "SELLER_PAYABLE"
)

// WalletService handles wallet business logic under the canonical
// gateway-funded escrow model.
//
// IMPLEMENTATION SCOPE:
// - GetOrCreateWallet: ensure user has a wallet row
// - CreateEscrowFromGatewaySettlement: insert escrow at gateway settlement
// - ReleaseGatewayEscrow: holding → released (status flip only)
// - RefundGatewayEscrow: holding → refunded (status flip only)
// - PartialRefundGatewayEscrow: holding → released (split between buyer/seller)
// - Invariant assertions (wallet vs finance consistency)
//
// All money movement is in the finance ledger, not in wallet balances —
// buyer.held_balance / seller.available_balance are NEVER mutated by the
// escrow lifecycle. The legacy wallet-hold methods (ReleaseEscrow /
// RefundEscrow / PartialRefundEscrow) have been demolished.
//
// DISPUTE INTEGRATION:
// - Blocks release/refund when an active under-review dispute exists
// - Dispute resolution triggers escrow flips via OrderCompletionService
type WalletService struct {
	walletRepo        walletrepo.WalletRepository
	escrowRepo        walletrepo.EscrowRepository
	disputeRepo       disputeRepo.DisputeRepository
	db                *db.DB
	logger            *zap.Logger
	financeLedgerRepo FinanceLedgerRepository
}

// NewWalletService creates a new WalletService.
func NewWalletService(db *db.DB, logger *zap.Logger) *WalletService {
	return &WalletService{
		walletRepo:  infraWalletRepo.NewWalletRepository(),
		escrowRepo:  infraWalletRepo.NewEscrowRepository(),
		disputeRepo: nil, // Set via SetDisputeRepository to avoid circular dependency
		db:          db,
		logger:      logger,
	}
}

// SetDisputeRepository sets the dispute repository.
// This is done separately to avoid circular dependency issues during initialization.
func (s *WalletService) SetDisputeRepository(repo disputeRepo.DisputeRepository) {
	s.disputeRepo = repo
}

// SetEscrowRepository replaces the escrow repository.
// Used by tests that need to inject a spy/mock without a real DB.
func (s *WalletService) SetEscrowRepository(repo walletrepo.EscrowRepository) {
	s.escrowRepo = repo
}

// SetFinanceLedgerRepository sets the finance ledger repository for consistency checks.
// Used by AssertWalletFinanceConsistency and CheckPayoutEligibility.
func (s *WalletService) SetFinanceLedgerRepository(repo FinanceLedgerRepository) {
	if repo == nil {
		s.logger.Panic("wallet_finance_ledger_repo_required",
			zap.String("severity", "critical"),
			zap.String("reason", "finance_ledger_repository_cannot_be_nil"),
			zap.String("requirement", "FinanceLedgerRepository is MANDATORY for withdrawal operations (PHASE 3.1)"),
		)
	}
	s.financeLedgerRepo = repo
	s.logger.Info("wallet_finance_ledger_repo_set",
		zap.String("status", "hardened"),
		zap.String("requirement", "finance_ledger_mandatory"),
	)
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
// - If disputeRepo is not configured: BLOCKS operation (returns error)
// - If dispute check fails with DB error: BLOCKS operation (returns error)
// - Financial safety > availability
//
// This is called BEFORE any escrow release/refund operation to ensure
// disputes control escrow outcome.
func (s *WalletService) checkActiveDispute(ctx context.Context, tx db.Tx, orderID uuid.UUID) error {
	// FAIL-CLOSED: If dispute repository is not set, BLOCK the operation
	// This prevents bypassing dispute checks when dispute integration is not properly configured.
	if s.disputeRepo == nil {
		s.logger.Error("wallet_dispute_repo_missing_blocked",
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
		s.logger.Error("wallet_dispute_check_failed_blocked",
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
		s.logger.Warn("wallet_escrow_blocked_by_dispute",
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
	s.logger.Info("wallet_dispute_resolved",
		zap.String("order_id", orderID.String()),
		zap.String("dispute_id", dispute.ID.String()),
		zap.String("dispute_status", string(dispute.Status)),
	)
	return nil
}

// ============================================================================
// WALLET MANAGEMENT
// ============================================================================

// GetOrCreateWallet ensures a user has a wallet.
// Creates a new wallet with zero balance if it doesn't exist.
// This is idempotent - safe to call multiple times.
func (s *WalletService) GetOrCreateWallet(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.Wallet, error) {
	wallet, err := s.walletRepo.EnsureWallet(ctx, tx, userID)
	if err != nil {
		s.logger.Error("wallet_ensure_failed",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to ensure wallet: %w", err)
	}

	s.logger.Debug("wallet_ensured",
		zap.String("user_id", userID.String()),
		zap.String("wallet_id", wallet.ID.String()),
		zap.Int64("available_balance", wallet.AvailableBalance),
		zap.Int64("held_balance", wallet.HeldBalance),
	)

	return wallet, nil
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
//   - DOES NOT mutate wallet.available_balance
//   - DOES NOT mutate wallet.held_balance
//   - DOES NOT create ledger entries
//   - Idempotent on existing escrow for same order_id (returns existing row)
//
// IMPORTANT: Must be called within the same transaction as the payment
// settlement and the order MarkPaid call.
func (s *WalletService) CreateEscrowFromGatewaySettlement(
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
		s.logger.Error("wallet_gateway_escrow_check_failed",
			zap.String("order_id", input.OrderID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to check existing escrow: %w", err)
	}
	if existing != nil {
		s.logger.Info("wallet_gateway_escrow_idempotent",
			zap.String("order_id", input.OrderID.String()),
			zap.String("escrow_id", existing.ID.String()),
		)
		return existing, nil
	}

	// Ensure buyer wallet row exists (balances stay 0 for gateway model).
	buyerWallet, err := s.walletRepo.EnsureWallet(ctx, tx, input.BuyerID)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure buyer wallet: %w", err)
	}

	// Ensure seller wallet row exists (balances stay 0 until Complete release).
	sellerWallet, err := s.walletRepo.EnsureWallet(ctx, tx, input.SellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure seller wallet: %w", err)
	}

	// Build escrow row.
	escrow, err := entity.NewEscrow(input.OrderID, buyerWallet.ID, input.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to build escrow entity: %w", err)
	}
	escrow.SetSellerWallet(sellerWallet.ID)

	if err := s.escrowRepo.Create(ctx, tx, escrow); err != nil {
		// Race: another concurrent webhook may have inserted between our
		// idempotency read and this insert. Read back and return.
		var alreadyExists *entity.ErrEscrowAlreadyExists
		if errors.As(err, &alreadyExists) {
			row, getErr := s.escrowRepo.GetByOrderID(ctx, tx, input.OrderID)
			if getErr != nil {
				return nil, fmt.Errorf("escrow already exists but read-back failed: %w", getErr)
			}
			s.logger.Info("wallet_gateway_escrow_race_idempotent",
				zap.String("order_id", input.OrderID.String()),
				zap.String("escrow_id", row.ID.String()),
			)
			return row, nil
		}
		s.logger.Error("wallet_gateway_escrow_create_failed",
			zap.String("order_id", input.OrderID.String()),
			zap.String("payment_id", input.PaymentID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create gateway escrow: %w", err)
	}

	s.logger.Info("wallet_gateway_escrow_created",
		zap.String("order_id", input.OrderID.String()),
		zap.String("escrow_id", escrow.ID.String()),
		zap.String("buyer_wallet_id", buyerWallet.ID.String()),
		zap.String("seller_wallet_id", sellerWallet.ID.String()),
		zap.Int64("amount", input.Amount),
		zap.String("payment_id", input.PaymentID.String()),
	)

	return escrow, nil
}

// GetEscrowForOrder retrieves the escrow for an order.
//
// This is a READ-ONLY operation used to derive Order.EscrowStatus from Wallet state.
// Returns nil if escrow not found (order hasn't been paid yet).
//
// CRITICAL: This method does NOT modify any state.
// It is ONLY used to check the current wallet escrow state for deriving Order.EscrowStatus.
//
// IMPORTANT: This method MUST be called within a transaction.
// The caller is responsible for beginning and committing the transaction.
func (s *WalletService) GetEscrowForOrder(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Escrow, error) {
	if orderID == uuid.Nil {
		return nil, fmt.Errorf("order_id cannot be nil")
	}

	escrow, err := s.escrowRepo.GetByOrderID(ctx, tx, orderID)
	if err != nil {
		s.logger.Error("wallet_escrow_get_failed",
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
// clearing account at the payment gateway. Buyer wallet is NOT debited at
// settlement and NOT credited at refund. The wallet-side primitives below
// only flip the escrow row's status (replay-safe); the canonical money
// movement is double-entry in the finance ledger, written by the caller via
// FinanceService.RecordOrderRelease / RecordRefundReversal in the same tx.
//
// The legacy wallet-hold ReleaseEscrow / RefundEscrow / PartialRefundEscrow
// methods (which mutated buyer.held_balance ↔ buyer/seller.available_balance)
// have been demolished. There is no fallback.
// ============================================================================

// ReleaseGatewayEscrow transitions a holding escrow to "released" without any
// wallet mutation. Caller is responsible for the matching ledger entry via
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
func (s *WalletService) ReleaseGatewayEscrow(
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
		s.logger.Info("wallet_release_gateway_idempotent",
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

	s.logger.Info("wallet_release_gateway_success",
		zap.String("order_id", orderID.String()),
		zap.String("escrow_id", escrow.ID.String()),
		zap.Int64("amount", escrow.Amount),
	)
	return escrow, true, nil
}

// RefundGatewayEscrow transitions a holding escrow to "refunded" without any
// wallet mutation. Caller is responsible for the matching ledger entry
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
func (s *WalletService) RefundGatewayEscrow(
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
		s.logger.Info("wallet_refund_gateway_idempotent",
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

	s.logger.Info("wallet_refund_gateway_success",
		zap.String("order_id", orderID.String()),
		zap.String("escrow_id", escrow.ID.String()),
		zap.Int64("amount", escrow.Amount),
	)
	return escrow, true, nil
}

// PartialRefundGatewayEscrow transitions a holding escrow to "released"
// (terminal) when a portion of the escrow is refunded to the buyer and the
// remainder released to the seller (e.g., partial dispute resolution where
// the buyer keeps shipping and refunds the item). No wallet mutation; caller
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
func (s *WalletService) PartialRefundGatewayEscrow(
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
		s.logger.Info("wallet_partial_refund_gateway_idempotent",
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

	s.logger.Info("wallet_partial_refund_gateway_success",
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

// GetWallet retrieves a wallet by user ID.
// Returns nil if wallet not found.
func (s *WalletService) GetWallet(ctx context.Context, userID uuid.UUID) (*entity.Wallet, error) {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	wallet, err := s.walletRepo.GetByUserID(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}

	return wallet, nil
}

// GetWalletBalance returns the available and held balances for a user.
// Returns zero values if wallet not found.
func (s *WalletService) GetWalletBalance(ctx context.Context, userID uuid.UUID) (available, held int64, err error) {
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	if wallet == nil {
		return 0, 0, nil // No wallet yet, zero balance
	}
	return wallet.AvailableBalance, wallet.HeldBalance, nil
}

// GetEscrowByOrderID retrieves the escrow for an order.
func (s *WalletService) GetEscrowByOrderID(ctx context.Context, orderID uuid.UUID) (*entity.Escrow, error) {
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
// ============================================================================
// RECONCILIATION QUERY METHODS (READ-ONLY FOR WALLET-FINANCE RECONCILIATION)
// ============================================================================

// GetWalletForReconciliation retrieves a wallet by user ID for reconciliation.
// This is a READ-ONLY method - no mutations allowed.
// Returns nil if wallet not found.
func (s *WalletService) GetWalletForReconciliation(ctx context.Context, userID uuid.UUID) (*entity.Wallet, error) {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	wallet, err := s.walletRepo.GetByUserID(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}

	return wallet, nil
}

// ============================================================================
// INVARIANT ENFORCEMENT METHODS (PHASE 2)
// ============================================================================

// AssertWalletFinanceConsistency verifies that wallet balance matches finance ledger.
//
// This is called AFTER critical financial operations to ensure consistency.
// If inconsistency is detected, it returns a CRITICAL error.
//
// INVARIANT: wallet.available_balance == financial_accounts.SELLER_PAYABLE
//
// If EnableInvariantChecks is false, this is a no-op (monitoring mode).
// If financeLedgerRepo is not set, this is a no-op (graceful degradation).
func (s *WalletService) AssertWalletFinanceConsistency(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) error {
	// GUARD: Feature flag check
	if !EnableInvariantChecks {
		return nil
	}

	// GUARD: Finance ledger not configured - skip gracefully
	if s.financeLedgerRepo == nil {
		s.logger.Debug("wallet_finance_consistency_check_skipped",
			zap.String("user_id", userID.String()),
			zap.String("reason", "finance_ledger_repo_not_configured"),
		)
		return nil
	}

	// Get wallet balance
	wallet, err := s.walletRepo.GetByUserID(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("failed to get wallet for consistency check: %w", err)
	}
	if wallet == nil {
		// No wallet yet - nothing to check
		return nil
	}

	// Get finance ledger balance (SELLER_PAYABLE account)
	sellerPayableID, err := s.financeLedgerRepo.GetUserAccountID(
		ctx, tx, FinanceAccountSellerPayable, userID,
	)
	if err != nil {
		// Account might not exist yet - skip gracefully
		s.logger.Debug("wallet_finance_consistency_check_skipped",
			zap.String("user_id", userID.String()),
			zap.String("reason", "finance_account_not_found"),
		)
		return nil
	}

	financeBalance, err := s.financeLedgerRepo.GetAccountBalance(ctx, tx, sellerPayableID)
	if err != nil {
		return fmt.Errorf("failed to get finance balance for consistency check: %w", err)
	}

	// ASSERT: Wallet available balance MUST equal finance SELLER_PAYABLE
	walletBalance := int64(wallet.AvailableBalance)
	financeBalanceInt := financeBalance.Int64()

	if walletBalance != financeBalanceInt {
		s.logger.Error("wallet_finance_inconsistency_detected",
			zap.String("user_id", userID.String()),
			zap.String("wallet_id", wallet.ID.String()),
			zap.Int64("wallet_available_balance", walletBalance),
			zap.String("finance_account_id", sellerPayableID.String()),
			zap.Int64("finance_seller_payable", financeBalanceInt),
			zap.Int64("difference", walletBalance-financeBalanceInt),
			zap.String("severity", "critical"),
			zap.String("action", "operation_blocked"),
		)
		return &ErrWalletFinanceInconsistency{
			UserID:           userID,
			WalletBalance:    walletBalance,
			FinanceBalance:   financeBalanceInt,
			Difference:       walletBalance - financeBalanceInt,
			FinanceAccountID: sellerPayableID,
		}
	}

	s.logger.Debug("wallet_finance_consistency_verified",
		zap.String("user_id", userID.String()),
		zap.Int64("balance", walletBalance),
	)

	return nil
}

// AssertEscrowInvariant verifies that held_balance equals sum of active escrows.
//
// This is called AFTER escrow operations to ensure consistency.
// If inconsistency is detected, it returns a CRITICAL error.
//
// INVARIANT: wallet.held_balance == SUM(escrows WHERE status = 'holding')
//
// If EnableInvariantChecks is false, this is a no-op (monitoring mode).
func (s *WalletService) AssertEscrowInvariant(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) error {
	// GUARD: Feature flag check
	if !EnableInvariantChecks {
		return nil
	}

	// Get wallet balance
	wallet, err := s.walletRepo.GetByUserID(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("failed to get wallet for escrow invariant check: %w", err)
	}
	if wallet == nil {
		// No wallet yet - nothing to check
		return nil
	}

	// Get all escrows for this buyer's wallet
	allEscrows, err := s.escrowRepo.GetByBuyerWalletID(ctx, tx, wallet.ID)
	if err != nil {
		return fmt.Errorf("failed to get escrows for invariant check: %w", err)
	}

	// Calculate sum of active (holding) escrows
	var activeEscrowSum int64 = 0
	var holdingCount int = 0
	for _, escrow := range allEscrows {
		if escrow.Status == entity.EscrowStatusHolding {
			activeEscrowSum += escrow.Amount
			holdingCount++
		}
	}

	// ASSERT: Wallet held balance MUST equal sum of active escrows
	walletHeldBalance := int64(wallet.HeldBalance)

	if walletHeldBalance != activeEscrowSum {
		s.logger.Error("wallet_escrow_invariant_violation_detected",
			zap.String("user_id", userID.String()),
			zap.String("wallet_id", wallet.ID.String()),
			zap.Int64("wallet_held_balance", walletHeldBalance),
			zap.Int64("computed_escrow_sum", activeEscrowSum),
			zap.Int("active_escrow_count", holdingCount),
			zap.Int("total_escrow_count", len(allEscrows)),
			zap.Int64("difference", walletHeldBalance-activeEscrowSum),
			zap.String("severity", "critical"),
			zap.String("action", "operation_blocked"),
		)
		return &ErrEscrowInvariantViolation{
			UserID:            userID,
			WalletID:          wallet.ID,
			WalletHeldBalance: walletHeldBalance,
			ComputedEscrowSum: activeEscrowSum,
			Difference:        walletHeldBalance - activeEscrowSum,
			ActiveEscrowCount: holdingCount,
		}
	}

	s.logger.Debug("wallet_escrow_invariant_verified",
		zap.String("user_id", userID.String()),
		zap.Int64("held_balance", walletHeldBalance),
		zap.Int("active_escrows", holdingCount),
	)

	return nil
}

// CheckPayoutEligibility verifies both wallet and finance ledger before allowing payout.
//
// This is called BEFORE withdrawal approval to prevent overdraft in either system.
//
// GUARDS:
// - Wallet available_balance >= amount
// - Finance SELLER_PAYABLE >= amount
// - Wallet and finance balances match (consistency check)
//
// If any guard fails, returns error with detailed reason.
func (s *WalletService) CheckPayoutEligibility(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	amount int64,
) error {
	if amount <= 0 {
		return fmt.Errorf("payout amount must be positive: got %d", amount)
	}

	// GUARD 1: Check wallet balance
	wallet, err := s.walletRepo.GetByUserID(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("failed to get wallet for payout check: %w", err)
	}
	if wallet == nil {
		return fmt.Errorf("wallet not found for user: %s", userID.String())
	}

	walletAvailable := int64(wallet.AvailableBalance)
	if walletAvailable < amount {
		s.logger.Warn("wallet_payout_rejected_insufficient_wallet_balance",
			zap.String("user_id", userID.String()),
			zap.Int64("requested_amount", amount),
			zap.Int64("wallet_available_balance", walletAvailable),
			zap.Int64("shortfall", amount-walletAvailable),
		)
		return &ErrPayoutInsufficientFunds{
			UserID:          userID,
			RequestedAmount: amount,
			WalletBalance:   walletAvailable,
			Shortfall:       amount - walletAvailable,
		}
	}

	// GUARD 2: Check finance ledger balance (if configured)
	if s.financeLedgerRepo != nil {
		sellerPayableID, err := s.financeLedgerRepo.GetUserAccountID(
			ctx, tx, FinanceAccountSellerPayable, userID,
		)
		if err != nil {
			// Account doesn't exist - create it first
			sellerPayableID, err = s.financeLedgerRepo.GetUserAccountID(
				ctx, tx, FinanceAccountSellerPayable, userID,
			)
			if err != nil {
				return fmt.Errorf("finance account not accessible for user: %s", userID.String())
			}
		}

		financeBalance, err := s.financeLedgerRepo.GetAccountBalance(ctx, tx, sellerPayableID)
		if err != nil {
			return fmt.Errorf("failed to get finance balance for payout check: %w", err)
		}

		financeAvailable := financeBalance.Int64()
		if financeAvailable < amount {
			s.logger.Warn("wallet_payout_rejected_insufficient_finance_balance",
				zap.String("user_id", userID.String()),
				zap.String("finance_account_id", sellerPayableID.String()),
				zap.Int64("requested_amount", amount),
				zap.Int64("finance_available_balance", financeAvailable),
				zap.Int64("shortfall", amount-financeAvailable),
				zap.String("severity", "critical"),
				zap.String("reason", "finance_ledger_balance_mismatch"),
			)
			return &ErrPayoutInsufficientFunds{
				UserID:           userID,
				RequestedAmount:  amount,
				WalletBalance:    walletAvailable,
				FinanceBalance:   financeAvailable,
				Shortfall:        amount - financeAvailable,
				FinanceAccountID: sellerPayableID,
			}
		}

		// GUARD 3: Assert consistency - balances must match
		if walletAvailable != financeAvailable {
			s.logger.Error("wallet_payout_blocked_balance_mismatch",
				zap.String("user_id", userID.String()),
				zap.Int64("wallet_balance", walletAvailable),
				zap.Int64("finance_balance", financeAvailable),
				zap.Int64("difference", walletAvailable-financeAvailable),
				zap.String("severity", "critical"),
				zap.String("action", "payout_blocked"),
				zap.String("reason", "wallet_finance_inconsistency"),
			)
			return &ErrPayoutBalanceMismatch{
				UserID:         userID,
				WalletBalance:  walletAvailable,
				FinanceBalance: financeAvailable,
				Difference:     walletAvailable - financeAvailable,
			}
		}
	}

	s.logger.Debug("wallet_payout_eligibility_verified",
		zap.String("user_id", userID.String()),
		zap.Int64("amount", amount),
		zap.Int64("wallet_balance", walletAvailable),
	)

	return nil
}

// ============================================================================
// INVARIANT VIOLATION ERRORS
// ============================================================================

// ErrWalletFinanceInconsistency is returned when wallet balance doesn't match finance ledger.
type ErrWalletFinanceInconsistency struct {
	UserID           uuid.UUID
	WalletBalance    int64
	FinanceBalance   int64
	Difference       int64
	FinanceAccountID uuid.UUID
}

func (e *ErrWalletFinanceInconsistency) Error() string {
	return fmt.Sprintf("CRITICAL: wallet-finance inconsistency detected for user %s: wallet=%d, finance=%d, diff=%d (account_id=%s)",
		e.UserID.String(), e.WalletBalance, e.FinanceBalance, e.Difference, e.FinanceAccountID.String())
}

// ErrEscrowInvariantViolation is returned when held_balance doesn't match sum of active escrows.
type ErrEscrowInvariantViolation struct {
	UserID            uuid.UUID
	WalletID          uuid.UUID
	WalletHeldBalance int64
	ComputedEscrowSum int64
	Difference        int64
	ActiveEscrowCount int
}

func (e *ErrEscrowInvariantViolation) Error() string {
	return fmt.Sprintf("CRITICAL: escrow invariant violation for user %s: held=%d, escrow_sum=%d, diff=%d, active_count=%d",
		e.UserID.String(), e.WalletHeldBalance, e.ComputedEscrowSum, e.Difference, e.ActiveEscrowCount)
}

// ErrPayoutInsufficientFunds is returned when payout amount exceeds available balance.
type ErrPayoutInsufficientFunds struct {
	UserID           uuid.UUID
	RequestedAmount  int64
	WalletBalance    int64
	FinanceBalance   int64 // 0 if finance not checked
	Shortfall        int64
	FinanceAccountID uuid.UUID // optional
}

func (e *ErrPayoutInsufficientFunds) Error() string {
	base := fmt.Sprintf("insufficient funds for payout: user=%s, requested=%d, wallet=%d, shortfall=%d",
		e.UserID.String(), e.RequestedAmount, e.WalletBalance, e.Shortfall)
	if e.FinanceAccountID != uuid.Nil {
		base += fmt.Sprintf(", finance=%d (account=%s)", e.FinanceBalance, e.FinanceAccountID.String())
	}
	return base
}

// ErrPayoutBalanceMismatch is returned when wallet and finance balances don't match.
type ErrPayoutBalanceMismatch struct {
	UserID         uuid.UUID
	WalletBalance  int64
	FinanceBalance int64
	Difference     int64
}

func (e *ErrPayoutBalanceMismatch) Error() string {
	return fmt.Sprintf("payout blocked: wallet-finance balance mismatch for user %s: wallet=%d, finance=%d, diff=%d",
		e.UserID.String(), e.WalletBalance, e.FinanceBalance, e.Difference)
}


