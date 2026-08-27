// Package repository provides the idempotency records repository.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

var (
	// ErrRecordNotFound is returned when no idempotency record exists for a key.
	ErrRecordNotFound = errors.New("idempotency record not found")
)

// Record represents an idempotency record.
type Record struct {
	ID             uuid.UUID
	IdempotencyKey string
	Operation      string
	EntityID       uuid.UUID
}

// Repository handles idempotency records.
//
// DESIGN PRINCIPLES:
// - Separate from entity tables (proper separation of concerns)
// - UNIQUE constraint on idempotency_key prevents duplicate operations
// - TryInsert returns nil on conflict (idempotent operation already exists)
// - GetOrCreate allows checking if operation was already performed
type Repository struct{}

// NewRepository creates a new IdempotencyRepository.
func NewRepository() *Repository {
	return &Repository{}
}

// TryInsert attempts to insert a new idempotency record.
// Uses ON CONFLICT DO NOTHING to avoid aborting the surrounding transaction on
// duplicate key — a naive INSERT would cause PostgreSQL to mark the transaction
// as ABORTED, making all subsequent SQL in the same WithTx closure fail.
//
// Returns:
// - nil on success (record inserted)
// - ErrAlreadyExists if idempotency_key already exists (operation already performed)
// - other error on database failure
func (r *Repository) TryInsert(
	ctx context.Context,
	tx db.Tx,
	idempotencyKey string,
	operation string,
	entityID uuid.UUID,
) error {
	result, err := tx.Exec(ctx, `
		INSERT INTO idempotency_records (idempotency_key, operation, entity_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, idempotencyKey, operation, entityID)
	if err != nil {
		return fmt.Errorf("failed to insert idempotency record: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("operation already performed: %w", ErrAlreadyExists)
	}
	return nil
}

// Get retrieves an idempotency record by key.
// Returns ErrRecordNotFound if no record exists.
func (r *Repository) Get(
	ctx context.Context,
	tx db.Tx,
	idempotencyKey string,
) (*Record, error) {
	var rec Record
	query := `
		SELECT id, idempotency_key, operation, entity_id
		FROM idempotency_records
		WHERE idempotency_key = $1
	`

	err := tx.QueryRow(ctx, query, idempotencyKey).Scan(
		&rec.ID,
		&rec.IdempotencyKey,
		&rec.Operation,
		&rec.EntityID,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrRecordNotFound
		}
		return nil, fmt.Errorf("failed to get idempotency record: %w", err)
	}

	return &rec, nil
}

// GetOrCreate retrieves an existing record or creates a new one.
// Returns the record (existing or new) and a boolean indicating if it was created.
func (r *Repository) GetOrCreate(
	ctx context.Context,
	tx db.Tx,
	idempotencyKey string,
	operation string,
	entityID uuid.UUID,
) (*Record, bool, error) {
	// First, try to get existing record
	rec, err := r.Get(ctx, tx, idempotencyKey)
	if err == nil {
		// Record exists - verify operation matches
		if rec.Operation != operation {
			return nil, false, fmt.Errorf("idempotency key exists with different operation: expected %s, got %s", operation, rec.Operation)
		}
		return rec, false, nil
	}

	if err != ErrRecordNotFound {
		// Some other error
		return nil, false, err
	}

	// Record doesn't exist, create it
	insertErr := r.TryInsert(ctx, tx, idempotencyKey, operation, entityID)
	if insertErr != nil && !errors.Is(insertErr, ErrAlreadyExists) {
		// Failed to insert (and not due to duplicate key)
		return nil, false, insertErr
	}

	// If we got ErrAlreadyExists, another transaction inserted it first
	// Reload to get the actual record
	if errors.Is(insertErr, ErrAlreadyExists) {
		rec, loadErr := r.Get(ctx, tx, idempotencyKey)
		if loadErr != nil {
			return nil, false, loadErr
		}
		return rec, false, nil
	}

	// Successfully created new record
	return &Record{
		ID:             uuid.New(), // Approximate, actual ID is DB-generated
		IdempotencyKey: idempotencyKey,
		Operation:      operation,
		EntityID:       entityID,
	}, true, nil
}

// ErrAlreadyExists is returned when TryInsert encounters a duplicate idempotency key.
var ErrAlreadyExists = errors.New("idempotency record already exists")
