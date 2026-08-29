//go:build integration

package tests

// Stage 6B — Product public availability convergence.
//
// Real Postgres runtime proof that:
//   1. a sold-out (qty=1) FPS disappears from public catalog/search/seller-
//      page discovery but stays visible in owner inventory;
//   2. a multi-quantity FPS stays discoverable while quantity_available > 0
//      and disappears at 0;
//   3. the public seller-scoped browse (GetPublicBySellerID) returns only
//      active + in-stock surfaces — never draft/sold/withdrawn;
//   4. default auction browse returns only public discovery states
//      (scheduled/active) and explicit-status filters stay owner-safe at the
//      handler boundary;
//   5. discovery search enforces quantity>0 for FPS and excludes non-public
//      auction states;
//   6. a Product cannot receive a second ForSale surface after its first
//      surface exists.

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	auctionApp "github.com/labuda/backend/internal/commerce/auction/application"
	auctionhttp "github.com/labuda/backend/internal/commerce/auction/delivery/http"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	auctioninfra "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	fpsApp "github.com/labuda/backend/internal/commerce/forsale/application"
	fpshttp "github.com/labuda/backend/internal/commerce/forsale/delivery/http"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	fpsinfra "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	fpsRepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

func seedStage6BUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), 'active', NOW(), NOW())
		`, userID, "fb-"+userID.String(), userID.String()+"@stage6b.invalid")
		return err
	}))
	return userID
}

func seedStage6BProduct(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	productID := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at)
			VALUES ($1, $2, 'Stage6B Koi', 'desc', '[]'::jsonb, 'kohaku', 'immediate', NOW(), NOW())
		`, productID, sellerID)
		return err
	}))
	return productID
}

func seedStage6BFPS(t *testing.T, ctx context.Context, tdb *testdb.TestDB, productID, sellerID uuid.UUID, status string, qty int, published bool) uuid.UUID {
	t.Helper()
	saleID := uuid.New()
	var publishedAt interface{}
	if published {
		publishedAt = "NOW()"
	} else {
		publishedAt = nil
	}
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, published_at, quantity_available, created_at, updated_at)
			VALUES ($1, $2, $3, 100000, false, $4, `+pubExpr(publishedAt)+`, $5, NOW(), NOW())
		`, saleID, productID, sellerID, status, qty)
		return err
	}))
	return saleID
}

func pubExpr(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	return "NOW()"
}

func seedStage6BAuction(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	productID := seedStage6BProduct(t, ctx, tdb, sellerID)
	auctionID := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO auctions (id, seller_id, product_id,
				start_price, bid_increment, buy_now_price, start_at, end_at, current_bid, status, created_at, updated_at)
			VALUES ($1, $2, $3, 10000, 1000, NULL, NOW(), NOW() + INTERVAL '24 hours', NULL, $4, NOW(), NOW())
		`, auctionID, sellerID, productID, status)
		return err
	}))
	return auctionID
}

func stage6bFPSSet(t *testing.T, ctx context.Context, tdb *testdb.TestDB, saleID uuid.UUID) (qty int, status string) {
	t.Helper()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT quantity_available, status FROM for_sales WHERE id = $1`, saleID).
			Scan(&qty, &status)
	}))
	return qty, status
}

func TestStage6B_FPS_SoldOut_HiddenFromPublicDiscovery_VisibleInSellerInventory(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	seller := seedStage6BUser(t, ctx, tdb)
	product := seedStage6BProduct(t, ctx, tdb, seller)
	sale := seedStage6BFPS(t, ctx, tdb, product, seller, "active", 1, true)

	repo := fpsinfra.NewForSaleRepository()

	// Active qty=1 must be discoverable.
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		got, err := repo.GetPublic(ctx, tx, 20, 0)
		require.NoError(t, err)
		require.Contains(t, saleIDs(got), sale, "active qty=1 must be in public catalog")

		got, _, err = repo.Search(ctx, tx, fpsRepo.SearchFilters{})
		require.NoError(t, err)
		require.Contains(t, saleIDs(got), sale, "active qty=1 must be in search")

		got, err = repo.GetPublicBySellerID(ctx, tx, seller, 20, 0)
		require.NoError(t, err)
		require.Contains(t, saleIDs(got), sale, "active qty=1 must be on public seller page")
		return nil
	}))

	// Sell out: ReduceQuantity to 0 flips sold.
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		l, err := repo.GetForUpdate(ctx, tx, sale)
		if err != nil {
			return err
		}
		if err := l.ReduceQuantity(1); err != nil {
			return err
		}
		return repo.UpdateStock(ctx, tx, l)
	}))
	qty, status := stage6bFPSSet(t, ctx, tdb, sale)
	require.Equal(t, 0, qty)
	require.Equal(t, string(fpsEntity.ForSaleStatusSold), status)

	// Sold-out must disappear from public discovery.
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		got, err := repo.GetPublic(ctx, tx, 20, 0)
		require.NoError(t, err)
		require.NotContains(t, saleIDs(got), sale, "sold-out must vanish from public catalog")

		got, _, err = repo.Search(ctx, tx, fpsRepo.SearchFilters{})
		require.NoError(t, err)
		require.NotContains(t, saleIDs(got), sale, "sold-out must vanish from search")

		got, err = repo.GetPublicBySellerID(ctx, tx, seller, 20, 0)
		require.NoError(t, err)
		require.NotContains(t, saleIDs(got), sale, "sold-out must vanish from public seller page")
		return nil
	}))

	// Sold-out must remain visible to the owning seller (inventory authority).
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		got, err := repo.GetBySellerIDPaginated(ctx, tx, seller, 20, 0, false)
		require.NoError(t, err)
		require.Contains(t, saleIDs(got), sale, "sold-out must remain in owner inventory")
		return nil
	}))
}

func TestStage6B_FPS_MultiQty_DiscoverableWhileStockRemains(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	seller := seedStage6BUser(t, ctx, tdb)
	product := seedStage6BProduct(t, ctx, tdb, seller)
	sale := seedStage6BFPS(t, ctx, tdb, product, seller, "active", 3, true)

	repo := fpsinfra.NewForSaleRepository()

	visible := func(want bool) {
		t.Helper()
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			got, err := repo.GetPublic(ctx, tx, 20, 0)
			require.NoError(t, err)
			inSearch, _, err := repo.Search(ctx, tx, fpsRepo.SearchFilters{})
			require.NoError(t, err)
			s := saleIDs(got)
			ss := saleIDs(inSearch)
			if want {
				require.Contains(t, s, sale, "qty>0 must be in catalog")
				require.Contains(t, ss, sale, "qty>0 must be in search")
			} else {
				require.NotContains(t, s, sale, "qty=0 must vanish from catalog")
				require.NotContains(t, ss, sale, "qty=0 must vanish from search")
			}
			return nil
		}))
	}

	visible(true)

	// Sell 2 of 3 — still discoverable.
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		l, err := repo.GetForUpdate(ctx, tx, sale)
		if err != nil {
			return err
		}
		if err := l.ReduceQuantity(2); err != nil {
			return err
		}
		return repo.UpdateStock(ctx, tx, l)
	}))
	visible(true)

	// Sell last unit — quantity 0, gone from discovery.
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		l, err := repo.GetForUpdate(ctx, tx, sale)
		if err != nil {
			return err
		}
		if err := l.ReduceQuantity(1); err != nil {
			return err
		}
		return repo.UpdateStock(ctx, tx, l)
	}))
	qty, status := stage6bFPSSet(t, ctx, tdb, sale)
	require.Equal(t, 0, qty)
	require.Equal(t, string(fpsEntity.ForSaleStatusSold), status)
	visible(false)
}

func TestStage6B_GetPublicBySellerID_OnlyActiveInStock(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	seller := seedStage6BUser(t, ctx, tdb)

	prodA := seedStage6BProduct(t, ctx, tdb, seller)
	prodB := seedStage6BProduct(t, ctx, tdb, seller)
	prodC := seedStage6BProduct(t, ctx, tdb, seller)
	prodD := seedStage6BProduct(t, ctx, tdb, seller)
	prodE := seedStage6BProduct(t, ctx, tdb, seller)

	saleDraft := seedStage6BFPS(t, ctx, tdb, prodA, seller, "draft", 1, false)
	saleActive := seedStage6BFPS(t, ctx, tdb, prodB, seller, "active", 2, true)
	saleActiveZero := seedStage6BFPS(t, ctx, tdb, prodC, seller, "active", 0, true) // drift case
	saleSold := seedStage6BFPS(t, ctx, tdb, prodD, seller, "sold", 0, true)
	saleWithdrawn := seedStage6BFPS(t, ctx, tdb, prodE, seller, "withdrawn", 1, false)

	repo := fpsinfra.NewForSaleRepository()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		got, err := repo.GetPublicBySellerID(ctx, tx, seller, 20, 0)
		require.NoError(t, err)
		ids := saleIDs(got)
		require.Len(t, ids, 1, "only the active in-stock surface may survive the public seller page")
		require.Contains(t, ids, saleActive)
		require.NotContains(t, ids, saleDraft)
		require.NotContains(t, ids, saleActiveZero)
		require.NotContains(t, ids, saleSold)
		require.NotContains(t, ids, saleWithdrawn)
		return nil
	}))
}

func TestStage6B_AuctionBrowse_DefaultOnlyPublicStates(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	seller := seedStage6BUser(t, ctx, tdb)

	draft := seedStage6BAuction(t, ctx, tdb, seller, "draft")
	scheduled := seedStage6BAuction(t, ctx, tdb, seller, "scheduled")
	active := seedStage6BAuction(t, ctx, tdb, seller, "active")
	waiting := seedStage6BAuction(t, ctx, tdb, seller, "waiting_settlement")
	ended := seedStage6BAuction(t, ctx, tdb, seller, "ended")
	cancelled := seedStage6BAuction(t, ctx, tdb, seller, "cancelled")
	expired := seedStage6BAuction(t, ctx, tdb, seller, "expired_bnr")

	repo := auctioninfra.NewAuctionRepository()

	// Default browse: only scheduled + active.
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		auctions, err := repo.List(ctx, tx, auctioninfra.AuctionFilter{})
		require.NoError(t, err)
		ids := auctionIDs(auctions)
		require.Contains(t, ids, scheduled)
		require.Contains(t, ids, active)
		require.NotContains(t, ids, draft)
		require.NotContains(t, ids, waiting)
		require.NotContains(t, ids, ended)
		require.NotContains(t, ids, cancelled)
		require.NotContains(t, ids, expired)
		return nil
	}))

	// Explicit status filter still functions (service-level owner scoping is
	// enforced at the handler boundary for non-public states).
	st := auctionEntity.StatusActive
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		auctions, err := repo.List(ctx, tx, auctioninfra.AuctionFilter{Status: &st})
		require.NoError(t, err)
		require.Contains(t, auctionIDs(auctions), active)
		return nil
	}))
}

func TestStage6B_FPSBrowse_AnonymousSellerFilter_PublicOnly(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	seller := seedStage6BUser(t, ctx, tdb)
	prodDraft := seedStage6BProduct(t, ctx, tdb, seller)
	prodActive := seedStage6BProduct(t, ctx, tdb, seller)
	prodSold := seedStage6BProduct(t, ctx, tdb, seller)
	saleDraft := seedStage6BFPS(t, ctx, tdb, prodDraft, seller, "draft", 1, false)
	saleActive := seedStage6BFPS(t, ctx, tdb, prodActive, seller, "active", 2, true)
	saleSold := seedStage6BFPS(t, ctx, tdb, prodSold, seller, "sold", 0, true)

	handler := fpshttp.NewForSaleHandler(
		fpsApp.NewForSaleService(),
		db.NewFromPool(tdb.Pool()),
		zap.NewNop(),
		nil,
	)

	// Anonymous seller-filter browse must only expose active + in-stock.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/for-sale?seller_id="+seller.String(), nil)
	handler.ListForSales(c)
	require.Equal(t, 200, w.Code)
	forSaleIDs := parseFPSBrowseIDs(t, w.Body.String())
	require.Contains(t, forSaleIDs, saleActive, "anonymous seller page must show active in-stock")
	require.NotContains(t, forSaleIDs, saleDraft, "anonymous seller page must NOT expose drafts")
	require.NotContains(t, forSaleIDs, saleSold, "anonymous seller page must NOT expose sold-out")

	// Same caller as owner: full inventory (draft + active + sold).
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/for-sale?seller_id="+seller.String(), nil)
	c.Set("userID", seller)
	handler.ListForSales(c)
	require.Equal(t, 200, w.Code)
	forSaleIDs = parseFPSBrowseIDs(t, w.Body.String())
	require.Contains(t, forSaleIDs, saleActive, "owner inventory must show active")
	require.Contains(t, forSaleIDs, saleDraft, "owner inventory must show draft")
	require.Contains(t, forSaleIDs, saleSold, "owner inventory must show sold-out")
}

func TestStage6B_AuctionBrowse_AnonymousRestricted_OwnerStatusScoped(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	seller := seedStage6BUser(t, ctx, tdb)
	auctionDraft := seedStage6BAuction(t, ctx, tdb, seller, "draft")
	auctionActive := seedStage6BAuction(t, ctx, tdb, seller, "active")

	handler := auctionhttp.NewAuctionHandler(
		auctionApp.NewAuctionService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil),
		nil,
		nil,
		db.NewFromPool(tdb.Pool()),
		zap.NewNop(),
	)

	// Anonymous default browse: public states only.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auctions", nil)
	handler.ListAuctions(c)
	require.Equal(t, 200, w.Code)
	ids := parseAuctionBrowseIDs(t, w.Body.String())
	require.Contains(t, ids, auctionActive, "anonymous browse must show active auction")
	require.NotContains(t, ids, auctionDraft, "anonymous browse must NOT show draft auction")

	// Anonymous status=draft must be rejected as empty (non-public state).
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auctions?status=draft", nil)
	handler.ListAuctions(c)
	require.Equal(t, 200, w.Code)
	ids = parseAuctionBrowseIDs(t, w.Body.String())
	require.Empty(t, ids, "anonymous status=draft must return empty")

	// Owner with seller filter + status=draft: own drafts visible.
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auctions?status=draft&seller_id="+seller.String(), nil)
	c.Set("userID", seller)
	handler.ListAuctions(c)
	require.Equal(t, 200, w.Code)
	ids = parseAuctionBrowseIDs(t, w.Body.String())
	require.Contains(t, ids, auctionDraft, "owner status=draft must list own drafts")
}

func TestStage6B_ReuseQuantity_RejectsSecondForSale(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	seller := seedStage6BUser(t, ctx, tdb)
	product := seedStage6BProduct(t, ctx, tdb, seller)
	repo := fpsinfra.NewForSaleRepository()

	// First surface: qty=10, then 3 units reserved → 7 remaining.
	saleA, err := fpsEntity.NewForSale(
		seller, "Koi A", "desc", []byte(`[]`), "Kohaku",
		nil, nil, nil, nil, nil, []string{},
		fpsEntity.ForSaleTypeFixedPrice, money.New(100000), 10, false,
		fpsEntity.ForSaleVisibilityPublic, fpsEntity.ForSaleOriginDirectCreate,
		nil, fpsEntity.PreparationTimeImmediate, nil,
	)
	require.NoError(t, err)
	require.NoError(t, saleA.Publish())
	saleA.ProductID = product
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error { return repo.Create(ctx, tx, saleA) }))
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		l, err := repo.GetForUpdate(ctx, tx, saleA.ID)
		if err != nil {
			return err
		}
		if err := l.ReduceQuantity(3); err != nil {
			return err
		}
		return repo.UpdateStock(ctx, tx, l)
	}))

	// A second ForSale cannot replace the existing stock-owning surface.
	saleB, err := fpsEntity.NewForSale(
		seller, "Koi B", "desc", []byte(`[]`), "Kohaku",
		nil, nil, nil, nil, nil, []string{},
		fpsEntity.ForSaleTypeFixedPrice, money.New(150000), 1, false,
		fpsEntity.ForSaleVisibilityPublic, fpsEntity.ForSaleOriginDirectCreate,
		nil, fpsEntity.PreparationTimeImmediate, nil,
	)
	require.NoError(t, err)
	require.NoError(t, saleB.Publish())
	saleB.ProductID = product
	require.Error(t, tdb.WithTx(ctx, func(tx db.Tx) error { return repo.Create(ctx, tx, saleB) }))

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		qtyA, _ := stage6bFPSSetRaw(ctx, tx, saleA.ID)
		require.Equal(t, 7, qtyA, "existing surface keeps its remaining units")
		return nil
	}))
}

func stage6bFPSSetRaw(ctx context.Context, tx db.Tx, saleID uuid.UUID) (int, string) {
	var qty int
	var status string
	if err := tx.QueryRow(ctx, `SELECT quantity_available, status FROM for_sales WHERE id = $1`, saleID).Scan(&qty, &status); err != nil {
		panic(err)
	}
	return qty, status
}

func saleIDs(sales []*fpsEntity.ForSale) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(sales))
	for _, s := range sales {
		if s != nil {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

func auctionIDs(auctions []*auctionEntity.Auction) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(auctions))
	for _, a := range auctions {
		ids = append(ids, a.ID)
	}
	return ids
}

func parseFPSBrowseIDs(t *testing.T, body string) []uuid.UUID {
	t.Helper()
	var env struct {
		Data struct {
			ForSales []struct {
				ID string `json:"id"`
			} `json:"for_sales"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &env))
	ids := make([]uuid.UUID, 0, len(env.Data.ForSales))
	for _, l := range env.Data.ForSales {
		ids = append(ids, uuid.MustParse(l.ID))
	}
	return ids
}

func parseAuctionBrowseIDs(t *testing.T, body string) []uuid.UUID {
	t.Helper()
	var env struct {
		Data struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &env))
	ids := make([]uuid.UUID, 0, len(env.Data.Data))
	for _, a := range env.Data.Data {
		ids = append(ids, uuid.MustParse(a.ID))
	}
	return ids
}
