// Package worker
//
// STATUS: LIVE — wired in serverboot/dependencies.go via SetupSupportUserReplyHandler.
package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	platformevent "github.com/labuda/backend/internal/platform/event"
)

// SupportUserReplyService defines the minimal interface for the support service
// needed by the user reply handler. This avoids importing the full support
// application package into the worker package.
type SupportUserReplyService interface {
	HandleUserReply(ctx context.Context, chatRoomID uuid.UUID, senderID uuid.UUID) error
}

// supportUserReplyHandler processes support.user_replied outbox events.
// When a user sends a message to a support chat room, this handler transitions
// the linked ticket from waiting_user → in_progress and notifies the admin.
type supportUserReplyHandler struct {
	supportService SupportUserReplyService
	log            *zap.Logger
}

// supportUserReplyPayload is the expected payload shape for support.user_replied events.
type supportUserReplyPayload struct {
	RoomID    string `json:"room_id"`
	SenderID  string `json:"sender_id"`
	MessageID string `json:"message_id"`
}

// Handle processes a support.user_replied event.
func (h *supportUserReplyHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	var p supportUserReplyPayload
	if err := json.Unmarshal(event.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal support.user_replied payload: %w", err)
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return fmt.Errorf("invalid room_id in support.user_replied: %w", err)
	}

	senderID, err := uuid.Parse(p.SenderID)
	if err != nil {
		return fmt.Errorf("invalid sender_id in support.user_replied: %w", err)
	}

	if err := h.supportService.HandleUserReply(ctx, roomID, senderID); err != nil {
		h.log.Error("support user reply handler failed",
			zap.String("room_id", p.RoomID),
			zap.String("sender_id", p.SenderID),
			zap.Error(err),
		)
		return err
	}

	h.log.Debug("support user reply processed",
		zap.String("room_id", p.RoomID),
		zap.String("sender_id", p.SenderID),
	)
	return nil
}

// SetupSupportUserReplyHandler registers the support.user_replied event handler.
//
// When a user sends a chat message in a support room, the chat service emits
// support.user_replied. This handler calls the support service to transition
// the ticket from waiting_user → in_progress and notify the assigned admin.
//
// Failure semantics: eventual consistency. A failed transition does NOT block
// the user's message (which was already committed by chat SendMessage).
func (w *OutboxWorker) SetupSupportUserReplyHandler(
	supportService SupportUserReplyService,
) *OutboxWorker {
	handler := &supportUserReplyHandler{
		supportService: supportService,
		log:            w.log,
	}

	w.dispatcher.Register("support.user_replied", handler)
	w.log.Info("Support user reply handler registered (ticket status transition)")
	return w
}


