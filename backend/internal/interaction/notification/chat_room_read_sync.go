package notification

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/interaction/notification/entity"
	"github.com/labuda/backend/pkg/db"
)

// ChatRoomReadSyncer marks a chat room's chat_message notifications as read
// for one recipient. It is the canonical implementation of the chat domain's
// ChatNotificationReadSyncer contract (TASK_3): reading a room is ONE user
// action, so the chat notifications that represent unread chat activity for
// that room complete in the SAME transaction as the chat read-state upsert.
//
// MATCHING RULE (the exact shape the chat-message notification producer
// writes in NotificationEventHandler.handleChatMessage):
//
//	recipient_id = recipient AND type = chat_message AND entity_id = room
//
// Only unread rows matching that shape are updated. Other rooms, other
// notification types and the chat unread authority (chat_read_states) are
// untouched — Chat and Notification keep their separate unread semantics.
type ChatRoomReadSyncer struct {
	Repo Repository

	// MutationEmitter emits the user-targeted notification.updated realtime
	// signal when this sync actually flips chat notifications to read
	// (TASK_4.2). Optional: nil (test harnesses / embeddings without
	// realtime wiring) keeps chat-only behavior.
	MutationEmitter *MutationRealtimeEmitter
}

// MarkChatRoomNotificationsRead completes the room's chat notifications for
// the recipient inside the caller's transaction. Idempotent: only unread
// matching rows are updated, so replays are no-ops.
//
// REALTIME (TASK_4.2): if the sync actually changed the recipient's
// notification unread state, exactly ONE notification.updated event is
// emitted in the SAME transaction. A no-op sync (nothing was unread) emits
// nothing — no phantom realtime events for unchanged state.
func (s *ChatRoomReadSyncer) MarkChatRoomNotificationsRead(
	ctx context.Context,
	tx db.Tx,
	recipientID, roomID uuid.UUID,
) error {
	unreadBefore, err := s.Repo.CountUnread(ctx, tx, recipientID)
	if err != nil {
		return fmt.Errorf("count unread before chat notification sync failed: %w", err)
	}

	if err := s.Repo.MarkAsReadByEntity(
		ctx,
		tx,
		recipientID,
		string(entity.TypeChatMessage),
		roomID,
	); err != nil {
		return err
	}

	unreadAfter, err := s.Repo.CountUnread(ctx, tx, recipientID)
	if err != nil {
		return fmt.Errorf("count unread after chat notification sync failed: %w", err)
	}
	if unreadBefore == unreadAfter {
		// No notification state change — no realtime event.
		return nil
	}

	return s.MutationEmitter.EmitStateUpdated(ctx, tx, recipientID)
}
