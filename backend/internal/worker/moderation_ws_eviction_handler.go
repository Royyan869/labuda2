package worker

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"go.uber.org/zap"
)

// ModerationWSEvictionHandler handles moderation.user.suspended outbox events
// by evicting active WebSocket sessions for the suspended user.
//
// This mirrors WSEvictionHandler (which handles user.banned) but parses the
// moderation payload format where the user ID is in resource_id (not user_id).
//
// INVARIANT: suspended-user eviction is event-driven, not polling-based.
type ModerationWSEvictionHandler struct {
	hub WSHub
	log *zap.Logger
}

// NewModerationWSEvictionHandler creates a new moderation WS eviction handler.
func NewModerationWSEvictionHandler(hub WSHub, log *zap.Logger) *ModerationWSEvictionHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &ModerationWSEvictionHandler{hub: hub, log: log}
}

// moderationSuspensionPayload is the minimal parse target for moderation.user.suspended.
// Only resource_id (the suspended user's UUID) is needed.
type moderationSuspensionPayload struct {
	ResourceID string `json:"resource_id"`
}

// Handle processes a moderation.user.suspended event and evicts all active WS sessions.
// Returns nil on parse errors (no retry); returns nil after eviction (idempotent).
func (h *ModerationWSEvictionHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	var payload moderationSuspensionPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to parse moderation.user.suspended payload for WS eviction",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return nil // parse errors are not retryable
	}

	userID, err := uuid.Parse(payload.ResourceID)
	if err != nil {
		h.log.Error("Invalid resource_id in moderation.user.suspended payload",
			zap.String("event_id", event.ID.String()),
			zap.String("raw_resource_id", payload.ResourceID),
			zap.Error(err),
		)
		return nil
	}

	h.hub.EvictUser(userID)

	h.log.Info("WS sessions evicted for suspended user",
		zap.String("user_id", userID.String()),
		zap.String("event_id", event.ID.String()),
	)

	return nil
}


