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

// TestStaleDisputeFreezeRule_OldFreezeCreatesAlert verifies that an active
// freeze older than the threshold triggers an alert with correct metadata.
func TestStaleDisputeFreezeRule_OldFreezeCreatesAlert(t *testing.T) {
	freezeID := uuid.New()
	disputeID := uuid.New()
	orderID := uuid.New()
	sellerID := uuid.New()
	frozenAmount := int64(500000)
	// Created 72 hours ago → above both threshold (48h) and critical (72h)
	createdAtMs := time.Now().Add(-72 * time.Hour).UnixMilli()

	tx := &mockTx{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				rows: [][]any{
					{freezeID, disputeID, orderID, sellerID, frozenAmount, createdAtMs},
				},
			}, nil
		},
	}

	rule := NewStaleDisputeFreezeRule(&mockDB{}, zaptest.NewLogger(t))
	detected, finding, err := rule.Detect(context.Background(), tx)

	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if !detected {
		t.Fatal("expected anomaly detected for old freeze")
	}
	if finding == nil {
		t.Fatal("expected non-nil finding")
	}

	// Verify alert type
	if finding.AlertType != alertentity.AlertTypeStaleDisputeFreeze {
		t.Errorf("AlertType = %q, want %q", finding.AlertType, alertentity.AlertTypeStaleDisputeFreeze)
	}

	// 72h >= critical threshold → CRITICAL
	if finding.Severity != alertentity.SeverityCritical {
		t.Errorf("Severity = %q, want %q (72h >= critical threshold)", finding.Severity, alertentity.SeverityCritical)
	}

	if finding.EntityType != "dispute_freeze" {
		t.Errorf("EntityType = %q, want %q", finding.EntityType, "dispute_freeze")
	}
	if finding.EntityID != freezeID {
		t.Errorf("EntityID = %s, want %s", finding.EntityID, freezeID)
	}

	// Verify metadata contains required fields
	if finding.Metadata["stale_count"] != 1 {
		t.Errorf("metadata.stale_count = %v, want 1", finding.Metadata["stale_count"])
	}
	if finding.Metadata["threshold_hours"] != StaleDisputeFreezeThresholdHours {
		t.Errorf("metadata.threshold_hours = %v, want %d", finding.Metadata["threshold_hours"], StaleDisputeFreezeThresholdHours)
	}

	oldest, ok := finding.Metadata["oldest_freeze"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata.oldest_freeze missing or wrong type")
	}
	if oldest["freeze_id"] != freezeID.String() {
		t.Errorf("oldest_freeze.freeze_id = %v, want %s", oldest["freeze_id"], freezeID)
	}
	if oldest["seller_id"] != sellerID.String() {
		t.Errorf("oldest_freeze.seller_id = %v, want %s", oldest["seller_id"], sellerID)
	}
}

// TestStaleDisputeFreezeRule_WarningSeverity verifies that freezes between
// threshold and critical threshold get WARNING severity.
func TestStaleDisputeFreezeRule_WarningSeverity(t *testing.T) {
	// 50 hours → above 48h threshold, below 72h critical
	createdAtMs := time.Now().Add(-50 * time.Hour).UnixMilli()

	tx := &mockTx{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				rows: [][]any{
					{uuid.New(), uuid.New(), uuid.New(), uuid.New(), int64(100000), createdAtMs},
				},
			}, nil
		},
	}

	rule := NewStaleDisputeFreezeRule(&mockDB{}, zaptest.NewLogger(t))
	detected, finding, err := rule.Detect(context.Background(), tx)

	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if !detected {
		t.Fatal("expected anomaly detected")
	}
	if finding.Severity != alertentity.SeverityWarning {
		t.Errorf("Severity = %q, want %q (50h < critical threshold)", finding.Severity, alertentity.SeverityWarning)
	}
}

// TestStaleDisputeFreezeRule_FreshFreezeNoAlert verifies that an active freeze
// within the threshold does NOT trigger an alert.
func TestStaleDisputeFreezeRule_FreshFreezeNoAlert(t *testing.T) {
	// Empty result set — the SQL WHERE clause filters out fresh freezes
	tx := &mockTx{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}

	rule := NewStaleDisputeFreezeRule(&mockDB{}, zaptest.NewLogger(t))
	detected, finding, err := rule.Detect(context.Background(), tx)

	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if detected {
		t.Error("expected no anomaly for fresh freeze")
	}
	if finding != nil {
		t.Error("expected nil finding for fresh freeze")
	}
}

// TestStaleDisputeFreezeRule_ReleasedFreezeNoAlert verifies that released
// freezes are excluded (SQL WHERE status = 'active').
func TestStaleDisputeFreezeRule_ReleasedFreezeNoAlert(t *testing.T) {
	// Same as fresh test — released freezes are filtered by SQL
	tx := &mockTx{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}

	rule := NewStaleDisputeFreezeRule(&mockDB{}, zaptest.NewLogger(t))
	detected, _, err := rule.Detect(context.Background(), tx)

	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if detected {
		t.Error("expected no anomaly for released freezes")
	}
}

// TestStaleDisputeFreezeRule_GroupKeyDedup verifies that the rule returns a
// consistent group_key so AlertService deduplicates across detection cycles.
func TestStaleDisputeFreezeRule_GroupKeyDedup(t *testing.T) {
	createdAtMs := time.Now().Add(-96 * time.Hour).UnixMilli()

	tx := &mockTx{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				rows: [][]any{
					{uuid.New(), uuid.New(), uuid.New(), uuid.New(), int64(200000), createdAtMs},
				},
			}, nil
		},
	}

	rule := NewStaleDisputeFreezeRule(&mockDB{}, zaptest.NewLogger(t))

	// First detection
	_, finding1, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("first Detect: %v", err)
	}

	// Reset mockRows cursor for second call
	tx.QueryFunc = func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
		return &mockRows{
			rows: [][]any{
				{uuid.New(), uuid.New(), uuid.New(), uuid.New(), int64(200000), createdAtMs},
			},
		}, nil
	}

	// Second detection
	_, finding2, err := rule.Detect(context.Background(), tx)
	if err != nil {
		t.Fatalf("second Detect: %v", err)
	}

	// Both should have the same group_key for AlertService dedup
	if finding1.GroupKey == nil || finding2.GroupKey == nil {
		t.Fatal("expected non-nil GroupKey")
	}
	if *finding1.GroupKey != *finding2.GroupKey {
		t.Errorf("GroupKey mismatch: %q vs %q", *finding1.GroupKey, *finding2.GroupKey)
	}
	if *finding1.GroupKey != "stale_dispute_freeze:active" {
		t.Errorf("GroupKey = %q, want %q", *finding1.GroupKey, "stale_dispute_freeze:active")
	}
}

// TestStaleDisputeFreezeRule_Name verifies the rule name for logging.
func TestStaleDisputeFreezeRule_Name(t *testing.T) {
	rule := NewStaleDisputeFreezeRule(&mockDB{}, zaptest.NewLogger(t))
	if rule.Name() != "stale_dispute_freeze" {
		t.Errorf("Name() = %q, want %q", rule.Name(), "stale_dispute_freeze")
	}
}


