package entity

import (
	"time"

	"github.com/google/uuid"
)

// SellerReputationState is the LIVE REPUTATION AUTHORITY for a seller.
//
// This is NOT a historical snapshot. It is recomputed nightly by
// SellerReputationRecomputeWorker from the trailing WindowDays-day window
// of base tables (orders, order_ratings, refunds).
//
// Key design invariants:
//   - One row per seller (PRIMARY KEY on seller_id in DB)
//   - Overwritten on every recompute cycle (UPSERT semantics; never append-only)
//   - Clock: order.completed_at — NOT rating.created_at
//   - Late refunds/disputes/invalidations self-correct on the next nightly run
//
// Authority boundary:
//   - This is the SOLE input to tier evaluation logic
//   - seller_monthly_metrics is ANALYTICS ONLY and must NOT drive tier decisions
type SellerReputationState struct {
	SellerID                uuid.UUID
	WindowDays              int        // Trailing window length in days (canonical: 90)
	WindowStart             time.Time  // Evaluation window start: now - WindowDays
	WindowEnd               time.Time  // Evaluation window end: now (at recompute time)
	RollingCompletedOrders  int        // Completed orders in window (refunded orders excluded)
	RollingCancelledTimeout int        // Cancelled-timeout orders in window
	RollingRatingAverage    float64    // Avg valid ratings; clock = order.completed_at
	RollingRatingCount      int        // Count of valid (non-invalidated) ratings in window
	RollingDisputeLossCount int        // Admin-decided dispute losses in window
	RollingFulfillmentRate  float64    // Stored derived: completed / (completed + timeout)
	CurrentTier             Tier       // Tier as of TierLastEvaluatedAt
	TierLastEvaluatedAt     *time.Time // Nil if reputation state was written before tier eval
	ReputationUpdatedAt     time.Time  // Timestamp of last successful recompute
}

// FulfillmentRate computes the fulfillment rate from stored counts.
// Returns 0.0 when there are no accountable orders (avoids division by zero).
// Matches the stored rolling_fulfillment_rate column.
func (s *SellerReputationState) FulfillmentRate() float64 {
	total := s.RollingCompletedOrders + s.RollingCancelledTimeout
	if total == 0 {
		return 0.0
	}
	return float64(s.RollingCompletedOrders) / float64(total)
}

// HasSufficientActivity returns true if the seller has enough data in the
// rolling window for tier evaluation. Prevents sellers with zero activity
// from being promoted on stale data.
func (s *SellerReputationState) HasSufficientActivity() bool {
	return s.RollingCompletedOrders > 0 && s.RollingRatingCount > 0
}


