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

// TestCreateContract_MarketAuthority_ReturnsCanonicalCode is the HTTP boundary
// contract test proving that when PromotionContractService.Create returns
// auth.ErrMarketAuthorityRequired, the handler emits HTTP 403 with
// code = MARKET_AUTHORITY_REQUIRED.
func TestCreateContract_MarketAuthority_ReturnsCanonicalCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	response.MarketAuthorityRequired(c, "Active seller subscription required to create a promotion contract")

	require.Equal(t, http.StatusForbidden, w.Code, "HTTP status must be 403")

	var resp response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	require.Equal(t, response.ErrCodeMarketAuthorityRequired, resp.Error.Code,
		"error.code must be MARKET_AUTHORITY_REQUIRED")
	require.Contains(t, resp.Error.Message, "Active seller subscription")
}

// TestContractHandler_MarketAuthority_VS_GenericForbidden proves the semantic
// distinction in the contract handler context.
func TestContractHandler_MarketAuthority_VS_GenericForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// MarketAuthorityRequired for contract creation
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	response.MarketAuthorityRequired(c1, "contract creation test")
	var resp1 response.Response
	json.Unmarshal(w1.Body.Bytes(), &resp1)

	// Generic Forbidden (e.g., CONTRACT_NOT_OWNED)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	response.Forbidden(c2, "not your contract")
	var resp2 response.Response
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	// Both 403
	require.Equal(t, http.StatusForbidden, w1.Code)
	require.Equal(t, http.StatusForbidden, w2.Code)

	// Different codes
	require.Equal(t, response.ErrCodeMarketAuthorityRequired, resp1.Error.Code)
	require.Equal(t, response.ErrCodeForbidden, resp2.Error.Code)
}
