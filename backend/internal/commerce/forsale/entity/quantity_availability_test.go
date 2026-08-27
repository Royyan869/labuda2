package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// PASS_19E: locks IsAvailable()/ReduceQuantity() business rules that the
// repository-layer quantity persistence fix (PASS_19E) depends on. These are
// pure entity tests — no DB needed — proving the rules were always correct;
// only persistence was broken.

func newActiveForSale(quantity int) *ForSale {
	now := time.Now()
	return &ForSale{
		ID:                uuid.New(),
		ForSaleType:       ForSaleTypeFixedPrice,
		PricePerUnit:      money.New(100000),
		QuantityAvailable: quantity,
		Status:            ForSaleStatusActive,
		Visibility:        ForSaleVisibilityPublic,
		PublishedAt:       &now,
	}
}

func TestIsAvailable_ActiveWithZeroQuantity_NotAvailable(t *testing.T) {
	l := newActiveForSale(0)
	if l.IsAvailable() {
		t.Fatal("an active for_sale with quantity=0 must not be available")
	}
}

func TestIsAvailable_ActiveWithPositiveQuantity_Available(t *testing.T) {
	l := newActiveForSale(3)
	if !l.IsAvailable() {
		t.Fatal("an active for_sale with quantity>0 must be available")
	}
}

func TestIsAvailable_Withdrawn_NeverAvailableRegardlessOfQuantity(t *testing.T) {
	l := newActiveForSale(5) // plenty of stock
	l.Status = ForSaleStatusWithdrawn
	if l.IsAvailable() {
		t.Fatal("a withdrawn for_sale must never be available, regardless of quantity")
	}
}

func TestIsAvailable_Sold_NeverAvailableRegardlessOfQuantity(t *testing.T) {
	l := newActiveForSale(5)
	l.Status = ForSaleStatusSold
	if l.IsAvailable() {
		t.Fatal("a sold for_sale must never be available, regardless of quantity")
	}
}

func TestReduceQuantity_OversellAttempt_Rejected(t *testing.T) {
	l := newActiveForSale(1)
	err := l.ReduceQuantity(3)
	if err == nil {
		t.Fatal("expected InsufficientQuantityError")
	}
	insufficientErr, ok := err.(*InsufficientQuantityError)
	if !ok {
		t.Fatalf("expected *InsufficientQuantityError, got %T: %v", err, err)
	}
	if insufficientErr.Available != 1 || insufficientErr.Requested != 3 {
		t.Fatalf("got Available=%d Requested=%d, want Available=1 Requested=3",
			insufficientErr.Available, insufficientErr.Requested)
	}
	// Rejected reduction must not mutate quantity.
	if l.QuantityAvailable != 1 {
		t.Fatalf("QuantityAvailable = %d after rejected reduce, want unchanged 1", l.QuantityAvailable)
	}
}

func TestReduceQuantity_MultiUnit_ReducesToPartialStock(t *testing.T) {
	l := newActiveForSale(5)
	if err := l.ReduceQuantity(3); err != nil {
		t.Fatalf("ReduceQuantity(3) on qty=5: %v", err)
	}
	if l.QuantityAvailable != 2 {
		t.Fatalf("QuantityAvailable = %d, want 2", l.QuantityAvailable)
	}
	if l.Status != ForSaleStatusActive {
		t.Fatalf("Status = %s, want active (stock remains)", l.Status)
	}
}

func TestReduceQuantity_ExhaustsStock_TransitionsToSold(t *testing.T) {
	l := newActiveForSale(2)
	if err := l.ReduceQuantity(2); err != nil {
		t.Fatalf("ReduceQuantity(2) on qty=2: %v", err)
	}
	if l.QuantityAvailable != 0 {
		t.Fatalf("QuantityAvailable = %d, want 0", l.QuantityAvailable)
	}
	if l.Status != ForSaleStatusSold {
		t.Fatalf("Status = %s, want sold", l.Status)
	}
}

func TestRestoreQuantity_FromSold_RevivesToActive(t *testing.T) {
	l := newActiveForSale(0)
	l.Status = ForSaleStatusSold
	if err := l.RestoreQuantity(1); err != nil {
		t.Fatalf("RestoreQuantity(1): %v", err)
	}
	if l.QuantityAvailable != 1 {
		t.Fatalf("QuantityAvailable = %d, want 1", l.QuantityAvailable)
	}
	if l.Status != ForSaleStatusActive {
		t.Fatalf("Status = %s, want active after restore", l.Status)
	}
}
