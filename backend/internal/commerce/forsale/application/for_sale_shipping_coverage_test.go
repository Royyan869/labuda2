package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	shippingEntity "github.com/labuda/backend/internal/commerce/shipping/entity"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// Stubs — product shipping option repo
// ============================================================================

type stubProductShippingRepo struct {
	options []*shippingEntity.ShippingSetup
	count   int64
	err     error
}

func (r *stubProductShippingRepo) CountByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.count, nil
}

func (r *stubProductShippingRepo) GetByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*shippingEntity.ShippingSetup, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.options, nil
}

func (r *stubProductShippingRepo) GetAvailableByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*shippingEntity.ShippingSetup, error) {
	return nil, nil
}

func (r *stubProductShippingRepo) Create(_ context.Context, _ db.Tx, _ uuid.UUID, _ uuid.UUID, _ int) error {
	return nil
}

func (r *stubProductShippingRepo) Delete(_ context.Context, _ db.Tx, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

func (r *stubProductShippingRepo) DeleteByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}

func (r *stubProductShippingRepo) DeleteByShippingSetup(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}

func (r *stubProductShippingRepo) CreateBulk(_ context.Context, _ db.Tx, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}

// ============================================================================
// Stubs — coverage repo
// ============================================================================

type stubFPSCoverageRepo struct {
	coveragesByOption map[uuid.UUID][]*shippingEntity.ShippingCoverage
	err               error
}

func (r *stubFPSCoverageRepo) GetByShippingSetup(_ context.Context, _ db.Tx, optionID uuid.UUID) ([]*shippingEntity.ShippingCoverage, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.coveragesByOption[optionID], nil
}

func (r *stubFPSCoverageRepo) Create(_ context.Context, _ db.Tx, _ *shippingEntity.ShippingCoverage) error {
	return nil
}
func (r *stubFPSCoverageRepo) Update(_ context.Context, _ db.Tx, _ *shippingEntity.ShippingCoverage) error {
	return nil
}
func (r *stubFPSCoverageRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*shippingEntity.ShippingCoverage, error) {
	return nil, nil
}
func (r *stubFPSCoverageRepo) GetByOptionAndProvince(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) (*shippingEntity.ShippingCoverage, error) {
	return nil, nil
}
func (r *stubFPSCoverageRepo) Delete(_ context.Context, _ db.Tx, _ uuid.UUID) error { return nil }
func (r *stubFPSCoverageRepo) DeleteByShippingSetup(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}

// ============================================================================
// Tests
// ============================================================================

func TestEnsureShippingConfigured_ZeroOptions_Errors(t *testing.T) {
	t.Parallel()

	svc := &ForSaleService{
		productShippingRepo: &stubProductShippingRepo{count: 0, options: nil},
		coverageRepo:        &stubFPSCoverageRepo{},
	}

	err := svc.EnsureShippingConfigured(context.Background(), nil, uuid.New())
	if err == nil {
		t.Fatal("expected ErrShippingNotConfigured for zero options, got nil")
	}
	if !errors.Is(err, shippingApp.ErrShippingNotConfigured) {
		t.Fatalf("expected ErrShippingNotConfigured, got: %v", err)
	}
}

func TestEnsureShippingConfigured_OptionWithZeroCoverages_Errors(t *testing.T) {
	t.Parallel()

	optID := uuid.New()
	svc := &ForSaleService{
		productShippingRepo: &stubProductShippingRepo{
			count:   1,
			options: []*shippingEntity.ShippingSetup{{ID: optID}},
		},
		coverageRepo: &stubFPSCoverageRepo{
			coveragesByOption: map[uuid.UUID][]*shippingEntity.ShippingCoverage{
				optID: {}, // no coverages
			},
		},
	}

	err := svc.EnsureShippingConfigured(context.Background(), nil, uuid.New())
	if err == nil {
		t.Fatal("expected ErrShippingNotConfigured for option with zero coverages, got nil")
	}
	if !errors.Is(err, shippingApp.ErrShippingNotConfigured) {
		t.Fatalf("expected ErrShippingNotConfigured, got: %v", err)
	}
}

func TestEnsureShippingConfigured_AllCoveragesInactive_Errors(t *testing.T) {
	t.Parallel()

	optID := uuid.New()
	svc := &ForSaleService{
		productShippingRepo: &stubProductShippingRepo{
			count:   1,
			options: []*shippingEntity.ShippingSetup{{ID: optID}},
		},
		coverageRepo: &stubFPSCoverageRepo{
			coveragesByOption: map[uuid.UUID][]*shippingEntity.ShippingCoverage{
				optID: {
					{ID: uuid.New(), ShippingSetupID: optID, IsAvailable: false},
					{ID: uuid.New(), ShippingSetupID: optID, IsAvailable: false},
				},
			},
		},
	}

	err := svc.EnsureShippingConfigured(context.Background(), nil, uuid.New())
	if err == nil {
		t.Fatal("expected ErrShippingNotConfigured when all coverages inactive, got nil")
	}
	if !errors.Is(err, shippingApp.ErrShippingNotConfigured) {
		t.Fatalf("expected ErrShippingNotConfigured, got: %v", err)
	}
}

func TestEnsureShippingConfigured_HasActiveCoverage_Passes(t *testing.T) {
	t.Parallel()

	optID := uuid.New()
	svc := &ForSaleService{
		productShippingRepo: &stubProductShippingRepo{
			count:   1,
			options: []*shippingEntity.ShippingSetup{{ID: optID}},
		},
		coverageRepo: &stubFPSCoverageRepo{
			coveragesByOption: map[uuid.UUID][]*shippingEntity.ShippingCoverage{
				optID: {
					{ID: uuid.New(), ShippingSetupID: optID, IsAvailable: false},
					{ID: uuid.New(), ShippingSetupID: optID, IsAvailable: true}, // one active
				},
			},
		},
	}

	err := svc.EnsureShippingConfigured(context.Background(), nil, uuid.New())
	if err != nil {
		t.Fatalf("expected nil error with one active coverage, got: %v", err)
	}
}

func TestEnsureShippingConfigured_MultipleOptions_FirstWithActiveCoverage_Passes(t *testing.T) {
	t.Parallel()

	opt1 := uuid.New()
	opt2 := uuid.New()
	svc := &ForSaleService{
		productShippingRepo: &stubProductShippingRepo{
			count: 2,
			options: []*shippingEntity.ShippingSetup{
				{ID: opt1},
				{ID: opt2},
			},
		},
		coverageRepo: &stubFPSCoverageRepo{
			coveragesByOption: map[uuid.UUID][]*shippingEntity.ShippingCoverage{
				opt1: {{ID: uuid.New(), ShippingSetupID: opt1, IsAvailable: true}},
				opt2: {}, // second has none — first already passes
			},
		},
	}

	err := svc.EnsureShippingConfigured(context.Background(), nil, uuid.New())
	if err != nil {
		t.Fatalf("expected nil error when first option has active coverage, got: %v", err)
	}
}
