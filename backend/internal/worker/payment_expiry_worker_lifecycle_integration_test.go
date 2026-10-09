//go:build integration

package worker

import (
	"context"
	"testing"

	"github.com/google/uuid"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	escrowApp "github.com/labuda/backend/internal/core/escrow/application"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// buildUnpaidExpiryWorker wires a PaymentExpiryWorker against a real test DB
// with a minimal OrderService (escrow service real; all creation-only deps
// nil — Expire-on-unpaid touches none of them).
func buildUnpaidExpiryWorker(t *testing.T, tdb *testdb.TestDB) *PaymentExpiryWorker {
	t.Helper()
	database := db.NewFromPool(tdb.Pool())
	escrowService := escrowApp.NewEscrowService(database, zap.NewNop())
	orderService := orderApp.NewOrderService(
		nil, // accountStatusChecker — unused by Expire
		nil, // shippingService — creation only
		outboxRepo.NewOutboxRepository(database),
		nil, // configService — creation only
		nil, // coinsService — Complete only
		nil, // roleChecker — creation only
		nil, // actorResolver — creation only
		nil, // auditService — creation only
		nil, // productShippingRepo — creation only
		escrowService,
		nil, // shippingQuoteService — no-op when order has no ShippingQuoteID
	)
	return NewPaymentExpiryWorker(database, orderService, zap.NewNop(), DefaultConfig())
}

// seedUnpaidExpiredOrderPayment inserts a pending order past its payment
// window together with its pending payment row past expired_at — the exact
// population PaymentExpiryWorker owns (the timeout worker excludes it).
func seedUnpaidExpiredOrderPayment(t *testing.T, tdb *testdb.TestDB) (orderID, buyerID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	buyerID, sellerID, orderID := uuid.New(), uuid.New(), uuid.New()
	_, err := tdb.Pool().Exec(ctx,
		`INSERT INTO users (id,firebase_uid,email) VALUES ($1,$2,$3),($4,$5,$6)`,
		buyerID, buyerID.String(), buyerID.String()+"@expiry-proof.test",
		sellerID, sellerID.String(), sellerID.String()+"@expiry-proof.test",
	)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO orders (
			id, buyer_id, seller_id, source_type, source_id,
			quantity, unit_price, subtotal, shipping_total,
			commission_percent, commission_amount,
			status, payment_expires_at, created_at, updated_at
		) VALUES ($1,$2,$3,'for_sale',$1,1,150000,150000,0,0,0,
		          'pending_payment', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '2 hour', NOW() - INTERVAL '2 hour')
	`, orderID, buyerID, sellerID)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO payments (
			id, user_id, payment_number, midtrans_order_id, gross_amount,
			status, reference_type, reference_id, expired_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,150000,'pending','order',$5,
		          NOW() - INTERVAL '1 hour', NOW() - INTERVAL '2 hour', NOW() - INTERVAL '2 hour')
	`, uuid.New(), buyerID, "PAY-EXPIRY-PROOF-"+orderID.String()[:8], "LAB-EXPIRY-"+orderID.String()[:8], orderID)
	require.NoError(t, err)
	return orderID, buyerID
}

// MANDATORY TEST #7 — expiry lifecycle: pending order + pending payment past
// the window → worker run → payment expires AND order expires in one
// consistent outcome (per-payment transaction boundary).
// MANDATORY TEST #8 — idempotency: a second run is a no-op (no duplicate
// transitions, no duplicate events, no financial side effects).
// MANDATORY TEST #9 — safety: the worker scope cannot select settled
// payments; a settled payment + paid order + holding escrow is untouched.
func TestPaymentExpiryWorker_UnpaidExpired_LifecycleIdempotencyAndSettledSafety(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	worker := buildUnpaidExpiryWorker(t, tdb)

	// ── #7: unpaid expired order is closed ─────────────────────────────────
	orderID, _ := seedUnpaidExpiredOrderPayment(t, tdb)

	worker.checkExpiredPayments()

	var payStatus, orderStatus string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status FROM payments WHERE reference_type='order' AND reference_id=$1`, orderID).Scan(&payStatus))
	require.Equal(t, "expire", payStatus, "pending payment past expired_at must flip to expire")
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status FROM orders WHERE id=$1`, orderID).Scan(&orderStatus))
	require.Equal(t, "expired", orderStatus, "order must expire with its payment in the same sweep")

	// Owner invariant: no escrow / no ledger movement for unpaid expiry.
	var escrowRows, ledgerTx int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM escrows WHERE order_id=$1`, orderID).Scan(&escrowRows))
	require.Equal(t, 0, escrowRows, "unpaid expiry must never create an escrow row")
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger_transactions WHERE reference_id=$1`, orderID).Scan(&ledgerTx))
	require.Equal(t, 0, ledgerTx, "unpaid expiry must never write the financial ledger")

	// Exactly one order.expired outbox event.
	var orderExpiredEvents int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM outbox WHERE event_type='order.expired' AND aggregate_id=$1`, orderID).Scan(&orderExpiredEvents))
	if orderExpiredEvents != 1 {
		rows, _ := tdb.Pool().Query(ctx, `SELECT event_type, aggregate_id::text, idempotency_key, status FROM outbox`)
		if rows != nil {
			for rows.Next() {
				var et, ag, ik, st string
				_ = rows.Scan(&et, &ag, &ik, &st)
				t.Logf("outbox row: type=%s agg=%s key=%s status=%s", et, ag, ik, st)
			}
			rows.Close()
		}
	}
	require.Equal(t, 1, orderExpiredEvents)

	// ── #8: second run is a pure no-op ─────────────────────────────────────
	worker.checkExpiredPayments()

	var payStatus2, orderStatus2 string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status FROM payments WHERE reference_type='order' AND reference_id=$1`, orderID).Scan(&payStatus2))
	require.Equal(t, "expire", payStatus2)
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status FROM orders WHERE id=$1`, orderID).Scan(&orderStatus2))
	require.Equal(t, "expired", orderStatus2)
	var orderExpiredEvents2 int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM outbox WHERE event_type='order.expired' AND aggregate_id=$1`, orderID).Scan(&orderExpiredEvents2))
	require.Equal(t, 1, orderExpiredEvents2, "second run must not emit a duplicate order.expired event")

	// ── #9: settled payment + holding escrow is out of scope ───────────────
	paidOrderID, paidBuyerID, paidSellerID := uuid.New(), uuid.New(), uuid.New()
	_, err := tdb.Pool().Exec(ctx,
		`INSERT INTO users (id,firebase_uid,email) VALUES ($1,$2,$3),($4,$5,$6)`,
		paidBuyerID, paidBuyerID.String(), paidBuyerID.String()+"@settled-proof.test",
		paidSellerID, paidSellerID.String(), paidSellerID.String()+"@settled-proof.test",
	)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO orders (
			id, buyer_id, seller_id, source_type, source_id,
			quantity, unit_price, subtotal, shipping_total,
			commission_percent, commission_amount,
			status, payment_expires_at, created_at, updated_at
		) VALUES ($1,$2,$3,'for_sale',$1,1,200000,200000,0,0,0,
		          'paid', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '2 hour', NOW() - INTERVAL '1 hour')
	`, paidOrderID, paidBuyerID, paidSellerID)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO payments (
			id, user_id, payment_number, midtrans_order_id, gross_amount,
			status, reference_type, reference_id, expired_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,200000,'settlement','order',$5,
		          NOW() - INTERVAL '1 hour', NOW() - INTERVAL '2 hour', NOW() - INTERVAL '1 hour')
	`, uuid.New(), paidBuyerID, "PAY-SETTLED-PROOF-"+paidOrderID.String()[:8], "LAB-SETTLED-"+paidOrderID.String()[:8], paidOrderID)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx,
		`INSERT INTO escrows (id, order_id, amount, status, created_at) VALUES ($1,$2,200000,'holding',NOW())`,
		uuid.New(), paidOrderID)
	require.NoError(t, err)

	worker.checkExpiredPayments()

	var settledStatus, paidOrderStatus, escrowStatus string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status FROM payments WHERE reference_type='order' AND reference_id=$1`, paidOrderID).Scan(&settledStatus))
	require.Equal(t, "settlement", settledStatus, "worker must never expire a settled payment")
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status FROM orders WHERE id=$1`, paidOrderID).Scan(&paidOrderStatus))
	require.Equal(t, "paid", paidOrderStatus, "worker must never expire a paid order")
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status FROM escrows WHERE order_id=$1`, paidOrderID).Scan(&escrowStatus))
	require.Equal(t, "holding", escrowStatus, "worker must never touch the escrow row")
}
