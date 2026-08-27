package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/alert/entity"
)

// AlertRepository defines the interface for alert persistence.
type AlertRepository interface {
	// Create persists a new alert within a transaction.
	Create(ctx context.Context, tx interface{}, alert *entity.Alert) error

	// GetByID retrieves an alert without locking.
	GetByID(ctx context.Context, tx interface{}, alertID uuid.UUID) (*entity.Alert, error)

	// GetForUpdate retrieves an alert with FOR UPDATE lock.
	GetForUpdate(ctx context.Context, tx interface{}, alertID uuid.UUID) (*entity.Alert, error)

	// Update persists alert changes within a transaction.
	Update(ctx context.Context, tx interface{}, alert *entity.Alert) error

	// List retrieves alerts with filtering and pagination.
	List(ctx context.Context, tx interface{}, filters AlertFilters) ([]*entity.Alert, error)

	// Count returns the total count of alerts matching filters.
	Count(ctx context.Context, tx interface{}, filters AlertFilters) (int64, error)

	// FindActiveByGroupKey finds active alerts with the given group key.
	FindActiveByGroupKey(ctx context.Context, tx interface{}, groupKey string) ([]*entity.Alert, error)

	// FindByDedupKeyInWindow finds alerts with the same dedup_key within time window.
	FindByDedupKeyInWindow(ctx context.Context, tx interface{}, dedupKey string, minutes int) ([]*entity.Alert, error)

	// DeleteOld deletes resolved alerts older than the given duration.
	DeleteOld(ctx context.Context, tx interface{}, olderThan int) (int, error)
}

// AlertFilters defines filtering options for alert queries.
type AlertFilters struct {
	// Status filter (optional). Ignored when Statuses is set.
	Status *entity.AlertStatus
	// Statuses filter matches any of the given statuses (status IN (...)). Takes precedence over Status.
	Statuses []entity.AlertStatus
	// Severity filter (optional)
	Severity *entity.AlertSeverity
	// AlertType filter (optional)
	AlertType *entity.AlertType
	// EntityType filter (optional)
	EntityType *string
	// EntityID filter (optional)
	EntityID *uuid.UUID
	// GroupKey filter (optional)
	GroupKey *string
	// DateFrom filter (optional)
	DateFrom *time.Time
	// DateTo filter (optional)
	DateTo *time.Time
	// Pagination
	Limit  int
	Offset int
}


