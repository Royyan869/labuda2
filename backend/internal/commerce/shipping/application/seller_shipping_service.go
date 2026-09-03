package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	shippingEntity "github.com/labuda/backend/internal/commerce/shipping/entity"
	shippingRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// SellerShippingService handles seller shipping option management.
// This service provides CRUD operations for sellers to manage their shipping options.
type SellerShippingService struct {
	shippingSetupRepo  shippingRepo.ShippingSetupRepository
	coverageRepo        shippingRepo.ShippingCoverageRepository
	cityOverrideRepo    shippingRepo.CityOverrideRepository
	productShippingRepo shippingRepo.ProductShippingSetupRepository
}

// NewSellerShippingService creates a new SellerShippingService.
func NewSellerShippingService(
	shippingSetupRepo shippingRepo.ShippingSetupRepository,
	coverageRepo shippingRepo.ShippingCoverageRepository,
	cityOverrideRepo shippingRepo.CityOverrideRepository,
	productShippingRepo shippingRepo.ProductShippingSetupRepository,
) *SellerShippingService {
	return &SellerShippingService{
		shippingSetupRepo:  shippingSetupRepo,
		coverageRepo:        coverageRepo,
		cityOverrideRepo:    cityOverrideRepo,
		productShippingRepo: productShippingRepo,
	}
}

// CreateShippingSetupInput contains parameters for creating a shipping option.
type CreateShippingSetupInput struct {
	// SellerID is the seller creating this shipping option
	SellerID uuid.UUID

	// Name is the display name for this shipping option
	Name string

	// TransportType is the category of transport (train, bus, travel, plane, custom)
	TransportType shippingEntity.TransportType
}

// CreateShippingSetup creates a new shipping option for a seller.
func (s *SellerShippingService) CreateShippingSetup(
	ctx context.Context,
	tx db.Tx,
	input CreateShippingSetupInput,
) (*shippingEntity.ShippingSetup, error) {
	// Validate name is not empty
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	// Validate transport type
	if !isValidTransportType(input.TransportType) {
		return nil, fmt.Errorf("invalid transport type: %s", input.TransportType)
	}

	// Check for duplicate name
	existing, err := s.shippingSetupRepo.GetByName(ctx, tx, input.SellerID, input.Name)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("shipping option with name '%s' already exists", input.Name)
	}

	// Create shipping option
	option := shippingEntity.NewShippingSetup(
		input.SellerID,
		input.Name,
		input.TransportType,
	)

	if err := s.shippingSetupRepo.Create(ctx, tx, option); err != nil {
		return nil, fmt.Errorf("failed to create shipping option: %w", err)
	}

	return option, nil
}

// UpdateShippingSetupInput contains parameters for updating a shipping option.
type UpdateShippingSetupInput struct {
	// ShippingSetupID is the ID of the shipping option to update
	ShippingSetupID uuid.UUID

	// SellerID is the authenticated seller ID (for ownership check)
	SellerID uuid.UUID

	// Name is the new display name (optional)
	Name string

	// TransportType is the new transport type (optional)
	TransportType shippingEntity.TransportType

	// IsActive indicates whether the option should be active
	IsActive *bool
}

// UpdateShippingSetup updates an existing shipping option.
func (s *SellerShippingService) UpdateShippingSetup(
	ctx context.Context,
	tx db.Tx,
	input UpdateShippingSetupInput,
) (*shippingEntity.ShippingSetup, error) {
	// Get the shipping option with lock
	option, err := s.shippingSetupRepo.GetForUpdate(ctx, tx, input.ShippingSetupID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}

	// Verify ownership
	if option.SellerID != input.SellerID {
		return nil, fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	// Update fields if provided
	updated := false

	if input.Name != "" && input.Name != option.Name {
		// Check for duplicate name
		existing, err := s.shippingSetupRepo.GetByName(ctx, tx, input.SellerID, input.Name)
		if err == nil && existing != nil && existing.ID != option.ID {
			return nil, fmt.Errorf("shipping option with name '%s' already exists", input.Name)
		}
		option.Name = input.Name
		updated = true
	}

	if input.TransportType != "" && input.TransportType != option.TransportType {
		if !isValidTransportType(input.TransportType) {
			return nil, fmt.Errorf("invalid transport type: %s", input.TransportType)
		}
		option.TransportType = input.TransportType
		updated = true
	}

	if input.IsActive != nil && *input.IsActive != option.IsActive {
		if *input.IsActive {
			option.Activate()
		} else {
			option.Deactivate()
		}
		updated = true
	}

	if updated {
		if err := s.shippingSetupRepo.Update(ctx, tx, option); err != nil {
			return nil, fmt.Errorf("failed to update shipping option: %w", err)
		}
	}

	return option, nil
}

// DeleteShippingSetup deletes a shipping option and its associated coverages.
func (s *SellerShippingService) DeleteShippingSetup(
	ctx context.Context,
	tx db.Tx,
	shippingSetupID uuid.UUID,
	sellerID uuid.UUID,
) error {
	// Verify ownership
	option, err := s.shippingSetupRepo.GetByID(ctx, tx, shippingSetupID)
	if err != nil {
		return fmt.Errorf("shipping option not found: %w", err)
	}

	if option.SellerID != sellerID {
		return fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	// Delete city overrides for all coverages (cascade through coverages)
	coverages, err := s.coverageRepo.GetByShippingSetup(ctx, tx, shippingSetupID)
	if err == nil {
		for _, coverage := range coverages {
			_ = s.cityOverrideRepo.DeleteByCoverage(ctx, tx, coverage.ID)
		}
	}

	// Delete all coverages
	_ = s.coverageRepo.DeleteByShippingSetup(ctx, tx, shippingSetupID)

	// Delete product-shipping links
	_ = s.productShippingRepo.DeleteByShippingSetup(ctx, tx, shippingSetupID)

	// Delete the shipping option
	if err := s.shippingSetupRepo.Delete(ctx, tx, shippingSetupID); err != nil {
		return fmt.Errorf("failed to delete shipping option: %w", err)
	}

	return nil
}

// ListSellerShippingSetups retrieves all shipping options for a seller.
func (s *SellerShippingService) ListSellerShippingSetups(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	includeInactive bool,
) ([]*shippingEntity.ShippingSetup, error) {
	return s.shippingSetupRepo.GetBySeller(ctx, tx, sellerID, !includeInactive)
}

// GetShippingSetupWithCoverages retrieves a shipping option with its coverages.
type GetShippingSetupWithCoveragesResult struct {
	ShippingSetup *shippingEntity.ShippingSetup
	Coverages      []*shippingEntity.ShippingCoverage
}

func (s *SellerShippingService) GetShippingSetupWithCoverages(
	ctx context.Context,
	tx db.Tx,
	shippingSetupID uuid.UUID,
	sellerID uuid.UUID,
) (*GetShippingSetupWithCoveragesResult, error) {
	// Get shipping option
	option, err := s.shippingSetupRepo.GetByID(ctx, tx, shippingSetupID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}

	// Verify ownership
	if option.SellerID != sellerID {
		return nil, fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	// Get coverages
	coverages, err := s.coverageRepo.GetByShippingSetup(ctx, tx, shippingSetupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get coverages: %w", err)
	}

	return &GetShippingSetupWithCoveragesResult{
		ShippingSetup: option,
		Coverages:      coverages,
	}, nil
}

// CreateCoverageInput contains parameters for creating a shipping coverage.
type CreateCoverageInput struct {
	// ShippingSetupID is the shipping option to add coverage for
	ShippingSetupID uuid.UUID

	// SellerID is the authenticated seller ID (for ownership check)
	SellerID uuid.UUID

	// ProvinceCode is the 2-digit BPS province code
	ProvinceCode string

	// ProvinceName is the province name
	ProvinceName string

	// Rate is the shipping rate for this province
	Rate int64

	// IsAvailable indicates whether shipping is available to this province
	IsAvailable bool
}

// CreateCoverage creates a new shipping coverage for a shipping option.
func (s *SellerShippingService) CreateCoverage(
	ctx context.Context,
	tx db.Tx,
	input CreateCoverageInput,
) (*shippingEntity.ShippingCoverage, error) {
	// Validate province code
	if input.ProvinceCode == "" {
		return nil, fmt.Errorf("province_code is required")
	}
	if input.ProvinceName == "" {
		return nil, fmt.Errorf("province_name is required")
	}

	// Verify ownership of shipping option
	option, err := s.shippingSetupRepo.GetByID(ctx, tx, input.ShippingSetupID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}
	if option.SellerID != input.SellerID {
		return nil, fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	// Check for existing coverage
	existing, err := s.coverageRepo.GetByOptionAndProvince(ctx, tx, input.ShippingSetupID, input.ProvinceCode)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("coverage for province '%s' already exists", input.ProvinceCode)
	}

	// Create coverage
	coverage := shippingEntity.NewShippingCoverage(
		input.ShippingSetupID,
		input.ProvinceCode,
		input.ProvinceName,
	).WithRate(money.New(input.Rate))

	if !input.IsAvailable {
		coverage.MarkUnavailable()
	}

	if err := s.coverageRepo.Create(ctx, tx, coverage); err != nil {
		return nil, fmt.Errorf("failed to create shipping coverage: %w", err)
	}

	return coverage, nil
}

// UpdateCoverageInput contains parameters for updating a shipping coverage.
type UpdateCoverageInput struct {
	// CoverageID is the ID of the coverage to update
	CoverageID uuid.UUID

	// SellerID is the authenticated seller ID (for ownership check)
	SellerID uuid.UUID

	// ProvinceName is the new province name (optional)
	ProvinceName string

	// Rate is the new shipping rate (optional)
	Rate *int64

	// IsAvailable indicates whether shipping is available (optional)
	IsAvailable *bool
}

// UpdateCoverage updates an existing shipping coverage.
func (s *SellerShippingService) UpdateCoverage(
	ctx context.Context,
	tx db.Tx,
	input UpdateCoverageInput,
) (*shippingEntity.ShippingCoverage, error) {
	// Get coverage
	coverage, err := s.coverageRepo.GetByID(ctx, tx, input.CoverageID)
	if err != nil {
		return nil, fmt.Errorf("coverage not found: %w", err)
	}

	// Verify ownership through shipping option
	option, err := s.shippingSetupRepo.GetByID(ctx, tx, coverage.ShippingSetupID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}
	if option.SellerID != input.SellerID {
		return nil, fmt.Errorf("forbidden: coverage does not belong to seller's shipping option")
	}

	// Update fields if provided
	updated := false

	if input.ProvinceName != "" && input.ProvinceName != coverage.ProvinceName {
		coverage.ProvinceName = input.ProvinceName
		updated = true
	}

	if input.Rate != nil {
		coverage.ProvinceRate = money.New(*input.Rate)
		updated = true
	}

	if input.IsAvailable != nil {
		if *input.IsAvailable {
			coverage.MarkAvailable()
		} else {
			coverage.MarkUnavailable()
		}
		updated = true
	}

	if updated {
		if err := s.coverageRepo.Update(ctx, tx, coverage); err != nil {
			return nil, fmt.Errorf("failed to update coverage: %w", err)
		}
	}

	return coverage, nil
}

// DeleteCoverage deletes a shipping coverage.
func (s *SellerShippingService) DeleteCoverage(
	ctx context.Context,
	tx db.Tx,
	coverageID uuid.UUID,
	sellerID uuid.UUID,
) error {
	// Get coverage
	coverage, err := s.coverageRepo.GetByID(ctx, tx, coverageID)
	if err != nil {
		return fmt.Errorf("coverage not found: %w", err)
	}

	// Verify ownership through shipping option
	option, err := s.shippingSetupRepo.GetByID(ctx, tx, coverage.ShippingSetupID)
	if err != nil {
		return fmt.Errorf("shipping option not found: %w", err)
	}
	if option.SellerID != sellerID {
		return fmt.Errorf("forbidden: coverage does not belong to seller's shipping option")
	}

	// Delete city overrides first
	_ = s.cityOverrideRepo.DeleteByCoverage(ctx, tx, coverageID)

	// Delete coverage
	if err := s.coverageRepo.Delete(ctx, tx, coverageID); err != nil {
		return fmt.Errorf("failed to delete coverage: %w", err)
	}

	return nil
}

// ListCoverages retrieves all coverages for a shipping option.
func (s *SellerShippingService) ListCoverages(
	ctx context.Context,
	tx db.Tx,
	shippingSetupID uuid.UUID,
	sellerID uuid.UUID,
) ([]*shippingEntity.ShippingCoverage, error) {
	// Verify ownership
	option, err := s.shippingSetupRepo.GetByID(ctx, tx, shippingSetupID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}
	if option.SellerID != sellerID {
		return nil, fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	return s.coverageRepo.GetByShippingSetup(ctx, tx, shippingSetupID)
}

// isValidTransportType checks if the transport type is valid.
func isValidTransportType(tt shippingEntity.TransportType) bool {
	switch tt {
	case shippingEntity.TransportTrain, shippingEntity.TransportBus,
		shippingEntity.TransportTravel, shippingEntity.TransportPlane,
		shippingEntity.TransportCustom:
		return true
	default:
		return false
	}
}
