//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	sellerhttp "github.com/hishumi/backend/internal/commerce/seller/delivery/http"
	"github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/testdb"
)

func seedAnalyticsUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', 'user', NOW(), NOW())
	`, id, "fb-"+id.String(), id.String()+"@test.invalid")
	require.NoError(t, err)
	return id
}

func seedAnalyticsProduct(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID uuid.UUID, surface string, title string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at)
		VALUES ($1, $2, $3, 'desc', '[]', 'kohaku', '1_3_days', $4, NOW(), NOW())
	`, id, sellerID, title, surface)
	require.NoError(t, err)
	return id
}

func seedAnalyticsForSale(t *testing.T, ctx context.Context, tdb *testdb.TestDB, productID, sellerID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, published_at, quantity_available, created_at, updated_at)
		VALUES ($1, $2, $3, 100000, false, $4, NOW(), 1, NOW(), NOW())
	`, id, productID, sellerID, status)
	require.NoError(t, err)
	return id
}

func seedAnalyticsAuction(t *testing.T, ctx context.Context, tdb *testdb.TestDB, productID, sellerID uuid.UUID, status string, winnerID *uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, buy_now_price, start_at, end_at, current_bid, current_winner_id, status, created_at, updated_at, anti_snipe_extension_seconds)
		VALUES ($1, $2, $3, 10000, 1000, NULL, NOW(), NOW() + interval '1 day', NULL, $5, $4, NOW(), NOW(), 0)
	`, id, sellerID, productID, status, winnerID)
	require.NoError(t, err)
	return id
}

func seedAnalyticsView(t *testing.T, ctx context.Context, tdb *testdb.TestDB, productID uuid.UUID, viewedAt time.Time) {
	t.Helper()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO product_view_events (id, product_id, viewer_user_id, viewed_at)
		VALUES ($1, $2, NULL, $3)
	`, uuid.New(), productID, viewedAt)
	require.NoError(t, err)
}

func seedAnalyticsBid(t *testing.T, ctx context.Context, tdb *testdb.TestDB, auctionID uuid.UUID, createdAt time.Time) {
	t.Helper()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO auction_bids (id, auction_id, bidder_id, amount, idempotency_key, created_at)
		VALUES ($1, $2, $3, 11000, $4, $5)
	`, uuid.New(), auctionID, seedAnalyticsUser(t, ctx, tdb), uuid.NewString(), createdAt)
	require.NoError(t, err)
}

func seedAnalyticsOrder(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID, productID uuid.UUID, status string, completedAt *time.Time) {
	seedAnalyticsOrderQty(t, ctx, tdb, sellerID, productID, status, completedAt, 1)
}

func seedAnalyticsOrderQty(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID, productID uuid.UUID, status string, completedAt *time.Time, quantity int) {
	t.Helper()
	orderID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO orders (id, buyer_id, seller_id, source_type, source_id, quantity, unit_price, subtotal, shipping_total, commission_percent, commission_amount, status, completed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'for_sale', $4, $7, 100000, $7 * 100000, 0, 0, 0, $5, $6, NOW(), NOW())
	`, orderID, seedAnalyticsUser(t, ctx, tdb), sellerID, productID, status, completedAt, quantity)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, unit_price_snapshot, quantity, name, created_at)
		VALUES ($1, $2, $3, 100000, $4, 'Koi', NOW())
	`, uuid.New(), orderID, productID, quantity)
	require.NoError(t, err)
}

func callSellerAnalytics(t *testing.T, h *sellerhttp.SellerHandler, userID uuid.UUID) sellerhttp.SellerAnalyticsResponse {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/seller/analytics", nil)
	c.Set("userID", userID)
	h.GetAnalytics(c)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data sellerhttp.SellerAnalyticsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Data
}

func newSellerAnalyticsHandler(tdb *testdb.TestDB) *sellerhttp.SellerHandler {
	return sellerhttp.NewSellerHandler(nil, db.NewFromPool(tdb.Pool()), zap.NewNop(), nil, nil, nil, nil, nil, nil)
}

func TestSellerAnalytics_ProductLevel_And_Summary(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	seller := seedAnalyticsUser(t, ctx, tdb)
	otherSeller := seedAnalyticsUser(t, ctx, tdb)
	h := newSellerAnalyticsHandler(tdb)

	now := time.Now().UTC()
	inWindow := now.Add(-1 * time.Hour)
	outWindow := now.Add(-40 * 24 * time.Hour)

	// no view → 0
	pNoView := seedAnalyticsProduct(t, ctx, tdb, seller, "for_sale", "No View")
	seedAnalyticsForSale(t, ctx, tdb, pNoView, seller, "active")

	// two in-window views → 2
	pViewed := seedAnalyticsProduct(t, ctx, tdb, seller, "for_sale", "Viewed")
	seedAnalyticsForSale(t, ctx, tdb, pViewed, seller, "active")
	seedAnalyticsView(t, ctx, tdb, pViewed, inWindow)
	seedAnalyticsView(t, ctx, tdb, pViewed, inWindow)

	// view outside window → 0
	pOldView := seedAnalyticsProduct(t, ctx, tdb, seller, "for_sale", "Old View")
	seedAnalyticsForSale(t, ctx, tdb, pOldView, seller, "active")
	seedAnalyticsView(t, ctx, tdb, pOldView, outWindow)

	// terminal sold product still appears
	pSold := seedAnalyticsProduct(t, ctx, tdb, seller, "for_sale", "Sold ForSale")
	seedAnalyticsForSale(t, ctx, tdb, pSold, seller, "sold")

	// auction: 1 in-window bid + 1 out-window bid → bid_count_30d = 1
	pAuction := seedAnalyticsProduct(t, ctx, tdb, seller, "auction", "Auction Item")
	auction := seedAnalyticsAuction(t, ctx, tdb, pAuction, seller, "active", nil)
	seedAnalyticsBid(t, ctx, tdb, auction, inWindow)
	seedAnalyticsBid(t, ctx, tdb, auction, outWindow)

	// completed sale in window → counts
	pSold30 := seedAnalyticsProduct(t, ctx, tdb, seller, "for_sale", "Sold 30d")
	seedAnalyticsForSale(t, ctx, tdb, pSold30, seller, "sold")
	seedAnalyticsOrder(t, ctx, tdb, seller, pSold30, "completed", &inWindow)

	// completed sale outside window → not counted
	pSoldOld := seedAnalyticsProduct(t, ctx, tdb, seller, "for_sale", "Sold 60d")
	seedAnalyticsForSale(t, ctx, tdb, pSoldOld, seller, "sold")
	seedAnalyticsOrder(t, ctx, tdb, seller, pSoldOld, "completed", &outWindow)

	// cancelled order → never counts
	pCancelled := seedAnalyticsProduct(t, ctx, tdb, seller, "for_sale", "Cancelled")
	seedAnalyticsForSale(t, ctx, tdb, pCancelled, seller, "withdrawn")
	seedAnalyticsOrder(t, ctx, tdb, seller, pCancelled, "cancelled", nil)

	// other seller's product must never leak
	seedAnalyticsProduct(t, ctx, tdb, otherSeller, "for_sale", "Other Seller Product")

	resp := callSellerAnalytics(t, h, seller)

	byTitle := map[string]sellerhttp.SellerAnalyticsProduct{}
	for _, p := range resp.Products {
		byTitle[p.Title] = p
	}

	require.Equal(t, int64(0), byTitle["No View"].Views30d)
	require.Equal(t, int64(2), byTitle["Viewed"].Views30d)
	require.Equal(t, int64(0), byTitle["Old View"].Views30d)
	require.True(t, byTitle["Sold ForSale"].Sold)
	require.Equal(t, "sold", byTitle["Sold ForSale"].State)
	require.Equal(t, int64(1), byTitle["Auction Item"].BidCount30d)
	require.Equal(t, "auction", byTitle["Auction Item"].SurfaceType)
	require.Equal(t, int64(0), byTitle["No View"].BidCount30d)

	_, leaked := byTitle["Other Seller Product"]
	require.False(t, leaked, "other seller's product must not appear")

	require.Equal(t, int64(2), resp.Summary.TotalViews30d)
	require.Equal(t, int64(1), resp.Summary.ProductsWithViews30d)
	require.Equal(t, int64(1), resp.Summary.ProductsSold30d)
}

func TestSellerAnalytics_ZeroData(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	seller := seedAnalyticsUser(t, ctx, tdb)
	h := newSellerAnalyticsHandler(tdb)

	resp := callSellerAnalytics(t, h, seller)

	require.Equal(t, int64(0), resp.Summary.TotalViews30d)
	require.Equal(t, int64(0), resp.Summary.ProductsWithViews30d)
	require.Equal(t, int64(0), resp.Summary.ProductsSold30d)
	require.Empty(t, resp.Products)
}

func TestSellerAnalytics_AuctionSoldOutcome(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	seller := seedAnalyticsUser(t, ctx, tdb)
	h := newSellerAnalyticsHandler(tdb)

	winner := seedAnalyticsUser(t, ctx, tdb)
	pEndedWin := seedAnalyticsProduct(t, ctx, tdb, seller, "auction", "Ended Win")
	seedAnalyticsAuction(t, ctx, tdb, pEndedWin, seller, "ended", &winner)
	pEndedNoWin := seedAnalyticsProduct(t, ctx, tdb, seller, "auction", "Ended NoWin")
	seedAnalyticsAuction(t, ctx, tdb, pEndedNoWin, seller, "ended", nil)

	resp := callSellerAnalytics(t, h, seller)
	byTitle := map[string]sellerhttp.SellerAnalyticsProduct{}
	for _, p := range resp.Products {
		byTitle[p.Title] = p
	}

	require.True(t, byTitle["Ended Win"].Sold)
	require.False(t, byTitle["Ended NoWin"].Sold)
}

// TestSellerAnalytics_ProductsSold30d_DistinctSemantics proves the summary
// products_sold_30d counts DISTINCT products with a completed sale in the
// window — not order rows, not quantity/units.
func TestSellerAnalytics_ProductsSold30d_DistinctSemantics(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	seller := seedAnalyticsUser(t, ctx, tdb)
	h := newSellerAnalyticsHandler(tdb)

	inWindow := time.Now().UTC().Add(-1 * time.Hour)

	// Case A: one product with MULTIPLE completed orders → still 1 distinct product.
	pMulti := seedAnalyticsProduct(t, ctx, tdb, seller, "for_sale", "Multi Orders")
	seedAnalyticsForSale(t, ctx, tdb, pMulti, seller, "sold")
	seedAnalyticsOrder(t, ctx, tdb, seller, pMulti, "completed", &inWindow)
	seedAnalyticsOrder(t, ctx, tdb, seller, pMulti, "completed", &inWindow)

	// Case B: one product with quantity > 1 (one completed order) → still 1.
	pQty := seedAnalyticsProduct(t, ctx, tdb, seller, "for_sale", "Multi Qty")
	seedAnalyticsForSale(t, ctx, tdb, pQty, seller, "sold")
	seedAnalyticsOrderQty(t, ctx, tdb, seller, pQty, "completed", &inWindow, 3)

	resp := callSellerAnalytics(t, h, seller)

	// Distinct products sold = 2 (Multi Orders + Multi Qty), NOT 3 orders and
	// NOT 4 units.
	require.Equal(t, int64(2), resp.Summary.ProductsSold30d)
}
