package http

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/interaction/bidding/application"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// BiddingHandler handles HTTP requests for user bidding data.
type BiddingHandler struct {
	biddingService *application.BiddingService
	db             *db.DB
	log            *zap.Logger
}

// NewBiddingHandler creates a new BiddingHandler.
func NewBiddingHandler(
	biddingService *application.BiddingService,
	database *db.DB,
	log *zap.Logger,
) *BiddingHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &BiddingHandler{
		biddingService: biddingService,
		db:             database,
		log:            log,
	}
}

// GetMyBidding handles GET /api/v1/bidding
//
// Returns all auctions where the authenticated user has placed bids,
// aggregated with user's bid information and derived status.
//
// Response format:
// {
//   "items": [...],
//   "active_count": X,
//   "won_count": X,
//   "lost_count": X
// }
func (h *BiddingHandler) GetMyBidding(c *gin.Context) {
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

	// Execute within transaction
	var result *application.BiddingResult
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.biddingService.GetUserBidding(ctx, tx, userID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get user bidding",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve bidding data")
		return
	}

	// Map to response format (snake_case for JSON)
	response.Success(c, gin.H{
		"items":       result.Items,
		"active_count": result.ActiveCount,
		"won_count":    result.WonCount,
		"lost_count":   result.LostCount,
	})
}


