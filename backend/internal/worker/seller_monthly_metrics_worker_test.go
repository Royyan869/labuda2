package worker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// TEST HELPERS
// =============================================================================

// countTx returns a fixed integer from QueryRow (for COUNT(*) queries).
type countTx struct {
	mockTx
	count int
}

func (m *countTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRow{values: []any{m.count}}
}

// metricsQueryTx returns a full metrics row from QueryRow (for GetSellerMetricsForPeriod).
type metricsQueryTx struct {
	mockTx
	sellerID              uuid.UUID
	year                  int
	month                 int
	totalItemsSold        int
	averageRating         float64
	fulfilledCount        int
	cancelledTimeoutCount int
}

func (m *metricsQueryTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRow{values: []any{
		m.sellerID,
		m.year,
		m.month,
		m.totalItemsSold,
		m.averageRating,
		m.fulfilledCount,
		m.cancelledTimeoutCount,
	}}
}

func fixedTime() time.Time {
	return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
}

// =============================================================================
// TESTS: aggregateFulfilledCount
// =============================================================================

func TestAggregateFulfilledCount_ReturnsCorrectCount(t *testing.T) {
	tx := &countTx{count: 15}
	w := &SellerMonthlyMetricsWorker{log: zaptest.NewLogger(t)}

	count, err := w.aggregateFulfilledCount(context.Background(), tx, uuid.New(), fixedTime(), fixedTime().AddDate(0, 1, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 15 {
		t.Errorf("expected 15, got %d", count)
	}
}

func TestAggregateFulfilledCount_ReturnsZero(t *testing.T) {
	tx := &countTx{count: 0}
	w := &SellerMonthlyMetricsWorker{log: zaptest.NewLogger(t)}

	count, err := w.aggregateFulfilledCount(context.Background(), tx, uuid.New(), fixedTime(), fixedTime().AddDate(0, 1, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

// =============================================================================
// TESTS: aggregateCancelledTimeoutCount
// =============================================================================

func TestAggregateCancelledTimeoutCount_ReturnsCorrectCount(t *testing.T) {
	tx := &countTx{count: 3}
	w := &SellerMonthlyMetricsWorker{log: zaptest.NewLogger(t)}

	count, err := w.aggregateCancelledTimeoutCount(context.Background(), tx, uuid.New(), fixedTime(), fixedTime().AddDate(0, 1, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestAggregateCancelledTimeoutCount_ReturnsZero(t *testing.T) {
	tx := &countTx{count: 0}
	w := &SellerMonthlyMetricsWorker{log: zaptest.NewLogger(t)}

	count, err := w.aggregateCancelledTimeoutCount(context.Background(), tx, uuid.New(), fixedTime(), fixedTime().AddDate(0, 1, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

// =============================================================================
// TESTS: GetSellerMetricsForPeriod (reads new columns)
// =============================================================================

func TestGetSellerMetricsForPeriod_ReturnsNewFields(t *testing.T) {
	sellerID := uuid.New()
	mdb := &mockDB{
		WithTxFunc: func(_ context.Context, fn func(tx db.Tx) error) error {
			return fn(&metricsQueryTx{
				sellerID:              sellerID,
				year:                  2026,
				month:                 5,
				totalItemsSold:        50,
				averageRating:         4.5,
				fulfilledCount:        42,
				cancelledTimeoutCount: 8,
			})
		},
	}
	w := &SellerMonthlyMetricsWorker{db: mdb, log: zaptest.NewLogger(t)}

	result, err := w.GetSellerMetricsForPeriod(context.Background(), sellerID, 2026, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.FulfilledCount != 42 {
		t.Errorf("expected FulfilledCount=42, got %d", result.FulfilledCount)
	}
	if result.CancelledTimeoutCount != 8 {
		t.Errorf("expected CancelledTimeoutCount=8, got %d", result.CancelledTimeoutCount)
	}
	if result.TotalItemsSold != 50 {
		t.Errorf("expected TotalItemsSold=50, got %d", result.TotalItemsSold)
	}
	if result.AverageRating != 4.5 {
		t.Errorf("expected AverageRating=4.5, got %f", result.AverageRating)
	}
}

func TestGetSellerMetricsForPeriod_MixedOrders_FulfillmentRate(t *testing.T) {
	sellerID := uuid.New()
	mdb := &mockDB{
		WithTxFunc: func(_ context.Context, fn func(tx db.Tx) error) error {
			return fn(&metricsQueryTx{
				sellerID:              sellerID,
				year:                  2026,
				month:                 5,
				totalItemsSold:        100,
				averageRating:         4.2,
				fulfilledCount:        7,
				cancelledTimeoutCount: 3,
			})
		},
	}
	w := &SellerMonthlyMetricsWorker{db: mdb, log: zaptest.NewLogger(t)}

	result, err := w.GetSellerMetricsForPeriod(context.Background(), sellerID, 2026, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that entity FulfillmentRate works from aggregation data
	entity := &sellerEntity.SellerMonthlyMetric{
		FulfilledCount:        result.FulfilledCount,
		CancelledTimeoutCount: result.CancelledTimeoutCount,
	}
	rate := entity.FulfillmentRate()
	if rate < 0.699 || rate > 0.701 {
		t.Errorf("expected ~0.7 fulfillment rate, got %f", rate)
	}
}

// =============================================================================
// TESTS: SQL query correctness (verify the right status and timestamp column)
// =============================================================================

func TestFulfilledCountQuery_UsesCompletedStatusAndCompletedAt(t *testing.T) {
	var capturedSQL string
	tx := &mockTx{
		QueryRowFunc: func(_ context.Context, sql string, _ ...any) pgx.Row {
			capturedSQL = sql
			return &mockRow{values: []any{0}}
		},
	}
	w := &SellerMonthlyMetricsWorker{log: zaptest.NewLogger(t)}

	_, _ = w.aggregateFulfilledCount(context.Background(), tx, uuid.New(), fixedTime(), fixedTime().AddDate(0, 1, 0))

	if !strings.Contains(capturedSQL, "status = 'completed'") {
		t.Errorf("expected SQL to filter by status='completed', got: %s", capturedSQL)
	}
	if !strings.Contains(capturedSQL, "completed_at") {
		t.Errorf("expected SQL to use completed_at for time bucketing, got: %s", capturedSQL)
	}
}

func TestCancelledTimeoutCountQuery_UsesCancelledTimeoutStatusAndUpdatedAt(t *testing.T) {
	var capturedSQL string
	tx := &mockTx{
		QueryRowFunc: func(_ context.Context, sql string, _ ...any) pgx.Row {
			capturedSQL = sql
			return &mockRow{values: []any{0}}
		},
	}
	w := &SellerMonthlyMetricsWorker{log: zaptest.NewLogger(t)}

	_, _ = w.aggregateCancelledTimeoutCount(context.Background(), tx, uuid.New(), fixedTime(), fixedTime().AddDate(0, 1, 0))

	if !strings.Contains(capturedSQL, "status = 'cancelled_timeout'") {
		t.Errorf("expected SQL to filter by status='cancelled_timeout', got: %s", capturedSQL)
	}
	if !strings.Contains(capturedSQL, "updated_at") {
		t.Errorf("expected SQL to use updated_at for time bucketing, got: %s", capturedSQL)
	}
}

// =============================================================================
// TESTS: CancelledTimeout is NOT confused with Cancelled
// =============================================================================

func TestCancelledTimeoutCountQuery_DoesNotMatchPlainCancelled(t *testing.T) {
	var capturedSQL string
	tx := &mockTx{
		QueryRowFunc: func(_ context.Context, sql string, _ ...any) pgx.Row {
			capturedSQL = sql
			return &mockRow{values: []any{0}}
		},
	}
	w := &SellerMonthlyMetricsWorker{log: zaptest.NewLogger(t)}

	_, _ = w.aggregateCancelledTimeoutCount(context.Background(), tx, uuid.New(), fixedTime(), fixedTime().AddDate(0, 1, 0))

	// The SQL must use the exact string 'cancelled_timeout', not just 'cancelled'
	if !strings.Contains(capturedSQL, "'cancelled_timeout'") {
		t.Errorf("SQL must use exact 'cancelled_timeout' literal, got: %s", capturedSQL)
	}
}


