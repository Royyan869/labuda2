// Package worker — handler for seller.subscription.expired outbox events.
//
// Expired-seller visibility convergence (Phase 6, scheduled-only): when a
// seller's subscription lapses, all auctions of that seller in the
// `scheduled` state are cancelled so they cannot go live. Active auctions
// are NOT cancelled in this iteration — they continue running but
// AuctionService.PlaceBid + Guard 6 already block any new bid or order
// from completing, so the live auction is functionally inert.
package worker

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	dbpkg "github.com/labuda/backend/pkg/db"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"go.uber.org/zap"
)

// SellerSubscriptionExpiredHandler cancels scheduled auctions when a
// seller's subscription transitions to expired. The handler is registered
// on the "seller.subscription.expired" outbox event topic emitted by
// SellerSubscriptionExpiryWorker.
//
// IDEMPOTENT: cancels only rows whose status is still 'scheduled'.
// Re-running the handler for the same event has no further effect.
type SellerSubscriptionExpiredHandler struct {
	db  *dbpkg.DB
	log *zap.Logger
}

// NewSellerSubscriptionExpiredHandler constructs the handler.
func NewSellerSubscriptionExpiredHandler(db *dbpkg.DB, log *zap.Logger) *SellerSubscriptionExpiredHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &SellerSubscriptionExpiredHandler{db: db, log: log}
}

// sellerSubscriptionExpiredPayload mirrors the worker's outbox payload
// (see seller_subscription_expiry_worker.go ProcessGraceToExpired).
type sellerSubscriptionExpiredPayload struct {
	SubscriptionID uuid.UUID `json:"subscription_id"`
	UserID         uuid.UUID `json:"user_id"`
}

// Handle parses the payload and cancels every scheduled auction owned by
// the now-expired seller. Active auctions are left running — they cannot
// settle because PlaceBid is gated and order creation Guard 6 rejects.
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

	// Atomic + idempotent: only flips rows still in 'scheduled' state.
	result, err := h.db.Pool().Exec(ctx, `
		UPDATE auctions
		SET status = 'cancelled', updated_at = NOW()
		WHERE seller_id = $1
		  AND status = 'scheduled'
	`, p.UserID)
	if err != nil {
		h.log.Error("seller_subscription_expired_handler: cancel scheduled auctions failed",
			zap.String("user_id", p.UserID.String()),
			zap.Error(err),
		)
		return err
	}

	rows := result.RowsAffected()
	if rows > 0 {
		h.log.Info("seller_subscription_expired_handler: cancelled scheduled auctions",
			zap.String("user_id", p.UserID.String()),
			zap.Int64("cancelled_count", rows),
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
// Auction cancellation runs FIRST, then notification delivery.
func (w *OutboxWorker) SetupSellerSubscriptionExpiredHandler(db *dbpkg.DB) *OutboxWorker {
	auctionHandler := NewSellerSubscriptionExpiredHandler(db, w.log)

	const eventType = "seller.subscription.expired"
	if existing, ok := w.dispatcher.handlers[eventType]; ok {
		// Notification handler already registered — compose via fanout.
		// Auction cancellation runs first (domain side-effect), then notification.
		w.dispatcher.handlers[eventType] = &fanoutHandler{
			handlers: []EventHandler{auctionHandler, existing},
		}
		w.log.Info("Seller subscription expired handler composed with existing notification handler (fanout)")
	} else {
		w.dispatcher.Register(eventType, auctionHandler)
		w.log.Info("Seller subscription expired event handler registered")
	}
	return w
}


