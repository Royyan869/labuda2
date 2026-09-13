// Package repository provides the interface for capability persistence.
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/capability/entity"
)

// CapabilityRepository defines the interface for capability data operations.
//
// DESIGN PRINCIPLES:
// - All database operations are abstracted behind this interface
// - Repository does NOT contain business logic
// - Transactions use interface{} tx parameter for flexibility
// - Only reads active capabilities (revoked_at IS NULL) for checks
type CapabilityRepository interface {
	// Create persists a new capability grant within a transaction.
	Create(ctx context.Context, tx interface{}, cap *entity.UserCapability) error

	// GetByID retrieves a capability grant by ID (including revoked).
	GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.UserCapability, error)

	// GetActiveCapability retrieves an active capability for a user.
	// Returns nil if the user does not have this capability active.
	GetActiveCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capability string) (*entity.UserCapability, error)

	// ListActiveCapabilities retrieves all active capabilities for a user.
	// Only returns capabilities where revoked_at IS NULL.
	ListActiveCapabilities(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*entity.UserCapability, error)

	// Revoke soft-deletes a capability by setting revoked_at.
	// Returns error if capability not found or already revoked.
	Revoke(ctx context.Context, tx interface{}, id uuid.UUID, revokedAt *interface{}) error

	// HasCapability checks if a user has an active capability.
	// This is a read-optimized query that returns true/false.
	HasCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capability string) (bool, error)

	// HasAnyCapability checks if a user has any of the given capabilities.
	// Returns true if at least one capability is active.
	HasAnyCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capabilities []string) (bool, error)

	// CountActiveCapabilities returns the number of active capabilities for a user.
	CountActiveCapabilities(ctx context.Context, tx interface{}, userID uuid.UUID) (int, error)

	// ListUsersByCapability returns distinct user IDs that hold the given
	// capability (revoked_at IS NULL). Deleted users (deleted_at IS NOT NULL)
	// are excluded. Banned/suspended users are NOT excluded — the policy
	// layer handles account status at notification delivery time.
	ListUsersByCapability(ctx context.Context, tx interface{}, capability string) ([]uuid.UUID, error)

	// CreateGrant persists a new capability grant in its own transaction.
	//
	// This is the canonical write path for capability assignment: callers state
	// the intent ("grant this capability") and the repository owns the
	// transaction boundary. Do not use the low-level Create with a nil tx —
	// a grant must be committed atomically.
	CreateGrant(ctx context.Context, cap *entity.UserCapability) error

	// RevokeGuarded soft-deletes a capability grant in its own transaction that
	// serializes against the full-access admin invariant and refuses to apply a
	// change that would leave the system with zero full-access admins.
	//
	// Returns invariant.ErrLastFullAccessAdmin when the change is refused, in
	// which case nothing is written.
	RevokeGuarded(ctx context.Context, id uuid.UUID) error

	// GetUserRole returns users.role for a user, or "" when the user does not
	// exist. Used to derive canonical full-access state (role + capability
	// coverage) from the single canonical authority.
	GetUserRole(ctx context.Context, userID uuid.UUID) (string, error)
}


