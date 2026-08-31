// DOMAIN: Moderation Domain (governance/moderation/)
// RESPONSIBILITY: Canonical Decision entity — immutable historical governance record
//
// SLICE 4: This entity is the canonical Decision authority.
// Decision is an immutable historical record of a governance outcome.
// Decision ≠ Case ≠ Report ≠ Enforcement.
// One Case may have multiple immutable Decisions (append-only).
//
// Canonical boundary:
//   Report → Case → Decision → Enforcement
//
// Decision outcome vocabulary:
//   no_violation — content complies, no enforcement needed
//   violation    — policy violated, enforcement warranted
//
// Decision is immutable: no Update, no Delete. A new Decision is created
// instead (e.g. Appeal produces Decision #2, never mutating Decision #1).

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DecisionOutcome represents the canonical outcome of a governance Decision.
// LOCKED: Only two values. No action, sanction, or enforcement_status fields.
type DecisionOutcome string

const (
	// DecisionOutcomeNoViolation means the content complies with policy.
	DecisionOutcomeNoViolation DecisionOutcome = "no_violation"

	// DecisionOutcomeViolation means the policy was violated.
	DecisionOutcomeViolation DecisionOutcome = "violation"
)

// IsValid returns true if the outcome is in the canonical set.
func (o DecisionOutcome) IsValid() bool {
	return o == DecisionOutcomeNoViolation || o == DecisionOutcomeViolation
}

// String returns the string representation.
func (o DecisionOutcome) String() string {
	return string(o)
}

// Decision represents an immutable historical governance record.
//
// Business rules:
//   - Decision is append-only (immutable): no UPDATE, no DELETE
//   - One Case may have multiple Decisions (Case 1 → N Decision)
//   - Decision #2 does not modify Decision #1
//   - Decision belongs to exactly one Case (case_id NOT NULL, FK)
//   - Decision records who made it (decided_by) and when (created_at)
//   - Decision outcome is strictly {no_violation, violation}
//
// Schema: decisions table (migration 000055)
// Trigger: trg_decisions_immutable blocks UPDATE
// Index: idx_decisions_case (case_id, created_at DESC)
type Decision struct {
	ID           uuid.UUID
	CaseID       uuid.UUID
	DecidedBy    uuid.UUID
	Outcome      DecisionOutcome
	DecisionNote *string
	CreatedAt    time.Time
}

// NewDecision creates a new immutable Decision.
//
// Business rules:
//   - Outcome must be valid (no_violation or violation)
//   - CaseID and DecidedBy are required
func NewDecision(
	caseID uuid.UUID,
	decidedBy uuid.UUID,
	outcome DecisionOutcome,
	decisionNote *string,
) (*Decision, error) {
	if !outcome.IsValid() {
		return nil, &ErrInvalidDecisionOutcome{Outcome: outcome}
	}

	now := time.Now().UTC()
	return &Decision{
		ID:           uuid.New(),
		CaseID:       caseID,
		DecidedBy:    decidedBy,
		Outcome:      outcome,
		DecisionNote: decisionNote,
		CreatedAt:    now,
	}, nil
}

// ErrInvalidDecisionOutcome is returned when an invalid outcome value is provided.
type ErrInvalidDecisionOutcome struct {
	Outcome DecisionOutcome
}

func (e *ErrInvalidDecisionOutcome) Error() string {
	return fmt.Sprintf("invalid decision outcome: %s (valid: no_violation, violation)", e.Outcome)
}

// ErrDecisionCaseNotFound is returned when the Case referenced by a Decision does not exist.
type ErrDecisionCaseNotFound struct {
	CaseID uuid.UUID
}

func (e *ErrDecisionCaseNotFound) Error() string {
	return fmt.Sprintf("decision case not found: %s", e.CaseID)
}
