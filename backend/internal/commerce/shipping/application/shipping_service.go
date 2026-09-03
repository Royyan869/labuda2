package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// DeliveryOption represents a delivery option available for a product to a specific location.
// This is a read-only DTO returned by CheckDeliveryAvailability.
type DeliveryOption struct {
	// ShippingSetupID identifies the seller's shipping option
	ShippingSetupID uuid.UUID

	// Name is the display name of the shipping option
	Name string

	// TransportType is the general transport category (train, bus, travel, plane, custom)
	TransportType entity.TransportType

	// Rate is the final shipping rate for this location (province rate or city override)
	Rate int64

	// IsAvailable indicates whether delivery is available to the specified location
	IsAvailable bool
}

// CheckDeliveryAvailabilityInput contains the parameters for checking delivery availability.
type CheckDeliveryAvailabilityInput struct {
	// ProductID is the physical product to check shipping options for
	ProductID uuid.UUID

	// ProvinceCode is the 2-digit BPS province code (e.g., "31" for DKI Jakarta)
	ProvinceCode string

	// CityCode is the optional 4-digit BPS city code (e.g., "3171" for Jakarta Selatan)
	// If empty, only province-level coverage is checked
	CityCode string
}

// ShippingService provides delivery availability checking functionality.
// This is a read-only service - no mutations are performed.
type ShippingService struct {
	shippingSetupRepo  repository.ShippingSetupRepository
	coverageRepo        repository.ShippingCoverageRepository
	cityOverrideRepo    repository.CityOverrideRepository
	productShippingRepo repository.ProductShippingSetupRepository
}

// NewShippingService creates a new ShippingService with the provided repositories.
func NewShippingService(
	shippingSetupRepo repository.ShippingSetupRepository,
	coverageRepo repository.ShippingCoverageRepository,
	cityOverrideRepo repository.CityOverrideRepository,
	productShippingRepo repository.ProductShippingSetupRepository,
) *ShippingService {
	return &ShippingService{
		shippingSetupRepo:  shippingSetupRepo,
		coverageRepo:        coverageRepo,
		cityOverrideRepo:    cityOverrideRepo,
		productShippingRepo: productShippingRepo,
	}
}

// CheckDeliveryAvailability checks which delivery options are available for a product to a specific location.
//
// Query Flow (Single-Rate Model):
//  1. Load all shipping options linked to the product via product_shipping_options
//  2. For each shipping option:
//     a. Skip if is_active = false
//     b. Find shipping_coverage by provinceCode
//     c. Skip if no coverage exists or is_available = false
//     d. Use coverage.province_rate as base rate
//     e. If cityCode is provided:
//     - Check for city_override
//     - If override.rate exists: use override rate instead of province_rate
//     - If override.is_available exists: use override availability
//     f. Build DeliveryOption with final rate
//  3. Return slice of DeliveryOption (empty if no options available)
//
// This is a read-only operation - no mutations occur, no DB locks are used.
func (s *ShippingService) CheckDeliveryAvailability(
	ctx context.Context,
	tx db.Tx,
	input CheckDeliveryAvailabilityInput,
) ([]DeliveryOption, error) {
	// Step 1: Load shipping options linked to this product
	shippingSetups, err := s.productShippingRepo.GetByProduct(ctx, tx, input.ProductID)
	if err != nil {
		return nil, err
	}

	// If no shipping options linked to product, return empty slice (not an error)
	if len(shippingSetups) == 0 {
		return []DeliveryOption{}, nil
	}

	var results []DeliveryOption

	// Step 2: For each shipping option, check coverage and build delivery option
	for _, opt := range shippingSetups {
		// 2a. Skip inactive options
		if !opt.IsActive {
			continue
		}

		// 2b. Find coverage by province
		coverage, err := s.coverageRepo.GetByOptionAndProvince(ctx, tx, opt.ID, input.ProvinceCode)
		if err != nil {
			// Coverage not found is not an error - just skip this option
			continue
		}

		// 2c. Skip if coverage is not available
		if !coverage.IsAvailable {
			continue
		}

		// 2d. Use province_rate as base rate
		rate := coverage.ProvinceRate.Int64()
		isAvailable := true

		// 2e. Check city override if cityCode is provided
		if input.CityCode != "" {
			cityOverride, err := s.cityOverrideRepo.GetByCoverageAndCity(ctx, tx, coverage.ID, input.CityCode)
			if err == nil && cityOverride != nil {
				// City override exists - apply overrides

				// Use override rate if set
				if cityOverride.Rate != nil {
					rate = cityOverride.Rate.Int64()
				}

				// Use override availability if set
				if cityOverride.IsAvailable != nil {
					isAvailable = *cityOverride.IsAvailable
				}
			}
			// If no city override found, province-level settings apply
		}

		// Skip final option if not available
		if !isAvailable {
			continue
		}

		// 2f. Build delivery option
		results = append(results, DeliveryOption{
			ShippingSetupID: opt.ID,
			Name:             opt.Name,
			TransportType:    opt.TransportType,
			Rate:             rate,
			IsAvailable:      true,
		})
	}

	// Step 3: Return results (empty slice if no options available)
	return results, nil
}

// CheckDeliveryAvailabilityForProduct is a convenience wrapper that checks delivery availability
// for a product.
func (s *ShippingService) CheckDeliveryAvailabilityForProduct(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
	provinceCode string,
	cityCode string,
) ([]DeliveryOption, error) {
	return s.CheckDeliveryAvailability(ctx, tx, CheckDeliveryAvailabilityInput{
		ProductID:    productID,
		ProvinceCode: provinceCode,
		CityCode:     cityCode,
	})
}

// HasAnyShippingSetupsForProduct reports whether a product has at least one
// shipping option linked. The buyer-facing HTTP handler uses this to expose
// `product_configured` so the UI can distinguish "no product shipping link
// exists yet" from "the buyer's address is outside coverage".
func (s *ShippingService) HasAnyShippingSetupsForProduct(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) (bool, error) {
	count, err := s.productShippingRepo.CountByProduct(ctx, tx, productID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
