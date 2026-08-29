package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/pricing/discount/entity"
	"github.com/labuda/backend/pkg/db"
)

// DiscountRepositoryImpl implements DiscountRepository using PostgreSQL.
type DiscountRepositoryImpl struct{}

// NewDiscountRepository creates a DiscountRepositoryImpl.
func NewDiscountRepository() *DiscountRepositoryImpl {
	return &DiscountRepositoryImpl{}
}

// Create creates a new discount.
func (r *DiscountRepositoryImpl) Create(ctx context.Context, tx db.Tx, discount *entity.Discount) error {
	query := `
		INSERT INTO discounts (
			id, code, type, value, min_purchase, applies_to, seller_id,
			valid_until, total_usage_limit, current_usage_count,
			is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10,
			$11, $12, $13
		)
	`

	_, err := tx.Exec(ctx, query,
		discount.ID,
		discount.Code,
		discount.Type,
		discount.Value,
		discount.MinPurchase,
		discount.AppliesTo,
		discount.SellerID,
		discount.ValidUntil,
		discount.TotalUsageLimit,
		discount.CurrentUsageCount,
		discount.IsActive,
		discount.CreatedAt,
		discount.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert discount: %w", err)
	}

	return nil
}

// Update updates an existing discount.
func (r *DiscountRepositoryImpl) Update(ctx context.Context, tx db.Tx, discount *entity.Discount) error {
	query := `
		UPDATE discounts SET
			code = $2,
			type = $3,
			value = $4,
			min_purchase = $5,
			applies_to = $6,
			seller_id = $7,
			valid_until = $8,
			total_usage_limit = $9,
			current_usage_count = $10,
			is_active = $11,
			updated_at = $12
		WHERE id = $1
	`

	result, err := tx.Exec(ctx, query,
		discount.ID,
		discount.Code,
		discount.Type,
		discount.Value,
		discount.MinPurchase,
		discount.AppliesTo,
		discount.SellerID,
		discount.ValidUntil,
		discount.TotalUsageLimit,
		discount.CurrentUsageCount,
		discount.IsActive,
		discount.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update discount: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("discount not found: id=%s", discount.ID)
	}

	return nil
}

// GetByID retrieves a discount by ID.
func (r *DiscountRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Discount, error) {
	query := `
		SELECT id, code, type, value, min_purchase, applies_to, seller_id,
		       valid_until, total_usage_limit, current_usage_count,
		       is_active, created_at, updated_at
		FROM discounts
		WHERE id = $1
	`

	var discount entity.Discount
	err := tx.QueryRow(ctx, query, id).Scan(
		&discount.ID,
		&discount.Code,
		&discount.Type,
		&discount.Value,
		&discount.MinPurchase,
		&discount.AppliesTo,
		&discount.SellerID,
		&discount.ValidUntil,
		&discount.TotalUsageLimit,
		&discount.CurrentUsageCount,
		&discount.IsActive,
		&discount.CreatedAt,
		&discount.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query discount: %w", err)
	}

	return &discount, nil
}

// GetByCode retrieves a discount by code (case-insensitive).
func (r *DiscountRepositoryImpl) GetByCode(ctx context.Context, tx db.Tx, code string) (*entity.Discount, error) {
	query := `
		SELECT id, code, type, value, min_purchase, applies_to, seller_id,
		       valid_until, total_usage_limit, current_usage_count,
		       is_active, created_at, updated_at
		FROM discounts
		WHERE LOWER(code) = LOWER($1)
	`

	var discount entity.Discount
	err := tx.QueryRow(ctx, query, code).Scan(
		&discount.ID,
		&discount.Code,
		&discount.Type,
		&discount.Value,
		&discount.MinPurchase,
		&discount.AppliesTo,
		&discount.SellerID,
		&discount.ValidUntil,
		&discount.TotalUsageLimit,
		&discount.CurrentUsageCount,
		&discount.IsActive,
		&discount.CreatedAt,
		&discount.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query discount by code: %w", err)
	}

	return &discount, nil
}

// GetBySeller retrieves all discounts for a specific seller.
func (r *DiscountRepositoryImpl) GetBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID) ([]*entity.Discount, error) {
	query := `
		SELECT id, code, type, value, min_purchase, applies_to, seller_id,
		       valid_until, total_usage_limit, current_usage_count,
		       is_active, created_at, updated_at
		FROM discounts
		WHERE seller_id = $1
		ORDER BY created_at DESC
	`

	rows, err := tx.Query(ctx, query, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query seller discounts: %w", err)
	}
	defer rows.Close()

	var discounts []*entity.Discount
	for rows.Next() {
		var discount entity.Discount
		if err := rows.Scan(
			&discount.ID,
			&discount.Code,
			&discount.Type,
			&discount.Value,
			&discount.MinPurchase,
			&discount.AppliesTo,
			&discount.SellerID,
			&discount.ValidUntil,
			&discount.TotalUsageLimit,
			&discount.CurrentUsageCount,
			&discount.IsActive,
			&discount.CreatedAt,
			&discount.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan discount: %w", err)
		}
		discounts = append(discounts, &discount)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating discounts: %w", rows.Err())
	}

	return discounts, nil
}

// ListActive retrieves all currently active discounts.
func (r *DiscountRepositoryImpl) ListActive(ctx context.Context, tx db.Tx) ([]*entity.Discount, error) {
	query := `
		SELECT id, code, type, value, min_purchase, applies_to, seller_id,
		       valid_until, total_usage_limit, current_usage_count,
		       is_active, created_at, updated_at
		FROM discounts
		WHERE is_active = true
		  AND NOW() <= valid_until
		ORDER BY created_at DESC
	`

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active discounts: %w", err)
	}
	defer rows.Close()

	var discounts []*entity.Discount
	for rows.Next() {
		var discount entity.Discount
		if err := rows.Scan(
			&discount.ID,
			&discount.Code,
			&discount.Type,
			&discount.Value,
			&discount.MinPurchase,
			&discount.AppliesTo,
			&discount.SellerID,
			&discount.ValidUntil,
			&discount.TotalUsageLimit,
			&discount.CurrentUsageCount,
			&discount.IsActive,
			&discount.CreatedAt,
			&discount.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan discount: %w", err)
		}
		discounts = append(discounts, &discount)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating discounts: %w", rows.Err())
	}

	return discounts, nil
}

// Deactivate marks a discount as inactive.
func (r *DiscountRepositoryImpl) Deactivate(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	query := `
		UPDATE discounts
		SET is_active = false, updated_at = NOW()
		WHERE id = $1
	`
	result, err := tx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to deactivate discount: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("discount not found: id=%s", id)
	}
	return nil
}

// RecordUsage records a usage of a discount.
func (r *DiscountRepositoryImpl) RecordUsage(ctx context.Context, tx db.Tx, usage *entity.DiscountUsage) error {
	query := `
		INSERT INTO discount_usages (id, discount_id, user_id, order_id, used_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := tx.Exec(ctx, query,
		usage.ID,
		usage.DiscountID,
		usage.UserID,
		usage.OrderID,
		usage.UsedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to record discount usage: %w", err)
	}
	return nil
}

// IncrementUsageCount increments the current_usage_count for a discount.
func (r *DiscountRepositoryImpl) IncrementUsageCount(ctx context.Context, tx db.Tx, discountID uuid.UUID) error {
	query := `SELECT total_usage_limit, current_usage_count FROM discounts WHERE id = $1`
	var totalLimit int
	var currentCount int
	if err := tx.QueryRow(ctx, query, discountID).Scan(&totalLimit, &currentCount); err != nil {
		return fmt.Errorf("failed to get discount: %w", err)
	}

	if totalLimit > 0 && currentCount >= totalLimit {
		return &entity.DiscountUsageLimitExceededError{
			Code:         discountID.String(),
			UsageLimit:   totalLimit,
			CurrentUsage: currentCount,
		}
	}

	var updateQuery string
	if totalLimit > 0 {
		updateQuery = `
			UPDATE discounts
			SET current_usage_count = current_usage_count + 1, updated_at = NOW()
			WHERE id = $1 AND current_usage_count < $2
		`
		result, err := tx.Exec(ctx, updateQuery, discountID, totalLimit)
		if err != nil {
			return fmt.Errorf("failed to increment usage count: %w", err)
		}
		if result.RowsAffected() == 0 {
			return &entity.DiscountUsageLimitExceededError{
				Code:         discountID.String(),
				UsageLimit:   totalLimit,
				CurrentUsage: currentCount,
			}
		}
	} else {
		updateQuery = `
			UPDATE discounts
			SET current_usage_count = current_usage_count + 1, updated_at = NOW()
			WHERE id = $1
		`
		result, err := tx.Exec(ctx, updateQuery, discountID)
		if err != nil {
			return fmt.Errorf("failed to increment usage count: %w", err)
		}
		if result.RowsAffected() == 0 {
			return fmt.Errorf("discount not found: id=%s", discountID)
		}
	}

	return nil
}

// GetUsageByUserAndOrder retrieves usage record for a specific user and order.
func (r *DiscountRepositoryImpl) GetUsageByUserAndOrder(ctx context.Context, tx db.Tx, userID, orderID uuid.UUID) (*entity.DiscountUsage, error) {
	query := `
		SELECT id, discount_id, user_id, order_id, used_at
		FROM discount_usages
		WHERE user_id = $1 AND order_id = $2
	`
	var usage entity.DiscountUsage
	err := tx.QueryRow(ctx, query, userID, orderID).Scan(
		&usage.ID,
		&usage.DiscountID,
		&usage.UserID,
		&usage.OrderID,
		&usage.UsedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query discount usage: %w", err)
	}
	return &usage, nil
}
