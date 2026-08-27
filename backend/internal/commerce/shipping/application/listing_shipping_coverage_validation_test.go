package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// Stubs — option repo with configurable per-ID return
// ============================================================================

type stubOptionRepoByID struct {
	options map[uuid.UUID]*entity.ShippingOption
	err     error
}

func (r *stubOptionRepoByID) GetByID(_ context.Context, _ db.Tx, id uuid.UUID) (*entity.ShippingOption, error) {
	if r.err != nil {
		return nil, r.err
	}
	opt, ok := r.options[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return opt, nil
}

func (r *stubOptionRepoByID) Create(_ context.Context, _ db.Tx, _ *entity.ShippingOption) error { return nil }
func (r *stubOptionRepoByID) Update(_ context.Context, _ db.Tx, _ *entity.ShippingOption) error { return nil }
func (r *stubOptionRepoByID) GetForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.ShippingOption, error) { return nil, nil }
func (r *stubOptionRepoByID) GetBySeller(_ context.Context, _ db.Tx, _ uuid.UUID, _ bool) ([]*entity.ShippingOption, error) { return nil, nil }
func (r *stubOptionRepoByID) GetByName(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) (*entity.ShippingOption, error) { return nil, nil }
func (r *stubOptionRepoByID) Delete(_ context.Context, _ db.Tx, _ uuid.UUID) error { return nil }

// ============================================================================
// Stubs — coverage repo with configurable per-option return
// ============================================================================

type stubCoverageRepoByOption struct {
	coveragesByOption map[uuid.UUID][]*entity.ShippingCoverage
	err               error
}

func (r *stubCoverageRepoByOption) GetByShippingOption(_ context.Context, _ db.Tx, optionID uuid.UUID) ([]*entity.ShippingCoverage, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.coveragesByOption[optionID], nil
}

func (r *stubCoverageRepoByOption) Create(_ context.Context, _ db.Tx, _ *entity.ShippingCoverage) error { return nil }
func (r *stubCoverageRepoByOption) Update(_ context.Context, _ db.Tx, _ *entity.ShippingCoverage) error { return nil }
func (r *stubCoverageRepoByOption) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.ShippingCoverage, error) { return nil, nil }
func (r *stubCoverageRepoByOption) GetByOptionAndProvince(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) (*entity.ShippingCoverage, error) { return nil, nil }
func (r *stubCoverageRepoByOption) Delete(_ context.Context, _ db.Tx, _ uuid.UUID) error { return nil }
func (r *stubCoverageRepoByOption) DeleteByShippingOption(_ context.Context, _ db.Tx, _ uuid.UUID) error { return nil }

// ============================================================================
// Tests
// ============================================================================

func TestValidateSellableCreateShippingSelection_EmptyIDs_Passes(t *testing.T) {
	t.Parallel()

	_, err := ValidateSellableCreateShippingSelection(
		context.Background(), nil,
		&stubOptionRepoByID{options: map[uuid.UUID]*entity.ShippingOption{}},
		&stubCoverageRepoByOption{},
		uuid.New(),
		nil, // empty list — no options to validate
	)
	if err != nil {
		t.Fatalf("expected nil error for empty option list, got: %v", err)
	}
}

func TestValidateSellableCreateShippingSelection_OptionNotFound_Errors(t *testing.T) {
	t.Parallel()

	sellerID := uuid.New()
	optionID := uuid.New()

	_, err := ValidateSellableCreateShippingSelection(
		context.Background(), nil,
		&stubOptionRepoByID{options: map[uuid.UUID]*entity.ShippingOption{}}, // not found
		&stubCoverageRepoByOption{},
		sellerID,
		[]uuid.UUID{optionID},
	)
	if err == nil {
		t.Fatal("expected error for missing option, got nil")
	}
	if !errors.Is(err, ErrInvalidSellableCreateShippingSelection) {
		t.Fatalf("expected ErrInvalidSellableCreateShippingSelection, got: %v", err)
	}
}

func TestValidateSellableCreateShippingSelection_WrongSeller_Errors(t *testing.T) {
	t.Parallel()

	sellerID := uuid.New()
	otherSellerID := uuid.New()
	optionID := uuid.New()

	_, err := ValidateSellableCreateShippingSelection(
		context.Background(), nil,
		&stubOptionRepoByID{options: map[uuid.UUID]*entity.ShippingOption{
			optionID: {ID: optionID, SellerID: otherSellerID}, // belongs to another seller
		}},
		&stubCoverageRepoByOption{},
		sellerID,
		[]uuid.UUID{optionID},
	)
	if err == nil {
		t.Fatal("expected error for wrong seller, got nil")
	}
	if !errors.Is(err, ErrInvalidSellableCreateShippingSelection) {
		t.Fatalf("expected ErrInvalidSellableCreateShippingSelection, got: %v", err)
	}
}

func TestValidateSellableCreateShippingSelection_ZeroCoverages_Errors(t *testing.T) {
	t.Parallel()

	sellerID := uuid.New()
	optionID := uuid.New()

	_, err := ValidateSellableCreateShippingSelection(
		context.Background(), nil,
		&stubOptionRepoByID{options: map[uuid.UUID]*entity.ShippingOption{
			optionID: {ID: optionID, SellerID: sellerID},
		}},
		&stubCoverageRepoByOption{
			coveragesByOption: map[uuid.UUID][]*entity.ShippingCoverage{
				optionID: {}, // zero coverages
			},
		},
		sellerID,
		[]uuid.UUID{optionID},
	)
	if err == nil {
		t.Fatal("expected error for option with zero coverages, got nil")
	}
	if !errors.Is(err, ErrInvalidSellableCreateShippingSelection) {
		t.Fatalf("expected ErrInvalidSellableCreateShippingSelection, got: %v", err)
	}
}

func TestValidateSellableCreateShippingSelection_AllCoveragesInactive_Errors(t *testing.T) {
	t.Parallel()

	sellerID := uuid.New()
	optionID := uuid.New()

	_, err := ValidateSellableCreateShippingSelection(
		context.Background(), nil,
		&stubOptionRepoByID{options: map[uuid.UUID]*entity.ShippingOption{
			optionID: {ID: optionID, SellerID: sellerID},
		}},
		&stubCoverageRepoByOption{
			coveragesByOption: map[uuid.UUID][]*entity.ShippingCoverage{
				optionID: {
					{ID: uuid.New(), ShippingOptionID: optionID, IsAvailable: false},
					{ID: uuid.New(), ShippingOptionID: optionID, IsAvailable: false},
				},
			},
		},
		sellerID,
		[]uuid.UUID{optionID},
	)
	if err == nil {
		t.Fatal("expected error for option with only inactive coverages, got nil")
	}
	if !errors.Is(err, ErrInvalidSellableCreateShippingSelection) {
		t.Fatalf("expected ErrInvalidSellableCreateShippingSelection, got: %v", err)
	}
}

func TestValidateSellableCreateShippingSelection_HasActiveCoverage_Passes(t *testing.T) {
	t.Parallel()

	sellerID := uuid.New()
	optionID := uuid.New()

	ids, err := ValidateSellableCreateShippingSelection(
		context.Background(), nil,
		&stubOptionRepoByID{options: map[uuid.UUID]*entity.ShippingOption{
			optionID: {ID: optionID, SellerID: sellerID},
		}},
		&stubCoverageRepoByOption{
			coveragesByOption: map[uuid.UUID][]*entity.ShippingCoverage{
				optionID: {
					{ID: uuid.New(), ShippingOptionID: optionID, IsAvailable: false},
					{ID: uuid.New(), ShippingOptionID: optionID, IsAvailable: true}, // one active
				},
			},
		},
		sellerID,
		[]uuid.UUID{optionID},
	)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(ids) != 1 || ids[0] != optionID {
		t.Fatalf("expected validated ids %v, got %v", []uuid.UUID{optionID}, ids)
	}
}

func TestValidateSellableCreateShippingSelection_MultipleOptions_AllNeedActiveCoverage(t *testing.T) {
	t.Parallel()

	sellerID := uuid.New()
	opt1 := uuid.New()
	opt2 := uuid.New()

	// opt1 has active coverage, opt2 does not
	_, err := ValidateSellableCreateShippingSelection(
		context.Background(), nil,
		&stubOptionRepoByID{options: map[uuid.UUID]*entity.ShippingOption{
			opt1: {ID: opt1, SellerID: sellerID},
			opt2: {ID: opt2, SellerID: sellerID},
		}},
		&stubCoverageRepoByOption{
			coveragesByOption: map[uuid.UUID][]*entity.ShippingCoverage{
				opt1: {{ID: uuid.New(), ShippingOptionID: opt1, IsAvailable: true}},
				opt2: {}, // zero coverages — should fail
			},
		},
		sellerID,
		[]uuid.UUID{opt1, opt2},
	)
	if err == nil {
		t.Fatal("expected error when one option has no active coverage, got nil")
	}
	if !errors.Is(err, ErrInvalidSellableCreateShippingSelection) {
		t.Fatalf("expected ErrInvalidSellableCreateShippingSelection, got: %v", err)
	}
}
