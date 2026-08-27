package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/finance/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestPayoutWebhookHandler constructs a handler with the given secret key.
// withdrawRepo/db/outboxRepo are left nil — safe because every scenario
// exercised in this file is rejected by signature verification (STEP 1-3)
// before the handler ever touches those dependencies.
func newTestPayoutWebhookHandler(secretKey string) *PayoutWebhookHandler {
	return NewPayoutWebhookHandlerWithConfig(nil, nil, secretKey, nil, "midtrans_payout", false, false, nil)
}

func performPayoutWebhookRequest(t *testing.T, h *PayoutWebhookHandler, payload []byte, signatureHeader string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/payout", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if signatureHeader != "" {
		req.Header.Set(WebhookSignatureHeader, signatureHeader)
	}
	c.Request = req

	h.HandlePayoutWebhook(c)
	return rec
}

// TestHandlePayoutWebhook_MissingSignatureHeader_Rejected is the PASS_18S
// regression test proving the fail-closed behavior stays intact: a request
// with no X-Webhook-Signature header at all must be rejected regardless of
// whether a secret is configured.
func TestHandlePayoutWebhook_MissingSignatureHeader_Rejected(t *testing.T) {
	h := newTestPayoutWebhookHandler("configured-secret")

	rec := performPayoutWebhookRequest(t, h, []byte(`{}`), "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "missing signature header must be rejected")
}

// TestHandlePayoutWebhook_EmptySecretConfigured_Rejected proves an empty
// PAYOUT_SECRET_KEY is never treated as "signature verification not
// required" — it must be treated as "cannot accept callbacks", even if the
// caller supplies a signature header.
func TestHandlePayoutWebhook_EmptySecretConfigured_Rejected(t *testing.T) {
	h := newTestPayoutWebhookHandler("")

	rec := performPayoutWebhookRequest(t, h, []byte(`{}`), "sha256=deadbeef")

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "empty secret must reject even a present signature header")
	require.NotNil(t, h.verifier)
	assert.Empty(t, h.verifier.SecretKey)
}

// TestHandlePayoutWebhook_BadSignature_Rejected proves a syntactically
// well-formed but incorrect signature (wrong secret / tampered payload) is
// rejected, not silently accepted.
func TestHandlePayoutWebhook_BadSignature_Rejected(t *testing.T) {
	h := newTestPayoutWebhookHandler("configured-secret")
	payload := []byte(`{"external_reference_id":"WD_1","status":"SUCCESS"}`)

	// Signature computed with the WRONG secret.
	badSignature := worker.GenerateSignature(payload, "wrong-secret")

	rec := performPayoutWebhookRequest(t, h, payload, badSignature)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "signature computed with the wrong secret must be rejected")
}

// TestHandlePayoutWebhook_TamperedPayload_Rejected proves a valid signature
// for one payload does not verify a different (tampered) payload.
func TestHandlePayoutWebhook_TamperedPayload_Rejected(t *testing.T) {
	h := newTestPayoutWebhookHandler("configured-secret")
	original := []byte(`{"external_reference_id":"WD_1","status":"SUCCESS"}`)
	tampered := []byte(`{"external_reference_id":"WD_1","status":"FAILED"}`)

	signatureForOriginal := worker.GenerateSignature(original, "configured-secret")

	rec := performPayoutWebhookRequest(t, h, tampered, signatureForOriginal)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "a signature valid for one payload must not verify a different payload")
}

// TestHandlePayoutWebhook_ValidSignature_PassesVerification proves that a
// correctly computed signature (real secret + exact payload) passes
// signature verification. The payload deliberately omits required fields
// (empty JSON object) so processing fails at STEP 4 (payload parsing) with
// 400 Bad Request rather than needing a real database — the meaningful
// assertion is that we do NOT get 401 Unauthorized, proving verification
// itself succeeded.
func TestHandlePayoutWebhook_ValidSignature_PassesVerification(t *testing.T) {
	h := newTestPayoutWebhookHandler("configured-secret")
	payload := []byte(`{}`)
	validSignature := worker.GenerateSignature(payload, "configured-secret")

	rec := performPayoutWebhookRequest(t, h, payload, validSignature)

	assert.NotEqual(t, http.StatusUnauthorized, rec.Code, "a valid signature must not be rejected as unauthorized")
	assert.Equal(t, http.StatusBadRequest, rec.Code, "empty payload should fail parsing (missing required fields), not signature verification")
}

// TestHandleHealthCheck_ReflectsSecretConfiguration proves the webhook
// handler's own health endpoint accurately reports whether signature
// verification is configured — this is the per-instance signal that feeds
// into the PASS_18S payout completion-safety picture.
func TestHandleHealthCheck_ReflectsSecretConfiguration(t *testing.T) {
	t.Run("secret configured", func(t *testing.T) {
		h := newTestPayoutWebhookHandler("configured-secret")
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/webhooks/payout/health", nil)

		h.HandleHealthCheck(c)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"signature_verification":"configured"`)
		assert.Contains(t, rec.Body.String(), `"webhook_ready_for_callbacks":true`)
	})

	t.Run("secret not configured", func(t *testing.T) {
		h := newTestPayoutWebhookHandler("")
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/webhooks/payout/health", nil)

		h.HandleHealthCheck(c)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"signature_verification":"not_configured"`)
		assert.Contains(t, rec.Body.String(), `"webhook_ready_for_callbacks":false`)
	})
}
