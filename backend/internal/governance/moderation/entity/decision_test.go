package entity

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDecision_Success verifies Decision creation with valid inputs.
func TestNewDecision_Success(t *testing.T) {
	caseID := uuid.New()
	decidedBy := uuid.New()
	note := "violation confirmed after review"

	d, err := NewDecision(caseID, decidedBy, DecisionOutcomeViolation, &note)

	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, caseID, d.CaseID)
	assert.Equal(t, decidedBy, d.DecidedBy)
	assert.Equal(t, DecisionOutcomeViolation, d.Outcome)
	assert.NotNil(t, d.DecisionNote)
	assert.Equal(t, note, *d.DecisionNote)
	assert.False(t, d.CreatedAt.IsZero())
}

// TestNewDecision_NoViolation verifies no_violation outcome.
func TestNewDecision_NoViolation(t *testing.T) {
	d, err := NewDecision(uuid.New(), uuid.New(), DecisionOutcomeNoViolation, nil)

	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, DecisionOutcomeNoViolation, d.Outcome)
	assert.Nil(t, d.DecisionNote)
}

// TestNewDecision_InvalidOutcome verifies invalid outcome is rejected.
func TestNewDecision_InvalidOutcome(t *testing.T) {
	d, err := NewDecision(uuid.New(), uuid.New(), DecisionOutcome("invalid"), nil)

	assert.Nil(t, d)
	assert.Error(t, err)
	var invalidErr *ErrInvalidDecisionOutcome
	assert.ErrorAs(t, err, &invalidErr)
	assert.Equal(t, DecisionOutcome("invalid"), invalidErr.Outcome)
}

// TestNewDecision_EmptyOutcome verifies empty outcome is rejected.
func TestNewDecision_EmptyOutcome(t *testing.T) {
	d, err := NewDecision(uuid.New(), uuid.New(), DecisionOutcome(""), nil)

	assert.Nil(t, d)
	assert.Error(t, err)
}

// TestDecisionOutcome_IsValid verifies all valid outcomes.
func TestDecisionOutcome_IsValid(t *testing.T) {
	assert.True(t, DecisionOutcomeNoViolation.IsValid())
	assert.True(t, DecisionOutcomeViolation.IsValid())
	assert.False(t, DecisionOutcome("approve").IsValid())
	assert.False(t, DecisionOutcome("reject").IsValid())
	assert.False(t, DecisionOutcome("enforce").IsValid())
	assert.False(t, DecisionOutcome("removed").IsValid())
	assert.False(t, DecisionOutcome("").IsValid())
}

// TestNewDecision_EachHasUniqueID verifies each Decision gets a unique ID.
func TestNewDecision_EachHasUniqueID(t *testing.T) {
	d1, _ := NewDecision(uuid.New(), uuid.New(), DecisionOutcomeViolation, nil)
	d2, _ := NewDecision(uuid.New(), uuid.New(), DecisionOutcomeNoViolation, nil)

	assert.NotEqual(t, d1.ID, d2.ID)
}

// TestErrInvalidDecisionOutcome_Message verifies error message.
func TestErrInvalidDecisionOutcome_Message(t *testing.T) {
	err := &ErrInvalidDecisionOutcome{Outcome: "enforce"}
	assert.Contains(t, err.Error(), "enforce")
	assert.Contains(t, err.Error(), "no_violation")
	assert.Contains(t, err.Error(), "violation")
}

// TestErrDecisionCaseNotFound_Message verifies error message.
func TestErrDecisionCaseNotFound_Message(t *testing.T) {
	caseID := uuid.New()
	err := &ErrDecisionCaseNotFound{CaseID: caseID}
	assert.Contains(t, err.Error(), caseID.String())
}
