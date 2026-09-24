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

// BUSINESS TRUTH (Owner-locked shipping contract):
//
// 1. A shipping option is saved as ONE PACKAGE: identity (name + transport
//    type) and destinations (provinces with rates + optional city
//    qualifications) are persisted in a single transaction. An option without
//    at least one destination-with-rate can never exist.
// 2. Tariffs belong to the seller (no live-animal courier API exists). The
//    input rate is ALL-IN: shipping + packing. There is no separate packing
//    field by design — only a UI hint on the seller form.
// 3. internal_purpose is a seller-private note (e.g. "kantong besar", "untuk
//    1 ekor"). It must never appear on buyer-facing payloads.
// 4. Options are editable at any time (seller subscription tariffs change).
//    Orders keep their checkout snapshot; order creation re-validates
//    coverage. The only protection: an option linked to any listing can never
//    be hard-deleted — only deactivated.
//
// Forbidden (killed) design: metadata-first creation (name + type only, bare
// options saved before coverage exists) and per-coverage CRUD as separate
// seller flows.

// SellerShippingService handles seller shipping option management.
type SellerShippingService struct {
	shippingSetupRepo   shippingRepo.ShippingSetupRepository
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
		shippingSetupRepo:   shippingSetupRepo,
		coverageRepo:        coverageRepo,
		cityOverrideRepo:    cityOverrideRepo,
		productShippingRepo: productShippingRepo,
	}
}

// ============================================================================
// One-package inputs
// ============================================================================

// CityQualificationInput is a city-level override inside a province input.
// Zero-value fields mean "inherit the province default".
type CityQualificationInput struct {
	CityCode    string
	CityName    string
	Rate        *int64 // nil = inherit province rate
	IsAvailable *bool  // nil = inherit province availability
}

// ProvinceDestinationInput is a destination province inside a shipping package.
type ProvinceDestinationInput struct {
	ProvinceCode string
	ProvinceName string
	Rate         int64
	IsAvailable  bool
	// CityQualifications are optional city-level overrides for this province.
	CityQualifications []CityQualificationInput
}

// ShippingPackageInput is the full seller-authored shipping option payload.
type ShippingPackageInput struct {
	SellerID        uuid.UUID
	Name            string
	TransportType   shippingEntity.TransportType
	InternalPurpose string // seller-private note; never exposed to buyers
	IsActive        *bool  // nil = default true on create / unchanged on update
	// Destinations MUST contain at least one province. This is the hard gate
	// that makes bare (destination-less) options unrepresentable.
	Destinations []ProvinceDestinationInput
}

// validatePackageInput enforces the one-package business gate.
func (s *SellerShippingService) validatePackageInput(input ShippingPackageInput) error {
	if input.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !isValidTransportType(input.TransportType) {
		return fmt.Errorf("invalid transport type: %s", input.TransportType)
	}
	if len(input.Destinations) == 0 {
		return fmt.Errorf("%w: at least one province destination with a rate is required", ErrShippingPackageIncomplete)
	}
	seenProvince := make(map[string]struct{}, len(input.Destinations))
	for _, dest := range input.Destinations {
		if dest.ProvinceCode == "" {
			return fmt.Errorf("province_code is required for every destination")
		}
		if dest.ProvinceName == "" {
			return fmt.Errorf("province_name is required for every destination")
		}
		if _, dup := seenProvince[dest.ProvinceCode]; dup {
			return fmt.Errorf("duplicate destination for province '%s'", dest.ProvinceCode)
		}
		seenProvince[dest.ProvinceCode] = struct{}{}

		seenCity := make(map[string]struct{}, len(dest.CityQualifications))
		for _, city := range dest.CityQualifications {
			if city.CityCode == "" {
				return fmt.Errorf("city_code is required for every city qualification")
			}
			if _, dup := seenCity[city.CityCode]; dup {
				return fmt.Errorf("duplicate city qualification '%s' for province '%s'", city.CityCode, dest.ProvinceCode)
			}
			seenCity[city.CityCode] = struct{}{}
		}
	}
	return nil
}

// buildCoverageEntities converts package destinations into coverage entities
// (with hydrated city override children). Shared by create and update paths.
func buildCoverageEntities(setupID uuid.UUID, destinations []ProvinceDestinationInput) []*shippingEntity.ShippingCoverage {
	coverages := make([]*shippingEntity.ShippingCoverage, 0, len(destinations))
	for _, dest := range destinations {
		coverage := shippingEntity.NewShippingCoverage(setupID, dest.ProvinceCode, dest.ProvinceName).
			WithRate(money.New(dest.Rate))
		if !dest.IsAvailable {
			coverage.MarkUnavailable()
		}
		for _, city := range dest.CityQualifications {
			override := shippingEntity.NewCityOverride(coverage.ID, city.CityCode, city.CityName)
			if city.Rate != nil {
				override.SetRate(money.New(*city.Rate))
			}
			// Availability is materialized (schema: is_available NOT NULL): an
			// unqualified city writes the province default. Effective value is
			// identical to inherit-semantics; rate stays NULL = inherit province.
			cityAvailable := dest.IsAvailable
			if city.IsAvailable != nil {
				cityAvailable = *city.IsAvailable
			}
			override.SetAvailable(cityAvailable)
			coverage.CityOverrides = append(coverage.CityOverrides, override)
		}
		coverages = append(coverages, coverage)
	}
	return coverages
}

// ============================================================================
// Create — one transaction, one package
// ============================================================================

// CreateShippingPackage creates a shipping option together with its full
// destination set (provinces + city qualifications) in ONE transaction.
// A bare option (zero destinations) can never be persisted.
func (s *SellerShippingService) CreateShippingPackage(
	ctx context.Context,
	tx db.Tx,
	input ShippingPackageInput,
) (*shippingEntity.ShippingSetup, error) {
	if err := s.validatePackageInput(input); err != nil {
		return nil, err
	}

	existing, err := s.shippingSetupRepo.GetByName(ctx, tx, input.SellerID, input.Name)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("shipping option with name '%s' already exists", input.Name)
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	option := shippingEntity.NewShippingSetup(
		input.SellerID,
		input.Name,
		input.TransportType,
		input.InternalPurpose,
	)
	option.IsActive = isActive

	if err := s.shippingSetupRepo.Create(ctx, tx, option); err != nil {
		return nil, fmt.Errorf("failed to create shipping option: %w", err)
	}

	for _, coverage := range buildCoverageEntities(option.ID, input.Destinations) {
		if err := s.coverageRepo.Create(ctx, tx, coverage); err != nil {
			return nil, fmt.Errorf("failed to create coverage for province '%s': %w", coverage.ProvinceCode, err)
		}
		for _, override := range coverage.CityOverrides {
			if err := s.cityOverrideRepo.Create(ctx, tx, override); err != nil {
				return nil, fmt.Errorf("failed to create city override '%s': %w", override.CityCode, err)
			}
		}
	}

	return option, nil
}

// ============================================================================
// Update — one transaction, full replace of destinations
// ============================================================================

// UpdateShippingPackage replaces the option identity and its complete
// destination set in ONE transaction (overwrite semantics, same as the
// product-link service). Allowed at any time: orders keep their checkout
// snapshot and order creation re-validates coverage at the gate.
// The result includes the persisted option (with refreshed identity fields).
func (s *SellerShippingService) UpdateShippingPackage(
	ctx context.Context,
	tx db.Tx,
	shippingSetupID uuid.UUID,
	input ShippingPackageInput,
) (*shippingEntity.ShippingSetup, error) {
	if err := s.validatePackageInput(input); err != nil {
		return nil, err
	}

	// Locked read so identity update + destination replacement are atomic
	// against concurrent edits and checkout-time reads.
	option, err := s.shippingSetupRepo.GetForUpdate(ctx, tx, shippingSetupID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}
	if option.SellerID != input.SellerID {
		return nil, fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	if input.Name != option.Name {
		existing, err := s.shippingSetupRepo.GetByName(ctx, tx, input.SellerID, input.Name)
		if err == nil && existing != nil && existing.ID != option.ID {
			return nil, fmt.Errorf("shipping option with name '%s' already exists", input.Name)
		}
		option.Name = input.Name
	}
	option.TransportType = input.TransportType
	option.InternalPurpose = input.InternalPurpose
	if input.IsActive != nil {
		option.IsActive = *input.IsActive
	}

	if err := s.shippingSetupRepo.Update(ctx, tx, option); err != nil {
		return nil, fmt.Errorf("failed to update shipping option: %w", err)
	}

	// Full replace of destinations: delete city overrides → coverages → insert.
	if err := s.coverageRepo.DeleteByShippingSetup(ctx, tx, shippingSetupID); err != nil {
		return nil, fmt.Errorf("failed to replace destinations: %w", err)
	}

	for _, coverage := range buildCoverageEntities(shippingSetupID, input.Destinations) {
		if err := s.coverageRepo.Create(ctx, tx, coverage); err != nil {
			return nil, fmt.Errorf("failed to create coverage for province '%s': %w", coverage.ProvinceCode, err)
		}
		for _, override := range coverage.CityOverrides {
			if err := s.cityOverrideRepo.Create(ctx, tx, override); err != nil {
				return nil, fmt.Errorf("failed to create city override '%s': %w", override.CityCode, err)
			}
		}
	}

	return option, nil
}

// ============================================================================
// Delete — guarded
// ============================================================================

// DeleteShippingSetup hard-deletes an option ONLY when it is not linked to
// any listing. Linked options are part of order history (snapshot name/type/
// rate) and must be deactivated instead. Coverages and city overrides are
// removed with the option.
func (s *SellerShippingService) DeleteShippingSetup(
	ctx context.Context,
	tx db.Tx,
	shippingSetupID uuid.UUID,
	sellerID uuid.UUID,
) error {
	option, err := s.shippingSetupRepo.GetByID(ctx, tx, shippingSetupID)
	if err != nil {
		return fmt.Errorf("shipping option not found: %w", err)
	}
	if option.SellerID != sellerID {
		return fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	linkCount, err := s.productShippingRepo.CountLinksByShippingSetup(ctx, tx, shippingSetupID)
	if err != nil {
		return fmt.Errorf("failed to check shipping option links: %w", err)
	}
	if linkCount > 0 {
		return fmt.Errorf("%w: %d listing link(s) exist", ErrShippingLinkedOptionUndeletable, linkCount)
	}

	coverages, err := s.coverageRepo.GetByShippingSetup(ctx, tx, shippingSetupID)
	if err != nil {
		return fmt.Errorf("failed to load coverages: %w", err)
	}
	for _, coverage := range coverages {
		if err := s.cityOverrideRepo.DeleteByCoverage(ctx, tx, coverage.ID); err != nil {
			return fmt.Errorf("failed to delete city overrides: %w", err)
		}
	}
	if err := s.coverageRepo.DeleteByShippingSetup(ctx, tx, shippingSetupID); err != nil {
		return fmt.Errorf("failed to delete coverages: %w", err)
	}
	if err := s.shippingSetupRepo.Delete(ctx, tx, shippingSetupID); err != nil {
		return fmt.Errorf("failed to delete shipping option: %w", err)
	}
	return nil
}

// SetShippingSetupActive toggles availability of an option (the canonical way
// to retire an option that is still linked to listings).
func (s *SellerShippingService) SetShippingSetupActive(
	ctx context.Context,
	tx db.Tx,
	shippingSetupID uuid.UUID,
	sellerID uuid.UUID,
	isActive bool,
) (*shippingEntity.ShippingSetup, error) {
	option, err := s.shippingSetupRepo.GetForUpdate(ctx, tx, shippingSetupID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}
	if option.SellerID != sellerID {
		return nil, fmt.Errorf("forbidden: shipping option does not belong to seller")
	}
	option.IsActive = isActive
	if err := s.shippingSetupRepo.Update(ctx, tx, option); err != nil {
		return nil, fmt.Errorf("failed to update shipping option: %w", err)
	}
	return option, nil
}

// ============================================================================
// Reads
// ============================================================================

// ListSellerShippingSetups retrieves all shipping options for a seller.
func (s *SellerShippingService) ListSellerShippingSetups(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	includeInactive bool,
) ([]*shippingEntity.ShippingSetup, error) {
	return s.shippingSetupRepo.GetBySeller(ctx, tx, sellerID, !includeInactive)
}

// GetShippingSetupWithCoveragesResult is a hydrated option read.
type GetShippingSetupWithCoveragesResult struct {
	ShippingSetup *shippingEntity.ShippingSetup
	Coverages     []*shippingEntity.ShippingCoverage
}

// GetShippingSetupWithCoverages retrieves an option with its coverages and
// city qualifications (seller-facing edit form needs the full package).
func (s *SellerShippingService) GetShippingSetupWithCoverages(
	ctx context.Context,
	tx db.Tx,
	shippingSetupID uuid.UUID,
	sellerID uuid.UUID,
) (*GetShippingSetupWithCoveragesResult, error) {
	option, err := s.shippingSetupRepo.GetByID(ctx, tx, shippingSetupID)
	if err != nil {
		return nil, fmt.Errorf("shipping option not found: %w", err)
	}
	if option.SellerID != sellerID {
		return nil, fmt.Errorf("forbidden: shipping option does not belong to seller")
	}

	coverages, err := s.coverageRepo.GetByShippingSetup(ctx, tx, shippingSetupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get coverages: %w", err)
	}
	for _, coverage := range coverages {
		overrides, err := s.cityOverrideRepo.GetByCoverage(ctx, tx, coverage.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get city overrides: %w", err)
		}
		coverage.CityOverrides = overrides
	}

	return &GetShippingSetupWithCoveragesResult{
		ShippingSetup: option,
		Coverages:     coverages,
	}, nil
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
