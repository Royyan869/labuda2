package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	walletrepo "github.com/labuda/backend/internal/core/wallet/repository"
	"github.com/labuda/backend/internal/core/wallet/entity"
	"github.com/labuda/backend/pkg/db"
)

// WalletRepositoryImpl implements WalletRepository using PostgreSQL.
type WalletRepositoryImpl struct{}

// NewWalletRepository creates a new WalletRepositoryImpl.
func NewWalletRepository() walletrepo.WalletRepository {
	return &WalletRepositoryImpl{}
}

// ============================================================================
// QUERY OPERATIONS
// ============================================================================

// GetByUserID retrieves a wallet by user ID.
func (r *WalletRepositoryImpl) GetByUserID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.Wallet, error) {
	query := `
		SELECT id, user_id, available_balance, held_balance, created_at, updated_at
		FROM wallets
		WHERE user_id = $1
	`

	var wallet entity.Wallet
	err := tx.QueryRow(ctx, query, userID).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.AvailableBalance,
		&wallet.HeldBalance,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Wallet not found
		}
		return nil, fmt.Errorf("failed to get wallet by user ID: %w", err)
	}

	return &wallet, nil
}

// GetWalletForUpdate retrieves a wallet by user ID with row-level lock.
//
// CRITICAL: This method MUST be used before balance mutations to prevent race conditions.
// The FOR UPDATE clause locks the row until the transaction commits or rolls back.
//
// Usage:
//   1. Begin transaction
//   2. Call GetWalletForUpdate (acquires lock)
//   3. Check balance
//   4. Update balance
//   5. Commit transaction (releases lock)
func (r *WalletRepositoryImpl) GetWalletForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.Wallet, error) {
	query := `
		SELECT id, user_id, available_balance, held_balance, created_at, updated_at
		FROM wallets
		WHERE user_id = $1
		FOR UPDATE
	`

	var wallet entity.Wallet
	err := tx.QueryRow(ctx, query, userID).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.AvailableBalance,
		&wallet.HeldBalance,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Wallet not found
		}
		return nil, fmt.Errorf("failed to get wallet for update: %w", err)
	}

	return &wallet, nil
}

// GetByID retrieves a wallet by its ID.
func (r *WalletRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, walletID uuid.UUID) (*entity.Wallet, error) {
	query := `
		SELECT id, user_id, available_balance, held_balance, created_at, updated_at
		FROM wallets
		WHERE id = $1
	`

	var wallet entity.Wallet
	err := tx.QueryRow(ctx, query, walletID).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.AvailableBalance,
		&wallet.HeldBalance,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Wallet not found
		}
		return nil, fmt.Errorf("failed to get wallet by ID: %w", err)
	}

	return &wallet, nil
}

// ============================================================================
// CREATE/UPDATE OPERATIONS
// ============================================================================

// Create creates a new wallet.
func (r *WalletRepositoryImpl) Create(ctx context.Context, tx db.Tx, wallet *entity.Wallet) error {
	query := `
		INSERT INTO wallets (id, user_id, available_balance, held_balance, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := tx.Exec(ctx, query,
		wallet.ID,
		wallet.UserID,
		wallet.AvailableBalance,
		wallet.HeldBalance,
		wallet.CreatedAt,
		wallet.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create wallet: %w", err)
	}

	return nil
}

// Update updates wallet balances.
// Use AtomicUpdateAvailableBalance for balance changes to prevent double-spend.
func (r *WalletRepositoryImpl) Update(ctx context.Context, tx db.Tx, wallet *entity.Wallet) error {
	query := `
		UPDATE wallets
		SET available_balance = $1, held_balance = $2, updated_at = NOW()
		WHERE id = $3
	`

	_, err := tx.Exec(ctx, query,
		wallet.AvailableBalance,
		wallet.HeldBalance,
		wallet.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update wallet: %w", err)
	}

	return nil
}

// ============================================================================
// ATOMIC OPERATIONS (CONCURRENCY CONTROL)
// ============================================================================

// AtomicUpdateAvailableBalance atomically deducts from available balance.
//
// CRITICAL: This is the PRIMARY concurrency control mechanism for preventing double-spend.
//
// The UPDATE with WHERE clause is guaranteed atomic by the database:
// - Single row lock (id PK) = no deadlock risk
// - WHERE available_balance >= amount = no overspending
// - Returns rows_affected = success/failure indicator
//
// If rows_affected == 0: insufficient funds (available_balance < amount)
// If rows_affected == 1: success (balance deducted atomically)
func (r *WalletRepositoryImpl) AtomicUpdateAvailableBalance(ctx context.Context, tx db.Tx, walletID uuid.UUID, amount int64) (int64, error) {
	query := `
		UPDATE wallets
		SET available_balance = available_balance - $1,
		    updated_at = NOW()
		WHERE id = $2
		  AND available_balance >= $1
	`

	result, err := tx.Exec(ctx, query, amount, walletID)
	if err != nil {
		return 0, fmt.Errorf("failed to atomic update available balance: %w", err)
	}

	return result.RowsAffected(), nil
}

// AtomicAddToHeldBalance atomically adds to held balance.
// Used when holding funds for escrow.
func (r *WalletRepositoryImpl) AtomicAddToHeldBalance(ctx context.Context, tx db.Tx, walletID uuid.UUID, amount int64) (int64, error) {
	query := `
		UPDATE wallets
		SET held_balance = held_balance + $1,
		    updated_at = NOW()
		WHERE id = $2
	`

	result, err := tx.Exec(ctx, query, amount, walletID)
	if err != nil {
		return 0, fmt.Errorf("failed to atomic add to held balance: %w", err)
	}

	return result.RowsAffected(), nil
}

// AtomicSubtractFromHeldBalance atomically subtracts from held balance.
// Used when releasing or refunding escrow.
//
// The WHERE held_balance >= amount clause prevents negative held_balance.
// If rows_affected == 0: insufficient held balance
// If rows_affected == 1: success
func (r *WalletRepositoryImpl) AtomicSubtractFromHeldBalance(ctx context.Context, tx db.Tx, walletID uuid.UUID, amount int64) (int64, error) {
	query := `
		UPDATE wallets
		SET held_balance = held_balance - $1,
		    updated_at = NOW()
		WHERE id = $2
		  AND held_balance >= $1
	`

	result, err := tx.Exec(ctx, query, amount, walletID)
	if err != nil {
		return 0, fmt.Errorf("failed to atomic subtract from held balance: %w", err)
	}

	return result.RowsAffected(), nil
}

// AtomicAddToAvailableBalance atomically adds to available balance.
// Used when refunding escrow to buyer.
func (r *WalletRepositoryImpl) AtomicAddToAvailableBalance(ctx context.Context, tx db.Tx, walletID uuid.UUID, amount int64) (int64, error) {
	query := `
		UPDATE wallets
		SET available_balance = available_balance + $1,
		    updated_at = NOW()
		WHERE id = $2
	`

	result, err := tx.Exec(ctx, query, amount, walletID)
	if err != nil {
		return 0, fmt.Errorf("failed to atomic add to available balance: %w", err)
	}

	return result.RowsAffected(), nil
}

// TransferHeldToAvailable atomically transfers funds from one wallet's held balance
// to another wallet's available balance.
//
// CRITICAL: This uses a SINGLE SQL statement with CTEs to ensure:
// 1. ATOMICITY: Both updates succeed or both fail - no split state
// 2. DEADLOCK PREVENTION: Locks are acquired in deterministic order by wallet ID
//
// The method determines which wallet ID is smaller and constructs the query
// to always lock the smaller ID first, preventing circular wait conditions.
func (r *WalletRepositoryImpl) TransferHeldToAvailable(ctx context.Context, tx db.Tx, fromWalletID, toWalletID uuid.UUID, amount int64) (sourceUpdated, destUpdated bool, err error) {
	// DEADLOCK PREVENTION: Always lock wallets in deterministic order
	// by comparing UUIDs and updating the smaller ID first.
	var firstID, secondID uuid.UUID
	var firstIsSource bool // true if firstID is the source wallet

	// Compare UUIDs to determine lock order
	if fromWalletID.String() < toWalletID.String() {
		firstID = fromWalletID
		secondID = toWalletID
		firstIsSource = true
	} else {
		firstID = toWalletID
		secondID = fromWalletID
		firstIsSource = false
	}

	// Build query with deterministic lock order
	// The first CTE always updates the smaller wallet ID
	// The second CTE always updates the larger wallet ID
	// This ensures locks are always acquired in the same order
	query := `
		WITH first_update AS (
			UPDATE wallets
			SET
				-- Update held balance if this is the source wallet
				held_balance = CASE WHEN $1 = $4 THEN held_balance - $5 ELSE held_balance END,
				-- Update available balance if this is the destination wallet
				available_balance = CASE WHEN $1 = $3 THEN available_balance + $5 ELSE available_balance END,
				updated_at = NOW()
			WHERE id = $1
				-- If source wallet: check sufficient held balance
				AND ($1 != $4 OR held_balance >= $5)
			RETURNING id
		),
		second_update AS (
			UPDATE wallets
			SET
				-- Update held balance if this is the source wallet
				held_balance = CASE WHEN $2 = $4 THEN held_balance - $5 ELSE held_balance END,
				-- Update available balance if this is the destination wallet
				available_balance = CASE WHEN $2 = $3 THEN available_balance + $5 ELSE available_balance END,
				updated_at = NOW()
			WHERE id = $2
				-- If source wallet: check sufficient held balance
				AND ($2 != $4 OR held_balance >= $5)
			RETURNING id
		)
		SELECT
			EXISTS(SELECT 1 FROM first_update) AS first_updated,
			EXISTS(SELECT 1 FROM second_update) AS second_updated
	`

	// Parameters:
	// $1 = firstID (smaller wallet ID)
	// $2 = secondID (larger wallet ID)
	// $3 = toWalletID (destination wallet ID)
	// $4 = fromWalletID (source wallet ID)
	// $5 = amount
	var firstUpdated, secondUpdated bool
	err = tx.QueryRow(ctx, query, firstID, secondID, toWalletID, fromWalletID, amount).Scan(&firstUpdated, &secondUpdated)
	if err != nil {
		return false, false, fmt.Errorf("failed to atomic transfer held to available: %w", err)
	}

	// Map results back to source/destination based on which wallet was updated first
	if firstIsSource {
		sourceUpdated = firstUpdated
		destUpdated = secondUpdated
	} else {
		sourceUpdated = secondUpdated
		destUpdated = firstUpdated
	}

	return sourceUpdated, destUpdated, nil
}

// AtomicTransferHeldToAvailableInWallet atomically transfers funds within
// a single wallet from held balance to available balance.
//
// CRITICAL: This uses a SINGLE SQL UPDATE to ensure atomicity.
// Both balances are updated in one operation - no split state possible.
//
// This is used for refunds where funds move from held -> available in the same wallet.
func (r *WalletRepositoryImpl) AtomicTransferHeldToAvailableInWallet(ctx context.Context, tx db.Tx, walletID uuid.UUID, amount int64) (int64, error) {
	query := `
		UPDATE wallets
		SET held_balance = held_balance - $1,
		    available_balance = available_balance + $1,
		    updated_at = NOW()
		WHERE id = $2
		  AND held_balance >= $1
	`

	result, err := tx.Exec(ctx, query, amount, walletID)
	if err != nil {
		return 0, fmt.Errorf("failed to atomic transfer held to available in wallet: %w", err)
	}

	return result.RowsAffected(), nil
}

// EnsureWallet ensures a user has a wallet (creates with 0 balance if not exists).
// This is idempotent - safe to call multiple times.
//
// Uses INSERT ... ON CONFLICT DO NOTHING pattern to prevent race condition
// on wallet creation between concurrent calls.
func (r *WalletRepositoryImpl) EnsureWallet(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.Wallet, error) {
	// Try to insert new wallet with ON CONFLICT DO NOTHING
	// This prevents race condition - if wallet already exists, insert does nothing
	newWallet := entity.NewWallet(userID)
	insertQuery := `
		INSERT INTO wallets (id, user_id, available_balance, held_balance, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING id, user_id, available_balance, held_balance, created_at, updated_at
	`

	var wallet entity.Wallet
	err := tx.QueryRow(ctx, insertQuery,
		newWallet.ID,
		newWallet.UserID,
		newWallet.AvailableBalance,
		newWallet.HeldBalance,
		newWallet.CreatedAt,
		newWallet.UpdatedAt,
	).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.AvailableBalance,
		&wallet.HeldBalance,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err == nil {
		// Insert succeeded - return the new wallet
		return &wallet, nil
	}

	// If insert returned no rows (conflict), fetch existing wallet
	if errors.Is(err, pgx.ErrNoRows) {
		return r.GetByUserID(ctx, tx, userID)
	}

	// Other error
	return nil, fmt.Errorf("failed to ensure wallet: %w", err)
}


