package worker

import (
	"context"

	"github.com/labuda/backend/internal/pricing/promotion/application"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// promotionEventHandlerWrapper wraps the promotion domain's event handler
// to avoid circular import between worker and promotion packages.
type promotionEventHandlerWrapper struct {
	handler *application.PromotionEventHandler
}

// NewPromotionEventHandlerWrapper creates a new wrapper for the promotion event handler.
// This function is called by SetupPromotionHandlers in outbox_worker.go.
func NewPromotionEventHandlerWrapper(dbConn *db.DB, promotionService interface{}, log *zap.Logger) EventHandler {
	// Type assert to *application.PromotionService
	ps, ok := promotionService.(*application.PromotionService)
	if !ok {
		// If type assertion fails, return a no-op handler to prevent crashes
		return &promotionNoOpHandler{log: log}
	}

	handler := application.NewPromotionEventHandler(ps, dbConn, log)
	return &promotionEventHandlerWrapper{handler: handler}
}

// Handle forwards the event to the promotion domain handler.
func (w *promotionEventHandlerWrapper) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	// Both worker and promotion handlers now use platformevent.OutboxEvent
	return w.handler.Handle(ctx, event)
}

// promotionNoOpHandler is a fallback handler that safely ignores events.
// This is used if the PromotionService type assertion fails.
type promotionNoOpHandler struct {
	log *zap.Logger
}

func (h *promotionNoOpHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	h.log.Debug("PromotionEventHandler in no-op mode (service not available)",
		zap.String("event_type", event.EventType),
		zap.String("event_id", event.ID.String()),
	)
	return nil
}


