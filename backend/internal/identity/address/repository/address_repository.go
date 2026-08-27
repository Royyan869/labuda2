package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/address/entity"
	"github.com/labuda/backend/pkg/db"
)

// AddressCount holds per-purpose address counts.
type AddressCount struct {
	Total         int64
	ShippingCount int64
	SenderCount   int64
}

// AddressRepository defines the interface for address persistence.
type AddressRepository interface {
	// Create persists a new address within a transaction.
	Create(ctx context.Context, tx db.Tx, address *entity.Address) error

	// GetByID retrieves an address without locking (for read-only operations).
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Address, error)

	// GetForUpdate retrieves an address with FOR UPDATE lock.
	// This prevents concurrent modifications and must be used within a transaction.
	GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Address, error)

	// Update persists changes to an address within a transaction.
	Update(ctx context.Context, tx db.Tx, address *entity.Address) error

	// Delete soft-deletes an address within a transaction.
	// Sets is_available_for_checkout to false instead of actual deletion.
	Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error

	// GetByUserID retrieves all addresses for a user (read-only).
	GetByUserID(ctx context.Context, tx db.Tx, userID uuid.UUID) ([]*entity.Address, error)

	// GetByUserIDFiltered retrieves addresses for a user filtered by purpose.
	// If purpose is empty, returns all addresses.
	GetByUserIDFiltered(ctx context.Context, tx db.Tx, userID uuid.UUID, purpose string) ([]*entity.Address, error)

	// GetPrimaryByUserID retrieves the primary address for a user (read-only).
	// Returns nil if no primary address is set.
	GetPrimaryByUserID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.Address, error)

	// GetPrimaryByUserIDFiltered retrieves the primary address for a user filtered by purpose.
	// Returns nil if no primary address is set.
	GetPrimaryByUserIDFiltered(ctx context.Context, tx db.Tx, userID uuid.UUID, purpose string) (*entity.Address, error)

	// SetPrimary sets an address as primary and unsets all other primary addresses for the user.
	// This must be executed within a transaction to ensure consistency.
	SetPrimary(ctx context.Context, tx db.Tx, addressID uuid.UUID) error

	// UnsetAllPrimary removes the primary flag from all addresses for a user.
	// Used internally when setting a new primary address.
	UnsetAllPrimary(ctx context.Context, tx db.Tx, userID uuid.UUID) error

	// CountByUserID returns address counts grouped by purpose.
	CountByUserID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*AddressCount, error)
}


