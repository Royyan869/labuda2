package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CANONICAL AUCTION SETTLEMENT STATE MACHINE TESTS
//
// Locks the canonical settlement lifecycle:
//
//	waiting_settlement --payment success--> ended
//	waiting_settlement --settlement failure--> scheduled (auto-reschedule: all
//	settlement context cleared, start=now, end=now+previous duration)
//
// expired_bnr does NOT exist as a state. Settlement failure NEVER produces a
// new state — it auto-reschedules the same record (owner decision Oct 2026:
// no draft detour).
// ============================================================================

func waitingAuction() *Auction {
	a := createTestAuction()
	if err := a.Activate(); err != nil {
		panic(err) // test helper precondition
	}
	if err := a.TransitionToWaitingSettlement(); err != nil {
		panic(err) // test helper precondition
	}
	return a
}

func TestSettlement_WaitingToEnded_OnSuccess(t *testing.T) {
	a := waitingAuction()
	require.NoError(t, a.Settle())
	assert.Equal(t, StatusEnded, a.Status)
}

func TestSettlement_WaitingToScheduled_OnFailure(t *testing.T) {
	a := waitingAuction()
	require.NoError(t, a.RescheduleAfterSettlementFailure())
	assert.Equal(t, StatusScheduled, a.Status, "settlement failure auto-reschedules — never draft")
	assert.WithinDuration(t, time.Now(), a.StartAt, time.Minute, "rescheduled run starts now")
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), a.EndAt, time.Minute,
		"rescheduled run keeps the previous run's duration (helper: 24h)")
}

func TestSettlement_WaitingSettlement_IsNotTerminal(t *testing.T) {
	// A waiting_settlement auction must be able to auto-reschedule — the
	// canonical failure path — and to settle to ended — the success path.
	a := waitingAuction()
	require.NoError(t, a.RescheduleAfterSettlementFailure())
	assert.Equal(t, StatusScheduled, a.Status)

	b := waitingAuction()
	require.NoError(t, b.Settle())
	assert.Equal(t, StatusEnded, b.Status)
}

func TestSettlement_NoExpiredStateExists(t *testing.T) {
	// The enum must NOT contain an expired/BNR state: settlement failure
	// auto-reschedules, never to a dedicated expired state.
	for _, s := range []Status{StatusScheduled, StatusActive, StatusWaitingSettlement, StatusEnded, StatusCancelled, StatusLapsed} {
		switch s {
		case StatusScheduled, StatusActive, StatusWaitingSettlement, StatusEnded, StatusCancelled, StatusLapsed:
		default:
			t.Fatalf("unexpected status %q in canonical enum", s)
		}
	}
}

func TestReschedule_ClearsSettlementContext(t *testing.T) {
	a := waitingAuction()

	orderID := uuid.New()
	a.OrderID = &orderID

	now := time.Now()
	a.ShippingResolvedAt = &now
	a.SellerActionRequired = true
	a.SellerQuoteProvided = true

	bid := int64(250_000)
	winnerID := uuid.New()
	a.CurrentBid = &bid
	a.CurrentWinnerID = &winnerID

	require.NoError(t, a.RescheduleAfterSettlementFailure())

	// SCHEDULED reset contract: order binding, shipping resolution, seller
	// flags, current bid, and current winner are ALL cleared.
	assert.Equal(t, StatusScheduled, a.Status, "failure must reschedule, never draft")
	assert.Nil(t, a.OrderID, "OrderID must be nil after settlement failure")
	assert.Nil(t, a.ShippingResolvedAt, "ShippingResolvedAt must be nil after settlement failure")
	assert.False(t, a.SellerActionRequired, "SellerActionRequired must reset on reschedule")
	assert.False(t, a.SellerQuoteProvided, "SellerQuoteProvided must reset on reschedule (old quote never becomes relist authority)")
	assert.Nil(t, a.CurrentBid, "CurrentBid must be nil after settlement failure")
	assert.Nil(t, a.CurrentWinnerID, "CurrentWinnerID must be nil after settlement failure")
}

func TestRelist_StartsFromStartPrice(t *testing.T) {
	a := waitingAuction()

	bid := int64(1_500_000)
	a.CurrentBid = &bid
	winner := uuid.New()
	a.CurrentWinnerID = &winner

	require.NoError(t, a.RescheduleAfterSettlementFailure())

	// Republished on the SAME auction record: bidding restarts from start_price.
	assert.Equal(t, a.StartPrice, a.MinimumBid(),
		"MinimumBid() after reschedule must equal StartPrice (no current bid)")
}

func TestShippingResolved_FirstResolutionWins(t *testing.T) {
	a := waitingAuction()
	now := time.Now()
	require.NoError(t, a.ResolveShipping(now))

	// Second resolution attempt must be rejected.
	err := a.ResolveShipping(now.Add(time.Minute))
	assert.ErrorIs(t, err, ErrShippingAlreadyResolved)
	assert.Equal(t, now, *a.ShippingResolvedAt, "shipping_resolved_at must never be overwritten")
}

func TestSettlementDeadline_IsDerivedEndAtPlus24h(t *testing.T) {
	a := createTestAuction()
	endAt := time.Date(2026, 9, 1, 20, 0, 0, 0, time.UTC)
	a.EndAt = endAt

	// Deadline authority: auction.end_at + 24h — never stored, never extended.
	want := endAt.Add(24 * time.Hour)
	assert.Equal(t, want, a.SettlementDeadline())
}

func TestReschedule_FromNonWaiting_Rejected(t *testing.T) {
	a := createTestAuction()
	err := a.RescheduleAfterSettlementFailure()
	assert.Error(t, err, "settlement-failure reschedule is only valid from waiting_settlement")
	var ite *InvalidTransitionError
	assert.ErrorAs(t, err, &ite)
}

func TestSettle_FromNonWaiting_Rejected(t *testing.T) {
	a := createTestAuction() // scheduled — never waiting_settlement
	err := a.Settle()
	assert.Error(t, err)
}
