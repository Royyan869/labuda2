package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/pkg/db"
)

// ProductShippingOptionRepositoryImpl handles product-shipping link persistence using pgx-based DB layer.
type ProductShippingOptionRepositoryImpl struct {
	optionRepo ShippingOptionRepository
}

// NewProductShippingOptionRepository creates a new ProductShippingOptionRepository.
func NewProductShippingOptionRepository(
	optionRepo ShippingOptionRepository,
) ProductShippingOptionRepository {
	return &ProductShippingOptionRepositoryImpl{
		optionRepo: optionRepo,
	}
}

// Create persists a new product-shipping link within a transaction.
func (r *ProductShippingOptionRepositoryImpl) Create(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
	shippingOptionID uuid.UUID,
	sortOrder int,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO product_shipping_options (product_id, shipping_option_id, sort_order, created_at)
		VALUES ($1, $2, $3, NOW())
	`, productID, shippingOptionID, sortOrder)

	if err != nil {
		return fmt.Errorf("create product shipping option failed: %w", err)
	}

	return nil
}

// Delete removes a product-shipping link within a transaction.
func (r *ProductShippingOptionRepositoryImpl) Delete(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
	shippingOptionID uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM product_shipping_options
		WHERE product_id = $1 AND shipping_option_id = $2
	`, productID, shippingOptionID)

	if err != nil {
		return fmt.Errorf("delete product shipping option failed: %w", err)
	}

	return nil
}

// GetByProduct retrieves all shipping options linked to a product.
//
// NOTE: so.expedition_name was dropped by migration 000014
// (shipping_authority_hard_purge) but remained in this SELECT/scan, making the
// query fail with column-not-found on every FPS/auction checkout shipping
// check. Fixed in Stage 5 (order-item identity convergence groundwork).
func (r *ProductShippingOptionRepositoryImpl) GetByProduct(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) ([]*entity.ShippingOption, error) {
	rows, err := tx.Query(ctx, `
		SELECT so.id, so.seller_id, so.name, so.transport_type,
		       so.is_active, so.created_at, so.updated_at
		FROM shipping_options so
		INNER JOIN product_shipping_options pso ON so.id = pso.shipping_option_id
		WHERE pso.product_id = $1
		ORDER BY pso.sort_order
	`, productID)
	if err != nil {
		return nil, fmt.Errorf("get shipping options by product failed: %w", err)
	}
	defer rows.Close()

	var options []*entity.ShippingOption
	for rows.Next() {
		var id, sellerID uuid.UUID
		var name string
		var transportType string
		var isActive bool
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&id, &sellerID, &name, &transportType,
			&isActive, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan shipping option failed: %w", err)
		}

		options = append(options, &entity.ShippingOption{
			ID:             id,
			SellerID:       sellerID,
			Name:           name,
			TransportType:  entity.TransportType(transportType),
			ExpeditionName: nil, // expedition_name removed (migration 000014)
			IsActive:       isActive,
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		})
	}

	return options, nil
}

// GetAvailableByProduct retrieves all available shipping options for a product, sorted by sort_order.
// (expedition_name dropped by migration 000014 — same fix as GetByProduct.)
func (r *ProductShippingOptionRepositoryImpl) GetAvailableByProduct(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) ([]*entity.ShippingOption, error) {
	rows, err := tx.Query(ctx, `
		SELECT so.id, so.seller_id, so.name, so.transport_type,
		       so.is_active, so.created_at, so.updated_at
		FROM shipping_options so
		INNER JOIN product_shipping_options pso ON so.id = pso.shipping_option_id
		WHERE pso.product_id = $1 AND so.is_active = true
		ORDER BY pso.sort_order
	`, productID)
	if err != nil {
		return nil, fmt.Errorf("get available shipping options by product failed: %w", err)
	}
	defer rows.Close()

	var options []*entity.ShippingOption
	for rows.Next() {
		var id, sellerID uuid.UUID
		var name string
		var transportType string
		var isActive bool
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&id, &sellerID, &name, &transportType,
			&isActive, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan shipping option failed: %w", err)
		}

		options = append(options, &entity.ShippingOption{
			ID:             id,
			SellerID:       sellerID,
			Name:           name,
			TransportType:  entity.TransportType(transportType),
			ExpeditionName: nil, // expedition_name removed (migration 000014)
			IsActive:       isActive,
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		})
	}

	return options, nil
}

// DeleteByProduct removes all shipping links for a product.
func (r *ProductShippingOptionRepositoryImpl) DeleteByProduct(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `DELETE FROM product_shipping_options WHERE product_id = $1`, productID)
	if err != nil {
		return fmt.Errorf("delete product shipping options by product failed: %w", err)
	}
	return nil
}

// DeleteByShippingOption removes all product links for a shipping option.
func (r *ProductShippingOptionRepositoryImpl) DeleteByShippingOption(
	ctx context.Context,
	tx db.Tx,
	shippingOptionID uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `DELETE FROM product_shipping_options WHERE shipping_option_id = $1`, shippingOptionID)
	if err != nil {
		return fmt.Errorf("delete product shipping options by shipping option failed: %w", err)
	}
	return nil
}

// CreateBulk creates multiple product-shipping links within a transaction.
func (r *ProductShippingOptionRepositoryImpl) CreateBulk(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
	shippingOptionIDs []uuid.UUID,
) error {
	if len(shippingOptionIDs) == 0 {
		return nil
	}

	// Build bulk insert query
	for i, optionID := range shippingOptionIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO product_shipping_options (product_id, shipping_option_id, sort_order, created_at)
			VALUES ($1, $2, $3, NOW())
		`, productID, optionID, i)
		if err != nil {
			return fmt.Errorf("create product shipping options failed: %w", err)
		}
	}

	return nil
}

// CountByProduct counts the number of shipping options linked to a product.
func (r *ProductShippingOptionRepositoryImpl) CountByProduct(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) (int64, error) {
	var count int64
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM product_shipping_options
		WHERE product_id = $1
	`, productID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count shipping options for product failed: %w", err)
	}
	return count, nil
}
