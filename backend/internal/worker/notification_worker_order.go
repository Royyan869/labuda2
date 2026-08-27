package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/events"
	"go.uber.org/zap"
)

func (h *NotificationEventHandler) handleOrderCreated(ctx context.Context, payload []byte) (notificationInfo, error) {
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

	// Navigation data for mobile - includes order ID for navigation
	data := map[string]interface{}{
		"orderId": orderID.String(),
	}

	// Notify SELLER: "Order Baru" - new order to fulfill (primary: returned for push dispatch)
	sellerInfo, err := h.insertNotificationWithPolicy(ctx, sellerID, buyerID, events.EventOrderCreated, orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert seller notification failed: %w", err)
	}

	// Notify BUYER: "Pesanan Berhasil Dibuat" - order confirmation (non-blocking)
	buyerInfo, bErr := h.insertNotificationWithPolicy(ctx, buyerID, buyerID, "order.created.buyer", orderID, data)
	if bErr != nil {
		h.log.Warn("Failed to create buyer notification for order created", zap.Error(bErr))
	} else if h.pushSender != nil && buyerInfo.inserted && buyerInfo.allowPush {
		go h.sendPushAsync(context.Background(), buyerInfo)
	}

	h.log.Debug("Order created notifications created",
		zap.String("order_id", orderID.String()),
	)

	// Return seller info for push dispatch in Handle()
	return sellerInfo, nil
}

// handleOrderPaid processes order.paid events.
// Buyer paid -> Both buyer and seller get notified.
// Seller: "Siap Dikirim" (prepare to ship)
// Buyer: "Pembayaran Berhasil" (payment confirmation - trust moment)
func (h *NotificationEventHandler) handleOrderPaid(ctx context.Context, payload []byte) (notificationInfo, error) {
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

	// Notify SELLER: "Siap Dikirim" - payment received, prepare to ship (primary: returned for push dispatch)
	sellerInfo, err := h.insertNotificationWithPolicy(ctx, sellerID, buyerID, events.EventOrderPaid, orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert seller notification failed: %w", err)
	}

	// Notify BUYER: "Pembayaran Berhasil" - payment confirmation (non-blocking)
	buyerInfo, bErr := h.insertNotificationWithPolicy(ctx, buyerID, buyerID, "order.paid.buyer", orderID, data)
	if bErr != nil {
		h.log.Warn("Failed to create buyer notification for order paid", zap.Error(bErr))
	} else if h.pushSender != nil && buyerInfo.inserted && buyerInfo.allowPush {
		go h.sendPushAsync(context.Background(), buyerInfo)
	}

	h.log.Debug("Order paid notifications created",
		zap.String("order_id", orderID.String()),
	)

	// Return seller info for push dispatch in Handle()
	return sellerInfo, nil
}

// handleOrderShipped processes order.shipped events.
// Seller shipped -> Buyer gets notified to expect delivery.
func (h *NotificationEventHandler) handleOrderShipped(ctx context.Context, payload []byte) (notificationInfo, error) {
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

	// Notify BUYER (seller shipped the item)
	info, err := h.insertNotificationWithPolicy(ctx, buyerID, sellerID, "order.shipped", orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Order shipped notification created",
		zap.String("recipient_id", buyerID.String()),
		zap.String("order_id", orderID.String()),
	)

	return info, nil
}

// handleOrderCompleted processes order.completed events.
// Order completed -> Both buyer and seller get notified.
func (h *NotificationEventHandler) handleOrderCompleted(ctx context.Context, payload []byte) (notificationInfo, error) {
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

	// Notify SELLER (escrow released, payment completed — primary: returned for push dispatch)
	sellerInfo, err := h.insertNotificationWithPolicy(ctx, sellerID, buyerID, events.EventOrderCompleted, orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert seller notification failed: %w", err)
	}

	// Notify BUYER (order successfully completed — non-blocking, push triggered inline)
	// NOTE: buyer push was previously missing; this migration restores the correct behavior.
	buyerInfo, bErr := h.insertNotificationWithPolicy(ctx, buyerID, sellerID, events.EventOrderCompleted, orderID, data)
	if bErr != nil {
		h.log.Warn("Failed to create buyer notification for completed", zap.Error(bErr))
	} else if h.pushSender != nil && buyerInfo.inserted && buyerInfo.allowPush {
		go h.sendPushAsync(context.Background(), buyerInfo)
	}

	h.log.Debug("Order completed notifications created",
		zap.String("order_id", orderID.String()),
	)

	// Return seller info for push dispatch in Handle()
	return sellerInfo, nil
}

// handleOrderCancelled processes order.cancelled events.
// Buyer cancelled -> Seller gets notified.
func (h *NotificationEventHandler) handleOrderCancelled(ctx context.Context, payload []byte) (notificationInfo, error) {
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

	// Notify SELLER (buyer cancelled the order)
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, buyerID, "order.cancelled", orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Order cancelled notification created",
		zap.String("recipient_id", sellerID.String()),
		zap.String("order_id", orderID.String()),
	)

	return info, nil
}

// handleOrderCancelledTimeout processes order.cancelled_timeout events.
// Shipment deadline exceeded -> Both buyer and seller get notified.
// Uses insertNotificationWithPolicy for governance compliance.
func (h *NotificationEventHandler) handleOrderCancelledTimeout(ctx context.Context, payload []byte) (notificationInfo, error) {
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
		"reason":  "timeout",
	}

	// Notify SELLER (shipment deadline exceeded — seller failed to ship)
	sellerInfo, sErr := h.insertNotificationWithPolicy(
		ctx,
		sellerID, uuid.Nil, // system-initiated
		"order.cancelled_timeout",
		orderID,
		data,
	)
	if sErr != nil {
		h.log.Error("Failed to insert seller cancelled_timeout notification",
			zap.String("order_id", orderID.String()),
			zap.Error(sErr),
		)
		// Continue to buyer notification even if seller notification fails
	}

	// Notify BUYER (their order was auto-cancelled)
	buyerInfo, bErr := h.insertNotificationWithPolicy(
		ctx,
		buyerID, uuid.Nil, // system-initiated
		"order.cancelled_timeout",
		orderID,
		data,
	)
	if bErr != nil {
		h.log.Error("Failed to insert buyer cancelled_timeout notification",
			zap.String("order_id", orderID.String()),
			zap.Error(bErr),
		)
	}

	// Any obligated recipient insert failure must retry.
	if sErr != nil || bErr != nil {
		if sErr != nil && bErr != nil {
			return notificationInfo{}, fmt.Errorf("order.cancelled_timeout: seller insert failed: %v; buyer insert failed: %w", sErr, bErr)
		}
		if sErr != nil {
			return notificationInfo{}, fmt.Errorf("order.cancelled_timeout: seller insert failed: %w", sErr)
		}
		return notificationInfo{}, fmt.Errorf("order.cancelled_timeout: buyer insert failed: %w", bErr)
	}

	h.log.Debug("Order cancelled_timeout notifications created",
		zap.String("order_id", orderID.String()),
		zap.String("buyer_id", buyerID.String()),
		zap.String("seller_id", sellerID.String()),
	)

	// Keep push dispatch path stable: primary return remains buyer when inserted,
	// otherwise fall back to seller so fresh inserted secondary can still push.
	if buyerInfo.inserted {
		return buyerInfo, nil
	}
	return sellerInfo, nil
}

// handleOrderExpired processes order.expired events.
// Payment expired -> Buyer gets notified.
func (h *NotificationEventHandler) handleOrderExpired(ctx context.Context, payload []byte) (notificationInfo, error) {
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

	data := map[string]interface{}{
		"orderId": orderID.String(),
	}

	// Notify BUYER (payment window expired — system-initiated)
	info, err := h.insertNotificationWithPolicy(ctx, buyerID, uuid.Nil, "order.expired", orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Order expired notification created",
		zap.String("recipient_id", buyerID.String()),
		zap.String("order_id", orderID.String()),
	)

	return info, nil
}

// handleOrderRefunded processes order.refunded events.
// Order refunded -> Buyer gets notified.
func (h *NotificationEventHandler) handleOrderRefunded(ctx context.Context, payload []byte) (notificationInfo, error) {
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

	// Notify BUYER (order refunded — actor is seller)
	info, err := h.insertNotificationWithPolicy(ctx, buyerID, sellerID, "order.refunded", orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Order refunded notification created",
		zap.String("recipient_id", buyerID.String()),
		zap.String("order_id", orderID.String()),
	)

	return info, nil
}

// handleOrderPartiallyRefunded processes order.partially_refunded events.
// Order partially refunded -> Buyer gets notified.
func (h *NotificationEventHandler) handleOrderPartiallyRefunded(ctx context.Context, payload []byte) (notificationInfo, error) {
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

	// Notify BUYER (partial refund processed — actor is seller)
	info, err := h.insertNotificationWithPolicy(ctx, buyerID, sellerID, "order.partially_refunded", orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Order partially refunded notification created",
		zap.String("recipient_id", buyerID.String()),
		zap.String("order_id", orderID.String()),
	)

	return info, nil
}

// handleOrderDisputeOpen processes order.dispute_open events.
// Order dispute opened -> Seller gets notified (buyer initiated dispute),
// then admins holding finance.dispute.resolve are notified so the dispute
// queue is not polling-only.
//
// This is the CANONICAL admin fanout point for disputes. refund.escalated
// also fires in the escalation path (same transaction), but admin fanout
// is intentionally placed here only — order.dispute_open fires in ALL
// dispute paths (direct + refund escalation via MarkDisputeOpen).
func (h *NotificationEventHandler) handleOrderDisputeOpen(ctx context.Context, payload []byte) (notificationInfo, error) {
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

	// 1. Notify SELLER (buyer opened dispute) — primary recipient.
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, buyerID, "order.dispute_open", orderID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert seller notification failed: %w", err)
	}

	h.log.Debug("Order dispute opened notification created",
		zap.String("recipient_id", sellerID.String()),
		zap.String("order_id", orderID.String()),
	)

	// 2. Admin fanout: notify all admins holding finance.dispute.resolve.
	if h.capabilityLister == nil {
		return info, nil
	}

	adminIDs, lErr := h.capabilityLister.ListUsersByCapability(ctx, "finance.dispute.resolve")
	if lErr != nil {
		h.log.Error("order.dispute_open: failed to list admins by capability",
			zap.String("order_id", p.OrderID),
			zap.String("buyer_id", p.BuyerID),
			zap.String("seller_id", p.SellerID),
			zap.String("capability", "finance.dispute.resolve"),
			zap.Error(lErr),
		)
		return notificationInfo{}, fmt.Errorf("order.dispute_open: list admins by capability: %w", lErr)
	}

	if len(adminIDs) == 0 {
		h.log.Warn("order.dispute_open: no admins hold finance.dispute.resolve",
			zap.String("order_id", p.OrderID),
		)
		return info, nil
	}

	// Admin notification data includes order context for review.
	adminData := map[string]interface{}{
		"orderId":  p.OrderID,
		"buyerId":  p.BuyerID,
		"sellerId": p.SellerID,
		"status":   p.Status,
	}

	var delivered, failed int
	for _, adminID := range adminIDs {
		_, insertErr := h.insertNotificationWithPolicy(
			ctx,
			adminID, buyerID,
			"order.dispute_open",
			orderID,
			adminData,
		)
		if insertErr != nil {
			failed++
			h.log.Error("order.dispute_open: admin fanout insert failed",
				zap.String("order_id", p.OrderID),
				zap.String("admin_id", adminID.String()),
				zap.String("capability", "finance.dispute.resolve"),
				zap.Error(insertErr),
			)
			continue
		}
		delivered++
	}

	// Total admin fanout failure → return error so outbox retries.
	if delivered == 0 && failed > 0 {
		return notificationInfo{}, fmt.Errorf("order.dispute_open: all %d admin fanout inserts failed for order %s", failed, p.OrderID)
	}

	return info, nil
}

// =============================================================================
// DISPUTE OPENED HANDLER — D1B
// =============================================================================

// handleDisputeOpened processes dispute.opened events.
//
// Routing logic:
//   - Current runtime only emits the pre-release dispute path.
//   - Legacy/parked post-release payloads are retained for compatibility only.


