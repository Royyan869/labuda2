package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

func (h *NotificationEventHandler) handleModerationContentRemoved(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p ModerationRemovedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	resourceID, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid resource_id: %w", err)
	}

	// Get content owner to notify
	var recipientID uuid.UUID
	err = h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return tx.QueryRow(ctx, "SELECT author_id FROM contents WHERE id = $1", resourceID).
			Scan(&recipientID)
	})
	if err != nil {
		return notificationInfo{}, fmt.Errorf("get content owner failed: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"targetId":   p.ResourceID,
		"targetType": "post",
	}

	// Notify content owner (system-initiated moderation; Moderation category bypasses block checks)
	info, err := h.insertNotificationWithPolicy(ctx, recipientID, uuid.Nil, "moderation.content.removed", resourceID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Moderation content removed notification created",
		zap.String("recipient_id", recipientID.String()),
		zap.String("resource_id", p.ResourceID),
	)

	return info, nil
}

// handleModerationCommentRemoved processes moderation.comment.removed events.
// Notifies the comment author that their comment was removed due to moderation.
func (h *NotificationEventHandler) handleModerationCommentRemoved(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p ModerationRemovedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	commentID, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid resource_id: %w", err)
	}

	// Get comment author to notify
	var recipientID uuid.UUID
	var contentID uuid.UUID
	err = h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return tx.QueryRow(ctx, "SELECT author_id, content_id FROM comments WHERE id = $1", commentID).
			Scan(&recipientID, &contentID)
	})
	if err != nil {
		return notificationInfo{}, fmt.Errorf("get comment author failed: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"targetId":   contentID.String(),
		"targetType": "post",
		"commentId":  p.ResourceID,
	}

	// Notify comment author (system-initiated moderation; Moderation category bypasses block checks)
	info, err := h.insertNotificationWithPolicy(ctx, recipientID, uuid.Nil, "moderation.comment.removed", commentID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Moderation comment removed notification created",
		zap.String("recipient_id", recipientID.String()),
		zap.String("comment_id", p.ResourceID),
	)

	return info, nil
}

// handleModerationContentRestored processes moderation.content.restored events.
// Notifies the content owner that their content was restored after appeal.
func (h *NotificationEventHandler) handleModerationContentRestored(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p ModerationRestoredPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	resourceID, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid resource_id: %w", err)
	}

	// Get content owner to notify
	var recipientID uuid.UUID
	err = h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return tx.QueryRow(ctx, "SELECT author_id FROM contents WHERE id = $1", resourceID).
			Scan(&recipientID)
	})
	if err != nil {
		return notificationInfo{}, fmt.Errorf("get content owner failed: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"targetId":   p.ResourceID,
		"targetType": "post",
	}

	// Notify content owner (appeal accepted; Moderation category bypasses block checks)
	info, err := h.insertNotificationWithPolicy(ctx, recipientID, uuid.Nil, "moderation.content.restored", resourceID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleModerationCommentRestored processes moderation.comment.restored events.
// Notifies the comment author that their comment was restored after appeal.
func (h *NotificationEventHandler) handleModerationCommentRestored(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p ModerationRestoredPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	commentID, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid resource_id: %w", err)
	}

	// Get comment author to notify
	var recipientID uuid.UUID
	var contentID uuid.UUID
	err = h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return tx.QueryRow(ctx, "SELECT author_id, content_id FROM comments WHERE id = $1", commentID).
			Scan(&recipientID, &contentID)
	})
	if err != nil {
		return notificationInfo{}, fmt.Errorf("get comment author failed: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"targetId":   contentID.String(),
		"targetType": "post",
		"commentId":  p.ResourceID,
	}

	// Notify comment author (appeal accepted; Moderation category bypasses block checks)
	info, err := h.insertNotificationWithPolicy(ctx, recipientID, uuid.Nil, "moderation.comment.restored", commentID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleModerationForSaleRemoved processes moderation.for_sale.removed events.
// Notifies the fixed-price sale seller that their sale was removed due to moderation.
func (h *NotificationEventHandler) handleModerationForSaleRemoved(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p ModerationRemovedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	forSaleID, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid resource_id: %w", err)
	}

	// Get fixed-price sale seller to notify
	var sellerID uuid.UUID
	err = h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return tx.QueryRow(ctx, "SELECT seller_id FROM for_sales WHERE id = $1", forSaleID).
			Scan(&sellerID)
	})
	if err != nil {
		return notificationInfo{}, fmt.Errorf("get fixed-price sale seller failed: %w", err)
	}

	// Navigation data for mobile — fixed-price sale is withdrawn so no deep-link to it
	data := map[string]interface{}{
		"targetId":   p.ResourceID,
		"targetType": "for_sale",
	}

	// Notify fixed-price sale seller (system-initiated moderation; Moderation category bypasses block checks)
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, uuid.Nil, "moderation.for_sale.removed", forSaleID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Moderation fixed-price sale removed notification created",
		zap.String("recipient_id", sellerID.String()),
		zap.String("for_sale_id", p.ResourceID),
	)

	return info, nil
}

// handleModerationForSaleRestored processes moderation.for_sale.restored events.
// Notifies the fixed-price sale seller that their appeal was approved and they can re-list.
func (h *NotificationEventHandler) handleModerationForSaleRestored(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p ModerationRestoredPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	forSaleID, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid resource_id: %w", err)
	}

	// Get fixed-price sale seller to notify
	var sellerID uuid.UUID
	err = h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return tx.QueryRow(ctx, "SELECT seller_id FROM for_sales WHERE id = $1", forSaleID).
			Scan(&sellerID)
	})
	if err != nil {
		return notificationInfo{}, fmt.Errorf("get fixed-price sale seller failed: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"targetId":   p.ResourceID,
		"targetType": "for_sale",
	}

	// Notify fixed-price sale seller (appeal accepted; Moderation category bypasses block checks)
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, uuid.Nil, "moderation.for_sale.restored", forSaleID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Moderation fixed-price sale restored notification created",
		zap.String("recipient_id", sellerID.String()),
		zap.String("for_sale_id", p.ResourceID),
	)

	return info, nil
}

// handleModerationUserSuspended processes moderation.user.suspended events.
// Notifies the user that their account was suspended due to moderation.
func (h *NotificationEventHandler) handleModerationUserSuspended(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p ModerationRemovedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	userID, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid resource_id: %w", err)
	}

	// Notify suspended user (system-initiated; Moderation category bypasses block + suspended filters)
	info, err := h.insertNotificationWithPolicy(ctx, userID, uuid.Nil, "moderation.user.suspended", userID, nil)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Moderation user suspended notification created",
		zap.String("user_id", userID.String()),
	)

	return info, nil
}

// handleModerationUserRestored processes moderation.user.restored events.
// Notifies the user that their account was restored after appeal.
func (h *NotificationEventHandler) handleModerationUserRestored(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p ModerationRestoredPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	userID, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid resource_id: %w", err)
	}

	// Notify restored user (appeal accepted; Moderation category bypasses block checks)
	info, err := h.insertNotificationWithPolicy(ctx, userID, uuid.Nil, "moderation.user.restored", userID, nil)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Moderation user restored notification created",
		zap.String("user_id", userID.String()),
	)

	return info, nil
}

// handleModerationWarningIssued processes moderation.warning.issued events.
// Notifies the user that they received a warning for policy violations.
func (h *NotificationEventHandler) handleModerationWarningIssued(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p struct {
		WarningID string `json:"warning_id"`
		UserID    string `json:"user_id"`
		Level     string `json:"level"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid user_id: %w", err)
	}

	warningID, err := uuid.Parse(p.WarningID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid warning_id: %w", err)
	}

	// Notify warned user (system-initiated; Moderation category bypasses block + suspended filters)
	info, err := h.insertNotificationWithPolicy(ctx, userID, uuid.Nil, "moderation.warning.issued", warningID, map[string]interface{}{
		"level":  p.Level,
		"reason": p.Reason,
	})
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	h.log.Debug("Moderation warning issued notification created",
		zap.String("user_id", userID.String()),
		zap.String("warning_id", warningID.String()),
		zap.String("level", p.Level),
	)

	return info, nil
}

// =============================================================================
// SUPPORT NOTIFICATION HANDLERS
// =============================================================================

// handleSupportTicketResolved processes support.ticket.resolved events.
// Notifies the user that their support ticket has been resolved.


