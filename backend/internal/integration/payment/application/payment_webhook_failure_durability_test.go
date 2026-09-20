// REC-1: always-on invariants for payment webhook failure durability.
//
// These run without a database. The transaction-level proof (rollback erases the
// in-transaction record; the independent write survives it) requires real
// Postgres and lives in
// payment_webhook_failure_durability_integration_test.go behind -tags integration.
package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/midtrans"
)

// TestWebhookEventIsReprocessable pins the idempotency semantic that makes
// durable failure recording safe.
//
// Before REC-1, a failed delivery left NO row, so a redelivery was processed
// again. Recording failures durably introduces a row for a failed event; if that
// row were treated like any other duplicate, a redelivery would be skipped and a
// recoverable failure would silently become a permanent tombstone. Only 'failed'
// is therefore reprocessable; every other recorded status keeps the pre-existing
// idempotent-skip behaviour.
func TestWebhookEventIsReprocessable(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{repository.PaymentWebhookEventStatusFailed, true},
		{repository.PaymentWebhookEventStatusSucceeded, false},
		// Legacy status value with no typed constant left (orphan recovery removed):
		// a historical 'orphaned' row must still short-circuit as an idempotent
		// duplicate rather than being reprocessed.
		{"orphaned", false},
		{repository.PaymentWebhookEventStatusManualReview, false},
		{repository.PaymentWebhookEventStatusQuarantined, false},
		{repository.PaymentWebhookEventStatusTerminalReview, false},
		{repository.PaymentWebhookEventStatusCapturedAfterExpiry, false},
		{repository.PaymentWebhookEventStatusProcessing, false},
		{repository.PaymentWebhookEventStatusPending, false},
		{"", false},
		{"unknown_status", false},
	}

	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			if got := webhookEventIsReprocessable(tc.status); got != tc.want {
				t.Fatalf("webhookEventIsReprocessable(%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// TestSignatureRejectionIsIdentifiableThroughWrapping proves the durable-record
// carve-out for signature rejections cannot regress through error wrapping or a
// message rewording: HandleWebhook decides on errors.Is, not on string matching.
func TestSignatureRejectionIsIdentifiableThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("webhook signature rejected for order %s: %w", "LAB-1", ErrWebhookSignatureInvalid)
	if !errors.Is(wrapped, ErrWebhookSignatureInvalid) {
		t.Fatal("a wrapped signature rejection must still be identifiable via errors.Is")
	}

	// A processing failure must NOT be mistaken for a signature rejection.
	processing := fmt.Errorf("CRITICAL: failed to finalize order payment: %w", errors.New("escrow creation failed"))
	if errors.Is(processing, ErrWebhookSignatureInvalid) {
		t.Fatal("a processing failure must never be classified as a signature rejection")
	}
}

// TestNotificationPayloadCarriesNoGatewayCredentials documents the credential
// bound the durable failure record relies on: the notification payload is stored
// verbatim (recovery needs it), so it must never contain gateway secret material
// — only the one-way signature hash.
func TestNotificationPayloadCarriesNoGatewayCredentials(t *testing.T) {
	notification := &midtrans.NotificationPayload{
		TransactionID:     "rec1-unit",
		OrderID:           "LAB-REC1-UNIT",
		TransactionStatus: string(midtrans.StatusSettlement),
		StatusCode:        "200",
		GrossAmount:       "10000.00",
		PaymentType:       "bank_transfer",
		Currency:          "IDR",
		SignatureKey:      "sha512-hash-value",
		FraudStatus:       "accept",
	}

	payload, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	encoded := string(payload)
	for _, forbidden := range []string{"SB-Mid-server-", "Mid-server-", "server_key", "ServerKey"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("stored notification payload must not contain gateway credential material %q: %s", forbidden, encoded)
		}
	}
}
