package application

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// TOTAL MONEY INVARIANT CHECKER — UNIT TESTS
// ============================================================================
// These tests verify the ledger-authority invariant:
//   SUM(financial_accounts.balance) == BankSettlementInitialSeed
//
// No wallet, payment, order, or refund queries exist in the checker.

// --- Seed constant ---

func TestBankSettlementSeedConstant(t *testing.T) {
	assert.Equal(t, int64(9_000_000_000_000_000), BankSettlementInitialSeed,
		"Seed must be 9Q (Rp 90 trillion reserve float)")
}

// --- Balanced system passes ---

func TestCheckTotalMoneyInvariant_BalancedSystem_Passes(t *testing.T) {
	// SUM(balance) == seed → no violation
	txDB := &invariantMockTransactor{balance: BankSettlementInitialSeed}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())

	require.NoError(t, err)
	assert.False(t, violated, "Balanced system should not flag violation")
	assert.Equal(t, 0, tracker.alertCount, "No alert for balanced system")
}

// --- Imbalanced system fails ---

func TestCheckTotalMoneyInvariant_Imbalance_Detected(t *testing.T) {
	// SUM(balance) is 1 unit off → violation
	txDB := &invariantMockTransactor{balance: BankSettlementInitialSeed + 1}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())

	require.NoError(t, err)
	assert.True(t, violated, "1-unit difference should flag violation")
	assert.Equal(t, 1, tracker.alertCount, "Alert should be created for imbalance")
}

func TestCheckTotalMoneyInvariant_NegativeImbalance_Detected(t *testing.T) {
	// SUM(balance) is below seed → still a violation
	txDB := &invariantMockTransactor{balance: BankSettlementInitialSeed - 500}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())

	require.NoError(t, err)
	assert.True(t, violated)
	assert.Equal(t, 1, tracker.alertCount)
}

// --- Shadow mode ---

func TestCheckTotalMoneyInvariant_ShadowMode_SuppressesAlert(t *testing.T) {
	// Imbalance exists but shadow mode → no alert created
	txDB := &invariantMockTransactor{balance: BankSettlementInitialSeed + 999}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), true)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())

	require.NoError(t, err)
	assert.True(t, violated, "Violation should still be detected")
	assert.Equal(t, 0, tracker.alertCount, "Shadow mode must suppress alerts")
}

func TestCheckTotalMoneyInvariant_NonShadow_CreatesAlert(t *testing.T) {
	txDB := &invariantMockTransactor{balance: BankSettlementInitialSeed - 1}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())

	require.NoError(t, err)
	assert.True(t, violated)
	assert.Equal(t, 1, tracker.alertCount, "Non-shadow must create alert")
}

// --- Alert metadata ---

func TestCheckTotalMoneyInvariant_AlertMetadata(t *testing.T) {
	diff := int64(42)
	txDB := &invariantMockTransactor{balance: BankSettlementInitialSeed + diff}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	_, err := checker.CheckTotalMoneyInvariant(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, tracker.alertCount)

	md := tracker.lastAlert.metadata
	assert.Contains(t, md, "actual_total")
	assert.Contains(t, md, "expected_total")
	assert.Contains(t, md, "difference")
	assert.Equal(t, BankSettlementInitialSeed+diff, md["actual_total"])
	assert.Equal(t, BankSettlementInitialSeed, md["expected_total"])
	assert.Equal(t, diff, md["difference"])
}

// --- Constructor ---

func TestNewTotalMoneyInvariantChecker_NilLogger(t *testing.T) {
	checker := NewTotalMoneyInvariantChecker(nil, nil, nil, true)
	require.NotNil(t, checker.log, "nil logger should default to nop")
}

func TestNewTotalMoneyInvariantChecker_ShadowModeFlag(t *testing.T) {
	t.Run("shadow=true", func(t *testing.T) {
		c := NewTotalMoneyInvariantChecker(nil, nil, nil, true)
		assert.True(t, c.shadowMode)
	})
	t.Run("shadow=false", func(t *testing.T) {
		c := NewTotalMoneyInvariantChecker(nil, nil, nil, false)
		assert.False(t, c.shadowMode)
	})
}

// --- Query error handling ---

func TestCheckTotalMoneyInvariant_QueryError_PropagatedNotViolation(t *testing.T) {
	txDB := &invariantMockTransactor{err: errors.New("db connection lost")}
	checker := NewTotalMoneyInvariantChecker(nil, txDB, zap.NewNop(), false)

	violated, err := checker.CheckTotalMoneyInvariant(context.Background())
	require.Error(t, err, "Query error should propagate")
	assert.False(t, violated, "Query error should NOT be reported as violation")
}

// --- Zero balance (empty system before bootstrap) ---

func TestCheckTotalMoneyInvariant_ZeroBalance_Violation(t *testing.T) {
	// If SUM(balance) = 0 (no accounts bootstrapped yet), that's a violation
	txDB := &invariantMockTransactor{balance: 0}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())

	require.NoError(t, err)
	assert.True(t, violated, "Zero balance should flag violation")
	assert.Equal(t, 1, tracker.alertCount)
}

// --- No dead wallet/payment/order imports ---

func TestTotalMoneyInvariantChecker_NoDeadDependencies(t *testing.T) {
	// Structural test: the checker must NOT depend on WalletService, PaymentRepository,
	// or any order/refund/payout table. This is verified by the constructor signature:
	// only alertService, db, log, shadowMode are accepted.
	//
	// If someone adds walletService or paymentRepo back, this test's comment
	// and the constructor call below will need updating — making the regression visible.
	checker := NewTotalMoneyInvariantChecker(nil, nil, nil, true)
	require.NotNil(t, checker)
}

// ============================================================================
// TEST MOCK INFRASTRUCTURE
// ============================================================================

// invariantMockTransactor provides a configurable Transactor for testing
// the total money invariant checker.
type invariantMockTransactor struct {
	balance int64
	err     error
}

func (m *invariantMockTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	if m.err != nil {
		return m.err
	}
	return fn(&invariantMockTx{balance: m.balance})
}

// invariantMockTx implements db.Tx for the invariant checker tests.
type invariantMockTx struct {
	balance int64
}

func (t *invariantMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &invariantMockRow{balance: t.balance}
}

func (t *invariantMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}

func (t *invariantMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0"), nil
}

func (t *invariantMockTx) Commit(_ context.Context) error   { return nil }
func (t *invariantMockTx) Rollback(_ context.Context) error { return nil }

// invariantMockRow returns a single int64 value for SUM(balance) queries.
type invariantMockRow struct {
	balance int64
}

func (r *invariantMockRow) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("expected 1 scan destination")
	}
	if p, ok := dest[0].(*int64); ok {
		*p = r.balance
		return nil
	}
	return errors.New("expected *int64 scan destination")
}


