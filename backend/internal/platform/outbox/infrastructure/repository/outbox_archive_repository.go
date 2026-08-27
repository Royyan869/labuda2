package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/pkg/db"
)

// ArchivedEvent represents an archived outbox event.
type ArchivedEvent struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       []byte
	Status        EventStatus
	RetryCount    int
	NextAttemptAt time.Time
	IdempotencyKey string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ArchivedAt    time.Time
}

// OutboxArchiveRepository handles archival operations for outbox events.
//
// DESIGN PRINCIPLES:
// - No GORM dependency - uses pgx via pkg/db
// - Atomic move operation using DELETE...RETURNING + INSERT
// - Only archives succeeded events
// - No modification of financial logic
// - Repository only - no business logic
type OutboxArchiveRepository struct {
	db *db.DB
}

// NewOutboxArchiveRepository creates a new OutboxArchiveRepository instance.
func NewOutboxArchiveRepository(database *db.DB) *OutboxArchiveRepository {
	return &OutboxArchiveRepository{
		db: database,
	}
}

// ArchiveBatch performs a fully atomic archival in a SINGLE transaction.
//
// This method does everything in one transaction:
// 1. SELECT id FROM outbox WHERE status='succeeded' AND created_at < cutoff
//    FOR UPDATE SKIP LOCKED LIMIT batch_size
// 2. DELETE FROM outbox WHERE id IN (selected_ids) RETURNING *
// 3. INSERT INTO outbox_archive (...)
//
// If any step fails, the entire transaction rolls back.
// No rows are lost - DELETE...RETURNING ensures data is captured before deletion.
//
// This is the PRIMARY method for archival and replaces the two-phase pattern.
func (r *OutboxArchiveRepository) ArchiveBatch(
	ctx context.Context,
	tx db.Tx,
	retentionDays int,
	batchSize int,
) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	now := time.Now()

	// Step 1: Fetch and lock IDs in one query
	// Using a CTE with DELETE...RETURNING followed by INSERT
	// This ensures all operations happen atomically
	query := `
		WITH locked_events AS (
			SELECT id
			FROM outbox
			WHERE status = $1
			  AND created_at < $2
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $3
		),
		deleted_events AS (
			DELETE FROM outbox
			WHERE id IN (SELECT id FROM locked_events)
			RETURNING
				id, aggregate_type, aggregate_id, event_type, payload,
				status, retry_count, next_attempt_at, idempotency_key,
				created_at, updated_at
		)
		INSERT INTO outbox_archive (
			id, aggregate_type, aggregate_id, event_type, payload,
			status, retry_count, next_attempt_at, idempotency_key,
			created_at, updated_at, archived_at
		)
		SELECT
			id, aggregate_type, aggregate_id, event_type, payload,
			status, retry_count, next_attempt_at, idempotency_key,
			created_at, updated_at, $4
		FROM deleted_events
		RETURNING id
	`

	rows, err := tx.Query(ctx, query, StatusSucceeded, cutoff, batchSize, now)
	if err != nil {
		return 0, fmt.Errorf("failed to execute archival query: %w", err)
	}
	defer rows.Close()

	// Collect all IDs to count how many were archived
	ids, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (uuid.UUID, error) {
		var id uuid.UUID
		if err := row.Scan(&id); err != nil {
			return uuid.Nil, err
		}
		return id, nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to scan archived IDs: %w", err)
	}

	return len(ids), nil
}

// DeleteAndReturn deletes events from outbox and returns the full row data.
//
// This is used with RETURNING * to get the full event data for insertion into archive.
// Returns the deleted events as ArchivedEvent (without ArchivedAt set).
func (r *OutboxArchiveRepository) DeleteAndReturn(
	ctx context.Context,
	tx db.Tx,
	ids []uuid.UUID,
) ([]ArchivedEvent, error) {
	if len(ids) == 0 {
		return []ArchivedEvent{}, nil
	}

	query := `
		DELETE FROM outbox
		WHERE id = ANY($1)
		RETURNING
			id, aggregate_type, aggregate_id, event_type, payload,
			status, retry_count, next_attempt_at, idempotency_key,
			created_at, updated_at
	`

	rows, err := tx.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to delete events from outbox: %w", err)
	}
	defer rows.Close()

	events, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ArchivedEvent, error) {
		var e ArchivedEvent
		err := row.Scan(
			&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &e.Payload,
			&e.Status, &e.RetryCount, &e.NextAttemptAt, &e.IdempotencyKey,
			&e.CreatedAt, &e.UpdatedAt,
		)
		return e, err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan deleted events: %w", err)
	}

	return events, nil
}

// InsertArchived inserts events into the outbox_archive table.
//
// Sets archived_at to current time.
// Returns error if any insert fails (atomicity).
func (r *OutboxArchiveRepository) InsertArchived(
	ctx context.Context,
	tx db.Tx,
	events []ArchivedEvent,
) error {
	if len(events) == 0 {
		return nil
	}

	now := time.Now()

	query := `
		INSERT INTO outbox_archive (
			id, aggregate_type, aggregate_id, event_type, payload,
			status, retry_count, next_attempt_at, idempotency_key,
			created_at, updated_at, archived_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12
		)
	`

	for _, event := range events {
		_, err := tx.Exec(ctx, query,
			event.ID, event.AggregateType, event.AggregateID, event.EventType, event.Payload,
			event.Status, event.RetryCount, event.NextAttemptAt, event.IdempotencyKey,
			event.CreatedAt, event.UpdatedAt, now,
		)
		if err != nil {
			return fmt.Errorf("failed to insert archived event %s: %w", event.ID, err)
		}
	}

	return nil
}

// MoveToArchive atomically moves events from outbox to outbox_archive.
//
// This is the main archival method that:
// 1. Deletes events from outbox with RETURNING
// 2. Inserts them into outbox_archive with archived_at timestamp
//
// All operations happen within the caller's transaction for atomicity.
// If insert fails, the delete will be rolled back.
func (r *OutboxArchiveRepository) MoveToArchive(
	ctx context.Context,
	tx db.Tx,
	ids []uuid.UUID,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	// Step 1: Delete from outbox and get the full data
	events, err := r.DeleteAndReturn(ctx, tx, ids)
	if err != nil {
		return 0, fmt.Errorf("delete from outbox failed: %w", err)
	}

	if len(events) == 0 {
		return 0, nil
	}

	// Step 2: Insert into archive
	// If this fails, the delete will be rolled back by the transaction
	if err := r.InsertArchived(ctx, tx, events); err != nil {
		return 0, fmt.Errorf("insert to archive failed: %w", err)
	}

	return len(events), nil
}

// CountSucceededOlderThan counts succeeded events older than retention period.
// Useful for monitoring and metrics.
func (r *OutboxArchiveRepository) CountSucceededOlderThan(
	ctx context.Context,
	db *db.DB,
	retentionDays int,
) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	query := `
		SELECT COUNT(*)
		FROM outbox
		WHERE status = $1 AND created_at < $2
	`

	var count int
	err := db.Pool().QueryRow(ctx, query, StatusSucceeded, cutoff).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count succeeded events: %w", err)
	}

	return count, nil
}

// CountArchived counts total archived events.
// Useful for monitoring and metrics.
func (r *OutboxArchiveRepository) CountArchived(
	ctx context.Context,
	db *db.DB,
) (int, error) {
	query := `SELECT COUNT(*) FROM outbox_archive`

	var count int
	err := db.Pool().QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count archived events: %w", err)
	}

	return count, nil
}

// isUniqueViolationArchive checks if an error is a PostgreSQL unique constraint violation (23505).
// Renamed to avoid conflict with outbox_repository.isUniqueViolation
func isUniqueViolationArchive(err error) bool {
	var pgErr *pgconn.PgError
	return err != nil && pgErr != nil && pgErr.Code == "23505"
}


