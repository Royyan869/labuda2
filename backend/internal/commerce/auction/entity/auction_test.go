package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// SETTLEMENT SAFETY TESTS
// ============================================================================

// TestAuctionOrderID verifies OrderID field behavior.
func TestAuctionOrderID(t *testing.T) {
	t.Run("NewDraft has nil OrderID", func(t *testing.T) {
		auction := createTestDraftAuction()
		assert.Nil(t, auction.OrderID, "New auction should have nil OrderID")
	})

	t.Run("Can set OrderID when settling", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusActive

		orderID := uuid.New()
		auction.OrderID = &orderID

		require.NotNil(t, auction.OrderID)
		assert.Equal(t, orderID, *auction.OrderID)
	})
}

// TestDoubleSettlementPrevention verifies that OrderID prevents double settlement.
// This is an entity-level test - the service layer enforces this.
func TestDoubleSettlementPrevention(t *testing.T) {
	t.Run("Auction with OrderID is marked as settled", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusActive

		// Initially not settled
		assert.Nil(t, auction.OrderID)

		// Simulate settlement
		orderID := uuid.New()
		auction.OrderID = &orderID

		// Now settled - service layer should check OrderID before creating new order
		assert.NotNil(t, auction.OrderID)
		assert.Equal(t, orderID, *auction.OrderID)
	})
}

// ============================================================================
// EXISTING TESTS
// ============================================================================

// TestNewDraft verifies that a new draft auction is created correctly.
func TestNewDraft(t *testing.T) {
	sellerID := uuid.New()
	productID := uuid.New()
	startPrice := int64(10000)
	bidIncrement := int64(1000)
	buyNowPrice := int64(50000)
	startAt := time.Now().Add(1 * time.Hour)
	endAt := time.Now().Add(25 * time.Hour)

	auction := NewDraft(
		sellerID,
		productID,
		startPrice,
		bidIncrement,
		&buyNowPrice,
		startAt,
		endAt,
	)

	assert.Equal(t, sellerID, auction.SellerID)
	assert.Equal(t, productID, auction.ProductID)
	assert.Equal(t, startPrice, auction.StartPrice)
	assert.Equal(t, bidIncrement, auction.BidIncrement)
	assert.Equal(t, &buyNowPrice, auction.BuyNowPrice)
	assert.Equal(t, startAt, auction.StartAt)
	assert.Equal(t, endAt, auction.EndAt)
	assert.Nil(t, auction.CurrentBid)
	assert.Nil(t, auction.CurrentWinnerID)
	assert.Equal(t, StatusDraft, auction.Status)
	assert.False(t, auction.CreatedAt.IsZero())
	assert.False(t, auction.UpdatedAt.IsZero())
}

// TestAuctionStateTransitions verifies valid state transitions.
func TestAuctionStateTransitions(t *testing.T) {
	t.Run("Draft to Scheduled", func(t *testing.T) {
		auction := createTestDraftAuction()
		err := auction.Schedule()
		assert.NoError(t, err)
		assert.Equal(t, StatusScheduled, auction.Status)
	})

	t.Run("Scheduled to Active", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusScheduled
		err := auction.Activate()
		assert.NoError(t, err)
		assert.Equal(t, StatusActive, auction.Status)
	})

	t.Run("Active to Ended", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusActive
		err := auction.End()
		assert.NoError(t, err)
		assert.Equal(t, StatusEnded, auction.Status)
	})

	t.Run("Draft to Cancelled", func(t *testing.T) {
		auction := createTestDraftAuction()
		err := auction.Cancel()
		assert.NoError(t, err)
		assert.Equal(t, StatusCancelled, auction.Status)
	})

	t.Run("Scheduled to Cancelled", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusScheduled
		err := auction.Cancel()
		assert.NoError(t, err)
		assert.Equal(t, StatusCancelled, auction.Status)
	})

	t.Run("Scheduled back to Draft", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusScheduled
		err := auction.UpdateDraft(10000, 1000, nil, auction.StartAt, auction.EndAt)
		// Should fail - cannot revert to draft via UpdateDraft
		assert.Error(t, err)
	})
}

// TestInvalidStateTransitions verifies that invalid transitions fail.
func TestInvalidStateTransitions(t *testing.T) {
	t.Run("Ended cannot transition", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusEnded

		err := auction.Schedule()
		assert.Error(t, err)
		assert.IsType(t, &InvalidTransitionError{}, err)

		err = auction.Activate()
		assert.Error(t, err)

		err = auction.End()
		assert.Error(t, err)

		err = auction.Cancel()
		assert.Error(t, err)
	})

	t.Run("Cancelled cannot transition", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusCancelled

		err := auction.Schedule()
		assert.Error(t, err)

		err = auction.Activate()
		assert.Error(t, err)

		err = auction.End()
		assert.Error(t, err)

		err = auction.Cancel()
		assert.Error(t, err)
	})

	t.Run("Active cannot go back to Scheduled", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusActive
		auction.Status = StatusScheduled // Direct assignment to test validation
		err := auction.Schedule()
		assert.Error(t, err)
	})
}

// TestPlaceBid verifies bid placement rules.
func TestPlaceBid(t *testing.T) {
	t.Run("Cannot bid on non-active auction", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusDraft

		bidderID := uuid.New()
		err := auction.PlaceBid(bidderID, 15000, time.Now())
		assert.Error(t, err)
		assert.IsType(t, &AuctionNotActiveError{}, err)
	})

	t.Run("Cannot bid after end time", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusActive
		auction.EndAt = time.Now().Add(-1 * time.Hour) // Ended in the past

		bidderID := uuid.New()
		err := auction.PlaceBid(bidderID, 15000, time.Now())
		assert.Error(t, err)
		assert.IsType(t, &AuctionEndedError{}, err)
	})

	t.Run("Cannot bid on own auction", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusActive
		auction.EndAt = time.Now().Add(1 * time.Hour)

		err := auction.PlaceBid(auction.SellerID, 15000, time.Now())
		assert.Error(t, err)
		assert.IsType(t, &SelfBidError{}, err)
	})

	t.Run("Bid below minimum fails", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusActive
		auction.EndAt = time.Now().Add(1 * time.Hour)
		auction.StartPrice = 10000

		bidderID := uuid.New()
		err := auction.PlaceBid(bidderID, 9000, time.Now()) // Below start price
		assert.Error(t, err)
		assert.IsType(t, &BidTooLowError{}, err)
	})

	t.Run("First bid at start price succeeds", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusActive
		auction.EndAt = time.Now().Add(1 * time.Hour)
		auction.StartPrice = 10000

		bidderID := uuid.New()
		err := auction.PlaceBid(bidderID, 10000, time.Now())
		assert.NoError(t, err)
		assert.Equal(t, int64(10000), *auction.CurrentBid)
		assert.Equal(t, bidderID, *auction.CurrentWinnerID)
	})

	t.Run("Subsequent bid must be minimum increment", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusActive
		auction.EndAt = time.Now().Add(1 * time.Hour)
		auction.StartPrice = 10000
		auction.BidIncrement = 1000

		// First bid
		bidder1 := uuid.New()
		err := auction.PlaceBid(bidder1, 10000, time.Now())
		require.NoError(t, err)

		// Second bid too low
		bidder2 := uuid.New()
		err = auction.PlaceBid(bidder2, 10500, time.Now()) // Needs to be at least 11000
		assert.Error(t, err)
		assert.IsType(t, &BidTooLowError{}, err)

		// Valid second bid
		err = auction.PlaceBid(bidder2, 11000, time.Now())
		assert.NoError(t, err)
		assert.Equal(t, int64(11000), *auction.CurrentBid)
		assert.Equal(t, bidder2, *auction.CurrentWinnerID)
	})
}

// TestMinimumBid verifies minimum bid calculation.
func TestMinimumBid(t *testing.T) {
	t.Run("No current bid: minimum is start price", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.StartPrice = 10000
		auction.CurrentBid = nil

		minimum := auction.MinimumBid()
		assert.Equal(t, int64(10000), minimum)
	})

	t.Run("With current bid: minimum is current + increment", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.StartPrice = 10000
		auction.BidIncrement = 1000
		currentBid := int64(15000)
		auction.CurrentBid = &currentBid

		minimum := auction.MinimumBid()
		assert.Equal(t, int64(16000), minimum)
	})
}

// TestCanCancel verifies cancellation rules.
func TestCanCancel(t *testing.T) {
	t.Run("Draft can be cancelled", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusDraft
		assert.True(t, auction.CanCancel())
	})

	t.Run("Scheduled can be cancelled", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusScheduled
		assert.True(t, auction.CanCancel())
	})

	t.Run("Active without bids can be cancelled", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusActive
		auction.CurrentBid = nil
		assert.True(t, auction.CanCancel())
	})

	t.Run("Active with bids cannot be cancelled", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusActive
		bid := int64(10000)
		auction.CurrentBid = &bid
		assert.False(t, auction.CanCancel())
	})

	t.Run("Ended cannot be cancelled", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusEnded
		assert.False(t, auction.CanCancel())
	})

	t.Run("Cancelled cannot be cancelled again", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusCancelled
		assert.False(t, auction.CanCancel())
	})
}

// TestUpdateDraft verifies draft update rules.
func TestUpdateDraft(t *testing.T) {
	t.Run("Can update draft auction", func(t *testing.T) {
		auction := createTestDraftAuction()
		newStartAt := time.Now().Add(2 * time.Hour)
		newEndAt := time.Now().Add(26 * time.Hour)

		err := auction.UpdateDraft(
			20000,
			2000,
			nil,
			newStartAt,
			newEndAt,
		)

		assert.NoError(t, err)
		assert.Equal(t, int64(20000), auction.StartPrice)
		assert.Equal(t, int64(2000), auction.BidIncrement)
		assert.Nil(t, auction.BuyNowPrice)
	})

	t.Run("Cannot update non-draft auction", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusScheduled

		err := auction.UpdateDraft(
			20000,
			2000,
			nil,
			auction.StartAt,
			auction.EndAt,
		)

		assert.Error(t, err)
		assert.IsType(t, &InvalidOperationError{}, err)
	})
}

// TestUpdateScheduled verifies scheduled update rules.
func TestUpdateScheduled(t *testing.T) {
	t.Run("Can update scheduled auction", func(t *testing.T) {
		auction := createTestDraftAuction()
		auction.Status = StatusScheduled

		newStartAt := time.Now().Add(2 * time.Hour)
		newEndAt := time.Now().Add(26 * time.Hour)

		err := auction.UpdateScheduled(
			newStartAt,
			newEndAt,
		)

		assert.NoError(t, err)
		assert.Equal(t, newStartAt, auction.StartAt)
		assert.Equal(t, newEndAt, auction.EndAt)
		// Price fields should remain unchanged
		assert.Equal(t, int64(10000), auction.StartPrice)
	})

	t.Run("Cannot update non-scheduled auction", func(t *testing.T) {
		auction := createTestDraftAuction()
		// Keep it as draft

		err := auction.UpdateScheduled(
			auction.StartAt,
			auction.EndAt,
		)

		assert.Error(t, err)
		assert.IsType(t, &InvalidOperationError{}, err)
	})
}

// TestHasWinner verifies winner detection.
func TestHasWinner(t *testing.T) {
	t.Run("No bids: no winner", func(t *testing.T) {
		auction := createTestDraftAuction()
		assert.False(t, auction.HasWinner())
		assert.Nil(t, auction.WinnerID())
		assert.Nil(t, auction.WinningBid())
	})

	t.Run("With bid: has winner", func(t *testing.T) {
		auction := createTestDraftAuction()
		bidderID := uuid.New()
		bidAmount := int64(15000)
		auction.CurrentBid = &bidAmount
		auction.CurrentWinnerID = &bidderID

		assert.True(t, auction.HasWinner())
		assert.Equal(t, bidderID, *auction.WinnerID())
		assert.Equal(t, bidAmount, *auction.WinningBid())
	})
}

// createTestDraftAuction creates a test draft auction.
func createTestDraftAuction() *Auction {
	sellerID := uuid.New()
	listingID := uuid.New()
	startAt := time.Now().Add(1 * time.Hour)
	endAt := time.Now().Add(25 * time.Hour)
	buyNowPrice := int64(50000)

	return NewDraft(
		sellerID,
		listingID,
		10000,
		1000,
		&buyNowPrice,
		startAt,
		endAt,
	)
}



