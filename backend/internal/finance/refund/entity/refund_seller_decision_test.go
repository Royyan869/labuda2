package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// Entity-level tests for SellerApprove / SellerReject state transitions.
// No DB or service wiring needed — pure state machine validation.
// ============================================================================

func newTestRefund() *Refund {
	return NewRefund(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		RefundReasonItemDamaged,
		nil,
		100_000,
	)
}

func TestSellerApprove_HappyPath(t *testing.T) {
	r := newTestRefund()
	now := time.Now()
	notes := "looks valid, approved"
	if err := r.SellerApprove(100_000, &notes, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Status != RefundStatusSellerApproved {
		t.Fatalf("status=%s want seller_approved", r.Status)
	}
	if r.SellerApprovedAmount == nil || *r.SellerApprovedAmount != 100_000 {
		t.Fatalf("approved_amount=%v want 100000", r.SellerApprovedAmount)
	}
	if r.SellerApprovedPercent == nil || *r.SellerApprovedPercent != 100 {
		t.Fatalf("approved_percent=%v want 100", r.SellerApprovedPercent)
	}
	if r.SellerNotes == nil || *r.SellerNotes != notes {
		t.Fatalf("seller_notes=%v want %q", r.SellerNotes, notes)
	}
	if r.SellerReviewedAt == nil {
		t.Fatal("seller_reviewed_at should be set")
	}
	if r.ApprovedAt == nil {
		t.Fatal("approved_at should be set")
	}
}

func TestSellerApprove_PartialAmount(t *testing.T) {
	r := newTestRefund()
	now := time.Now()
	if err := r.SellerApprove(50_000, nil, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.SellerApprovedPercent == nil || *r.SellerApprovedPercent != 50 {
		t.Fatalf("approved_percent=%v want 50", r.SellerApprovedPercent)
	}
}

func TestSellerApprove_RejectsFromWrongStatus(t *testing.T) {
	statuses := []struct {
		name  string
		setup func() *Refund
	}{
		{"seller_rejected", func() *Refund {
			r := newTestRefund()
			_ = r.SellerReject(nil, time.Now())
			return r
		}},
		{"escalated_to_admin", func() *Refund {
			r := newTestRefund()
			_ = r.SellerReject(nil, time.Now())
			_ = r.EscalateToAdmin(time.Now())
			return r
		}},
		{"admin_released (terminal)", func() *Refund {
			r := newTestRefund()
			_ = r.SellerReject(nil, time.Now())
			_ = r.EscalateToAdmin(time.Now())
			_ = r.AdminRelease(uuid.New(), nil, time.Now())
			return r
		}},
	}
	for _, tc := range statuses {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.setup()
			err := r.SellerApprove(100_000, nil, time.Now())
			if err == nil {
				t.Fatalf("expected error for status=%s", r.Status)
			}
		})
	}
}

func TestSellerReject_HappyPath(t *testing.T) {
	r := newTestRefund()
	now := time.Now()
	notes := "item was delivered in good condition"
	if err := r.SellerReject(&notes, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Status != RefundStatusSellerRejected {
		t.Fatalf("status=%s want seller_rejected", r.Status)
	}
	if r.SellerNotes == nil || *r.SellerNotes != notes {
		t.Fatalf("seller_notes=%v want %q", r.SellerNotes, notes)
	}
	if r.SellerReviewedAt == nil {
		t.Fatal("seller_reviewed_at should be set")
	}
	if r.RejectedAt == nil {
		t.Fatal("rejected_at should be set")
	}
	// Seller approved fields should NOT be set
	if r.SellerApprovedAmount != nil {
		t.Fatal("seller_approved_amount should be nil on rejection")
	}
	if r.ApprovedAt != nil {
		t.Fatal("approved_at should be nil on rejection")
	}
}

func TestSellerReject_RejectsFromWrongStatus(t *testing.T) {
	r := newTestRefund()
	_ = r.SellerApprove(100_000, nil, time.Now())

	err := r.SellerReject(nil, time.Now())
	if err == nil {
		t.Fatal("expected error: cannot reject an already-approved refund")
	}
}

func TestSellerReject_ThenEscalate_Works(t *testing.T) {
	r := newTestRefund()
	now := time.Now()
	if err := r.SellerReject(nil, now); err != nil {
		t.Fatalf("reject error: %v", err)
	}
	if !r.CanEscalate() {
		t.Fatal("should be able to escalate after seller rejection")
	}
	if err := r.EscalateToAdmin(now); err != nil {
		t.Fatalf("escalate error: %v", err)
	}
	if r.Status != RefundStatusEscalatedToAdmin {
		t.Fatalf("status=%s want escalated_to_admin", r.Status)
	}
}

// CANONICAL CONTRACT: seller ACCEPT is the FINAL refund decision. There is no
// later decision state — gateway settlement is a separate axis and never
// rewrites the decision.
func TestSellerApprove_IsFinalDecision(t *testing.T) {
	r := newTestRefund()
	now := time.Now()
	if err := r.SellerApprove(100_000, nil, now); err != nil {
		t.Fatalf("approve error: %v", err)
	}
	if r.Status != RefundStatusSellerApproved {
		t.Fatalf("status=%s want seller_approved", r.Status)
	}
	if !r.IsDecisionFinal() {
		t.Fatal("seller_approved must be a FINAL refund decision")
	}
	if !r.OwesBuyerRefund() {
		t.Fatal("seller_approved owes the buyer a refund")
	}
	// Still unsubmitted at the gateway: settlement is pending, so the order must
	// not release money to the seller yet.
	if !r.IsSettlementPending() {
		t.Fatal("unsubmitted gateway settlement must count as pending")
	}
	if !r.BlocksOrderRelease(true) || !r.BlocksOrderRelease(false) {
		t.Fatal("unsettled buyer-owed refund must block release regardless of window")
	}
	// Once the gateway settles, the refund no longer blocks release.
	if err := r.MarkGatewayDispatched("idem-1", nil, now); err != nil {
		t.Fatalf("gateway dispatch error: %v", err)
	}
	if err := r.MarkGatewayAckSucceeded("gw-1", now); err != nil {
		t.Fatalf("gateway ack error: %v", err)
	}
	if r.IsSettlementPending() {
		t.Fatal("succeeded gateway settlement must not be pending")
	}
	if r.BlocksOrderRelease(true) {
		t.Fatal("settled refund must not block release")
	}
}

// TestBlocksOrderRelease_CanonicalTable locks the single predicate every
// consumer uses (order completion guard, auto-complete worker query, read path).
func TestBlocksOrderRelease_CanonicalTable(t *testing.T) {
	now := time.Now()
	escalated := func() *Refund {
		r := newTestRefund()
		_ = r.SellerReject(nil, now)
		_ = r.EscalateToAdmin(now)
		return r
	}
	adminReleased := func() *Refund {
		r := escalated()
		_ = r.AdminRelease(uuid.New(), nil, now)
		return r
	}

	cases := []struct {
		name        string
		refund      *Refund
		windowOpen  bool
		wantBlock   bool
		wantFinal   bool
	}{
		{"pending review, window open", newTestRefund(), true, true, false},
		{"pending review, window closed (order lifecycle ends it)", newTestRefund(), false, false, false},
		{"seller rejected, window open", func() *Refund { r := newTestRefund(); _ = r.SellerReject(nil, now); return r }(), true, true, false},
		{"seller rejected, window closed (no escalation)", func() *Refund { r := newTestRefund(); _ = r.SellerReject(nil, now); return r }(), false, false, false},
		{"escalated to admin, window open", escalated(), true, true, false},
		{"admin released (seller wins)", adminReleased(), true, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.refund.BlocksOrderRelease(tc.windowOpen); got != tc.wantBlock {
				t.Fatalf("BlocksOrderRelease(%v) = %v, want %v (status=%s)", tc.windowOpen, got, tc.wantBlock, tc.refund.Status)
			}
			if got := tc.refund.IsDecisionFinal(); got != tc.wantFinal {
				t.Fatalf("IsDecisionFinal() = %v, want %v (status=%s)", got, tc.wantFinal, tc.refund.Status)
			}
		})
	}
}

// CANONICAL CONTRACT: an admin buyer-wins decision is FINAL, recorded on the
// SAME refund row that was escalated, and settlement stays separate.
func TestAdminRefund_FinalDecisionOnSameRow(t *testing.T) {
	r := newTestRefund()
	now := time.Now()
	_ = r.SellerReject(nil, now)
	_ = r.EscalateToAdmin(now)
	adminID := uuid.New()
	if err := r.AdminRefund(adminID, 100_000, nil, now); err != nil {
		t.Fatalf("admin refund error: %v", err)
	}
	if r.Status != RefundStatusAdminRefunded {
		t.Fatalf("status=%s want admin_refunded", r.Status)
	}
	if !r.IsDecisionFinal() || r.AwaitsDecision() {
		t.Fatal("admin_refunded must be a final decision")
	}
	if r.ReviewedBy != adminID {
		t.Fatal("admin attribution must be recorded on the escalated row")
	}
	if !r.BlocksOrderRelease(true) {
		t.Fatal("unsettled admin refund decision must block release")
	}
}

// TestSystemRefund_IsFinalPlatformDecision locks the separation between an
// admin decision on an escalated refund and a platform-initiated refund.
func TestSystemRefund_IsFinalPlatformDecision(t *testing.T) {
	r := NewSystemRefund(uuid.New(), uuid.New(), uuid.New(), uuid.New(), RefundReasonOther, 100_000, 0, nil)
	if r.Status != RefundStatusSystemRefunded {
		t.Fatalf("status=%s want system_refunded", r.Status)
	}
	if !r.IsDecisionFinal() || !r.OwesBuyerRefund() {
		t.Fatal("a system refund is a final, buyer-owed decision")
	}
}

func TestSellerApprove_Idempotency_RejectsDoubleApprove(t *testing.T) {
	r := newTestRefund()
	now := time.Now()
	_ = r.SellerApprove(100_000, nil, now)

	// Second approve should fail (already seller_approved, not pending)
	err := r.SellerApprove(100_000, nil, now)
	if err == nil {
		t.Fatal("expected error on double approve")
	}
}

func TestSellerReject_Idempotency_RejectsDoubleReject(t *testing.T) {
	r := newTestRefund()
	now := time.Now()
	_ = r.SellerReject(nil, now)

	// Second reject should fail (already seller_rejected, not pending)
	err := r.SellerReject(nil, now)
	if err == nil {
		t.Fatal("expected error on double reject")
	}
}

func TestSellerApprove_RejectsTerminalRefund(t *testing.T) {
	r := newTestRefund()
	_ = r.SellerReject(nil, time.Now())
	_ = r.EscalateToAdmin(time.Now())
	_ = r.AdminRelease(uuid.New(), nil, time.Now())

	err := r.SellerApprove(100_000, nil, time.Now())
	if err == nil {
		t.Fatal("expected ErrAlreadyResolved for terminal refund")
	}
	if _, ok := err.(*ErrAlreadyResolved); !ok {
		t.Fatalf("expected *ErrAlreadyResolved, got %T", err)
	}
}

func TestSellerReject_RejectsTerminalRefund(t *testing.T) {
	r := newTestRefund()
	_ = r.SellerReject(nil, time.Now())
	_ = r.EscalateToAdmin(time.Now())
	_ = r.AdminRelease(uuid.New(), nil, time.Now())

	err := r.SellerReject(nil, time.Now())
	if err == nil {
		t.Fatal("expected ErrAlreadyResolved for terminal refund")
	}
	if _, ok := err.(*ErrAlreadyResolved); !ok {
		t.Fatalf("expected *ErrAlreadyResolved, got %T", err)
	}
}


