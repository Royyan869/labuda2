// Tests for PASS_18T: a Midtrans webhook reporting a successful gateway
// transaction (settlement/capture) for a payment PaymentExpiryWorker has
// already expired must never be silently labeled "succeeded". These tests
// exercise recordCapturedAfterExpiry directly (it only touches tx.Exec, so a
// minimal fake Tx suffices — no real Postgres required) plus structural
// regression tests proving the fix is wired into handleWebhookInTransaction
// at the right point and never attempts to recover the order to paid.
package application

import (
	"bufio"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/money"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeExecTx is a minimal db.Tx fake that only supports Exec, which is all
// updateWebhookEventStatus (and therefore recordCapturedAfterExpiry) needs.
// Embedding a nil db.Tx is safe because no other method is ever called.
type fakeExecTx struct {
	db.Tx
	execCalls []fakeExecCall
}

type fakeExecCall struct {
	sql  string
	args []any
}

func (f *fakeExecTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execCalls = append(f.execCalls, fakeExecCall{sql: sql, args: args})
	return pgconn.CommandTag{}, nil
}

// fakeAlertService records CreateAlert calls without touching the DB.
type fakeAlertService struct {
	calls []fakeAlertCall
	err   error
}

type fakeAlertCall struct {
	alertType  alertentity.AlertType
	severity   alertentity.AlertSeverity
	entityType string
	entityID   uuid.UUID
	message    string
	metadata   alertentity.AlertMetadata
	groupKey   *string
}

func (f *fakeAlertService) CreateAlert(
	ctx context.Context,
	alertType alertentity.AlertType,
	severity alertentity.AlertSeverity,
	entityType string,
	entityID uuid.UUID,
	message string,
	metadata alertentity.AlertMetadata,
	groupKey *string,
) (*alertapp.CreateAlertResult, error) {
	f.calls = append(f.calls, fakeAlertCall{
		alertType:  alertType,
		severity:   severity,
		entityType: entityType,
		entityID:   entityID,
		message:    message,
		metadata:   metadata,
		groupKey:   groupKey,
	})
	if f.err != nil {
		return nil, f.err
	}
	return &alertapp.CreateAlertResult{Created: true}, nil
}

func testExpiredPayment() *repository.Payment {
	referenceID := uuid.New()
	return &repository.Payment{
		ID:            uuid.New(),
		Status:        repository.PaymentStatusExpire,
		ReferenceType: repository.ReferenceTypeOrder,
		ReferenceID:   &referenceID,
		GrossAmount:   money.New(103000),
	}
}

func testSettlementNotification() *midtrans.NotificationPayload {
	return &midtrans.NotificationPayload{
		TransactionID:     "trx-late-1",
		OrderID:           "LAB-ORDER-LATE-1",
		TransactionStatus: "settlement",
		GrossAmount:       "103000.00",
	}
}

// TestRecordCapturedAfterExpiry_NeverMarksSucceeded is the PASS_18T core
// regression test: the webhook event must be recorded with the distinct
// captured_after_expiry status, never "succeeded" (which would falsely imply
// the platform reconciled the money).
func TestRecordCapturedAfterExpiry_NeverMarksSucceeded(t *testing.T) {
	svc := &PaymentWebhookService{log: zap.NewNop()}
	tx := &fakeExecTx{}
	payment := testExpiredPayment()

	svc.recordCapturedAfterExpiry(context.Background(), tx, "evt-1", payment, testSettlementNotification())

	require.Len(t, tx.execCalls, 1, "expected exactly one status-update Exec call")
	call := tx.execCalls[0]
	require.GreaterOrEqual(t, len(call.args), 1)
	status, ok := call.args[0].(string)
	require.True(t, ok, "first arg to updateWebhookEventStatus query must be the status string")
	assert.Equal(t, repository.PaymentWebhookEventStatusCapturedAfterExpiry, status)
	assert.NotEqual(t, "succeeded", status)
}

// TestRecordCapturedAfterExpiry_CreatesCriticalAlert proves an operator alert
// is raised with the correct type/severity/entity so the unreconciled capture
// is visible rather than only logged.
func TestRecordCapturedAfterExpiry_CreatesCriticalAlert(t *testing.T) {
	svc := &PaymentWebhookService{log: zap.NewNop()}
	alerts := &fakeAlertService{}
	svc.SetAlertService(alerts)
	tx := &fakeExecTx{}
	payment := testExpiredPayment()

	svc.recordCapturedAfterExpiry(context.Background(), tx, "evt-1", payment, testSettlementNotification())

	require.Len(t, alerts.calls, 1, "expected exactly one alert to be raised")
	call := alerts.calls[0]
	assert.Equal(t, alertentity.AlertTypePaymentCapturedAfterExpiry, call.alertType)
	assert.Equal(t, alertentity.SeverityCritical, call.severity)
	assert.Equal(t, payment.ID, call.entityID)
	require.NotNil(t, call.groupKey, "groupKey must be set so retries dedup instead of paging repeatedly")
	assert.Contains(t, *call.groupKey, payment.ID.String())
}

// TestRecordCapturedAfterExpiry_AlertServiceNil_StillRecordsEvent proves the
// durable webhook-event record does not depend on the alert service being
// wired — losing the alert sink must not also lose the durable signal.
func TestRecordCapturedAfterExpiry_AlertServiceNil_StillRecordsEvent(t *testing.T) {
	svc := &PaymentWebhookService{log: zap.NewNop()} // alertService left nil
	tx := &fakeExecTx{}
	payment := testExpiredPayment()

	assert.NotPanics(t, func() {
		svc.recordCapturedAfterExpiry(context.Background(), tx, "evt-1", payment, testSettlementNotification())
	})

	require.Len(t, tx.execCalls, 1)
	status, _ := tx.execCalls[0].args[0].(string)
	assert.Equal(t, repository.PaymentWebhookEventStatusCapturedAfterExpiry, status)
}

// TestRecordCapturedAfterExpiry_GroupKeyStableAcrossRetries proves repeated
// late notifications for the SAME payment produce the SAME alert group key,
// which is what lets AlertService's dedup window collapse retries into one
// alert instead of paging on every redelivery.
func TestRecordCapturedAfterExpiry_GroupKeyStableAcrossRetries(t *testing.T) {
	svc := &PaymentWebhookService{log: zap.NewNop()}
	alerts := &fakeAlertService{}
	svc.SetAlertService(alerts)
	payment := testExpiredPayment()

	svc.recordCapturedAfterExpiry(context.Background(), &fakeExecTx{}, "evt-1", payment, testSettlementNotification())
	svc.recordCapturedAfterExpiry(context.Background(), &fakeExecTx{}, "evt-2", payment, testSettlementNotification())

	require.Len(t, alerts.calls, 2)
	require.NotNil(t, alerts.calls[0].groupKey)
	require.NotNil(t, alerts.calls[1].groupKey)
	assert.Equal(t, *alerts.calls[0].groupKey, *alerts.calls[1].groupKey)
}

// --- Structural regression tests (mirror the existing structural test style
// in payment_webhook_test.go — this codebase tests transaction-embedded
// control flow by source-scanning rather than mocking the whole tx chain) ---

func readWebhookSource(t *testing.T) []string {
	t.Helper()
	f, err := os.Open("payment_webhook.go")
	require.NoError(t, err)
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	require.NoError(t, scanner.Err())
	return lines
}

// TestStep7ChecksExpiredCaptureBeforeGenericSucceeded proves the
// captured-after-expiry branch is checked INSIDE the "!payment.IsPending()"
// block and BEFORE the generic "mark succeeded, already processed" line —
// otherwise the old silent-success bug reappears for expired payments.
func TestStep7ChecksExpiredCaptureBeforeGenericSucceeded(t *testing.T) {
	lines := readWebhookSource(t)

	step7Idx, expiredCheckIdx, genericSucceededIdx := -1, -1, -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "STEP 7:") {
			step7Idx = i
		}
		if step7Idx != -1 && expiredCheckIdx == -1 && strings.Contains(trimmed, "payment.IsExpired()") {
			expiredCheckIdx = i
		}
		if step7Idx != -1 && strings.Contains(trimmed, `updateWebhookEventStatus(ctx, tx, eventID, "succeeded", &payment.ID, nil)`) {
			genericSucceededIdx = i
			break
		}
	}

	require.NotEqual(t, -1, step7Idx, "STEP 7 block not found")
	require.NotEqual(t, -1, expiredCheckIdx, "payment.IsExpired() check not found in STEP 7 — captured-after-expiry fix missing")
	require.NotEqual(t, -1, genericSucceededIdx, "generic already-processed succeeded line not found")
	assert.Less(t, expiredCheckIdx, genericSucceededIdx,
		"payment.IsExpired() branch must be checked BEFORE the generic succeeded fallback, or expired payments silently fall through to it again")
}

// TestRecordCapturedAfterExpiryNeverCallsFinalization proves the recovery
// path does not attempt to settle/finalize the expired order. Doctrine: the
// canonical state machine (SettlePaymentByID) already hard-blocks settling an
// expired order, and this pass must not bypass that guard.
func TestRecordCapturedAfterExpiryNeverCallsFinalization(t *testing.T) {
	lines := readWebhookSource(t)

	inFunc := false
	found := false
	for _, line := range lines {
		if strings.Contains(line, "func (s *PaymentWebhookService) recordCapturedAfterExpiry(") {
			inFunc = true
			found = true
			continue
		}
		if inFunc {
			if strings.HasPrefix(line, "}") {
				break
			}
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, "canonicalFinalizationService") || strings.Contains(trimmed, "FinalizeOrderPayment") {
				t.Fatalf("recordCapturedAfterExpiry must never call canonical finalization: %q", trimmed)
			}
			if strings.Contains(trimmed, "SettlePayment") {
				t.Fatalf("recordCapturedAfterExpiry must never call settlement: %q", trimmed)
			}
		}
	}
	require.True(t, found, "recordCapturedAfterExpiry function not found")
}

// TestRecordCapturedAfterExpiryNoScaling is the PASS_18T money-doctrine
// regression test: no /100 or *100 scaling anywhere in the new function.
func TestRecordCapturedAfterExpiryNoScaling(t *testing.T) {
	lines := readWebhookSource(t)
	inFunc := false
	for _, line := range lines {
		if strings.Contains(line, "func (s *PaymentWebhookService) recordCapturedAfterExpiry(") {
			inFunc = true
			continue
		}
		if inFunc {
			if strings.HasPrefix(line, "}") {
				break
			}
			if strings.Contains(line, "/ 100") || strings.Contains(line, "/100") ||
				strings.Contains(line, "* 100") || strings.Contains(line, "*100") {
				t.Fatalf("recordCapturedAfterExpiry must not scale amounts: %q", line)
			}
		}
	}
	require.True(t, inFunc, "recordCapturedAfterExpiry function not found")
}

// TestPaymentIsExpired locks the entity-level helper this fix depends on.
func TestPaymentIsExpired(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{repository.PaymentStatusExpire, true},
		{repository.PaymentStatusPending, false},
		{repository.PaymentStatusSettlement, false},
		{repository.PaymentStatusCapture, false},
		{repository.PaymentStatusDeny, false},
		{repository.PaymentStatusCancel, false},
	}
	for _, tt := range tests {
		p := &repository.Payment{Status: tt.status}
		assert.Equal(t, tt.expected, p.IsExpired(), "status=%s", tt.status)
	}
}

// TestSetAlertService_WiresField mirrors the existing SetPromotionService/
// SetCanonicalFinalizationService setter test style.
func TestSetAlertService_WiresField(t *testing.T) {
	svc := &PaymentWebhookService{}
	assert.Nil(t, svc.alertService)

	svc.SetAlertService(&fakeAlertService{})
	assert.NotNil(t, svc.alertService, "SetAlertService must wire the field")
}
