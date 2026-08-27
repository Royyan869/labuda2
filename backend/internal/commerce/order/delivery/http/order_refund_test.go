package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

// ============================================================================
// TEST: CreateRefund - HTTP handler validation layer
// ============================================================================

// setupRefundTestContext creates a gin context for refund endpoint testing.
func setupRefundTestContext(orderID uuid.UUID, headers map[string]string, contextValues map[string]interface{}, body interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var reqBody *bytes.Buffer
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(bodyBytes)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest("POST", "/api/v1/orders/"+orderID.String()+"/refund", reqBody)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req

	for k, v := range contextValues {
		c.Set(k, v)
	}

	c.Params = gin.Params{gin.Param{Key: "id", Value: orderID.String()}}

	return c, w
}

func TestCreateRefund_InvalidOrderID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/v1/orders/not-a-uuid/refund", nil)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: "not-a-uuid"}}

	handler.CreateRefund(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateRefund_MissingIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	c, w := setupRefundTestContext(orderID, nil, map[string]interface{}{
		"userID": uuid.New(),
	}, map[string]interface{}{
		"reason": "item_damaged",
	})

	handler.CreateRefund(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Idempotency-Key")
}

func TestCreateRefund_Unauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	c, w := setupRefundTestContext(orderID, map[string]string{
		"Idempotency-Key": uuid.New().String(),
	}, nil, map[string]interface{}{
		"reason": "item_damaged",
	})

	handler.CreateRefund(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateRefund_InvalidUserIDType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	c, w := setupRefundTestContext(orderID, map[string]string{
		"Idempotency-Key": uuid.New().String(),
	}, map[string]interface{}{
		"userID": "not-a-uuid-object", // String instead of uuid.UUID
	}, map[string]interface{}{
		"reason": "item_damaged",
	})

	handler.CreateRefund(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateRefund_MissingReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	c, w := setupRefundTestContext(orderID, map[string]string{
		"Idempotency-Key": uuid.New().String(),
	}, map[string]interface{}{
		"userID": uuid.New(),
	}, map[string]interface{}{
		// No reason field
		"description": "test",
	})

	handler.CreateRefund(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateRefund_EvidenceMerge verifies that both "evidence_urls" and "evidence"
// fields are accepted and merged into the service input.
func TestCreateRefund_EvidenceMerge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// This test verifies request parsing succeeds. The handler panics at
	// h.db.WithTx (nil db) which proves it got past all validation checks.
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	body := map[string]interface{}{
		"reason":        "item_damaged",
		"evidence_urls": []string{"https://s3.example.com/vid1.mp4"},
		"evidence":      []string{"https://s3.example.com/photo1.jpg"},
	}

	c, _ := setupRefundTestContext(orderID, map[string]string{
		"Idempotency-Key": uuid.New().String(),
	}, map[string]interface{}{
		"userID": uuid.New(),
	}, body)

	// Panic at db.WithTx confirms parsing passed (400 errors return before tx)
	assert.Panics(t, func() { handler.CreateRefund(c) })
}

// ============================================================================
// TEST: CreateDispute - reason_code passthrough
// ============================================================================

func setupDisputeTestContext(orderID uuid.UUID, headers map[string]string, contextValues map[string]interface{}, body interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var reqBody *bytes.Buffer
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(bodyBytes)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest("POST", "/api/v1/orders/"+orderID.String()+"/dispute", reqBody)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req

	for k, v := range contextValues {
		c.Set(k, v)
	}

	c.Params = gin.Params{gin.Param{Key: "id", Value: orderID.String()}}

	return c, w
}

func TestCreateDispute_AcceptsReasonCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	body := map[string]interface{}{
		"reason":      "Barang rusak",
		"reason_code": "item_damaged",
	}

	c, _ := setupDisputeTestContext(orderID, map[string]string{
		"Idempotency-Key": uuid.New().String(),
	}, map[string]interface{}{
		"userID": uuid.New(),
	}, body)

	// Panic at db.WithTx confirms parsing passed
	assert.Panics(t, func() { handler.CreateDispute(c) })
}

func TestCreateDispute_AcceptsEvidenceURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	body := map[string]interface{}{
		"reason":        "Barang rusak",
		"evidence_urls": []string{"https://s3.example.com/vid1.mp4"},
		"video_url":     "https://s3.example.com/unboxing.mp4",
	}

	c, _ := setupDisputeTestContext(orderID, map[string]string{
		"Idempotency-Key": uuid.New().String(),
	}, map[string]interface{}{
		"userID": uuid.New(),
	}, body)

	// Panic at db.WithTx confirms parsing passed
	assert.Panics(t, func() { handler.CreateDispute(c) })
}

// ============================================================================
// TEST: C1B - Direct Dispute with Video
// ============================================================================

func TestCreateDispute_WithVideoURL_PassesValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	body := map[string]interface{}{
		"reason":      "Barang rusak",
		"reason_code": "item_damaged",
		"video_url":   "https://cdn.example.com/videos/unboxing.mp4",
		"description": "Barang rusak saat diterima",
	}

	c, _ := setupDisputeTestContext(orderID, map[string]string{
		"Idempotency-Key": uuid.New().String(),
	}, map[string]interface{}{
		"userID": uuid.New(),
	}, body)

	// Panic at db.WithTx confirms validation passed (400 errors return before tx)
	assert.Panics(t, func() { handler.CreateDispute(c) })
}

func TestCreateDispute_WithoutVideoOrEvidence_PassesHandlerValidation(t *testing.T) {
	// Handler does not enforce video — that's the service's job.
	// Handler passes whatever the client sends. This test confirms the handler
	// doesn't reject the request at the HTTP layer.
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	body := map[string]interface{}{
		"reason":      "Barang rusak",
		"reason_code": "item_damaged",
	}

	c, _ := setupDisputeTestContext(orderID, map[string]string{
		"Idempotency-Key": uuid.New().String(),
	}, map[string]interface{}{
		"userID": uuid.New(),
	}, body)

	// Panic at db.WithTx confirms handler-level validation passed
	assert.Panics(t, func() { handler.CreateDispute(c) })
}

func TestCreateDispute_MissingIdempotencyKey_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	body := map[string]interface{}{
		"reason":    "Barang rusak",
		"video_url": "https://cdn.example.com/videos/unboxing.mp4",
	}

	c, w := setupDisputeTestContext(orderID, nil, map[string]interface{}{
		"userID": uuid.New(),
	}, body)

	handler.CreateDispute(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Idempotency-Key")
}

func TestCreateDispute_Unauthenticated_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	body := map[string]interface{}{
		"reason":    "Barang rusak",
		"video_url": "https://cdn.example.com/videos/unboxing.mp4",
	}

	c, w := setupDisputeTestContext(orderID, map[string]string{
		"Idempotency-Key": uuid.New().String(),
	}, nil, body)

	handler.CreateDispute(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateDispute_MissingReason_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	body := map[string]interface{}{
		"video_url": "https://cdn.example.com/videos/unboxing.mp4",
	}

	c, w := setupDisputeTestContext(orderID, map[string]string{
		"Idempotency-Key": uuid.New().String(),
	}, map[string]interface{}{
		"userID": uuid.New(),
	}, body)

	handler.CreateDispute(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// TEST: C1B - Refund Escalation with Evidence (no separate video_url)
// ============================================================================

func TestCreateDispute_EscalationWithEvidenceOnly_PassesValidation(t *testing.T) {
	// When escalating from rejected refund, evidence_urls from original refund
	// are carried forward. Backend accepts evidence_urls as satisfying the video
	// requirement (dispute_service checks VideoURL OR len(evidenceURLs) > 0).
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, logger)

	orderID := uuid.New()
	body := map[string]interface{}{
		"reason":        "Barang rusak",
		"reason_code":   "item_damaged",
		"evidence_urls": []string{"https://cdn.example.com/videos/unboxing.mp4", "https://cdn.example.com/images/damage1.jpg"},
		// No video_url — evidence from refund is sufficient
	}

	c, _ := setupDisputeTestContext(orderID, map[string]string{
		"Idempotency-Key": uuid.New().String(),
	}, map[string]interface{}{
		"userID": uuid.New(),
	}, body)

	// Panic at db.WithTx confirms handler-level validation passed
	assert.Panics(t, func() { handler.CreateDispute(c) })
}

// Decision builder tests are in dto/decision_refund_test.go (same package as buildDecisionV2ForOrder).


