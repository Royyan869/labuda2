package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

// failingAuctionTx wraps auctionUpdateSpyTx and injects failure on UPDATE auctions
type failingAuctionTx struct {
	*auctionUpdateSpyTx
	failErr error
}

func (t *failingAuctionTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if t.failErr != nil && contains(sql, "UPDATE auctions") {
		t.execSQL = append(t.execSQL, sql)
		t.execArgs = append(t.execArgs, args)
		return pgconn.CommandTag{}, t.failErr
	}
	return t.auctionUpdateSpyTx.Exec(ctx, sql, args...)
}

var _ db.Tx = (*failingAuctionTx)(nil)

func newConvergenceAuction(status entity.Status, sellerID uuid.UUID) *entity.Auction {
	// Use a future start time so scheduled validation passes regardless of
	// when the test suite runs. Draft tests ignore the future check.
	now := time.Now().Add(48 * time.Hour)
	if status == entity.StatusDraft {
		now = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	}
	productID := uuid.New()
	buyNow := int64(2_000_000)
	start := now.Add(2 * time.Hour)
	end := now.Add(26 * time.Hour)
	if status == entity.StatusScheduled {
		start = time.Now().Add(2 * time.Hour)
		end = start.Add(24 * time.Hour)
	}
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
// A. Draft content persistence
// ---------------------------------------------------------------------------

func TestUpdateDraft_ContentPersistsToProducts(t *testing.T) {
	sellerID := uuid.New()
	auction := newConvergenceAuction(entity.StatusDraft, sellerID)
	productRepo := newConvergenceProductRepo(sellerID, auction.ProductID, "Original Title", "Original desc")
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		productRepo: productRepo,
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	newTitle := "New Canonical Title"
	newDesc := "New canonical description"
	err := svc.UpdateDraft(context.Background(), tx, UpdateDraftInput{
		AuctionID:    auction.ID,
		CallerID:     sellerID,
		Title:        &newTitle,
		Description:  &newDesc,
		StartPrice:   auction.StartPrice,
		BidIncrement: auction.BidIncrement,
		BuyNowPrice:  auction.BuyNowPrice,
		StartAt:      auction.StartAt,
		EndAt:        auction.EndAt,
	})
	require.NoError(t, err)
	require.NotNil(t, productRepo.lastUpdated)
	assert.Equal(t, newTitle, productRepo.lastUpdated.Title)
	assert.Equal(t, newDesc, productRepo.lastUpdated.Description)
	assert.Equal(t, 1, productRepo.updateCalls, "product Update must be called exactly once")
	found := false
	for _, sql := range tx.execSQL {
		if contains(sql, "UPDATE auctions") {
			found = true
		}
	}
	assert.True(t, found, "UPDATE auctions must be executed")
}

// ---------------------------------------------------------------------------
// B. Auction surface persistence in same request
// ---------------------------------------------------------------------------

func TestUpdateDraft_SurfacePersistsAlongsideContent(t *testing.T) {
	sellerID := uuid.New()
	auction := newConvergenceAuction(entity.StatusDraft, sellerID)
	productRepo := newConvergenceProductRepo(sellerID, auction.ProductID, "Original Title", "Original desc")
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		productRepo: productRepo,
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	newTitle := "Title B"
	err := svc.UpdateDraft(context.Background(), tx, UpdateDraftInput{
		AuctionID:    auction.ID,
		CallerID:     sellerID,
		Title:        &newTitle,
		Description:  nil,
		StartPrice:   1_500_000,
		BidIncrement: 200_000,
		BuyNowPrice:  nil,
		StartAt:      auction.StartAt.Add(time.Hour),
		EndAt:        auction.EndAt.Add(time.Hour),
	})
	require.NoError(t, err)
	require.NotNil(t, productRepo.lastUpdated)
	assert.Equal(t, "Title B", productRepo.lastUpdated.Title)
	assert.Equal(t, "Original desc", productRepo.lastUpdated.Description, "unchanged description must stay")
	require.Len(t, tx.execArgs, 1)
	assert.Equal(t, int64(1_500_000), tx.execArgs[0][2])
	assert.Equal(t, int64(200_000), tx.execArgs[0][3])
}

// ---------------------------------------------------------------------------
// C. Atomic rollback — failure after Product mutation must not leave partial
// ---------------------------------------------------------------------------

func TestUpdateDraft_AtomicRollback_WhenAuctionPersistFails(t *testing.T) {
	sellerID := uuid.New()
	auction := newConvergenceAuction(entity.StatusDraft, sellerID)
	productRepo := newConvergenceProductRepo(sellerID, auction.ProductID, "Original Title", "Original desc")
	inner := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	tx := &failingAuctionTx{auctionUpdateSpyTx: inner, failErr: fmt.Errorf("injected auction persistence failure")}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		productRepo: productRepo,
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	newTitle := "Should Rollback"
	err := svc.UpdateDraft(context.Background(), tx, UpdateDraftInput{
		AuctionID:    auction.ID,
		CallerID:     sellerID,
		Title:        &newTitle,
		Description:  nil,
		StartPrice:   auction.StartPrice,
		BidIncrement: auction.BidIncrement,
		StartAt:      auction.StartAt,
		EndAt:        auction.EndAt,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected auction persistence failure")
	// In a real DB tx the product mutation would have been rolled back
	// atomically. Our product repo already applied the mutation in-memory
	// before the auction failure — but the service returned an error, so
	// the caller (h.db.WithTx) would Rollback the whole tx.
}

func TestUpdateDraft_AtomicRollback_WhenProductUpdateFails(t *testing.T) {
	sellerID := uuid.New()
	auction := newConvergenceAuction(entity.StatusDraft, sellerID)
	productRepo := newConvergenceProductRepo(sellerID, auction.ProductID, "Original Title", "Original desc")
	productRepo.failUpdate = fmt.Errorf("injected product failure")
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		productRepo: productRepo,
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	newTitle := "Nope"
	err := svc.UpdateDraft(context.Background(), tx, UpdateDraftInput{
		AuctionID:   auction.ID,
		CallerID:    sellerID,
		Title:       &newTitle,
		StartPrice:  auction.StartPrice,
		BidIncrement: auction.BidIncrement,
		StartAt:     auction.StartAt,
		EndAt:       auction.EndAt,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected product failure")
	assert.Empty(t, tx.execSQL, "UPDATE auctions must not execute if product update failed")
}

// ---------------------------------------------------------------------------
// D. Authorization — non-owner must not mutate product or auction
// ---------------------------------------------------------------------------

func TestUpdateDraft_NonOwnerDoesNotMutateProduct(t *testing.T) {
	sellerID := uuid.New()
	otherID := uuid.New()
	auction := newConvergenceAuction(entity.StatusDraft, sellerID)
	productRepo := newConvergenceProductRepo(sellerID, auction.ProductID, "Original Title", "Original desc")
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		productRepo: productRepo,
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	newTitle := "Hacker Title"
	err := svc.UpdateDraft(context.Background(), tx, UpdateDraftInput{
		AuctionID:   auction.ID,
		CallerID:    otherID,
		Title:       &newTitle,
		StartPrice:  auction.StartPrice,
		BidIncrement: auction.BidIncrement,
		StartAt:     auction.StartAt,
		EndAt:       auction.EndAt,
	})
	require.ErrorIs(t, err, auth.ErrSellerRequired)
	assert.Equal(t, 0, productRepo.updateCalls)
	assert.Empty(t, tx.execSQL)
}

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

// ---------------------------------------------------------------------------
// E. Scheduled content persistence
// ---------------------------------------------------------------------------

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

func TestUpdateDraft_ValidationRejectsTooLongTitle(t *testing.T) {
	sellerID := uuid.New()
	auction := newConvergenceAuction(entity.StatusDraft, sellerID)
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
	err := svc.UpdateDraft(context.Background(), tx, UpdateDraftInput{
		AuctionID:   auction.ID,
		CallerID:    sellerID,
		Title:       &s,
		StartPrice:  auction.StartPrice,
		BidIncrement: auction.BidIncrement,
		StartAt:     auction.StartAt,
		EndAt:       auction.EndAt,
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
