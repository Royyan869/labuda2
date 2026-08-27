package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"go.uber.org/zap"
)

// RefundFailedAlertCreator is the minimal interface for creating alerts.
// Satisfied by *alertapp.AlertService and MockAlertService.
type RefundFailedAlertCreator interface {
	CreateAlert(
		ctx context.Context,
		alertType alertentity.AlertType,
		severity alertentity.AlertSeverity,
		entityType string,
		entityID uuid.UUID,
		message string,
		metadata alertentity.AlertMetadata,
		groupKey *string,
	) (*alertapp.CreateAlertResult, error)
}

// RefundFailedAlertHandler handles money.refund_failed outbox events by
// creating CRITICAL operator alerts in the system_alerts table.
//
// O1A: This handler closes the silent-failure gap where gateway refund
// failures were logged but had no operator-visible alert surface.
//
// IDEMPOTENCY: AlertService.CreateAlert uses dedup_key (refund_gateway_failed:refund:<refundID>)
// with a 60-minute window. Duplicate events for the same refund within the
// window update occurrence_count instead of creating new rows.
type RefundFailedAlertHandler struct {
	alertService RefundFailedAlertCreator
	log          *zap.Logger
}

// NewRefundFailedAlertHandler creates a handler that converts money.refund_failed
// events into operator-visible alerts.
func NewRefundFailedAlertHandler(alertService RefundFailedAlertCreator, log *zap.Logger) *RefundFailedAlertHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &RefundFailedAlertHandler{
		alertService: alertService,
		log:          log,
	}
}

// refundFailedPayload mirrors the shape emitted by RefundService.emitGatewayOutbox.
type refundFailedPayload struct {
	RefundID              uuid.UUID `json:"refund_id"`
	OrderID               uuid.UUID `json:"order_id"`
	GatewayStatus         string    `json:"gateway_status"`
	GatewayAttempts       int       `json:"gateway_attempts"`
	GatewayRefundID       string    `json:"gateway_refund_id"`
	GatewayIdempotencyKey string    `json:"gateway_idempotency_key"`
	Amount                int64     `json:"amount"`
	Error                 string    `json:"error"`
}

// Handle processes a money.refund_failed event and creates a CRITICAL alert.
func (h *RefundFailedAlertHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	var p refundFailedPayload
	if err := json.Unmarshal(event.Payload, &p); err != nil {
		h.log.Error("refund_failed_alert: invalid payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		// Return nil to avoid infinite retries on malformed payload.
		return nil
	}

	refundID := p.RefundID
	if refundID == uuid.Nil {
		refundID = event.AggregateID
	}

	metadata := alertentity.AlertMetadata{
		"refund_id":        p.RefundID.String(),
		"order_id":         p.OrderID.String(),
		"gateway_status":   p.GatewayStatus,
		"gateway_attempts": p.GatewayAttempts,
		"gateway_error":    p.Error,
		"amount":           p.Amount,
		"event_id":         event.ID.String(),
	}

	message := fmt.Sprintf("Gateway refund failed for order %s: %s", p.OrderID, p.Error)

	result, err := h.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeRefundGatewayFailed,
		alertentity.SeverityCritical,
		"refund",
		refundID,
		message,
		metadata,
		nil, // no group key; dedup by refund_id via dedup_key
	)
	if err != nil {
		return fmt.Errorf("refund_failed_alert: create alert: %w", err)
	}

	if result.Created {
		h.log.Warn("REFUND_GATEWAY_FAILED: operator alert created",
			zap.String("alert_id", result.Alert.ID.String()),
			zap.String("refund_id", p.RefundID.String()),
			zap.String("order_id", p.OrderID.String()),
			zap.String("gateway_error", p.Error),
		)
	} else {
		h.log.Info("refund_failed_alert: deduplicated",
			zap.String("refund_id", p.RefundID.String()),
			zap.String("reason", result.Reason),
		)
	}

	return nil
}


