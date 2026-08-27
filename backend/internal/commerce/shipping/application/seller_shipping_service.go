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
	shippingOptionRepo  shippingRepo.ShippingOptionRepository
	coverageRepo        shippingRepo.ShippingCoverageRepository
	cityOverrideRepo    shippingRepo.CityOverrideRepository
	productShippingRepo shippingRepo.ProductShippingOptionRepository
}

// NewSellerShippingService creates a new SellerShippingService.
func NewSellerShippingService(
	shippingOptionRepo shippingRepo.ShippingOptionRepository,
	coverageRepo shippingRepo.ShippingCoverageRepository,
	cityOverrideRepo shippingRepo.CityOverrideRepository,
	productShippingRepo shippingRepo.ProductShippingOptionRepository,
) *SellerShippingService {
	return &SellerShippingService{
		shippingOptionRepo:  shippingOptionRepo,
		coverageRepo:        coverageRepo,
		cityOverrideRepo:    cityOverrideRepo,
		productShippingRepo: productShippingRepo,
	}
}

// CreateShippingOptionInput contains parameters for creating a shipping option.
type CreateShippingOptionInput struct {
	// SellerID is the seller creating this shipping option
	SellerID uuid.UUID

	// Name is the display name for this shipping option
	Name string

	// TransportType is the category of transport (train, bus, travel, plane, custom)
	TransportType shippingEntity.TransportType

	// ExpeditionName is the optional expedition/company name
	ExpeditionName string
}

// CreateShippingOption creates a new shipping option for a seller.
func (s *SellerShippingService) CreateShippingOption(
	ctx context.Context,
	tx db.Tx,
	input CreateShippingOptionInput,
) (*shippingEntity.ShippingOption, error) {
	// Validate name is not empty
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	// Validate transport type
	if !isValidTransportType(input.TransportType) {
		return nil, fmt.Errorf("invalid transport type: %s", input.TransportType)
	}

	// Check for duplicate name
	existing, err := s.shippingOptionRepo.GetByName(ctx, tx, input.SellerID, input.Name)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("shipping option with name '%s' already exists", input.Name)
	}

	// Create shipping option
	option := shippingEntity.NewShippingOption(
		input.SellerID,
		input.Name,
		input.TransportType,
		input.ExpeditionName,
	)

	if err := s.shippingOptionRepo.Create(ctx, tx, option); err != nil {
		return nil, fmt.Errorf("failed to create shipping option: %w", err)
	}

	return option, nil
}

// UpdateShippingOptionInput contains parameters for updating a shipping option.
type UpdateShippingOptionInput struct {
	// ShippingOptionID is the ID of the shipping option to update
	ShippingOptionID uuid.UUID

	// SellerID is the authenticated seller ID (for ownership check)
	SellerID uuid.UUID

	// Name is the new display name (optional)
	Name string

	// TransportType is the new transport type (optional)
	TransportType shippingEntity.TransportType

	// ExpeditionName is the new expedition name (optional, empty string to clear)
	ExpeditionName string

	// IsActive indicates whether the option should be active
	IsActive *bool
}

// UpdateShippingOption updates an existing shipping option.
func (s *SellerShippingService) UpdateShippingOption(
	ctx context.Context,
	tx db.Tx,
	input UpdateShippingOptionInput,
) (*shippingEntity.ShippingOption, error) {
	// Get the shipping option with lock
	option, err := s.shippingOptionRepo.GetForUpdate(ctx, tx, input.ShippingOptionID)
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
		existing, err := s.shippingOptionRepo.GetByName(ctx, tx, input.SellerID, input.Name)
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

	if input.ExpeditionName != option.GetExpeditionName() {
		option.SetExpeditionName(input.ExpeditionName)
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
		if err := s.shippingOptionRepo.Update(ctx, tx, option); err != nil {
			return nil, fmt.Errorf("failed to update shipping option: %w", err)
		}
	}

	return option, nil
}

// DeleteShippingOption deletes a shipping option and its associated coverages.
func (s *SellerShippingService) DeleteShippingOption(
	ctx context.Context,
	tx db.Tx,
	shippingOptionID uuid.UUID,
	sellerID uuid.UUID,
) error {
	// Verify ownership
	option, err := s.shippingOptionRepo.GetByID(ctx, tx, shippingOptionID)
	if err != nil {
		return fmt.Errorf("shipping option not found: %w", err)
	}

	if option.SellerID != sellerID {
		return fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	// Delete city overrides for all coverages (cascade through coverages)
	coverages, err := s.coverageRepo.GetByShippingOption(ctx, tx, shippingOptionID)
	if err == nil {
		for _, coverage := range coverages {
			_ = s.cityOverrideRepo.DeleteByCoverage(ctx, tx, coverage.ID)
		}
	}

	// Delete all coverages
	_ = s.coverageRepo.DeleteByShippingOption(ctx, tx, shippingOptionID)

	// Delete product-shipping links
	_ = s.productShippingRepo.DeleteByShippingOption(ctx, tx, shippingOptionID)

	// Delete the shipping option
	if err := s.shippingOptionRepo.Delete(ctx, tx, shippingOptionID); err != nil {
		return fmt.Errorf("failed to delete shipping option: %w", err)
	}

	return nil
}

// ListSellerShippingOptions retrieves all shipping options for a seller.
func (s *SellerShippingService) ListSellerShippingOptions(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	includeInactive bool,
) ([]*shippingEntity.ShippingOption, error) {
	return s.shippingOptionRepo.GetBySeller(ctx, tx, sellerID, !includeInactive)
}

// GetShippingOptionWithCoverages retrieves a shipping option with its coverages.
type GetShippingOptionWithCoveragesResult struct {
	ShippingOption *shippingEntity.ShippingOption
	Coverages      []*shippingEntity.ShippingCoverage
}

func (s *SellerShippingService) GetShippingOptionWithCoverages(
	ctx context.Context,
	tx db.Tx,
	shippingOptionID uuid.UUID,
	sellerID uuid.UUID,
) (*GetShippingOptionWithCoveragesResult, error) {
	// Get shipping option
	option, err := s.shippingOptionRepo.GetByID(ctx, tx, shippingOptionID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}

	// Verify ownership
	if option.SellerID != sellerID {
		return nil, fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	// Get coverages
	coverages, err := s.coverageRepo.GetByShippingOption(ctx, tx, shippingOptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get coverages: %w", err)
	}

	return &GetShippingOptionWithCoveragesResult{
		ShippingOption: option,
		Coverages:      coverages,
	}, nil
}

// CreateCoverageInput contains parameters for creating a shipping coverage.
type CreateCoverageInput struct {
	// ShippingOptionID is the shipping option to add coverage for
	ShippingOptionID uuid.UUID

	// SellerID is the authenticated seller ID (for ownership check)
	SellerID uuid.UUID

	// ProvinceCode is the 2-digit BPS province code
	ProvinceCode string

	// ProvinceName is the province name
	ProvinceName string

	// Rate is the shipping rate for this province
	Rate int64

	// EstimatedDays is the estimated delivery time (e.g., "1-2 hari")
	EstimatedDays string

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
	option, err := s.shippingOptionRepo.GetByID(ctx, tx, input.ShippingOptionID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}
	if option.SellerID != input.SellerID {
		return nil, fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	// Check for existing coverage
	existing, err := s.coverageRepo.GetByOptionAndProvince(ctx, tx, input.ShippingOptionID, input.ProvinceCode)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("coverage for province '%s' already exists", input.ProvinceCode)
	}

	// Create coverage
	coverage := shippingEntity.NewShippingCoverage(
		input.ShippingOptionID,
		input.ProvinceCode,
		input.ProvinceName,
	).WithRate(money.New(input.Rate))

	if input.EstimatedDays != "" {
		coverage.WithEstimatedDays(input.EstimatedDays)
	}

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

	// EstimatedDays is the new estimated delivery time (optional)
	EstimatedDays *string

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
	option, err := s.shippingOptionRepo.GetByID(ctx, tx, coverage.ShippingOptionID)
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

	if input.EstimatedDays != nil {
		coverage.EstimatedDays = input.EstimatedDays
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
	option, err := s.shippingOptionRepo.GetByID(ctx, tx, coverage.ShippingOptionID)
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
	shippingOptionID uuid.UUID,
	sellerID uuid.UUID,
) ([]*shippingEntity.ShippingCoverage, error) {
	// Verify ownership
	option, err := s.shippingOptionRepo.GetByID(ctx, tx, shippingOptionID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}
	if option.SellerID != sellerID {
		return nil, fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	return s.coverageRepo.GetByShippingOption(ctx, tx, shippingOptionID)
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
