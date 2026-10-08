package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Helpers for convergence tests
// ---------------------------------------------------------------------------

type convergenceProductRepo struct {
	products    map[uuid.UUID]*productEntity.Product
	updateCalls int
	getCalls    int
	failUpdate  error
	failGet     error
	lastUpdated *productEntity.Product
}

func newConvergenceProductRepo(sellerID, productID uuid.UUID, title, desc string) *convergenceProductRepo {
	return &convergenceProductRepo{
		products: map[uuid.UUID]*productEntity.Product{
			productID: {
				ID:          productID,
				SellerID:    sellerID,
				Title:       title,
				Description: desc,
			},
		},
	}
}

func (r *convergenceProductRepo) Create(_ context.Context, _ db.Tx, _ *productEntity.Product) error {
	return nil
}
func (r *convergenceProductRepo) ClaimSellingSurface(_ context.Context, _ db.Tx, _ uuid.UUID, _ productEntity.SellingSurface) error {
	return nil
}
func (r *convergenceProductRepo) GetByID(_ context.Context, _ db.Tx, id uuid.UUID) (*productEntity.Product, error) {
	r.getCalls++
	if r.failGet != nil {
		return nil, r.failGet
	}
	p, ok := r.products[id]
	if !ok {
		return nil, fmt.Errorf("product not found: %s", id)
	}
	cp := *p
	return &cp, nil
}
func (r *convergenceProductRepo) Update(_ context.Context, _ db.Tx, p *productEntity.Product) error {
	r.updateCalls++
	if r.failUpdate != nil {
		return r.failUpdate
	}
	cp := *p
	r.lastUpdated = &cp
	r.products[p.ID] = &cp
	return nil
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func newConvergenceAuction(status entity.Status, sellerID uuid.UUID) *entity.Auction {
	// Use a future start time so scheduled validation passes regardless of
	// when the test suite runs.
	now := time.Now().Add(48 * time.Hour)
	productID := uuid.UUID{}
	start := now.Add(2 * time.Hour)
	end := now.Add(26 * time.Hour)
	if status == entity.StatusScheduled || status == entity.StatusActive {
		start = time.Now().Add(2 * time.Hour)
		end = start.Add(24 * time.Hour)
	} else {
		start = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
		end = start.Add(24 * time.Hour)
	}
	productID = uuid.New()
	buyNow := int64(2_000_000)
	return &entity.Auction{
		ID:           uuid.New(),
		SellerID:     sellerID,
		ProductID:    productID,
		StartPrice:   1_000_000,
		BidIncrement: 100_000,
		BuyNowPrice:  &buyNow,
		StartAt:      start,
		EndAt:        end,
		Status:       status,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Product: &productEntity.Product{
			ID:          productID,
			SellerID:    sellerID,
			Title:       "Original Title",
			Description: "Original desc",
		},
	}
}

// ---------------------------------------------------------------------------
// Scheduled update authority (create = publish: scheduled is the only
// editable lifecycle state)
// ---------------------------------------------------------------------------

func TestUpdateScheduled_NonOwnerDoesNotMutateProduct(t *testing.T) {
	sellerID := uuid.New()
	auction := newConvergenceAuction(entity.StatusScheduled, sellerID)
	productRepo := newConvergenceProductRepo(sellerID, auction.ProductID, "Original Title", "Original desc")
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		productRepo: productRepo,
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	newTitle := "Hacker Title 2"
	err := svc.UpdateScheduled(context.Background(), tx, UpdateScheduledInput{
		AuctionID: auction.ID,
		CallerID:  uuid.New(),
		Title:     &newTitle,
		StartAt:   auction.StartAt,
		EndAt:     auction.EndAt,
	})
	require.ErrorIs(t, err, auth.ErrSellerRequired)
	assert.Equal(t, 0, productRepo.updateCalls)
	assert.Empty(t, tx.execSQL)
}

func TestUpdateScheduled_ContentAndTimingPersist(t *testing.T) {
	sellerID := uuid.New()
	auction := newConvergenceAuction(entity.StatusScheduled, sellerID)
	productRepo := newConvergenceProductRepo(sellerID, auction.ProductID, "Original Title", "Original desc")
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		productRepo: productRepo,
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	newTitle := "Scheduled New Title"
	newDesc := "Scheduled new desc"
	newStart := auction.StartAt.Add(time.Hour)
	newEnd := auction.EndAt.Add(time.Hour)

	err := svc.UpdateScheduled(context.Background(), tx, UpdateScheduledInput{
		AuctionID:   auction.ID,
		CallerID:    sellerID,
		Title:       &newTitle,
		Description: &newDesc,
		StartAt:     newStart,
		EndAt:       newEnd,
	})
	require.NoError(t, err)
	require.NotNil(t, productRepo.lastUpdated)
	assert.Equal(t, newTitle, productRepo.lastUpdated.Title)
	assert.Equal(t, newDesc, productRepo.lastUpdated.Description)
	found := false
	for _, sql := range tx.execSQL {
		if contains(sql, "UPDATE auctions") {
			found = true
		}
	}
	assert.True(t, found)
}

func TestUpdateScheduled_ValidationRejectsTooLongTitle(t *testing.T) {
	sellerID := uuid.New()
	auction := newConvergenceAuction(entity.StatusScheduled, sellerID)
	productRepo := newConvergenceProductRepo(sellerID, auction.ProductID, "Original Title", "Original desc")
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		productRepo: productRepo,
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	s := string(long)
	err := svc.UpdateScheduled(context.Background(), tx, UpdateScheduledInput{
		AuctionID: auction.ID,
		CallerID:  sellerID,
		Title:     &s,
		StartAt:   auction.StartAt,
		EndAt:     auction.EndAt,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "200")
	assert.Equal(t, 0, productRepo.updateCalls)
}

func TestUpdateScheduled_WithoutContent_OnlyTimingPersists(t *testing.T) {
	sellerID := uuid.New()
	auction := newConvergenceAuction(entity.StatusScheduled, sellerID)
	productRepo := newConvergenceProductRepo(sellerID, auction.ProductID, "Original Title", "Original desc")
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		productRepo: productRepo,
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	err := svc.UpdateScheduled(context.Background(), tx, UpdateScheduledInput{
		AuctionID: auction.ID,
		CallerID:  sellerID,
		StartAt:   auction.StartAt.Add(30 * time.Minute),
		EndAt:     auction.EndAt.Add(30 * time.Minute),
	})
	require.NoError(t, err)
	assert.Equal(t, 0, productRepo.updateCalls, "no product call when title/description not provided")
	assert.NotEmpty(t, tx.execSQL)
}

func TestUpdateScheduled_GuardOrder_NoProductWriteWhenNotScheduled(t *testing.T) {
	sellerID := uuid.New()
	auction := newConvergenceAuction(entity.StatusActive, sellerID)
	productRepo := newConvergenceProductRepo(sellerID, auction.ProductID, "Original Title", "Original desc")
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		productRepo: productRepo,
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}
	title := "Should Not Persist"
	err := svc.UpdateScheduled(context.Background(), tx, UpdateScheduledInput{
		AuctionID: auction.ID,
		CallerID:  sellerID,
		Title:     &title,
		StartAt:   auction.StartAt,
		EndAt:     auction.EndAt,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can only update scheduled auctions")
	assert.Equal(t, 0, productRepo.updateCalls)
	assert.Empty(t, tx.execSQL)
}
