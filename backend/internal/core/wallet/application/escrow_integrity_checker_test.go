package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	walletentity "github.com/labuda/backend/internal/core/wallet/entity"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	alertrepo "github.com/labuda/backend/internal/platform/alert/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// ESCROW INTEGRITY CHECKER - UNIT TESTS
// ============================================================================
// These tests verify the gateway-funded escrow invariant:
// - Order.escrow_amount must match the canonical escrows.amount row
// - Total holding order escrow must match total holding escrows.amount
// - Shadow mode suppresses alerts

func TestToleranceConstant(t *testing.T) {
	assert.Equal(t, int64(100), EscrowToleranceAmount)
}

func TestTimingGraceConstant(t *testing.T) {
	assert.Equal(t, 5, TimingGraceMinutes)
}

func TestCheckOrderEscrow_MatchingEscrowRow_NoAlert(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	checker := NewEscrowIntegrityChecker(
		&mockEscrowLookup{
			escrows: map[uuid.UUID]*walletentity.Escrow{},
		},
		alertSvc,
		nil,
		zap.NewNop(),
		false,
	)

	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	row := holdingOrderRow{
		ID:           orderID,
		BuyerID:      buyerID,
		SellerID:     sellerID,
		EscrowAmount: 15000,
	}
	checker.escrowLookup.(*mockEscrowLookup).escrows[orderID] = &walletentity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  15000,
		Status:  walletentity.EscrowStatusHolding,
	}

	err := checker.checkOrderEscrow(context.Background(), &mockTx{}, row)
	require.NoError(t, err)
	assert.Equal(t, 0, tracker.alertCount)
}

func TestCheckOrderEscrow_MissingEscrow_Alerts(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	checker := NewEscrowIntegrityChecker(
		&mockEscrowLookup{escrows: map[uuid.UUID]*walletentity.Escrow{}},
		alertSvc,
		nil,
		zap.NewNop(),
		false,
	)

	row := holdingOrderRow{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		EscrowAmount: 15000,
	}

	err := checker.checkOrderEscrow(context.Background(), &mockTx{}, row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escrow row not found")
	require.Equal(t, 1, tracker.alertCount)
	assert.Contains(t, tracker.lastAlert.metadata, "escrow_amount")
	assert.Equal(t, "escrow_row_not_found_for_order_with_escrow", tracker.lastAlert.metadata["reason"])
}

func TestCheckOrderEscrow_StatusMismatch_Alerts(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	orderID := uuid.New()
	checker := NewEscrowIntegrityChecker(
		&mockEscrowLookup{
			escrows: map[uuid.UUID]*walletentity.Escrow{
				orderID: {
					ID:      uuid.New(),
					OrderID: orderID,
					Amount:  15000,
					Status:  walletentity.EscrowStatusReleased,
				},
			},
		},
		alertSvc,
		nil,
		zap.NewNop(),
		false,
	)

	row := holdingOrderRow{
		ID:           orderID,
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		EscrowAmount: 15000,
	}

	err := checker.checkOrderEscrow(context.Background(), &mockTx{}, row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escrow status mismatch")
	require.Equal(t, 1, tracker.alertCount)
	assert.Equal(t, "holding", tracker.lastAlert.metadata["expected_status"])
	assert.Equal(t, string(walletentity.EscrowStatusReleased), tracker.lastAlert.metadata["actual_status"])
}

func TestCheckOrderEscrow_AmountMismatch_Alerts(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	orderID := uuid.New()
	checker := NewEscrowIntegrityChecker(
		&mockEscrowLookup{
			escrows: map[uuid.UUID]*walletentity.Escrow{
				orderID: {
					ID:      uuid.New(),
					OrderID: orderID,
					Amount:  14800,
					Status:  walletentity.EscrowStatusHolding,
				},
			},
		},
		alertSvc,
		nil,
		zap.NewNop(),
		false,
	)

	row := holdingOrderRow{
		ID:           orderID,
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		EscrowAmount: 15000,
	}

	err := checker.checkOrderEscrow(context.Background(), &mockTx{}, row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escrow amount mismatch")
	require.Equal(t, 1, tracker.alertCount)
	assert.Contains(t, tracker.lastAlert.metadata, "escrow_row_amount")
	assert.Equal(t, int64(14800), tracker.lastAlert.metadata["escrow_row_amount"])
	assert.Equal(t, int64(15000), tracker.lastAlert.metadata["order_escrow_amount"])
}

func TestCheckGlobalEscrowInvariant_MatchingTotals_NoAlert(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	checker := NewEscrowIntegrityChecker(nil, alertSvc, &mockCheckerTransactor{
		orderTotal:  1000000,
		escrowTotal: 1000000,
	}, zap.NewNop(), false)

	mismatch, err := checker.checkGlobalEscrowInvariant(context.Background(), &mockTx{
		orderTotal:  1000000,
		escrowTotal: 1000000,
	})
	require.NoError(t, err)
	assert.False(t, mismatch)
	assert.Equal(t, 0, tracker.alertCount)
}

func TestCheckGlobalEscrowInvariant_ExceedsTolerance_Alert(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	checker := NewEscrowIntegrityChecker(nil, alertSvc, &mockCheckerTransactor{
		orderTotal:  1000000,
		escrowTotal: 1000200,
	}, zap.NewNop(), false)

	mismatch, err := checker.checkGlobalEscrowInvariant(context.Background(), &mockTx{
		orderTotal:  1000000,
		escrowTotal: 1000200,
	})
	require.NoError(t, err)
	assert.True(t, mismatch)
	require.Equal(t, 1, tracker.alertCount)
	assert.Equal(t, int64(1000000), tracker.lastAlert.metadata["total_order_escrow"])
	assert.Equal(t, int64(1000200), tracker.lastAlert.metadata["total_escrow_rows"])
}

func TestCheckEscrowIntegrity_EndToEnd_MatchingSnapshot_NoAlert(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	alertSvc, tracker := newTrackingAlertService(t)
	checker := NewEscrowIntegrityChecker(
		&mockEscrowLookup{
			escrows: map[uuid.UUID]*walletentity.Escrow{
				orderID: {
					ID:      uuid.New(),
					OrderID: orderID,
					Amount:  15000,
					Status:  walletentity.EscrowStatusHolding,
				},
			},
		},
		alertSvc,
		&mockCheckerTransactor{
			orders: []holdingOrderRow{
				{ID: orderID, BuyerID: buyerID, SellerID: sellerID, EscrowAmount: 15000},
			},
			orderTotal:  15000,
			escrowTotal: 15000,
		},
		zap.NewNop(),
		false,
	)

	totalMismatches, err := checker.CheckEscrowIntegrity(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, totalMismatches)
	assert.Equal(t, 0, tracker.alertCount)
}

func TestShadowMode_SuppressesAlerts(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	checker := NewEscrowIntegrityChecker(
		&mockEscrowLookup{escrows: map[uuid.UUID]*walletentity.Escrow{}},
		alertSvc,
		nil,
		zap.NewNop(),
		true,
	)

	row := holdingOrderRow{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		EscrowAmount: 15000,
	}

	checker.emitInvalidEscrowAmountAlert(context.Background(), row)
	checker.emitMissingEscrowAlert(context.Background(), row)
	checker.emitEscrowStatusMismatchAlert(context.Background(), row, string(walletentity.EscrowStatusReleased))
	checker.emitEscrowAmountMismatchAlert(context.Background(), row, 5000)
	checker.emitGlobalEscrowImbalanceAlert(context.Background(), 100000, 200000)

	assert.Equal(t, 0, tracker.alertCount)
}

func TestConstructor_ShadowModeAndNilLogger(t *testing.T) {
	t.Run("shadow=true", func(t *testing.T) {
		checker := NewEscrowIntegrityChecker(nil, nil, nil, nil, true)
		assert.True(t, checker.shadowMode)
	})

	t.Run("shadow=false", func(t *testing.T) {
		checker := NewEscrowIntegrityChecker(nil, nil, nil, nil, false)
		assert.False(t, checker.shadowMode)
	})

	t.Run("nil logger defaults to nop", func(t *testing.T) {
		checker := NewEscrowIntegrityChecker(nil, nil, nil, nil, false)
		require.NotNil(t, checker.log)
	})
}

func TestAlertMetadata_ContainsCurrentModelFields(t *testing.T) {
	alertSvc, tracker := newTrackingAlertService(t)
	checker := NewEscrowIntegrityChecker(nil, alertSvc, nil, zap.NewNop(), false)

	row := holdingOrderRow{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		EscrowAmount: 15000,
	}

	checker.emitEscrowAmountMismatchAlert(context.Background(), row, 5000)
	require.Equal(t, 1, tracker.alertCount)

	lastAlert := tracker.lastAlert
	assert.Equal(t, alertentity.AlertTypeReconciliationDrift, lastAlert.alertType)
	assert.Equal(t, alertentity.SeverityCritical, lastAlert.severity)
	assert.Equal(t, "order", lastAlert.entityType)
	assert.Equal(t, row.ID, lastAlert.entityID)
	assert.Contains(t, lastAlert.metadata, "order_escrow_amount")
	assert.Contains(t, lastAlert.metadata, "escrow_row_amount")
	assert.NotContains(t, lastAlert.metadata, "wallet_held_balance")
}

// ============================================================================
// TEST HELPERS - MOCK INFRASTRUCTURE
// ============================================================================

type mockEscrowLookup struct {
	escrows map[uuid.UUID]*walletentity.Escrow
	err     error
}

func (m *mockEscrowLookup) GetEscrowForOrder(_ context.Context, _ db.Tx, orderID uuid.UUID) (*walletentity.Escrow, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.escrows == nil {
		return nil, nil
	}
	return m.escrows[orderID], nil
}

type mockCheckerTransactor struct {
	orders      []holdingOrderRow
	orderTotal  int64
	escrowTotal int64
}

func (m *mockCheckerTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(&mockTx{
		orders:      m.orders,
		orderTotal:  m.orderTotal,
		escrowTotal: m.escrowTotal,
	})
}

type mockTx struct {
	orders      []holdingOrderRow
	orderTotal  int64
	escrowTotal int64
}

func (m *mockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0"), nil
}

func (m *mockTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "FROM orders") && strings.Contains(sql, "escrow_status = 'holding'") {
		return &mockRows{orders: m.orders}, nil
	}
	return nil, errors.New("unexpected query")
}

func (m *mockTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	if strings.Contains(sql, "FROM orders") {
		return &mockRow{value: m.orderTotal}
	}
	if strings.Contains(sql, "FROM escrows") {
		return &mockRow{value: m.escrowTotal}
	}
	return &mockRow{err: errors.New("unexpected query")}
}

func (m *mockTx) Commit(_ context.Context) error   { return nil }
func (m *mockTx) Rollback(_ context.Context) error { return nil }

type mockRows struct {
	orders []holdingOrderRow
	idx    int
}

func (r *mockRows) Close()                                       {}
func (r *mockRows) Err() error                                   { return nil }
func (r *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 0") }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) Next() bool {
	if r.idx >= len(r.orders) {
		return false
	}
	r.idx++
	return true
}
func (r *mockRows) Scan(dest ...any) error {
	order := r.orders[r.idx-1]
	if len(dest) != 4 {
		return errors.New("expected 4 scan destinations")
	}
	if p, ok := dest[0].(*uuid.UUID); ok {
		*p = order.ID
	}
	if p, ok := dest[1].(*uuid.UUID); ok {
		*p = order.BuyerID
	}
	if p, ok := dest[2].(*uuid.UUID); ok {
		*p = order.SellerID
	}
	if p, ok := dest[3].(*int64); ok {
		*p = order.EscrowAmount
	}
	return nil
}
func (r *mockRows) Values() ([]any, error) { return nil, nil }
func (r *mockRows) RawValues() [][]byte    { return nil }
func (r *mockRows) Conn() *pgx.Conn        { return nil }

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
	p, ok := dest[0].(*int64)
	if !ok {
		return errors.New("expected *int64 destination")
	}
	*p = r.value
	return nil
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
// Passes nil as db.Tx â€” the counting repo ignores it.
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


