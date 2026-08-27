// 🔥 PHASE 2: DOMAIN ACTION REPOSITORY IMPLEMENTATION
//
// Provides persistence operations for idempotent domain actions.

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

// DomainActionRepositoryImpl handles domain action persistence using pgx-based DB layer.
type DomainActionRepositoryImpl struct{}

// NewDomainActionRepository creates a new DomainActionRepository.
func NewDomainActionRepository() DomainActionRepository {
	return &DomainActionRepositoryImpl{}
}

// Create persists a new domain action within a transaction.
func (r *DomainActionRepositoryImpl) Create(
	ctx context.Context,
	tx interface{},
	action *entity.DomainAction,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	var executionGroupID *uuid.UUID
	if action.ExecutionGroupID != nil {
		executionGroupID = action.ExecutionGroupID
	}

	var targetSellerID *uuid.UUID
	if action.TargetSellerID != nil {
		targetSellerID = action.TargetSellerID
	}

	var executedAt *time.Time
	if action.ExecutedAt != nil {
		executedAt = action.ExecutedAt
	}

	var reversedBy *uuid.UUID
	if action.ReversedBy != nil {
		reversedBy = action.ReversedBy
	}

	var reversedAt *time.Time
	if action.ReversedAt != nil {
		reversedAt = action.ReversedAt
	}

	_, err := dbTx.Exec(ctx, `
		INSERT INTO domain_actions (
			id, idempotency_key, action_type, target_resource_id, target_seller_id,
			execution_status, execution_group_id, previous_state, new_state,
			error_message, created_at, executed_at, reversed_by, reversed_at,
			reversal_reason, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`,
		action.ID,
		action.IdempotencyKey,
		string(action.ActionType),
		action.TargetResourceID,
		targetSellerID,
		string(action.ExecutionStatus),
		executionGroupID,
		action.PreviousState,
		action.NewState,
		action.ErrorMessage,
		action.CreatedAt,
		executedAt,
		reversedBy,
		reversedAt,
		action.ReversalReason,
		action.Metadata,
	)

	if err != nil {
		return fmt.Errorf("create domain action failed: %w", err)
	}

	return nil
}

// CreateWithIdempotencyCheck creates a domain action with idempotency check.
//
// 🔥 PHASE 2: If an action with the same idempotency key exists,
// the existing action is returned instead of creating a duplicate.
func (r *DomainActionRepositoryImpl) CreateWithIdempotencyCheck(
	ctx context.Context,
	tx interface{},
	action *entity.DomainAction,
) (*entity.DomainAction, error) {
	// First, try to create the action
	err := r.Create(ctx, tx, action)
	if err == nil {
		// Successfully created
		return action, nil
	}

	// Check if error is a unique constraint violation on idempotency_key
	if isUniqueViolation(err) {
		// Action with this idempotency key already exists
		// Fetch and return the existing action
		existing, err := r.GetByIdempotencyKey(ctx, tx, action.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch existing action by idempotency key: %w", err)
		}
		return existing, nil
	}

	// Some other error occurred
	return nil, fmt.Errorf("create domain action failed: %w", err)
}

// GetByID retrieves a domain action by ID without locking.
func (r *DomainActionRepositoryImpl) GetByID(
	ctx context.Context,
	tx interface{},
	actionID uuid.UUID,
) (*entity.DomainAction, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	return r.scanOne(dbTx.QueryRow(ctx, `
		SELECT id, idempotency_key, action_type, target_resource_id, target_seller_id,
		       execution_status, execution_group_id, previous_state, new_state,
		       error_message, created_at, executed_at, reversed_by, reversed_at,
		       reversal_reason, metadata
		FROM domain_actions
		WHERE id = $1
	`, actionID))
}

// GetForUpdate retrieves a domain action with FOR UPDATE lock.
func (r *DomainActionRepositoryImpl) GetForUpdate(
	ctx context.Context,
	tx interface{},
	actionID uuid.UUID,
) (*entity.DomainAction, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	return r.scanOne(dbTx.QueryRow(ctx, `
		SELECT id, idempotency_key, action_type, target_resource_id, target_seller_id,
		       execution_status, execution_group_id, previous_state, new_state,
		       error_message, created_at, executed_at, reversed_by, reversed_at,
		       reversal_reason, metadata
		FROM domain_actions
		WHERE id = $1
		FOR UPDATE
	`, actionID))
}

// GetByIdempotencyKey retrieves a domain action by idempotency key.
func (r *DomainActionRepositoryImpl) GetByIdempotencyKey(
	ctx context.Context,
	tx interface{},
	idempotencyKey string,
) (*entity.DomainAction, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	return r.scanOne(dbTx.QueryRow(ctx, `
		SELECT id, idempotency_key, action_type, target_resource_id, target_seller_id,
		       execution_status, execution_group_id, previous_state, new_state,
		       error_message, created_at, executed_at, reversed_by, reversed_at,
		       reversal_reason, metadata
		FROM domain_actions
		WHERE idempotency_key = $1
	`, idempotencyKey))
}

// GetByGovernanceCaseID retrieves all domain actions for a governance case.
func (r *DomainActionRepositoryImpl) GetByGovernanceCaseID(
	ctx context.Context,
	tx interface{},
	governanceCaseID uuid.UUID,
) ([]*entity.DomainAction, error) {
	_, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	// TODO: Add governance_case_id to domain_actions table
	// For now, return empty list
	return []*entity.DomainAction{}, nil
}

// GetByTargetResourceID retrieves all domain actions for a target resource.
func (r *DomainActionRepositoryImpl) GetByTargetResourceID(
	ctx context.Context,
	tx interface{},
	targetResourceID uuid.UUID,
) ([]*entity.DomainAction, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT id, idempotency_key, action_type, target_resource_id, target_seller_id,
		       execution_status, execution_group_id, previous_state, new_state,
		       error_message, created_at, executed_at, reversed_by, reversed_at,
		       reversal_reason, metadata
		FROM domain_actions
		WHERE target_resource_id = $1
		ORDER BY created_at ASC
	`

	rows, err := dbTx.Query(ctx, query, targetResourceID)
	if err != nil {
		return nil, fmt.Errorf("get domain actions by target resource failed: %w", err)
	}
	defer rows.Close()

	var actions []*entity.DomainAction
	for rows.Next() {
		action, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan domain action failed: %w", err)
		}
		actions = append(actions, action)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("scan domain actions failed: %w", rows.Err())
	}

	return actions, nil
}

// Update persists domain action changes within a transaction.
func (r *DomainActionRepositoryImpl) Update(
	ctx context.Context,
	tx interface{},
	action *entity.DomainAction,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	var executionGroupID *uuid.UUID
	if action.ExecutionGroupID != nil {
		executionGroupID = action.ExecutionGroupID
	}


	var executedAt *time.Time
	if action.ExecutedAt != nil {
		executedAt = action.ExecutedAt
	}

	var reversedBy *uuid.UUID
	if action.ReversedBy != nil {
		reversedBy = action.ReversedBy
	}

	var reversedAt *time.Time
	if action.ReversedAt != nil {
		reversedAt = action.ReversedAt
	}

	_, err := dbTx.Exec(ctx, `
		UPDATE domain_actions
		SET execution_status = $2,
		    execution_group_id = $3,
		    previous_state = $4,
		    new_state = $5,
		    error_message = $6,
		    executed_at = $7,
		    reversed_by = $8,
		    reversed_at = $9,
		    reversal_reason = $10,
		    metadata = $11
		WHERE id = $1
	`,
		action.ID,
		string(action.ExecutionStatus),
		executionGroupID,
		action.PreviousState,
		action.NewState,
		action.ErrorMessage,
		executedAt,
		reversedBy,
		reversedAt,
		action.ReversalReason,
		action.Metadata,
	)

	if err != nil {
		return fmt.Errorf("update domain action failed: %w", err)
	}

	return nil
}

// ListPending retrieves all pending actions awaiting execution.
func (r *DomainActionRepositoryImpl) ListPending(
	ctx context.Context,
	tx interface{},
	limit, offset int,
) ([]*entity.DomainAction, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT id, idempotency_key, action_type, target_resource_id, target_seller_id,
		       execution_status, execution_group_id, previous_state, new_state,
		       error_message, created_at, executed_at, reversed_by, reversed_at,
		       reversal_reason, metadata
		FROM domain_actions
		WHERE execution_status = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := dbTx.Query(ctx, query, entity.ExecutionStatusPending, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list pending domain actions failed: %w", err)
	}
	defer rows.Close()

	var actions []*entity.DomainAction
	for rows.Next() {
		action, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan domain action failed: %w", err)
		}
		actions = append(actions, action)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("scan domain actions failed: %w", rows.Err())
	}

	return actions, nil
}

// ListFailed retrieves all failed actions that can be retried.
func (r *DomainActionRepositoryImpl) ListFailed(
	ctx context.Context,
	tx interface{},
	limit, offset int,
) ([]*entity.DomainAction, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT id, idempotency_key, action_type, target_resource_id, target_seller_id,
		       execution_status, execution_group_id, previous_state, new_state,
		       error_message, created_at, executed_at, reversed_by, reversed_at,
		       reversal_reason, metadata
		FROM domain_actions
		WHERE execution_status = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := dbTx.Query(ctx, query, entity.ExecutionStatusFailed, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list failed domain actions failed: %w", err)
	}
	defer rows.Close()

	var actions []*entity.DomainAction
	for rows.Next() {
		action, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan domain action failed: %w", err)
		}
		actions = append(actions, action)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("scan domain actions failed: %w", rows.Err())
	}

	return actions, nil
}

// ListByExecutionGroup retrieves all actions in an execution group.
func (r *DomainActionRepositoryImpl) ListByExecutionGroup(
	ctx context.Context,
	tx interface{},
	executionGroupID uuid.UUID,
) ([]*entity.DomainAction, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT id, idempotency_key, action_type, target_resource_id, target_seller_id,
		       execution_status, execution_group_id, previous_state, new_state,
		       error_message, created_at, executed_at, reversed_by, reversed_at,
		       reversal_reason, metadata
		FROM domain_actions
		WHERE execution_group_id = $1
		ORDER BY created_at ASC
	`

	rows, err := dbTx.Query(ctx, query, executionGroupID)
	if err != nil {
		return nil, fmt.Errorf("list domain actions by execution group failed: %w", err)
	}
	defer rows.Close()

	var actions []*entity.DomainAction
	for rows.Next() {
		action, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan domain action failed: %w", err)
		}
		actions = append(actions, action)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("scan domain actions failed: %w", rows.Err())
	}

	return actions, nil
}

// MarkAsSucceeded marks an action as succeeded atomically.
func (r *DomainActionRepositoryImpl) MarkAsSucceeded(
	ctx context.Context,
	tx interface{},
	actionID uuid.UUID,
	previousState, newState []byte,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	now := time.Now()

	_, err := dbTx.Exec(ctx, `
		UPDATE domain_actions
		SET execution_status = $2,
		    previous_state = $3,
		    new_state = $4,
		    executed_at = $5
		WHERE id = $1 AND execution_status = $6
	`, actionID, entity.ExecutionStatusSucceeded, previousState, newState, now, entity.ExecutionStatusPending)

	if err != nil {
		return fmt.Errorf("mark domain action as succeeded failed: %w", err)
	}

	return nil
}

// MarkAsFailed marks an action as failed atomically.
func (r *DomainActionRepositoryImpl) MarkAsFailed(
	ctx context.Context,
	tx interface{},
	actionID uuid.UUID,
	errorMessage string,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		UPDATE domain_actions
		SET execution_status = $2,
		    error_message = $3
		WHERE id = $1 AND execution_status = $4
	`, actionID, entity.ExecutionStatusFailed, &errorMessage, entity.ExecutionStatusPending)

	if err != nil {
		return fmt.Errorf("mark domain action as failed failed: %w", err)
	}

	return nil
}

// scanOne scans a single domain action from a row.
func (r *DomainActionRepositoryImpl) scanOne(row pgx.Row) (*entity.DomainAction, error) {
	var id, targetResourceID uuid.UUID
	var idempotencyKey, actionType, executionStatus string
	var targetSellerID *uuid.UUID
	var executionGroupID *uuid.UUID
	var previousState, newState, metadata json.RawMessage
	var errorMessage *string
	var createdAt time.Time
	var executedAt *time.Time
	var reversedBy *uuid.UUID
	var reversedAt *time.Time
	var reversalReason *string

	err := row.Scan(
		&id, &idempotencyKey, &actionType, &targetResourceID, &targetSellerID,
		&executionStatus, &executionGroupID, &previousState, &newState,
		&errorMessage, &createdAt, &executedAt, &reversedBy, &reversedAt,
		&reversalReason, &metadata,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("domain action not found")
		}
		return nil, fmt.Errorf("scan domain action failed: %w", err)
	}

	return &entity.DomainAction{
		ID:               id,
		IdempotencyKey:   idempotencyKey,
		ActionType:       entity.ActionType(actionType),
		TargetResourceID: targetResourceID,
		TargetSellerID:   targetSellerID,
		ExecutionStatus:  entity.ExecutionStatus(executionStatus),
		ExecutionGroupID: executionGroupID,
		PreviousState:    previousState,
		NewState:         newState,
		ErrorMessage:     errorMessage,
		CreatedAt:        createdAt,
		ExecutedAt:       executedAt,
		ReversedBy:       reversedBy,
		ReversedAt:       reversedAt,
		ReversalReason:   reversalReason,
		Metadata:         metadata,
	}, nil
}

// scanRow scans a domain action from a row.
func (r *DomainActionRepositoryImpl) scanRow(rows pgx.Rows) (*entity.DomainAction, error) {
	var id, targetResourceID uuid.UUID
	var idempotencyKey, actionType, executionStatus string
	var targetSellerID *uuid.UUID
	var executionGroupID *uuid.UUID
	var previousState, newState, metadata json.RawMessage
	var errorMessage *string
	var createdAt time.Time
	var executedAt *time.Time
	var reversedBy *uuid.UUID
	var reversedAt *time.Time
	var reversalReason *string

	err := rows.Scan(
		&id, &idempotencyKey, &actionType, &targetResourceID, &targetSellerID,
		&executionStatus, &executionGroupID, &previousState, &newState,
		&errorMessage, &createdAt, &executedAt, &reversedBy, &reversedAt,
		&reversalReason, &metadata,
	)

	if err != nil {
		return nil, fmt.Errorf("scan domain action failed: %w", err)
	}

	return &entity.DomainAction{
		ID:               id,
		IdempotencyKey:   idempotencyKey,
		ActionType:       entity.ActionType(actionType),
		TargetResourceID: targetResourceID,
		TargetSellerID:   targetSellerID,
		ExecutionStatus:  entity.ExecutionStatus(executionStatus),
		ExecutionGroupID: executionGroupID,
		PreviousState:    previousState,
		NewState:         newState,
		ErrorMessage:     errorMessage,
		CreatedAt:        createdAt,
		ExecutedAt:       executedAt,
		ReversedBy:       reversedBy,
		ReversedAt:       reversedAt,
		ReversalReason:   reversalReason,
		Metadata:         metadata,
	}, nil
}

// isUniqueViolation checks if an error is a PostgreSQL unique constraint violation (23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}


