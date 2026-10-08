package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// endedNoBidAuction builds the exact shape produced by EndAuctionInternal for
// an auction that ran to completion with nobody bidding: status ended, no bid,
// no winner, no bound order.
func endedNoBidAuction(t *testing.T) *Auction {
	t.Helper()

	a := createTestAuction()
	a.Status = StatusActive
	require.NoError(t, a.End())
	require.Equal(t, StatusEnded, a.Status)

	return a
}

// endedAuctionWithOutcome builds an ended auction that carries the outcome of
// its run, which must never be relistable.
func endedAuctionWithOutcome(t *testing.T) *Auction {
	t.Helper()

	a := endedNoBidAuction(t)

	bid := int64(250_000)
	winner := uuid.New()
	orderID := uuid.New()
	a.CurrentBid = &bid
	a.CurrentWinnerID = &winner
	a.OrderID = &orderID

	return a
}

// POSITIVE PROOF: relist is REPUBLISH — an auction that ended with no bids
// goes STRAIGHT TO SCHEDULED (no draft detour, owner decision Oct 2026) with
// the caller-supplied run timing/pricing, and every field of the finished
// lifecycle cleared.
func TestRelist_EndedNoBid_RepublishesToScheduled(t *testing.T) {
	a := endedNoBidAuction(t)

	// Dirty every field a no-bid ended auction may still carry so the reset is
	// observable. Bid/winner/order are nil by construction — that is exactly
	// what makes the auction relistable — and are asserted below.
	shippingResolvedAt := time.Now()
	a.ShippingResolvedAt = &shippingResolvedAt
	a.SellerActionRequired = true
	a.SellerQuoteProvided = true
	a.AntiSnipeExtensionTotal = MaxAntiSnipingTotalExtension

	startAt := time.Now().Add(time.Hour)
	endAt := startAt.Add(48 * time.Hour)
	require.NoError(t, a.Relist(startAt, endAt, 20_000, 2_000, nil))

	assert.Equal(t, StatusScheduled, a.Status, "relist is republish — scheduled, never draft")
	assert.Equal(t, startAt, a.StartAt, "relist commits the create-form timing")
	assert.Equal(t, endAt, a.EndAt)
	assert.Equal(t, int64(20_000), a.StartPrice, "relist commits the create-form pricing")
	assert.Equal(t, int64(2_000), a.BidIncrement)
	assert.Nil(t, a.BuyNowPrice)
	assert.Nil(t, a.OrderID, "old order binding must not survive a relist")
	assert.Nil(t, a.CurrentBid, "MinimumBid() must return StartPrice again")
	assert.Nil(t, a.CurrentWinnerID)
	assert.Nil(t, a.ShippingResolvedAt)
	assert.False(t, a.SellerActionRequired)
	assert.False(t, a.SellerQuoteProvided)
	assert.Zero(t, a.AntiSnipeExtensionTotal, "anti-sniping budget is per-lifecycle")
}

// POSITIVE PROOF: a lapsed auction (never went live, held back by the system)
// relists through the SAME single gate — the owner-approved lapsed -> scheduled
// path used after the seller renews.
func TestRelist_Lapsed_RepublishesToScheduled(t *testing.T) {
	a := createTestAuction()
	require.NoError(t, a.Lapse())
	require.Equal(t, StatusLapsed, a.Status)

	now := time.Now()
	require.NoError(t, a.Relist(now, now.Add(24*time.Hour), a.StartPrice, a.BidIncrement, a.BuyNowPrice))

	assert.Equal(t, StatusScheduled, a.Status, "lapsed must be relistable (lapsed -> scheduled)")
}

// NEGATIVE PROOF: an auction that produced an outcome is settled history and
// cannot be republished, whatever field carries that outcome.
func TestRelist_RejectsAuctionWithOutcome(t *testing.T) {
	t.Run("bid placed", func(t *testing.T) {
		a := endedNoBidAuction(t)
		bid := int64(250_000)
		a.CurrentBid = &bid

		now := time.Now()
		err := a.Relist(now, now.Add(24*time.Hour), a.StartPrice, a.BidIncrement, nil)

		require.ErrorIs(t, err, ErrAuctionNotRelistable)
		assert.Equal(t, StatusEnded, a.Status, "status must be untouched on rejection")
	})

	t.Run("winner recorded", func(t *testing.T) {
		a := endedNoBidAuction(t)
		winner := uuid.New()
		a.CurrentWinnerID = &winner

		now := time.Now()
		err := a.Relist(now, now.Add(24*time.Hour), a.StartPrice, a.BidIncrement, nil)

		require.ErrorIs(t, err, ErrAuctionNotRelistable)
		assert.Equal(t, StatusEnded, a.Status)
	})

	t.Run("order bound", func(t *testing.T) {
		a := endedAuctionWithOutcome(t)

		now := time.Now()
		err := a.Relist(now, now.Add(24*time.Hour), a.StartPrice, a.BidIncrement, nil)

		require.ErrorIs(t, err, ErrAuctionNotRelistable)
		assert.Equal(t, StatusEnded, a.Status)
		assert.NotNil(t, a.OrderID, "settlement binding must stay intact")
	})
}

// NEGATIVE PROOF: relist is reachable from ended and lapsed only. Live or
// in-flight runs (and terminal states) must never reopen through this path.
func TestRelist_RejectsNonRelistableStatuses(t *testing.T) {
	for _, status := range []Status{
		StatusScheduled,
		StatusActive,
		StatusWaitingSettlement,
		StatusCancelled,
	} {
		t.Run(string(status), func(t *testing.T) {
			a := createTestAuction()
			a.Status = status

			now := time.Now()
			err := a.Relist(now, now.Add(24*time.Hour), a.StartPrice, a.BidIncrement, nil)

			require.Error(t, err)
			assert.IsType(t, &InvalidTransitionError{}, err)
			assert.Equal(t, status, a.Status, "status must be untouched on rejection")
		})
	}
}

// NEGATIVE PROOF / RESIDUE GUARD: widening the transition map with
// ended -> scheduled must NOT open the settlement-failure path to an ended
// auction. The settlement reschedule is waiting_settlement-only.
func TestRescheduleAfterSettlementFailure_StaysClosedForEnded(t *testing.T) {
	a := endedNoBidAuction(t)

	err := a.RescheduleAfterSettlementFailure()

	require.Error(t, err)
	assert.IsType(t, &InvalidTransitionError{}, err)
	assert.Equal(t, StatusEnded, a.Status, "ended must not reach the settlement path")
}

// NEGATIVE PROOF: relist must not become a cancellation backdoor — ended still
// cannot transition to cancelled.
func TestRelist_DoesNotOpenEndedToCancelled(t *testing.T) {
	a := endedNoBidAuction(t)

	assert.False(t, canTransition(StatusEnded, StatusCancelled),
		"relist must not widen ended beyond a single scheduled transition")
	assert.False(t, a.CanCancel())
}

// NEGATIVE PROOF: the transition map exposes exactly one target from ended.
func TestRelist_ExposesOnlyScheduledFromEnded(t *testing.T) {
	targets := transitionAllowed[StatusEnded]

	assert.Equal(t, []Status{StatusScheduled}, targets,
		"ended may only ever lead back to scheduled, and only through Relist (republish)")
}

// FIX #2 PROOF (strengthened): a stale end_at can never commit a republished
// run — entity.Relist re-checks RequireFutureAuctionEnd itself (belt and
// suspenders behind ResolveAuctionTiming), so the auction stays in its
// finished state until the seller refreshes the timing.
func TestRelist_RejectsStaleEndAt(t *testing.T) {
	a := endedNoBidAuction(t)
	staleStart := time.Now().Add(-25 * time.Hour)
	staleEnd := time.Now().Add(-time.Hour) // the finished run's elapsed end

	err := a.Relist(staleStart, staleEnd, a.StartPrice, a.BidIncrement, a.BuyNowPrice)

	var endErr *ErrAuctionEndAlreadyPassed
	require.ErrorAs(t, err, &endErr)
	assert.Equal(t, StatusEnded, a.Status, "a stale end_at must not reach scheduled")
}

// BOUNDARY PROOF: the gate targets end_at ONLY. A start_at in the past is
// legitimate (StartModeNow resolves it), so a republish must still succeed as
// long as end_at is ahead of the clock — covered by the relist end gate.

// REJECTED-STATE PROOF: a failed end gate is side-effect free — status stays
// ended so the seller can retry with corrected timing.
func TestRelist_PastEndAt_LeavesAuctionRetryable(t *testing.T) {
	a := createTestAuction()
	a.Status = StatusEnded
	a.EndAt = time.Now().Add(-time.Minute)

	staleStart := time.Now().Add(-2 * time.Hour)
	staleEnd := time.Now().Add(-time.Minute)
	require.Error(t, a.Relist(staleStart, staleEnd, a.StartPrice, a.BidIncrement, a.BuyNowPrice))
	require.Equal(t, StatusEnded, a.Status)

	now := time.Now()
	require.NoError(t, a.Relist(now, now.Add(24*time.Hour), a.StartPrice, a.BidIncrement, a.BuyNowPrice))
	assert.Equal(t, StatusScheduled, a.Status)
}
