package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/response"
	promotionApp "github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
)

// CreateExternalProductRequest creates a draft external product.
type CreateExternalProductRequest struct {
	Title       string  `json:"title" binding:"required"`
	Description *string `json:"description"`
	ExternalURL string  `json:"external_url" binding:"required"`
}

// UpdateExternalProductRequest updates an owned external product.
type UpdateExternalProductRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	ExternalURL *string `json:"external_url"`
}

// SubmitExternalProductRequest optionally carries a note for the submit action.
type SubmitExternalProductRequest struct {
	Note *string `json:"note,omitempty"`
}

// ResubmitExternalProductRequest optionally carries a note for the resubmit action.
type ResubmitExternalProductRequest struct {
	Note *string `json:"note,omitempty"`
}

// AttachExternalProductMediaRequest attaches already-uploaded media metadata.
type AttachExternalProductMediaRequest struct {
	MediaType    string          `json:"media_type" binding:"required,oneof=image video"`
	StorageKey   string          `json:"storage_key" binding:"required"`
	URL          string          `json:"url" binding:"required"`
	ThumbnailURL *string         `json:"thumbnail_url"`
	SortOrder    *int            `json:"sort_order"`
	Metadata     json.RawMessage `json:"metadata"`
}

// ExternalProductResponse is the owned user-facing external product DTO.
type ExternalProductResponse struct {
	ID                    uuid.UUID                      `json:"id"`
	OwnerUserID           uuid.UUID                      `json:"owner_user_id"`
	Title                 string                         `json:"title"`
	Description           *string                        `json:"description"`
	ExternalURL           string                         `json:"external_url"`
	NormalizedExternalURL string                         `json:"normalized_external_url"`
	ReviewStatus          string                         `json:"review_status"`
	RejectionReason       *string                        `json:"rejection_reason"`
	UnsafeURLFlag         bool                           `json:"unsafe_url_flag"`
	SubmittedAt           *string                        `json:"submitted_at,omitempty"`
	ApprovedAt            *string                        `json:"approved_at,omitempty"`
	RejectedAt            *string                        `json:"rejected_at,omitempty"`
	HiddenAt              *string                        `json:"hidden_at,omitempty"`
	LastReviewedBy        *uuid.UUID                     `json:"last_reviewed_by,omitempty"`
	CreatedAt             string                         `json:"created_at"`
	UpdatedAt             string                         `json:"updated_at"`
	Media                 []ExternalProductMediaResponse `json:"media,omitempty"`
	CanEdit               bool                           `json:"can_edit"`
	CanSubmit             bool                           `json:"can_submit"`
	CanResubmit           bool                           `json:"can_resubmit"`
	PublicVisible         bool                           `json:"public_visible"`
}

// ExternalProductMediaResponse is reserved for optional detail/list media support.
type ExternalProductMediaResponse struct {
	ID                uuid.UUID       `json:"id"`
	ExternalProductID uuid.UUID       `json:"external_product_id"`
	MediaType         string          `json:"media_type"`
	StorageKey        string          `json:"storage_key"`
	URL               string          `json:"url"`
	ThumbnailURL      *string         `json:"thumbnail_url"`
	SortOrder         int             `json:"sort_order"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	CreatedAt         string          `json:"created_at"`
}

// ExternalProductMediaListResponse wraps media list results.
type ExternalProductMediaListResponse struct {
	Items []ExternalProductMediaResponse `json:"items"`
	Count int                            `json:"count"`
}

// ExternalProductListResponse wraps list results.
type ExternalProductListResponse struct {
	Items []ExternalProductResponse `json:"items"`
	Count int                       `json:"count"`
}

// ExternalProductReviewHistoryResponse exposes admin review history rows.
type ExternalProductReviewHistoryResponse struct {
	ID                uuid.UUID  `json:"id"`
	ExternalProductID uuid.UUID  `json:"external_product_id"`
	ActorAdminID      *uuid.UUID `json:"actor_admin_id,omitempty"`
	ActorUserID       *uuid.UUID `json:"actor_user_id,omitempty"`
	FromStatus        *string    `json:"from_status,omitempty"`
	ToStatus          string     `json:"to_status"`
	Reason            *string    `json:"reason,omitempty"`
	CreatedAt         string     `json:"created_at"`
}

// AdminExternalProductResponse is the admin-facing external product DTO.
type AdminExternalProductResponse struct {
	ExternalProductResponse
	ReviewHistory []ExternalProductReviewHistoryResponse `json:"review_history,omitempty"`
	CanApprove    bool                                   `json:"can_approve"`
	CanReject     bool                                   `json:"can_reject"`
	CanHide       bool                                   `json:"can_hide"`
}

// AdminExternalProductListResponse wraps admin review queue results.
type AdminExternalProductListResponse struct {
	Items []AdminExternalProductResponse `json:"items"`
	Count int                            `json:"count"`
	Page  int                            `json:"page"`
	Limit int                            `json:"limit"`
}

// ExternalProductReviewHistoryListResponse wraps review history results.
type ExternalProductReviewHistoryListResponse struct {
	Items []ExternalProductReviewHistoryResponse `json:"items"`
	Count int                                    `json:"count"`
}

// CreateExternalProduct handles POST /api/v1/external-products.
func (h *PromotionHandler) CreateExternalProduct(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req CreateExternalProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var (
		product *entity.ExternalProduct
		resp    ExternalProductResponse
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		product, txErr = h.promotionService.CreateExternalProductDraft(ctx, tx, promotionApp.CreateExternalProductDraftInput{
			UserID:      userID,
			Title:       req.Title,
			Description: req.Description,
			ExternalURL: req.ExternalURL,
		})
		if txErr != nil {
			return txErr
		}
		resp, txErr = h.externalProductResponseWithMedia(ctx, tx, product)
		return txErr
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to process external product")
		return
	}

	response.Created(c, resp)
}

// UpdateExternalProduct handles PATCH /api/v1/external-products/:id.
func (h *PromotionHandler) UpdateExternalProduct(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	externalProductID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid external product ID")
		return
	}

	var req UpdateExternalProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.Title == nil && req.Description == nil && req.ExternalURL == nil {
		response.BadRequest(c, "at least one field is required")
		return
	}

	var (
		product *entity.ExternalProduct
		resp    ExternalProductResponse
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		product, txErr = h.promotionService.UpdateExternalProduct(ctx, tx, promotionApp.UpdateExternalProductInput{
			UserID:            userID,
			ExternalProductID: externalProductID,
			Update: entity.ExternalProductUpdateInput{
				Title:       req.Title,
				Description: req.Description,
				ExternalURL: req.ExternalURL,
			},
		})
		if txErr != nil {
			return txErr
		}
		resp, txErr = h.externalProductResponseWithMedia(ctx, tx, product)
		return txErr
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to process external product")
		return
	}

	response.Success(c, resp)
}

// SubmitExternalProduct handles POST /api/v1/external-products/:id/submit.
func (h *PromotionHandler) SubmitExternalProduct(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	externalProductID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid external product ID")
		return
	}

	var req SubmitExternalProductRequest
	if c.Request != nil && c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}

	var (
		product *entity.ExternalProduct
		resp    ExternalProductResponse
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		product, txErr = h.promotionService.SubmitExternalProduct(ctx, tx, userID, externalProductID)
		if txErr != nil {
			return txErr
		}
		resp, txErr = h.externalProductResponseWithMedia(ctx, tx, product)
		return txErr
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to process external product")
		return
	}

	response.Success(c, resp)
}

// ResubmitExternalProduct handles POST /api/v1/external-products/:id/resubmit.
func (h *PromotionHandler) ResubmitExternalProduct(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	externalProductID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid external product ID")
		return
	}

	var req ResubmitExternalProductRequest
	if c.Request != nil && c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}

	var (
		product *entity.ExternalProduct
		resp    ExternalProductResponse
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		product, txErr = h.promotionService.ResubmitExternalProduct(ctx, tx, userID, externalProductID)
		if txErr != nil {
			return txErr
		}
		resp, txErr = h.externalProductResponseWithMedia(ctx, tx, product)
		return txErr
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to process external product")
		return
	}

	response.Success(c, resp)
}

// GetExternalProduct handles GET /api/v1/external-products/:id.
func (h *PromotionHandler) GetExternalProduct(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	externalProductID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid external product ID")
		return
	}

	var (
		product *entity.ExternalProduct
		resp    ExternalProductResponse
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		product, txErr = h.promotionService.GetOwnedExternalProduct(ctx, tx, userID, externalProductID)
		if txErr != nil {
			return txErr
		}
		resp, txErr = h.externalProductResponseWithMedia(ctx, tx, product)
		return txErr
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to process external product")
		return
	}

	response.Success(c, resp)
}

// ListMyExternalProducts handles GET /api/v1/my/external-products.
func (h *PromotionHandler) ListMyExternalProducts(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var (
		items []*entity.ExternalProduct
		resp  ExternalProductListResponse
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		items, txErr = h.promotionService.ListExternalProducts(ctx, tx, userID, repository.ExternalProductListFilters{
			Limit:  pageSize,
			Offset: offset,
		})
		if txErr != nil {
			return txErr
		}
		resp.Items = make([]ExternalProductResponse, 0, len(items))
		for _, item := range items {
			itemResp, itemErr := h.externalProductResponseWithMedia(ctx, tx, item)
			if itemErr != nil {
				return itemErr
			}
			resp.Items = append(resp.Items, itemResp)
		}
		resp.Count = len(resp.Items)
		return nil
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to retrieve external products")
		return
	}

	response.Success(c, ExternalProductListResponse{
		Items: resp.Items,
		Count: resp.Count,
	})
}

// AttachExternalProductMedia handles POST /api/v1/external-products/:id/media.
func (h *PromotionHandler) AttachExternalProductMedia(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	externalProductID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid external product ID")
		return
	}

	var req AttachExternalProductMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}
	metadata := req.Metadata
	if len(metadata) == 0 {
		metadata = nil
	}

	var (
		product *entity.ExternalProduct
		resp    ExternalProductResponse
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		txErr = h.promotionService.AttachExternalProductMedia(ctx, tx, promotionApp.AttachExternalProductMediaInput{
			UserID:            userID,
			ExternalProductID: externalProductID,
			MediaType:         entity.ExternalProductMediaType(req.MediaType),
			StorageKey:        req.StorageKey,
			URL:               req.URL,
			ThumbnailURL:      req.ThumbnailURL,
			SortOrder:         sortOrder,
			Metadata:          metadata,
		})
		if txErr != nil {
			return txErr
		}
		product, txErr = h.promotionService.GetOwnedExternalProduct(ctx, tx, userID, externalProductID)
		if txErr != nil {
			return txErr
		}
		resp, txErr = h.externalProductResponseWithMedia(ctx, tx, product)
		return txErr
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to process external product media")
		return
	}

	response.Created(c, resp)
}

// ListExternalProductMedia handles GET /api/v1/external-products/:id/media.
func (h *PromotionHandler) ListExternalProductMedia(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	externalProductID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid external product ID")
		return
	}

	var items []*entity.ExternalProductMedia
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		if _, txErr = h.promotionService.GetOwnedExternalProduct(ctx, tx, userID, externalProductID); txErr != nil {
			return txErr
		}
		items, txErr = h.promotionService.ListExternalProductMedia(ctx, tx, externalProductID)
		return txErr
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to retrieve external product media")
		return
	}

	resp := make([]ExternalProductMediaResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, externalProductMediaToResponse(item))
	}

	response.Success(c, ExternalProductMediaListResponse{
		Items: resp,
		Count: len(resp),
	})
}

// DeleteExternalProductMedia handles DELETE /api/v1/external-products/:id/media/:media_id.
func (h *PromotionHandler) DeleteExternalProductMedia(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	externalProductID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid external product ID")
		return
	}
	mediaID, err := uuid.Parse(c.Param("media_id"))
	if err != nil {
		response.BadRequest(c, "Invalid media ID")
		return
	}

	var (
		product *entity.ExternalProduct
		resp    ExternalProductResponse
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		txErr = h.promotionService.DeleteExternalProductMedia(ctx, tx, userID, externalProductID, mediaID)
		if txErr != nil {
			return txErr
		}
		product, txErr = h.promotionService.GetOwnedExternalProduct(ctx, tx, userID, externalProductID)
		if txErr != nil {
			return txErr
		}
		resp, txErr = h.externalProductResponseWithMedia(ctx, tx, product)
		return txErr
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to process external product media")
		return
	}

	response.Success(c, resp)
}

// ListAdminExternalProducts handles GET /api/v1/admin/external-products.
func (h *PromotionHandler) ListAdminExternalProducts(c *gin.Context) {
	ctx := c.Request.Context()

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	statusFilters, err := parseExternalProductReviewStatusFilters(c.Query("status"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var (
		items []*entity.ExternalProduct
		resp  AdminExternalProductListResponse
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		items, txErr = h.promotionService.ListExternalProductsForReview(ctx, tx, repository.ExternalProductAdminListFilters{
			ReviewStatuses: statusFilters,
			Limit:          limit,
			Offset:         offset,
		})
		if txErr != nil {
			return txErr
		}
		resp.Items = make([]AdminExternalProductResponse, 0, len(items))
		for _, item := range items {
			itemResp, itemErr := h.adminExternalProductResponseWithRelations(ctx, tx, item, false)
			if itemErr != nil {
				return itemErr
			}
			resp.Items = append(resp.Items, itemResp)
		}
		resp.Count = len(resp.Items)
		resp.Page = page
		resp.Limit = limit
		return nil
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to retrieve external products")
		return
	}

	response.Success(c, resp)
}

// GetAdminExternalProduct handles GET /api/v1/admin/external-products/:id.
func (h *PromotionHandler) GetAdminExternalProduct(c *gin.Context) {
	ctx := c.Request.Context()

	externalProductID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid external product ID")
		return
	}

	var (
		product *entity.ExternalProduct
		resp    AdminExternalProductResponse
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		product, txErr = h.promotionService.GetExternalProduct(ctx, tx, externalProductID)
		if txErr != nil {
			return txErr
		}
		if product == nil {
			return &promotionApp.ExternalProductNotFoundError{ExternalProductID: externalProductID}
		}
		resp, txErr = h.adminExternalProductResponseWithRelations(ctx, tx, product, true)
		return txErr
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to process external product")
		return
	}

	response.Success(c, resp)
}

// ListAdminExternalProductReviews handles GET /api/v1/admin/external-products/:id/reviews.
func (h *PromotionHandler) ListAdminExternalProductReviews(c *gin.Context) {
	ctx := c.Request.Context()

	externalProductID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid external product ID")
		return
	}

	var items []*entity.ExternalProductReviewHistory
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		product, txErr := h.promotionService.GetExternalProduct(ctx, tx, externalProductID)
		if txErr != nil {
			return txErr
		}
		if product == nil {
			return &promotionApp.ExternalProductNotFoundError{ExternalProductID: externalProductID}
		}
		items, txErr = h.promotionService.ListExternalProductReviewHistory(ctx, tx, externalProductID)
		return txErr
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to retrieve external product review history")
		return
	}

	resp := make([]ExternalProductReviewHistoryResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, externalProductReviewHistoryToResponse(item))
	}

	response.Success(c, ExternalProductReviewHistoryListResponse{
		Items: resp,
		Count: len(resp),
	})
}

// ApproveExternalProduct handles POST /api/v1/admin/external-products/:id/approve.
func (h *PromotionHandler) ApproveExternalProduct(c *gin.Context) {
	h.handleAdminExternalProductDecision(c, "approve", func(ctx context.Context, tx db.Tx, adminID, externalProductID uuid.UUID, reason *string) (*entity.ExternalProduct, error) {
		return h.promotionService.ApproveExternalProduct(ctx, tx, promotionApp.AdminExternalProductReviewInput{
			AdminID:           adminID,
			ExternalProductID: externalProductID,
			Reason:            reason,
		})
	})
}

// RejectExternalProduct handles POST /api/v1/admin/external-products/:id/reject.
func (h *PromotionHandler) RejectExternalProduct(c *gin.Context) {
	h.handleAdminExternalProductDecision(c, "reject", func(ctx context.Context, tx db.Tx, adminID, externalProductID uuid.UUID, reason *string) (*entity.ExternalProduct, error) {
		return h.promotionService.RejectExternalProduct(ctx, tx, promotionApp.AdminExternalProductReviewInput{
			AdminID:           adminID,
			ExternalProductID: externalProductID,
			Reason:            reason,
		})
	})
}

// RequestChangesExternalProduct handles POST /api/v1/admin/external-products/:id/request-changes.
func (h *PromotionHandler) RequestChangesExternalProduct(c *gin.Context) {
	h.handleAdminExternalProductDecision(c, "request_changes", func(ctx context.Context, tx db.Tx, adminID, externalProductID uuid.UUID, reason *string) (*entity.ExternalProduct, error) {
		return h.promotionService.RequestChangesExternalProduct(ctx, tx, promotionApp.AdminExternalProductReviewInput{
			AdminID:           adminID,
			ExternalProductID: externalProductID,
			Reason:            reason,
		})
	})
}

// HideExternalProduct handles POST /api/v1/admin/external-products/:id/hide.
func (h *PromotionHandler) HideExternalProduct(c *gin.Context) {
	h.handleAdminExternalProductDecision(c, "hide", func(ctx context.Context, tx db.Tx, adminID, externalProductID uuid.UUID, reason *string) (*entity.ExternalProduct, error) {
		return h.promotionService.HideExternalProduct(ctx, tx, promotionApp.AdminExternalProductReviewInput{
			AdminID:           adminID,
			ExternalProductID: externalProductID,
			Reason:            reason,
		})
	})
}

func (h *PromotionHandler) handleAdminExternalProductDecision(
	c *gin.Context,
	action string,
	apply func(context.Context, db.Tx, uuid.UUID, uuid.UUID, *string) (*entity.ExternalProduct, error),
) {
	ctx := c.Request.Context()

	adminID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	externalProductID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid external product ID")
		return
	}

	var req struct {
		Note   *string `json:"note,omitempty"`
		Reason *string `json:"reason,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		response.BadRequest(c, err.Error())
		return
	}

	reason := req.Reason
	if action == "approve" {
		reason = req.Note
	}

	var (
		product *entity.ExternalProduct
		resp    AdminExternalProductResponse
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		product, txErr = apply(ctx, tx, adminID, externalProductID, reason)
		if txErr != nil {
			return txErr
		}
		resp, txErr = h.adminExternalProductResponseWithRelations(ctx, tx, product, true)
		return txErr
	})
	if handleExternalProductError(c, err) {
		return
	}
	if err != nil {
		response.InternalServerError(c, "Failed to process external product review")
		return
	}

	if h.adminAuditLogger != nil {
		h.adminAuditLogger.LogSafe(ctx, adminID, "external_product_"+action, "external_product", externalProductID, map[string]interface{}{
			"reason": stringOrNil(reason),
		})
	}

	response.Success(c, resp)
}

func handleExternalProductError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	switch e := err.(type) {
	case *promotionApp.ExternalProductNotFoundError:
		response.NotFound(c, e.Error())
		return true
	case *promotionApp.ExternalProductNotOwnedError:
		response.Forbidden(c, e.Error())
		return true
	case *promotionApp.ExternalProductInvalidTransitionError:
		response.Error(c, http.StatusUnprocessableEntity, response.ErrCodeInvalidInput, e.Error())
		return true
	case *promotionApp.ExternalProductMediaNotFoundError:
		response.NotFound(c, e.Error())
		return true
	}

	if isExternalProductValidationError(err) {
		response.BadRequest(c, err.Error())
		return true
	}

	return false
}

func isExternalProductValidationError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "title is required"),
		strings.Contains(msg, "title must be 200 characters or fewer"),
		strings.Contains(msg, "external_url is required"),
		strings.Contains(msg, "external_url must use http or https"),
		strings.Contains(msg, "external_url must include a host"),
		strings.Contains(msg, "invalid external_url"),
		strings.Contains(msg, "invalid media type"),
		strings.Contains(msg, "storage_key is required"),
		strings.Contains(msg, "url is required"),
		strings.Contains(msg, "reason is required"):
		return true
	default:
		return false
	}
}

func (h *PromotionHandler) externalProductResponseWithMedia(ctx context.Context, tx db.Tx, product *entity.ExternalProduct) (ExternalProductResponse, error) {
	if product == nil {
		return ExternalProductResponse{}, nil
	}

	media, err := h.promotionService.ListExternalProductMedia(ctx, tx, product.ID)
	if err != nil {
		return ExternalProductResponse{}, err
	}

	publicVisible, err := h.externalProductPublicVisible(ctx, tx, product)
	if err != nil {
		return ExternalProductResponse{}, err
	}

	return externalProductToResponse(product, media, publicVisible), nil
}

func (h *PromotionHandler) externalProductPublicVisible(ctx context.Context, tx db.Tx, product *entity.ExternalProduct) (bool, error) {
	if product == nil {
		return false, nil
	}
	if product.ReviewStatus != entity.ExternalProductReviewStatusApproved {
		return false, nil
	}
	return h.promotionService.IsTargetPromotedInTx(ctx, tx, entity.TargetTypeExternalProduct, product.ID)
}

func externalProductToResponse(product *entity.ExternalProduct, media []*entity.ExternalProductMedia, publicVisible bool) ExternalProductResponse {
	if product == nil {
		return ExternalProductResponse{}
	}

	respMedia := make([]ExternalProductMediaResponse, 0, len(media))
	for _, item := range media {
		respMedia = append(respMedia, externalProductMediaToResponse(item))
	}

	return ExternalProductResponse{
		ID:                    product.ID,
		OwnerUserID:           product.OwnerUserID,
		Title:                 product.Title,
		Description:           product.Description,
		ExternalURL:           product.ExternalURL,
		NormalizedExternalURL: product.NormalizedExternalURL,
		ReviewStatus:          string(product.ReviewStatus),
		RejectionReason:       product.RejectionReason,
		UnsafeURLFlag:         product.UnsafeURLFlag,
		SubmittedAt:           formatExternalProductTimePtr(product.SubmittedAt),
		ApprovedAt:            formatExternalProductTimePtr(product.ApprovedAt),
		RejectedAt:            formatExternalProductTimePtr(product.RejectedAt),
		HiddenAt:              formatExternalProductTimePtr(product.HiddenAt),
		LastReviewedBy:        product.LastReviewedBy,
		CreatedAt:             product.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             product.UpdatedAt.Format(time.RFC3339),
		Media:                 respMedia,
		CanEdit:               product.CanMaterialEdit(),
		CanSubmit:             product.CanSubmit(),
		CanResubmit:           product.CanResubmit(),
		PublicVisible:         publicVisible,
	}
}

func externalProductMediaToResponse(media *entity.ExternalProductMedia) ExternalProductMediaResponse {
	if media == nil {
		return ExternalProductMediaResponse{}
	}

	return ExternalProductMediaResponse{
		ID:                media.ID,
		ExternalProductID: media.ExternalProductID,
		MediaType:         string(media.MediaType),
		StorageKey:        media.StorageKey,
		URL:               media.URL,
		ThumbnailURL:      media.ThumbnailURL,
		SortOrder:         media.SortOrder,
		Metadata:          media.Metadata,
		CreatedAt:         media.CreatedAt.Format(time.RFC3339),
	}
}

func (h *PromotionHandler) adminExternalProductResponseWithRelations(ctx context.Context, tx db.Tx, product *entity.ExternalProduct, includeHistory bool) (AdminExternalProductResponse, error) {
	publicVisible := false
	if product != nil {
		var err error
		publicVisible, err = h.externalProductPublicVisible(ctx, tx, product)
		if err != nil {
			return AdminExternalProductResponse{}, err
		}
	}
	base := externalProductToResponse(product, nil, publicVisible)

	media, err := h.promotionService.ListExternalProductMedia(ctx, tx, product.ID)
	if err != nil {
		return AdminExternalProductResponse{}, err
	}
	base.Media = make([]ExternalProductMediaResponse, 0, len(media))
	for _, item := range media {
		base.Media = append(base.Media, externalProductMediaToResponse(item))
	}

	resp := AdminExternalProductResponse{
		ExternalProductResponse: base,
		CanApprove:              product.ReviewStatus.CanApprove(),
		CanReject:               product.ReviewStatus.CanReject(),
		CanHide:                 product.ReviewStatus.CanHide(),
	}

	if includeHistory {
		history, err := h.promotionService.ListExternalProductReviewHistory(ctx, tx, product.ID)
		if err != nil {
			return AdminExternalProductResponse{}, err
		}
		resp.ReviewHistory = make([]ExternalProductReviewHistoryResponse, 0, len(history))
		for _, item := range history {
			resp.ReviewHistory = append(resp.ReviewHistory, externalProductReviewHistoryToResponse(item))
		}
	}

	return resp, nil
}

func externalProductReviewHistoryToResponse(history *entity.ExternalProductReviewHistory) ExternalProductReviewHistoryResponse {
	if history == nil {
		return ExternalProductReviewHistoryResponse{}
	}

	var fromStatus *string
	if history.FromStatus != nil {
		value := history.FromStatus.String()
		fromStatus = &value
	}

	return ExternalProductReviewHistoryResponse{
		ID:                history.ID,
		ExternalProductID: history.ExternalProductID,
		ActorAdminID:      history.ActorAdminID,
		ActorUserID:       history.ActorUserID,
		FromStatus:        fromStatus,
		ToStatus:          history.ToStatus.String(),
		Reason:            history.Reason,
		CreatedAt:         history.CreatedAt.Format(time.RFC3339),
	}
}

func parseExternalProductReviewStatusFilters(raw string) ([]entity.ExternalProductReviewStatus, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	statuses := make([]entity.ExternalProductReviewStatus, 0, len(parts))
	for _, part := range parts {
		status := entity.ExternalProductReviewStatus(strings.TrimSpace(part))
		if !status.IsValid() || status == entity.ExternalProductReviewStatusDraft {
			return nil, fmt.Errorf("invalid review status filter: %s", strings.TrimSpace(part))
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func stringOrNil(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func formatExternalProductTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
