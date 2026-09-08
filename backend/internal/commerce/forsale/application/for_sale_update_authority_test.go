package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	forsaleRepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeForSaleRepository is a minimal stub for UpdateSeller authority tests.
type fakeForSaleRepository struct {
	current      *entity.ForSale
	updateCalled bool
}

func (r *fakeForSaleRepository) Create(_ context.Context, _ db.Tx, _ *entity.ForSale) error { return nil }
func (r *fakeForSaleRepository) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.ForSale, error) {
	return r.current, nil
}
func (r *fakeForSaleRepository) GetByProductID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.ForSale, error) {
	return nil, nil
}
func (r *fakeForSaleRepository) GetForUpdate(_ context.Context, _ db.Tx, id uuid.UUID) (*entity.ForSale, error) {
	return r.current, nil
}
func (r *fakeForSaleRepository) Update(_ context.Context, _ db.Tx, _ *entity.ForSale) error {
	r.updateCalled = true
	return nil
}
func (r *fakeForSaleRepository) UpdateStock(_ context.Context, _ db.Tx, _ *entity.ForSale) error { return nil }
func (r *fakeForSaleRepository) UpdateStatus(_ context.Context, _ db.Tx, _ *entity.ForSale) error { return nil }
func (r *fakeForSaleRepository) GetBySellerIDPaginated(_ context.Context, _ db.Tx, _ uuid.UUID, _, _ int, _ bool) ([]*entity.ForSale, error) {
	return nil, nil
}
func (r *fakeForSaleRepository) GetPublicBySellerID(_ context.Context, _ db.Tx, _ uuid.UUID, _, _ int) ([]*entity.ForSale, error) {
	return nil, nil
}
func (r *fakeForSaleRepository) GetPublic(_ context.Context, _ db.Tx, _, _ int) ([]*entity.ForSale, error) {
	return nil, nil
}
func (r *fakeForSaleRepository) Search(_ context.Context, _ db.Tx, _ forsaleRepo.SearchFilters) ([]*entity.ForSale, *time.Time, error) {
	return nil, nil, nil
}

type fakeProductRepoForUpdateSeller struct {
	updateCalled bool
}

func (f *fakeProductRepoForUpdateSeller) Create(_ context.Context, _ db.Tx, _ *productEntity.Product) error { return nil }
func (f *fakeProductRepoForUpdateSeller) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*productEntity.Product, error) {
	return nil, nil
}
func (f *fakeProductRepoForUpdateSeller) Update(_ context.Context, _ db.Tx, _ *productEntity.Product) error {
	f.updateCalled = true
	return nil
}
func (f *fakeProductRepoForUpdateSeller) ClaimSellingSurface(_ context.Context, _ db.Tx, _ uuid.UUID, _ productEntity.SellingSurface) error {
	return nil
}

func newDraftForSaleForUpdateSeller(sellerID, forSaleID, productID uuid.UUID, status entity.ForSaleStatus) *entity.ForSale {
	return &entity.ForSale{
		ID:        forSaleID,
		ProductID: productID,
		SellerID:  sellerID,
		Status:    status,
		Product: &productEntity.Product{
			ID:       productID,
			SellerID: sellerID,
			Title:    "orig",
		},
	}
}

func TestUpdateSeller_AllowsDraft(t *testing.T) {
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	repo := &fakeForSaleRepository{current: newDraftForSaleForUpdateSeller(sellerID, forSaleID, productID, entity.ForSaleStatusDraft)}
	prodRepo := &fakeProductRepoForUpdateSeller{}
	svc := &ForSaleService{repo: repo, productRepo: prodRepo}
	title := "new title"
	_, err := svc.UpdateSeller(context.Background(), nil, UpdateSellerInput{
		ForSaleID: forSaleID,
		SellerID:  sellerID,
		Title:     &title,
	})
	require.NoError(t, err)
	assert.True(t, repo.updateCalled, "for_sale Update should be called")
	assert.True(t, prodRepo.updateCalled, "product Update should be called")
}

func TestUpdateSeller_RejectsActive(t *testing.T) {
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	repo := &fakeForSaleRepository{current: newDraftForSaleForUpdateSeller(sellerID, forSaleID, productID, entity.ForSaleStatusActive)}
	prodRepo := &fakeProductRepoForUpdateSeller{}
	svc := &ForSaleService{repo: repo, productRepo: prodRepo}
	title := "hacked"
	_, err := svc.UpdateSeller(context.Background(), nil, UpdateSellerInput{
		ForSaleID: forSaleID,
		SellerID:  sellerID,
		Title:     &title,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LIVE_IMMUTABLE")
	assert.False(t, repo.updateCalled)
	assert.False(t, prodRepo.updateCalled)
}

func TestUpdateSeller_RejectsSold(t *testing.T) {
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	repo := &fakeForSaleRepository{current: newDraftForSaleForUpdateSeller(sellerID, forSaleID, productID, entity.ForSaleStatusSold)}
	prodRepo := &fakeProductRepoForUpdateSeller{}
	svc := &ForSaleService{repo: repo, productRepo: prodRepo}
	title := "hacked"
	_, err := svc.UpdateSeller(context.Background(), nil, UpdateSellerInput{
		ForSaleID: forSaleID,
		SellerID:  sellerID,
		Title:     &title,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LIVE_IMMUTABLE")
	assert.False(t, repo.updateCalled)
}

func TestUpdateSeller_RejectsWithdrawn(t *testing.T) {
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	repo := &fakeForSaleRepository{current: newDraftForSaleForUpdateSeller(sellerID, forSaleID, productID, entity.ForSaleStatusWithdrawn)}
	prodRepo := &fakeProductRepoForUpdateSeller{}
	svc := &ForSaleService{repo: repo, productRepo: prodRepo}
	title := "hacked"
	_, err := svc.UpdateSeller(context.Background(), nil, UpdateSellerInput{
		ForSaleID: forSaleID,
		SellerID:  sellerID,
		Title:     &title,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LIVE_IMMUTABLE")
	assert.False(t, repo.updateCalled)
}

func TestUpdateSeller_CommerceRestriction_Blocked(t *testing.T) {
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	repo := &fakeForSaleRepository{current: newDraftForSaleForUpdateSeller(sellerID, forSaleID, productID, entity.ForSaleStatusDraft)}
	prodRepo := &fakeProductRepoForUpdateSeller{}
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
