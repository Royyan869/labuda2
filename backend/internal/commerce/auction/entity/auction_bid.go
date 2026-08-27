package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuctionBid represents a single bid on an auction.
// Bids are immutable once created - they form a bid history.
type AuctionBid struct {
	ID             uuid.UUID
	AuctionID      uuid.UUID
	BidderID       uuid.UUID
	Amount         int64 // In minor unit
	IdempotencyKey string // For idempotent bid operations
	CreatedAt      time.Time
}

// InvalidAmountError is returned when bid amount is invalid.
type InvalidAmountError struct {
	Amount int64
	Reason string
}

func (e *InvalidAmountError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("invalid bid amount: %s", e.Reason)
	}
	return fmt.Sprintf("invalid bid amount: %d", e.Amount)
}

// NewAuctionBid creates a new auction bid.
func NewAuctionBid(
	auctionID, bidderID uuid.UUID,
	amount int64,
	idempotencyKey string,
) (*AuctionBid, error) {
	// Validate amount
	if amount <= 0 {
		return nil, &InvalidAmountError{Amount: amount, Reason: "must be positive"}
	}

	now := time.Now()

	return &AuctionBid{
		ID:             uuid.New(),
		AuctionID:      auctionID,
		BidderID:       bidderID,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      now,
	}, nil
}

// ValidateIdempotencyKey validates the idempotency key format.
func ValidateIdempotencyKey(key string) error {
	if key == "" {
		return fmt.Errorf("idempotency key cannot be empty")
	}
	// Add additional validation if needed (e.g., length, format)
	return nil
}


