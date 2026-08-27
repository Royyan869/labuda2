package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/audit/entity"
	"github.com/labuda/backend/pkg/db"
)

// AuditEventRepository defines the interface for audit event persistence operations.
//
// The audit events are immutable - once created, they are never updated or deleted.
// This ensures a complete and trustworthy audit trail.
type AuditEventRepository interface {
	// Emit inserts a new audit event into the audit log.
	// This is an append-only operation - events are never modified.
	Emit(ctx context.Context, tx db.Tx, event *entity.AuditEvent) error

	// GetByID retrieves an audit event by ID.
	// Returns nil if not found.
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.AuditEvent, error)

	// GetByEntity retrieves audit events for a specific entity, ordered by created_at DESC.
	// Useful for viewing the complete history of an entity.
	GetByEntity(ctx context.Context, tx db.Tx, entityType string, entityID uuid.UUID, limit int) ([]*entity.AuditEvent, error)

	// GetByActor retrieves audit events performed by a specific actor, ordered by created_at DESC.
	// Useful for auditing user/admin activity.
	GetByActor(ctx context.Context, tx db.Tx, actorType string, actorID uuid.UUID, limit int) ([]*entity.AuditEvent, error)

	// GetByEventType retrieves audit events of a specific type, ordered by created_at DESC.
	// Useful for analyzing specific event patterns.
	GetByEventType(ctx context.Context, tx db.Tx, eventType string, limit int) ([]*entity.AuditEvent, error)

	// GetByTimeRange retrieves audit events within a time range, ordered by created_at DESC.
	// Useful for generating audit reports.
	GetByTimeRange(ctx context.Context, tx db.Tx, startTime, endTime string, limit int) ([]*entity.AuditEvent, error)
}

// AuditEventRepositoryImpl implements AuditEventRepository using PostgreSQL.
type AuditEventRepositoryImpl struct{}

// NewAuditEventRepository creates a new AuditEventRepositoryImpl.
func NewAuditEventRepository() *AuditEventRepositoryImpl {
	return &AuditEventRepositoryImpl{}
}

// Emit inserts a new audit event into the audit log.
func (r *AuditEventRepositoryImpl) Emit(ctx context.Context, tx db.Tx, event *entity.AuditEvent) error {
	query := `
		INSERT INTO audit_events (
			id, event_type, entity_type, entity_id,
			actor_type, actor_id, payload_json, created_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8
		)
	`

	_, err := tx.Exec(ctx, query,
		event.ID,
		event.EventType,
		event.EntityType,
		event.EntityID,
		event.ActorType,
		event.ActorID,
		event.PayloadJSON,
		event.CreatedAt,
	)

	if err != nil {
		return &ErrEmitFailed{
			EventType: event.EventType,
			EntityID:  event.EntityID,
			Err:       err,
		}
	}

	return nil
}

// GetByID retrieves an audit event by ID.
func (r *AuditEventRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.AuditEvent, error) {
	query := `
		SELECT id, event_type, entity_type, entity_id,
		       actor_type, actor_id, payload_json, created_at
		FROM audit_events
		WHERE id = $1
	`

	var event entity.AuditEvent
	err := tx.QueryRow(ctx, query, id).Scan(
		&event.ID,
		&event.EventType,
		&event.EntityType,
		&event.EntityID,
		&event.ActorType,
		&event.ActorID,
		&event.PayloadJSON,
		&event.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, err
	}

	return &event, nil
}

// GetByEntity retrieves audit events for a specific entity.
func (r *AuditEventRepositoryImpl) GetByEntity(ctx context.Context, tx db.Tx, entityType string, entityID uuid.UUID, limit int) ([]*entity.AuditEvent, error) {
	query := `
		SELECT id, event_type, entity_type, entity_id,
		       actor_type, actor_id, payload_json, created_at
		FROM audit_events
		WHERE entity_type = $1 AND entity_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := tx.Query(ctx, query, entityType, entityID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetByActor retrieves audit events performed by a specific actor.
func (r *AuditEventRepositoryImpl) GetByActor(ctx context.Context, tx db.Tx, actorType string, actorID uuid.UUID, limit int) ([]*entity.AuditEvent, error) {
	query := `
		SELECT id, event_type, entity_type, entity_id,
		       actor_type, actor_id, payload_json, created_at
		FROM audit_events
		WHERE actor_type = $1 AND actor_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := tx.Query(ctx, query, actorType, actorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetByEventType retrieves audit events of a specific type.
func (r *AuditEventRepositoryImpl) GetByEventType(ctx context.Context, tx db.Tx, eventType string, limit int) ([]*entity.AuditEvent, error) {
	query := `
		SELECT id, event_type, entity_type, entity_id,
		       actor_type, actor_id, payload_json, created_at
		FROM audit_events
		WHERE event_type = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := tx.Query(ctx, query, eventType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetByTimeRange retrieves audit events within a time range.
func (r *AuditEventRepositoryImpl) GetByTimeRange(ctx context.Context, tx db.Tx, startTime, endTime string, limit int) ([]*entity.AuditEvent, error) {
	query := `
		SELECT id, event_type, entity_type, entity_id,
		       actor_type, actor_id, payload_json, created_at
		FROM audit_events
		WHERE created_at >= $1 AND created_at <= $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := tx.Query(ctx, query, startTime, endTime, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// scanRows scans a rowset into AuditEvent structs.
func (r *AuditEventRepositoryImpl) scanRows(rows pgx.Rows) ([]*entity.AuditEvent, error) {
	var events []*entity.AuditEvent

	for rows.Next() {
		var event entity.AuditEvent
		err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.EntityType,
			&event.EntityID,
			&event.ActorType,
			&event.ActorID,
			&event.PayloadJSON,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

// =============================================================================
// ERROR TYPES
// =============================================================================

// ErrEmitFailed is returned when audit event emission fails.
// This error should be logged but should NOT break the business flow.
type ErrEmitFailed struct {
	EventType string
	EntityID  uuid.UUID
	Err       error
}

func (e *ErrEmitFailed) Error() string {
	return "audit emit failed: " + e.EventType
}

func (e *ErrEmitFailed) Unwrap() error {
	return e.Err
}


