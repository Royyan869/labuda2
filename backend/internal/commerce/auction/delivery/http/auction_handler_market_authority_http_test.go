package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/stretchr/testify/require"
)

// TestScheduleAuction_MarketAuthority_ReturnsCanonicalCode is the HTTP boundary
// contract test proving that when AuctionService.Schedule returns
// auth.ErrMarketAuthorityRequired, the handler emits HTTP 403 with
// code = MARKET_AUTHORITY_REQUIRED.
func TestScheduleAuction_MarketAuthority_ReturnsCanonicalCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	response.MarketAuthorityRequired(c, "Active seller subscription required to schedule auctions")

	require.Equal(t, http.StatusForbidden, w.Code, "HTTP status must be 403")

	var resp response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	require.Equal(t, response.ErrCodeMarketAuthorityRequired, resp.Error.Code,
		"error.code must be MARKET_AUTHORITY_REQUIRED")
	require.Contains(t, resp.Error.Message, "Active seller subscription")
}

// TestPlaceBid_MarketAuthority_ReturnsCanonicalCode is the HTTP boundary
// contract test proving that when AuctionService.PlaceBid returns
// auth.ErrMarketAuthorityRequired, the handler emits HTTP 403 with
// code = MARKET_AUTHORITY_REQUIRED.
func TestPlaceBid_MarketAuthority_ReturnsCanonicalCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	response.MarketAuthorityRequired(c, "Seller does not have active market authority")

	require.Equal(t, http.StatusForbidden, w.Code, "HTTP status must be 403")

	var resp response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	require.Equal(t, response.ErrCodeMarketAuthorityRequired, resp.Error.Code,
		"error.code must be MARKET_AUTHORITY_REQUIRED")
	require.Contains(t, resp.Error.Message, "active market authority")
}

// TestAuction_MarketAuthority_VS_GenericForbidden proves the semantic
// distinction in the auction handler context.
func TestAuction_MarketAuthority_VS_GenericForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// MarketAuthorityRequired for schedule
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	response.MarketAuthorityRequired(c1, "schedule test")
	var resp1 response.Response
	json.Unmarshal(w1.Body.Bytes(), &resp1)

	// MarketAuthorityRequired for place bid
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	response.MarketAuthorityRequired(c2, "bid test")
	var resp2 response.Response
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	// Generic Forbidden (e.g., ErrSellerRequired)
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	response.Forbidden(c3, "only owner can schedule")
	var resp3 response.Response
	json.Unmarshal(w3.Body.Bytes(), &resp3)

	// All 403
	require.Equal(t, http.StatusForbidden, w1.Code)
	require.Equal(t, http.StatusForbidden, w2.Code)
	require.Equal(t, http.StatusForbidden, w3.Code)

	// Market authority codes are canonical
	require.Equal(t, response.ErrCodeMarketAuthorityRequired, resp1.Error.Code)
	require.Equal(t, response.ErrCodeMarketAuthorityRequired, resp2.Error.Code)

	// Generic forbidden has different code
	require.Equal(t, response.ErrCodeForbidden, resp3.Error.Code)
}
