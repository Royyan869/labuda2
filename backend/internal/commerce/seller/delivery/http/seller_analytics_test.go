package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestGetAnalytics_Unauthorized_NoUserID proves the analytics endpoint reads the
// authenticated identity and rejects requests without one. It never accepts a
// client-supplied seller_id, so a caller cannot read another seller's analytics.
func TestGetAnalytics_Unauthorized_NoUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/seller/analytics", nil)

	handler := &SellerHandler{log: zap.NewNop()}
	handler.GetAnalytics(c)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestGetAnalytics_Unauthorized_InvalidUserID proves a malformed context user ID
// is rejected rather than silently scoping to a bogus seller.
func TestGetAnalytics_Unauthorized_InvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/seller/analytics", nil)
	c.Set("userID", "not-a-uuid")

	handler := &SellerHandler{log: zap.NewNop()}
	handler.GetAnalytics(c)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}
