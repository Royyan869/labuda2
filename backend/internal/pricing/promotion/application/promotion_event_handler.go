package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// PromotionEventHandler handles outbox events that trigger promotion pause/stop/resume.
// This is the PRIMARY mechanism for event-driven promotion lifecycle management.
//
// Events handled:
// - for_sale.sold: Stop all promotions for the sold sale (permanent)
// - for_sale.withdrawn: Pause all promotions for the withdrawn sale (reversible)
// - for_sale.updated: Pause or stop based on operability, resume if operable again
// - auction.ended: Stop all promotions for the ended auction (permanent)
// - auction.cancelled: Stop all promotions for the cancelled auction (permanent)
// - seller.subscription.activated: Resume paused promotions for the seller
// - seller.subscription.expired: Pause active promotions for the seller
// - moderation.for_sale.restored: Resume paused promotions for the restored fixed-price sale
type PromotionEventHandler struct {
	service *PromotionService
	db      *db.DB
	log     *zap.Logger
}

// NewPromotionEventHandler creates a new PromotionEventHandler.
func NewPromotionEventHandler(
	service *PromotionService,
	dbConn *db.DB,
	log *zap.Logger,
) *PromotionEventHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &PromotionEventHandler{
		service: service,
		db:      dbConn,
		log:     log,
	}
}

// forSaleEventPayload represents the payload for fixed-price sale events.
type forSaleEventPayload struct {
	ForSaleID string `json:"for_sale_id"`
	SellerID  string `json:"seller_id"`
	Status    string `json:"status,omitempty"`
	Title     string `json:"title,omitempty"`
	Variety   string `json:"variety,omitempty"`
	Price     int64  `json:"price,omitempty"`
}

// auctionEventPayload represents the payload for auction events.
type auctionEventPayload struct {
	AuctionID     string  `json:"auction_id"`
	SellerID      string  `json:"seller_id"`
	Status        string  `json:"status"`
	StartPrice    int64   `json:"start_price"`
	CurrentBid    *int64  `json:"current_bid,omitempty"`
	CurrentWinner *string `json:"current_winner,omitempty"`
}

// Handle processes an outbox event and stops promotions if needed.
func (h *PromotionEventHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	h.log.Debug("PromotionEventHandler received event",
		zap.String("event_type", event.EventType),
		zap.String("aggregate_id", event.AggregateID.String()),
	)

	switch event.EventType {
	case "for_sale.sold":
		return h.handleForSaleSold(ctx, event)
	case "for_sale.withdrawn":
		return h.handleForSaleWithdrawn(ctx, event)
	case "for_sale.updated":
		return h.handleForSaleUpdated(ctx, event)
	case "auction.ended":
		return h.handleAuctionEnded(ctx, event)
	case "auction.cancelled":
		return h.handleAuctionCancelled(ctx, event)
	// Seller governance events
	case "seller.subscription.activated":
		return h.handleSellerSubscriptionActivated(ctx, event)
	case "seller.subscription.expired":
		return h.handleSellerSubscriptionExpired(ctx, event)
	// Moderation events
	case "moderation.for_sale.restored":
		return h.handleModerationForSaleRestored(ctx, event)
	default:
		h.log.Debug("PromotionEventHandler ignoring unknown event type",
			zap.String("event_type", event.EventType),
		)
		return nil
	}
}

// handleForSaleSold stops all promotions for a sold fixed-price sale.
func (h *PromotionEventHandler) handleForSaleSold(ctx context.Context, event platformevent.OutboxEvent) error {
	var payload forSaleEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to unmarshal for_sale.sold payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	forSaleID, err := uuid.Parse(payload.ForSaleID)
	if err != nil {
		return fmt.Errorf("invalid for_sale_id in payload: %w", err)
	}

	// Stop all promotions for this sale (permanent - fixed-price sale sold)
	return h.handlePromotionsForTarget(ctx, entity.TargetTypeForSale, forSaleID, "sold")
}

// handleForSaleWithdrawn pauses all promotions for a withdrawn sale.
// Withdrawn is reversible - the sale can be re-activated.
func (h *PromotionEventHandler) handleForSaleWithdrawn(ctx context.Context, event platformevent.OutboxEvent) error {
	var payload forSaleEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to unmarshal for_sale.withdrawn payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	forSaleID, err := uuid.Parse(payload.ForSaleID)
	if err != nil {
		return fmt.Errorf("invalid for_sale_id in payload: %w", err)
	}

	// Pause (reversible) - for_sale_hidden is classified as reversible
	return h.handlePromotionsForTarget(ctx, entity.TargetTypeForSale, forSaleID, "for_sale_hidden")
}

// handleForSaleUpdated checks sale operability and pauses, stops, or resumes promotions.
func (h *PromotionEventHandler) handleForSaleUpdated(ctx context.Context, event platformevent.OutboxEvent) error {
	var payload forSaleEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to unmarshal for_sale.updated payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	forSaleID, err := uuid.Parse(payload.ForSaleID)
	if err != nil {
		return fmt.Errorf("invalid for_sale_id in payload: %w", err)
	}

	isOperable, reason, err := h.service.operabilityChecker.CheckOperability(
		ctx, entity.TargetTypeForSale, &forSaleID)
	if err != nil {
		h.log.Error("Failed to check fixed-price sale operability",
			zap.String("for_sale_id", forSaleID.String()),
			zap.Error(err),
		)
		return nil // Don't fail - safety worker will catch it
	}

	if !isOperable {
		h.log.Info("Fixed-price sale became non-operable, pausing or stopping promotions",
			zap.String("for_sale_id", forSaleID.String()),
			zap.String("reason", reason),
		)
		// OnTargetStatusChanged now classifies reversible vs permanent
		return h.handlePromotionsForTarget(ctx, entity.TargetTypeForSale, forSaleID, reason)
	}

	// Sale is operable - try to resume any paused promotions
	return h.resumePromotionsForTarget(ctx, entity.TargetTypeForSale, forSaleID)
}

// handleAuctionEnded stops all promotions for an ended auction.
func (h *PromotionEventHandler) handleAuctionEnded(ctx context.Context, event platformevent.OutboxEvent) error {
	var payload auctionEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to unmarshal auction.ended payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	auctionID, err := uuid.Parse(payload.AuctionID)
	if err != nil {
		return fmt.Errorf("invalid auction_id in payload: %w", err)
	}

	// Stop all promotions for this auction (permanent — auction ended)
	return h.handlePromotionsForTarget(ctx, entity.TargetTypeAuction, auctionID, "ended")
}

// handleAuctionCancelled stops all promotions for a cancelled auction.
func (h *PromotionEventHandler) handleAuctionCancelled(ctx context.Context, event platformevent.OutboxEvent) error {
	var payload auctionEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to unmarshal auction.cancelled payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	auctionID, err := uuid.Parse(payload.AuctionID)
	if err != nil {
		return fmt.Errorf("invalid auction_id in payload: %w", err)
	}

	// Stop all promotions for this auction (permanent — auction cancelled)
	return h.handlePromotionsForTarget(ctx, entity.TargetTypeAuction, auctionID, "cancelled")
}

// handlePromotionsForTarget pauses or stops active promotions for a target
// based on the reversibility of the reason. Delegates to OnTargetStatusChanged
// which classifies reversible vs permanent.
func (h *PromotionEventHandler) handlePromotionsForTarget(
	ctx context.Context,
	targetType entity.TargetType,
	targetID uuid.UUID,
	status string,
) error {
	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	err = h.service.OnTargetStatusChanged(ctx, tx, targetType, targetID, status)
	if err != nil {
		h.log.Error("Failed to handle promotions for target",
			zap.String("target_type", string(targetType)),
			zap.String("target_id", targetID.String()),
			zap.String("status", status),
			zap.Error(err),
		)
		return fmt.Errorf("failed to handle promotions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	h.log.Info("Handled promotions for target",
		zap.String("target_type", string(targetType)),
		zap.String("target_id", targetID.String()),
		zap.String("status", status),
	)

	return nil
}

// resumePromotionsForTarget resumes paused promotions for a target
// that has become operable again.
func (h *PromotionEventHandler) resumePromotionsForTarget(
	ctx context.Context,
	targetType entity.TargetType,
	targetID uuid.UUID,
) error {
	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	resumedCount, err := h.service.ResumeByTarget(ctx, tx, targetType, targetID)
	if err != nil {
		h.log.Error("Failed to resume promotions for target",
			zap.String("target_type", string(targetType)),
			zap.String("target_id", targetID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to resume promotions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	if resumedCount > 0 {
		h.log.Info("Resumed promotions for target",
			zap.String("target_type", string(targetType)),
			zap.String("target_id", targetID.String()),
			zap.Int("resumed_count", resumedCount),
		)
	}

	return nil
}

// ========================================================================
// SELLER GOVERNANCE EVENT HANDLERS
// ========================================================================

// sellerEventPayload represents the payload for seller subscription events.
type sellerEventPayload struct {
	SellerID string `json:"seller_id"`
	UserID   string `json:"user_id,omitempty"`
	Status   string `json:"status,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// moderationRestoredPayload represents the payload for moderation.for_sale.restored events.
type moderationRestoredPayload struct {
	CaseID       string `json:"case_id"`
	AppealID     string `json:"appeal_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
}

// handleSellerSubscriptionActivated resumes paused promotions for a seller
// whose subscription has been re-activated.
func (h *PromotionEventHandler) handleSellerSubscriptionActivated(ctx context.Context, event platformevent.OutboxEvent) error {
	sellerID, err := h.parseSellerID(event)
	if err != nil {
		return err
	}

	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	resumedCount, err := h.service.ResumeByUser(ctx, tx, sellerID)
	if err != nil {
		h.log.Error("Failed to resume promotions for seller subscription activated",
			zap.String("seller_id", sellerID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to resume promotions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	if resumedCount > 0 {
		h.log.Info("Resumed promotions for seller subscription activated",
			zap.String("seller_id", sellerID.String()),
			zap.Int("resumed_count", resumedCount),
		)
	}

	return nil
}

// handleSellerSubscriptionExpired pauses active promotions for a seller
// whose subscription has expired. Subscription expired is reversible.
func (h *PromotionEventHandler) handleSellerSubscriptionExpired(ctx context.Context, event platformevent.OutboxEvent) error {
	sellerID, err := h.parseSellerID(event)
	if err != nil {
		return err
	}

	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	pausedCount, err := h.service.PauseByUser(ctx, tx, sellerID)
	if err != nil {
		h.log.Error("Failed to pause promotions for seller subscription expired",
			zap.String("seller_id", sellerID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to pause promotions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	if pausedCount > 0 {
		h.log.Info("Paused promotions for seller subscription expired",
			zap.String("seller_id", sellerID.String()),
			zap.Int("paused_count", pausedCount),
		)
	}

	return nil
}

// ========================================================================
// MODERATION EVENT HANDLERS
// ========================================================================

// handleModerationForSaleRestored resumes paused promotions for a fixed-price sale
// that has been restored after moderation (appeal approved).
func (h *PromotionEventHandler) handleModerationForSaleRestored(ctx context.Context, event platformevent.OutboxEvent) error {
	var payload moderationRestoredPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to unmarshal moderation.for_sale.restored payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	if payload.ResourceType != "for_sale" {
		return nil // Not a fixed-price sale restoration — ignore
	}

	forSaleID, err := uuid.Parse(payload.ResourceID)
	if err != nil {
		return fmt.Errorf("invalid resource_id in moderation.for_sale.restored payload: %w", err)
	}

	return h.resumePromotionsForTarget(ctx, entity.TargetTypeForSale, forSaleID)
}

// ========================================================================
// HELPERS
// ========================================================================

// parseSellerID extracts the seller/user ID from a seller event payload.
func (h *PromotionEventHandler) parseSellerID(event platformevent.OutboxEvent) (uuid.UUID, error) {
	var payload sellerEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to unmarshal seller event payload",
			zap.String("event_type", event.EventType),
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return uuid.Nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	idStr := payload.SellerID
	if idStr == "" {
		idStr = payload.UserID
	}
	if idStr == "" {
		return uuid.Nil, fmt.Errorf("no seller_id or user_id in %s payload", event.EventType)
	}

	sellerID, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid seller_id in %s payload: %w", event.EventType, err)
	}

	return sellerID, nil
}
