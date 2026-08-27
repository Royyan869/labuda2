package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	finrepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
)

// DisputeFreezeRepository implements finrepo.DisputeFreezeRepository
// using the transaction-scoped db.Tx pattern (no stored pool).
type DisputeFreezeRepository struct{}

// NewDisputeFreezeRepository creates a new DisputeFreezeRepository.
func NewDisputeFreezeRepository() *DisputeFreezeRepository {
	return &DisputeFreezeRepository{}
}

// Create inserts a new active dispute_freeze row.
// Returns a wrapped error if the UNIQUE constraint on dispute_id fires (duplicate).
func (r *DisputeFreezeRepository) Create(
	ctx context.Context,
	tx db.Tx,
	freeze *finrepo.DisputeFreeze,
) error {
	now := time.Now().UnixMilli()
	_, err := tx.Exec(ctx, `
		INSERT INTO dispute_freezes
			(id, dispute_id, order_id, seller_id, frozen_amount, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'active', $6, $6)
	`, freeze.ID, freeze.DisputeID, freeze.OrderID, freeze.SellerID, freeze.FrozenAmount, now)
	if err != nil {
		return fmt.Errorf("dispute_freeze: create failed: %w", err)
	}
	return nil
}

// GetByDisputeID fetches the freeze row for the given dispute, or nil if none.
func (r *DisputeFreezeRepository) GetByDisputeID(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
) (*finrepo.DisputeFreeze, error) {
	var f finrepo.DisputeFreeze
	err := tx.QueryRow(ctx, `
		SELECT id, dispute_id, order_id, seller_id, frozen_amount, status, created_at, updated_at
		FROM dispute_freezes WHERE dispute_id = $1
	`, disputeID).Scan(
		&f.ID, &f.DisputeID, &f.OrderID, &f.SellerID,
		&f.FrozenAmount, &f.Status, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("dispute_freeze: get by dispute_id failed: %w", err)
	}
	return &f, nil
}

// Release transitions an active freeze to released. Idempotent on already-released rows.
func (r *DisputeFreezeRepository) Release(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
) error {
	now := time.Now().UnixMilli()
	_, err := tx.Exec(ctx, `
		UPDATE dispute_freezes
		SET status = 'released', updated_at = $2
		WHERE dispute_id = $1 AND status = 'active'
	`, disputeID, now)
	if err != nil {
		return fmt.Errorf("dispute_freeze: release failed: %w", err)
	}
	return nil
}

// ReleaseByOrderID transitions any active freeze for the order to released.
// Idempotent: no-op if no active freeze exists.
func (r *DisputeFreezeRepository) ReleaseByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	now := time.Now().UnixMilli()
	_, err := tx.Exec(ctx, `
		UPDATE dispute_freezes
		SET status = 'released', updated_at = $2
		WHERE order_id = $1 AND status = 'active'
	`, orderID, now)
	if err != nil {
		return fmt.Errorf("dispute_freeze: release by order_id failed: %w", err)
	}
	return nil
}

// GetTotalActiveBySeller sums frozen_amount for all active freezes for the seller.
func (r *DisputeFreezeRepository) GetTotalActiveBySeller(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (int64, error) {
	var total int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(frozen_amount), 0)
		FROM dispute_freezes
		WHERE seller_id = $1 AND status = 'active'
	`, sellerID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("dispute_freeze: get total active failed: %w", err)
	}
	return total, nil
}


