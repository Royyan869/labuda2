package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
)

// ExternalProductRepository is the canonical external product persistence boundary.
type ExternalProductRepository interface {
	GetDBTime(ctx context.Context, tx db.Tx) (time.Time, error)
	CreateDraft(ctx context.Context, tx db.Tx, product *entity.ExternalProduct) error
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ExternalProduct, error)
	UpdateOwned(ctx context.Context, tx db.Tx, userID, id uuid.UUID, update entity.ExternalProductUpdateInput) (*entity.ExternalProduct, error)
	SubmitOwned(ctx context.Context, tx db.Tx, userID, id uuid.UUID) (*entity.ExternalProduct, error)
	ResubmitOwned(ctx context.Context, tx db.Tx, userID, id uuid.UUID) (*entity.ExternalProduct, error)
	UpdateByID(ctx context.Context, tx db.Tx, product *entity.ExternalProduct) error
	AppendReviewHistory(ctx context.Context, tx db.Tx, history *entity.ExternalProductReviewHistory) error
	ListOwned(ctx context.Context, tx db.Tx, userID uuid.UUID, filters ExternalProductListFilters) ([]*entity.ExternalProduct, error)
	ListForReview(ctx context.Context, tx db.Tx, filters ExternalProductAdminListFilters) ([]*entity.ExternalProduct, error)
	ListReviewHistory(ctx context.Context, tx db.Tx, id uuid.UUID) ([]*entity.ExternalProductReviewHistory, error)
	ListMedia(ctx context.Context, tx db.Tx, id uuid.UUID) ([]*entity.ExternalProductMedia, error)
	AddMedia(ctx context.Context, tx db.Tx, media *entity.ExternalProductMedia) error
	SoftDeleteMedia(ctx context.Context, tx db.Tx, userID, externalProductID, mediaID uuid.UUID) error
}

// ExternalProductListFilters controls owner-scoped listing behavior.
type ExternalProductListFilters struct {
	ReviewStatus   *entity.ExternalProductReviewStatus
	IncludeDeleted bool
	Limit          int
	Offset         int
}

// ExternalProductAdminListFilters controls admin review-queue listing behavior.
type ExternalProductAdminListFilters struct {
	ReviewStatuses []entity.ExternalProductReviewStatus
	IncludeDeleted bool
	Limit          int
	Offset         int
}
