package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
)

// WarningRepository defines the interface for warning persistence operations.
type WarningRepository interface {
	// Create persists a new warning within a transaction.
	Create(ctx context.Context, tx interface{}, warning *entity.UserWarning) error

	// GetByID retrieves a warning by ID without locking.
	GetByID(ctx context.Context, tx interface{}, warningID uuid.UUID) (*entity.UserWarning, error)

	// GetForUpdate retrieves a warning with FOR UPDATE lock.
	// Must be used for all revoke operations to prevent concurrent modifications.
	GetForUpdate(ctx context.Context, tx interface{}, warningID uuid.UUID) (*entity.UserWarning, error)

	// Update persists warning changes within a transaction.
	Update(ctx context.Context, tx interface{}, warning *entity.UserWarning) error

	// ListByUser retrieves all warnings for a specific user.
	// Ordered by created_at DESC (newest first).
	ListByUser(ctx context.Context, tx interface{}, userID uuid.UUID, limit, offset int) ([]*entity.UserWarning, error)

	// ListActiveByUser retrieves active warnings for a specific user.
	ListActiveByUser(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*entity.UserWarning, error)

	// ListAll retrieves all warnings with optional user filter.
	// If userID is nil, returns all warnings.
	// Ordered by created_at DESC (newest first).
	ListAll(ctx context.Context, tx interface{}, userID *uuid.UUID, isActive *bool, limit, offset int) ([]*entity.UserWarning, int64, error)
}


