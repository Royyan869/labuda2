package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/config/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/shopspring/decimal"
)

// Repository defines the interface for platform config persistence.
// No business logic is enforced here - only data operations.
type Repository interface {
	// Get retrieves a config by key without locking (for read-only operations).
	Get(ctx context.Context, tx db.Tx, key string) (*entity.Config, error)

	// GetAll retrieves all platform configs.
	// MANAGEMENT PRE-FIX M1: Added to support listing all configs in admin view.
	GetAll(ctx context.Context, tx db.Tx) ([]*entity.Config, error)

	// SetNumeric upserts a config with a numeric value.
	SetNumeric(ctx context.Context, tx db.Tx, key string, value decimal.Decimal, updatedBy uuid.UUID) error

	// SetText upserts a config with a text value.
	SetText(ctx context.Context, tx db.Tx, key string, value string, updatedBy uuid.UUID) error
}


