package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
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
type OutboxRepository interface {
	// InsertEvent creates a new outbox event within the provided transaction.
	//
	// IDEMPOTENCY:
	//   - idempotency_key is deterministically generated as: "<eventType>.<entityID>"
	//     Example: "offer.created.123e4567-e89b-12d3-a456-426614174000"
	//   - If a unique constraint violation occurs (duplicate idempotency_key),
	//     the error is treated as success - the event already exists.
	//   - This makes event emission idempotent and safe for retries.
	//
	// This should be called within the same transaction as the domain state change
	// to guarantee atomicity.
	InsertEvent(
		ctx context.Context,
		tx db.Tx,
		eventType string,
		entityID uuid.UUID,
		payload []byte,
	) error

	// InsertTx inserts an outbox event with an explicit idempotency key.
	//
	// This is a convenience method for workers that need to emit events
	// with a specific idempotency key pattern. The payload must be JSON-serializable.
	//
	// The idempotency key follows the format: "<eventType>.<idempotencyKey>"
	InsertTx(
		ctx context.Context,
		tx db.Tx,
		eventType string,
		payload any,
		idempotencyKey string,
	) error

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
	FetchPendingBatch(
		ctx context.Context,
		tx db.Tx,
		limit int,
	) ([]Event, error)

	// MarkProcessing marks an event as being processed.
	//
	// ATOMIC STATUS TRANSITION:
	// Accepts events in 'pending' or 'failed' status and transitions them to 'processing'.
	// This enables retry: failed events ready for retry can be claimed by a worker.
	// Returns ErrInvalidStatusTransition if the event is not in pending or failed state.
	MarkProcessing(
		ctx context.Context,
		tx db.Tx,
		eventID uuid.UUID,
	) error

	// MarkSucceeded marks an event as successfully processed.
	//
	// Updates the event status to 'succeeded'.
	// Returns ErrEventNotFound if the event doesn't exist.
	MarkSucceeded(
		ctx context.Context,
		tx db.Tx,
		eventID uuid.UUID,
	) error

	// MarkFailedWithRetry marks an event as failed with explicit retry count and next attempt time.
	//
	// This allows the worker to control backoff logic while keeping the status update atomic.
	// Returns ErrEventNotFound if the event doesn't exist.
	MarkFailedWithRetry(
		ctx context.Context,
		tx db.Tx,
		eventID uuid.UUID,
		retryCount int,
		nextAttemptAt time.Time,
	) error

	// MoveToDeadLetter moves an event to dead_letter status.
	//
	// Called when max retry attempts have been exhausted.
	// Returns ErrEventNotFound if the event doesn't exist.
	MoveToDeadLetter(
		ctx context.Context,
		tx db.Tx,
		eventID uuid.UUID,
	) error
}


