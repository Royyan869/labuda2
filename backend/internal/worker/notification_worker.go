package worker

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/interaction/notification/policy"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/platform/events"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// BlockChecker defines the interface for checking block relationships.
// Implementations manage their own DB connection; no tx is passed from call sites.
type BlockChecker interface {
	ExistsBlock(ctx context.Context, userA, userB uuid.UUID) (bool, error)
}

// PushSender defines the interface for sending push notifications.
type PushSender interface {
	SendNotification(ctx context.Context, tx interface{}, notification interface{}, title, body string) error
}

// AccountStatusChecker defines the interface for checking user account status.
type AccountStatusChecker interface {
	GetStatus(ctx context.Context, userID uuid.UUID) (string, error)
}

// DeliveryLogger defines the interface for logging notification delivery events.
type DeliveryLogger interface {
	LogDelivery(
		ctx context.Context,
		notificationID, userID uuid.UUID,
		channel string,
		status string,
		reason string,
		metadata map[string]interface{},
	)
}

// NotificationEventHandler handles outbox events for notifications.
//
// HARDENED POLICY LAYER (SESSION 2):
// 1. Category-based filtering (commerce_critical, moderation, social, marketing)
// 2. Account status filtering (suspended/banned users only receive critical/moderation)
// 3. Block interaction policy (commerce bypasses block but anonymizes actor)
// 4. Fail-safe push (no push on status check failure)
// 5. Safe default (unknown types = Social = filtered)
//
// AUDIT TRAIL (SESSION 3):
// - DeliveryLogger for observability (non-blocking, async)
//
// Processes:
// - events.EventUserFollowed - User A follows User B
// - events.EventContentLiked - User A likes User B's content
// - events.EventCommentCreated - User A comments on User B's content
// - "comment.reply" - User A replies to User B's comment
// - "seller.response" - Seller responds to a request with fixed-price sale reference
// - "chat.message.sent" - User A sends a message to User B
type NotificationEventHandler struct {
	db                   Transactor
	blockChecker         BlockChecker
	notificationInserter NotificationInserter
	pushSender           PushSender // Optional: If nil, push is skipped
	accountStatusChecker AccountStatusChecker
	policyFilter         *policy.AccountStatusFilter
	policyBlock          *policy.BlockPolicy
	policyMute           *policy.MutePolicy // Optional: CHAT-5 shadow-first mute gate
	deliveryLogger       DeliveryLogger     // Optional: for audit trail
	capabilityLister     CapabilityLister   // Optional: for capability-based fanout
	log                  *zap.Logger
}

// CapabilityLister resolves user IDs holding a given capability.
// Used for admin fanout notifications (e.g., unassigned support tickets).
type CapabilityLister interface {
	ListUsersByCapability(ctx context.Context, capability string) ([]uuid.UUID, error)
}

// NotificationInserter defines the interface for inserting notifications with navigation payload.
type NotificationInserter interface {
	InsertNotification(ctx context.Context, tx dbpkg.Tx, recipientID, actorID uuid.UUID, notificationType string, entityID uuid.UUID, data map[string]interface{}) (uuid.UUID, error)
}

// NewNotificationEventHandler creates a new NotificationEventHandler.
//
// PARAMETERS:
// - db: Transactor for database operations
// - blockChecker: Optional; if nil, no block filtering will be performed
// - notificationInserter: Interface for inserting notifications
// - pushSender: Optional; if nil, push is skipped
// - accountStatusChecker: Optional; if nil, all notifications are allowed (unsafe)
// - log: Logger; if nil, nop logger is used
func NewNotificationEventHandler(
	db Transactor,
	blockChecker BlockChecker,
	notificationInserter NotificationInserter,
	pushSender PushSender,
	accountStatusChecker AccountStatusChecker,
	log *zap.Logger,
) *NotificationEventHandler {
	if log == nil {
		log = zap.NewNop()
	}

	var policyFilter *policy.AccountStatusFilter
	if accountStatusChecker != nil {
		policyFilter = policy.NewAccountStatusFilter(accountStatusChecker)
	}

	var policyBlock *policy.BlockPolicy
	if blockChecker != nil {
		policyBlock = policy.NewBlockPolicy(blockChecker)
	}

	return &NotificationEventHandler{
		db:                   db,
		blockChecker:         blockChecker,
		notificationInserter: notificationInserter,
		pushSender:           pushSender,
		accountStatusChecker: accountStatusChecker,
		policyFilter:         policyFilter,
		policyBlock:          policyBlock,
		log:                  log,
	}
}

// SetDeliveryLogger sets the delivery logger for audit trail.
// This is optional and can be called after handler creation.
func (h *NotificationEventHandler) SetDeliveryLogger(logger DeliveryLogger) {
	h.deliveryLogger = logger
}

// SetCapabilityLister sets the capability lister for admin fanout notifications.
// When set, unassigned support tickets will notify all admins holding support.ticket.claim.
func (h *NotificationEventHandler) SetCapabilityLister(lister CapabilityLister) {
	h.capabilityLister = lister
}

// SetMutePolicy sets the mute policy for chat notification governance.
// Shadow-first by default (MuteShadow): evaluate mute, emit telemetry, always deliver.
// Promote to MuteEnforce via MUTE_CHAT_NOTIFICATION_ENFORCE=true to suppress delivery.
// Scope: chat_message notification type only. REST and WebSocket are unaffected.
func (h *NotificationEventHandler) SetMutePolicy(p *policy.MutePolicy) {
	h.policyMute = p
}

func (h *NotificationEventHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) (retErr error) {
	// O3: Recover from handler panics so outbox retries immediately instead of
	// waiting for stuck-event recovery. Panic is logged with stack trace.
	defer func() {
		if r := recover(); r != nil {
			h.log.Error("NOTIFICATION_HANDLER_PANIC: recovered",
				zap.String("event_type", event.EventType),
				zap.String("event_id", event.ID.String()),
				zap.Any("panic_value", r),
				zap.String("stack", string(debug.Stack())),
			)
			retErr = fmt.Errorf("notification handler panic: %v", r)
		}
	}()

	h.log.Debug("Handling notification event",
		zap.String("event_type", event.EventType),
		zap.String("event_id", event.ID.String()),
	)

	// Parse payload based on event type
	var info notificationInfo
	var err error

	switch event.EventType {
	case events.EventUserFollowed:
		info, err = h.handleUserFollowed(ctx, event.Payload)

	// =============================================================================
	// SOCIAL GRAPH CLEANUP EVENTS - Notification history alignment
	// =============================================================================
	case events.EventUserBlocked:
		return h.handleUserBlocked(ctx, event.Payload)

	case events.EventUserUnfollowed:
		return h.handleUserUnfollowed(ctx, event.Payload)

	case events.EventContentLiked:
		info, err = h.handleContentLiked(ctx, event.Payload)

	case events.EventCommentCreated:
		info, err = h.handleCommentCreated(ctx, event.Payload)

	case "comment.reply":
		info, err = h.handleCommentReply(ctx, event.Payload)

	case "seller.response":
		info, err = h.handleSellerResponse(ctx, event.Payload)

	case "chat.message.sent":
		info, err = h.handleChatMessage(ctx, event.Payload)

	// =============================================================================
	// ORDER EVENTS - Seller receives notifications for buyer actions
	// =============================================================================
	case events.EventOrderCreated:
		info, err = h.handleOrderCreated(ctx, event.Payload)

	case events.EventOrderPaid:
		info, err = h.handleOrderPaid(ctx, event.Payload)

	case "order.shipped":
		info, err = h.handleOrderShipped(ctx, event.Payload)

	case events.EventOrderCompleted:
		info, err = h.handleOrderCompleted(ctx, event.Payload)

	case "order.cancelled":
		info, err = h.handleOrderCancelled(ctx, event.Payload)

	case "order.expired":
		info, err = h.handleOrderExpired(ctx, event.Payload)

	case "order.refunded":
		info, err = h.handleOrderRefunded(ctx, event.Payload)

	case "order.partially_refunded":
		info, err = h.handleOrderPartiallyRefunded(ctx, event.Payload)

	case "order.dispute_open":
		info, err = h.handleOrderDisputeOpen(ctx, event.Payload)

	case "order.confirmation_extended":
		info, err = h.handleOrderConfirmationExtended(ctx, event.Payload)

	case "order.cancelled_timeout":
		info, err = h.handleOrderCancelledTimeout(ctx, event.Payload)

	// =============================================================================
	// REFUND / DISPUTE LIFECYCLE NOTIFICATIONS — D1A
	// =============================================================================
	case "refund.opened":
		info, err = h.handleRefundOpened(ctx, event.Payload)

	case "refund.escalated":
		info, err = h.handleRefundEscalated(ctx, event.Payload)

	case "refund.approved":
		info, err = h.handleRefundApproved(ctx, event.Payload)

	case "refund.rejected":
		info, err = h.handleRefundRejected(ctx, event.Payload)

	case "dispute.opened":
		info, err = h.handleDisputeOpened(ctx, event.Payload)

	case events.EventDisputeResolved:
		info, err = h.handleDisputeResolved(ctx, event.Payload)

	// =============================================================================
	// DISPUTE AGING ADMIN NOTIFICATIONS
	// =============================================================================
	case "dispute.overdue":
		info, err = h.handleDisputeOverdue(ctx, event.Payload)

	case "dispute.timeout_escalation":
		info, err = h.handleDisputeTimeoutEscalation(ctx, event.Payload)

	// =============================================================================
	// MONEY FAILURE ADMIN NOTIFICATIONS
	// =============================================================================
	case "money.refund_failed":
		info, err = h.handleMoneyRefundFailed(ctx, event.Payload)

	// =============================================================================
	// ORDER OVERDUE REMINDER EVENTS - OVERDUE ENFORCEMENT CLOSURE
	// =============================================================================
	case "order.overdue_reminder.seller":
		info, err = h.handleOrderOverdueReminderSeller(ctx, event.Payload)

	case "order.overdue_reminder.buyer":
		info, err = h.handleOrderOverdueReminderBuyer(ctx, event.Payload)

	// =============================================================================
	// MODERATION NOTIFICATION EVENTS - User notifications for moderation actions
	// =============================================================================
	case "moderation.content.removed":
		info, err = h.handleModerationContentRemoved(ctx, event.Payload)

	case "moderation.comment.removed":
		info, err = h.handleModerationCommentRemoved(ctx, event.Payload)

	case "moderation.content.restored":
		info, err = h.handleModerationContentRestored(ctx, event.Payload)

	case "moderation.comment.restored":
		info, err = h.handleModerationCommentRestored(ctx, event.Payload)

	case "moderation.user.suspended":
		info, err = h.handleModerationUserSuspended(ctx, event.Payload)

	case "moderation.user.restored":
		info, err = h.handleModerationUserRestored(ctx, event.Payload)

	case "moderation.for_sale.removed":
		info, err = h.handleModerationForSaleRemoved(ctx, event.Payload)

	case "moderation.for_sale.restored":
		info, err = h.handleModerationForSaleRestored(ctx, event.Payload)

	case "moderation.warning.issued":
		info, err = h.handleModerationWarningIssued(ctx, event.Payload)

	// =============================================================================
	// SUPPORT NOTIFICATION EVENTS - User notifications for support ticket updates
	// =============================================================================
	case "support.ticket.resolved":
		info, err = h.handleSupportTicketResolved(ctx, event.Payload)

	case "support.ticket.closed":
		info, err = h.handleSupportTicketClosed(ctx, event.Payload)

	case "support.ticket_waiting_user":
		info, err = h.handleSupportTicketWaitingUser(ctx, event.Payload)

	case "support.ticket.user_responded":
		info, err = h.handleSupportTicketUserResponded(ctx, event.Payload)

	case "support.ticket.created":
		info, err = h.handleSupportTicketCreated(ctx, event.Payload)

	// =============================================================================
	// NEGOTIATION NOTIFICATION EVENTS - User notifications for negotiation updates
	// =============================================================================
	case "negotiation.started":
		info, err = h.handleNegotiationStarted(ctx, event.Payload)

	case "negotiation.message_sent":
		info, err = h.handleNegotiationMessageSent(ctx, event.Payload)

	case "negotiation.accepted":
		info, err = h.handleNegotiationAccepted(ctx, event.Payload)

	case "negotiation.expired":
		info, err = h.handleNegotiationExpired(ctx, event.Payload)

	case "negotiation.cancelled":
		info, err = h.handleNegotiationCancelled(ctx, event.Payload)

	// =============================================================================
	// SELLER TIER CHANGE NOTIFICATION EVENTS — B1
	// =============================================================================
	case "seller.tier.upgraded":
		info, err = h.handleSellerTierUpgraded(ctx, event.Payload)

	case "seller.tier.downgraded":
		info, err = h.handleSellerTierDowngraded(ctx, event.Payload)

	// =============================================================================
	// WITHDRAWAL NOTIFICATION EVENTS - Seller notifications for withdrawal updates
	// =============================================================================
	case "withdrawal.requested":
		info, err = h.handleWithdrawalRequested(ctx, event.Payload)

	case "withdrawal.approved":
		info, err = h.handleWithdrawalApproved(ctx, event.Payload)

	case "withdrawal.rejected":
		info, err = h.handleWithdrawalRejected(ctx, event.Payload)

	case "withdrawal.completed":
		info, err = h.handleWithdrawalCompleted(ctx, event.Payload)

	case "withdrawal.failed":
		info, err = h.handleWithdrawalFailed(ctx, event.Payload)

	// =============================================================================
	// VERIFICATION NOTIFICATION EVENTS - User notifications for verification decisions
	// =============================================================================
	case "verification.document.approved":
		info, err = h.handleVerificationDocumentApproved(ctx, event.Payload)

	case "verification.document.rejected":
		info, err = h.handleVerificationDocumentRejected(ctx, event.Payload)

	// Seller lifecycle (seller_verifications) events. These notify the
	// seller of their canonical trust state transitions; per doctrine
	// (verification-review-governance.md) every state change must produce a
	// notification, and negative outcomes must carry the reason as the
	// recourse path.
	case "seller.verification.submitted":
		info, err = h.handleSellerVerificationSubmitted(ctx, event.Payload)

	case "seller.verification.approved":
		info, err = h.handleSellerVerificationLifecycle(ctx, event.Payload, "seller.verification.approved",
			"Verifikasi Disetujui", "Verifikasi Anda disetujui. Penarikan saldo kini terbuka.")

	case "seller.verification.rejected":
		info, err = h.handleSellerVerificationLifecycle(ctx, event.Payload, "seller.verification.rejected",
			"Verifikasi Ditolak", "Verifikasi Anda ditolak. Periksa alasan dan ajukan kembali bila perlu.")

	case "seller.verification.needs_resubmission":
		info, err = h.handleSellerVerificationLifecycle(ctx, event.Payload, "seller.verification.needs_resubmission",
			"Perlu Pengiriman Ulang", "Admin meminta penyesuaian dokumen verifikasi Anda.")

	case "seller.verification.suspended":
		info, err = h.handleSellerVerificationLifecycle(ctx, event.Payload, "seller.verification.suspended",
			"Verifikasi Ditangguhkan", "Verifikasi penjual Anda ditangguhkan sementara oleh admin.")

	case "seller.verification.revoked":
		info, err = h.handleSellerVerificationLifecycle(ctx, event.Payload, "seller.verification.revoked",
			"Verifikasi Dicabut", "Verifikasi penjual Anda telah dicabut oleh admin.")

	case "seller.verification.under_investigation":
		info, err = h.handleSellerVerificationLifecycle(ctx, event.Payload, "seller.verification.under_investigation",
			"Verifikasi Dalam Investigasi", "Verifikasi penjual Anda sedang dalam investigasi.")

	case "seller.verification.restored":
		info, err = h.handleSellerVerificationLifecycle(ctx, event.Payload, "seller.verification.restored",
			"Verifikasi Dipulihkan", "Verifikasi Anda telah dipulihkan. Penjualan dan penarikan kini aktif kembali.")

	// =============================================================================
	// AUCTION NOTIFICATION EVENTS - Seller notifications for auction activity
	// =============================================================================
	case "auction.bid.placed":
		info, err = h.handleAuctionBidPlaced(ctx, event.Payload)
	case "auction.waiting_settlement":
		info, err = h.handleAuctionWaitingSettlement(ctx, event.Payload)
	case "auction.ended":
		info, err = h.handleAuctionEndedNoWinner(ctx, event.Payload)
	case "auction_bnr_detected":
		info, err = h.handleAuctionBNRDetected(ctx, event.Payload)

	// =============================================================================
	// EXTERNAL PRODUCT REVIEW DECISION EVENTS — owner-facing review notifications
	// =============================================================================
	case "external_product.review.approved":
		info, err = h.handleExternalProductReviewLifecycle(ctx, event.Payload, "external_product.review.approved",
			"Produk Eksternal Disetujui", "Produk eksternal Anda telah disetujui dan siap untuk dipromosikan.")

	case "external_product.review.rejected":
		info, err = h.handleExternalProductReviewLifecycle(ctx, event.Payload, "external_product.review.rejected",
			"Produk Eksternal Ditolak", "Produk eksternal Anda ditolak. Periksa alasan dan ajukan kembali.")

	case "external_product.review.request_changes":
		info, err = h.handleExternalProductReviewLifecycle(ctx, event.Payload, "external_product.review.request_changes",
			"Perubahan Diperlukan", "Admin meminta perubahan pada produk eksternal Anda.")

	case "external_product.review.hidden":
		info, err = h.handleExternalProductReviewLifecycle(ctx, event.Payload, "external_product.review.hidden",
			"Produk Eksternal Disembunyikan", "Produk eksternal Anda telah disembunyikan oleh admin.")

	// =============================================================================
	// SELLER SUBSCRIPTION EVENTS - Seller notifications for subscription lifecycle
	// =============================================================================
	case "seller.subscription.expiring":
		info, err = h.handleSellerSubscriptionExpiring(ctx, event.Payload)
	case "seller.subscription.expired":
		info, err = h.handleSellerSubscriptionExpired(ctx, event.Payload)

	default:
		h.log.Warn("Unknown notification event type",
			zap.String("event_type", event.EventType),
		)
		return nil // Don't fail for unknown types - allows future extension
	}

	if err != nil {
		return err
	}

	// Send push notification asynchronously if PushSender is available
	if h.pushSender != nil && info.inserted && info.recipientID != (uuid.UUID{}) {
		go h.sendPushAsync(context.Background(), info)
	}

	return nil
}

// sendPushAsync sends push notification asynchronously.
// This ensures notification record creation is not affected by push failures.
//
// POLICY CHECK: Only sends push if allowPush flag is true (set by policy layer).
// AUDIT TRAIL (SESSION 3): Logs push delivery events.
func (h *NotificationEventHandler) sendPushAsync(ctx context.Context, info notificationInfo) {
	// O3: Recover from push panics so worker goroutine doesn't crash.
	defer func() {
		if r := recover(); r != nil {
			h.log.Error("PUSH_ASYNC_PANIC: recovered",
				zap.String("recipient_id", info.recipientID.String()),
				zap.String("type", info.notifyType),
				zap.Any("panic_value", r),
				zap.String("stack", string(debug.Stack())),
			)
		}
	}()

	// FAIL-SAFE: Check if push is allowed by policy
	if !info.allowPush {
		h.log.Debug("Push blocked by policy",
			zap.String("recipient_id", info.recipientID.String()),
			zap.String("type", info.notifyType),
			zap.String("reason", info.filterReason),
		)

		// AUDIT: Log blocked push
		h.logDelivery(ctx, info.notificationID, info.recipientID, "push", "skipped", info.filterReason, map[string]interface{}{
			"type":     info.notifyType,
			"category": string(info.category),
		})

		return
	}

	// Create a minimal notification object for the push sender
	// The actual notification was already inserted to DB
	notif := map[string]interface{}{
		"id":           info.notificationID.String(),
		"recipient_id": info.recipientID.String(),
		"actor_id":     info.actorID.String(),
		"type":         info.notifyType,
	}

	err := h.pushSender.SendNotification(ctx, nil, notif, info.title, info.body)
	if err != nil {
		h.log.Warn("Failed to send push notification (async)",
			zap.String("recipient_id", info.recipientID.String()),
			zap.Error(err),
		)

		// AUDIT: Log failed push
		h.logDelivery(ctx, info.notificationID, info.recipientID, "push", "failed", err.Error(), map[string]interface{}{
			"type":     info.notifyType,
			"category": string(info.category),
		})
	} else {
		// AUDIT: Log successful push send attempt (actual delivery confirmed by FCM)
		h.logDelivery(ctx, info.notificationID, info.recipientID, "push", "sent", "", map[string]interface{}{
			"type":     info.notifyType,
			"category": string(info.category),
		})
	}
}

type NotificationServiceInserter struct{}

// NewNotificationServiceInserter creates a new NotificationServiceInserter.
func NewNotificationServiceInserter() *NotificationServiceInserter {
	return &NotificationServiceInserter{}
}

// InsertNotification inserts a notification into the database with navigation payload.
// Returns the ID of the inserted notification.
func (s *NotificationServiceInserter) InsertNotification(
	ctx context.Context,
	tx dbpkg.Tx,
	recipientID, actorID uuid.UUID,
	notificationType string,
	entityID uuid.UUID,
	data map[string]interface{},
) (uuid.UUID, error) {
	query := `
		INSERT INTO notifications (id, recipient_id, actor_id, type, entity_id, data, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (recipient_id, actor_id, type, entity_id) DO NOTHING
		RETURNING id
	`

	id := uuid.New()
	err := tx.QueryRow(ctx, query, id, recipientID, actorID, notificationType, entityID, data, false).Scan(&id)
	if err != nil {
		// ON CONFLICT DO NOTHING returns no rows when a duplicate exists.
		// This is expected dedup behavior, not an error.
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, nil
		}
		return uuid.Nil, fmt.Errorf("insert notification query failed: %w", err)
	}

	return id, nil
}

// =============================================================================
// WORKER SETUP HELPER
// =============================================================================

// SetupNotificationHandlers registers notification event handlers with the dispatcher.
//
// PARAMETERS:
// - db: Transactor for database operations
// - blockChecker: Optional; if nil, no block filtering will be performed
// - pushSender: Optional; if nil, push is skipped
// - accountStatusChecker: Optional; if nil, all notifications are allowed (unsafe)
// - mutePolicy: Optional; if nil, STEP 3C mute evaluation is disabled (no mute enforcement)
//
// Usage:
//
//	worker.SetupNotificationHandlers(db, blockChecker, pushSender, accountStatusChecker, mutePolicy)
//
// SetupNotificationHandlers registers the notification handler for all event
// types that only need push/in-app notifications.
//
// Returns the NotificationEventHandler so callers can pass it to
// SetupNegotiationHandlers (or other fanout sites) that need both a domain
// handler AND a notification handler for the same event type.
func (w *OutboxWorker) SetupNotificationHandlers(
	db Transactor,
	blockChecker BlockChecker,
	pushSender PushSender,
	accountStatusChecker AccountStatusChecker,
	mutePolicy *policy.MutePolicy,
) (*OutboxWorker, *NotificationEventHandler) {
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(db, blockChecker, inserter, pushSender, accountStatusChecker, w.log)
	handler.SetMutePolicy(mutePolicy)

	w.dispatcher.RegisterMultiple([]string{
		events.EventUserFollowed,
		events.EventContentLiked,
		events.EventCommentCreated,
		"comment.reply",
		"seller.response",
		"chat.message.sent",
	}, handler)

	// =============================================================================
	// ORDER NOTIFICATION HANDLERS
	// =============================================================================
	w.dispatcher.RegisterMultiple([]string{
		events.EventOrderCreated,
		events.EventOrderPaid,
		"order.shipped",
		events.EventOrderCompleted,
		"order.cancelled",
		"order.cancelled_timeout",
		"order.expired",
		"order.refunded",
		"order.partially_refunded",
		"order.dispute_open",          // D1A: was NoHandlerHandlerUnregistered, now active + push
		"order.confirmation_extended", // Z7: was NoHandlerHandlerUnregistered, now active + push
	}, handler)

	// =============================================================================
	// REFUND / DISPUTE LIFECYCLE NOTIFICATION HANDLERS — D1A
	// =============================================================================
	w.dispatcher.RegisterMultiple([]string{
		"refund.opened",
		"refund.approved",
		"refund.rejected",
		"refund.escalated",
		"dispute.opened", // D1B: legacy/parked dispute-open payload compatibility
		events.EventDisputeResolved,
		"dispute.overdue",            // G1: dispute aging admin notification
		"dispute.timeout_escalation", // G1: dispute timeout admin notification
	}, handler)

	// =============================================================================
	// MONEY FAILURE ADMIN NOTIFICATION HANDLERS
	// =============================================================================
	// NOTE: money.refund_failed is also handled by RefundFailedAlertHandler
	// (creates system_alert row). SetupRefundFailedAlertHandler uses the
	// fanout-ready pattern to compose alert + notification handlers.
	// Alert handler runs FIRST, then notification fanout.
	w.dispatcher.Register("money.refund_failed", handler)

	// =============================================================================
	// ORDER OVERDUE REMINDER HANDLERS - OVERDUE ENFORCEMENT CLOSURE
	// =============================================================================
	// These events trigger notifications for orders that have exceeded their
	// ready_to_ship_by deadline without being shipped.
	//
	// Events:
	// - order.overdue_reminder.seller: Notifies seller their order is overdue
	// - order.overdue_reminder.buyer: Notifies buyer their order is delayed
	w.dispatcher.RegisterMultiple([]string{
		"order.overdue_reminder.seller",
		"order.overdue_reminder.buyer",
	}, handler)

	// =============================================================================
	// MODERATION NOTIFICATION HANDLERS
	// =============================================================================
	// moderation.warning.issued: notification-only (no enforcement handler).
	// Enforcement events (moderation.*.removed / .restored) are fanned out via
	// SetupModerationHandlers — enforcement runs FIRST, then notification SECOND.
	w.dispatcher.Register("moderation.warning.issued", handler)

	// =============================================================================
	// SUPPORT NOTIFICATION HANDLERS
	// =============================================================================
	w.dispatcher.RegisterMultiple([]string{
		"support.ticket.created",
		"support.ticket.resolved",
		"support.ticket.closed",
		"support.ticket_waiting_user",
		"support.ticket.user_responded",
	}, handler)

	// =============================================================================
	// NEGOTIATION NOTIFICATION HANDLERS
	// =============================================================================
	// NOTE: negotiation.started and negotiation.message_sent are wired via
	// RegisterFanout in dependencies.go because they need BOTH the chat-domain
	// handler (NegotiationStartedHandler / NegotiationMessageSentHandler) AND
	// the notification handler. Register() would panic on duplicate.
	w.dispatcher.RegisterMultiple([]string{
		"negotiation.accepted",
		"negotiation.expired",
		"negotiation.cancelled", // B1: buyer+seller notified on cancellation
	}, handler)

	// =============================================================================
	// WITHDRAWAL NOTIFICATION HANDLERS
	// =============================================================================
	w.dispatcher.RegisterMultiple([]string{
		"withdrawal.requested",
		"withdrawal.approved",
		"withdrawal.rejected",
		"withdrawal.completed",
		"withdrawal.failed",
	}, handler)

	// =============================================================================
	// VERIFICATION NOTIFICATION HANDLERS
	// =============================================================================
	w.dispatcher.RegisterMultiple([]string{
		"verification.document.approved",
		"verification.document.rejected",
		// Seller-lifecycle events.
		"seller.verification.submitted",
		"seller.verification.approved",
		"seller.verification.rejected",
		"seller.verification.needs_resubmission",
		"seller.verification.suspended",
		"seller.verification.revoked",
		"seller.verification.under_investigation",
		"seller.verification.restored",
	}, handler)

	// =============================================================================
	// AUCTION NOTIFICATION HANDLERS
	// =============================================================================
	w.dispatcher.RegisterMultiple([]string{
		"auction.bid.placed",
		"auction.waiting_settlement",
		// auction.ended — no-winner path (P14): seller notified auction closed without bids.
		// SetupPromotionHandlers composes a fanout so promotion auto-stop also fires.
		// ORDERING: SetupPromotionHandlers must run AFTER this registration.
		"auction.ended",
	}, handler)

	// =============================================================================
	// SELLER TIER CHANGE NOTIFICATION HANDLERS — B1
	// =============================================================================
	w.dispatcher.RegisterMultiple([]string{
		"seller.tier.upgraded",
		"seller.tier.downgraded",
	}, handler)

	// =============================================================================
	// SELLER SUBSCRIPTION NOTIFICATION HANDLERS
	// =============================================================================
	// seller.subscription.expiring and seller.subscription.expired are registered
	// here (notification delivery). SetupSellerSubscriptionExpiredHandler, called
	// later in serverboot, composes auction-cancellation side-effect via fanout
	// (auction runs first, then this).
	w.dispatcher.RegisterMultiple([]string{
		"seller.subscription.expiring",
		"seller.subscription.expired",
	}, handler)

	// =============================================================================
	// EXTERNAL PRODUCT REVIEW NOTIFICATION HANDLERS
	// =============================================================================
	w.dispatcher.RegisterMultiple([]string{
		"external_product.review.approved",
		"external_product.review.rejected",
		"external_product.review.request_changes",
		"external_product.review.hidden",
	}, handler)

	// =============================================================================
	// SOCIAL GRAPH CLEANUP HANDLERS - Block/unfollow notification history alignment
	// =============================================================================
	// On block: delete all SOCIAL-category notifications between both users.
	// On unfollow: delete the specific "user.followed" notification only.
	// Commerce/moderation/support notifications are PRESERVED (obligation history).
	w.dispatcher.RegisterMultiple([]string{
		events.EventUserBlocked,
		events.EventUserUnfollowed,
	}, handler)

	w.log.Info("Notification event handlers registered")
	return w, handler
}
