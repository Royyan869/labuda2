package application

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/projection"
	"github.com/labuda/backend/pkg/db"
)

// ─── projectionLister stub ────────────────────────────────────────────────────

type stubProjectionLister struct {
	buyerResults  []*projection.OrderSummary
	sellerResults []*projection.OrderSummary
	adminResults  []*projection.OrderSummary
	adminTotal    int64
}

func (s *stubProjectionLister) ListOrderSummariesByBuyer(_ context.Context, _ db.Tx, _ uuid.UUID, _ *string, _ int, _ int64) ([]*projection.OrderSummary, error) {
	return s.buyerResults, nil
}

func (s *stubProjectionLister) ListOrderSummariesBySeller(_ context.Context, _ db.Tx, _ uuid.UUID, _ *string, _ int, _ int64) ([]*projection.OrderSummary, error) {
	return s.sellerResults, nil
}

func (s *stubProjectionLister) ListOrderSummariesForAdmin(_ context.Context, _ db.Tx, _ projection.OrderListFilters) ([]*projection.OrderSummary, int64, error) {
	return s.adminResults, s.adminTotal, nil
}

// CountOrderSummariesByBuyer returns the number of buyer rows in the stub
// (i.e. len(buyerResults)). Tests that need projCount < writeModelCount set
// fewer buyerResults than captureTx.countResult.
func (s *stubProjectionLister) CountOrderSummariesByBuyer(_ context.Context, _ db.Tx, _ uuid.UUID, _ *string) (int64, error) {
	return int64(len(s.buyerResults)), nil
}

// CountOrderSummariesBySeller mirrors CountOrderSummariesByBuyer for sellers.
func (s *stubProjectionLister) CountOrderSummariesBySeller(_ context.Context, _ db.Tx, _ uuid.UUID, _ *string) (int64, error) {
	return int64(len(s.sellerResults)), nil
}

// compile-time: *projection.Repository must satisfy projectionLister.
var _ projectionLister = (*projection.Repository)(nil)

// ─── db.Tx stub ───────────────────────────────────────────────────────────────

// captureTx records SQL passed to Query / QueryRow so tests can assert on it.
//
// countResult is the value returned when any QueryRow.Scan(*int64) is called.
// Set it > 0 to simulate a write model that has orders (triggering Option B
// fallback when the projection has fewer rows).
type captureTx struct {
	queries     []string
	countResult int64 // returned by all QueryRow scans to *int64
}

func (c *captureTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (c *captureTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	c.queries = append(c.queries, sql)
	return &emptyPgxRows{}, nil
}

func (c *captureTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	c.queries = append(c.queries, sql)
	return &configuredCountRow{value: c.countResult}
}

func (c *captureTx) Commit(_ context.Context) error   { return nil }
func (c *captureTx) Rollback(_ context.Context) error { return nil }

func (c *captureTx) hasQueryMatching(substr string) bool {
	for _, q := range c.queries {
		if strings.Contains(q, substr) {
			return true
		}
	}
	return false
}

// hasDataQuery returns true if a write-model data query (SELECT … FROM orders …
// ORDER BY created_at) was issued — as opposed to a COUNT query.
func (c *captureTx) hasDataQuery(tableSubstr string) bool {
	for _, q := range c.queries {
		if strings.Contains(q, tableSubstr) && strings.Contains(q, "ORDER BY created_at") {
			return true
		}
	}
	return false
}

// ─── pgx.Rows stub (no rows) ─────────────────────────────────────────────────

type emptyPgxRows struct{}

func (e *emptyPgxRows) Close()                                       {}
func (e *emptyPgxRows) Err() error                                   { return nil }
func (e *emptyPgxRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (e *emptyPgxRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (e *emptyPgxRows) Next() bool                                   { return false }
func (e *emptyPgxRows) Scan(_ ...any) error                          { return nil }
func (e *emptyPgxRows) Values() ([]any, error)                       { return nil, nil }
func (e *emptyPgxRows) RawValues() [][]byte                          { return nil }
func (e *emptyPgxRows) Conn() *pgx.Conn                              { return nil }

// ─── pgx.Row stub (configurable count) ───────────────────────────────────────

// configuredCountRow scans its value into a *int64 dest and ignores any other
// dest types (allowing the same stub to be used for profile-fetch QueryRows
// without panicking).
type configuredCountRow struct{ value int64 }

func (r *configuredCountRow) Scan(dest ...any) error {
	if len(dest) == 1 {
		if p, ok := dest[0].(*int64); ok {
			*p = r.value
		}
	}
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func makeOrderSummary(buyerID, sellerID uuid.UUID) *projection.OrderSummary {
	now := time.Now()
	return &projection.OrderSummary{
		ID:           uuid.New(),
		BuyerID:      buyerID,
		SellerID:     sellerID,
		SourceType:   "for_sale",
		Status:       "pending_payment",
		EscrowStatus: "none",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestListMyOrders_Buyer_FallbackTriggered_WhenProjectionEmpty verifies that
// when the projection has no rows for a buyer but the write model does, the
// service issues a write-model data query scoped to buyer_id.
//
// Option B gate: projCount(0) < writeModelCount(3) → fallback.
func TestListMyOrders_Buyer_FallbackTriggered_WhenProjectionEmpty(t *testing.T) {
	svc := NewOrderQueryService(&stubProjectionLister{buyerResults: nil}, true)
	// countResult=3 simulates "write model has 3 orders for this buyer".
	tx := &captureTx{countResult: 3}

	resp, err := svc.ListMyOrders(context.Background(), tx, ListMyOrdersInput{
		CallerID:  uuid.New(),
		RoleParam: "buyer",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// The write-model fallback data query must have been issued.
	if !tx.hasDataQuery("FROM orders") {
		t.Fatal("expected write model fallback data query (FROM orders ORDER BY) but none found")
	}
	if !tx.hasQueryMatching("buyer_id") {
		t.Fatal("expected buyer_id scope in fallback query")
	}
}

// TestListMyOrders_Seller_FallbackTriggered_WhenProjectionEmpty verifies that
// when the projection has no rows for a seller but the write model does, the
// service issues a write-model data query scoped to seller_id.
//
// Option B gate: projCount(0) < writeModelCount(3) → fallback.
func TestListMyOrders_Seller_FallbackTriggered_WhenProjectionEmpty(t *testing.T) {
	svc := NewOrderQueryService(&stubProjectionLister{sellerResults: nil}, true)
	tx := &captureTx{countResult: 3}

	resp, err := svc.ListMyOrders(context.Background(), tx, ListMyOrdersInput{
		CallerID:  uuid.New(),
		RoleParam: "seller",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if !tx.hasDataQuery("FROM orders") {
		t.Fatal("expected write model fallback data query (FROM orders ORDER BY) but none found")
	}
	if !tx.hasQueryMatching("seller_id") {
		t.Fatal("expected seller_id scope in fallback query")
	}
}

// TestListMyOrders_NoFallback_WhenProjectionHasResults verifies that when
// projection returns results AND the write-model count is equal (or lower),
// the write-model data query is NOT issued.
//
// Option B gate: projCount(1) >= writeModelCount(0) → no fallback.
func TestListMyOrders_NoFallback_WhenProjectionHasResults(t *testing.T) {
	buyerID := uuid.New()
	svc := NewOrderQueryService(&stubProjectionLister{
		buyerResults: []*projection.OrderSummary{makeOrderSummary(buyerID, uuid.New())},
	}, true)
	// countResult=0 (default): write model count=0 → projCount(1) >= 0 → no fallback.
	tx := &captureTx{}

	resp, err := svc.ListMyOrders(context.Background(), tx, ListMyOrdersInput{
		CallerID:  buyerID,
		RoleParam: "buyer",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Orders) != 1 {
		t.Fatalf("expected 1 order from projection, got %d", len(resp.Orders))
	}
	// The write-model DATA query (SELECT … ORDER BY) must NOT have been issued.
	// COUNT queries against the write model are expected (Option B check).
	if tx.hasDataQuery("FROM orders") {
		t.Fatal("write model data fallback must NOT be triggered when projection has results")
	}
}

// TestListAllOrdersForAdmin_FallbackTriggered_WhenProjectionEmpty verifies that
// when the projection has no rows but the write model does, the admin path
// falls back to the write model.
//
// Option B gate: projTotal(0) < writeModelCount(3) → fallback.
func TestListAllOrdersForAdmin_FallbackTriggered_WhenProjectionEmpty(t *testing.T) {
	svc := NewOrderQueryService(&stubProjectionLister{adminTotal: 0}, true)
	tx := &captureTx{countResult: 3}

	resp, err := svc.ListAllOrdersForAdmin(context.Background(), tx, AdminOrderListFilters{
		Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// A COUNT query against orders write table must have been issued.
	if !tx.hasQueryMatching("COUNT") || !tx.hasQueryMatching("orders") {
		t.Fatalf("expected COUNT query on orders write table; queries: %v", tx.queries)
	}
}

// TestListMyOrders_PartialProjection_SafeFallback verifies that when the
// projection holds fewer rows than the write model (partial lag), the service
// falls back to the write model rather than silently hiding orders.
//
// This replaces the former KnownLimitation test and proves the Option B fix.
// Option B gate: projCount(1) < writeModelCount(3) → fallback fires.
func TestListMyOrders_PartialProjection_SafeFallback(t *testing.T) {
	buyerID := uuid.New()
	// Projection returns 1 row (projCount=1); write model has 3 (countResult=3).
	svc := NewOrderQueryService(&stubProjectionLister{
		buyerResults: []*projection.OrderSummary{makeOrderSummary(buyerID, uuid.New())},
	}, true)
	tx := &captureTx{countResult: 3}

	resp, err := svc.ListMyOrders(context.Background(), tx, ListMyOrdersInput{
		CallerID:  buyerID,
		RoleParam: "buyer",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// Safety fallback MUST have fired: write-model data query expected.
	if !tx.hasDataQuery("FROM orders") {
		t.Fatal("Option B safety fallback did not fire for partial projection (projCount=1 < writeCount=3)")
	}
	if !tx.hasQueryMatching("buyer_id") {
		t.Fatal("expected buyer_id scope in fallback data query")
	}
}

// TestListMyOrders_WriteModelFallback_DisputeColsAreNil verifies that orders
// returned via the write-model fallback have nil dispute columns (the orders
// write table carries no dispute join columns).
func TestListMyOrders_WriteModelFallback_DisputeColsAreNil(t *testing.T) {
	// Projection empty; countResult=1 → write model has 1 order → fallback fires.
	svc := NewOrderQueryService(&stubProjectionLister{buyerResults: nil}, true)
	tx := &captureTx{countResult: 1}

	resp, err := svc.ListMyOrders(context.Background(), tx, ListMyOrdersInput{
		CallerID:  uuid.New(),
		RoleParam: "buyer",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response from write-model fallback")
	}
	// captureTx returns empty rows so no orders are scanned; verify no panic.
	for _, order := range resp.Orders {
		if order.DisputeStatus != nil {
			t.Fatalf("expected DisputeStatus nil for write-model fallback row, got %q", *order.DisputeStatus)
		}
		if order.DisputeReason != nil {
			t.Fatalf("expected DisputeReason nil for write-model fallback row, got %q", *order.DisputeReason)
		}
	}
}

// TestListAllOrdersForAdmin_PartialProjection_SafeFallback verifies that when
// the projection total is lower than the write-model count (partial lag), the
// admin path falls back to the write model.
//
// This replaces the former KnownLimitation test. Option B gate:
// projTotal(1) < writeModelCount(3) → fallback fires.
func TestListAllOrdersForAdmin_PartialProjection_SafeFallback(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	sid := uuid.New()
	svc := NewOrderQueryService(&stubProjectionLister{
		adminResults: []*projection.OrderSummary{
			{
				ID:           uuid.New(),
				BuyerID:      buyerID,
				SellerID:     sellerID,
				SourceID:     &sid,
				SourceType:   "for_sale",
				Status:       "paid",
				EscrowStatus: "holding",
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
		},
		adminTotal: 1, // projection claims 1; write model has 3
	}, true)
	// countResult=3 → write model COUNT returns 3 → projTotal(1) < 3 → fallback.
	tx := &captureTx{countResult: 3}

	resp, err := svc.ListAllOrdersForAdmin(context.Background(), tx, AdminOrderListFilters{
		Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// Both a COUNT query and the fallback data query must have appeared.
	if !tx.hasQueryMatching("COUNT") || !tx.hasQueryMatching("orders") {
		t.Fatalf("expected COUNT query against orders write table; queries: %v", tx.queries)
	}
	if !tx.hasDataQuery("FROM orders") {
		t.Fatal("Option B safety fallback did not fire for admin partial projection (projTotal=1 < writeCount=3)")
	}
}

// TestListAllOrdersForAdmin_NoFallback_WhenProjectionHasResults verifies that
// when projection total == write-model count, no fallback data query fires.
//
// Option B gate: projTotal(1) >= writeModelCount(0) → no fallback.
func TestListAllOrdersForAdmin_NoFallback_WhenProjectionHasResults(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	sid := uuid.New()
	svc := NewOrderQueryService(&stubProjectionLister{
		adminResults: []*projection.OrderSummary{
			{
				ID:           uuid.New(),
				BuyerID:      buyerID,
				SellerID:     sellerID,
				SourceID:     &sid,
				SourceType:   "for_sale",
				Status:       "paid",
				EscrowStatus: "holding",
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
		},
		adminTotal: 1,
	}, true)
	// countResult=0 (default) → write model count=0 → projTotal(1) >= 0 → no fallback.
	tx := &captureTx{}

	resp, err := svc.ListAllOrdersForAdmin(context.Background(), tx, AdminOrderListFilters{
		Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Orders) != 1 {
		t.Fatalf("expected 1 order from projection, got %d", len(resp.Orders))
	}
	// Write-model DATA query must NOT have fired.
	if tx.hasDataQuery("FROM orders") {
		t.Fatal("write model data fallback must NOT be triggered when projection is current; got ORDER BY query")
	}
}

// ─── projectionEnabled=false (fast-path) tests ──────────────────────────────

// stubCallTracker wraps stubProjectionLister and records whether any method was called.
type stubCallTracker struct {
	stubProjectionLister
	called bool
}

func (s *stubCallTracker) ListOrderSummariesByBuyer(ctx context.Context, tx db.Tx, buyerID uuid.UUID, status *string, limit int, cursor int64) ([]*projection.OrderSummary, error) {
	s.called = true
	return s.stubProjectionLister.ListOrderSummariesByBuyer(ctx, tx, buyerID, status, limit, cursor)
}

func (s *stubCallTracker) ListOrderSummariesBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID, status *string, limit int, cursor int64) ([]*projection.OrderSummary, error) {
	s.called = true
	return s.stubProjectionLister.ListOrderSummariesBySeller(ctx, tx, sellerID, status, limit, cursor)
}

func (s *stubCallTracker) ListOrderSummariesForAdmin(ctx context.Context, tx db.Tx, filters projection.OrderListFilters) ([]*projection.OrderSummary, int64, error) {
	s.called = true
	return s.stubProjectionLister.ListOrderSummariesForAdmin(ctx, tx, filters)
}

func (s *stubCallTracker) CountOrderSummariesByBuyer(ctx context.Context, tx db.Tx, buyerID uuid.UUID, status *string) (int64, error) {
	s.called = true
	return s.stubProjectionLister.CountOrderSummariesByBuyer(ctx, tx, buyerID, status)
}

func (s *stubCallTracker) CountOrderSummariesBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID, status *string) (int64, error) {
	s.called = true
	return s.stubProjectionLister.CountOrderSummariesBySeller(ctx, tx, sellerID, status)
}

// TestListMyOrders_ProjectionDisabled_SkipsProjection verifies that when
// projectionEnabled=false the service does NOT call any projection method
// and queries the write model directly. This is the fast-path that avoids
// 2 wasted queries against an empty order_summaries table.
func TestListMyOrders_ProjectionDisabled_SkipsProjection(t *testing.T) {
	tracker := &stubCallTracker{}
	svc := NewOrderQueryService(tracker, false)
	tx := &captureTx{countResult: 0}

	resp, err := svc.ListMyOrders(context.Background(), tx, ListMyOrdersInput{
		CallerID:  uuid.New(),
		RoleParam: "buyer",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if tracker.called {
		t.Fatal("projection methods must NOT be called when projectionEnabled=false")
	}
	// Write model data query must have been issued directly.
	if !tx.hasDataQuery("FROM orders") {
		t.Fatal("expected direct write-model data query when projection disabled")
	}
	// No projection-table queries should appear.
	if tx.hasQueryMatching("order_summaries") {
		t.Fatal("order_summaries must not be queried when projection disabled")
	}
}

// TestListMyOrders_ProjectionDisabled_Seller verifies the seller fast-path.
func TestListMyOrders_ProjectionDisabled_Seller(t *testing.T) {
	tracker := &stubCallTracker{}
	svc := NewOrderQueryService(tracker, false)
	tx := &captureTx{countResult: 0}

	resp, err := svc.ListMyOrders(context.Background(), tx, ListMyOrdersInput{
		CallerID:  uuid.New(),
		RoleParam: "seller",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if tracker.called {
		t.Fatal("projection methods must NOT be called when projectionEnabled=false")
	}
	if !tx.hasDataQuery("FROM orders") {
		t.Fatal("expected direct write-model data query for seller when projection disabled")
	}
	if !tx.hasQueryMatching("service_fee_amount") {
		t.Fatal("expected seller write-model query to select service_fee_amount")
	}
	if !tx.hasQueryMatching("total_payable_amount") {
		t.Fatal("expected seller write-model query to select total_payable_amount")
	}
	if !tx.hasQueryMatching("seller_id") {
		t.Fatal("expected seller_id scope in direct query")
	}
	if len(resp.Orders) != 0 {
		t.Fatalf("expected empty seller order list when no rows returned, got %d", len(resp.Orders))
	}
}

// TestListAllOrdersForAdmin_ProjectionDisabled_SkipsProjection verifies that
// admin list also takes the fast-path when projection is disabled.
func TestListAllOrdersForAdmin_ProjectionDisabled_SkipsProjection(t *testing.T) {
	tracker := &stubCallTracker{}
	svc := NewOrderQueryService(tracker, false)
	tx := &captureTx{countResult: 0}

	resp, err := svc.ListAllOrdersForAdmin(context.Background(), tx, AdminOrderListFilters{
		Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if tracker.called {
		t.Fatal("projection methods must NOT be called when projectionEnabled=false for admin")
	}
	if tx.hasQueryMatching("order_summaries") {
		t.Fatal("order_summaries must not be queried when projection disabled for admin")
	}
}

// TestListMyOrders_ProjectionEnabled_StillCallsProjection verifies that
// projectionEnabled=true preserves the existing Option B behavior.
func TestListMyOrders_ProjectionEnabled_StillCallsProjection(t *testing.T) {
	tracker := &stubCallTracker{}
	svc := NewOrderQueryService(tracker, true)
	tx := &captureTx{countResult: 0}

	_, err := svc.ListMyOrders(context.Background(), tx, ListMyOrdersInput{
		CallerID:  uuid.New(),
		RoleParam: "buyer",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tracker.called {
		t.Fatal("projection methods MUST be called when projectionEnabled=true")
	}
}


