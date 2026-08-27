package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/pkg/db"
)

var (
	// ErrEventNotFound is returned when an event is not found.
	ErrEventNotFound = errors.New("event not found")

	// ErrInvalidStatusTransition is returned when an invalid status transition is attempted.
	ErrInvalidStatusTransition = errors.New("invalid status transition")
)

// EventStatus represents the possible states of an outbox event.
type EventStatus string

const (
	// StatusPending means the event is waiting to be processed.
	StatusPending EventStatus = "pending"
	// StatusProcessing means the event is currently being processed by a worker.
	StatusProcessing EventStatus = "processing"
	// StatusSucceeded means the event was successfully delivered.
	StatusSucceeded EventStatus = "succeeded"
	// StatusFailed means the event failed and will be retried.
	StatusFailed EventStatus = "failed"
	// StatusDeadLetter means the event exceeded max retries and will not be retried.
	StatusDeadLetter EventStatus = "dead_letter"
)

// Event represents an outbox event.
// This is a simple, focused entity without business logic dependencies.
type Event struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       []byte
	Status        EventStatus
	RetryCount    int
	NextAttemptAt time.Time // When this event is ready for retry
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// OutboxRepository handles database operations for the outbox table.
//
// DESIGN PRINCIPLES:
// - No GORM dependency - uses pgx via pkg/db
// - FOR UPDATE SKIP LOCKED for concurrent worker safety
// - Idempotent operations
// - Atomic status transitions
// - No business logic (doesn't know about Payment, Order, etc.)
type OutboxRepository struct {
	db *db.DB
}

// NewOutboxRepository creates a new OutboxRepository instance.
func NewOutboxRepository(database *db.DB) *OutboxRepository {
	return &OutboxRepository{
		db: database,
	}
}

// InsertEvent creates a new outbox event within the provided transaction.
//
// IDEMPOTENCY:
//   - idempotency_key is deterministically generated as: "<eventType>.<entityID>"
//     Example: "offer.created.123e4567-e89b-12d3-a456-426614174000"
//   - Duplicate inserts are silently ignored at the SQL layer via
//     ON CONFLICT (idempotency_key) DO NOTHING. This is REQUIRED so that a
//     duplicate does not raise a 23505 unique violation, which would otherwise
//     abort the surrounding PostgreSQL transaction (subsequent statements would
//     fail with 25P02 "current transaction is aborted").
//   - The Go-side isUniqueViolation check is kept as a defensive fallback only.
//
// This should be called within the same transaction as the domain state change
// to guarantee atomicity.
func (r *OutboxRepository) InsertEvent(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	entityID uuid.UUID,
	payload []byte,
) error {
	now := time.Now()
	id := uuid.New()

	// Deterministic idempotency key: eventType.entityID
	idempotencyKey := fmt.Sprintf("%s.%s", eventType, entityID.String())

	// Extract aggregate type from event type (e.g., "offer.created" -> "offer")
	aggregateType := eventType
	if dotIdx := strings.Index(eventType, "."); dotIdx > 0 {
		aggregateType = eventType[:dotIdx]
	}

	// ON CONFLICT (idempotency_key) DO NOTHING ensures duplicate inserts are
	// ignored at the SQL level instead of raising a 23505 unique violation that
	// would abort the surrounding transaction.
	query := `
		INSERT INTO outbox (
			id, aggregate_type, aggregate_id, event_type, payload,
			status, retry_count, next_attempt_at, created_at, updated_at, idempotency_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (idempotency_key) DO NOTHING
	`

	_, err := tx.Exec(ctx, query,
		id, aggregateType, entityID, eventType, payload,
		StatusPending, 0, now, now, now, idempotencyKey,
	)

	// Defensive fallback: ON CONFLICT should already swallow duplicates, but if
	// the constraint definition ever changes (e.g. a partial index that doesn't
	// match the inferred conflict target), still treat 23505 as a no-op.
	if err != nil && isUniqueViolation(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}


// InsertTx inserts an outbox event with an explicit idempotency key.
//
// This is a convenience method for workers that need to emit events
// with a specific idempotency key pattern. The payload must be JSON-serializable.
//
// The idempotency key follows the format: "<eventType>.<idempotencyKey>"
//
// IDEMPOTENCY: duplicate inserts are silently ignored at the SQL layer via
// ON CONFLICT (idempotency_key) DO NOTHING so a duplicate does not abort the
// surrounding transaction. The Go-side isUniqueViolation check is kept as a
// defensive fallback only.
func (r *OutboxRepository) InsertTx(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	payload any,
	idempotencyKey string,
) error {
	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create deterministic idempotency key: "<eventType>.<idempotencyKey>"
	fullIdempotencyKey := fmt.Sprintf("%s.%s", eventType, idempotencyKey)

	// Use a nil UUID as entity ID since we're using custom idempotency key
	// The actual entity relationship is carried in the payload
	entityID := uuid.Nil

	now := time.Now()
	id := uuid.New()

	// Extract aggregate type from event type (e.g., "seller.subscription.expired" -> "seller.subscription")
	aggregateType := eventType
	if dotIdx := strings.LastIndex(eventType, "."); dotIdx > 0 {
		aggregateType = eventType[:dotIdx]
	}

	// ON CONFLICT (idempotency_key) DO NOTHING ensures duplicate inserts are
	// ignored at the SQL level instead of raising a 23505 unique violation that
	// would abort the surrounding transaction.
	query := `
		INSERT INTO outbox (
			id, aggregate_type, aggregate_id, event_type, payload,
			status, retry_count, next_attempt_at, created_at, updated_at, idempotency_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (idempotency_key) DO NOTHING
	`

	_, err = tx.Exec(ctx, query,
		id, aggregateType, entityID, eventType, payloadBytes,
		StatusPending, 0, now, now, now, fullIdempotencyKey,
	)

	// Defensive fallback: ON CONFLICT should already swallow duplicates.
	if err != nil && isUniqueViolation(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}


// FetchPendingBatch atomically locks and returns a batch of pending events.
//
// CRITICAL: Uses FOR UPDATE SKIP LOCKED to:
// - Lock selected rows for this transaction
// - Skip rows already locked by other workers
// - Never block waiting for locks
//
// Query conditions:
// - status IN ('pending', 'failed')
// - next_attempt_at <= NOW()
//
// This ensures multiple workers can process different batches concurrently
// without race conditions.
func (r *OutboxRepository) FetchPendingBatch(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]Event, error) {
	query := `
		SELECT
			id, aggregate_type, aggregate_id, event_type, payload,
			status, retry_count, next_attempt_at, created_at, updated_at
		FROM outbox
		WHERE status IN ($1, $2)
		  AND next_attempt_at <= NOW()
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $3
	`

	rows, err := tx.Query(ctx, query, StatusPending, StatusFailed, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pending batch: %w", err)
	}
	defer rows.Close()

	events, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Event, error) {
		var e Event
		err := row.Scan(
			&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &e.Payload,
			&e.Status, &e.RetryCount, &e.NextAttemptAt, &e.CreatedAt, &e.UpdatedAt,
		)
		return e, err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan events: %w", err)
	}

	return events, nil
}

// MarkProcessing marks an event as being processed.
//
// ATOMIC STATUS TRANSITION:
// Only updates if the current status is 'pending'.
// Returns ErrInvalidStatusTransition if the event is not in pending state.
func (r *OutboxRepository) MarkProcessing(
	ctx context.Context,
	tx db.Tx,
	eventID uuid.UUID,
) error {
	now := time.Now()
	query := `
		UPDATE outbox
		SET status = $1, updated_at = $2
		WHERE id = $3 AND status = $4
	`

	result, err := tx.Exec(ctx, query, StatusProcessing, now, eventID, StatusPending)
	if err != nil {
		return fmt.Errorf("failed to mark event as processing: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrInvalidStatusTransition
	}

	return nil
}

// MarkSucceeded marks an event as successfully processed.
//
// Updates the event status to 'succeeded'.
// Returns ErrInvalidStatusTransition if the event doesn't exist.
func (r *OutboxRepository) MarkSucceeded(
	ctx context.Context,
	tx db.Tx,
	eventID uuid.UUID,
) error {
	now := time.Now()
	query := `
		UPDATE outbox
		SET status = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := tx.Exec(ctx, query, StatusSucceeded, now, eventID)
	if err != nil {
		return fmt.Errorf("failed to mark event as succeeded: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrEventNotFound
	}

	return nil
}

// MarkFailedWithRetry marks an event as failed with explicit retry count and next attempt time.
//
// This allows the worker to control backoff logic while keeping the status update atomic.
// Returns ErrEventNotFound if the event doesn't exist.
func (r *OutboxRepository) MarkFailedWithRetry(
	ctx context.Context,
	tx db.Tx,
	eventID uuid.UUID,
	retryCount int,
	nextAttemptAt time.Time,
) error {
	query := `
		UPDATE outbox
		SET status = $1, retry_count = $2, next_attempt_at = $3, updated_at = $4
		WHERE id = $5
	`

	result, err := tx.Exec(ctx, query, StatusFailed, retryCount, nextAttemptAt, time.Now(), eventID)
	if err != nil {
		return fmt.Errorf("failed to mark event as failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrEventNotFound
	}

	return nil
}

// MoveToDeadLetter moves an event to dead_letter status.
//
// Called when max retry attempts have been exhausted.
// Returns ErrEventNotFound if the event doesn't exist.
func (r *OutboxRepository) MoveToDeadLetter(
	ctx context.Context,
	tx db.Tx,
	eventID uuid.UUID,
) error {
	query := `
		UPDATE outbox
		SET status = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := tx.Exec(ctx, query, StatusDeadLetter, time.Now(), eventID)
	if err != nil {
		return fmt.Errorf("failed to move event to dead letter: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrEventNotFound
	}

	return nil
}

// ResetStuckEvents resets events that have been stuck in 'processing' status for too long.
//
// SELF-HEALING MECHANISM:
// This function addresses the critical gap where events can get stuck if a worker crashes
// after marking an event as 'processing' but before completing it.
//
// SAFETY:
// - Increments retry_count to track recovery attempts
// - Sets next_attempt_at to NOW() for immediate reprocessing
// - Uses WHERE clause to only reset genuinely stuck events
// - Returns count of recovered events for monitoring
//
// Parameters:
// - timeout: Duration after which an event is considered stuck
//
// Returns:
// - Number of events recovered
// - Error if database operation fails
func (r *OutboxRepository) ResetStuckEvents(
	ctx context.Context,
	timeout time.Duration,
) (int, error) {
	query := `
		UPDATE outbox
		SET
			status = $1,
			retry_count = retry_count + 1,
			next_attempt_at = NOW(),
			updated_at = NOW()
		WHERE status = $2
		  AND updated_at < NOW() - INTERVAL '1 microsecond' * $3
	`

	result, err := r.db.Pool().Exec(ctx, query,
		StatusPending,
		StatusProcessing,
		int64(timeout.Microseconds()),
	)

	if err != nil {
		return 0, fmt.Errorf("failed to reset stuck events: %w", err)
	}

	rowsAffected := result.RowsAffected()
	return int(rowsAffected), nil
}

// isUniqueViolation checks if an error is a PostgreSQL unique constraint violation (23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}


