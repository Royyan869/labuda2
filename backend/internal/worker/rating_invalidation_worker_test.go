package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	ratingApp "github.com/labuda/backend/internal/commerce/order/rating/application"
	ratingEntity "github.com/labuda/backend/internal/commerce/order/rating/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// TEST HELPERS
// =============================================================================

// mockRatingMutator implements ratingApp.RatingMutator for testing.
// Only InvalidateForOrder is exercised by the worker; CreateRating is a no-op stub.
type mockRatingMutator struct {
	invalidateCalls []uuid.UUID
	invalidateErr   error
}

func (m *mockRatingMutator) CreateRating(
	_ context.Context, _ db.Tx, _ ratingApp.CreateRatingInput,
) (*ratingEntity.OrderRating, error) {
	return nil, nil
}

func (m *mockRatingMutator) InvalidateForOrder(
	_ context.Context, _ db.Tx, orderID uuid.UUID,
) error {
	m.invalidateCalls = append(m.invalidateCalls, orderID)
	return m.invalidateErr
}

// scanTx returns the given order IDs from Query and panics on any other method.
type ratingInvalidationScanTx struct {
	mockTx
	orderIDs []uuid.UUID
}

func (m *ratingInvalidationScanTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	rows := make([][]any, len(m.orderIDs))
	for i, id := range m.orderIDs {
		rows[i] = []any{id}
	}
	return &mockRows{rows: rows}, nil
}

// mutateTx returns the given status string from QueryRow.
type ratingInvalidationMutateTx struct {
	mockTx
	orderStatus string
}

func (m *ratingInvalidationMutateTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRow{values: []any{m.orderStatus}}
}

// newRatingTestDB builds a mockDB that serves a scan call then per-order calls.
//
//   - Call 0  → scan: returns orderIDs from Query
//   - Call 1+ → per-order mutation: returns statuses[i-1] from QueryRow
//
// statuses must have at least len(orderIDs) entries.
func newRatingTestDB(orderIDs []uuid.UUID, statuses []string) *mockDB {
	callCount := 0
	return &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			curr := callCount
			callCount++
			if curr == 0 {
				return fn(&ratingInvalidationScanTx{orderIDs: orderIDs})
			}
			orderIdx := curr - 1
			var status string
			if orderIdx < len(statuses) {
				status = statuses[orderIdx]
			}
			return fn(&ratingInvalidationMutateTx{orderStatus: status})
		},
	}
}

// newRatingEmptyDB returns a mockDB whose scan always returns zero rows.
func newRatingEmptyDB() *mockDB {
	return &mockDB{
		WithTxFunc: func(_ context.Context, fn func(tx db.Tx) error) error {
			return fn(&ratingInvalidationScanTx{orderIDs: nil})
		},
	}
}

// =============================================================================
// CONSTRUCTION
// =============================================================================

func TestRatingInvalidationWorker_Construction(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewRatingInvalidationWorker(nil, log, nil)
	if w == nil {
		t.Fatal("NewRatingInvalidationWorker() returned nil")
	}
	if w.checkInterval != DefaultRatingInvalidationInterval {
		t.Errorf("checkInterval = %v, want %v", w.checkInterval, DefaultRatingInvalidationInterval)
	}
	if w.batchSize != DefaultRatingInvalidationBatchSize {
		t.Errorf("batchSize = %d, want %d", w.batchSize, DefaultRatingInvalidationBatchSize)
	}
}

func TestRatingInvalidationWorker_NilLoggerFallsBackToNop(t *testing.T) {
	// Must not panic with nil logger.
	w := NewRatingInvalidationWorker(nil, nil, nil)
	if w == nil {
		t.Fatal("NewRatingInvalidationWorker(nil logger) returned nil")
	}
}

// =============================================================================
// LIFECYCLE
// =============================================================================

func TestRatingInvalidationWorker_IsRunningInitiallyFalse(t *testing.T) {
	w := NewRatingInvalidationWorker(nil, zaptest.NewLogger(t), nil)
	if w.IsRunning() {
		t.Fatal("must not be running before Start()")
	}
}

func TestRatingInvalidationWorker_StopBeforeStartIsSafe(t *testing.T) {
	w := NewRatingInvalidationWorker(nil, zaptest.NewLogger(t), nil)
	w.Stop() // must not panic
	if w.IsRunning() {
		t.Error("IsRunning() should still be false after Stop() on unstarted worker")
	}
}

func TestRatingInvalidationWorker_StartStop(t *testing.T) {
	w := NewRatingInvalidationWorker(
		newRatingEmptyDB(),
		zaptest.NewLogger(t),
		&mockRatingMutator{},
	)
	// Override interval so the goroutine doesn't tick during the test.
	w.checkInterval = 10 * time.Second

	if w.IsRunning() {
		t.Fatal("must not be running before Start()")
	}

	w.Start()
	if !w.IsRunning() {
		t.Fatal("must be running after Start()")
	}

	w.Stop()
	if w.IsRunning() {
		t.Fatal("must not be running after Stop()")
	}
}

func TestRatingInvalidationWorker_DoubleStartIsSafe(t *testing.T) {
	w := NewRatingInvalidationWorker(
		newRatingEmptyDB(),
		zaptest.NewLogger(t),
		&mockRatingMutator{},
	)
	w.checkInterval = 10 * time.Second

	w.Start()
	w.Start() // must not panic or launch a second goroutine
	if !w.IsRunning() {
		t.Fatal("must still be running after double Start()")
	}
	w.Stop()
}

// =============================================================================
// SCENARIO: no orders needing invalidation
// =============================================================================

// TestRatingInvalidationWorker_NoOrders proves that a cycle with zero matching
// orders completes without calling InvalidateForOrder.
func TestRatingInvalidationWorker_NoOrders(t *testing.T) {
	mutator := &mockRatingMutator{}
	w := NewRatingInvalidationWorker(newRatingEmptyDB(), zaptest.NewLogger(t), mutator)

	w.cleanupOnce(context.Background())

	if len(mutator.invalidateCalls) != 0 {
		t.Errorf("InvalidateForOrder called %d times, want 0", len(mutator.invalidateCalls))
	}
}

// =============================================================================
// SCENARIO: refunded order → rating must be invalidated
// =============================================================================

// TestRatingInvalidationWorker_RefundedOrder proves that a refunded order with a
// valid rating triggers exactly one InvalidateForOrder call.
func TestRatingInvalidationWorker_RefundedOrder(t *testing.T) {
	orderID := uuid.New()
	mutator := &mockRatingMutator{}

	w := NewRatingInvalidationWorker(
		newRatingTestDB([]uuid.UUID{orderID}, []string{"refunded"}),
		zaptest.NewLogger(t),
		mutator,
	)

	w.cleanupOnce(context.Background())

	if len(mutator.invalidateCalls) != 1 {
		t.Fatalf("InvalidateForOrder call count = %d, want 1", len(mutator.invalidateCalls))
	}
	if mutator.invalidateCalls[0] != orderID {
		t.Errorf("InvalidateForOrder called with %v, want %v", mutator.invalidateCalls[0], orderID)
	}
}

// =============================================================================
// SCENARIO: partially_refunded order → rating must be invalidated
// =============================================================================

// TestRatingInvalidationWorker_PartiallyRefundedOrder proves that a
// partially_refunded order is treated the same as a fully refunded order.
func TestRatingInvalidationWorker_PartiallyRefundedOrder(t *testing.T) {
	orderID := uuid.New()
	mutator := &mockRatingMutator{}

	w := NewRatingInvalidationWorker(
		newRatingTestDB([]uuid.UUID{orderID}, []string{"partially_refunded"}),
		zaptest.NewLogger(t),
		mutator,
	)

	w.cleanupOnce(context.Background())

	if len(mutator.invalidateCalls) != 1 {
		t.Fatalf("InvalidateForOrder call count = %d, want 1", len(mutator.invalidateCalls))
	}
	if mutator.invalidateCalls[0] != orderID {
		t.Errorf("InvalidateForOrder called with %v, want %v", mutator.invalidateCalls[0], orderID)
	}
}

// =============================================================================
// SCENARIO: multiple orders in one batch
// =============================================================================

// TestRatingInvalidationWorker_MultiplOrders proves that all orders in a batch
// are processed and InvalidateForOrder is called once per order.
func TestRatingInvalidationWorker_MultipleOrders(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	mutator := &mockRatingMutator{}

	w := NewRatingInvalidationWorker(
		newRatingTestDB(
			[]uuid.UUID{id1, id2},
			[]string{"refunded", "partially_refunded"},
		),
		zaptest.NewLogger(t),
		mutator,
	)

	w.cleanupOnce(context.Background())

	if len(mutator.invalidateCalls) != 2 {
		t.Fatalf("InvalidateForOrder call count = %d, want 2", len(mutator.invalidateCalls))
	}
}

// =============================================================================
// SCENARIO: non-refunded order in scan result (status guard)
// =============================================================================

// TestRatingInvalidationWorker_StatusGuardSkipsNonRefunded proves that if an order
// somehow appears in the scan but has a non-refunded status (e.g., race condition),
// the status guard inside invalidateRatingForOrder rejects it without calling
// InvalidateForOrder.
func TestRatingInvalidationWorker_StatusGuardSkipsNonRefunded(t *testing.T) {
	orderID := uuid.New()
	mutator := &mockRatingMutator{}

	w := NewRatingInvalidationWorker(
		newRatingTestDB([]uuid.UUID{orderID}, []string{"completed"}),
		zaptest.NewLogger(t),
		mutator,
	)

	w.cleanupOnce(context.Background())

	if len(mutator.invalidateCalls) != 0 {
		t.Errorf("InvalidateForOrder should not be called for completed order, got %d calls",
			len(mutator.invalidateCalls))
	}
}

// =============================================================================
// SCENARIO: duplicate run → idempotent
// =============================================================================

// TestRatingInvalidationWorker_DuplicateRunIsIdempotent proves that running
// cleanupOnce twice is safe:
//   - First run: scan finds order, invalidates.
//   - Second run: scan finds nothing (already invalidated → query returns empty).
//
// This mirrors the real DB behaviour where the scan SQL filters invalidated_at IS NULL.
func TestRatingInvalidationWorker_DuplicateRunIsIdempotent(t *testing.T) {
	orderID := uuid.New()
	mutator := &mockRatingMutator{}

	// First run: scan returns the order, status = refunded.
	callCount := 0
	firstRunDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			curr := callCount
			callCount++
			if curr == 0 {
				return fn(&ratingInvalidationScanTx{orderIDs: []uuid.UUID{orderID}})
			}
			return fn(&ratingInvalidationMutateTx{orderStatus: "refunded"})
		},
	}

	w := NewRatingInvalidationWorker(firstRunDB, zaptest.NewLogger(t), mutator)
	w.cleanupOnce(context.Background())

	if len(mutator.invalidateCalls) != 1 {
		t.Fatalf("first run: InvalidateForOrder call count = %d, want 1", len(mutator.invalidateCalls))
	}

	// Second run: scan returns empty (rating is now invalidated in real DB).
	// Swap the DB to return an empty scan.
	w.db = newRatingEmptyDB()
	w.cleanupOnce(context.Background())

	// Total call count must still be 1 — second run added nothing.
	if len(mutator.invalidateCalls) != 1 {
		t.Errorf("second run: InvalidateForOrder call count = %d, want still 1 (idempotent)",
			len(mutator.invalidateCalls))
	}
}

// =============================================================================
// SCENARIO: InvalidateForOrder error → log, continue, no panic
// =============================================================================

// TestRatingInvalidationWorker_InvalidateErrorContinues proves that a failure in
// InvalidateForOrder for one order does not panic and does not block other orders.
func TestRatingInvalidationWorker_InvalidateErrorContinues(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()

	// id1 will get an error from InvalidateForOrder; id2 should still be called.
	callCount := 0
	testDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			curr := callCount
			callCount++
			if curr == 0 {
				return fn(&ratingInvalidationScanTx{orderIDs: []uuid.UUID{id1, id2}})
			}
			return fn(&ratingInvalidationMutateTx{orderStatus: "refunded"})
		},
	}

	mutator := &mockRatingMutator{
		// First call fails, second succeeds.
		invalidateErr: nil, // will be overridden per call below
	}

	// Override mutator to fail on the first order only.
	failOnce := &failOnceMutator{target: id1}
	w := NewRatingInvalidationWorker(testDB, zaptest.NewLogger(t), failOnce)

	// Must not panic.
	w.cleanupOnce(context.Background())

	// id2 must still have been processed even though id1 failed.
	if !failOnce.id2Called {
		t.Error("id2 was not processed after id1 failure — cleanupOnce must continue on error")
	}

	_ = mutator // silence unused warning
}

// failOnceMutator fails InvalidateForOrder for target and tracks whether id2 was called.
type failOnceMutator struct {
	target    uuid.UUID
	id2Called bool
}

func (m *failOnceMutator) CreateRating(
	_ context.Context, _ db.Tx, _ ratingApp.CreateRatingInput,
) (*ratingEntity.OrderRating, error) {
	return nil, nil
}

func (m *failOnceMutator) InvalidateForOrder(
	_ context.Context, _ db.Tx, orderID uuid.UUID,
) error {
	if orderID == m.target {
		return errors.New("simulated gateway error")
	}
	m.id2Called = true
	return nil
}

// =============================================================================
// SCENARIO: missing rating (no rows) must not panic
// =============================================================================

// TestRatingInvalidationWorker_MissingRatingNoError proves that if the scan returns
// an order ID but the rating does not actually exist, the worker does not panic.
//
// In practice the scan JOIN ensures a rating exists, but this guards against
// concurrent deletes or schema anomalies.
func TestRatingInvalidationWorker_MissingOrderNoError(t *testing.T) {
	orderID := uuid.New()
	mutator := &mockRatingMutator{}

	// The scan finds the order, but QueryRow returns an error (order not found).
	callCount := 0
	testDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			curr := callCount
			callCount++
			if curr == 0 {
				return fn(&ratingInvalidationScanTx{orderIDs: []uuid.UUID{orderID}})
			}
			// QueryRow returns an error (no rows).
			return fn(&mockTx{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRow{err: errors.New("no rows in result set")}
				},
			})
		},
	}

	w := NewRatingInvalidationWorker(testDB, zaptest.NewLogger(t), mutator)

	// Must not panic; error is logged and skipped.
	w.cleanupOnce(context.Background())

	if len(mutator.invalidateCalls) != 0 {
		t.Errorf("InvalidateForOrder should not be called when order is missing, got %d calls",
			len(mutator.invalidateCalls))
	}
}

// =============================================================================
// SCENARIO: default constants
// =============================================================================

func TestRatingInvalidationWorker_DefaultConstants(t *testing.T) {
	if DefaultRatingInvalidationInterval != 5*time.Minute {
		t.Errorf("DefaultRatingInvalidationInterval = %v, want 5m", DefaultRatingInvalidationInterval)
	}
	if DefaultRatingInvalidationBatchSize != 100 {
		t.Errorf("DefaultRatingInvalidationBatchSize = %d, want 100", DefaultRatingInvalidationBatchSize)
	}
}


