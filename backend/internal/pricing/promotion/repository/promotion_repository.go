package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
)

// AdminCampaignFilter holds filters for the admin campaign list query.
type AdminCampaignFilter struct {
	Status      string     // "" = all
	TargetType  string     // "" = all
	OwnerUserID *uuid.UUID // nil = all
	PackageID   *uuid.UUID // nil = all
	Limit       int        // 0 = default 50
	Offset      int
}

// AdminCampaignRow is the result of a single row from ListCampaignsAdmin.
// It augments a PromotionInstance with denormalized package / ownership fields.
type AdminCampaignRow struct {
	Instance               *entity.PromotionInstance
	PackageID              uuid.UUID
	PackageName            string
	OwnershipTotalHours    int
	OwnershipConsumedHours int
}

// PromotionRepository defines the interface for promotion persistence.
// This repository manages three entities: PromotionPackage, PromotionOwnership, and PromotionInstance.
//
// Business truth: Duration is ONLY stored at ownership level.
// Instances are pointers that consume duration from their ownership.
type PromotionRepository interface {
	// ========================================================================
	// DATABASE TIME (CRITICAL FOR ACCOUNTING)
	// ========================================================================

	// GetDBTime returns the current database time.
	// CRITICAL: All accounting operations MUST use this method instead of time.Now()
	// to ensure consistency across multiple app servers and prevent clock skew issues.
	GetDBTime(ctx context.Context, tx db.Tx) (time.Time, error)

	// ========================================================================
	// PROMOTION PACKAGES
	// ========================================================================

	// CreatePackage persists a new promotion package.
	CreatePackage(ctx context.Context, tx db.Tx, pkg *entity.PromotionPackage) error

	// GetPackageByID retrieves a package by ID.
	GetPackageByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionPackage, error)

	// ListPackages retrieves all active packages.
	ListPackages(ctx context.Context, tx db.Tx, includeInactive bool) ([]*entity.PromotionPackage, error)

	// UpdatePackage updates a package.
	UpdatePackage(ctx context.Context, tx db.Tx, pkg *entity.PromotionPackage) error

	// ========================================================================
	// PROMOTION OWNERSHIP
	// ========================================================================

	// CreateOwnership persists a new promotion ownership (entitlement).
	CreateOwnership(ctx context.Context, tx db.Tx, ownership *entity.PromotionOwnership) error

	// GetOwnershipByID retrieves an ownership by ID.
	GetOwnershipByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionOwnership, error)

	// GetOwnershipForUpdate retrieves an ownership with FOR UPDATE lock.
	// Required for all duration mutations to prevent concurrent modifications.
	GetOwnershipForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionOwnership, error)

	// GetOwnershipWithInstances retrieves an ownership with all its instances for calculation.
	GetOwnershipWithInstances(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionOwnership, []*entity.PromotionInstance, error)

	// UpdateOwnership persists changes to an ownership.
	UpdateOwnership(ctx context.Context, tx db.Tx, ownership *entity.PromotionOwnership) error

	// AddConsumedDurationToOwnership adds consumed duration (in seconds) to an ownership.
	// This is used when finalizing instances to bake their consumed duration into the ownership.
	AddConsumedDurationToOwnership(ctx context.Context, tx db.Tx, ownershipID uuid.UUID, seconds int) error

	// ListOwnershipsByUser retrieves all ownerships for a user, optionally filtered by status.
	ListOwnershipsByUser(ctx context.Context, tx db.Tx, userID uuid.UUID, status entity.OwnershipStatus) ([]*entity.PromotionOwnership, error)

	// ListOwnershipsByUserPaginated retrieves ownerships for a user with pagination.
	ListOwnershipsByUserPaginated(ctx context.Context, tx db.Tx, userID uuid.UUID, status entity.OwnershipStatus, limit, offset int) ([]*entity.PromotionOwnership, error)

	// FindActiveOwnershipByUserAndPackage finds an available ownership for a user's specific package.
	FindActiveOwnershipByUserAndPackage(ctx context.Context, tx db.Tx, userID, packageID uuid.UUID) (*entity.PromotionOwnership, error)

	// ListExpiredOwnerships retrieves ownerships that have passed their validity window.
	ListExpiredOwnerships(ctx context.Context, tx db.Tx, limit int) ([]*entity.PromotionOwnership, error)

	// ========================================================================
	// PROMOTION INSTANCES
	// ========================================================================

	// CreateInstance persists a new promotion instance.
	CreateInstance(ctx context.Context, tx db.Tx, instance *entity.PromotionInstance) error

	// GetInstanceByID retrieves an instance by ID.
	GetInstanceByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionInstance, error)

	// GetInstanceForUpdate retrieves an instance by ID with FOR UPDATE lock.
	GetInstanceForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionInstance, error)

	// UpdateInstance persists changes to an instance.
	UpdateInstance(ctx context.Context, tx db.Tx, instance *entity.PromotionInstance) error

	// ListInstancesByUser retrieves all instances for a user, optionally filtered by status.
	ListInstancesByUser(ctx context.Context, tx db.Tx, userID uuid.UUID, status entity.InstanceStatus) ([]*entity.PromotionInstance, error)

	// ListInstancesByOwnership retrieves all instances for an ownership.
	ListInstancesByOwnership(ctx context.Context, tx db.Tx, ownershipID uuid.UUID) ([]*entity.PromotionInstance, error)

	// GetActiveInstanceByOwnership retrieves the active instance for an ownership, if any.
	GetActiveInstanceByOwnership(ctx context.Context, tx db.Tx, ownershipID uuid.UUID) (*entity.PromotionInstance, error)

	// GetActiveInstanceByOwnershipForUpdate retrieves the active instance with FOR UPDATE lock.
	// This is used during activation to prevent race conditions when checking/stopping
	// the existing active instance. Ensures only one active instance per ownership.
	GetActiveInstanceByOwnershipForUpdate(ctx context.Context, tx db.Tx, ownershipID uuid.UUID) (*entity.PromotionInstance, error)

	// GetActiveInstancesByTarget retrieves all active instances for a specific target.
	GetActiveInstancesByTarget(ctx context.Context, tx db.Tx, targetType entity.TargetType, targetID uuid.UUID) ([]*entity.PromotionInstance, error)

	// GetPausedInstancesByTarget retrieves all paused instances for a specific target.
	GetPausedInstancesByTarget(ctx context.Context, tx db.Tx, targetType entity.TargetType, targetID uuid.UUID) ([]*entity.PromotionInstance, error)

	// GetActiveInstancesForDiscovery retrieves active instances for discovery surfaces.
	GetActiveInstancesForDiscovery(ctx context.Context, tx db.Tx, limit int) ([]*entity.PromotionInstance, error)

	// GetAllActiveInstances retrieves all active instances (for worker processing).
	GetAllActiveInstances(ctx context.Context, tx db.Tx, limit int) ([]*entity.PromotionInstance, error)

	// GetAllPausedInstances retrieves all paused instances (for worker resume sweep).
	GetAllPausedInstances(ctx context.Context, tx db.Tx, limit int) ([]*entity.PromotionInstance, error)

	// ListCampaignsAdmin retrieves promotion instances for admin visibility with filters.
	// Returns the rows and total count (for pagination).
	ListCampaignsAdmin(ctx context.Context, tx db.Tx, filter AdminCampaignFilter) ([]*AdminCampaignRow, int, error)

	// ========================================================================
	// VALIDATION
	// ========================================================================

	// HasActivePromotionForTarget checks if there's already an active promotion for a target.
	// Used to enforce the "1 for_sale = 1 active promotion" rule.
	HasActivePromotionForTarget(ctx context.Context, tx db.Tx, targetType entity.TargetType, targetID uuid.UUID) (bool, error)

	// GetActiveInstanceByTargetForUpdate retrieves an active instance for a target with FOR UPDATE lock.
	// Used to prevent race conditions during activation.
	GetActiveInstanceByTargetForUpdate(ctx context.Context, tx db.Tx, targetType entity.TargetType, targetID uuid.UUID) (*entity.PromotionInstance, error)

	// ========================================================================
	// EXTERNAL PRODUCTS
	// ========================================================================

	// CreateDraft persists a new external product draft.
	CreateDraft(ctx context.Context, tx db.Tx, product *entity.ExternalProduct) error

	// UpdateOwned updates an owned external product.
	UpdateOwned(ctx context.Context, tx db.Tx, ownerUserID, externalProductID uuid.UUID, input entity.ExternalProductUpdateInput) (*entity.ExternalProduct, error)

	// UpdateByID updates an external product by ID regardless of owner scope.
	UpdateByID(ctx context.Context, tx db.Tx, product *entity.ExternalProduct) error

	// SubmitOwned moves an owned draft external product to pending review.
	SubmitOwned(ctx context.Context, tx db.Tx, ownerUserID, externalProductID uuid.UUID) (*entity.ExternalProduct, error)

	// ResubmitOwned moves a rejected owned external product back to pending review.
	ResubmitOwned(ctx context.Context, tx db.Tx, ownerUserID, externalProductID uuid.UUID) (*entity.ExternalProduct, error)

	// GetOwnedByID retrieves an owned external product by ID.
	GetOwnedByID(ctx context.Context, tx db.Tx, ownerUserID, externalProductID uuid.UUID) (*entity.ExternalProduct, error)

	// ListOwned retrieves owned external products with optional filtering.
	ListOwned(ctx context.Context, tx db.Tx, ownerUserID uuid.UUID, filters ExternalProductListFilters) ([]*entity.ExternalProduct, error)

	// ListForReview retrieves external products visible in the admin review queue.
	ListForReview(ctx context.Context, tx db.Tx, filters ExternalProductAdminListFilters) ([]*entity.ExternalProduct, error)

	// GetByID retrieves an external product by ID.
	GetByID(ctx context.Context, tx db.Tx, externalProductID uuid.UUID) (*entity.ExternalProduct, error)

	// ListReviewHistory retrieves the review history for an external product.
	ListReviewHistory(ctx context.Context, tx db.Tx, externalProductID uuid.UUID) ([]*entity.ExternalProductReviewHistory, error)

	// AppendReviewHistory stores a lifecycle transition record.
	AppendReviewHistory(ctx context.Context, tx db.Tx, history *entity.ExternalProductReviewHistory) error

	// AddMedia attaches uploaded media to an external product.
	AddMedia(ctx context.Context, tx db.Tx, media *entity.ExternalProductMedia) error

	// ListMedia lists uploaded media for an external product.
	ListMedia(ctx context.Context, tx db.Tx, externalProductID uuid.UUID) ([]*entity.ExternalProductMedia, error)

	// SoftDeleteMedia soft-deletes an owned media attachment.
	SoftDeleteMedia(ctx context.Context, tx db.Tx, ownerUserID, externalProductID, mediaID uuid.UUID) error
}
