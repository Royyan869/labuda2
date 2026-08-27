package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/labuda/backend/internal/platform/config/entity"
	"github.com/labuda/backend/pkg/db"
)

// PlatformConfigRepositoryImpl handles platform config persistence using pgx.
type PlatformConfigRepositoryImpl struct{}

// NewPlatformConfigRepository creates a new PlatformConfigRepository.
func NewPlatformConfigRepository() Repository {
	return &PlatformConfigRepositoryImpl{}
}

// Get retrieves a config by key without locking.
func (r *PlatformConfigRepositoryImpl) Get(
	ctx context.Context,
	tx db.Tx,
	key string,
) (*entity.Config, error) {
	var valueNum *string
	var valueText *string
	var updatedBy *uuid.UUID
	var updatedAt int64

	err := tx.QueryRow(ctx, `
		SELECT key, value_numeric::TEXT, value_text, updated_by, updated_at
		FROM platform_configs
		WHERE key = $1
	`, key).Scan(
		&key, &valueNum, &valueText, &updatedBy, &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, &entity.ConfigNotFoundError{Key: key}
		}
		return nil, fmt.Errorf("get platform config failed: %w", err)
	}

	config := &entity.Config{
		Key:       key,
		ValueText: valueText,
		UpdatedBy: updatedBy,
		UpdatedAt: time.Unix(updatedAt, 0),
	}

	// Parse numeric value if present
	if valueNum != nil {
		decimalValue, err := parseDecimal(*valueNum)
		if err != nil {
			return nil, fmt.Errorf("parse decimal failed for key %s: %w", key, err)
		}
		config.ValueNum = &decimalValue
	}

	return config, nil
}

// GetAll retrieves all platform configs.
// MANAGEMENT PRE-FIX M1: Added to support listing all configs in admin view.
func (r *PlatformConfigRepositoryImpl) GetAll(
	ctx context.Context,
	tx db.Tx,
) ([]*entity.Config, error) {
	rows, err := tx.Query(ctx, `
		SELECT key, value_numeric::TEXT, value_text, updated_by, updated_at
		FROM platform_configs
		ORDER BY key ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("get all platform configs failed: %w", err)
	}
	defer rows.Close()

	var configs []*entity.Config
	for rows.Next() {
		var key string
		var valueNum *string
		var valueText *string
		var updatedBy *uuid.UUID
		var updatedAt int64

		err := rows.Scan(&key, &valueNum, &valueText, &updatedBy, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan platform config failed: %w", err)
		}

		config := &entity.Config{
			Key:       key,
			ValueText: valueText,
			UpdatedBy: updatedBy,
			UpdatedAt: time.Unix(updatedAt, 0),
		}

		// Parse numeric value if present
		if valueNum != nil {
			decimalValue, err := parseDecimal(*valueNum)
			if err != nil {
				return nil, fmt.Errorf("parse decimal failed for key %s: %w", key, err)
			}
			config.ValueNum = &decimalValue
		}

		configs = append(configs, config)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate platform configs failed: %w", rows.Err())
	}

	return configs, nil
}

// SetNumeric upserts a config with a numeric value.
func (r *PlatformConfigRepositoryImpl) SetNumeric(
	ctx context.Context,
	tx db.Tx,
	key string,
	value decimal.Decimal,
	updatedBy uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO platform_configs (key, value_numeric, value_text, updated_by, updated_at)
		VALUES ($1, $2, NULL, $3, $4)
		ON CONFLICT (key) DO UPDATE
			SET value_numeric = EXCLUDED.value_numeric,
			    value_text = EXCLUDED.value_text,
			    updated_by = EXCLUDED.updated_by,
			    updated_at = EXCLUDED.updated_at
	`,
		key, value.String(), updatedBy, time.Now().Unix(),
	)

	if err != nil {
		return fmt.Errorf("set platform config numeric failed: %w", err)
	}

	return nil
}

// SetText upserts a config with a text value.
func (r *PlatformConfigRepositoryImpl) SetText(
	ctx context.Context,
	tx db.Tx,
	key string,
	value string,
	updatedBy uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO platform_configs (key, value_numeric, value_text, updated_by, updated_at)
		VALUES ($1, NULL, $2, $3, $4)
		ON CONFLICT (key) DO UPDATE
			SET value_numeric = EXCLUDED.value_numeric,
			    value_text = EXCLUDED.value_text,
			    updated_by = EXCLUDED.updated_by,
			    updated_at = EXCLUDED.updated_at
	`,
		key, value, updatedBy, time.Now().Unix(),
	)

	if err != nil {
		return fmt.Errorf("set platform config text failed: %w", err)
	}

	return nil
}

// parseDecimal is a helper to parse decimal from string.
// Imported from shopspring/decimal for the repository implementation.
func parseDecimal(s string) (decimal.Decimal, error) {
	return decimal.NewFromString(s)
}


