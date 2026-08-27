package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatvalidator "github.com/labuda/backend/internal/interaction/chat/attachmentvalidator"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// NegotiationEventHandler handles negotiation events for chat domain.
//
// ❗ CRITICAL IDEMPOTENCY:
// - Uses event ID as idempotency key
// - Database-level constraints prevent duplicates
// - Safe for retry without creating duplicates
//
// EVENTS HANDLED:
// - negotiation.started → send initial proposal message into chat_room_id
// - negotiation.message_sent → send proposal message into chat_room_id
//
// PASS_8A / F4: this handler no longer creates or resolves a separate
// room_type=negotiation room. Both events carry chat_room_id — the same
// direct room the buyer/seller are already chatting in and the same room
// StartNegotiation validated and persisted onto the session (Pass 7B) —
// so proposal messages are posted directly into it. This removes the
// duplicate/orphaned negotiation-only room that previously appeared
// alongside the real conversation in the room list.
//
// ❗ NO CROSS-DOMAIN WRITE:
// - Does NOT write to negotiation_sessions table
// - Only creates chat_messages (no chat_rooms creation since PASS_8A)
type NegotiationEventHandler struct {
	db          Transactor
	chatService *chatApp.Service
	log         *zap.Logger
}

// Transactor represents the ability to execute functions within transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// NewNegotiationEventHandler creates a new NegotiationEventHandler.
func NewNegotiationEventHandler(
	db Transactor,
	chatService *chatApp.Service,
	log *zap.Logger,
) *NegotiationEventHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &NegotiationEventHandler{
		db:          db,
		chatService: chatService,
		log:         log,
	}
}

// NegotiationStartedHandler implements EventHandler for negotiation.started events.
type NegotiationStartedHandler struct {
	handler *NegotiationEventHandler
}

// NewNegotiationStartedHandler creates a new negotiation.started event handler.
func NewNegotiationStartedHandler(
	db Transactor,
	chatService *chatApp.Service,
	log *zap.Logger,
) *NegotiationStartedHandler {
	return &NegotiationStartedHandler{
		handler: NewNegotiationEventHandler(db, chatService, log),
	}
}

// Handle processes the negotiation.started event.
func (h *NegotiationStartedHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	return h.handler.HandleNegotiationStarted(ctx, event.ID, event.Payload)
}

// NegotiationMessageSentHandler implements EventHandler for negotiation.message_sent events.
type NegotiationMessageSentHandler struct {
	handler *NegotiationEventHandler
}

// NewNegotiationMessageSentHandler creates a new negotiation.message_sent event handler.
func NewNegotiationMessageSentHandler(
	db Transactor,
	chatService *chatApp.Service,
	log *zap.Logger,
) *NegotiationMessageSentHandler {
	return &NegotiationMessageSentHandler{
		handler: NewNegotiationEventHandler(db, chatService, log),
	}
}

// Handle processes the negotiation.message_sent event.
func (h *NegotiationMessageSentHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	return h.handler.HandleNegotiationMessageSent(ctx, event.ID, event.Payload)
}

// NegotiationStartedPayload represents the payload of negotiation.started event.
type NegotiationStartedPayload struct {
	SessionID    uuid.UUID `json:"session_id"`
	ChatRoomID   string    `json:"chat_room_id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   uuid.UUID `json:"resource_id"`
	BuyerID      uuid.UUID `json:"buyer_id"`
	SellerID     uuid.UUID `json:"seller_id"`
	InitialPrice int64     `json:"initial_price"`
	Note         string    `json:"note"`
}

// NegotiationMessageSentPayload represents the payload of negotiation.message_sent event.
type NegotiationMessageSentPayload struct {
	SessionID        uuid.UUID `json:"session_id"`
	ChatRoomID       string    `json:"chat_room_id"`
	BuyerID          uuid.UUID `json:"buyer_id"`
	SellerID         uuid.UUID `json:"seller_id"`
	SenderID         uuid.UUID `json:"sender_id"`
	Price            int64     `json:"price"`
	ProposalSequence int       `json:"proposal_sequence"`
}

// HandleNegotiationStarted processes negotiation.started events.
//
// Sends the initial proposal message directly into chat_room_id — the
// buyer/seller's direct room, validated and persisted by StartNegotiation
// (Pass 7B). No room is created or resolved here (PASS_8A / F4).
//
// IDEMPOTENCY STRATEGY:
// - Message creation: Uses event ID as idempotency key
// - Additional DB constraint: UNIQUE (room_id, session_id, proposal_sequence)
// - Safe for retry: duplicates are silently ignored
func (h *NegotiationEventHandler) HandleNegotiationStarted(
	ctx context.Context,
	eventID uuid.UUID,
	payload []byte,
) error {
	var parsedPayload NegotiationStartedPayload
	if err := json.Unmarshal(payload, &parsedPayload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	roomID, err := uuid.Parse(parsedPayload.ChatRoomID)
	if err != nil {
		return fmt.Errorf("negotiation.started: invalid or missing chat_room_id %q: %w", parsedPayload.ChatRoomID, err)
	}

	h.log.Info("Handling negotiation.started",
		zap.String("event_id", eventID.String()),
		zap.String("session_id", parsedPayload.SessionID.String()),
		zap.String("room_id", roomID.String()),
		zap.String("buyer_id", parsedPayload.BuyerID.String()),
		zap.String("seller_id", parsedPayload.SellerID.String()),
	)

	// Execute within transaction for atomicity
	return h.db.WithTx(ctx, func(tx db.Tx) error {
		// Send initial proposal message (idempotent via event ID)
		idempotencyKey := fmt.Sprintf("negotiation.started.%s", eventID.String())
		_, err := h.sendProposalMessageTx(ctx, tx, roomID, parsedPayload.BuyerID, &parsedPayload, idempotencyKey)
		if err != nil {
			// If message already exists, it's a retry - treat as success
			if err == chatRepo.ErrDuplicateMessage {
				h.log.Info("Initial proposal message already exists, treating as success",
					zap.String("event_id", eventID.String()),
					zap.String("room_id", roomID.String()),
				)
				return nil
			}
			return fmt.Errorf("failed to send initial proposal: %w", err)
		}

		h.log.Info("Initial proposal message created successfully",
			zap.String("room_id", roomID.String()),
			zap.String("session_id", parsedPayload.SessionID.String()),
		)

		return nil
	})
}

// HandleNegotiationMessageSent processes negotiation.message_sent events.
//
// Sends the proposal message directly into chat_room_id (PASS_8A / F4) —
// no room lookup/resolution needed.
//
// IDEMPOTENCY STRATEGY:
// - Uses event ID as idempotency key
// - Additional DB constraint: UNIQUE (room_id, session_id, proposal_sequence)
// - Safe for retry: duplicates are silently ignored
func (h *NegotiationEventHandler) HandleNegotiationMessageSent(
	ctx context.Context,
	eventID uuid.UUID,
	payload []byte,
) error {
	var parsedPayload NegotiationMessageSentPayload
	if err := json.Unmarshal(payload, &parsedPayload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	roomID, err := uuid.Parse(parsedPayload.ChatRoomID)
	if err != nil {
		return fmt.Errorf("negotiation.message_sent: invalid or missing chat_room_id %q: %w", parsedPayload.ChatRoomID, err)
	}

	h.log.Info("Handling negotiation.message_sent",
		zap.String("event_id", eventID.String()),
		zap.String("session_id", parsedPayload.SessionID.String()),
		zap.String("room_id", roomID.String()),
		zap.String("buyer_id", parsedPayload.BuyerID.String()),
		zap.String("seller_id", parsedPayload.SellerID.String()),
		zap.String("sender_id", parsedPayload.SenderID.String()),
		zap.Int64("price", parsedPayload.Price),
	)

	// Execute within transaction for atomicity
	return h.db.WithTx(ctx, func(tx db.Tx) error {
		// Send proposal message (idempotent via event ID)
		idempotencyKey := fmt.Sprintf("negotiation.message_sent.%s", eventID.String())
		_, err := h.sendProposalMessageTx(ctx, tx, roomID, parsedPayload.SenderID, &parsedPayload, idempotencyKey)
		if err != nil {
			// If message already exists, it's a retry - treat as success
			if err == chatRepo.ErrDuplicateMessage {
				h.log.Info("Proposal message already exists, treating as success",
					zap.String("event_id", eventID.String()),
					zap.String("room_id", roomID.String()),
					zap.String("proposal_sequence", fmt.Sprintf("%d", parsedPayload.ProposalSequence)),
				)
				return nil
			}
			return fmt.Errorf("failed to send proposal message: %w", err)
		}

		h.log.Info("Proposal message created successfully",
			zap.String("room_id", roomID.String()),
			zap.String("session_id", parsedPayload.SessionID.String()),
			zap.Int("proposal_sequence", parsedPayload.ProposalSequence),
		)

		return nil
	})
}

// sendProposalMessageTx sends a proposal message within a transaction.
//
// Creates a negotiation proposal message with attachment containing:
// - session_id
// - resource_type, resource_id
// - price, proposal_sequence
// - note (for initial proposal)
//
// IDEMPOTENCY:
// - Uses event ID as idempotency key
// - DB constraint prevents duplicate messages with same idempotency key
func (h *NegotiationEventHandler) sendProposalMessageTx(
	ctx context.Context,
	tx db.Tx,
	roomID uuid.UUID,
	senderID uuid.UUID,
	payload interface{},
	idempotencyKey string,
) (*chatEntity.ChatMessage, error) {
	var attachment map[string]interface{}

	switch p := payload.(type) {
	case *NegotiationStartedPayload:
		attachment = buildNegotiationProposalFromStarted(p)
	case *NegotiationMessageSentPayload:
		attachment = buildNegotiationProposalFromMessageSent(p)
	default:
		return nil, fmt.Errorf("invalid payload type")
	}
	if err := validateCanonicalAttachmentJSON(attachment); err != nil {
		return nil, fmt.Errorf("invalid internal negotiation attachment: %w", err)
	}

	// Build message body
	body := fmt.Sprintf("Proposal: %d", getPriceFromPayload(payload))

	// Send message with idempotency key
	message, err := h.chatService.SendMessage(
		ctx,
		roomID,
		senderID,
		chatEntity.MessageTypeNegotiationProposal,
		&body,
		attachment,
		idempotencyKey,
	)

	return message, err
}

func buildNegotiationProposalFromStarted(p *NegotiationStartedPayload) map[string]interface{} {
	return map[string]interface{}{
		"type": "negotiation_proposal",
		"data": map[string]interface{}{
			"session_id":        p.SessionID.String(),
			"resource_type":     p.ResourceType,
			"resource_id":       p.ResourceID.String(),
			"price":             p.InitialPrice,
			"proposal_sequence": 1, // Initial proposal is always sequence 1
			"note":              p.Note,
		},
	}
}

func buildNegotiationProposalFromMessageSent(p *NegotiationMessageSentPayload) map[string]interface{} {
	return map[string]interface{}{
		"type": "negotiation_proposal",
		"data": map[string]interface{}{
			"session_id":        p.SessionID.String(),
			"price":             p.Price,
			"proposal_sequence": p.ProposalSequence,
		},
	}
}

func validateCanonicalAttachmentJSON(attachment map[string]interface{}) error {
	errs := chatvalidator.ValidateAttachmentJSON(attachment)
	if !chatvalidator.HasValidationErrors(errs) {
		return nil
	}
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, fmt.Sprintf("%s: %s", e.Field, e.Message))
	}
	return errors.New(strings.Join(parts, "; "))
}

// getPriceFromPayload extracts the price from a payload.
func getPriceFromPayload(payload interface{}) int64 {
	switch p := payload.(type) {
	case *NegotiationStartedPayload:
		return p.InitialPrice
	case *NegotiationMessageSentPayload:
		return p.Price
	default:
		return 0
	}
}


