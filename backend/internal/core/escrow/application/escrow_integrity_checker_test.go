package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	alertrepo "github.com/labuda/backend/internal/platform/alert/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// ESCROW INTEGRITY CHECKER - UNIT TESTS (canonical escrow model)
// ============================================================================
// The escrows table is the SOLE authority for escrow existence/amount/state.
// These tests prove the rewritten checker:
//   - settled payment without escrow row  => missing-escrow-<order_id> alert
//   - escrow row without settled payment => phantom-escrow-<order_id> alert
//   - amount drift beyond tolerance       => escrow-amount-mismatch-<order_id>
//   - clean state                         => 0 mismatches, 0 alerts

func TestToleranceConstant(t *testing.T) {
	assert.Equal(t, int64(100), EscrowToleranceAmount)
}

// (a) A settled payment with no escrow row is a settlement-invariant break:
// exactly one mismatch and a missing-escrow-<order_id> alert.
func TestCheckEscrowIntegrity_SettledPaymentWithoutEscrow_Alerts(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	checker := NewEscrowIntegrityChecker(alertSvc, &mockCheckerTransactor{
		tx: &mockTx{
			settledMissingEscrow: []settledMissingEscrowRow{
				{OrderID: orderID, BuyerID: buyerID, SellerID: sellerID, Amount: 15000},
			},
		},
	}, zap.NewNop(), false)

	totalMismatches, err := checker.CheckEscrowIntegrity(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, totalMismatches)
	require.Equal(t, 1, tracker.alertCount)
	assert.Equal(t, "missing-escrow-"+orderID.String(), *tracker.lastAlert.groupKey)
	assert.Equal(t, alertentity.AlertTypeReconciliationDrift, tracker.lastAlert.alertType)
	assert.Equal(t, alertentity.SeverityCritical, tracker.lastAlert.severity)
	assert.Equal(t, "order", tracker.lastAlert.entityType)
	assert.Equal(t, orderID, tracker.lastAlert.entityID)
	assert.Equal(t, "settled_payment_without_escrow_row", tracker.lastAlert.metadata["reason"])
	assert.Equal(t, buyerID.String(), tracker.lastAlert.metadata["buyer_id"])
	assert.Equal(t, sellerID.String(), tracker.lastAlert.metadata["seller_id"])
}

// (b) An escrow row without a settled payment is a phantom escrow:
// exactly one mismatch and a phantom-escrow-<order_id> alert.
func TestCheckEscrowIntegrity_EscrowWithoutSettledPayment_Alerts(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	checker := NewEscrowIntegrityChecker(alertSvc, &mockCheckerTransactor{
		tx: &mockTx{
			phantomEscrows: []phantomEscrowRow{
				{
					OrderID:      orderID,
					BuyerID:      buyerID,
					SellerID:     sellerID,
					Amount:       15000,
					EscrowStatus: "holding",
					OrderStatus:  "paid",
				},
			},
		},
	}, zap.NewNop(), false)

	totalMismatches, err := checker.CheckEscrowIntegrity(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, totalMismatches)
	require.Equal(t, 1, tracker.alertCount)
	assert.Equal(t, "phantom-escrow-"+orderID.String(), *tracker.lastAlert.groupKey)
	assert.Equal(t, "escrow_row_without_settled_payment", tracker.lastAlert.metadata["reason"])
	assert.Equal(t, "holding", tracker.lastAlert.metadata["escrow_status"])
	assert.Equal(t, "paid", tracker.lastAlert.metadata["order_status"])
}

// (c) Escrow amount drifting beyond the rounding tolerance is a money-leak
// risk: escrow-amount-mismatch-<order_id> alert with both amounts recorded.
func TestCheckEscrowIntegrity_AmountMismatchBeyondTolerance_Alerts(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	checker := NewEscrowIntegrityChecker(alertSvc, &mockCheckerTransactor{
		tx: &mockTx{
			amountMismatches: []amountMismatchRow{
				{OrderID: orderID, BuyerID: buyerID, SellerID: sellerID, EscrowAmount: 14800, OrderBase: 15000},
			},
		},
	}, zap.NewNop(), false)

	totalMismatches, err := checker.CheckEscrowIntegrity(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, totalMismatches)
	require.Equal(t, 1, tracker.alertCount)
	assert.Equal(t, "escrow-amount-mismatch-"+orderID.String(), *tracker.lastAlert.groupKey)
	assert.Equal(t, "escrow_amount_does_not_match_order_base", tracker.lastAlert.metadata["reason"])
	assert.Equal(t, int64(14800), tracker.lastAlert.metadata["escrow_amount"])
	assert.Equal(t, int64(15000), tracker.lastAlert.metadata["order_base_amount"])
}

// (d) A clean canonical state (settled payments all have matching escrow
// rows) yields zero mismatches and zero alerts.
func TestCheckEscrowIntegrity_CleanState_ZeroMismatches(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewEscrowIntegrityChecker(alertSvc, &mockCheckerTransactor{
		tx: &mockTx{
			totalHoldingEscrow: 1000000,
		},
	}, zap.NewNop(), false)

	totalMismatches, err := checker.CheckEscrowIntegrity(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, totalMismatches)
	assert.Equal(t, 0, tracker.alertCount)
}

// A holding escrow sitting on a terminal order means release/refund never
// flipped the canonical row.
func TestCheckEscrowIntegrity_HoldingOnTerminalOrder_Alerts(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	checker := NewEscrowIntegrityChecker(alertSvc, &mockCheckerTransactor{
		tx: &mockTx{
			holdingTerminal: []holdingTerminalRow{
				{
					OrderID:     orderID,
					BuyerID:     buyerID,
					SellerID:    sellerID,
					Amount:      15000,
					OrderStatus: "completed",
				},
			},
			totalHoldingEscrow: 15000,
		},
	}, zap.NewNop(), false)

	totalMismatches, err := checker.CheckEscrowIntegrity(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, totalMismatches)
	require.Equal(t, 1, tracker.alertCount)
	assert.Equal(t, "holding-escrow-terminal-order-"+orderID.String(), *tracker.lastAlert.groupKey)
	assert.Equal(t, "holding_escrow_on_terminal_order", tracker.lastAlert.metadata["reason"])
}

// Duplicate escrow rows per order (defense-in-depth over UNIQUE(order_id)).
func TestCheckEscrowIntegrity_DuplicateEscrowRows_Alerts(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)

	checker := NewEscrowIntegrityChecker(alertSvc, &mockCheckerTransactor{
		tx: &mockTx{
			duplicateCount: 2,
		},
	}, zap.NewNop(), false)

	totalMismatches, err := checker.CheckEscrowIntegrity(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, totalMismatches)
	require.Equal(t, 1, tracker.alertCount)
	assert.Equal(t, "duplicate-escrow-rows", *tracker.lastAlert.groupKey)
	assert.Equal(t, "multiple_escrow_rows_per_order", tracker.lastAlert.metadata["reason"])
}

func TestShadowMode_SuppressesAlerts(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	orderID := uuid.New()

	checker := NewEscrowIntegrityChecker(alertSvc, &mockCheckerTransactor{
		tx: &mockTx{
			settledMissingEscrow: []settledMissingEscrowRow{
				{OrderID: orderID, BuyerID: uuid.New(), SellerID: uuid.New(), Amount: 15000},
			},
		},
	}, zap.NewNop(), true)

	totalMismatches, err := checker.CheckEscrowIntegrity(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, totalMismatches, "shadow mode still counts mismatches")
	assert.Equal(t, 0, tracker.alertCount, "shadow mode must not create alerts")
}

func TestConstructor_ShadowModeAndNilLogger(t *testing.T) {
	t.Run("shadow=true", func(t *testing.T) {
		checker := NewEscrowIntegrityChecker(nil, nil, nil, true)
		assert.True(t, checker.shadowMode)
	})

	t.Run("shadow=false", func(t *testing.T) {
		checker := NewEscrowIntegrityChecker(nil, nil, nil, false)
		assert.False(t, checker.shadowMode)
	})

	t.Run("nil logger defaults to nop", func(t *testing.T) {
		checker := NewEscrowIntegrityChecker(nil, nil, nil, false)
		require.NotNil(t, checker.log)
	})
}

func TestCheckEscrowIntegrity_NilDB_Error(t *testing.T) {
	checker := NewEscrowIntegrityChecker(nil, nil, nil, false)
	_, err := checker.CheckEscrowIntegrity(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db transactor not configured")
}

// ============================================================================
// TEST HELPERS - MOCK INFRASTRUCTURE
// ============================================================================

type settledMissingEscrowRow struct {
	OrderID  uuid.UUID
	BuyerID  uuid.UUID
	SellerID uuid.UUID
	Amount   int64
}

type phantomEscrowRow struct {
	OrderID      uuid.UUID
	BuyerID      uuid.UUID
	SellerID     uuid.UUID
	Amount       int64
	EscrowStatus string
	OrderStatus  string
}

type amountMismatchRow struct {
	OrderID      uuid.UUID
	BuyerID      uuid.UUID
	SellerID     uuid.UUID
	EscrowAmount int64
	OrderBase    int64
}

type holdingTerminalRow struct {
	OrderID     uuid.UUID
	BuyerID     uuid.UUID
	SellerID    uuid.UUID
	Amount      int64
	OrderStatus string
}

type mockCheckerTransactor struct {
	tx db.Tx
}

func (m *mockCheckerTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(m.tx)
}

// mockTx routes the checker's five canonical queries by SQL fingerprint and
// returns rows matching each query's exact column layout.
type mockTx struct {
	settledMissingEscrow []settledMissingEscrowRow
	phantomEscrows       []phantomEscrowRow
	amountMismatches     []amountMismatchRow
	holdingTerminal      []holdingTerminalRow
	duplicateCount       int
	totalHoldingEscrow  int64
}

func (m *mockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0"), nil
}

func (m *mockTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM payments p") && strings.Contains(sql, "e.id IS NULL"):
		return &settledMissingRows{rows: m.settledMissingEscrow}, nil
	case strings.Contains(sql, "FROM escrows e") && strings.Contains(sql, "LEFT JOIN payments p"):
		return &phantomRows{rows: m.phantomEscrows}, nil
	case strings.Contains(sql, "ABS(e.amount"):
		return &amountMismatchRows{rows: m.amountMismatches}, nil
	case strings.Contains(sql, "FROM escrows e") && strings.Contains(sql, "e.status = 'holding'"):
		return &holdingTerminalRows{rows: m.holdingTerminal}, nil
	}
	return nil, errors.New("unexpected query: " + sql)
}

func (m *mockTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	if strings.Contains(sql, "SUM(n - 1)") {
		return &mockRow{value: int64(m.duplicateCount)}
	}
	if strings.Contains(sql, "SUM(amount)") {
		return &mockRow{value: m.totalHoldingEscrow}
	}
	return &mockRow{err: errors.New("unexpected query: " + sql)}
}

func (m *mockTx) Commit(_ context.Context) error   { return nil }
func (m *mockTx) Rollback(_ context.Context) error { return nil }

type settledMissingRows struct {
	rows []settledMissingEscrowRow
	idx  int
}

func (r *settledMissingRows) Close()                                       {}
func (r *settledMissingRows) Err() error                                   { return nil }
func (r *settledMissingRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 0") }
func (r *settledMissingRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *settledMissingRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *settledMissingRows) Scan(dest ...any) error {
	row := r.rows[r.idx-1]
	if len(dest) != 4 {
		return errors.New("expected 4 scan destinations")
	}
	if p, ok := dest[0].(*uuid.UUID); ok {
		*p = row.OrderID
	}
	if p, ok := dest[1].(*uuid.UUID); ok {
		*p = row.BuyerID
	}
	if p, ok := dest[2].(*uuid.UUID); ok {
		*p = row.SellerID
	}
	if p, ok := dest[3].(*int64); ok {
		*p = row.Amount
	}
	return nil
}
func (r *settledMissingRows) Values() ([]any, error) { return nil, nil }
func (r *settledMissingRows) RawValues() [][]byte    { return nil }
func (r *settledMissingRows) Conn() *pgx.Conn        { return nil }

type phantomRows struct {
	rows []phantomEscrowRow
	idx  int
}

func (r *phantomRows) Close()                                       {}
func (r *phantomRows) Err() error                                   { return nil }
func (r *phantomRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 0") }
func (r *phantomRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *phantomRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *phantomRows) Scan(dest ...any) error {
	row := r.rows[r.idx-1]
	if len(dest) != 6 {
		return errors.New("expected 6 scan destinations")
	}
	if p, ok := dest[0].(*uuid.UUID); ok {
		*p = row.OrderID
	}
	if p, ok := dest[1].(*uuid.UUID); ok {
		*p = row.BuyerID
	}
	if p, ok := dest[2].(*uuid.UUID); ok {
		*p = row.SellerID
	}
	if p, ok := dest[3].(*int64); ok {
		*p = row.Amount
	}
	if p, ok := dest[4].(*string); ok {
		*p = row.EscrowStatus
	}
	if p, ok := dest[5].(*string); ok {
		*p = row.OrderStatus
	}
	return nil
}
func (r *phantomRows) Values() ([]any, error) { return nil, nil }
func (r *phantomRows) RawValues() [][]byte    { return nil }
func (r *phantomRows) Conn() *pgx.Conn        { return nil }

type amountMismatchRows struct {
	rows []amountMismatchRow
	idx  int
}

func (r *amountMismatchRows) Close()                                       {}
func (r *amountMismatchRows) Err() error                                   { return nil }
func (r *amountMismatchRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 0") }
func (r *amountMismatchRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *amountMismatchRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *amountMismatchRows) Scan(dest ...any) error {
	row := r.rows[r.idx-1]
	if len(dest) != 5 {
		return errors.New("expected 5 scan destinations")
	}
	if p, ok := dest[0].(*uuid.UUID); ok {
		*p = row.OrderID
	}
	if p, ok := dest[1].(*uuid.UUID); ok {
		*p = row.BuyerID
	}
	if p, ok := dest[2].(*uuid.UUID); ok {
		*p = row.SellerID
	}
	if p, ok := dest[3].(*int64); ok {
		*p = row.EscrowAmount
	}
	if p, ok := dest[4].(*int64); ok {
		*p = row.OrderBase
	}
	return nil
}
func (r *amountMismatchRows) Values() ([]any, error) { return nil, nil }
func (r *amountMismatchRows) RawValues() [][]byte    { return nil }
func (r *amountMismatchRows) Conn() *pgx.Conn        { return nil }

type holdingTerminalRows struct {
	rows []holdingTerminalRow
	idx  int
}

func (r *holdingTerminalRows) Close()                                       {}
func (r *holdingTerminalRows) Err() error                                   { return nil }
func (r *holdingTerminalRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 0") }
func (r *holdingTerminalRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *holdingTerminalRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *holdingTerminalRows) Scan(dest ...any) error {
	row := r.rows[r.idx-1]
	if len(dest) != 5 {
		return errors.New("expected 5 scan destinations")
	}
	if p, ok := dest[0].(*uuid.UUID); ok {
		*p = row.OrderID
	}
	if p, ok := dest[1].(*uuid.UUID); ok {
		*p = row.BuyerID
	}
	if p, ok := dest[2].(*uuid.UUID); ok {
		*p = row.SellerID
	}
	if p, ok := dest[3].(*int64); ok {
		*p = row.Amount
	}
	if p, ok := dest[4].(*string); ok {
		*p = row.OrderStatus
	}
	return nil
}
func (r *holdingTerminalRows) Values() ([]any, error) { return nil, nil }
func (r *holdingTerminalRows) RawValues() [][]byte    { return nil }
func (r *holdingTerminalRows) Conn() *pgx.Conn        { return nil }

type mockRow struct {
	value int64
	err   error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("expected 1 scan destination")
	}
	switch p := dest[0].(type) {
	case *int64:
		*p = r.value
		return nil
	case *int:
		*p = int(r.value)
		return nil
	}
	return errors.New("expected *int64 or *int destination")
}

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
