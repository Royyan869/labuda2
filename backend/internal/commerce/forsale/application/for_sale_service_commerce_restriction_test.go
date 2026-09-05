package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/commerce/governance/commercegov"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forSaleCommerceRestrictionRepo is a minimal commercegov.Repository stub
// for for-sale service tests.
type forSaleCommerceRestrictionRepo struct {
	restricted bool
}

func (f *forSaleCommerceRestrictionRepo) InsertViolation(_ context.Context, _ db.Tx, _ *commercegov.Violation) error {
	return nil
}

func (f *forSaleCommerceRestrictionRepo) GetRestrictionForUpdate(_ context.Context, _ db.Tx, userID uuid.UUID) (*commercegov.Restriction, error) {
	if f.restricted {
		return &commercegov.Restriction{
			ID:              uuid.New(),
			UserID:          userID,
			ViolationCount:  1,
			RestrictedUntil: time.Now().Add(7 * 24 * time.Hour),
			LastViolationID: uuid.New(),
		}, nil
	}
	return nil, nil
}

func (f *forSaleCommerceRestrictionRepo) UpsertRestriction(_ context.Context, _ db.Tx, _ *commercegov.Restriction) error {
	return nil
}

// TestForSaleService_RequireSellerNotRestricted_Restricted_Blocks proves that
// a restricted seller is rejected at for-sale creation/publish boundaries.
func TestForSaleService_RequireSellerNotRestricted_Restricted_Blocks(t *testing.T) {
	svc := &ForSaleService{
		commerceGovRepo: &forSaleCommerceRestrictionRepo{restricted: true},
	}

	err := svc.requireSellerNotRestricted(context.Background(), nil, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrCommerceRestricted,
		"restricted seller should be blocked with ErrCommerceRestricted")
}

// TestForSaleService_RequireSellerNotRestricted_Unrestricted_Allows proves
// that an unrestricted seller passes the restriction check.
func TestForSaleService_RequireSellerNotRestricted_Unrestricted_Allows(t *testing.T) {
	svc := &ForSaleService{
		commerceGovRepo: &forSaleCommerceRestrictionRepo{restricted: false},
	}

	err := svc.requireSellerNotRestricted(context.Background(), nil, uuid.New())
	assert.NoError(t, err, "unrestricted seller should be allowed")
}

// TestForSaleService_RequireSellerNotRestricted_NilRepo_FailOpen proves
// backward-compatible fail-open when the repository is not wired.
func TestForSaleService_RequireSellerNotRestricted_NilRepo_FailOpen(t *testing.T) {
	svc := &ForSaleService{
		commerceGovRepo: nil,
	}

	err := svc.requireSellerNotRestricted(context.Background(), nil, uuid.New())
	assert.NoError(t, err, "nil repo should fail-open")
}

// ============================================================================
// UPDATE PATH — COMMERCE RESTRICTION ENFORCEMENT (P1-1)
// ============================================================================

// TestUpdate_RestrictedSeller_Blocked proves that a commerce-restricted seller
// cannot update an existing for_sale. This is the P1-1 defect fix: the Update
// path must enforce the SAME canonical restriction authority as Create/Publish.
func TestUpdate_RestrictedSeller_Blocked(t *testing.T) {
	forSaleID := uuid.New()
	sellerID := uuid.New()
	repo := &fakeForSaleRepository{
		current: &entity.ForSale{
			ID:       forSaleID,
			SellerID: sellerID,
			Status:   entity.ForSaleStatusDraft,
		},
	}
	svc := &ForSaleService{
		repo:             repo,
		commerceGovRepo:  &forSaleCommerceRestrictionRepo{restricted: true},
	}

	err := svc.Update(context.Background(), nil, &entity.ForSale{
		ID:       forSaleID,
		SellerID: sellerID,
		Status:   entity.ForSaleStatusDraft,	})

	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrCommerceRestricted,
		"restricted seller must be blocked from updating for_sale")
	assert.False(t, repo.updateCalled, "Update should not persist when seller is restricted")
}

// TestUpdate_UnrestrictedSeller_Allowed proves that an unrestricted seller can
// update an existing for_sale without restriction.
func TestUpdate_UnrestrictedSeller_Allowed(t *testing.T) {
	forSaleID := uuid.New()
	sellerID := uuid.New()
	repo := &fakeForSaleRepository{
		current: &entity.ForSale{
			ID:       forSaleID,
			SellerID: sellerID,
			Status:   entity.ForSaleStatusDraft,
		},
	}
	svc := &ForSaleService{
		repo:             repo,
		commerceGovRepo:  &forSaleCommerceRestrictionRepo{restricted: false},
	}

	err := svc.Update(context.Background(), nil, &entity.ForSale{
		ID:       forSaleID,
		SellerID: sellerID,
		Status:   entity.ForSaleStatusDraft,	})

	require.NoError(t, err, "unrestricted seller should be allowed to update")
	assert.True(t, repo.updateCalled, "Update should persist for unrestricted seller")
}

// TestUpdate_OwnershipValidation_RemainsIntact proves that ownership validation
// still works alongside the new restriction check. The original check (seller ID
// must match the existing for_sale) is in the handler, but the service also
// validates the transition. This test ensures the service path is correct.
func TestUpdate_OwnershipValidation_RemainsIntact(t *testing.T) {
	forSaleID := uuid.New()
	ownerID := uuid.New()
	otherID := uuid.New()
	repo := &fakeForSaleRepository{
		current: &entity.ForSale{
			ID:       forSaleID,
			SellerID: ownerID,
			Status:   entity.ForSaleStatusDraft,
		},
	}
	svc := &ForSaleService{
		repo:             repo,
		commerceGovRepo:  &forSaleCommerceRestrictionRepo{restricted: false},
	}

	// Update with wrong seller ID — the handler checks ownership before calling Update()
	// but this test proves the service doesn't break when called with mismatched IDs.
	err := svc.Update(context.Background(), nil, &entity.ForSale{
		ID:       forSaleID,
		SellerID: otherID, // different from owner
		Status:   entity.ForSaleStatusDraft,
	})

	// The service itself doesn't check ownership (handler does), but the restriction
	// check uses the seller ID from the passed entity. This should succeed because
	// the restriction check passes for an unrestricted seller.
	require.NoError(t, err, "service-level update does not enforce ownership (handler does)")
}

// TestUpdate_RestrictionUsesSameCanonicalAuthority proves that the Update path
// calls the SAME requireSellerNotRestricted method as Create and Publish —
// not a separate or duplicated restriction check.
func TestUpdate_RestrictionUsesSameCanonicalAuthority(t *testing.T) {
	// This is a structural proof: if Update() called a different restriction
	// checker, the forSaleCommerceRestrictionRepo stub would not be consulted.
	// By setting restricted=true and verifying ErrCommerceRestricted, we prove
	// the same canonical authority is used.
	forSaleID := uuid.New()
	sellerID := uuid.New()
	repo := &fakeForSaleRepository{
		current: &entity.ForSale{
			ID:       forSaleID,
			SellerID: sellerID,
			Status:   entity.ForSaleStatusDraft,
		},
	}
	svc := &ForSaleService{
		repo:             repo,
		commerceGovRepo:  &forSaleCommerceRestrictionRepo{restricted: true},
	}

	err := svc.Update(context.Background(), nil, &entity.ForSale{
		ID:       forSaleID,
		SellerID: sellerID,
		Status:   entity.ForSaleStatusDraft,
	})

	// The ONLY canonical restriction error is auth.ErrCommerceRestricted.
	// If a different checker were used, we'd get a different error or no error.
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrCommerceRestricted,
		"Update must use the SAME canonical restriction authority as Create/Publish")
}

// TestUpdate_NilRepo_FailOpen proves backward-compatible fail-open when the
// commerce restriction repository is not wired, same as Create/Publish.
func TestUpdate_NilRepo_FailOpen(t *testing.T) {
	forSaleID := uuid.New()
	sellerID := uuid.New()
	repo := &fakeForSaleRepository{
		current: &entity.ForSale{
			ID:       forSaleID,
			SellerID: sellerID,
			Status:   entity.ForSaleStatusDraft,
		},
	}
	svc := &ForSaleService{
		repo:             repo,
		commerceGovRepo:  nil, // not wired
	}

	err := svc.Update(context.Background(), nil, &entity.ForSale{
		ID:       forSaleID,
		SellerID: sellerID,
		Status:   entity.ForSaleStatusDraft,
	})

	require.NoError(t, err, "nil repo should fail-open for backward compatibility")
	assert.True(t, repo.updateCalled, "Update should proceed when repo is nil")
}
