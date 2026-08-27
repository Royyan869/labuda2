package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/pkg/db"
)

func TestShippingService_CheckDeliveryAvailability_UsesProductID(t *testing.T) {
	t.Parallel()

	productID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	productRepo := &recordingProductShippingRepo{}
	service := NewShippingService(
		&stubShippingOptionRepository{},
		&stubShippingCoverageRepository{},
		&stubCityOverrideRepository{},
		productRepo,
	)

	options, err := service.CheckDeliveryAvailability(
		context.Background(),
		nil,
		CheckDeliveryAvailabilityInput{
			ProductID:    productID,
			ProvinceCode: "31",
			CityCode:     "3171",
		},
	)
	if err != nil {
		t.Fatalf("CheckDeliveryAvailability returned error: %v", err)
	}
	if len(options) != 0 {
		t.Fatalf("expected no options, got %d", len(options))
	}
	if productRepo.lastGetByProduct != productID {
		t.Fatalf("expected GetByProduct to use %s, got %s", productID, productRepo.lastGetByProduct)
	}
}

func TestShippingService_HasAnyShippingOptionsForProduct_UsesProductID(t *testing.T) {
	t.Parallel()

	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	productRepo := &recordingProductShippingRepo{countByProduct: 2}
	service := NewShippingService(
		&stubShippingOptionRepository{},
		&stubShippingCoverageRepository{},
		&stubCityOverrideRepository{},
		productRepo,
	)

	hasAny, err := service.HasAnyShippingOptionsForProduct(context.Background(), nil, productID)
	if err != nil {
		t.Fatalf("HasAnyShippingOptionsForProduct returned error: %v", err)
	}
	if !hasAny {
		t.Fatal("expected product to report shipping options")
	}
	if productRepo.lastCountByProduct != productID {
		t.Fatalf("expected CountByProduct to use %s, got %s", productID, productRepo.lastCountByProduct)
	}
}

type stubShippingOptionRepository struct{}

func (r *stubShippingOptionRepository) Create(ctx context.Context, tx db.Tx, option *entity.ShippingOption) error {
	return nil
}

func (r *stubShippingOptionRepository) Update(ctx context.Context, tx db.Tx, option *entity.ShippingOption) error {
	return nil
}

func (r *stubShippingOptionRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ShippingOption, error) {
	return nil, nil
}

func (r *stubShippingOptionRepository) GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ShippingOption, error) {
	return nil, nil
}

func (r *stubShippingOptionRepository) GetBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID, onlyActive bool) ([]*entity.ShippingOption, error) {
	return nil, nil
}

func (r *stubShippingOptionRepository) GetByName(ctx context.Context, tx db.Tx, sellerID uuid.UUID, name string) (*entity.ShippingOption, error) {
	return nil, nil
}

func (r *stubShippingOptionRepository) Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	return nil
}

type stubShippingCoverageRepository struct{}

func (r *stubShippingCoverageRepository) Create(ctx context.Context, tx db.Tx, coverage *entity.ShippingCoverage) error {
	return nil
}

func (r *stubShippingCoverageRepository) Update(ctx context.Context, tx db.Tx, coverage *entity.ShippingCoverage) error {
	return nil
}

func (r *stubShippingCoverageRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ShippingCoverage, error) {
	return nil, nil
}

func (r *stubShippingCoverageRepository) GetByShippingOption(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID) ([]*entity.ShippingCoverage, error) {
	return nil, nil
}

func (r *stubShippingCoverageRepository) GetByOptionAndProvince(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID, provinceCode string) (*entity.ShippingCoverage, error) {
	return nil, nil
}

func (r *stubShippingCoverageRepository) Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	return nil
}

func (r *stubShippingCoverageRepository) DeleteByShippingOption(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID) error {
	return nil
}

type stubCityOverrideRepository struct{}

func (r *stubCityOverrideRepository) Create(ctx context.Context, tx db.Tx, override *entity.CityOverride) error {
	return nil
}

func (r *stubCityOverrideRepository) Update(ctx context.Context, tx db.Tx, override *entity.CityOverride) error {
	return nil
}

func (r *stubCityOverrideRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.CityOverride, error) {
	return nil, nil
}

func (r *stubCityOverrideRepository) GetByCoverage(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID) ([]*entity.CityOverride, error) {
	return nil, nil
}

func (r *stubCityOverrideRepository) GetByCoverageAndCity(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID, cityCode string) (*entity.CityOverride, error) {
	return nil, nil
}

func (r *stubCityOverrideRepository) Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	return nil
}

func (r *stubCityOverrideRepository) DeleteByCoverage(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID) error {
	return nil
}

type recordingProductShippingRepo struct {
	lastGetByProduct   uuid.UUID
	lastCountByProduct uuid.UUID
	countByProduct     int64
}

func (r *recordingProductShippingRepo) Create(ctx context.Context, tx db.Tx, productID uuid.UUID, shippingOptionID uuid.UUID, sortOrder int) error {
	return nil
}

func (r *recordingProductShippingRepo) Delete(ctx context.Context, tx db.Tx, productID uuid.UUID, shippingOptionID uuid.UUID) error {
	return nil
}

func (r *recordingProductShippingRepo) GetByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) ([]*entity.ShippingOption, error) {
	r.lastGetByProduct = productID
	return nil, nil
}

func (r *recordingProductShippingRepo) GetAvailableByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) ([]*entity.ShippingOption, error) {
	return nil, nil
}

func (r *recordingProductShippingRepo) DeleteByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) error {
	return nil
}

func (r *recordingProductShippingRepo) DeleteByShippingOption(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID) error {
	return nil
}

func (r *recordingProductShippingRepo) CreateBulk(ctx context.Context, tx db.Tx, productID uuid.UUID, shippingOptionIDs []uuid.UUID) error {
	return nil
}

func (r *recordingProductShippingRepo) CountByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) (int64, error) {
	r.lastCountByProduct = productID
	return r.countByProduct, nil
}
