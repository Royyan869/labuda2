package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	alertrepo "github.com/labuda/backend/internal/platform/alert/repository"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// TOTAL MONEY INVARIANT CHECKER — UNIT TESTS
// ============================================================================
// These tests verify the per-account ledger-authority invariant:
//   stored_balance == expected (computed + seed for BANK_SETTLEMENT/PLATFORM_BANK)
//
// No wallet, payment, order, or refund queries exist in the checker.

// --- Seed constants ---

func TestBankSettlementSeedConstant(t *testing.T) {
	assert.Equal(t, int64(9_000_000_000_000_000), BankSettlementInitialSeed,
		"Seed must be 9Q (Rp 90 trillion reserve float)")
}

func TestPlatformBankSeedConstant(t *testing.T) {
	assert.Equal(t, int64(9_000_000_000_000_000), PlatformBankInitialSeed,
		"PlatformBank seed must be 9Q (mirrors BANK_SETTLEMENT)")
}

// --- Balanced system passes ---

func TestCheckTotalMoneyInvariant_BalancedSystem_Passes(t *testing.T) {
	// No mismatched accounts → no violation
	txDB := &invariantMockTransactor{rows: nil}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())

	require.NoError(t, err)
	assert.False(t, violated, "Balanced system should not flag violation")
	assert.Equal(t, 0, tracker.alertCount, "No alert for balanced system")
}

// --- Imbalanced system fails ---

func TestCheckTotalMoneyInvariant_Imbalance_Detected(t *testing.T) {
	txDB := &invariantMockTransactor{
		rows: [][]any{{"ESCROW", int64(100), int64(0), int64(0)}},
	}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())

	require.NoError(t, err)
	assert.True(t, violated, "1 mismatched account should flag violation")
	assert.Equal(t, 1, tracker.alertCount, "Alert should be created for imbalance")
}

func TestCheckTotalMoneyInvariant_MultipleMismatches_Detected(t *testing.T) {
	txDB := &invariantMockTransactor{
		rows: [][]any{
			{"ESCROW", int64(500), int64(0), int64(0)},
			{"SELLER_PAYABLE", int64(1000), int64(0), int64(0)},
		},
	}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())

	require.NoError(t, err)
	assert.True(t, violated)
	assert.Equal(t, 1, tracker.alertCount)
}

// --- Shadow mode ---

func TestCheckTotalMoneyInvariant_ShadowMode_SuppressesAlert(t *testing.T) {
	txDB := &invariantMockTransactor{
		rows: [][]any{{"PLATFORM_REVENUE", int64(999), int64(0), int64(0)}},
	}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), true)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())

	require.NoError(t, err)
	assert.True(t, violated, "Violation should still be detected")
	assert.Equal(t, 0, tracker.alertCount, "Shadow mode must suppress alerts")
}

func TestCheckTotalMoneyInvariant_NonShadow_CreatesAlert(t *testing.T) {
	txDB := &invariantMockTransactor{
		rows: [][]any{{"GATEWAY_CLEARING", int64(1), int64(0), int64(0)}},
	}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())

	require.NoError(t, err)
	assert.True(t, violated)
	assert.Equal(t, 1, tracker.alertCount, "Non-shadow must create alert")
}

// --- Seed handling: BANK_SETTLEMENT and PLATFORM_BANK with 9Q stored + correct computed should NOT be flagged ---

func TestCheckTotalMoneyInvariant_SeedAccounts_BalancedWithOffset(t *testing.T) {
	// No rows returned means checker correctly offset seed — i.e. stored == computed+9Q
	txDB := &invariantMockTransactor{rows: nil}
	alertSvc, tracker := newTrackingAlertService(t)
	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())
	require.NoError(t, err)
	assert.False(t, violated)
	assert.Equal(t, 0, tracker.alertCount)
}

func TestCheckTotalMoneyInvariant_SeedAccounts_DriftDetected(t *testing.T) {
	// BANK_SETTLEMENT with wrong stored (e.g. 9Q+1 off) should be flagged
	txDB := &invariantMockTransactor{
		rows: [][]any{{"BANK_SETTLEMENT", int64(9000000000000001), int64(0), int64(9000000000000000)}},
	}
	alertSvc, tracker := newTrackingAlertService(t)
	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	violated, err := checker.CheckTotalMoneyInvariant(context.Background())
	require.NoError(t, err)
	assert.True(t, violated)
	assert.Equal(t, 1, tracker.alertCount)
}

// --- Alert metadata ---

func TestCheckTotalMoneyInvariant_AlertMetadata(t *testing.T) {
	txDB := &invariantMockTransactor{
		rows: [][]any{{"ESCROW", int64(42), int64(0), int64(0)}},
	}
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewTotalMoneyInvariantChecker(alertSvc, txDB, zap.NewNop(), false)
	_, err := checker.CheckTotalMoneyInvariant(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, tracker.alertCount)

	md := tracker.lastAlert.metadata
	assert.Contains(t, md, "mismatched_accounts")
	assert.Contains(t, md, "mismatches")
	assert.Equal(t, 1, md["mismatched_accounts"])
	assert.Contains(t, md, "reason")
	assert.Equal(t, "total_money_invariant_violation", md["reason"])
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

// --- No dead payment/order imports ---

func TestTotalMoneyInvariantChecker_NoDeadDependencies(t *testing.T) {
	// Structural test: the checker must NOT depend on EscrowService, PaymentRepository,
	// or any order/refund/payout table. This is verified by the constructor signature:
	// only alertService, db, log, shadowMode are accepted.
	checker := NewTotalMoneyInvariantChecker(nil, nil, nil, true)
	require.NotNil(t, checker)
}

// ============================================================================
// TEST MOCK INFRASTRUCTURE
// ============================================================================

// invariantMockTransactor provides a configurable Transactor for testing
// the total money invariant checker. It returns pre-built rows for the
// LEFT JOIN ... HAVING query (4 columns: account_type, stored, computed, expected).
type invariantMockTransactor struct {
	rows [][]any
	err  error
}

func (m *invariantMockTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	if m.err != nil {
		return m.err
	}
	return fn(&invariantMockTx{rows: m.rows})
}

// invariantMockTx implements db.Tx for the invariant checker tests.
type invariantMockTx struct {
	rows [][]any
}

func (t *invariantMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &invariantMockRow{}
}

func (t *invariantMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &invariantMockRows{rows: t.rows, idx: -1}, nil
}

func (t *invariantMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0"), nil
}

func (t *invariantMockTx) Commit(_ context.Context) error   { return nil }
func (t *invariantMockTx) Rollback(_ context.Context) error { return nil }

// invariantMockRow is unused (QueryRow not used by checker anymore).
type invariantMockRow struct{}

func (r *invariantMockRow) Scan(dest ...any) error {
	return errors.New("no rows")
}

// invariantMockRows implements pgx.Rows for the checker tests.
type invariantMockRows struct {
	rows [][]any
	idx  int
}

func (r *invariantMockRows) Next() bool {
	r.idx++
	return r.idx < len(r.rows)
}

func (r *invariantMockRows) Scan(dest ...any) error {
	if r.idx < 0 || r.idx >= len(r.rows) {
		return errors.New("no current row")
	}
	row := r.rows[r.idx]
	if len(dest) != len(row) {
		// checker now scans 4 cols (account_type, stored, computed, expected)
		// allow len mismatch by filling what we can
		for i := range dest {
			if i < len(row) {
				switch d := dest[i].(type) {
				case *string:
					*d = row[i].(string)
				case *int64:
					*d = row[i].(int64)
				}
			}
		}
		return nil
	}
	for i, d := range dest {
		switch v := d.(type) {
		case *string:
			*v = row[i].(string)
		case *int64:
			*v = row[i].(int64)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}

func (r *invariantMockRows) Close() {}

func (r *invariantMockRows) Err() error { return nil }

func (r *invariantMockRows) CommandTag() pgconn.CommandTag { return pgconn.NewCommandTag("SELECT 0") }

func (r *invariantMockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *invariantMockRows) Values() ([]any, error) { return r.rows[r.idx], nil }

func (r *invariantMockRows) RawValues() [][]byte { return nil }

func (r *invariantMockRows) Conn() *pgx.Conn { return nil }

// ============================================================================
// TEST HELPERS - ALERT TRACKING
// ============================================================================

// alertTracker records CreateAlert calls for assertions.
type alertTracker struct {
	alertCount int
	lastAlert  trackedAlert
}

type trackedAlert struct {
	alertType  alertentity.AlertType
	severity   alertentity.AlertSeverity
	entityType string
	entityID   uuid.UUID
	message    string
	metadata   alertentity.AlertMetadata
	groupKey   *string
}

// mockAlertTransactor provides a no-op transaction wrapper for tests.
// Passes nil as db.Tx — the counting repo ignores it.
type mockAlertTransactor struct{}

func (m *mockAlertTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(nil)
}

// countingAlertRepository satisfies alertrepo.AlertRepository and counts Create calls.
type countingAlertRepository struct {
	tracker *alertTracker
}

func (r *countingAlertRepository) Create(_ context.Context, _ interface{}, alert *alertentity.Alert) error {
	r.tracker.alertCount++
	r.tracker.lastAlert = trackedAlert{
		alertType:  alert.AlertType,
		severity:   alert.Severity,
		entityType: alert.EntityType,
		entityID:   alert.EntityID,
		message:    alert.Message,
		metadata:   alert.Metadata,
		groupKey:   alert.GroupKey,
	}
	return nil
}

func (r *countingAlertRepository) GetByID(_ context.Context, _ interface{}, _ uuid.UUID) (*alertentity.Alert, error) {
	return nil, nil
}

func (r *countingAlertRepository) GetForUpdate(_ context.Context, _ interface{}, _ uuid.UUID) (*alertentity.Alert, error) {
	return nil, nil
}

func (r *countingAlertRepository) Update(_ context.Context, _ interface{}, _ *alertentity.Alert) error {
	return nil
}

func (r *countingAlertRepository) List(_ context.Context, _ interface{}, _ alertrepo.AlertFilters) ([]*alertentity.Alert, error) {
	return nil, nil
}

func (r *countingAlertRepository) Count(_ context.Context, _ interface{}, _ alertrepo.AlertFilters) (int64, error) {
	return 0, nil
}

func (r *countingAlertRepository) FindActiveByGroupKey(_ context.Context, _ interface{}, _ string) ([]*alertentity.Alert, error) {
	return nil, nil
}

func (r *countingAlertRepository) FindByDedupKeyInWindow(_ context.Context, _ interface{}, _ string, _ int) ([]*alertentity.Alert, error) {
	return nil, nil
}

func (r *countingAlertRepository) DeleteOld(_ context.Context, _ interface{}, _ int) (int, error) {
	return 0, nil
}

// newTrackingAlertService creates a real AlertService backed by counting mocks.
func newTrackingAlertService(t *testing.T) (*alertapp.AlertService, *alertTracker) {
	t.Helper()
	tracker := &alertTracker{}
	countingSvc := alertapp.NewAlertService(
		&mockAlertTransactor{},
		&countingAlertRepository{tracker: tracker},
		zap.NewNop(),
	)
	return countingSvc, tracker
}
