package entity

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestNewAuctionBid verifies bid creation.
func TestNewAuctionBid(t *testing.T) {
	auctionID := uuid.New()
	bidderID := uuid.New()
	amount := int64(10000)
	idempotencyKey := "test-key-123"

	bid, err := NewAuctionBid(auctionID, bidderID, amount, idempotencyKey)

	assert.NoError(t, err)
	assert.Equal(t, auctionID, bid.AuctionID)
	assert.Equal(t, bidderID, bid.BidderID)
	assert.Equal(t, amount, bid.Amount)
	assert.Equal(t, idempotencyKey, bid.IdempotencyKey)
	assert.False(t, bid.CreatedAt.IsZero())
}

// TestNewAuctionBidValidation verifies bid validation rules.
func TestNewAuctionBidValidation(t *testing.T) {
	t.Run("Negative amount fails", func(t *testing.T) {
		_, err := NewAuctionBid(uuid.New(), uuid.New(), -1000, "key")
		assert.Error(t, err)
		assert.IsType(t, &InvalidAmountError{}, err)
	})

	t.Run("Zero amount fails", func(t *testing.T) {
		_, err := NewAuctionBid(uuid.New(), uuid.New(), 0, "key")
		assert.Error(t, err)
		assert.IsType(t, &InvalidAmountError{}, err)
	})

	t.Run("Positive amount succeeds", func(t *testing.T) {
		_, err := NewAuctionBid(uuid.New(), uuid.New(), 1, "key")
		assert.NoError(t, err)
	})
}

// TestValidateIdempotencyKey verifies idempotency key validation.
func TestValidateIdempotencyKey(t *testing.T) {
	t.Run("Empty key fails", func(t *testing.T) {
		err := ValidateIdempotencyKey("")
		assert.Error(t, err)
	})

	t.Run("Non-empty key succeeds", func(t *testing.T) {
		err := ValidateIdempotencyKey("valid-key")
		assert.NoError(t, err)
	})
}

// TestIdempotentBid verifies that same idempotency key returns same bid logic.
// Note: Actual idempotency is enforced at repository level via UNIQUE constraint
// scoped to (auction_id, bidder_id, idempotency_key) — see migration 000214.
func TestIdempotentBid(t *testing.T) {
	auctionID := uuid.New()
	bidderID := uuid.New()
	amount := int64(10000)
	idempotencyKey := "auction.bid.placed." + auctionID.String()

	bid1, err := NewAuctionBid(auctionID, bidderID, amount, idempotencyKey)
	assert.NoError(t, err)
	assert.Equal(t, idempotencyKey, bid1.IdempotencyKey)

	// Creating another bid with same key should be allowed at entity level
	// The constraint is enforced at database level
	bid2, err := NewAuctionBid(auctionID, bidderID, amount+1000, idempotencyKey)
	assert.NoError(t, err)
	assert.Equal(t, idempotencyKey, bid2.IdempotencyKey)
	// This would fail on INSERT due to UNIQUE constraint
}

// TestCrossBidderSameKeyAreIndependent verifies that two different bidders can
// create bid entities with the same idempotency key on the same auction.
// Entity layer imposes no uniqueness; the DB constraint
// UNIQUE (auction_id, bidder_id, idempotency_key) allows both rows because
// bidder_id differs. If the constraint were auction-scoped only, bidder B's
// INSERT would collide with bidder A's row, triggering the P1 described in
// migration 000214.
func TestCrossBidderSameKeyAreIndependent(t *testing.T) {
	auctionID := uuid.New()
	bidderA := uuid.New()
	bidderB := uuid.New()
	sharedKey := "mobile-retry-key-abc"

	bidA, err := NewAuctionBid(auctionID, bidderA, 10_000, sharedKey)
	assert.NoError(t, err)
	assert.Equal(t, bidderA, bidA.BidderID)
	assert.Equal(t, sharedKey, bidA.IdempotencyKey)

	bidB, err := NewAuctionBid(auctionID, bidderB, 20_000, sharedKey)
	assert.NoError(t, err)
	assert.Equal(t, bidderB, bidB.BidderID)
	assert.Equal(t, sharedKey, bidB.IdempotencyKey)

	// Different bidder IDs — bids are independent at entity level and will not
	// collide at the DB level under the bidder-scoped unique constraint.
	assert.NotEqual(t, bidA.BidderID, bidB.BidderID)
	assert.Equal(t, bidA.IdempotencyKey, bidB.IdempotencyKey)
	assert.Equal(t, bidA.AuctionID, bidB.AuctionID)
}


