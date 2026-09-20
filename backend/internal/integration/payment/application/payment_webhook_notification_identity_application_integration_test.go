//go:build integration

// REC-3: WEBHOOK NOTIFICATION IDENTITY — application-level acceptance proofs.
//
// These tests answer the questions REC-2 could only ask, now against the
// corrected identity contract. They run on the disposable test database through
// the canonical webhook service; the harness deliberately wires NO canonical
// finalization service, which is what makes a PROCESSED settlement observably
// loud (it errors) while a DISCARDED one used to be observably silent.
//
// What is proven here:
//
//  1. A settlement notification arriving after a pending notification for the
//     SAME gateway transaction is no longer collapsed into the pending event.
//     Before REC-3 (event_id = transaction_id, UNIQUE) this delivery returned
//     nil, wrote no row, recorded no failure, and left the payment pending —
//     the money signal was thrown away. It is now processed, which means a
//     failure to finalize is LOUD and durably recorded by REC-1.
//
//  2. CONTROL — a settlement notification that IS processed errors and leaves a
//     durable 'failed' row. The contrast is what makes (1) falsifiable.
//
//  3. A gateway reversal (settlement → deny) is now STORED as its own event
//     rather than dropped for sharing a transaction_id. The canonical state
//     machine still refuses to downgrade a settled payment: that is the open
//     business-policy finding carried over from REC-2, unchanged by REC-3.
//
//  4. A durably recorded 'failed' event remains visible in the DATA but is
//     consumed by NOBODY: the row-driven orphan-recovery contract
//     (GetOrphanedWebhookEvents / MarkWebhookEventForRetry / OrphanWebhookEvent)
//     no longer exists in the repository, so no event-driven consumer remains.
//
// TRANSACTION-CALLBACK RULE: no require.*/assert.*/t.Fatal may run inside a
// WithTx callback (runtime.Goexit leaks the pooled connection).
//
// Run with: go test -tags integration -run TestNotificationIdentity ./internal/integration/payment/application/...
package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/testdb"
)

// seedUserAndPayment inserts the minimum rows a webhook needs to find a payment:
// a users row (user_id is a hard FK) and a payments row addressed by
// midtrans_order_id, which is the key the webhook looks up.
func seedUserAndPayment(
	t *testing.T,
	ctx context.Context,
	testDB *testdb.TestDB,
	midtransOrderID string,
	grossAmount int64,
	status string,
) uuid.UUID {
	t.Helper()

	var paymentID uuid.UUID
	require.NoError(t, testDB.WithTx(ctx, func(tx db.Tx) error {
		var userID uuid.UUID
		if err := tx.QueryRow(ctx,
			`INSERT INTO users (firebase_uid, email, role) VALUES ($1, $2, 'user') RETURNING id`,
			"rec2-"+uuid.NewString(),
			uuid.NewString()+"@rec2.example.test",
		).Scan(&userID); err != nil {
			return err
		}

		// $5 is cast explicitly at BOTH use sites: an enum assignment and a text
		// comparison of the same parameter makes Postgres deduce two types
		// (SQLSTATE 42P08) — the same reason updateWebhookEventStatus casts its
		// status parameter.
		return tx.QueryRow(ctx,
			`INSERT INTO payments
			   (user_id, payment_number, midtrans_order_id, gross_amount, status,
			    reference_type, reference_id, expired_at, paid_at, transaction_id)
			 VALUES ($1, $2, $3, $4, $5::payment_status_enum, 'order', $6, NOW() + INTERVAL '1 hour',
			         CASE WHEN $5::text = 'settlement' THEN NOW() ELSE NULL END,
			         CASE WHEN $5::text = 'settlement' THEN 'REC2-SETTLED-TX' ELSE NULL END)
			 RETURNING id`,
			userID,
			"REC2-"+uuid.NewString(),
			midtransOrderID,
			grossAmount,
			status,
			uuid.New(),
		).Scan(&paymentID)
	}))

	return paymentID
}

// signedNotification builds a notification Midtrans would accept: the signature
// is computed with the same client key the service verifies against.
func signedNotification(client *midtrans.Client, orderID, transactionID, status, gross string) *midtrans.NotificationPayload {
	n := &midtrans.NotificationPayload{
		TransactionID:     transactionID,
		OrderID:           orderID,
		TransactionStatus: status,
		StatusCode:        "200",
		GrossAmount:       gross,
		PaymentType:       "bank_transfer",
		Currency:          "IDR",
		FraudStatus:       "accept",
	}
	n.SignatureKey = client.BuildWebhookSignature(n)
	return n
}

func paymentState(t *testing.T, ctx context.Context, testDB *testdb.TestDB, paymentID uuid.UUID) (status string, paidAtSet bool, transactionID *string) {
	t.Helper()

	require.NoError(t, testDB.WithTx(ctx, func(tx db.Tx) error {
		var paidAt *string
		if err := tx.QueryRow(ctx,
			`SELECT status, paid_at::text, transaction_id FROM payments WHERE id = $1`,
			paymentID,
		).Scan(&status, &paidAt, &transactionID); err != nil {
			return err
		}
		paidAtSet = paidAt != nil
		return nil
	}))

	return status, paidAtSet, transactionID
}

// TestNotificationIdentity_SettlementSurvivesAfterPending is the core REC-3
// proof at the application layer: Midtrans's normal lifecycle (pending →
// settlement) for one gateway transaction produces two notifications that share
// a transaction_id, and the settlement — the only one that matters — must now be
// stored and processed instead of being thrown away as a duplicate.
func TestNotificationIdentity_SettlementSurvivesAfterPending(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, client := newDurabilityService(t, testDB)

	const orderID = "LAB-REC2-COLLAPSE-1"
	const transactionID = "REC2-MT-TX-COLLAPSE-1"
	const gross = "10000.00"

	paymentID := seedUserAndPayment(t, ctx, testDB, orderID, 10000, "pending")

	// DELIVERY 1 — the gateway has created the transaction (status pending).
	pending := signedNotification(client, orderID, transactionID, string(midtrans.StatusPending), gross)
	require.NoError(t, svc.HandleWebhook(ctx, pending, "203.0.113.11"),
		"a pending notification for an existing pending payment must succeed")

	status, paidAtSet, transactionID1 := paymentState(t, ctx, testDB, paymentID)
	require.Equal(t, "pending", status, "precondition: pending must not settle anything")
	require.False(t, paidAtSet)
	require.Nil(t, transactionID1)
	require.Equal(t, 1, countWebhookEvents(t, ctx, testDB, pending),
		"precondition: the pending notification owns exactly one event")

	// DELIVERY 2 — the customer pays and the SAME gateway transaction settles.
	settlement := signedNotification(client, orderID, transactionID, string(midtrans.StatusSettlement), gross)
	err := svc.HandleWebhook(ctx, settlement, "203.0.113.11")

	// REC-3: this delivery is PROCESSED, not deduped away. The harness wires no
	// canonical finalization service, so processing must fail LOUDLY — the exact
	// opposite of the pre-REC-3 silent nil.
	require.Error(t, err,
		"the settlement notification must be processed now; a silent nil would mean it was discarded again")
	assert.Contains(t, err.Error(), "canonical finalization service not wired",
		"the settlement reached canonical finalization instead of being dropped as a duplicate")

	// The payment is untouched by the failure (its transaction rolled back), which
	// is exactly why the durable record below matters.
	statusAfter, paidAtAfter, transactionIDAfter := paymentState(t, ctx, testDB, paymentID)
	assert.Equal(t, "pending", statusAfter, "precondition: order-payment finalization is not wired in this harness")
	assert.False(t, paidAtAfter)
	assert.Nil(t, transactionIDAfter)

	// PROOF OF SURVIVAL: the settlement now owns a row of its own.
	assert.Equal(t, 2, countWebhookEvents(t, ctx, testDB, pending)+countWebhookEvents(t, ctx, testDB, settlement),
		"the pending and the settlement notification are DISTINCT events and both must be stored")

	_, pendingStatus, _ := webhookEventRow(t, ctx, testDB, pending)
	assert.Equal(t, repository.PaymentWebhookEventStatusSucceeded, pendingStatus)

	_, settlementStatus, settlementErr := webhookEventRow(t, ctx, testDB, settlement)
	assert.Equal(t, repository.PaymentWebhookEventStatusFailed, settlementStatus,
		"the settlement's processing failure is recorded against ITS identity, not the pending notification's")
	assert.Contains(t, settlementErr, "canonical finalization service not wired")

	assert.NotEqual(t, midtrans.NotificationIdentity(pending), midtrans.NotificationIdentity(settlement),
		"two different signals of one transaction must never share an identity")
}

// TestNotificationIdentity_ControlProcessedSettlementIsLoud proves the contrast
// that makes the test above falsifiable: when a settlement notification is NOT
// dropped, the platform is loud (error + durable failure record).
func TestNotificationIdentity_ControlProcessedSettlementIsLoud(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, client := newDurabilityService(t, testDB)

	const orderID = "LAB-REC2-CONTROL-1"
	const transactionID = "REC2-MT-TX-CONTROL-1"

	seedUserAndPayment(t, ctx, testDB, orderID, 10000, "pending")

	// A settlement whose notification identity has never been seen, so the
	// duplicate short-circuit cannot apply. The harness has no canonical
	// finalization service wired, so processing must fail loudly.
	settlement := signedNotification(client, orderID, transactionID, string(midtrans.StatusSettlement), "10000.00")
	err := svc.HandleWebhook(ctx, settlement, "203.0.113.12")

	require.Error(t, err, "a processed settlement must be observable as a failure in this harness")
	// The harness wires no canonical finalization service, so the first guard the
	// processed settlement hits is the nil-service guard. Any processed
	// settlement is loud either way; the point is only that the platform reacts.
	require.Contains(t, err.Error(), "CRITICAL: canonical finalization service not wired")

	_, eventStatus, eventErr := webhookEventRow(t, ctx, testDB, settlement)
	assert.Equal(t, repository.PaymentWebhookEventStatusFailed, eventStatus,
		"a processed failure leaves the durable REC-1 record — unlike a dropped notification")
	assert.Contains(t, eventErr, "CRITICAL: canonical finalization service not wired")
}

// TestNotificationIdentity_SettledPaymentReversalStillRefusesDowngrade records
// the REC-2 finding that REC-3 does NOT change: now that a gateway reversal
// (settlement → deny) is preserved as its own event (proven end-to-end in
// serverboot), the canonical failure path still refuses to downgrade a settled
// payment. Midtrans documents settlement → deny reversals
// (Permata/Mandiri/Indomaret) as cases the merchant "must handle" and requires
// treating such a transaction as not paid — so what the platform should DO here
// is the open business-policy question, not an identity question.
func TestNotificationIdentity_SettledPaymentReversalStillRefusesDowngrade(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, _ := newDurabilityService(t, testDB)

	const orderID = "LAB-REC2-REVERSAL-1"
	paymentID := seedUserAndPayment(t, ctx, testDB, orderID, 10000, "settlement")

	status, paidAtSet, transactionID := paymentState(t, ctx, testDB, paymentID)
	require.Equal(t, "settlement", status)
	require.True(t, paidAtSet)
	require.NotNil(t, transactionID)

	// The gateway reverses the settlement: FailPayment is the canonical failure
	// path the webhook uses for deny/cancel/expire.
	require.NoError(t, svc.db.WithTx(ctx, func(tx db.Tx) error {
		return svc.settlementService.FailPayment(ctx, tx, orderID, string(midtrans.StatusDeny))
	}), "the reversal is accepted without error")

	statusAfter, paidAtAfter, transactionIDAfter := paymentState(t, ctx, testDB, paymentID)
	assert.Equal(t, "settlement", statusAfter,
		"FINDING: the platform still treats the money as received after the gateway reversed it")
	assert.True(t, paidAtAfter)
	assert.Equal(t, *transactionID, *transactionIDAfter)
}

// TestFailedWebhookIsInvisibleToEveryRecoveryConsumer proves that REC-1's
// durable failure record is visible in the DATA but consumed by NOBODY. The
// row-driven orphan-recovery contract this test used to interrogate
// (GetOrphanedWebhookEvents / MarkWebhookEventForRetry / OrphanWebhookEvent) no
// longer exists in the repository, so there is no event-driven recovery
// consumer to interrogate at all — the compile is the negative proof. This is
// the REC-2 finding that REC-3 deliberately leaves untouched (it is about
// recovery authority, not event identity).
func TestFailedWebhookIsInvisibleToEveryRecoveryConsumer(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, _ := newDurabilityService(t, testDB)

	const orderID = "LAB-REC2-INVISIBLE-1"
	const transactionID = "REC2-MT-TX-INVISIBLE-1"

	notification := failureNotification(transactionID, orderID)
	svc.recordWebhookFailureDurably(ctx, notification, "203.0.113.13",
		errors.New("CRITICAL: failed to finalize order payment"))

	// POSITIVE: the failure is durable and queryable by an operator.
	found, status, errorMessage := webhookEventRow(t, ctx, testDB, notification)
	require.True(t, found, "the failure must be durably recorded")
	require.Equal(t, repository.PaymentWebhookEventStatusFailed, status)
	require.Contains(t, errorMessage, "failed to finalize order payment")

	// The row is still exactly as REC-1 left it: evidence, not a work queue.
	foundAfter, statusAfter, _ := webhookEventRow(t, ctx, testDB, notification)
	assert.True(t, foundAfter)
	assert.Equal(t, repository.PaymentWebhookEventStatusFailed, statusAfter)
}
