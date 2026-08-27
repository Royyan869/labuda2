package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	ratingEntity "github.com/labuda/backend/internal/commerce/order/rating/entity"
	"github.com/labuda/backend/internal/commerce/order/rating/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// ⚠️ RATING DOMAIN BOUNDARY WARNING ⚠️
//
// This package contains the RATING DOMAIN implementation.
//
// SECURITY RULES:
// 1. External domains MUST NOT import this package directly
// 2. External domains MUST use RatingDomainFactory to get interfaces
// 3. External domains MUST work through RatingReader/RatingMutator interfaces
// 4. NEVER access OrderRatingRepository directly from external domains
//
// VIOLATION EXAMPLES (DO NOT DO THIS):
// ❌ import "github.com/labuda/backend/internal/commerce/order/rating/application"
// ❌ service := application.NewRatingService()
// ❌ repo := repository.NewOrderRatingRepository()
//
// CORRECT USAGE:
// ✅ factory := ratingApp.NewRatingDomainFactory()
// ✅ reader := factory.GetReader()
// ✅ mutator := factory.GetMutator()
//
// ARCHITECTURAL ENFORCEMENT:
// - This file implements RatingReader and RatingMutator interfaces
// - External code only sees interfaces, not concrete implementation
// - Direct SQL access to order_ratings is BLOCKED by design
//
// If you need rating functionality:
// -> Use RatingDomainFactory in your domain
// -> Request appropriate interface (Reader/Mutator)
// -> Document why you need access in your PR


const (
	// Default limit for pagination
	defaultLimit = 20
	// Maximum limit for pagination
	maxLimit = 50
)

// RatingService handles rating operations for completed orders.
//
// INVARIANTS:
// - Buyer-only: Only buyer can rate seller
// - Order must be completed
// - One rating per order
// - Immutable: No update or delete operations
// - No financial mutations
// - No side effects on other domains
type RatingService struct {
	ratingRepo *repository.OrderRatingRepository
	orderRepo  *repository.OrderQueryRepository
}

// NewRatingService creates a new RatingService.
func NewRatingService() *RatingService {
	return &RatingService{
		ratingRepo: repository.NewOrderRatingRepository(),
		orderRepo:  repository.NewOrderQueryRepository(),
	}
}

// CreateRatingInput contains the parameters for creating a rating.
type CreateRatingInput struct {
	OrderID     uuid.UUID
	CallerID    uuid.UUID // User making the request (must be the buyer)
	RatingValue int       // 1-5
	Comment     *string   // Optional comment
}

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
func (s *RatingService) CreateRating(
	ctx context.Context,
	tx db.Tx,
	input CreateRatingInput,
) (*ratingEntity.OrderRating, error) {
	// Step 1: Lock order FOR UPDATE to prevent concurrent rating
	order, err := s.orderRepo.GetForUpdate(ctx, tx, input.OrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to lock order: %w", err)
	}

	// Step 2: Validate order exists
	if order == nil {
		return nil, &ratingEntity.ErrOrderNotFound{OrderID: input.OrderID}
	}

	// Step 3: Validate order status is completed
	// RATING SAFETY: This single check prevents multiple threats:
	// - ❌ Rating pending/paid/shipped orders
	// - ❌ Rating cancelled orders
	// - ❌ Rating refunded orders (StatusRefunded/StatusPartiallyRefunded)
	// - ❌ Rating expired orders
	// Only "completed" orders can be rated
	if order.Status != "completed" {
		return nil, &ratingEntity.ErrOrderNotCompleted{
			OrderID:   order.ID,
			Status:    order.Status,
			OrderType: order.OrderType,
		}
	}

	// Step 3.5: 🔥 TASK 4: DISPUTE EXCLUSION
	// Prevent rating orders with active disputes to avoid bias
	if order.HasDispute {
		return nil, &ratingEntity.ErrOrderHasActiveDispute{
			OrderID: order.ID,
		}
	}

	// Step 4: Validate caller is the buyer
	if order.BuyerID != input.CallerID {
		return nil, &ratingEntity.ErrNotBuyer{
			OrderID:  order.ID,
			CallerID: input.CallerID,
			BuyerID:  order.BuyerID,
		}
	}

	// Step 5: Check for existing rating (double-guard, also enforced by UNIQUE constraint)
	// RATING SAFETY: This check prevents re-rating even if previous rating was invalidated.
	// Use case: If order was rated → refunded (rating invalidated) → user tries to rate again
	// Result: ErrAlreadyRated (prevents rating farming via refund/re-rate cycle)
	existingRating, err := s.ratingRepo.GetByOrderID(ctx, tx, input.OrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing rating: %w", err)
	}
	if existingRating != nil {
		return nil, &ratingEntity.ErrAlreadyRated{OrderID: input.OrderID}
	}

	// Step 6: Create rating entity with validation
	rating, err := ratingEntity.NewOrderRating(
		input.OrderID,
		order.BuyerID,
		order.SellerID,
		input.RatingValue,
		input.Comment,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rating entity: %w", err)
	}

	// Step 7: Persist rating
	if err := s.ratingRepo.Create(ctx, tx, rating); err != nil {
		return nil, fmt.Errorf("failed to persist rating: %w", err)
	}

	// No outbox event - rating is a standalone operation with no side effects
	// No order state modification - rating is optional and does not affect order

	return rating, nil
}

// GetRatingByOrder retrieves a rating by order ID.
// Returns nil if no rating exists for the order.
func (s *RatingService) GetRatingByOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*ratingEntity.OrderRating, error) {
	rating, err := s.ratingRepo.GetByOrderID(ctx, tx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get rating: %w", err)
	}
	return rating, nil
}

// ListRatingsGivenByBuyerInput contains parameters for listing ratings given by a buyer.
type ListRatingsGivenByBuyerInput struct {
	BuyerID uuid.UUID
	Limit   int    // Optional, defaults to 20, max 50
	Cursor  int64  // Optional Unix timestamp in nanoseconds
}

// ListRatingsGivenByBuyer retrieves ratings given by a buyer with cursor-based pagination.
// Returns ratings ordered by created_at DESC.
func (s *RatingService) ListRatingsGivenByBuyer(
	ctx context.Context,
	tx db.Tx,
	input ListRatingsGivenByBuyerInput,
) ([]*ratingEntity.OrderRating, error) {
	limit := normalizeLimit(input.Limit)
	ratings, err := s.ratingRepo.ListByBuyer(ctx, tx, input.BuyerID, limit, input.Cursor)
	if err != nil {
		return nil, fmt.Errorf("failed to list ratings given by buyer: %w", err)
	}
	return ratings, nil
}

// ListRatingsReceivedBySellerInput contains parameters for listing ratings received by a seller.
type ListRatingsReceivedBySellerInput struct {
	SellerID uuid.UUID
	Limit    int   // Optional, defaults to 20, max 50
	Cursor   int64 // Optional Unix timestamp in nanoseconds
}

// ListRatingsReceivedBySeller retrieves ratings received by a seller with cursor-based pagination.
// Returns ratings ordered by created_at DESC.
func (s *RatingService) ListRatingsReceivedBySeller(
	ctx context.Context,
	tx db.Tx,
	input ListRatingsReceivedBySellerInput,
) ([]*ratingEntity.OrderRating, error) {
	limit := normalizeLimit(input.Limit)
	ratings, err := s.ratingRepo.ListBySeller(ctx, tx, input.SellerID, limit, input.Cursor)
	if err != nil {
		return nil, fmt.Errorf("failed to list ratings received by seller: %w", err)
	}
	return ratings, nil
}

// normalizeLimit ensures limit is within acceptable bounds.
func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

// GetRatingSummary retrieves the aggregated rating summary for a seller.
// Returns total count, average rating, and distribution by star rating.
//
// RATING INVALIDATION: Only includes valid ratings (invalidated_at IS NULL).
// Invalidated ratings from refunded orders are excluded from the summary.
func (s *RatingService) GetRatingSummary(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (*repository.RatingSummary, error) {
	summary, err := s.ratingRepo.GetRatingSummary(ctx, tx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get rating summary: %w", err)
	}
	return summary, nil
}

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
func (s *RatingService) InvalidateForOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	return s.ratingRepo.InvalidateForOrder(ctx, tx, orderID)
}

// GetAverageRatingForPeriod calculates the average rating for a seller within a time period.
// Returns 0.0 if no ratings exist for the period.
//
// RATING INVALIDATION: Only counts valid ratings (invalidated_at IS NULL).
// Invalidated ratings from refunded orders are excluded from aggregation.
//
// This method enforces the rating domain boundary by providing a service method
// for time-based rating aggregation instead of allowing direct SQL access.
func (s *RatingService) GetAverageRatingForPeriod(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	periodStart time.Time,
	periodEnd time.Time,
) (float64, error) {
	return s.ratingRepo.GetAverageRatingForPeriod(ctx, tx, sellerID, periodStart, periodEnd)
}


