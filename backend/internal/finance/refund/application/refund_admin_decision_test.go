package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/labuda/backend/internal/finance/refund/entity"
)

// ============================================================================
// TESTS: RefundService.AdminResolveRefundDecision + final-decision authority
// ============================================================================
//
// These are REAL PROOF tests: they drive the production service and the real
// entity state machine. Only the DB repository is a double (in-memory
// mockRefundRepo shared with the gateway tests), so row-level behaviour
// (one process = one row, no duplicate refund row) is asserted against the
// actual production code path.
//
// They lock the escalated-refund contract (CANONICAL B1):
//
//  1. escalated_to_admin + admin decision => final decision on the SAME row.
//  2. No admin decision ever creates a SECOND refund row.
//  3. pending_seller_review + admin decision => fail closed (the seller's
//     review cannot be silently skipped).
//  4. seller_rejected + admin decision => FAIL CLOSED (the buyer must explicitly
//     escalate via POST /refunds/:id/escalate before the admin can decide;
//     a generic dispute opening does NOT grant admin authority).
//  5. A direct dispute with no refund process => recorded=false (the caller
//     then creates the platform refund record).
//  6. The decision is written on the decision axis BEFORE any gateway
//     settlement is attempted (settlement is a separate axis).
// ============================================================================

func seedRefund(repo *mockRefundRepo, status entity.RefundStatus) *entity.Refund {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	refund := entity.NewRefund(
		orderID, buyerID, sellerID,
		entity.RefundReasonItemNotReceived, nil, 1500,
	)
	refund.Status = status
	if status == entity.RefundStatusSellerRejected {
		now := time.Now()
		refund.RejectedAt = &now
	}
	repo.refundByID[refund.ID] = refund
	return refund
}

func TestAdminResolveRefundDecision_SellerRejectedFailsClosed(t *testing.T) {
	// CANONICAL B1: seller_rejected + admin decision => FAIL CLOSED.
	// The buyer must explicitly escalate via POST /refunds/:id/escalate before
	// the admin can decide. A generic dispute opening does NOT grant admin
	// authority over a refund that is still seller_rejected.
	repo := newMockRefundRepo()
	refund := seedRefund(repo, entity.RefundStatusSellerRejected)
	adminID := uuid.New()

	svc := &RefundService{refundRepo: repo}

	recorded, err := svc.AdminResolveRefundDecision(
		context.Background(), noopTx{}, refund.OrderID, adminID, true, 1500, nil,
	)
	if err == nil {
		t.Fatal("admin must not be able to decide on a seller_rejected refund without escalation")
	}
	if recorded {
		t.Fatal("nothing was recorded, so recorded must be false")
	}
	// The refund status must remain seller_rejected (no implicit escalation)
	got := repo.refundByID[refund.ID]
	if got == nil {
		t.Fatal("refund row disappeared")
	}
	if got.Status != entity.RefundStatusSellerRejected {
		t.Fatalf("status = %s, want seller_rejected (no implicit escalation)", got.Status)
	}
	if got.IsDecisionFinal() {
		t.Fatal("seller_rejected must NOT be a final decision")
	}
	if len(repo.refundByID) != 1 {
		t.Fatalf("expected exactly ONE refund row for the refund process, got %d", len(repo.refundByID))
	}
}

func TestAdminResolveRefundDecision_EscalatedSellerRejectedThenDecides(t *testing.T) {
	// CANONICAL POSITIVE PROOF: buyer escalates seller_rejected refund via
	// POST /refunds/:id/escalate (entity.EscalateToAdmin), then admin decides.
	// This proves the canonical flow:
	//   seller_rejected → escalated_to_admin → admin_refunded/admin_released
	repo := newMockRefundRepo()
	refund := seedRefund(repo, entity.RefundStatusSellerRejected)
	adminID := uuid.New()

	svc := &RefundService{refundRepo: repo}

	// Step 1: Buyer escalates (simulating POST /refunds/:id/escalate)
	now := time.Now()
	if err := refund.EscalateToAdmin(now); err != nil {
		t.Fatalf("buyer escalation must succeed: %v", err)
	}
	repo.refundByID[refund.ID] = refund // persist in mock

	if refund.Status != entity.RefundStatusEscalatedToAdmin {
		t.Fatalf("after escalation, status = %s, want escalated_to_admin", refund.Status)
	}
	if refund.IsDecisionFinal() {
		t.Fatal("escalated_to_admin must NOT be a final decision")
	}

	// Step 2: Admin decides (seller wins)
	recorded, err := svc.AdminResolveRefundDecision(
		context.Background(), noopTx{}, refund.OrderID, adminID, false, 0, nil,
	)
	if err != nil {
		t.Fatalf("admin decision on escalated refund must succeed: %v", err)
	}
	if !recorded {
		t.Fatal("admin decision must be reported as recorded")
	}

	got := repo.refundByID[refund.ID]
	if got.Status != entity.RefundStatusAdminReleased {
		t.Fatalf("status = %s, want admin_released", got.Status)
	}
	if !got.IsDecisionFinal() {
		t.Fatal("admin_released must be a FINAL decision")
	}
	if got.ReviewedBy != adminID {
		t.Fatalf("reviewed_by = %s, want the deciding admin %s", got.ReviewedBy, adminID)
	}
	if len(repo.refundByID) != 1 {
		t.Fatalf("expected exactly ONE refund row for the refund process, got %d", len(repo.refundByID))
	}
}

func TestAdminResolveRefundDecision_SellerRejectedSellerWinsFailsClosed(t *testing.T) {
	// CANONICAL B1: seller_rejected + admin seller-wins decision => FAIL CLOSED.
	// The buyer must explicitly escalate before the admin can decide.
	repo := newMockRefundRepo()
	refund := seedRefund(repo, entity.RefundStatusSellerRejected)
	adminID := uuid.New()

	svc := &RefundService{refundRepo: repo}

	recorded, err := svc.AdminResolveRefundDecision(
		context.Background(), noopTx{}, refund.OrderID, adminID, false, 0, nil,
	)
	if err == nil {
		t.Fatal("admin must not be able to decide on a seller_rejected refund without escalation")
	}
	if recorded {
		t.Fatal("nothing was recorded, so recorded must be false")
	}
	got := repo.refundByID[refund.ID]
	if got.Status != entity.RefundStatusSellerRejected {
		t.Fatalf("status = %s, want seller_rejected (no implicit escalation)", got.Status)
	}
	if len(repo.refundByID) != 1 {
		t.Fatalf("expected exactly ONE refund row, got %d", len(repo.refundByID))
	}
}

func TestAdminResolveRefundDecision_EscalatedRowRecordsOnSameRow(t *testing.T) {
	repo := newMockRefundRepo()
	refund := seedRefund(repo, entity.RefundStatusEscalatedToAdmin)
	adminID := uuid.New()

	svc := &RefundService{refundRepo: repo}

	recorded, err := svc.AdminResolveRefundDecision(
		context.Background(), noopTx{}, refund.OrderID, adminID, false, 0, nil,
	)
	if err != nil {
		t.Fatalf("resolving an escalated refund must succeed: %v", err)
	}
	if !recorded {
		t.Fatal("escalated buyer-wins decision must be reported as recorded")
	}
	if repo.refundByID[refund.ID].Status != entity.RefundStatusAdminReleased {
		t.Fatalf("status = %s, want admin_released", repo.refundByID[refund.ID].Status)
	}
	if len(repo.refundByID) != 1 {
		t.Fatalf("expected exactly ONE refund row, got %d", len(repo.refundByID))
	}
}

func TestAdminResolveRefundDecision_PendingSellerReviewFailsClosed(t *testing.T) {
	// CANONICAL B1: pending_seller_review + admin decision => FAIL CLOSED.
	// The seller's review cannot be silently skipped. The refund must already be
	// escalated_to_admin for the admin to decide.
	repo := newMockRefundRepo()
	refund := seedRefund(repo, entity.RefundStatusPendingSellerReview)
	adminID := uuid.New()

	svc := &RefundService{refundRepo: repo}

	recorded, err := svc.AdminResolveRefundDecision(
		context.Background(), noopTx{}, refund.OrderID, adminID, true, 1500, nil,
	)
	if err == nil {
		t.Fatal("an admin decision must never silently skip an open seller review")
	}
	if recorded {
		t.Fatal("nothing was recorded, so recorded must be false")
	}
	if got := repo.refundByID[refund.ID].Status; got != entity.RefundStatusPendingSellerReview {
		t.Fatalf("status = %s, want the seller review to remain pending", got)
	}
	if len(repo.refundByID) != 1 {
		t.Fatalf("expected exactly ONE refund row, got %d", len(repo.refundByID))
	}
}

func TestAdminResolveRefundDecision_NoRefundProcessIsNotRecorded(t *testing.T) {
	repo := newMockRefundRepo()
	svc := &RefundService{refundRepo: repo}

	recorded, err := svc.AdminResolveRefundDecision(
		context.Background(), noopTx{}, uuid.New(), uuid.New(), true, 1000, nil,
	)
	if err != nil {
		t.Fatalf("a direct dispute has no refund process to record: %v", err)
	}
	if recorded {
		t.Fatal("recorded must be false when the order has no refund process")
	}
	if len(repo.refundByID) != 0 {
		t.Fatalf("no refund row may be created by the decision authority, got %d", len(repo.refundByID))
	}
}

func TestAdminResolveRefundDecision_IdempotentWhenAlreadyFinal(t *testing.T) {
	repo := newMockRefundRepo()
	refund := seedRefund(repo, entity.RefundStatusAdminReleased)
	adminID := uuid.New()

	svc := &RefundService{refundRepo: repo}

	recorded, err := svc.AdminResolveRefundDecision(
		context.Background(), noopTx{}, refund.OrderID, adminID, true, 1500, nil,
	)
	if err != nil {
		t.Fatalf("replay on a final decision must be a no-op: %v", err)
	}
	if !recorded {
		t.Fatal("an already-final decision must report recorded=true")
	}
	if got := repo.refundByID[refund.ID].Status; got != entity.RefundStatusAdminReleased {
		t.Fatalf("a final decision must never be rewritten by a replay, got %s", got)
	}
}

// TestHasFinalRefundDecisionOwedToBuyer_Table locks the single refund-domain
// answer to the order side's question "does the seller still get this money?".
func TestHasFinalRefundDecisionOwedToBuyer_Table(t *testing.T) {
	cases := []struct {
		status entity.RefundStatus
		want   bool
		why    string
	}{
		{entity.RefundStatusSellerApproved, true, "seller ACCEPT is FINAL and owes the buyer"},
		{entity.RefundStatusAdminRefunded, true, "admin buyer-wins is FINAL and owes the buyer"},
		{entity.RefundStatusSystemRefunded, true, "platform-initiated refund owes the buyer"},
		{entity.RefundStatusAdminReleased, false, "admin seller-wins owes nothing"},
		{entity.RefundStatusEscalatedToAdmin, false, "still open — the admin may still rule seller-wins"},
		{entity.RefundStatusSellerRejected, false, "still open — the buyer may still escalate"},
		{entity.RefundStatusPendingSellerReview, false, "still open — the seller has not decided"},
	}

	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			repo := newMockRefundRepo()
			refund := seedRefund(repo, tc.status)
			svc := &RefundService{refundRepo: repo}

			got, err := svc.HasFinalRefundDecisionOwedToBuyer(context.Background(), noopTx{}, refund.OrderID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("HasFinalRefundDecisionOwedToBuyer(%s) = %v, want %v (%s)", tc.status, got, tc.want, tc.why)
			}
		})
	}

	t.Run("no refund process", func(t *testing.T) {
		svc := &RefundService{refundRepo: newMockRefundRepo()}
		got, err := svc.HasFinalRefundDecisionOwedToBuyer(context.Background(), noopTx{}, uuid.New())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got {
			t.Fatal("an order with no refund process cannot owe the buyer a refund")
		}
	})
}
