package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/core/wallet/entity"
	"github.com/labuda/backend/pkg/db"
)

// EscrowRepository defines the interface for escrow persistence operations.
type EscrowRepository interface {
	// GetByID retrieves an escrow by its ID.
	// Returns nil if not found.
	GetByID(ctx context.Context, tx db.Tx, escrowID uuid.UUID) (*entity.Escrow, error)

	// GetByOrderID retrieves an escrow by order ID.
	// Returns nil if not found.
	GetByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Escrow, error)

	// GetByOrderIDForUpdate retrieves an escrow by order ID with row-level lock.
	//
	// CRITICAL: This method MUST be used before escrow status mutations to prevent race conditions.
	// The FOR UPDATE clause locks the row until the transaction commits or rolls back.
	//
	// Usage:
	//   1. Begin transaction
	//   2. Call GetByOrderIDForUpdate (acquires lock)
	//   3. Validate status
	//   4. Update status
	//   5. Commit transaction (releases lock)
	GetByOrderIDForUpdate(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Escrow, error)

	// Create creates a new escrow.
	//
	// HARD IDEMPOTENCY:
	// Uses UNIQUE constraint on order_id to prevent duplicate escrows.
	// Returns ErrEscrowAlreadyExists if escrow already exists for the order.
	Create(ctx context.Context, tx db.Tx, escrow *entity.Escrow) error

	// Update updates an escrow (status changes).
	// Used for release and refund operations.
	Update(ctx context.Context, tx db.Tx, escrow *entity.Escrow) error

	// GetByBuyerWalletID retrieves active escrows for a buyer wallet.
	// Returns escrows in HOLDING status.
	GetByBuyerWalletID(ctx context.Context, tx db.Tx, buyerWalletID uuid.UUID) ([]*entity.Escrow, error)

	// GetBySellerWalletID retrieves active escrows for a seller wallet.
	// Returns escrows in HOLDING status.
	GetBySellerWalletID(ctx context.Context, tx db.Tx, sellerWalletID uuid.UUID) ([]*entity.Escrow, error)
}


