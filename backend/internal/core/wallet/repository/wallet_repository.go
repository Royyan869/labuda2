package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/core/wallet/entity"
	"github.com/labuda/backend/pkg/db"
)

// WalletRepository defines the interface for wallet persistence operations.
type WalletRepository interface {
	// GetByUserID retrieves a wallet by user ID.
	// Returns nil if not found.
	GetByUserID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.Wallet, error)

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
	GetWalletForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.Wallet, error)

	// GetByID retrieves a wallet by its ID.
	// Returns nil if not found.
	GetByID(ctx context.Context, tx db.Tx, walletID uuid.UUID) (*entity.Wallet, error)

	// Create creates a new wallet.
	Create(ctx context.Context, tx db.Tx, wallet *entity.Wallet) error

	// Update updates wallet balances.
	// Use AtomicUpdateAvailableBalance for balance changes to prevent double-spend.
	Update(ctx context.Context, tx db.Tx, wallet *entity.Wallet) error

	// AtomicUpdateAvailableBalance atomically deducts from available balance.
	// This is the PRIMARY concurrency control mechanism for preventing double-spend.
	//
	// SQL: UPDATE wallets SET available_balance = available_balance - $1
	//      WHERE id = $2 AND available_balance >= $1
	//
	// Returns:
	//   - rowsAffected: 1 if success, 0 if insufficient funds
	//   - err: database error on failure
	AtomicUpdateAvailableBalance(ctx context.Context, tx db.Tx, walletID uuid.UUID, amount int64) (int64, error)

	// AtomicAddToHeldBalance atomically adds to held balance.
	// Used when holding funds for escrow.
	//
	// SQL: UPDATE wallets SET held_balance = held_balance + $1
	//      WHERE id = $2
	//
	// Returns rows affected (1 if success, 0 if wallet not found).
	AtomicAddToHeldBalance(ctx context.Context, tx db.Tx, walletID uuid.UUID, amount int64) (int64, error)

	// AtomicSubtractFromHeldBalance atomically subtracts from held balance.
	// Used when releasing or refunding escrow.
	//
	// SQL: UPDATE wallets SET held_balance = held_balance - $1
	//      WHERE id = $2 AND held_balance >= $1
	//
	// Returns rows affected (1 if success, 0 if insufficient held balance).
	AtomicSubtractFromHeldBalance(ctx context.Context, tx db.Tx, walletID uuid.UUID, amount int64) (int64, error)

	// AtomicAddToAvailableBalance atomically adds to available balance.
	// Used when refunding escrow to buyer.
	//
	// SQL: UPDATE wallets SET available_balance = available_balance + $1
	//      WHERE id = $2
	//
	// Returns rows affected (1 if success, 0 if wallet not found).
	AtomicAddToAvailableBalance(ctx context.Context, tx db.Tx, walletID uuid.UUID, amount int64) (int64, error)

	// TransferHeldToAvailable atomically transfers funds from one wallet's held balance
	// to another wallet's available balance.
	//
	// CRITICAL: This is a SINGLE atomic operation that prevents:
	// - Split balance updates (no inconsistent state)
	// - Deadlocks (deterministic lock ordering)
	//
	// DEADLOCK PREVENTION:
	// - Wallets are locked in DETERMINISTIC ORDER by sorting wallet IDs
	// - Always locks smaller ID first, then larger ID
	// - This prevents circular wait conditions
	//
	// SQL (single CTE operation):
	//   WITH source_update AS (
	//     UPDATE wallets SET held_balance = held_balance - $3
	//     WHERE id = $1 AND held_balance >= $3
	//     RETURNING id
	//   ),
	//   dest_update AS (
	//     UPDATE wallets SET available_balance = available_balance + $3
	//     WHERE id = $2
	//     RETURNING id
	//   )
	//   SELECT EXISTS(SELECT 1 FROM source_update) AS source_updated,
	//          EXISTS(SELECT 1 FROM dest_update) AS dest_updated
	//
	// Returns:
	//   - sourceUpdated: true if source wallet had sufficient held balance
	//   - destUpdated: true if dest wallet exists
	//   - err: database error on failure
	//
	// If sourceUpdated is false: insufficient held balance
	// If destUpdated is false: destination wallet not found
	TransferHeldToAvailable(ctx context.Context, tx db.Tx, fromWalletID, toWalletID uuid.UUID, amount int64) (sourceUpdated, destUpdated bool, err error)

	// AtomicTransferHeldToAvailableInWallet atomically transfers funds within
	// a single wallet from held balance to available balance.
	//
	// CRITICAL: This is a SINGLE atomic operation that prevents split state.
	// Used for refunds where funds move from held -> available in the same wallet.
	//
	// SQL (single UPDATE):
	//   UPDATE wallets
	//   SET held_balance = held_balance - $2,
	//       available_balance = available_balance + $2,
	//       updated_at = NOW()
	//   WHERE id = $1 AND held_balance >= $2
	//
	// Returns:
	//   - rowsAffected: 1 if success, 0 if insufficient held balance
	//   - err: database error on failure
	//
	// If rowsAffected == 0: insufficient held balance
	// If rowsAffected == 1: success (both balances updated atomically)
	AtomicTransferHeldToAvailableInWallet(ctx context.Context, tx db.Tx, walletID uuid.UUID, amount int64) (rowsAffected int64, err error)

	// EnsureWallet ensures a user has a wallet (creates with 0 balance if not exists).
	// This is idempotent - safe to call multiple times.
	EnsureWallet(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.Wallet, error)
}


