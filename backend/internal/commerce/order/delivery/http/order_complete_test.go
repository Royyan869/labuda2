package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap/zaptest"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TEST: CompleteOrder
// ============================================================================

// setupTestContext creates a gin context with proper parameter extraction
func setupTestContext(method, url string, orderID uuid.UUID, headers map[string]string, contextValues map[string]interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create request with URL containing order ID
	req := httptest.NewRequest(method, url+"/"+orderID.String()+"/complete", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req

	// Set context values (like userID)
	for k, v := range contextValues {
		c.Set(k, v)
	}

	// Set params manually to simulate route parameter extraction
	c.Params = gin.Params{gin.Param{Key: "id", Value: orderID.String()}}

	return c, w
}

// TestCompleteOrder_Validation tests the request validation layer of CompleteOrder
// These tests verify:
// - Invalid order ID returns 400
// - Missing Idempotency-Key header returns 400
// - Unauthenticated user returns 401
func TestCompleteOrder_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)

	// Create a minimal handler with nil services (validation happens before service calls)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	t.Run("error - invalid order ID", func(t *testing.T) {
		idempotencyKey := uuid.New().String()
		buyerID := uuid.New()
		invalidOrderID := uuid.UUID{} // Empty/invalid UUID

		c, w := setupTestContext("POST", "/api/v1/orders", invalidOrderID, map[string]string{
			"Idempotency-Key": idempotencyKey,
		}, map[string]interface{}{
			"userID": buyerID,
		})

		// Override the param to test invalid UUID
		c.Params = gin.Params{gin.Param{Key: "id", Value: "not-a-uuid"}}

		handler.CompleteOrder(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "Invalid order ID")
	})

	t.Run("error - missing Idempotency-Key header", func(t *testing.T) {
		orderID := uuid.New()
		buyerID := uuid.New()

		c, w := setupTestContext("POST", "/api/v1/orders", orderID, map[string]string{}, map[string]interface{}{
			"userID": buyerID,
		})

		handler.CompleteOrder(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "Idempotency-Key header required")
	})

	t.Run("error - unauthenticated user", func(t *testing.T) {
		orderID := uuid.New()
		idempotencyKey := uuid.New().String()

		c, w := setupTestContext("POST", "/api/v1/orders", orderID, map[string]string{
			"Idempotency-Key": idempotencyKey,
		}, map[string]interface{}{}) // NO userID set

		handler.CompleteOrder(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "User not authenticated")
	})
}

// TestCompleteOrder_InvalidUserID tests invalid userID type in context
func TestCompleteOrder_InvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)

	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	t.Run("error - invalid user ID type in context", func(t *testing.T) {
		orderID := uuid.New()
		idempotencyKey := uuid.New().String()

		c, w := setupTestContext("POST", "/api/v1/orders", orderID, map[string]string{
			"Idempotency-Key": idempotencyKey,
		}, map[string]interface{}{
			"userID": "not-a-uuid", // Wrong type
		})

		handler.CompleteOrder(c)

		// Handler returns 500 for invalid userID type in context
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}


