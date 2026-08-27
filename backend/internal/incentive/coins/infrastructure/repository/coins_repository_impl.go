package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/incentive/coins/entity"
	coinsrepo "github.com/labuda/backend/internal/incentive/coins/repository"
	"github.com/labuda/backend/pkg/db"
)

// CoinsRepositoryImpl implements CoinsRepository using PostgreSQL.
// Balance is ALWAYS derived from transactions - no stored balance.
type CoinsRepositoryImpl struct{}

// NewCoinsRepository creates a new CoinsRepositoryImpl.
func NewCoinsRepository() *CoinsRepositoryImpl {
	return &CoinsRepositoryImpl{}
}

// ============================================================================
// TRANSACTION OPERATIONS
// ============================================================================

// CreateTransaction creates a new loyalty points transaction record.
//
// HARD IDEMPOTENCY:
// Uses UNIQUE constraint on (user_id, reference_type, reference_id) to prevent
// duplicate transactions even under race conditions.
//
// If a duplicate insert occurs:
// - Returns ErrDuplicateTransaction (wrapped)
// - Caller should treat this as idempotent success
// - No double-grant possible (database enforces uniqueness)
func (r *CoinsRepositoryImpl) CreateTransaction(ctx context.Context, tx db.Tx, transaction *entity.CoinsTransaction) error {
	query := `
		INSERT INTO coins_transactions (
			id, user_id, type, amount,
			reference_type, reference_id,
			created_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7
		)
	`

	_, err := tx.Exec(ctx, query,
		transaction.ID,
		transaction.UserID,
		transaction.Type,
		transaction.Amount,
		transaction.ReferenceType,
		transaction.ReferenceID,
		transaction.CreatedAt,
	)

	if err != nil {
		// Check for unique constraint violation (PostgreSQL error code 23505)
		// This indicates the transaction already exists - treat as idempotent success
		if isDuplicateKeyError(err) {
			return &ErrDuplicateTransaction{
				UserID:        transaction.UserID,
				ReferenceType: transaction.ReferenceType,
				ReferenceID:   transaction.ReferenceID,
				Err:           err,
			}
		}
		return fmt.Errorf("failed to create loyalty points transaction: %w", err)
	}

	return nil
}

// GetTransactions retrieves a paginated list of transactions for a user.
func (r *CoinsRepositoryImpl) GetTransactions(ctx context.Context, tx db.Tx, userID uuid.UUID, limit, offset int) ([]*entity.CoinsTransaction, error) {
	query := `
		SELECT id, user_id, type, amount,
		       reference_type, reference_id,
		       created_at
		FROM coins_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := tx.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*entity.CoinsTransaction
	for rows.Next() {
		var txEntity entity.CoinsTransaction
		err := rows.Scan(
			&txEntity.ID,
			&txEntity.UserID,
			&txEntity.Type,
			&txEntity.Amount,
			&txEntity.ReferenceType,
			&txEntity.ReferenceID,
			&txEntity.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, &txEntity)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating transactions: %w", rows.Err())
	}

	return transactions, nil
}

// CountTransactions returns the total number of transactions for a user.
func (r *CoinsRepositoryImpl) CountTransactions(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM coins_transactions
		WHERE user_id = $1
	`

	var count int64
	err := tx.QueryRow(ctx, query, userID).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	return count, nil
}

// FindSpendByReference finds a spend transaction for a user by reference ID.
// Used for idempotency checks to prevent double-spending on retry.
// Returns nil if not found.
func (r *CoinsRepositoryImpl) FindSpendByReference(ctx context.Context, tx db.Tx, userID uuid.UUID, referenceID uuid.UUID) (*entity.CoinsTransaction, error) {
	query := `
		SELECT id, user_id, type, amount,
		       reference_type, reference_id,
		       created_at
		FROM coins_transactions
		WHERE user_id = $1
		  AND type = 'spend'
		  AND reference_id = $2
		LIMIT 1
	`

	var transaction entity.CoinsTransaction
	err := tx.QueryRow(ctx, query, userID, referenceID).Scan(
		&transaction.ID,
		&transaction.UserID,
		&transaction.Type,
		&transaction.Amount,
		&transaction.ReferenceType,
		&transaction.ReferenceID,
		&transaction.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No spend transaction found for this reference
		}
		return nil, fmt.Errorf("failed to find spend transaction by reference: %w", err)
	}

	return &transaction, nil
}

// FindEarnByReference finds an earn transaction for a user by reference ID and reference type.
// Used for idempotency checks to prevent double-granting rewards.
// Returns nil if not found.
func (r *CoinsRepositoryImpl) FindEarnByReference(ctx context.Context, tx db.Tx, userID uuid.UUID, referenceID uuid.UUID, referenceType entity.CoinReferenceType) (*entity.CoinsTransaction, error) {
	query := `
		SELECT id, user_id, type, amount,
		       reference_type, reference_id,
		       created_at
		FROM coins_transactions
		WHERE user_id = $1
		  AND type = 'earn'
		  AND reference_id = $2
		  AND reference_type = $3
		LIMIT 1
	`

	var transaction entity.CoinsTransaction
	err := tx.QueryRow(ctx, query, userID, referenceID, referenceType).Scan(
		&transaction.ID,
		&transaction.UserID,
		&transaction.Type,
		&transaction.Amount,
		&transaction.ReferenceType,
		&transaction.ReferenceID,
		&transaction.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No earn transaction found for this reference
		}
		return nil, fmt.Errorf("failed to find earn transaction by reference: %w", err)
	}

	return &transaction, nil
}

// GetLifetimeStats computes lifetime statistics from transactions.
//
// This query aggregates all transactions to compute:
// - lifetime_earned: sum of all earn transactions
// - lifetime_spent: sum of all spend transactions (amount is positive in DB, we sum it)
// - first_transaction_at: MIN(created_at) across all transactions
// - last_transaction_at: MAX(created_at) across all transactions
//
// Returns zero values if user has no transactions.
func (r *CoinsRepositoryImpl) GetLifetimeStats(ctx context.Context, tx db.Tx, userID uuid.UUID) (*coinsrepo.LifetimeStatsQuery, error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN type = 'earn' THEN amount ELSE 0 END), 0) AS lifetime_earned,
			COALESCE(SUM(CASE WHEN type = 'spend' THEN amount ELSE 0 END), 0) AS lifetime_spent,
			MIN(created_at) AS first_transaction_at,
			MAX(created_at) AS last_transaction_at
		FROM coins_transactions
		WHERE user_id = $1
	`

	var stats coinsrepo.LifetimeStatsQuery
	err := tx.QueryRow(ctx, query, userID).Scan(
		&stats.LifetimeEarned,
		&stats.LifetimeSpent,
		&stats.FirstTransactionAt,
		&stats.LastTransactionAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to query lifetime stats: %w", err)
	}

	return &stats, nil
}

// ============================================================================
// BALANCE OPERATIONS (DERIVED FROM TRANSACTIONS)
// ============================================================================

// GetActiveBalance calculates the current balance for a user.
//
// Sum of all earn transactions minus all spend transactions.
func (r *CoinsRepositoryImpl) GetActiveBalance(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error) {
	query := `
		SELECT COALESCE(SUM(CASE WHEN type = 'earn' THEN amount ELSE -amount END), 0) AS balance
		FROM coins_transactions
		WHERE user_id = $1
	`

	var balance int64
	err := tx.QueryRow(ctx, query, userID).Scan(&balance)

	if err != nil {
		return 0, fmt.Errorf("failed to query balance: %w", err)
	}

	return balance, nil
}

// GetDailyEarnedTotal returns the total coins earned by a user today (UTC).
// Used for anti-abuse: enforces daily earn limits.
func (r *CoinsRepositoryImpl) GetDailyEarnedTotal(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0) AS daily_total
		FROM coins_transactions
		WHERE user_id = $1
		  AND type = 'earn'
		  AND DATE_TRUNC('day', created_at) = DATE_TRUNC('day', NOW() AT TIME ZONE 'UTC')
	`

	var dailyTotal int64
	err := tx.QueryRow(ctx, query, userID).Scan(&dailyTotal)

	if err != nil {
		return 0, fmt.Errorf("failed to query daily earned total: %w", err)
	}

	return dailyTotal, nil
}

// GetBalanceRowForUpdate retrieves the current aggregate balance for a user
// while holding a row lock. This is the serialization point for reservation
// creation.
func (r *CoinsRepositoryImpl) GetBalanceRowForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.UserCoinBalance, error) {
	query := `
		SELECT user_id, balance, version, created_at, updated_at
		FROM user_coin_balance
		WHERE user_id = $1
		FOR UPDATE
	`
	var balance entity.UserCoinBalance
	err := tx.QueryRow(ctx, query, userID).Scan(
		&balance.UserID,
		&balance.Balance,
		&balance.Version,
		&balance.CreatedAt,
		&balance.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get balance row for update: %w", err)
	}
	return &balance, nil
}

// SumActiveReservations returns the amount held by active reservations.
func (r *CoinsRepositoryImpl) SumActiveReservations(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM coin_reservations
		WHERE user_id = $1
		  AND status = 'reserved'
	`
	var reserved int64
	if err := tx.QueryRow(ctx, query, userID).Scan(&reserved); err != nil {
		return 0, fmt.Errorf("failed to sum active reservations: %w", err)
	}
	return reserved, nil
}

// GetReservationByPaymentID loads a reservation by payment ID.
func (r *CoinsRepositoryImpl) GetReservationByPaymentID(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*entity.CoinReservation, error) {
	return r.loadReservationByPaymentID(ctx, tx, paymentID, false)
}

func (r *CoinsRepositoryImpl) loadReservationByPaymentID(ctx context.Context, tx db.Tx, paymentID uuid.UUID, forUpdate bool) (*entity.CoinReservation, error) {
	query := `
		SELECT id, payment_id, user_id, amount, status,
		       expires_at, consumed_at, released_at,
		       created_at, updated_at
		FROM coin_reservations
		WHERE payment_id = $1
	`
	if forUpdate {
		query += " FOR UPDATE"
	}

	var reservation entity.CoinReservation
	var consumedAt sql.NullTime
	var releasedAt sql.NullTime
	err := tx.QueryRow(ctx, query, paymentID).Scan(
		&reservation.ID,
		&reservation.PaymentID,
		&reservation.UserID,
		&reservation.Amount,
		&reservation.Status,
		&reservation.ExpiresAt,
		&consumedAt,
		&releasedAt,
		&reservation.CreatedAt,
		&reservation.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load reservation by payment id: %w", err)
	}

	reservation.ConsumedAt = db.ToTimePtr(consumedAt)
	reservation.ReleasedAt = db.ToTimePtr(releasedAt)
	return &reservation, nil
}

// CreateReservation inserts a new active reservation for a payment.
func (r *CoinsRepositoryImpl) CreateReservation(ctx context.Context, tx db.Tx, reservation *entity.CoinReservation) error {
	if reservation == nil {
		return fmt.Errorf("reservation cannot be nil")
	}
	if reservation.Status != entity.CoinReservationStatusReserved {
		return fmt.Errorf("reservation must start in reserved state: got %s", reservation.Status)
	}

	if err := r.EnsureBalanceRow(ctx, tx, reservation.UserID); err != nil {
		return fmt.Errorf("failed to ensure balance row: %w", err)
	}

	balanceRow, err := r.GetBalanceRowForUpdate(ctx, tx, reservation.UserID)
	if err != nil {
		return err
	}
	if balanceRow == nil {
		return fmt.Errorf("failed to lock balance row for user %s", reservation.UserID)
	}

	existing, err := r.GetReservationByPaymentID(ctx, tx, reservation.PaymentID)
	if err != nil {
		return err
	}
	if existing != nil {
		return &entity.ErrReservationConflict{
			PaymentID:       reservation.PaymentID,
			ExistingAmount:  existing.Amount,
			RequestedAmount: reservation.Amount,
		}
	}

	reserved, err := r.SumActiveReservations(ctx, tx, reservation.UserID)
	if err != nil {
		return err
	}
	available := balanceRow.Balance - reserved
	if reservation.Amount > available {
		return &entity.ErrReservationInsufficientBalance{
			UserID:           reservation.UserID,
			RequestedAmount:  reservation.Amount,
			AvailableBalance: available,
			TotalBalance:     balanceRow.Balance,
			ReservedBalance:  reserved,
		}
	}

	query := `
		INSERT INTO coin_reservations (
			id, payment_id, user_id, amount, status,
			expires_at, consumed_at, released_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9, $10
		)
	`
	_, err = tx.Exec(ctx, query,
		reservation.ID,
		reservation.PaymentID,
		reservation.UserID,
		reservation.Amount,
		reservation.Status,
		reservation.ExpiresAt,
		db.ToNullTime(reservation.ConsumedAt),
		db.ToNullTime(reservation.ReleasedAt),
		reservation.CreatedAt,
		reservation.UpdatedAt,
	)
	if err != nil {
		if db.IsUniqueViolation(err) {
			duplicate, lookupErr := r.GetReservationByPaymentID(ctx, tx, reservation.PaymentID)
			if lookupErr != nil {
				return fmt.Errorf("failed to check existing reservation after duplicate insert: %w", lookupErr)
			}
			if duplicate != nil {
				return &entity.ErrReservationConflict{
					PaymentID:       reservation.PaymentID,
					ExistingAmount:  duplicate.Amount,
					RequestedAmount: reservation.Amount,
				}
			}
			return &entity.ErrReservationConflict{
				PaymentID:       reservation.PaymentID,
				ExistingAmount:  reservation.Amount,
				RequestedAmount: reservation.Amount,
			}
		}
		return fmt.Errorf("failed to create coin reservation: %w", err)
	}

	return nil
}

// ConsumeReservation transitions a reservation from reserved to consumed.
func (r *CoinsRepositoryImpl) ConsumeReservation(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*entity.CoinReservation, error) {
	reservation, err := r.loadReservationByPaymentID(ctx, tx, paymentID, true)
	if err != nil {
		return nil, err
	}
	if reservation == nil {
		return nil, nil
	}

	switch reservation.Status {
	case entity.CoinReservationStatusConsumed:
		return nil, nil
	case entity.CoinReservationStatusReleased:
		return nil, &entity.ErrReservationAlreadyReleased{PaymentID: paymentID}
	case entity.CoinReservationStatusReserved:
		now := time.Now()
		query := `
			UPDATE coin_reservations
			SET status = 'consumed',
			    consumed_at = $2,
			    updated_at = $2
			WHERE payment_id = $1
			  AND status = 'reserved'
		`
		if _, err := tx.Exec(ctx, query, paymentID, now); err != nil {
			return nil, fmt.Errorf("failed to consume reservation: %w", err)
		}
		reservation.Status = entity.CoinReservationStatusConsumed
		reservation.ConsumedAt = &now
		reservation.UpdatedAt = now
		return reservation, nil
	default:
		return nil, fmt.Errorf("unknown reservation status: %s", reservation.Status)
	}
}

// ReleaseReservation transitions a reservation from reserved to released.
func (r *CoinsRepositoryImpl) ReleaseReservation(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*entity.CoinReservation, error) {
	reservation, err := r.loadReservationByPaymentID(ctx, tx, paymentID, true)
	if err != nil {
		return nil, err
	}
	if reservation == nil {
		return nil, nil
	}

	switch reservation.Status {
	case entity.CoinReservationStatusReleased:
		return nil, nil
	case entity.CoinReservationStatusConsumed:
		return nil, &entity.ErrReservationAlreadyConsumed{PaymentID: paymentID}
	case entity.CoinReservationStatusReserved:
		now := time.Now()
		query := `
			UPDATE coin_reservations
			SET status = 'released',
			    released_at = $2,
			    updated_at = $2
			WHERE payment_id = $1
			  AND status = 'reserved'
		`
		if _, err := tx.Exec(ctx, query, paymentID, now); err != nil {
			return nil, fmt.Errorf("failed to release reservation: %w", err)
		}
		reservation.Status = entity.CoinReservationStatusReleased
		reservation.ReleasedAt = &now
		reservation.UpdatedAt = now
		return reservation, nil
	default:
		return nil, fmt.Errorf("unknown reservation status: %s", reservation.Status)
	}
}

// ============================================================================
// ERROR TYPES
// ============================================================================

// ErrDuplicateTransaction is returned when attempting to create a transaction
// that already exists (violates UNIQUE constraint on user_id, reference_type, reference_id).
// This should be treated as idempotent success, not failure.
type ErrDuplicateTransaction struct {
	UserID        uuid.UUID
	ReferenceType entity.CoinReferenceType
	ReferenceID   *uuid.UUID
	Err           error
}

func (e *ErrDuplicateTransaction) Error() string {
	if e.ReferenceID != nil {
		return fmt.Sprintf("duplicate coins transaction: user=%s reference_type=%s reference_id=%s",
			e.UserID, e.ReferenceType, *e.ReferenceID)
	}
	return fmt.Sprintf("duplicate coins transaction: user=%s reference_type=%s",
		e.UserID, e.ReferenceType)
}

func (e *ErrDuplicateTransaction) Unwrap() error {
	return e.Err
}

// IsDuplicateTransaction checks if an error is ErrDuplicateTransaction.
func IsDuplicateTransaction(err error) bool {
	_, ok := err.(*ErrDuplicateTransaction)
	return ok
}

// isDuplicateKeyError checks if the error is a PostgreSQL unique constraint violation.
// PostgreSQL error code 23505 = "unique_violation"
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// Check for pgx.Error with SQLState 23505 (unique_violation)
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	// Fallback: check error message (case-insensitive)
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "unique constraint") ||
		strings.Contains(errMsg, "duplicate key")
}

// ============================================================================
// AGGREGATE BALANCE OPERATIONS (ATOMIC CONCURRENCY CONTROL)
// ============================================================================

// EnsureBalanceRow ensures a user has a balance row (creates with 0 if not exists).
func (r *CoinsRepositoryImpl) EnsureBalanceRow(ctx context.Context, tx db.Tx, userID uuid.UUID) error {
	query := `
		INSERT INTO user_coin_balance (user_id, balance, version)
		VALUES ($1, 0, 1)
		ON CONFLICT (user_id) DO NOTHING
	`
	_, err := tx.Exec(ctx, query, userID)
	return err
}

// GetBalanceRow retrieves the current aggregate balance for a user.
func (r *CoinsRepositoryImpl) GetBalanceRow(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.UserCoinBalance, error) {
	query := `
		SELECT user_id, balance, version, created_at, updated_at
		FROM user_coin_balance
		WHERE user_id = $1
	`
	var balance entity.UserCoinBalance
	err := tx.QueryRow(ctx, query, userID).Scan(
		&balance.UserID,
		&balance.Balance,
		&balance.Version,
		&balance.CreatedAt,
		&balance.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No balance row exists
		}
		return nil, fmt.Errorf("failed to get balance row: %w", err)
	}
	return &balance, nil
}

// AtomicDeductBalance performs an atomic deduct operation on the aggregate balance.
//
// CRITICAL: This is the PRIMARY concurrency control mechanism for coin spending.
//
// The UPDATE with WHERE clause is guaranteed atomic by the database:
// - Single row lock (user_id PK) = no deadlock risk
// - WHERE balance >= amount = no overspending
// - Returns rows_affected = success/failure indicator
//
// If rows_affected == 0: insufficient funds (balance < amount)
// If rows_affected == 1: success (balance deducted atomically)
func (r *CoinsRepositoryImpl) AtomicDeductBalance(ctx context.Context, tx db.Tx, userID uuid.UUID, amount int64) (int64, error) {
	query := `
		UPDATE user_coin_balance
		SET balance = balance - $1,
		    updated_at = NOW(),
		    version = version + 1
		WHERE user_id = $2
		  AND balance >= $1
	`
	result, err := tx.Exec(ctx, query, amount, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to atomic deduct balance: %w", err)
	}
	rowsAffected := result.RowsAffected()
	return rowsAffected, nil
}

// AtomicAddBalance adds coins to the user's aggregate balance.
//
// Used when coins are earned (order completion, grants, etc).
// Returns rows affected (1 if row exists, 0 if not).
func (r *CoinsRepositoryImpl) AtomicAddBalance(ctx context.Context, tx db.Tx, userID uuid.UUID, amount int64) (int64, error) {
	query := `
		UPDATE user_coin_balance
		SET balance = balance + $1,
		    updated_at = NOW(),
		    version = version + 1
		WHERE user_id = $2
	`
	result, err := tx.Exec(ctx, query, amount, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to atomic add balance: %w", err)
	}
	return result.RowsAffected(), nil
}

// ReconcileBalance compares aggregate balance with derived balance from transactions.
// Returns the difference (positive = aggregate is higher, negative = aggregate is lower).
func (r *CoinsRepositoryImpl) ReconcileBalance(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error) {
	// Get aggregate balance
	balanceRow, err := r.GetBalanceRow(ctx, tx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance row: %w", err)
	}
	if balanceRow == nil {
		// No balance row - difference is full derived balance (should be 0)
		return 0, nil
	}

	// Calculate derived balance from transactions
	derivedBalance, err := r.GetActiveBalance(ctx, tx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get derived balance: %w", err)
	}

	return balanceRow.Balance - derivedBalance, nil
}
