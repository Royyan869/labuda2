// Package worker
//
// STATUS: PARKED — not wired in serverboot/dependencies.go.
package worker

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"go.uber.org/zap"
)

// WSHub is the interface for WebSocket hub operations consumed by governance events.
// Defined here so the worker package does not import the realtime package.
type WSHub interface {
	EvictUser(userID uuid.UUID)
}

// WSEvictionHandler handles user.banned outbox events by evicting active
// WebSocket sessions for the banned user.
//
// CHAT-3 INVARIANT: banned-user eviction is event-driven, not polling-based
// (ADR-005). The user.banned outbox event is the canonical trigger; no DB
// polling for ban status is performed.
//
// If user.banned also has an order-processing handler registered (SetupUserBanHandler),
// register both handlers as a composite (see outbox_worker.go) so neither overwrites
// the other.
type WSEvictionHandler struct {
	hub WSHub
	log *zap.Logger
}

// NewWSEvictionHandler creates a new WS eviction handler.
func NewWSEvictionHandler(hub WSHub, log *zap.Logger) *WSEvictionHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &WSEvictionHandler{hub: hub, log: log}
}

// wsBanPayload is the minimal parse target for the user.banned outbox event.
// Only user_id is needed; other fields are ignored.
type wsBanPayload struct {
	UserID string `json:"user_id"`
}

// Handle processes a user.banned event and evicts all active WS sessions.
// Returns nil on parse errors (no retry); returns nil after eviction (idempotent).
func (h *WSEvictionHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	var payload wsBanPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to parse user.banned payload for WS eviction",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return nil // parse errors are not retryable
	}

	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		h.log.Error("Invalid user_id in user.banned payload",
			zap.String("event_id", event.ID.String()),
			zap.String("raw_user_id", payload.UserID),
			zap.Error(err),
		)
		return nil
	}

	h.hub.EvictUser(userID)

	h.log.Info("WS sessions evicted for banned user",
		zap.String("user_id", userID.String()),
		zap.String("event_id", event.ID.String()),
	)

	return nil
}


