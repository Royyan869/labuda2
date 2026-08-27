// 🔥 PHASE 2: DOMAIN ACTION REPOSITORY INTERFACE
//
// Provides persistence operations for idempotent domain actions.

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
)

// DomainActionRepository defines the interface for domain action persistence operations.
//
// 🔥 PHASE 2: All operations are idempotent and retry-safe.
type DomainActionRepository interface {
	// Create persists a new domain action within a transaction.
	//
	// 🔥 PHASE 2: Idempotent - duplicate idempotency keys are rejected.
	Create(ctx context.Context, tx interface{}, action *entity.DomainAction) error

	// CreateWithIdempotencyCheck creates a domain action with idempotency check.
	//
	// 🔥 PHASE 2: If an action with the same idempotency key exists,
	// the existing action is returned instead of creating a duplicate.
	// This makes the operation idempotent and safe for retries.
	CreateWithIdempotencyCheck(ctx context.Context, tx interface{}, action *entity.DomainAction) (*entity.DomainAction, error)

	// GetByID retrieves a domain action by ID without locking.
	GetByID(ctx context.Context, tx interface{}, actionID uuid.UUID) (*entity.DomainAction, error)

	// GetForUpdate retrieves a domain action with FOR UPDATE lock.
	// Must be used for all execution operations to prevent concurrent execution.
	GetForUpdate(ctx context.Context, tx interface{}, actionID uuid.UUID) (*entity.DomainAction, error)

	// GetByIdempotencyKey retrieves a domain action by idempotency key.
	//
	// 🔥 PHASE 2: Used for idempotency checks.
	GetByIdempotencyKey(ctx context.Context, tx interface{}, idempotencyKey string) (*entity.DomainAction, error)

	// GetByGovernanceCaseID retrieves all domain actions for a governance case.
	GetByGovernanceCaseID(ctx context.Context, tx interface{}, governanceCaseID uuid.UUID) ([]*entity.DomainAction, error)

	// GetByTargetResourceID retrieves all domain actions for a target resource.
	GetByTargetResourceID(ctx context.Context, tx interface{}, targetResourceID uuid.UUID) ([]*entity.DomainAction, error)

	// Update persists domain action changes within a transaction.
	Update(ctx context.Context, tx interface{}, action *entity.DomainAction) error

	// ListPending retrieves all pending actions awaiting execution.
	// Ordered by created_at ASC (oldest first).
	ListPending(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.DomainAction, error)

	// ListFailed retrieves all failed actions that can be retried.
	// Ordered by created_at ASC (oldest first).
	ListFailed(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.DomainAction, error)

	// ListByExecutionGroup retrieves all actions in an execution group.
	ListByExecutionGroup(ctx context.Context, tx interface{}, executionGroupID uuid.UUID) ([]*entity.DomainAction, error)

	// MarkAsSucceeded marks an action as succeeded atomically.
	MarkAsSucceeded(ctx context.Context, tx interface{}, actionID uuid.UUID, previousState, newState []byte) error

	// MarkAsFailed marks an action as failed atomically.
	MarkAsFailed(ctx context.Context, tx interface{}, actionID uuid.UUID, errorMessage string) error
}


