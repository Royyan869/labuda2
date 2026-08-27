//go:build integration

package serverboot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/internal/config"
	paymentrepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/internal/integration/payment/reconciliation"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/testdb"
)

type lookupResponder func(orderID string) (*midtrans.NotificationPayload, int, error)

type lookupTransport struct {
	responder lookupResponder
}

func (t *lookupTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[len(parts)-1] != "status" {
		return nil, fmt.Errorf("unexpected midtrans path: %s", req.URL.Path)
	}
	orderID := parts[len(parts)-2]
	payload, statusCode, err := t.responder(orderID)
	if err != nil {
		return nil, err
	}
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	body := []byte{}
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}, nil
}

func newReconciliationMidtransClient(t *testing.T, responder lookupResponder) *midtrans.Client {
	t.Helper()

	log, err := logger.New("error", "json", "stdout")
	require.NoError(t, err)

	httpClient := &http.Client{Transport: &lookupTransport{responder: responder}}
	client := midtrans.NewClient(&config.MidtransConfig{
		ServerKey:   "test-server-key",
		ClientKey:   "test-client-key",
		Environment: "sandbox",
	}, log)
	setPrivateField(client, "httpClient", httpClient)
	return client
}

func newReconciliationHarness(t *testing.T, responder lookupResponder) *paymentSettlementHarness {
	t.Helper()

	h := newPaymentSettlementHarness(t)
	h.setWebhookGateway(newReconciliationMidtransClient(t, responder))
	return h
}

func (h *paymentSettlementHarness) reconcilePayment(ctx context.Context, paymentID uuid.UUID) (reconciliation.Result, error) {
	var payment *paymentrepo.Payment
	if err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		payment, err = h.paymentRepo.GetByID(ctx, tx, paymentID)
		return err
	}); err != nil {
		return reconciliation.Result{}, err
	}
	if payment == nil {
		return reconciliation.Result{}, fmt.Errorf("payment not found: %s", paymentID)
	}

	result := reconciliation.Result{
		PaymentID:     payment.ID,
		ReferenceType: payment.ReferenceType,
		PaymentStatus: payment.Status,
		TransactionID: payment.MidtransOrderID,
	}
	if payment.IsFailed() || payment.IsSettled() {
		result.Outcome = reconciliation.OutcomeAlreadyTerminal
		return result, nil
	}
	if h.midtransClient == nil {
		result.Outcome = reconciliation.OutcomeUncertain
		result.Notes = "midtrans client not configured"
		return result, nil
	}

	gatewayStatus, err := h.midtransClient.GetTransactionStatus(payment.MidtransOrderID)
	if err != nil {
		result.Outcome = reconciliation.OutcomeUncertain
		if strings.Contains(strings.ToLower(err.Error()), "status 404") {
			result.Notes = "not found"
		} else {
			result.Notes = err.Error()
		}
		return result, nil
	}

	result.GatewayStatus = strings.ToLower(gatewayStatus.TransactionStatus)
	result.TransactionID = gatewayStatus.TransactionID

	switch {
	case h.midtransClient.IsTransactionSuccess(gatewayStatus.TransactionStatus):
		if _, err := h.webhookService.ReplayVerifiedWebhookFromGateway(ctx, paymentID, "127.0.0.1"); err != nil {
			return result, err
		}
		result.Outcome = reconciliation.OutcomeSuccessFinalized
	case h.midtransClient.IsTransactionFailed(gatewayStatus.TransactionStatus):
		gatewayStatus.SignatureKey = h.midtransClient.BuildWebhookSignature(gatewayStatus)
		if err := h.webhookService.HandleWebhook(ctx, gatewayStatus, "127.0.0.1"); err != nil {
			return result, err
		}
		result.Outcome = reconciliation.OutcomeTerminalFailure
	case h.midtransClient.IsTransactionPending(gatewayStatus.TransactionStatus):
		result.Outcome = reconciliation.OutcomeUncertain
	default:
		result.Outcome = reconciliation.OutcomeUnsupported
	}

	return result, nil
}

func gatewayStatusPayload(orderID, transactionStatus, paymentType, transactionID, grossAmount string) *midtrans.NotificationPayload {
	return &midtrans.NotificationPayload{
		TransactionTime:   time.Now().Format(time.RFC3339),
		TransactionStatus: transactionStatus,
		TransactionID:     transactionID,
		StatusMessage:     "OK",
		StatusCode:        "200",
		PaymentType:       paymentType,
		OrderID:           orderID,
		GrossAmount:       grossAmount,
		FraudStatus:       "accept",
		Currency:          "IDR",
	}
}

func countPaymentSpendRows(ctx context.Context, tdb *testdb.TestDB, userID, paymentID uuid.UUID) (int64, error) {
	var count int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM coins_transactions
			WHERE user_id = $1
			  AND type = 'spend'
			  AND reference_type = 'payment_spend'
			  AND reference_id = $2
		`, userID, paymentID).Scan(&count)
	})
	return count, err
}

func loadOrderStatusByID(ctx context.Context, tdb *testdb.TestDB, orderID uuid.UUID) (orderentity.Status, error) {
	var status orderentity.Status
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&status)
	})
	return status, err
}

func TestPaymentReconcileAuthoritativeExpireReleasesReservation(t *testing.T) {
	ctx := context.Background()
	grossAmount := ""
	h := newReconciliationHarness(t, func(orderID string) (*midtrans.NotificationPayload, int, error) {
		return gatewayStatusPayload(orderID, string(midtrans.StatusExpire), "bank_transfer", "trx-expire", grossAmount), http.StatusOK, nil
	})

	fx := h.createSettlementFixture(t, 15000, 4000, 20000)
	grossAmount = fmt.Sprintf("%d.00", fx.Payment.GrossAmount.Int64())
	result, err := h.reconcilePayment(ctx, fx.Payment.ID)
	require.NoError(t, err)
	require.Equal(t, reconciliation.OutcomeTerminalFailure, result.Outcome)
	require.Equal(t, strings.ToLower(string(midtrans.StatusExpire)), result.GatewayStatus)

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusExpire, payment.Status)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "released", reservation.Status)

	total, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), total)

	reserved, err := loadReservedCoins(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(0), reserved)

	spendCount, err := countPaymentSpendRows(ctx, h.tdb, h.buyerID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)

	orderStatus, err := loadOrderStatusByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	require.Equal(t, orderentity.StatusExpired, orderStatus)
}

func TestPaymentReconcileTerminalFailuresReleaseReservation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status string
	}{
		{name: "deny", status: string(midtrans.StatusDeny)},
		{name: "cancel", status: string(midtrans.StatusCancel)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			grossAmount := ""
			h := newReconciliationHarness(t, func(orderID string) (*midtrans.NotificationPayload, int, error) {
				return gatewayStatusPayload(orderID, tc.status, "bank_transfer", "trx-"+tc.name, grossAmount), http.StatusOK, nil
			})

			fx := h.createSettlementFixture(t, 15000, 4000, 20000)
			grossAmount = fmt.Sprintf("%d.00", fx.Payment.GrossAmount.Int64())
			result, err := h.reconcilePayment(ctx, fx.Payment.ID)
			require.NoError(t, err)
			require.Equal(t, reconciliation.OutcomeTerminalFailure, result.Outcome)

			payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
			require.NoError(t, err)
			require.Equal(t, tc.status, payment.Status)

			reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
			require.NoError(t, err)
			require.NotNil(t, reservation)
			require.Equal(t, "released", reservation.Status)

			spendCount, err := countPaymentSpendRows(ctx, h.tdb, h.buyerID, payment.ID)
			require.NoError(t, err)
			require.Equal(t, int64(0), spendCount)
		})
	}
}

func TestPaymentReconcileKZeroFailureSkipsReservationMutation(t *testing.T) {
	ctx := context.Background()
	grossAmount := ""
	h := newReconciliationHarness(t, func(orderID string) (*midtrans.NotificationPayload, int, error) {
		return gatewayStatusPayload(orderID, string(midtrans.StatusDeny), "bank_transfer", "trx-k0", grossAmount), http.StatusOK, nil
	})

	fx := h.createSettlementFixture(t, 0, 4000, 20000)
	grossAmount = fmt.Sprintf("%d.00", fx.Payment.GrossAmount.Int64())
	result, err := h.reconcilePayment(ctx, fx.Payment.ID)
	require.NoError(t, err)
	require.Equal(t, reconciliation.OutcomeTerminalFailure, result.Outcome)

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusDeny, payment.Status)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.Nil(t, reservation)

	total, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(20000), total)

	spendCount, err := countPaymentSpendRows(ctx, h.tdb, h.buyerID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)
}

func TestPaymentReconcilePendingLookupTimeoutAndNotFoundStayUncertain(t *testing.T) {
	tests := []struct {
		name      string
		responder lookupResponder
	}{
		{
			name: "not_found",
			responder: func(orderID string) (*midtrans.NotificationPayload, int, error) {
				return nil, http.StatusNotFound, nil
			},
		},
		{
			name: "timeout",
			responder: func(orderID string) (*midtrans.NotificationPayload, int, error) {
				return nil, 0, errors.New("network timeout")
			},
		},
		{
			name: "pending",
			responder: func(orderID string) (*midtrans.NotificationPayload, int, error) {
				return gatewayStatusPayload(orderID, string(midtrans.StatusPending), "bank_transfer", "trx-pending", "15000.00"), http.StatusOK, nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			grossAmount := ""
			h := newReconciliationHarness(t, tc.responder)

			fx := h.createSettlementFixture(t, 15000, 4000, 20000)
			grossAmount = fmt.Sprintf("%d.00", fx.Payment.GrossAmount.Int64())
			if tc.name == "pending" {
				h.setWebhookGateway(newReconciliationMidtransClient(t, func(orderID string) (*midtrans.NotificationPayload, int, error) {
					return gatewayStatusPayload(orderID, string(midtrans.StatusPending), "bank_transfer", "trx-pending", grossAmount), http.StatusOK, nil
				}))
			}
			result, err := h.reconcilePayment(ctx, fx.Payment.ID)
			require.NoError(t, err)
			require.Equal(t, reconciliation.OutcomeUncertain, result.Outcome)

			payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
			require.NoError(t, err)
			require.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)

			reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
			require.NoError(t, err)
			require.NotNil(t, reservation)
			require.Equal(t, "reserved", reservation.Status)
		})
	}
}

func TestPaymentReconcileSettlementConsumesReservationExactlyOnce(t *testing.T) {
	ctx := context.Background()
	grossAmount := ""
	h := newReconciliationHarness(t, func(orderID string) (*midtrans.NotificationPayload, int, error) {
		return gatewayStatusPayload(orderID, string(midtrans.StatusSettlement), "bank_transfer", "trx-settle", grossAmount), http.StatusOK, nil
	})

	fx := h.createSettlementFixture(t, 15000, 4000, 20000)
	grossAmount = fmt.Sprintf("%d.00", fx.Payment.GrossAmount.Int64())
	result, err := h.reconcilePayment(ctx, fx.Payment.ID)
	require.NoError(t, err)
	require.Equal(t, reconciliation.OutcomeSuccessFinalized, result.Outcome)

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusSettlement, payment.Status)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "consumed", reservation.Status)

	spendCount, err := countPaymentSpendRows(ctx, h.tdb, h.buyerID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), spendCount)

	total, err := loadUserCoinBalance(ctx, h.tdb, h.buyerID)
	require.NoError(t, err)
	require.Equal(t, int64(5000), total)

	orderStatus, err := loadOrderStatusByID(ctx, h.tdb, fx.OrderID)
	require.NoError(t, err)
	require.Equal(t, orderentity.StatusPaid, orderStatus)
}

func TestPaymentReconcileDuplicateTerminalFailure_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	grossAmount := ""
	h := newReconciliationHarness(t, func(orderID string) (*midtrans.NotificationPayload, int, error) {
		return gatewayStatusPayload(orderID, string(midtrans.StatusExpire), "bank_transfer", "trx-expire-idem", grossAmount), http.StatusOK, nil
	})

	fx := h.createSettlementFixture(t, 15000, 4000, 20000)
	grossAmount = fmt.Sprintf("%d.00", fx.Payment.GrossAmount.Int64())
	first, err := h.reconcilePayment(ctx, fx.Payment.ID)
	require.NoError(t, err)
	require.Equal(t, reconciliation.OutcomeTerminalFailure, first.Outcome)

	second, err := h.reconcilePayment(ctx, fx.Payment.ID)
	require.NoError(t, err)
	require.Equal(t, reconciliation.OutcomeAlreadyTerminal, second.Outcome)

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusExpire, payment.Status)

	reservation, err := loadReservationByPaymentID(ctx, h.tdb, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.Equal(t, "released", reservation.Status)

	spendCount, err := countPaymentSpendRows(ctx, h.tdb, h.buyerID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), spendCount)
}

func TestPaymentReconcileProviderNotFoundIsUncertain(t *testing.T) {
	ctx := context.Background()
	h := newReconciliationHarness(t, func(orderID string) (*midtrans.NotificationPayload, int, error) {
		return nil, http.StatusNotFound, nil
	})

	fx := h.createSettlementFixture(t, 15000, 4000, 20000)
	result, err := h.reconcilePayment(ctx, fx.Payment.ID)
	require.NoError(t, err)
	require.Equal(t, reconciliation.OutcomeUncertain, result.Outcome)
	require.Contains(t, strings.ToLower(result.Notes), "not found")

	payment, err := loadPaymentSnapshotByMidtransOrderID(ctx, h.tdb, fx.MidtransID)
	require.NoError(t, err)
	require.Equal(t, paymentrepo.PaymentStatusPending, payment.Status)
}
