package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/refund/entity"
)

// ============================================================================
// Unit tests for SellerRefundHandler HTTP layer.
// Tests auth extraction, error mapping, and response building.
// No DB required — these validate handler-level behavior only.
// ============================================================================

func init() {
	gin.SetMode(gin.TestMode)
}

func TestExtractUserID_Missing(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	_, ok := extractUserID(c)
	if ok {
		t.Fatal("expected false when userID missing from context")
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", w.Code)
	}
}

func TestExtractUserID_WrongType(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	c.Set("userID", "not-a-uuid")

	_, ok := extractUserID(c)
	if ok {
		t.Fatal("expected false when userID is wrong type")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", w.Code)
	}
}

func TestExtractUserID_Valid(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	uid := uuid.New()
	c.Set("userID", uid)

	got, ok := extractUserID(c)
	if !ok {
		t.Fatal("expected true for valid userID")
	}
	if got != uid {
		t.Fatalf("userID=%s want %s", got, uid)
	}
}

func TestBuildRefundResponse_AllFields(t *testing.T) {
	refund := entity.NewRefund(uuid.New(), uuid.New(), uuid.New(), entity.RefundReasonItemDamaged, nil, 50_000)
	resp := buildRefundResponse(refund)

	if resp["status"] != string(entity.RefundStatusPendingSellerReview) {
		t.Fatalf("status=%v want pending_seller_review", resp["status"])
	}
	if resp["requested_amount"] != int64(50_000) {
		t.Fatalf("requested_amount=%v want 50000", resp["requested_amount"])
	}
	// Optional fields should NOT be present
	if _, exists := resp["seller_approved_amount"]; exists {
		t.Fatal("seller_approved_amount should not be present for pending refund")
	}
	if _, exists := resp["approved_at"]; exists {
		t.Fatal("approved_at should not be present for pending refund")
	}
}

func TestBuildRefundResponse_WithSellerDecision(t *testing.T) {
	refund := entity.NewRefund(uuid.New(), uuid.New(), uuid.New(), entity.RefundReasonItemDamaged, nil, 50_000)
	notes := "approved"
	_ = refund.SellerApprove(50_000, &notes, refund.CreatedAt)

	resp := buildRefundResponse(refund)

	if resp["status"] != string(entity.RefundStatusSellerApproved) {
		t.Fatalf("status=%v want seller_approved", resp["status"])
	}
	if resp["seller_approved_amount"] != int64(50_000) {
		t.Fatalf("seller_approved_amount=%v want 50000", resp["seller_approved_amount"])
	}
	if resp["seller_notes"] != "approved" {
		t.Fatalf("seller_notes=%v want 'approved'", resp["seller_notes"])
	}
	if _, exists := resp["approved_at"]; !exists {
		t.Fatal("approved_at should be present for approved refund")
	}
}

func TestApproveRefund_InvalidRefundID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	c.Params = gin.Params{gin.Param{Key: "id", Value: "not-a-uuid"}}
	c.Set("userID", uuid.New())

	handler := NewSellerRefundHandler(nil, nil, nil)
	handler.ApproveRefund(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", w.Code)
	}
}

func TestRejectRefund_InvalidRefundID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	c.Params = gin.Params{gin.Param{Key: "id", Value: "not-a-uuid"}}
	c.Set("userID", uuid.New())

	handler := NewSellerRefundHandler(nil, nil, nil)
	handler.RejectRefund(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", w.Code)
	}
}

func TestApproveRefund_NoAuth(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	c.Params = gin.Params{gin.Param{Key: "id", Value: uuid.New().String()}}
	// No userID set in context

	handler := NewSellerRefundHandler(nil, nil, nil)
	handler.ApproveRefund(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", w.Code)
	}
}

func TestRejectRefund_NoAuth(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	c.Params = gin.Params{gin.Param{Key: "id", Value: uuid.New().String()}}
	// No userID set in context

	handler := NewSellerRefundHandler(nil, nil, nil)
	handler.RejectRefund(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", w.Code)
	}
}

func TestHandleError_OwnershipForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewSellerRefundHandler(nil, nil, nil)
	handler.handleError(c, fmt.Errorf("only the seller of this order can approve the refund"), uuid.New(), uuid.New(), "approve")

	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", w.Code)
	}
}

func TestHandleError_InvalidState(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewSellerRefundHandler(nil, nil, nil)
	handler.handleError(c, fmt.Errorf("failed to reject refund: invalid refund status transition: seller_approved -> seller_rejected"), uuid.New(), uuid.New(), "reject")

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409", w.Code)
	}
}

func TestHandleError_AlreadyResolved(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewSellerRefundHandler(nil, nil, nil)
	handler.handleError(c, fmt.Errorf("failed to approve refund: refund already resolved: some-id (status: admin_released)"), uuid.New(), uuid.New(), "approve")

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409", w.Code)
	}
}

func TestHandleError_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewSellerRefundHandler(nil, nil, nil)
	handler.handleError(c, fmt.Errorf("refund not found: sql: no rows"), uuid.New(), uuid.New(), "approve")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", w.Code)
	}
}

func TestHandleError_AdminReviewRequired(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewSellerRefundHandler(nil, nil, nil)
	handler.handleError(c, fmt.Errorf(`refund reason "item_not_as_described" requires admin review; seller cannot auto-approve`), uuid.New(), uuid.New(), "approve")

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422", w.Code)
	}
}


