package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
)

// ============================================================================
// CANONICAL SETTLEMENT DEADLINE AUTHORITY TESTS
//
// The settlement window is DERIVED — auction.end_at + 24h — and never stored
// or extended. The single deadline predicate (Auction.SettlementDeadlinePassed)
// is shared by every enforcement point: the advisory pricing-preview check and
// the authoritative POST /orders bid-win re-check under the row lock.
// ============================================================================

func TestSettlementDeadline_DerivedFromEndAtOnly(t *testing.T) {
	// Business truth: quote/timing events after auction end do NOT move the
	// deadline. It is derived purely from end_at: seller-quote activity at
	// T+1h / T+23h / T+23h59m all keep the SAME settlement deadline T+24h.
	endAt := time.Date(2026, 9, 1, 20, 0, 0, 0, time.UTC)
	want := endAt.Add(24 * time.Hour)

	a := &entity.Auction{EndAt: endAt}
	a.SellerActionRequired = true
	a.SellerQuoteProvided = true

	if got := a.SettlementDeadline(); !got.Equal(want) {
		t.Fatalf("SettlementDeadline() = %v, want %v (must stay end_at+24h regardless of quote timing)", got, want)
	}
}

func TestSettlementDeadlinePassed_Boundary(t *testing.T) {
	sellerID := uuid.New()
	endAt := time.Date(2026, 9, 1, 20, 0, 0, 0, time.UTC)
	a := newAuctionForUpdateAuthority(entity.StatusWaitingSettlement, sellerID)
	a.EndAt = endAt
	winner := uuid.New()
	a.CurrentWinnerID = &winner
	bid := int64(1_200_000)
	a.CurrentBid = &bid

	deadline := a.SettlementDeadline() // endAt + 24h

	t.Run("accepted just before deadline (deadline - 1s)", func(t *testing.T) {
		if a.SettlementDeadlinePassed(deadline.Add(-time.Second)) {
			t.Fatal("must not be passed 1s before the derived deadline")
		}
	})

	t.Run("accepted exactly at the deadline instant", func(t *testing.T) {
		// now.After(deadline) is false AT the deadline — same semantics every
		// enforcement point shares via the single predicate.
		if a.SettlementDeadlinePassed(deadline) {
			t.Fatal("must not be passed exactly AT the derived deadline")
		}
	})

	t.Run("rejected after deadline (deadline + 1s)", func(t *testing.T) {
		if !a.SettlementDeadlinePassed(deadline.Add(time.Second)) {
			t.Fatal("must be passed 1s after the derived deadline")
		}
	})
}
