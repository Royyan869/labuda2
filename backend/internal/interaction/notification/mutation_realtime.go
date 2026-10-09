package notification

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/realtime"
	"github.com/labuda/backend/pkg/db"
)

// OutboxInserter inserts a durable outbox event inside the caller's
// transaction. Satisfied by the platform outbox repository.
type OutboxInserter interface {
	InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}

// UnreadCounter computes the canonical unread count for a recipient inside
// the caller's transaction. Satisfied by the notification repository's
// CountUnread — the SAME authority as GET /notifications/unread-count.
type UnreadCounter interface {
	CountUnread(ctx context.Context, tx interface{}, recipientID uuid.UUID) (int, error)
}

// MutationRealtimeEmitter is THE single canonical emission point for the
// user-targeted `notification.updated` realtime signal (TASK_4.2).
//
// Every committed notification-state mutation (mark-read, mark-all-read,
// delete, chat-room-read notification sync) emits through this one helper,
// inside the SAME transaction as the mutation, so:
//   - a committed state change always has exactly one outbox event;
//   - a rollback removes both the mutation and the event;
//   - the payload's unread_count is measured by CountUnread (canonical
//     authority) and is a supplemental hint — never client truth.
//
// One event represents the whole user-level state change (never one event
// per affected row). Nil-safe: a nil emitter or missing outbox (test
// harnesses without realtime wiring) is a no-op.
type MutationRealtimeEmitter struct {
	Outbox  OutboxInserter
	Counter UnreadCounter
}

// EmitStateUpdated writes one notification.updated outbox event for the
// recipient inside the caller's transaction. Callers invoke it ONLY when the
// mutation actually changed notification state (state-change rule).
func (e *MutationRealtimeEmitter) EmitStateUpdated(
	ctx context.Context,
	tx db.Tx,
	recipientID uuid.UUID,
) error {
	if e == nil || e.Outbox == nil || e.Counter == nil {
		return nil
	}
	unreadCount, err := e.Counter.CountUnread(ctx, tx, recipientID)
	if err != nil {
		return fmt.Errorf("count unread for notification.updated failed: %w", err)
	}
	payload := map[string]interface{}{
		"recipient_id": recipientID.String(),
		"unread_count": unreadCount,
	}
	// Each committed mutation is a distinct logical event: a fresh key keeps
	// the outbox unique (idempotency_key) satisfied without dedup semantics.
	idempotencyKey := "notification.updated." + uuid.NewString()
	if err := e.Outbox.InsertTx(ctx, tx, realtime.EventTypeNotificationUpdated, payload, idempotencyKey); err != nil {
		return fmt.Errorf("insert notification.updated outbox event failed: %w", err)
	}
	return nil
}
