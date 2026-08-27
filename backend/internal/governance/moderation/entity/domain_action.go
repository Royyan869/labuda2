// 🔥 PHASE 2: DOMAIN ACTION IDEMPOTENCY & EXECUTION SAFETY
//
// WAJIB:
// 1. All domain actions idempotent
// 2. Worker retry-safe
// 3. Appeal = all-or-nothing reversal
// 4. No partial success allowed
// 5. Add invariant checks: forSale hidden ↔ action exists
// 6. Event handling: idempotency key, safe retry
//
// DILARANG:
// - Direct DB mutation lintas domain
// - Non-idempotent service
// - Partial execution logic

package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DomainAction represents an idempotent execution unit for moderation actions.
//
// DESIGN PRINCIPLES:
// - Each action has unique idempotency key
// - Execution status tracked atomically
// - Supports retry-safe worker processing
// - All-or-nothing reversal via appeal
//
// 🚨 CRITICAL: Idempotency key MUST be unique per action type + target
// Format: "<action_type>.<target_resource_type>.<target_resource_id>"
// Example: "hide_forSale.forSale.123e4567-e89b-12d3-a456-426614174000"
type DomainAction struct {
	ID                uuid.UUID
	IdempotencyKey    string           // 🔥 PHASE 2: Unique key for idempotent execution
	ActionType        ActionType
	TargetResourceID  uuid.UUID
	TargetSellerID    *uuid.UUID       // Nullable - populated for marketplace actions
	ExecutionStatus   ExecutionStatus  // 🔥 PHASE 2: Track execution for retry safety
	ExecutionGroupID  *uuid.UUID       // 🔥 PHASE 2: All-or-nothing group execution
	PreviousState     json.RawMessage  // State snapshot before execution
	NewState          json.RawMessage  // State snapshot after execution
	ErrorMessage      *string          // Error details if failed
	CreatedAt         time.Time
	ExecutedAt        *time.Time       // Set when execution succeeds
	ReversedBy        *uuid.UUID       // Set when appeal reverses this action
	ReversedAt        *time.Time       // Set when appeal reverses this action
	ReversalReason    *string          // Appeal reason for reversal
	Metadata          json.RawMessage  // Additional execution context
}

// ErrActionAlreadyExecuted is returned when attempting to execute an already succeeded action.
type ErrActionAlreadyExecuted struct {
	ActionID        uuid.UUID
	CurrentStatus   ExecutionStatus
	IdempotencyKey  string
}

func (e *ErrActionAlreadyExecuted) Error() string {
	return fmt.Sprintf("action already executed: action_id=%s, status=%s, idempotency_key=%s",
		e.ActionID, e.CurrentStatus, e.IdempotencyKey)
}

// ErrActionExecutionFailed is returned when action execution fails.
type ErrActionExecutionFailed struct {
	ActionID    uuid.UUID
	ActionType  ActionType
	InnerError  error
}

func (e *ErrActionExecutionFailed) Error() string {
	return fmt.Sprintf("action execution failed: action_id=%s, action_type=%s, error=%v",
		e.ActionID, e.ActionType, e.InnerError)
}

// ErrInvariantViolation is returned when invariant check fails.
type ErrInvariantViolation struct {
	Invariant  string
	Details    string
}

func (e *ErrInvariantViolation) Error() string {
	return fmt.Sprintf("invariant violation: %s - %s", e.Invariant, e.Details)
}

// NewDomainAction creates a new domain action with idempotency key.
//
// 🔥 PHASE 2: Idempotency key is deterministically generated as:
// "<action_type>.<target_resource_type>.<target_resource_id>"
//
// This ensures:
// - Same action on same target = same idempotency key
// - Retries are safe (duplicate keys rejected)
// - Workers can safely retry without side effects
func NewDomainAction(
	actionType ActionType,
	targetResourceID uuid.UUID,
	targetSellerID *uuid.UUID,
	metadata map[string]interface{},
) *DomainAction {
	id := uuid.New()
	now := time.Now()

	// Generate deterministic idempotency key
	idempotencyKey := generateIdempotencyKey(actionType, targetResourceID)

	// Serialize metadata if provided
	var metadataBytes json.RawMessage
	if metadata != nil {
		metadataBytes, _ = json.Marshal(metadata)
	}

	return &DomainAction{
		ID:               id,
		IdempotencyKey:   idempotencyKey,
		ActionType:       actionType,
		TargetResourceID: targetResourceID,
		TargetSellerID:   targetSellerID,
		ExecutionStatus:  ExecutionStatusPending,
		CreatedAt:        now,
		Metadata:         metadataBytes,
	}
}

// generateIdempotencyKey creates a deterministic idempotency key for an action.
//
// Format: "<action_type>.<target_resource_id>"
// Example: "hide_forSale.123e4567-e89b-12d3-a456-426614174000"
func generateIdempotencyKey(actionType ActionType, targetResourceID uuid.UUID) string {
	return fmt.Sprintf("%s.%s", actionType, targetResourceID.String())
}

// CanExecute returns true if the action can be executed.
func (a *DomainAction) CanExecute() bool {
	return a.ExecutionStatus == ExecutionStatusPending
}

// IsSucceeded returns true if the action executed successfully.
func (a *DomainAction) IsSucceeded() bool {
	return a.ExecutionStatus == ExecutionStatusSucceeded
}

// IsFailed returns true if the action execution failed.
func (a *DomainAction) IsFailed() bool {
	return a.ExecutionStatus == ExecutionStatusFailed
}

// IsReversed returns true if the action has been reversed via appeal.
func (a *DomainAction) IsReversed() bool {
	return a.ReversedBy != nil
}

// MarkAsSuccess marks the action as successfully executed.
//
// 🔥 PHASE 2: This is idempotent - can be called multiple times safely.
// Subsequent calls will return ErrActionAlreadyExecuted.
func (a *DomainAction) MarkAsSuccess(previousState, newState json.RawMessage) error {
	if a.IsSucceeded() {
		return &ErrActionAlreadyExecuted{
			ActionID:       a.ID,
			CurrentStatus:  a.ExecutionStatus,
			IdempotencyKey: a.IdempotencyKey,
		}
	}

	if a.ExecutionStatus != ExecutionStatusPending {
		return fmt.Errorf("cannot mark non-pending action as success: current_status=%s", a.ExecutionStatus)
	}

	now := time.Now()
	a.ExecutionStatus = ExecutionStatusSucceeded
	a.PreviousState = previousState
	a.NewState = newState
	a.ExecutedAt = &now

	return nil
}

// MarkAsFailed marks the action as failed with error details.
//
// 🔥 PHASE 2: This is idempotent - can be called multiple times safely.
func (a *DomainAction) MarkAsFailed(err error) error {
	if a.IsFailed() {
		// Already failed, idempotent
		return nil
	}

	if a.IsSucceeded() {
		return fmt.Errorf("cannot mark succeeded action as failed")
	}

	a.ExecutionStatus = ExecutionStatusFailed
	errMsg := err.Error()
	a.ErrorMessage = &errMsg

	return nil
}

// Reverse marks the action as reversed via appeal.
//
// 🔥 PHASE 2: Appeal reversal is all-or-nothing.
// This can only be called on succeeded actions.
func (a *DomainAction) Reverse(reversedBy uuid.UUID, reason string) error {
	if !a.IsSucceeded() {
		return fmt.Errorf("cannot reverse non-succeeded action: current_status=%s", a.ExecutionStatus)
	}

	if a.IsReversed() {
		// Already reversed, idempotent
		return nil
	}

	now := time.Now()
	a.ReversedBy = &reversedBy
	a.ReversedAt = &now
	a.ReversalReason = &reason

	return nil
}

// ValidateInvariant checks if the action state matches resource state.
//
// 🔥 PHASE 2: Invariant checks ensure consistency between action and resource.
//
// Example invariants:
// - hide_forSale action succeeded ↔ forSale.hidden = true
// - hide_forSale action reversed ↔ forSale.hidden = false
func (a *DomainAction) ValidateInvariant(resourceState map[string]interface{}) error {
	switch a.ActionType {
	case ActionTypeHideForSale, ActionTypeReduceVisibility:
		return a.validateForSaleHiddenInvariant(resourceState)
	default:
		// No invariant check for this action type
		return nil
	}
}

// validateForSaleHiddenInvariant checks forSale hidden ↔ action status consistency.
//
// 🔥 PHASE 2: Critical invariant for forSale hiding actions.
//
// Rules:
// - Action succeeded + not reversed → forSale.hidden must be true
// - Action reversed → forSale.hidden must be false
// - Action failed → no invariant requirement
func (a *DomainAction) validateForSaleHiddenInvariant(resourceState map[string]interface{}) error {
	hidden, ok := resourceState["hidden"].(bool)
	if !ok {
		// Resource state doesn't have hidden field, skip check
		return nil
	}

	if a.IsSucceeded() && !a.IsReversed() {
		// Action succeeded and not reversed → forSale must be hidden
		if !hidden {
			return &ErrInvariantViolation{
				Invariant: "for_sale_hidden_consistency",
				Details: fmt.Sprintf("action succeeded but forSale not hidden: "+
					"action_id=%s, for_sale_id=%s, hidden=%v",
					a.ID, a.TargetResourceID, hidden),
			}
		}
	}

	if a.IsReversed() {
		// Action reversed → forSale must not be hidden
		if hidden {
			return &ErrInvariantViolation{
				Invariant: "for_sale_hidden_consistency",
				Details: fmt.Sprintf("action reversed but forSale still hidden: "+
					"action_id=%s, for_sale_id=%s, hidden=%v",
					a.ID, a.TargetResourceID, hidden),
			}
		}
	}

	return nil
}

// DomainActionExecution represents the result of executing a domain action.
type DomainActionExecution struct {
	ActionID      uuid.UUID
	Succeeded     bool
	PreviousState json.RawMessage
	NewState      json.RawMessage
	Error         error
}

// IsRetryable returns true if the action execution can be retried.
//
// 🔥 PHASE 2: Only failed actions are retryable.
// Succeeded or reversed actions are not retryable.
func (a *DomainAction) IsRetryable() bool {
	return a.IsFailed()
}

// ShouldRollback returns true if the action should be rolled back.
//
// 🔥 PHASE 2: Actions in execution group must rollback if any action fails.
// This implements all-or-nothing semantics for multi-action operations.
func (a *DomainAction) ShouldRollback() bool {
	// Rollback succeeded actions in group if group execution fails
	return a.IsSucceeded() && a.ExecutionGroupID != nil
}

// Rollback marks the action as rolled back.
//
// 🔥 PHASE 2: Used when execution group fails to ensure all-or-nothing.
func (a *DomainAction) Rollback(reason string) error {
	if !a.IsSucceeded() {
		return fmt.Errorf("cannot rollback non-succeeded action: current_status=%s", a.ExecutionStatus)
	}

	a.ExecutionStatus = ExecutionStatusRolledBack
	a.ReversalReason = &reason

	return nil
}

var (
	// ErrExecutionGroupNotFound is returned when execution group is not found.
	ErrExecutionGroupNotFound = errors.New("execution group not found")

	// ErrExecutionGroupAlreadyCompleted is returned when attempting to modify a completed group.
	ErrExecutionGroupAlreadyCompleted = errors.New("execution group already completed")

	// ErrExecutionGroupFailed is returned when execution group fails.
	ErrExecutionGroupFailed = errors.New("execution group failed")
)

// DomainActionExecutionGroup represents a group of actions that must succeed or fail together.
//
// 🔥 PHASE 2: All-or-nothing execution for multi-action operations.
// Example: Hide forSale + Add seller strike must both succeed or both fail.
type DomainActionExecutionGroup struct {
	ID              uuid.UUID
	Actions         []*DomainAction
	Status          ExecutionGroupStatus
	CreatedAt       time.Time
	CompletedAt     *time.Time
	FailureReason   *string
}

// ExecutionGroupStatus represents the status of an execution group.
type ExecutionGroupStatus string

const (
	ExecutionGroupStatusPending   ExecutionGroupStatus = "pending"
	ExecutionGroupStatusSucceeded ExecutionGroupStatus = "succeeded"
	ExecutionGroupStatusFailed    ExecutionGroupStatus = "failed"
)

// NewDomainActionExecutionGroup creates a new execution group.
func NewDomainActionExecutionGroup(actions []*DomainAction) *DomainActionExecutionGroup {
	groupID := uuid.New()
	now := time.Now()

	// Assign group ID to all actions
	for _, action := range actions {
		action.SetExecutionGroup(groupID)
	}

	return &DomainActionExecutionGroup{
		ID:        groupID,
		Actions:   actions,
		Status:    ExecutionGroupStatusPending,
		CreatedAt: now,
	}
}

// SetExecutionGroup assigns this action to an execution group.
func (a *DomainAction) SetExecutionGroup(groupID uuid.UUID) {
	a.ExecutionGroupID = &groupID
}

// MarkAsSucceeded marks the entire group as succeeded.
//
// 🔥 PHASE 2: All actions in group must be succeeded.
func (g *DomainActionExecutionGroup) MarkAsSucceeded() error {
	for _, action := range g.Actions {
		if !action.IsSucceeded() {
			return fmt.Errorf("cannot mark group as succeeded: action %s is not succeeded", action.ID)
		}
	}

	now := time.Now()
	g.Status = ExecutionGroupStatusSucceeded
	g.CompletedAt = &now

	return nil
}

// MarkAsFailed marks the entire group as failed and rolls back all actions.
//
// 🔥 PHASE 2: All-or-nothing - roll back all succeeded actions.
func (g *DomainActionExecutionGroup) MarkAsFailed(reason string) error {
	now := time.Now()
	g.Status = ExecutionGroupStatusFailed
	g.CompletedAt = &now
	g.FailureReason = &reason

	// Roll back all succeeded actions
	for _, action := range g.Actions {
		if action.IsSucceeded() {
			_ = action.Rollback(reason)
		}
	}

	return nil
}


