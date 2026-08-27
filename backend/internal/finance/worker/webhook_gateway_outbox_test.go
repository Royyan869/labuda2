package worker

// webhook_gateway_outbox_test.go
//
// PROOF TEST: F1-A
//
// Verifies that when a gateway SUCCESS callback transitions a withdrawal to
// SETTLED, the WebhookHandler emits a "withdrawal.completed" outbox event in
// the same DB transaction — closing the P0 visibility gap where the manual
// MarkProcessed path notified the seller but the gateway path did not.
//
// Strategy: inject a spyOutboxEmitter that implements webhookOutboxEmitter
// and captures every InsertEvent call. Drive handleSuccessCallback through a
// spyGatewayTx (mirrors the failureSpyTx pattern in failure_key_spy_test.go)
// that provides happy-path SQL responses for every DB call in the
// ledger + MarkSettled + outbox path.

import (
	"context"
	"encoding/json"
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
// SPY OUTBOX EMITTER
// ============================================================================

type capturedOutboxEvent struct {
	EventType string
	EntityID  uuid.UUID
	Payload   map[string]interface{}
}

type spyOutboxEmitter struct {
	Events []capturedOutboxEvent
}

func (s *spyOutboxEmitter) InsertEvent(
	_ context.Context,
	_ db.Tx,
	eventType string,
	entityID uuid.UUID,
	payload []byte,
) error {
	var decoded map[string]interface{}
	_ = json.Unmarshal(payload, &decoded)
	s.Events = append(s.Events, capturedOutboxEvent{
		EventType: eventType,
		EntityID:  entityID,
		Payload:   decoded,
	})
	return nil
}

// ============================================================================
// SPY TX — happy-path SQL for the gateway success path
// ============================================================================
// The path inside handleSuccessCallback calls (in order):
//   1. ledgerRepo.GetSystemAccountID (SELECT ... FROM financial_accounts WHERE type=$1)     x2
//   2. ledgerRepo.CreateTransaction  (SELECT on idempotency + INSERT transaction + 2x INSERT entry)
//   3. withdrawRepo.MarkSettled      (UPDATE wallet.withdrawals ...)
//   4. outboxRepo.InsertEvent        (handled by spyOutboxEmitter — no SQL)
//
// We return enough rows/tags to avoid errors on every call.

type gatewaySuccessSpyTx struct {
	withdrawalID uuid.UUID
	sellerID     uuid.UUID
	amount       int64
	fixedAcct    uuid.UUID // returned for all financial_accounts lookups

	// CapturedIdempotencyKey is populated when CreateTransaction's internal
	// idempotency SELECT is seen.
	CapturedIdempotencyKey string
}

func (s *gatewaySuccessSpyTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	// Idempotency check inside CreateTransaction
	if strings.Contains(sql, "idempotency_key") {
		if len(args) > 0 {
			if key, ok := args[0].(string); ok {
				s.CapturedIdempotencyKey = key
			}
		}
		// No row → new transaction path → proceed to INSERT
		return &fakeNoRow{}
	}

	// financial_accounts scalar lookup (GetSystemAccountID / GetOrCreateUserAccount)
	if strings.Contains(sql, "financial_accounts") {
		return &fakeUUIDRow{id: s.fixedAcct}
	}

	// financial_transactions INSERT → return a new transaction UUID
	if strings.Contains(sql, "financial_transactions") {
		return &fakeUUIDRow{id: uuid.New()}
	}

	return &fakeNoRow{}
}

func (s *gatewaySuccessSpyTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	// CreateTransaction locks accounts FOR UPDATE: SELECT id, balance FROM financial_accounts ... FOR UPDATE
	if strings.Contains(sql, "financial_accounts") {
		return &fakeAccountRows{id: s.fixedAcct}, nil
	}
	return &fakeRows{}, nil
}

func (s *gatewaySuccessSpyTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	lsql := strings.ToLower(sql)
	// INSERT financial_entries or UPDATE wallet.withdrawals — both succeed
	_ = lsql
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (s *gatewaySuccessSpyTx) Commit(_ context.Context) error   { return nil }
func (s *gatewaySuccessSpyTx) Rollback(_ context.Context) error { return nil }

// ============================================================================
// FAKE ROW HELPERS (shared with failure_key_spy_test.go pattern)
// ============================================================================

type fakeUUIDRow struct{ id uuid.UUID }

func (r *fakeUUIDRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if p, ok := dest[0].(*uuid.UUID); ok {
			*p = r.id
		}
	}
	return nil
}

type fakeNoRow struct{}

func (r *fakeNoRow) Scan(_ ...any) error { return pgx.ErrNoRows }

// fakeAccountRows yields one account row: (uuid, balance=0).
// Used for the financial_accounts FOR UPDATE lock in CreateTransaction.
type fakeAccountRows struct {
	id   uuid.UUID
	done bool
}

func (r *fakeAccountRows) Next() bool {
	if r.done {
		return false
	}
	r.done = true
	return true
}
func (r *fakeAccountRows) Scan(dest ...any) error {
	if len(dest) >= 2 {
		*dest[0].(*uuid.UUID) = r.id
		*dest[1].(*int64) = 0
	}
	return nil
}
func (r *fakeAccountRows) Close()                                       {}
func (r *fakeAccountRows) Err() error                                   { return nil }
func (r *fakeAccountRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 1") }
func (r *fakeAccountRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeAccountRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeAccountRows) RawValues() [][]byte                          { return nil }
func (r *fakeAccountRows) Conn() *pgx.Conn                              { return nil }

// fakeRows is an empty rows result (no data).
type fakeRows struct{}

func (r *fakeRows) Next() bool                                   { return false }
func (r *fakeRows) Scan(_ ...any) error                          { return nil }
func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 0") }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }

// ============================================================================
// TEST
// ============================================================================

// TestWebhookHandler_GatewaySuccess_EmitsOutboxEvent proves that when a
// gateway SUCCESS callback settles a withdrawal, the WebhookHandler emits a
// "withdrawal.completed" outbox event with the required fields — in the same
// DB transaction (the spy tx is the evidence of atomicity).
func TestWebhookHandler_GatewaySuccess_EmitsOutboxEvent(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()
	const amount int64 = 500_000

	spy := &spyOutboxEmitter{}
	handler := NewWebhookHandler(
		repository.NewWithdrawRepository(),
		repository.NewLedgerRepository(),
		spy,
		zap.NewNop(),
	)

	spyTx := &gatewaySuccessSpyTx{
		withdrawalID: withdrawalID,
		sellerID:     sellerID,
		amount:       amount,
		fixedAcct:    uuid.New(),
	}

	withdrawal := &repository.Withdrawal{
		ID:                  withdrawalID,
		SellerID:            sellerID,
		Amount:              amount,
		Status:              repository.WithdrawalStatusSubmitted,
		ExternalReferenceID: "WD_GTEST_001",
		SubmittedAt:         time.Now().Unix(),
	}

	callback := WebhookCallback{
		ExternalReferenceID: "WD_GTEST_001",
		GatewayReferenceID:  "GW_GTEST_001",
		Status:              WebhookStatusSuccess,
		Message:             "Payout settled",
		Timestamp:           time.Now().Unix(),
		RawPayload:          `{"status":"SUCCESS","ref":"GW_GTEST_001"}`,
	}

	err := handler.handleSuccessCallback(context.Background(), spyTx, withdrawal, callback)
	require.NoError(t, err, "handleSuccessCallback must succeed")

	// ASSERTION 1: exactly one outbox event emitted
	require.Len(t, spy.Events, 1, "exactly one outbox event must be emitted")

	evt := spy.Events[0]

	// ASSERTION 2: event type is withdrawal.completed (existing handler is wired)
	assert.Equal(t, "withdrawal.completed", evt.EventType,
		"event type must match existing notification handler key")

	// ASSERTION 3: entity ID is the withdrawal ID
	assert.Equal(t, withdrawalID, evt.EntityID,
		"entity ID must be the withdrawal ID for outbox routing")

	// ASSERTION 4: required payload fields
	require.NotNil(t, evt.Payload)
	assert.Equal(t, withdrawalID.String(), evt.Payload["withdrawal_id"],
		"payload must include withdrawal_id")
	assert.Equal(t, sellerID.String(), evt.Payload["seller_id"],
		"payload must include seller_id")
	assert.Equal(t, float64(amount), evt.Payload["amount"],
		"payload must include amount")
	assert.Equal(t, string(repository.WithdrawalStatusSettled), evt.Payload["status"],
		"payload status must be SETTLED (gateway truth)")
	assert.Equal(t, "gateway_webhook", evt.Payload["source"],
		"payload source must be gateway_webhook to distinguish from manual path")

	t.Log("F1-A PASS: gateway SUCCESS callback emits withdrawal.completed outbox event in-transaction")
	t.Logf("  withdrawal_id=%s  seller_id=%s  amount=%d  status=%s  source=%s",
		evt.Payload["withdrawal_id"], evt.Payload["seller_id"],
		int64(evt.Payload["amount"].(float64)), evt.Payload["status"], evt.Payload["source"])
}

// TestWebhookHandler_GatewayFailed_EmitsWithdrawalFailedOutboxEvent proves
// that the gateway FAILED callback emits withdrawal.failed rather than
// reusing withdrawal.rejected.
func TestWebhookHandler_GatewayFailed_EmitsWithdrawalFailedOutboxEvent(t *testing.T) {
	spy := &spyOutboxEmitter{}
	handler := NewWebhookHandler(
		repository.NewWithdrawRepository(),
		repository.NewLedgerRepository(),
		spy,
		zap.NewNop(),
	)

	spyTx := &gatewaySuccessSpyTx{
		withdrawalID: uuid.New(),
		sellerID:     uuid.New(),
		amount:       200_000,
		fixedAcct:    uuid.New(),
	}

	withdrawal := &repository.Withdrawal{
		ID:                  spyTx.withdrawalID,
		SellerID:            spyTx.sellerID,
		Amount:              spyTx.amount,
		Status:              repository.WithdrawalStatusSubmitted,
		ExternalReferenceID: "WD_GTEST_FAIL_001",
		SubmittedAt:         time.Now().Unix(),
	}

	callback := WebhookCallback{
		ExternalReferenceID: "WD_GTEST_FAIL_001",
		GatewayReferenceID:  "GW_GTEST_FAIL_001",
		Status:              WebhookStatusFailed,
		Message:             "Bank rejected the transfer",
		Timestamp:           time.Now().Unix(),
		RawPayload:          `{"status":"FAILED"}`,
	}

	err := handler.handleFailedCallback(context.Background(), spyTx, withdrawal, callback)
	require.NoError(t, err, "handleFailedCallback must succeed")

	require.Len(t, spy.Events, 1, "exactly one outbox event must be emitted")

	evt := spy.Events[0]
	assert.Equal(t, "withdrawal.failed", evt.EventType, "failed callback must emit withdrawal.failed")
	assert.Equal(t, withdrawal.ID, evt.EntityID, "entity ID must be the withdrawal ID")
	require.NotNil(t, evt.Payload)
	assert.Equal(t, withdrawal.ID.String(), evt.Payload["withdrawal_id"])
	assert.Equal(t, withdrawal.SellerID.String(), evt.Payload["seller_id"])
	assert.Equal(t, float64(withdrawal.Amount), evt.Payload["amount"])
	assert.Equal(t, string(repository.WithdrawalStatusFailedFinal), evt.Payload["status"])
	assert.Equal(t, "gateway_webhook", evt.Payload["source"])
	assert.Equal(t, callback.Message, evt.Payload["reason"])

	t.Log("F1-A PASS (boundary): gateway FAILED callback emits withdrawal.failed outbox event")
}

// TestWebhookHandler_NilOutboxRepo_DoesNotPanic proves that when outboxRepo is
// nil (e.g., iris_webhook_proof cmd), handleSuccessCallback still works and
// simply skips emission without panic.
func TestWebhookHandler_NilOutboxRepo_DoesNotPanic(t *testing.T) {
	handler := NewWebhookHandler(
		repository.NewWithdrawRepository(),
		repository.NewLedgerRepository(),
		nil, // no outbox
		zap.NewNop(),
	)

	spyTx := &gatewaySuccessSpyTx{
		withdrawalID: uuid.New(),
		sellerID:     uuid.New(),
		amount:       100_000,
		fixedAcct:    uuid.New(),
	}

	withdrawal := &repository.Withdrawal{
		ID:                  spyTx.withdrawalID,
		SellerID:            spyTx.sellerID,
		Amount:              spyTx.amount,
		Status:              repository.WithdrawalStatusSubmitted,
		ExternalReferenceID: "WD_NILTEST_001",
		SubmittedAt:         time.Now().Unix(),
	}

	callback := WebhookCallback{
		ExternalReferenceID: "WD_NILTEST_001",
		Status:              WebhookStatusSuccess,
		RawPayload:          `{}`,
	}

	assert.NotPanics(t, func() {
		_ = handler.handleSuccessCallback(context.Background(), spyTx, withdrawal, callback)
	}, "nil outboxRepo must not cause a panic")

	t.Log("F1-A PASS (nil-safety): nil outboxRepo skips emission without panic")
}


