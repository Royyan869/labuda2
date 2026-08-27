package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/discount/entity"
	"github.com/labuda/backend/pkg/db"
)

// DiscountRepository defines the interface for discount persistence operations.
type DiscountRepository interface {
	// Create creates a new discount.
	Create(ctx context.Context, tx db.Tx, discount *entity.Discount) error

	// Update updates an existing discount.
	Update(ctx context.Context, tx db.Tx, discount *entity.Discount) error

	// GetByID retrieves a discount by ID.
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Discount, error)

	// GetByCode retrieves a discount by code (case-insensitive).
	GetByCode(ctx context.Context, tx db.Tx, code string) (*entity.Discount, error)

	// GetBySeller retrieves all discounts for a specific seller.
	GetBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID) ([]*entity.Discount, error)

	// ListActive retrieves all currently active discounts.
	ListActive(ctx context.Context, tx db.Tx) ([]*entity.Discount, error)

	// Deactivate marks a discount as inactive.
	Deactivate(ctx context.Context, tx db.Tx, id uuid.UUID) error

	// RecordUsage records a usage of a discount and increments usage count.
	// This should be called within the same transaction as order creation.
	RecordUsage(ctx context.Context, tx db.Tx, usage *entity.DiscountUsage) error

	// CountUsageByUser counts how many times a user has used a specific discount.
	CountUsageByUser(ctx context.Context, tx db.Tx, discountID, userID uuid.UUID) (int, error)

	// IncrementUsageCount increments the current_usage_count for a discount.
	IncrementUsageCount(ctx context.Context, tx db.Tx, discountID uuid.UUID) error

	// GetUsageByUserAndOrder retrieves usage record for a specific user and order.
	// Useful for idempotency checks.
	GetUsageByUserAndOrder(ctx context.Context, tx db.Tx, userID, orderID uuid.UUID) (*entity.DiscountUsage, error)
}

// DiscountQueryRepository defines read-only query operations for discounts.
type DiscountQueryRepository interface {
	// ValidateForCheckout validates if a discount can be used at checkout.
	// Returns the discount if valid, or an error if validation fails.
	ValidateForCheckout(ctx context.Context, tx db.Tx, code string, userID uuid.UUID, subtotal int64) (*entity.Discount, error)
}


