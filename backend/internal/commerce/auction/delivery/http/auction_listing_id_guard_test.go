// PASS_21B negative guard: auction must never be created from a Listing.
//
// CreateAuction already omits listing_id/product_id from its request struct
// (a Product is always created inline), but nothing previously stopped a
// stale/legacy client from sending listing_id anyway and having it silently
// ignored. This test locks in the explicit rejection added in PASS_21B,
// mirroring the equivalent guard already proven for order creation
// (TestCreateOrder_RejectsLegacyListingID).
package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreateAuction_RejectsLegacyListingID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAuctionHandler(nil, nil, nil, nil, nil)

	body := map[string]interface{}{
		"listing_id":          uuid.New().String(),
		"title":               "Showa Auction",
		"description":         "A test auction",
		"shipping_option_ids": []string{uuid.New().String()},
		"start_price":         10000,
		"bid_increment":       1000,
		"start_mode":          "now",
		"duration_hours":      24,
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/auctions", bytes.NewReader(payload))
	require.NoError(t, err)
	c.Request = req
	c.Set("userID", uuid.New())

	handler.CreateAuction(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAuction_RejectsLegacyListingIdCamelCase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAuctionHandler(nil, nil, nil, nil, nil)

	body := map[string]interface{}{
		"listingId":           uuid.New().String(),
		"title":               "Showa Auction",
		"description":         "A test auction",
		"shipping_option_ids": []string{uuid.New().String()},
		"start_price":         10000,
		"bid_increment":       1000,
		"start_mode":          "now",
		"duration_hours":      24,
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/auctions", bytes.NewReader(payload))
	require.NoError(t, err)
	c.Request = req
	c.Set("userID", uuid.New())

	handler.CreateAuction(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
