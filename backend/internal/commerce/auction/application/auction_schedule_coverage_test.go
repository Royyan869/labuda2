package application

// Tests for shipping coverage validation in auction Schedule flow.
//
// ensureShippingCoverage is the private gate introduced to block Schedule when
// the auction product has no shipping options with active (is_available=true)
// coverage. Tests exercise it directly (same package) and via Schedule().

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingEntity "github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// Stubs — product shipping option repo
// ============================================================================

type scheduleStubProductShippingRepo struct {
	options []*shippingEntity.ShippingOption
	err     error
}

func (r *scheduleStubProductShippingRepo) GetByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*shippingEntity.ShippingOption, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.options, nil
}

// Unused interface methods — required to satisfy ProductShippingOptionRepository.
func (r *scheduleStubProductShippingRepo) GetAvailableByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*shippingEntity.ShippingOption, error) {
	return nil, nil
}
func (r *scheduleStubProductShippingRepo) Create(_ context.Context, _ db.Tx, _ uuid.UUID, _ uuid.UUID, _ int) error {
	return nil
}
func (r *scheduleStubProductShippingRepo) Delete(_ context.Context, _ db.Tx, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}
func (r *scheduleStubProductShippingRepo) DeleteByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}
func (r *scheduleStubProductShippingRepo) DeleteByShippingOption(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}
func (r *scheduleStubProductShippingRepo) CreateBulk(_ context.Context, _ db.Tx, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}
func (r *scheduleStubProductShippingRepo) CountByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return int64(len(r.options)), nil
}

// ============================================================================
// Stubs — coverage repo
// ============================================================================

type scheduleStubCoverageRepo struct {
	coveragesByOption map[uuid.UUID][]*shippingEntity.ShippingCoverage
	err               error
}

func (r *scheduleStubCoverageRepo) GetByShippingOption(_ context.Context, _ db.Tx, optionID uuid.UUID) ([]*shippingEntity.ShippingCoverage, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.coveragesByOption[optionID], nil
}

// Unused interface methods.
func (r *scheduleStubCoverageRepo) Create(_ context.Context, _ db.Tx, _ *shippingEntity.ShippingCoverage) error {
	return nil
}
func (r *scheduleStubCoverageRepo) Update(_ context.Context, _ db.Tx, _ *shippingEntity.ShippingCoverage) error {
	return nil
}
func (r *scheduleStubCoverageRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*shippingEntity.ShippingCoverage, error) {
	return nil, nil
}
func (r *scheduleStubCoverageRepo) GetByOptionAndProvince(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) (*shippingEntity.ShippingCoverage, error) {
	return nil, nil
}
func (r *scheduleStubCoverageRepo) Delete(_ context.Context, _ db.Tx, _ uuid.UUID) error { return nil }
func (r *scheduleStubCoverageRepo) DeleteByShippingOption(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}

// ============================================================================
// helpers
// ============================================================================

func newAuctionServiceWithShippingStubs(
	productShippingRepo *scheduleStubProductShippingRepo,
	coverageRepo *scheduleStubCoverageRepo,
) *AuctionService {
	return &AuctionService{
		productShippingRepo:  productShippingRepo,
		shippingCoverageRepo: coverageRepo,
	}
}

// ============================================================================
// ensureShippingCoverage tests (private method, same package)
// ============================================================================

func TestEnsureShippingCoverage_ZeroOptions_Errors(t *testing.T) {
	t.Parallel()

	svc := newAuctionServiceWithShippingStubs(
		&scheduleStubProductShippingRepo{options: nil},
		&scheduleStubCoverageRepo{},
	)

	err := svc.ensureShippingCoverage(context.Background(), nil, uuid.New())
	if err == nil {
		t.Fatal("expected ErrShippingNotConfigured for zero options, got nil")
	}
	if !errors.Is(err, shippingApp.ErrShippingNotConfigured) {
		t.Fatalf("expected ErrShippingNotConfigured, got: %v", err)
	}
}

func TestEnsureShippingCoverage_OptionWithNoCoverages_Errors(t *testing.T) {
	t.Parallel()

	optID := uuid.New()
	svc := newAuctionServiceWithShippingStubs(
		&scheduleStubProductShippingRepo{
			options: []*shippingEntity.ShippingOption{{ID: optID}},
		},
		&scheduleStubCoverageRepo{
			coveragesByOption: map[uuid.UUID][]*shippingEntity.ShippingCoverage{
				optID: {}, // zero coverages
			},
		},
	)

	err := svc.ensureShippingCoverage(context.Background(), nil, uuid.New())
	if err == nil {
		t.Fatal("expected ErrShippingNotConfigured for option with no coverages, got nil")
	}
	if !errors.Is(err, shippingApp.ErrShippingNotConfigured) {
		t.Fatalf("expected ErrShippingNotConfigured, got: %v", err)
	}
}

func TestEnsureShippingCoverage_AllCoveragesInactive_Errors(t *testing.T) {
	t.Parallel()

	optID := uuid.New()
	svc := newAuctionServiceWithShippingStubs(
		&scheduleStubProductShippingRepo{
			options: []*shippingEntity.ShippingOption{{ID: optID}},
		},
		&scheduleStubCoverageRepo{
			coveragesByOption: map[uuid.UUID][]*shippingEntity.ShippingCoverage{
				optID: {
					{ID: uuid.New(), ShippingOptionID: optID, IsAvailable: false},
					{ID: uuid.New(), ShippingOptionID: optID, IsAvailable: false},
				},
			},
		},
	)

	err := svc.ensureShippingCoverage(context.Background(), nil, uuid.New())
	if err == nil {
		t.Fatal("expected ErrShippingNotConfigured when all coverages inactive, got nil")
	}
	if !errors.Is(err, shippingApp.ErrShippingNotConfigured) {
		t.Fatalf("expected ErrShippingNotConfigured, got: %v", err)
	}
}

func TestEnsureShippingCoverage_HasActiveCoverage_Passes(t *testing.T) {
	t.Parallel()

	optID := uuid.New()
	svc := newAuctionServiceWithShippingStubs(
		&scheduleStubProductShippingRepo{
			options: []*shippingEntity.ShippingOption{{ID: optID}},
		},
		&scheduleStubCoverageRepo{
			coveragesByOption: map[uuid.UUID][]*shippingEntity.ShippingCoverage{
				optID: {
					{ID: uuid.New(), ShippingOptionID: optID, IsAvailable: false},
					{ID: uuid.New(), ShippingOptionID: optID, IsAvailable: true}, // one active
				},
			},
		},
	)

	err := svc.ensureShippingCoverage(context.Background(), nil, uuid.New())
	if err != nil {
		t.Fatalf("expected nil error with one active coverage, got: %v", err)
	}
}

func TestEnsureShippingCoverage_MultipleOptions_OnlySecondHasCoverage_Passes(t *testing.T) {
	t.Parallel()

	opt1 := uuid.New()
	opt2 := uuid.New()
	svc := newAuctionServiceWithShippingStubs(
		&scheduleStubProductShippingRepo{
			options: []*shippingEntity.ShippingOption{{ID: opt1}, {ID: opt2}},
		},
		&scheduleStubCoverageRepo{
			coveragesByOption: map[uuid.UUID][]*shippingEntity.ShippingCoverage{
				opt1: {}, // first has none
				opt2: {{ID: uuid.New(), ShippingOptionID: opt2, IsAvailable: true}}, // second passes
			},
		},
	)

	err := svc.ensureShippingCoverage(context.Background(), nil, uuid.New())
	if err != nil {
		t.Fatalf("expected nil error when second option has active coverage, got: %v", err)
	}
}

func TestEnsureShippingCoverage_ProductShippingRepoError_Propagates(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("db connection failed")
	svc := newAuctionServiceWithShippingStubs(
		&scheduleStubProductShippingRepo{err: repoErr},
		&scheduleStubCoverageRepo{},
	)

	err := svc.ensureShippingCoverage(context.Background(), nil, uuid.New())
	if err == nil {
		t.Fatal("expected error from repo failure, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected wrapped repo error, got: %v", err)
	}
}
