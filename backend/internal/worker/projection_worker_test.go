package worker

import (
	"testing"

	"github.com/labuda/backend/internal/platform/events"
	"github.com/labuda/backend/internal/projection"
)

// ─── event classification ─────────────────────────────────────────────────────

func TestIsOrderEvent(t *testing.T) {
	cases := []struct {
		eventType string
		want      bool
	}{
		{events.EventOrderCreated, true},
		{events.EventOrderPaid, true},
		{"order.shipped", true},
		{events.EventOrderCompleted, true},
		{"order.cancelled", true},
		{"order.expired", true},
		{"order.refunded", true},
		{"order.partially_refunded", true},
		// non-order events must not match
		{"dispute.opened", false},
		{"dispute.resolved", false},
		{"ledger.transaction.completed", false},
		{"user.banned", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isOrderEvent(tc.eventType); got != tc.want {
			t.Errorf("isOrderEvent(%q) = %v, want %v", tc.eventType, got, tc.want)
		}
	}
}

func TestIsDisputeEvent(t *testing.T) {
	cases := []struct {
		eventType string
		want      bool
	}{
		{"dispute.opened", true},
		{"dispute.resolved", true},
		// non-dispute events must not match
		{events.EventOrderCreated, false},
		{"ledger.transaction.completed", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isDisputeEvent(tc.eventType); got != tc.want {
			t.Errorf("isDisputeEvent(%q) = %v, want %v", tc.eventType, got, tc.want)
		}
	}
}

func TestIsLedgerEvent(t *testing.T) {
	cases := []struct {
		eventType string
		want      bool
	}{
		{"ledger.transaction.completed", true},
		// non-ledger events must not match
		{events.EventOrderCreated, false},
		{"dispute.opened", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isLedgerEvent(tc.eventType); got != tc.want {
			t.Errorf("isLedgerEvent(%q) = %v, want %v", tc.eventType, got, tc.want)
		}
	}
}

// ─── event routing ────────────────────────────────────────────────────────────

// TestIsXxxEvent_Disjoint verifies the three classification predicates are
// mutually exclusive: no event type belongs to more than one category.
func TestIsXxxEvent_Disjoint(t *testing.T) {
	allTypes := []string{
		events.EventOrderCreated, events.EventOrderPaid, "order.shipped",
		events.EventOrderCompleted, "order.cancelled", "order.expired",
		"order.refunded", "order.partially_refunded",
		"dispute.opened", "dispute.resolved",
		"ledger.transaction.completed",
		"user.banned", "notification.sent", "",
	}
	for _, et := range allTypes {
		count := 0
		if isOrderEvent(et) {
			count++
		}
		if isDisputeEvent(et) {
			count++
		}
		if isLedgerEvent(et) {
			count++
		}
		if count > 1 {
			t.Errorf("event type %q matches multiple categories (count=%d)", et, count)
		}
	}
}

// ─── struct field presence (compile-time guard) ───────────────────────────────

// TestOrderSummary_EscrowFields verifies that EscrowAmount and RefundedAmount
// are present on projection.OrderSummary with the correct type (int64).
// If either field is removed or renamed, this test fails to compile.
func TestOrderSummary_EscrowFields(t *testing.T) {
	s := projection.OrderSummary{
		EscrowAmount:   100_000,
		RefundedAmount: 0,
	}
	if s.EscrowAmount != 100_000 {
		t.Fatalf("EscrowAmount = %d, want 100000", s.EscrowAmount)
	}
	if s.RefundedAmount != 0 {
		t.Fatalf("RefundedAmount = %d, want 0", s.RefundedAmount)
	}
}

// TestOrderSummary_DisputeFields verifies that dispute nullable fields are
// present on projection.OrderSummary.
func TestOrderSummary_DisputeFields(t *testing.T) {
	status := "open"
	reason := "item_not_received"
	s := projection.OrderSummary{
		DisputeStatus: &status,
		DisputeReason: &reason,
	}
	if s.DisputeStatus == nil || *s.DisputeStatus != status {
		t.Fatal("DisputeStatus field missing or wrong value")
	}
	if s.DisputeReason == nil || *s.DisputeReason != reason {
		t.Fatal("DisputeReason field missing or wrong value")
	}
	// nil timestamps are valid (no dispute opened/resolved yet)
	if s.DisputeOpenedAt != nil {
		t.Fatal("expected DisputeOpenedAt nil by default")
	}
	if s.DisputeResolvedAt != nil {
		t.Fatal("expected DisputeResolvedAt nil by default")
	}
}


