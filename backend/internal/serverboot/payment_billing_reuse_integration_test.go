//go:build integration

package serverboot

import (
	"context"
	"testing"

	"github.com/google/uuid"
	billingentity "github.com/hishumi/backend/internal/finance/billing/entity"
	billingrepo "github.com/hishumi/backend/internal/finance/billing/infrastructure/repository"
	paymentrepo "github.com/hishumi/backend/internal/integration/payment/infrastructure/repository"
	"github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/midtrans"
	"github.com/hishumi/backend/pkg/money"
	"github.com/stretchr/testify/require"
)

// seedPendingBilling inserts a pending promote_balance_top_up billing row for
// the harness buyer and returns its ID. It uses the canonical billing authority
// so no test-only architecture is introduced.
func seedPendingBilling(t *testing.T, h *paymentIntentHarness, amount int64) uuid.UUID {
	t.Helper()
	bt, err := billingentity.NewBillingTransaction(
		h.buyerID,
		uuid.New(),
		billingentity.TypePromoteBalanceTopUp,
		money.New(amount),
		0,
	)
	require.NoError(t, err)
	require.NoError(t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return billingrepo.NewBillingRepository().CreateBillingTransaction(context.Background(), tx, bt)
	}))
	return bt.ID
}

// TestInitiateBillingPayment_SameMethodReusesPending is F1 proof #1: a pending
// billing payment created for the SAME method is still reused idempotently, and
// its persisted method identity equals the requested method.
func TestInitiateBillingPayment_SameMethodReusesPending(t *testing.T) {
	gateway := &recordingSnapGateway{}
	h := newPaymentIntentHarness(t, 0, gateway, nil)
	ctx := context.Background()

	billingID := seedPendingBilling(t, h, 20_000)

	first, err := h.handler.InitiateBillingPayment(ctx, h.buyerID, billingID, "bank_transfer")
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := h.handler.InitiateBillingPayment(ctx, h.buyerID, billingID, "bank_transfer")
	require.NoError(t, err)

	require.Equal(t, first.PaymentID, second.PaymentID, "same method must reuse the pending payment")
	require.Equal(t, int32(1), gateway.calls, "reuse must not call Midtrans again")

	var persisted *paymentrepo.Payment
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		persisted, err = h.paymentRepo.GetByID(ctx, tx, first.PaymentID)
		return err
	}))
	require.NotNil(t, persisted.PaymentMethodCode)
	require.Equal(t, "bank_transfer", *persisted.PaymentMethodCode)
}

// TestInitiateBillingPayment_MethodSwitchSupersedesPending is F1 proof #2: a
// DIFFERENT method must never silently reuse the stale pending payment. The
// superseded attempt is terminalized and a fresh payment carries the requested
// method identity and its own Snap session.
func TestInitiateBillingPayment_MethodSwitchSupersedesPending(t *testing.T) {
	var snapChannelCalls [][]string
	gateway := &recordingSnapGateway{inspect: func(req *midtrans.SnapRequest) error {
		snapChannelCalls = append(snapChannelCalls, append([]string(nil), req.EnabledPayments...))
		return nil
	}}
	h := newPaymentIntentHarness(t, 0, gateway, nil)
	ctx := context.Background()

	billingID := seedPendingBilling(t, h, 20_000)

	first, err := h.handler.InitiateBillingPayment(ctx, h.buyerID, billingID, "bank_transfer")
	require.NoError(t, err)

	second, err := h.handler.InitiateBillingPayment(ctx, h.buyerID, billingID, "qris")
	require.NoError(t, err)

	require.NotEqual(t, first.PaymentID, second.PaymentID, "different method must not reuse the stale payment")

	var superseded, fresh *paymentrepo.Payment
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		superseded, err = h.paymentRepo.GetByID(ctx, tx, first.PaymentID)
		if err != nil {
			return err
		}
		fresh, err = h.paymentRepo.GetByID(ctx, tx, second.PaymentID)
		return err
	}))
	require.Equal(t, paymentrepo.PaymentStatusCancel, superseded.Status, "stale pending payment must be terminalized")
	require.Equal(t, paymentrepo.PaymentStatusPending, fresh.Status)
	require.NotNil(t, fresh.PaymentMethodCode)
	require.Equal(t, "qris", *fresh.PaymentMethodCode, "persisted method must equal the requested method")

	// F — the fresh payment's Snap session uses the requested method's Midtrans
	// channels (qris → other_qris), not the superseded method's channels.
	require.Contains(t, snapChannelCalls, []string{"other_qris"})
}

// TestFindPendingBillingPayment_ExcludesTerminalRows is F2 proof #3/#4: a
// terminal billing payment is never a reuse candidate, while a coexisting
// active pending payment is selected deterministically.
func TestFindPendingBillingPayment_ExcludesTerminalRows(t *testing.T) {
	h := newPaymentIntentHarness(t, 0, &recordingSnapGateway{}, nil)
	ctx := context.Background()

	billingID := seedPendingBilling(t, h, 20_000)

	first, err := h.handler.InitiateBillingPayment(ctx, h.buyerID, billingID, "bank_transfer")
	require.NoError(t, err)

	// Terminalize the first payment (deny/cancel/expire family).
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return h.paymentRepo.MarkAsFailed(ctx, tx, first.PaymentID, paymentrepo.PaymentStatusCancel)
	}))

	// No active pending payment exists → the lookup must return a clean miss
	// rather than the terminal row.
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		got, lookupErr := h.paymentRepo.FindPendingBillingPayment(ctx, tx, billingID)
		require.NoError(t, lookupErr)
		require.Nil(t, got, "terminal billing payment must never be a reuse candidate")
		return nil
	}))

	// Retrying after the failure creates a fresh pending payment, not a 500.
	second, err := h.handler.InitiateBillingPayment(ctx, h.buyerID, billingID, "qris")
	require.NoError(t, err)
	require.NotEqual(t, first.PaymentID, second.PaymentID)

	// With a terminal history row and one active pending row coexisting, the
	// active row is selected deterministically.
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		got, lookupErr := h.paymentRepo.FindPendingBillingPayment(ctx, tx, billingID)
		require.NoError(t, lookupErr)
		require.NotNil(t, got)
		require.Equal(t, second.PaymentID, got.ID, "the active pending row must win over terminal history")
		return nil
	}))
}
