// Package repository defines the persistence surface for the canonical
// Promotion Contract aggregate.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	"github.com/labuda/backend/pkg/db"
)

// ErrContractNotFound is returned when no contract row matches the requested id.
var ErrContractNotFound = errors.New("promotion contract not found")

// Repository persists promotion contracts. All methods run inside the
// caller-provided transaction so lifecycle changes stay atomic with their
// ledger movements.
type Repository interface {
	// Create inserts a contract row.
	Create(ctx context.Context, tx db.Tx, contract *entity.Contract) error

	// GetByID reads a contract without locking (read-only operations).
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Contract, error)

	// GetForUpdate reads a contract with FOR UPDATE so concurrent lifecycle
	// transitions serialize on the row.
	GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Contract, error)

	// ListBySeller returns every contract owned by the seller, newest first.
	// Read-only (no row locks): seller-scoped reads are safe without locks
	// because lifecycle correctness is protected by the FOR UPDATE paths.
	ListBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID) ([]*entity.Contract, error)

	// Update persists the mutable lifecycle fields (status, paused_at,
	// finalized_at, planned_finish shift). Immutable fields are never written.
	Update(ctx context.Context, tx db.Tx, contract *entity.Contract) error

	// GetDBTime returns the database clock. All lifecycle-sensitive time
	// (planned_start, planned_finish shifts, pause bookkeeping) MUST use the
	// DB clock, never the application clock.
	GetDBTime(ctx context.Context, tx db.Tx) (time.Time, error)
}
