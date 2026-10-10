// Package worker — handler for seller.subscription.expired outbox events.
//
// Seller market-authority lapse (owner decision, Oct 2026): when a seller's
// subscription expires, their SCHEDULED auctions are lapsed (status
// 'lapsed') — NOT cancelled. The cancellation vocabulary is reserved for
// seller-initiated and moderation/admin outcomes; a seller who simply did
// not renew chose nothing.
//
// Deliberately NOT touched by this handler:
//   - RUNNING auctions (with or without bids): they continue to completion
//     (buyer fairness; PlaceBid and Guard 6 keep them inert). A bidless run
//     finishes as 'ended' with no winner and is relistable like any other
//     no-bid end.
//   - ForSale surfaces: they stay 'active'. The seller's listings vanish
//     from every viewer surface through the read-side market-authority
//     filters (browse/search/seller page), their CTAs are disabled by the
//     sellerTrust lifecycle axis, and checkout is blocked by Guard 6. A
//     renewal makes them sellable again with no republish and no state
//     change — the opposite of the old active→draft demotion, which was
//     purged together with the draft state itself.
package worker

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	platformevent "github.com/hishumi/backend/internal/platform/event"
	"github.com/hishumi/backend/pkg/db"
	"go.uber.org/zap"
)

// SellerSubscriptionExpiredHandler lapses scheduled auctions when a seller's
// subscription transitions to expired. The handler is registered on the
// "seller.subscription.expired" outbox event topic emitted by
// SellerSubscriptionExpiryWorker.
//
// IDEMPOTENT: lapses only rows whose status is still 'scheduled'. Re-running
// the handler for the same event has no further effect.
type SellerSubscriptionExpiredHandler struct {
	db  *db.DB
	log *zap.Logger
}

// NewSellerSubscriptionExpiredHandler constructs the handler.
func NewSellerSubscriptionExpiredHandler(db *db.DB, log *zap.Logger) *SellerSubscriptionExpiredHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &SellerSubscriptionExpiredHandler{db: db, log: log}
}

// sellerSubscriptionExpiredPayload mirrors the worker's outbox payload
// (see seller_subscription_expiry_worker.go ProcessActiveToExpired).
type sellerSubscriptionExpiredPayload struct {
	SubscriptionID uuid.UUID `json:"subscription_id"`
	UserID         uuid.UUID `json:"user_id"`
}

// Handle parses the payload and applies the market-authority lapse owned by
// the marketplace:
//
//   - auctions: scheduled → lapsed. The auction never went live without its
//     seller's market authority; it is hidden from viewer surfaces and
//     becomes relistable after renewal. No auction.cancelled event is
//     emitted — this outcome is not a cancellation, and the seller already
//     receives the global seller.subscription.expired notification.
//
// IDEMPOTENT: the UPDATE only touches rows still in 'scheduled'.
// Re-running the handler for the same event has no further effect.
func (h *SellerSubscriptionExpiredHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	var p sellerSubscriptionExpiredPayload
	if err := json.Unmarshal(event.Payload, &p); err != nil {
		h.log.Error("seller_subscription_expired_handler: failed to parse payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		// Parse errors are non-retryable.
		return nil
	}
	if p.UserID == uuid.Nil {
		return nil
	}

	// LAPSE: scheduled auctions cannot go live without market authority.
	// Atomic + idempotent: only flips rows still in 'scheduled'.
	result, err := h.db.Pool().Exec(ctx, `
		UPDATE auctions
		SET status = 'lapsed', updated_at = NOW()
		WHERE seller_id = $1
		  AND status = 'scheduled'
	`, p.UserID)
	if err != nil {
		h.log.Error("seller_subscription_expired_handler: lapse scheduled auctions failed",
			zap.String("user_id", p.UserID.String()),
			zap.Error(err),
		)
		return err
	}
	if rows := result.RowsAffected(); rows > 0 {
		h.log.Info("seller_subscription_expired_handler: lapsed scheduled auctions",
			zap.String("user_id", p.UserID.String()),
			zap.Int64("lapsed_count", rows),
		)
	}
	return nil
}

// SetupSellerSubscriptionExpiredHandler registers the handler on the outbox
// dispatcher. Call once during serverboot wiring.
//
// FANOUT-READY: If SetupNotificationHandlers was called before this (i.e.
// seller.subscription.expired already has a notification handler), this method
// composes both handlers via fanout so neither overwrites the other.
// The lapse runs first (domain side-effect), then notification delivery.
func (w *OutboxWorker) SetupSellerSubscriptionExpiredHandler(db *db.DB) *OutboxWorker {
	handler := NewSellerSubscriptionExpiredHandler(db, w.log)

	const eventType = "seller.subscription.expired"
	if existing, ok := w.dispatcher.handlers[eventType]; ok {
		// Notification handler already registered — compose via fanout.
		w.dispatcher.handlers[eventType] = &fanoutHandler{
			handlers: []EventHandler{handler, existing},
		}
		w.log.Info("Seller subscription expired handler composed with existing notification handler (fanout)")
	} else {
		w.dispatcher.Register(eventType, handler)
		w.log.Info("Seller subscription expired event handler registered")
	}
	return w
}
