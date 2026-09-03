package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

func (h *NotificationEventHandler) handleNegotiationStarted(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p NegotiationPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	sessionID, err := uuid.Parse(p.SessionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid session_id: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	// Enrich chatRoomId: payload has no chat_room_id at started time (room created after).
	// Try DB lookup; use payload value as fallback (empty string → no deeplink).
	chatRoomID := p.ChatRoomID
	if chatRoomID == "" {
		_ = h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
			var cr string
			if scanErr := tx.QueryRow(ctx,
				`SELECT COALESCE(chat_room_id::text, '') FROM negotiation_sessions WHERE id = $1`,
				sessionID,
			).Scan(&cr); scanErr == nil {
				chatRoomID = cr
			}
			return nil // fail-soft: always succeed regardless of lookup result
		})
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"sessionId":    p.SessionID,
		"chatRoomId":   chatRoomID,
		"resourceType": p.ResourceType,
		"resourceId":   p.ResourceID,
	}

	// Notify SELLER (buyer-initiated negotiation)
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, uuid.Nil, "negotiation.started", sessionID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleNegotiationMessageSent processes negotiation.message_sent events.
// Notifies the OTHER party (not the sender) that a counter-offer has been sent.
// Both buyer and seller can send counter-offers; recipient = the non-sender.
func (h *NotificationEventHandler) handleNegotiationMessageSent(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p NegotiationPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	sessionID, err := uuid.Parse(p.SessionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid session_id: %w", err)
	}

	senderID, err := uuid.Parse(p.SenderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid sender_id: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid buyer_id: %w", err)
	}

	// Validate sender membership and compute recipient: the party who did NOT send.
	var recipientID uuid.UUID
	if senderID == sellerID {
		recipientID = buyerID
	} else if senderID == buyerID {
		recipientID = sellerID
	} else {
		h.log.Warn("negotiation.message_sent: sender is not buyer/seller, skipping notification",
			zap.String("session_id", p.SessionID),
			zap.String("sender_id", senderID.String()),
			zap.String("buyer_id", buyerID.String()),
			zap.String("seller_id", sellerID.String()),
		)
		return notificationInfo{}, nil
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"sessionId":  p.SessionID,
		"chatRoomId": p.ChatRoomID,
	}

	// Notify the other party (not the sender)
	info, err := h.insertNotificationWithPolicy(ctx, recipientID, uuid.Nil, "negotiation.message_sent", sessionID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleNegotiationAccepted processes negotiation.accepted events.
// Notifies the buyer that the seller has accepted their offer.
func (h *NotificationEventHandler) handleNegotiationAccepted(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p NegotiationPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	sessionID, err := uuid.Parse(p.SessionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid session_id: %w", err)
	}

	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid buyer_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"sessionId":    p.SessionID,
		"chatRoomId":   p.ChatRoomID,
		"resourceType": p.ResourceType,
		"resourceId":   p.ResourceID,
	}

	// Notify BUYER (seller accepted the offer)
	info, err := h.insertNotificationWithPolicy(ctx, buyerID, uuid.Nil, "negotiation.accepted", sessionID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleNegotiationExpired processes negotiation.expired events.
// Notifies both parties that the negotiation has expired.
func (h *NotificationEventHandler) handleNegotiationExpired(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p NegotiationPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	sessionID, err := uuid.Parse(p.SessionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid session_id: %w", err)
	}

	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid buyer_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"sessionId":  p.SessionID,
		"chatRoomId": p.ChatRoomID,
	}

	// Notify BUYER (session timed out — seller can be notified similarly if needed)
	info, err := h.insertNotificationWithPolicy(ctx, buyerID, uuid.Nil, "negotiation.expired", sessionID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleNegotiationCancelled processes negotiation.cancelled events.
// Notifies both buyer AND seller that the session was cancelled.
// Self-send is prevented when buyer_id == seller_id (shouldn't happen in
// practice but guarded for safety).
func (h *NotificationEventHandler) handleNegotiationCancelled(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p NegotiationPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	sessionID, err := uuid.Parse(p.SessionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid session_id: %w", err)
	}

	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid buyer_id: %w", err)
	}

	data := map[string]interface{}{
		"sessionId":  p.SessionID,
		"chatRoomId": p.ChatRoomID,
	}

	// Notify BUYER (primary — returned for push dispatch)
	info, err := h.insertNotificationWithPolicy(ctx, buyerID, uuid.Nil, "negotiation.cancelled", sessionID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert buyer notification failed: %w", err)
	}

	// Notify SELLER if seller_id present and differs from buyer (self-send guard)
	if p.SellerID != "" {
		sellerID, sErr := uuid.Parse(p.SellerID)
		if sErr != nil {
			h.log.Warn("negotiation.cancelled: invalid seller_id, skipping seller notification",
				zap.String("session_id", p.SessionID),
			)
		} else if sellerID != buyerID {
			sellerInfo, sErr := h.insertNotificationWithPolicy(ctx, sellerID, uuid.Nil, "negotiation.cancelled", sessionID, data)
			if sErr != nil {
				h.log.Warn("negotiation.cancelled: failed to notify seller", zap.Error(sErr))
			} else if h.pushSender != nil && sellerInfo.inserted && sellerInfo.allowPush {
				go h.sendPushAsync(context.Background(), sellerInfo)
			}
		}
	}

	return info, nil
}

// =============================================================================
// SELLER TIER CHANGE NOTIFICATION HANDLERS — B1
// =============================================================================

// handleSellerTierUpgraded processes seller.tier.upgraded events.
func (h *NotificationEventHandler) handleSellerTierUpgraded(ctx context.Context, payload []byte) (notificationInfo, error) {
	return h.handleSellerTierChanged(ctx, payload, "seller.tier.upgraded")
}

// handleSellerTierDowngraded processes seller.tier.downgraded events.
func (h *NotificationEventHandler) handleSellerTierDowngraded(ctx context.Context, payload []byte) (notificationInfo, error) {
	return h.handleSellerTierChanged(ctx, payload, "seller.tier.downgraded")
}

// handleSellerTierChanged is the shared implementation for seller tier notifications.
// Notifies the seller only (system-initiated reputation evaluation).
func (h *NotificationEventHandler) handleSellerTierChanged(ctx context.Context, payload []byte, eventType string) (notificationInfo, error) {
	var p SellerTierChangedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"previousTier": p.PreviousTier,
		"newTier":      p.NewTier,
	}

	// Notify SELLER (system-initiated — uuid.Nil actor)
	return h.insertNotificationWithPolicy(ctx, sellerID, uuid.Nil, eventType, sellerID, data)
}

// =============================================================================
// WITHDRAWAL NOTIFICATION HANDLERS
// =============================================================================

// handleWithdrawalRequested processes withdrawal.requested events.
// Notifies the seller that their withdrawal request was received, then fans out
// to all admins holding finance.withdraw.review so the approval queue is visible.
func (h *NotificationEventHandler) handleAuctionBidPlaced(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p AuctionBidPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	auctionID, err := uuid.Parse(p.AuctionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid auction_id: %w", err)
	}

	bidderID, err := uuid.Parse(p.BidderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid bidder_id: %w", err)
	}

	// Resolve seller from auction table (not present in bid payload).
	var sellerID uuid.UUID
	if err := h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return tx.QueryRow(ctx, "SELECT seller_id FROM auctions WHERE id = $1", auctionID).
			Scan(&sellerID)
	}); err != nil {
		return notificationInfo{}, fmt.Errorf("get auction seller failed: %w", err)
	}

	// Don't notify self (seller bidding on own auction — shouldn't happen but guard anyway)
	if bidderID == sellerID {
		return notificationInfo{}, nil
	}

	data := map[string]interface{}{
		"auctionId": auctionID.String(),
		"bidId":     p.BidID,
	}

	return h.insertNotificationWithPolicy(
		ctx,
		sellerID, bidderID,
		"auction.bid.placed",
		auctionID,
		data,
	)
}

// handleAuctionWaitingSettlement processes auction.waiting_settlement events.
// Notifies:
//   - WINNER (primary): they won the auction and must claim within 24 hours.
//   - SELLER (secondary): their auction has a winner waiting to claim/pay.
func (h *NotificationEventHandler) handleAuctionWaitingSettlement(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p AuctionLifecyclePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	if p.CurrentWinner == nil || *p.CurrentWinner == "" {
		// No winner — nothing to notify. This shouldn't happen for waiting_settlement
		// but guard defensively.
		return notificationInfo{}, nil
	}

	auctionID, err := uuid.Parse(p.AuctionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid auction_id: %w", err)
	}

	winnerID, err := uuid.Parse(*p.CurrentWinner)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid current_winner: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"auctionId": auctionID.String(),
	}

	// Notify WINNER — must claim within 24 hours (primary obligation).
	winnerInfo, wErr := h.insertNotificationWithPolicy(
		ctx,
		winnerID, sellerID,
		"auction.waiting_settlement",
		auctionID,
		data,
	)

	// Notify SELLER — auction has a winner; wait for buyer to complete the order.
	// Distinct type so seller copy ("Ada Pemenang Lelang") differs from winner copy.
	_, sErr := h.insertNotificationWithPolicy(
		ctx,
		sellerID, winnerID,
		"auction.seller_has_winner",
		auctionID,
		data,
	)
	if sErr != nil {
		h.log.Error("Failed to insert seller auction.seller_has_winner notification",
			zap.String("auction_id", auctionID.String()),
			zap.Error(sErr),
		)
	}

	// Return error if either insert failed — outbox will retry; both inserts are
	// idempotent (ON CONFLICT DO NOTHING), so retry is safe.
	if wErr != nil && sErr != nil {
		return notificationInfo{}, fmt.Errorf("auction.waiting_settlement: winner insert failed: %v; seller insert failed: %w", wErr, sErr)
	}
	if wErr != nil {
		return notificationInfo{}, fmt.Errorf("auction.waiting_settlement: winner insert failed: %w", wErr)
	}
	if sErr != nil {
		return notificationInfo{}, fmt.Errorf("auction.waiting_settlement: seller insert failed: %w", sErr)
	}

	return winnerInfo, nil
}

// handleAuctionSettlementFailed processes auction.settlement_failed events.
// Notifies the affected party when an auction's settlement fails and the
// auction returns to DRAFT:
//   - seller violation (seller_shipping_default): notify the SELLER.
//   - buyer violation (buyer_shipping_timeout / buyer_bnr): notify the WINNER
//     (violating buyer) AND the seller that the auction is available to relist.
//
// Payload shape (from AuctionSettlementWorker.emitSettlementFailedEvent):
//
//	{ "auction_id", "violated_user_id", "violation_type", "seller_id",
//	  "winner_id", "violation_id", "restricted_until", "timestamp" }
//
// Idempotency: insertNotificationWithPolicy uses ON CONFLICT (recipient_id, actor_id, type, entity_id)
// DO NOTHING, so replays are safe for each (recipient, type, auction) tuple.
func (h *NotificationEventHandler) handleAuctionSettlementFailed(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p struct {
		AuctionID       string `json:"auction_id"`
		ViolatedUserID  string `json:"violated_user_id"`
		ViolationType   string `json:"violation_type"`
		SellerID        string `json:"seller_id"`
		WinnerID        string `json:"winner_id"`
		ViolationID     string `json:"violation_id"`
		RestrictedUntil string `json:"restricted_until"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	auctionID, err := uuid.Parse(p.AuctionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid auction_id: %w", err)
	}
	violatedUserID, err := uuid.Parse(p.ViolatedUserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid violated_user_id: %w", err)
	}
	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"auctionId": auctionID.String(),
	}

	// SELLER violation — the seller defaulted (failed to provide a required
	// private quote). Notify the seller of the restriction.
	if p.ViolationType == "seller_shipping_default" {
		info, sErr := h.insertNotificationWithPolicy(
			ctx,
			sellerID, uuid.Nil, // system-initiated
			"auction.settlement_failed.seller_default",
			auctionID,
			data,
		)
		if sErr != nil {
			h.log.Error("Failed to insert seller settlement-failure notification",
				zap.String("auction_id", auctionID.String()),
				zap.Error(sErr),
			)
			return notificationInfo{}, fmt.Errorf("auction.settlement_failed: seller insert failed: %w", sErr)
		}
		return info, nil
	}

	// BUYER violation — notify the violating winner (primary; returned for push).
	winnerInfo, wErr := h.insertNotificationWithPolicy(
		ctx,
		violatedUserID, uuid.Nil, // system-initiated
		"auction.settlement_failed.buyer",
		auctionID,
		data,
	)
	if wErr != nil {
		h.log.Error("Failed to insert buyer settlement-failure notification",
			zap.String("auction_id", auctionID.String()),
			zap.Error(wErr),
		)
		return notificationInfo{}, fmt.Errorf("auction.settlement_failed: buyer insert failed: %w", wErr)
	}

	// Notify the SELLER that the auction is back in DRAFT and can be relisted.
	if sellerID != violatedUserID {
		sellerInfo, sErr := h.insertNotificationWithPolicy(
			ctx,
			sellerID, uuid.Nil, // system-initiated
			"auction.settlement_failed.relistable",
			auctionID,
			data,
		)
		if sErr != nil {
			h.log.Warn("Failed to insert seller relist notification after settlement failure",
				zap.String("auction_id", auctionID.String()),
				zap.Error(sErr),
			)
		} else if h.pushSender != nil && sellerInfo.inserted && sellerInfo.allowPush {
			go h.sendPushAsync(context.Background(), sellerInfo)
		}
	}

	h.log.Debug("Auction settlement-failure notifications created",
		zap.String("auction_id", auctionID.String()),
		zap.String("violated_user_id", violatedUserID.String()),
		zap.String("violation_type", p.ViolationType),
	)

	return winnerInfo, nil
}

// handleAuctionEndedNoWinner processes auction.ended events where no bid was placed.
// Notifies the SELLER that their auction ended without a winner.
// This handler is composed via fanout with the promotion handler so that
// promotion auto-stop (P4C1) still fires on the same event.
//
// NOTE: auction.waiting_settlement covers the with-winner path. This handler
// is ONLY reached when auction.ended is emitted (status="ended", no current_winner).
func (h *NotificationEventHandler) handleAuctionEndedNoWinner(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p AuctionLifecyclePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	auctionID, err := uuid.Parse(p.AuctionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid auction_id: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"auctionId": auctionID.String(),
	}

	return h.insertNotificationWithPolicy(
		ctx,
		sellerID, uuid.Nil, // system-initiated — no human actor
		"auction.ended_no_winner",
		auctionID,
		data,
	)
}

// =============================================================================
// EXTERNAL PRODUCT REVIEW NOTIFICATION HANDLERS
// =============================================================================

// handleExternalProductReviewLifecycle processes external_product.review.* events.
// Notifies the product owner of the admin review decision (approved / rejected /
// request_changes / hidden). The navigation data carries the external_product_id,
// title, review_status, and optional reason so the mobile UI can show inline context
// without a follow-up call.
func (h *NotificationEventHandler) handleExternalProductReviewLifecycle(
	ctx context.Context,
	payload []byte,
	notifyType, title, body string,
) (notificationInfo, error) {
	var p ExternalProductReviewPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	ownerID, err := uuid.Parse(p.OwnerUserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid owner_user_id: %w", err)
	}

	productID, err := uuid.Parse(p.ExternalProductID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid external_product_id: %w", err)
	}

	data := map[string]interface{}{
		"externalProductId": p.ExternalProductID,
		"title":             p.Title,
		"reviewStatus":      p.ReviewStatus,
	}
	if p.Reason != "" {
		data["reason"] = p.Reason
	}

	// Notify OWNER — admin-initiated review decision; actor is the system (uuid.Nil).
	info, err := h.insertNotificationWithPolicy(ctx, ownerID, uuid.Nil, notifyType, productID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	info.title = title
	info.body = body
	return info, nil
}

// =============================================================================
// SELLER SUBSCRIPTION NOTIFICATION HANDLERS
// =============================================================================

// handleSellerSubscriptionExpiringLegacy processes seller.subscription.expiring events.
// Notifies the seller that their subscription has expire soon.
