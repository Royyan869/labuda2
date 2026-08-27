package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (h *NotificationEventHandler) handleSupportTicketResolved(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p SupportTicketPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	ticketID, err := uuid.Parse(p.TicketID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid ticket_id: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid user_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"ticketId":   p.TicketID,
		"chatRoomId": p.ChatRoomID,
	}

	// Notify USER (admin-resolved support ticket)
	info, err := h.insertNotificationWithPolicy(ctx, userID, uuid.Nil, "support.ticket.resolved", ticketID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleSupportTicketClosed processes support.ticket.closed events.
// Notifies the user that their support ticket has been closed.
func (h *NotificationEventHandler) handleSupportTicketClosed(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p SupportTicketPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	ticketID, err := uuid.Parse(p.TicketID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid ticket_id: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid user_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"ticketId":   p.TicketID,
		"chatRoomId": p.ChatRoomID,
	}

	// Notify USER (support ticket closed)
	info, err := h.insertNotificationWithPolicy(ctx, userID, uuid.Nil, "support.ticket.closed", ticketID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleSupportTicketWaitingUser processes support.ticket_waiting_user events.
// Notifies the user that admin is waiting for their response.
func (h *NotificationEventHandler) handleSupportTicketWaitingUser(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p SupportTicketPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	ticketID, err := uuid.Parse(p.TicketID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid ticket_id: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid user_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"ticketId":   p.TicketID,
		"chatRoomId": p.ChatRoomID,
	}

	// Notify USER (admin is waiting for user reply)
	info, err := h.insertNotificationWithPolicy(ctx, userID, uuid.Nil, "support.ticket_waiting_user", ticketID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleSupportTicketUserResponded processes support.ticket.user_responded events.
// Notifies the assigned admin that the user has replied to their support ticket.
func (h *NotificationEventHandler) handleSupportTicketUserResponded(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p SupportTicketPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	ticketID, err := uuid.Parse(p.TicketID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid ticket_id: %w", err)
	}

	adminID, err := uuid.Parse(p.AdminID)
	if err != nil {
		// No admin assigned — skip notification
		return notificationInfo{}, nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid user_id: %w", err)
	}

	// Navigation data for admin
	data := map[string]interface{}{
		"ticketId":   p.TicketID,
		"chatRoomId": p.ChatRoomID,
	}

	// Notify admin (user replied to their ticket)
	info, err := h.insertNotificationWithPolicy(ctx, adminID, userID, "support.ticket.user_responded", ticketID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	return info, nil
}

// handleSupportTicketCreated processes support.ticket.created events.
// If an admin is assigned, notifies that admin directly.
// If unassigned, fans out to all admins holding support.ticket.claim capability.
func (h *NotificationEventHandler) handleSupportTicketCreated(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p SupportTicketPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	ticketID, err := uuid.Parse(p.TicketID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid ticket_id: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid user_id: %w", err)
	}

	data := map[string]interface{}{
		"ticketId":   p.TicketID,
		"chatRoomId": p.ChatRoomID,
	}

	// If an admin is assigned, notify them via policy-based insertion.
	if p.AdminID != "" {
		adminID, aErr := uuid.Parse(p.AdminID)
		if aErr == nil {
			return h.insertNotificationWithPolicy(
				ctx,
				adminID, userID,
				"support.ticket.created",
				ticketID,
				data,
			)
		}
	}

	// No assigned admin — fanout to all admins holding support.ticket.claim.
	if h.capabilityLister == nil {
		h.log.Warn("support.ticket.created: no assigned admin and no capability lister — skipping notification",
			zap.String("ticket_id", p.TicketID),
			zap.String("category", p.Category),
			zap.String("priority", p.Priority),
		)
		return notificationInfo{}, nil
	}

	adminIDs, err := h.capabilityLister.ListUsersByCapability(ctx, "support.ticket.claim")
	if err != nil {
		h.log.Error("support.ticket.created: failed to list admins by capability",
			zap.String("ticket_id", p.TicketID),
			zap.Error(err),
		)
		return notificationInfo{}, fmt.Errorf("list admins by capability: %w", err)
	}

	if len(adminIDs) == 0 {
		h.log.Warn("support.ticket.created: no admins hold support.ticket.claim",
			zap.String("ticket_id", p.TicketID),
		)
		return notificationInfo{}, nil
	}

	// Fanout: one notification per admin. insertNotificationWithPolicy handles
	// policy (account status, blocks) per recipient. DISTINCT in the repository
	// query guarantees no duplicate capabilities produce duplicate notifications.
	var lastInfo notificationInfo
	var delivered, failed int
	for _, adminID := range adminIDs {
		info, insertErr := h.insertNotificationWithPolicy(
			ctx,
			adminID, userID,
			"support.ticket.created",
			ticketID,
			data,
		)
		if insertErr != nil {
			failed++
			h.log.Error("support.ticket.created: fanout insert failed",
				zap.String("ticket_id", p.TicketID),
				zap.String("admin_id", adminID.String()),
				zap.String("capability", "support.ticket.claim"),
				zap.Error(insertErr),
			)
			// Continue fanout — partial delivery is better than none.
			continue
		}
		delivered++
		lastInfo = info
	}

	// If every insert failed, return error so outbox retries the event.
	// Partial success is accepted — retried inserts are DB-deduplicated.
	if delivered == 0 && failed > 0 {
		return notificationInfo{}, fmt.Errorf("support.ticket.created: all %d fanout inserts failed for ticket %s", failed, p.TicketID)
	}

	return lastInfo, nil
}

// =============================================================================
// NEGOTIATION NOTIFICATION HANDLERS
// =============================================================================

// handleNegotiationStarted processes negotiation.started events.
// Notifies the seller that a buyer has started a negotiation.
//
// P2-A FIX: The negotiation.started outbox event is emitted before the chat consumer
// creates the room, so chat_room_id is absent from the payload. We enrich it here via
// a fail-soft DB lookup — notification is always delivered; deeplink is best-effort.


