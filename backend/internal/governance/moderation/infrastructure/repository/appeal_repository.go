package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
)

// AppealRepository defines the interface for appeal persistence operations.
//
// SLICE A: Appeal → Decision (canonical: appeals.decision_id → decisions.id).
// Legacy report_id references have been removed.
type AppealRepository interface {
	// Create persists a new appeal within a transaction.
	Create(ctx context.Context, tx interface{}, appeal *entity.Appeal) error

	// CreateWithPendingCheck atomically creates an appeal only if no pending appeal
	// exists for the same Decision. Uses CTE with FOR UPDATE to prevent race conditions.
	// Returns entity.ErrDuplicatePendingAppeal if a pending appeal already exists.
	CreateWithPendingCheck(ctx context.Context, tx interface{}, appeal *entity.Appeal) error

	// GetByID retrieves an appeal by ID without locking.
	GetByID(ctx context.Context, tx interface{}, appealID uuid.UUID) (*entity.Appeal, error)

	// GetForUpdate retrieves an appeal with FOR UPDATE lock.
	// Must be used for all review operations to prevent double-review.
	GetForUpdate(ctx context.Context, tx interface{}, appealID uuid.UUID) (*entity.Appeal, error)

	// Update persists appeal changes within a transaction.
	Update(ctx context.Context, tx interface{}, appeal *entity.Appeal) error

	// ListByUser retrieves all appeals created by a specific user.
	// Ordered by created_at DESC (newest first).
	ListByUser(ctx context.Context, tx interface{}, userID uuid.UUID, limit, offset int) ([]*entity.Appeal, error)

	// ListByDecisionID retrieves all appeals for a specific Decision.
	// Canonical: Decision 1 → 0..N Appeal (Design §5).
	ListByDecisionID(ctx context.Context, tx interface{}, decisionID uuid.UUID) ([]*entity.Appeal, error)

	// ListByCase retrieves all appeals for a specific Case.
	// Joins through decisions table: appeals → decisions → case.
	ListByCase(ctx context.Context, tx interface{}, caseID uuid.UUID) ([]*entity.Appeal, error)

	// ListAll retrieves all appeals with optional status filter.
	// If statusFilter is nil, returns all appeals regardless of status.
	// Ordered by created_at ASC (oldest first).
	ListAll(ctx context.Context, tx interface{}, statusFilter *entity.AppealStatus, limit, offset int) ([]*entity.Appeal, error)

	// ListPending retrieves pending appeals awaiting review.
	// Ordered by created_at ASC (oldest first).
	ListPending(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.Appeal, error)
}
