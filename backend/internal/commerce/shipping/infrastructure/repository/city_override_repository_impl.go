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

// CityOverrideRepositoryImpl handles city override persistence using pgx-based DB layer.
type CityOverrideRepositoryImpl struct{}

// NewCityOverrideRepository creates a new CityOverrideRepository.
func NewCityOverrideRepository() CityOverrideRepository {
	return &CityOverrideRepositoryImpl{}
}

// Create persists a new city override within a transaction.
func (r *CityOverrideRepositoryImpl) Create(
	ctx context.Context,
	tx db.Tx,
	override *entity.CityOverride,
) error {
	var ratePtr *int64
	if override.Rate != nil {
		rateVal := override.Rate.Int64()
		ratePtr = &rateVal
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO shipping_city_overrides (
			id, shipping_coverage_id, city_code, city_name,
			rate, estimated_days, is_available, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		override.ID,
		override.ShippingCoverageID,
		override.CityCode,
		override.CityName,
		ratePtr,
		override.EstimatedDays,
		override.IsAvailable,
		override.CreatedAt,
		override.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create city override failed: %w", err)
	}

	return nil
}

// Update persists city override changes within a transaction.
func (r *CityOverrideRepositoryImpl) Update(
	ctx context.Context,
	tx db.Tx,
	override *entity.CityOverride,
) error {
	var ratePtr *int64
	if override.Rate != nil {
		rateVal := override.Rate.Int64()
		ratePtr = &rateVal
	}

	_, err := tx.Exec(ctx, `
		UPDATE shipping_city_overrides
		SET city_name = $2, rate = $3, estimated_days = $4,
		    is_available = $5, updated_at = $6
		WHERE id = $1
	`,
		override.ID,
		override.CityName,
		ratePtr,
		override.EstimatedDays,
		override.IsAvailable,
		override.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("update city override failed: %w", err)
	}

	return nil
}

// GetByID retrieves a city override without locking.
func (r *CityOverrideRepositoryImpl) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.CityOverride, error) {
	var shippingCoverageID uuid.UUID
	var cityCode string
	var cityName string
	var ratePtr *int64
	var estimatedDays *string
	var isAvailable *bool
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, shipping_coverage_id, city_code, city_name,
		       rate, estimated_days, is_available, created_at, updated_at
		FROM shipping_city_overrides
		WHERE id = $1
	`, id).Scan(
		&id, &shippingCoverageID, &cityCode, &cityName,
		&ratePtr, &estimatedDays, &isAvailable, &createdAt, &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("city override not found: %s", id)
		}
		return nil, fmt.Errorf("get city override failed: %w", err)
	}

	var rate *money.Money
	if ratePtr != nil {
		rate = &money.Money{}
		*rate = money.New(*ratePtr)
	}

	return &entity.CityOverride{
		ID:                 id,
		ShippingCoverageID: shippingCoverageID,
		CityCode:           cityCode,
		CityName:           cityName,
		Rate:               rate,
		EstimatedDays:      estimatedDays,
		IsAvailable:        isAvailable,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}, nil
}

// GetByCoverage retrieves all overrides for a shipping coverage.
func (r *CityOverrideRepositoryImpl) GetByCoverage(
	ctx context.Context,
	tx db.Tx,
	shippingCoverageID uuid.UUID,
) ([]*entity.CityOverride, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, shipping_coverage_id, city_code, city_name,
		       rate, estimated_days, is_available, created_at, updated_at
		FROM shipping_city_overrides
		WHERE shipping_coverage_id = $1
		ORDER BY city_code
	`, shippingCoverageID)
	if err != nil {
		return nil, fmt.Errorf("get overrides by coverage failed: %w", err)
	}
	defer rows.Close()

	var overrides []*entity.CityOverride
	for rows.Next() {
		var id, shippingCoverageID uuid.UUID
		var cityCode string
		var cityName string
		var ratePtr *int64
		var estimatedDays *string
		var isAvailable *bool
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&id, &shippingCoverageID, &cityCode, &cityName,
			&ratePtr, &estimatedDays, &isAvailable, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan city override failed: %w", err)
		}

		var rate *money.Money
		if ratePtr != nil {
			rate = &money.Money{}
			*rate = money.New(*ratePtr)
		}

		overrides = append(overrides, &entity.CityOverride{
			ID:                 id,
			ShippingCoverageID: shippingCoverageID,
			CityCode:           cityCode,
			CityName:           cityName,
			Rate:               rate,
			EstimatedDays:      estimatedDays,
			IsAvailable:        isAvailable,
			CreatedAt:          createdAt,
			UpdatedAt:          updatedAt,
		})
	}

	return overrides, nil
}

// GetByCoverageAndCity retrieves a specific override by coverage and city.
func (r *CityOverrideRepositoryImpl) GetByCoverageAndCity(
	ctx context.Context,
	tx db.Tx,
	shippingCoverageID uuid.UUID,
	cityCode string,
) (*entity.CityOverride, error) {
	var id uuid.UUID
	var cityName string
	var ratePtr *int64
	var estimatedDays *string
	var isAvailable *bool
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, shipping_coverage_id, city_code, city_name,
		       rate, estimated_days, is_available, created_at, updated_at
		FROM shipping_city_overrides
		WHERE shipping_coverage_id = $1 AND city_code = $2
	`, shippingCoverageID, cityCode).Scan(
		&id, &shippingCoverageID, &cityCode, &cityName,
		&ratePtr, &estimatedDays, &isAvailable, &createdAt, &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("city override not found for city: %s", cityCode)
		}
		return nil, fmt.Errorf("get override by coverage and city failed: %w", err)
	}

	var rate *money.Money
	if ratePtr != nil {
		rate = &money.Money{}
		*rate = money.New(*ratePtr)
	}

	return &entity.CityOverride{
		ID:                 id,
		ShippingCoverageID: shippingCoverageID,
		CityCode:           cityCode,
		CityName:           cityName,
		Rate:               rate,
		EstimatedDays:      estimatedDays,
		IsAvailable:        isAvailable,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}, nil
}

// Delete removes a city override.
func (r *CityOverrideRepositoryImpl) Delete(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `DELETE FROM shipping_city_overrides WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete city override failed: %w", err)
	}
	return nil
}

// DeleteByCoverage removes all overrides for a shipping coverage.
func (r *CityOverrideRepositoryImpl) DeleteByCoverage(
	ctx context.Context,
	tx db.Tx,
	shippingCoverageID uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `DELETE FROM shipping_city_overrides WHERE shipping_coverage_id = $1`, shippingCoverageID)
	if err != nil {
		return fmt.Errorf("delete overrides by coverage failed: %w", err)
	}
	return nil
}


