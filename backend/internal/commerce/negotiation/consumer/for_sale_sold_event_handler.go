package consumer

// DOMAIN: Negotiation Event Consumer
// NOTE: Handles for_sale.sold event to inform buyers in negotiation chats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	negotiationImpl "github.com/labuda/backend/internal/commerce/negotiation/infrastructure/repository"
	negotiationRepo "github.com/labuda/backend/internal/commerce/negotiation/repository"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ForSaleSoldEventHandler handles the for_sale.sold event (OWNER DECISION: WIRED).
//
// Two responsibilities, per the honesty doctrine ("negotiation grants a price
// path, not an inventory hold"):
//
//  1. Notifies all OTHER buyers (system message in their negotiation chat)
//     that the item has been sold and their negotiation is closed.
//  2. Bulk-cancels accepted, not-yet-ordered negotiations for the sold sale
//     (first-come-first-served: the winning buyer holds the only valid path).
//
// The winning buyer is excluded: their accepted negotiation is the canonical
// source of their order (negotiation_sessions.order_id) and must not be
// touched.
//
// Idempotency: event delivery is at-least-once. Re-delivery is safe because
// the bulk cancel is a no-op the second time and duplicate system messages in
// chat are cosmetic (dev/test data only; zero production data, zero-to-one).
type ForSaleSoldEventHandler struct {
	db              *db.DB
	chatService     *chatApp.Service
	negotiationRepo negotiationRepo.Repository
	log             *zap.Logger
}

// NewForSaleSoldEventHandler creates a new event handler.
func NewForSaleSoldEventHandler(
	db *db.DB,
	chatService *chatApp.Service,
	log *zap.Logger,
) *ForSaleSoldEventHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &ForSaleSoldEventHandler{
		db:              db,
		chatService:     chatService,
		negotiationRepo: negotiationImpl.NewNegotiationRepository(),
		log:             log,
	}
}

// Handle adapts the handler to the outbox worker's EventHandler interface
// (previously this handler was never registered — see outbox_event_registry
// for_sale.sold history).
func (h *ForSaleSoldEventHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	return h.HandleEvent(ctx, event.Payload)
}

// HandleEvent processes the for_sale.sold event.
//
// Wire contract (order_creation_service emitter):
//   - for_sale_id, seller_id, status are always present
//   - buyer_id is present from the WIRED payload (additive field); absent on
//     historical events → treated as unknown, exclusion filter skipped
//   - title is optional; falls back to a neutral message when absent
func (h *ForSaleSoldEventHandler) HandleEvent(ctx context.Context, payload []byte) error {
	var event struct {
		ForSaleID string `json:"for_sale_id"`
		SellerID  string `json:"seller_id"`
		BuyerID   string `json:"buyer_id"`
		Status    string `json:"status"`
		Title     string `json:"title"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	forSaleID, err := uuid.Parse(event.ForSaleID)
	if err != nil {
		return fmt.Errorf("invalid for_sale_id: %w", err)
	}

	if _, err := uuid.Parse(event.SellerID); err != nil {
		return fmt.Errorf("invalid seller_id: %w", err)
	}

	// buyer_id is additive — historical payloads may omit it. Unknown buyer
	// means the exclusion filter cannot run; all active/accepted negotiations
	// are treated as "other buyers".
	winningBuyerID := uuid.Nil
	if event.BuyerID != "" {
		winningBuyerID, err = uuid.Parse(event.BuyerID)
		if err != nil {
			return fmt.Errorf("invalid buyer_id: %w", err)
		}
	}

	// Tolerant parsing: uuid.Nil parses fine, so validate after parse.
	if event.BuyerID != "" && winningBuyerID == uuid.Nil {
		return fmt.Errorf("invalid buyer_id: nil uuid")
	}

	// The payload never carries the product title (the canonical Product lives
	// outside the sale surface). Keep the message honest and neutral.
	systemMessage := fmt.Sprintf(
		"Item Sold: fixed-price sale %s has been sold to another buyer. Your negotiation is automatically closed.",
		event.ForSaleID,
	)

	var negotiationsNotified int
	var acceptedCancelled int
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		negotiations, err := h.negotiationRepo.GetActiveNegotiationsByForSaleExcludingBuyer(
			ctx, tx, forSaleID, winningBuyerID,
		)
		if err != nil {
			return fmt.Errorf("failed to fetch negotiations: %w", err)
		}

		for _, neg := range negotiations {
			if neg.ChatRoomID == nil {
				continue
			}

			// The seller is the real actor the system speaks for in this
			// buyer↔seller room (sender_id is NOT NULL + FK; uuid.Nil violates
			// the constraint — see chat.SendSystemMessage root-cause note).
			if err := h.chatService.SendSystemMessage(ctx, *neg.ChatRoomID, neg.SellerID, systemMessage); err != nil {
				h.log.Error("failed to send system message",
					zap.String("chat_room_id", neg.ChatRoomID.String()),
					zap.String("negotiation_id", neg.ID.String()),
					zap.Error(err),
				)
				continue
			}

			negotiationsNotified++
			h.log.Info("sent item sold notification",
				zap.String("chat_room_id", neg.ChatRoomID.String()),
				zap.String("negotiation_id", neg.ID.String()),
				zap.String("for_sale_id", event.ForSaleID),
			)
		}

		// Bulk-cancel accepted, unordered negotiations for the sold sale.
		// With a known winning buyer this never touches the winner (their row
		// already carries order_id). The repo predicate is accepted+no-order;
		// the exclusion filter above only gates the notification loop.
		cancelled, err := h.negotiationRepo.BulkCancelAcceptedByForSaleNoOrder(ctx, tx, forSaleID)
		if err != nil {
			return fmt.Errorf("failed to cancel accepted negotiations: %w", err)
		}
		acceptedCancelled = cancelled

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to process for_sale.sold event: %w", err)
	}

	h.log.Info("processed for_sale.sold event",
		zap.String("for_sale_id", event.ForSaleID),
		zap.Int("negotiations_notified", negotiationsNotified),
		zap.Int("accepted_cancelled", acceptedCancelled),
	)

	return nil
}
