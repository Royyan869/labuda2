//go:build integration

package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	forSaleApp "github.com/labuda/backend/internal/commerce/forsale/application"
	productviewInfraRepo "github.com/labuda/backend/internal/commerce/productview/infrastructure/repository"
	productviewRepo "github.com/labuda/backend/internal/commerce/productview/repository"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func seedForSaleViewUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', 'user', NOW(), NOW())
	`, id, "fb-"+id.String(), id.String()+"@test.invalid")
	require.NoError(t, err)
	return id
}

// seedForSaleViewListing inserts a product + for_sale. When publishedAt is nil
// the surface is private (derived visibility), which proves the forbidden path.
func seedForSaleViewListing(t *testing.T, ctx context.Context, tdb *testdb.TestDB, status string, sellerID uuid.UUID, published bool) (productID, forSaleID uuid.UUID) {
	t.Helper()
	productID = uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at)
		VALUES ($1, $2, 'Koi', 'desc', '[]', 'kohaku', '1_3_days', 'for_sale', NOW(), NOW())
	`, productID, sellerID)
	require.NoError(t, err)

	forSaleID = uuid.New()
	if published {
		_, err = tdb.Pool().Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, published_at, quantity_available, created_at, updated_at)
			VALUES ($1, $2, $3, 100000, false, $4, NOW(), 1, NOW(), NOW())
		`, forSaleID, productID, sellerID, status)
	} else {
		_, err = tdb.Pool().Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, published_at, quantity_available, created_at, updated_at)
			VALUES ($1, $2, $3, 100000, false, $4, NULL, 1, NOW(), NOW())
		`, forSaleID, productID, sellerID, status)
	}
	require.NoError(t, err)
	return productID, forSaleID
}

func newForSaleViewHandler(tdb *testdb.TestDB, pvRepo productviewRepo.ProductViewRepository) *ForSaleHandler {
	return NewForSaleHandler(
		forSaleApp.NewForSaleService(),
		db.NewFromPool(tdb.Pool()),
		zap.NewNop(),
		nil,
		nil,
		pvRepo,
	)
}

func callGetForSale(t *testing.T, h *ForSaleHandler, forSaleID string, userID *uuid.UUID, actor *capabilityEntity.Actor) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: forSaleID}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/for-sale/"+forSaleID, nil)
	if userID != nil {
		c.Set("userID", *userID)
	}
	if actor != nil {
		c.Request = c.Request.WithContext(capability.WithActor(c.Request.Context(), actor))
	}
	h.GetForSale(c)
	return w
}

func countForSaleProductViews(t *testing.T, tdb *testdb.TestDB, pvRepo productviewRepo.ProductViewRepository, productID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, tdb.WithTx(context.Background(), func(tx db.Tx) error {
		var err error
		n, err = pvRepo.CountByProduct(context.Background(), tx, productID)
		return err
	}))
	return n
}

func totalForSaleProductViews(t *testing.T, tdb *testdb.TestDB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT COUNT(*) FROM product_view_events`).Scan(&n)
	}))
	return n
}

// TestForSaleDetail_ProductView covers the full canonical Product View contract
// for the For Sale detail surface in one DB session (SetupDB is expensive).
func TestForSaleDetail_ProductView(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	pvRepo := productviewInfraRepo.NewProductViewRepository()
	h := newForSaleViewHandler(tdb, pvRepo)
	seller := seedForSaleViewUser(t, ctx, tdb)

	t.Run("anonymous creates exactly one view", func(t *testing.T) {
		productID, forSaleID := seedForSaleViewListing(t, ctx, tdb, "active", seller, true)
		w := callGetForSale(t, h, forSaleID.String(), nil, nil)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, int64(1), countForSaleProductViews(t, tdb, pvRepo, productID))
	})

	t.Run("repeated views create multiple events", func(t *testing.T) {
		productID, forSaleID := seedForSaleViewListing(t, ctx, tdb, "active", seller, true)
		for i := 0; i < 5; i++ {
			require.Equal(t, http.StatusOK, callGetForSale(t, h, forSaleID.String(), nil, nil).Code)
		}
		require.Equal(t, int64(5), countForSaleProductViews(t, tdb, pvRepo, productID))
	})

	t.Run("authenticated buyer counted with viewer identity", func(t *testing.T) {
		productID, forSaleID := seedForSaleViewListing(t, ctx, tdb, "active", seller, true)
		buyer := seedForSaleViewUser(t, ctx, tdb)
		w := callGetForSale(t, h, forSaleID.String(), &buyer, nil)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, int64(1), countForSaleProductViews(t, tdb, pvRepo, productID))

		var viewers int64
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT COUNT(*) FROM product_view_events WHERE product_id = $1 AND viewer_user_id = $2`, productID, buyer).Scan(&viewers)
		}))
		require.Equal(t, int64(1), viewers)
	})

	t.Run("seller self-view not counted", func(t *testing.T) {
		productID, forSaleID := seedForSaleViewListing(t, ctx, tdb, "active", seller, true)
		w := callGetForSale(t, h, forSaleID.String(), &seller, nil)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, int64(0), countForSaleProductViews(t, tdb, pvRepo, productID))
	})

	t.Run("admin not counted", func(t *testing.T) {
		productID, forSaleID := seedForSaleViewListing(t, ctx, tdb, "active", seller, true)
		adminID := seedForSaleViewUser(t, ctx, tdb)
		admin := &capabilityEntity.Actor{ID: adminID, Role: capabilityEntity.AdminRole}
		w := callGetForSale(t, h, forSaleID.String(), &adminID, admin)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, int64(0), countForSaleProductViews(t, tdb, pvRepo, productID))
	})

	t.Run("not found records nothing", func(t *testing.T) {
		before := totalForSaleProductViews(t, tdb)
		w := callGetForSale(t, h, uuid.New().String(), nil, nil)
		require.Equal(t, http.StatusNotFound, w.Code)
		require.Equal(t, before, totalForSaleProductViews(t, tdb))
	})

	t.Run("forbidden private listing records nothing", func(t *testing.T) {
		productID, forSaleID := seedForSaleViewListing(t, ctx, tdb, "active", seller, false)
		w := callGetForSale(t, h, forSaleID.String(), nil, nil)
		require.Equal(t, http.StatusNotFound, w.Code)
		require.Equal(t, int64(0), countForSaleProductViews(t, tdb, pvRepo, productID))
	})

	t.Run("historical view preserved after sold", func(t *testing.T) {
		productID, forSaleID := seedForSaleViewListing(t, ctx, tdb, "active", seller, true)
		require.Equal(t, http.StatusOK, callGetForSale(t, h, forSaleID.String(), nil, nil).Code)
		require.Equal(t, int64(1), countForSaleProductViews(t, tdb, pvRepo, productID))

		_, err := tdb.Pool().Exec(ctx, `UPDATE for_sales SET status = 'sold', quantity_available = 0, sold_at = NOW(), updated_at = NOW() WHERE id = $1`, forSaleID)
		require.NoError(t, err)

		require.Equal(t, int64(1), countForSaleProductViews(t, tdb, pvRepo, productID))
	})
}
