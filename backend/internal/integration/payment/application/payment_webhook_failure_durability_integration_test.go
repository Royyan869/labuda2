//go:build integration

// REC-1: PAYMENT WEBHOOK FAILURE DURABILITY — real-Postgres proof.
//
// The webhook processing path runs inside db.WithTx, which ALWAYS rolls back
// when the transaction function returns an error (pkg/db.withRetry). A failure
// record written from inside that transaction is therefore erased — including
// the payment_webhook_events row and its status='failed' update.
//
// These tests prove, against a real Postgres instance running the full
// migration chain:
//
//	rollback DOES erase an in-transaction failure record;
//	an INDEPENDENT transaction survives that rollback and leaves the record;
//	a committed terminal record is never overwritten by a later failure;
//	repeated recording is idempotent, and a recorded failure is reprocessable;
//	the success path is unchanged and durable;
//	a bad signature persists nothing at all;
//	gateway credentials never reach the stored record.
//
// TRANSACTION-CALLBACK RULE: no require.*/assert.*/t.Fatal may run inside a
// WithTx callback. Those abort the calling goroutine via runtime.Goexit, which
// leaves the transaction (and its pooled connection) leaked forever — the exact
// hazard documented on pkg/testdb.TestDB.WithTx. Callbacks therefore only ever
// return errors.
//
// The HTTP boundary (handler ack semantics unchanged while the durable failure
// record is written from a separate transaction) is proven by
// payment_webhook_failure_durability_handler_integration_test.go, which lives in
// the external application_test package because the in-package test variant may
// not import delivery/http (it imports this package).
//
// Run with: go test -tags integration ./internal/integration/payment/application/...
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/testdb"
)

// testServerKey is a distinctive, non-secret value used to prove that gateway
// credentials never reach the durable failure record.
const testServerKey = "SB-Mid-server-REC1-DO-NOT-PERSIST"

// newDurabilityService builds the smallest honest webhook service through the
// CANONICAL constructor: a real *db.DB (real transactions on a pooled
// connection), the real payment repository, and a real Midtrans client whose
// signature key is a distinctive value. OrderService/EscrowService are nil
// because the durability path never reaches finalization, and building local
// copies would violate the WIRING RULE documented on PaymentWebhookService.
func newDurabilityService(t *testing.T, testDB *testdb.TestDB) (*PaymentWebhookService, *midtrans.Client) {
	t.Helper()

	log, err := logger.NewDevelopment()
	require.NoError(t, err, "logger init")

	client := midtrans.NewClient(&config.MidtransConfig{
		Environment: "sandbox",
		ServerKey:   testServerKey,
		ClientKey:   "SB-Mid-client-REC1",
	}, log)

	svc := NewPaymentWebhookService(db.NewFromPool(testDB.Pool()), client, nil, nil, zap.NewNop())
	require.NotNil(t, svc.midtransClient, "the canonical constructor must inject the gateway client")

	return svc, client
}

// failureNotification builds a settlement notification. The caller's "eventID"
// value is the Midtrans gateway TRANSACTION reference (stored in event_id).
func failureNotification(transactionID, orderID string) *midtrans.NotificationPayload {
	return &midtrans.NotificationPayload{
		TransactionID:     transactionID,
		OrderID:           orderID,
		TransactionStatus: string(midtrans.StatusSettlement),
		StatusCode:        "200",
		GrossAmount:       "10000.00",
		PaymentType:       "bank_transfer",
		Currency:          "IDR",
	}
}

// insertEventTx mirrors handleWebhookInTransaction STEP 1 without asserting
// inside the transaction callback.
//
// IDENTITY (REC-3): the notification is the identity source. event_id still
// holds the gateway transaction reference, but it is deliberately NOT unique —
// the row is keyed by midtrans.NotificationIdentity(notification).
func insertEventTx(ctx context.Context, svc *PaymentWebhookService, tx db.Tx, notification *midtrans.NotificationPayload, payload string) error {
	inserted, err := svc.insertWebhookEvent(ctx, tx, notification.TransactionID, midtrans.NotificationIdentity(notification), notification.OrderID, notification.SignatureKey, []byte(payload))
	if err != nil {
		return fmt.Errorf("insert webhook event: %w", err)
	}
	if !inserted {
		return fmt.Errorf("expected a fresh insert for notification %s", midtrans.NotificationIdentity(notification))
	}
	return nil
}

func setEventStatusTx(ctx context.Context, svc *PaymentWebhookService, tx db.Tx, notification *midtrans.NotificationPayload, status string, errorMessage *string) error {
	return svc.updateWebhookEventStatus(ctx, tx, midtrans.NotificationIdentity(notification), status, nil, errorMessage)
}

// webhookEventRow reads ONE notification's stored row by its canonical identity.
// Reading by event_id is no longer valid: one gateway transaction legitimately
// owns several notifications (REC-3).
func webhookEventRow(t *testing.T, ctx context.Context, testDB *testdb.TestDB, notification *midtrans.NotificationPayload) (found bool, status string, errorMessage string) {
	t.Helper()

	require.NoError(t, testDB.WithTx(ctx, func(tx db.Tx) error {
		var errMsg *string
		err := tx.QueryRow(ctx,
			`SELECT status, error_message FROM payment_webhook_events WHERE notification_key = $1`,
			midtrans.NotificationIdentity(notification),
		).Scan(&status, &errMsg)
		if err != nil {
			if err.Error() == "no rows in result set" {
				return nil
			}
			return err
		}
		found = true
		if errMsg != nil {
			errorMessage = *errMsg
		}
		return nil
	}))

	return found, status, errorMessage
}

func countWebhookEvents(t *testing.T, ctx context.Context, testDB *testdb.TestDB, notification *midtrans.NotificationPayload) int {
	t.Helper()

	var count int
	require.NoError(t, testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT count(*) FROM payment_webhook_events WHERE notification_key = $1`, midtrans.NotificationIdentity(notification)).Scan(&count)
	}))
	return count
}

// TestWebhookFailureDurability_RollbackErasesInTransactionRecord proves the
// exact failure pattern REC-1 exists to fix: the processing transaction inserts
// the event and writes status='failed', then rolls back because processing
// failed — and NOTHING survives, not even the failure detail.
func TestWebhookFailureDurability_RollbackErasesInTransactionRecord(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, _ := newDurabilityService(t, testDB)

	const eventID = "rec1-rollback-erases"
	notification := failureNotification(eventID, "LAB-REC1-ORDER-1")

	// Mirror handleWebhookInTransaction's failing path exactly: insert the event,
	// record the failure on the row, then return an error.
	processingErr := svc.db.WithTx(ctx, func(tx db.Tx) error {
		if err := insertEventTx(ctx, svc, tx, notification, `{"test":"rollback"}`); err != nil {
			return err
		}
		msg := "amount mismatch: expected 1000, got 999999"
		if err := setEventStatusTx(ctx, svc, tx, notification, "failed", &msg); err != nil {
			return err
		}
		return errors.New("boom: processing failed")
	})
	require.Error(t, processingErr, "the failing path must surface an error")

	// The row AND its failure detail are gone: this is the gap.
	assert.Equal(t, 0, countWebhookEvents(t, ctx, testDB, notification),
		"a rolled-back transaction must leave no webhook event row — proving the in-transaction failure record is not durable")

	found, _, _ := webhookEventRow(t, ctx, testDB, notification)
	assert.False(t, found, "no failure detail may survive the rollback")
}

// TestWebhookFailureDurability_IndependentTransactionSurvivesRollback proves the
// fix: recording the failure in its OWN transaction after the rollback leaves a
// durable, queryable record with enough context to diagnose.
func TestWebhookFailureDurability_IndependentTransactionSurvivesRollback(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, _ := newDurabilityService(t, testDB)

	const eventID = "rec1-durable-failure"
	notification := failureNotification(eventID, "LAB-REC1-ORDER-2")

	// 1. The processing transaction fails and rolls back (record erased).
	require.Error(t, svc.db.WithTx(ctx, func(tx db.Tx) error {
		if err := insertEventTx(ctx, svc, tx, notification, `{"test":"processing"}`); err != nil {
			return err
		}
		return errors.New("CRITICAL: failed to finalize order payment: escrow creation failed")
	}))
	require.Equal(t, 0, countWebhookEvents(t, ctx, testDB, notification),
		"precondition: the rollback erased the in-transaction record")

	// 2. The independent durability write runs after the rollback.
	svc.recordWebhookFailureDurably(ctx, notification, "203.0.113.9",
		errors.New("CRITICAL: failed to finalize order payment: escrow creation failed"))

	// 3. The failure is durable and diagnosable.
	found, status, errorMessage := webhookEventRow(t, ctx, testDB, notification)
	require.True(t, found, "the processing failure MUST leave a durable record")
	assert.Equal(t, repository.PaymentWebhookEventStatusFailed, status,
		"a processing failure must be recorded as 'failed', never as 'succeeded'")
	assert.Contains(t, errorMessage, "failed to finalize order payment",
		"the record must carry enough context to diagnose the failure")

	var processedAt *time.Time
	var orderID string
	var provider string
	require.NoError(t, testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT processed_at, midtrans_order_id, provider FROM payment_webhook_events WHERE notification_key = $1`,
			midtrans.NotificationIdentity(notification)).Scan(&processedAt, &orderID, &provider)
	}))
	require.NotNil(t, processedAt, "a terminal failure record must be stamped as processed")
	assert.Equal(t, notification.OrderID, orderID)
	assert.Equal(t, "midtrans", provider)
}

// TestWebhookFailureDurability_NeverOverwritesCommittedTerminalRecord proves the
// concurrency guard: once an event is durably recorded in a terminal
// non-failure state, a later failing delivery of the SAME notification cannot
// rewrite it. A concurrent success always wins the audit record.
func TestWebhookFailureDurability_NeverOverwritesCommittedTerminalRecord(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, _ := newDurabilityService(t, testDB)

	const eventID = "rec1-terminal-wins"
	notification := failureNotification(eventID, "LAB-REC1-ORDER-3")

	// Commit a successful event.
	require.NoError(t, svc.db.WithTx(ctx, func(tx db.Tx) error {
		if err := insertEventTx(ctx, svc, tx, notification, `{"test":"success"}`); err != nil {
			return err
		}
		return setEventStatusTx(ctx, svc, tx, notification, "succeeded", nil)
	}))

	// A failing delivery of the same event tries to record a failure.
	svc.recordWebhookFailureDurably(ctx, notification, "203.0.113.9", errors.New("late duplicate failure"))

	found, status, errorMessage := webhookEventRow(t, ctx, testDB, notification)
	require.True(t, found)
	assert.Equal(t, repository.PaymentWebhookEventStatusSucceeded, status,
		"a committed terminal record must never be overwritten by a later failure")
	assert.Empty(t, errorMessage, "the durable success record must stay clean")
	assert.Equal(t, 1, countWebhookEvents(t, ctx, testDB, notification), "no duplicate row may be created")
}

// TestWebhookFailureDurability_IdempotentAndReprocessable proves repeated
// recording stays a single row, and that a recorded failure is classified as
// reprocessable — the property that keeps redelivery semantics identical to
// before failures were recorded.
func TestWebhookFailureDurability_IdempotentAndReprocessable(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, _ := newDurabilityService(t, testDB)

	const eventID = "rec1-idempotent"
	notification := failureNotification(eventID, "LAB-REC1-ORDER-4")

	svc.recordWebhookFailureDurably(ctx, notification, "203.0.113.9", errors.New("first failure: amount mismatch"))
	svc.recordWebhookFailureDurably(ctx, notification, "203.0.113.9", errors.New("second failure: refund ack dispatch failed"))

	assert.Equal(t, 1, countWebhookEvents(t, ctx, testDB, notification),
		"recording the same event twice must not create duplicate rows")

	_, status, errorMessage := webhookEventRow(t, ctx, testDB, notification)
	assert.Equal(t, repository.PaymentWebhookEventStatusFailed, status)
	assert.Contains(t, errorMessage, "second failure", "the latest failure detail must win")

	// The status the durable record leaves behind is exactly the one that must be
	// reprocessed on redelivery.
	var persisted string
	require.NoError(t, svc.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		persisted, err = svc.paymentRepo.GetWebhookEventStatus(ctx, tx, midtrans.NotificationIdentity(notification))
		return err
	}))
	assert.Equal(t, repository.PaymentWebhookEventStatusFailed, persisted)
	assert.True(t, webhookEventIsReprocessable(persisted),
		"a recorded failure must be reprocessable, otherwise durable recording would silently convert recoverable failures into permanent tombstones")

	// And every other recorded status must keep short-circuiting as before.
	for _, terminal := range []string{
		repository.PaymentWebhookEventStatusSucceeded,
		// Legacy status value with no typed constant left (orphan recovery
		// removed): a historical 'orphaned' row must keep short-circuiting.
		"orphaned",
		repository.PaymentWebhookEventStatusManualReview,
		repository.PaymentWebhookEventStatusQuarantined,
		repository.PaymentWebhookEventStatusTerminalReview,
		repository.PaymentWebhookEventStatusCapturedAfterExpiry,
		repository.PaymentWebhookEventStatusProcessing,
		"",
	} {
		assert.False(t, webhookEventIsReprocessable(terminal),
			"status %q must remain an idempotent duplicate, not a reprocessable event", terminal)
	}
}

// TestWebhookFailureDurability_SuccessPathUnchanged proves the success path is
// still durable and clean: identical settlement semantics, no error detail.
func TestWebhookFailureDurability_SuccessPathUnchanged(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, _ := newDurabilityService(t, testDB)

	const eventID = "rec1-success"
	notification := failureNotification(eventID, "LAB-REC1-ORDER-5")

	require.NoError(t, svc.db.WithTx(ctx, func(tx db.Tx) error {
		if err := insertEventTx(ctx, svc, tx, notification, `{"test":"success"}`); err != nil {
			return err
		}
		if err := setEventStatusTx(ctx, svc, tx, notification, "processing", nil); err != nil {
			return err
		}
		return setEventStatusTx(ctx, svc, tx, notification, "succeeded", nil)
	}))

	found, status, errorMessage := webhookEventRow(t, ctx, testDB, notification)
	require.True(t, found, "a successful webhook must remain durably recorded")
	assert.Equal(t, repository.PaymentWebhookEventStatusSucceeded, status)
	assert.Empty(t, errorMessage)

	// Re-delivering an already-succeeded event is NOT reprocessable.
	assert.False(t, webhookEventIsReprocessable(status),
		"an already-succeeded event must stay an idempotent duplicate")
}

// TestWebhookFailureDurability_SignatureRejectionPersistsNothing proves an
// invalid signature neither records a failure nor creates a misleading
// successful event end-to-end through the real HandleWebhook entry point.
func TestWebhookFailureDurability_SignatureRejectionPersistsNothing(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, client := newDurabilityService(t, testDB)

	const eventID = "rec1-bad-signature"
	notification := failureNotification(eventID, "LAB-REC1-ORDER-6")
	notification.SignatureKey = "deadbeef-not-a-valid-signature"
	require.False(t, client.VerifySignature(notification), "precondition: the signature must not verify")

	err := svc.HandleWebhook(ctx, notification, "203.0.113.9")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrWebhookSignatureInvalid),
		"a signature rejection must be identifiable, so the durable failure record can exclude it; got %v", err)

	assert.Equal(t, 0, countWebhookEvents(t, ctx, testDB, notification),
		"a rejected signature must persist nothing: unauthenticated input must not enter the event store")
}

// TestWebhookFailureDurability_CredentialsNeverPersisted proves the durable
// failure record bounds sensitive data: the gateway server key never reaches the
// stored payload or error message.
func TestWebhookFailureDurability_CredentialsNeverPersisted(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, _ := newDurabilityService(t, testDB)

	const eventID = "rec1-no-secrets"
	notification := failureNotification(eventID, "LAB-REC1-ORDER-7")
	notification.SignatureKey = "signature-hash-value"

	svc.recordWebhookFailureDurably(ctx, notification, "203.0.113.9",
		errors.New("failed to finalize order payment"))

	var payload []byte
	require.NoError(t, testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT payload FROM payment_webhook_events WHERE notification_key = $1`, midtrans.NotificationIdentity(notification)).Scan(&payload)
	}))

	assert.NotContains(t, string(payload), testServerKey,
		"the gateway server key must never be persisted in a webhook event payload")

	// The stored payload must still be the notification itself (recovery needs it).
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	assert.Equal(t, notification.TransactionID, decoded["transaction_id"])

	_, _, errorMessage := webhookEventRow(t, ctx, testDB, notification)
	assert.NotContains(t, errorMessage, testServerKey,
		"the gateway server key must never be persisted in a webhook event error message")
	assert.False(t, strings.Contains(errorMessage, "SB-Mid-server"),
		"no gateway credential material may appear in the failure record")
}

// TestWebhookFailureDurability_AmountMismatchIsDistinguishableFromSuccess proves
// the two outcomes can never be confused when reading the event store.
func TestWebhookFailureDurability_AmountMismatchIsDistinguishableFromSuccess(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, _ := newDurabilityService(t, testDB)

	const failedEventID = "rec1-amount-mismatch"
	const successEventID = "rec1-amount-match"

	// The failing path in handleWebhookInTransaction records this message shape
	// before rolling back.
	mismatch := failureNotification(failedEventID, "LAB-REC1-ORDER-8")
	svc.recordWebhookFailureDurably(ctx, mismatch, "203.0.113.9",
		errors.New("amount validation failed: expected 10000, got 1"))

	successful := failureNotification(successEventID, "LAB-REC1-ORDER-9")
	require.NoError(t, svc.db.WithTx(ctx, func(tx db.Tx) error {
		if err := insertEventTx(ctx, svc, tx, successful, `{"test":"success"}`); err != nil {
			return err
		}
		return setEventStatusTx(ctx, svc, tx, successful, "succeeded", nil)
	}))

	_, failedStatus, failedMsg := webhookEventRow(t, ctx, testDB, mismatch)
	_, successStatus, successMsg := webhookEventRow(t, ctx, testDB, successful)

	assert.Equal(t, repository.PaymentWebhookEventStatusFailed, failedStatus)
	assert.Equal(t, repository.PaymentWebhookEventStatusSucceeded, successStatus)
	assert.NotEqual(t, failedStatus, successStatus,
		"an amount mismatch must never read as a successful processing outcome")
	assert.Contains(t, failedMsg, "amount validation failed")
	assert.NotContains(t, failedMsg, "succeeded")
	assert.Empty(t, successMsg)

	// A reviewer must be able to FIND the failure, not just the success.
	var failedCount int
	require.NoError(t, testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT count(*) FROM payment_webhook_events WHERE status = 'failed' AND error_message ILIKE '%amount%'`).Scan(&failedCount)
	}))
	assert.Positive(t, failedCount, "failed webhook events must be queryable by status")
}

// TestWebhookFailureDurability_NoDuplicateNotificationRow proves the event store
// keeps exactly one row per notification identity, so the durability path cannot
// create a parallel record for the same gateway delivery.
//
// Note what is deliberately NOT asserted: uniqueness on event_id. Under REC-3 one
// gateway transaction owns several notification rows, so uniqueness lives on
// notification_key instead.
func TestWebhookFailureDurability_NoDuplicateNotificationRow(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, _ := newDurabilityService(t, testDB)

	const eventID = "rec1-single-row"
	notification := failureNotification(eventID, "LAB-REC1-ORDER-10")

	svc.recordWebhookFailureDurably(ctx, notification, "203.0.113.9", errors.New("first"))

	// A redelivery attempt whose processing transaction also inserts the event
	// hits the ON CONFLICT no-op, then rolls back on its own failure — leaving the
	// durable row untouched.
	require.Error(t, svc.db.WithTx(ctx, func(tx db.Tx) error {
		inserted, err := svc.insertWebhookEvent(ctx, tx, notification.TransactionID, midtrans.NotificationIdentity(notification), notification.OrderID, notification.SignatureKey, []byte(`{"test":"redelivery"}`))
		if err != nil {
			return err
		}
		if inserted {
			return errors.New("expected ON CONFLICT DO NOTHING for the recorded notification_key")
		}
		return errors.New("redelivery failed too")
	}))
	svc.recordWebhookFailureDurably(ctx, notification, "203.0.113.9", errors.New("second"))

	assert.Equal(t, 1, countWebhookEvents(t, ctx, testDB, notification),
		"exactly one row per notification identity: no parallel failure store, no duplicate event authority")

	var id uuid.UUID
	require.NoError(t, testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT id FROM payment_webhook_events WHERE notification_key = $1`, midtrans.NotificationIdentity(notification)).Scan(&id)
	}))
	assert.NotEqual(t, uuid.Nil, id)
}
