package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/incentive/coins/entity"
	"github.com/labuda/backend/pkg/db"
)

// LifetimeStatsQuery contains computed lifetime statistics from transactions.
type LifetimeStatsQuery struct {
	LifetimeEarned     int64      // Total coins earned (sum of earn transactions)
	LifetimeSpent      int64      // Total coins spent (sum of spend transactions)
	FirstTransactionAt *time.Time // Timestamp of earliest transaction (nil if none)
	LastTransactionAt  *time.Time // Timestamp of latest transaction (nil if none)
}

// CoinsRepository defines the interface for coins persistence operations.
//
// IMPORTANT: Coins are loyalty points only, NOT money.
// This repository manages loyalty point transactions, NOT stored balances.
// Balance is ALWAYS derived from transactions using GetActiveBalance().
type CoinsRepository interface {
	// Transaction operations

	// CreateTransaction creates a new coins transaction record.
	CreateTransaction(ctx context.Context, tx db.Tx, transaction *entity.CoinsTransaction) error

	// GetTransactions retrieves a paginated list of transactions for a user.
	GetTransactions(ctx context.Context, tx db.Tx, userID uuid.UUID, limit, offset int) ([]*entity.CoinsTransaction, error)

	// CountTransactions returns the total number of transactions for a user.
	CountTransactions(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error)

	// FindSpendByReference finds a spend transaction for a user by reference ID.
	// Used for idempotency checks to prevent double-spending on retry.
	// Returns nil if not found.
	FindSpendByReference(ctx context.Context, tx db.Tx, userID uuid.UUID, referenceID uuid.UUID) (*entity.CoinsTransaction, error)

	// FindEarnByReference finds an earn transaction for a user by reference ID and reference type.
	// Used for idempotency checks to prevent double-granting rewards.
	// Returns nil if not found.
	FindEarnByReference(ctx context.Context, tx db.Tx, userID uuid.UUID, referenceID uuid.UUID, referenceType entity.CoinReferenceType) (*entity.CoinsTransaction, error)

	// GetLifetimeStats computes lifetime statistics from transactions.
	// Returns aggregated values from coins_transactions table.
	// Returns zero values if user has no transactions.
	GetLifetimeStats(ctx context.Context, tx db.Tx, userID uuid.UUID) (*LifetimeStatsQuery, error)

	// Balance operations

	// GetActiveBalance calculates the current active balance for a user.
	// Sum of all earn transactions minus all spend transactions.
	GetActiveBalance(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error)

	// GetDailyEarnedTotal returns the total coins earned by a user today (UTC).
	// Used for anti-abuse: enforces daily earn limits.
	GetDailyEarnedTotal(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error)

	// ============================================================================
	// AGGREGATE BALANCE OPERATIONS (ATOMIC CONCURRENCY CONTROL)
	// ============================================================================

	// EnsureBalanceRow ensures a user has a balance row (creates with 0 if not exists).
	// This is idempotent - safe to call multiple times.
	EnsureBalanceRow(ctx context.Context, tx db.Tx, userID uuid.UUID) error

	// GetBalanceRow retrieves the current aggregate balance for a user.
	// Returns nil if no balance row exists (use EnsureBalanceRow to create).
	GetBalanceRow(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.UserCoinBalance, error)

	// GetBalanceRowForUpdate retrieves the current aggregate balance for a user
	// while holding a row lock for reservation serialization.
	// Returns nil if no balance row exists (use EnsureBalanceRow to create).
	GetBalanceRowForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.UserCoinBalance, error)

	// SumActiveReservations returns the total amount held by active reservations
	// for a user. Only status='reserved' rows are counted.
	SumActiveReservations(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error)

	// ============================================================================
	// RESERVATION OPERATIONS (MODEL R)
	// ============================================================================

	// CreateReservation creates a reservation for the given payment.
	CreateReservation(ctx context.Context, tx db.Tx, reservation *entity.CoinReservation) error

	// GetReservationByPaymentID loads a reservation by payment ID.
	// Returns nil if no reservation exists for the payment.
	GetReservationByPaymentID(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*entity.CoinReservation, error)

	// ConsumeReservation transitions a reserved payment to consumed.
	// Returns nil if the reservation is already in the same terminal state.
	ConsumeReservation(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*entity.CoinReservation, error)

	// ReleaseReservation transitions a reserved payment to released.
	// Returns nil if the reservation is already in the same terminal state.
	ReleaseReservation(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*entity.CoinReservation, error)

	// AtomicDeductBalance performs an atomic deduct operation on the aggregate balance.
	//
	// This is the PRIMARY concurrency control mechanism for coin spending.
	// Uses a single UPDATE with WHERE clause - no row locking needed.
	//
	// SQL: UPDATE user_coin_balance SET balance = balance - $1
	//      WHERE user_id = $2 AND balance >= $1
	//
	// Returns:
	//   - rowsAffected: 1 if success, 0 if insufficient funds
	//   - err: database error on failure
	//
	// CRITICAL: This is the ONLY method that should deduct balance.
	// All spend operations must go through this method first.
	AtomicDeductBalance(ctx context.Context, tx db.Tx, userID uuid.UUID, amount int64) (int64, error)

	// AtomicAddBalance adds coins to the user's aggregate balance.
	// Used when coins are earned (order completion, grants, etc).
	//
	// SQL: UPDATE user_coin_balance SET balance = balance + $1
	//      WHERE user_id = $2
	//
	// Returns rows affected (1 if row exists, 0 if not - use EnsureBalanceRow first).
	AtomicAddBalance(ctx context.Context, tx db.Tx, userID uuid.UUID, amount int64) (int64, error)

	// ReconcileBalance compares aggregate balance with derived balance from transactions.
	// Returns the difference (positive = aggregate is higher, negative = aggregate is lower).
	// Returns 0 if balances match.
	ReconcileBalance(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error)
}
