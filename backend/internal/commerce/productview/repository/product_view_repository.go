// Package repository defines the canonical Product View persistence contract.
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/productview/entity"
	"github.com/labuda/backend/pkg/db"
)

// ProductViewRepository is the single persistence surface for Product View.
//
// Record writes one immutable view event. CountByProduct is the aggregate read
// that a future Seller Analytics consumer uses (COUNT GROUP BY product_id).
type ProductViewRepository interface {
	// Record persists exactly one Product View event. It always appends; there
	// is no dedup and no mutable counter.
	Record(ctx context.Context, tx db.Tx, event *entity.ProductViewEvent) error

	// CountByProduct returns the total number of Product View events recorded
	// for a product.
	CountByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) (int64, error)
}
