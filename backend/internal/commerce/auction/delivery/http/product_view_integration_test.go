//go:build integration

package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	actionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	auctionApp "github.com/labuda/backend/internal/commerce/auction/application"
	productviewInfraRepo "github.com/labuda/backend/internal/commerce/productview/infrastructure/repository"
	productviewRepo "github.com/labuda/backend/internal/commerce/productview/repository"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func seedAuctionViewUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', 'user', NOW(), NOW())
	`, id, "fb-"+id.String(), id.String()+"@test.invalid")
	require.NoError(t, err)
	return id
}

func seedAuctionViewListing(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID uuid.UUID) (productID, auctionID uuid.UUID) {
	t.Helper()
	productID = uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at)
		VALUES ($1, $2, 'Koi', 'desc', '[]', 'kohaku', '1_3_days', 'auction', NOW(), NOW())
	`, productID, sellerID)
	require.NoError(t, err)

	auctionID = uuid.New()
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, buy_now_price, start_at, end_at, current_bid, current_winner_id, status, created_at, updated_at, anti_snipe_extension_seconds)
		VALUES ($1, $2, $3, 10000, 1000, NULL, NOW(), NOW() + interval '1 day', NULL, NULL, 'active', NOW(), NOW(), 0)
	`, auctionID, sellerID, productID)
	require.NoError(t, err)
	return productID, auctionID
}

func newAuctionViewHandler(tdb *testdb.TestDB, pvRepo productviewRepo.ProductViewRepository) *AuctionHandler {
	svc := auctionApp.NewAuctionService(nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	return NewAuctionHandler(svc, nil, nil, db.NewFromPool(tdb.Pool()), zap.NewNop(), pvRepo)
}

func callGetAuction(t *testing.T, h *AuctionHandler, auctionID string, viewerID *uuid.UUID, actor *capabilityEntity.Actor) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: auctionID}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auctions/"+auctionID, nil)
	if viewerID != nil {
		c.Set("userID", *viewerID)
	}
	if actor != nil {
		c.Request = c.Request.WithContext(capability.WithActor(c.Request.Context(), actor))
	}
	h.GetAuction(c)
	return w
}

func countAuctionProductViews(t *testing.T, tdb *testdb.TestDB, pvRepo productviewRepo.ProductViewRepository, productID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, tdb.WithTx(context.Background(), func(tx db.Tx) error {
		var err error
		n, err = pvRepo.CountByProduct(context.Background(), tx, productID)
		return err
	}))
	return n
}

func totalAuctionProductViews(t *testing.T, tdb *testdb.TestDB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT COUNT(*) FROM product_view_events`).Scan(&n)
	}))
	return n
}

// TestAuctionDetail_ProductView covers the canonical Product View contract for
// the Auction detail surface in one DB session.
func TestAuctionDetail_ProductView(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	pvRepo := productviewInfraRepo.NewProductViewRepository()
	h := newAuctionViewHandler(tdb, pvRepo)
	seller := seedAuctionViewUser(t, ctx, tdb)

	t.Run("anonymous creates exactly one view", func(t *testing.T) {
		productID, auctionID := seedAuctionViewListing(t, ctx, tdb, seller)
		w := callGetAuction(t, h, auctionID.String(), nil, nil)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, int64(1), countAuctionProductViews(t, tdb, pvRepo, productID))
	})

	t.Run("repeated views create multiple events", func(t *testing.T) {
		productID, auctionID := seedAuctionViewListing(t, ctx, tdb, seller)
		for i := 0; i < 3; i++ {
			require.Equal(t, http.StatusOK, callGetAuction(t, h, auctionID.String(), nil, nil).Code)
		}
		require.Equal(t, int64(3), countAuctionProductViews(t, tdb, pvRepo, productID))
	})

	t.Run("authenticated viewer counted with identity", func(t *testing.T) {
		productID, auctionID := seedAuctionViewListing(t, ctx, tdb, seller)
		viewer := seedAuctionViewUser(t, ctx, tdb)
		w := callGetAuction(t, h, auctionID.String(), &viewer, nil)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, int64(1), countAuctionProductViews(t, tdb, pvRepo, productID))

		var viewers int64
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT COUNT(*) FROM product_view_events WHERE product_id = $1 AND viewer_user_id = $2`, productID, viewer).Scan(&viewers)
		}))
		require.Equal(t, int64(1), viewers)
	})

	t.Run("seller self-view not counted", func(t *testing.T) {
		productID, auctionID := seedAuctionViewListing(t, ctx, tdb, seller)
		w := callGetAuction(t, h, auctionID.String(), &seller, nil)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, int64(0), countAuctionProductViews(t, tdb, pvRepo, productID))
	})

	t.Run("admin not counted", func(t *testing.T) {
		productID, auctionID := seedAuctionViewListing(t, ctx, tdb, seller)
		adminID := seedAuctionViewUser(t, ctx, tdb)
		admin := &capabilityEntity.Actor{ID: adminID, Role: capabilityEntity.AdminRole}
		w := callGetAuction(t, h, auctionID.String(), &adminID, admin)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, int64(0), countAuctionProductViews(t, tdb, pvRepo, productID))
	})

	t.Run("not found records nothing", func(t *testing.T) {
		before := totalAuctionProductViews(t, tdb)
		w := callGetAuction(t, h, uuid.New().String(), nil, nil)
		require.Equal(t, http.StatusNotFound, w.Code)
		require.Equal(t, before, totalAuctionProductViews(t, tdb))
	})

	t.Run("active terminal relist refreshes current detail lifecycle", func(t *testing.T) {
		productID, auctionID := seedAuctionViewListing(t, ctx, tdb, seller)
		viewer := seedAuctionViewUser(t, ctx, tdb)
		require.Equal(t, http.StatusOK, callGetAuction(t, h, auctionID.String(), &viewer, nil).Code)

		repo := actionRepo.NewAuctionRepository()
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			auction, err := repo.GetForUpdate(ctx, tx, auctionID)
			if err != nil {
				return err
			}
			if err := auction.End(); err != nil {
				return err
			}
			return repo.UpdateTx(ctx, tx, auction)
		}))

		ended := callGetAuction(t, h, auctionID.String(), &viewer, nil)
		require.Equal(t, http.StatusOK, ended.Code)
		require.Contains(t, ended.Body.String(), `"ended"`)
		require.Equal(t, int64(2), countAuctionProductViews(t, tdb, pvRepo, productID))

		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			auction, err := repo.GetForUpdate(ctx, tx, auctionID)
			if err != nil {
				return err
			}
			if err := auction.Relist(
				time.Now().UTC(),
				time.Now().UTC().Add(24*time.Hour),
				10000,
				1000,
				nil,
			); err != nil {
				return err
			}
			if err := repo.UpdateTx(ctx, tx, auction); err != nil {
				return err
			}
			return nil
		}))

		relisted := callGetAuction(t, h, auctionID.String(), &viewer, nil)
		require.Equal(t, http.StatusOK, relisted.Code)
		require.Contains(t, relisted.Body.String(), `"scheduled"`)
		// Relist canonical lifecycle is scheduled first; activation is the
		// separate canonical worker/entity transition.
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			auction, err := repo.GetForUpdate(ctx, tx, auctionID)
			if err != nil {
				return err
			}
			if err := auction.Activate(); err != nil {
				return err
			}
			return repo.UpdateTx(ctx, tx, auction)
		}))

		active := callGetAuction(t, h, auctionID.String(), &viewer, nil)
		require.Equal(t, http.StatusOK, active.Code)
		require.Contains(t, active.Body.String(), `"active"`)
	})

	t.Run("historical view preserved after ended", func(t *testing.T) {
		productID, auctionID := seedAuctionViewListing(t, ctx, tdb, seller)
		require.Equal(t, http.StatusOK, callGetAuction(t, h, auctionID.String(), nil, nil).Code)
		require.Equal(t, int64(1), countAuctionProductViews(t, tdb, pvRepo, productID))

		_, err := tdb.Pool().Exec(ctx, `UPDATE auctions SET status = 'ended', updated_at = NOW() WHERE id = $1`, auctionID)
		require.NoError(t, err)

		require.Equal(t, int64(1), countAuctionProductViews(t, tdb, pvRepo, productID))
	})
}
