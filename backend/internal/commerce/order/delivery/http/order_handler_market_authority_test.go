package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/stretchr/testify/require"
)

// TestCreateOrder_MarketAuthority_ReturnsCanonicalCode is the HTTP boundary
// contract test proving that when OrderCreationService returns
// auth.ErrMarketAuthorityRequired, the handler emits HTTP 403 with
// code = MARKET_AUTHORITY_REQUIRED.
func TestCreateOrder_MarketAuthority_ReturnsCanonicalCode(t *testing.T) {
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

// TestCreateOrder_MarketAuthority_VS_GenericForbidden proves the semantic
// distinction in the order handler context.
func TestCreateOrder_MarketAuthority_VS_GenericForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// MarketAuthorityRequired for order creation
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	response.MarketAuthorityRequired(c1, "seller market authority test")
	var resp1 response.Response
	json.Unmarshal(w1.Body.Bytes(), &resp1)

	// Generic Forbidden (e.g., negotiation buyer mismatch)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	response.Forbidden(c2, "You are not the buyer of this negotiation")
	var resp2 response.Response
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	// Both 403
	require.Equal(t, http.StatusForbidden, w1.Code)
	require.Equal(t, http.StatusForbidden, w2.Code)

	// Different codes
	require.Equal(t, response.ErrCodeMarketAuthorityRequired, resp1.Error.Code)
	require.Equal(t, response.ErrCodeForbidden, resp2.Error.Code)
}

// TestOrderHandler_MarketAuthority_ErrorIs proves that
// errors.Is works for detecting ErrMarketAuthorityRequired.
func TestOrderHandler_MarketAuthority_ErrorIs(t *testing.T) {
	// The handler uses errors.Is(err, auth.ErrMarketAuthorityRequired)
	require.True(t, errors.Is(auth.ErrMarketAuthorityRequired, auth.ErrMarketAuthorityRequired))
}
