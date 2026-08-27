// Package repository defines the dispute repository interface.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/dispute/entity"
	"github.com/labuda/backend/pkg/db"
)

// DisputeRepository defines the interface for dispute persistence.
type DisputeRepository interface {
	// Create creates a new dispute within a transaction.
	Create(ctx context.Context, tx db.Tx, dispute *entity.Dispute) error

	// GetByOrderID retrieves a dispute by order ID without locking.
	GetByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Dispute, error)

	// GetForUpdate retrieves a dispute with FOR UPDATE lock.
	// This must be used within a transaction for state changes.
	GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Dispute, error)

	// Update updates an existing dispute within a transaction.
	Update(ctx context.Context, tx db.Tx, dispute *entity.Dispute) error

	// CreateMedia creates a media attachment for a dispute.
	CreateMedia(ctx context.Context, tx db.Tx, disputeID uuid.UUID, mediaURL string) error

	// ListMedia retrieves all media URLs for a dispute.
	ListMedia(ctx context.Context, tx db.Tx, disputeID uuid.UUID) ([]string, error)

	// ============================================================================
	// ADMIN QUERY METHODS
	// ============================================================================

	// ListAll retrieves all disputes with optional filters for admin.
	// Returns disputes with total count for pagination.
	ListAll(ctx context.Context, tx db.Tx, filters DisputeListFilters) ([]*entity.Dispute, int64, error)

	// GetByID retrieves a dispute by ID without locking (for read-only admin view).
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Dispute, error)

	// ============================================================================
	// DEADLOCK PREVENTION METHODS
	// ============================================================================

	// FindOverdueCandidates retrieves disputes that should be marked as overdue.
	// Returns disputes that are under_review, opened more than 3 days ago, and not yet marked overdue.
	FindOverdueCandidates(ctx context.Context, tx db.Tx, limit int) ([]uuid.UUID, error)

	// FindTimeoutCandidates retrieves disputes that should be auto-resolved.
	// Returns disputes that are under_review and opened more than their timeout_days ago.
	FindTimeoutCandidates(ctx context.Context, tx db.Tx, limit int) ([]uuid.UUID, error)

	// ============================================================================
	// ABUSE DETECTION METHODS
	// ============================================================================

	// GetCallerDisputeCount returns the number of disputes opened by callerID at or after since.
	// Counts all dispute statuses (under_review, resolved_*) — only caller intent matters for abuse.
	GetCallerDisputeCount(ctx context.Context, tx db.Tx, callerID uuid.UUID, since time.Time) (int, error)

	// GetCallerDisputeCountAgainstParty returns the number of disputes opened by callerID
	// against partyID (as buyer or seller) at or after since.
	GetCallerDisputeCountAgainstParty(ctx context.Context, tx db.Tx, callerID uuid.UUID, partyID uuid.UUID, since time.Time) (int, error)
}

// DisputeListFilters contains filters for admin dispute listing.
type DisputeListFilters struct {
	Status   *string // Filter by dispute status (opened, resolved_refund, resolved_release)
	DateFrom *time.Time
	DateTo   *time.Time
	Page     int
	PageSize int
}


