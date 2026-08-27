//go:build integration

package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingrepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type sellerShippingListResponse struct {
	Success bool                           `json:"success"`
	Data    sellerShippingListResponseData `json:"data"`
}

type sellerShippingListResponseData struct {
	ShippingOptions []map[string]any `json:"shipping_options"`
	Count           int              `json:"count"`
}

func setupSellerShippingHandlerTest(t *testing.T) (*testdb.TestDB, *SellerShippingHandler, func()) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)

	cfg, err := config.Load()
	require.NoError(t, err)

	appDB, err := db.New(context.Background(), db.Config{
		ConnString: cfg.Database.GetTestDSN(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { appDB.Close() })

	optionRepo := shippingrepo.NewShippingOptionRepository()
	coverageRepo := shippingrepo.NewShippingCoverageRepository()
	cityOverrideRepo := shippingrepo.NewCityOverrideRepository()
	productShippingRepo := shippingrepo.NewProductShippingOptionRepository(optionRepo)
	service := shippingApp.NewSellerShippingService(
		optionRepo,
		coverageRepo,
		cityOverrideRepo,
		productShippingRepo,
	)

	handler := NewSellerShippingHandler(service, appDB, zap.NewNop())
	return tdb, handler, cleanup
}

func seedSellerShippingOptions(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	sellerID := uuid.New()

	_, err := pool.Exec(
		ctx,
		`INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'user')`,
		sellerID, sellerID.String(), sellerID.String()+"@test.invalid",
	)
	require.NoError(t, err)

	activeID := uuid.New()
	inactiveID := uuid.New()
	_, err = pool.Exec(
		ctx,
		`INSERT INTO shipping_options (id, seller_id, name, transport_type, expedition_name, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW()),
		        ($6, $2, $7, $8, $9, false, NOW(), NOW())`,
		activeID, sellerID, "JNE Reguler", "train", "JNE",
		inactiveID, "Kirim Kustom", "custom", nil,
	)
	require.NoError(t, err)

	return sellerID
}

func TestSellerShippingHandler_ListShippingOptions_ReturnsWrappedObject(t *testing.T) {
	tdb, handler, cleanup := setupSellerShippingHandlerTest(t)
	defer cleanup()

	sellerID := seedSellerShippingOptions(t, tdb.Pool())

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/seller/shipping/options?include_inactive=true", nil)
	c.Set("userID", sellerID)

	handler.ListShippingOptions(c)

	require.Equal(t, http.StatusOK, w.Code)

	var resp sellerShippingListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Equal(t, 2, resp.Data.Count)
	require.Len(t, resp.Data.ShippingOptions, 2)

	first := resp.Data.ShippingOptions[0]
	require.Contains(t, first, "id")
	require.Contains(t, first, "name")
	require.Contains(t, first, "transport_type")
	require.Contains(t, first, "is_active")
	require.Contains(t, first, "expedition_name")
	require.Contains(t, first, "created_at")
	require.Contains(t, first, "updated_at")
	require.NotContains(t, first, "seller_id")
	require.NotContains(t, first, "display_name")
	require.NotContains(t, first, "province_code")
}

func TestSellerShippingHandler_ListShippingOptions_RequiresUserID(t *testing.T) {
	tdb, handler, cleanup := setupSellerShippingHandlerTest(t)
	_ = tdb
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/seller/shipping/options", nil)

	handler.ListShippingOptions(c)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}
