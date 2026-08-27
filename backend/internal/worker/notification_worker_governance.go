package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (h *NotificationEventHandler) handleDisputeOpened(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p struct {
		DisputeID     string `json:"dispute_id"`
		OrderID       string `json:"order_id"`
		BuyerID       string `json:"buyer_id"`
		SellerID      string `json:"seller_id"`
		CallerID      string `json:"caller_id"`
		Reason        string `json:"reason"`
		IsPostRelease bool   `json:"is_post_release"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("dispute.opened: unmarshal payload failed: %w", err)
	}

	// PRE-RELEASE: order.dispute_open already notified seller and admins — skip.
	if !p.IsPostRelease {
		return notificationInfo{}, nil
	}

	orderID, err := uuid.Parse(p.OrderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("dispute.opened: invalid order_id: %w", err)
	}
	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("dispute.opened: invalid buyer_id: %w", err)
	}
	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("dispute.opened: invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"orderId": orderID.String(),
	}

	// 1. Notify SELLER — compatibility-only path for legacy post-release payloads.
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, buyerID, "dispute.opened", orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("dispute.opened: insert seller notification failed: %w", err)
	}

	h.log.Debug("dispute.opened notification created (legacy post-release payload)",
		zap.String("recipient_id", sellerID.String()),
		zap.String("order_id", orderID.String()),
	)

	// 2. Admin fanout: same capability gate as order.dispute_open.
	if h.capabilityLister == nil {
		return info, nil
	}

	adminIDs, lErr := h.capabilityLister.ListUsersByCapability(ctx, "finance.dispute.resolve")
	if lErr != nil {
		h.log.Error("dispute.opened: failed to list admins by capability",
			zap.String("order_id", p.OrderID),
			zap.String("capability", "finance.dispute.resolve"),
			zap.Error(lErr),
		)
		return notificationInfo{}, fmt.Errorf("dispute.opened: list admins by capability: %w", lErr)
	}

	if len(adminIDs) == 0 {
		h.log.Warn("dispute.opened: no admins hold finance.dispute.resolve",
			zap.String("order_id", p.OrderID),
		)
		return info, nil
	}

	adminData := map[string]interface{}{
		"orderId":  p.OrderID,
		"buyerId":  p.BuyerID,
		"sellerId": p.SellerID,
	}

	var delivered, failed int
	for _, adminID := range adminIDs {
		_, insertErr := h.insertNotificationWithPolicy(
			ctx,
			adminID, buyerID,
			"dispute.opened",
			orderID,
			adminData,
		)
		if insertErr != nil {
			failed++
			h.log.Error("dispute.opened: admin fanout insert failed",
				zap.String("order_id", p.OrderID),
				zap.String("admin_id", adminID.String()),
				zap.Error(insertErr),
			)
			continue
		}
		delivered++
	}

	if delivered == 0 && failed > 0 {
		return notificationInfo{}, fmt.Errorf("dispute.opened: all %d admin fanout inserts failed for order %s", failed, p.OrderID)
	}

	return info, nil
}

// =============================================================================
// REFUND / DISPUTE LIFECYCLE NOTIFICATION HANDLERS — D1A
// =============================================================================

// handleRefundOpened processes refund.opened events.
// Buyer requested refund -> Seller gets notified.
func (h *NotificationEventHandler) handleRefundOpened(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p struct {
		RefundID string `json:"refund_id"`
		OrderID  string `json:"order_id"`
		BuyerID  string `json:"buyer_id"`
		SellerID string `json:"seller_id"`
		Reason   string `json:"reason"`
		Status   string `json:"status"`
		Amount   int64  `json:"amount"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	orderID, err := uuid.Parse(p.OrderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid order_id: %w", err)
	}
	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid buyer_id: %w", err)
	}
	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"orderId": orderID.String(),
	}

	// Notify SELLER (buyer requested refund)
	info, err := h.insertNotificationWithPolicy(
		ctx,
		sellerID, buyerID,
		"refund.opened",
		orderID,
		data,
	)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert seller notification failed: %w", err)
	}

	h.log.Debug("Refund opened notification created",
		zap.String("recipient_id", sellerID.String()),
		zap.String("order_id", orderID.String()),
	)

	return info, nil
}

// handleRefundApproved processes refund.approved events.
// Seller approved refund -> Buyer gets notified.
func (h *NotificationEventHandler) handleRefundApproved(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p struct {
		RefundID string `json:"refund_id"`
		OrderID  string `json:"order_id"`
		BuyerID  string `json:"buyer_id"`
		SellerID string `json:"seller_id"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	orderID, err := uuid.Parse(p.OrderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid order_id: %w", err)
	}
	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid buyer_id: %w", err)
	}
	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"orderId": orderID.String(),
	}

	// Notify BUYER (seller approved their refund)
	info, err := h.insertNotificationWithPolicy(
		ctx,
		buyerID, sellerID,
		"refund.approved",
		orderID,
		data,
	)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert buyer notification failed: %w", err)
	}

	h.log.Debug("Refund approved notification created",
		zap.String("recipient_id", buyerID.String()),
		zap.String("order_id", orderID.String()),
	)

	return info, nil
}

// handleRefundRejected processes refund.rejected events.
// Seller rejected refund -> Buyer gets notified.
func (h *NotificationEventHandler) handleRefundRejected(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p struct {
		RefundID string `json:"refund_id"`
		OrderID  string `json:"order_id"`
		BuyerID  string `json:"buyer_id"`
		SellerID string `json:"seller_id"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	orderID, err := uuid.Parse(p.OrderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid order_id: %w", err)
	}
	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid buyer_id: %w", err)
	}
	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"orderId": orderID.String(),
	}

	// Notify BUYER (seller rejected their refund)
	info, err := h.insertNotificationWithPolicy(
		ctx,
		buyerID, sellerID,
		"refund.rejected",
		orderID,
		data,
	)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert buyer notification failed: %w", err)
	}

	h.log.Debug("Refund rejected notification created",
		zap.String("recipient_id", buyerID.String()),
		zap.String("order_id", orderID.String()),
	)

	return info, nil
}

// handleRefundEscalated processes refund.escalated events.
// Refund escalated to dispute -> Both buyer and seller get notified.
func (h *NotificationEventHandler) handleRefundEscalated(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p struct {
		RefundID string `json:"refund_id"`
		OrderID  string `json:"order_id"`
		BuyerID  string `json:"buyer_id"`
		SellerID string `json:"seller_id"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	orderID, err := uuid.Parse(p.OrderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid order_id: %w", err)
	}
	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid buyer_id: %w", err)
	}
	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"orderId": orderID.String(),
	}

	// Notify SELLER
	sellerInfo, sErr := h.insertNotificationWithPolicy(
		ctx,
		sellerID, buyerID,
		"refund.escalated",
		orderID,
		data,
	)
	if sErr != nil {
		h.log.Error("Failed to insert seller refund.escalated notification",
			zap.String("order_id", orderID.String()),
			zap.Error(sErr),
		)
	} else if h.pushSender != nil && sellerInfo.inserted && sellerInfo.recipientID != (uuid.UUID{}) {
		// Explicit push for seller (buyer push handled by Handle() return)
		go h.sendPushAsync(context.Background(), sellerInfo)
	}

	// Notify BUYER (system-initiated escalation confirmation)
	buyerInfo, bErr := h.insertNotificationWithPolicy(
		ctx,
		buyerID, uuid.Nil, // system actor
		"refund.escalated",
		orderID,
		data,
	)
	if bErr != nil {
		h.log.Error("Failed to insert buyer refund.escalated notification",
			zap.String("order_id", orderID.String()),
			zap.Error(bErr),
		)
	}

	// Any obligated recipient insert failure must retry.
	if sErr != nil || bErr != nil {
		if sErr != nil && bErr != nil {
			return notificationInfo{}, fmt.Errorf("refund.escalated: seller insert failed: %v; buyer insert failed: %w", sErr, bErr)
		}
		if sErr != nil {
			return notificationInfo{}, fmt.Errorf("refund.escalated: seller insert failed: %w", sErr)
		}
		return notificationInfo{}, fmt.Errorf("refund.escalated: buyer insert failed: %w", bErr)
	}

	h.log.Debug("Refund escalated notifications created",
		zap.String("order_id", orderID.String()),
	)

	if buyerInfo.inserted {
		return buyerInfo, nil
	}
	return sellerInfo, nil
}

// handleDisputeResolved processes dispute.resolved events.
// Admin resolved dispute -> Both buyer and seller get notified.
// Copy varies by resolution type.
func (h *NotificationEventHandler) handleDisputeResolved(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p struct {
		DisputeID  string `json:"dispute_id"`
		OrderID    string `json:"order_id"`
		BuyerID    string `json:"buyer_id"`
		SellerID   string `json:"seller_id"`
		Resolution string `json:"resolution"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	orderID, err := uuid.Parse(p.OrderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid order_id: %w", err)
	}
	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid buyer_id: %w", err)
	}
	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"orderId":    orderID.String(),
		"resolution": p.Resolution,
	}

	// Resolution-specific copy
	title, body := getTitleAndBodyForDisputeResolution(p.Resolution)

	// Notify SELLER
	sellerInfo, sErr := h.insertNotificationWithPolicy(
		ctx,
		sellerID, uuid.Nil, // admin/system actor
		"dispute.resolved",
		orderID,
		data,
	)
	if sErr != nil {
		h.log.Error("Failed to insert seller dispute.resolved notification",
			zap.String("order_id", orderID.String()),
			zap.Error(sErr),
		)
	} else if h.pushSender != nil && sellerInfo.inserted && sellerInfo.recipientID != (uuid.UUID{}) {
		// Override with resolution-specific copy for push
		sellerInfo.title = title
		sellerInfo.body = body
		go h.sendPushAsync(context.Background(), sellerInfo)
	}

	// Notify BUYER
	buyerInfo, bErr := h.insertNotificationWithPolicy(
		ctx,
		buyerID, uuid.Nil, // admin/system actor
		"dispute.resolved",
		orderID,
		data,
	)
	if bErr != nil {
		h.log.Error("Failed to insert buyer dispute.resolved notification",
			zap.String("order_id", orderID.String()),
			zap.Error(bErr),
		)
	}

	// Any obligated recipient insert failure must retry.
	if sErr != nil || bErr != nil {
		if sErr != nil && bErr != nil {
			return notificationInfo{}, fmt.Errorf("dispute.resolved: seller insert failed: %v; buyer insert failed: %w", sErr, bErr)
		}
		if sErr != nil {
			return notificationInfo{}, fmt.Errorf("dispute.resolved: seller insert failed: %w", sErr)
		}
		return notificationInfo{}, fmt.Errorf("dispute.resolved: buyer insert failed: %w", bErr)
	}

	// Override with resolution-specific copy for push
	buyerInfo.title = title
	buyerInfo.body = body

	h.log.Debug("Dispute resolved notifications created",
		zap.String("order_id", orderID.String()),
		zap.String("resolution", p.Resolution),
	)

	if buyerInfo.inserted {
		return buyerInfo, nil
	}
	return sellerInfo, nil
}

// getTitleAndBodyForDisputeResolution returns resolution-specific notification copy.
func getTitleAndBodyForDisputeResolution(resolution string) (string, string) {
	switch resolution {
	case "refund":
		return "Sengketa Selesai", "Dana dikembalikan ke pembeli"
	case "release":
		return "Sengketa Selesai", "Dana diteruskan ke penjual"
	case "partial_split":
		return "Sengketa Selesai", "Sebagian dana dikembalikan ke pembeli"
	default:
		return "Sengketa Selesai", "Sengketa telah diselesaikan"
	}
}

// =============================================================================
// DISPUTE AGING ADMIN NOTIFICATION HANDLERS — G1
// =============================================================================

// handleDisputeOverdue processes dispute.overdue events.
// Admin-only fanout: notifies admins holding finance.dispute.resolve that a
// dispute has exceeded the 3-day overdue threshold and needs escalation.
func (h *NotificationEventHandler) handleDisputeOverdue(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p struct {
		DisputeID string `json:"dispute_id"`
		OrderID   string `json:"order_id"`
		BuyerID   string `json:"buyer_id"`
		SellerID  string `json:"seller_id"`
		DaysOpen  int    `json:"days_open"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	orderID, err := uuid.Parse(p.OrderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid order_id: %w", err)
	}

	if h.capabilityLister == nil {
		h.log.Warn("dispute.overdue: no capability lister — skipping admin notification",
			zap.String("dispute_id", p.DisputeID),
			zap.String("order_id", p.OrderID),
		)
		return notificationInfo{}, nil
	}

	adminIDs, lErr := h.capabilityLister.ListUsersByCapability(ctx, "finance.dispute.resolve")
	if lErr != nil {
		h.log.Error("dispute.overdue: failed to list admins by capability",
			zap.String("dispute_id", p.DisputeID),
			zap.String("order_id", p.OrderID),
			zap.String("capability", "finance.dispute.resolve"),
			zap.Error(lErr),
		)
		return notificationInfo{}, fmt.Errorf("dispute.overdue: list admins by capability: %w", lErr)
	}

	if len(adminIDs) == 0 {
		h.log.Warn("dispute.overdue: no admins hold finance.dispute.resolve",
			zap.String("dispute_id", p.DisputeID),
		)
		return notificationInfo{}, nil
	}

	data := map[string]interface{}{
		"disputeId": p.DisputeID,
		"orderId":   p.OrderID,
		"daysOpen":  p.DaysOpen,
		"status":    "under_review",
		"phase":     "overdue",
	}

	var lastInfo notificationInfo
	var delivered, failed int
	for _, adminID := range adminIDs {
		info, insertErr := h.insertNotificationWithPolicy(
			ctx,
			adminID, uuid.Nil, // system event
			"dispute.overdue",
			orderID,
			data,
		)
		if insertErr != nil {
			failed++
			h.log.Error("dispute.overdue: admin fanout insert failed",
				zap.String("dispute_id", p.DisputeID),
				zap.String("admin_id", adminID.String()),
				zap.String("capability", "finance.dispute.resolve"),
				zap.Error(insertErr),
			)
			continue
		}
		delivered++
		lastInfo = info
	}

	if delivered == 0 && failed > 0 {
		return notificationInfo{}, fmt.Errorf("dispute.overdue: all %d admin fanout inserts failed for dispute %s", failed, p.DisputeID)
	}

	return lastInfo, nil
}

// handleDisputeTimeoutEscalation processes dispute.timeout_escalation events.
// Admin-only fanout: notifies admins holding finance.dispute.resolve that a
// dispute has exceeded its timeout period (default 14 days) and requires
// immediate admin review. This is a critical-priority escalation.
func (h *NotificationEventHandler) handleDisputeTimeoutEscalation(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p struct {
		DisputeID       string `json:"dispute_id"`
		OrderID         string `json:"order_id"`
		BuyerID         string `json:"buyer_id"`
		SellerID        string `json:"seller_id"`
		DaysOpen        int    `json:"days_open"`
		TimeoutDays     int    `json:"timeout_days"`
		EscalationLevel string `json:"escalation_level"`
		Reason          string `json:"reason"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	orderID, err := uuid.Parse(p.OrderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid order_id: %w", err)
	}

	if h.capabilityLister == nil {
		h.log.Warn("dispute.timeout_escalation: no capability lister — skipping admin notification",
			zap.String("dispute_id", p.DisputeID),
			zap.String("order_id", p.OrderID),
		)
		return notificationInfo{}, nil
	}

	adminIDs, lErr := h.capabilityLister.ListUsersByCapability(ctx, "finance.dispute.resolve")
	if lErr != nil {
		h.log.Error("dispute.timeout_escalation: failed to list admins by capability",
			zap.String("dispute_id", p.DisputeID),
			zap.String("order_id", p.OrderID),
			zap.String("capability", "finance.dispute.resolve"),
			zap.Error(lErr),
		)
		return notificationInfo{}, fmt.Errorf("dispute.timeout_escalation: list admins by capability: %w", lErr)
	}

	if len(adminIDs) == 0 {
		h.log.Warn("dispute.timeout_escalation: no admins hold finance.dispute.resolve",
			zap.String("dispute_id", p.DisputeID),
		)
		return notificationInfo{}, nil
	}

	data := map[string]interface{}{
		"disputeId": p.DisputeID,
		"orderId":   p.OrderID,
		"daysOpen":  p.DaysOpen,
		"status":    "under_review",
		"phase":     "timeout",
	}

	var lastInfo notificationInfo
	var delivered, failed int
	for _, adminID := range adminIDs {
		info, insertErr := h.insertNotificationWithPolicy(
			ctx,
			adminID, uuid.Nil, // system event
			"dispute.timeout_escalation",
			orderID,
			data,
		)
		if insertErr != nil {
			failed++
			h.log.Error("dispute.timeout_escalation: admin fanout insert failed",
				zap.String("dispute_id", p.DisputeID),
				zap.String("admin_id", adminID.String()),
				zap.String("capability", "finance.dispute.resolve"),
				zap.Error(insertErr),
			)
			continue
		}
		delivered++
		lastInfo = info
	}

	if delivered == 0 && failed > 0 {
		return notificationInfo{}, fmt.Errorf("dispute.timeout_escalation: all %d admin fanout inserts failed for dispute %s", failed, p.DisputeID)
	}

	return lastInfo, nil
}

// handleOrderConfirmationExtended processes order.confirmation_extended events.
// Order confirmation extended -> Seller gets notified (buyer extended confirmation window).
func (h *NotificationEventHandler) handleOrderConfirmationExtended(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p OrderPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	orderID, err := uuid.Parse(p.OrderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid order_id: %w", err)
	}

	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid buyer_id: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"orderId": orderID.String(),
	}

	// Notify SELLER (buyer extended confirmation)
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, buyerID, "order.confirmation_extended", orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Order confirmation extended notification created",
		zap.String("recipient_id", sellerID.String()),
		zap.String("order_id", orderID.String()),
	)

	return info, nil
}

// =============================================================================
// ORDER OVERDUE REMINDER HANDLERS - OVERDUE ENFORCEMENT CLOSURE
// =============================================================================

// OrderOverdueReminderPayload represents the payload for overdue reminder events.
type OrderOverdueReminderPayload struct {
	OrderID  string `json:"order_id"`
	BuyerID  string `json:"buyer_id"`
	SellerID string `json:"seller_id"`
	Tier     string `json:"tier"` // tier1, tier2, tier3
	DaysLate int    `json:"days_late"`
}

// =============================================================================
// MONEY FAILURE ADMIN NOTIFICATION HANDLERS
// =============================================================================

// handleMoneyRefundFailed processes money.refund_failed events.
// Fans out to all admins holding governance.alert.read so critical gateway
// refund failures are surfaced as notifications, not only as system_alert rows.
//
// No user notifications — this is admin-only. The existing
// RefundFailedAlertHandler (O1A) creates the system_alert row; this handler
// runs in fanout alongside it to provide push-capable admin notifications.
func (h *NotificationEventHandler) handleMoneyRefundFailed(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p struct {
		RefundID uuid.UUID `json:"refund_id"`
		OrderID  uuid.UUID `json:"order_id"`
		Amount   int64     `json:"amount"`
		Error    string    `json:"error"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	// EntityID: prefer refund_id, fall back to order_id.
	entityID := p.RefundID
	if entityID == uuid.Nil {
		entityID = p.OrderID
	}

	if h.capabilityLister == nil {
		h.log.Warn("money.refund_failed: no capability lister — skipping admin notification",
			zap.String("refund_id", p.RefundID.String()),
			zap.String("order_id", p.OrderID.String()),
		)
		return notificationInfo{}, nil
	}

	adminIDs, lErr := h.capabilityLister.ListUsersByCapability(ctx, "governance.alert.read")
	if lErr != nil {
		h.log.Error("money.refund_failed: failed to list admins by capability",
			zap.String("refund_id", p.RefundID.String()),
			zap.String("order_id", p.OrderID.String()),
			zap.String("capability", "governance.alert.read"),
			zap.Error(lErr),
		)
		return notificationInfo{}, fmt.Errorf("money.refund_failed: list admins by capability: %w", lErr)
	}

	if len(adminIDs) == 0 {
		h.log.Warn("money.refund_failed: no admins hold governance.alert.read",
			zap.String("refund_id", p.RefundID.String()),
			zap.String("order_id", p.OrderID.String()),
		)
		return notificationInfo{}, nil
	}

	data := map[string]interface{}{
		"refundId": p.RefundID.String(),
		"orderId":  p.OrderID.String(),
		"amount":   p.Amount,
		"error":    p.Error,
	}

	var lastInfo notificationInfo
	var delivered, failed int
	for _, adminID := range adminIDs {
		info, insertErr := h.insertNotificationWithPolicy(
			ctx,
			adminID, uuid.Nil, // system actor
			"money.refund_failed",
			entityID,
			data,
		)
		if insertErr != nil {
			failed++
			h.log.Error("money.refund_failed: admin fanout insert failed",
				zap.String("refund_id", p.RefundID.String()),
				zap.String("order_id", p.OrderID.String()),
				zap.String("admin_id", adminID.String()),
				zap.String("capability", "governance.alert.read"),
				zap.Error(insertErr),
			)
			continue
		}
		delivered++
		lastInfo = info
	}

	// Total admin fanout failure → return error so outbox retries.
	if delivered == 0 && failed > 0 {
		return notificationInfo{}, fmt.Errorf("money.refund_failed: all %d admin fanout inserts failed for refund %s", failed, p.RefundID)
	}

	return lastInfo, nil
}

// =============================================================================
// ORDER OVERDUE REMINDER NOTIFICATION HANDLERS
// =============================================================================

// handleOrderOverdueReminderSeller processes order.overdue_reminder.seller events.
// Notifies seller that an order has exceeded its ready_to_ship_by deadline.
func (h *NotificationEventHandler) handleOrderOverdueReminderSeller(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p OrderOverdueReminderPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	orderID, err := uuid.Parse(p.OrderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid order_id: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"orderId":  orderID.String(),
		"tier":     p.Tier,
		"daysLate": p.DaysLate,
	}

	// Notify SELLER (system-initiated overdue alert)
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, uuid.Nil, "order.overdue_reminder.seller", orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Order overdue reminder notification created for seller",
		zap.String("recipient_id", sellerID.String()),
		zap.String("order_id", orderID.String()),
		zap.String("tier", p.Tier),
		zap.Int("days_late", p.DaysLate),
	)

	// Override title/body with tier-specific copy (getTitleAndBody has no tier context).
	info.title, info.body = h.getTitleAndBodyForOverdueReminder("order.overdue_reminder.seller", p.Tier, p.DaysLate)
	return info, nil
}

// handleOrderOverdueReminderBuyer processes order.overdue_reminder.buyer events.
// Notifies buyer that their order is delayed beyond the ready_to_ship_by deadline.
func (h *NotificationEventHandler) handleOrderOverdueReminderBuyer(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p OrderOverdueReminderPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	orderID, err := uuid.Parse(p.OrderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid order_id: %w", err)
	}

	buyerID, err := uuid.Parse(p.BuyerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid buyer_id: %w", err)
	}

	data := map[string]interface{}{
		"orderId":  orderID.String(),
		"tier":     p.Tier,
		"daysLate": p.DaysLate,
	}

	// Notify BUYER (system-initiated overdue alert)
	info, err := h.insertNotificationWithPolicy(ctx, buyerID, uuid.Nil, "order.overdue_reminder.buyer", orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Order overdue reminder notification created for buyer",
		zap.String("recipient_id", buyerID.String()),
		zap.String("order_id", orderID.String()),
		zap.String("tier", p.Tier),
		zap.Int("days_late", p.DaysLate),
	)

	// Override title/body with tier-specific copy (getTitleAndBody has no tier context).
	info.title, info.body = h.getTitleAndBodyForOverdueReminder("order.overdue_reminder.buyer", p.Tier, p.DaysLate)
	return info, nil
}

// getTitleAndBodyForOverdueReminder returns title and body for overdue reminder notifications.
// The messaging varies based on tier (severity) and recipient.
func (h *NotificationEventHandler) getTitleAndBodyForOverdueReminder(eventType, tier string, daysLate int) (title, body string) {
	switch eventType {
	case "order.overdue_reminder.seller":
		switch tier {
		case "tier1":
			return "Pesanan Melewati Estimasi Siap Kirim", "Pesanan telah melewati estimasi waktu siap kirim. Mohon segera kirim pesanan atau update statusnya."
		case "tier2":
			return "Pesanan Terlambat Dikirim", "Pesanan sudah terlambat beberapa hari dari estimasi. Segera kirim pesanan untuk menjaga kepercayaan pembeli."
		case "tier3":
			return "Pesanan Sangat Terlambat - Segera Kirim", "Pesanan sudah sangat terlambat. Segera kirim pesanan atau hubungi pembeli untuk koordinasi."
		default:
			return "Pesanan Terlambat", "Pesanan telah melewati estimasi siap kirim. Segera kirim pesanan."
		}
	case "order.overdue_reminder.buyer":
		switch tier {
		case "tier1":
			return "Pesanan Sedang Disiapkan", "Penjual sedang menyiapkan pesanan Anda. Estimasi pengiriman mungkin sedikit terlambat."
		case "tier2":
			return "Pesanan Sedang Disiapkan", "Mohon maaf, penjual memerlukan waktu lebih lama untuk menyiapkan pesanan Anda."
		case "tier3":
			return "Update Pesanan", "Kami meminta maaf atas keterlambatan pesanan Anda. Kami sedang menghubungi penjual untuk update lebih lanjut."
		default:
			return "Update Pesanan", "Kami akan terus memantau status pesanan Anda."
		}
	}
	return "Update Pesanan", "Kami akan memberikan update terkait pesanan Anda."
}

// =============================================================================
// MODERATION NOTIFICATION HANDLERS
// =============================================================================

// handleModerationContentRemoved processes moderation.content.removed events.
// Notifies the content owner that their content was removed due to moderation.


