package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/entity"
)

// ReconciliationRepository defines the interface for reconciliation result persistence.
type ReconciliationRepository interface {
	// Create persists a new reconciliation result.
	Create(ctx context.Context, tx interface{}, result *entity.ReconciliationResult) error

	// GetByID retrieves a reconciliation result by ID.
	GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.ReconciliationResult, error)

	// List retrieves reconciliation results with filtering and pagination.
	List(ctx context.Context, tx interface{}, filters ReconciliationFilters) ([]*entity.ReconciliationResult, error)

	// Count returns the total count of reconciliation results matching filters.
	Count(ctx context.Context, tx interface{}, filters ReconciliationFilters) (int64, error)

	// GetLatest returns the most recent reconciliation result.
	GetLatest(ctx context.Context, tx interface{}) (*entity.ReconciliationResult, error)

	// GetLatestBySeverity returns the most recent result with a given severity.
	GetLatestBySeverity(ctx context.Context, tx interface{}, severity entity.ReconcileSeverity) (*entity.ReconciliationResult, error)

	// DeleteOld deletes reconciliation results older than the given duration.
	DeleteOld(ctx context.Context, tx interface{}, olderThan time.Duration) (int, error)
}

// ReconciliationFilters defines filtering options for reconciliation result queries.
type ReconciliationFilters struct {
	// Severity filter (optional)
	Severity *entity.ReconcileSeverity
	// ActionTaken filter (optional)
	ActionTaken *entity.ReconcileAction
	// AutoRepaired filter (optional)
	AutoRepaired *bool
	// DateFrom filter (optional)
	DateFrom *time.Time
	// DateTo filter (optional)
	DateTo *time.Time
	// Pagination
	Limit  int
	Offset int
}


