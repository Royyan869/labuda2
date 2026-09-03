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
//	waiting_settlement --settlement failure--> draft (all settlement context cleared)
//
// expired_bnr does NOT exist as a state. Settlement failure NEVER produces a
// new state — it returns to DRAFT.
// ============================================================================

func waitingAuction() *Auction {
	a := createTestDraftAuction()
	if err := a.Schedule(); err != nil {
		panic(err) // test helper precondition
	}
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

func TestSettlement_WaitingToDraft_OnFailure(t *testing.T) {
	a := waitingAuction()
	require.NoError(t, a.TransitionToDraftOnSettlementFailure())
	assert.Equal(t, StatusDraft, a.Status)
}

func TestSettlement_WaitingSettlement_IsNotTerminal(t *testing.T) {
	// A waiting_settlement auction must be able to return to draft — the
	// canonical failure path — and to settle to ended — the success path.
	a := waitingAuction()
	require.NoError(t, a.TransitionToDraftOnSettlementFailure())
	assert.Equal(t, StatusDraft, a.Status)

	b := waitingAuction()
	require.NoError(t, b.Settle())
	assert.Equal(t, StatusEnded, b.Status)
}

func TestSettlement_NoExpiredStateExists(t *testing.T) {
	// The enum must NOT contain an expired/BNR state: settlement failure
	// returns to draft, never to a dedicated expired state.
	for _, s := range []Status{StatusDraft, StatusScheduled, StatusActive, StatusWaitingSettlement, StatusEnded, StatusCancelled} {
		switch s {
		case StatusDraft, StatusScheduled, StatusActive, StatusWaitingSettlement, StatusEnded, StatusCancelled:
		default:
			t.Fatalf("unexpected status %q in canonical enum", s)
		}
	}
}

func TestTransitionToDraft_ClearsSettlementContext(t *testing.T) {
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

	require.NoError(t, a.TransitionToDraftOnSettlementFailure())

	// DRAFT reset contract: order binding, shipping resolution, seller flags,
	// current bid, and current winner are ALL cleared.
	assert.Nil(t, a.OrderID, "OrderID must be nil after settlement failure")
	assert.Nil(t, a.ShippingResolvedAt, "ShippingResolvedAt must be nil after settlement failure")
	assert.False(t, a.SellerActionRequired, "SellerActionRequired must reset on DRAFT")
	assert.False(t, a.SellerQuoteProvided, "SellerQuoteProvided must reset on DRAFT (old quote never becomes relist authority)")
	assert.Nil(t, a.CurrentBid, "CurrentBid must be nil after settlement failure")
	assert.Nil(t, a.CurrentWinnerID, "CurrentWinnerID must be nil after settlement failure")
}

func TestRelist_StartsFromStartPrice(t *testing.T) {
	a := waitingAuction()

	bid := int64(1_500_000)
	a.CurrentBid = &bid
	winner := uuid.New()
	a.CurrentWinnerID = &winner

	require.NoError(t, a.TransitionToDraftOnSettlementFailure())

	// Relist on the SAME auction record: bidding restarts from start_price.
	assert.Equal(t, a.StartPrice, a.MinimumBid(),
		"MinimumBid() after relist must equal StartPrice (no current bid)")
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
	a := createTestDraftAuction()
	endAt := time.Date(2026, 9, 1, 20, 0, 0, 0, time.UTC)
	a.EndAt = endAt

	// Deadline authority: auction.end_at + 24h — never stored, never extended.
	want := endAt.Add(24 * time.Hour)
	assert.Equal(t, want, a.SettlementDeadline())
}

func TestTransitionToDraft_FromNonWaiting_Rejected(t *testing.T) {
	a := createTestDraftAuction()
	err := a.TransitionToDraftOnSettlementFailure()
	assert.Error(t, err, "settlement-failure DRAFT return is only valid from waiting_settlement")
	var ite *InvalidTransitionError
	assert.ErrorAs(t, err, &ite)
}

func TestSettle_FromNonWaiting_Rejected(t *testing.T) {
	a := createTestDraftAuction() // draft
	err := a.Settle()
	assert.Error(t, err)
}
