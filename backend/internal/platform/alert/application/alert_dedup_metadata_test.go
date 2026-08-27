package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/internal/platform/alert/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// K-1 FIX PROOF: alert dedup must merge the LATEST occurrence's domain
// metadata (e.g. reconciliation account_mismatches), not discard it in
// favor of the first occurrence's stale snapshot.
//
// These tests exercise the real AlertService.CreateAlertWithDedupWindow
// implementation (not a mock of it), using an in-memory AlertRepository
// test double so no real database is required.
// ============================================================================

// fakeAlertAppTransactor runs the callback directly; the fake repository
// below never dereferences tx.
type fakeAlertAppTransactor struct{}

func (fakeAlertAppTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(nil)
}

// fakeAlertAppRepo is an in-memory AlertRepository test double.
type fakeAlertAppRepo struct {
	alerts map[uuid.UUID]*entity.Alert
}

func newFakeAlertAppRepo() *fakeAlertAppRepo {
	return &fakeAlertAppRepo{alerts: map[uuid.UUID]*entity.Alert{}}
}

func (f *fakeAlertAppRepo) Create(ctx context.Context, tx interface{}, alert *entity.Alert) error {
	f.alerts[alert.ID] = alert
	return nil
}

func (f *fakeAlertAppRepo) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Alert, error) {
	a, ok := f.alerts[id]
	if !ok {
		return nil, fmt.Errorf("alert not found: %s", id)
	}
	return a, nil
}

func (f *fakeAlertAppRepo) GetForUpdate(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Alert, error) {
	return f.GetByID(ctx, tx, id)
}

func (f *fakeAlertAppRepo) Update(ctx context.Context, tx interface{}, alert *entity.Alert) error {
	f.alerts[alert.ID] = alert
	return nil
}

func (f *fakeAlertAppRepo) List(ctx context.Context, tx interface{}, filters repository.AlertFilters) ([]*entity.Alert, error) {
	return nil, nil
}

func (f *fakeAlertAppRepo) Count(ctx context.Context, tx interface{}, filters repository.AlertFilters) (int64, error) {
	return 0, nil
}

func (f *fakeAlertAppRepo) FindActiveByGroupKey(ctx context.Context, tx interface{}, groupKey string) ([]*entity.Alert, error) {
	var out []*entity.Alert
	for _, a := range f.alerts {
		if a.GroupKey != nil && *a.GroupKey == groupKey && a.IsActive() {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAlertAppRepo) FindByDedupKeyInWindow(ctx context.Context, tx interface{}, dedupKey string, minutes int) ([]*entity.Alert, error) {
	var out []*entity.Alert
	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)
	for _, a := range f.alerts {
		if a.DedupKey == dedupKey && a.CreatedAt.After(cutoff) {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAlertAppRepo) DeleteOld(ctx context.Context, tx interface{}, olderThan int) (int, error) {
	return 0, nil
}

var _ repository.AlertRepository = (*fakeAlertAppRepo)(nil)

// ============================================================================
// DEDUP-KEY BRANCH
// ============================================================================

func TestCreateAlertWithDedupWindow_DedupKeyBranch_MergesLatestMetadata(t *testing.T) {
	repo := newFakeAlertAppRepo()
	svc := NewAlertService(fakeAlertAppTransactor{}, repo, zap.NewNop())
	ctx := context.Background()
	groupKey := "reconciliation_drift:active"

	result1, err := svc.CreateAlertWithDedupWindow(
		ctx,
		entity.AlertTypeReconciliationDrift,
		entity.SeverityHigh,
		"system", uuid.Nil,
		"Reconciliation drift detected: 1/5 accounts with mismatches",
		entity.AlertMetadata{
			"mismatched_accounts": 1,
			"total_accounts":      5,
			"account_mismatches":  []string{"account-A"},
		},
		&groupKey,
		60,
	)
	require.NoError(t, err)
	require.True(t, result1.Created)

	// Second occurrence within the dedup window: drift evolved to a
	// different, larger set of accounts and a higher severity.
	result2, err := svc.CreateAlertWithDedupWindow(
		ctx,
		entity.AlertTypeReconciliationDrift,
		entity.SeverityCritical,
		"system", uuid.Nil,
		"Reconciliation drift detected: 3/5 accounts with mismatches",
		entity.AlertMetadata{
			"mismatched_accounts": 3,
			"total_accounts":      5,
			"account_mismatches":  []string{"account-A", "account-B", "account-C"},
		},
		&groupKey,
		60,
	)
	require.NoError(t, err)
	assert.False(t, result2.Created, "second occurrence within window must dedup, not create a new alert")
	assert.Equal(t, result1.Alert.ID, result2.Alert.ID)

	// K-1: the merged alert must reflect the LATEST occurrence's domain
	// details, not the stale first-occurrence snapshot.
	assert.Equal(t, 3, result2.Alert.Metadata["mismatched_accounts"])
	assert.Equal(t, []string{"account-A", "account-B", "account-C"}, result2.Alert.Metadata["account_mismatches"])

	// Severity must escalate to the more severe incoming value.
	assert.Equal(t, entity.SeverityCritical, result2.Alert.Severity)

	// Dedup bookkeeping must remain correct across the merge.
	assert.Equal(t, 2, result2.Alert.Metadata["occurrence_count"])
	assert.NotNil(t, result2.Alert.Metadata["first_occurrence"], "first_occurrence must be preserved across merges")
	assert.NotNil(t, result2.Alert.Metadata["last_occurrence"])
}

func TestCreateAlertWithDedupWindow_DedupKeyBranch_DoesNotDowngradeSeverity(t *testing.T) {
	repo := newFakeAlertAppRepo()
	svc := NewAlertService(fakeAlertAppTransactor{}, repo, zap.NewNop())
	ctx := context.Background()
	groupKey := "reconciliation_drift:active"

	result1, err := svc.CreateAlertWithDedupWindow(
		ctx,
		entity.AlertTypeReconciliationDrift,
		entity.SeverityCritical,
		"system", uuid.Nil,
		"Reconciliation drift detected: 3/5 accounts with mismatches",
		entity.AlertMetadata{"mismatched_accounts": 3},
		&groupKey,
		60,
	)
	require.NoError(t, err)
	require.True(t, result1.Created)

	// A later, LESS severe occurrence must still merge fresh metadata but
	// must not downgrade the alert's severity below what was already seen.
	result2, err := svc.CreateAlertWithDedupWindow(
		ctx,
		entity.AlertTypeReconciliationDrift,
		entity.SeverityMedium,
		"system", uuid.Nil,
		"Reconciliation drift detected: 1/5 accounts with mismatches",
		entity.AlertMetadata{"mismatched_accounts": 1},
		&groupKey,
		60,
	)
	require.NoError(t, err)
	assert.False(t, result2.Created)
	assert.Equal(t, entity.SeverityCritical, result2.Alert.Severity, "severity must not downgrade within the dedup window")
	assert.Equal(t, 1, result2.Alert.Metadata["mismatched_accounts"], "metadata must still merge the latest occurrence")
}

// ============================================================================
// GROUP-KEY BRANCH (different dedup_key, same manual group_key)
// ============================================================================

func TestCreateAlertWithDedupWindow_GroupKeyBranch_MergesLatestMetadata(t *testing.T) {
	repo := newFakeAlertAppRepo()
	svc := NewAlertService(fakeAlertAppTransactor{}, repo, zap.NewNop())
	ctx := context.Background()
	groupKey := "reconciliation_drift:active"

	result1, err := svc.CreateAlertWithDedupWindow(
		ctx,
		entity.AlertTypeReconciliationDrift,
		entity.SeverityHigh,
		"system", uuid.Nil,
		"drift 1",
		entity.AlertMetadata{"mismatched_accounts": 1},
		&groupKey,
		60,
	)
	require.NoError(t, err)
	require.True(t, result1.Created)

	// A different entity_id produces a different dedup_key, so this call
	// misses the dedup-key branch and must fall through to the group_key
	// branch instead.
	result2, err := svc.CreateAlertWithDedupWindow(
		ctx,
		entity.AlertTypeReconciliationDrift,
		entity.SeverityCritical,
		"system", uuid.New(),
		"drift 2",
		entity.AlertMetadata{"mismatched_accounts": 4},
		&groupKey,
		60,
	)
	require.NoError(t, err)
	assert.False(t, result2.Created, "group_key match must dedup even with a different dedup_key")
	assert.Equal(t, result1.Alert.ID, result2.Alert.ID)
	assert.Equal(t, 4, result2.Alert.Metadata["mismatched_accounts"], "group-key branch must also merge latest metadata")
	assert.Equal(t, entity.SeverityCritical, result2.Alert.Severity)
	assert.Equal(t, 2, result2.Alert.Metadata["occurrence_count"])
}

// ============================================================================
// OUTSIDE WINDOW — a genuinely new alert must not inherit stale metadata
// ============================================================================

func TestCreateAlertWithDedupWindow_OutsideWindow_CreatesFreshAlert(t *testing.T) {
	repo := newFakeAlertAppRepo()
	svc := NewAlertService(fakeAlertAppTransactor{}, repo, zap.NewNop())
	ctx := context.Background()
	groupKey := "reconciliation_drift:active"

	result1, err := svc.CreateAlertWithDedupWindow(
		ctx,
		entity.AlertTypeReconciliationDrift,
		entity.SeverityHigh,
		"system", uuid.Nil,
		"drift 1",
		entity.AlertMetadata{"mismatched_accounts": 1},
		&groupKey,
		60,
	)
	require.NoError(t, err)

	// Simulate the first alert having aged out of both the dedup window and
	// the group-key "active" set by resolving it.
	result1.Alert.Resolve(uuid.New())
	repo.alerts[result1.Alert.ID] = result1.Alert

	result2, err := svc.CreateAlertWithDedupWindow(
		ctx,
		entity.AlertTypeReconciliationDrift,
		entity.SeverityHigh,
		"system", uuid.Nil,
		"drift 2",
		entity.AlertMetadata{"mismatched_accounts": 9},
		&groupKey,
		60,
	)
	require.NoError(t, err)
	assert.True(t, result2.Created, "a resolved alert must not suppress a new occurrence")
	assert.NotEqual(t, result1.Alert.ID, result2.Alert.ID)
	assert.Equal(t, 9, result2.Alert.Metadata["mismatched_accounts"])
	assert.Equal(t, 1, result2.Alert.Metadata["occurrence_count"])
}
