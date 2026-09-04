package worker

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/interaction/notification/policy"
	"github.com/labuda/backend/internal/platform/events"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// NotificationPayload represents the expected payload structure for notification events.
type NotificationPayload struct {
	ActorID     string `json:"actor_id"`
	RecipientID string `json:"recipient_id"`
}

// ContentLikedPayload represents the payload for content.liked events.
type ContentLikedPayload struct {
	ActorID     string `json:"actor_id"`
	RecipientID string `json:"recipient_id"`
	ContentID   string `json:"content_id"`
	// OccurrenceAt identifies the specific LIKE occurrence (the content_likes
	// row's created_at, RFC3339Nano). The delivery guard uses it to reject
	// stale events for an occurrence that was unliked, so a LIKE after an
	// UNLIKE (a new occurrence) can never be confused with the old event.
	OccurrenceAt string `json:"occurrence_at,omitempty"`
}

// CommentCreatedPayload represents the payload for comment.created events.
type CommentCreatedPayload struct {
	CommentID string `json:"comment_id"`
	ContentID string `json:"content_id"`
	AuthorID  string `json:"author_id"`
	CreatedAt string `json:"created_at"`
}

// CommentReplyPayload represents the payload for comment.reply events.
type CommentReplyPayload struct {
	CommentID      string `json:"comment_id"`
	ContentID      string `json:"content_id"`
	AuthorID       string `json:"author_id"`
	ParentAuthorID string `json:"parent_author_id"`
	ParentID       string `json:"parent_id"`
	CreatedAt      string `json:"created_at"`
}

// SellerResponsePayload represents the payload for seller.response / auction.response events.
// Canonical shape (Closure): request_creator_id is recipient (content author), seller_id is actor,
// resource_id + resource_type discriminate for_sale vs auction. No legacy aliases.
type SellerResponsePayload struct {
	CommentID        string `json:"comment_id"`
	ContentID        string `json:"content_id"`
	ResourceID       string `json:"resource_id"`
	ResourceType     string `json:"resource_type"`
	SellerID         string `json:"seller_id"`
	RequestCreatorID string `json:"request_creator_id"`
	CreatedAt        string `json:"created_at"`
}

// ContentMentionedPayload represents the payload for content.mentioned events.
type ContentMentionedPayload struct {
	ContentID       string `json:"content_id"`
	AuthorID        string `json:"author_id"`
	MentionedUserID string `json:"mentioned_user_id"`
	CreatedAt       string `json:"created_at,omitempty"`
}

// ChatMessagePayload represents the payload for chat.message events.
type ChatMessagePayload struct {
	RoomID      string `json:"room_id"`
	MessageID   string `json:"message_id"`
	SenderID    string `json:"sender_id"`
	RecipientID string `json:"recipient_id"`
	MessageType string `json:"message_type"`
	CreatedAt   string `json:"created_at"`
}

// OrderPayload represents the payload for order events.
type OrderPayload struct {
	OrderID          string `json:"order_id"`
	BuyerID          string `json:"buyer_id"`
	SellerID         string `json:"seller_id"`
	SourceType       string `json:"source_type"`
	SourceID         string `json:"source_id"`
	Status           string `json:"status"`
	EscrowStatus     string `json:"escrow_status"`
	Subtotal         int64  `json:"subtotal"`
	ShippingTotal    int64  `json:"shipping_total"`
	CommissionAmount int64  `json:"commission_amount"`
	EscrowAmount     int64  `json:"escrow_amount"`
	CreatedAt        int64  `json:"created_at"`
}

// ModerationRemovedPayload represents the payload for moderation removal events.
type ModerationRemovedPayload struct {
	CaseID       string `json:"case_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	DecisionNote string `json:"decision_note,omitempty"`
}

// ModerationRestoredPayload represents the payload for moderation restoration events.
type ModerationRestoredPayload struct {
	CaseID       string `json:"case_id"`
	AppealID     string `json:"appeal_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
}

// SupportTicketPayload represents the payload for support ticket events.
type SupportTicketPayload struct {
	TicketID      string `json:"ticket_id"`
	UserID        string `json:"user_id"`
	AdminID       string `json:"admin_id,omitempty"`
	ChatRoomID    string `json:"chat_room_id,omitempty"`
	Status        string `json:"status"`
	Category      string `json:"category,omitempty"`
	Priority      string `json:"priority,omitempty"`
	LinkedOrderID string `json:"linked_order_id,omitempty"`
}

// AuctionBidPayload represents the payload for auction.bid.placed events.
type AuctionBidPayload struct {
	BidID     string `json:"bid_id"`
	AuctionID string `json:"auction_id"`
	BidderID  string `json:"bidder_id"`
	Amount    int64  `json:"amount"`
}

// AuctionLifecyclePayload represents the payload for auction lifecycle events
// (auction.waiting_settlement, auction.ended, etc.) produced by buildAuctionPayload.
type AuctionLifecyclePayload struct {
	AuctionID     string  `json:"auction_id"`
	SellerID      string  `json:"seller_id"`
	Status        string  `json:"status"`
	CurrentBid    *int64  `json:"current_bid,omitempty"`
	CurrentWinner *string `json:"current_winner,omitempty"`
}

// SellerSubscriptionExpiringPayload represents the payload for seller.subscription.expiring events.
type SellerSubscriptionExpiringPayload struct {
	SubscriptionID  string `json:"subscription_id"`
	UserID          string `json:"user_id"`
	ExpiresAt       string `json:"expires_at"`
	DaysUntilExpiry int    `json:"days_until_expiry"`
}

// SellerSubscriptionExpiredPayload represents the payload for seller.subscription.expired events.
type SellerSubscriptionExpiredPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
}

// SellerTierChangedPayload represents the payload for seller tier change events.
// Produced by SellerReputationRecomputeWorker on each tier transition.
type SellerTierChangedPayload struct {
	SellerID     string `json:"seller_id"`
	PreviousTier string `json:"previous_tier"`
	NewTier      string `json:"new_tier"`
	EvaluatedAt  string `json:"evaluated_at"`
	WindowDays   int    `json:"window_days"`
}

// NegotiationPayload represents the payload for negotiation events.
type NegotiationPayload struct {
	SessionID    string `json:"session_id"`
	ChatRoomID   string `json:"chat_room_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	BuyerID      string `json:"buyer_id"`
	SellerID     string `json:"seller_id"`
	SenderID     string `json:"sender_id"`
	Price        int64  `json:"price,omitempty"`
	Status       string `json:"status,omitempty"`
}

// WithdrawalPayload represents the payload for withdrawal events.
type WithdrawalPayload struct {
	WithdrawalID string `json:"withdrawal_id"`
	SellerID     string `json:"seller_id"`
	Amount       int64  `json:"amount"`
	ApprovedBy   string `json:"approved_by,omitempty"`
	RejectedBy   string `json:"rejected_by,omitempty"`
}

// VerificationDocumentPayload represents the payload for verification document events.
type VerificationDocumentPayload struct {
	DocumentID   string `json:"document_id"`
	UserID       string `json:"user_id"`
	DocumentType string `json:"document_type"`
	ApprovedBy   string `json:"approved_by,omitempty"`
	RejectedBy   string `json:"rejected_by,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// SellerVerificationPayload represents the payload for seller-lifecycle
// verification events (seller.verification.*). The seller_id is the
// authoritative target.
type SellerVerificationPayload struct {
	SellerID    string `json:"seller_id"`
	Status      string `json:"status,omitempty"`
	Reason      string `json:"reason,omitempty"`
	ApprovedBy  string `json:"approved_by,omitempty"`
	RejectedBy  string `json:"rejected_by,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
}

// ExternalProductReviewPayload represents the payload for external_product.review.* events.
// Emitted by PromotionService.emitExternalProductReviewEventTx after each admin review decision.
type ExternalProductReviewPayload struct {
	OwnerUserID       string `json:"owner_user_id"`
	ExternalProductID string `json:"external_product_id"`
	Title             string `json:"title"`
	ReviewStatus      string `json:"review_status"`
	Reason            string `json:"reason,omitempty"`
	ReviewedBy        string `json:"reviewed_by,omitempty"`
}

// notificationInfo holds info needed for push notification after DB insert.
type notificationInfo struct {
	notificationID uuid.UUID // Set after DB insert
	inserted       bool      // True when a new DB row was inserted (not dedup no-op)
	recipientID    uuid.UUID
	actorID        uuid.UUID
	notifyType     string
	category       policy.NotificationCategory
	title          string
	body           string
	data           map[string]interface{} // Navigation data
	allowDB        bool                   // Whether to store in DB
	allowPush      bool                   // Whether to send push
	filterReason   string                 // Reason for filtering (for logging)
}

// Handle processes an outbox event and creates a notification.
//
// 1 event = 1 transaction pattern:
// - Extract payload
// - Insert notification with navigation data
// - Send push notification (non-blocking, failures don't affect notification record)
// - Mark outbox as succeeded (done by worker)
func (h *NotificationEventHandler) getTitleAndBody(notifyType string) (title, body string) {
	switch notifyType {
	case events.EventUserFollowed:
		return "New Follower", "Someone started following you"
	case events.EventContentLiked:
		return "New Like", "Someone liked your content"
	case "comment":
		return "New Comment", "Someone commented on your post"
	case "comment_reply":
		return "New Reply", "Someone replied to your comment"
	case "seller.response", "auction.response":
		return "Seller Responded", "A seller responded to your request"
	case "content.mentioned":
		return "Mentioned You", "Someone mentioned you in a post"
	case "chat_message":
		return "New Message", "You received a new message"
	// =============================================================================
	// ORDER NOTIFICATIONS
	// =============================================================================
	// Seller-facing notifications
	case events.EventOrderCreated:
		return "Order Baru", "Anda memiliki pesanan baru"
	case events.EventOrderPaid:
		return "Siap Dikirim", "Pembayaran masuk, pesanan siap dikirim"
	// Buyer-facing confirmation notifications (trust moments)
	case "order.created.buyer":
		return "Pesanan Berhasil Dibuat", "Pesanan Anda telah dibuat, silakan lanjutkan pembayaran"
	case "order.paid.buyer":
		return "Pembayaran Berhasil", "Pembayaran Anda telah diterima, pesanan sedang diproses"
	// Shipment notifications
	case "order.shipped":
		return "Pesanan Dikirim", "Pesanan Anda sedang dalam perjalanan"
	case events.EventOrderCompleted:
		// B4A: This is now the canonical acceptance notification.
		// "Terima Barang" → Complete() → this notification.
		return "Pesanan Diterima & Selesai", "Barang telah diterima, pembayaran diteruskan ke penjual"
	// Cancellation - explicit about WHO cancelled
	case "order.cancelled":
		return "Pesanan Dibatalkan", "Pembeli membatalkan pesanan ini"
	// Expiry and refunds
	case "order.expired":
		return "Pesanan Kadaluarsa", "Waktu pembayaran habis"
	case "order.refunded":
		return "Pengembalian Dana", "Dana akan dikembalikan ke metode pembayaran Anda"
	case "order.partially_refunded":
		return "Pengembalian Dana Sebagian", "Sebagian dana akan dikembalikan ke metode pembayaran Anda"
	// Dispute and confirmation events
	case "order.dispute_open":
		return "Pesanan Disengketakan", "Pembeli membuka sengketa untuk pesanan ini"
	case "order.confirmation_extended":
		return "Perpanjangan Konfirmasi", "Pembeli memperpanjang masa konfirmasi pesanan"
	// Refund lifecycle events (D1A)
	case "refund.opened":
		return "Pengajuan Refund", "Pembeli mengajukan refund pada pesanan ini"
	case "refund.escalated":
		return "Refund Dieskalasi", "Pembeli mengajukan sengketa ke admin."
	case "refund.approved":
		return "Refund Disetujui Penjual", "Refund disetujui. Proses pengembalian dana sedang berjalan."
	case "refund.rejected":
		return "Refund Ditolak Penjual", "Anda dapat mengajukan sengketa ke admin."
	// Dispute opened (D1B) is parked and not emitted by current runtime
	case "dispute.opened":
		return "Sengketa Dibuka (Pasca-Rilis)", "Event D1B; tidak dipakai oleh runtime aktif"
	// Dispute resolution (D1A) — generic; handler overrides per resolution type
	case events.EventDisputeResolved:
		return "Sengketa Selesai", "Sengketa telah diselesaikan"
	// Dispute aging — admin-only
	case "dispute.overdue":
		return "Sengketa Overdue", "Sengketa melampaui 3 hari — perlu eskalasi admin"
	case "dispute.timeout_escalation":
		return "Sengketa Kritis", "Sengketa melampaui batas waktu — tindakan admin segera diperlukan"
	// Money failure — admin-only
	case "money.refund_failed":
		return "Refund Gateway Gagal", "Pengembalian dana gagal diproses gateway. Tindakan admin diperlukan."
	case "order.cancelled_timeout":
		return "Pesanan Dibatalkan (Timeout)", "Batas waktu pengiriman terlampaui, pesanan otomatis dibatalkan"
	case "support.ticket.created":
		return "Tiket Support Baru", "Pengguna membuat tiket support baru"
	case "support.ticket.user_responded":
		return "User Membalas Tiket Support", "User telah membalas tiket support yang menunggu respons"
	// ==========================================================================
	// WITHDRAWAL NOTIFICATIONS
	// ==========================================================================
	case "withdrawal.requested":
		return "Permintaan Penarikan Dibuat", "Permintaan penarikan dana Anda telah dibuat"
	case "withdrawal.approved":
		return "Penarikan Disetujui", "Permintaan penarikan Anda telah disetujui dan sedang diproses"
	case "withdrawal.rejected":
		return "Penarikan Ditolak", "Permintaan penarikan Anda telah ditolak. Dana telah dikembalikan ke saldo Anda"
	case "withdrawal.completed":
		return "Penarikan Selesai", "Penarikan dana Anda telah selesai dan dana telah ditransfer"
	case "withdrawal.failed":
		return "Penarikan Gagal", "Penarikan dana Anda gagal diproses. Dana telah dikembalikan ke saldo Anda"
	// ==========================================================================
	// VERIFICATION DOCUMENT NOTIFICATIONS
	// ==========================================================================
	case "verification.document.approved":
		return "Verifikasi Disetujui", "Dokumen verifikasi Anda telah disetujui"
	case "verification.document.rejected":
		return "Verifikasi Ditolak", "Dokumen verifikasi Anda ditolak. Silakan periksa dan upload ulang."
	// ==========================================================================
	// NEGOTIATION NOTIFICATIONS
	// ==========================================================================
	case "negotiation.started":
		return "Tawaran Negosiasi Baru", "Seorang pembeli ingin menawar produk Anda"
	case "negotiation.message_sent":
		return "Tawaran Baru", "Ada tawaran baru pada negosiasi Anda"
	case "negotiation.accepted":
		return "Tawaran Diterima", "Penjual telah menerima tawaran Anda. Silakan lanjutkan ke pembayaran."
	case "negotiation.expired":
		return "Negosiasi Kadaluarsa", "Sesi negosiasi Anda telah kadaluarsa"
	case "negotiation.cancelled":
		return "Negosiasi Dibatalkan", "Sesi negosiasi telah dibatalkan"
	// ==========================================================================
	// SELLER TIER CHANGE NOTIFICATIONS — B1
	// ==========================================================================
	case "seller.tier.upgraded":
		return "Tier Penjual Naik", "Tier penjual Anda telah meningkat"
	case "seller.tier.downgraded":
		return "Tier Penjual Berubah", "Tier penjual Anda telah berubah"
	// ==========================================================================
	// SUPPORT TICKET NOTIFICATIONS (user-facing)
	// ==========================================================================
	case "support.ticket.resolved":
		return "Tiket Support Terselesaikan", "Tiket support Anda telah ditandai sebagai terselesaikan"
	case "support.ticket.closed":
		return "Tiket Support Ditutup", "Tiket support Anda telah ditutup"
	case "support.ticket_waiting_user":
		return "Tiket Support Menunggu Respon", "Admin menunggu respons Anda pada tiket support"
	// ==========================================================================
	// MODERATION NOTIFICATIONS
	// ==========================================================================
	case "moderation.content.removed":
		return "Konten Dihapus", "Konten Anda telah dihapus karena melanggar kebijakan komunitas"
	case "moderation.comment.removed":
		return "Komentar Dihapus", "Komentar Anda telah dihapus karena melanggar kebijakan komunitas"
	case "moderation.content.restored":
		return "Konten Dikembalikan", "Konten Anda telah dikembalikan setelah proses banding"
	case "moderation.comment.restored":
		return "Komentar Dikembalikan", "Komentar Anda telah dikembalikan setelah proses banding"
	case "moderation.for_sale.removed":
		return "Fixed-Price Sale Dihapus", "Fixed-price sale Anda telah dihapus karena melanggar kebijakan komunitas"
	case "moderation.for_sale.restored":
		return "Fixed-Price Sale Dikembalikan", "Banding Anda diterima. Silakan buat fixed-price sale baru jika diperlukan."
	case "moderation.auction.removed":
		return "Lelang Dihapus", "Lelang Anda telah dihapus karena melanggar kebijakan komunitas"
	case "moderation.auction.restored":
		return "Banding Lelang Diterima", "Banding Anda diterima. Buat lelang baru untuk melanjutkan penjualan."
	case "moderation.user.suspended":
		return "Akun Ditangguhkan", "Akun Anda telah ditangguhkan karena melanggar kebijakan komunitas"
	case "moderation.user.restored":
		return "Akun Dipulihkan", "Akun Anda telah dipulihkan setelah proses banding"
	case "moderation.warning.issued":
		return "Peringatan Diterima", "Anda menerima peringatan karena melanggar kebijakan komunitas"
	case "auction.bid.placed":
		return "Bid Baru", "Ada penawaran baru di lelang Anda"
	case "auction.waiting_settlement":
		return "Lelang Dimenangkan!", "Selamat! Anda memenangkan lelang. Segera klaim dalam 24 jam."
	case "auction.seller_has_winner":
		return "Ada Pemenang Lelang", "Lelang Anda memiliki pemenang. Tunggu hingga pembayaran masuk."
	case "auction.ended_no_winner":
		return "Lelang Berakhir Tanpa Pemenang", "Lelang Anda telah berakhir tanpa ada pemenang."
	case "auction.bnr_seller":
		return "Lelang Tidak Diselesaikan", "Pemenang tidak menyelesaikan pembayaran dalam batas waktu"
	case "auction.bnr_winner":
		return "Klaim Lelang Kedaluwarsa", "Batas waktu klaim lelang terlewat. Pelanggaran BNR tercatat."
	case "seller.subscription.expiring":
		return "Langganan Akan Berakhir", "Langganan Anda akan berakhir, segera perpanjang untuk melanjutkan penjualan"
	case "seller.subscription.expired":
		return "Langganan Berakhir", "Langganan Anda telah berakhir. Perpanjang untuk melanjutkan penjualan."
	// ==========================================================================
	// EXTERNAL PRODUCT REVIEW NOTIFICATIONS
	// ==========================================================================
	case "external_product.review.approved":
		return "Produk Eksternal Disetujui", "Produk eksternal Anda telah disetujui dan siap untuk dipromosikan"
	case "external_product.review.rejected":
		return "Produk Eksternal Ditolak", "Produk eksternal Anda ditolak. Periksa alasan dan ajukan kembali"
	case "external_product.review.request_changes":
		return "Perubahan Diperlukan", "Admin meminta perubahan pada produk eksternal Anda"
	case "external_product.review.hidden":
		return "Produk Eksternal Disembunyikan", "Produk eksternal Anda telah disembunyikan oleh admin"
	default:
		return "Notification", "You have a new notification"
	}
}

// ============================================================================
// POLICY LAYER HELPERS (SESSION 2)
// ============================================================================

// applyPolicyLayer applies all policy checks and returns the processed notification info.
// This is the MAIN ENTRY POINT for policy enforcement.
func (h *NotificationEventHandler) applyPolicyLayer(
	ctx context.Context,
	recipientID, actorID uuid.UUID,
	notifyType string,
	data map[string]interface{},
) notificationInfo {
	// STEP 1: Determine category
	category := policy.GetCategory(notifyType)

	// STEP 2: Account status filtering
	var allowDB, allowPush bool
	var filterReason string

	if h.policyFilter != nil {
		decision := h.policyFilter.ShouldDeliver(ctx, recipientID, category, notifyType)
		allowDB = decision.AllowDB
		allowPush = decision.AllowPush
		filterReason = decision.Reason
	} else {
		// No filter configured - allow everything (unsafe)
		allowDB = true
		allowPush = true
		filterReason = "no_filter_configured"
	}

	// STEP 3: Block policy check (for social notifications)
	finalActorID := actorID
	if category == policy.Social || category == policy.Marketing {
		if h.policyBlock != nil {
			action := h.policyBlock.ShouldApplyBlock(ctx, actorID, recipientID, category)
			if !action.Deliver {
				// Block this notification
				return notificationInfo{
					notificationID: uuid.Nil,
					recipientID:    recipientID,
					actorID:        actorID,
					notifyType:     notifyType,
					category:       category,
					title:          "",
					body:           "",
					data:           data,
					allowDB:        false,
					allowPush:      false,
					filterReason:   action.Reason,
				}
			}
			if action.Anonymize {
				// For social, anonymization means don't deliver
				return notificationInfo{
					notificationID: uuid.Nil,
					recipientID:    recipientID,
					actorID:        actorID,
					notifyType:     notifyType,
					category:       category,
					title:          "",
					body:           "",
					data:           data,
					allowDB:        false,
					allowPush:      false,
					filterReason:   "social_anonymized_to_blocked",
				}
			}
		}

		// STEP 3B: Actor lifecycle check — delivery-time sender governance.
		// Banned/deleted actors must not have their identity surface even for
		// historical outbox events; suspended actors may deliver in-app only.
		if h.policyFilter != nil {
			actorDecision := h.policyFilter.ShouldDeliverFromActor(ctx, actorID, category)
			if !actorDecision.AllowDB {
				return notificationInfo{
					notificationID: uuid.Nil,
					recipientID:    recipientID,
					actorID:        actorID,
					notifyType:     notifyType,
					category:       category,
					title:          "",
					body:           "",
					data:           data,
					allowDB:        false,
					allowPush:      false,
					filterReason:   actorDecision.Reason,
				}
			}
			// Suspended actor: restrict push even if recipient status allows it.
			if !actorDecision.AllowPush {
				allowPush = false
			}
		}

		// STEP 3C: Mute policy — chat notification surface only, shadow-first.
		// Recipient-muted-sender: emit telemetry in shadow mode; suppress in enforce mode.
		// Sender-muted-recipient has no delivery effect (direction-specific).
		// Block (STEP 3) always wins before mute is evaluated.
		if h.policyMute != nil && notifyType == "chat_message" {
			muteAction := h.policyMute.ShouldApplyMute(ctx, actorID, recipientID)
			if muteAction.PolicyError {
				h.log.Warn("mute.notification.error",
					zap.String("notify_type", notifyType),
					zap.String("reason", muteAction.Reason),
				)
			} else if muteAction.WouldSuppress {
				// Emit divergence telemetry whether shadow or enforce.
				h.log.Info("mute.notification.evaluated",
					zap.String("event", muteAction.Reason),
					zap.String("notify_type", notifyType),
					zap.String("mute_mode", string(h.policyMute.Mode())),
					zap.Bool("would_suppress", muteAction.WouldSuppress),
					zap.Bool("suppressed", muteAction.Suppressed),
				)
			}
			if muteAction.Suppressed {
				// Enforce mode: suppress both in-app and push.
				return notificationInfo{
					notificationID: uuid.Nil,
					recipientID:    recipientID,
					actorID:        actorID,
					notifyType:     notifyType,
					category:       category,
					title:          "",
					body:           "",
					data:           data,
					allowDB:        false,
					allowPush:      false,
					filterReason:   muteAction.Reason,
				}
			}
			// Shadow mode or not muted: continue delivery unchanged.
		}
	} else if category == policy.CommerceCritical || category == policy.Moderation {
		// STEP 4: Block bypass with anonymization for commerce/moderation
		if h.policyBlock != nil {
			action := h.policyBlock.ShouldApplyBlock(ctx, actorID, recipientID, category)
			if action.Anonymize {
				// Use role-based display instead of actor identity
				finalActorID = uuid.Nil
				// Add actor_display to data for frontend
				actorDisplay := policy.InferActorDisplayFromNotificationType(notifyType, actorID, recipientID, data)
				if data == nil {
					data = make(map[string]interface{})
				}
				data["actor_display"] = actorDisplay
			}
		}
	}

	// Get title and body
	title, body := h.getTitleAndBody(notifyType)

	return notificationInfo{
		notificationID: uuid.Nil, // Set after DB insert
		recipientID:    recipientID,
		actorID:        finalActorID,
		notifyType:     notifyType,
		category:       category,
		title:          title,
		body:           body,
		data:           data,
		allowDB:        allowDB,
		allowPush:      allowPush,
		filterReason:   filterReason,
	}
}

// insertNotificationWithPolicy inserts a notification after applying policy checks.
// Returns notificationInfo with policy decisions applied.
//
// AUDIT TRAIL (SESSION 3): Logs delivery events to notification_delivery_log.
//
// Optional preconditions run INSIDE the same transaction as the notification
// insert. When a precondition returns false the insert is skipped (treated as
// a dedup no-op: no notification, no push). Used by content.liked to confirm
// the like row still exists at delivery time so an UNLIKE that raced an
// in-flight event never leaves a stale notification.
func (h *NotificationEventHandler) insertNotificationWithPolicy(
	ctx context.Context,
	recipientID, actorID uuid.UUID,
	notifyType string,
	entityID uuid.UUID,
	data map[string]interface{},
	preconditions ...func(tx dbpkg.Tx) (bool, error),
) (notificationInfo, error) {
	// Apply policy layer
	info := h.applyPolicyLayer(ctx, recipientID, actorID, notifyType, data)

	// Generate a notification ID for logging (will be replaced by actual ID after insert)
	notificationID := uuid.New()

	// Check if DB insertion is allowed
	if !info.allowDB {
		h.log.Debug("Notification blocked by policy (DB)",
			zap.String("recipient_id", recipientID.String()),
			zap.String("type", notifyType),
			zap.String("reason", info.filterReason),
		)

		// AUDIT: Log skipped notification
		h.logDelivery(ctx, notificationID, recipientID, "in_app", "skipped", info.filterReason, map[string]interface{}{
			"category": string(info.category),
			"type":     notifyType,
		})

		info.notificationID = notificationID
		info.inserted = false
		return info, nil // Return empty info, no error
	}

	// Insert notification
	var insertedNotificationID uuid.UUID
	err := h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		for _, pre := range preconditions {
			if pre == nil {
				continue
			}
			ok, preErr := pre(tx)
			if preErr != nil {
				return preErr
			}
			if !ok {
				// Precondition not met (e.g. like row already removed): treat
				// as handled no-op, no notification, no push.
				return nil
			}
		}
		insertedID, err := h.notificationInserter.InsertNotification(
			ctx, tx,
			recipientID, info.actorID,
			notifyType,
			entityID,
			info.data,
		)
		if err == nil {
			insertedNotificationID = insertedID
		}
		return err
	})

	if err != nil {
		// AUDIT: Log failed insertion
		h.logDelivery(ctx, notificationID, recipientID, "in_app", "failed", err.Error(), map[string]interface{}{
			"category": string(info.category),
			"type":     notifyType,
		})

		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}

	// ON CONFLICT DO NOTHING path: dedup no-op. Treated as handled (idempotent)
	// but push must NOT be sent.
	if insertedNotificationID == uuid.Nil {
		info.notificationID = uuid.Nil
		info.inserted = false
		h.logDelivery(ctx, notificationID, recipientID, "in_app", "skipped", "dedup_no_insert", map[string]interface{}{
			"category": string(info.category),
			"type":     notifyType,
		})
		return info, nil
	}

	// Set the actual notification ID for push logging
	info.notificationID = insertedNotificationID
	info.inserted = true

	// AUDIT: Log successful in-app delivery
	metadata := map[string]interface{}{
		"category":  string(info.category),
		"type":      notifyType,
		"allowPush": info.allowPush,
	}
	if info.actorID != actorID {
		metadata["anonymized"] = true
	}

	h.logDelivery(ctx, insertedNotificationID, recipientID, "in_app", "sent", "", metadata)

	return info, nil
}

// logDelivery is a helper that logs delivery events without affecting the main flow.
// Logging failures are silently ignored to ensure delivery is not impacted.
func (h *NotificationEventHandler) logDelivery(
	ctx context.Context,
	notificationID, recipientID uuid.UUID,
	channel, status, reason string,
	metadata map[string]interface{},
) {
	if h.deliveryLogger == nil {
		return
	}

	// Non-blocking async logging
	go func() {
		// Use background context to ensure logging completes even if caller cancels
		bgCtx := context.Background()
		h.deliveryLogger.LogDelivery(bgCtx, notificationID, recipientID, channel, status, reason, metadata)
	}()
}

// handleUserFollowed processes user.followed events.
// UPDATED: Uses policy layer for filtering (SESSION 2).
