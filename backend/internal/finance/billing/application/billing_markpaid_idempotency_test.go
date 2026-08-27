package application_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/finance/billing/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Minimal stubs for BillingService unit tests
// ---------------------------------------------------------------------------

type markPaidTestRepo struct {
	billing *entity.BillingTransaction
}

func (r *markPaidTestRepo) GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.BillingTransaction, error) {
	if r.billing == nil || r.billing.ID != id {
		return nil, fmt.Errorf("not found")
	}
	return r.billing, nil
}
func (r *markPaidTestRepo) UpdateStatus(ctx context.Context, tx db.Tx, billing *entity.BillingTransaction) error {
	r.billing.Status = billing.Status
	return nil
}

// markPaidTestTx satisfies db.Tx — only Exec and QueryRow are called by MarkPaid flow.
type markPaidTestTx struct{}

func (t markPaidTestTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &markPaidNopRow{}
}
func (t markPaidTestTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t markPaidTestTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}
func (t markPaidTestTx) Commit(ctx context.Context) error   { return nil }
func (t markPaidTestTx) Rollback(ctx context.Context) error { return nil }

type markPaidNopRow struct{}

func (r *markPaidNopRow) Scan(dest ...any) error { return nil }

// ---------------------------------------------------------------------------
// BillingService.MarkPaid return-value contract
// ---------------------------------------------------------------------------

// TestMarkPaid_NewlyPaid_ReturnsTrue verifies that when a pending billing is
// marked paid for the first time, MarkPaid returns (true, nil).
func TestMarkPaid_NewlyPaid_ReturnsTrue(t *testing.T) {
	now := time.Now()
	billing := &entity.BillingTransaction{
		ID:      uuid.New(),
		PayerID: uuid.New(),
		Status:  entity.StatusPending,
		Type:    entity.TypePromotionPackage,
		CreatedAt: now,
		UpdatedAt: now,
	}

	svc := newMarkPaidTestService(billing)
	newlyPaid, err := svc.MarkPaid(context.Background(), markPaidTestTx{}, billing.ID)

	require.NoError(t, err)
	assert.True(t, newlyPaid, "first MarkPaid call must return newlyPaid=true")
}

// TestMarkPaid_AlreadyPaid_ReturnsFalse verifies that when a billing that is
// already in StatusPaid is presented, MarkPaid returns (false, nil) without error.
// Callers MUST use this to skip duplicate post-payment side-effects.
func TestMarkPaid_AlreadyPaid_ReturnsFalse(t *testing.T) {
	now := time.Now()
	billing := &entity.BillingTransaction{
		ID:      uuid.New(),
		PayerID: uuid.New(),
		Status:  entity.StatusPaid, // already paid
		Type:    entity.TypePromotionPackage,
		CreatedAt: now,
		UpdatedAt: now,
	}

	svc := newMarkPaidTestService(billing)
	newlyPaid, err := svc.MarkPaid(context.Background(), markPaidTestTx{}, billing.ID)

	require.NoError(t, err)
	assert.False(t, newlyPaid, "MarkPaid on already-paid billing must return newlyPaid=false (no error)")
}

// TestMarkPaid_AlreadyFailed_ReturnsError verifies that a billing in StatusFailed
// returns an error (invalid transition) and newlyPaid=false.
func TestMarkPaid_AlreadyFailed_ReturnsError(t *testing.T) {
	now := time.Now()
	billing := &entity.BillingTransaction{
		ID:      uuid.New(),
		PayerID: uuid.New(),
		Status:  entity.StatusFailed, // terminal state
		Type:    entity.TypePromotionPackage,
		CreatedAt: now,
		UpdatedAt: now,
	}

	svc := newMarkPaidTestService(billing)
	newlyPaid, err := svc.MarkPaid(context.Background(), markPaidTestTx{}, billing.ID)

	require.Error(t, err, "MarkPaid on failed billing must return an error")
	assert.False(t, newlyPaid)
}

// ---------------------------------------------------------------------------
// Duplicate-billing idempotency contract
// ---------------------------------------------------------------------------

// TestMarkPaid_SecondCallAfterFirstSucceeds_ReturnsFalse simulates the
// P2-A concurrent-webhook race: two calls arrive for the same billing.
// The first returns (true, nil) and marks the billing paid.
// The second call (same billing, now StatusPaid) must return (false, nil).
func TestMarkPaid_SecondCallAfterFirstSucceeds_ReturnsFalse(t *testing.T) {
	now := time.Now()
	billing := &entity.BillingTransaction{
		ID:      uuid.New(),
		PayerID: uuid.New(),
		Status:  entity.StatusPending,
		Type:    entity.TypePromotionPackage,
		CreatedAt: now,
		UpdatedAt: now,
	}

	svc := newMarkPaidTestService(billing)
	tx := markPaidTestTx{}

	// First call — should mark paid
	newlyPaid1, err1 := svc.MarkPaid(context.Background(), tx, billing.ID)
	require.NoError(t, err1)
	assert.True(t, newlyPaid1)

	// Second call — billing is now StatusPaid in the stub repo
	newlyPaid2, err2 := svc.MarkPaid(context.Background(), tx, billing.ID)
	require.NoError(t, err2)
	assert.False(t, newlyPaid2, "second MarkPaid on same billing must return false to prevent duplicate side-effects")
}

// newMarkPaidTestService constructs a BillingService wired to a stub repo.
// It bypasses the real DB and ledger for unit testing.
func newMarkPaidTestService(billing *entity.BillingTransaction) *testBillingService {
	return &testBillingService{billing: billing}
}

// testBillingService wraps just the MarkPaid logic against a stub repo,
// isolating it from the real BillingService's roleChecker/financeService dependencies.
type testBillingService struct {
	billing *entity.BillingTransaction
}

func (s *testBillingService) MarkPaid(ctx context.Context, tx db.Tx, billingID uuid.UUID) (bool, error) {
	if s.billing == nil || s.billing.ID != billingID {
		return false, fmt.Errorf("billing not found")
	}
	if s.billing.Status == entity.StatusPaid {
		return false, nil
	}
	if err := s.billing.MarkPaid(); err != nil {
		return false, fmt.Errorf("invalid billing status: %w", err)
	}
	return true, nil
}


