package realtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	"go.uber.org/zap"
)

type testStatusChecker struct{}

func (testStatusChecker) EnsureActive(context.Context, uuid.UUID) error { return nil }
func (testStatusChecker) GetStatus(context.Context, uuid.UUID) (string, error) {
	return "active", nil
}
func (testStatusChecker) IsBanned(context.Context, uuid.UUID) (bool, error) { return false, nil }

type testRoomResolver struct {
	roomID uuid.UUID
}

func (r testRoomResolver) ResolveRoomIDByMessageID(
	context.Context,
	uuid.UUID,
) (uuid.UUID, error) {
	return r.roomID, nil
}

var _ auth.AccountStatusChecker = testStatusChecker{}

func TestDispatcher_ModerationChatHidden_BroadcastsRealtimeFrame(t *testing.T) {
	hub := NewHub(zap.NewNop())
	roomID := uuid.New()
	userID := uuid.New()
	conn := &Connection{
		ID:     "c1",
		UserID: userID,
		Send:   make(chan []byte, 1),
		Rooms:  map[uuid.UUID]struct{}{},
	}
	hub.Register(conn)
	hub.Subscribe(conn, roomID)

	dispatcher := NewDispatcherWithRoomResolver(
		hub,
		testStatusChecker{},
		testRoomResolver{roomID: roomID},
		zap.NewNop(),
	)
	messageID := uuid.New()
	payload := []byte(`{"resource_id":"` + messageID.String() + `"}`)

	if err := dispatcher.Dispatch(EventTypeModerationChatHidden, payload); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	select {
	case raw := <-conn.Send:
		var env WSEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("invalid ws envelope: %v", err)
		}
		if env.Type != "chat.message.hidden" {
			t.Fatalf("type=%q want chat.message.hidden", env.Type)
		}
		if env.Data["room_id"] != roomID.String() {
			t.Fatalf("room_id=%v want %s", env.Data["room_id"], roomID.String())
		}
		if env.Data["message_id"] != messageID.String() {
			t.Fatalf("message_id=%v want %s", env.Data["message_id"], messageID.String())
		}
	default:
		t.Fatal("expected realtime frame to be emitted")
	}
}

func TestDispatcher_ModerationChatRestored_BroadcastsRealtimeFrame(t *testing.T) {
	hub := NewHub(zap.NewNop())
	roomID := uuid.New()
	userID := uuid.New()
	conn := &Connection{
		ID:     "c1",
		UserID: userID,
		Send:   make(chan []byte, 1),
		Rooms:  map[uuid.UUID]struct{}{},
	}
	hub.Register(conn)
	hub.Subscribe(conn, roomID)

	dispatcher := NewDispatcherWithRoomResolver(
		hub,
		testStatusChecker{},
		testRoomResolver{roomID: roomID},
		zap.NewNop(),
	)
	messageID := uuid.New()
	payload := []byte(`{"resource_id":"` + messageID.String() + `"}`)

	if err := dispatcher.Dispatch(EventTypeModerationChatRestored, payload); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	select {
	case raw := <-conn.Send:
		var env WSEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("invalid ws envelope: %v", err)
		}
		if env.Type != "chat.message.restored" {
			t.Fatalf("type=%q want chat.message.restored", env.Type)
		}
		if env.Data["room_id"] != roomID.String() {
			t.Fatalf("room_id=%v want %s", env.Data["room_id"], roomID.String())
		}
		if env.Data["message_id"] != messageID.String() {
			t.Fatalf("message_id=%v want %s", env.Data["message_id"], messageID.String())
		}
	default:
		t.Fatal("expected realtime frame to be emitted")
	}
}


