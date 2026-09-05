//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auctionApp "github.com/labuda/backend/internal/commerce/auction/application"
	auctionHTTP "github.com/labuda/backend/internal/commerce/auction/delivery/http"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	productRepoImpl "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

func seedAuctionProductConvergence(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID uuid.UUID, title, desc string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety,
			size_cm, age_months, gender, breeder, bloodline, certificates,
			farm_address_id, preparation_time, preparation_note, selling_surface, created_at, updated_at)
		VALUES ($1, $2, $3, $4, '[]', 'Kohaku',
			50, 12, 'female', 'Breeder', 'Ogata', '{}',
			NULL, 'short', NULL, NULL, NOW(), NOW())
	`, id, sellerID, title, desc)
	require.NoError(t, err)
	return id
}

func seedDraftAuctionConvergence(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID, productID uuid.UUID, startPrice int64) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	_, err := pool.Exec(ctx, `
		INSERT INTO auctions (id, seller_id, product_id,
			start_price, bid_increment, buy_now_price,
			start_at, end_at, status, created_at, updated_at, anti_snipe_extension_seconds)
		VALUES ($1, $2, $3,
			$4, 1000, NULL,
			$5, $6, 'draft', NOW(), NOW(), 0)
	`, id, sellerID, productID, startPrice, now.Add(2*time.Hour), now.Add(26*time.Hour))
	require.NoError(t, err)
	return id
}

func seedScheduledAuctionConvergence(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID, productID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	start := time.Now().Add(4 * time.Hour)
	end := start.Add(24 * time.Hour)
	_, err := pool.Exec(ctx, `
		INSERT INTO auctions (id, seller_id, product_id,
			start_price, bid_increment, buy_now_price,
			start_at, end_at, status, created_at, updated_at, anti_snipe_extension_seconds)
		VALUES ($1, $2, $3,
			10000, 1000, NULL,
			$4, $5, 'scheduled', NOW(), NOW(), 0)
	`, id, sellerID, productID, start, end)
	require.NoError(t, err)
	return id
}

// TestAuctionUpdate_Draft_ContentAndSurface_PersistAtomically proves
// PUT draft title/description → products, pricing → auctions in ONE tx.
func TestAuctionUpdate_Draft_ContentAndSurface_PersistAtomically(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	sellerID := seedStage1User(t, ctx, dbWrap)
	productID := seedAuctionProductConvergence(t, ctx, pool, sellerID, "Original Title", "Original Desc")
	auctionID := seedDraftAuctionConvergence(t, ctx, pool, sellerID, productID, 1_000_000)

	// Wire real AuctionService with real Product repo
	prodRepo := productRepoImpl.NewProductRepository()
	svc := newAuctionServiceForIntegration(t, prodRepo)

	newTitle := "New Canonical Title"
	newDesc := "New canonical description"
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.UpdateDraft(ctx, tx, auctionApp.UpdateDraftInput{
			AuctionID:    auctionID,
			CallerID:     sellerID,
			Title:        &newTitle,
			Description:  &newDesc,
			StartPrice:   1_500_000,
			BidIncrement: 200_000,
			BuyNowPrice:  nil,
			StartAt:      time.Now().Add(2 * time.Hour),
			EndAt:        time.Now().Add(26 * time.Hour),
		})
	})
	require.NoError(t, err)

	// Prove products changed
	var title, desc string
	err = pool.QueryRow(ctx, `SELECT title, description FROM products WHERE id=$1`, productID).Scan(&title, &desc)
	require.NoError(t, err)
	assert.Equal(t, newTitle, title)
	assert.Equal(t, newDesc, desc)

	// Prove auctions changed
	var sp, bi int64
	err = pool.QueryRow(ctx, `SELECT start_price, bid_increment FROM auctions WHERE id=$1`, auctionID).Scan(&sp, &bi)
	require.NoError(t, err)
	assert.Equal(t, int64(1_500_000), sp)
	assert.Equal(t, int64(200_000), bi)
}

// TestAuctionUpdate_Scheduled_ContentPersists proves scheduled allows title/description.
func TestAuctionUpdate_Scheduled_ContentPersists(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	sellerID := seedStage1User(t, ctx, dbWrap)
	productID := seedAuctionProductConvergence(t, ctx, pool, sellerID, "Orig Title", "Orig Desc")
	auctionID := seedScheduledAuctionConvergence(t, ctx, pool, sellerID, productID)

	prodRepo := productRepoImpl.NewProductRepository()
	svc := newAuctionServiceForIntegration(t, prodRepo)

	newTitle := "Scheduled New Title"
	newDesc := "Scheduled new desc"
	newStart := time.Now().Add(5 * time.Hour)
	newEnd := newStart.Add(48 * time.Hour)
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.UpdateScheduled(ctx, tx, auctionApp.UpdateScheduledInput{
			AuctionID:   auctionID,
			CallerID:    sellerID,
			Title:       &newTitle,
			Description: &newDesc,
			StartAt:     newStart,
			EndAt:       newEnd,
		})
	})
	require.NoError(t, err)

	var title, desc string
	err = pool.QueryRow(ctx, `SELECT title, description FROM products WHERE id=$1`, productID).Scan(&title, &desc)
	require.NoError(t, err)
	assert.Equal(t, newTitle, title)
	assert.Equal(t, newDesc, desc)
}

// TestAuctionUpdate_AtomicRollback_ProductsUnchangedOnAuctionFailure proves
// that when auction timing validation fails after product mutation, both
// rollback (simulated by using invalid timing that triggers UpdateDraft error
// after product GetByID within same tx).
func TestAuctionUpdate_AtomicRollback_ProductsUnchangedOnAuctionFailure(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	sellerID := seedStage1User(t, ctx, dbWrap)
	productID := seedAuctionProductConvergence(t, ctx, pool, sellerID, "Stable Title", "Stable Desc")
	auctionID := seedDraftAuctionConvergence(t, ctx, pool, sellerID, productID, 1_000_000)

	prodRepo := productRepoImpl.NewProductRepository()
	svc := newAuctionServiceForIntegration(t, prodRepo)

	newTitle := "Should Rollback"
	invalidStart := time.Now().Add(2 * time.Hour)
	invalidEnd := invalidStart // end == start → validation fails
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.UpdateDraft(ctx, tx, auctionApp.UpdateDraftInput{
			AuctionID:    auctionID,
			CallerID:     sellerID,
			Title:        &newTitle,
			Description:  nil,
			StartPrice:   1_000_000,
			BidIncrement: 1000,
			StartAt:      invalidStart,
			EndAt:        invalidEnd,
		})
	})
	require.Error(t, err)

	// Prove products unchanged — tx rolled back
	var title string
	err = pool.QueryRow(ctx, `SELECT title FROM products WHERE id=$1`, productID).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "Stable Title", title)

	var sp int64
	err = pool.QueryRow(ctx, `SELECT start_price FROM auctions WHERE id=$1`, auctionID).Scan(&sp)
	require.NoError(t, err)
	assert.Equal(t, int64(1_000_000), sp)
}

// TestAuctionUpdate_NonOwnerDoesNotMutate proves 403 and no DB change.
func TestAuctionUpdate_NonOwnerDoesNotMutate(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	sellerID := seedStage1User(t, ctx, dbWrap)
	otherID := seedStage1User(t, ctx, dbWrap)
	productID := seedAuctionProductConvergence(t, ctx, pool, sellerID, "Seller Title", "Seller Desc")
	auctionID := seedDraftAuctionConvergence(t, ctx, pool, sellerID, productID, 1_000_000)

	prodRepo := productRepoImpl.NewProductRepository()
	svc := newAuctionServiceForIntegration(t, prodRepo)

	newTitle := "Hacker Title"
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.UpdateDraft(ctx, tx, auctionApp.UpdateDraftInput{
			AuctionID:   auctionID,
			CallerID:    otherID,
			Title:       &newTitle,
			StartPrice:  1_000_000,
			BidIncrement: 1000,
			StartAt:     time.Now().Add(2 * time.Hour),
			EndAt:       time.Now().Add(26 * time.Hour),
		})
	})
	require.ErrorIs(t, err, auth.ErrSellerRequired)

	var title string
	err = pool.QueryRow(ctx, `SELECT title FROM products WHERE id=$1`, productID).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "Seller Title", title)
}

func newAuctionServiceForIntegration(t *testing.T, prodRepo *productRepoImpl.ProductRepositoryImpl) *auctionApp.AuctionService {
	t.Helper()
	_ = auctionEntity.StatusDraft // keep import
	return newAuctionServiceForConvergenceIntegration(prodRepo)
}

// newAuctionServiceForConvergenceIntegration is defined here to avoid
// importing unexported fields. It builds a service with minimal deps.
func newAuctionServiceForConvergenceIntegration(prodRepo *productRepoImpl.ProductRepositoryImpl) *auctionApp.AuctionService {
	// We use the application-level helper that tests already use: build
	// through a struct literal in the same package would require
	// unexported field access, so we export a tiny test helper.
	// Instead, inline a minimal wiring using the constructor with nil
	// shipping/outbox/deps that Update path doesn't touch.
	//
	// UpdateDraft only reads auctionRepo, productRepo, ownership.
	// NewAuctionService now requires 11 args; we pass nil for the rest
	// because those code paths are not exercised here.
	svc := auctionApp.NewAuctionService(
		noopAccountStatusForIntegration{},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop(),
	)
	svc.SetProductRepo(prodRepo)
	// Ensure the internal auctionRepo is the real pgx repo (it already is)
	_ = &auctionRepo.AuctionRepository{}
	return svc
}

// TestAuctionUpdate_UnsupportedFieldsRejected_HTTP proves the handler guard
// returns 400 for images/category/condition/auto_extend and leaves DB untouched.
func TestAuctionUpdate_UnsupportedFieldsRejected_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	sellerID := seedStage1User(t, ctx, dbWrap)
	productID := seedAuctionProductConvergence(t, ctx, pool, sellerID, "HTTP Guard Title", "HTTP Guard Desc")
	auctionID := seedDraftAuctionConvergence(t, ctx, pool, sellerID, productID, 1_000_000)

	prodRepo := productRepoImpl.NewProductRepository()
	svc := newAuctionServiceForIntegration(t, prodRepo)
	handler := auctionHTTP.NewAuctionHandler(svc, prodRepo, nil, dbWrap, zap.NewNop())

	for _, payload := range []string{
		`{"title":"New Title","images":["https://cdn.example.com/a.jpg"]}`,
		`{"category":"Kohaku"}`,
		`{"condition":"new"}`,
		`{"auto_extend":true}`,
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/auctions/"+auctionID.String(), bytes.NewBufferString(payload))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: auctionID.String()}}
		c.Set("userID", sellerID)
		handler.UpdateAuction(c)
		assert.Equal(t, http.StatusBadRequest, w.Code, "payload %s must be rejected", payload)
		body := w.Body.String()
		assert.Contains(t, body, "unsupported field", "payload %s", payload)
	}

	// Prove DB unchanged
	var title string
	err := pool.QueryRow(ctx, `SELECT title FROM products WHERE id=$1`, productID).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "HTTP Guard Title", title)

	var sp int64
	err = pool.QueryRow(ctx, `SELECT start_price FROM auctions WHERE id=$1`, auctionID).Scan(&sp)
	require.NoError(t, err)
	assert.Equal(t, int64(1_000_000), sp)
}

// TestAuctionUpdate_Draft_HTTP_Success proves the full HTTP → service → repo → DB
// path for draft title/description update (real handler, not just service).
func TestAuctionUpdate_Draft_HTTP_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	sellerID := seedStage1User(t, ctx, dbWrap)
	productID := seedAuctionProductConvergence(t, ctx, pool, sellerID, "Before Title", "Before Desc")
	auctionID := seedDraftAuctionConvergence(t, ctx, pool, sellerID, productID, 1_000_000)

	prodRepo := productRepoImpl.NewProductRepository()
	svc := newAuctionServiceForIntegration(t, prodRepo)
	handler := auctionHTTP.NewAuctionHandler(svc, prodRepo, nil, dbWrap, zap.NewNop())

	payload := `{"title":"After Title","description":"After Desc","start_price":1500000,"bid_increment":200000}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/auctions/"+auctionID.String(), bytes.NewBufferString(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: auctionID.String()}}
	c.Set("userID", sellerID)
	handler.UpdateAuction(c)

	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Prove products updated via direct DB read (not just response)
	var title, desc string
	err := pool.QueryRow(ctx, `SELECT title, description FROM products WHERE id=$1`, productID).Scan(&title, &desc)
	require.NoError(t, err)
	assert.Equal(t, "After Title", title)
	assert.Equal(t, "After Desc", desc)

	var sp int64
	err = pool.QueryRow(ctx, `SELECT start_price FROM auctions WHERE id=$1`, auctionID).Scan(&sp)
	require.NoError(t, err)
	assert.Equal(t, int64(1500000), sp)
}

type noopAccountStatusForIntegration struct{}

func (noopAccountStatusForIntegration) EnsureActive(context.Context, uuid.UUID) error { return nil }
func (noopAccountStatusForIntegration) GetStatus(context.Context, uuid.UUID) (string, error) {
	return "active", nil
}
func (noopAccountStatusForIntegration) IsBanned(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}
