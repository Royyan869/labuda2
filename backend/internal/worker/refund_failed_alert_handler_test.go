package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"go.uber.org/zap/zaptest"
)

// TestRefundFailedAlertHandler_CreatesAlert verifies that a money.refund_failed
// event produces a CRITICAL alert row in the system_alerts table.
func TestRefundFailedAlertHandler_CreatesAlert(t *testing.T) {
	mock := NewMockAlertService()
	handler := NewRefundFailedAlertHandler(mock, zaptest.NewLogger(t))

	refundID := uuid.New()
	orderID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id":               refundID,
		"order_id":                orderID,
		"gateway_status":          "failed",
		"gateway_attempts":        3,
		"gateway_refund_id":       "gw-ref-123",
		"gateway_idempotency_key": "idem-key-456",
		"amount":                  150000,
		"error":                   "midtrans: insufficient balance",
	})

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		AggregateID: refundID,
		EventType:   "money.refund_failed",
		Payload:     payload,
	}

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	alerts := mock.GetAlertsCreated()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	alert := alerts[0]

	// Verify alert type
	if alert.AlertType != alertentity.AlertTypeRefundGatewayFailed {
		t.Errorf("AlertType = %q, want %q", alert.AlertType, alertentity.AlertTypeRefundGatewayFailed)
	}

	// Verify severity is CRITICAL
	if alert.Severity != alertentity.SeverityCritical {
		t.Errorf("Severity = %q, want %q", alert.Severity, alertentity.SeverityCritical)
	}

	// Verify entity type and ID
	if alert.EntityType != "refund" {
		t.Errorf("EntityType = %q, want %q", alert.EntityType, "refund")
	}
	if alert.EntityID != refundID {
		t.Errorf("EntityID = %s, want %s", alert.EntityID, refundID)
	}

	// Verify status is open
	if alert.Status != alertentity.StatusOpen {
		t.Errorf("Status = %q, want %q", alert.Status, alertentity.StatusOpen)
	}

	// Verify metadata contains required fields
	if alert.Metadata["refund_id"] != refundID.String() {
		t.Errorf("metadata.refund_id = %v, want %s", alert.Metadata["refund_id"], refundID)
	}
	if alert.Metadata["order_id"] != orderID.String() {
		t.Errorf("metadata.order_id = %v, want %s", alert.Metadata["order_id"], orderID)
	}
	if alert.Metadata["gateway_error"] != "midtrans: insufficient balance" {
		t.Errorf("metadata.gateway_error = %v, want %q", alert.Metadata["gateway_error"], "midtrans: insufficient balance")
	}
	if alert.Metadata["gateway_status"] != "failed" {
		t.Errorf("metadata.gateway_status = %v, want %q", alert.Metadata["gateway_status"], "failed")
	}
}

// TestRefundFailedAlertHandler_Idempotent verifies that duplicate events for
// the same refund within the dedup window update occurrence_count instead of
// creating a new alert.
func TestRefundFailedAlertHandler_Idempotent(t *testing.T) {
	mock := NewMockAlertService()
	handler := NewRefundFailedAlertHandler(mock, zaptest.NewLogger(t))

	refundID := uuid.New()
	orderID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id":        refundID,
		"order_id":         orderID,
		"gateway_status":   "failed",
		"gateway_attempts": 1,
		"amount":           100000,
		"error":            "timeout",
	})

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		AggregateID: refundID,
		EventType:   "money.refund_failed",
		Payload:     payload,
	}

	// First call — creates alert
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("first Handle: %v", err)
	}

	alerts := mock.GetAlertsCreated()
	if len(alerts) != 1 {
		t.Fatalf("after first call: expected 1 alert, got %d", len(alerts))
	}

	// Second call with different event ID but same refund — should dedup
	event.ID = uuid.New()
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("second Handle: %v", err)
	}

	alerts = mock.GetAlertsCreated()
	if len(alerts) != 1 {
		t.Fatalf("after second call: expected 1 alert (deduplicated), got %d", len(alerts))
	}
}

// TestRefundFailedAlertHandler_MalformedPayload verifies that malformed JSON
// does not cause infinite retries (returns nil).
func TestRefundFailedAlertHandler_MalformedPayload(t *testing.T) {
	mock := NewMockAlertService()
	handler := NewRefundFailedAlertHandler(mock, zaptest.NewLogger(t))

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		AggregateID: uuid.New(),
		EventType:   "money.refund_failed",
		Payload:     []byte(`{invalid json`),
	}

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("expected nil error for malformed payload, got: %v", err)
	}

	if len(mock.GetAlertsCreated()) != 0 {
		t.Error("no alert should be created for malformed payload")
	}
}

// TestRefundFailedAlertHandler_AlertServiceFailure verifies that alert service
// errors propagate (triggering outbox retry).
func TestRefundFailedAlertHandler_AlertServiceFailure(t *testing.T) {
	mock := NewMockAlertService()
	mock.SetFailure(true)
	handler := NewRefundFailedAlertHandler(mock, zaptest.NewLogger(t))

	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id":        uuid.New(),
		"order_id":         uuid.New(),
		"gateway_status":   "failed",
		"gateway_attempts": 1,
		"amount":           50000,
		"error":            "network error",
	})

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		AggregateID: uuid.New(),
		EventType:   "money.refund_failed",
		Payload:     payload,
	}

	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("expected error when alert service fails")
	}
}


