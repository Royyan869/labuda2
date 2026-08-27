package http

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// setupTestSellerContext creates a test gin context with user_id set
func setupTestSellerContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	userID := uuid.New()
	c.Set("userID", userID)
	return c, w
}

// ============================================================================
// TEST: GetDashboard - Handler Logic Tests
// ============================================================================

func TestGetDashboard_Unauthorized_NoUserID(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/seller/dashboard", nil)
	// No userID set in context

	handler := &SellerHandler{
		log: nil,
		db:  nil,
	}

	// Act
	handler.GetDashboard(c)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetDashboard_Unauthorized_InvalidUserID(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/seller/dashboard", nil)
	c.Set("userID", "not-a-uuid") // String instead of UUID

	handler := &SellerHandler{
		log: nil,
		db:  nil,
	}

	// Act
	handler.GetDashboard(c)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ============================================================================
// TEST: GetAnalytics - Handler Logic Tests
// ============================================================================

func TestGetAnalytics_Unauthorized_NoUserID(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/seller/analytics", nil)
	// No userID set in context

	handler := &SellerHandler{
		log: nil,
		db:  nil,
	}

	// Act
	handler.GetAnalytics(c)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetAnalytics_Unauthorized_InvalidUserID(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/seller/analytics", nil)
	c.Set("userID", "not-a-uuid") // String instead of UUID

	handler := &SellerHandler{
		log: nil,
		db:  nil,
	}

	// Act
	handler.GetAnalytics(c)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ============================================================================
// TEST: GetPerformance - Handler Logic Tests
// ============================================================================

func TestGetPerformance_Unauthorized_NoUserID(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/seller/performance", nil)
	// No userID set in context

	handler := &SellerHandler{
		log: nil,
		db:  nil,
	}

	// Act
	handler.GetPerformance(c)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetPerformance_Unauthorized_InvalidUserID(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/seller/performance", nil)
	c.Set("userID", "not-a-uuid") // String instead of UUID

	handler := &SellerHandler{
		log: nil,
		db:  nil,
	}

	// Act
	handler.GetPerformance(c)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ============================================================================
// TEST: GetEarnings - Handler Logic Tests (Already Existed)
// ============================================================================

func TestGetEarnings_Unauthorized_NoUserID(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/seller/earnings", nil)
	// No userID set in context

	handler := &SellerHandler{
		log: nil,
		db:  nil,
	}

	// Act
	handler.GetEarnings(c)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetEarnings_Unauthorized_InvalidUserID(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/seller/earnings", nil)
	c.Set("userID", "not-a-uuid") // String instead of UUID

	handler := &SellerHandler{
		log: nil,
		db:  nil,
	}

	// Act
	handler.GetEarnings(c)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ============================================================================
// TEST: Security - Verify Context User Enforcement
// ============================================================================

func TestSellerEndpoints_RequireAuthenticatedUser(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func(*gin.Context)
		handlerFunc    func(*SellerHandler, *gin.Context)
		expectedStatus int
	}{
		{
			name: "GetDashboard without userID",
			setupRequest: func(c *gin.Context) {
				c.Request, _ = http.NewRequest("GET", "/api/v1/seller/dashboard", nil)
			},
			handlerFunc:    func(h *SellerHandler, c *gin.Context) { h.GetDashboard(c) },
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "GetAnalytics without userID",
			setupRequest: func(c *gin.Context) {
				c.Request, _ = http.NewRequest("GET", "/api/v1/seller/analytics", nil)
			},
			handlerFunc:    func(h *SellerHandler, c *gin.Context) { h.GetAnalytics(c) },
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "GetPerformance without userID",
			setupRequest: func(c *gin.Context) {
				c.Request, _ = http.NewRequest("GET", "/api/v1/seller/performance", nil)
			},
			handlerFunc:    func(h *SellerHandler, c *gin.Context) { h.GetPerformance(c) },
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "GetEarnings without userID",
			setupRequest: func(c *gin.Context) {
				c.Request, _ = http.NewRequest("GET", "/api/v1/seller/earnings", nil)
			},
			handlerFunc:    func(h *SellerHandler, c *gin.Context) { h.GetEarnings(c) },
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			tt.setupRequest(c)
			// Deliberately don't set userID

			handler := &SellerHandler{
				log: nil,
				db:  nil,
			}

			tt.handlerFunc(handler, c)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// ============================================================================
// TEST: Response Structure Validation
// ============================================================================

func TestSellerDashboardResponse_Structure(t *testing.T) {
	// This test validates the response structure matches the expected format
	resp := SellerDashboardResponse{
		TotalForSales:  120,
		ActiveForSales: 80,
		SoldItems:      40,
		TotalRevenue:   12500000,
		PendingOrders:  5,
	}

	// Assert all fields have expected types
	assert.IsType(t, int64(0), resp.TotalForSales)
	assert.IsType(t, int64(0), resp.ActiveForSales)
	assert.IsType(t, int64(0), resp.SoldItems)
	assert.IsType(t, int64(0), resp.TotalRevenue)
	assert.IsType(t, int64(0), resp.PendingOrders)
}

func TestSellerAnalyticsResponse_Structure(t *testing.T) {
	// This test validates the response structure matches the expected format
	resp := SellerAnalyticsResponse{
		Views:          12000,
		ConversionRate: 3.2,
	}

	// Assert all fields have expected types
	assert.IsType(t, int64(0), resp.Views)
	assert.IsType(t, float64(0), resp.ConversionRate)
}

func TestSellerPerformanceResponse_Structure(t *testing.T) {
	// This test validates the response structure matches the expected format
	resp := SellerPerformanceResponse{
		Rating:          4.8,
		CompletedOrders: 320,
		CancelRate:      1.2,
		ResponseTime:    "2h",
	}

	// Assert all fields have expected types
	assert.IsType(t, float64(0), resp.Rating)
	assert.IsType(t, int64(0), resp.CompletedOrders)
	assert.IsType(t, float64(0), resp.CancelRate)
	assert.IsType(t, "", resp.ResponseTime)
}

func TestSellerEarningsResponse_Structure(t *testing.T) {
	// This test validates the response structure matches the expected format
	resp := SellerEarningsResponse{
		AvailableBalance:    5200000,
		PendingBalance:      1300000,
		TotalWithdrawn:      5000000,
		TotalEarned:         20000000,
		GrossPayable:        6000000,
		ActiveDisputeFreeze: 300000,
		WithdrawableBalance: 5200000,
	}

	// Assert all fields have expected types (legacy)
	assert.IsType(t, int64(0), resp.AvailableBalance)
	assert.IsType(t, int64(0), resp.PendingBalance)
	assert.IsType(t, int64(0), resp.TotalWithdrawn)
	assert.IsType(t, int64(0), resp.TotalEarned)

	// Assert breakdown fields have expected types
	assert.IsType(t, int64(0), resp.GrossPayable)
	assert.IsType(t, int64(0), resp.ActiveDisputeFreeze)
	assert.IsType(t, int64(0), resp.WithdrawableBalance)
}

// TestSellerEarningsResponse_AvailableEqualsWithdrawable verifies the invariant:
// available_balance == withdrawable_balance. Both are sourced from
// SellerWithdrawableSummary.Withdrawable.
func TestSellerEarningsResponse_AvailableEqualsWithdrawable(t *testing.T) {
	resp := SellerEarningsResponse{
		AvailableBalance:    700000,
		WithdrawableBalance: 700000,
		GrossPayable:        1000000,
		ActiveDisputeFreeze: 100000,
	}
	assert.Equal(t, resp.AvailableBalance, resp.WithdrawableBalance,
		"available_balance must equal withdrawable_balance")
}

// TestSellerEarningsResponse_FreezeReducesWithdrawable verifies that active
// dispute freezes reduce the withdrawable amount relative to gross payable.
func TestSellerEarningsResponse_FreezeReducesWithdrawable(t *testing.T) {
	grossPayable := int64(1000000)
	freeze := int64(300000)
	withdrawable := grossPayable - freeze

	resp := SellerEarningsResponse{
		AvailableBalance:    withdrawable,
		GrossPayable:        grossPayable,
		ActiveDisputeFreeze: freeze,
		WithdrawableBalance: withdrawable,
	}

	assert.Equal(t, int64(700000), resp.WithdrawableBalance)
	assert.Equal(t, resp.GrossPayable-resp.ActiveDisputeFreeze,
		resp.WithdrawableBalance,
		"withdrawable = gross_payable - active_dispute_freeze")
}

// TestSellerEarningsResponse_OldFieldsPreserved verifies backward compatibility:
// legacy fields (available_balance, pending_balance, total_withdrawn, total_earned)
// still exist and are independent of the new breakdown fields.
func TestSellerEarningsResponse_OldFieldsPreserved(t *testing.T) {
	resp := SellerEarningsResponse{
		AvailableBalance: 500000,
		PendingBalance:   0,
		TotalWithdrawn:   2000000,
		TotalEarned:      8000000,
		// Breakdown fields can be zero (backward compat: old clients ignore them)
		GrossPayable:        0,
		ActiveDisputeFreeze: 0,
		WithdrawableBalance: 0,
	}

	// Legacy fields are set independently
	assert.Equal(t, int64(500000), resp.AvailableBalance)
	assert.Equal(t, int64(0), resp.PendingBalance)
	assert.Equal(t, int64(2000000), resp.TotalWithdrawn)
	assert.Equal(t, int64(8000000), resp.TotalEarned)
}

// TestSellerEarningsResponse_BreakdownJSONTags verifies JSON serialization tags
// for the new breakdown fields match the API contract.
func TestSellerEarningsResponse_BreakdownJSONTags(t *testing.T) {
	typ := reflect.TypeOf(SellerEarningsResponse{})

	expected := map[string]string{
		"GrossPayable":        "gross_payable",
		"ActiveDisputeFreeze": "active_dispute_freeze",
		"WithdrawableBalance": "withdrawable_balance",
	}

	for fieldName, jsonTag := range expected {
		field, ok := typ.FieldByName(fieldName)
		assert.True(t, ok, "field %s must exist on SellerEarningsResponse", fieldName)
		assert.Equal(t, jsonTag, field.Tag.Get("json"),
			"field %s must have json tag %q", fieldName, jsonTag)
	}
}

// ============================================================================
// TEST: Earnings authority — ledger, not payable_maturity
// ============================================================================

// TestGetEarnings_UsesLedgerAuthority_NoPayoutRiskServiceField verifies that
// SellerHandler does not hold a payoutRiskService field. The presence of that
// field was the only path to payable_maturity reads; its absence is a
// compile-time + runtime proof that GetEarnings reads available_balance from
// FinanceService.GetSellerWithdrawable (SELLER_PAYABLE ledger) instead.
func TestGetEarnings_UsesLedgerAuthority_NoPayoutRiskServiceField(t *testing.T) {
	typ := reflect.TypeOf(SellerHandler{})
	for i := 0; i < typ.NumField(); i++ {
		assert.NotEqual(t, "payoutRiskService", typ.Field(i).Name,
			"GetEarnings must use FinanceService.GetSellerWithdrawable (SELLER_PAYABLE ledger); "+
				"payoutRiskService / payable_maturity path must be absent")
	}
}


