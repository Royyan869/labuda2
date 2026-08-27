// Package service provides decision version tracking for optimistic concurrency.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/pkg/db"
)

var (
	// ErrVersionMismatch is returned when the provided version doesn't match.
	ErrVersionMismatch = errors.New("decision version mismatch")
	// ErrInvalidVersion is returned when the version is invalid.
	ErrInvalidVersion = errors.New("invalid decision version")
	// ErrVersionLogNotFound is returned when version log entry not found.
	ErrVersionLogNotFound = errors.New("decision version log not found")
)

// EntityType defines the types of entities that can have version tracking.
type EntityType string

const (
	EntityOrder   EntityType = "order"
	EntityPayment EntityType = "payment"
	EntityEscrow  EntityType = "escrow"
	EntityDispute EntityType = "dispute"
)

// VersionLog represents a single version log entry.
type VersionLog struct {
	ID              uuid.UUID
	EntityType      EntityType
	EntityID        uuid.UUID
	DecisionVersion int64
	PreviousVersion *int64
	ChangedByUserID *uuid.UUID
	ChangeReason    *string
	CreatedAt       time.Time
}

// VersionCheckResult represents the result of a version check.
type VersionCheckResult struct {
	CurrentVersion int64
	IsValid        bool
	IsStale        bool // True if the client version is outdated
}

// Service handles decision version tracking operations.
//
// DESIGN PRINCIPLES:
// - Version increments on every mutation
// - Optimistic concurrency via version checks
// - Full audit trail of all version changes
// - Tied to entity updated_at timestamp
type Service struct {
	db *db.DB
}

// NewService creates a new DecisionVersionService.
func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

// GetCurrentVersion retrieves the current decision version for an entity.
// The decision version is typically the entity's updated_at timestamp as Unix timestamp.
func (s *Service) GetCurrentVersion(
	ctx context.Context,
	entityType EntityType,
	entityID uuid.UUID,
) (int64, error) {
	// For most entities, decision version is the updated_at timestamp
	// This ensures version changes whenever the entity is modified
	query := ""
	args := []interface{}{entityID}

	switch entityType {
	case EntityOrder:
		query = `SELECT EXTRACT(EPOCH FROM updated_at)::bigint FROM orders WHERE id = $1`
	case EntityPayment:
		query = `SELECT EXTRACT(EPOCH FROM updated_at)::bigint FROM payments WHERE id = $1`
	case EntityEscrow:
		// For escrow, we look at the order's escrow status
		query = `
			SELECT EXTRACT(EPOCH FROM updated_at)::bigint
			FROM orders
			WHERE id = $1
		`
	case EntityDispute:
		query = `SELECT EXTRACT(EPOCH FROM updated_at)::bigint FROM disputes WHERE id = $1`
	default:
		return 0, fmt.Errorf("unsupported entity type: %s", entityType)
	}

	var version int64
	err := s.db.Pool().QueryRow(ctx, query, args...).Scan(&version)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return 0, fmt.Errorf("entity not found")
		}
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}

	return version, nil
}

// CheckVersion checks if the provided version matches the current version.
// Returns a VersionCheckResult with validity information.
func (s *Service) CheckVersion(
	ctx context.Context,
	entityType EntityType,
	entityID uuid.UUID,
	clientVersion int64,
) (*VersionCheckResult, error) {
	currentVersion, err := s.GetCurrentVersion(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}

	return &VersionCheckResult{
		CurrentVersion: currentVersion,
		IsValid:        currentVersion == clientVersion,
		IsStale:        clientVersion < currentVersion,
	}, nil
}

// ValidateVersion validates that the provided version is current.
// Returns ErrVersionMismatch if versions don't match.
func (s *Service) ValidateVersion(
	ctx context.Context,
	entityType EntityType,
	entityID uuid.UUID,
	clientVersion int64,
) error {
	result, err := s.CheckVersion(ctx, entityType, entityID, clientVersion)
	if err != nil {
		return err
	}

	if !result.IsValid {
		return fmt.Errorf("%w: current=%d, provided=%d",
			ErrVersionMismatch, result.CurrentVersion, clientVersion)
	}

	return nil
}

// LogChange logs a version change to the decision_version_log table.
// This creates an audit trail of all version changes.
func (s *Service) LogChange(
	ctx context.Context,
	entityType EntityType,
	entityID uuid.UUID,
	decisionVersion int64,
	changedByUserID *uuid.UUID,
	changeReason *string,
) error {
	// Get previous version if exists
	var previousVersion *int64
	query := `
		SELECT decision_version
		FROM decision_version_log
		WHERE entity_type = $1 AND entity_id = $2
		ORDER BY decision_version DESC
		LIMIT 1
	`

	var prev int64
	err := s.db.Pool().QueryRow(ctx, query, string(entityType), entityID).Scan(&prev)
	if err == nil {
		previousVersion = &prev
	}
	// If no previous version found, that's okay - this is the first version

	// Insert new version log entry
	query = `
		INSERT INTO decision_version_log (
			entity_type, entity_id, decision_version,
			previous_version, changed_by_user_id, change_reason, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`

	_, err = s.db.Pool().Exec(
		ctx,
		query,
		string(entityType), entityID, decisionVersion,
		previousVersion, changedByUserID, changeReason,
	)

	if err != nil {
		if isUniqueViolation(err) {
			// This version already logged - idempotent, return success
			return nil
		}
		return fmt.Errorf("failed to log version change: %w", err)
	}

	return nil
}

// GetVersionHistory retrieves the version history for an entity.
func (s *Service) GetVersionHistory(
	ctx context.Context,
	entityType EntityType,
	entityID uuid.UUID,
	limit int,
) ([]*VersionLog, error) {
	query := `
		SELECT id, entity_type, entity_id, decision_version,
		       previous_version, changed_by_user_id, change_reason, created_at
		FROM decision_version_log
		WHERE entity_type = $1 AND entity_id = $2
		ORDER BY decision_version DESC
		LIMIT $3
	`

	rows, err := s.db.Pool().Query(ctx, query, string(entityType), entityID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get version history: %w", err)
	}
	defer rows.Close()

	var logs []*VersionLog
	for rows.Next() {
		var log VersionLog
		err := rows.Scan(
			&log.ID,
			&log.EntityType,
			&log.EntityID,
			&log.DecisionVersion,
			&log.PreviousVersion,
			&log.ChangedByUserID,
			&log.ChangeReason,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan version log row: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating version log rows: %w", err)
	}

	return logs, nil
}

// GetLatestVersionLog retrieves the latest version log entry for an entity.
func (s *Service) GetLatestVersionLog(
	ctx context.Context,
	entityType EntityType,
	entityID uuid.UUID,
) (*VersionLog, error) {
	query := `
		SELECT id, entity_type, entity_id, decision_version,
		       previous_version, changed_by_user_id, change_reason, created_at
		FROM decision_version_log
		WHERE entity_type = $1 AND entity_id = $2
		ORDER BY decision_version DESC
		LIMIT 1
	`

	var log VersionLog
	err := s.db.Pool().QueryRow(ctx, query, string(entityType), entityID).Scan(
		&log.ID,
		&log.EntityType,
		&log.EntityID,
		&log.DecisionVersion,
		&log.PreviousVersion,
		&log.ChangedByUserID,
		&log.ChangeReason,
		&log.CreatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrVersionLogNotFound
		}
		return nil, fmt.Errorf("failed to get latest version log: %w", err)
	}

	return &log, nil
}

// RecordMutation records a mutation and increments the decision version.
// This is typically called after successfully updating an entity.
// Returns the new decision version.
func (s *Service) RecordMutation(
	ctx context.Context,
	entityType EntityType,
	entityID uuid.UUID,
	changedByUserID *uuid.UUID,
	changeReason *string,
) (int64, error) {
	// Get the new version (updated_at timestamp after mutation)
	newVersion, err := s.GetCurrentVersion(ctx, entityType, entityID)
	if err != nil {
		return 0, fmt.Errorf("failed to get new version: %w", err)
	}

	// Log the version change
	if err := s.LogChange(ctx, entityType, entityID, newVersion, changedByUserID, changeReason); err != nil {
		return 0, fmt.Errorf("failed to log version change: %w", err)
	}

	return newVersion, nil
}

// CleanupOldVersionLogs removes old version log entries.
// Keeps only the most recent N entries per entity.
func (s *Service) CleanupOldVersionLogs(
	ctx context.Context,
	entityType EntityType,
	keepCount int,
) (int, error) {
	query := `
		DELETE FROM decision_version_log
		WHERE entity_type = $1
		  AND id NOT IN (
		    SELECT id
		    FROM (
		      SELECT id, entity_type, entity_id,
		             ROW_NUMBER() OVER (PARTITION BY entity_type, entity_id ORDER BY decision_version DESC) as rn
		      FROM decision_version_log
		      WHERE entity_type = $2
		    ) sub
		    WHERE rn <= $3
		  )
	`

	result, err := s.db.Pool().Exec(ctx, query, keepCount, string(entityType), keepCount)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old version logs: %w", err)
	}

	return int(result.RowsAffected()), nil
}

// GetUserChanges retrieves all version changes made by a specific user.
func (s *Service) GetUserChanges(
	ctx context.Context,
	userID uuid.UUID,
	limit, offset int,
) ([]*VersionLog, error) {
	query := `
		SELECT id, entity_type, entity_id, decision_version,
		       previous_version, changed_by_user_id, change_reason, created_at
		FROM decision_version_log
		WHERE changed_by_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.Pool().Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user changes: %w", err)
	}
	defer rows.Close()

	var logs []*VersionLog
	for rows.Next() {
		var log VersionLog
		err := rows.Scan(
			&log.ID,
			&log.EntityType,
			&log.EntityID,
			&log.DecisionVersion,
			&log.PreviousVersion,
			&log.ChangedByUserID,
			&log.ChangeReason,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan version log row: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating version log rows: %w", err)
	}

	return logs, nil
}

// isUniqueViolation checks if an error is a PostgreSQL UNIQUE constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}


