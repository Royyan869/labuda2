package policy

import (
	"strings"
)

// NotificationCategory defines the delivery policy for notification types.
type NotificationCategory string

const (
	// CommerceCritical: MUST deliver - affects user's money, goods, or account status.
	// Bypasses blocks and account filters.
	CommerceCritical NotificationCategory = "commerce_critical"

	// Moderation: MUST deliver - affects user's content standing.
	// Bypasses blocks and account filters.
	Moderation NotificationCategory = "moderation"

	// Social: OPTIONAL - filtered by blocks and account status.
	// Respects user blocks and account status.
	Social NotificationCategory = "social"

	// Marketing: OPTIONAL - filtered by account status and preferences.
	// Reserved for future promotional content.
	Marketing NotificationCategory = "marketing"

	// Blocked: Explicitly blocked - should never be delivered.
	Blocked NotificationCategory = "blocked"
)

// GetCategory returns the policy category for a notification type.
//
// SAFE DEFAULT: Returns Social for unknown types to prevent over-delivery.
// Explicitly listed types are categorized; everything else defaults to Social
// (which means it will be filtered by blocks and account status).
func GetCategory(notifyType string) NotificationCategory {
	switch {
	// ============================================================================
	// COMMERCE CRITICAL - affects money, goods, or account status
	// ============================================================================
	case strings.HasPrefix(notifyType, "order."),
		strings.HasPrefix(notifyType, "withdrawal."),
		strings.HasPrefix(notifyType, "verification."),
		strings.HasPrefix(notifyType, "seller.verification."), // seller lifecycle (distinct from document-level verification.*)
		strings.HasPrefix(notifyType, "seller.subscription."), // subscription lifecycle — system-initiated (uuid.Nil actor)
		strings.HasPrefix(notifyType, "seller.tier."),        // B1: reputation tier change — affects seller trust and market authority
		strings.HasPrefix(notifyType, "dispute."),
		strings.HasPrefix(notifyType, "refund."), // D1A
		strings.HasPrefix(notifyType, "negotiation."),
		strings.HasPrefix(notifyType, "external_product.review."), // review decision — owner must not miss approval/rejection
		notifyType == "auction.bid.placed",                    // seller must always receive bid notifications
		notifyType == "auction.waiting_settlement",            // winner must always receive claim notification
		notifyType == "auction.seller_has_winner",             // seller must know their auction has a winner pending claim
		notifyType == "auction.ended_no_winner",               // seller must know their auction closed without a winner
		notifyType == "auction.settlement_failed.buyer",       // buyer must know their settlement failed (violation/restriction)
		notifyType == "auction.settlement_failed.seller_default", // seller must know their quote default caused DRAFT
		notifyType == "auction.settlement_failed.relistable",  // seller must know the auction is back in DRAFT and relistable
		notifyType == "support.ticket.created",    // admin must see all tickets regardless of submitter status
		notifyType == "support.ticket.resolved",
		notifyType == "support.ticket.closed",
		notifyType == "support.ticket_waiting_user",
		notifyType == "support.ticket.user_responded", // admin must see user replies regardless of status
		notifyType == "money.refund_failed":            // admin-only: gateway refund failure requires immediate attention
		return CommerceCritical

	// ============================================================================
	// MODERATION - affects content standing
	// ============================================================================
	case strings.HasPrefix(notifyType, "moderation."):
		return Moderation

	// ============================================================================
	// SOCIAL - filtered by blocks and account status
	// ============================================================================
	case notifyType == "user.followed",
		notifyType == "content.liked",
		notifyType == "comment",
		notifyType == "comment_reply",
		notifyType == "chat_message",
		notifyType == "seller.response":
		return Social

	// ============================================================================
	// SAFE DEFAULT - prevent over-delivery
	// ============================================================================
	// Unknown notification types default to Social (filtered), NOT CommerceCritical.
	// This ensures new notification types must be explicitly categorized,
	// preventing accidental over-delivery to blocked/suspended users.
	default:
		return Social
	}
}

// IsAllowedForBlockedUser returns true if the notification type should be
// delivered even when the recipient has blocked the actor.
func IsAllowedForBlockedUser(category NotificationCategory) bool {
	switch category {
	case CommerceCritical, Moderation:
		return true
	case Social, Marketing:
		return false
	default:
		return false
	}
}

// IsAllowedForSuspendedUser returns true if the notification type should be
// delivered to a suspended or banned user.
func IsAllowedForSuspendedUser(category NotificationCategory) bool {
	switch category {
	case CommerceCritical, Moderation:
		return true
	case Social, Marketing:
		return false
	default:
		return false
	}
}

// RequiresPush returns true if the notification type should trigger
// a push notification attempt.
//
// PRIORITY FILTER (FCM IMPLEMENTATION):
// Only sends push for high-priority notifications to reduce noise:
// - dispute.* (dispute updates)
// - withdrawal.* (payout/withdrawal updates)
// - support.* (support ticket updates)
//
// Other notification types are in-app only.
func RequiresPush(category NotificationCategory) bool {
	// All categories except Marketing require push attempts
	// Marketing can be added later with user preferences
	return category != Marketing
}

// RequiresPushByType returns true if the specific notification type should
// trigger a push notification attempt.
//
// PRIORITY FILTER (FCM IMPLEMENTATION):
// Only sends push for high-priority notifications to reduce noise.
// This is more granular than RequiresPush which works at category level.
func RequiresPushByType(notifyType string) bool {
	// Priority: Dispute notifications
	if strings.HasPrefix(notifyType, "dispute.") {
		return true
	}

	// Priority: Refund lifecycle notifications (D1A)
	if strings.HasPrefix(notifyType, "refund.") {
		return true
	}

	// Priority: Withdrawal/Payout notifications
	if strings.HasPrefix(notifyType, "withdrawal.") {
		return true
	}

	// Priority: Support ticket notifications
	if strings.HasPrefix(notifyType, "support.") {
		return true
	}

	// Priority: Chat messages — direct user communication requires push.
	// Device/user push preferences may suppress actual FCM delivery; governance
	// allows push and device settings gate the final send.
	if notifyType == "chat_message" {
		return true
	}

	// Priority: All order events — money-affecting, users must not miss.
	if strings.HasPrefix(notifyType, "order.") {
		return true
	}

	// Priority: Verification lifecycle events — seller account status.
	if strings.HasPrefix(notifyType, "verification.") {
		return true
	}

	// Priority: Seller verification lifecycle (separate prefix from document-level verification.*).
	if strings.HasPrefix(notifyType, "seller.verification.") {
		return true
	}

	// Priority: Seller subscription — seller must not miss expiry notifications.
	if strings.HasPrefix(notifyType, "seller.subscription.") {
		return true
	}

	// Priority: Seller tier changes — seller must know when their tier rises or falls.
	// Downgrade materially affects seller visibility and trust authority.
	if strings.HasPrefix(notifyType, "seller.tier.") {
		return true
	}

	// Priority: Auction bids — seller must not miss new bids on their auctions.
	if notifyType == "auction.bid.placed" {
		return true
	}

	// Priority: Auction won — winner must not miss their 24h claim window.
	if notifyType == "auction.waiting_settlement" {
		return true
	}

	// Priority: Auction seller has winner — seller must know their auction has a
	// pending claim so they can prepare for the incoming order.
	if notifyType == "auction.seller_has_winner" {
		return true
	}

	// Priority: Auction ended without winner — seller must know so they can re-list.
	if notifyType == "auction.ended_no_winner" {
		return true
	}

	// Priority: Auction settlement failure — buyer/seller must know the outcome
	// (violation/restriction applied, auction returned to DRAFT).
	if strings.HasPrefix(notifyType, "auction.settlement_failed") {
		return true
	}

	// Priority: Negotiation events — active commerce communication.
	if strings.HasPrefix(notifyType, "negotiation.") {
		return true
	}

	// Priority: Gateway refund failure — admin must not miss critical money failures.
	if notifyType == "money.refund_failed" {
		return true
	}

	// Priority: External product review decisions — owner must not miss approval/rejection.
	if strings.HasPrefix(notifyType, "external_product.review.") {
		return true
	}

	// Priority: Moderation user-awareness events — suspension/warning/restoration
	// require the user to be notified even when the app is closed.
	// Removal events (content/fixed-price sale/comment.removed) are in-app only —
	// they inform but do not require immediate disruption.
	if notifyType == "moderation.user.suspended" ||
		notifyType == "moderation.warning.issued" ||
		notifyType == "moderation.content.restored" ||
		notifyType == "moderation.for_sale.restored" ||
		notifyType == "moderation.auction.restored" {
		return true
	}

	// All other types: in-app only, no push
	return false
}


