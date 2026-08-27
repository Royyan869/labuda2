package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/rating/entity"
	"github.com/labuda/backend/internal/commerce/order/rating/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// RatingReader provides read-only access to rating data.
//
// SECURITY: This interface enforces read-only boundary.
// External domains CANNOT modify ratings through this interface.
//
// Use cases:
// - Seller dashboard (average rating display)
// - Monthly metrics worker (period-based aggregation)
// - Public profile display
type RatingReader interface {
	// GetRatingSummary retrieves the aggregated rating summary for a seller.
	// Returns total count, average rating, and distribution by star rating.
	//
	// RATING INVALIDATION: Only includes valid ratings (invalidated_at IS NULL).
	// Invalidated ratings from refunded orders are excluded from the summary.
	GetRatingSummary(
		ctx context.Context,
		tx db.Tx,
		sellerID uuid.UUID,
	) (*repository.RatingSummary, error)

	// GetAverageRatingForPeriod calculates the average rating for a seller within a time period.
	// Returns 0.0 if no ratings exist for the period.
	//
	// RATING INVALIDATION: Only counts valid ratings (invalidated_at IS NULL).
	// Invalidated ratings from refunded orders are excluded from aggregation.
	//
	// Use case: Monthly metrics worker
	GetAverageRatingForPeriod(
		ctx context.Context,
		tx db.Tx,
		sellerID uuid.UUID,
		periodStart time.Time,
		periodEnd time.Time,
	) (float64, error)

	// GetRatingByOrder retrieves a rating by order ID without locking.
	// Returns nil if no rating exists for the order.
	//
	// Use case: Order detail display
	GetRatingByOrder(
		ctx context.Context,
		tx db.Tx,
		orderID uuid.UUID,
	) (*entity.OrderRating, error)

	// ListRatingsGivenByBuyer retrieves ratings given by a buyer with cursor-based pagination.
	// Returns ratings ordered by created_at DESC.
	//
	// RATING INVALIDATION: Only returns valid ratings (invalidated_at IS NULL).
	// Invalidated ratings from refunded orders are excluded from the list.
	ListRatingsGivenByBuyer(
		ctx context.Context,
		tx db.Tx,
		input ListRatingsGivenByBuyerInput,
	) ([]*entity.OrderRating, error)

	// ListRatingsReceivedBySeller retrieves ratings received by a seller with cursor-based pagination.
	// Returns ratings ordered by created_at DESC.
	//
	// RATING INVALIDATION: Only returns valid ratings (invalidated_at IS NULL).
	// Invalidated ratings from refunded orders are excluded from the list.
	ListRatingsReceivedBySeller(
		ctx context.Context,
		tx db.Tx,
		input ListRatingsReceivedBySellerInput,
	) ([]*entity.OrderRating, error)
}

// RatingMutator provides write access to rating data.
//
// SECURITY: This interface enforces write boundary.
// External domains MUST go through this interface for any rating modifications.
//
// INVARIANTS enforced:
// - Buyer-only: Only buyer can rate seller
// - Order must be completed
// - One rating per order
// - Immutable: No update or delete operations
// - No financial mutations
//
// Use cases:
// - Order completion (buyer rates seller)
// - Refund processing (invalidate rating)
type RatingMutator interface {
	// CreateRating creates a new rating for a completed order.
	//
	// VALIDATION FLOW:
	// 1. BEGIN transaction (caller-provided)
	// 2. Lock order FOR UPDATE
	// 3. Validate order exists
	// 4. Validate order.status == "completed"
	// 5. Validate callerID == order.buyer_id
	// 6. Check no existing rating for this order
	// 7. Create rating
	// 8. COMMIT transaction (caller-managed)
	//
	// IMPORTANT:
	// - Does NOT modify order state
	// - Does NOT emit outbox events (no downstream side effects)
	// - Does NOT modify ledger
	// - Returns domain errors for validation failures
	//
	// This method is idempotent - if rating already exists, returns ErrAlreadyRated.
	CreateRating(
		ctx context.Context,
		tx db.Tx,
		input CreateRatingInput,
	) (*entity.OrderRating, error)

	// InvalidateForOrder marks the rating for an order as invalid.
	// This is called when an order is refunded to prevent rating abuse.
	//
	// RATING INVALIDATION:
	// - Sets invalidated_at = NOW() on the order's rating
	// - Prevents the rating from being counted in aggregations
	// - Idempotent: Safe to call multiple times
	//
	// Use cases:
	// - Full refund (RefundOrder)
	// - Dispute refund (RefundFromDispute)
	// - Partial refund (PartialRefund)
	//
	// This method enforces the rating domain boundary by ensuring that all
	// rating invalidation operations go through the rating domain, not direct SQL.
	InvalidateForOrder(
		ctx context.Context,
		tx db.Tx,
		orderID uuid.UUID,
	) error
}

// Ensure RatingService implements the interfaces
var (
	_ RatingReader = (*RatingService)(nil)
	_ RatingMutator = (*RatingService)(nil)
)


