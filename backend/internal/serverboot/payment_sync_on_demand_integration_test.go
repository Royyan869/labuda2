//go:build integration

package serverboot

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	paymentrepo "github.com/hishumi/backend/internal/integration/payment/infrastructure/repository"
	"github.com/hishumi/backend/internal/worker"
	"github.com/hishumi/backend/pkg/midtrans"
)

// ---------------------------------------------------------------------------
// PAYMENT_SYNC_ON_DEMAND — POST /payments/:id/sync pipeline proofs.
//
// The on-demand sync MUST run the same canonical discovery → inquiry →
// settle → domain-finalization pipeline as the scan loop, but WITHOUT the
// 10-minute inquiry-eligibility age: the payer explicitly asked for a status
// check. These tests pin:
//
//	SYNC-1  fresh pending subscription payment + gateway settlement →
//	        payment settled, canonical subscription processor invoked,
//	        (settled, mutated=true) — with created_at = NOW (no age backdate).
//	SYNC-2  gateway still pending → row stays pending, (pending, mutated=false).
//	SYNC-3  already-settled row → short-circuits WITHOUT a gateway call,
//	        (settled, mutated=false) — no duplicate financial effect.
//	SYNC-4  unknown payment id → worker.ErrPaymentNotFound (HTTP 404 mapping).
// ---------------------------------------------------------------------------

func TestPaymentSync_OnDemand_Sync1_SettlesFreshPendingSubscription(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	fx := h.createSubscriptionDiscoveryFixture(t)

	gw := newIntegrationGateway()
	gw.setSuccess(fx.MidtransID, h.settlementPayload(fx))

	subProcessor := &recordingSubscriptionProcessor{tdb: h.tdb}
	w := h.newDiscoveryWorker(gw, h.finalizer, subProcessor)

	// NOTE: no makeDiscoveryEligible — created_at is NOW, which would fail the
	// scan-loop eligibility predicate. The on-demand path must not care.
	state, mutated, err := w.SyncPaymentByID(context.Background(), fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, midtrans.ProviderStateSettled, state)
	assert.True(t, mutated)

	payment, err := h.loadPaymentByID(context.Background(), fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)
	assert.Equal(t, int32(1), subProcessor.callCount.Load(),
		"canonical subscription processor invoked exactly once by the on-demand sync")

	statusAtCall, _ := subProcessor.statusAtCall.Load().(string)
	assert.Equal(t, paymentrepo.PaymentStatusSettlement, statusAtCall,
		"payment MUST already be settled when the subscription processor is invoked")
}

func TestPaymentSync_OnDemand_Sync2_GatewayPendingStaysPending(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	fx := h.createSubscriptionDiscoveryFixture(t)

	gw := newIntegrationGateway()
	gw.setSuccess(fx.MidtransID, &midtrans.NotificationPayload{
		OrderID:           fx.MidtransID,
		TransactionStatus: string(midtrans.StatusPending),
		TransactionID:     fx.TransactionID,
		GrossAmount:       "50000.00",
		PaymentType:       "bank_transfer",
	})

	subProcessor := &recordingSubscriptionProcessor{tdb: h.tdb}
	w := h.newDiscoveryWorker(gw, h.finalizer, subProcessor)

	state, mutated, err := w.SyncPaymentByID(context.Background(), fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, midtrans.ProviderStatePending, state)
	assert.False(t, mutated)
	assert.Equal(t, int32(0), subProcessor.callCount.Load(),
		"subscription processor must NOT run while the gateway has not confirmed the money")

	payment, err := h.loadPaymentByID(context.Background(), fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)
}

func TestPaymentSync_OnDemand_Sync3_AlreadySettledShortCircuits(t *testing.T) {
	h := newDiscoveryTestHarness(t)
	ctx := context.Background()
	fx := h.createDiscoveryFixture(t)

	// Settle through the canonical scan path first.
	gw := newIntegrationGateway()
	gw.setSuccess(fx.MidtransID, h.settlementPayload(fx))
	h.makeDiscoveryEligible(t, fx.Payment.ID)
	w := h.newDiscoveryWorker(gw, h.finalizer, nil)
	w.ScanOnce()

	payment, err := h.loadPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)

	gwCalledBefore := gw.called

	// On-demand sync on the terminal row: no gateway call, no mutation.
	state, mutated, err := w.SyncPaymentByID(ctx, fx.Payment.ID)
	require.NoError(t, err)
	assert.Equal(t, midtrans.ProviderStateSettled, state)
	assert.False(t, mutated)
	assert.Equal(t, gwCalledBefore, gw.called,
		"terminal payments must short-circuit without a gateway inquiry")
}

func TestPaymentSync_OnDemand_Sync4_UnknownPaymentReturnsNotFound(t *testing.T) {
	h := newDiscoveryTestHarness(t)

	gw := newIntegrationGateway()
	w := h.newDiscoveryWorker(gw, h.finalizer, nil)

	state, mutated, err := w.SyncPaymentByID(context.Background(), uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, worker.ErrPaymentNotFound),
		"unknown payment must surface the canonical not-found sentinel for the 404 mapping")
	assert.Equal(t, midtrans.ProviderState(""), state)
	assert.False(t, mutated)
}
