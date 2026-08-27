//go:build integration

// PASS_21B regression test: GetDashboard counts real for_sales rows.
// Before PASS_21B this counted rows in the legacy `listings` table, which
// nothing writes to anymore — every seller's dashboard reported 0 total/
// active listings regardless of how many real fixed-price sales they had.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestGetDashboard_CountsRealForSales(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := uuid.New()
	activeProductID := uuid.New()
	draftProductID := uuid.New()

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email) VALUES ($1, $2, $3)`,
			sellerID, "fb-"+sellerID.String(), sellerID.String()+"@dashboard.test",
		); err != nil {
			return err
		}

		for _, p := range []struct {
			id uuid.UUID
		}{{activeProductID}, {draftProductID}} {
			if _, err := tx.Exec(ctx, `
				INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, p.id, sellerID, "Koi", "desc", `[]`, "kohaku", "immediate"); err != nil {
				return err
			}
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, published_at, quantity_available)
			VALUES ($1, $2, $3, 100000, 'active', NOW(), 1)
		`, uuid.New(), activeProductID, sellerID); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, quantity_available)
			VALUES ($1, $2, $3, 100000, 'draft', 1)
		`, uuid.New(), draftProductID, sellerID)
		return err
	}))

	handler := &SellerHandler{
		db:  db.NewFromPool(tdb.Pool()),
		log: zap.NewNop(),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodGet, "/api/v1/seller/dashboard", nil)
	require.NoError(t, err)
	c.Request = req
	c.Set("userID", sellerID)

	handler.GetDashboard(c)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data SellerDashboardResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, int64(2), resp.Data.TotalListings)
	require.Equal(t, int64(1), resp.Data.ActiveListings)
}
