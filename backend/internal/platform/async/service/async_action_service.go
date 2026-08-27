// Package service provides async action result tracking.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/pkg/db"
)

var (
	// ErrActionResultNotFound is returned when no action result exists.
	ErrActionResultNotFound = errors.New("async action result not found")
)

// Status represents the status of an async action.
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

// ActionResult represents the result of an async operation.
type ActionResult struct {
	ID             uuid.UUID
	ActionID       uuid.UUID
	UserID         uuid.UUID
	ActionType     string
	EntityType     string
	EntityID       uuid.UUID
	Status         Status
	EventID        *uuid.UUID
	ErrorMessage   *string
	ErrorCode      *string
	ResultData     map[string]interface{}
	DecisionVersion *int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

// CreateOptions holds optional parameters for creating an action result.
type CreateOptions struct {
	EventID        *uuid.UUID
	DecisionVersion *int64
	Metadata       map[string]interface{}
}

// UpdateOptions holds optional parameters for updating an action result.
type UpdateOptions struct {
	Status       *Status
	EventID      *uuid.UUID
	ErrorMessage *string
	ErrorCode    *string
	ResultData   map[string]interface{}
}

// Service handles async action result operations.
//
// DESIGN PRINCIPLES:
// - Track status of all async operations
// - Link to outbox events for full traceability
// - Store decision version for optimistic concurrency
// - Queryable by user, entity, status
type Service struct {
	db *db.DB
}

// NewService creates a new AsyncActionService.
func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

// Create creates a new async action result in pending state.
// Returns the ID of the created result.
func (s *Service) Create(
	ctx context.Context,
	actionID uuid.UUID,
	userID uuid.UUID,
	actionType string,
	entityType string,
	entityID uuid.UUID,
	options CreateOptions,
) (uuid.UUID, error) {
	id := uuid.New()
	now := time.Now()

	var eventIDPtr *uuid.UUID
	if options.EventID != nil {
		eventIDPtr = options.EventID
	}

	var decisionVersionPtr *int64
	if options.DecisionVersion != nil {
		decisionVersionPtr = options.DecisionVersion
	}

	var resultDataJSON []byte
	if options.Metadata != nil {
		bytes, err := json.Marshal(options.Metadata)
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to marshal result data: %w", err)
		}
		resultDataJSON = bytes
	}

	query := `
		INSERT INTO async_action_results (
			id, action_id, user_id, action_type, entity_type, entity_id,
			status, event_id, decision_version, result_data, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := s.db.Pool().Exec(
		ctx,
		query,
		id, actionID, userID, actionType, entityType, entityID,
		StatusPending, eventIDPtr, decisionVersionPtr, resultDataJSON, now, now,
	)

	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create async action result: %w", err)
	}

	return id, nil
}

// Get retrieves an async action result by action ID.
func (s *Service) Get(ctx context.Context, actionID uuid.UUID) (*ActionResult, error) {
	var result ActionResult
	var resultDataJSON []byte

	query := `
		SELECT id, action_id, user_id, action_type, entity_type, entity_id,
		       status, event_id, error_message, error_code, result_data,
		       decision_version, created_at, updated_at, completed_at
		FROM async_action_results
		WHERE action_id = $1
	`

	err := s.db.Pool().QueryRow(ctx, query, actionID).Scan(
		&result.ID,
		&result.ActionID,
		&result.UserID,
		&result.ActionType,
		&result.EntityType,
		&result.EntityID,
		&result.Status,
		&result.EventID,
		&result.ErrorMessage,
		&result.ErrorCode,
		&resultDataJSON,
		&result.DecisionVersion,
		&result.CreatedAt,
		&result.UpdatedAt,
		&result.CompletedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrActionResultNotFound
		}
		return nil, fmt.Errorf("failed to get async action result: %w", err)
	}

	// Parse result data
	if resultDataJSON != nil {
		if err := json.Unmarshal(resultDataJSON, &result.ResultData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result data: %w", err)
		}
	}

	return &result, nil
}

// Update updates an async action result.
func (s *Service) Update(ctx context.Context, actionID uuid.UUID, options UpdateOptions) error {
	// Build dynamic update query
	updates := []string{"updated_at = NOW()"}
	args := []interface{}{actionID}
	argIdx := 2

	if options.Status != nil {
		updates = append(updates, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*options.Status))
		argIdx++

		// Set completed_at when moving to terminal state
		if *options.Status == StatusCompleted || *options.Status == StatusFailed {
			updates = append(updates, fmt.Sprintf("completed_at = CASE WHEN completed_at IS NULL THEN NOW() ELSE completed_at END"))
		}
	}

	if options.EventID != nil {
		updates = append(updates, fmt.Sprintf("event_id = $%d", argIdx))
		args = append(args, *options.EventID)
		argIdx++
	}

	if options.ErrorMessage != nil {
		updates = append(updates, fmt.Sprintf("error_message = $%d", argIdx))
		args = append(args, *options.ErrorMessage)
		argIdx++
	}

	if options.ErrorCode != nil {
		updates = append(updates, fmt.Sprintf("error_code = $%d", argIdx))
		args = append(args, *options.ErrorCode)
		argIdx++
	}

	if options.ResultData != nil {
		resultDataJSON, err := json.Marshal(options.ResultData)
		if err != nil {
			return fmt.Errorf("failed to marshal result data: %w", err)
		}
		updates = append(updates, fmt.Sprintf("result_data = $%d", argIdx))
		args = append(args, resultDataJSON)
		argIdx++
	}

	query := fmt.Sprintf(`
		UPDATE async_action_results
		SET %s
		WHERE action_id = $1
	`, fmt.Sprintf("%s", updates))

	_, err := s.db.Pool().Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update async action result: %w", err)
	}

	return nil
}

// MarkProcessing marks an action as being processed.
func (s *Service) MarkProcessing(ctx context.Context, actionID uuid.UUID) error {
	return s.Update(ctx, actionID, UpdateOptions{
		Status: ptrStatus(StatusProcessing),
	})
}

// MarkCompleted marks an action as completed with result data.
func (s *Service) MarkCompleted(
	ctx context.Context,
	actionID uuid.UUID,
	eventID uuid.UUID,
	resultData map[string]interface{},
) error {
	return s.Update(ctx, actionID, UpdateOptions{
		Status:     ptrStatus(StatusCompleted),
		EventID:    &eventID,
		ResultData: resultData,
	})
}

// MarkFailed marks an action as failed with error details.
func (s *Service) MarkFailed(
	ctx context.Context,
	actionID uuid.UUID,
	errorCode string,
	errorMessage string,
) error {
	return s.Update(ctx, actionID, UpdateOptions{
		Status:       ptrStatus(StatusFailed),
		ErrorCode:    &errorCode,
		ErrorMessage: &errorMessage,
	})
}

// GetByUser retrieves async action results for a user, paginated.
func (s *Service) GetByUser(
	ctx context.Context,
	userID uuid.UUID,
	status *Status,
	limit, offset int,
) ([]*ActionResult, error) {
	query := `
		SELECT id, action_id, user_id, action_type, entity_type, entity_id,
		       status, event_id, error_message, error_code, result_data,
		       decision_version, created_at, updated_at, completed_at
		FROM async_action_results
		WHERE user_id = $1
	`
	args := []interface{}{userID}
	argIdx := 2

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(*status))
		argIdx++
	}

	query += " ORDER BY created_at DESC LIMIT $" + fmt.Sprint(argIdx) + " OFFSET $" + fmt.Sprint(argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query async action results: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// GetByEntity retrieves async action results for an entity, paginated.
func (s *Service) GetByEntity(
	ctx context.Context,
	entityType string,
	entityID uuid.UUID,
	status *Status,
	limit, offset int,
) ([]*ActionResult, error) {
	query := `
		SELECT id, action_id, user_id, action_type, entity_type, entity_id,
		       status, event_id, error_message, error_code, result_data,
		       decision_version, created_at, updated_at, completed_at
		FROM async_action_results
		WHERE entity_type = $1 AND entity_id = $2
	`
	args := []interface{}{entityType, entityID}
	argIdx := 3

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(*status))
		argIdx++
	}

	query += " ORDER BY created_at DESC LIMIT $" + fmt.Sprint(argIdx) + " OFFSET $" + fmt.Sprint(argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query async action results by entity: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// GetPending retrieves all pending or processing async actions.
// Useful for worker processing.
func (s *Service) GetPending(ctx context.Context, limit int) ([]*ActionResult, error) {
	query := `
		SELECT id, action_id, user_id, action_type, entity_type, entity_id,
		       status, event_id, error_message, error_code, result_data,
		       decision_version, created_at, updated_at, completed_at
		FROM async_action_results
		WHERE status IN ('pending', 'processing')
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := s.db.Pool().Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending async actions: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// GetPendingByType retrieves pending actions for a specific action type.
func (s *Service) GetPendingByType(
	ctx context.Context,
	actionType string,
	limit int,
) ([]*ActionResult, error) {
	query := `
		SELECT id, action_id, user_id, action_type, entity_type, entity_id,
		       status, event_id, error_message, error_code, result_data,
		       decision_version, created_at, updated_at, completed_at
		FROM async_action_results
		WHERE action_type = $1 AND status IN ('pending', 'processing')
		ORDER BY created_at ASC
		LIMIT $2
	`

	rows, err := s.db.Pool().Query(ctx, query, actionType, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending async actions by type: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// GetByEventID retrieves async action results linked to a specific event.
func (s *Service) GetByEventID(ctx context.Context, eventID uuid.UUID) (*ActionResult, error) {
	var result ActionResult
	var resultDataJSON []byte

	query := `
		SELECT id, action_id, user_id, action_type, entity_type, entity_id,
		       status, event_id, error_message, error_code, result_data,
		       decision_version, created_at, updated_at, completed_at
		FROM async_action_results
		WHERE event_id = $1
	`

	err := s.db.Pool().QueryRow(ctx, query, eventID).Scan(
		&result.ID,
		&result.ActionID,
		&result.UserID,
		&result.ActionType,
		&result.EntityType,
		&result.EntityID,
		&result.Status,
		&result.EventID,
		&result.ErrorMessage,
		&result.ErrorCode,
		&resultDataJSON,
		&result.DecisionVersion,
		&result.CreatedAt,
		&result.UpdatedAt,
		&result.CompletedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrActionResultNotFound
		}
		return nil, fmt.Errorf("failed to get async action result by event: %w", err)
	}

	// Parse result data
	if resultDataJSON != nil {
		if err := json.Unmarshal(resultDataJSON, &result.ResultData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result data: %w", err)
		}
	}

	return &result, nil
}

// Cleanup removes old completed/failed action results.
func (s *Service) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	query := `
		DELETE FROM async_action_results
		WHERE status IN ('completed', 'failed')
		  AND completed_at < NOW() - ($1::interval)
	`

	result, err := s.db.Pool().Exec(ctx, query, fmt.Sprintf("%d seconds", int(olderThan.Seconds())))
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup async action results: %w", err)
	}

	return int(result.RowsAffected()), nil
}

// scanRows scans a rowset into ActionResult structs.
func (s *Service) scanRows(rows pgx.Rows) ([]*ActionResult, error) {
	var results []*ActionResult

	for rows.Next() {
		var result ActionResult
		var resultDataJSON []byte

		err := rows.Scan(
			&result.ID,
			&result.ActionID,
			&result.UserID,
			&result.ActionType,
			&result.EntityType,
			&result.EntityID,
			&result.Status,
			&result.EventID,
			&result.ErrorMessage,
			&result.ErrorCode,
			&resultDataJSON,
			&result.DecisionVersion,
			&result.CreatedAt,
			&result.UpdatedAt,
			&result.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan async action result row: %w", err)
		}

		// Parse result data
		if resultDataJSON != nil {
			if err := json.Unmarshal(resultDataJSON, &result.ResultData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal result data: %w", err)
			}
		}

		results = append(results, &result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating async action result rows: %w", err)
	}

	return results, nil
}

// Helper function to create a pointer to Status
func ptrStatus(s Status) *Status {
	return &s
}


