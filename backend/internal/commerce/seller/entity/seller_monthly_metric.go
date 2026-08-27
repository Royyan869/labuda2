package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SellerMonthlyMetric represents an immutable monthly performance snapshot.
//
// ANALYTICS ONLY — never used for tier evaluation.
// Tier decisions are made by SellerReputationRecomputeWorker which queries
// base tables (orders, order_ratings) directly in a rolling 90-day window.
//
// Written by SellerMonthlyMetricsWorker for measurement/analytics purposes:
// - Total items sold
// - Average rating
// - Fulfilled order count (completed orders)
// - Cancelled timeout order count (auto-cancelled due to shipment timeout)
//
// Invariants:
// - Immutable (no update-in-place)
// - One metric per seller per month (UNIQUE(seller_id, year, month))
// - Created at month-end by worker
type SellerMonthlyMetric struct {
	ID                    uuid.UUID
	SellerID              uuid.UUID
	Year                  int
	Month                 int
	TotalItemsSold        int
	AverageRating         float64
	FulfilledCount        int // Count of completed orders in the month
	CancelledTimeoutCount int // Count of cancelled_timeout orders in the month
	CreatedAt             time.Time
}

// FulfillmentRate returns the ratio of fulfilled orders to total accountable orders.
// Returns 0.0 when there are no accountable orders (avoids division by zero).
// Accountable orders = fulfilled + cancelled_timeout.
func (m *SellerMonthlyMetric) FulfillmentRate() float64 {
	total := m.FulfilledCount + m.CancelledTimeoutCount
	if total == 0 {
		return 0.0
	}
	return float64(m.FulfilledCount) / float64(total)
}

// Period returns the year-month as a string "YYYY-MM".
func (m *SellerMonthlyMetric) Period() string {
	return fmt.Sprintf("%04d-%02d", m.Year, m.Month)
}


