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

// NewDiscountRepository creates a new DiscountRepositoryImpl.
func NewDiscountRepository() *DiscountRepositoryImpl {
	return &DiscountRepositoryImpl{}
}

// Create creates a new discount and its associated target rows.
func (r *DiscountRepositoryImpl) Create(ctx context.Context, tx db.Tx, discount *entity.Discount) error {
	query := `
		INSERT INTO discounts (
			id, code, type, value, min_purchase, max_discount,
			scope, target_mode, seller_id, valid_from, valid_until,
			max_usage_per_user, total_usage_limit, current_usage_count,
			is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14,
			$15, $16, $17
		)
	`

	_, err := tx.Exec(ctx, query,
		discount.ID,
		discount.Code,
		discount.Type,
		discount.Value,
		discount.MinPurchase,
		discount.MaxDiscount,
		discount.AppliesTo,
		discount.TargetMode,
		discount.SellerID,
		discount.ValidFrom,
		discount.ValidUntil,
		discount.MaxUsagePerUser,
		discount.TotalUsageLimit,
		discount.CurrentUsageCount,
		discount.IsActive,
		discount.CreatedAt,
		discount.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert discount: %w", err)
	}

	if discount.TargetMode == entity.DiscountTargetModeSelectedItems && len(discount.ForSaleIDs)+len(discount.AuctionIDs) > 0 {
		if err := r.insertDiscountTargets(ctx, tx, discount.ID, discount.ForSaleIDs, discount.AuctionIDs); err != nil {
			return fmt.Errorf("failed to insert discount targets: %w", err)
		}
	}

	return nil
}

// Update updates an existing discount and its target rows.
func (r *DiscountRepositoryImpl) Update(ctx context.Context, tx db.Tx, discount *entity.Discount) error {
	query := `
		UPDATE discounts SET
			code = $2,
			type = $3,
			value = $4,
			min_purchase = $5,
			max_discount = $6,
			scope = $7,
			target_mode = $8,
			seller_id = $9,
			valid_from = $10,
			valid_until = $11,
			max_usage_per_user = $12,
			total_usage_limit = $13,
			current_usage_count = $14,
			is_active = $15,
			updated_at = $16
		WHERE id = $1
	`

	result, err := tx.Exec(ctx, query,
		discount.ID,
		discount.Code,
		discount.Type,
		discount.Value,
		discount.MinPurchase,
		discount.MaxDiscount,
		discount.AppliesTo,
		discount.TargetMode,
		discount.SellerID,
		discount.ValidFrom,
		discount.ValidUntil,
		discount.MaxUsagePerUser,
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

	if _, err = tx.Exec(ctx, `DELETE FROM discount_targets WHERE discount_id = $1`, discount.ID); err != nil {
		return fmt.Errorf("failed to clear discount targets: %w", err)
	}

	if discount.TargetMode == entity.DiscountTargetModeSelectedItems && len(discount.ForSaleIDs)+len(discount.AuctionIDs) > 0 {
		if err := r.insertDiscountTargets(ctx, tx, discount.ID, discount.ForSaleIDs, discount.AuctionIDs); err != nil {
			return fmt.Errorf("failed to insert discount targets: %w", err)
		}
	}

	return nil
}

// GetByID retrieves a discount by ID with its target data loaded.
func (r *DiscountRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Discount, error) {
	query := `
		SELECT id, code, type, value, min_purchase, max_discount,
		       scope, target_mode, seller_id, valid_from, valid_until,
		       max_usage_per_user, total_usage_limit, current_usage_count,
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
		&discount.MaxDiscount,
		&discount.AppliesTo,
		&discount.TargetMode,
		&discount.SellerID,
		&discount.ValidFrom,
		&discount.ValidUntil,
		&discount.MaxUsagePerUser,
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

	if err := r.loadDiscountTargetData(ctx, tx, &discount); err != nil {
		return nil, fmt.Errorf("failed to load discount target data: %w", err)
	}

	return &discount, nil
}

// GetByCode retrieves a discount by code (case-insensitive) with target data loaded.
func (r *DiscountRepositoryImpl) GetByCode(ctx context.Context, tx db.Tx, code string) (*entity.Discount, error) {
	query := `
		SELECT id, code, type, value, min_purchase, max_discount,
		       scope, target_mode, seller_id, valid_from, valid_until,
		       max_usage_per_user, total_usage_limit, current_usage_count,
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
		&discount.MaxDiscount,
		&discount.AppliesTo,
		&discount.TargetMode,
		&discount.SellerID,
		&discount.ValidFrom,
		&discount.ValidUntil,
		&discount.MaxUsagePerUser,
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

	if err := r.loadDiscountTargetData(ctx, tx, &discount); err != nil {
		return nil, fmt.Errorf("failed to load discount target data: %w", err)
	}

	return &discount, nil
}

// GetBySeller retrieves all discounts for a specific seller with target data loaded.
func (r *DiscountRepositoryImpl) GetBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID) ([]*entity.Discount, error) {
	query := `
		SELECT id, code, type, value, min_purchase, max_discount,
		       scope, target_mode, seller_id, valid_from, valid_until,
		       max_usage_per_user, total_usage_limit, current_usage_count,
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
			&discount.MaxDiscount,
			&discount.AppliesTo,
			&discount.TargetMode,
			&discount.SellerID,
			&discount.ValidFrom,
			&discount.ValidUntil,
			&discount.MaxUsagePerUser,
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

	if err := r.loadDiscountsTargetDataBatch(ctx, tx, discounts); err != nil {
		return nil, fmt.Errorf("failed to load discount target data: %w", err)
	}

	return discounts, nil
}

// ListActive retrieves all currently active discounts with target data loaded.
func (r *DiscountRepositoryImpl) ListActive(ctx context.Context, tx db.Tx) ([]*entity.Discount, error) {
	query := `
		SELECT id, code, type, value, min_purchase, max_discount,
		       scope, target_mode, seller_id, valid_from, valid_until,
		       max_usage_per_user, total_usage_limit, current_usage_count,
		       is_active, created_at, updated_at
		FROM discounts
		WHERE is_active = true
		  AND NOW() >= valid_from
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
			&discount.MaxDiscount,
			&discount.AppliesTo,
			&discount.TargetMode,
			&discount.SellerID,
			&discount.ValidFrom,
			&discount.ValidUntil,
			&discount.MaxUsagePerUser,
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

	if err := r.loadDiscountsTargetDataBatch(ctx, tx, discounts); err != nil {
		return nil, fmt.Errorf("failed to load discount target data: %w", err)
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

// CountUsageByUser counts how many times a user has used a specific discount.
func (r *DiscountRepositoryImpl) CountUsageByUser(ctx context.Context, tx db.Tx, discountID, userID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM discount_usages
		WHERE discount_id = $1 AND user_id = $2
	`

	var count int
	err := tx.QueryRow(ctx, query, discountID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count user discount usage: %w", err)
	}

	return count, nil
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
			IsUserLimit:  false,
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
				IsUserLimit:  false,
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

// ============================================================================
// TARGET TABLE HELPERS
// ============================================================================

func (r *DiscountRepositoryImpl) loadDiscountTargetData(ctx context.Context, tx db.Tx, discount *entity.Discount) error {
	if discount.TargetMode != entity.DiscountTargetModeSelectedItems {
		return nil
	}

	query := `
		SELECT target_type, target_id
		FROM discount_targets
		WHERE discount_id = $1
		ORDER BY target_type, target_id
	`

	rows, err := tx.Query(ctx, query, discount.ID)
	if err != nil {
		return fmt.Errorf("failed to query discount targets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var targetType string
		var targetID uuid.UUID
		if err := rows.Scan(&targetType, &targetID); err != nil {
			return fmt.Errorf("failed to scan discount target: %w", err)
		}

		switch targetType {
		case "for_sale":
			discount.ForSaleIDs = append(discount.ForSaleIDs, targetID)
		case "auction":
			discount.AuctionIDs = append(discount.AuctionIDs, targetID)
		}
	}

	if rows.Err() != nil {
		return fmt.Errorf("error iterating discount targets: %w", rows.Err())
	}

	return nil
}

func (r *DiscountRepositoryImpl) loadDiscountsTargetDataBatch(ctx context.Context, tx db.Tx, discounts []*entity.Discount) error {
	if len(discounts) == 0 {
		return nil
	}

	discountMap := make(map[uuid.UUID]*entity.Discount)
	var selectedIDs []uuid.UUID
	for _, d := range discounts {
		discountMap[d.ID] = d
		if d.TargetMode == entity.DiscountTargetModeSelectedItems {
			selectedIDs = append(selectedIDs, d.ID)
		}
	}

	if len(selectedIDs) == 0 {
		return nil
	}

	query := `
		SELECT discount_id, target_type, target_id
		FROM discount_targets
		WHERE discount_id = ANY($1)
		ORDER BY discount_id, target_type, target_id
	`

	rows, err := tx.Query(ctx, query, selectedIDs)
	if err != nil {
		return fmt.Errorf("failed to batch query discount targets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var discountID uuid.UUID
		var targetType string
		var targetID uuid.UUID
		if err := rows.Scan(&discountID, &targetType, &targetID); err != nil {
			return fmt.Errorf("failed to scan discount target: %w", err)
		}

		discount, ok := discountMap[discountID]
		if !ok {
			continue
		}

		switch targetType {
		case "for_sale":
			discount.ForSaleIDs = append(discount.ForSaleIDs, targetID)
		case "auction":
			discount.AuctionIDs = append(discount.AuctionIDs, targetID)
		}
	}

	if rows.Err() != nil {
		return fmt.Errorf("error iterating discount targets: %w", rows.Err())
	}

	return nil
}

func (r *DiscountRepositoryImpl) insertDiscountTargets(ctx context.Context, tx db.Tx, discountID uuid.UUID, forSaleIDs []uuid.UUID, auctionIDs []uuid.UUID) error {
	query := `
		INSERT INTO discount_targets (discount_id, target_type, target_id, created_at)
		VALUES ($1, $2, $3, NOW())
	`

	for _, forSaleID := range forSaleIDs {
		if _, err := tx.Exec(ctx, query, discountID, "for_sale", forSaleID); err != nil {
			return fmt.Errorf("failed to insert discount forSale target: %w", err)
		}
	}
	for _, auctionID := range auctionIDs {
		if _, err := tx.Exec(ctx, query, discountID, "auction", auctionID); err != nil {
			return fmt.Errorf("failed to insert discount auction target: %w", err)
		}
	}

	return nil
}


