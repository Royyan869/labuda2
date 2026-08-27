package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	notificationrepo "github.com/labuda/backend/internal/interaction/notification"
	notificationEntity "github.com/labuda/backend/internal/interaction/notification/entity"
	notificationrepoImpl "github.com/labuda/backend/internal/interaction/notification/infrastructure/repository"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// NotificationHandler handles HTTP requests for notification operations.
type NotificationHandler struct {
	repo notificationrepo.Repository
	db   *db.DB
	log  *zap.Logger
}

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(
	repo notificationrepo.Repository,
	db *db.DB,
	log *zap.Logger,
) *NotificationHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &NotificationHandler{
		repo: repo,
		db:   db,
		log:  log,
	}
}

// NewNotificationHandlerWithDefaults creates a new NotificationHandler with default repository.
func NewNotificationHandlerWithDefaults(db *db.DB, log *zap.Logger) *NotificationHandler {
	if log == nil {
		log = zap.NewNop()
	}
	repo := notificationrepoImpl.NewNotificationRepository()
	return &NotificationHandler{
		repo: repo,
		db:   db,
		log:  log,
	}
}

// NotificationResponse represents the notification response.
//
// PUBLIC BOUNDARY (Phase 2A):
//   - `actor` is the canonical NotificationActorCard. It is emitted when
//     the notification row carries a non-nil ActorID. The legacy `data.actor_id`
//     is preserved for backward compatibility with current mobile consumers
//     that read the actor UUID out of the opaque data map.
//   - `data` remains an opaque map for now — Phase 2A does NOT redesign all
//     notification payloads. Producers may still place additional context
//     in `data`; the canonical actor identity now lives in `actor`.
type NotificationResponse struct {
	ID        uuid.UUID                           `json:"id"`
	UserID    uuid.UUID                           `json:"user_id"`
	Type      notificationEntity.NotificationType `json:"type"`
	Title     string                              `json:"title"`
	Body      string                              `json:"body"`
	Actor     *publiccard.UserCard                `json:"actor,omitempty"`
	Data      map[string]interface{}              `json:"data,omitempty"`
	IsRead    bool                                `json:"is_read"`
	CreatedAt string                              `json:"created_at"`
}

// ListNotificationsRequest holds the query parameters for listing notifications.
type ListNotificationsRequest struct {
	Limit  int    `form:"limit" binding:"omitempty,min=1,max=100"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
}

// ListNotificationsResponse represents the paginated list of notifications.
type ListNotificationsResponse struct {
	Notifications []NotificationResponse `json:"notifications"`
	TotalCount    int                   `json:"total_count"`
	UnreadCount   int                   `json:"unread_count"`
	Limit         int                   `json:"limit"`
	Offset        int                   `json:"offset"`
}

// GetNotifications handles GET /api/v1/notifications
//
// Returns paginated list of notifications for the authenticated user.
// Query parameters:
// - limit (optional): Number of results per page (default: 20, max: 100)
// - offset (optional): Number of results to skip (default: 0)
func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse query parameters
	var req ListNotificationsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// Fetch notifications and unread count within transaction
	var notifications []*notificationEntity.Notification
	var unreadCount int
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		notifications, err = h.repo.ListByRecipient(ctx, tx, userID, req.Limit, req.Offset)
		if err != nil {
			return err
		}
		unreadCount, err = h.repo.CountUnread(ctx, tx, userID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list notifications",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve notifications")
		return
	}

	// Batch-hydrate NotificationActorCards for every distinct actor in the
	// page. Single SQL via publiccard.BuildMany; no N+1. Failure is
	// non-fatal — degrades to omitting the `actor` field; `data.actor_id`
	// remains for backward compatibility.
	actorCards := h.hydrateNotificationActors(ctx, notifications)

	// Map to response DTOs
	respNotifications := make([]NotificationResponse, len(notifications))
	for i, n := range notifications {
		respNotifications[i] = h.mapToResponse(n, actorCards)
	}

	// Build response
	resp := ListNotificationsResponse{
		Notifications: respNotifications,
		TotalCount:    len(notifications), // Note: This is current page count, not total
		UnreadCount:   unreadCount,
		Limit:         req.Limit,
		Offset:        req.Offset,
	}

	response.Success(c, resp)
}

// mapToResponse converts a Notification entity to a response DTO.
// Generates title and body based on notification type.
// Uses the Data field from the entity for navigation payload.
//
// actorCards is the batch-hydrated NotificationActorCard map keyed by
// actor user_id. When present and matching, the canonical `actor` field is
// emitted; the legacy `data.actor_id` remains for backward compatibility.
func (h *NotificationHandler) mapToResponse(
	n *notificationEntity.Notification,
	actorCards map[uuid.UUID]publiccard.UserCard,
) NotificationResponse {
	title, body := h.getNotificationContent(n.Type)

	// Use the Data field from the entity if available, otherwise build minimal data
	data := n.Data
	if data == nil {
		data = make(map[string]interface{})
	}

	// Ensure actor_id and entity_id are always present for backward compatibility
	if _, ok := data["actor_id"]; !ok {
		data["actor_id"] = n.ActorID.String()
	}
	if _, ok := data["entity_id"]; !ok {
		data["entity_id"] = n.EntityID.String()
	}

	resp := NotificationResponse{
		ID:        n.ID,
		UserID:    n.RecipientID,
		Type:      n.Type,
		Title:     title,
		Body:      body,
		Data:      data,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Canonical NotificationActorCard. Only emit when an actor was hydrated;
	// system-generated notifications with no real actor leave `actor` absent.
	if actorCards != nil && n.ActorID != uuid.Nil {
		if card, ok := actorCards[n.ActorID]; ok {
			resp.Actor = &card
		}
	}

	return resp
}

// hydrateNotificationActors batch-loads NotificationActorCards for every
// distinct ActorID across the notification page. Single SQL via
// publiccard.BuildMany; no N+1. Returns an empty (non-nil) map on failure
// so the caller can degrade to the legacy `data.actor_id`-only shape.
func (h *NotificationHandler) hydrateNotificationActors(
	ctx context.Context,
	notifications []*notificationEntity.Notification,
) map[uuid.UUID]publiccard.UserCard {
	if len(notifications) == 0 {
		return map[uuid.UUID]publiccard.UserCard{}
	}
	ids := make([]uuid.UUID, 0, len(notifications))
	seen := make(map[uuid.UUID]struct{}, len(notifications))
	for _, n := range notifications {
		if n.ActorID == uuid.Nil {
			continue
		}
		if _, ok := seen[n.ActorID]; ok {
			continue
		}
		seen[n.ActorID] = struct{}{}
		ids = append(ids, n.ActorID)
	}
	if len(ids) == 0 {
		return map[uuid.UUID]publiccard.UserCard{}
	}
	var cards map[uuid.UUID]publiccard.UserCard
	if err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		cards, err = publiccard.BuildMany(ctx, tx, ids)
		return err
	}); err != nil {
		h.log.Warn("notification: actor hydration failed; degrading to data.actor_id only",
			zap.Int("actor_count", len(ids)),
			zap.Error(err))
		return map[uuid.UUID]publiccard.UserCard{}
	}
	return cards
}

// getNotificationContent returns title and body for a given notification type.
// These match the mobile expectations for notification rendering.
func (h *NotificationHandler) getNotificationContent(notificationType notificationEntity.NotificationType) (title, body string) {
	switch notificationType {
	case notificationEntity.TypeUserFollowed:
		return "New Follower", "Someone started following you"
	case notificationEntity.TypeContentLiked:
		return "New Like", "Someone liked your content"
	case notificationEntity.TypeComment:
		return "New Comment", "Someone commented on your post"
	case notificationEntity.TypeCommentReply:
		return "New Reply", "Someone replied to your comment"
	case notificationEntity.TypeSellerResponse:
		return "Seller Responded", "A seller responded to your request"
	case notificationEntity.TypeChatMessage:
		return "New Message", "You received a new message"
	// =============================================================================
	// ORDER NOTIFICATIONS
	// =============================================================================
	// Seller-facing notifications
	case notificationEntity.TypeOrderCreated:
		return "Order Baru", "Anda memiliki pesanan baru"
	case notificationEntity.TypeOrderPaid:
		return "Siap Dikirim", "Pembayaran masuk, pesanan siap dikirim"
	// Buyer-facing confirmation notifications (trust moments)
	case notificationEntity.TypeOrderCreatedBuyer:
		return "Pesanan Berhasil Dibuat", "Pesanan Anda telah dibuat, silakan lanjutkan pembayaran"
	case notificationEntity.TypeOrderPaidBuyer:
		return "Pembayaran Berhasil", "Pembayaran Anda telah diterima, pesanan sedang diproses"
	// Shipment notifications
	case notificationEntity.TypeOrderShipped:
		return "Pesanan Dikirim", "Pesanan Anda sedang dalam perjalanan"
	case notificationEntity.TypeOrderCompleted:
		return "Transaksi Selesai", "Pembayaran telah diteruskan ke penjual"
	// Cancellation - explicit about WHO cancelled
	case notificationEntity.TypeOrderCancelled:
		return "Pesanan Dibatalkan", "Pembeli membatalkan pesanan ini"
	// Expiry and refunds
	case notificationEntity.TypeOrderExpired:
		return "Pesanan Kadaluarsa", "Waktu pembayaran habis"
	case notificationEntity.TypeOrderRefunded:
		return "Pengembalian Dana", "Dana akan dikembalikan ke metode pembayaran Anda"
	case notificationEntity.TypeOrderPartiallyRefunded:
		return "Pengembalian Dana Sebagian", "Sebagian dana akan dikembalikan ke metode pembayaran Anda"
	// Dispute and refund lifecycle (D1A)
	case notificationEntity.TypeOrderDisputeOpen:
		return "Pesanan Disengketakan", "Pembeli membuka sengketa untuk pesanan ini"
	case notificationEntity.TypeRefundOpened:
		return "Pengajuan Refund", "Pembeli mengajukan refund pada pesanan ini"
	case notificationEntity.TypeRefundEscalated:
		return "Eskalasi ke Sengketa", "Refund dieskalasi menjadi sengketa"
	case notificationEntity.TypeDisputeResolved:
		return "Sengketa Selesai", "Sengketa telah diselesaikan"
	default:
		return "Notification", "You have a new notification"
	}
}

// MarkNotificationAsRead handles POST /api/v1/notifications/{id}/read
//
// Marks a specific notification as read for the authenticated user.
func (h *NotificationHandler) MarkNotificationAsRead(c *gin.Context) {
	ctx := c.Request.Context()

	// Get notification ID from URL
	notificationIDStr := c.Param("id")
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid notification ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Mark as read within transaction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Verify notification belongs to user
		notif, err := h.repo.GetByID(ctx, tx, notificationID)
		if err != nil {
			return err
		}
		if notif.RecipientID != userID {
			return &notificationEntity.ErrNotificationNotFound{NotificationID: notificationID}
		}
		return h.repo.MarkAsRead(ctx, tx, notificationID)
	})

	if err != nil {
		h.log.Error("Failed to mark notification as read",
			zap.String("user_id", userID.String()),
			zap.String("notification_id", notificationID.String()),
			zap.Error(err),
		)
		response.NotFound(c, "Notification not found")
		return
	}

	response.Success(c, gin.H{"success": true})
}

// MarkAllAsRead handles POST /api/v1/notifications/read-all
//
// Marks all notifications as read for the authenticated user.
func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Mark all as read within transaction
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.repo.MarkAllAsRead(ctx, tx, userID)
	})

	if err != nil {
		h.log.Error("Failed to mark all notifications as read",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to mark all as read")
		return
	}

	response.Success(c, gin.H{"success": true})
}

// MarkAsReadByEntityRequest holds the request body for marking notifications as read by entity.
type MarkAsReadByEntityRequest struct {
	EntityType string `json:"entity_type" binding:"required"` // e.g., "chat_message"
	EntityID   string `json:"entity_id" binding:"required"`   // UUID of the entity
}

// MarkAsReadByEntity handles POST /api/v1/notifications/read-by-entity
//
// Marks notifications as read for a specific entity type and entity ID.
// This is used for cross-domain sync (e.g., chat read → chat notifications read).
// Only affects notifications matching: recipient_id, type (entityType), and entity_id.
func (h *NotificationHandler) MarkAsReadByEntity(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body
	var req MarkAsReadByEntityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Parse entity ID as UUID
	entityID, err := uuid.Parse(req.EntityID)
	if err != nil {
		response.BadRequest(c, "Invalid entity ID")
		return
	}

	// Map frontend entity type to notification type
	// Frontend sends "chat" → backend expects "chat_message"
	notificationType := req.EntityType
	if req.EntityType == "chat" {
		notificationType = string(notificationEntity.TypeChatMessage)
	}

	// Mark as read within transaction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.repo.MarkAsReadByEntity(ctx, tx, userID, notificationType, entityID)
	})

	if err != nil {
		h.log.Error("Failed to mark notifications as read by entity",
			zap.String("user_id", userID.String()),
			zap.String("entity_type", req.EntityType),
			zap.String("entity_id", req.EntityID),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to mark notifications as read")
		return
	}

	response.Success(c, gin.H{"success": true})
}

// GetUnreadCount handles GET /api/v1/notifications/unread-count
//
// Returns the count of unread notifications for the authenticated user.
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Get unread count within transaction
	var count int
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		count, err = h.repo.CountUnread(ctx, tx, userID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get unread count",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to get unread count")
		return
	}

	response.Success(c, gin.H{"count": count})
}

// DeleteNotification handles DELETE /api/v1/notifications/{id}
//
// Deletes a specific notification for the authenticated user.
func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	ctx := c.Request.Context()

	// Get notification ID from URL
	notificationIDStr := c.Param("id")
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid notification ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Delete within transaction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Verify notification belongs to user before deleting
		notif, err := h.repo.GetByID(ctx, tx, notificationID)
		if err != nil {
			return err
		}
		if notif.RecipientID != userID {
			return &notificationEntity.ErrNotificationNotFound{NotificationID: notificationID}
		}
		return h.repo.Delete(ctx, tx, notificationID)
	})

	if err != nil {
		h.log.Error("Failed to delete notification",
			zap.String("user_id", userID.String()),
			zap.String("notification_id", notificationID.String()),
			zap.Error(err),
		)
		response.NotFound(c, "Notification not found")
		return
	}

	response.Success(c, gin.H{"success": true})
}

// ============================================================================
// FLUTTER-COMPATIBLE ENDPOINTS
// ============================================================================
// Note: Notification settings endpoints removed - unused by Flutter app.
// Flutter uses /notifications/preferences instead (implemented separately).


