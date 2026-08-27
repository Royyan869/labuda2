package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NotificationType represents the type of notification.
type NotificationType string

const (
	// TypeUserFollowed is emitted when a user follows another user.
	TypeUserFollowed NotificationType = "user.followed"
	// TypeContentLiked is emitted when a user likes content.
	TypeContentLiked NotificationType = "content.liked"
	// TypeComment is emitted when a user comments on content.
	TypeComment NotificationType = "comment"
	// TypeCommentReply is emitted when a user replies to a comment.
	TypeCommentReply NotificationType = "comment_reply"
	// TypeSellerResponse is emitted when a seller responds to a request with a for_sale reference.
	TypeSellerResponse NotificationType = "seller.response"
	// TypeChatMessage is emitted when a user sends a chat message.
	TypeChatMessage NotificationType = "chat_message"

	// =============================================================================
	// ORDER NOTIFICATION TYPES
	// =============================================================================
	// TypeOrderCreated is emitted when an order is created (buyer places order).
	TypeOrderCreated NotificationType = "order.created"
	// TypeOrderCreatedBuyer is emitted when buyer creates an order (buyer confirmation).
	TypeOrderCreatedBuyer NotificationType = "order.created.buyer"
	// TypeOrderPaid is emitted when an order payment is confirmed.
	TypeOrderPaid NotificationType = "order.paid"
	// TypeOrderPaidBuyer is emitted when buyer's payment is confirmed (buyer confirmation).
	TypeOrderPaidBuyer NotificationType = "order.paid.buyer"
	// TypeOrderShipped is emitted when a seller marks an order as shipped.
	TypeOrderShipped NotificationType = "order.shipped"
	// TypeOrderCompleted is emitted when an order is completed and escrow is released.
	TypeOrderCompleted NotificationType = "order.completed"
	// TypeOrderCancelled is emitted when an order is cancelled.
	TypeOrderCancelled NotificationType = "order.cancelled"
	// TypeOrderExpired is emitted when an order payment expires.
	TypeOrderExpired NotificationType = "order.expired"
	// TypeOrderRefunded is emitted when an order is refunded.
	TypeOrderRefunded NotificationType = "order.refunded"
	// TypeOrderPartiallyRefunded is emitted when an order is partially refunded.
	TypeOrderPartiallyRefunded NotificationType = "order.partially_refunded"
	// TypeOrderDisputeOpen is emitted when a buyer opens a dispute for an order.
	TypeOrderDisputeOpen NotificationType = "order.dispute_open"

	// =============================================================================
	// REFUND NOTIFICATION TYPES
	// =============================================================================
	// TypeRefundOpened is emitted when a buyer requests a refund.
	TypeRefundOpened NotificationType = "refund.opened"
	// TypeRefundEscalated is emitted when a refund is escalated to a dispute.
	TypeRefundEscalated NotificationType = "refund.escalated"

	// =============================================================================
	// DISPUTE NOTIFICATION TYPES
	// =============================================================================
	// TypeDisputeResolved is emitted when an admin resolves a dispute.
	TypeDisputeResolved NotificationType = "dispute.resolved"
)

// Notification represents a user notification.
type Notification struct {
	ID          uuid.UUID
	RecipientID uuid.UUID // User who receives the notification
	ActorID     uuid.UUID // User who triggered the notification
	Type        NotificationType
	EntityID    uuid.UUID              // ID of the related entity (user or content)
	EntityType  string                 // Type of the entity (comment, message, order, content, user)
	Data        map[string]interface{} // Additional navigation payload for mobile
	IsRead      bool
	CreatedAt   time.Time
}

// ErrNotificationNotFound is returned when a notification is not found.
type ErrNotificationNotFound struct {
	NotificationID uuid.UUID
}

func (e *ErrNotificationNotFound) Error() string {
	return fmt.Sprintf("notification not found: %s", e.NotificationID)
}

// NewNotification creates a new notification with optional data payload.
func NewNotification(recipientID, actorID uuid.UUID, notificationType NotificationType, entityID uuid.UUID, data map[string]interface{}) *Notification {
	return &Notification{
		ID:          uuid.New(),
		RecipientID: recipientID,
		ActorID:     actorID,
		Type:        notificationType,
		EntityID:    entityID,
		Data:        data,
		IsRead:      false,
		CreatedAt:   time.Now(),
	}
}

// MarkAsRead marks the notification as read.
func (n *Notification) MarkAsRead() {
	n.IsRead = true
}
