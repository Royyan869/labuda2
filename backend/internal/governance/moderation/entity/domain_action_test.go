// 🔥 PHASE 2: DOMAIN ACTION IDEMPOTENCY & SAFETY TESTS
//
// Tests for:
// - Idempotent execution
// - All-or-nothing reversal
// - Invariant checks
// - Retry safety

package entity

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// TestNewDomainAction tests creating a new domain action.
func TestNewDomainAction(t *testing.T) {
	actionType := ActionTypeHideForSale
	targetResourceID := uuid.New()
	targetSellerID := uuid.New()
	metadata := map[string]interface{}{
		"reason": "policy_violation",
	}

	action := NewDomainAction(actionType, targetResourceID, &targetSellerID, metadata)

	if action.ID == uuid.Nil {
		t.Error("Expected action ID to be set")
	}

	if action.IdempotencyKey == "" {
		t.Error("Expected idempotency key to be generated")
	}

	expectedKey := generateIdempotencyKey(actionType, targetResourceID)
	if action.IdempotencyKey != expectedKey {
		t.Errorf("Expected idempotency key %s, got %s", expectedKey, action.IdempotencyKey)
	}

	if action.ActionType != actionType {
		t.Errorf("Expected action type %s, got %s", actionType, action.ActionType)
	}

	if action.TargetResourceID != targetResourceID {
		t.Errorf("Expected target resource ID %s, got %s", targetResourceID, action.TargetResourceID)
	}

	if action.ExecutionStatus != ExecutionStatusPending {
		t.Errorf("Expected execution status %s, got %s", ExecutionStatusPending, action.ExecutionStatus)
	}
}

// TestGenerateIdempotencyKey tests idempotency key generation.
func TestGenerateIdempotencyKey(t *testing.T) {
	actionType := ActionTypeHideForSale
	targetResourceID := uuid.New()

	key := generateIdempotencyKey(actionType, targetResourceID)
	expected := string(actionType) + "." + targetResourceID.String()

	if key != expected {
		t.Errorf("Expected idempotency key %s, got %s", expected, key)
	}
}

// TestIdempotencyKeyDeterministic tests that idempotency keys are deterministic.
func TestIdempotencyKeyDeterministic(t *testing.T) {
	actionType := ActionTypeHideForSale
	targetResourceID := uuid.New()

	key1 := generateIdempotencyKey(actionType, targetResourceID)
	key2 := generateIdempotencyKey(actionType, targetResourceID)

	if key1 != key2 {
		t.Error("Expected idempotency keys to be deterministic")
	}
}

// TestMarkAsSuccess tests marking an action as succeeded.
func TestMarkAsSuccess(t *testing.T) {
	action := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
	previousState := json.RawMessage(`{"hidden": false}`)
	newState := json.RawMessage(`{"hidden": true}`)

	err := action.MarkAsSuccess(previousState, newState)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if action.ExecutionStatus != ExecutionStatusSucceeded {
		t.Errorf("Expected execution status %s, got %s", ExecutionStatusSucceeded, action.ExecutionStatus)
	}

	if action.ExecutedAt == nil {
		t.Error("Expected executed_at to be set")
	}
}

// TestMarkAsSuccessIdempotent tests that marking as success is idempotent.
func TestMarkAsSuccessIdempotent(t *testing.T) {
	action := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
	previousState := json.RawMessage(`{"hidden": false}`)
	newState := json.RawMessage(`{"hidden": true}`)

	// First call should succeed
	err := action.MarkAsSuccess(previousState, newState)
	if err != nil {
		t.Errorf("Expected no error on first call, got %v", err)
	}

	// Second call should return ErrActionAlreadyExecuted
	err = action.MarkAsSuccess(previousState, newState)
	if err == nil {
		t.Error("Expected error on second call")
	}

	if !Implements(err) {
		t.Errorf("Expected ErrActionAlreadyExecuted, got %T", err)
	}
}

// TestMarkAsFailed tests marking an action as failed.
func TestMarkAsFailed(t *testing.T) {
	action := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
	testErr := &TestError{Message: "execution failed"}

	err := action.MarkAsFailed(testErr)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if action.ExecutionStatus != ExecutionStatusFailed {
		t.Errorf("Expected execution status %s, got %s", ExecutionStatusFailed, action.ExecutionStatus)
	}

	if action.ErrorMessage == nil {
		t.Error("Expected error message to be set")
	}
}

// TestMarkAsFailedIdempotent tests that marking as failed is idempotent.
func TestMarkAsFailedIdempotent(t *testing.T) {
	action := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
	testErr := &TestError{Message: "execution failed"}

	// First call should succeed
	err := action.MarkAsFailed(testErr)
	if err != nil {
		t.Errorf("Expected no error on first call, got %v", err)
	}

	// Second call should be idempotent (no error)
	err = action.MarkAsFailed(testErr)
	if err != nil {
		t.Errorf("Expected no error on second call, got %v", err)
	}
}

// TestReverse tests reversing an action.
func TestReverse(t *testing.T) {
	action := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
	previousState := json.RawMessage(`{"hidden": false}`)
	newState := json.RawMessage(`{"hidden": true}`)

	// First mark as succeeded
	err := action.MarkAsSuccess(previousState, newState)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Then reverse
	reversedBy := uuid.New()
	reason := "appeal approved"
	err = action.Reverse(reversedBy, reason)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if action.ReversedBy == nil {
		t.Error("Expected reversed_by to be set")
	}

	if *action.ReversedBy != reversedBy {
		t.Errorf("Expected reversed_by %s, got %s", reversedBy, *action.ReversedBy)
	}

	if action.ReversalReason == nil {
		t.Error("Expected reversal_reason to be set")
	}

	if *action.ReversalReason != reason {
		t.Errorf("Expected reversal_reason %s, got %s", reason, *action.ReversalReason)
	}
}

// TestReverseIdempotent tests that reversing is idempotent.
func TestReverseIdempotent(t *testing.T) {
	action := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
	previousState := json.RawMessage(`{"hidden": false}`)
	newState := json.RawMessage(`{"hidden": true}`)

	// First mark as succeeded
	err := action.MarkAsSuccess(previousState, newState)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// First reversal
	reversedBy := uuid.New()
	reason := "appeal approved"
	err = action.Reverse(reversedBy, reason)
	if err != nil {
		t.Errorf("Expected no error on first reversal, got %v", err)
	}

	// Second reversal should be idempotent
	err = action.Reverse(reversedBy, reason)
	if err != nil {
		t.Errorf("Expected no error on second reversal, got %v", err)
	}
}

// TestReverseNonSucceededAction tests that reversing a non-succeeded action fails.
func TestReverseNonSucceededAction(t *testing.T) {
	action := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)

	reversedBy := uuid.New()
	reason := "appeal approved"
	err := action.Reverse(reversedBy, reason)
	if err == nil {
		t.Error("Expected error when reversing non-succeeded action")
	}
}

// TestValidateInvariantForSaleHidden tests forSale hidden invariant validation.
func TestValidateInvariantForSaleHidden(t *testing.T) {
	tests := []struct {
		name          string
		actionStatus  ExecutionStatus
		isReversed    bool
		hidden        bool
		expectError   bool
	}{
		{
			name:         "succeeded not reversed - forSale hidden",
			actionStatus: ExecutionStatusSucceeded,
			isReversed:   false,
			hidden:       true,
			expectError:  false,
		},
		{
			name:         "succeeded not reversed - forSale not hidden (error)",
			actionStatus: ExecutionStatusSucceeded,
			isReversed:   false,
			hidden:       false,
			expectError:  true,
		},
		{
			name:         "reversed - forSale not hidden",
			actionStatus: ExecutionStatusSucceeded,
			isReversed:   true,
			hidden:       false,
			expectError:  false,
		},
		{
			name:         "reversed - forSale still hidden (error)",
			actionStatus: ExecutionStatusSucceeded,
			isReversed:   true,
			hidden:       true,
			expectError:  true,
		},
		{
			name:         "failed - no invariant check",
			actionStatus: ExecutionStatusFailed,
			isReversed:   false,
			hidden:       false,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
			action.ExecutionStatus = tt.actionStatus

			if tt.isReversed {
				reversedBy := uuid.New()
				reason := "appeal approved"
				action.ReversedBy = &reversedBy
				reversalReason := reason
				action.ReversalReason = &reversalReason
			}

			resourceState := map[string]interface{}{
				"hidden": tt.hidden,
			}

			err := action.ValidateInvariant(resourceState)

			if tt.expectError && err == nil {
				t.Error("Expected invariant violation error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

// TestDomainActionExecutionGroup tests execution group all-or-nothing semantics.
func TestDomainActionExecutionGroup(t *testing.T) {
	action1 := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
	action2 := NewDomainAction(ActionTypeWarnSeller, uuid.New(), nil, nil)

	group := NewDomainActionExecutionGroup([]*DomainAction{action1, action2})

	if group.ID == uuid.Nil {
		t.Error("Expected group ID to be set")
	}

	if group.Status != ExecutionGroupStatusPending {
		t.Errorf("Expected group status %s, got %s", ExecutionGroupStatusPending, group.Status)
	}

	if action1.ExecutionGroupID == nil || *action1.ExecutionGroupID != group.ID {
		t.Error("Expected action1 execution_group_id to be set")
	}

	if action2.ExecutionGroupID == nil || *action2.ExecutionGroupID != group.ID {
		t.Error("Expected action2 execution_group_id to be set")
	}
}

// TestMarkAsSucceeded tests marking a group as succeeded.
func TestMarkAsSucceeded(t *testing.T) {
	action1 := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
	action2 := NewDomainAction(ActionTypeWarnSeller, uuid.New(), nil, nil)

	group := NewDomainActionExecutionGroup([]*DomainAction{action1, action2})

	// Mark both actions as succeeded
	previousState := json.RawMessage(`{}`)
	newState := json.RawMessage(`{}`)
	action1.MarkAsSuccess(previousState, newState)
	action2.MarkAsSuccess(previousState, newState)

	// Mark group as succeeded
	err := group.MarkAsSucceeded()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if group.Status != ExecutionGroupStatusSucceeded {
		t.Errorf("Expected group status %s, got %s", ExecutionGroupStatusSucceeded, group.Status)
	}

	if group.CompletedAt == nil {
		t.Error("Expected completed_at to be set")
	}
}

// TestMarkAsFailedGroup tests marking a group as failed and rolling back actions.
func TestMarkAsFailedGroup(t *testing.T) {
	action1 := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
	action2 := NewDomainAction(ActionTypeWarnSeller, uuid.New(), nil, nil)

	group := NewDomainActionExecutionGroup([]*DomainAction{action1, action2})

	// Mark first action as succeeded
	previousState := json.RawMessage(`{}`)
	newState := json.RawMessage(`{}`)
	action1.MarkAsSuccess(previousState, newState)

	// Mark group as failed
	reason := "execution failed"
	err := group.MarkAsFailed(reason)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if group.Status != ExecutionGroupStatusFailed {
		t.Errorf("Expected group status %s, got %s", ExecutionGroupStatusFailed, group.Status)
	}

	if group.FailureReason == nil || *group.FailureReason != reason {
		t.Errorf("Expected failure_reason %s, got %v", reason, group.FailureReason)
	}

	if action1.ExecutionStatus != ExecutionStatusRolledBack {
		t.Errorf("Expected action1 status %s, got %s", ExecutionStatusRolledBack, action1.ExecutionStatus)
	}
}

// TestIsRetryable tests retryable status check.
func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name         string
		status       ExecutionStatus
		expectRetry  bool
	}{
		{
			name:        "pending - not retryable",
			status:      ExecutionStatusPending,
			expectRetry: false,
		},
		{
			name:        "succeeded - not retryable",
			status:      ExecutionStatusSucceeded,
			expectRetry: false,
		},
		{
			name:        "failed - retryable",
			status:      ExecutionStatusFailed,
			expectRetry: true,
		},
		{
			name:        "rolled_back - not retryable",
			status:      ExecutionStatusRolledBack,
			expectRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
			action.ExecutionStatus = tt.status

			result := action.IsRetryable()
			if result != tt.expectRetry {
				t.Errorf("Expected retryable %v, got %v", tt.expectRetry, result)
			}
		})
	}
}

// TestRollback tests rolling back an action.
func TestRollback(t *testing.T) {
	action := NewDomainAction(ActionTypeHideForSale, uuid.New(), nil, nil)
	previousState := json.RawMessage(`{}`)
	newState := json.RawMessage(`{}`)

	// Mark as succeeded
	err := action.MarkAsSuccess(previousState, newState)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Rollback
	reason := "execution group failed"
	err = action.Rollback(reason)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if action.ExecutionStatus != ExecutionStatusRolledBack {
		t.Errorf("Expected execution status %s, got %s", ExecutionStatusRolledBack, action.ExecutionStatus)
	}

	if action.ReversalReason == nil || *action.ReversalReason != reason {
		t.Errorf("Expected reversal_reason %s, got %v", reason, action.ReversalReason)
	}
}

// =============================================================================
// TEST UTILITIES
// =============================================================================

// TestError is a test error type.
type TestError struct {
	Message string
}

func (e *TestError) Error() string {
	return e.Message
}

// Implements is a helper to check if error implements a specific interface.
func Implements(err error) bool {
	_, ok := err.(*ErrActionAlreadyExecuted)
	return ok
}


