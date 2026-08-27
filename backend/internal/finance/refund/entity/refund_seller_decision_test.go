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
		name   string
		setup  func() *Refund
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

func TestSellerApprove_ThenCompleteRefund_Works(t *testing.T) {
	r := newTestRefund()
	now := time.Now()
	if err := r.SellerApprove(100_000, nil, now); err != nil {
		t.Fatalf("approve error: %v", err)
	}
	if err := r.CompleteRefund(100_000, now); err != nil {
		t.Fatalf("complete error: %v", err)
	}
	if r.Status != RefundStatusRefunded {
		t.Fatalf("status=%s want refunded", r.Status)
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


