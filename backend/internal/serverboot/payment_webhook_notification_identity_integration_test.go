//go:build integration

// REC-3: WEBHOOK NOTIFICATION IDENTITY — full-wiring acceptance proofs.
//
// THE DEFECT THESE TESTS CLOSE (proven by REC-2 on the disposable database):
// webhook ingestion keyed its event row on the Midtrans gateway TRANSACTION
// (event_id = transaction_id, UNIQUE). Midtrans emits SEVERAL notifications per
// transaction as its status advances, so the FIRST notification occupied the
// only row and every later one — including the settlement that carries the money
// signal — was classified as an "idempotent duplicate" and silently dropped.
//
// THE CANONICAL CONTRACT NOW: notification_key = midtrans.NotificationIdentity
// is the uniqueness authority. An identical redelivery dedups; a different
// status transition or a different refund of the same transaction is PRESERVED
// as its own event.
//
// These tests run the CANONICAL wiring (real finalizer, escrow, finance, coins,
// payment repository) through PaymentWebhookService.HandleWebhook — the same
// entry point the HTTP handler calls.
//
// Run with: go test -tags integration -run TestWebhookNotificationIdentity ./internal/serverboot/...
package serverboot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	paymentrepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/testdb"
)

// countWebhookEventsForTransaction counts every notification row stored for one
// gateway transaction. Before REC-3 this could only ever be 0 or 1; it is now
// the direct measure of "distinct notifications survived".
func countWebhookEventsForTransaction(ctx context.Context, tdb *testdb.TestDB, transactionID string) (int64, error) {
	var count int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM payment_webhook_events WHERE event_id = $1`,
			transactionID).Scan(&count)
	})
	return count, err
}

// loadWebhookNotificationKeys returns the canonical identities stored for one
// gateway transaction.
func loadWebhookNotificationKeys(ctx context.Context, tdb *testdb.TestDB, transactionID string) ([]string, error) {
	var keys []string
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT notification_key FROM payment_webhook_events WHERE event_id = $1 ORDER BY received_at`,
			transactionID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				return err
			}
			keys = append(keys, key)
		}
		return rows.Err()
	})
	return keys, err
}

func loadWebhookEventStatusForKey(ctx context.Context, tdb *testdb.TestDB, notificationKey string) (string, bool, error) {
	var status string
	found := false
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		err := tx.QueryRow(ctx,
			`SELECT status FROM payment_webhook_events WHERE notification_key = $1`,
			notificationKey).Scan(&status)
		if err != nil {
			if err.Error() == "no rows in result set" {
				return nil
			}
			return err
		}
		found = true
		return nil
	})
	return status, found, err
}

// TestWebhookNotificationIdentity_PendingThenSettlementStillSettles is the
// primary REC-3 proof: the ordinary gateway lifecycle (pending → settlement) for
// ONE transaction must settle the payment. Before REC-3 the second notification
// was dropped and the payment stayed pending forever.
func TestWebhookNotificationIdentity_PendingThenSettlementStillSettles(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	pending := h.makeWebhookNotification(fx, string(midtrans.StatusPending))
	settlement := h.makeWebhookNotification(fx, string(midtrans.StatusSettlement))

	// DELIVERY 1 — the gateway created the transaction.
	require.NoError(t, h.handleWebhook(ctx, pending))

	before, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, before.Status, "a pending notification must not settle anything")
	rows, err := countWebhookEventsForTransaction(ctx, h.tdb, fx.TransactionID)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	// DELIVERY 2 — the customer paid; the SAME gateway transaction settles.
	// This delivery used to be discarded as a duplicate.
	require.NoError(t, h.handleWebhook(ctx, settlement),
		"the settlement notification must be processed, not reported as a duplicate")

	after, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, after.Status,
		"the payment must be settled: the gateway settled the transaction and the signal survived")

	escrowAmount := h.loadEscrowAmount(t, fx.OrderID)
	assert.Greater(t, escrowAmount, int64(0), "a settled order payment must create gateway-funded escrow")

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, after.ReferenceID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), spendCount, "coin consumption must happen exactly once")

	// Both notifications are preserved as distinct events.
	rows, err = countWebhookEventsForTransaction(ctx, h.tdb, fx.TransactionID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), rows,
		"the pending and the settlement notification are DISTINCT events and both must be stored")

	keys, err := loadWebhookNotificationKeys(ctx, h.tdb, fx.TransactionID)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	assert.NotEqual(t, keys[0], keys[1], "one gateway transaction must be able to own several distinct notification identities")
	assert.Contains(t, keys, midtrans.NotificationIdentity(pending))
	assert.Contains(t, keys, midtrans.NotificationIdentity(settlement))
}

// TestWebhookNotificationIdentity_ExactRedeliveryStaysIdempotent proves the other
// half of the invariant: preserving distinct signals must not turn an identical
// redelivery into a second event.
func TestWebhookNotificationIdentity_ExactRedeliveryStaysIdempotent(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	pending := h.makeWebhookNotification(fx, string(midtrans.StatusPending))
	settlement := h.makeWebhookNotification(fx, string(midtrans.StatusSettlement))

	require.NoError(t, h.handleWebhook(ctx, pending))
	require.NoError(t, h.handleWebhook(ctx, settlement))
	require.NoError(t, h.handleWebhook(ctx, settlement), "an exact redelivery must stay a clean idempotent no-op")

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), spendCount, "redelivery must never consume coins twice")

	rows, err := countWebhookEventsForTransaction(ctx, h.tdb, fx.TransactionID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), rows,
		"the redelivery must not create a third row: 2 notifications, 2 identities")

	status, found, err := loadWebhookEventStatusForKey(ctx, h.tdb, midtrans.NotificationIdentity(settlement))
	require.NoError(t, err)
	require.True(t, found, "the settlement identity must be recorded")
	assert.Equal(t, paymentrepo.PaymentWebhookEventStatusSucceeded, status)
}

// TestWebhookNotificationIdentity_OutOfOrderSettlementBeforePending proves
// out-of-order delivery still converges: a settlement that arrives BEFORE the
// pending notification settles the payment, and the late pending notification is
// stored as its own event without downgrading anything.
func TestWebhookNotificationIdentity_OutOfOrderSettlementBeforePending(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	settlement := h.makeWebhookNotification(fx, string(midtrans.StatusSettlement))
	pending := h.makeWebhookNotification(fx, string(midtrans.StatusPending))

	require.NoError(t, h.handleWebhook(ctx, settlement))

	settled, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, settled.Status)

	// The late (out-of-order) pending notification must be preserved, not dropped
	// as a duplicate of the settlement identity.
	require.NoError(t, h.handleWebhook(ctx, pending))

	after, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, after.Status,
		"an out-of-order pending notification must never downgrade a settled payment")

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, after.ReferenceID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), spendCount, "out-of-order delivery must not finalize twice")

	rows, err := countWebhookEventsForTransaction(ctx, h.tdb, fx.TransactionID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), rows, "both distinct notifications must be preserved")
}

// TestWebhookNotificationIdentity_ReversalIsPreservedNotDropped proves a gateway
// reversal (settlement → deny) produces its own stored event instead of being
// discarded because it shares the transaction_id.
//
// SCOPE NOTE: what the reversal must DO to a settled payment is the open
// business-policy finding carried over from REC-2 (the canonical state machine
// deliberately refuses to downgrade a settled payment). REC-3 only guarantees
// that the reversal signal now EXISTS to act on. This test asserts preservation
// and does not encode a recovery policy.
func TestWebhookNotificationIdentity_ReversalIsPreservedNotDropped(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	settlement := h.makeWebhookNotification(fx, string(midtrans.StatusSettlement))
	reversal := h.makeWebhookNotification(fx, string(midtrans.StatusDeny))

	require.NoError(t, h.handleWebhook(ctx, settlement))
	require.NoError(t, h.handleWebhook(ctx, reversal))

	keys, err := loadWebhookNotificationKeys(ctx, h.tdb, fx.TransactionID)
	require.NoError(t, err)
	require.Len(t, keys, 2, "the reversal notification must be stored as its own event, not dropped as a duplicate")
	assert.Contains(t, keys, midtrans.NotificationIdentity(settlement))
	assert.Contains(t, keys, midtrans.NotificationIdentity(reversal))

	status, found, err := loadWebhookEventStatusForKey(ctx, h.tdb, midtrans.NotificationIdentity(reversal))
	require.NoError(t, err)
	assert.True(t, found, "a reversal must be queryable in the event store so recovery can find it")
	assert.NotEmpty(t, status)

	// The reversal is discoverable FROM THE STORE alone — which is exactly what
	// REC-2 proved was impossible before, because no row was ever written.
	rows, err := countWebhookEventsForTransaction(ctx, h.tdb, fx.TransactionID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), rows)
}

// TestWebhookNotificationIdentity_UnrelatedTransactionsDoNotCollide proves the
// identity cannot merge two different payments that happen to share a status.
func TestWebhookNotificationIdentity_UnrelatedTransactionsDoNotCollide(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fxA := h.createSettlementFixture(t, 10000, 4000, 20000)
	fxB := h.createSettlementFixture(t, 0, 4000, 20000)

	require.NotEqual(t, fxA.TransactionID, fxB.TransactionID)

	pendingA := h.makeWebhookNotification(fxA, string(midtrans.StatusPending))
	pendingB := h.makeWebhookNotification(fxB, string(midtrans.StatusPending))
	settlementA := h.makeWebhookNotification(fxA, string(midtrans.StatusSettlement))

	require.NoError(t, h.handleWebhook(ctx, pendingA))
	require.NoError(t, h.handleWebhook(ctx, pendingB))
	require.NoError(t, h.handleWebhook(ctx, settlementA))

	paymentA, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fxA.MidtransID)
	require.NoError(t, err)
	paymentB, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fxB.MidtransID)
	require.NoError(t, err)

	assert.Equal(t, paymentrepo.PaymentStatusSettlement, paymentA.Status)
	assert.Equal(t, paymentrepo.PaymentStatusPending, paymentB.Status,
		"settling one transaction must not settle another that merely shares a status")

	rowsA, err := countWebhookEventsForTransaction(ctx, h.tdb, fxA.TransactionID)
	require.NoError(t, err)
	rowsB, err := countWebhookEventsForTransaction(ctx, h.tdb, fxB.TransactionID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), rowsA)
	assert.Equal(t, int64(1), rowsB)

	assert.NotEqual(t, midtrans.NotificationIdentity(pendingA), midtrans.NotificationIdentity(pendingB),
		"two different transactions with the same status must have different identities")
}

// TestWebhookNotificationIdentity_ConcurrentDifferentNotificationsConverge
// proves the stronger concurrency property REC-3 must not have broken: a pending
// and a settlement notification for the same transaction delivered at the same
// time leave exactly ONE settled payment with ONE coin spend — even though both
// are now distinct events rather than one deduped away.
func TestWebhookNotificationIdentity_ConcurrentDifferentNotificationsConverge(t *testing.T) {
	ctx := context.Background()
	h := newPaymentSettlementHarness(t)

	fx := h.createSettlementFixture(t, 10000, 4000, 20000)
	notifications := []*midtrans.NotificationPayload{
		h.makeWebhookNotification(fx, string(midtrans.StatusPending)),
		h.makeWebhookNotification(fx, string(midtrans.StatusSettlement)),
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, len(notifications))
	for i, notif := range notifications {
		wg.Add(1)
		go func(idx int, n *midtrans.NotificationPayload) {
			defer wg.Done()
			<-start
			errs[idx] = h.handleWebhook(ctx, n)
		}(i, notif)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "delivery %d must complete cleanly", i)
	}

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status,
		"concurrent pending + settlement must converge on a single settled payment")

	spendCount, err := countCoinSpendRows(ctx, h.tdb, h.buyerID, payment.ReferenceID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), spendCount, "concurrency must not double-consume coins")

	rows, err := countWebhookEventsForTransaction(ctx, h.tdb, fx.TransactionID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), rows, "each distinct notification owns exactly one row")
}

// TestWebhookNotificationKey_MigrationBackfillMatchesGo proves the REC-3
// lockstep requirement between migration 000098 and pkg/midtrans: the SQL
// expression that backfilled historical rows must compute the SAME identity the
// runtime computes, for every notification shape (including omitted optional
// fields, where Go's `omitempty` and SQL's COALESCE must agree).
//
// The expression is EXTRACTED FROM THE MIGRATION FILE rather than retyped, so
// this test fails if the two drift apart.
func TestWebhookNotificationKey_MigrationBackfillMatchesGo(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000098_webhook_notification_identity.up.sql"))
	require.NoError(t, err, "migration 000098 must exist for the identity backfill to be reproducible")

	text := string(raw)
	const marker = "SET notification_key = "
	start := strings.Index(text, marker)
	require.NotEqual(t, -1, start, "the backfill UPDATE must set notification_key from the payload")
	expr := text[start+len(marker):]
	end := strings.Index(expr, "WHERE notification_key IS NULL")
	require.NotEqual(t, -1, end, "the backfill must be guarded by WHERE notification_key IS NULL")
	expr = strings.TrimSpace(expr[:end])
	require.Contains(t, expr, "md5(", "the backfill must use a SQL-computable hash (pgcrypto is not available)")

	// Every shape the webhook can receive: settlement, pending, a reversal, a
	// refund ack, and one with all optional fields omitted.
	shapes := []*midtrans.NotificationPayload{
		{
			TransactionTime:   "2026-09-18T10:00:00Z",
			TransactionStatus: "settlement",
			TransactionID:     "PARITY-TX-1",
			StatusMessage:     "midtrans payment notification",
			StatusCode:        "200",
			SignatureKey:      "irrelevant-for-identity",
			PaymentType:       "bank_transfer",
			OrderID:           "LAB-PARITY-1",
			MerchantID:        "M1",
			GrossAmount:       "10000.00",
			FraudStatus:       "accept",
			Currency:          "IDR",
		},
		{
			TransactionTime:   "2026-09-18T10:01:00Z",
			TransactionStatus: "pending",
			TransactionID:     "PARITY-TX-1",
			StatusCode:        "201",
			PaymentType:       "qris",
			OrderID:           "LAB-PARITY-1",
			GrossAmount:       "10000.00",
			Currency:          "IDR",
		},
		{
			TransactionStatus: "deny",
			TransactionID:     "PARITY-TX-1",
			StatusCode:        "202",
			PaymentType:       "bank_transfer",
			OrderID:           "LAB-PARITY-1",
			GrossAmount:       "10000.00",
		},
		{
			TransactionStatus: "refund",
			TransactionID:     "PARITY-TX-1",
			StatusCode:        "200",
			PaymentType:       "credit_card",
			OrderID:           "LAB-PARITY-1",
			GrossAmount:       "10000.00",
			RefundKey:         "REFUND-KEY-1",
			RefundAmount:      "10000.00",
			RefundChargeID:    "42",
		},
	}

	computed := make(map[string]string, len(shapes))
	for i, notif := range shapes {
		payload, err := json.Marshal(notif)
		require.NoError(t, err)

		rowID := uuid.New()
		// A placeholder identity: the stored value is irrelevant to the expression
		// under test, which is evaluated against the payload column.
		placeholder := fmt.Sprintf("parity-placeholder-%d", i)
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO payment_webhook_events
					(id, provider, event_id, notification_key, midtrans_order_id, signature_key, payload, status, received_at)
				VALUES ($1, 'midtrans', $2, $3, $4, '', $5, 'succeeded', NOW())
			`, rowID, notif.TransactionID, placeholder, notif.OrderID, payload)
			return err
		}))

		var sqlKey string
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			return tx.QueryRow(ctx,
				"SELECT "+expr+" FROM payment_webhook_events WHERE id = $1",
				rowID).Scan(&sqlKey)
		}))

		goKey := midtrans.NotificationIdentity(notif)
		assert.NotEqual(t, placeholder, sqlKey, "the placeholder must not accidentally equal the computed identity")
		assert.Equal(t, goKey, sqlKey,
			"shape %d: the migration's SQL identity must equal pkg/midtrans.NotificationIdentity — field order, separator and payload JSON keys are locked in lockstep", i)

		computed[goKey] = notif.TransactionStatus
	}

	// Distinct signals of the SAME transaction must never collapse into one key.
	assert.Len(t, computed, len(shapes),
		"every distinct notification shape must produce a distinct identity")
}
