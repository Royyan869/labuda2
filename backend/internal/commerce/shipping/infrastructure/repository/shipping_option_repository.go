package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/pkg/db"
)

// ShippingSetupRepository defines the interface for shipping option persistence.
type ShippingSetupRepository interface {
	// Create persists a new shipping option within a transaction.
	Create(ctx context.Context, tx db.Tx, option *entity.ShippingSetup) error

	// Update persists shipping option changes within a transaction.
	Update(ctx context.Context, tx db.Tx, option *entity.ShippingSetup) error

	// GetByID retrieves a shipping option without locking (for read-only operations).
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ShippingSetup, error)

	// GetForUpdate retrieves a shipping option with FOR UPDATE lock.
	// This prevents concurrent modifications and must be used within a transaction.
	GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ShippingSetup, error)

	// GetBySeller retrieves all shipping options for a seller.
	// Returns active options only if onlyActive is true.
	GetBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID, onlyActive bool) ([]*entity.ShippingSetup, error)

	// GetByName retrieves a shipping option by seller and name.
	GetByName(ctx context.Context, tx db.Tx, sellerID uuid.UUID, name string) (*entity.ShippingSetup, error)

	// Delete removes a shipping option.
	Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error
}

// ShippingCoverageRepository defines the interface for shipping coverage persistence.
type ShippingCoverageRepository interface {
	// Create persists a new shipping coverage within a transaction.
	Create(ctx context.Context, tx db.Tx, coverage *entity.ShippingCoverage) error

	// Update persists shipping coverage changes within a transaction.
	Update(ctx context.Context, tx db.Tx, coverage *entity.ShippingCoverage) error

	// GetByID retrieves a shipping coverage without locking.
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ShippingCoverage, error)

	// GetByShippingSetup retrieves all coverages for a shipping option.
	GetByShippingSetup(ctx context.Context, tx db.Tx, shippingSetupID uuid.UUID) ([]*entity.ShippingCoverage, error)

	// GetByOptionAndProvince retrieves a specific coverage by option and province.
	GetByOptionAndProvince(ctx context.Context, tx db.Tx, shippingSetupID uuid.UUID, provinceCode string) (*entity.ShippingCoverage, error)

	// Delete removes a shipping coverage.
	Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error

	// DeleteByShippingSetup removes all coverages for a shipping option.
	DeleteByShippingSetup(ctx context.Context, tx db.Tx, shippingSetupID uuid.UUID) error
}

// CityOverrideRepository defines the interface for city override persistence.
type CityOverrideRepository interface {
	// Create persists a new city override within a transaction.
	Create(ctx context.Context, tx db.Tx, override *entity.CityOverride) error

	// Update persists city override changes within a transaction.
	Update(ctx context.Context, tx db.Tx, override *entity.CityOverride) error

	// GetByID retrieves a city override without locking.
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.CityOverride, error)

	// GetByCoverage retrieves all overrides for a shipping coverage.
	GetByCoverage(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID) ([]*entity.CityOverride, error)

	// GetByCoverageAndCity retrieves a specific override by coverage and city.
	GetByCoverageAndCity(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID, cityCode string) (*entity.CityOverride, error)

	// Delete removes a city override.
	Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error

	// DeleteByCoverage removes all overrides for a shipping coverage.
	DeleteByCoverage(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID) error
}

// ProductShippingSetupRepository defines the interface for product-shipping link persistence.
type ProductShippingSetupRepository interface {
	// Create persists a new product-shipping link within a transaction.
	Create(ctx context.Context, tx db.Tx, productID uuid.UUID, shippingSetupID uuid.UUID, sortOrder int) error

	// Delete removes a product-shipping link within a transaction.
	Delete(ctx context.Context, tx db.Tx, productID uuid.UUID, shippingSetupID uuid.UUID) error

	// GetByProduct retrieves all shipping options linked to a product.
	GetByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) ([]*entity.ShippingSetup, error)

	// GetAvailableByProduct retrieves all available shipping options for a product, sorted by sort_order.
	GetAvailableByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) ([]*entity.ShippingSetup, error)

	// DeleteByProduct removes all shipping links for a product.
	DeleteByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) error

	// DeleteByShippingSetup removes all product links for a shipping option.
	DeleteByShippingSetup(ctx context.Context, tx db.Tx, shippingSetupID uuid.UUID) error

	// CreateBulk creates multiple product-shipping links within a transaction.
	CreateBulk(ctx context.Context, tx db.Tx, productID uuid.UUID, shippingSetupIDs []uuid.UUID) error

	// CountByProduct counts the number of shipping options linked to a product.
	CountByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) (int64, error)
}
