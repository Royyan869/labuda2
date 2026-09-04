package events

// Domain event constants for the platform.
// Centralized event naming ensures consistency across services.
const (
	// Fixed-price sale events
	EventForSaleCreated   = "for_sale.created"
	EventForSaleUpdated   = "for_sale.updated"
	EventForSaleWithdrawn = "for_sale.withdrawn"
	EventForSaleSold      = "for_sale.sold"
	EventForSalePublished = "for_sale.published"

	// Order events
	EventOrderCreated          = "order.created"
	EventOrderPaid             = "order.paid"
	EventOrderShipped          = "order.shipped"
	EventOrderCompleted        = "order.completed"
	EventOrderCancelled        = "order.cancelled"
	EventOrderCancelledTimeout = "order.cancelled_timeout"
	EventOrderExpired          = "order.expired"
	EventOrderRefunded         = "order.refunded"

	// EventOrderChatLinkRequested is emitted by the order tx and consumed by
	// the chat domain to link the new order to the buyer↔seller canonical
	// direct room (LATEST ACTIVE ORDER RULE). Decouples chat_rooms mutation
	// from the canonical commerce transaction. Eventual consistency: chat link
	// failure MUST NOT roll back the order.
	EventOrderChatLinkRequested = "order.chat_link_requested"

	// EventMoneyReleased is emitted when a gateway-funded escrow is released
	// to the seller. Carries gross / commission / sellerNet amounts so finance
	// consumers can reconcile without re-deriving from the order.
	EventMoneyReleased = "money.released"

	// Comment events
	EventCommentCreated = "comment.created"
	EventCommentUpdated = "comment.updated"
	EventCommentDeleted = "comment.deleted"

	// Social interaction events
	EventUserFollowed   = "user.followed"
	EventUserUnfollowed = "user.unfollowed"
	EventUserBlocked    = "user.blocked"
	EventContentLiked   = "content.liked"
	EventContentMentioned = "content.mentioned"
	EventCommentReply   = "comment.reply"
	EventSellerResponse = "seller.response"
	EventAuctionResponse = "auction.response"

	// Presence events
	EventUserPresenceLastSeenRecord = "presence.last_seen_record"

	// Account lifecycle events
	EventUserDeleted = "user.deleted"

	// Seller subscription events
	EventSellerSubscriptionActivated = "seller.subscription.activated"
	EventSellerSubscriptionExpired   = "seller.subscription.expired"
	EventSellerSubscriptionExpiring  = "seller.subscription.expiring"

	// Dispute events
	EventDisputeOpened   = "dispute.opened"
	EventDisputeResolved = "dispute.resolved"
)
