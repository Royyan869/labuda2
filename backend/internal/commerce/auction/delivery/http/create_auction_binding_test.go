// PASS_18E: locks CreateAuctionRequest's JSON binding contract.
//
// PASS_18D found mobile never sent shipping_option_ids, so every mobile
// create-auction request 400'd against this binding before ever reaching
// AuctionService.CreateDraft. These tests exercise the binding rule in
// isolation — auction is still a physical fish that must ship, so
// shipping_option_ids stays required (min=1), independent of any client.
//
// Full handler construction (AuctionService + *db.DB) is unnecessary here:
// c.ShouldBindJSON runs before either dependency is touched in CreateAuction,
// so testing the request struct's binding tags directly is the correct,
// lighter-weight boundary.
package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// bindCreateAuctionRequest exercises exactly the binding step CreateAuction
// runs before touching h.auctionService/h.db, capturing the parsed struct
// on success so field-level assertions can be made without constructing the
// full handler.
func bindCreateAuctionRequest(body string) (captured *CreateAuctionRequest, statusCode int, responseBody string) {
	router := gin.New()
	router.POST("/auctions", func(c *gin.Context) {
		var req CreateAuctionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		captured = &req
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auctions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	return captured, w.Code, w.Body.String()
}

func TestCreateAuctionRequest_MissingShippingOptionIDs_Returns400(t *testing.T) {
	body := `{
		"title": "Showa Auction",
		"description": "A test auction",
		"start_price": 10000,
		"bid_increment": 1000,
		"start_mode": "now",
		"duration_hours": 24
	}`

	captured, code, respBody := bindCreateAuctionRequest(body)

	assert.Nil(t, captured, "binding must fail before the handler ever sees a populated request")
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, respBody, "ShippingOptionIDs")
}

func TestCreateAuctionRequest_EmptyShippingOptionIDs_Returns400(t *testing.T) {
	body := `{
		"title": "Showa Auction",
		"description": "A test auction",
		"shipping_option_ids": [],
		"start_price": 10000,
		"bid_increment": 1000,
		"start_mode": "now",
		"duration_hours": 24
	}`

	captured, code, respBody := bindCreateAuctionRequest(body)

	assert.Nil(t, captured)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, respBody, "ShippingOptionIDs")
}

func TestCreateAuctionRequest_ValidPayload_BindsShippingMediaAndVariety(t *testing.T) {
	body := `{
		"title": "Showa Auction",
		"description": "A test auction",
		"media_urls": ["https://cdn.example.com/a.jpg"],
		"variety": "showa",
		"size_cm": 28,
		"age_months": 18,
		"gender": "female",
		"shipping_option_ids": ["11111111-1111-1111-1111-111111111111"],
		"start_price": 10000,
		"bid_increment": 1000,
		"start_mode": "now",
		"duration_hours": 24
	}`

	captured, code, _ := bindCreateAuctionRequest(body)

	require.NotNil(t, captured, "valid payload must pass the binding boundary")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, []string{"11111111-1111-1111-1111-111111111111"}, captured.ShippingOptionIDs)
	assert.Equal(t, []string{"https://cdn.example.com/a.jpg"}, captured.MediaURLs)
	assert.Equal(t, "showa", captured.Variety)
	require.NotNil(t, captured.SizeCM)
	assert.Equal(t, 28, *captured.SizeCM)
	require.NotNil(t, captured.AgeMonths)
	assert.Equal(t, 18, *captured.AgeMonths)
	require.NotNil(t, captured.Gender)
	assert.Equal(t, "female", *captured.Gender)
}

func TestCreateAuctionRequest_MissingStartMode_Returns400(t *testing.T) {
	body := `{
		"title": "Showa Auction",
		"description": "A test auction",
		"shipping_option_ids": ["11111111-1111-1111-1111-111111111111"],
		"start_price": 10000,
		"bid_increment": 1000,
		"duration_hours": 24
	}`

	captured, code, _ := bindCreateAuctionRequest(body)

	assert.Nil(t, captured)
	assert.Equal(t, http.StatusBadRequest, code)
}
