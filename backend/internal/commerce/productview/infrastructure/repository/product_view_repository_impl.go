// Package repository implements the canonical Product View persistence.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hishumi/backend/internal/commerce/productview/entity"
	"github.com/hishumi/backend/pkg/db"
)

// ProductViewRepositoryImpl persists Product View events in product_view_events.
type ProductViewRepositoryImpl struct{}

// NewProductViewRepository creates a new ProductViewRepositoryImpl.
func NewProductViewRepository() *ProductViewRepositoryImpl {
	return &ProductViewRepositoryImpl{}
}

// Record appends one immutable Product View event.
func (r *ProductViewRepositoryImpl) Record(ctx context.Context, tx db.Tx, event *entity.ProductViewEvent) error {
	if event == nil {
		return fmt.Errorf("product view event is nil")
	}
	if event.ProductID == uuid.Nil {
		return fmt.Errorf("product view event requires product_id")
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.ViewedAt.IsZero() {
		event.ViewedAt = time.Now().UTC()
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO product_view_events (id, product_id, viewer_user_id, viewed_at)
		VALUES ($1, $2, $3, $4)
	`, event.ID, event.ProductID, event.ViewerUserID, event.ViewedAt)
	if err != nil {
		return fmt.Errorf("record product view failed: %w", err)
	}
	return nil
}

// CountByProduct returns the total number of view events for a product.
func (r *ProductViewRepositoryImpl) CountByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) (int64, error) {
	var count int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM product_view_events
		WHERE product_id = $1
	`, productID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count product views failed: %w", err)
	}
	return count, nil
}
