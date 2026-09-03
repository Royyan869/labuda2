// Package repository provides the PostgreSQL implementation of the canonical
// commerce violation/restriction repository.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/governance/commercegov"
	"github.com/labuda/backend/pkg/db"
)

// Repository implements commercegov.Repository against PostgreSQL.
type Repository struct{}

// NewRepository creates a new commerce violation/restriction repository.
func NewRepository() *Repository {
	return &Repository{}
}

// InsertViolation appends an immutable violation row.
func (r *Repository) InsertViolation(ctx context.Context, tx db.Tx, v *commercegov.Violation) error {
	metadataRaw := commercegov.MarshalMetadata(v.Metadata)

	_, err := tx.Exec(ctx, `
		INSERT INTO commerce_violations (
			id, user_id, violation_type, source_type, source_id, reason, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		v.ID,
		v.UserID,
		string(v.ViolationType),
		string(v.SourceType),
		v.SourceID,
		v.Reason,
		metadataRaw,
	)
	if err != nil {
		return fmt.Errorf("insert commerce_violation failed: %w", err)
	}
	return nil
}

// GetRestrictionForUpdate loads a user's restriction row with FOR UPDATE
// (serializes concurrent EXTEND stacking). Returns nil when no row exists.
func (r *Repository) GetRestrictionForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*commercegov.Restriction, error) {
	const q = `
		SELECT id, user_id, violation_count, restricted_until, last_violation_id, created_at, updated_at
		FROM commerce_restrictions
		WHERE user_id = $1
		FOR UPDATE
	`
	var res commercegov.Restriction
	err := tx.QueryRow(ctx, q, userID).Scan(
		&res.ID, &res.UserID, &res.ViolationCount, &res.RestrictedUntil,
		&res.LastViolationID, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get commerce_restriction failed: %w", err)
	}
	return &res, nil
}

// UpsertRestriction inserts or updates the user's restriction row.
func (r *Repository) UpsertRestriction(ctx context.Context, tx db.Tx, res *commercegov.Restriction) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		INSERT INTO commerce_restrictions (
			id, user_id, violation_count, restricted_until, last_violation_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE
		SET violation_count = EXCLUDED.violation_count,
		    restricted_until = EXCLUDED.restricted_until,
		    last_violation_id = EXCLUDED.last_violation_id,
		    updated_at = EXCLUDED.updated_at
	`,
		res.ID,
		res.UserID,
		res.ViolationCount,
		res.RestrictedUntil,
		res.LastViolationID,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("upsert commerce_restriction failed: %w", err)
	}
	return nil
}

var _ commercegov.Repository = (*Repository)(nil)
