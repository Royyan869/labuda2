package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"go.uber.org/zap/zaptest"
)

// TestOutboxDLQSpikeRule_SpikeCreatesWarningAlert verifies that hitting the
// threshold of dead-letter events in the window triggers a WARNING alert.
func TestOutboxDLQSpikeRule_SpikeCreatesWarningAlert(t *testing.T) {
	now := time.Now()

	// 4 DLQ events (above threshold=3, below critical=10) → WARNING
	tx := &mockTx{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				rows: [][]any{
					{uuid.New(), "support.ticket.created", now.Add(-1 * time.Minute)},
					{uuid.New(), "coins.refund_required", now.Add(-2 * time.Minute)},
					{uuid.New(), "support.ticket.created", now.Add(-3 * time.Minute)},
					{uuid.New(), "withdrawal.completed", now.Add(-5 * time.Minute)},
				},
			}, nil
		},
	}

	rule := NewOutboxDLQSpikeRule(&mockDB{}, zaptest.NewLogger(t))
	detected, finding, err := rule.Detect(context.Background(), tx)

	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if !detected {
		t.Fatal("expected anomaly detected for DLQ spike")
	}
	if finding == nil {
		t.Fatal("expected non-nil finding")
	}

	if finding.AlertType != alertentity.AlertTypeOutboxDLQSpike {
		t.Errorf("AlertType = %q, want %q", finding.AlertType, alertentity.AlertTypeOutboxDLQSpike)
	}
	if finding.Severity != alertentity.SeverityWarning {
		t.Errorf("Severity = %q, want %q", finding.Severity, alertentity.SeverityWarning)
	}
	if finding.EntityType != "outbox" {
		t.Errorf("EntityType = %q, want %q", finding.EntityType, "outbox")
	}

	// Verify metadata
	if finding.Metadata["dlq_count"] != 4 {
		t.Errorf("metadata.dlq_count = %v, want 4", finding.Metadata["dlq_count"])
	}
	if finding.Metadata["window_minutes"] != OutboxDLQSpikeWindowMinutes {
		t.Errorf("metadata.window_minutes = %v, want %d", finding.Metadata["window_minutes"], OutboxDLQSpikeWindowMinutes)
	}

	breakdown, ok := finding.Metadata["type_breakdown"].(map[string]int)
	if !ok {
		t.Fatal("metadata.type_breakdown missing or wrong type")
	}
	if breakdown["support.ticket.created"] != 2 {
		t.Errorf("type_breakdown[support.ticket.created] = %d, want 2", breakdown["support.ticket.created"])
	}
}

// TestOutboxDLQSpikeRule_CriticalEscalation verifies that exceeding the
// critical threshold produces a CRITICAL alert.
func TestOutboxDLQSpikeRule_CriticalEscalation(t *testing.T) {
	now := time.Now()

	// Build 12 DLQ events (above critical=10)
	dlqRows := make([][]any, 12)
	for i := range dlqRows {
		dlqRows[i] = []any{uuid.New(), "order.created", now.Add(-time.Duration(i) * time.Minute)}
	}

	tx := &mockTx{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{rows: dlqRows}, nil
		},
	}

	rule := NewOutboxDLQSpikeRule(&mockDB{}, zaptest.NewLogger(t))
	detected, finding, err := rule.Detect(context.Background(), tx)

	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if !detected {
		t.Fatal("expected anomaly detected")
	}
	if finding.Severity != alertentity.SeverityCritical {
		t.Errorf("Severity = %q, want %q (12 >= critical threshold)", finding.Severity, alertentity.SeverityCritical)
	}
}

// TestOutboxDLQSpikeRule_BelowThresholdNoAlert verifies that fewer than
// threshold DLQ events do NOT trigger an alert.
func TestOutboxDLQSpikeRule_BelowThresholdNoAlert(t *testing.T) {
	now := time.Now()

	// Only 2 events (below threshold=3)
	tx := &mockTx{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				rows: [][]any{
					{uuid.New(), "order.created", now.Add(-1 * time.Minute)},
					{uuid.New(), "order.paid", now.Add(-2 * time.Minute)},
				},
			}, nil
		},
	}

	rule := NewOutboxDLQSpikeRule(&mockDB{}, zaptest.NewLogger(t))
	detected, finding, err := rule.Detect(context.Background(), tx)

	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if detected {
		t.Error("expected no anomaly below threshold")
	}
	if finding != nil {
		t.Error("expected nil finding below threshold")
	}
}

// TestOutboxDLQSpikeRule_EmptyNoAlert verifies that no DLQ events produce
// no alert.
func TestOutboxDLQSpikeRule_EmptyNoAlert(t *testing.T) {
	tx := &mockTx{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}

	rule := NewOutboxDLQSpikeRule(&mockDB{}, zaptest.NewLogger(t))
	detected, _, err := rule.Detect(context.Background(), tx)

	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if detected {
		t.Error("expected no anomaly for empty result")
	}
}

// TestOutboxDLQSpikeRule_GroupKeyDedup verifies stable group_key across
// detection cycles for AlertService deduplication.
func TestOutboxDLQSpikeRule_GroupKeyDedup(t *testing.T) {
	now := time.Now()

	makeTx := func() *mockTx {
		return &mockTx{
			QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return &mockRows{
					rows: [][]any{
						{uuid.New(), "order.created", now.Add(-1 * time.Minute)},
						{uuid.New(), "order.paid", now.Add(-2 * time.Minute)},
						{uuid.New(), "order.shipped", now.Add(-3 * time.Minute)},
					},
				}, nil
			},
		}
	}

	rule := NewOutboxDLQSpikeRule(&mockDB{}, zaptest.NewLogger(t))

	_, finding1, _ := rule.Detect(context.Background(), makeTx())
	_, finding2, _ := rule.Detect(context.Background(), makeTx())

	if finding1.GroupKey == nil || finding2.GroupKey == nil {
		t.Fatal("expected non-nil GroupKey")
	}
	if *finding1.GroupKey != *finding2.GroupKey {
		t.Errorf("GroupKey mismatch: %q vs %q", *finding1.GroupKey, *finding2.GroupKey)
	}
	if *finding1.GroupKey != "outbox_dlq_spike:recent" {
		t.Errorf("GroupKey = %q, want %q", *finding1.GroupKey, "outbox_dlq_spike:recent")
	}
}

// TestOutboxDLQSpikeRule_Name verifies the rule name.
func TestOutboxDLQSpikeRule_Name(t *testing.T) {
	rule := NewOutboxDLQSpikeRule(&mockDB{}, zaptest.NewLogger(t))
	if rule.Name() != "outbox_dlq_spike" {
		t.Errorf("Name() = %q, want %q", rule.Name(), "outbox_dlq_spike")
	}
}


