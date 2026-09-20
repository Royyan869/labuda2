package realtime

import (
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TestOwnedOutboxEventTypes_AreDeliverable proves the ownership declaration and
// the dispatcher agree: every realtime-owned event type can actually be
// delivered to a WebSocket recipient. An owned type the dispatcher cannot
// deliver would be a permanently stuck claim.
func TestOwnedOutboxEventTypes_AreDeliverable(t *testing.T) {
	if len(OwnedOutboxEventTypes) == 0 {
		t.Fatal("realtime ownership declaration is empty")
	}

	dispatcher := NewDispatcherWithRoomResolver(NewHub(zap.NewNop()), testStatusChecker{}, nil, zap.NewNop())

	roomID := uuid.New()
	recipientID := uuid.New()

	fixtures := map[string][]byte{
		EventTypeChatMessageSent: mustRoomEventPayload(t, map[string]any{
			"room_id":    roomID.String(),
			"message_id": uuid.NewString(),
		}),
		EventTypeChatRoomCreated: mustRoomEventPayload(t, map[string]any{
			"recipient_id":    recipientID.String(),
			"room_id":         roomID.String(),
			"room_type":       "direct",
			"other_user_id":   uuid.NewString(),
			"unread_count":    0,
			"created_at":      "2026-06-14T00:00:00Z",
			"updated_at":      "2026-06-14T00:00:00Z",
			"last_message_at": "2026-06-14T00:00:00Z",
		}),
		EventTypeChatRoomUpdated: mustRoomEventPayload(t, map[string]any{
			"recipient_id":    recipientID.String(),
			"room_id":         roomID.String(),
			"room_type":       "direct",
			"other_user_id":   uuid.NewString(),
			"unread_count":    1,
			"created_at":      "2026-06-14T00:00:00Z",
			"updated_at":      "2026-06-14T00:01:00Z",
			"last_message_at": "2026-06-14T00:01:00Z",
		}),
	}

	for _, eventType := range OwnedOutboxEventTypes {
		payload, ok := fixtures[eventType]
		if !ok {
			t.Fatalf("ownership declares %q but the dispatcher has no canonical payload for it", eventType)
		}
		if err := dispatcher.Dispatch(eventType, payload); err != nil {
			t.Errorf("owned event type %q is not deliverable: %v", eventType, err)
		}
	}
}

// TestOwnedOutboxEventTypes_NoDuplicates proves the declaration lists each owned
// event type exactly once.
func TestOwnedOutboxEventTypes_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, eventType := range OwnedOutboxEventTypes {
		if seen[eventType] {
			t.Errorf("duplicate ownership entry %q", eventType)
		}
		seen[eventType] = true
	}
}

// TestDispatcher_UndeliverableOwnedEventTypeFailsLoudly proves NO SILENT SUCCESS:
// an event type the realtime dispatcher cannot deliver returns an error, so the
// worker routes it through retry/backoff → dead_letter instead of marking it
// delivered.
func TestDispatcher_UndeliverableOwnedEventTypeFailsLoudly(t *testing.T) {
	dispatcher := NewDispatcherWithRoomResolver(NewHub(zap.NewNop()), testStatusChecker{}, nil, zap.NewNop())

	if err := dispatcher.Dispatch("definitely.not.owned.event", mustRoomEventPayload(t, map[string]any{
		"room_id": uuid.NewString(),
	})); err == nil {
		t.Fatal("expected an error for an event type the realtime dispatcher cannot deliver, got nil (silent success)")
	}

	if err := dispatcher.Dispatch("totally.unknown.event", []byte(`{}`)); err == nil {
		t.Fatal("expected an error for an unknown event type, got nil (silent success)")
	}
}
