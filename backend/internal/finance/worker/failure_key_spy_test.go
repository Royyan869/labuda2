package worker

// failure_key_spy_test.go
//
// PROOF TESTS: C5.3-B
//
// These tests call the ACTUAL production code paths for:
//   - WebhookHandler.handleFailedCallback
//   - PayoutWorker.markSubmissionFailed
//
// and capture the idempotency key that reaches ledgerRepo.CreateTransaction,
// proving the live key is "withdrawal_gateway_restore_<withdrawal_id>" for both paths.
//
// This is NOT a string-comparison test. Both methods are exercised via a spy
// db.Tx that intercepts the QueryRow used for the idempotency uniqueness check
// inside LedgerRepository.CreateTransaction.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// SPY TX
// ============================================================================

// failureSpyTx implements db.Tx. It captures the idempotency key that
// LedgerRepository.CreateTransaction passes to its idempotency SELECT, and
// provides minimal happy-path responses for every other SQL call in the
// handleFailedCallback / markSubmissionFailed code paths.
type failureSpyTx struct {
	// Withdrawal data served by LockForUpdate
	withdrawalID     uuid.UUID
	sellerID         uuid.UUID
	amount           int64
	withdrawalStatus repository.WithdrawalStatus

	// Single fixed UUID returned for all financial_accounts lookups.
	// Using the same UUID for both DR and CR accounts keeps the double-entry
	// balance sum at zero, satisfying LedgerRepository's invariant check.
	fixedAcct uuid.UUID

	// OUTPUT populated by the spy when CreateTransaction calls the idempotency
	// SELECT ("WHERE idempotency_key = $1").
	CapturedIdemKey string
}

func (s *failureSpyTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "FROM withdrawals"):
		// LockForUpdate — return mock withdrawal (18 scanned fields)
		return &fspyWithdrawalRow{
			id:     s.withdrawalID,
			seller: s.sellerID,
			amount: s.amount,
			status: s.withdrawalStatus,
		}

	case strings.Contains(sql, "idempotency_key"):
		// CreateTransaction idempotency SELECT — capture the key and return
		// ErrNoRows so the code continues to INSERT (new-transaction path).
		if len(args) > 0 {
			if key, ok := args[0].(string); ok {
				s.CapturedIdemKey = key
			}
		}
		return &fspyNoRowsRow{}

	case strings.Contains(sql, "financial_accounts"):
		// GetSystemAccountID / GetUserAccountID — return fixed UUID
		return &fspyUUIDRow{id: s.fixedAcct}

	default:
		return &fspyErrRow{err: fmt.Errorf("failureSpyTx: unhandled QueryRow sql=%q", sql)}
	}
}

func (s *failureSpyTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "financial_accounts") {
		// FOR UPDATE account lock inside CreateTransaction
		return &fspyAccountRows{id: s.fixedAcct}, nil
	}
	return &fspyEmptyRows{}, nil
}

func (s *failureSpyTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	// MarkFailed requires RowsAffected() == 1 (new SQL guard checks status IN).
	if strings.Contains(sql, "UPDATE withdrawals") && strings.Contains(sql, "status IN") {
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	// IncrementRetryCount, INSERT ledger_transactions,
	// UPDATE financial_accounts, INSERT ledger_entries — all succeed.
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (s *failureSpyTx) Commit(_ context.Context) error   { return nil }
func (s *failureSpyTx) Rollback(_ context.Context) error { return nil }

// ============================================================================
// ROW / ROWS HELPERS
// ============================================================================

// fspyWithdrawalRow implements pgx.Row for LockForUpdate's 18-field Scan.
type fspyWithdrawalRow struct {
	id     uuid.UUID
	seller uuid.UUID
	amount int64
	status repository.WithdrawalStatus
}

func (r *fspyWithdrawalRow) Scan(dest ...any) error {
	now := time.Now()
	for i, d := range dest {
		switch i {
		case 0:
			*d.(*uuid.UUID) = r.id
		case 1:
			*d.(*uuid.UUID) = r.seller
		case 2:
			*d.(*int64) = r.amount
		case 3:
			*d.(*int64) = 500000
		case 4:
			*d.(*repository.WithdrawalStatus) = r.status
		case 5, 6, 7, 8, 9, 10, 11, 12: // string fields — zero value fine
			*d.(*string) = ""
		case 13, 14: // int64 unix timestamps
			*d.(*int64) = 0
		case 15: // retry_count (int)
			*d.(*int) = 0
		case 16, 17: // created_at / updated_at
			*d.(*time.Time) = now
		}
	}
	return nil
}

// fspyUUIDRow implements pgx.Row returning a single UUID.
type fspyUUIDRow struct{ id uuid.UUID }

func (r *fspyUUIDRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if p, ok := dest[0].(*uuid.UUID); ok {
			*p = r.id
		}
	}
	return nil
}

// fspyNoRowsRow implements pgx.Row returning pgx.ErrNoRows.
type fspyNoRowsRow struct{}

func (r *fspyNoRowsRow) Scan(_ ...any) error { return pgx.ErrNoRows }

// fspyErrRow implements pgx.Row always returning an error.
type fspyErrRow struct{ err error }

func (r *fspyErrRow) Scan(_ ...any) error { return r.err }

// fspyAccountRows implements pgx.Rows yielding one row: (uuid, int64=0).
// Satisfies CreateTransaction's FOR UPDATE account lock query.
type fspyAccountRows struct {
	id   uuid.UUID
	done bool
}

func (r *fspyAccountRows) Close()                                       {}
func (r *fspyAccountRows) Err() error                                   { return nil }
func (r *fspyAccountRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 1") }
func (r *fspyAccountRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fspyAccountRows) RawValues() [][]byte                          { return nil }
func (r *fspyAccountRows) Values() ([]any, error)                       { return nil, nil }
func (r *fspyAccountRows) Conn() *pgx.Conn                              { return nil }
func (r *fspyAccountRows) Next() bool {
	if r.done {
		return false
	}
	r.done = true
	return true
}
func (r *fspyAccountRows) Scan(dest ...any) error {
	if len(dest) >= 2 {
		*dest[0].(*uuid.UUID) = r.id
		*dest[1].(*int64) = 0 // balance = 0
	}
	return nil
}

// fspyEmptyRows implements pgx.Rows returning no rows.
type fspyEmptyRows struct{}

func (r *fspyEmptyRows) Close()                                       {}
func (r *fspyEmptyRows) Err() error                                   { return nil }
func (r *fspyEmptyRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 0") }
func (r *fspyEmptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fspyEmptyRows) RawValues() [][]byte                          { return nil }
func (r *fspyEmptyRows) Values() ([]any, error)                       { return nil, nil }
func (r *fspyEmptyRows) Conn() *pgx.Conn                              { return nil }
func (r *fspyEmptyRows) Next() bool                                   { return false }
func (r *fspyEmptyRows) Scan(_ ...any) error                          { return nil }

// ============================================================================
// TRANSACTOR
// ============================================================================

// fspyTransactor implements worker.Transactor and passes the spy db.Tx to fn.
type fspyTransactor struct{ spy *failureSpyTx }

func (t *fspyTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(t.spy)
}

// ============================================================================
// BUILDER HELPERS
// ============================================================================

func newHandlerForSpy() *WebhookHandler {
	return NewWebhookHandler(
		repository.NewWithdrawRepository(),
		repository.NewLedgerRepository(),
		nil, // outboxRepo not needed for failure-key spy tests
		zap.NewNop(),
	)
}

func newWorkerForSpy(spy *failureSpyTx) *PayoutWorker {
	return &PayoutWorker{
		db:           &fspyTransactor{spy: spy},
		withdrawRepo: repository.NewWithdrawRepository(),
		ledgerRepo:   repository.NewLedgerRepository(),
		log:          zap.NewNop(),
		metrics:      NewPayoutMetrics(zap.NewNop()),
	}
}

// ============================================================================
// PROOF TESTS
// ============================================================================

// TestHandleFailedCallback_CapturesIdempotencyKey proves that the live
// WebhookHandler.handleFailedCallback code path sends
// "withdrawal_gateway_restore_<withdrawal_id>" to LedgerRepository.CreateTransaction.
func TestHandleFailedCallback_CapturesIdempotencyKey(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()
	fixedAcct := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-000000000001")

	spy := &failureSpyTx{
		withdrawalID:     withdrawalID,
		sellerID:         sellerID,
		amount:           100_000,
		withdrawalStatus: repository.WithdrawalStatusSubmitted,
		fixedAcct:        fixedAcct,
	}

	h := newHandlerForSpy()

	// Status must be SUBMITTED or SETTLING for handleFailedCallback's state guard.
	withdrawal := &repository.Withdrawal{
		ID:       withdrawalID,
		SellerID: sellerID,
		Amount:   100_000,
		Status:   repository.WithdrawalStatusSubmitted,
	}

	err := h.handleFailedCallback(context.Background(), spy, withdrawal, WebhookCallback{
		ExternalReferenceID: "WD_spy_webhook",
		GatewayReferenceID:  "GW_spy_webhook",
		Status:              WebhookStatusFailed,
		Message:             "Bank rejected",
		RawPayload:          `{"status":"FAILED"}`,
	})
	require.NoError(t, err, "handleFailedCallback must succeed with spy tx")

	// PROOF: key captured from live code, not computed manually.
	wantKey := fmt.Sprintf(gatewayRestoreKeyFmt, withdrawalID.String())
	assert.Equal(t, wantKey, spy.CapturedIdemKey,
		"handleFailedCallback must pass key %q to CreateTransaction; got %q",
		wantKey, spy.CapturedIdemKey)

	assert.NotEmpty(t, spy.CapturedIdemKey,
		"CapturedIdemKey empty — spy QueryRow dispatch is misconfigured")
}

// TestMarkSubmissionFailed_Permanent_CapturesIdempotencyKey proves that the
// live PayoutWorker.markSubmissionFailed(ErrorTypePermanent) code path sends
// "withdrawal_gateway_restore_<withdrawal_id>" to LedgerRepository.CreateTransaction.
func TestMarkSubmissionFailed_Permanent_CapturesIdempotencyKey(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()
	fixedAcct := uuid.MustParse("bbbbbbbb-cccc-dddd-eeee-000000000002")

	spy := &failureSpyTx{
		withdrawalID:     withdrawalID,
		sellerID:         sellerID,
		amount:           250_000,
		withdrawalStatus: repository.WithdrawalStatusSubmitted,
		fixedAcct:        fixedAcct,
	}

	w := newWorkerForSpy(spy)

	err := w.markSubmissionFailed(context.Background(), withdrawalID, ErrorTypePermanent, "permanent gateway rejection")
	require.NoError(t, err, "markSubmissionFailed(Permanent) must succeed with spy tx")

	wantKey := fmt.Sprintf(gatewayRestoreKeyFmt, withdrawalID.String())
	assert.Equal(t, wantKey, spy.CapturedIdemKey,
		"markSubmissionFailed(Permanent) must pass key %q to CreateTransaction; got %q",
		wantKey, spy.CapturedIdemKey)

	assert.NotEmpty(t, spy.CapturedIdemKey,
		"CapturedIdemKey empty — spy QueryRow dispatch is misconfigured")
}

// TestMarkSubmissionFailed_Retryable_DoesNotWriteLedger proves that
// ErrorTypeRetryable does NOT invoke CreateTransaction. Funds stay committed
// for the retry attempt; no restoration occurs.
func TestMarkSubmissionFailed_Retryable_DoesNotWriteLedger(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()
	fixedAcct := uuid.MustParse("cccccccc-dddd-eeee-ffff-000000000003")

	spy := &failureSpyTx{
		withdrawalID:     withdrawalID,
		sellerID:         sellerID,
		amount:           50_000,
		withdrawalStatus: repository.WithdrawalStatusSubmitted,
		fixedAcct:        fixedAcct,
	}

	w := newWorkerForSpy(spy)

	err := w.markSubmissionFailed(context.Background(), withdrawalID, ErrorTypeRetryable, "timeout")
	require.NoError(t, err)

	// PROOF: no idempotency key captured → CreateTransaction was never called.
	assert.Empty(t, spy.CapturedIdemKey,
		"markSubmissionFailed(Retryable) must NOT call CreateTransaction (no fund restoration)")
}

// TestBothPaths_ProduceSameKeyForSameWithdrawal is the shared-key proof:
// both live code paths produce the identical idempotency key for the same
// withdrawal ID. The second path's ledger entry is therefore a no-op via the
// unique DB constraint — no double-restoration is possible.
func TestBothPaths_ProduceSameKeyForSameWithdrawal(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()
	fixedAcct := uuid.MustParse("dddddddd-eeee-ffff-aaaa-000000000004")

	buildSpy := func() *failureSpyTx {
		return &failureSpyTx{
			withdrawalID:     withdrawalID,
			sellerID:         sellerID,
			amount:           75_000,
			withdrawalStatus: repository.WithdrawalStatusSubmitted,
			fixedAcct:        fixedAcct,
		}
	}

	// Webhook path
	webhookSpy := buildSpy()
	h := newHandlerForSpy()
	err := h.handleFailedCallback(context.Background(), webhookSpy, &repository.Withdrawal{
		ID:       withdrawalID,
		SellerID: sellerID,
		Amount:   75_000,
		Status:   repository.WithdrawalStatusSubmitted,
	}, WebhookCallback{Status: WebhookStatusFailed, Message: "rejection"})
	require.NoError(t, err, "webhook path must succeed")

	// Worker path
	workerSpy := buildSpy()
	w := newWorkerForSpy(workerSpy)
	err = w.markSubmissionFailed(context.Background(), withdrawalID, ErrorTypePermanent, "sync rejection")
	require.NoError(t, err, "worker path must succeed")

	// PROOF: both live paths produced the same key from the same withdrawal ID.
	require.NotEmpty(t, webhookSpy.CapturedIdemKey, "webhook key must be captured")
	require.NotEmpty(t, workerSpy.CapturedIdemKey, "worker key must be captured")

	assert.Equal(t, webhookSpy.CapturedIdemKey, workerSpy.CapturedIdemKey,
		"webhook and worker paths must produce identical idempotency keys for withdrawal %s",
		withdrawalID)

	// Belt-and-suspenders: both match the expected format.
	wantKey := fmt.Sprintf(gatewayRestoreKeyFmt, withdrawalID.String())
	assert.Equal(t, wantKey, webhookSpy.CapturedIdemKey, "webhook key matches expected format")
	assert.Equal(t, wantKey, workerSpy.CapturedIdemKey, "worker key matches expected format")
}


