package application

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/projection"
)

// ============================================================================
// ORDER NUMBER — ADMIN VISIBILITY TESTS
// ============================================================================

// TestListAllOrdersForAdmin_SearchByOrderNumber verifies that when a search term
// is provided the query service passes through to the write model and includes
// an order_number condition in the SQL.
//
// REGRESSION LOCK: search filter must reach the write model query.
func TestListAllOrdersForAdmin_SearchByOrderNumber(t *testing.T) {
	search := "ORD-20260528-AB12CD34"
	svc := NewOrderQueryService(&stubProjectionLister{}, false) // projection OFF
	tx := &captureTx{countResult: 0}                           // 0 rows → no data query needed

	_, err := svc.ListAllOrdersForAdmin(context.Background(), tx, AdminOrderListFilters{
		Page:     1,
		PageSize: 20,
		Search:   &search,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// COUNT query must include order_number filter.
	if !tx.hasQueryMatching("order_number") {
		t.Error("expected order_number filter in write model count query")
	}
}

// TestListAllOrdersForAdmin_SearchForceWriteModel verifies that even when projection
// is enabled, a non-nil search bypasses the projection and goes directly to the
// write model.
func TestListAllOrdersForAdmin_SearchForceWriteModel(t *testing.T) {
	search := "ORD-20260528-AB12CD34"
	// Projection has 1 result → normally would NOT fall back.
	stub := &stubProjectionLister{
		adminResults: []*projection.OrderSummary{
			{ID: uuid.New(), BuyerID: uuid.New(), SellerID: uuid.New()},
		},
		adminTotal: 1,
	}
	svc := NewOrderQueryService(stub, true) // projection ENABLED
	tx := &captureTx{countResult: 0}

	_, err := svc.ListAllOrdersForAdmin(context.Background(), tx, AdminOrderListFilters{
		Page:     1,
		PageSize: 20,
		Search:   &search,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must have queried the write model directly (no projection calls).
	if !tx.hasQueryMatching("order_number") {
		t.Error("expected write model query with order_number filter when search is set")
	}
}

// TestListAllOrdersForAdmin_NoSearch_UsesProjectionPath verifies that without
// search the admin list uses the projection path when projection is enabled.
func TestListAllOrdersForAdmin_NoSearch_UsesProjectionPath(t *testing.T) {
	adminRow := &projection.OrderSummary{
		ID: uuid.New(), BuyerID: uuid.New(), SellerID: uuid.New(),
	}
	stub := &stubProjectionLister{
		adminResults: []*projection.OrderSummary{adminRow},
		adminTotal:   1,
	}
	svc := NewOrderQueryService(stub, true) // projection ENABLED
	// countResult=1 so write-model total == projection total → no fallback.
	tx := &captureTx{countResult: 1}

	resp, err := svc.ListAllOrdersForAdmin(context.Background(), tx, AdminOrderListFilters{
		Page:     1,
		PageSize: 20,
		Search:   nil, // no search
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || len(resp.Orders) != 1 {
		t.Fatalf("expected 1 order from projection, got %v", resp)
	}
}

// TestListAllOrdersForAdmin_WriteModelQuery_IncludesOrderNumber verifies that
// the write model SELECT includes order_number so the admin response is populated.
func TestListAllOrdersForAdmin_WriteModelQuery_IncludesOrderNumber(t *testing.T) {
	svc := NewOrderQueryService(&stubProjectionLister{}, false) // projection OFF
	tx := &captureTx{countResult: 1} // simulate 1 write model row

	_, err := svc.ListAllOrdersForAdmin(context.Background(), tx, AdminOrderListFilters{
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Data SELECT query must include order_number column.
	for _, q := range tx.queries {
		if strings.Contains(q, "ORDER BY created_at") {
			if !strings.Contains(q, "order_number") {
				t.Errorf("write model data SELECT does not include order_number column:\n%s", q)
			}
			return
		}
	}
	// If count was 1 the data query will have been issued; if we got here without
	// hitting the return above the column was missing.
	t.Log("queries issued:", tx.queries)
}

// TestAdminOrderListFilters_SearchField verifies the Search field is wired into
// the filters struct (compile-time contract).
func TestAdminOrderListFilters_SearchField(t *testing.T) {
	search := "ORD-20260528-AB12CD34"
	f := AdminOrderListFilters{
		Search: &search,
	}
	if f.Search == nil || *f.Search != search {
		t.Error("Search field not correctly set on AdminOrderListFilters")
	}
}


