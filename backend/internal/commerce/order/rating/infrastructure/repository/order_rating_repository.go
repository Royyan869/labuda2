package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	ratingEntity "github.com/labuda/backend/internal/commerce/order/rating/entity"
	"github.com/labuda/backend/pkg/db"
)

// OrderRatingRepository handles order rating persistence using pgx-based DB layer.
// Enforces uniqueness constraint via UNIQUE(order_id).
type OrderRatingRepository struct{}

// NewOrderRatingRepository creates a new OrderRatingRepository.
func NewOrderRatingRepository() *OrderRatingRepository {
	return &OrderRatingRepository{}
}

// Create persists a new order rating within a transaction.
// Returns ratingEntity.ErrAlreadyRated if a rating already exists for the order.
func (r *OrderRatingRepository) Create(
	ctx context.Context,
	tx db.Tx,
	rating *ratingEntity.OrderRating,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO order_ratings (
			id, order_id, buyer_id, seller_id, rating_value, comment, created_at, invalidated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		rating.ID,
		rating.OrderID,
		rating.BuyerID,
		rating.SellerID,
		rating.RatingValue,
		rating.Comment,
		rating.CreatedAt,
		rating.InvalidatedAt,
	)

	if err != nil {
		// Check for UNIQUE violation on order_id
		if isUniqueViolationError(err) {
			return &ratingEntity.ErrAlreadyRated{OrderID: rating.OrderID}
		}
		return fmt.Errorf("create order rating failed: %w", err)
	}

	return nil
}

// GetByOrderID retrieves a rating by order ID without locking.
// Returns nil if no rating exists for the order.
//
// RATING SAFETY: This method does NOT filter by invalidated_at IS NULL.
// It returns the rating regardless of validation state.
//
// Use cases:
// - Internal validation: Check if ANY rating exists (even invalidated)
// - NOT for public queries: Use ListByBuyer/ListBySeller instead
//
// IMPORTANT: This method is intentionally permissive for validation purposes.
// The UNIQUE constraint on order_id prevents multiple ratings, so we need
// to know if a rating exists at all, even if it was invalidated.
func (r *OrderRatingRepository) GetByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*ratingEntity.OrderRating, error) {
	var id, buyerID, sellerID uuid.UUID
	var ratingValue int
	var comment *string
	var createdAt time.Time
	var invalidatedAt *time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, buyer_id, seller_id, rating_value, comment, created_at, invalidated_at
		FROM order_ratings
		WHERE order_id = $1
	`, orderID).Scan(
		&id, &buyerID, &sellerID, &ratingValue, &comment, &createdAt, &invalidatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // No rating exists for this order
		}
		return nil, fmt.Errorf("get order rating failed: %w", err)
	}

	return &ratingEntity.OrderRating{
		ID:            id,
		OrderID:       orderID,
		BuyerID:       buyerID,
		SellerID:      sellerID,
		RatingValue:   ratingValue,
		Comment:       comment,
		CreatedAt:     createdAt,
		InvalidatedAt: invalidatedAt,
	}, nil
}

// ListByBuyer retrieves ratings given by a buyer with cursor-based pagination.
// Cursor is Unix timestamp in nanoseconds (from created_at).
// Returns ratings ordered by created_at DESC.
//
// RATING INVALIDATION: Only returns valid ratings (invalidated_at IS NULL).
// Invalidated ratings from refunded orders are excluded from the list.
func (r *OrderRatingRepository) ListByBuyer(
	ctx context.Context,
	tx db.Tx,
	buyerID uuid.UUID,
	limit int,
	cursor int64,
) ([]*ratingEntity.OrderRating, error) {
	query := `
		SELECT id, order_id, buyer_id, seller_id, rating_value, comment, created_at, invalidated_at
		FROM order_ratings
		WHERE buyer_id = $1
		AND invalidated_at IS NULL
	`
	args := []any{buyerID}
	argPos := 2

	// Add cursor filter if provided
	if cursor > 0 {
		query += fmt.Sprintf(" AND created_at < $%d", argPos)
		args = append(args, time.Unix(0, cursor).UTC())
		argPos++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argPos)
	args = append(args, limit)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ratings by buyer failed: %w", err)
	}
	defer rows.Close()

	var ratings []*ratingEntity.OrderRating
	for rows.Next() {
		var id, orderID, buyerIDRow, sellerID uuid.UUID
		var ratingValue int
		var comment *string
		var createdAt time.Time
		var invalidatedAt *time.Time

		if err := rows.Scan(
			&id, &orderID, &buyerIDRow, &sellerID, &ratingValue, &comment, &createdAt, &invalidatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order rating failed: %w", err)
		}

		ratings = append(ratings, &ratingEntity.OrderRating{
			ID:            id,
			OrderID:       orderID,
			BuyerID:       buyerIDRow,
			SellerID:      sellerID,
			RatingValue:   ratingValue,
			Comment:       comment,
			CreatedAt:     createdAt,
			InvalidatedAt: invalidatedAt,
		})
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate ratings failed: %w", rows.Err())
	}

	return ratings, nil
}

// ListBySeller retrieves ratings received by a seller with cursor-based pagination.
// Cursor is Unix timestamp in nanoseconds (from created_at).
// Returns ratings ordered by created_at DESC.
//
// RATING INVALIDATION: Only returns valid ratings (invalidated_at IS NULL).
// Invalidated ratings from refunded orders are excluded from the list.
func (r *OrderRatingRepository) ListBySeller(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	limit int,
	cursor int64,
) ([]*ratingEntity.OrderRating, error) {
	query := `
		SELECT id, order_id, buyer_id, seller_id, rating_value, comment, created_at, invalidated_at
		FROM order_ratings
		WHERE seller_id = $1
		AND invalidated_at IS NULL
	`
	args := []any{sellerID}
	argPos := 2

	// Add cursor filter if provided
	if cursor > 0 {
		query += fmt.Sprintf(" AND created_at < $%d", argPos)
		args = append(args, time.Unix(0, cursor).UTC())
		argPos++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argPos)
	args = append(args, limit)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ratings by seller failed: %w", err)
	}
	defer rows.Close()

	var ratings []*ratingEntity.OrderRating
	for rows.Next() {
		var id, orderID, buyerID, sellerIDRow uuid.UUID
		var ratingValue int
		var comment *string
		var createdAt time.Time
		var invalidatedAt *time.Time

		if err := rows.Scan(
			&id, &orderID, &buyerID, &sellerIDRow, &ratingValue, &comment, &createdAt, &invalidatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order rating failed: %w", err)
		}

		ratings = append(ratings, &ratingEntity.OrderRating{
			ID:            id,
			OrderID:       orderID,
			BuyerID:       buyerID,
			SellerID:      sellerIDRow,
			RatingValue:   ratingValue,
			Comment:       comment,
			CreatedAt:     createdAt,
			InvalidatedAt: invalidatedAt,
		})
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate ratings failed: %w", rows.Err())
	}

	return ratings, nil
}

// isUniqueViolationError checks if the error is a PostgreSQL UNIQUE constraint violation.
func isUniqueViolationError(err error) bool {
	if err == nil {
		return false
	}
	pgErr, ok := err.(*pgconn.PgError)
	return ok && pgErr.Code == "23505" // UNIQUE_VIOLATION
}

// RatingSummary represents the aggregated rating summary for a seller.
// The json tags lock the wire keys to snake_case (the Rating HTTP contract).
type RatingSummary struct {
	TotalRatings   int     `json:"total_ratings"`
	AverageRating  float64 `json:"average_rating"`
	OneStarCount   int     `json:"one_star_count"`
	TwoStarCount   int     `json:"two_star_count"`
	ThreeStarCount int     `json:"three_star_count"`
	FourStarCount  int     `json:"four_star_count"`
	FiveStarCount  int     `json:"five_star_count"`
}

// GetRatingSummary retrieves the aggregated rating summary for a seller.
// Returns 0.0 for average_rating and 0 for counts if no ratings exist.
//
// RATING INVALIDATION: Only includes valid ratings (invalidated_at IS NULL).
// Invalidated ratings from refunded orders are excluded from the summary.
func (r *OrderRatingRepository) GetRatingSummary(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (*RatingSummary, error) {
	var summary RatingSummary

	query := `
		SELECT
			COUNT(*) as total_ratings,
			COALESCE(AVG(rating_value), 0.0) as average_rating,
			COUNT(*) FILTER (WHERE rating_value = 1) as one_star_count,
			COUNT(*) FILTER (WHERE rating_value = 2) as two_star_count,
			COUNT(*) FILTER (WHERE rating_value = 3) as three_star_count,
			COUNT(*) FILTER (WHERE rating_value = 4) as four_star_count,
			COUNT(*) FILTER (WHERE rating_value = 5) as five_star_count
		FROM order_ratings
		WHERE seller_id = $1
		  AND invalidated_at IS NULL
	`

	err := tx.QueryRow(ctx, query, sellerID).Scan(
		&summary.TotalRatings,
		&summary.AverageRating,
		&summary.OneStarCount,
		&summary.TwoStarCount,
		&summary.ThreeStarCount,
		&summary.FourStarCount,
		&summary.FiveStarCount,
	)

	if err != nil {
		return nil, fmt.Errorf("get rating summary failed: %w", err)
	}

	return &summary, nil
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
func (r *OrderRatingRepository) InvalidateForOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE order_ratings
		SET invalidated_at = NOW()
		WHERE order_id = $1 AND invalidated_at IS NULL
	`, orderID)

	if err != nil {
		return fmt.Errorf("invalidate rating for order failed: %w", err)
	}

	return nil
}

// GetAverageRatingForPeriod calculates the average rating for a seller within a time period.
// Returns 0.0 if no ratings exist for the period.
//
// RATING INVALIDATION: Only counts valid ratings (invalidated_at IS NULL).
// Invalidated ratings from refunded orders are excluded from aggregation.
//
// This method enforces the rating domain boundary by providing a repository method
// for time-based rating aggregation instead of allowing direct SQL access.
func (r *OrderRatingRepository) GetAverageRatingForPeriod(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	periodStart time.Time,
	periodEnd time.Time,
) (float64, error) {
	var averageRating float64

	query := `
		SELECT COALESCE(AVG(rating_value), 0.0)
		FROM order_ratings
		WHERE seller_id = $1
		  AND created_at >= $2
		  AND created_at < $3
		  AND invalidated_at IS NULL
	`

	err := tx.QueryRow(ctx, query, sellerID, periodStart, periodEnd).Scan(&averageRating)
	if err != nil {
		return 0.0, fmt.Errorf("get average rating for period failed: %w", err)
	}

	return averageRating, nil
}


