package http

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ratingApp "github.com/labuda/backend/internal/commerce/order/rating/application"
	ratingEntity "github.com/labuda/backend/internal/commerce/order/rating/entity"
	ratingRepo "github.com/labuda/backend/internal/commerce/order/rating/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// RatingHandler handles HTTP requests for rating operations.
type RatingHandler struct {
	ratingService *ratingApp.RatingService
	db            *db.DB
	log           *zap.Logger
}

// NewRatingHandler creates a new RatingHandler.
func NewRatingHandler(
	ratingService *ratingApp.RatingService,
	db *db.DB,
	log *zap.Logger,
) *RatingHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &RatingHandler{
		ratingService: ratingService,
		db:            db,
		log:           log,
	}
}

// CreateRatingRequest holds the request body for creating a rating.
type CreateRatingRequest struct {
	RatingValue int     `json:"rating_value" binding:"required,min=1,max=5"`
	Comment     *string `json:"comment,omitempty"`
}

// CreateRating handles POST /api/v1/orders/{id}/ratings
//
// Authorization:
// - Only buyer can rate seller
// - Order must be completed
// - No update/delete endpoint (ratings are immutable)
//
// Idempotency: The UNIQUE constraint on order_id ensures that an order
// can only be rated once. Attempting to rate again returns ErrAlreadyRated.
func (h *RatingHandler) CreateRating(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

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

	// Parse request body
	var req CreateRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Execute create within transaction
	var newRating *ratingEntity.OrderRating
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		input := ratingApp.CreateRatingInput{
			OrderID:     orderID,
			CallerID:    userID,
			RatingValue: req.RatingValue,
			Comment:     req.Comment,
		}
		var err error
		newRating, err = h.ratingService.CreateRating(ctx, tx, input)
		return err
	})

	if err != nil {
		h.log.Error("Failed to create rating",
			zap.String("order_id", orderID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)

		// Check for specific errors
		if _, ok := err.(*ratingEntity.ErrOrderNotCompleted); ok {
			response.Error(c, 409, "INVALID_STATE", "Can only rate completed orders")
			return
		}
		if _, ok := err.(*ratingEntity.ErrNotBuyer); ok {
			response.Forbidden(c, "Only the buyer can rate the seller")
			return
		}
		if _, ok := err.(*ratingEntity.ErrAlreadyRated); ok {
			response.Error(c, 409, "ALREADY_RATED", "Order has already been rated")
			return
		}
		if _, ok := err.(*ratingEntity.ErrOrderNotFound); ok {
			response.NotFound(c, "Order not found")
			return
		}
		if _, ok := err.(*ratingEntity.ErrInvalidRatingValue); ok {
			response.BadRequest(c, "Invalid request")
			return
		}

		response.InternalServerError(c, "Failed to create rating")
		return
	}

	// Idempotent response: if newRating is nil (but no error), request was already processed
	if newRating == nil {
		response.Success(c, gin.H{"message": "Rating already created"})
		return
	}

	response.Created(c, newRating)
}

// ListRatingsReceivedRequest holds the query parameters for listing ratings received.
type ListRatingsReceivedRequest struct {
	Limit  int   `form:"limit" binding:"omitempty,min=1,max=50"`
	Cursor int64 `form:"cursor" binding:"omitempty,min=0"`
}

// ListRatingsReceived handles GET /api/v1/users/{id}/ratings
//
// Query parameters:
// - limit (optional): Number of results per page (default: 20, max: 50)
// - cursor (optional): Unix timestamp in nanoseconds for pagination
//
// Returns ratings received by the user (as seller).
func (h *RatingHandler) ListRatingsReceived(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse user ID
	idStr := c.Param("id")
	sellerID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Parse query parameters
	var req ListRatingsReceivedRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// Execute list within transaction
	var ratings []*ratingEntity.OrderRating
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		input := ratingApp.ListRatingsReceivedBySellerInput{
			SellerID: sellerID,
			Limit:    req.Limit,
			Cursor:   req.Cursor,
		}
		var err error
		ratings, err = h.ratingService.ListRatingsReceivedBySeller(ctx, tx, input)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list ratings",
			zap.String("seller_id", sellerID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve ratings")
		return
	}

	response.Success(c, ratings)
}

// ListRatingsGivenRequest holds the query parameters for listing ratings given.
type ListRatingsGivenRequest struct {
	Limit  int   `form:"limit" binding:"omitempty,min=1,max=50"`
	Cursor int64 `form:"cursor" binding:"omitempty,min=0"`
}

// ListRatingsGiven handles GET /api/v1/users/me/ratings/given
//
// Query parameters:
// - limit (optional): Number of results per page (default: 20, max: 50)
// - cursor (optional): Unix timestamp in nanoseconds for pagination
//
// Returns ratings given by the authenticated user (as buyer).
func (h *RatingHandler) ListRatingsGiven(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	buyerID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse query parameters
	var req ListRatingsGivenRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// Execute list within transaction
	var ratings []*ratingEntity.OrderRating
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		input := ratingApp.ListRatingsGivenByBuyerInput{
			BuyerID: buyerID,
			Limit:   req.Limit,
			Cursor:  req.Cursor,
		}
		var err error
		ratings, err = h.ratingService.ListRatingsGivenByBuyer(ctx, tx, input)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list ratings given",
			zap.String("buyer_id", buyerID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve ratings")
		return
	}

	response.Success(c, ratings)
}

// GetRatingSummary handles GET /api/v1/users/{id}/ratings/summary
//
// Returns the aggregated rating summary for a seller:
// - total_ratings: Total number of valid ratings
// - average_rating: Average rating value (1-5)
// - Distribution by star rating (1-5 stars)
//
// Only includes valid ratings (invalidated_at IS NULL).
// Invalidated ratings from refunded orders are excluded from the summary.
func (h *RatingHandler) GetRatingSummary(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse user ID
	idStr := c.Param("id")
	sellerID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Execute get summary within transaction
	var summary *ratingRepo.RatingSummary
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		summary, err = h.ratingService.GetRatingSummary(ctx, tx, sellerID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get rating summary",
			zap.String("seller_id", sellerID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve rating summary")
		return
	}

	response.Success(c, summary)
}


