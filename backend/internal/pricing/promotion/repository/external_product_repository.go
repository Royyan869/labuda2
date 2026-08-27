package repository

import (
	"github.com/labuda/backend/internal/pricing/promotion/entity"
)

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
