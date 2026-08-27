package http

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	forSaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	savedItemApp "github.com/labuda/backend/internal/interaction/saved_item/application"
	savedItemEntity "github.com/labuda/backend/internal/interaction/saved_item/entity"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// SavedItemHandler handles HTTP requests for saved item operations
// This is the UNIFIED endpoint for all saved items (forSales + auctions)
type SavedItemHandler struct {
	savedItemService *savedItemApp.SavedItemService
	db               *db.DB
	log              *zap.Logger
}

// NewSavedItemHandler creates a new SavedItemHandler
func NewSavedItemHandler(
	savedItemService *savedItemApp.SavedItemService,
	db *db.DB,
	log *zap.Logger,
) *SavedItemHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &SavedItemHandler{
		savedItemService: savedItemService,
		db:               db,
		log:              log,
	}
}

// SetSavedItemService sets the saved item service (for dependency injection)
func (h *SavedItemHandler) SetSavedItemService(service *savedItemApp.SavedItemService) {
	h.savedItemService = service
}

// ============================================================================
// REQUEST DTOs
// ============================================================================

// AddSavedItemRequest represents the request body for adding a saved item
type AddSavedItemRequest struct {
	TargetType string `json:"target_type" binding:"required"`
	TargetID   string `json:"target_id" binding:"required"`
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

// GetSavedItems handles GET /api/v1/saved-items
//
// Query parameters:
// - type: filter by type ("for_sale" or "auction", optional)
//
// Returns the user's saved items with details
func (h *SavedItemHandler) GetSavedItems(c *gin.Context) {
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

	// Get type filter
	targetTypeFilter := c.Query("type")

	// Get saved items
	savedList, err := h.savedItemService.GetSavedItems(ctx, userID)
	if err != nil {
		h.log.Error("Failed to get saved items",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve saved items")
		return
	}

	// Filter by type if specified
	if targetTypeFilter != "" {
		if targetTypeFilter == "for_sale" {
			savedList.Auctions = nil
			savedList.Total = len(savedList.Items)
		} else if targetTypeFilter == "auction" {
			savedList.Items = nil
			savedList.Total = len(savedList.Auctions)
		}
	}

	response.Success(c, savedList)
}

// AddSavedItem handles POST /api/v1/saved-items
//
// Request body:
// - target_type: "for_sale" or "auction"
// - target_id: UUID of the forSale or auction
//
// Adds an item to the user's saved items
// If the item is already saved, returns success (idempotent)
func (h *SavedItemHandler) AddSavedItem(c *gin.Context) {
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
	var req AddSavedItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Validate target type
	targetType := savedItemEntity.TargetType(req.TargetType)
	if !targetType.IsValid() {
		response.BadRequest(c, "Invalid target_type. Must be 'for_sale' or 'auction'")
		return
	}

	// Parse target ID
	targetID, err := uuid.Parse(req.TargetID)
	if err != nil {
		response.BadRequest(c, "Invalid target_id")
		return
	}

	var item *savedItemEntity.SavedItem

	// Add based on type
	switch targetType {
	case savedItemEntity.TargetTypeForSale:
		input := savedItemApp.AddForSaleInput{
			UserID:    userID,
			ForSaleID: targetID,
		}
		item, err = h.savedItemService.AddForSale(ctx, input)
		if err != nil {
			h.log.Error("Failed to add forSale to saved items",
				zap.String("user_id", userID.String()),
				zap.String("for_sale_id", targetID.String()),
				zap.Error(err),
			)

			// Check for specific errors
			if errors.Is(err, &forSaleEntity.ForSaleNotActiveError{}) {
				response.Error(c, 400, "FOR_SALE_NOT_ACTIVE", "ForSale is not active")
				return
			}
			if errors.Is(err, &forSaleEntity.ForSaleNotAvailableError{}) {
				response.Error(c, 400, "FOR_SALE_NOT_AVAILABLE", "ForSale is not available")
				return
			}
			if contains(err.Error(), "cannot save your own forSale") {
				response.Error(c, 400, "OWN_FOR_SALE", "Cannot save your own forSale")
				return
			}

			response.InternalServerError(c, "Failed to add forSale to saved items")
			return
		}

	case savedItemEntity.TargetTypeAuction:
		input := savedItemApp.AddAuctionInput{
			UserID:    userID,
			AuctionID: targetID,
		}
		item, err = h.savedItemService.AddAuction(ctx, input)
		if err != nil {
			h.log.Error("Failed to add auction to saved items",
				zap.String("user_id", userID.String()),
				zap.String("auction_id", targetID.String()),
				zap.Error(err),
			)

			// Check for specific errors
			if contains(err.Error(), "cannot watch ended auction") {
				response.Error(c, 400, "AUCTION_ENDED", "Cannot watch ended auction")
				return
			}
			if contains(err.Error(), "cannot watch cancelled auction") {
				response.Error(c, 400, "AUCTION_CANCELLED", "Cannot watch cancelled auction")
				return
			}

			response.InternalServerError(c, "Failed to add auction to saved items")
			return
		}

	default:
		response.BadRequest(c, "Invalid target_type")
		return
	}

	response.Created(c, item)
}

// RemoveSavedItem handles DELETE /api/v1/saved-items/{id}
//
// URL parameters:
// - type: "for_sale" or "auction"
// - id: UUID of the forSale or auction
//
// Removes an item from the user's saved items
func (h *SavedItemHandler) RemoveSavedItem(c *gin.Context) {
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

	// Get target type from query
	targetTypeStr := c.Query("type")
	if targetTypeStr == "" {
		response.BadRequest(c, "Missing 'type' query parameter")
		return
	}

	targetType := savedItemEntity.TargetType(targetTypeStr)
	if !targetType.IsValid() {
		response.BadRequest(c, "Invalid type. Must be 'for_sale' or 'auction'")
		return
	}

	// Parse target ID from URL
	targetIDStr := c.Param("id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid target ID")
		return
	}

	// Remove item
	err = h.savedItemService.RemoveItem(ctx, userID, targetType, targetID)
	if err != nil {
		h.log.Error("Failed to remove saved item",
			zap.String("user_id", userID.String()),
			zap.String("target_type", string(targetType)),
			zap.String("target_id", targetID.String()),
			zap.Error(err),
		)

		response.InternalServerError(c, "Failed to remove saved item")
		return
	}

	response.SuccessWithMessage(c, "Item removed from saved items", gin.H{
		"target_type": targetType,
		"target_id":   targetID,
	})
}

// ClearSavedItems handles DELETE /api/v1/saved-items
//
// Query parameters:
// - type: filter by type ("for_sale" or "auction", optional)
//
// Clears all saved items or all items of a specific type
func (h *SavedItemHandler) ClearSavedItems(c *gin.Context) {
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

	// Get type filter
	targetTypeFilter := c.Query("type")

	var err error

	if targetTypeFilter != "" {
		targetType := savedItemEntity.TargetType(targetTypeFilter)
		if !targetType.IsValid() {
			response.BadRequest(c, "Invalid type. Must be 'for_sale' or 'auction'")
			return
		}
		err = h.savedItemService.ClearByType(ctx, userID, targetType)
	} else {
		err = h.savedItemService.ClearAll(ctx, userID)
	}

	if err != nil {
		h.log.Error("Failed to clear saved items",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to clear saved items")
		return
	}

	response.SuccessWithMessage(c, "Saved items cleared", nil)
}

// GetSavedItemsCount handles GET /api/v1/saved-items/count
//
// Query parameters:
// - type: filter by type ("for_sale" or "auction", optional)
//
// Returns the number of saved items
func (h *SavedItemHandler) GetSavedItemsCount(c *gin.Context) {
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

	var count int
	var err error

	// Get type filter
	targetTypeFilter := c.Query("type")

	if targetTypeFilter != "" {
		targetType := savedItemEntity.TargetType(targetTypeFilter)
		if !targetType.IsValid() {
			response.BadRequest(c, "Invalid type. Must be 'for_sale' or 'auction'")
			return
		}
		count, err = h.savedItemService.CountByType(ctx, userID, targetType)
	} else {
		count, err = h.savedItemService.Count(ctx, userID)
	}

	if err != nil {
		h.log.Error("Failed to get saved items count",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve saved items count")
		return
	}

	response.Success(c, gin.H{
		"count": count,
	})
}

// IsSaved handles GET /api/v1/saved-items/check
//
// Query parameters:
// - type: "for_sale" or "auction" (required)
// - id: UUID of the forSale or auction (required)
//
// Returns whether the user has saved an item
func (h *SavedItemHandler) IsSaved(c *gin.Context) {
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

	// Get target type from query
	targetTypeStr := c.Query("type")
	if targetTypeStr == "" {
		response.BadRequest(c, "Missing 'type' query parameter")
		return
	}

	targetType := savedItemEntity.TargetType(targetTypeStr)
	if !targetType.IsValid() {
		response.BadRequest(c, "Invalid type. Must be 'for_sale' or 'auction'")
		return
	}

	// Get target ID from query
	targetIDStr := c.Query("id")
	if targetIDStr == "" {
		response.BadRequest(c, "Missing 'id' query parameter")
		return
	}

	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid target ID")
		return
	}

	// Check if saved
	isSaved, err := h.savedItemService.IsSaved(ctx, userID, targetType, targetID)
	if err != nil {
		h.log.Error("Failed to check if item is saved",
			zap.String("user_id", userID.String()),
			zap.String("target_type", string(targetType)),
			zap.String("target_id", targetID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to check saved status")
		return
	}

	response.Success(c, gin.H{
		"is_saved": isSaved,
	})
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
