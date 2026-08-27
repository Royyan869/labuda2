package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/config/entity"
	"github.com/labuda/backend/internal/platform/config/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/shopspring/decimal"
)

// ConfigService provides strongly-typed getters for platform configuration.
// Values are read at domain entity creation time and snapshotted.
// This ensures historical transactions are not affected by config changes.
type ConfigService struct {
	repo repository.Repository
}

// NewConfigService creates a new ConfigService.
func NewConfigService(repo repository.Repository) *ConfigService {
	return &ConfigService{
		repo: repo,
	}
}

// Config keys - centralized to avoid typos
const (
	KeyForSaleCommissionPercent = "for_sale_commission_percent"
	KeyAuctionCommissionPercent = "auction_commission_percent"
)

// ============================================================================
// ORDER COMMISSION GETTERS
// ============================================================================

// GetForSaleCommission returns the commission percentage for for-sale orders.
// Panics if config is missing or has wrong type - fail fast for misconfiguration.
func (s *ConfigService) GetForSaleCommission(ctx context.Context, tx db.Tx) decimal.Decimal {
	return s.getNumericPercent(ctx, tx, KeyForSaleCommissionPercent)
}

// GetAuctionCommission returns the commission percentage for auction orders.
// Panics if config is missing or has wrong type - fail fast for misconfiguration.
func (s *ConfigService) GetAuctionCommission(ctx context.Context, tx db.Tx) decimal.Decimal {
	return s.getNumericPercent(ctx, tx, KeyAuctionCommissionPercent)
}

// GetOrderForSaleCommission is an alias for GetForSaleCommission for order creation context.
// Panics if config is missing or has wrong type - fail fast for misconfiguration.
func (s *ConfigService) GetOrderForSaleCommission(ctx context.Context, tx db.Tx) decimal.Decimal {
	return s.GetForSaleCommission(ctx, tx)
}

// GetOrderAuctionCommission is an alias for GetAuctionCommission for order creation context.
// Panics if config is missing or has wrong type - fail fast for misconfiguration.
func (s *ConfigService) GetOrderAuctionCommission(ctx context.Context, tx db.Tx) decimal.Decimal {
	return s.GetAuctionCommission(ctx, tx)
}

// ============================================================================
// GENERIC GETTERS
// ============================================================================

// getNumeric retrieves a numeric config value, panicking on error.
// This fail-fast approach ensures configuration errors are caught early.
func (s *ConfigService) getNumeric(ctx context.Context, tx db.Tx, key string) decimal.Decimal {
	config, err := s.repo.Get(ctx, tx, key)
	if err != nil {
		if _, ok := err.(*entity.ConfigNotFoundError); ok {
			panic(fmt.Sprintf("platform config missing: %s", key))
		}
		panic(fmt.Sprintf("platform config error for %s: %v", key, err))
	}

	value, err := config.NumericValue()
	if err != nil {
		panic(fmt.Sprintf("platform config type error for %s: %v", key, err))
	}

	return value
}

// getNumericPercent retrieves a percentage config value (0-100 scale).
// Panics if the value is invalid (e.g., negative, > 100).
func (s *ConfigService) getNumericPercent(ctx context.Context, tx db.Tx, key string) decimal.Decimal {
	value := s.getNumeric(ctx, tx, key)

	// Validate percentage range
	if value.IsNegative() || value.GreaterThan(decimal.NewFromInt(100)) {
		panic(fmt.Sprintf("platform config invalid percentage for %s: %s (must be 0-100)", key, value.String()))
	}

	return value
}

// ============================================================================
// RAW CONFIG ACCESS (for admin use)
// ============================================================================

// GetAllConfigs retrieves all platform configs.
// MANAGEMENT PRE-FIX M1: Added to support listing all configs in admin view.
// Authorization should be enforced at the handler layer.
func (s *ConfigService) GetAllConfigs(ctx context.Context, tx db.Tx) ([]*entity.Config, error) {
	return s.repo.GetAll(ctx, tx)
}

// GetConfig retrieves the raw config entity by key.
// Returns ConfigNotFoundError if key does not exist.
func (s *ConfigService) GetConfig(ctx context.Context, tx db.Tx, key string) (*entity.Config, error) {
	return s.repo.Get(ctx, tx, key)
}

// SetConfigNumeric sets a numeric config value.
// For admin use only - authorization should be enforced at the handler layer.
func (s *ConfigService) SetConfigNumeric(ctx context.Context, tx db.Tx, key string, value decimal.Decimal, updatedBy string) error {
	updatedByUUID, err := parseUUID(updatedBy)
	if err != nil {
		return fmt.Errorf("invalid updated_by UUID: %w", err)
	}
	return s.repo.SetNumeric(ctx, tx, key, value, updatedByUUID)
}

// SetConfigText sets a text config value.
// For admin use only - authorization should be enforced at the handler layer.
func (s *ConfigService) SetConfigText(ctx context.Context, tx db.Tx, key string, value string, updatedBy string) error {
	updatedByUUID, err := parseUUID(updatedBy)
	if err != nil {
		return fmt.Errorf("invalid updated_by UUID: %w", err)
	}
	return s.repo.SetText(ctx, tx, key, value, updatedByUUID)
}

// parseUUID is a helper to parse UUID from string.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
