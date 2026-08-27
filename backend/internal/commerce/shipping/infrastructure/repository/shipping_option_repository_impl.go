package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/pkg/db"
)

// ShippingOptionRepositoryImpl handles shipping option persistence using pgx-based DB layer.
type ShippingOptionRepositoryImpl struct{}

// NewShippingOptionRepository creates a new ShippingOptionRepository.
func NewShippingOptionRepository() ShippingOptionRepository {
	return &ShippingOptionRepositoryImpl{}
}

// Create persists a new shipping option within a transaction.
func (r *ShippingOptionRepositoryImpl) Create(
	ctx context.Context,
	tx db.Tx,
	option *entity.ShippingOption,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO shipping_options (
			id, seller_id, name, transport_type, expedition_name,
			is_active, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		option.ID,
		option.SellerID,
		option.Name,
		string(option.TransportType),
		option.ExpeditionName,
		option.IsActive,
		option.CreatedAt,
		option.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create shipping option failed: %w", err)
	}

	return nil
}

// Update persists shipping option changes within a transaction.
func (r *ShippingOptionRepositoryImpl) Update(
	ctx context.Context,
	tx db.Tx,
	option *entity.ShippingOption,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE shipping_options
		SET name = $2, transport_type = $3, expedition_name = $4,
		    is_active = $5, updated_at = $6
		WHERE id = $1
	`,
		option.ID,
		option.Name,
		string(option.TransportType),
		option.ExpeditionName,
		option.IsActive,
		option.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("update shipping option failed: %w", err)
	}

	return nil
}

// GetByID retrieves a shipping option without locking (for read-only operations).
func (r *ShippingOptionRepositoryImpl) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.ShippingOption, error) {
	var sellerID uuid.UUID
	var name string
	var transportType string
	var expeditionName *string
	var isActive bool
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, seller_id, name, transport_type, expedition_name,
		       is_active, created_at, updated_at
		FROM shipping_options
		WHERE id = $1
	`, id).Scan(
		&id, &sellerID, &name, &transportType, &expeditionName,
		&isActive, &createdAt, &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("shipping option not found: %s", id)
		}
		return nil, fmt.Errorf("get shipping option failed: %w", err)
	}

	return &entity.ShippingOption{
		ID:             id,
		SellerID:       sellerID,
		Name:           name,
		TransportType:  entity.TransportType(transportType),
		ExpeditionName: expeditionName,
		IsActive:       isActive,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

// GetForUpdate retrieves a shipping option with FOR UPDATE lock.
// This prevents concurrent modifications and must be used within a transaction.
func (r *ShippingOptionRepositoryImpl) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.ShippingOption, error) {
	var sellerID uuid.UUID
	var name string
	var transportType string
	var expeditionName *string
	var isActive bool
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, seller_id, name, transport_type, expedition_name,
		       is_active, created_at, updated_at
		FROM shipping_options
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&id, &sellerID, &name, &transportType, &expeditionName,
		&isActive, &createdAt, &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("shipping option not found: %s", id)
		}
		return nil, fmt.Errorf("get shipping option for update failed: %w", err)
	}

	return &entity.ShippingOption{
		ID:             id,
		SellerID:       sellerID,
		Name:           name,
		TransportType:  entity.TransportType(transportType),
		ExpeditionName: expeditionName,
		IsActive:       isActive,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

// GetBySeller retrieves all shipping options for a seller.
// Returns active options only if onlyActive is true.
func (r *ShippingOptionRepositoryImpl) GetBySeller(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	onlyActive bool,
) ([]*entity.ShippingOption, error) {
	query := `
		SELECT id, seller_id, name, transport_type, expedition_name,
		       is_active, created_at, updated_at
		FROM shipping_options
		WHERE seller_id = $1
	`
	args := []interface{}{sellerID}

	if onlyActive {
		query += " AND is_active = true"
	}

	query += " ORDER BY created_at DESC"

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get shipping options by seller failed: %w", err)
	}
	defer rows.Close()

	var options []*entity.ShippingOption
	for rows.Next() {
		var id, sellerID uuid.UUID
		var name string
		var transportType string
		var expeditionName *string
		var isActive bool
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&id, &sellerID, &name, &transportType, &expeditionName,
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
			ExpeditionName: expeditionName,
			IsActive:       isActive,
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		})
	}

	return options, nil
}

// GetByName retrieves a shipping option by seller and name.
func (r *ShippingOptionRepositoryImpl) GetByName(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	name string,
) (*entity.ShippingOption, error) {
	var id uuid.UUID
	var transportType string
	var expeditionName *string
	var isActive bool
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, seller_id, name, transport_type, expedition_name,
		       is_active, created_at, updated_at
		FROM shipping_options
		WHERE seller_id = $1 AND name = $2
	`, sellerID, name).Scan(
		&id, &sellerID, &name, &transportType, &expeditionName,
		&isActive, &createdAt, &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("shipping option not found: %s", name)
		}
		return nil, fmt.Errorf("get shipping option by name failed: %w", err)
	}

	return &entity.ShippingOption{
		ID:             id,
		SellerID:       sellerID,
		Name:           name,
		TransportType:  entity.TransportType(transportType),
		ExpeditionName: expeditionName,
		IsActive:       isActive,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

// Delete removes a shipping option.
func (r *ShippingOptionRepositoryImpl) Delete(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `DELETE FROM shipping_options WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete shipping option failed: %w", err)
	}
	return nil
}


