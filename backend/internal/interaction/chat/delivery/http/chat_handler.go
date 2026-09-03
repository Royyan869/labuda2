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
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	negotiationApp "github.com/labuda/backend/internal/commerce/negotiation/application"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationImpl "github.com/labuda/backend/internal/commerce/negotiation/infrastructure/repository"
	negotiationRepo "github.com/labuda/backend/internal/commerce/negotiation/repository"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderRepoImpl "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	"github.com/labuda/backend/internal/governance/viewercontext"
	addressentity "github.com/labuda/backend/internal/identity/address/entity"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/internal/pkg/blockcheck"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/response"
	pricingtokenapp "github.com/labuda/backend/internal/pricing/token/application"
	pricingtokenentity "github.com/labuda/backend/internal/pricing/token/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// handlerAccountStatusChecker is the handler-layer interface for account status enforcement.
// Defined locally to avoid importing the auth package from this delivery layer.
type handlerAccountStatusChecker interface {
	EnsureActive(ctx context.Context, userID uuid.UUID) error
}

// Handler handles HTTP requests for chat operations.
type Handler struct {
	chatService                *chatApp.Service
	orderService               *orderApp.OrderService
	negotiationRepo            negotiationRepo.Repository
	negotiationService         *negotiationApp.NegotiationService
	pricingTokenService        *pricingtokenapp.PricingTokenService
	statusChecker              handlerAccountStatusChecker // Account status enforcement
	db                         *db.DB
	log                        *zap.Logger
	resourceProjectionResolver chatApp.ResourceProjectionResolver
}

// NewHandler creates a new chat handler.
func NewHandler(
	chatService *chatApp.Service,
	orderService *orderApp.OrderService,
	negotiationService *negotiationApp.NegotiationService,
	pricingTokenService *pricingtokenapp.PricingTokenService,
	statusChecker handlerAccountStatusChecker,
	database *db.DB,
	log *zap.Logger,
) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{
		chatService:         chatService,
		orderService:        orderService,
		negotiationRepo:     negotiationImpl.NewNegotiationRepository(),
		negotiationService:  negotiationService,
		pricingTokenService: pricingTokenService,
		statusChecker:       statusChecker,
		db:                  database,
		log:                 log,
	}
}

// SetResourceProjectionResolver injects the canonical resource projection
// resolver used by ListMessages (and eventually SendMessage) to hydrate
// resource_projection blocks on messages that carry a resource occurrence.
// Call once during handler wiring; nil means no projection hydration.
func (h *Handler) SetResourceProjectionResolver(r chatApp.ResourceProjectionResolver) {
	h.resourceProjectionResolver = r
}

// ========================================================================
// REQUEST DTOs
// ========================================================================

// SendMessageRequest holds the request body for sending a message.
type SendMessageRequest struct {
	MessageType    string                 `json:"message_type" binding:"required,oneof=text negotiation_proposal system"`
	Body           string                 `json:"body"`
	AttachmentJSON map[string]interface{} `json:"attachment_json"`
	IdempotencyKey string                 `json:"idempotency_key" binding:"required"`
}

// MarkAsReadRequest holds the request body for marking messages as read.
type MarkAsReadRequest struct {
	Timestamp string `json:"timestamp" binding:"required"`
}

// CreateOrderFromChatRequest holds the request body for creating an order from a chat room.
type CreateOrderFromChatRequest struct {
	// Shipping destination (required)
	AddressID string `json:"address_id" binding:"required,uuid"`

	// Shipping method (one of these is required)
	ShippingQuoteID *string `json:"shipping_quote_id,omitempty"`
	ShippingSetupID *string `json:"shipping_option_id,omitempty"`

	// Pricing token (required for anti-tamper)
	// CRITICAL: All pricing data comes from the validated token
	PricingToken string `json:"pricing_token" binding:"required"`

	// Quantity (optional, defaults to 1)
	Quantity int `json:"quantity" binding:"omitempty,min=1,max=100"`
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

	unreadCountByRoom, unreadErr := h.batchUnreadCounts(ctx, roomIDs, userID)
	if unreadErr != nil {
		h.log.Warn("chat: list-rooms failed to batch load unread counts",
			zap.String("user_id", userID.String()),
			zap.Error(unreadErr),
		)
		unreadCountByRoom = map[uuid.UUID]int{}
	}

	// Convert to response
	data := make([]map[string]interface{}, len(rooms))
	for i, room := range rooms {
		latestMessage := latestMessageByRoom[room.ID]
		unreadCount := unreadCountByRoom[room.ID]

		data[i] = roomListItemResponse(room, userID, participantCards, latestMessage, unreadCount)
	}

	response.Success(c, gin.H{
		"data": data,
	})
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

	// Convert messages to response
	messageData := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		messageData[i] = messageToResponse(msg, senderCards, sellerLifecycles)
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

// CreateOrderFromChat handles POST /api/v1/chat/rooms/:room_id/order
//
// CHAT-CENTRIC ORDER CREATION:
// Creates an order from an accepted negotiation in the chat room.
// This makes chat a TRUE commerce entry point - no need to leave chat context.
//
// Behavior:
// 1. Find active negotiation by chat_room_id
// 2. Validate negotiation.status = accepted AND not already converted to order
// 3. Extract for_sale_id and accepted_price from NegotiationSession
// 4. Create order with source_type = "negotiation"
// 5. Link order ↔ chat ↔ negotiation
//
// CRITICAL RULE:
// - ❌ DO NOT trust chat message data
// - ✅ ALWAYS use NegotiationSession for authoritative pricing
func (h *Handler) CreateOrderFromChat(c *gin.Context) {
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
	var req CreateOrderFromChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Set default quantity
	quantity := req.Quantity
	if quantity == 0 {
		quantity = 1
	}

	// STEP 1: Verify user is a participant in the room
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

	if !room.HasParticipant(userID) {
		response.Forbidden(c, "You are not a participant in this room")
		return
	}

	// Account status enforcement: buyer must be active before creating binding commerce records.
	if h.statusChecker != nil {
		if err := h.statusChecker.EnsureActive(ctx, userID); err != nil {
			response.RespondWithError(c, h.log, err)
			return
		}
	}

	// STEP 2: Parse and validate UUIDs
	addressID, err := uuid.Parse(req.AddressID)
	if err != nil {
		response.BadRequest(c, "Invalid address ID")
		return
	}

	var shippingSetupID uuid.UUID
	if req.ShippingSetupID != nil && *req.ShippingSetupID != "" {
		shippingSetupID, err = uuid.Parse(*req.ShippingSetupID)
		if err != nil {
			response.BadRequest(c, "Invalid shipping option ID")
			return
		}
	}

	// STEP 3: QUANTITY VALIDATION RULES
	// CRITICAL: Negotiation is for 1 unit only (prevents abuse)
	if quantity != 1 {
		response.BadRequest(c, "Negotiation orders must have quantity = 1. The negotiated price is for a single unit.")
		return
	}

	// STEP 4: SINGLE ATOMIC TRANSACTION FOR ENTIRE FLOW
	// CRITICAL: All operations MUST happen in ONE transaction to prevent race conditions
	// This includes: lock negotiation, validate, create order, update negotiation, link chat
	var (
		order         *orderentity.Order
		negotiationID uuid.UUID
		acceptedPrice int64
		tokenID       uuid.UUID
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// STEP 4.1: LOCK negotiation FIRST with FOR UPDATE (mandatory order)
		session, err := h.negotiationRepo.GetAcceptedSessionByChatRoomIDForUpdate(ctx, tx, roomID)
		if err != nil {
			return err
		}
		if session == nil {
			return &ErrNoAcceptedNegotiation{RoomID: roomID}
		}
		negotiation := session

		// Store for response after commit
		negotiationID = negotiation.ID
		if negotiation.AcceptedPrice != nil {
			acceptedPrice = *negotiation.AcceptedPrice
		}

		// STEP 4.2: VALIDATION (inside TX only - no validation outside!)
		// All validation MUST happen inside the transaction while lock is held

		// Check status = accepted (already enforced by GetAcceptedSessionByChatRoomIDForUpdate)

		// Check order_id IS NULL
		if negotiation.OrderID != nil && *negotiation.OrderID != uuid.Nil {
			return &ErrNegotiationAlreadySettled{NegotiationID: negotiation.ID, OrderID: *negotiation.OrderID}
		}

		// Check expiry
		if negotiation.IsExpired() {
			return &ErrNegotiationExpired{NegotiationID: negotiation.ID}
		}

		// Check user is buyer
		if !negotiation.IsBuyer(userID) {
			return &ErrUnauthorizedBuyer{UserID: userID, NegotiationID: negotiation.ID}
		}

		// Validate accepted_price exists
		if negotiation.AcceptedPrice == nil || *negotiation.AcceptedPrice <= 0 {
			return &ErrInvalidNegotiationData{Field: "accepted_price", NegotiationID: negotiation.ID}
		}

		if negotiation.ForSaleID == uuid.Nil {
			return &ErrInvalidNegotiationData{Field: "for_sale_id", NegotiationID: negotiation.ID}
		}

		forSale, err := h.orderService.GetCreationService().GetForSaleByID(ctx, tx, negotiation.ForSaleID)
		if err != nil {
			return fmt.Errorf("failed to load fixed-price sale for negotiation checkout: %w", err)
		}
		if forSale == nil {
			return &ErrInvalidNegotiationData{Field: "for_sale_id", NegotiationID: negotiation.ID}
		}

		// STEP 4.3: VALIDATE PRICING TOKEN (inside TX, under lock)
		// CRITICAL: All pricing data comes from the validated token
		// The token must have been generated for this negotiation with the negotiated price
		parsedTokenID, parseErr := uuid.Parse(req.PricingToken)
		if parseErr != nil {
			return fmt.Errorf("invalid pricing token format: %w", parseErr)
		}
		tokenID = parsedTokenID

		validatedToken, err := h.pricingTokenService.ValidateForOrderLocked(
			ctx,
			tx,
			tokenID,
			userID,
			forSale.ProductID,
			"negotiation",
			forSale.ID,
			0, // Quantity from token, not request
			addressID,
			shippingSetupID,
		)
		if err != nil {
			return fmt.Errorf("pricing token validation failed: %w", err)
		}

		// STEP 4.4: BUILD PRICING SNAPSHOT FROM TOKEN
		pricingSnapshot := buildPricingSnapshotFromToken(validatedToken)

		// STEP 4.5: CREATE ORDER (still inside TX, still under lock)
		// CRITICAL: Quantity comes from token, not from request
		input := buildNegotiationCheckoutInput(
			negotiation,
			forSale,
			userID,
			addressID,
			shippingSetupID,
			pricingSnapshot,
			&tokenID,
			validatedToken.Quantity,
		)

		createdOrder, createErr := h.orderService.GetCreationService().CreateFromSaleSurface(ctx, tx, input)
		if createErr != nil {
			return createErr
		}

		if err := h.pricingTokenService.FinalizeOrderConsumption(ctx, tx, validatedToken, createdOrder.ID); err != nil {
			return fmt.Errorf("pricing token consume failed: %w", err)
		}

		// STEP 4.4: UPDATE NEGOTIATION (CRITICAL - still inside TX)
		// Set negotiation.order_id = order.ID BEFORE commit
		// This prevents double-order race condition
		updateErr := h.negotiationRepo.UpdateOrderID(ctx, tx, negotiation.ID, createdOrder.ID)
		if updateErr != nil {
			return updateErr
		}

		// STEP 4.5: LINK CHAT (still same TX)
		_, linkErr := h.chatService.LinkOrderToChat(ctx, roomID, createdOrder.ID, userID)
		if linkErr != nil {
			// Log warning but don't fail - link is non-critical
			// Order is already created and negotiation is updated
			h.log.Warn("Failed to link order to chat (non-critical)",
				zap.String("room_id", roomID.String()),
				zap.String("order_id", createdOrder.ID.String()),
				zap.Error(linkErr),
			)
		}

		order = createdOrder
		return nil
	})

	// STEP 5: HANDLE ERRORS with proper HTTP status codes
	if err != nil {
		var tokenValidationErr *pricingtokenentity.ValidationError
		if errors.As(err, &tokenValidationErr) && tokenValidationErr.Code == pricingtokenentity.CodeTokenAlreadyUsed {
			if tokenValidationErr.OrderID == nil || *tokenValidationErr.OrderID == uuid.Nil {
				response.Error(c, 409, "PRICING_TOKEN_ALREADY_USED", "Pricing token already used but not linked to an order")
				return
			}

			existingOrder, fetchErr := recoverChatOrderFromUsedPricingToken(ctx, h.db, tokenID, *tokenValidationErr.OrderID)
			if fetchErr == nil && existingOrder != nil {
				response.Success(c, gin.H{
					"order_id":       existingOrder.ID.String(),
					"order_number":   existingOrder.OrderNumber,
					"room_id":        roomID.String(),
					"negotiation_id": negotiationID.String(),
					"unit_price":     existingOrder.UnitPrice,
					"quantity":       existingOrder.Quantity,
					"source_type":    "negotiation",
					"message":        "Order already exists (idempotent response)",
					"idempotent":     true,
				})
				return
			}

			h.log.Warn("Used pricing token detected but chat recovery fetch failed",
				zap.String("pricing_token_id", tokenID.String()),
				zap.String("order_id", tokenValidationErr.OrderID.String()),
				zap.String("user_id", userID.String()),
				zap.Error(fetchErr),
			)
			response.Error(c, 409, "PRICING_TOKEN_ALREADY_USED", "Pricing token already used for an existing order")
			return
		}

		if errors.Is(err, orderrepository.ErrDuplicatePricingToken) {
			var existingOrder *orderentity.Order
			fetchErr := h.db.WithTx(ctx, func(tx db.Tx) error {
				var lookupErr error
				existingOrder, lookupErr = orderRepoImpl.NewOrderRepository().GetByPricingTokenID(ctx, tx, tokenID)
				return lookupErr
			})
			if fetchErr == nil && existingOrder != nil {
				response.Success(c, gin.H{
					"order_id":       existingOrder.ID.String(),
					"order_number":   existingOrder.OrderNumber,
					"room_id":        roomID.String(),
					"negotiation_id": negotiationID.String(),
					"unit_price":     existingOrder.UnitPrice,
					"quantity":       existingOrder.Quantity,
					"source_type":    "negotiation",
					"message":        "Order already exists (idempotent response)",
					"idempotent":     true,
				})
				return
			}
		}

		if errors.Is(err, orderrepository.ErrDuplicateIdempotencyKey) {
			h.log.Info("Duplicate buyer idempotency key on chat order create",
				zap.String("user_id", userID.String()),
				zap.String("negotiation_id", negotiationID.String()),
			)
			response.Error(c, 409, "DUPLICATE_IDEMPOTENCY_KEY", "Idempotency key already used by a different request")
			return
		}

		switch e := err.(type) {
		case *ErrNoAcceptedNegotiation:
			response.BadRequest(c, "No accepted negotiation found in this chat room. Please complete a negotiation first.")
			return
		case *ErrNegotiationAlreadySettled:
			// IDEMPOTENCY UX: Instead of returning 409, fetch and return existing order
			// This provides better UX - user gets their order instead of error
			//
			// Use a separate read-only transaction to fetch the complete order
			var existingOrder *orderentity.Order
			fetchErr := h.db.WithTx(ctx, func(tx db.Tx) error {
				var err error
				existingOrder, err = h.orderService.GetOrder(ctx, tx, e.OrderID)
				return err
			})

			if fetchErr != nil {
				h.log.Warn("Order already exists but fetch failed",
					zap.String("order_id", e.OrderID.String()),
					zap.String("negotiation_id", e.NegotiationID.String()),
					zap.Error(fetchErr),
				)
				// Fallback to 409 if we can't fetch the order
				response.ErrorWithDetails(c, 409, "NEGOTIATION_ALREADY_SETTLED", "This negotiation has already been converted to an order", gin.H{
					"order_id":       e.OrderID.String(),
					"negotiation_id": e.NegotiationID.String(),
				})
				return
			}

			// Return existing order (idempotent response)
			response.Success(c, gin.H{
				"order_id":       existingOrder.ID.String(),
				"order_number":   existingOrder.OrderNumber,
				"room_id":        roomID.String(),
				"negotiation_id": e.NegotiationID.String(),
				"unit_price":     existingOrder.UnitPrice,
				"quantity":       existingOrder.Quantity,
				"source_type":    "negotiation",
				"message":        "Order already exists (idempotent response)",
				"idempotent":     true,
			})
			return
		case *ErrNegotiationExpired:
			response.BadRequest(c, "This negotiation has expired. Please start a new negotiation.")
			return
		case *ErrUnauthorizedBuyer:
			response.Forbidden(c, "Only the buyer can create an order from a negotiation")
			return
		case *ErrInvalidNegotiationData:
			h.log.Error("Invalid negotiation data",
				zap.String("field", e.Field),
				zap.String("negotiation_id", e.NegotiationID.String()),
				zap.Error(err),
			)
			response.InternalServerError(c, "Negotiation data is invalid")
			return
		default:
			// Phase 0 honesty: surface typed shipping gate errors from the
			// chat-driven order creation path with machine-readable codes.
			if errors.Is(err, shippingApp.ErrNoShippingSetups) {
				response.Error(c, 400, "NO_SHIPPING_OPTIONS",
					"Penjual belum mengatur pengiriman untuk produk ini.")
				return
			}
			if errors.Is(err, shippingApp.ErrShippingSetupUnavailable) {
				response.Error(c, 400, "SHIPPING_OPTION_UNAVAILABLE",
					"Produk ini di luar area pengiriman untuk alamat Anda.")
				return
			}

			// Check for PostgreSQL unique violation (concurrent checkout)
			// With unique constraint in place, this should be rare
			if isUniqueViolationError(err) {
				// Try to extract order_id from error message (PostgreSQL format)
				// If successful, fetch and return existing order (idempotency)
				response.ErrorWithDetails(c, 409, "CONCURRENT_CHECKOUT", "Another checkout attempt is in progress or already completed", gin.H{
					"error": "concurrent_checkout",
				})
				return
			}

			h.log.Error("Failed to create order from negotiation",
				zap.String("room_id", roomID.String()),
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to create order: "+err.Error())
			return
		}
	}

	// STEP 6: Return success response
	response.Success(c, gin.H{
		"order_id":       order.ID.String(),
		"order_number":   order.OrderNumber,
		"room_id":        roomID.String(),
		"negotiation_id": negotiationID.String(),
		"unit_price":     acceptedPrice,
		"quantity":       quantity, // Always 1 for negotiation orders
		"source_type":    "negotiation",
		"message":        "Order created successfully from chat negotiation",
	})
}

func recoverChatOrderFromUsedPricingToken(
	ctx context.Context,
	transactor interface {
		WithTx(context.Context, func(tx db.Tx) error) error
	},
	tokenID uuid.UUID,
	orderID uuid.UUID,
) (*orderentity.Order, error) {
	repo := orderRepoImpl.NewOrderRepository()
	var recoveredOrder *orderentity.Order
	err := transactor.WithTx(ctx, func(tx db.Tx) error {
		var lookupErr error
		recoveredOrder, lookupErr = repo.GetByID(ctx, tx, orderID)
		if lookupErr == nil && recoveredOrder != nil {
			return nil
		}

		recoveredOrder, lookupErr = repo.GetByPricingTokenID(ctx, tx, tokenID)
		return lookupErr
	})
	if err != nil {
		return nil, err
	}
	return recoveredOrder, nil
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

	// Convert to response
	data := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		data[i] = messageToResponse(msg, senderCards, sellerLifecycles)
	}

	// Resource projection hydration: batch-fetch occurrences, resolve
	// via the canonical aggregate resolver, and attach the projection
	// envelope to each message response.
	if len(messages) > 0 {
		occurrences, err := h.getResourceOccurrencesByMessageIDs(ctx, messages)
		if err != nil {
			h.log.Error("Failed to fetch resource occurrences",
				zap.String("room_id", roomID.String()),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to resolve resource projections")
			return
		}
		if len(occurrences) > 0 {
			if h.resourceProjectionResolver == nil {
				response.InternalServerError(c, "Resource projection resolver not configured")
				return
			}
			projections, err := h.resourceProjectionResolver.ResolveResourceProjections(ctx, userID, occurrences)
			if err != nil {
				h.log.Error("Failed to resolve resource projections",
					zap.String("room_id", roomID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to resolve resource projections")
				return
			}
			for i, msg := range messages {
				if proj, ok := projections[msg.ID]; ok {
					data[i]["resource_projection"] = proj
				}
			}
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

	// For text messages, body is required
	var body *string
	if messageType == chatEntity.MessageTypeText {
		if req.Body == "" {
			response.BadRequest(c, "Body is required for text messages")
			return
		}
		body = &req.Body
	} else if req.Body != "" {
		body = &req.Body
	}

	// Send message within transaction
	var message *chatEntity.ChatMessage
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var svcErr error
		message, svcErr = h.chatService.SendMessage(
			ctx,
			roomID,
			senderID,
			messageType,
			body,
			req.AttachmentJSON,
			req.IdempotencyKey,
		)
		return svcErr
	})

	if err != nil {
		if err == chatRepo.ErrInvalidIdempotencyKey {
			response.BadRequest(c, "Idempotency key is required")
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
	response.Success(c, messageToResponse(message, senderCards, sellerLifecycles))
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
) map[string]interface{} {
	resp := roomToResponse(room, userID, participantCards)
	if lastMessage != nil {
		resp["last_message"] = messageToResponse(lastMessage, nil, nil)
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

func (h *Handler) batchUnreadCounts(
	ctx context.Context,
	roomIDs []uuid.UUID,
	userID uuid.UUID,
) (map[uuid.UUID]int, error) {
	out := make(map[uuid.UUID]int, len(roomIDs))
	if len(roomIDs) == 0 {
		return out, nil
	}
	for _, id := range roomIDs {
		out[id] = 0
	}

	const q = `
		WITH target_rooms AS (
			SELECT UNNEST($1::uuid[]) AS room_id
		),
		room_read_states AS (
			SELECT room_id, last_read_at
			FROM chat_read_states
			WHERE user_id = $2 AND room_id = ANY($1)
		)
		SELECT
			tr.room_id,
			COALESCE(COUNT(m.id), 0) AS unread_count
		FROM target_rooms tr
		LEFT JOIN room_read_states rs ON rs.room_id = tr.room_id
		LEFT JOIN chat_messages m ON
			m.room_id = tr.room_id
			AND m.deleted_at IS NULL
			AND (rs.last_read_at IS NULL OR m.created_at > rs.last_read_at)
			AND m.sender_id NOT IN (
				SELECT muted_id FROM user_mutes WHERE muter_id = $2
			)
		GROUP BY tr.room_id
	`

	rows, err := h.db.Pool().Query(ctx, q, roomIDs, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			roomID uuid.UUID
			count  int
		)
		if err := rows.Scan(&roomID, &count); err != nil {
			return nil, err
		}
		out[roomID] = count
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
func messageToResponse(
	msg *chatEntity.ChatMessage,
	senderCards map[uuid.UUID]publiccard.UserCard,
	sellerLifecycles map[string]attachmentSellerLifecycle,
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
// E4.2 — bounded chat-only activation per docs/contracts/governance-
// constitution.md §5 (chat = fail-CLOSED on relationship overlay; lifecycle
// presence is mandatory on the participant card). Mirrors the comment-
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
// plumbing; the public boundary (single canonical exposure authority per
// docs/contracts/public-card-boundary.md §1) is preserved by passing the
// pre-coarsened lifecycle string into publiccard.NewWithLifecycle. Raw
// account_status enum strings never leave this function.
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
	// dropping the row would break thread continuity and violate the
	// chat-specific carve-out in content-detail-visibility-doctrine.md
	// §2.5.
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
	case "negotiation_offer", "negotiation_result", "negotiation_proposal":
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
		case "negotiation_offer", "negotiation_result", "negotiation_proposal":
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

// ErrNoAcceptedNegotiation is returned when no accepted negotiation is found for a chat room.
type ErrNoAcceptedNegotiation struct {
	RoomID uuid.UUID
}

func (e *ErrNoAcceptedNegotiation) Error() string {
	return fmt.Sprintf("no accepted negotiation found for room: %s", e.RoomID)
}

// ErrNegotiationAlreadySettled is returned when negotiation already has an order.
type ErrNegotiationAlreadySettled struct {
	NegotiationID uuid.UUID
	OrderID       uuid.UUID
}

func (e *ErrNegotiationAlreadySettled) Error() string {
	return fmt.Sprintf("negotiation %s already settled with order %s", e.NegotiationID, e.OrderID)
}

// ErrNegotiationExpired is returned when negotiation has expired.
type ErrNegotiationExpired struct {
	NegotiationID uuid.UUID
}

func (e *ErrNegotiationExpired) Error() string {
	return fmt.Sprintf("negotiation %s has expired", e.NegotiationID)
}

// ErrUnauthorizedBuyer is returned when user is not the buyer.
type ErrUnauthorizedBuyer struct {
	UserID        uuid.UUID
	NegotiationID uuid.UUID
}

func (e *ErrUnauthorizedBuyer) Error() string {
	return fmt.Sprintf("user %s is not the buyer of negotiation %s", e.UserID, e.NegotiationID)
}

// ErrInvalidNegotiationData is returned when negotiation data is invalid.
type ErrInvalidNegotiationData struct {
	Field         string
	NegotiationID uuid.UUID
}

func (e *ErrInvalidNegotiationData) Error() string {
	return fmt.Sprintf("negotiation %s has invalid %s", e.NegotiationID, e.Field)
}

// isUniqueViolationError checks if error is a PostgreSQL unique violation.
func isUniqueViolationError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "23505")
}

// buildPricingSnapshotFromToken converts a validated PricingToken to a PricingSnapshot.
//
// This helper function extracts pricing data from the validated pricing token
// and converts it to the format expected by the order creation service.
//
// CRITICAL: The token is the SINGLE SOURCE OF TRUTH for all pricing data.
// No frontend values are used in pricing calculations.
func buildPricingSnapshotFromToken(token *pricingtokenentity.PricingToken) *orderApp.PricingSnapshot {
	// Determine shipping source
	var shippingSource *string
	if token.ShippingQuoteID != nil {
		source := "shipping_quote"
		shippingSource = &source
	} else {
		source := "for_sale"
		shippingSource = &source
	}

	// Parse address snapshot from JSONB
	var addressSnapshot *addressentity.AddressSnapshot
	if len(token.AddressSnapshot) > 0 {
		var snapshot addressentity.AddressSnapshot
		if err := json.Unmarshal(token.AddressSnapshot, &snapshot); err == nil {
			addressSnapshot = &snapshot
		}
	}

	return &orderApp.PricingSnapshot{
		UnitPrice:             token.UnitPrice,
		Subtotal:              token.Subtotal,
		ShippingTotal:         token.ShippingTotal,
		CommissionPercent:     token.CommissionPercent,
		CommissionAmount:      token.CommissionAmount,
		EscrowAmount:          token.EscrowAmount,
		ServiceFeeAmount:      token.ServiceFeeAmount,
		TotalPayableAmount:    token.TotalPayableAmount,
		DiscountAmount:        token.DiscountAmount,
		MaxCoinsAllowed:       token.MaxCoinsAllowed,
		CoinsUsed:             token.CoinsUsed,
		OrderValueForCoins:    token.OrderValueForCoins,
		ShippingSetupName:     token.ShippingSetupName,
		ShippingTransportType: token.ShippingTransportType,
		ShippingDestination:   addressSnapshot,
		ShippingSource:        shippingSource,
		ShippingQuoteID:       token.ShippingQuoteID,
		ChatID:                nil, // Set during chat checkout if needed
		AuctionID:             token.AuctionID,
		PaymentMethod:         "default",   // TODO: Add payment method to token
		TokenID:               token.Token, // Store token ID to prevent double-ordering
	}
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

// sessionToResponse converts a NegotiationSession to a JSON-friendly response map.
func sessionToResponse(s *negotiationEntity.NegotiationSession) gin.H {
	resp := gin.H{
		"id":                s.ID.String(),
		"resource_type":     string(s.ResourceType),
		"buyer_id":          s.BuyerID.String(),
		"seller_id":         s.SellerID.String(),
		"status":            string(s.Status),
		"proposal_sequence": s.ProposalSequence,
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

// buildNegotiationCheckoutInput builds the canonical order input for a negotiation checkout.
func buildNegotiationCheckoutInput(
	negotiation *negotiationEntity.NegotiationSession,
	forSale *forsaleEntity.ForSale,
	buyerID uuid.UUID,
	addressID uuid.UUID,
	shippingSetupID uuid.UUID,
	pricingSnapshot *orderApp.PricingSnapshot,
	pricingTokenID *uuid.UUID,
	quantity int,
) orderApp.CreateFromSaleSurfaceInput {
	return orderApp.CreateFromSaleSurfaceInput{
		ProductID:       forSale.ProductID,
		SourceType:      orderentity.OrderSourceForSale,
		SourceID:        forSale.ID,
		BuyerID:         buyerID,
		Quantity:        quantity,
		AddressID:       addressID,
		ShippingSetupID: shippingSetupID,
		NegotiationID:   &negotiation.ID,
		PricingSnapshot: pricingSnapshot,
		PricingTokenID:  pricingTokenID,
	}
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
	// is also what GetNegotiation/CreateOrderFromChat will later look it up by.
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

	response.Success(c, sessionToResponse(session))
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

	err = h.negotiationService.SendCounterOffer(ctx, negotiationApp.SendCounterOfferRequest{
		SessionID: sessionID,
		SenderID:  userID,
		Price:     req.Price,
		Note:      req.Note,
	})
	if err != nil {
		h.log.Error("Failed to send counter offer",
			zap.String("room_id", roomID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.RespondWithError(c, h.log, err)
		return
	}

	response.Success(c, gin.H{"message": "Counter offer sent"})
}

// RespondToNegotiation handles POST /api/v1/chat/rooms/:room_id/respond
//
// Accepts or cancels a negotiation. Only seller can accept; only buyer can cancel.
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
			SellerID:  userID,
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
		response.Success(c, sessionToResponse(session))

	case "cancel":
		// No EnsureActive for cancel — suspended users can cancel (cleanup exemption)
		err := h.negotiationService.CancelNegotiation(ctx, negotiationApp.CancelNegotiationRequest{
			SessionID: sessionID,
			BuyerID:   userID,
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

	response.Success(c, sessionToResponse(session))
}
