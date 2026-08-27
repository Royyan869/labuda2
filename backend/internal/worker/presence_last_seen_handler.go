package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/presence"
	"go.uber.org/zap"
)

// PresenceLastSeenHandler replays durable last-seen persistence retries from outbox.
type PresenceLastSeenHandler struct {
	presence *presence.Service
	log      *zap.Logger
}

func NewPresenceLastSeenHandler(presenceSvc *presence.Service, log *zap.Logger) *PresenceLastSeenHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &PresenceLastSeenHandler{presence: presenceSvc, log: log}
}

func (h *PresenceLastSeenHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	if h.presence == nil {
		return fmt.Errorf("presence service unavailable")
	}

	var payload presence.LastSeenRecordPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("parse presence last_seen payload: %w", err)
	}
	if payload.UserID == uuid.Nil {
		return fmt.Errorf("presence last_seen payload missing user_id")
	}
	if payload.LastSeenAt == "" {
		return fmt.Errorf("presence last_seen payload missing last_seen_at")
	}

	occurredAt, err := time.Parse(time.RFC3339, payload.LastSeenAt)
	if err != nil {
		return fmt.Errorf("parse presence last_seen_at: %w", err)
	}

	return h.presence.PersistLastSeen(ctx, payload.UserID, occurredAt, payload.Version)
}

var _ EventHandler = (*PresenceLastSeenHandler)(nil)
