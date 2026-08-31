// SLICE 4: Canonical Decision repository interface.
// The only producer of Decision rows is the Decision service via this repository.
// Decision is append-only: no Update, no Delete operations.

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

// DecisionRepository defines the persistence contract for canonical Decisions.
type DecisionRepository interface {
	// Create persists a new immutable Decision within a transaction.
	Create(ctx context.Context, tx db.Tx, decision *entity.Decision) error

	// GetByID retrieves a Decision by its ID. Returns nil when not found.
	GetByID(ctx context.Context, tx db.Tx, decisionID uuid.UUID) (*entity.Decision, error)

	// ListByCase retrieves all Decisions for a Case, ordered by created_at DESC.
	ListByCase(ctx context.Context, tx db.Tx, caseID uuid.UUID, limit, offset int) ([]*entity.Decision, error)
}
