package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	negotiationApp "github.com/labuda/backend/internal/commerce/negotiation/application"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationImpl "github.com/labuda/backend/internal/commerce/negotiation/infrastructure/repository"
	negotiationRepo "github.com/labuda/backend/internal/commerce/negotiation/repository"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/governance/viewercontext"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/internal/pkg/blockcheck"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/internal/platform/s3presign"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// handlerAccountStatusChecker is the handler-layer interface for account status enforcement.
// Defined locally to avoid importing the auth package from this delivery layer.
type handlerAccountStatusChecker interface {
	EnsureActive(ctx context.Context, userID uuid.UUID) error
}

// Handler handles HTTP requests for chat operations.
//
// N6: orderService and pricingTokenService were removed from this struct — they
// existed solely for the deleted POST /chat/rooms/:room_id/order endpoint.
// Canonical negotiation checkout lives entirely in pricing preview + POST /orders.
type Handler struct {
	chatService                *chatApp.Service
	negotiationRepo            negotiationRepo.Repository
	negotiationService         *negotiationApp.NegotiationService
	statusChecker              handlerAccountStatusChecker // Account status enforcement
	db                         *db.DB
	log                        *zap.Logger
	resourceProjectionResolver chatApp.ResourceProjectionResolver

	// shippingQuoteProjectionResolver projects a viewer-scoped actionability
	// envelope onto shipping-quote messages. Implemented by the Shipping
	// commerce authority in wiring; nil means no projection hydration.
	shippingQuoteProjectionResolver chatApp.ShippingQuoteProjectionResolver

	// mediaPresign is the canonical S3 presign configuration for chat media.
	// When AccessKey is empty (CI / unconfigured env) the register endpoint
	// answers 503 instead of minting an unusable URL.
	mediaPresign    s3presign.Config
	mediaCDNBaseURL string
}

// NewHandler creates a new chat handler.
func NewHandler(
	chatService *chatApp.Service,
	negotiationService *negotiationApp.NegotiationService,
	statusChecker handlerAccountStatusChecker,
	database *db.DB,
	log *zap.Logger,
) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{
		chatService:        chatService,
		negotiationRepo:    negotiationImpl.NewNegotiationRepository(),
		negotiationService: negotiationService,
		statusChecker:      statusChecker,
		db:                 database,
		log:                log,
	}
}

// SetResourceProjectionResolver injects the canonical resource projection
// resolver used by ListMessages (and eventually SendMessage) to hydrate
// resource_projection blocks on messages that carry a resource occurrence.
// Call once during handler wiring; nil means no projection hydration.
func (h *Handler) SetResourceProjectionResolver(r chatApp.ResourceProjectionResolver) {
	h.resourceProjectionResolver = r
}

// SetShippingQuoteProjectionResolver injects the canonical shipping-quote
// projection resolver used by the message read paths to hydrate the
// viewer-scoped `shipping_quote_projection` envelope on shipping-quote
// messages. Call once during handler wiring; nil means no projection hydration.
func (h *Handler) SetShippingQuoteProjectionResolver(r chatApp.ShippingQuoteProjectionResolver) {
	h.shippingQuoteProjectionResolver = r
}

// SetMediaPresigner injects the S3 presign configuration used by the chat media
// register endpoint. Call once during handler wiring; an empty AccessKey leaves
// the endpoint answering 503 (same contract as the general media upload
// handler). The CDN base is shared with mediaresolve so the read_url the client
// gets at register time is the same URL every later read resolves to.
func (h *Handler) SetMediaPresigner(cfg s3presign.Config, cdnBaseURL string) {
	h.mediaPresign = cfg
	h.mediaCDNBaseURL = strings.TrimRight(strings.TrimSpace(cdnBaseURL), "/")
}

// ========================================================================
// REQUEST DTOs
// ========================================================================

// SendMessageRequest holds the request body for sending a message.
type SendMessageRequest struct {
	MessageType    string                 `json:"message_type" binding:"required,oneof=text negotiation_proposal system"`
	Body           string                 `json:"body"`
	AttachmentJSON map[string]interface{} `json:"attachment_json"`
	// MediaAssetIDs are the chat media assets this message carries, in display
	// order. Each id comes from POST /chat/rooms/:room_id/media (register step)
	// and is attached ATOMICALLY with the message: either the message exists
	// with all of its media, or it does not exist at all.
	MediaAssetIDs  []uuid.UUID `json:"media_asset_ids"`
	IdempotencyKey string      `json:"idempotency_key" binding:"required"`

	// ResourceOccurrence is the optional communication reference carried by
	// the message (the resource the message is about). Preview/snapshot data
	// is intentionally not accepted — the server builds any display fallback.
	ResourceOccurrence *resourceOccurrenceRequest `json:"resource_occurrence"`
}

// resourceOccurrenceRequest is the canonical wire shape for a message
// resource reference. It carries identity only.
type resourceOccurrenceRequest struct {
	Operation    string    `json:"operation"`
	ResourceType string    `json:"resource_type"`
	ResourceID   uuid.UUID `json:"resource_id"`
	Preview      any       `json:"preview,omitempty"`
}

// MarkAsReadRequest holds the request body for marking messages as read.
type MarkAsReadRequest struct {
	Timestamp string `json:"timestamp" binding:"required"`
}

// ========================================================================
// ROOM ENDPOINTS
// ========================================================================

// ListRooms handles GET /api/v1/chat/rooms
//
// Lists all chat rooms for the authenticated user.
// Query parameters:
//   - cursor_last_message_at: ISO 8601 timestamp for pagination
//   - cursor_id: UUID for pagination
//   - limit: number of rooms to return (default: 50, max: 100)
func (h *Handler) ListRooms(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse cursor
	var cursorLastMessageAt *time.Time
	var cursorID *uuid.UUID

	if cursorStr := c.Query("cursor_last_message_at"); cursorStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, cursorStr); err == nil {
			cursorLastMessageAt = &t
		}
	}

	if cursorStr := c.Query("cursor_id"); cursorStr != "" {
		if id, err := uuid.Parse(cursorStr); err == nil {
			cursorID = &id
		}
	}

	// Parse limit
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Execute query
	rooms, err := h.chatService.ListRoomsByUser(ctx, userID, cursorLastMessageAt, cursorID, limit)
	if err != nil {
		h.log.Error("Failed to list rooms",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve rooms")
		return
	}

	// Block enforcement: hide rooms with blocked participants.
	// EXCEPTION: order-linked rooms and support rooms are preserved (commerce continuity).
	if len(rooms) > 0 {
		otherIDs := make([]uuid.UUID, 0, len(rooms))
		for _, room := range rooms {
			otherIDs = append(otherIDs, room.OtherParticipant(userID))
		}
		var blockedSet map[uuid.UUID]bool
		_ = h.db.WithTx(ctx, func(tx db.Tx) error {
			var err error
			blockedSet, err = blockcheck.BlockedSet(ctx, tx, userID, otherIDs)
			return err
		})
		if len(blockedSet) > 0 {
			filtered := make([]*chatEntity.ChatRoom, 0, len(rooms))
			for _, room := range rooms {
				other := room.OtherParticipant(userID)
				if blockedSet[other] {
					// Exempt: order-linked or support rooms
					if room.HasOrderContext() || room.RoomType == chatEntity.RoomTypeSupport {
						filtered = append(filtered, room)
					}
					// else: hidden (blocked social room)
				} else {
					filtered = append(filtered, room)
				}
			}
			rooms = filtered
		}
	}

	// Batch-hydrate ChatParticipantCards for every distinct other-participant
	// in the list. Single SQL via buildChatParticipantCardsWithLifecycle;
	// no N+1. Lifecycle field populated per E4.2 doctrine.
	participantCards := h.hydrateRoomParticipants(ctx, rooms, userID)

	roomIDs := make([]uuid.UUID, 0, len(rooms))
	for _, room := range rooms {
		roomIDs = append(roomIDs, room.ID)
	}

	latestMessageByRoom, latestErr := h.batchLatestMessages(ctx, roomIDs)
	if latestErr != nil {
		h.log.Warn("chat: list-rooms failed to batch load latest message preview",
			zap.String("user_id", userID.String()),
			zap.Error(latestErr),
		)
		latestMessageByRoom = map[uuid.UUID]*chatEntity.ChatMessage{}
	}

	unreadCountByRoom, unreadErr := h.chatService.GetUnreadCountsByRooms(ctx, roomIDs, userID)
	if unreadErr != nil {
		h.log.Warn("chat: list-rooms failed to load unread counts",
			zap.String("user_id", userID.String()),
			zap.Error(unreadErr),
		)
		unreadCountByRoom = map[uuid.UUID]int{}
	}

	// Resource projection hydration for the room-list preview: the
	// representation is derived from the latest Chat message's occurrence, not
	// from an embedded Commerce object. Degrade gracefully (the room list is a
	// primary surface) — a projection failure must not fail the list.
	latestMessages := make([]*chatEntity.ChatMessage, 0, len(latestMessageByRoom))
	for _, msg := range latestMessageByRoom {
		latestMessages = append(latestMessages, msg)
	}

	// Chat media hydration for the preview: "🙏đ Foto" in a room list comes from
	// the message's media, not from its text.
	latestMedia := h.hydrateMessageMedia(ctx, latestMessages)
	latestProjections, projErr := h.resolveMessageProjections(ctx, userID, latestMessages)
	if projErr != nil {
		h.log.Warn("chat: list-rooms failed to resolve resource projections",
			zap.String("user_id", userID.String()),
			zap.Error(projErr),
		)
		latestProjections = map[uuid.UUID]*commerceshared.ResourceProjection{}
	}

	// Shipping-quote actionability projection for the preview. Degrade
	// gracefully like the resource projection: a failure must not fail the list.
	latestShippingProjections, shipProjErr := h.resolveShippingQuoteProjections(ctx, userID, latestMessages)
	if shipProjErr != nil {
		h.log.Warn("chat: list-rooms failed to resolve shipping quote projections",
			zap.String("user_id", userID.String()),
			zap.Error(shipProjErr),
		)
		latestShippingProjections = map[uuid.UUID]chatApp.ShippingQuoteProjection{}
	}

	// Convert to response
	data := make([]map[string]interface{}, len(rooms))
	for i, room := range rooms {
		latestMessage := latestMessageByRoom[room.ID]
		unreadCount := unreadCountByRoom[room.ID]

		var latestMediaURLs []string
		if latestMessage != nil {
			latestMediaURLs = latestMedia[latestMessage.ID]
		}

		data[i] = roomListItemResponse(room, userID, participantCards, latestMessage, unreadCount, latestProjections, latestShippingProjections, latestMediaURLs)
	}

	response.Success(c, gin.H{
		"data": data,
	})
}

// GetRoom handles GET /api/v1/chat/rooms/:room_id
//
// Returns a single chat room for the authenticated participant. This is the
// canonical single-room read: it uses the existing canonical room reader
// (chatService.GetRoom) and the same room-summary contract as a room-list item
// (roomListItemResponse), so the mobile conversation surface can render the
// room identity it was opened for.
//
// Authorization is the existing Chat authority — no parallel rule:
//   - the caller must be a participant of the room (same as ListMessages);
//   - a blocked direct room resolves to NotFound, exactly as the room list
//     hides it and ListMessages denies its messages;
//   - order-linked and support rooms stay visible (commerce continuity).
func (h *Handler) GetRoom(c *gin.Context) {
	ctx := c.Request.Context()

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

	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		response.BadRequest(c, "Invalid room ID")
		return
	}

	room, err := h.chatService.GetRoom(ctx, roomID)
	if err != nil {
		if err == chatRepo.ErrRoomNotFound {
			response.NotFound(c, "Room not found")
			return
		}
		h.log.Error("Failed to get room",
			zap.String("room_id", roomID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve room")
		return
	}

	// Authorization: participant-only. Same authority and status contract as
	// ListMessages' ErrParticipantMismatch.
	if !room.HasParticipant(userID) {
		response.Forbidden(c, "You are not a participant in this room")
		return
	}

	// Block enforcement: deny reads for blocked social rooms.
	// EXCEPTION: order-linked and support rooms are preserved.
	if !room.HasOrderContext() && room.RoomType != chatEntity.RoomTypeSupport {
		other := room.OtherParticipant(userID)
		var blocked bool
		_ = h.db.WithTx(ctx, func(tx db.Tx) error {
			blocked, _ = blockcheck.IsBidirectionallyBlocked(ctx, tx, userID, other)
			return nil
		})
		if blocked {
			response.NotFound(c, "Room not found")
			return
		}
	}

	// Same hydration contract as a room-list item: participant card, latest
	// message preview, viewer-scoped unread count, and resource projection.
	cards := h.hydrateRoomParticipants(ctx, []*chatEntity.ChatRoom{room}, userID)

	latestMessageByRoom, latestErr := h.batchLatestMessages(ctx, []uuid.UUID{room.ID})
	if latestErr != nil {
		h.log.Warn("chat: get-room failed to load latest message preview",
			zap.String("room_id", room.ID.String()),
			zap.Error(latestErr),
		)
		latestMessageByRoom = map[uuid.UUID]*chatEntity.ChatMessage{}
	}

	unreadCountByRoom, unreadErr := h.chatService.GetUnreadCountsByRooms(ctx, []uuid.UUID{room.ID}, userID)
	if unreadErr != nil {
		h.log.Warn("chat: get-room failed to load unread count",
			zap.String("room_id", room.ID.String()),
			zap.Error(unreadErr),
		)
		unreadCountByRoom = map[uuid.UUID]int{}
	}

	latestMessage := latestMessageByRoom[room.ID]

	var projections map[uuid.UUID]*commerceshared.ResourceProjection
	if latestMessage != nil {
		resolved, projErr := h.resolveMessageProjections(ctx, userID, []*chatEntity.ChatMessage{latestMessage})
		if projErr != nil {
			h.log.Warn("chat: get-room failed to resolve resource projections",
				zap.String("room_id", room.ID.String()),
				zap.Error(projErr),
			)
		} else {
			projections = resolved
		}
	}

	var latestMediaURLs []string
	if latestMessage != nil {
		latestMediaURLs = h.hydrateMessageMedia(ctx, []*chatEntity.ChatMessage{latestMessage})[latestMessage.ID]
	}

	var shippingProjections map[uuid.UUID]chatApp.ShippingQuoteProjection
	if latestMessage != nil {
		resolved, shipProjErr := h.resolveShippingQuoteProjections(ctx, userID, []*chatEntity.ChatMessage{latestMessage})
		if shipProjErr != nil {
			h.log.Warn("chat: get-room failed to resolve shipping quote projections",
				zap.String("room_id", room.ID.String()),
				zap.Error(shipProjErr),
			)
		} else {
			shippingProjections = resolved
		}
	}

	response.Success(c, roomListItemResponse(
		room,
		userID,
		cards,
		latestMessage,
		unreadCountByRoom[room.ID],
		projections,
		shippingProjections,
		latestMediaURLs,
	))
}

// GetOrCreateDirectRoom handles POST /api/v1/chat/direct/:user_id
//
// Gets or creates a direct chat room with another user.
func (h *Handler) GetOrCreateDirectRoom(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse other user ID
	otherUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Get or create room
	room, err := h.chatService.GetOrCreateDirectRoom(ctx, userID, otherUserID)
	if err != nil {
		if err == chatRepo.ErrSelfChat {
			response.BadRequest(c, "Cannot create chat with yourself")
			return
		}
		if err == chatRepo.ErrRateLimited {
			response.TooManyRequests(c, "Too many room creation attempts. Please try again later.")
			return
		}
		if err == chatRepo.ErrUserBlocked {
			response.Error(c, 403, "USER_BLOCKED", "Cannot create a room with this user.")
			return
		}
		if response.IsAuthError(err) {
			response.RespondWithError(c, h.log, err)
			return
		}
		h.log.Error("Failed to get or create direct room",
			zap.String("user_id", userID.String()),
			zap.String("other_user_id", otherUserID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to get or create room")
		return
	}

	cards := h.hydrateRoomParticipants(ctx, []*chatEntity.ChatRoom{room}, userID)
	response.Success(c, roomToResponse(room, userID, cards))
}

// GetRoomByOrderID handles GET /api/v1/chat/rooms/by-order/:order_id
//
// Gets a chat room by linked order ID for commerce continuity.
// Returns the room with last 50 messages for context.
//
// Authorization: Buyer, Seller, or Admin
func (h *Handler) GetRoomByOrderID(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse order ID
	orderID, err := uuid.Parse(c.Param("order_id"))
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	// Get room by order ID
	room, err := h.chatService.GetRoomByOrderID(ctx, orderID)
	if err != nil {
		if err == chatRepo.ErrRoomNotFound {
			response.NotFound(c, "Room not found for this order")
			return
		}
		h.log.Error("Failed to get room by order ID",
			zap.String("order_id", orderID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve room")
		return
	}

	// Authorization check: only buyer, seller, or admin can access
	isAdmin, _ := c.Get("isAdmin")
	if !room.HasParticipant(userID) && isAdmin != true {
		response.Forbidden(c, "You are not authorized to access this room")
		return
	}

	// Get last 50 messages for context
	messages, err := h.chatService.ListMessages(ctx, room.ID, userID, nil, nil, 50)
	if err != nil {
		h.log.Error("Failed to list messages for room",
			zap.String("room_id", room.ID.String()),
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		// Continue anyway - we can still return the room
		messages = []*chatEntity.ChatMessage{}
	}

	// Batch-hydrate ChatParticipantCards for room + every distinct sender.
	participantCards := h.hydrateRoomParticipants(ctx, []*chatEntity.ChatRoom{room}, userID)
	senderCards := h.hydrateMessageSenders(ctx, messages)
	sellerLifecycles := h.hydrateAttachmentSellerLifecycles(ctx, messages)

	// Media hydration: ONE batch query for the page, resolved through the same
	// read authority (mediaresolve) every other media surface uses.
	mediaByMessage := h.hydrateMessageMedia(ctx, messages)

	// Convert messages to response
	messageData := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		messageData[i] = messageToResponse(msg, senderCards, sellerLifecycles, mediaByMessage[msg.ID])
	}

	// Build response with room and messages
	resp := roomToResponse(room, userID, participantCards)
	resp["messages"] = messageData

	response.Success(c, resp)
}

// LinkOrderToChat handles PUT /api/v1/chat/rooms/:room_id/link-order
//
// Links an order to a chat room for commerce continuity.
// This is used when:
// - Order is created from chat (chat-born order)
// - User navigates from order detail to chat (direct order → chat continuity)
//
// Request body:
//   - order_id: UUID of the order to link
func (h *Handler) LinkOrderToChat(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse room ID
	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		response.BadRequest(c, "Invalid room ID")
		return
	}

	// Parse request body
	var req struct {
		OrderID string `json:"order_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	// Link order to chat
	room, err := h.chatService.LinkOrderToChat(ctx, roomID, orderID, userID)
	if err != nil {
		if err == chatRepo.ErrRoomNotFound {
			response.NotFound(c, "Room not found")
			return
		}
		if err == chatRepo.ErrParticipantMismatch {
			response.Forbidden(c, "You are not a participant in this room")
			return
		}
		if err == chatRepo.ErrOrderNotFound {
			response.NotFound(c, "Order not found")
			return
		}
		if err == chatRepo.ErrOrderOwnershipMismatch {
			response.Error(c, 403, "ORDER_OWNERSHIP_MISMATCH", "You are not the buyer or seller of this order")
			return
		}
		if err == chatRepo.ErrOrderRoomParticipantMismatch {
			response.Error(c, 403, "ORDER_ROOM_MISMATCH", "This order does not belong to this chat room's participants")
			return
		}
		if err == chatRepo.ErrOrderAlreadyLinkedElsewhere {
			response.Error(c, 409, "ORDER_ALREADY_LINKED", "This order is already linked to a different chat room")
			return
		}
		h.log.Error("Failed to link order to chat",
			zap.String("room_id", roomID.String()),
			zap.String("order_id", orderID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to link order to chat")
		return
	}

	cards := h.hydrateRoomParticipants(ctx, []*chatEntity.ChatRoom{room}, userID)
	response.Success(c, roomToResponse(room, userID, cards))
}

// ========================================================================
// MESSAGE ENDPOINTS
// ========================================================================

// ListMessages handles GET /api/v1/chat/rooms/:room_id/messages
//
// Lists messages in a chat room.
// Query parameters:
//   - cursor_created_at: ISO 8601 timestamp for pagination
//   - cursor_id: UUID for pagination
//   - limit: number of messages to return (default: 50, max: 100)
func (h *Handler) ListMessages(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse room ID
	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		response.BadRequest(c, "Invalid room ID")
		return
	}

	// Parse cursor
	var cursorCreatedAt *time.Time
	var cursorID *uuid.UUID

	if cursorStr := c.Query("cursor_created_at"); cursorStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, cursorStr); err == nil {
			cursorCreatedAt = &t
		}
	}

	if cursorStr := c.Query("cursor_id"); cursorStr != "" {
		if id, err := uuid.Parse(cursorStr); err == nil {
			cursorID = &id
		}
	}

	// Parse limit
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Block enforcement: deny message reads for blocked social rooms.
	// EXCEPTION: order-linked and support rooms are preserved.
	if room, roomErr := h.chatService.GetRoom(ctx, roomID); roomErr == nil && room.HasParticipant(userID) {
		if !room.HasOrderContext() && room.RoomType != chatEntity.RoomTypeSupport {
			other := room.OtherParticipant(userID)
			var blocked bool
			_ = h.db.WithTx(ctx, func(tx db.Tx) error {
				blocked, _ = blockcheck.IsBidirectionallyBlocked(ctx, tx, userID, other)
				return nil
			})
			if blocked {
				response.NotFound(c, "Room not found")
				return
			}
		}
	}

	// Execute query
	messages, err := h.chatService.ListMessages(ctx, roomID, userID, cursorCreatedAt, cursorID, limit)
	if err != nil {
		if err == chatRepo.ErrParticipantMismatch {
			response.Forbidden(c, "You are not a participant in this room")
			return
		}
		if err == chatRepo.ErrRoomNotFound {
			response.NotFound(c, "Room not found")
			return
		}
		h.log.Error("Failed to list messages",
			zap.String("room_id", roomID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve messages")
		return
	}

	// Batch-hydrate ChatParticipantCards for every distinct sender. Single
	// SQL via publiccard.BuildMany; no N+1.
	senderCards := h.hydrateMessageSenders(ctx, messages)
	sellerLifecycles := h.hydrateAttachmentSellerLifecycles(ctx, messages)

	// Media hydration: ONE batch query for the page (no N+1).
	mediaByMessage := h.hydrateMessageMedia(ctx, messages)

	// Convert to response
	data := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		data[i] = messageToResponse(msg, senderCards, sellerLifecycles, mediaByMessage[msg.ID])
	}

	// Resource projection hydration: batch-fetch occurrences, resolve via the
	// canonical aggregate resolver, and attach the projection envelope to each
	// message response.
	projections, projErr := h.resolveMessageProjections(ctx, userID, messages)
	if projErr != nil {
		h.log.Error("Failed to resolve resource projections",
			zap.String("room_id", roomID.String()),
			zap.Error(projErr),
		)
		response.InternalServerError(c, "Failed to resolve resource projections")
		return
	}
	for i, msg := range messages {
		if proj, ok := projections[msg.ID]; ok {
			data[i]["resource_projection"] = proj
		}
	}

	// Shipping-quote actionability projection: viewer-scoped, resolved in ONE
	// batch call by the Shipping commerce authority. Fail closed — a
	// shipping-quote message without its projection would render a lying CTA.
	shippingProjections, shipErr := h.resolveShippingQuoteProjections(ctx, userID, messages)
	if shipErr != nil {
		h.log.Error("Failed to resolve shipping quote projections",
			zap.String("room_id", roomID.String()),
			zap.Error(shipErr),
		)
		response.InternalServerError(c, "Failed to resolve shipping quote projections")
		return
	}
	for i, msg := range messages {
		if p, ok := shippingProjections[msg.ID]; ok {
			data[i]["shipping_quote_projection"] = shippingQuoteProjectionJSON(p)
		}
	}

	response.Success(c, gin.H{
		"data": data,
	})
}

// SendMessage handles POST /api/v1/chat/rooms/:room_id/messages
//
// Sends a message to a chat room.
func (h *Handler) SendMessage(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	senderID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse room ID
	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		response.BadRequest(c, "Invalid room ID")
		return
	}

	// Parse request body
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Validate attachment structure if provided
	if req.AttachmentJSON != nil {
		validationErrs := ValidateAttachmentJSON(req.AttachmentJSON)
		if HasValidationErrors(validationErrs) {
			response.ValidationError(c, gin.H{
				"field":  "attachment_json",
				"errors": validationErrs,
			})
			return
		}
	}

	// Map message type
	messageType := chatEntity.MessageType(req.MessageType)
	if !messageType.IsValid() {
		response.BadRequest(c, "Invalid message type")
		return
	}

	// A text message must carry a body OR media OR a commerce reference: a
	// caption-less foto/video message is legitimate, a product-only share is
	// legitimate, an entirely empty message is not.
	var body *string
	if messageType == chatEntity.MessageTypeText {
		if req.Body == "" && len(req.MediaAssetIDs) == 0 && req.ResourceOccurrence == nil && req.AttachmentJSON == nil {
			response.BadRequest(c, "Body is required for text messages without media or product reference")
			return
		}
		if req.Body != "" {
			body = &req.Body
		}
	} else if req.Body != "" {
		body = &req.Body
	}

	// Resource occurrence (communication reference) contract validation.
	// Chat validates only the REFERENCE CONTRACT + communication permission —
	// never Commerce business truth (price/availability/lifecycle).
	var occurrence *chatEntity.ResourceOccurrenceIdentity
	if req.ResourceOccurrence != nil {
		if req.ResourceOccurrence.Preview != nil {
			response.BadRequest(c, "Invalid request: resource_occurrence.preview is not supported")
			return
		}
		op := chatEntity.ResourceOccurrenceOperation(req.ResourceOccurrence.Operation)
		rt := chatEntity.ResourceOccurrenceResourceType(req.ResourceOccurrence.ResourceType)
		if !op.IsValid() || !rt.IsValid() || req.ResourceOccurrence.ResourceID == uuid.Nil {
			response.BadRequest(c, "Invalid request: invalid resource occurrence")
			return
		}
		if op == chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat && !rt.CanDirectCommerceInsert() {
			response.BadRequest(c, "Invalid request: invalid resource occurrence")
			return
		}
		occurrence = &chatEntity.ResourceOccurrenceIdentity{
			Operation:    op,
			ResourceType: rt,
			ResourceID:   req.ResourceOccurrence.ResourceID,
		}
	}

	// Send message within transaction
	var message *chatEntity.ChatMessage
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var svcErr error
		message, svcErr = h.chatService.SendMessageWithResourceOccurrence(
			ctx,
			roomID,
			senderID,
			messageType,
			body,
			req.AttachmentJSON,
			req.IdempotencyKey,
			req.MediaAssetIDs,
			occurrence,
		)
		return svcErr
	})

	if err != nil {
		if err == chatRepo.ErrInvalidIdempotencyKey {
			response.BadRequest(c, "Idempotency key is required")
			return
		}
		if err == chatRepo.ErrIdempotencyKeyConflict {
			response.Error(c, 409, "IDEMPOTENCY_KEY_CONFLICT", "This idempotency key was already used with a different message")
			return
		}
		if err == chatRepo.ErrInvalidResourceOccurrence {
			response.BadRequest(c, "Invalid resource occurrence")
			return
		}
		if err == chatRepo.ErrInvalidMessageType {
			response.BadRequest(c, "Invalid message type")
			return
		}
		if err == chatRepo.ErrParticipantMismatch {
			response.Forbidden(c, "You are not a participant in this room")
			return
		}
		if err == chatRepo.ErrRoomNotFound {
			response.NotFound(c, "Room not found")
			return
		}
		if err == chatRepo.ErrTooManyMediaAssets {
			response.BadRequest(c, "A message can carry at most 5 media items")
			return
		}
		if err == chatRepo.ErrMediaAssetNotFound {
			response.BadRequest(c, "Unknown media attachment")
			return
		}
		if err == chatRepo.ErrMediaAssetForbidden {
			response.Forbidden(c, "Media attachment does not belong to you or this room")
			return
		}
		if err == chatRepo.ErrMediaAssetNotAttachable {
			response.Error(c, 409, "MEDIA_ASSET_NOT_ATTACHABLE",
				"Media upload expired or was already attached. Please upload it again.")
			return
		}
		if err == chatRepo.ErrRateLimited {
			response.TooManyRequests(c, "You are sending messages too fast. Please slow down.")
			return
		}
		if err == chatRepo.ErrAttachmentForSaleNotFound {
			response.BadRequest(c, "Commerce attachment references a non-existent fixed-price sale")
			return
		}
		if err == chatRepo.ErrAttachmentAuctionNotFound {
			response.BadRequest(c, "Auction attachment references a non-existent auction")
			return
		}
		if err == chatRepo.ErrAttachmentPostNotFound {
			response.BadRequest(c, "Post attachment references a non-existent post")
			return
		}
		if err == chatRepo.ErrAttachmentRequestNotFound {
			response.BadRequest(c, "Request attachment references a non-existent request")
			return
		}
		if err == chatRepo.ErrAttachmentProfileNotFound {
			response.BadRequest(c, "Profile attachment references a non-existent profile")
			return
		}
		if err == chatRepo.ErrUserBlocked {
			response.Error(c, 403, "USER_BLOCKED", "You cannot send messages to this user.")
			return
		}
		if response.IsAuthError(err) {
			response.RespondWithError(c, h.log, err)
			return
		}
		h.log.Error("Failed to send message",
			zap.String("room_id", roomID.String()),
			zap.String("sender_id", senderID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to send message")
		return
	}

	senderCards := h.hydrateMessageSenders(ctx, []*chatEntity.ChatMessage{message})
	sellerLifecycles := h.hydrateAttachmentSellerLifecycles(ctx, []*chatEntity.ChatMessage{message})
	mediaURLs := h.hydrateMessageMedia(ctx, []*chatEntity.ChatMessage{message})
	resp := messageToResponse(message, senderCards, sellerLifecycles, mediaURLs[message.ID])

	// Attach the viewer-aware resource projection when the message carries a
	// resource occurrence. Projection is communication-surface representation
	// resolved from the owning domain — Chat never becomes Commerce authority.
	projections, projErr := h.resolveMessageProjections(ctx, senderID, []*chatEntity.ChatMessage{message})
	if projErr != nil {
		h.log.Error("Failed to resolve resource projections",
			zap.String("room_id", roomID.String()),
			zap.Error(projErr),
		)
		response.InternalServerError(c, "Failed to resolve resource projections")
		return
	}
	if proj, ok := projections[message.ID]; ok {
		resp["resource_projection"] = proj
	}

	if shippingProjections, shipErr := h.resolveShippingQuoteProjections(ctx, senderID, []*chatEntity.ChatMessage{message}); shipErr != nil {
		h.log.Error("Failed to resolve shipping quote projections",
			zap.String("room_id", roomID.String()),
			zap.Error(shipErr),
		)
		response.InternalServerError(c, "Failed to resolve shipping quote projections")
		return
	} else if p, ok := shippingProjections[message.ID]; ok {
		resp["shipping_quote_projection"] = shippingQuoteProjectionJSON(p)
	}

	response.Success(c, resp)
}

// MarkAsRead handles POST /api/v1/chat/rooms/:room_id/read
//
// Marks all messages in a room as read for the authenticated user.
func (h *Handler) MarkAsRead(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse room ID
	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		response.BadRequest(c, "Invalid room ID")
		return
	}

	// Parse request body
	var req MarkAsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339Nano, req.Timestamp)
	if err != nil {
		response.BadRequest(c, "Invalid timestamp format")
		return
	}

	// Mark as read
	err = h.chatService.MarkAsRead(ctx, roomID, userID, timestamp)
	if err != nil {
		if err == chatRepo.ErrParticipantMismatch {
			response.Forbidden(c, "You are not a participant in this room")
			return
		}
		if err == chatRepo.ErrRoomNotFound {
			response.NotFound(c, "Room not found")
			return
		}
		h.log.Error("Failed to mark as read",
			zap.String("room_id", roomID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to mark as read")
		return
	}

	response.SuccessWithMessage(c, "Marked as read", nil)
}

// GetUnreadCount handles GET /api/v1/chat/rooms/:room_id/unread
//
// Returns the unread message count for the authenticated user in a room.
func (h *Handler) GetUnreadCount(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse room ID
	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		response.BadRequest(c, "Invalid room ID")
		return
	}

	// Get unread count
	count, err := h.chatService.GetUnreadCount(ctx, roomID, userID)
	if err != nil {
		if err == chatRepo.ErrParticipantMismatch {
			response.Forbidden(c, "You are not a participant in this room")
			return
		}
		if err == chatRepo.ErrRoomNotFound {
			response.NotFound(c, "Room not found")
			return
		}
		h.log.Error("Failed to get unread count",
			zap.String("room_id", roomID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to get unread count")
		return
	}

	response.Success(c, gin.H{
		"room_id":      roomID.String(),
		"unread_count": count,
	})
}

// ========================================================================
// HELPERS
// ========================================================================

// roomToResponse converts a room entity to API response.
//
// participantCards may be nil; when non-nil, the room's other-participant
// UUID is looked up and emitted as a canonical ChatParticipantCard under
// the `other_user` key, alongside `other_user_id`.
func roomToResponse(
	room *chatEntity.ChatRoom,
	userID uuid.UUID,
	participantCards map[uuid.UUID]publiccard.UserCard,
) map[string]interface{} {
	otherID := room.OtherParticipant(userID)
	resp := map[string]interface{}{
		"id":              room.ID.String(),
		"room_type":       string(room.RoomType),
		"other_user_id":   otherID.String(),
		"created_at":      room.CreatedAt.Format(time.RFC3339),
		"updated_at":      room.UpdatedAt.Format(time.RFC3339),
		"last_message_at": room.LastMessageAt.Format(time.RFC3339),
	}

	// Canonical ChatParticipantCard (Phase 2A PublicCard landing). Emitted
	// when the caller pre-hydrated participant cards in a batch query.
	if participantCards != nil && otherID != uuid.Nil {
		if card, ok := participantCards[otherID]; ok {
			resp["other_user"] = card
		}
	}

	// Include linked_order_id if present (order↔chat commerce continuity)
	if room.HasLinkedOrder() && room.LinkedOrderID != nil {
		resp["linked_order_id"] = room.LinkedOrderID.String()
	}

	return resp
}

func roomListItemResponse(
	room *chatEntity.ChatRoom,
	userID uuid.UUID,
	participantCards map[uuid.UUID]publiccard.UserCard,
	lastMessage *chatEntity.ChatMessage,
	unreadCount int,
	projections map[uuid.UUID]*commerceshared.ResourceProjection,
	shippingProjections map[uuid.UUID]chatApp.ShippingQuoteProjection,
	mediaURLs []string,
) map[string]interface{} {
	resp := roomToResponse(room, userID, participantCards)
	if lastMessage != nil {
		last := messageToResponse(lastMessage, nil, nil, mediaURLs)
		if proj, ok := projections[lastMessage.ID]; ok {
			last["resource_projection"] = proj
		}
		if p, ok := shippingProjections[lastMessage.ID]; ok {
			last["shipping_quote_projection"] = shippingQuoteProjectionJSON(p)
		}
		resp["last_message"] = last
	} else {
		resp["last_message"] = nil
	}
	resp["unread_count"] = unreadCount
	return resp
}

func (h *Handler) batchLatestMessages(
	ctx context.Context,
	roomIDs []uuid.UUID,
) (map[uuid.UUID]*chatEntity.ChatMessage, error) {
	out := make(map[uuid.UUID]*chatEntity.ChatMessage, len(roomIDs))
	if len(roomIDs) == 0 {
		return out, nil
	}

	const q = `
		SELECT DISTINCT ON (room_id)
			id, room_id, sender_id, message_type, body, attachment_json,
			idempotency_key, created_at, deleted_at, deleted_by, deletion_reason
		FROM chat_messages
		WHERE room_id = ANY($1)
		ORDER BY room_id, created_at DESC, id DESC
	`

	rows, err := h.db.Pool().Query(ctx, q, roomIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			msg                 chatEntity.ChatMessage
			attachmentJSONBytes []byte
		)
		if err := rows.Scan(
			&msg.ID,
			&msg.RoomID,
			&msg.SenderID,
			&msg.MessageType,
			&msg.Body,
			&attachmentJSONBytes,
			&msg.IdempotencyKey,
			&msg.CreatedAt,
			&msg.DeletedAt,
			&msg.DeletedBy,
			&msg.DeletionReason,
		); err != nil {
			return nil, err
		}
		if attachmentJSONBytes != nil {
			if err := json.Unmarshal(attachmentJSONBytes, &msg.AttachmentJSON); err != nil {
				return nil, err
			}
		}
		m := msg
		out[msg.RoomID] = &m
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// messageToResponse converts a message entity to API response.
//
// senderCards may be nil; when non-nil, the message's sender_id is looked
// up and emitted as a canonical ChatParticipantCard under the `sender` key,
// alongside `sender_id`.
//
// sellerLifecycles may be nil; when non-nil, the attachment's referenced item
// ID is looked up and seller lifecycle fields are injected at the top level
// of attachment_json (seller_user_lifecycle, seller_trust_lifecycle). This
// enables mobile to show SellerInactiveBadge on embedded commerce cards and
// gate CTAs without a separate item fetch.
// ChatMediaUploadTTL is the lifetime of a chat media presigned PUT URL. The
// PENDING asset row outlives it (chatEntity.PendingAssetTTL), so a slow link
// never turns a registered upload into a lost one — the client can re-register.
const ChatMediaUploadTTL = 15 * time.Minute

// RegisterMediaRequest is the body for POST /api/v1/chat/rooms/:room_id/media.
type RegisterMediaRequest struct {
	// ContentType must be image/jpeg, image/png, image/webp, image/gif or
	// video/mp4 — the canonical chat media vocabulary.
	ContentType string `json:"content_type" binding:"required"`
	// ByteSize is the file size in bytes. The server enforces the per-type
	// ceiling (10 MB foto / 100 MB video), so a client that lies about its own
	// picker limits is rejected here.
	ByteSize int64 `json:"byte_size" binding:"required,min=1"`
}

// RegisterMediaResponse is the register step of register → upload → attach.
type RegisterMediaResponse struct {
	// AssetID is the id to send back in media_asset_ids on the message.
	AssetID string `json:"asset_id"`
	// StorageKey is the S3 object key the presigned PUT targets.
	StorageKey string `json:"storage_key"`
	// MediaType is image or video — the asset's canonical type.
	MediaType string `json:"media_type"`
	// UploadURL is the presigned PUT URL (expires per ChatMediaUploadTTL). The
	// PUT must carry a Content-Type matching content_type.
	UploadURL string `json:"upload_url"`
	// ReadURL is the canonical CDN read URL the object will be served at.
	ReadURL string `json:"read_url"`
	// ExpiresAt is the UTC expiry of the PENDING window: after this moment the
	// asset is swept and must be registered again.
	ExpiresAt time.Time `json:"expires_at"`
}

// RegisterMedia handles POST /api/v1/chat/rooms/:room_id/media.
//
// Step 1 of the canonical chat media pipeline:
//
//  1. POST here → {asset_id, storage_key, upload_url}
//  2. PUT the bytes to upload_url with a matching Content-Type
//  3. POST /chat/rooms/:room_id/messages with media_asset_ids: [asset_id, ...]
//
// The asset stays PENDING until a message references it. An upload that is
// never attached expires (24h) and is swept — that is what makes "pilih media
// lalu batal" cost nothing and leave no orphan authority behind.
//
// Returns:
//   - 200: {asset_id, storage_key, upload_url, read_url, expires_at}
//   - 400: invalid content_type or byte_size
//   - 403: caller is not a participant of the room
//   - 404: room not found
//   - 503: AWS not configured
func (h *Handler) RegisterMedia(c *gin.Context) {
	ctx := c.Request.Context()

	if h.mediaPresign.AccessKey == "" {
		h.log.Error("chat media register: AWS not configured")
		response.Error(c, 503, "UPLOAD_NOT_CONFIGURED", "Media upload service not configured")
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	uploaderID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		response.BadRequest(c, "Invalid room ID")
		return
	}

	var req RegisterMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	asset, svcErr := h.chatService.CreatePendingMediaAsset(ctx, roomID, uploaderID, req.ContentType, req.ByteSize)
	if svcErr != nil {
		switch {
		case errors.Is(svcErr, chatRepo.ErrParticipantMismatch):
			response.Forbidden(c, "You are not a participant in this room")
		case errors.Is(svcErr, chatRepo.ErrRoomNotFound):
			response.NotFound(c, "Room not found")
		case errors.Is(svcErr, chatRepo.ErrMediaContentTypeRejected):
			response.Error(c, 400, "INVALID_CONTENT_TYPE",
				"content_type must be one of "+strings.Join(chatEntity.AllowedChatMediaContentTypes(), ", "))
		case errors.Is(svcErr, chatRepo.ErrMediaSizeExceeded):
			response.Error(c, 400, "INVALID_FILE_SIZE",
				"byte_size must be within the limit fotos 10MB / video 100MB")
		default:
			h.log.Error("chat media register: failed to create pending asset",
				zap.String("room_id", roomID.String()),
				zap.String("uploader_id", uploaderID.String()),
				zap.Error(svcErr),
			)
			response.InternalServerError(c, "Failed to register media")
		}
		return
	}

	uploadURL, presignErr := s3presign.PresignPUT(h.mediaPresign, asset.StorageKey, asset.ContentType, ChatMediaUploadTTL)
	if presignErr != nil {
		h.log.Error("chat media register: presign failed",
			zap.String("asset_id", asset.ID.String()),
			zap.Error(presignErr),
		)
		response.InternalServerError(c, "Failed to generate upload URL")
		return
	}

	readURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		h.mediaPresign.Bucket, h.mediaPresign.Region, asset.StorageKey)
	if h.mediaCDNBaseURL != "" {
		readURL = h.mediaCDNBaseURL + "/" + asset.StorageKey
	}

	response.Success(c, RegisterMediaResponse{
		AssetID:    asset.ID.String(),
		StorageKey: asset.StorageKey,
		MediaType:  string(asset.MediaType),
		UploadURL:  uploadURL,
		ReadURL:    readURL,
		ExpiresAt:  asset.ExpiresAt,
	})
}

// hydrateMessageMedia batch-loads the media of a page of messages and maps each
// message id to its ordered, ready-to-render URLs.
//
// ONE query per page (no N+1) and ONE resolution authority: mediaresolve turns
// the stored storage_key into the CDN read URL (or a presigned GET when no CDN
// is configured), exactly like every other media surface.
//
// Media is a PROJECTION of a message, never a reason to fail a page: a hydration
// or resolution error degrades to "no media" and is logged.
func (h *Handler) hydrateMessageMedia(ctx context.Context, messages []*chatEntity.ChatMessage) map[uuid.UUID][]string {
	out := make(map[uuid.UUID][]string, len(messages))
	if len(messages) == 0 {
		return out
	}

	ids := make([]uuid.UUID, 0, len(messages))
	for _, msg := range messages {
		if msg != nil {
			ids = append(ids, msg.ID)
		}
	}
	if len(ids) == 0 {
		return out
	}

	mediaByMessage, err := h.chatService.ListMessageMedia(ctx, ids)
	if err != nil {
		h.log.Warn("chat: failed to hydrate message media", zap.Error(err))
		return out
	}

	for messageID, assets := range mediaByMessage {
		urls := make([]string, 0, len(assets))
		for _, asset := range assets {
			if asset == nil || asset.StorageKey == "" {
				continue
			}
			resolved, resolveErr := mediaresolve.ResolveMediaReadURL(asset.StorageKey)
			if resolveErr != nil {
				h.log.Warn("chat: failed to resolve media read url",
					zap.String("message_id", messageID.String()),
					zap.String("storage_key", asset.StorageKey),
					zap.Error(resolveErr),
				)
				continue
			}
			urls = append(urls, resolved)
		}
		if len(urls) > 0 {
			out[messageID] = urls
		}
	}

	return out
}

func messageToResponse(
	msg *chatEntity.ChatMessage,
	senderCards map[uuid.UUID]publiccard.UserCard,
	sellerLifecycles map[string]attachmentSellerLifecycle,
	mediaURLs []string,
) map[string]interface{} {
	resp := map[string]interface{}{
		"id":           msg.ID.String(),
		"room_id":      msg.RoomID.String(),
		"sender_id":    msg.SenderID.String(),
		"message_type": string(msg.MessageType),
		"created_at":   msg.CreatedAt.Format(time.RFC3339),
	}

	// Tombstone: hidden messages suppress body and attachment for regular users.
	// Timeline structure (id, room_id, sender_id, message_type, created_at) preserved.
	if msg.DeletedAt != nil {
		resp["is_hidden"] = true
		return resp
	}

	if msg.Body != nil {
		resp["body"] = *msg.Body
	}

	// Canonical chat media projection. Media is ORTHOGONAL to message_type — the
	// asset rows carry image|video, the message carries the ordered list — so a
	// photo/video message is "text + media_urls", and has_media mirrors the
	// chat_messages.has_media flag a room list reads for its preview.
	if len(mediaURLs) > 0 {
		resp["media_urls"] = mediaURLs
		resp["has_media"] = true
	}

	if msg.AttachmentJSON != nil {
		attachment := msg.AttachmentJSON

		// B1/B2: Enrich attachment with seller lifecycle when available.
		// Keep attachment_json canonical and emit lifecycle in attachment_metadata.
		if sellerLifecycles != nil {
			itemID := extractReferencedItemIDFromAttachment(msg.AttachmentJSON)
			if lc, ok := sellerLifecycles[itemID]; ok {
				resp["attachment_metadata"] = map[string]interface{}{
					"seller_user_lifecycle":  lc.userLifecycle,
					"seller_trust_lifecycle": lc.sellerTrustLifecycle,
				}
			}
		}

		resp["attachment_json"] = attachment
	}

	// Canonical ChatParticipantCard (Phase 2A). Emitted when the caller
	// pre-hydrated sender cards in a batch query.
	if senderCards != nil && msg.SenderID != uuid.Nil {
		if card, ok := senderCards[msg.SenderID]; ok {
			resp["sender"] = card
		}
	}

	return resp
}

// errResourceProjectionResolverNotConfigured is returned when a message carries
// a resource occurrence but no projection resolver has been wired. The HTTP
// layer fails closed (500) rather than silently omitting the projection.
var errResourceProjectionResolverNotConfigured = errors.New("resource projection resolver not configured")

// resolveMessageProjections batch-loads resource occurrences for the given
// messages and resolves viewer-aware resource projections via the canonical
// aggregate resolver.
//
// This is the SINGLE Chat-side path that turns a message's resource reference
// into a display projection. Chat never reads Commerce business truth directly
// here — the resolver delegates to the owning domain.
//
// Returns an empty (non-nil) map when no message in the set carries an
// occurrence.
func (h *Handler) resolveMessageProjections(
	ctx context.Context,
	viewerID uuid.UUID,
	messages []*chatEntity.ChatMessage,
) (map[uuid.UUID]*commerceshared.ResourceProjection, error) {
	out := map[uuid.UUID]*commerceshared.ResourceProjection{}
	if len(messages) == 0 {
		return out, nil
	}
	occurrences, err := h.getResourceOccurrencesByMessageIDs(ctx, messages)
	if err != nil {
		return nil, err
	}
	if len(occurrences) == 0 {
		return out, nil
	}
	if h.resourceProjectionResolver == nil {
		return nil, errResourceProjectionResolverNotConfigured
	}
	return h.resourceProjectionResolver.ResolveResourceProjections(ctx, viewerID, occurrences)
}

// errShippingQuoteProjectionResolverNotConfigured is returned when a message
// carries a shipping quote but no projection resolver has been wired. The HTTP
// layer fails closed (500) rather than silently omitting the projection.
var errShippingQuoteProjectionResolverNotConfigured = errors.New("shipping quote projection resolver not configured")

// resolveShippingQuoteProjections maps each shipping-quote message in the set
// to its viewer-scoped actionability projection by delegating to the Shipping
// commerce authority (one batch call). Chat never computes quote lifecycle.
//
// Returns an empty (non-nil) map when no message in the set carries a shipping
// quote.
func (h *Handler) resolveShippingQuoteProjections(
	ctx context.Context,
	viewerID uuid.UUID,
	messages []*chatEntity.ChatMessage,
) (map[uuid.UUID]chatApp.ShippingQuoteProjection, error) {
	out := map[uuid.UUID]chatApp.ShippingQuoteProjection{}
	if len(messages) == 0 {
		return out, nil
	}
	quoteIDByMessage := make(map[uuid.UUID]uuid.UUID)
	quoteIDs := make([]uuid.UUID, 0)
	seen := make(map[uuid.UUID]struct{})
	for _, msg := range messages {
		if msg.AttachmentJSON == nil {
			continue
		}
		if typ, _ := msg.AttachmentJSON["type"].(string); typ != "shipping_quote" {
			continue
		}
		data, _ := msg.AttachmentJSON["data"].(map[string]interface{})
		if data == nil {
			continue
		}
		offerIDStr, _ := data["offer_id"].(string)
		if offerIDStr == "" {
			continue
		}
		quoteID, err := uuid.Parse(offerIDStr)
		if err != nil {
			continue
		}
		quoteIDByMessage[msg.ID] = quoteID
		if _, ok := seen[quoteID]; !ok {
			seen[quoteID] = struct{}{}
			quoteIDs = append(quoteIDs, quoteID)
		}
	}
	if len(quoteIDs) == 0 {
		return out, nil
	}
	if h.shippingQuoteProjectionResolver == nil {
		return nil, errShippingQuoteProjectionResolverNotConfigured
	}
	resolved, err := h.shippingQuoteProjectionResolver.ResolveShippingQuoteProjections(ctx, viewerID, quoteIDs)
	if err != nil {
		return nil, err
	}
	for messageID, quoteID := range quoteIDByMessage {
		if p, ok := resolved[quoteID]; ok {
			out[messageID] = p
		}
	}
	return out, nil
}

// shippingQuoteProjectionJSON renders a projection in the canonical wire shape
// (snake_case, matching GET /shipping-quote/:id).
func shippingQuoteProjectionJSON(p chatApp.ShippingQuoteProjection) map[string]interface{} {
	return map[string]interface{}{
		"is_current":        p.IsCurrent,
		"viewer_actionable": p.ViewerActionable,
	}
}

// getResourceOccurrencesByMessageIDs batch-fetches resource occurrences for
// the given messages from chat_message_resource_occurrences. Returns a map
// of messageID → occurrence. An empty (non-nil) map means no messages in
// the page have occurrences.
func (h *Handler) getResourceOccurrencesByMessageIDs(
	ctx context.Context,
	messages []*chatEntity.ChatMessage,
) (map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, error) {
	out := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence)
	if len(messages) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(messages))
	for i, msg := range messages {
		ids[i] = msg.ID
	}
	rows, err := h.db.Pool().Query(ctx, `
		SELECT message_id, operation, profile_source_id, content_source_id,
		       for_sale_source_id, auction_source_id, fallback_snapshot, created_at
		FROM chat_message_resource_occurrences
		WHERE message_id = ANY($1)
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("query resource occurrences: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var occ chatEntity.ChatMessageResourceOccurrence
		var fallbackSnapshot []byte
		if err := rows.Scan(
			&occ.MessageID, &occ.Operation,
			&occ.ProfileSourceID, &occ.ContentSourceID,
			&occ.ForSaleSourceID, &occ.AuctionSourceID,
			&fallbackSnapshot, &occ.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan resource occurrence: %w", err)
		}
		if fallbackSnapshot != nil {
			occ.FallbackSnapshot = json.RawMessage(fallbackSnapshot)
		}
		out[occ.MessageID] = &occ
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resource occurrences: %w", err)
	}
	return out, nil
}

// hydrateRoomParticipants batch-loads ChatParticipantCards for the other
// participant of each room (relative to the calling userID). Single query
// via the chat-local lifecycle hydrator; no N+1. Returns an empty (non-nil)
// map on failure so the caller can degrade to the UUID-only shape
// without crashing the response.
//
// E4.2 — Lifecycle field carries the coarsened public lifecycle state for
// the chat participant identity (publiccard.UserCard.Lifecycle on the
// chat-participant seam), sourced from users.account_status +
// users.deleted_at and materialised via viewercontext.CoarsenLifecycle in
// Go (NEVER in SQL). Empty string when the row is missing → nil Lifecycle
// (rollback-safe). Non-empty values are constrained to the canonical
// public lifecycle vocabulary: "active", "unavailable", "removed". Chat
// preserves slot-persistence (no users.deleted_at IS NULL filter) so
// deleted/suspended participants still produce rows on the wire — only the
// Lifecycle field on the nested UserCard reflects the degradation.
func (h *Handler) hydrateRoomParticipants(
	ctx context.Context,
	rooms []*chatEntity.ChatRoom,
	userID uuid.UUID,
) map[uuid.UUID]publiccard.UserCard {
	if len(rooms) == 0 {
		return map[uuid.UUID]publiccard.UserCard{}
	}
	ids := make([]uuid.UUID, 0, len(rooms))
	seen := make(map[uuid.UUID]struct{}, len(rooms))
	for _, room := range rooms {
		other := room.OtherParticipant(userID)
		if other == uuid.Nil {
			continue
		}
		if _, ok := seen[other]; ok {
			continue
		}
		seen[other] = struct{}{}
		ids = append(ids, other)
	}
	if len(ids) == 0 {
		return map[uuid.UUID]publiccard.UserCard{}
	}
	cards, err := h.buildChatParticipantCardsWithLifecycle(ctx, ids)
	if err != nil {
		h.log.Warn("chat: participant hydration failed; degrading to bare UUID response",
			zap.Int("participant_count", len(ids)),
			zap.Error(err))
		return map[uuid.UUID]publiccard.UserCard{}
	}
	return cards
}

// hydrateMessageSenders batch-loads ChatParticipantCards for every distinct
// sender across the message list. Single query; no N+1. See
// hydrateRoomParticipants for the E4.2 lifecycle-emission contract — both
// hydrators share the same buildChatParticipantCardsWithLifecycle path.
func (h *Handler) hydrateMessageSenders(
	ctx context.Context,
	messages []*chatEntity.ChatMessage,
) map[uuid.UUID]publiccard.UserCard {
	if len(messages) == 0 {
		return map[uuid.UUID]publiccard.UserCard{}
	}
	ids := make([]uuid.UUID, 0, len(messages))
	seen := make(map[uuid.UUID]struct{}, len(messages))
	for _, msg := range messages {
		if msg.SenderID == uuid.Nil {
			continue
		}
		if _, ok := seen[msg.SenderID]; ok {
			continue
		}
		seen[msg.SenderID] = struct{}{}
		ids = append(ids, msg.SenderID)
	}
	if len(ids) == 0 {
		return map[uuid.UUID]publiccard.UserCard{}
	}
	cards, err := h.buildChatParticipantCardsWithLifecycle(ctx, ids)
	if err != nil {
		h.log.Warn("chat: message-sender hydration failed; degrading to bare UUID response",
			zap.Int("sender_count", len(ids)),
			zap.Error(err))
		return map[uuid.UUID]publiccard.UserCard{}
	}
	return cards
}

// buildChatParticipantCardsWithLifecycle is the chat-local lifecycle-aware
// hydrator for participant/sender cards.
//
// E4.2 — bounded chat-only activation (chat = fail-CLOSED on relationship
// overlay; lifecycle presence is mandatory on the participant card). Mirrors
// the comment-
// handler E3.2 recipe (single ANY($1) query + viewercontext.CoarsenLifecycle
// + publiccard.NewWithLifecycle) but deliberately omits the
// `users.deleted_at IS NULL` filter that comments uses — chat doctrine
// requires slot-persistence (deleted/suspended senders still produce rows
// so threads remain readable). The coarsener correctly emits "removed" for
// deleted authors; the WHERE clause is what makes that visible vs. hidden.
//
// Returns one entry per non-nil input id (no row → publiccard.Anonymous(id)
// per the publiccard.BuildMany contract). Errors propagate so the calling
// hydrator can apply its existing degradation strategy.
//
// IMPORTANT: This helper does NOT mutate shared publiccard / userdisplay
// plumbing; the public boundary (single canonical exposure authority) is
// preserved by passing the pre-coarsened lifecycle string into
// publiccard.NewWithLifecycle. Raw account_status enum strings never leave
// this function.
func (h *Handler) buildChatParticipantCardsWithLifecycle(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]publiccard.UserCard, error) {
	out := make(map[uuid.UUID]publiccard.UserCard, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	// SLOT-PERSISTENCE: no `u.deleted_at IS NULL` filter here. Deleted
	// participants must still surface in chat with Lifecycle="removed";
	// dropping the row would break thread continuity (chat-specific
	// slot-persistence carve-out).
	const query = `
		SELECT
			u.id,
			COALESCE(p.username, '') AS username,
			p.avatar_url,
			u.account_status,
			(u.deleted_at IS NOT NULL) AS is_deleted
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.id = ANY($1)
	`

	rows, err := h.db.Pool().Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("chat lifecycle hydration: query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			userID        uuid.UUID
			username      string
			avatarURL     *string
			accountStatus string
			isDeleted     bool
		)
		if err := rows.Scan(&userID, &username, &avatarURL, &accountStatus, &isDeleted); err != nil {
			return nil, fmt.Errorf("chat lifecycle hydration: scan failed: %w", err)
		}
		out[userID] = chatParticipantCardFromRow(userID, username, avatarURL, accountStatus, isDeleted)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat lifecycle hydration: rows iteration failed: %w", err)
	}

	// Anonymous-safe handling for input ids that didn't match a row
	// (hard-deleted, never-existed). Matches publiccard.BuildMany semantics:
	// every non-nil input id gets a card, never an absent map entry. The
	// Lifecycle field is left nil (rollback-safe; this signals "no truth"
	// rather than asserting active).
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := out[id]; ok {
			continue
		}
		out[id] = publiccard.Anonymous(id)
	}

	return out, nil
}

// chatParticipantCardFromRow is the pure, DB-free per-row builder that
// coarsens raw users.account_status + users.deleted_at into the canonical
// public lifecycle vocabulary and produces the wire-shape card.
//
// Extracted from buildChatParticipantCardsWithLifecycle so the lifecycle
// threading is unit-testable without a database — the coarsening rule, the
// vocabulary constraint, and the rollback-safe nil-when-empty contract of
// publiccard.NewWithLifecycle can all be exercised in pure Go.
func chatParticipantCardFromRow(
	userID uuid.UUID,
	username string,
	avatarURL *string,
	accountStatus string,
	isDeleted bool,
) publiccard.UserCard {
	// Avatar normalisation: treat empty string the same as nil so the
	// wire card never carries an empty avatar_url.
	var avatar *string
	if avatarURL != nil && *avatarURL != "" {
		v := *avatarURL
		avatar = &v
	}
	lifecycle := string(viewercontext.CoarsenLifecycle(accountStatus, isDeleted))
	return publiccard.NewWithLifecycle(userID, username, avatar, lifecycle)
}

// =========================================================================
// Attachment Seller Lifecycle Hydration (B1/B2)
// =========================================================================

// attachmentSellerLifecycle caches both lifecycle axes for an item's seller,
// resolved from fixed-price sales/auctions → users + seller_subscriptions.
type attachmentSellerLifecycle struct {
	userLifecycle        string // coarsened user-identity axis ("active"/"unavailable"/"removed")
	sellerTrustLifecycle string // coarsened seller-trust axis ("active"/"unavailable")
}

// extractReferencedItemIDFromAttachment returns the item ID referenced by
// the given attachment JSON, or "" if the attachment type is not commerce-related.
func extractReferencedItemIDFromAttachment(att map[string]interface{}) string {
	typ, _ := att["type"].(string)
	data, _ := att["data"].(map[string]interface{})
	if data == nil {
		return ""
	}
	switch typ {
	case "reference":
		targetType, _ := data["target_type"].(string)
		if targetType != "for_sale" && targetType != "auction" {
			return ""
		}
		s, _ := data["target_id"].(string)
		return s
	case "for_sale":
		s, _ := data["for_sale_id"].(string)
		return s
	case "auction":
		s, _ := data["auction_id"].(string)
		return s
	case "negotiation_proposal":
		s, _ := data["for_sale_id"].(string)
		return s
	case "shipping_quote":
		s, _ := data["linked_item_id"].(string)
		return s
	default:
		return ""
	}
}

// hydrateAttachmentSellerLifecycles batch-resolves seller lifecycle for items
// referenced in message attachments (fixed-price sales, auctions, negotiations,
// shipping quotes). Returns a map keyed by item ID string → lifecycles.
//
// Degrades gracefully: returns empty map on error so the caller emits the
// attachment shape without lifecycle fields. Single SQL with UNION across
// fixed-price sales + auctions; no N+1.
//
// This is the seller-trust parallel of hydrateMessageSenders for the
// user-identity axis on sender cards.
func (h *Handler) hydrateAttachmentSellerLifecycles(
	ctx context.Context,
	messages []*chatEntity.ChatMessage,
) map[string]attachmentSellerLifecycle {
	empty := map[string]attachmentSellerLifecycle{}
	if len(messages) == 0 {
		return empty
	}

	// 1. Extract item IDs from attachment JSON.
	forSaleIDs := make([]uuid.UUID, 0)
	auctionIDs := make([]uuid.UUID, 0)
	seen := make(map[string]struct{})

	for _, msg := range messages {
		if msg.AttachmentJSON == nil {
			continue
		}
		typ, _ := msg.AttachmentJSON["type"].(string)
		data, _ := msg.AttachmentJSON["data"].(map[string]interface{})
		if data == nil {
			continue
		}

		var itemIDStr string
		isAuction := false
		switch typ {
		case "reference":
			targetType, _ := data["target_type"].(string)
			if targetType != "for_sale" && targetType != "auction" {
				continue
			}
			itemIDStr, _ = data["target_id"].(string)
			isAuction = targetType == "auction"
		case "for_sale":
			itemIDStr, _ = data["for_sale_id"].(string)
		case "auction":
			itemIDStr, _ = data["auction_id"].(string)
			isAuction = true
		case "negotiation_proposal":
			itemIDStr, _ = data["for_sale_id"].(string)
		case "shipping_quote":
			itemIDStr, _ = data["linked_item_id"].(string)
		}

		if itemIDStr == "" {
			continue
		}
		if _, ok := seen[itemIDStr]; ok {
			continue
		}
		seen[itemIDStr] = struct{}{}

		id, err := uuid.Parse(itemIDStr)
		if err != nil {
			continue
		}
		if isAuction {
			auctionIDs = append(auctionIDs, id)
		} else {
			forSaleIDs = append(forSaleIDs, id)
		}
	}

	if len(forSaleIDs) == 0 && len(auctionIDs) == 0 {
		return empty
	}

	// 2. Batch query: resolve item → seller → lifecycle (both axes).
	const q = `
		WITH item_sellers AS (
			SELECT id::text AS item_id, seller_id FROM for_sales WHERE id = ANY($1)
			UNION ALL
			SELECT id::text AS item_id, seller_id FROM auctions WHERE id = ANY($2)
		)
		SELECT
			is2.item_id,
			COALESCE(u.account_status::text, '') AS account_status,
			(u.deleted_at IS NOT NULL)            AS is_deleted,
			COALESCE(ss.status::text, '')          AS subscription_status
		FROM item_sellers is2
		LEFT JOIN users u ON u.id = is2.seller_id
		LEFT JOIN LATERAL (
			SELECT status FROM seller_subscriptions
			WHERE user_id = is2.seller_id
			ORDER BY created_at DESC LIMIT 1
		) ss ON true
	`

	rows, err := h.db.Pool().Query(ctx, q, forSaleIDs, auctionIDs)
	if err != nil {
		h.log.Warn("chat: attachment seller lifecycle hydration failed",
			zap.Int("for_sale_count", len(forSaleIDs)),
			zap.Int("auction_count", len(auctionIDs)),
			zap.Error(err))
		return empty
	}
	defer rows.Close()

	result := make(map[string]attachmentSellerLifecycle, len(seen))
	for rows.Next() {
		var (
			itemID             string
			accountStatus      string
			isDeleted          bool
			subscriptionStatus string
		)
		if err := rows.Scan(&itemID, &accountStatus, &isDeleted, &subscriptionStatus); err != nil {
			h.log.Warn("chat: attachment seller lifecycle scan failed", zap.Error(err))
			return empty
		}
		result[itemID] = attachmentSellerLifecycle{
			userLifecycle:        string(viewercontext.CoarsenLifecycle(accountStatus, isDeleted)),
			sellerTrustLifecycle: string(viewercontext.CoarsenSellerTrust(subscriptionStatus)),
		}
	}
	if err := rows.Err(); err != nil {
		h.log.Warn("chat: attachment seller lifecycle rows iteration failed", zap.Error(err))
		return empty
	}

	return result
}

// ========================================================================
// NEGOTIATION ENDPOINTS (Chat-Owned)
// ========================================================================

// StartNegotiationRequest holds the request body for starting a negotiation.
type StartNegotiationRequest struct {
	ForSaleID string `json:"for_sale_id" binding:"required,uuid"`
	Price     int64  `json:"price" binding:"required,min=1"`
	Note      string `json:"note,omitempty"`
}

// CounterOfferRequest holds the request body for sending a counter offer.
type CounterOfferRequest struct {
	SessionID string `json:"session_id" binding:"required,uuid"`
	Price     int64  `json:"price" binding:"required,min=1"`
	Note      string `json:"note,omitempty"`
}

// RespondNegotiationRequest holds the request body for responding to a negotiation.
type RespondNegotiationRequest struct {
	SessionID string `json:"session_id" binding:"required,uuid"`
	Action    string `json:"action" binding:"required,oneof=accept cancel"`
}

// sessionToResponse converts a NegotiationSession to a JSON-friendly response
// map. viewerCanAct is the canonical Commerce actionability projection for the
// requesting viewer (see NegotiationService.ViewerCanAct) — conversation
// surfaces render it verbatim and must never reconstruct turn themselves.
func sessionToResponse(s *negotiationEntity.NegotiationSession, viewerCanAct bool) gin.H {
	resp := gin.H{
		"id":                s.ID.String(),
		"resource_type":     string(s.ResourceType),
		"buyer_id":          s.BuyerID.String(),
		"seller_id":         s.SellerID.String(),
		"status":            string(s.Status),
		"proposal_sequence": s.ProposalSequence,
		"viewer_can_act":    viewerCanAct,
		"is_expired":        s.IsExpired(),
		"created_at":        s.CreatedAt.Format(time.RFC3339),
		"updated_at":        s.UpdatedAt.Format(time.RFC3339),
	}
	if s.ForSaleID != uuid.Nil {
		resp["for_sale_id"] = s.ForSaleID.String()
	}
	if s.ChatRoomID != nil {
		resp["chat_room_id"] = s.ChatRoomID.String()
	}
	if s.CurrentPrice != nil {
		resp["current_price"] = *s.CurrentPrice
	}
	if s.AcceptedPrice != nil {
		resp["accepted_price"] = *s.AcceptedPrice
	}
	if s.ExpiresAt != nil {
		resp["expires_at"] = s.ExpiresAt.Format(time.RFC3339)
	}
	if s.AcceptedAt != nil {
		resp["accepted_at"] = s.AcceptedAt.Format(time.RFC3339)
	}
	if s.OrderID != nil {
		resp["order_id"] = s.OrderID.String()
	}
	return resp
}

// viewerCanActProjection asks the Commerce negotiation authority whether the
// given viewer may act, and fail-closes on error. The chat layer only carries
// the returned fact into the response — it never computes turn itself.
func (h *Handler) viewerCanActProjection(
	ctx context.Context,
	session *negotiationEntity.NegotiationSession,
	viewerID uuid.UUID,
) bool {
	canAct, err := h.negotiationService.ViewerCanAct(ctx, session, viewerID)
	if err != nil {
		h.log.Error("Failed to evaluate negotiation actionability",
			zap.String("session_id", session.ID.String()),
			zap.String("viewer_id", viewerID.String()),
			zap.Error(err),
		)
		return false
	}
	return canAct
}

// StartNegotiation handles POST /api/v1/chat/rooms/:room_id/negotiate
//
// Starts a price negotiation in a chat room. The authenticated user is the buyer;
// the other room participant is the seller. All business logic is delegated to
// NegotiationService — the chat handler only provides room membership gating.
func (h *Handler) StartNegotiation(c *gin.Context) {
	ctx := c.Request.Context()

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

	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		response.BadRequest(c, "Invalid room ID")
		return
	}

	var req StartNegotiationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Room membership check
	room, err := h.chatService.GetRoom(ctx, roomID)
	if err != nil {
		if err == chatRepo.ErrRoomNotFound {
			response.NotFound(c, "Room not found")
			return
		}
		h.log.Error("Failed to get room", zap.String("room_id", roomID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve room")
		return
	}
	if !room.HasParticipant(userID) {
		response.Forbidden(c, "You are not a participant in this room")
		return
	}

	// Account status enforcement
	if h.statusChecker != nil {
		if err := h.statusChecker.EnsureActive(ctx, userID); err != nil {
			response.RespondWithError(c, h.log, err)
			return
		}
	}

	forSaleID, err := uuid.Parse(req.ForSaleID)
	if err != nil {
		response.BadRequest(c, "Invalid fixed-price sale ID")
		return
	}

	// Delegate to NegotiationService — all business logic lives there.
	// RoomID + RoomOtherParticipantID let the service verify the room's
	// counterparty is exactly the resolved seller (PASS_7B / F2) and persist
	// chat_room_id on the session at creation time (PASS_7B / F1) — this room
	// is also what GetNegotiation will later look it up by.
	session, err := h.negotiationService.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType:           negotiationEntity.NegotiationResourceForSale,
		ForSaleID:              forSaleID,
		BuyerID:                userID,
		InitialPrice:           req.Price,
		Note:                   req.Note,
		RoomID:                 roomID,
		RoomOtherParticipantID: room.OtherParticipant(userID),
	})
	if err != nil {
		var roomMismatchErr *negotiationApp.ErrNegotiationRoomMismatch
		if errors.As(err, &roomMismatchErr) {
			response.Error(c, 403, "NEGOTIATION_ROOM_MISMATCH", "This chat room's other participant is not the seller of this for_sale item")
			return
		}
		h.log.Error("Failed to start negotiation",
			zap.String("room_id", roomID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.RespondWithError(c, h.log, err)
		return
	}

	response.Success(c, sessionToResponse(session, h.viewerCanActProjection(ctx, session, userID)))
}

// SendCounterOffer handles POST /api/v1/chat/rooms/:room_id/counter
//
// Sends a counter-offer in an active negotiation. Either buyer or seller can counter.
func (h *Handler) SendCounterOffer(c *gin.Context) {
	ctx := c.Request.Context()

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

	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		response.BadRequest(c, "Invalid room ID")
		return
	}

	var req CounterOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Room membership check
	room, err := h.chatService.GetRoom(ctx, roomID)
	if err != nil {
		if err == chatRepo.ErrRoomNotFound {
			response.NotFound(c, "Room not found")
			return
		}
		h.log.Error("Failed to get room", zap.String("room_id", roomID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve room")
		return
	}
	if !room.HasParticipant(userID) {
		response.Forbidden(c, "You are not a participant in this room")
		return
	}

	// Account status enforcement
	if h.statusChecker != nil {
		if err := h.statusChecker.EnsureActive(ctx, userID); err != nil {
			response.RespondWithError(c, h.log, err)
			return
		}
	}

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		response.BadRequest(c, "Invalid session ID")
		return
	}

	session, err := h.negotiationService.SendCounterOffer(ctx, negotiationApp.SendCounterOfferRequest{
		SessionID: sessionID,
		SenderID:  userID,
		Price:     req.Price,
		Note:      req.Note,
	})
	if err != nil {
		// TURN: alternating counter is a business rule — surface it as its own
		// contract code instead of a generic error mapping.
		var notYourTurn *negotiationApp.ErrNotYourTurn
		if errors.As(err, &notYourTurn) {
			response.Error(c, 409, "NEGOTIATION_NOT_YOUR_TURN", "Bukan giliran Anda untuk membalas penawaran ini")
			return
		}
		h.log.Error("Failed to send counter offer",
			zap.String("room_id", roomID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.RespondWithError(c, h.log, err)
		return
	}

	// COUNTER RESPONSE CONTRACT: same sessionToResponse envelope as start/
	// respond/get. The previous {"message":"Counter offer sent"} body made the
	// mobile DTO read data.id as null → "Failed to counter offer" although the
	// counter had committed — the client then kept its stale turn state.
	response.Success(c, sessionToResponse(session, h.viewerCanActProjection(ctx, session, userID)))
}

// RespondToNegotiation handles POST /api/v1/chat/rooms/:room_id/respond
//
// Accepts or cancels a negotiation. Either PARTICIPANT may accept or cancel
// (owner truth: Terima and Tolak exist on both sides).
// Suspended users CAN cancel (cleanup exemption).
func (h *Handler) RespondToNegotiation(c *gin.Context) {
	ctx := c.Request.Context()

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

	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		response.BadRequest(c, "Invalid room ID")
		return
	}

	var req RespondNegotiationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Room membership check
	room, err := h.chatService.GetRoom(ctx, roomID)
	if err != nil {
		if err == chatRepo.ErrRoomNotFound {
			response.NotFound(c, "Room not found")
			return
		}
		h.log.Error("Failed to get room", zap.String("room_id", roomID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve room")
		return
	}
	if !room.HasParticipant(userID) {
		response.Forbidden(c, "You are not a participant in this room")
		return
	}

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		response.BadRequest(c, "Invalid session ID")
		return
	}

	switch req.Action {
	case "accept":
		// Account status enforcement for accept (seller must be active)
		if h.statusChecker != nil {
			if err := h.statusChecker.EnsureActive(ctx, userID); err != nil {
				response.RespondWithError(c, h.log, err)
				return
			}
		}

		session, err := h.negotiationService.AcceptNegotiation(ctx, negotiationApp.AcceptNegotiationRequest{
			SessionID: sessionID,
			ActorID:   userID,
		})
		if err != nil {
			h.log.Error("Failed to accept negotiation",
				zap.String("room_id", roomID.String()),
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
			response.RespondWithError(c, h.log, err)
			return
		}
		response.Success(c, sessionToResponse(session, h.viewerCanActProjection(ctx, session, userID)))

	case "cancel":
		// No EnsureActive for cancel — suspended users can cancel (cleanup exemption)
		err := h.negotiationService.CancelNegotiation(ctx, negotiationApp.CancelNegotiationRequest{
			SessionID: sessionID,
			ActorID:   userID,
		})
		if err != nil {
			h.log.Error("Failed to cancel negotiation",
				zap.String("room_id", roomID.String()),
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
			response.RespondWithError(c, h.log, err)
			return
		}
		response.Success(c, gin.H{"message": "Negotiation cancelled"})

	default:
		response.BadRequest(c, "Invalid action: must be 'accept' or 'cancel'")
	}
}

// GetNegotiation handles GET /api/v1/chat/rooms/:room_id/negotiation
//
// Returns the latest negotiation session for a chat room, regardless of status.
func (h *Handler) GetNegotiation(c *gin.Context) {
	ctx := c.Request.Context()

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

	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		response.BadRequest(c, "Invalid room ID")
		return
	}

	// Room membership check
	room, err := h.chatService.GetRoom(ctx, roomID)
	if err != nil {
		if err == chatRepo.ErrRoomNotFound {
			response.NotFound(c, "Room not found")
			return
		}
		h.log.Error("Failed to get room", zap.String("room_id", roomID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve room")
		return
	}
	if !room.HasParticipant(userID) {
		response.Forbidden(c, "You are not a participant in this room")
		return
	}

	// Read-only: use nil tx (no transaction needed)
	var session *negotiationEntity.NegotiationSession
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		session, txErr = h.negotiationRepo.GetLatestSessionByChatRoomID(ctx, tx, roomID)
		return txErr
	})
	if err != nil {
		h.log.Error("Failed to get negotiation",
			zap.String("room_id", roomID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve negotiation")
		return
	}

	if session == nil {
		response.NotFound(c, "No negotiation found for this room")
		return
	}

	response.Success(c, sessionToResponse(session, h.viewerCanActProjection(ctx, session, userID)))
}
