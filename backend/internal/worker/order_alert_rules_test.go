package worker

import (
	"context"
	"testing"

	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/jackc/pgx/v5"
)

// =============================================================================
// EscrowStuckRule tests — FIX-2
// =============================================================================

// escrowQueryRow builds a QueryRowFunc that returns (stuckCount, oldestOrderID, oldestAgeHours)
// for the first call and (criticalCount) for the second call.
func escrowQueryRow(stuckCount int, oldestOrderID string, oldestAgeHours float64, criticalCount int) func(ctx context.Context, sql string, args ...any) pgx.Row {
	call := 0
	return func(_ context.Context, _ string, _ ...any) pgx.Row {
		call++
		switch call {
		case 1:
			return &mockRow{values: []any{stuckCount, oldestOrderID, oldestAgeHours}}
		default:
			return &mockRow{values: []any{criticalCount}}
		}
	}
}

func TestEscrowStuckRule_NoOrders_NoAlert(t *testing.T) {
	tx := &mockTx{
		QueryRowFunc: escrowQueryRow(0, "", 0.0, 0),
	}
	rule := NewEscrowStuckRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detected {
		t.Fatal("expected no alert when stuck_count=0")
	}
	if finding != nil {
		t.Fatal("expected nil finding when stuck_count=0")
	}
}

func TestEscrowStuckRule_7to14Days_Warn(t *testing.T) {
	// 7 days = 168 hours → WARN
	tx := &mockTx{
		QueryRowFunc: escrowQueryRow(3, "order-uuid-1", float64(EscrowStuckThresholdDays*24), 0),
	}
	rule := NewEscrowStuckRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert for 7+ day stuck escrow")
	}
	if finding.AlertType != alertentity.AlertTypeEscrowStuck {
		t.Errorf("alert type = %q, want escrow_stuck", finding.AlertType)
	}
	if finding.Severity != alertentity.SeverityWarning {
		t.Errorf("severity = %q, want warning", finding.Severity)
	}
	if finding.Metadata["stuck_count"] != 3 {
		t.Errorf("stuck_count = %v, want 3", finding.Metadata["stuck_count"])
	}
}

func TestEscrowStuckRule_Over14Days_Critical(t *testing.T) {
	// 15 days = 360 hours → CRITICAL
	tx := &mockTx{
		QueryRowFunc: escrowQueryRow(2, "order-uuid-2", float64(EscrowStuckCriticalDays*24+24), 2),
	}
	rule := NewEscrowStuckRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert for 15+ day stuck escrow")
	}
	if finding.Severity != alertentity.SeverityCritical {
		t.Errorf("severity = %q, want critical for >14 days", finding.Severity)
	}
}

func TestEscrowStuckRule_Name(t *testing.T) {
	if NewEscrowStuckRule(nil, nil).Name() != "escrow_stuck" {
		t.Fatal("unexpected rule name")
	}
}

// =============================================================================
// OrderPaidStuckRule tests — FIX-3
// =============================================================================

// orderPaidQueryRow returns (stuckCount) for QueryRow, then (sampleIDs) for Query.
func orderPaidTx(stuckCount int, sampleIDs []string) *mockTx {
	return &mockTx{
		QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{values: []any{stuckCount}}
		},
		QueryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			rows := make([][]any, len(sampleIDs))
			for i, id := range sampleIDs {
				rows[i] = []any{id}
			}
			return &mockRows{rows: rows}, nil
		},
	}
}

func TestOrderPaidStuckRule_NoOrders_NoAlert(t *testing.T) {
	rule := NewOrderPaidStuckRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), orderPaidTx(0, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detected || finding != nil {
		t.Fatal("expected no alert when stuck_count=0")
	}
}

func TestOrderPaidStuckRule_FewOrders_Warn(t *testing.T) {
	rule := NewOrderPaidStuckRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), orderPaidTx(3, []string{"o1", "o2", "o3"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert for stuck paid orders")
	}
	if finding.AlertType != alertentity.AlertTypeOrderPaidStuck {
		t.Errorf("alert type = %q, want order_paid_stuck", finding.AlertType)
	}
	if finding.Severity != alertentity.SeverityWarning {
		t.Errorf("severity = %q, want warning for count<=5", finding.Severity)
	}
}

func TestOrderPaidStuckRule_ManyOrders_Critical(t *testing.T) {
	rule := NewOrderPaidStuckRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), orderPaidTx(OrderPaidStuckCriticalCount+1, []string{"o1", "o2", "o3", "o4", "o5"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert")
	}
	if finding.Severity != alertentity.SeverityCritical {
		t.Errorf("severity = %q, want critical for count>%d", finding.Severity, OrderPaidStuckCriticalCount)
	}
}

func TestOrderPaidStuckRule_Name(t *testing.T) {
	if NewOrderPaidStuckRule(nil, nil).Name() != "order_paid_stuck" {
		t.Fatal("unexpected rule name")
	}
}

// =============================================================================
// OrderShippedStuckRule tests — FIX-3
// =============================================================================

func orderShippedTx(stuckCount int, sampleIDs []string) *mockTx {
	return &mockTx{
		QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{values: []any{stuckCount}}
		},
		QueryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			rows := make([][]any, len(sampleIDs))
			for i, id := range sampleIDs {
				rows[i] = []any{id}
			}
			return &mockRows{rows: rows}, nil
		},
	}
}

func TestOrderShippedStuckRule_NoOrders_NoAlert(t *testing.T) {
	rule := NewOrderShippedStuckRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), orderShippedTx(0, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detected || finding != nil {
		t.Fatal("expected no alert when stuck_count=0")
	}
}

func TestOrderShippedStuckRule_FewOrders_Warn(t *testing.T) {
	rule := NewOrderShippedStuckRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), orderShippedTx(2, []string{"o1", "o2"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert for stuck shipped orders")
	}
	if finding.AlertType != alertentity.AlertTypeOrderShippedStuck {
		t.Errorf("alert type = %q, want order_shipped_stuck", finding.AlertType)
	}
	if finding.Severity != alertentity.SeverityWarning {
		t.Errorf("severity = %q, want warning", finding.Severity)
	}
}

func TestOrderShippedStuckRule_ManyOrders_Critical(t *testing.T) {
	rule := NewOrderShippedStuckRule(nil, nil)
	detected, finding, err := rule.Detect(context.Background(), orderShippedTx(OrderShippedStuckCriticalCount+1, []string{"o1", "o2", "o3", "o4", "o5"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert")
	}
	if finding.Severity != alertentity.SeverityCritical {
		t.Errorf("severity = %q, want critical for count>%d", finding.Severity, OrderShippedStuckCriticalCount)
	}
}

// =============================================================================
// DisputeOpenStuckRule tests — FIX-3
// =============================================================================

func disputeStuckRow(disputeCount int, oldestID string, oldestAgeHours float64) func(ctx context.Context, sql string, args ...any) pgx.Row {
	return func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{values: []any{disputeCount, oldestID, oldestAgeHours}}
	}
}

func TestDisputeOpenStuckRule_NoDisputes_NoAlert(t *testing.T) {
	rule := NewDisputeOpenStuckRule(nil, nil)
	tx := &mockTx{QueryRowFunc: disputeStuckRow(0, "", 0.0)}
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detected || finding != nil {
		t.Fatal("expected no alert when dispute_count=0")
	}
}

func TestDisputeOpenStuckRule_48to72h_Warn(t *testing.T) {
	rule := NewDisputeOpenStuckRule(nil, nil)
	tx := &mockTx{QueryRowFunc: disputeStuckRow(2, "dispute-uuid-1", float64(DisputeOpenStuckWarnHours))}
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert for 48h+ stuck dispute")
	}
	if finding.AlertType != alertentity.AlertTypeDisputeOpenStuck {
		t.Errorf("alert type = %q, want dispute_open_stuck", finding.AlertType)
	}
	if finding.Severity != alertentity.SeverityWarning {
		t.Errorf("severity = %q, want warning for <72h", finding.Severity)
	}
	if finding.Metadata["dispute_count"] != 2 {
		t.Errorf("dispute_count = %v, want 2", finding.Metadata["dispute_count"])
	}
}

func TestDisputeOpenStuckRule_Over72h_Critical(t *testing.T) {
	rule := NewDisputeOpenStuckRule(nil, nil)
	tx := &mockTx{QueryRowFunc: disputeStuckRow(1, "dispute-uuid-2", float64(DisputeOpenStuckCriticalHours+1))}
	detected, finding, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected {
		t.Fatal("expected alert")
	}
	if finding.Severity != alertentity.SeverityCritical {
		t.Errorf("severity = %q, want critical for >%dh", finding.Severity, DisputeOpenStuckCriticalHours)
	}
}

func TestDisputeOpenStuckRule_Name(t *testing.T) {
	if NewDisputeOpenStuckRule(nil, nil).Name() != "dispute_open_stuck" {
		t.Fatal("unexpected rule name")
	}
}


