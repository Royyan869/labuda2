//go:build integration

// REC-1: PAYMENT WEBHOOK FAILURE DURABILITY — HTTP boundary proof.
//
// WHY THIS FILE IS IN THE EXTERNAL application_test PACKAGE: the in-package test
// variant (package application) may not import delivery/http, because that
// package imports application — Go rejects the import cycle. The external test
// package is the only place where the real Gin handler and the real webhook
// service can be driven together.
//
// What it proves: durable failure recording is INDEPENDENT of the webhook HTTP
// response. The handler keeps answering 200 to a delivery whose processing
// failed, exactly as before REC-1, so gateway retry behaviour is unchanged; the
// durable record is written by a separate transaction in the service, not by the
// response path. That is why REC-1 did not need to change the response status.
//
// The failure exercised here is the signature gate — the only failure class that
// reaches the handler without a payment fixture — and it is also the class that
// must persist nothing, so both properties are asserted at the boundary.
//
// Run with: go test -tags integration ./internal/integration/payment/application/...
package application_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/config"
	paymentapp "github.com/labuda/backend/internal/integration/payment/application"
	paymenthttp "github.com/labuda/backend/internal/integration/payment/delivery/http"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/testdb"
)

func TestWebhookFailureDurabilityHandler_Acks200WithoutPersistingUnverifiedInput(t *testing.T) {
	ctx := context.Background()

	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	log, err := logger.NewDevelopment()
	require.NoError(t, err, "logger init")

	// A distinctive, non-secret server key so a valid signature is impossible
	// for the payload built below.
	client := midtrans.NewClient(&config.MidtransConfig{
		Environment: "sandbox",
		ServerKey:   "SB-Mid-server-REC1-HANDLER-DO-NOT-PERSIST",
		ClientKey:   "SB-Mid-client-REC1",
	}, log)

	svc := paymentapp.NewPaymentWebhookService(db.NewFromPool(testDB.Pool()), client, nil, nil, zap.NewNop())

	gin.SetMode(gin.TestMode)
	handler := paymenthttp.NewPaymentWebhookHandler(svc, zap.NewNop())

	const eventID = "rec1-handler-200"
	notification := &midtrans.NotificationPayload{
		TransactionID:     eventID,
		OrderID:           "LAB-REC1-HANDLER-ORDER",
		TransactionStatus: string(midtrans.StatusSettlement),
		StatusCode:        "200",
		GrossAmount:       "10000.00",
		PaymentType:       "bank_transfer",
		Currency:          "IDR",
		SignatureKey:      "not-a-valid-signature",
	}
	require.False(t, client.VerifySignature(notification), "precondition: the signature must not verify")

	body, err := json.Marshal(notification)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/webhooks/payment/midtrans", strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMidtransWebhook(c)

	assert.Equal(t, http.StatusOK, w.Code,
		"the webhook handler must keep acknowledging a failed delivery with 200; REC-1 must not change gateway retry behaviour")

	var count int
	require.NoError(t, testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT count(*) FROM payment_webhook_events WHERE notification_key = $1`, midtrans.NotificationIdentity(notification)).Scan(&count)
	}))
	assert.Equal(t, 0, count,
		"an unverified delivery must persist nothing, even though the handler acked it with 200")

	// The failure record must also be absent, i.e. the handler's 200 did not come
	// from a successful settlement either.
	var succeeded int
	require.NoError(t, testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT count(*) FROM payment_webhook_events WHERE notification_key = $1 AND status = 'succeeded'`, midtrans.NotificationIdentity(notification)).Scan(&succeeded)
	}))
	assert.Equal(t, 0, succeeded, "an unverified delivery must never be recorded as a successful processing outcome")
}
