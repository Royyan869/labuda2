package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	shippingQuoteApp "github.com/labuda/backend/internal/commerce/shipping/quote/application"
	shippingQuoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// RoomGetter retrieves chat rooms for participant validation at the handler level.
type RoomGetter interface {
	GetRoomByID(ctx context.Context, tx db.Tx, roomID uuid.UUID) (*chatEntity.ChatRoom, error)
}

// Handler handles HTTP requests for shipping quote operations.
type Handler struct {
	quoteService *shippingQuoteApp.Service
	roomGetter   RoomGetter
	db           *db.DB
	log          *zap.Logger
}

// NewHandler creates a new shipping quote handler.
func NewHandler(
	quoteService *shippingQuoteApp.Service,
	roomGetter RoomGetter,
	database *db.DB,
	log *zap.Logger,
) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{
		quoteService: quoteService,
		roomGetter:   roomGetter,
		db:           database,
		log:          log,
	}
}

// ========================================================================
// REQUEST DTOs
// ========================================================================

// CreateShippingQuoteRequest holds the request body for creating a shipping quote.
type CreateShippingQuoteRequest struct {
	ProductID             string `json:"product_id" binding:"required"`
	SourceType            string `json:"source_type" binding:"required"`
	SourceID              string `json:"source_id" binding:"required"`
	Cost                  int64  `json:"cost" binding:"required,min=0"`
	Note                  string `json:"note"`
	DestinationCityID     string `json:"destination_city_id,omitempty"`     // Optional address lock (TASK D)
	DestinationProvinceID string `json:"destination_province_id,omitempty"` // Optional address lock (TASK D)
	ExpiresInHours        *int   `json:"expires_in_hours,omitempty"`        // Optional expiration (TASK C)
}

// ========================================================================
// SHIPPING QUOTE ENDPOINTS
// ========================================================================

// CreateShippingQuote handles POST /api/v1/chat/:chat_id/shipping-quote
//
// Creates a new shipping quote and sends a chat message.
//
// TASK A-G: Enhanced with auction support, address lock, lifecycle
//
// Request body:
//   - product_id, source_type, source_id: product and sale-surface binding
//   - cost: Shipping cost in smallest currency unit (e.g., cents)
//   - note: Optional note from seller
//   - destination_city_id: Optional destination city lock
//   - destination_province_id: Optional destination province lock
//   - expires_in_hours: Optional quote expiration in hours. Backend is
//     authoritative (PASS_18P): omitted defaults to 24h, must be positive,
//     and must not exceed 168h (7 days) or the request is rejected.
func (h *Handler) CreateShippingQuote(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID (seller)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	sellerID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse chat_id from URL
	chatIDStr := c.Param("chat_id")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid chat ID")
		return
	}

	// Verify caller is a participant of the chat room
	room, err := h.roomGetter.GetRoomByID(ctx, nil, chatID)
	if err != nil {
		response.NotFound(c, "Chat room not found")
		return
	}
	if !room.HasParticipant(sellerID) {
		response.Forbidden(c, "You are not a participant in this chat")
		return
	}

	// Parse request body
	var req CreateShippingQuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Prepare input with defaults
	input := shippingQuoteApp.CreateShippingQuoteInput{
		ChatID:         chatID,
		SellerID:       sellerID,
		Cost:           money.New(req.Cost),
		ExpiresInHours: req.ExpiresInHours, // TASK C: Optional expiration
	}

	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		response.BadRequest(c, "Invalid product ID")
		return
	}
	sourceID, err := uuid.Parse(req.SourceID)
	if err != nil {
		response.BadRequest(c, "Invalid source ID")
		return
	}
	input.ProductID = productID
	input.SourceType = req.SourceType
	input.SourceID = sourceID

	// Handle destination address lock (TASK D)
	if req.DestinationCityID != "" {
		input.DestinationCityID = &req.DestinationCityID
	}
	if req.DestinationProvinceID != "" {
		input.DestinationProvinceID = &req.DestinationProvinceID
	}

	// Handle note (optional)
	if req.Note != "" {
		input.Note = &req.Note
	}

	// Create shipping quote
	quote, err := h.quoteService.CreateShippingQuote(ctx, input)
	if err != nil {
		h.log.Error("failed to create shipping quote",
			zap.Error(err),
			zap.String("chat_id", chatID.String()),
			zap.String("seller_id", sellerID.String()),
		)
		response.InternalServerError(c, "Failed to create shipping quote")
		return
	}

	response.Success(c, h.quoteToResponse(quote))
}

// GetShippingQuoteByID handles GET /api/v1/shipping-quote/:quote_id
//
// Retrieves a shipping quote by ID. Caller must be the buyer or seller of the quote.
func (h *Handler) GetShippingQuoteByID(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID
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

	// Parse quote_id from URL
	quoteIDStr := c.Param("quote_id")
	quoteID, err := uuid.Parse(quoteIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid quote ID")
		return
	}

	// Get shipping quote
	quote, err := h.quoteService.GetByID(ctx, quoteID)
	if err != nil {
		h.log.Error("failed to get shipping quote",
			zap.Error(err),
			zap.String("quote_id", quoteID.String()),
		)
		response.InternalServerError(c, "Failed to get shipping quote")
		return
	}

	if quote == nil {
		response.NotFound(c, "Shipping quote not found")
		return
	}

	// Verify caller is the buyer or seller of this quote
	if quote.SellerID != userID && quote.BuyerID != userID {
		response.Forbidden(c, "You are not authorized to view this quote")
		return
	}

	response.Success(c, h.quoteToResponse(quote))
}

// ========================================================================
// RESPONSE DTOs
// ========================================================================

// ShippingQuoteResponse represents a shipping quote response.
// TASK A-C: Enhanced with status, destination lock, expiration
type ShippingQuoteResponse struct {
	ID                    string `json:"id"`
	ChatID                string `json:"chat_id"`
	ProductID             string `json:"product_id"`
	SourceType            string `json:"source_type"`
	SourceID              string `json:"source_id"`
	SellerID              string `json:"seller_id"`
	BuyerID               string `json:"buyer_id"`
	Cost                  int64  `json:"cost"`
	Note                  *string `json:"note,omitempty"`
	Status                string  `json:"status"`                            // TASK C: Quote status
	DestinationCityID     *string `json:"destination_city_id,omitempty"`     // TASK D
	DestinationProvinceID *string `json:"destination_province_id,omitempty"` // TASK D
	ExpiresAt             *string `json:"expires_at,omitempty"`              // TASK C
	UsedAt                *string `json:"used_at,omitempty"`                 // TASK C
	CreatedAt             string  `json:"created_at"`
}

// quoteToResponse converts a ShippingQuote entity to a response DTO.
func (h *Handler) quoteToResponse(quote *shippingQuoteEntity.ShippingQuote) ShippingQuoteResponse {
	resp := ShippingQuoteResponse{
		ID:         quote.ID.String(),
		ChatID:     quote.ChatID.String(),
		ProductID:  quote.ProductID.String(),
		SourceType: derefString(quote.SourceType),
		SourceID:   derefUUID(quote.SourceID),
		SellerID:   quote.SellerID.String(),
		BuyerID:    quote.BuyerID.String(),
		Cost:       quote.Cost.Int64(),
		Note:       quote.Note,
		Status:     string(quote.Status), // TASK C: Include status
		CreatedAt:  quote.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Handle destination lock (TASK D)
	resp.DestinationCityID = quote.DestinationCityID
	resp.DestinationProvinceID = quote.DestinationProvinceID

	// Handle expiration (TASK C)
	if quote.ExpiresAt != nil {
		expiresAt := quote.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		resp.ExpiresAt = &expiresAt
	}

	// Handle used timestamp (TASK C)
	if quote.UsedAt != nil {
		usedAt := quote.UsedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.UsedAt = &usedAt
	}

	return resp
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefUUID(v *uuid.UUID) string {
	if v == nil {
		return ""
	}
	return v.String()
}
