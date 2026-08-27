package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/core/wallet/entity"
	walletrepo "github.com/labuda/backend/internal/core/wallet/repository"
	"github.com/labuda/backend/pkg/db"
)

// EscrowRepositoryImpl implements EscrowRepository using PostgreSQL.
type EscrowRepositoryImpl struct{}

// NewEscrowRepository creates a new EscrowRepositoryImpl.
func NewEscrowRepository() walletrepo.EscrowRepository {
	return &EscrowRepositoryImpl{}
}

// ============================================================================
// QUERY OPERATIONS
// ============================================================================

const escrowSelectColumns = `id, order_id, buyer_wallet_id, seller_wallet_id, amount, status,
	payment_id, created_at, released_at, refunded_at`

func scanEscrow(row pgx.Row) (*entity.Escrow, error) {
	var escrow entity.Escrow
	err := row.Scan(
		&escrow.ID,
		&escrow.OrderID,
		&escrow.BuyerWalletID,
		&escrow.SellerWalletID,
		&escrow.Amount,
		&escrow.Status,
		&escrow.PaymentID,
		&escrow.CreatedAt,
		&escrow.ReleasedAt,
		&escrow.RefundedAt,
	)
	if err != nil {
		return nil, err
	}
	return &escrow, nil
}

// GetByID retrieves an escrow by its ID.
func (r *EscrowRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, escrowID uuid.UUID) (*entity.Escrow, error) {
	query := `SELECT ` + escrowSelectColumns + ` FROM escrows WHERE id = $1`

	escrow, err := scanEscrow(tx.QueryRow(ctx, query, escrowID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get escrow by ID: %w", err)
	}
	return escrow, nil
}

// GetByOrderID retrieves an escrow by order ID.
func (r *EscrowRepositoryImpl) GetByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Escrow, error) {
	query := `SELECT ` + escrowSelectColumns + ` FROM escrows WHERE order_id = $1`

	escrow, err := scanEscrow(tx.QueryRow(ctx, query, orderID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get escrow by order ID: %w", err)
	}
	return escrow, nil
}

// GetByOrderIDForUpdate retrieves an escrow by order ID with row-level lock.
//
// CRITICAL: This method MUST be used before escrow status mutations to prevent race conditions.
// The FOR UPDATE clause locks the row until the transaction commits or rolls back.
func (r *EscrowRepositoryImpl) GetByOrderIDForUpdate(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Escrow, error) {
	query := `SELECT ` + escrowSelectColumns + ` FROM escrows WHERE order_id = $1 FOR UPDATE`

	escrow, err := scanEscrow(tx.QueryRow(ctx, query, orderID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get escrow by order ID for update: %w", err)
	}
	return escrow, nil
}

// ============================================================================
// CREATE/UPDATE OPERATIONS
// ============================================================================

// Create creates a new escrow.
//
// HARD IDEMPOTENCY:
// Uses UNIQUE constraint on order_id to prevent duplicate escrows.
// Returns ErrEscrowAlreadyExists if escrow already exists for the order.
func (r *EscrowRepositoryImpl) Create(ctx context.Context, tx db.Tx, escrow *entity.Escrow) error {
	query := `
		INSERT INTO escrows (
			id, order_id, buyer_wallet_id, seller_wallet_id, amount, status,
			created_at, released_at, refunded_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9
		)
	`

	_, err := tx.Exec(ctx, query,
		escrow.ID,
		escrow.OrderID,
		escrow.BuyerWalletID,
		escrow.SellerWalletID,
		escrow.Amount,
		escrow.Status,
		escrow.CreatedAt,
		escrow.ReleasedAt,
		escrow.RefundedAt,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return &entity.ErrEscrowAlreadyExists{OrderID: escrow.OrderID}
		}
		return fmt.Errorf("failed to create escrow: %w", err)
	}

	return nil
}

// Update updates an escrow (status changes).
// Used for release and refund operations.
func (r *EscrowRepositoryImpl) Update(ctx context.Context, tx db.Tx, escrow *entity.Escrow) error {
	query := `
		UPDATE escrows
		SET status = $1, released_at = $2, refunded_at = $3
		WHERE id = $4
	`

	_, err := tx.Exec(ctx, query,
		escrow.Status,
		escrow.ReleasedAt,
		escrow.RefundedAt,
		escrow.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update escrow: %w", err)
	}

	return nil
}

// ============================================================================
// ACTIVE ESCROW QUERIES
// ============================================================================

// GetByBuyerWalletID retrieves active escrows for a buyer wallet.
// Returns escrows in HOLDING status.
func (r *EscrowRepositoryImpl) GetByBuyerWalletID(ctx context.Context, tx db.Tx, buyerWalletID uuid.UUID) ([]*entity.Escrow, error) {
	query := `SELECT ` + escrowSelectColumns + `
		FROM escrows
		WHERE buyer_wallet_id = $1
		  AND status = 'holding'
		ORDER BY created_at DESC`

	rows, err := tx.Query(ctx, query, buyerWalletID)
	if err != nil {
		return nil, fmt.Errorf("failed to query escrows by buyer wallet: %w", err)
	}
	defer rows.Close()

	var escrows []*entity.Escrow
	for rows.Next() {
		escrow, err := scanEscrow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan escrow: %w", err)
		}
		escrows = append(escrows, escrow)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating escrows: %w", rows.Err())
	}

	return escrows, nil
}

// GetBySellerWalletID retrieves active escrows for a seller wallet.
// Returns escrows in HOLDING status.
func (r *EscrowRepositoryImpl) GetBySellerWalletID(ctx context.Context, tx db.Tx, sellerWalletID uuid.UUID) ([]*entity.Escrow, error) {
	query := `SELECT ` + escrowSelectColumns + `
		FROM escrows
		WHERE seller_wallet_id = $1
		  AND status = 'holding'
		ORDER BY created_at DESC`

	rows, err := tx.Query(ctx, query, sellerWalletID)
	if err != nil {
		return nil, fmt.Errorf("failed to query escrows by seller wallet: %w", err)
	}
	defer rows.Close()

	var escrows []*entity.Escrow
	for rows.Next() {
		escrow, err := scanEscrow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan escrow: %w", err)
		}
		escrows = append(escrows, escrow)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating escrows: %w", rows.Err())
	}

	return escrows, nil
}


