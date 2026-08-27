package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
)

// ModerationRepository defines the interface for governance case persistence.
//
// DOMAIN TERMINOLOGY:
// - REPORT: User action (not persisted here, converted to CASE)
// - CASE: Moderation case entity (GovernanceCase) persisted in moderation_cases table
// - APPEAL: User contest (handled by AppealRepository)
//
// DESIGN PRINCIPLES:
// - All write operations must use explicit transaction (tx interface{})
// - Review operations must use GetForUpdate to prevent concurrent reviews
// - Read operations (GetByID, ListByResource) do not lock
// - Repository does NOT contain business logic
type ModerationRepository interface {
	// Create persists a new governance case within a transaction.
	Create(ctx context.Context, tx interface{}, caseEntity *entity.GovernanceCase) error

	// GetByID retrieves a case without locking (for read-only operations).
	GetByID(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error)

	// GetForUpdate retrieves a case with FOR UPDATE lock.
	// CRITICAL: Must be used for all review operations to prevent double-review.
	GetForUpdate(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error)

	// Update persists case changes within a transaction.
	Update(ctx context.Context, tx interface{}, caseEntity *entity.GovernanceCase) error

	// ListPending retrieves pending cases awaiting review.
	// Ordered by created_at ASC (oldest first).
	ListPending(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.GovernanceCase, error)

	// ListByResource retrieves all cases for a specific resource.
	// Useful for checking governance history.
	ListByResource(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) ([]*entity.GovernanceCase, error)

	// ListByReporter retrieves all cases created by a specific reporter.
	// Ordered by created_at DESC (newest first).
	// Supports pagination with limit and offset.
	ListByReporter(ctx context.Context, tx interface{}, reporterID uuid.UUID, limit, offset int) ([]*entity.GovernanceCase, error)

	// ListWithStatus retrieves cases filtered by status and optional resource type.
	// If a filter is nil, that dimension is not applied.
	// Ordered by created_at ASC (oldest first).
	// Supports pagination with limit and offset.
	ListWithStatus(ctx context.Context, tx interface{}, statusFilter *entity.GovernanceCaseStatus, resourceTypeFilter *entity.ResourceType, limit, offset int) ([]*entity.GovernanceCase, int64, error)

	// ResourceExists checks if a resource exists in the system.
	// Supported types: content, comment, for_sale, auction, user, chat_message
	// Returns true if resource exists, false otherwise.
	ResourceExists(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error)

	// HasUserReportedResource checks if a user has already reported a specific resource.
	// Returns true if the user has reported this resource before, false otherwise.
	HasUserReportedResource(ctx context.Context, tx interface{}, reporterID uuid.UUID, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error)

	// ValidateChatMessageReporter checks that the reporter is a room participant
	// and the message's room is not a support room.
	// Returns (authorized, rejectReason, err).
	ValidateChatMessageReporter(ctx context.Context, tx interface{}, messageID uuid.UUID, reporterID uuid.UUID) (bool, string, error)
}


