// Package worker provides user ban event handling for moderation-safe transactions.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/internal/governance/dispute/application"
	"github.com/labuda/backend/internal/identity/auth"
	platformevent "github.com/labuda/backend/internal/platform/event"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// UserBanEventHandler handles user.banned events.
//
// MODERATION DOMAIN HARD LOCK:
// - Processes ban events safely with evidence-based decisions
// - Prevents abuse vectors through double-processing checks
// - Ensures no unfair loss for honest parties
//
// Event types handled:
// - user.banned -> handle ban with safe refund logic
type UserBanEventHandler struct {
	db             *dbpkg.DB
	orderService   *orderApp.OrderService
	disputeService *application.DisputeService
	log            *zap.Logger
}

// NewUserBanEventHandler creates a new user ban event handler.
func NewUserBanEventHandler(
	db *dbpkg.DB,
	orderService *orderApp.OrderService,
	disputeService *application.DisputeService,
	log *zap.Logger,
) *UserBanEventHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &UserBanEventHandler{
		db:             db,
		orderService:   orderService,
		disputeService: disputeService,
		log:            log,
	}
}

// userBannedPayload represents the payload of a user.banned event.
type userBannedPayload struct {
	UserID         string `json:"user_id"`
	PreviousStatus string `json:"previous_status"`
	Reason         string `json:"reason"`
	BannedBy       string `json:"banned_by"`
}

// Handle processes a user.banned event.
//
// STEP 4 — EVENT IDEMPOTENCY:
// - Each order is processed only once per ban event
// - Uses processed_ban_events table for tracking
//
// STEP 3 — SAFE REFUND LOGIC:
// - If no shipment evidence -> refund
// - If shipment possible -> force dispute
//
// Returns error only on critical failures (triggers retry).
func (h *UserBanEventHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	h.log.Info("Handling user ban event",
		zap.String("event_id", event.ID.String()),
	)

	// Parse payload
	var payload userBannedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to parse user banned payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return nil // Don't retry on parse error
	}

	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		h.log.Error("Invalid user_id in payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		return nil // Don't retry on invalid ID
	}

	// STEP 4 — EVENT IDEMPOTENCY:
	// Check if we've already processed this ban event for this order
	// This prevents double-refund and other abuse scenarios

	// Get all active orders for the banned user
	orders, err := h.getActiveOrdersForUser(ctx, userID)
	if err != nil {
		h.log.Error("Failed to get active orders for banned user",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to get active orders: %w", err)
	}

	h.log.Info("Processing ban for user",
		zap.String("user_id", userID.String()),
		zap.Int("active_orders", len(orders)),
	)

	// Process each order with safe refund logic.
	// Per-order failures are observable via structured log (order_id + user_id + role).
	// We still process all orders for visibility, but return an error when any
	// order fails so outbox retry can re-run failed orders.
	var firstErr error
	for _, order := range orders {
		if err := h.processOrderForBan(ctx, userID, order, event.ID); err != nil {
			h.log.Error("user ban handler: order processing failed — manual review required",
				zap.String("event_id", event.ID.String()),
				zap.String("user_id", userID.String()),
				zap.String("order_id", order.ID.String()),
				zap.String("order_status", string(order.Status)),
				zap.String("user_role", h.getUserRole(order, userID)),
				zap.Error(err),
			)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if firstErr != nil {
		return fmt.Errorf("user ban handler: one or more orders failed: %w", firstErr)
	}

	return nil
}

// getActiveOrdersForUser returns all active (non-terminal) orders for a user.
// This includes orders where the user is buyer or seller.
func (h *UserBanEventHandler) getActiveOrdersForUser(ctx context.Context, userID uuid.UUID) ([]*orderEntity.Order, error) {
	// Get orders as buyer
	var orders []*orderEntity.Order

	err := h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		// Query for orders where user is buyer OR seller
		// Only include non-terminal statuses
		rows, err := tx.Query(ctx, `
			SELECT id, buyer_id, seller_id, status, escrow_status,
			       proof_type, tracking_number, shipping_proof_media,
			       has_dispute, created_at
			FROM orders
			WHERE (buyer_id = $1 OR seller_id = $1)
			  AND status NOT IN ('completed', 'cancelled', 'expired', 'refunded', 'partially_refunded')
			ORDER BY created_at DESC
		`, userID)
		if err != nil {
			return fmt.Errorf("failed to query orders: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var o orderEntity.Order
			var buyerID, sellerID uuid.UUID
			var status orderEntity.Status
			var escrowStatus orderEntity.EscrowStatus
			var proofType, trackingNumber, shippingProofMedia *string
			var hasDispute bool
			var createdAt time.Time

			if err := rows.Scan(
				&o.ID, &buyerID, &sellerID, &status, &escrowStatus,
				&proofType, &trackingNumber, &shippingProofMedia,
				&hasDispute, &createdAt,
			); err != nil {
				return fmt.Errorf("failed to scan order: %w", err)
			}

			o.BuyerID = buyerID
			o.SellerID = sellerID
			o.Status = status
			o.EscrowStatus = escrowStatus
			o.ProofType = proofType
			o.TrackingNumber = trackingNumber
			o.ShippingProofMedia = shippingProofMedia
			o.HasDispute = hasDispute
			o.CreatedAt = createdAt

			orders = append(orders, &o)
		}

		return rows.Err()
	})

	return orders, err
}

// processOrderForBan processes a single order for a banned user.
//
// STEP 3 — SAFE REFUND LOGIC:
// Replace: "seller banned + paid -> auto refund"
// With:
// - if no shipment evidence -> refund
// - if shipment possible -> force dispute
//
// STEP 4 — EVENT IDEMPOTENCY:
// - Check if already processed for this event
// - Mark as processed after handling
func (h *UserBanEventHandler) processOrderForBan(
	ctx context.Context,
	bannedUserID uuid.UUID,
	order *orderEntity.Order,
	eventID uuid.UUID,
) error {
	// STEP 4 — EVENT IDEMPOTENCY CHECK:
	// Check if we've already processed this ban event for this order
	alreadyProcessed, err := h.checkAlreadyProcessed(ctx, order.ID, bannedUserID)
	if err != nil {
		return fmt.Errorf("failed to check idempotency: %w", err)
	}
	if alreadyProcessed {
		h.log.Info("Order already processed for this banned user, skipping",
			zap.String("order_id", order.ID.String()),
			zap.String("user_id", bannedUserID.String()),
		)
		return nil
	}

	h.log.Info("Processing order for banned user",
		zap.String("order_id", order.ID.String()),
		zap.String("banned_user_id", bannedUserID.String()),
		zap.String("order_status", string(order.Status)),
		zap.String("user_role", h.getUserRole(order, bannedUserID)),
	)

	// Determine action based on evidence and order state
	switch {
	case h.shouldAutoCompleteForBannedBuyer(bannedUserID, order):
		// STEP 1 — Buyer banned + delivered -> auto-complete immediately
		// Seller gets paid, buyer cannot complete after ban
		return h.completeOrderForBan(ctx, order, bannedUserID, eventID)

	case h.shouldRefundDirectly(order):
		// No shipment evidence - safe to refund
		return h.refundOrderForBan(ctx, order, bannedUserID, eventID)

	case h.shouldForceDispute(bannedUserID, order):
		// Has shipment evidence - need dispute to determine outcome
		return h.forceDisputeForBan(ctx, order, bannedUserID, eventID)

	default:
		// No action needed (e.g., already in terminal state)
		h.log.Info("No action needed for order",
			zap.String("order_id", order.ID.String()),
			zap.String("reason", "order state doesn't require action"),
		)
		if err := h.markAsProcessed(ctx, eventID, order.ID, bannedUserID, "no_action"); err != nil {
			return fmt.Errorf("failed to mark no-op as processed: %w", err)
		}
		return nil
	}
}

// getUserRole returns "buyer" or "seller" for the banned user in this order.
func (h *UserBanEventHandler) getUserRole(order *orderEntity.Order, userID uuid.UUID) string {
	if order.BuyerID == userID {
		return "buyer"
	}
	return "seller"
}

// ============================================================================
// SHIPMENT EVIDENCE (STEP 2)
// ============================================================================

// hasShipmentEvidence deterministically checks if order has shipment evidence.
//
// DEFINITION: Shipment evidence = tracking_number exists OR order.status >= shipped
//
// This rule is used consistently throughout the ban handler to determine
// whether an order can be safely refunded or requires admin review.
func (h *UserBanEventHandler) hasShipmentEvidence(order *orderEntity.Order) bool {
	// Evidence exists if tracking number is present
	if order.TrackingNumber != nil && *order.TrackingNumber != "" {
		return true
	}

	// OR if order status is shipped or later (shipped, delivered, etc.)
	return order.Status == orderEntity.StatusShipped ||
		order.Status == orderEntity.StatusDelivered ||
		order.Status == orderEntity.StatusCompleted ||
		order.Status == orderEntity.StatusDisputeOpen ||
		order.Status == orderEntity.StatusPartiallyRefunded
}

// shouldRefundDirectly determines if an order should be refunded directly.
// Returns true if there's no shipment evidence and funds can be safely returned.
func (h *UserBanEventHandler) shouldRefundDirectly(order *orderEntity.Order) bool {
	// Only refund if escrow is still holding (not already released/refunded)
	if order.EscrowStatus != orderEntity.EscrowStatusHolding {
		return false
	}

	// STEP 2 — SHIPMENT EVIDENCE RULE:
	// - If no shipment evidence -> refund
	// - Shipment evidence = tracking_number exists OR order.status >= shipped
	return !h.hasShipmentEvidence(order)
}

// shouldForceDispute determines if a dispute should be forced.
//
// Routing table for shipped/delivered status:
//
//	buyer banned  + shipped/delivered → false (shouldAutoCompleteForBannedBuyer owns this)
//	seller banned + shipped/delivered → true  (seller's good faith is the open question;
//	                                            admin must decide with ban context)
//
// For all other statuses: dispute iff shipment evidence exists.
func (h *UserBanEventHandler) shouldForceDispute(bannedUserID uuid.UUID, order *orderEntity.Order) bool {
	// Only force dispute if escrow is still holding
	if order.EscrowStatus != orderEntity.EscrowStatusHolding {
		return false
	}

	if order.Status == orderEntity.StatusShipped || order.Status == orderEntity.StatusDelivered {
		// Buyer banned + shipped/delivered: handled by shouldAutoCompleteForBannedBuyer.
		if order.BuyerID == bannedUserID {
			return false
		}
		// Seller banned + shipped/delivered: tracking proves transit, not legitimacy.
		// Route to admin dispute; do not auto-complete or auto-refund.
		return h.hasShipmentEvidence(order)
	}

	// All other non-terminal statuses: dispute iff shipment evidence exists.
	return h.hasShipmentEvidence(order)
}

// ============================================================================
// AUTO COMPLETE FOR BANNED BUYER (STEP 1)
// ============================================================================

// shouldAutoCompleteForBannedBuyer determines if an order should be auto-completed
// when the buyer is banned and the order is in shipped or delivered status.
//
// B4A: Since buyer "Terima Barang" now goes shipped→completed directly,
// banned buyers with shipped orders should also auto-complete immediately.
// Seller already shipped — they should get paid regardless of buyer ban.
func (h *UserBanEventHandler) shouldAutoCompleteForBannedBuyer(
	bannedUserID uuid.UUID,
	order *orderEntity.Order,
) bool {
	// Only applies when buyer is the banned user
	if order.BuyerID != bannedUserID {
		return false
	}

	// B4A: Auto-complete from shipped or delivered (both have shipment evidence)
	if order.Status != orderEntity.StatusShipped && order.Status != orderEntity.StatusDelivered {
		return false
	}

	// Only if escrow is still holding
	if order.EscrowStatus != orderEntity.EscrowStatusHolding {
		return false
	}

	// No dispute should be active
	if order.HasDispute {
		return false
	}

	return true
}

// completeOrderForBan auto-completes an order when buyer is banned and status = delivered.
//
// STEP 1 — AUTO COMPLETE FOR BANNED BUYER:
// - Completes the order immediately (releases escrow to seller)
// - Does not wait for buyer action (buyer is banned, cannot act)
// - Fair outcome: seller fulfilled obligation, should receive payment
func (h *UserBanEventHandler) completeOrderForBan(
	ctx context.Context,
	order *orderEntity.Order,
	bannedUserID uuid.UUID,
	eventID uuid.UUID,
) error {
	h.log.Info("Auto-completing order for banned buyer (delivered status)",
		zap.String("order_id", order.ID.String()),
		zap.String("banned_buyer_id", bannedUserID.String()),
	)

	return h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		// Use the order service's complete method with system caller ID
		// This bypasses the buyer authorization check since buyer is banned
		// B4A: System caller passes empty idempotency key (uses ledger-level idempotency)
		if err := h.orderService.Complete(ctx, tx, auth.SystemCallerID, order.ID, ""); err != nil {
			return fmt.Errorf("failed to complete order: %w", err)
		}
		if err := h.markAsProcessedTx(ctx, tx, eventID, order.ID, bannedUserID, "auto_completed"); err != nil {
			return fmt.Errorf("failed to mark as processed: %w", err)
		}

		h.log.Info("Successfully auto-completed order for banned buyer",
			zap.String("order_id", order.ID.String()),
			zap.String("banned_buyer_id", bannedUserID.String()),
		)
		return nil
	})
}

// refundOrderForBan refunds an order directly due to ban with no shipment evidence.
func (h *UserBanEventHandler) refundOrderForBan(
	ctx context.Context,
	order *orderEntity.Order,
	bannedUserID uuid.UUID,
	eventID uuid.UUID,
) error {
	h.log.Info("Refunding order due to ban (no shipment evidence)",
		zap.String("order_id", order.ID.String()),
		zap.String("banned_user_id", bannedUserID.String()),
	)

	return h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		// Use the order service's refund method
		// This handles ledger operations and state updates atomically
		if err := h.orderService.RefundOrder(ctx, tx, order.ID); err != nil {
			return fmt.Errorf("failed to refund order: %w", err)
		}
		if err := h.markAsProcessedTx(ctx, tx, eventID, order.ID, bannedUserID, "refunded"); err != nil {
			return fmt.Errorf("failed to mark as processed: %w", err)
		}

		h.log.Info("Successfully refunded order for banned user",
			zap.String("order_id", order.ID.String()),
		)
		return nil
	})
}

// forceDisputeForBan opens a dispute for orders with shipment evidence.
// This ensures fair resolution when there's evidence of shipment.
func (h *UserBanEventHandler) forceDisputeForBan(
	ctx context.Context,
	order *orderEntity.Order,
	bannedUserID uuid.UUID,
	eventID uuid.UUID,
) error {
	h.log.Info("Opening dispute for banned user's order (has shipment evidence)",
		zap.String("order_id", order.ID.String()),
		zap.String("banned_user_id", bannedUserID.String()),
	)

	return h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		// Check if dispute already exists
		if order.HasDispute {
			h.log.Info("Dispute already exists for order",
				zap.String("order_id", order.ID.String()),
			)
			return nil
		}

		// Open dispute for admin resolution
		reason := "user_banned_review"
		description := fmt.Sprintf("User %s banned. Order requires admin review for fair resolution.",
			bannedUserID.String())

		input := application.OpenDisputeInput{
			Reason:      reason,
			Description: &description,
			MediaURLs:   []string{},
		}

		if _, err := h.disputeService.OpenDispute(ctx, tx, order.ID, auth.SystemCallerID, input); err != nil {
			return fmt.Errorf("failed to open dispute: %w", err)
		}
		if err := h.markAsProcessedTx(ctx, tx, eventID, order.ID, bannedUserID, "dispute_opened"); err != nil {
			return fmt.Errorf("failed to mark as processed: %w", err)
		}

		h.log.Info("Successfully opened dispute for banned user's order",
			zap.String("order_id", order.ID.String()),
		)
		return nil
	})
}

// ============================================================================
// EVENT IDEMPOTENCY (STEP 4)
// ============================================================================

// checkAlreadyProcessed checks if this (order, banned user) pair has already been actioned.
// Keyed on (order_id, user_id) so the check holds across re-bans of the same user.
func (h *UserBanEventHandler) checkAlreadyProcessed(ctx context.Context, orderID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := h.db.Pool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM processed_ban_events
			WHERE order_id = $1 AND user_id = $2
		)
	`, orderID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check processed status: %w", err)
	}
	return exists, nil
}

// markAsProcessed records that this (order, banned user) pair has been actioned.
// event_id is stored for audit trail; the unique key is (order_id, user_id).
func (h *UserBanEventHandler) markAsProcessed(ctx context.Context, eventID, orderID, userID uuid.UUID, action string) error {
	return h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return h.markAsProcessedTx(ctx, tx, eventID, orderID, userID, action)
	})
}

// markAsProcessedTx records processing state within caller-owned transaction.
func (h *UserBanEventHandler) markAsProcessedTx(
	ctx context.Context,
	tx dbpkg.Tx,
	eventID, orderID, userID uuid.UUID,
	action string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO processed_ban_events (event_id, user_id, order_id, action_taken, processed_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (order_id, user_id) DO NOTHING
	`, eventID, userID, orderID, action)
	if err != nil {
		return fmt.Errorf("failed to mark as processed: %w", err)
	}
	return nil
}


