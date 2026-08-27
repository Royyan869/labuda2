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
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ForSaleSoldEventHandler handles the for_sale.sold event.
// Sends system messages to all active negotiation chats for the sold sale.
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

// HandleEvent processes the for_sale.sold event.
// Sends system messages to all active negotiation chats for this sale.
func (h *ForSaleSoldEventHandler) HandleEvent(ctx context.Context, payload []byte) error {
	var event struct {
		ForSaleID string `json:"for_sale_id"`
		SellerID         string `json:"seller_id"`
		BuyerID          string `json:"buyer_id"`
		OrderID          string `json:"order_id"`
		Title            string `json:"title"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	forSaleID, err := uuid.Parse(event.ForSaleID)
	if err != nil {
		return fmt.Errorf("invalid for_sale_id: %w", err)
	}

	buyerID, err := uuid.Parse(event.BuyerID)
	if err != nil {
		return fmt.Errorf("invalid buyer_id: %w", err)
	}

	var negotiationsNotified int
	var acceptedCancelled int
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		negotiations, err := h.negotiationRepo.GetActiveNegotiationsByForSaleExcludingBuyer(
			ctx, tx, forSaleID, buyerID,
		)
		if err != nil {
			return fmt.Errorf("failed to fetch negotiations: %w", err)
		}

		for _, neg := range negotiations {
			if neg.ChatRoomID == nil {
				continue
			}

			systemMessage := fmt.Sprintf(
				"Item Sold: \"%s\" has been sold to another buyer. Your negotiation is automatically closed.",
				event.Title,
			)

			if err := h.chatService.SendSystemMessage(ctx, *neg.ChatRoomID, systemMessage); err != nil {
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
