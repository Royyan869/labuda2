package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlaceBid_AntiSniping_OutsideWindow_NoExtension proves a bid landing
// well before the closing window leaves EndAt untouched.
func TestPlaceBid_AntiSniping_OutsideWindow_NoExtension(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusActive
	now := time.Now()
	auction.EndAt = now.Add(10 * time.Minute) // outside the 5-minute window

	originalEnd := auction.EndAt
	err := auction.PlaceBid(uuid.New(), auction.StartPrice, now)
	require.NoError(t, err)

	assert.Equal(t, originalEnd, auction.EndAt, "bid outside the closing window must not extend EndAt")
	assert.Equal(t, time.Duration(0), auction.AntiSnipeExtensionTotal)
}

// TestPlaceBid_AntiSniping_InsideWindow_Extends proves a bid landing inside
// the final 5 minutes extends EndAt by exactly 5 minutes.
func TestPlaceBid_AntiSniping_InsideWindow_Extends(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusActive
	now := time.Now()
	auction.EndAt = now.Add(2 * time.Minute) // inside the 5-minute window

	originalEnd := auction.EndAt
	err := auction.PlaceBid(uuid.New(), auction.StartPrice, now)
	require.NoError(t, err)

	assert.Equal(t, originalEnd.Add(AntiSnipingExtension), auction.EndAt)
	assert.Equal(t, AntiSnipingExtension, auction.AntiSnipeExtensionTotal)
}

// TestPlaceBid_AntiSniping_AtExactWindowBoundary_Extends proves the window is
// inclusive: a bid exactly AntiSnipingWindow before EndAt still extends.
func TestPlaceBid_AntiSniping_AtExactWindowBoundary_Extends(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusActive
	now := time.Now()
	auction.EndAt = now.Add(AntiSnipingWindow)

	err := auction.PlaceBid(uuid.New(), auction.StartPrice, now)
	require.NoError(t, err)
	assert.Equal(t, AntiSnipingExtension, auction.AntiSnipeExtensionTotal)
}

// TestPlaceBid_AntiSniping_CapEnforced proves the cumulative extension never
// exceeds MaxAntiSnipingTotalExtension, regardless of how many late bids land.
func TestPlaceBid_AntiSniping_CapEnforced(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusActive
	auction.StartPrice = 10000
	auction.BidIncrement = 1000
	now := time.Now()
	auction.EndAt = now.Add(1 * time.Minute) // inside window

	bidAmount := auction.StartPrice
	extensionsApplied := 0
	maxPossibleExtensions := int(MaxAntiSnipingTotalExtension/AntiSnipingExtension) + 2 // deliberately overshoot the cap

	for i := 0; i < maxPossibleExtensions; i++ {
		// Keep re-arming the closing window so every bid in the loop lands
		// inside it (simulates a sustained sniping war), isolating the cap
		// as the only thing that should stop further extension.
		auction.EndAt = now.Add(1 * time.Minute)

		err := auction.PlaceBid(uuid.New(), bidAmount, now)
		require.NoError(t, err)
		bidAmount += auction.BidIncrement

		if auction.AntiSnipeExtensionTotal < MaxAntiSnipingTotalExtension {
			extensionsApplied++
		}
	}

	assert.Equal(t, MaxAntiSnipingTotalExtension, auction.AntiSnipeExtensionTotal,
		"cumulative extension must be capped at MaxAntiSnipingTotalExtension")
	assert.Greater(t, extensionsApplied, 0, "at least one extension should have been applied before the cap")
}

// TestPlaceBid_AntiSniping_BidAfterEnd_StillRejected proves the anti-sniping
// feature does not create a loophole around the hard end-time cutoff — a bid
// at/after the current EndAt is rejected exactly as before.
func TestPlaceBid_AntiSniping_BidAfterEnd_StillRejected(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusActive
	now := time.Now()
	auction.EndAt = now.Add(-1 * time.Minute) // already ended

	err := auction.PlaceBid(uuid.New(), auction.StartPrice, now)
	assert.Error(t, err)
	assert.IsType(t, &AuctionEndedError{}, err)
	assert.Equal(t, time.Duration(0), auction.AntiSnipeExtensionTotal)
}

// TestPlaceBid_AntiSniping_ExtendsPastCap_NoOp proves that once the cap is
// reached, further late bids succeed (bidding itself is unaffected) but stop
// extending EndAt — the auction still ends normally.
func TestPlaceBid_AntiSniping_ExtendsPastCap_NoOp(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusActive
	auction.StartPrice = 10000
	auction.BidIncrement = 1000
	auction.AntiSnipeExtensionTotal = MaxAntiSnipingTotalExtension // already at the cap
	now := time.Now()
	auction.EndAt = now.Add(1 * time.Minute)
	endBeforeBid := auction.EndAt

	err := auction.PlaceBid(uuid.New(), auction.StartPrice, now)
	require.NoError(t, err, "bidding must still succeed even once the anti-sniping cap is reached")
	assert.Equal(t, endBeforeBid, auction.EndAt, "EndAt must not move once the cap is reached")
	assert.Equal(t, MaxAntiSnipingTotalExtension, auction.AntiSnipeExtensionTotal)
}
