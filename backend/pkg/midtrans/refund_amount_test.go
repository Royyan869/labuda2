package midtrans

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
)

// capturingTransport records the outgoing request body instead of making a
// real network call, and returns a canned successful refund response.
type capturingTransport struct {
	capturedBody []byte
}

func (t *capturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		t.capturedBody = body
	}
	respBody, _ := json.Marshal(RefundResponse{
		StatusCode:    "200",
		StatusMessage: "Success, refund transaction is successful",
		OrderID:       "LAB-refund-test",
	})
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
		Header:     make(http.Header),
	}, nil
}

func newTestClientWithCapture() (*Client, *capturingTransport) {
	transport := &capturingTransport{}
	client := &Client{
		serverKey:    "test-server-key",
		isProduction: false,
		httpClient:   &http.Client{Transport: transport},
		cb:           newCircuitBreaker(),
		log:          &logger.Logger{Logger: zap.NewNop()},
	}
	return client, transport
}

// TestRefund_SendsAmountAsRupiahIntegerNoConversion locks the PASS_18J fix:
// Refund must send the caller's amount to Midtrans unscaled — a Rp100,000
// refund produces an outgoing gross amount of 100000, never 1000.
func TestRefund_SendsAmountAsRupiahIntegerNoConversion(t *testing.T) {
	client, transport := newTestClientWithCapture()

	if err := client.Refund(context.TODO(), "LAB-order-1", 100_000, "buyer requested refund"); err != nil {
		t.Fatalf("Refund returned error: %v", err)
	}

	var sent RefundRequest
	if err := json.Unmarshal(transport.capturedBody, &sent); err != nil {
		t.Fatalf("failed to unmarshal captured request body: %v", err)
	}
	if sent.Amount != 100000 {
		t.Errorf("outgoing refund amount: want 100000, got %v", sent.Amount)
	}
}

// TestRefundWithKey_SendsAmountAsRupiahIntegerNoConversion mirrors the above
// for the idempotency-key-aware refund path used by the canonical refund
// orchestration pipeline.
func TestRefundWithKey_SendsAmountAsRupiahIntegerNoConversion(t *testing.T) {
	client, transport := newTestClientWithCapture()

	resp, err := client.RefundWithKey(context.TODO(), "LAB-order-2", "refund-key-1", 100_000, "buyer requested refund")
	if err != nil {
		t.Fatalf("RefundWithKey returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("RefundWithKey returned nil response")
	}

	var sent RefundRequest
	if err := json.Unmarshal(transport.capturedBody, &sent); err != nil {
		t.Fatalf("failed to unmarshal captured request body: %v", err)
	}
	if sent.Amount != 100000 {
		t.Errorf("outgoing refund amount: want 100000, got %v", sent.Amount)
	}
	if sent.RefundKey != "refund-key-1" {
		t.Errorf("outgoing refund_key: want refund-key-1, got %v", sent.RefundKey)
	}
}
