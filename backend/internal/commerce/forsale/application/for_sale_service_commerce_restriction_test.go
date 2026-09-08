package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/commerce/governance/commercegov"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
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
// UPDATESELLER PATH — COMMERCE RESTRICTION ENFORCEMENT (canonical)
// ============================================================================

func newUpdateSellerRepo(sellerID, forSaleID, productID uuid.UUID, status entity.ForSaleStatus) (*fakeForSaleRepository, *fakeProductRepoForUpdateSeller) {
	productIDCopy := productID
	repo := &fakeForSaleRepository{
		current: &entity.ForSale{
			ID:        forSaleID,
			ProductID: productIDCopy,
			SellerID:  sellerID,
			Status:    status,
			Product: &productEntity.Product{
				ID:       productIDCopy,
				SellerID: sellerID,
				Title:    "orig",
			},
		},
	}
	prodRepo := &fakeProductRepoForUpdateSeller{}
	return repo, prodRepo
}

// TestUpdateSeller_RestrictedSeller_Blocked proves that a commerce-restricted seller
// cannot update an existing for_sale via UpdateSeller.
func TestUpdateSeller_RestrictedSeller_Blocked(t *testing.T) {
	forSaleID := uuid.New()
	sellerID := uuid.New()
	productID := uuid.New()
	repo, prodRepo := newUpdateSellerRepo(sellerID, forSaleID, productID, entity.ForSaleStatusDraft)
	svc := &ForSaleService{
		repo:            repo,
		productRepo:     prodRepo,
		commerceGovRepo: &forSaleCommerceRestrictionRepo{restricted: true},
	}
	title := "new"
	_, err := svc.UpdateSeller(context.Background(), nil, UpdateSellerInput{
		ForSaleID: forSaleID,
		SellerID:  sellerID,
		Title:     &title,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrCommerceRestricted)
	assert.False(t, repo.updateCalled)
}

// TestUpdateSeller_UnrestrictedSeller_Allowed proves that an unrestricted seller can
// update an existing for_sale via UpdateSeller.
func TestUpdateSeller_UnrestrictedSeller_Allowed(t *testing.T) {
	forSaleID := uuid.New()
	sellerID := uuid.New()
	productID := uuid.New()
	repo, prodRepo := newUpdateSellerRepo(sellerID, forSaleID, productID, entity.ForSaleStatusDraft)
	svc := &ForSaleService{
		repo:            repo,
		productRepo:     prodRepo,
		commerceGovRepo: &forSaleCommerceRestrictionRepo{restricted: false},
	}
	title := "new"
	_, err := svc.UpdateSeller(context.Background(), nil, UpdateSellerInput{
		ForSaleID: forSaleID,
		SellerID:  sellerID,
		Title:     &title,
	})
	require.NoError(t, err)
	assert.True(t, repo.updateCalled)
}

// TestUpdateSeller_OwnershipValidation_RemainsIntact proves ownership check.
func TestUpdateSeller_OwnershipValidation_RemainsIntact(t *testing.T) {
	forSaleID := uuid.New()
	ownerID := uuid.New()
	otherID := uuid.New()
	productID := uuid.New()
	repo, prodRepo := newUpdateSellerRepo(ownerID, forSaleID, productID, entity.ForSaleStatusDraft)
	svc := &ForSaleService{
		repo:            repo,
		productRepo:     prodRepo,
		commerceGovRepo: &forSaleCommerceRestrictionRepo{restricted: false},
	}
	title := "hacked"
	_, err := svc.UpdateSeller(context.Background(), nil, UpdateSellerInput{
		ForSaleID: forSaleID,
		SellerID:  otherID,
		Title:     &title,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

// TestUpdateSeller_RestrictionUsesSameCanonicalAuthority proves same authority.
func TestUpdateSeller_RestrictionUsesSameCanonicalAuthority(t *testing.T) {
	forSaleID := uuid.New()
	sellerID := uuid.New()
	productID := uuid.New()
	repo, prodRepo := newUpdateSellerRepo(sellerID, forSaleID, productID, entity.ForSaleStatusDraft)
	svc := &ForSaleService{
		repo:            repo,
		productRepo:     prodRepo,
		commerceGovRepo: &forSaleCommerceRestrictionRepo{restricted: true},
	}
	title := "new"
	_, err := svc.UpdateSeller(context.Background(), nil, UpdateSellerInput{
		ForSaleID: forSaleID,
		SellerID:  sellerID,
		Title:     &title,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrCommerceRestricted)
}

// TestUpdateSeller_NilRepo_FailOpen proves fail-open when repo not wired.
func TestUpdateSeller_NilRepo_FailOpen(t *testing.T) {
	forSaleID := uuid.New()
	sellerID := uuid.New()
	productID := uuid.New()
	repo, prodRepo := newUpdateSellerRepo(sellerID, forSaleID, productID, entity.ForSaleStatusDraft)
	svc := &ForSaleService{
		repo:        repo,
		productRepo: prodRepo,
	}
	title := "new"
	_, err := svc.UpdateSeller(context.Background(), nil, UpdateSellerInput{
		ForSaleID: forSaleID,
		SellerID:  sellerID,
		Title:     &title,
	})
	require.NoError(t, err)
	assert.True(t, repo.updateCalled)
}
