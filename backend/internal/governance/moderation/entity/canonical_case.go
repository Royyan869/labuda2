// DOMAIN: Moderation Domain (governance/moderation/)
// RESPONSIBILITY: Canonical Case entity — governance investigation unit
//
// SLICE 3: This entity is the canonical Case authority.
// It replaces the rejected GovernanceCase super-entity (moderation_cases).
// Canonical authority: LABUDA — CANONICAL MODERATION DESIGN v1 §7-8.
//
// Case is a governance investigation unit for one moderation subject.
// Case ≠ Report ≠ Decision ≠ Enforcement.
// Multiple Reports may point to one Case (canonical cardinality N → 1).

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Note: ErrCaseNotFound is defined in appeal.go and reused here.
// The existing error type serves both appeal and case lookup use cases.

// CaseStatus represents the canonical Case lifecycle.
//
// Design §8: Case lifecycle is open → resolved.
// Decision and Enforcement are NEVER represented as Case status.
type CaseStatus string

const (
	CaseStatusOpen     CaseStatus = "open"
	CaseStatusResolved CaseStatus = "resolved"
)

// IsValid returns true if the status is in the canonical set.
func (s CaseStatus) IsValid() bool {
	return s == CaseStatusOpen || s == CaseStatusResolved
}

// String returns the string representation.
func (s CaseStatus) String() string {
	return string(s)
}

// CanonicalCase represents a governance investigation/review unit for one subject.
//
// Business rules (Design §7):
//   - One active Case per subject (subject_type + subject_id)
//   - Terminal Case is never reopened
//   - New Report after terminal Case → new Case
//   - Case does NOT contain decision fields (Case ≠ Decision)
//   - Case does NOT contain enforcement fields (Case ≠ Enforcement)
//
// Schema: cases table (migration 000055)
// Unique: uniq_active_case_per_subject (subject_type, subject_id) WHERE status = 'open'
type CanonicalCase struct {
	ID          uuid.UUID
	SubjectType ReportTargetType
	SubjectID   uuid.UUID
	Status      CaseStatus
	CreatedAt   time.Time
	ClosedAt    *time.Time
	UpdatedAt   time.Time
}

// NewCanonicalCase creates a new open Case for a subject.
func NewCanonicalCase(subjectType ReportTargetType, subjectID uuid.UUID) *CanonicalCase {
	now := time.Now().UTC()
	return &CanonicalCase{
		ID:          uuid.New(),
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Status:      CaseStatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// IsOpen returns true if the Case is still open for governance resolution.
func (c *CanonicalCase) IsOpen() bool {
	return c.Status == CaseStatusOpen
}

// IsTerminal returns true if the Case has been resolved.
func (c *CanonicalCase) IsTerminal() bool {
	return c.Status == CaseStatusResolved
}

// CanResolve returns true if the Case can transition to resolved.
func (c *CanonicalCase) CanResolve() bool {
	return c.Status == CaseStatusOpen
}

// Resolve transitions the Case to resolved status.
// Business rules (Design §7):
//   - Only open Cases can be resolved
//   - Terminal Cases are never reopened
//   - Resolution happens when a Decision is made
func (c *CanonicalCase) Resolve() error {
	if c.Status != CaseStatusOpen {
		return &ErrCaseAlreadyResolved{CaseID: c.ID, Status: c.Status}
	}
	now := time.Now().UTC()
	c.Status = CaseStatusResolved
	c.ClosedAt = &now
	c.UpdatedAt = now
	return nil
}

// ErrCaseAlreadyResolved is returned when attempting to resolve an already resolved Case.
type ErrCaseAlreadyResolved struct {
	CaseID uuid.UUID
	Status CaseStatus
}

func (e *ErrCaseAlreadyResolved) Error() string {
	return fmt.Sprintf("case already resolved: case_id=%s, status=%s", e.CaseID, e.Status)
}
