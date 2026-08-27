package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	disputeEntity "github.com/labuda/backend/internal/governance/dispute/entity"
	"github.com/labuda/backend/internal/finance/refund/entity"
)

// ============================================================================
// Unit tests for BuyerEscalationHandler HTTP layer (H2-B).
//
// Tests auth extraction, error mapping, reason-code mapping, and response
// building. No DB required — these validate handler-level behavior only.
// ============================================================================

// ============================================================================
// TEST: Invalid refund ID → 400
// ============================================================================

func TestEscalateRefund_InvalidRefundID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	c.Params = gin.Params{gin.Param{Key: "id", Value: "not-a-uuid"}}
	c.Set("userID", uuid.New())

	handler := NewBuyerEscalationHandler(nil, nil, nil, nil)
	handler.EscalateRefund(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", w.Code)
	}
}

// ============================================================================
// TEST: No auth → 401
// ============================================================================

func TestEscalateRefund_NoAuth(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	c.Params = gin.Params{gin.Param{Key: "id", Value: uuid.New().String()}}
	// No userID in context

	handler := NewBuyerEscalationHandler(nil, nil, nil, nil)
	handler.EscalateRefund(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", w.Code)
	}
}

// ============================================================================
// TEST: Seller cannot escalate → 403
// ============================================================================

func TestEscalateError_SellerForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewBuyerEscalationHandler(nil, nil, nil, nil)
	handler.handleError(c, fmt.Errorf("only the buyer can escalate this refund"), uuid.New(), uuid.New())

	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", w.Code)
	}
}

// ============================================================================
// TEST: Wrong buyer cannot escalate → 403
// ============================================================================

func TestEscalateError_WrongBuyerForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewBuyerEscalationHandler(nil, nil, nil, nil)
	handler.handleError(c, fmt.Errorf("only the buyer can escalate this refund"), uuid.New(), uuid.New())

	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", w.Code)
	}
}

// ============================================================================
// TEST: Pending refund cannot escalate → 409
// ============================================================================

func TestEscalateError_PendingStatus(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewBuyerEscalationHandler(nil, nil, nil, nil)
	handler.handleError(c, fmt.Errorf("failed to escalate refund: invalid refund status transition: pending_seller_review -> escalated_to_admin"), uuid.New(), uuid.New())

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409", w.Code)
	}
}

// ============================================================================
// TEST: Already escalated (duplicate dispute) → 409
// ============================================================================

func TestEscalateError_AlreadyEscalated(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewBuyerEscalationHandler(nil, nil, nil, nil)
	handler.handleError(c, fmt.Errorf("cannot open dispute: order already has an active dispute"), uuid.New(), uuid.New())

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409", w.Code)
	}
}

// ============================================================================
// TEST: Already resolved → 409
// ============================================================================

func TestEscalateError_AlreadyResolved(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewBuyerEscalationHandler(nil, nil, nil, nil)
	handler.handleError(c, fmt.Errorf("failed to escalate refund: refund already resolved: some-id (status: admin_released)"), uuid.New(), uuid.New())

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409", w.Code)
	}
}

// ============================================================================
// TEST: Refund not found → 404
// ============================================================================

func TestEscalateError_RefundNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewBuyerEscalationHandler(nil, nil, nil, nil)
	handler.handleError(c, fmt.Errorf("refund not found: sql: no rows"), uuid.New(), uuid.New())

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", w.Code)
	}
}

// ============================================================================
// TEST: Escrow not holding → 409
// ============================================================================

func TestEscalateError_EscrowNotHolding(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)

	handler := NewBuyerEscalationHandler(nil, nil, nil, nil)
	handler.handleError(c, fmt.Errorf("cannot escalate: escrow not in holding state"), uuid.New(), uuid.New())

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409", w.Code)
	}
}

// ============================================================================
// TEST: Reason code mapping (all 8 refund reasons)
// ============================================================================

func TestMapRefundReasonToDisputeCode(t *testing.T) {
	tests := []struct {
		reason entity.RefundReason
		want   string
	}{
		{entity.RefundReasonItemNotReceived, disputeEntity.ReasonCodeItemNotReceived},
		{entity.RefundReasonItemNotAsDescribed, disputeEntity.ReasonCodeItemNotAsDescribed},
		{entity.RefundReasonItemDamaged, disputeEntity.ReasonCodeShippingDamage},
		{entity.RefundReasonDefectiveItem, disputeEntity.ReasonCodeOther},
		{entity.RefundReasonWrongItem, disputeEntity.ReasonCodeOther},
		{entity.RefundReasonChangeOfMind, disputeEntity.ReasonCodeOther},
		{entity.RefundReasonDeliveryDelay, disputeEntity.ReasonCodeOther},
		{entity.RefundReasonOther, disputeEntity.ReasonCodeOther},
	}

	for _, tt := range tests {
		got := mapRefundReasonToDisputeCode(tt.reason)
		if got != tt.want {
			t.Errorf("mapRefundReasonToDisputeCode(%s)=%s want %s", tt.reason, got, tt.want)
		}
	}
}


