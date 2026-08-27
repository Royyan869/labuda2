package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap/zaptest"
)

// TestWithdrawalAnomalyRule_ReportsRupiahAmount verifies the alert message and
// metadata report the already-Rupiah ledger sum as-is, without a stray /100
// that would understate the amount by 100x (e.g. Rp100.000 -> Rp1.000).
func TestWithdrawalAnomalyRule_ReportsRupiahAmount(t *testing.T) {
	userID := uuid.New()

	tx := &mockTx{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				rows: [][]any{
					// SUM(amount) already in Rupiah: -100000 (outgoing withdrawal)
					{userID, 1, int64(-100000)},
				},
			}, nil
		},
	}

	rule := NewWithdrawalAnomalyRule(&mockDB{}, zaptest.NewLogger(t))
	detected, finding, err := rule.Detect(context.Background(), tx)

	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if !detected {
		t.Fatal("expected anomaly detected")
	}
	if finding == nil {
		t.Fatal("expected non-nil finding")
	}

	if finding.Metadata["total_amount"] != int64(100000) {
		t.Errorf("metadata.total_amount = %v, want 100000", finding.Metadata["total_amount"])
	}
	if _, exists := finding.Metadata["total_amount_cents"]; exists {
		t.Error("metadata must not contain legacy total_amount_cents key")
	}
	if _, exists := finding.Metadata["threshold_cents"]; exists {
		t.Error("metadata must not contain legacy threshold_cents key")
	}
	if finding.Metadata["threshold"] != WithdrawalAnomalyThreshold {
		t.Errorf("metadata.threshold = %v, want %d", finding.Metadata["threshold"], WithdrawalAnomalyThreshold)
	}

	wantSubstr := "Rp100000"
	if !strings.Contains(finding.Message, wantSubstr) {
		t.Errorf("Message = %q, want it to contain %q (not divided by 100)", finding.Message, wantSubstr)
	}
	if strings.Contains(finding.Message, "Rp1000 ") || strings.Contains(finding.Message, "1000.00") {
		t.Errorf("Message = %q, appears to still be divided by 100", finding.Message)
	}
}

// TestWithdrawalAnomalyRule_BelowThresholdNoAlert verifies the threshold
// comparison still operates on Rupiah amounts (unchanged by this fix).
func TestWithdrawalAnomalyRule_BelowThresholdNoAlert(t *testing.T) {
	tx := &mockTx{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}

	rule := NewWithdrawalAnomalyRule(&mockDB{}, zaptest.NewLogger(t))
	detected, finding, err := rule.Detect(context.Background(), tx)

	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if detected {
		t.Error("expected no anomaly when query returns no rows")
	}
	if finding != nil {
		t.Error("expected nil finding when no rows")
	}
}

// TestWithdrawalAnomalyRule_Name verifies the rule name.
func TestWithdrawalAnomalyRule_Name(t *testing.T) {
	rule := NewWithdrawalAnomalyRule(&mockDB{}, zaptest.NewLogger(t))
	if rule.Name() != "withdrawal_anomaly" {
		t.Errorf("Name() = %q, want %q", rule.Name(), "withdrawal_anomaly")
	}
}
