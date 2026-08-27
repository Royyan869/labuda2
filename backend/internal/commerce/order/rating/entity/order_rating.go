package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OrderRating represents a buyer's rating of a seller after a completed order.
// This entity is immutable after creation - no Update or Delete methods.
//
// Invariants:
// - Rating value must be between 1 and 5
// - Only buyer can rate seller (not the other way around)
// - Order must be completed before rating
// - One rating per order (enforced by UNIQUE constraint)
//
// Rating Invalidation:
// - InvalidatedAt is NULL for valid ratings (counted in seller metrics)
// - InvalidatedAt is set when order is refunded (rating excluded from metrics)
// - Soft deletion preserves audit trail while preventing invalid ratings
// OrderRating is the canonical rating domain model and, when serialized to
// JSON, the canonical rating HTTP resource. The json tags lock the wire keys
// to snake_case (the Rating HTTP contract), preserving buyer_id/seller_id as
// the raw domain identity — no reviewer card, no verified_purchase, no
// aggregation fields.
type OrderRating struct {
	ID            uuid.UUID  `json:"id"`
	OrderID       uuid.UUID  `json:"order_id"`
	BuyerID       uuid.UUID  `json:"buyer_id"`
	SellerID      uuid.UUID  `json:"seller_id"`
	RatingValue   int        `json:"rating_value"`
	Comment       *string    `json:"comment,omitempty"` // Optional comment
	CreatedAt     time.Time  `json:"created_at"`
	InvalidatedAt *time.Time `json:"invalidated_at,omitempty"` // NULL = valid rating, set = invalidated (e.g., due to refund)
}

// Domain Errors

// ErrInvalidRatingValue is returned when rating value is not between 1 and 5.
type ErrInvalidRatingValue struct {
	Value int
}

func (e *ErrInvalidRatingValue) Error() string {
	return fmt.Sprintf("invalid rating value: %d (must be between 1 and 5)", e.Value)
}

// ErrOrderNotCompleted is returned when attempting to rate a non-completed order.
type ErrOrderNotCompleted struct {
	OrderID   uuid.UUID
	Status    string
	OrderType string
}

func (e *ErrOrderNotCompleted) Error() string {
	if e.OrderType != "" {
		return fmt.Sprintf("cannot rate order: %s is not completed (status: %s, type: %s)", e.OrderID, e.Status, e.OrderType)
	}
	return fmt.Sprintf("cannot rate order: %s is not completed (status: %s)", e.OrderID, e.Status)
}

// ErrNotBuyer is returned when caller is not the buyer of the order.
type ErrNotBuyer struct {
	OrderID  uuid.UUID
	CallerID uuid.UUID
	BuyerID  uuid.UUID
}

func (e *ErrNotBuyer) Error() string {
	return fmt.Sprintf("caller %s is not the buyer of order %s (buyer: %s)", e.CallerID, e.OrderID, e.BuyerID)
}

// ErrAlreadyRated is returned when an order already has a rating.
type ErrAlreadyRated struct {
	OrderID uuid.UUID
}

func (e *ErrAlreadyRated) Error() string {
	return fmt.Sprintf("order %s already rated", e.OrderID)
}

// ErrOrderNotFound is returned when the order does not exist.
type ErrOrderNotFound struct {
	OrderID uuid.UUID
}

func (e *ErrOrderNotFound) Error() string {
	return fmt.Sprintf("order not found: %s", e.OrderID)
}

// ErrOrderHasActiveDispute is returned when attempting to rate an order with an active dispute.
// This prevents rating bias during disputes - ratings should only be allowed after disputes are resolved.
type ErrOrderHasActiveDispute struct {
	OrderID uuid.UUID
}

func (e *ErrOrderHasActiveDispute) Error() string {
	return fmt.Sprintf("cannot rate order %s: order has an active dispute", e.OrderID)
}

// NewOrderRating creates a new OrderRating with validation.
//
// Rules:
// - Rating value must be between 1 and 5
// - Comment can be nil (optional) or a non-empty string
//
// This constructor enforces invariants at creation time.
func NewOrderRating(
	orderID uuid.UUID,
	buyerID uuid.UUID,
	sellerID uuid.UUID,
	ratingValue int,
	comment *string,
) (*OrderRating, error) {
	// Validate rating value range
	if ratingValue < 1 || ratingValue > 5 {
		return nil, &ErrInvalidRatingValue{Value: ratingValue}
	}

	// Validate comment if provided
	if comment != nil && *comment == "" {
		// Convert empty string to nil for consistency
		comment = nil
	}

	return &OrderRating{
		ID:            uuid.New(),
		OrderID:       orderID,
		BuyerID:       buyerID,
		SellerID:      sellerID,
		RatingValue:   ratingValue,
		Comment:       comment,
		CreatedAt:     time.Now(),
		InvalidatedAt: nil, // New ratings are valid by default
	}, nil
}

// ValidateRatingValue checks if a rating value is valid (1-5).
// This is a utility function for validation without creating an entity.
func ValidateRatingValue(ratingValue int) error {
	if ratingValue < 1 || ratingValue > 5 {
		return &ErrInvalidRatingValue{Value: ratingValue}
	}
	return nil
}


