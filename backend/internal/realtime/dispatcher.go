package realtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/identity/auth"
	"go.uber.org/zap"
)

// Event constants
const (
	EventTypeChatMessageSent        = "chat.message.sent"
	EventTypeModerationChatHidden   = "moderation.chat_message.hidden"
	EventTypeModerationChatRestored = "moderation.chat_message.restored"
)

// OutboxPayload represents the raw outbox event payload.
// This matches the structure emitted by the chat domain.
type OutboxPayload struct {
	RoomID    uuid.UUID `json:"room_id"`
	MessageID uuid.UUID `json:"message_id"`
}

type moderationChatMessagePayload struct {
	ResourceID string `json:"resource_id"`
	RoomID     string `json:"room_id"`
	MessageID  string `json:"message_id"`
}

type ChatMessageRoomResolver interface {
	ResolveRoomIDByMessageID(ctx context.Context, messageID uuid.UUID) (uuid.UUID, error)
}

// Dispatcher converts outbox events to minimal realtime signals and delivers
// them to governance-passing subscribers only.
//
// RESPONSIBILITIES:
// - Deserialize outbox payload
// - Construct fresh per-subscriber ViewerContext inputs at broadcast time
// - Filter subscribers through EvaluateWSBroadcast before delivery
// - Send minimal room-message envelope for message events
// - Send room-summary envelope for room-list events
//
// CONSTRAINTS per governance-constitution.md §2.2 and ADR-005:
// - No all-subscriber blind fanout
// - Broadcast-time lifecycle is always fresh (never reuse subscribe-time state)
// - No message payload in WS frame; client re-fetches via REST
// - Room-list events are user-targeted and never fan out to unrelated users
// - Evaluator performs no IO; Dispatcher hydrates lifecycle inputs
type Dispatcher struct {
	hub           *Hub
	statusChecker auth.AccountStatusChecker
	roomResolver  ChatMessageRoomResolver
	log           *zap.Logger
}

// NewDispatcher creates a new Dispatcher instance.
func NewDispatcher(hub *Hub, statusChecker auth.AccountStatusChecker, log *zap.Logger) *Dispatcher {
	return NewDispatcherWithRoomResolver(hub, statusChecker, nil, log)
}

func NewDispatcherWithRoomResolver(
	hub *Hub,
	statusChecker auth.AccountStatusChecker,
	roomResolver ChatMessageRoomResolver,
	log *zap.Logger,
) *Dispatcher {
	if log == nil {
		log = zap.NewNop()
	}
	return &Dispatcher{
		hub:           hub,
		statusChecker: statusChecker,
		roomResolver:  roomResolver,
		log:           log,
	}
}

// Dispatch processes an outbox event and delivers to governance-passing subscribers.
//
// Per-subscriber governance filter (CHAT-3 invariant):
//  1. Fetch fresh account_status for the subscriber from DB
//  2. Coarsen to PublicLifecycleState
//  3. EvaluateWSBroadcast → ALLOW or DROP
//  4. Deliver minimal envelope only on ALLOW
//
// The filter is IO-bound and called outside the hub lock. Stale subscribe-time
// lifecycle MUST NOT be trusted here (ADR-005).
func (d *Dispatcher) Dispatch(eventType string, payload []byte) error {
	if eventType != EventTypeChatMessageSent &&
		eventType != EventTypeModerationChatHidden &&
		eventType != EventTypeModerationChatRestored &&
		eventType != EventTypeChatRoomCreated &&
		eventType != EventTypeChatRoomUpdated {
		d.log.Debug("Skipping non-chat event",
			zap.String("event_type", eventType),
		)
		return nil
	}

	if eventType == EventTypeChatRoomCreated || eventType == EventTypeChatRoomUpdated {
		recipientID, roomPayload, err := d.resolveRoomEventRoute(payload)
		if err != nil {
			return err
		}

		serverMsg := marshalChatRoomSignal(eventType, roomPayload)
		d.hub.BroadcastToUserFiltered(recipientID, serverMsg, func(userID uuid.UUID) bool {
			ctx := context.Background()
			status, err := d.statusChecker.GetStatus(ctx, userID)
			if err != nil {
				d.log.Debug("Lifecycle check failed for user-targeted subscriber, dropping broadcast delivery",
					zap.String("user_id", userID.String()),
					zap.String("room_id", roomPayload.RoomID),
					zap.Error(err),
				)
				return false
			}
			lifecycle := viewercontext.CoarsenLifecycle(status, false)
			decision := evaluator.EvaluateWSBroadcast(lifecycle)
			return decision == evaluator.WSBroadcastAllow
		})

		d.log.Debug("Room event dispatched with per-user governance filter",
			zap.String("event_type", eventType),
			zap.String("recipient_id", recipientID.String()),
			zap.String("room_id", roomPayload.RoomID),
		)

		return nil
	}

	roomID, messageID, err := d.resolveEventRoute(context.Background(), eventType, payload)
	if err != nil {
		return err
	}

	serverMsg := marshalChatSignal(eventType, roomID, messageID)

	// Per-subscriber governance filter.
	// Fresh lifecycle is fetched for each subscriber; no session-time state reuse.
	d.hub.BroadcastToRoomFiltered(roomID, serverMsg, func(userID uuid.UUID) bool {
		ctx := context.Background()
		status, err := d.statusChecker.GetStatus(ctx, userID)
		if err != nil {
			d.log.Debug("Lifecycle check failed for subscriber, dropping broadcast delivery",
				zap.String("user_id", userID.String()),
				zap.String("room_id", roomID.String()),
				zap.Error(err),
			)
			return false // fail-closed per governance-constitution.md §5
		}
		lifecycle := viewercontext.CoarsenLifecycle(status, false)
		decision := evaluator.EvaluateWSBroadcast(lifecycle)
		return decision == evaluator.WSBroadcastAllow
	})

	d.log.Debug("Event dispatched with per-subscriber governance filter",
		zap.String("event_type", eventType),
		zap.String("room_id", roomID.String()),
		zap.String("message_id", messageID.String()),
	)

	return nil
}

func marshalChatRoomSignal(eventType string, payload ChatRoomSummaryPayload) []byte {
	switch eventType {
	case EventTypeChatRoomCreated:
		return marshalChatRoomCreated(payload)
	default:
		return marshalChatRoomUpdated(payload)
	}
}

func marshalChatSignal(eventType string, roomID, messageID uuid.UUID) []byte {
	switch eventType {
	case EventTypeModerationChatHidden:
		return marshalChatMessageHidden(roomID, messageID)
	case EventTypeModerationChatRestored:
		return marshalChatMessageRestored(roomID, messageID)
	default:
		return marshalChatMessageSent(roomID, messageID)
	}
}

type roomEventRecipientPayload struct {
	RecipientID string `json:"recipient_id"`
}

func (d *Dispatcher) resolveRoomEventRoute(payload []byte) (uuid.UUID, ChatRoomSummaryPayload, error) {
	var recipient roomEventRecipientPayload
	if err := json.Unmarshal(payload, &recipient); err != nil {
		return uuid.Nil, ChatRoomSummaryPayload{}, fmt.Errorf("failed to parse room payload recipient: %w", err)
	}
	if recipient.RecipientID == "" {
		return uuid.Nil, ChatRoomSummaryPayload{}, fmt.Errorf("recipient_id is required")
	}
	recipientID, err := uuid.Parse(recipient.RecipientID)
	if err != nil {
		return uuid.Nil, ChatRoomSummaryPayload{}, fmt.Errorf("invalid recipient_id: %w", err)
	}

	var roomPayload ChatRoomSummaryPayload
	if err := json.Unmarshal(payload, &roomPayload); err != nil {
		return uuid.Nil, ChatRoomSummaryPayload{}, fmt.Errorf("failed to parse room payload: %w", err)
	}
	if roomPayload.RoomID == "" {
		return uuid.Nil, ChatRoomSummaryPayload{}, fmt.Errorf("room_id is required")
	}

	return recipientID, roomPayload, nil
}

func (d *Dispatcher) resolveEventRoute(
	ctx context.Context,
	eventType string,
	payload []byte,
) (uuid.UUID, uuid.UUID, error) {
	if eventType == EventTypeChatMessageSent {
		var outboxPayload OutboxPayload
		if err := json.Unmarshal(payload, &outboxPayload); err != nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("failed to parse payload: %w", err)
		}
		if outboxPayload.RoomID == uuid.Nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("room_id is required")
		}
		if outboxPayload.MessageID == uuid.Nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("message_id is required")
		}
		return outboxPayload.RoomID, outboxPayload.MessageID, nil
	}

	var p moderationChatMessagePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("failed to parse moderation payload: %w", err)
	}

	messageID := uuid.Nil
	if p.MessageID != "" {
		parsed, err := uuid.Parse(p.MessageID)
		if err != nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("invalid message_id: %w", err)
		}
		messageID = parsed
	} else if p.ResourceID != "" {
		parsed, err := uuid.Parse(p.ResourceID)
		if err != nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("invalid resource_id: %w", err)
		}
		messageID = parsed
	}
	if messageID == uuid.Nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("message_id/resource_id is required")
	}

	if p.RoomID != "" {
		roomID, err := uuid.Parse(p.RoomID)
		if err != nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("invalid room_id: %w", err)
		}
		return roomID, messageID, nil
	}

	if d.roomResolver == nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("room resolver unavailable for moderation event")
	}
	roomID, err := d.roomResolver.ResolveRoomIDByMessageID(ctx, messageID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("resolve room_id failed: %w", err)
	}
	if roomID == uuid.Nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("resolved room_id is empty")
	}
	return roomID, messageID, nil
}

// DispatchMany processes multiple events in batch.
// Returns count of successfully dispatched events.
func (d *Dispatcher) DispatchMany(events []EventToDispatch) (successCount int, err error) {
	for _, e := range events {
		if dispatchErr := d.Dispatch(e.EventType, e.Payload); dispatchErr != nil {
			d.log.Error("Failed to dispatch event",
				zap.String("event_id", e.ID.String()),
				zap.String("event_type", e.EventType),
				zap.Error(dispatchErr),
			)
			err = dispatchErr
		} else {
			successCount++
		}
	}
	return successCount, err
}

// EventToDispatch represents an event to be dispatched.
type EventToDispatch struct {
	ID        uuid.UUID
	EventType string
	Payload   []byte
}

// ParseEventToDispatch creates an EventToDispatch from raw outbox data.
func ParseEventToDispatch(id uuid.UUID, eventType string, payload []byte) EventToDispatch {
	return EventToDispatch{
		ID:        id,
		EventType: eventType,
		Payload:   payload,
	}
}


