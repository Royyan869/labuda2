package entity_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGovernanceCase_Enforce_RequiresNote verifies the entity invariant that
// an enforce decision must have a non-empty, non-whitespace note.
func TestGovernanceCase_Enforce_RequiresNote(t *testing.T) {
	t.Run("nil note rejected", func(t *testing.T) {
		kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "spam")
		err := kase.Enforce(uuid.New(), nil)
		require.Error(t, err)
		var target *entity.ErrEnforceRequiresNote
		assert.ErrorAs(t, err, &target)
		assert.Equal(t, entity.GovernanceCaseStatusPending, kase.Status, "case must not be mutated on error")
		assert.Nil(t, kase.ReviewedBy, "reviewer must not be set on error")
		assert.Nil(t, kase.DecisionNote, "note must not be set on error")
	})

	t.Run("empty string rejected", func(t *testing.T) {
		kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "spam")
		empty := ""
		err := kase.Enforce(uuid.New(), &empty)
		require.Error(t, err)
		var target *entity.ErrEnforceRequiresNote
		assert.ErrorAs(t, err, &target)
		assert.Equal(t, entity.GovernanceCaseStatusPending, kase.Status)
	})

	t.Run("whitespace-only rejected", func(t *testing.T) {
		kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "spam")
		spaces := "   \t\n  "
		err := kase.Enforce(uuid.New(), &spaces)
		require.Error(t, err)
		var target *entity.ErrEnforceRequiresNote
		assert.ErrorAs(t, err, &target)
		assert.Equal(t, entity.GovernanceCaseStatusPending, kase.Status)
	})

	t.Run("valid note accepted", func(t *testing.T) {
		kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "spam")
		note := "Content violates community guidelines: hate speech"
		adminID := uuid.New()
		err := kase.Enforce(adminID, &note)
		require.NoError(t, err)
		assert.Equal(t, entity.GovernanceCaseStatusEnforced, kase.Status)
		assert.Equal(t, &adminID, kase.ReviewedBy)
		require.NotNil(t, kase.DecisionNote)
		assert.Equal(t, note, *kase.DecisionNote)
	})

	t.Run("note with leading/trailing spaces accepted if non-empty", func(t *testing.T) {
		kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "spam")
		note := "  valid note  "
		err := kase.Enforce(uuid.New(), &note)
		require.NoError(t, err)
		assert.Equal(t, entity.GovernanceCaseStatusEnforced, kase.Status)
	})
}

// TestGovernanceCase_Enforce_DoesNotMutateOnInvalidNote verifies that no state
// change occurs when enforce is called with an invalid note.
func TestGovernanceCase_Enforce_DoesNotMutateOnInvalidNote(t *testing.T) {
	kase := entity.NewGovernanceCase(entity.ResourceTypeForSale, uuid.New(), uuid.New(), "fake fixed-price sale")
	originalID := kase.ID
	err := kase.Enforce(uuid.New(), nil)
	require.Error(t, err)
	// All state must be unchanged
	assert.Equal(t, entity.GovernanceCaseStatusPending, kase.Status)
	assert.Equal(t, originalID, kase.ID)
	assert.Nil(t, kase.ReviewedBy)
	assert.Nil(t, kase.ReviewedAt)
	assert.Nil(t, kase.DecisionNote)
}

// TestGovernanceCase_Approve_NilNote_OK verifies approve still accepts nil note (no regression).
func TestGovernanceCase_Approve_NilNote_OK(t *testing.T) {
	kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "test")
	err := kase.Approve(uuid.New(), nil)
	require.NoError(t, err)
	assert.Equal(t, entity.GovernanceCaseStatusApproved, kase.Status)
}

// TestGovernanceCase_Reject_NilNote_OK verifies reject still accepts nil note (no regression).
func TestGovernanceCase_Reject_NilNote_OK(t *testing.T) {
	kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "test")
	err := kase.Reject(uuid.New(), nil)
	require.NoError(t, err)
	assert.Equal(t, entity.GovernanceCaseStatusRejected, kase.Status)
}

// TestErrEnforceRequiresNote_ErrorMessage verifies the error message is stable.
func TestErrEnforceRequiresNote_ErrorMessage(t *testing.T) {
	err := &entity.ErrEnforceRequiresNote{}
	assert.Equal(t, "enforce decision requires a non-empty note for audit trail", err.Error())
}


