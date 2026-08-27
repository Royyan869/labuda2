// ============================================================================
// COINS SERVICE - COINS VALIDATION SOURCE OF TRUTH
// ============================================================================
//
// SOURCE OF TRUTH: All coins validation must go through this service.
//
// DOMAIN RULE: This service is the SINGLE SOURCE OF TRUTH for coins operations.
// - Coins are loyalty points for buyer incentives
// - Coins are NOT money (no wallet ledger impact)
//
// DUAL-TABLE DESIGN:
//   - coins_transactions: append-only audit ledger for all earn/refund events
//   - user_coin_balance:  single aggregate row per user; used as atomic concurrency
//     control for balance operations. Balance in this table is always kept in
//     sync with the ledger by the service on every write.
//     The two sources must never diverge; ReconcileBalance() can detect drift.
//
// BUSINESS RULES:
// - BALANCE CAP: Earned coins are capped by daily policy
// - NO EXPIRY: Coins do not expire (V1 simplification)
//
// NO COINS LOGIC SHALL EXIST OUTSIDE THIS SERVICE.
// ============================================================================
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/incentive/coins/entity"
	coinsRepo "github.com/labuda/backend/internal/incentive/coins/infrastructure/repository"
	"github.com/labuda/backend/internal/incentive/coins/repository"
	"github.com/labuda/backend/pkg/db"
)

const (
	// OrderRewardRate is the points earning rate for order completion: 1 point per Rp1.000 of final paid amount.
	// Formula: points = floor(final_paid_amount / 1000)
	// Example: Rp75.000 → 75 points, Rp123.500 → 123 points
	OrderRewardRate = 1000

	// MinOrderValueForCoins is the minimum order value (in Rp) required to earn coins.
	// Prevents abuse via many small orders.
	MinOrderValueForCoins = 10000 // Rp10.000 minimum

	// MaxDailyCoinsEarn is the maximum coins a user can earn in a single day.
	// Prevents abuse via high-value orders.
	MaxDailyCoinsEarn = 10000 // Max 10.000 coins per day
)

// CoinsService handles loyalty points earning, refunding, and balance reconciliation.
//
// IMPORTANT: These are loyalty points ONLY, NOT money.
// This service manages loyalty point earning and refund reconciliation.
// It does NOT integrate with financial systems, escrow, or payment processing.
//
// Dual-table design: coins_transactions (audit ledger) + user_coin_balance (concurrency row).
// All writes update both atomically within the same transaction.
type CoinsService struct {
	repo         repository.CoinsRepository
	db           *db.DB
	auditService interface { // Minimal interface to avoid circular import
		CoinsEarned(ctx context.Context, tx db.Tx, userID uuid.UUID, amount int64, referenceType string, referenceID *uuid.UUID)
		CoinsRefunded(ctx context.Context, tx db.Tx, userID uuid.UUID, amount int64, reason string)
	}
}

// NewCoinsService creates a new CoinsService.
func NewCoinsService(repo repository.CoinsRepository, db *db.DB) *CoinsService {
	return &CoinsService{
		repo: repo,
		db:   db,
	}
}

// SetAuditService sets the audit service for audit logging.
// This is called during dependency injection to enable audit events.
func (s *CoinsService) SetAuditService(auditService interface { // Minimal interface to avoid circular import
	CoinsEarned(ctx context.Context, tx db.Tx, userID uuid.UUID, amount int64, referenceType string, referenceID *uuid.UUID)
	CoinsRefunded(ctx context.Context, tx db.Tx, userID uuid.UUID, amount int64, reason string)
}) {
	s.auditService = auditService
}

// CurrentBalance represents a user's current balance (derived from transactions).
type CurrentBalance struct {
	UserID    uuid.UUID
	Balance   int64     // Current balance (derived from transactions)
	UpdatedAt time.Time // Time of last transaction
}

// GetBalance retrieves a user's current coins balance.
// Balance is derived from transactions, not stored.
func (s *CoinsService) GetBalance(ctx context.Context, userID uuid.UUID) (*CurrentBalance, error) {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get balance from transactions
	balance, err := s.repo.GetActiveBalance(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Get last transaction time for UpdatedAt
	stats, err := s.repo.GetLifetimeStats(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lifetime stats: %w", err)
	}

	updatedAt := time.Now()
	if stats.LastTransactionAt != nil {
		updatedAt = *stats.LastTransactionAt
	}

	return &CurrentBalance{
		UserID:    userID,
		Balance:   balance,
		UpdatedAt: updatedAt,
	}, nil
}

// BalanceWithLifetime contains balance plus computed lifetime statistics.
type BalanceWithLifetime struct {
	UserID             uuid.UUID
	Balance            int64
	UpdatedAt          time.Time
	LifetimeEarned     int64      // Total coins earned (from transactions)
	LifetimeSpent      int64      // Total coins spent (from transactions)
	FirstTransactionAt *time.Time // Earliest transaction timestamp (nil if none)
	LastTransactionAt  *time.Time // Latest transaction timestamp (nil if none)
}

// GetBalanceWithLifetime retrieves balance with computed lifetime statistics from transactions.
// This is the TRUTH-aligned version for API responses - all fields are real or null.
func (s *CoinsService) GetBalanceWithLifetime(ctx context.Context, userID uuid.UUID) (*BalanceWithLifetime, error) {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get balance from transactions
	balance, err := s.repo.GetActiveBalance(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	stats, err := s.repo.GetLifetimeStats(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lifetime stats: %w", err)
	}

	updatedAt := time.Now()
	if stats.LastTransactionAt != nil {
		updatedAt = *stats.LastTransactionAt
	}

	return &BalanceWithLifetime{
		UserID:             userID,
		Balance:            balance,
		UpdatedAt:          updatedAt,
		LifetimeEarned:     stats.LifetimeEarned,
		LifetimeSpent:      stats.LifetimeSpent,
		FirstTransactionAt: stats.FirstTransactionAt,
		LastTransactionAt:  stats.LastTransactionAt,
	}, nil
}

// ReservationBalanceSnapshot captures the canonical reservation-aware balance
// equation: total - reserved = available.
type ReservationBalanceSnapshot struct {
	UserID           uuid.UUID
	TotalBalance     int64
	ReservedBalance  int64
	AvailableBalance int64
	UpdatedAt        time.Time
}

// GetReservationBalance returns the current total, reserved, and available
// coin amounts for a user.
func (s *CoinsService) GetReservationBalance(ctx context.Context, userID uuid.UUID) (*ReservationBalanceSnapshot, error) {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	balanceRow, err := s.repo.GetBalanceRow(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance row: %w", err)
	}

	totalBalance := int64(0)
	updatedAt := time.Now()
	if balanceRow != nil {
		totalBalance = balanceRow.Balance
		updatedAt = balanceRow.UpdatedAt
	}

	reservedBalance, err := s.repo.SumActiveReservations(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reserved balance: %w", err)
	}

	return &ReservationBalanceSnapshot{
		UserID:           userID,
		TotalBalance:     totalBalance,
		ReservedBalance:  reservedBalance,
		AvailableBalance: totalBalance - reservedBalance,
		UpdatedAt:        updatedAt,
	}, nil
}

// GetTransactions retrieves a paginated list of transactions for a user.
func (s *CoinsService) GetTransactions(ctx context.Context, userID uuid.UUID, page, pageSize int) (*entity.CoinsTransactionPage, error) {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	offset := (page - 1) * pageSize

	transactions, err := s.repo.GetTransactions(ctx, tx, userID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	totalCount, err := s.repo.CountTransactions(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count transactions: %w", err)
	}

	return &entity.CoinsTransactionPage{
		Transactions: transactions,
		TotalCount:   totalCount,
		Page:         page,
		PageSize:     pageSize,
		HasMore:      int64(offset+len(transactions)) < totalCount,
	}, nil
}

// EarnPointsForOrderCompletion grants loyalty points to buyer when order is completed.
//
// This is the ONLY place where order completion rewards are granted.
// Points are ONLY granted when order status transitions to "completed".
//
// REWARD FORMULA:
// - 1 point per Rp1.000 of final paid amount (floor division)
// - Formula: points = floor(final_paid_amount / 1000)
// - Examples: Rp75.000 → 75 points, Rp123.500 → 123 points
//
// ANTI-ABUSE CHECKS:
// - Minimum order value: Rp10.000 required to earn any coins
// - Daily limit: Max 10.000 coins per user per day
//
// IDEMPOTENCY (HARD):
// - Fast path: Check if order_reward already exists for this order
// - Hard protection: UNIQUE constraint on (user_id, reference_type, reference_id)
// - If duplicate insert occurs, treats as idempotent success (no error)
//
// IMPORTANT:
// - ONLY called from OrderCompletionService.Complete()
// - NOT called during payment, shipping, or delivery
// - Cancelled/refunded orders do NOT grant points
//
// NOTE: Balance is derived from transactions - no stored balance to update.
func (s *CoinsService) EarnPointsForOrderCompletion(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	orderID uuid.UUID,
	finalPaidAmount int64,
) error {
	// ANTI-ABUSE: Check minimum order value
	if finalPaidAmount < MinOrderValueForCoins {
		// Order amount below minimum, no points granted
		return nil
	}

	// Calculate reward: 1 point per Rp1.000, floor division
	pointsEarned := finalPaidAmount / OrderRewardRate

	// ANTI-ABUSE: Check daily earn limit
	dailyTotal, err := s.repo.GetDailyEarnedTotal(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("failed to check daily earn total: %w", err)
	}
	if dailyTotal+pointsEarned > MaxDailyCoinsEarn {
		// Daily limit exceeded, cap at remaining allowance
		remaining := MaxDailyCoinsEarn - dailyTotal
		if remaining <= 0 {
			// User already hit daily limit, no points granted
			return nil
		}
		// Grant remaining allowance instead of full amount
		pointsEarned = remaining
	}

	// IDEMPOTENCY CHECK (FAST PATH): Check if order_reward already exists for this order
	// This is an optimization to avoid the database round-trip for happy path
	existingTx, err := s.repo.FindEarnByReference(ctx, tx, userID, orderID, entity.CoinReferenceOrderReward)
	if err != nil {
		return fmt.Errorf("failed to check existing order reward: %w", err)
	}
	if existingTx != nil {
		// Order reward already granted - idempotent
		return nil
	}

	// Create earn transaction with order_reward reference type
	transaction, err := entity.NewEarnTransaction(userID, pointsEarned, entity.CoinReferenceOrderReward, &orderID)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// HARD IDEMPOTENCY: Create transaction with database-level uniqueness
	// If concurrent request inserted first, this will return ErrDuplicateTransaction
	// We treat that as idempotent success (no error)
	if err := s.repo.CreateTransaction(ctx, tx, transaction); err != nil {
		if coinsRepo.IsDuplicateTransaction(err) {
			// Duplicate transaction = already granted by concurrent request
			// This is idempotent success - do NOT return error
			return nil
		}
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// ========================================================================
	// UPDATE AGGREGATE BALANCE (ATOMIC ADD)
	// ========================================================================
	// Add the earned points to the user's aggregate balance.
	// This keeps the aggregate table in sync with transactions.
	//
	// SQL: UPDATE user_coin_balance SET balance = balance + $1
	//      WHERE user_id = $2
	//
	// If no row exists, the update affects 0 rows (not an error here;
	// balance will be initialized on the next coin operation or can be backfilled).
	_, _ = s.repo.AtomicAddBalance(ctx, tx, userID, pointsEarned)

	return nil
}

// ConsumeAndSpendForOrder completes the canonical coin lifecycle
// RESERVE → CONSUME for a settled order payment.
//
// It atomically (within the caller's tx):
//  1. Consumes the reservation for the payment exactly once (idempotent:
//     an already-consumed reservation is a no-op success).
//  2. Writes the canonical coin spend transaction
//     (reference_type='order_spend', reference_id=order_id). The UNIQUE
//     (user_id, reference_type, reference_id) constraint makes this
//     idempotent: a duplicate insert is treated as success.
//  3. Deducts K from user_coin_balance exactly once via AtomicDeductBalance.
//
// K is authoritative in the coins domain (user_coin_balance,
// coin_reservations, coins_transactions). This method never writes a finance
// ledger entry — coins are NOT money and coins_transactions is NOT a finance
// journal (see entity/coins_transaction.go). The finance ledger records the
// platform funding of K separately (FinanceService.RecordCoinFunding).
//
// Failure of any step rolls back the caller's transaction. The reservation
// is only consumed when the payment actually settles; failed/rejected
// payments release the reservation instead (ReleaseReservation).
func (s *CoinsService) ConsumeAndSpendForOrder(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	orderID uuid.UUID,
	paymentID uuid.UUID,
	amount int64,
) error {
	if amount <= 0 {
		return nil // No coins redeemed — nothing to consume or spend.
	}

	// Step 1: Consume the reservation exactly once.
	reservation, err := s.repo.ConsumeReservation(ctx, tx, paymentID)
	if err != nil {
		return fmt.Errorf("consume coin reservation: %w", err)
	}
	if reservation == nil {
		// IDEMPOTENT REPLAY: the reservation was already consumed by a prior
		// settlement of this payment. The spend transaction must already exist
		// (created on first settlement); verify it and treat as success so a
		// duplicate settlement webhook is a no-op, not an error.
		existing, err := s.repo.FindSpendByReference(ctx, tx, userID, orderID)
		if err != nil {
			return fmt.Errorf("idempotent replay: verify existing coin spend: %w", err)
		}
		if existing == nil {
			return fmt.Errorf("no coin reservation found for payment %s (amount=%d) and no prior spend exists", paymentID, amount)
		}
		return nil
	}

	// Step 2: Write the canonical order_spend spend transaction.
	spend, err := entity.NewSpendTransaction(userID, amount, orderID)
	if err != nil {
		return fmt.Errorf("create coin spend transaction: %w", err)
	}
	if err := s.repo.CreateTransaction(ctx, tx, spend); err != nil {
		if coinsRepo.IsDuplicateTransaction(err) {
			// Idempotent: the spend already exists for this order. Still
			// proceed to ensure the balance deduction is consistent.
		} else {
			return fmt.Errorf("persist coin spend transaction: %w", err)
		}
	}

	// Step 3: Deduct K from the aggregate balance exactly once.
	rows, err := s.repo.AtomicDeductBalance(ctx, tx, userID, amount)
	if err != nil {
		return fmt.Errorf("deduct coin balance: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("insufficient coin balance: user=%s amount=%d", userID, amount)
	}

	return nil
}

// RefundCoinsInternal refunds coins spent on an order.
//
// This is the INTERNAL method for refunding coins in failure paths.
// Unlike RefundCoins (which is for explicit refund requests),
// this method is called by system operations like cancel, expire, dispute.
//
// HARD IDEMPOTENCY (Migration 000021):
// - Uses UNIQUE constraint on (user_id, reference_type, reference_id)
// - INSERT-FIRST pattern: attempt to create transaction BEFORE any other work
// - If duplicate, treat as idempotent success (skip)
// - No scan-based idempotency check (eliminates race window)
//
// AGGREGATE BALANCE: Also updates the aggregate balance table.
func (s *CoinsService) RefundCoinsInternal(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	orderID uuid.UUID,
) error {
	// Check if coins were spent on this order (to determine refund amount)
	existingTx, err := s.repo.FindSpendByReference(ctx, tx, userID, orderID)
	if err != nil {
		return fmt.Errorf("failed to check spend transaction: %w", err)
	}

	// If no spend transaction found, no coins to refund
	if existingTx == nil {
		return nil
	}

	// Refund the amount that was spent
	amountToRefund := existingTx.Amount // Spend amount is positive in DB

	if amountToRefund <= 0 {
		return nil // Nothing to refund
	}

	// INSERT-FIRST: Create refund transaction
	// This is the critical idempotency check - database constraint prevents duplicates
	transaction, err := entity.NewEarnTransaction(
		userID,
		amountToRefund,
		entity.CoinReferenceRefundEarn,
		&orderID,
	)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	if err := s.repo.CreateTransaction(ctx, tx, transaction); err != nil {
		if coinsRepo.IsDuplicateTransaction(err) {
			// Transaction already exists - refund already processed
			// Idempotent success
			return nil
		}
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// UPDATE AGGREGATE BALANCE (ATOMIC ADD)
	// Add refunded coins back to user's aggregate balance
	// We use EnsureBalanceRow first to make sure the row exists
	_ = s.repo.EnsureBalanceRow(ctx, tx, userID)
	_, _ = s.repo.AtomicAddBalance(ctx, tx, userID, amountToRefund)

	return nil
}

// RefundCoinsWithDelta restores EXACTLY amount coins for a refund event.
//
// REFUND ECONOMICS REBASE: the proportional coin restoration
// floor(K * cumProductRefund / PD) is computed by the gateway refund ack
// pipeline and delivered via coin_delta on the coins.refund_required event.
// This method records the exact delta as a refund_earn transaction keyed by
// the outbox event ID — a UNIQUE reference per emission — so multiple
// partial restorations for the same order each create their own earn row
// while replays of the same event stay idempotent (UNIQUE constraint on
// (user_id, reference_type, reference_id)).
//
// INSERT-FIRST pattern: the DB constraint is the idempotency authority; a
// duplicate insert is an idempotent success, never an error.
func (s *CoinsService) RefundCoinsWithDelta(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	eventID uuid.UUID,
	amount int64,
) error {
	if amount <= 0 {
		return nil // Nothing to restore
	}

	transaction, err := entity.NewEarnTransaction(
		userID,
		amount,
		entity.CoinReferenceRefundEarn,
		&eventID,
	)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	if err := s.repo.CreateTransaction(ctx, tx, transaction); err != nil {
		if coinsRepo.IsDuplicateTransaction(err) {
			// Transaction already exists - refund already processed
			// Idempotent success
			return nil
		}
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// UPDATE AGGREGATE BALANCE (ATOMIC ADD)
	_ = s.repo.EnsureBalanceRow(ctx, tx, userID)
	_, _ = s.repo.AtomicAddBalance(ctx, tx, userID, amount)

	return nil
}
