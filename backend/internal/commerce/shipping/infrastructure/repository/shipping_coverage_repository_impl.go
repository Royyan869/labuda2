package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// ShippingCoverageRepositoryImpl handles shipping coverage persistence using pgx-based DB layer.
type ShippingCoverageRepositoryImpl struct{}

// NewShippingCoverageRepository creates a new ShippingCoverageRepository.
func NewShippingCoverageRepository() ShippingCoverageRepository {
	return &ShippingCoverageRepositoryImpl{}
}

// Create persists a new shipping coverage within a transaction.
func (r *ShippingCoverageRepositoryImpl) Create(
	ctx context.Context,
	tx db.Tx,
	coverage *entity.ShippingCoverage,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO shipping_coverages (
			id, shipping_option_id, province_code, province_name,
			province_rate, estimated_days, is_available, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		coverage.ID,
		coverage.ShippingOptionID,
		coverage.ProvinceCode,
		coverage.ProvinceName,
		coverage.ProvinceRate.Int64(),
		coverage.EstimatedDays,
		coverage.IsAvailable,
		coverage.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("create shipping coverage failed: %w", err)
	}

	return nil
}

// Update persists shipping coverage changes within a transaction.
func (r *ShippingCoverageRepositoryImpl) Update(
	ctx context.Context,
	tx db.Tx,
	coverage *entity.ShippingCoverage,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE shipping_coverages
		SET province_name = $2, province_rate = $3, estimated_days = $4,
		    is_available = $5
		WHERE id = $1
	`,
		coverage.ID,
		coverage.ProvinceName,
		coverage.ProvinceRate.Int64(),
		coverage.EstimatedDays,
		coverage.IsAvailable,
	)

	if err != nil {
		return fmt.Errorf("update shipping coverage failed: %w", err)
	}

	return nil
}

// GetByID retrieves a shipping coverage without locking.
func (r *ShippingCoverageRepositoryImpl) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.ShippingCoverage, error) {
	var shippingOptionID uuid.UUID
	var provinceCode string
	var provinceName string
	var provinceRate int64
	var estimatedDays *string
	var isAvailable bool
	var createdAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, shipping_option_id, province_code, province_name,
		       province_rate, estimated_days, is_available, created_at
		FROM shipping_coverages
		WHERE id = $1
	`, id).Scan(
		&id, &shippingOptionID, &provinceCode, &provinceName,
		&provinceRate, &estimatedDays, &isAvailable, &createdAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("shipping coverage not found: %s", id)
		}
		return nil, fmt.Errorf("get shipping coverage failed: %w", err)
	}

	return &entity.ShippingCoverage{
		ID:               id,
		ShippingOptionID: shippingOptionID,
		ProvinceCode:     provinceCode,
		ProvinceName:     provinceName,
		ProvinceRate:     money.New(provinceRate),
		EstimatedDays:    estimatedDays,
		IsAvailable:      isAvailable,
		CreatedAt:        createdAt,
	}, nil
}

// GetByShippingOption retrieves all coverages for a shipping option.
func (r *ShippingCoverageRepositoryImpl) GetByShippingOption(
	ctx context.Context,
	tx db.Tx,
	shippingOptionID uuid.UUID,
) ([]*entity.ShippingCoverage, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, shipping_option_id, province_code, province_name,
		       province_rate, estimated_days, is_available, created_at
		FROM shipping_coverages
		WHERE shipping_option_id = $1
		ORDER BY province_code
	`, shippingOptionID)
	if err != nil {
		return nil, fmt.Errorf("get coverages by shipping option failed: %w", err)
	}
	defer rows.Close()

	var coverages []*entity.ShippingCoverage
	for rows.Next() {
		var id, shippingOptionID uuid.UUID
		var provinceCode string
		var provinceName string
		var provinceRate int64
		var estimatedDays *string
		var isAvailable bool
		var createdAt time.Time

		err := rows.Scan(
			&id, &shippingOptionID, &provinceCode, &provinceName,
			&provinceRate, &estimatedDays, &isAvailable, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan shipping coverage failed: %w", err)
		}

		coverages = append(coverages, &entity.ShippingCoverage{
			ID:               id,
			ShippingOptionID: shippingOptionID,
			ProvinceCode:     provinceCode,
			ProvinceName:     provinceName,
			ProvinceRate:     money.New(provinceRate),
			EstimatedDays:    estimatedDays,
			IsAvailable:      isAvailable,
			CreatedAt:        createdAt,
		})
	}

	return coverages, nil
}

// GetByOptionAndProvince retrieves a specific coverage by option and province.
func (r *ShippingCoverageRepositoryImpl) GetByOptionAndProvince(
	ctx context.Context,
	tx db.Tx,
	shippingOptionID uuid.UUID,
	provinceCode string,
) (*entity.ShippingCoverage, error) {
	var id uuid.UUID
	var provinceName string
	var provinceRate int64
	var isAvailable bool
	var createdAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, shipping_option_id, province_code, province_name,
		       province_rate, is_available, created_at
		FROM shipping_coverages
		WHERE shipping_option_id = $1 AND province_code = $2
	`, shippingOptionID, provinceCode).Scan(
		&id, &shippingOptionID, &provinceCode, &provinceName,
		&provinceRate, &isAvailable, &createdAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("shipping coverage not found for province: %s", provinceCode)
		}
		return nil, fmt.Errorf("get coverage by option and province failed: %w", err)
	}

	return &entity.ShippingCoverage{
		ID:               id,
		ShippingOptionID: shippingOptionID,
		ProvinceCode:     provinceCode,
		ProvinceName:     provinceName,
		ProvinceRate:     money.New(provinceRate),
		EstimatedDays:    nil, // estimated_days removed (migration 000014)
		IsAvailable:      isAvailable,
		CreatedAt:        createdAt,
	}, nil
}

// Delete removes a shipping coverage.
func (r *ShippingCoverageRepositoryImpl) Delete(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `DELETE FROM shipping_coverages WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete shipping coverage failed: %w", err)
	}
	return nil
}

// DeleteByShippingOption removes all coverages for a shipping option.
func (r *ShippingCoverageRepositoryImpl) DeleteByShippingOption(
	ctx context.Context,
	tx db.Tx,
	shippingOptionID uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `DELETE FROM shipping_coverages WHERE shipping_option_id = $1`, shippingOptionID)
	if err != nil {
		return fmt.Errorf("delete coverages by shipping option failed: %w", err)
	}
	return nil
}
