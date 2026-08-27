package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"go.uber.org/zap"
)

// OrderChatLinkHandler consumes order.chat_link_requested outbox events and
// performs the buyer↔seller direct-room linkage in the chat domain.
//
// This handler exists so chat_rooms mutation is OUT of the canonical order
// transaction (RUNTIME-INVARIANTS §1.2 — A transaction MUST NOT span two
// domain authorities).
//
// FAILURE SEMANTICS (RUNTIME-INVARIANTS §6.4):
//   - Eventual consistency. The order is canonical; chat linkage is UX.
//   - On error, the outbox worker retries with exponential backoff. Persistent
//     failure goes to DLQ — visible via outbox_dead_letter_total metric.
//
// REPLAY SAFETY (RUNTIME-INVARIANTS §2.5, §3.3):
//   - AutoLinkOrderToDirectRoom is idempotent end-state. UNIQUE constraint on
//     (participant_a, participant_b, room_type) prevents duplicate rooms;
//     setting linked_order_id to the same orderID twice is a no-op.
//   - A malformed payload returns nil (not an error) so the event is marked
//     succeeded rather than infinite-retried. Malformed events are doctrinally
//     unreachable (emitted by trusted internal code) but defending against
//     payload corruption avoids poison-event loops.
type OrderChatLinkHandler struct {
	chatService *chatApp.Service
	log         *zap.Logger
}

// NewOrderChatLinkHandler creates a handler for order.chat_link_requested.
func NewOrderChatLinkHandler(chatService *chatApp.Service, log *zap.Logger) *OrderChatLinkHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &OrderChatLinkHandler{
		chatService: chatService,
		log:         log,
	}
}

// OrderChatLinkRequestedPayload mirrors the payload emitted by
// OrderCreationService.emitChatLinkRequestedEvent. Field names MUST stay in
// sync with the producer.
type OrderChatLinkRequestedPayload struct {
	OrderID  uuid.UUID `json:"order_id"`
	BuyerID  uuid.UUID `json:"buyer_id"`
	SellerID uuid.UUID `json:"seller_id"`
}

// Handle processes one order.chat_link_requested event.
func (h *OrderChatLinkHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	var payload OrderChatLinkRequestedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		// Malformed payload — log and ack to avoid poison loop. This branch is
		// not reachable from current producer code.
		h.log.Error("order.chat_link_requested: invalid payload, acking to avoid poison loop",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return nil
	}

	if _, err := h.chatService.AutoLinkOrderToDirectRoom(
		ctx,
		payload.BuyerID,
		payload.SellerID,
		payload.OrderID,
	); err != nil {
		return fmt.Errorf("auto-link order to direct room: %w", err)
	}

	h.log.Debug("Order linked to direct chat room",
		zap.String("event_id", event.ID.String()),
		zap.String("order_id", payload.OrderID.String()),
	)
	return nil
}


