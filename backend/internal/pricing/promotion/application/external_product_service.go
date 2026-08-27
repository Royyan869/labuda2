package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	promotionRepo "github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
)

// CreateExternalProductDraftInput carries the data required to create a draft.
type CreateExternalProductDraftInput struct {
	UserID      uuid.UUID
	Title       string
	Description *string
	ExternalURL string
}

// UpdateExternalProductInput carries the data required to update an owned product.
type UpdateExternalProductInput struct {
	UserID            uuid.UUID
	ExternalProductID uuid.UUID
	Update            entity.ExternalProductUpdateInput
}

// AttachExternalProductMediaInput carries the data required to attach media.
type AttachExternalProductMediaInput struct {
	UserID            uuid.UUID
	ExternalProductID uuid.UUID
	MediaType         entity.ExternalProductMediaType
	StorageKey        string
	URL               string
	ThumbnailURL      *string
	SortOrder         int
	Metadata          json.RawMessage
}

// AdminExternalProductReviewInput carries the data required for admin review actions.
type AdminExternalProductReviewInput struct {
	AdminID           uuid.UUID
	ExternalProductID uuid.UUID
	Reason            *string
}

// ExternalProductNotFoundError is returned when a product cannot be found.
type ExternalProductNotFoundError struct {
	ExternalProductID uuid.UUID
}

func (e *ExternalProductNotFoundError) Error() string {
	return fmt.Sprintf("external product not found: %s", e.ExternalProductID)
}

// ExternalProductNotOwnedError is returned when the caller does not own the product.
type ExternalProductNotOwnedError struct {
	ExternalProductID uuid.UUID
	UserID            uuid.UUID
}

func (e *ExternalProductNotOwnedError) Error() string {
	return fmt.Sprintf("external product %s is not owned by user %s", e.ExternalProductID, e.UserID)
}

// ExternalProductInvalidTransitionError is returned when a lifecycle action is not allowed.
type ExternalProductInvalidTransitionError struct {
	ExternalProductID uuid.UUID
	Action            string
	Status            entity.ExternalProductReviewStatus
}

func (e *ExternalProductInvalidTransitionError) Error() string {
	return fmt.Sprintf("external product %s cannot %s while in state %s", e.ExternalProductID, e.Action, e.Status)
}

// ExternalProductMediaNotFoundError is returned when a media item cannot be found.
type ExternalProductMediaNotFoundError struct {
	ExternalProductID uuid.UUID
	MediaID           uuid.UUID
}

func (e *ExternalProductMediaNotFoundError) Error() string {
	return fmt.Sprintf("external product media not found: %s", e.MediaID)
}

// CreateExternalProductDraft validates and persists a new external product draft.
func (s *PromotionService) CreateExternalProductDraft(
	ctx context.Context,
	tx db.Tx,
	input CreateExternalProductDraftInput,
) (*entity.ExternalProduct, error) {
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get database time: %w", err)
	}

	product, err := entity.NewExternalProductDraft(
		input.UserID,
		input.Title,
		input.Description,
		input.ExternalURL,
		dbTime,
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.CreateDraft(ctx, tx, product); err != nil {
		return nil, err
	}

	return product, nil
}

// UpdateExternalProduct updates an owned product and applies lifecycle rules.
func (s *PromotionService) UpdateExternalProduct(
	ctx context.Context,
	tx db.Tx,
	input UpdateExternalProductInput,
) (*entity.ExternalProduct, error) {
	product, err := s.repo.GetByID(ctx, tx, input.ExternalProductID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, &ExternalProductNotFoundError{ExternalProductID: input.ExternalProductID}
	}
	if product.OwnerUserID != input.UserID {
		return nil, &ExternalProductNotOwnedError{
			ExternalProductID: input.ExternalProductID,
			UserID:            input.UserID,
		}
	}
	if !product.CanMaterialEdit() {
		return nil, &ExternalProductInvalidTransitionError{
			ExternalProductID: input.ExternalProductID,
			Action:            "update",
			Status:            product.ReviewStatus,
		}
	}

	updated, err := s.repo.UpdateOwned(ctx, tx, input.UserID, input.ExternalProductID, input.Update)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// SubmitExternalProduct submits an owned draft for review.
func (s *PromotionService) SubmitExternalProduct(
	ctx context.Context,
	tx db.Tx,
	userID, externalProductID uuid.UUID,
) (*entity.ExternalProduct, error) {
	product, err := s.repo.GetByID(ctx, tx, externalProductID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, &ExternalProductNotFoundError{ExternalProductID: externalProductID}
	}
	if product.OwnerUserID != userID {
		return nil, &ExternalProductNotOwnedError{
			ExternalProductID: externalProductID,
			UserID:            userID,
		}
	}
	if !product.CanSubmit() {
		return nil, &ExternalProductInvalidTransitionError{
			ExternalProductID: externalProductID,
			Action:            "submit",
			Status:            product.ReviewStatus,
		}
	}

	return s.repo.SubmitOwned(ctx, tx, userID, externalProductID)
}

// ResubmitExternalProduct resubmits an owned rejected product for review.
func (s *PromotionService) ResubmitExternalProduct(
	ctx context.Context,
	tx db.Tx,
	userID, externalProductID uuid.UUID,
) (*entity.ExternalProduct, error) {
	product, err := s.repo.GetByID(ctx, tx, externalProductID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, &ExternalProductNotFoundError{ExternalProductID: externalProductID}
	}
	if product.OwnerUserID != userID {
		return nil, &ExternalProductNotOwnedError{
			ExternalProductID: externalProductID,
			UserID:            userID,
		}
	}
	if !product.CanResubmit() {
		return nil, &ExternalProductInvalidTransitionError{
			ExternalProductID: externalProductID,
			Action:            "resubmit",
			Status:            product.ReviewStatus,
		}
	}

	return s.repo.ResubmitOwned(ctx, tx, userID, externalProductID)
}

// GetExternalProduct retrieves a product without owner scoping.
func (s *PromotionService) GetExternalProduct(
	ctx context.Context,
	tx db.Tx,
	externalProductID uuid.UUID,
) (*entity.ExternalProduct, error) {
	return s.repo.GetByID(ctx, tx, externalProductID)
}

// GetOwnedExternalProduct retrieves a product and enforces ownership.
func (s *PromotionService) GetOwnedExternalProduct(
	ctx context.Context,
	tx db.Tx,
	userID, externalProductID uuid.UUID,
) (*entity.ExternalProduct, error) {
	product, err := s.repo.GetByID(ctx, tx, externalProductID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, &ExternalProductNotFoundError{ExternalProductID: externalProductID}
	}
	if product.OwnerUserID != userID {
		return nil, &ExternalProductNotOwnedError{
			ExternalProductID: externalProductID,
			UserID:            userID,
		}
	}
	return product, nil
}

// ListExternalProducts retrieves the current user's external products.
func (s *PromotionService) ListExternalProducts(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	filters promotionRepo.ExternalProductListFilters,
) ([]*entity.ExternalProduct, error) {
	return s.repo.ListOwned(ctx, tx, userID, filters)
}

// ListExternalProductMedia retrieves media for a product.
func (s *PromotionService) ListExternalProductMedia(
	ctx context.Context,
	tx db.Tx,
	externalProductID uuid.UUID,
) ([]*entity.ExternalProductMedia, error) {
	return s.repo.ListMedia(ctx, tx, externalProductID)
}

// ApproveExternalProduct approves a pending external product.
func (s *PromotionService) ApproveExternalProduct(
	ctx context.Context,
	tx db.Tx,
	input AdminExternalProductReviewInput,
) (*entity.ExternalProduct, error) {
	return s.applyAdminReview(ctx, tx, input, "approve", func(status entity.ExternalProductReviewStatus) bool {
		return status.CanApprove()
	}, func(product *entity.ExternalProduct, adminID uuid.UUID, dbTime time.Time) error {
		return product.ApproveByAdmin(adminID, dbTime)
	})
}

// RejectExternalProduct rejects a pending external product.
func (s *PromotionService) RejectExternalProduct(
	ctx context.Context,
	tx db.Tx,
	input AdminExternalProductReviewInput,
) (*entity.ExternalProduct, error) {
	return s.applyAdminReview(ctx, tx, input, "reject", func(status entity.ExternalProductReviewStatus) bool {
		return status.CanReject()
	}, func(product *entity.ExternalProduct, adminID uuid.UUID, dbTime time.Time) error {
		reason := ""
		if input.Reason != nil {
			reason = *input.Reason
		}
		return product.RejectByAdmin(adminID, reason, dbTime)
	})
}

// RequestChangesExternalProduct records an admin request-changes decision.
// This sets the distinct review_status = 'request_changes' so the mobile can show
// a specific UI ("Perlu Perbaikan") distinct from a hard rejection.
func (s *PromotionService) RequestChangesExternalProduct(
	ctx context.Context,
	tx db.Tx,
	input AdminExternalProductReviewInput,
) (*entity.ExternalProduct, error) {
	return s.applyAdminReview(ctx, tx, input, "request_changes", func(status entity.ExternalProductReviewStatus) bool {
		return status.CanRequestChanges()
	}, func(product *entity.ExternalProduct, adminID uuid.UUID, dbTime time.Time) error {
		reason := ""
		if input.Reason != nil {
			reason = *input.Reason
		}
		return product.RequestChangesByAdmin(adminID, reason, dbTime)
	})
}

// HideExternalProduct hides a reviewed external product.
func (s *PromotionService) HideExternalProduct(
	ctx context.Context,
	tx db.Tx,
	input AdminExternalProductReviewInput,
) (*entity.ExternalProduct, error) {
	return s.applyAdminReview(ctx, tx, input, "hide", func(status entity.ExternalProductReviewStatus) bool {
		return status.CanHide()
	}, func(product *entity.ExternalProduct, adminID uuid.UUID, dbTime time.Time) error {
		reason := ""
		if input.Reason != nil {
			reason = *input.Reason
		}
		return product.HideByAdmin(adminID, reason, dbTime)
	})
}

func (s *PromotionService) applyAdminReview(
	ctx context.Context,
	tx db.Tx,
	input AdminExternalProductReviewInput,
	action string,
	allowed func(entity.ExternalProductReviewStatus) bool,
	mutate func(*entity.ExternalProduct, uuid.UUID, time.Time) error,
) (*entity.ExternalProduct, error) {
	product, err := s.repo.GetByID(ctx, tx, input.ExternalProductID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, &ExternalProductNotFoundError{ExternalProductID: input.ExternalProductID}
	}
	if input.AdminID == uuid.Nil {
		return nil, fmt.Errorf("admin_id is required")
	}
	if allowed != nil && !allowed(product.ReviewStatus) {
		return nil, &ExternalProductInvalidTransitionError{
			ExternalProductID: input.ExternalProductID,
			Action:            action,
			Status:            product.ReviewStatus,
		}
	}

	previousStatus := product.ReviewStatus
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get database time: %w", err)
	}
	if err := mutate(product, input.AdminID, dbTime); err != nil {
		return nil, err
	}

	history, err := entity.NewExternalProductReviewHistory(
		product.ID,
		&previousStatus,
		product.ReviewStatus,
		input.Reason,
		&input.AdminID,
		nil,
		dbTime,
	)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateByID(ctx, tx, product); err != nil {
		return nil, err
	}
	if err := s.repo.AppendReviewHistory(ctx, tx, history); err != nil {
		return nil, err
	}

	if eventType := externalProductReviewEventType(action); eventType != "" {
		if err := s.emitExternalProductReviewEventTx(ctx, tx, eventType, product, input.Reason, input.AdminID); err != nil {
			return nil, err
		}
	}
	return product, nil
}

// ListExternalProductsForReview retrieves products for the admin review queue.
func (s *PromotionService) ListExternalProductsForReview(
	ctx context.Context,
	tx db.Tx,
	filters promotionRepo.ExternalProductAdminListFilters,
) ([]*entity.ExternalProduct, error) {
	return s.repo.ListForReview(ctx, tx, filters)
}

// ListExternalProductReviewHistory retrieves a product's review history.
func (s *PromotionService) ListExternalProductReviewHistory(
	ctx context.Context,
	tx db.Tx,
	externalProductID uuid.UUID,
) ([]*entity.ExternalProductReviewHistory, error) {
	return s.repo.ListReviewHistory(ctx, tx, externalProductID)
}

// AttachExternalProductMedia attaches a media asset to an owned external product.
func (s *PromotionService) AttachExternalProductMedia(
	ctx context.Context,
	tx db.Tx,
	input AttachExternalProductMediaInput,
) error {
	product, err := s.repo.GetByID(ctx, tx, input.ExternalProductID)
	if err != nil {
		return err
	}
	if product == nil {
		return &ExternalProductNotFoundError{ExternalProductID: input.ExternalProductID}
	}
	if product.OwnerUserID != input.UserID {
		return &ExternalProductNotOwnedError{
			ExternalProductID: input.ExternalProductID,
			UserID:            input.UserID,
		}
	}
	if !product.CanMaterialEdit() {
		return &ExternalProductInvalidTransitionError{
			ExternalProductID: input.ExternalProductID,
			Action:            "attach media",
			Status:            product.ReviewStatus,
		}
	}

	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to get database time: %w", err)
	}

	media, err := entity.NewExternalProductMedia(
		input.ExternalProductID,
		input.MediaType,
		input.StorageKey,
		input.URL,
		input.ThumbnailURL,
		input.SortOrder,
		input.Metadata,
		dbTime,
	)
	if err != nil {
		return err
	}

	if err := s.repo.AddMedia(ctx, tx, media); err != nil {
		return err
	}

	if product.ReviewStatus == entity.ExternalProductReviewStatusApproved {
		if _, updateErr := s.repo.UpdateOwned(ctx, tx, input.UserID, input.ExternalProductID, entity.ExternalProductUpdateInput{}); updateErr != nil {
			return updateErr
		}
	}

	return nil
}

// DeleteExternalProductMedia soft-deletes a media asset from an owned external product.
func (s *PromotionService) DeleteExternalProductMedia(
	ctx context.Context,
	tx db.Tx,
	userID, externalProductID, mediaID uuid.UUID,
) error {
	product, err := s.repo.GetByID(ctx, tx, externalProductID)
	if err != nil {
		return err
	}
	if product == nil {
		return &ExternalProductNotFoundError{ExternalProductID: externalProductID}
	}
	if product.OwnerUserID != userID {
		return &ExternalProductNotOwnedError{
			ExternalProductID: externalProductID,
			UserID:            userID,
		}
	}
	if !product.CanMaterialEdit() {
		return &ExternalProductInvalidTransitionError{
			ExternalProductID: externalProductID,
			Action:            "delete media",
			Status:            product.ReviewStatus,
		}
	}

	mediaItems, err := s.repo.ListMedia(ctx, tx, externalProductID)
	if err != nil {
		return err
	}
	found := false
	for _, item := range mediaItems {
		if item.ID == mediaID {
			found = true
			break
		}
	}
	if !found {
		return &ExternalProductMediaNotFoundError{
			ExternalProductID: externalProductID,
			MediaID:           mediaID,
		}
	}

	if err := s.repo.SoftDeleteMedia(ctx, tx, userID, externalProductID, mediaID); err != nil {
		return err
	}

	if product.ReviewStatus == entity.ExternalProductReviewStatusApproved {
		if _, err := s.repo.UpdateOwned(ctx, tx, userID, externalProductID, entity.ExternalProductUpdateInput{}); err != nil {
			return err
		}
	}

	return nil
}
