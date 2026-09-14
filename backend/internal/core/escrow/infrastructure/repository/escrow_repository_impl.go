package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/core/escrow/entity"
	escrowrepo "github.com/labuda/backend/internal/core/escrow/repository"
	"github.com/labuda/backend/pkg/db"
)

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
	return false
}

// isNoRowsError checks if the error is a "no rows" error.
func isNoRowsError(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// EscrowRepositoryImpl implements EscrowRepository using PostgreSQL.
type EscrowRepositoryImpl struct{}

// NewEscrowRepository creates a new EscrowRepositoryImpl.
func NewEscrowRepository() escrowrepo.EscrowRepository {
	return &EscrowRepositoryImpl{}
}

// ============================================================================
// QUERY OPERATIONS
// ============================================================================

const escrowSelectColumns = `id, order_id, amount, status,
	payment_id, created_at, released_at, refunded_at`

func scanEscrow(row pgx.Row) (*entity.Escrow, error) {
	var escrow entity.Escrow
	err := row.Scan(
		&escrow.ID,
		&escrow.OrderID,
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
			id, order_id, amount, status,
			created_at, released_at, refunded_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7
		)
	`

	_, err := tx.Exec(ctx, query,
		escrow.ID,
		escrow.OrderID,
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
