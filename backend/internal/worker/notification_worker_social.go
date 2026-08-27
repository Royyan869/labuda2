package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/events"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

func (h *NotificationEventHandler) handleUserFollowed(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p NotificationPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	actorID, err := uuid.Parse(p.ActorID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid actor_id: %w", err)
	}

	recipientID, err := uuid.Parse(p.RecipientID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid recipient_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"userId": actorID.String(), // For navigation to user profile
	}

	// Use policy-based insertion (handles account status, block filtering)
	return h.insertNotificationWithPolicy(
		ctx,
		recipientID,
		actorID,
		events.EventUserFollowed,
		actorID, // For follow, entity_id is the actor
		data,
	)
}

// handleContentLiked processes content.liked events.
// Social governance: applyPolicyLayer via insertNotificationWithPolicy.
// shouldFilterNotification removed; recipient lifecycle, actor lifecycle, block, and account
// status are all enforced at delivery time. Push and in-app governed by the same policy result.
//
// LIKE-OCCURRENCE GUARD: before inserting, re-confirm the like row still
// exists for (content_id, actor_id) inside the same transaction. An UNLIKE
// that raced this event (row removed, notification scrubbed) therefore never
// resurrects a stale like notification; a LIKE after an UNLIKE carries a new
// occurrence and is delivered normally.
func (h *NotificationEventHandler) handleContentLiked(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p ContentLikedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	actorID, err := uuid.Parse(p.ActorID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid actor_id: %w", err)
	}

	recipientID, err := uuid.Parse(p.RecipientID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid recipient_id: %w", err)
	}

	contentID, err := uuid.Parse(p.ContentID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid content_id: %w", err)
	}

	// Don't notify self (user liking their own content)
	if actorID == recipientID {
		h.log.Debug("Skipping self-notification for content like",
			zap.String("user_id", actorID.String()),
		)
		return notificationInfo{}, nil
	}

	data := map[string]interface{}{
		"targetId":   contentID.String(),
		"targetType": "content",
	}

	likeActive := func(tx dbpkg.Tx) (bool, error) {
		query := `SELECT EXISTS(SELECT 1 FROM content_likes WHERE content_id = $1 AND user_id = $2`
		args := []any{contentID, actorID}
		if p.OccurrenceAt != "" {
			// Match the CURRENT occurrence: only the like row for this exact
			// occurrence may deliver a notification. A stale event whose
			// occurrence was unliked-and-replaced never matches and is skipped.
			query += ` AND created_at = $3`
			args = append(args, p.OccurrenceAt)
		}
		query += `)`
		var exists bool
		err := tx.QueryRow(ctx, query, args...).Scan(&exists)
		if err != nil {
			return false, fmt.Errorf("check like state failed: %w", err)
		}
		return exists, nil
	}

	return h.insertNotificationWithPolicy(
		ctx,
		recipientID,
		actorID,
		events.EventContentLiked,
		contentID,
		data,
		likeActive,
	)
}

// handleCommentCreated processes comment.created events.
// Social governance: applyPolicyLayer via insertNotificationWithPolicy.
// shouldFilterNotification removed; recipient lifecycle, actor lifecycle, block, and account
// status are all enforced at delivery time. Push and in-app governed by the same policy result.
// Recipient is resolved from DB (not present in outbox payload); SQL enrichment precedes policy gate.
func (h *NotificationEventHandler) handleCommentCreated(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p CommentCreatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	authorID, err := uuid.Parse(p.AuthorID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid author_id: %w", err)
	}

	commentID, err := uuid.Parse(p.CommentID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid comment_id: %w", err)
	}

	contentID, err := uuid.Parse(p.ContentID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid content_id: %w", err)
	}

	// Resolve recipient from DB; not present in outbox payload.
	var recipientID uuid.UUID
	if err := h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return tx.QueryRow(ctx, `SELECT author_id FROM contents WHERE id = $1`, contentID).
			Scan(&recipientID)
	}); err != nil {
		return notificationInfo{}, fmt.Errorf("get content owner failed: %w", err)
	}

	// Don't notify self (additional safety check)
	if authorID == recipientID {
		h.log.Debug("Skipping self-notification for comment created",
			zap.String("user_id", authorID.String()),
		)
		return notificationInfo{}, nil
	}

	data := map[string]interface{}{
		"targetId":   contentID.String(),
		"targetType": "content",
		"commentId":  commentID.String(),
	}

	return h.insertNotificationWithPolicy(
		ctx,
		recipientID,
		authorID,
		"comment",
		contentID,
		data,
	)
}

// handleCommentReply processes comment.reply events.
// Social governance: applyPolicyLayer via insertNotificationWithPolicy.
// shouldFilterNotification removed; recipient lifecycle, actor lifecycle, block, and account
// status are all enforced at delivery time. Push and in-app governed by the same policy result.
// Content-type SQL enrichment removed; navigation now uses canonical content targets.
func (h *NotificationEventHandler) handleCommentReply(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p CommentReplyPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	authorID, err := uuid.Parse(p.AuthorID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid author_id: %w", err)
	}

	commentID, err := uuid.Parse(p.CommentID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid comment_id: %w", err)
	}

	parentID, err := uuid.Parse(p.ParentID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid parent_id: %w", err)
	}

	parentAuthorID, err := uuid.Parse(p.ParentAuthorID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid parent_author_id: %w", err)
	}

	contentID, err := uuid.Parse(p.ContentID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid content_id: %w", err)
	}

	// Don't notify self (additional safety check - replying to own comment)
	if authorID == parentAuthorID {
		h.log.Debug("Skipping self-notification for comment reply",
			zap.String("user_id", authorID.String()),
		)
		return notificationInfo{}, nil
	}

	data := map[string]interface{}{
		"targetId":   contentID.String(),
		"targetType": "content",
		"commentId":  commentID.String(),
		"parentId":   parentID.String(),
	}

	return h.insertNotificationWithPolicy(
		ctx,
		parentAuthorID,
		authorID,
		"comment_reply",
		contentID,
		data,
	)
}

// handleSellerResponse processes seller.response events.
// Social governance: applyPolicyLayer via insertNotificationWithPolicy.
// shouldFilterNotification removed; recipient lifecycle, actor lifecycle, block, and account
// status are all enforced at delivery time. Push and in-app governed by the same policy result.
// seller.response is Social category; no CommerceCritical reclassification.
func (h *NotificationEventHandler) handleSellerResponse(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p SellerResponsePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	commentID, err := uuid.Parse(p.CommentID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid comment_id: %w", err)
	}

	requestCreatorID, err := uuid.Parse(p.RequestCreatorID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid request_creator_id: %w", err)
	}

	contentID, err := uuid.Parse(p.ContentID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid content_id: %w", err)
	}

	forSaleID, err := uuid.Parse(p.ForSaleID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid for_sale_id: %w", err)
	}

	// Don't notify self (seller responding to own request - edge case)
	if sellerID == requestCreatorID {
		h.log.Debug("Skipping self-notification for seller response",
			zap.String("user_id", sellerID.String()),
		)
		return notificationInfo{}, nil
	}

	data := map[string]interface{}{
		"targetId":   contentID.String(),
		"targetType": "request",
		"commentId":  commentID.String(),
		"forSaleId":  forSaleID.String(),
	}

	return h.insertNotificationWithPolicy(
		ctx,
		requestCreatorID,
		sellerID,
		"seller.response",
		contentID,
		data,
	)
}

// handleChatMessage processes chat.message.sent events.
// CHAT-4: Canonical delivery-time governance via applyPolicyLayer.
// - Sender/recipient lifecycle evaluated at delivery time (not at enqueue time).
// - Block is bidirectional; suspended/banned/deleted lifecycle gates delivery.
// - Push and in-app use the same policy decision.
func (h *NotificationEventHandler) handleChatMessage(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p ChatMessagePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	senderID, err := uuid.Parse(p.SenderID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid sender_id: %w", err)
	}

	messageID, err := uuid.Parse(p.MessageID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid message_id: %w", err)
	}

	recipientID, err := uuid.Parse(p.RecipientID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid recipient_id: %w", err)
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid room_id: %w", err)
	}

	// Don't notify self (shouldn't happen due to service validation, but safety check)
	if senderID == recipientID {
		h.log.Debug("Skipping self-notification for chat message",
			zap.String("user_id", senderID.String()),
		)
		return notificationInfo{}, nil
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"chatId":    roomID.String(),
		"messageId": messageID.String(),
	}

	// Canonical policy-based insertion: applyPolicyLayer evaluates sender/recipient
	// lifecycle, block (bidirectional), and account status at delivery time.
	// Push and in-app both consume the same policy decision from insertNotificationWithPolicy.
	return h.insertNotificationWithPolicy(
		ctx,
		recipientID,
		senderID,
		"chat_message",
		roomID,
		data,
	)
}

// =============================================================================
// ORDER EVENT HANDLERS
// =============================================================================

// handleOrderCreated processes order.created events.
// Buyer creates order -> Both buyer and seller get notified.
// Seller: "Order Baru" (new order to fulfill)
// =============================================================================
// SOCIAL GRAPH CLEANUP HANDLERS
// =============================================================================

// socialNotificationTypes lists notification types that belong to the Social category.
// These are removed on block; commerce/moderation/support notifications are preserved.
//
// SOURCE OF TRUTH: policy/category.go GetCategory() Social case list.
var socialNotificationTypes = []string{
	"user.followed",
	"content.liked",
	"comment",
	"comment_reply",
	"chat_message",
	"seller.response",
}

// handleUserBlocked removes all SOCIAL-category notification history between two users
// (bidirectional) when a block relationship is created.
//
// PRESERVES: order.*, withdrawal.*, verification.*, dispute.*, negotiation.*,
// moderation.*, support.* — these are obligation/legal records.
//
// REMOVES: user.followed, content.liked, comment, comment_reply, chat_message,
// seller.response — these are social interactions that a block should erase.
//
// This handler does NOT create a new notification (block is silent to the blocked party).
func (h *NotificationEventHandler) handleUserBlocked(ctx context.Context, payload []byte) error {
	var p NotificationPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("handleUserBlocked: unmarshal payload failed: %w", err)
	}

	blockerID, err := uuid.Parse(p.ActorID)
	if err != nil {
		return fmt.Errorf("handleUserBlocked: invalid actor_id: %w", err)
	}

	blockedID, err := uuid.Parse(p.RecipientID)
	if err != nil {
		return fmt.Errorf("handleUserBlocked: invalid recipient_id: %w", err)
	}

	// Delete social notifications bidirectionally (A→B and B→A).
	// Commerce/moderation/support notifications are intentionally preserved.
	err = h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		query := `
			DELETE FROM notifications
			WHERE ((actor_id = $1 AND recipient_id = $2) OR (actor_id = $2 AND recipient_id = $1))
			  AND type = ANY($3)
		`
		_, execErr := tx.Exec(ctx, query, blockerID, blockedID, socialNotificationTypes)
		return execErr
	})
	if err != nil {
		return fmt.Errorf("handleUserBlocked: delete social notifications failed: %w", err)
	}

	h.log.Info("Block notification cleanup: social notifications removed",
		zap.String("blocker_id", blockerID.String()),
		zap.String("blocked_id", blockedID.String()),
		zap.Int("social_types_count", len(socialNotificationTypes)),
	)

	return nil
}

// handleUserUnfollowed removes the specific "user.followed" notification that was
// created when the follow relationship was established.
//
// Only removes notifications in the actor→recipient direction (unfollow is directional,
// unlike block which is bidirectional).
//
// This handler does NOT create a new notification (unfollow is silent).
func (h *NotificationEventHandler) handleUserUnfollowed(ctx context.Context, payload []byte) error {
	var p NotificationPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("handleUserUnfollowed: unmarshal payload failed: %w", err)
	}

	// actor_id = the user who unfollowed (was the follower)
	// recipient_id = the user who was followed (received the original notification)
	followerID, err := uuid.Parse(p.ActorID)
	if err != nil {
		return fmt.Errorf("handleUserUnfollowed: invalid actor_id: %w", err)
	}

	followedID, err := uuid.Parse(p.RecipientID)
	if err != nil {
		return fmt.Errorf("handleUserUnfollowed: invalid recipient_id: %w", err)
	}

	// Delete only the "user.followed" notification from follower→followed direction.
	err = h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		query := `
			DELETE FROM notifications
			WHERE actor_id = $1 AND recipient_id = $2 AND type = $3
		`
		_, execErr := tx.Exec(ctx, query, followerID, followedID, "user.followed")
		return execErr
	})
	if err != nil {
		return fmt.Errorf("handleUserUnfollowed: delete follow notification failed: %w", err)
	}

	h.log.Info("Unfollow notification cleanup: user.followed notification removed",
		zap.String("follower_id", followerID.String()),
		zap.String("followed_id", followedID.String()),
	)

	return nil
}
