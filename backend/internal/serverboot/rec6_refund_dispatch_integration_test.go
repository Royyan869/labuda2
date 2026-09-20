//go:build integration

// REC-6 SLICE 2: integration proofs for the gateway refund DISPATCH executor.
//
// These tests exercise DispatchPendingRec6Refunds (the single dispatch
// authority) against the REAL refund/payment schema through the real
// RefundService + Rec6RefundDispatchWorker, proving the Slice-2 acceptance
// gate:
//
//  1. Happy path: a Slice-1 intent (status=system_refunded,
//     gateway_status=unsubmitted, key=rec6:payment:<payment_id>,
//     amount=payment.gross_amount) is dispatched via the existing
//     RefundWithKey capability and lands in gateway_status='pending'
//     (REQUESTED/SUBMITTED — NOT confirmed/succeeded).
//  2. No residue: no escrow row, no ledger_transactions row, order untouched
//     (not finalized/paid), decision axis untouched (system_refunded,
//     reviewed_by NULL).
//  3. Failure/timeout persists gateway_status='failed' + last_gateway_error —
//     never a false confirmed state — and a later success flips it to
//     'pending' with the SAME deterministic key.
//  4. Crash recovery: a claimed-but-unresolved intent (pending,
//     gateway_requested_at IS NULL, older than the grace) is reclaimed and
//     completed on the next scan.
//  5. Succeeded terminal guard: dispatch never regresses a confirmed refund.
//  6. Concurrent scans: exactly one dispatch effect, exactly one intent row.
//  7. Gateway spy: RefundWithKey receives the canonical key + gross amount.
//
// Run with: go test -tags integration -run TestRec6Dispatch ./internal/serverboot/

package serverboot

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	refundapp "github.com/labuda/backend/internal/finance/refund/application"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/internal/worker"
)

// ---------------------------------------------------------------------
// Refund-capable gateway spy (RefundWithKey only — what Slice 2 uses)
// ---------------------------------------------------------------------

type rec6RefundCall struct {
	OrderID    string
	RefundKey  string
	Amount     int64
	Reason     string
}

type rec6RefundGateway struct {
	mu        sync.Mutex
	failKeys  map[string]bool // refund keys that fail
	calls     []rec6RefundCall
}

func newRec6RefundGateway() *rec6RefundGateway {
	return &rec6RefundGateway{failKeys: make(map[string]bool)}
}

func (g *rec6RefundGateway) setFail(refundKey string, fail bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failKeys[refundKey] = fail
}

func (g *rec6RefundGateway) callsForOrder(orderID string) []rec6RefundCall {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []rec6RefundCall
	for _, c := range g.calls {
		if c.OrderID == orderID {
			out = append(out, c)
		}
	}
	return out
}

// RefundWithKey mirrors the real Midtrans Core API refund endpoint contract:
// a merchant refund key, an amount, a reason; 200/201 with a parsed response
// or an error. It records every call so tests can assert idempotency.
func (g *rec6RefundGateway) RefundWithKey(
	_ context.Context, orderID, refundKey string, amount int64, reason string,
) (*midtrans.RefundResponse, error) {
	g.mu.Lock()
	g.calls = append(g.calls, rec6RefundCall{
		OrderID: orderID, RefundKey: refundKey, Amount: amount, Reason: reason,
	})
	fail := g.failKeys[refundKey]
	g.mu.Unlock()

	if fail {
		return nil, fmt.Errorf("simulated gateway outage for refund key %s", refundKey)
	}
	chargeID := "rchg-" + refundKey[len(refundKey)-8:]
	return &midtrans.RefundResponse{
		RefundChargeID: chargeID,
		StatusCode:     "200",
		StatusMessage:  "Success, refund is waiting for approval",
	}, nil
}

// wireRec6Dispatcher wires the harness refund service as the dispatch
// authority: tx runner for the claim/outcome transactions and the gateway
// client (existing GatewayRefundClient surface).
func (h *discoveryTestHarness) wireRec6Dispatcher(gw *rec6RefundGateway) {
	h.refundService.SetTxRunner(h.dbConn)
	h.refundService.SetGatewayClient(gw, zap.NewNop())
}

// ---------------------------------------------------------------------
// Fixture: order + payment + a Slice-1 REC-6 refund intent (expired order)
// ---------------------------------------------------------------------

type rec6DispatchFixture struct {
	OrderID     uuid.UUID
	MidtransID  string
	GrossAmount int64
	PaymentID   uuid.UUID
	IdemKey     string
	RefundID    uuid.UUID
}

func (h *discoveryTestHarness) createRec6IntentFixture(t *testing.T) *rec6DispatchFixture {
	t.Helper()
	ctx := context.Background()

	fx := h.createDiscoveryFixture(t)

	// Expire the order so the REC-6 invalid predicate holds (mirrors Slice-1
	// scenarios E/F: order expired ⇒ payment must not finalize).
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE orders
			SET status = 'expired', payment_expires_at = NOW() - INTERVAL '1 hour'
			WHERE id = $1
		`, fx.OrderID)
		return err
	}))

	// Seed the REC-6 intent exactly as the canonical Slice-1 authority does.
	var refundID uuid.UUID
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		refund, err := h.refundService.CreateRefundIntentForInvalidOrder(ctx, tx, refundapp.Rec6RefundIntentInput{
			PaymentID:              fx.Payment.ID,
			OrderID:                fx.OrderID,
			BuyerID:                h.buyerID,
			SellerID:               h.sellerID,
			GrossAmount:            fx.GrossAmount,
			PaymentMidtransOrderID: fx.MidtransID,
		})
		if err != nil {
			return err
		}
		refundID = refund.ID
		return nil
	}))

	// Sanity: the intent row is unsubmitted before dispatch.
	var gwStatus string
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT gateway_status FROM refunds WHERE id = $1`, refundID).Scan(&gwStatus)
	}))
	require.Equal(t, "unsubmitted", gwStatus)

	return &rec6DispatchFixture{
		OrderID:     fx.OrderID,
		MidtransID:  fx.MidtransID,
		GrossAmount: fx.GrossAmount,
		PaymentID:   fx.Payment.ID,
		IdemKey:     refundapp.Rec6IdempotencyKey(fx.Payment.ID),
		RefundID:    refundID,
	}
}

func (h *discoveryTestHarness) loadRec6DispatchRefundRow(ctx context.Context, refundID uuid.UUID) (
	idemKey, gwStatus string, attempts int, requestedAt, acknowledgedAt *time.Time, err error,
) {
	err = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT gateway_idempotency_key, gateway_status, gateway_attempts,
			       gateway_requested_at, gateway_acknowledged_at
			FROM refunds WHERE id = $1
		`, refundID).Scan(&idemKey, &gwStatus, &attempts, &requestedAt, &acknowledgedAt)
	})
	return
}

func (h *discoveryTestHarness) countRec6OutboxEvents(ctx context.Context, refundID uuid.UUID, eventType string) int64 {
	var n int64
	require.NoError(h.t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM outbox WHERE event_type = $1 AND aggregate_id = $2`,
			eventType, refundID).Scan(&n)
	}))
	return n
}

func newRec6DispatchWorker(svc *refundapp.RefundService) *worker.Rec6RefundDispatchWorker {
	return worker.NewRec6RefundDispatchWorker(svc, zap.NewNop(), worker.DefaultRec6RefundDispatchConfig())
}

// ---------------------------------------------------------------------
// 1. Happy path — dispatch produces REQUESTED/SUBMITTED with zero residue
// ---------------------------------------------------------------------

func TestRec6Dispatch_Integration_HappyPath_PendingState_NoResidue(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createRec6IntentFixture(t)

	gw := newRec6RefundGateway()
	h.wireRec6Dispatcher(gw)
	newRec6DispatchWorker(h.refundService).ScanOnce()

	// Settlement axis: REQUESTED/SUBMITTED, distinct from confirmed.
	idemKey, gwStatus, attempts, requestedAt, ackAt, err := h.loadRec6DispatchRefundRow(ctx, fx.RefundID)
	require.NoError(t, err)
	assert.Equal(t, fx.IdemKey, idemKey, "merchant refund key must be rec6:payment:<payment_id>")
	assert.Equal(t, "pending", gwStatus, "successful dispatch must persist REQUESTED/SUBMITTED (pending), never succeeded")
	assert.Equal(t, 1, attempts, "exactly one dispatch attempt for a single scan")
	require.NotNil(t, requestedAt, "gateway_requested_at must be set by dispatch")
	assert.Nil(t, ackAt, "gateway_acknowledged_at must stay NULL — no webhook ack in Slice 2")

	// Decision axis untouched.
	var status, reason string
	var reviewedBy sql.NullString
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT status, reason, reviewed_by FROM refunds WHERE id = $1`,
			fx.RefundID).Scan(&status, &reason, &reviewedBy)
	}))
	assert.Equal(t, "system_refunded", status, "decision axis must stay system_refunded")
	assert.Equal(t, "gateway_captured_after_order_invalid", reason)
	assert.False(t, reviewedBy.Valid, "reviewed_by must stay NULL for automatic system refund")

	// Amount authority untouched.
	var reqAmt int64
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT requested_amount FROM refunds WHERE id = $1`, fx.RefundID).Scan(&reqAmt)
	}))
	assert.Equal(t, fx.GrossAmount, reqAmt, "refund amount must remain payment.gross_amount")

	// Residue proofs.
	assert.Equal(t, int64(0), h.countEscrows(fx.OrderID), "no escrow may be created by dispatch")
	var ledger int64
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM ledger_transactions WHERE reference_id = $1`, fx.OrderID).Scan(&ledger)
	}))
	assert.Equal(t, int64(0), ledger, "no ledger mutation allowed")
	var orderStatus string
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, fx.OrderID).Scan(&orderStatus)
	}))
	assert.Equal(t, "expired", orderStatus, "dispatch must not finalize the order")

	// Outbox audit trail.
	assert.Equal(t, int64(1), h.countRec6OutboxEvents(ctx, fx.RefundID, "money.refund_requested"))
}

// ---------------------------------------------------------------------
// 2. Idempotent re-run — a dispatched row is never re-attempted
// ---------------------------------------------------------------------

func TestRec6Dispatch_Integration_ReRunSkipsDispatchedRow(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createRec6IntentFixture(t)

	gw := newRec6RefundGateway()
	h.wireRec6Dispatcher(gw)
	w := newRec6DispatchWorker(h.refundService)

	w.ScanOnce()
	_, status1, attempts1, req1, _, err := h.loadRec6DispatchRefundRow(ctx, fx.RefundID)
	require.NoError(t, err)
	require.Equal(t, "pending", status1)

	w.ScanOnce() // must be a no-op: pending+requested rows are in flight, not reclaimable

	_, status2, attempts2, req2, _, err := h.loadRec6DispatchRefundRow(ctx, fx.RefundID)
	require.NoError(t, err)
	assert.Equal(t, "pending", status2)
	assert.Equal(t, attempts1, attempts2, "a dispatched row must not be re-attempted by the next scan")
	assert.Equal(t, req1, req2, "gateway_requested_at must be preserved across re-scans")
	assert.Len(t, gw.callsForOrder(fx.MidtransID), 1, "exactly one gateway HTTP call across both scans")
}

// ---------------------------------------------------------------------
// 3. Failure semantics — HTTP error persists failed, never confirmed
// ---------------------------------------------------------------------

func TestRec6Dispatch_Integration_GatewayFailure_NotConfirmed_Retryable(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createRec6IntentFixture(t)

	gw := newRec6RefundGateway()
	gw.setFail(fx.IdemKey, true)
	h.wireRec6Dispatcher(gw)
	w := newRec6DispatchWorker(h.refundService)

	w.ScanOnce()

	_, gwStatus, _, _, ackAt, err := h.loadRec6DispatchRefundRow(ctx, fx.RefundID)
	require.NoError(t, err)
	assert.Equal(t, "failed", gwStatus, "gateway failure must persist failed, never a confirmed state")
	assert.Nil(t, ackAt)

	var lastErr *string
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT last_gateway_error FROM refunds WHERE id = $1`, fx.RefundID).Scan(&lastErr)
	}))
	require.NotNil(t, lastErr, "failure must persist last_gateway_error")
	assert.Equal(t, int64(1), h.countRec6OutboxEvents(ctx, fx.RefundID, "money.refund_failed"))
	assert.Equal(t, int64(0), h.countRec6OutboxEvents(ctx, fx.RefundID, "money.refund_requested"))

	// Recovery: gateway healthy again; the SAME key is retried and wins.
	gw.setFail(fx.IdemKey, false)
	w.ScanOnce()

	idemKey, gwStatus2, attempts, requestedAt, ackAt2, err := h.loadRec6DispatchRefundRow(ctx, fx.RefundID)
	require.NoError(t, err)
	assert.Equal(t, "pending", gwStatus2, "retry after failure must reach REQUESTED/SUBMITTED")
	assert.Equal(t, fx.IdemKey, idemKey, "retry must reuse the same deterministic key")
	assert.GreaterOrEqual(t, attempts, 2, "retry must bump gateway_attempts")
	require.NotNil(t, requestedAt)
	assert.Nil(t, ackAt2, "still not confirmed — webhook ack territory")
	assert.Equal(t, int64(1), h.countRec6OutboxEvents(ctx, fx.RefundID, "money.refund_requested"))
}

// ---------------------------------------------------------------------
// 4. Crash recovery — claimed-but-unresolved intent is reclaimed
// ---------------------------------------------------------------------

func TestRec6Dispatch_Integration_CrashRecovery_ReclaimsStaleClaim(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createRec6IntentFixture(t)

	// Simulate a worker that crashed after claiming: pending, no requested_at,
	// updated_at older than the recovery grace.
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE refunds
			SET gateway_status = 'pending',
			    gateway_requested_at = NULL,
			    updated_at = NOW() - INTERVAL '5 minutes'
			WHERE id = $1
		`, fx.RefundID)
		return err
	}))

	gw := newRec6RefundGateway()
	h.wireRec6Dispatcher(gw)
	newRec6DispatchWorker(h.refundService).ScanOnce()

	idemKey, gwStatus, _, requestedAt, ackAt, err := h.loadRec6DispatchRefundRow(ctx, fx.RefundID)
	require.NoError(t, err)
	assert.Equal(t, "pending", gwStatus)
	require.NotNil(t, requestedAt, "reclaimed intent must be completed with gateway_requested_at")
	assert.Equal(t, fx.IdemKey, idemKey, "recovery must keep the deterministic key")
	assert.Nil(t, ackAt, "recovery must not fabricate a confirmed state")
}

// ---------------------------------------------------------------------
// 5. Succeeded terminal guard — dispatch never regresses confirmation
// ---------------------------------------------------------------------

func TestRec6Dispatch_Integration_NeverOverwritesSucceeded(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createRec6IntentFixture(t)

	// Row already confirmed by webhook (final confirmation path).
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE refunds
			SET gateway_status = 'succeeded',
			    gateway_requested_at = NOW(),
			    gateway_acknowledged_at = NOW()
			WHERE id = $1
		`, fx.RefundID)
		return err
	}))

	gw := newRec6RefundGateway()
	h.wireRec6Dispatcher(gw)
	newRec6DispatchWorker(h.refundService).ScanOnce()

	_, gwStatus, _, _, ackAt, err := h.loadRec6DispatchRefundRow(ctx, fx.RefundID)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", gwStatus, "dispatch must never regress a confirmed refund")
	require.NotNil(t, ackAt)
	assert.Len(t, gw.callsForOrder(fx.MidtransID), 0, "terminal rows must never reach the gateway")
}

// ---------------------------------------------------------------------
// 6. Concurrent scans — one dispatch effect, one intent row
// ---------------------------------------------------------------------

func TestRec6Dispatch_Integration_ConcurrentWorkers_SingleDispatch(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createRec6IntentFixture(t)

	gw := newRec6RefundGateway()
	h.wireRec6Dispatcher(gw)
	w := newRec6DispatchWorker(h.refundService)

	const n = 4
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			w.ScanOnce()
		}()
	}
	wg.Wait()

	// The claimed row can only be finalized to pending or failed — never a
	// second row, never a fabricated succeeded.
	_, gwStatus, attempts, _, ackAt, err := h.loadRec6DispatchRefundRow(ctx, fx.RefundID)
	require.NoError(t, err)
	assert.Contains(t, []string{"pending", "failed"}, gwStatus)
	assert.Equal(t, 1, attempts, "concurrent scans must produce exactly one claim")
	assert.Nil(t, ackAt)

	var refundCount int64
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM refunds WHERE gateway_idempotency_key = $1`, fx.IdemKey).Scan(&refundCount)
	}))
	assert.Equal(t, int64(1), refundCount, "concurrent dispatch must not create a second intent")
	assert.Equal(t, int64(1), h.countRec6OutboxEvents(ctx, fx.RefundID, "money.refund_requested"),
		"exactly one requested-state audit event")
}

// ---------------------------------------------------------------------
// 7. Gateway spy — RefundWithKey receives canonical key + gross amount
// ---------------------------------------------------------------------

func TestRec6Dispatch_Integration_GatewayReceivesCanonicalKeyAndAmount(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	fx := h.createRec6IntentFixture(t)

	gw := newRec6RefundGateway()
	h.wireRec6Dispatcher(gw)
	newRec6DispatchWorker(h.refundService).ScanOnce()

	calls := gw.callsForOrder(fx.MidtransID)
	require.Len(t, calls, 1, "exactly one RefundWithKey call for this intent")
	assert.Equal(t, fx.IdemKey, calls[0].RefundKey)
	assert.Equal(t, fx.GrossAmount, calls[0].Amount, "dispatch amount must equal payment.gross_amount")
	assert.NotEmpty(t, calls[0].Reason)
}
