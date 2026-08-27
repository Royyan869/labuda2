package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	promotionRepo "github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
)

type ExternalProductListFilters = promotionRepo.ExternalProductListFilters
type ExternalProductAdminListFilters = promotionRepo.ExternalProductAdminListFilters

// CreateDraft persists a new external product draft.
func (r *PromotionRepositoryImpl) CreateDraft(ctx context.Context, tx db.Tx, product *entity.ExternalProduct) error {
	if product == nil {
		return fmt.Errorf("external product is nil")
	}

	now := product.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	product.CreatedAt = now
	product.UpdatedAt = now
	if product.ID == uuid.Nil {
		product.ID = uuid.New()
	}
	if product.ReviewStatus == "" {
		product.ReviewStatus = entity.ExternalProductReviewStatusDraft
	}
	if product.NormalizedExternalURL == "" && product.ExternalURL != "" {
		normalized, err := entity.NormalizeExternalURL(product.ExternalURL)
		if err != nil {
			return err
		}
		product.NormalizedExternalURL = normalized
	}

	query := `
		INSERT INTO external_products (
			id, owner_user_id, title, description, external_url,
			normalized_external_url, review_status, rejection_reason,
			unsafe_url_flag, submitted_at, approved_at, rejected_at,
			hidden_at, last_reviewed_by, created_at, updated_at, deleted_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9, $10, $11, $12,
			$13, $14, $15, $16, $17
		)
	`

	_, err := tx.Exec(ctx, query,
		product.ID,
		product.OwnerUserID,
		product.Title,
		product.Description,
		product.ExternalURL,
		product.NormalizedExternalURL,
		string(product.ReviewStatus),
		product.RejectionReason,
		product.UnsafeURLFlag,
		product.SubmittedAt,
		product.ApprovedAt,
		product.RejectedAt,
		product.HiddenAt,
		product.LastReviewedBy,
		product.CreatedAt,
		product.UpdatedAt,
		product.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create external product draft: %w", err)
	}
	return nil
}

// UpdateOwned updates an owned external product.
func (r *PromotionRepositoryImpl) UpdateOwned(
	ctx context.Context,
	tx db.Tx,
	ownerUserID, externalProductID uuid.UUID,
	input entity.ExternalProductUpdateInput,
) (*entity.ExternalProduct, error) {
	dbTime, err := r.GetDBTime(ctx, tx)
	if err != nil {
		return nil, err
	}

	product, err := r.getOwnedExternalProductForUpdate(ctx, tx, ownerUserID, externalProductID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, fmt.Errorf("external product not found or not owned: %s", externalProductID)
	}

	if !product.CanMaterialEdit() {
		return nil, fmt.Errorf("external product is not editable in state %s", product.ReviewStatus)
	}

	previousStatus := product.ReviewStatus
	if err := product.ApplyOwnerUpdate(input, dbTime); err != nil {
		return nil, err
	}

	if previousStatus == entity.ExternalProductReviewStatusApproved {
		reason := "material_edit"
		history, histErr := entity.NewExternalProductReviewHistory(
			product.ID,
			&previousStatus,
			product.ReviewStatus,
			&reason,
			nil,
			&ownerUserID,
			dbTime,
		)
		if histErr != nil {
			return nil, histErr
		}
		if err := r.AppendReviewHistory(ctx, tx, history); err != nil {
			return nil, err
		}
	}

	if err := r.updateExternalProduct(ctx, tx, product); err != nil {
		return nil, err
	}

	return product, nil
}

// UpdateByID updates an external product by ID without owner scoping.
func (r *PromotionRepositoryImpl) UpdateByID(
	ctx context.Context,
	tx db.Tx,
	product *entity.ExternalProduct,
) error {
	if product == nil {
		return fmt.Errorf("external product is nil")
	}

	query := `
		UPDATE external_products
		SET title = $2,
		    description = $3,
		    external_url = $4,
		    normalized_external_url = $5,
		    review_status = $6,
		    rejection_reason = $7,
		    unsafe_url_flag = $8,
		    submitted_at = $9,
		    approved_at = $10,
		    rejected_at = $11,
		    hidden_at = $12,
		    last_reviewed_by = $13,
		    updated_at = $14
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	result, err := tx.Exec(ctx, query,
		product.ID,
		product.Title,
		product.Description,
		product.ExternalURL,
		product.NormalizedExternalURL,
		string(product.ReviewStatus),
		product.RejectionReason,
		product.UnsafeURLFlag,
		product.SubmittedAt,
		product.ApprovedAt,
		product.RejectedAt,
		product.HiddenAt,
		product.LastReviewedBy,
		product.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update external product: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("external product not found: %s", product.ID)
	}
	return nil
}

// SubmitOwned moves a draft into pending review.
func (r *PromotionRepositoryImpl) SubmitOwned(
	ctx context.Context,
	tx db.Tx,
	ownerUserID, externalProductID uuid.UUID,
) (*entity.ExternalProduct, error) {
	dbTime, err := r.GetDBTime(ctx, tx)
	if err != nil {
		return nil, err
	}

	product, err := r.getOwnedExternalProductForUpdate(ctx, tx, ownerUserID, externalProductID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, fmt.Errorf("external product not found or not owned: %s", externalProductID)
	}
	if !product.CanSubmit() {
		return nil, fmt.Errorf("external product is not in a submittable state")
	}

	previousStatus := product.ReviewStatus
	if err := product.Submit(dbTime); err != nil {
		return nil, err
	}

	reason := "submit"
	history, histErr := entity.NewExternalProductReviewHistory(
		product.ID,
		&previousStatus,
		product.ReviewStatus,
		&reason,
		nil,
		&ownerUserID,
		dbTime,
	)
	if histErr != nil {
		return nil, histErr
	}
	if err := r.AppendReviewHistory(ctx, tx, history); err != nil {
		return nil, err
	}

	if err := r.updateExternalProduct(ctx, tx, product); err != nil {
		return nil, err
	}

	return product, nil
}

// ResubmitOwned moves a rejected external product back into pending review.
func (r *PromotionRepositoryImpl) ResubmitOwned(
	ctx context.Context,
	tx db.Tx,
	ownerUserID, externalProductID uuid.UUID,
) (*entity.ExternalProduct, error) {
	dbTime, err := r.GetDBTime(ctx, tx)
	if err != nil {
		return nil, err
	}

	product, err := r.getOwnedExternalProductForUpdate(ctx, tx, ownerUserID, externalProductID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, fmt.Errorf("external product not found or not owned: %s", externalProductID)
	}
	if !product.CanResubmit() {
		return nil, fmt.Errorf("external product is not in a resubmittable state")
	}

	previousStatus := product.ReviewStatus
	if err := product.Resubmit(dbTime); err != nil {
		return nil, err
	}

	reason := "resubmit"
	history, histErr := entity.NewExternalProductReviewHistory(
		product.ID,
		&previousStatus,
		product.ReviewStatus,
		&reason,
		nil,
		&ownerUserID,
		dbTime,
	)
	if histErr != nil {
		return nil, histErr
	}
	if err := r.AppendReviewHistory(ctx, tx, history); err != nil {
		return nil, err
	}

	if err := r.updateExternalProduct(ctx, tx, product); err != nil {
		return nil, err
	}

	return product, nil
}

// GetOwnedByID retrieves an owned external product by ID.
func (r *PromotionRepositoryImpl) GetOwnedByID(
	ctx context.Context,
	tx db.Tx,
	ownerUserID, externalProductID uuid.UUID,
) (*entity.ExternalProduct, error) {
	query := `
		SELECT id, owner_user_id, title, description, external_url,
		       normalized_external_url, review_status, rejection_reason,
		       unsafe_url_flag, submitted_at, approved_at, rejected_at,
		       hidden_at, last_reviewed_by, created_at, updated_at, deleted_at
		FROM external_products
		WHERE id = $1
		  AND owner_user_id = $2
		  AND deleted_at IS NULL
	`

	return r.scanExternalProduct(ctx, tx.QueryRow(ctx, query, externalProductID, ownerUserID))
}

// ListOwned retrieves owned external products with optional filtering.
func (r *PromotionRepositoryImpl) ListOwned(
	ctx context.Context,
	tx db.Tx,
	ownerUserID uuid.UUID,
	filters ExternalProductListFilters,
) ([]*entity.ExternalProduct, error) {
	query := strings.Builder{}
	query.WriteString(`
		SELECT id, owner_user_id, title, description, external_url,
		       normalized_external_url, review_status, rejection_reason,
		       unsafe_url_flag, submitted_at, approved_at, rejected_at,
		       hidden_at, last_reviewed_by, created_at, updated_at, deleted_at
		FROM external_products
		WHERE owner_user_id = $1
	`)
	args := []any{ownerUserID}
	argIdx := 2

	if !filters.IncludeDeleted {
		query.WriteString(" AND deleted_at IS NULL")
	}
	if filters.ReviewStatus != nil {
		query.WriteString(fmt.Sprintf(" AND review_status = $%d", argIdx))
		args = append(args, string(*filters.ReviewStatus))
		argIdx++
	}
	query.WriteString(" ORDER BY created_at DESC")
	if filters.Limit > 0 {
		query.WriteString(fmt.Sprintf(" LIMIT $%d", argIdx))
		args = append(args, filters.Limit)
		argIdx++
	}
	if filters.Offset > 0 {
		query.WriteString(fmt.Sprintf(" OFFSET $%d", argIdx))
		args = append(args, filters.Offset)
	}

	rows, err := tx.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list external products: %w", err)
	}
	defer rows.Close()

	var products []*entity.ExternalProduct
	for rows.Next() {
		product, scanErr := r.scanExternalProductFromRows(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		products = append(products, product)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating external products: %w", rows.Err())
	}
	return products, nil
}

// ListForReview retrieves external products for the admin review queue.
func (r *PromotionRepositoryImpl) ListForReview(
	ctx context.Context,
	tx db.Tx,
	filters ExternalProductAdminListFilters,
) ([]*entity.ExternalProduct, error) {
	query := strings.Builder{}
	query.WriteString(`
		SELECT id, owner_user_id, title, description, external_url,
		       normalized_external_url, review_status, rejection_reason,
		       unsafe_url_flag, submitted_at, approved_at, rejected_at,
		       hidden_at, last_reviewed_by, created_at, updated_at, deleted_at
		FROM external_products
		WHERE 1=1
	`)
	args := make([]any, 0)
	argIdx := 1

	if len(filters.ReviewStatuses) == 0 {
		filters.ReviewStatuses = []entity.ExternalProductReviewStatus{
			entity.ExternalProductReviewStatusPendingReview,
			entity.ExternalProductReviewStatusApproved,
			entity.ExternalProductReviewStatusRejected,
			entity.ExternalProductReviewStatusRequestChanges,
			entity.ExternalProductReviewStatusHidden,
		}
	}

	query.WriteString(fmt.Sprintf(" AND review_status IN ("))
	for i, status := range filters.ReviewStatuses {
		if i > 0 {
			query.WriteString(", ")
		}
		query.WriteString(fmt.Sprintf("$%d", argIdx))
		args = append(args, string(status))
		argIdx++
	}
	query.WriteString(")")

	if !filters.IncludeDeleted {
		query.WriteString(" AND deleted_at IS NULL")
	}
	query.WriteString(" ORDER BY updated_at DESC, created_at DESC")
	if filters.Limit > 0 {
		query.WriteString(fmt.Sprintf(" LIMIT $%d", argIdx))
		args = append(args, filters.Limit)
		argIdx++
	}
	if filters.Offset > 0 {
		query.WriteString(fmt.Sprintf(" OFFSET $%d", argIdx))
		args = append(args, filters.Offset)
	}

	rows, err := tx.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list external products for review: %w", err)
	}
	defer rows.Close()

	var products []*entity.ExternalProduct
	for rows.Next() {
		product, scanErr := r.scanExternalProductFromRows(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		products = append(products, product)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating external products for review: %w", rows.Err())
	}
	return products, nil
}

// GetByID retrieves an external product by ID.
func (r *PromotionRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, externalProductID uuid.UUID) (*entity.ExternalProduct, error) {
	query := `
		SELECT id, owner_user_id, title, description, external_url,
		       normalized_external_url, review_status, rejection_reason,
		       unsafe_url_flag, submitted_at, approved_at, rejected_at,
		       hidden_at, last_reviewed_by, created_at, updated_at, deleted_at
		FROM external_products
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	return r.scanExternalProduct(ctx, tx.QueryRow(ctx, query, externalProductID))
}

// AppendReviewHistory stores a lifecycle transition record.
func (r *PromotionRepositoryImpl) AppendReviewHistory(ctx context.Context, tx db.Tx, history *entity.ExternalProductReviewHistory) error {
	if history == nil {
		return fmt.Errorf("external product review history is nil")
	}

	query := `
		INSERT INTO external_product_review_history (
			id, external_product_id, actor_admin_id, actor_user_id,
			from_status, to_status, reason, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	var fromStatus *string
	if history.FromStatus != nil {
		from := history.FromStatus.String()
		fromStatus = &from
	}

	_, err := tx.Exec(ctx, query,
		history.ID,
		history.ExternalProductID,
		history.ActorAdminID,
		history.ActorUserID,
		fromStatus,
		string(history.ToStatus),
		history.Reason,
		history.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to append external product review history: %w", err)
	}
	return nil
}

// ListReviewHistory retrieves the review history for an external product.
func (r *PromotionRepositoryImpl) ListReviewHistory(
	ctx context.Context,
	tx db.Tx,
	externalProductID uuid.UUID,
) ([]*entity.ExternalProductReviewHistory, error) {
	query := `
		SELECT id, external_product_id, actor_admin_id, actor_user_id,
		       from_status, to_status, reason, created_at
		FROM external_product_review_history
		WHERE external_product_id = $1
		ORDER BY created_at ASC
	`

	rows, err := tx.Query(ctx, query, externalProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to list external product review history: %w", err)
	}
	defer rows.Close()

	var history []*entity.ExternalProductReviewHistory
	for rows.Next() {
		item, scanErr := r.scanExternalProductReviewHistoryFromRows(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		history = append(history, item)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating external product review history: %w", rows.Err())
	}
	return history, nil
}

// AddMedia attaches uploaded media to an external product.
func (r *PromotionRepositoryImpl) AddMedia(ctx context.Context, tx db.Tx, media *entity.ExternalProductMedia) error {
	if media == nil {
		return fmt.Errorf("external product media is nil")
	}
	if media.ID == uuid.Nil {
		media.ID = uuid.New()
	}
	if media.CreatedAt.IsZero() {
		media.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO external_product_media (
			id, external_product_id, media_type, storage_key, url,
			thumbnail_url, sort_order, metadata, created_at, deleted_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := tx.Exec(ctx, query,
		media.ID,
		media.ExternalProductID,
		string(media.MediaType),
		media.StorageKey,
		media.URL,
		media.ThumbnailURL,
		media.SortOrder,
		media.Metadata,
		media.CreatedAt,
		media.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to add external product media: %w", err)
	}
	return nil
}

// ListMedia lists uploaded media for an external product.
func (r *PromotionRepositoryImpl) ListMedia(ctx context.Context, tx db.Tx, externalProductID uuid.UUID) ([]*entity.ExternalProductMedia, error) {
	query := `
		SELECT id, external_product_id, media_type, storage_key, url,
		       thumbnail_url, sort_order, metadata, created_at, deleted_at
		FROM external_product_media
		WHERE external_product_id = $1
		  AND deleted_at IS NULL
		ORDER BY sort_order ASC, created_at ASC
	`

	rows, err := tx.Query(ctx, query, externalProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to list external product media: %w", err)
	}
	defer rows.Close()

	var mediaItems []*entity.ExternalProductMedia
	for rows.Next() {
		item, scanErr := r.scanExternalProductMediaFromRows(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		mediaItems = append(mediaItems, item)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating external product media: %w", rows.Err())
	}
	return mediaItems, nil
}

// SoftDeleteMedia soft-deletes an owned media attachment.
func (r *PromotionRepositoryImpl) SoftDeleteMedia(
	ctx context.Context,
	tx db.Tx,
	ownerUserID, externalProductID, mediaID uuid.UUID,
) error {
	query := `
		UPDATE external_product_media m
		SET deleted_at = NOW()
		FROM external_products p
		WHERE m.id = $1
		  AND m.external_product_id = $2
		  AND p.id = m.external_product_id
		  AND p.owner_user_id = $3
		  AND p.deleted_at IS NULL
		  AND m.deleted_at IS NULL
	`

	result, err := tx.Exec(ctx, query, mediaID, externalProductID, ownerUserID)
	if err != nil {
		return fmt.Errorf("failed to soft delete external product media: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("external product media not found or not owned: %s", mediaID)
	}
	return nil
}

func (r *PromotionRepositoryImpl) getOwnedExternalProductForUpdate(
	ctx context.Context,
	tx db.Tx,
	ownerUserID, externalProductID uuid.UUID,
) (*entity.ExternalProduct, error) {
	query := `
		SELECT id, owner_user_id, title, description, external_url,
		       normalized_external_url, review_status, rejection_reason,
		       unsafe_url_flag, submitted_at, approved_at, rejected_at,
		       hidden_at, last_reviewed_by, created_at, updated_at, deleted_at
		FROM external_products
		WHERE id = $1
		  AND owner_user_id = $2
		  AND deleted_at IS NULL
		FOR UPDATE
	`

	return r.scanExternalProduct(ctx, tx.QueryRow(ctx, query, externalProductID, ownerUserID))
}

func (r *PromotionRepositoryImpl) updateExternalProduct(ctx context.Context, tx db.Tx, product *entity.ExternalProduct) error {
	query := `
		UPDATE external_products
		SET title = $2,
		    description = $3,
		    external_url = $4,
		    normalized_external_url = $5,
		    review_status = $6,
		    rejection_reason = $7,
		    unsafe_url_flag = $8,
		    submitted_at = $9,
		    approved_at = $10,
		    rejected_at = $11,
		    hidden_at = $12,
		    last_reviewed_by = $13,
		    updated_at = $14
		WHERE id = $1
		  AND owner_user_id = $15
		  AND deleted_at IS NULL
	`

	result, err := tx.Exec(ctx, query,
		product.ID,
		product.Title,
		product.Description,
		product.ExternalURL,
		product.NormalizedExternalURL,
		string(product.ReviewStatus),
		product.RejectionReason,
		product.UnsafeURLFlag,
		product.SubmittedAt,
		product.ApprovedAt,
		product.RejectedAt,
		product.HiddenAt,
		product.LastReviewedBy,
		product.UpdatedAt,
		product.OwnerUserID,
	)
	if err != nil {
		return fmt.Errorf("failed to update external product: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("external product not found or not owned: %s", product.ID)
	}
	return nil
}

func (r *PromotionRepositoryImpl) scanExternalProduct(ctx context.Context, row pgx.Row) (*entity.ExternalProduct, error) {
	var product entity.ExternalProduct
	var reviewStatus string
	var fromDeletedAt *time.Time
	if err := row.Scan(
		&product.ID,
		&product.OwnerUserID,
		&product.Title,
		&product.Description,
		&product.ExternalURL,
		&product.NormalizedExternalURL,
		&reviewStatus,
		&product.RejectionReason,
		&product.UnsafeURLFlag,
		&product.SubmittedAt,
		&product.ApprovedAt,
		&product.RejectedAt,
		&product.HiddenAt,
		&product.LastReviewedBy,
		&product.CreatedAt,
		&product.UpdatedAt,
		&fromDeletedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan external product: %w", err)
	}
	product.ReviewStatus = entity.ExternalProductReviewStatus(reviewStatus)
	product.DeletedAt = fromDeletedAt
	return &product, nil
}

func (r *PromotionRepositoryImpl) scanExternalProductFromRows(rows pgx.Rows) (*entity.ExternalProduct, error) {
	var product entity.ExternalProduct
	var reviewStatus string
	if err := rows.Scan(
		&product.ID,
		&product.OwnerUserID,
		&product.Title,
		&product.Description,
		&product.ExternalURL,
		&product.NormalizedExternalURL,
		&reviewStatus,
		&product.RejectionReason,
		&product.UnsafeURLFlag,
		&product.SubmittedAt,
		&product.ApprovedAt,
		&product.RejectedAt,
		&product.HiddenAt,
		&product.LastReviewedBy,
		&product.CreatedAt,
		&product.UpdatedAt,
		&product.DeletedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan external product: %w", err)
	}
	product.ReviewStatus = entity.ExternalProductReviewStatus(reviewStatus)
	return &product, nil
}

func (r *PromotionRepositoryImpl) scanExternalProductMediaFromRows(rows pgx.Rows) (*entity.ExternalProductMedia, error) {
	var media entity.ExternalProductMedia
	var mediaType string
	if err := rows.Scan(
		&media.ID,
		&media.ExternalProductID,
		&mediaType,
		&media.StorageKey,
		&media.URL,
		&media.ThumbnailURL,
		&media.SortOrder,
		&media.Metadata,
		&media.CreatedAt,
		&media.DeletedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan external product media: %w", err)
	}
	media.MediaType = entity.ExternalProductMediaType(mediaType)
	return &media, nil
}

func (r *PromotionRepositoryImpl) scanExternalProductReviewHistoryFromRows(rows pgx.Rows) (*entity.ExternalProductReviewHistory, error) {
	var history entity.ExternalProductReviewHistory
	var fromStatus *string
	var toStatus string
	if err := rows.Scan(
		&history.ID,
		&history.ExternalProductID,
		&history.ActorAdminID,
		&history.ActorUserID,
		&fromStatus,
		&toStatus,
		&history.Reason,
		&history.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan external product review history: %w", err)
	}
	if fromStatus != nil {
		status := entity.ExternalProductReviewStatus(*fromStatus)
		history.FromStatus = &status
	}
	history.ToStatus = entity.ExternalProductReviewStatus(toStatus)
	return &history, nil
}
