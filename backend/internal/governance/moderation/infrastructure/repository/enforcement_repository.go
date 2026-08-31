// SLICE 5: Canonical Enforcement repository interface.
// The only producer of Enforcement rows is the Decision service via this repository.
// Enforcement is mutable: UpdateStatus transitions lifecycle.

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

// EnforcementRepository defines the persistence contract for canonical Enforcements.
type EnforcementRepository interface {
	// Create persists a new pending Enforcement within a transaction.
	// Returns error on duplicate (enforcements_decision_target_unique).
	Create(ctx context.Context, tx db.Tx, enforcement *entity.Enforcement) error

	// GetByID retrieves an Enforcement by its ID. Returns nil when not found.
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Enforcement, error)

	// GetByDecisionAndTarget retrieves an Enforcement by decision + target.
	// Returns nil when not found. Used for enforcement write-back.
	GetByDecisionAndTarget(ctx context.Context, tx db.Tx, decisionID uuid.UUID, targetType entity.ModerationTargetType, targetID uuid.UUID) (*entity.Enforcement, error)

	// UpdateStatus transitions the Enforcement lifecycle atomically.
	// Sets updated_at = now(). Returns nil if no rows affected (idempotent).
	UpdateStatus(ctx context.Context, tx db.Tx, id uuid.UUID, status entity.EnforcementStatus, lastError *string) error

	// MarkProcessing transitions pending/failed → processing atomically.
	// Increments attempt_count, sets started_at = now(). Returns nil if no rows affected.
	MarkProcessing(ctx context.Context, tx db.Tx, id uuid.UUID) error

	// MarkSucceeded transitions processing → succeeded atomically.
	// Sets finished_at = now(). Returns nil if no rows affected.
	MarkSucceeded(ctx context.Context, tx db.Tx, id uuid.UUID) error

	// MarkFailed transitions processing → failed atomically.
	// Sets finished_at = now(), last_error, next_attempt_at for retry.
	// Returns nil if no rows affected.
	MarkFailed(ctx context.Context, tx db.Tx, id uuid.UUID, lastError string, nextAttemptAt *time.Time) error

	// ListByDecision retrieves all Enforcements for a Decision, ordered by created_at ASC.
	ListByDecision(ctx context.Context, tx db.Tx, decisionID uuid.UUID) ([]*entity.Enforcement, error)
}
