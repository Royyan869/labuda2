package entity

// Repost governance regression lock for ForSaleStatus.IsRepostable()
// and ForSale.MarkActiveFromModeration().
//
// FIX-1 (2026-05-28): validateForSaleTarget() in content_service now rejects
// any status that is not active. IsRepostable() is the single source of truth.
//
// FIX-5 (2026-05-28): ForSale.MarkActiveFromModeration() bypasses the state
// machine to restore moderation-withdrawn for_sales. Sold inventory is rejected.

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// TestForSaleStatus_IsRepostable pins the exact set of statuses that are/are not
// repostable. Any change to this set must be reviewed for feed/search SQL alignment.
func TestForSaleStatus_IsRepostable(t *testing.T) {
	cases := []struct {
		status     ForSaleStatus
		repostable bool
	}{
		{ForSaleStatusActive, true},
		{ForSaleStatusDraft, false},
		{ForSaleStatusSold, false},
		{ForSaleStatusWithdrawn, false},
		{ForSaleStatus("unknown"), false},
		{ForSaleStatus(""), false},
	}
	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			got := tc.status.IsRepostable()
			if got != tc.repostable {
				t.Errorf("ForSaleStatus(%q).IsRepostable() = %v; want %v", tc.status, got, tc.repostable)
			}
		})
	}
}

// TestForSale_MarkActiveFromModeration_WithdrawnRestored verifies that a
// withdrawn for_sale is successfully transitioned to active.
func TestForSale_MarkActiveFromModeration_WithdrawnRestored(t *testing.T) {
	l := newTestForSale(ForSaleStatusWithdrawn)
	err := l.MarkActiveFromModeration()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l.Status != ForSaleStatusActive {
		t.Errorf("expected status active after restore, got %q", l.Status)
	}
	if l.Visibility != ForSaleVisibilityPublic {
		t.Errorf("expected visibility public after restore, got %q", l.Visibility)
	}
}

// TestForSale_MarkActiveFromModeration_AlreadyActive verifies idempotency.
func TestForSale_MarkActiveFromModeration_AlreadyActive(t *testing.T) {
	l := newTestForSale(ForSaleStatusActive)
	l.Visibility = ForSaleVisibilityPublic
	err := l.MarkActiveFromModeration()
	if err != nil {
		t.Fatalf("unexpected error on already-active for_sale: %v", err)
	}
	if l.Status != ForSaleStatusActive {
		t.Errorf("expected status to remain active, got %q", l.Status)
	}
}

// TestForSale_MarkActiveFromModeration_SoldRejected verifies that sold
// for_sales (inventory claimed) cannot be restored via moderation.
func TestForSale_MarkActiveFromModeration_SoldRejected(t *testing.T) {
	l := newTestForSale(ForSaleStatusSold)
	err := l.MarkActiveFromModeration()
	if err == nil {
		t.Fatal("expected error for sold for_sale restore, got nil")
	}
}

// TestForSale_MarkActiveFromModeration_DraftRejected verifies that draft
// for_sales are not eligible for moderation restoration.
func TestForSale_MarkActiveFromModeration_DraftRejected(t *testing.T) {
	l := newTestForSale(ForSaleStatusDraft)
	err := l.MarkActiveFromModeration()
	if err == nil {
		t.Fatal("expected error for draft for_sale restore, got nil")
	}
}

// TestForSale_MarkActiveFromModeration_RepeatedRestore verifies idempotency
// when called twice on withdrawn for_sale.
func TestForSale_MarkActiveFromModeration_RepeatedRestore(t *testing.T) {
	l := newTestForSale(ForSaleStatusWithdrawn)
	if err := l.MarkActiveFromModeration(); err != nil {
		t.Fatalf("first restore failed: %v", err)
	}
	// Second call — for_sale is now active, should be a no-op.
	if err := l.MarkActiveFromModeration(); err != nil {
		t.Fatalf("second restore (idempotent) failed: %v", err)
	}
	if l.Status != ForSaleStatusActive {
		t.Errorf("expected active after repeat restore, got %q", l.Status)
	}
}

// newTestForSale creates a minimal ForSale with the given status for unit tests.
func newTestForSale(status ForSaleStatus) *ForSale {
	return &ForSale{
		ID:                uuid.New(),
		SellerID:          uuid.New(),
		Title:             "test",
		Status:            status,
		Visibility:        ForSaleVisibilityPrivate,
		QuantityAvailable: 1,
		PricePerUnit:      money.New(10000),
	}
}




