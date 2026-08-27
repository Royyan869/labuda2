// DOMAIN: Moderation Domain (governance/moderation/)
// RESPONSIBILITY: Enforcement, actions, consequences, appeals

package entity

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GovernanceCase represents a moderation case created from a user report.
//
// DOMAIN TERMINOLOGY:
// - REPORT: User action of flagging content for moderation
// - CASE: Internal moderation object that tracks the review process
// - APPEAL: User contest of a moderation decision
//
// Domain: Moderation Domain (governance/moderation/)
// Responsibility: Case intake, enforcement, actions, consequences, appeals
// Events: EMITS events (never calls domain services directly)
//
// STRICT BOUNDARY RULES:
// - This entity does NOT mutate financial state
// - This entity does NOT modify orders, offers, or withdrawals
// - This entity only tracks enforcement decisions and actions
// - Downstream domains react to governance events via workers
type GovernanceCase struct {
	ID           uuid.UUID
	ResourceType ResourceType
	ResourceID   uuid.UUID
	Status       GovernanceCaseStatus
	ReportedBy   uuid.UUID // User who reported the content
	ReviewedBy   *uuid.UUID
	Reason       string     // Original report reason
	DecisionNote *string
	ReviewedAt   *time.Time
	CreatedAt    time.Time
}

// GovernanceCaseStatus represents the current status of a moderation case.
type GovernanceCaseStatus string

const (
	GovernanceCaseStatusPending  GovernanceCaseStatus = "pending"  // Awaiting review
	GovernanceCaseStatusApproved GovernanceCaseStatus = "approved" // Content complies, no action needed
	GovernanceCaseStatusRejected GovernanceCaseStatus = "rejected" // Case dismissed as false positive
	GovernanceCaseStatusEnforced GovernanceCaseStatus = "enforced" // Actions applied
)

// Decision represents the admin's review decision.
type Decision string

const (
	DecisionApprove Decision = "approve" // Content complies, no action needed
	DecisionReject  Decision = "reject"  // Case dismissed as false positive
	DecisionEnforce Decision = "enforce" // Actions applied (replaces old "remove" decision)
)

// ErrAlreadyReviewed is returned when attempting to transition a non-pending case.
type ErrAlreadyReviewed struct {
	CaseID  uuid.UUID
	Status  GovernanceCaseStatus
}

func (e *ErrAlreadyReviewed) Error() string {
	return fmt.Sprintf("governance case already reviewed: case_id=%s, status=%s", e.CaseID, e.Status)
}

// ErrInvalidTransition is returned when attempting an invalid status transition.
type ErrInvalidTransition struct {
	CaseID        uuid.UUID
	CurrentStatus GovernanceCaseStatus
	TargetStatus  GovernanceCaseStatus
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("invalid governance status transition: case_id=%s, %s -> %s",
		e.CaseID, e.CurrentStatus, e.TargetStatus)
}

// ErrEnforceRequiresNote is returned when an enforce decision is submitted without a non-empty note.
// Enforce actions are governance-sensitive (they trigger downstream removal/suspension) and require
// a human-readable audit reason so the decision is traceable and contestable via appeal.
type ErrEnforceRequiresNote struct{}

func (e *ErrEnforceRequiresNote) Error() string {
	return "enforce decision requires a non-empty note for audit trail"
}

// NewGovernanceCase creates a new governance case in pending status.
//
// Business rules:
// - Initial status is always "pending"
// - Created directly from user reports
func NewGovernanceCase(
	resourceType ResourceType,
	resourceID uuid.UUID,
	reportedBy uuid.UUID,
	reason string,
) *GovernanceCase {
	now := time.Now()
	return &GovernanceCase{
		ID:           uuid.New(),
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Status:       GovernanceCaseStatusPending,
		ReportedBy:   reportedBy,
		Reason:       reason,
		CreatedAt:    now,
	}
}

// ShouldEmitEnforcementEvents returns true if this case should emit enforcement events.
// Only "enforced" status triggers downstream enforcement actions.
func (c *GovernanceCase) ShouldEmitEnforcementEvents() bool {
	return c.Status == GovernanceCaseStatusEnforced
}

// Approve transitions the case to approved status.
//
// Business rules:
// - Only pending cases can be approved
// - Requires admin ID and optional note
// - No enforcement actions will be taken
func (c *GovernanceCase) Approve(adminID uuid.UUID, note *string) error {
	return c.transitionTo(adminID, note, GovernanceCaseStatusApproved)
}

// Reject transitions the case to rejected status.
//
// Business rules:
// - Only pending cases can be rejected
// - Requires admin ID (note recommended for audit trail)
// - Report was a false positive, no enforcement needed
func (c *GovernanceCase) Reject(adminID uuid.UUID, note *string) error {
	return c.transitionTo(adminID, note, GovernanceCaseStatusRejected)
}

// Enforce transitions the case to enforced status.
//
// CRITICAL: This triggers downstream enforcement via event emission.
//
// Business rules:
// - Only pending cases can be enforced
// - Requires admin ID
// - note MUST be non-nil and non-empty (no whitespace-only): governance-sensitive actions
//   require a human-readable audit reason for traceability and contestability via appeal.
//   Returns ErrEnforceRequiresNote if the note is absent or blank.
// - Downstream domains will react to governance events
func (c *GovernanceCase) Enforce(adminID uuid.UUID, note *string) error {
	if note == nil || strings.TrimSpace(*note) == "" {
		return &ErrEnforceRequiresNote{}
	}
	return c.transitionTo(adminID, note, GovernanceCaseStatusEnforced)
}

// transitionTo is the internal state transition method.
//
// ENFORCES:
// - Only pending -> {approved, rejected, enforced} transitions allowed
// - Terminal states cannot change
// - Reviewed cases cannot be re-reviewed
func (c *GovernanceCase) transitionTo(adminID uuid.UUID, note *string, targetStatus GovernanceCaseStatus) error {
	// Guard: Already reviewed?
	if c.Status != GovernanceCaseStatusPending {
		return &ErrAlreadyReviewed{
			CaseID: c.ID,
			Status: c.Status,
		}
	}

	// Validate transition
	if !CanTransition(c.Status, targetStatus) {
		return &ErrInvalidTransition{
			CaseID:        c.ID,
			CurrentStatus: c.Status,
			TargetStatus:  targetStatus,
		}
	}

	// Apply transition
	now := time.Now()
	c.Status = targetStatus
	c.ReviewedBy = &adminID
	c.DecisionNote = note
	c.ReviewedAt = &now

	return nil
}

// CanTransition returns true if the transition is valid.
func CanTransition(from, to GovernanceCaseStatus) bool {
	// Can only transition from pending to terminal states
	if from != GovernanceCaseStatusPending {
		return false
	}

	// Valid transitions
	switch to {
	case GovernanceCaseStatusApproved, GovernanceCaseStatusRejected, GovernanceCaseStatusEnforced:
		return true
	default:
		return false
	}
}

// IsPending returns true if the case is awaiting review.
func (c *GovernanceCase) IsPending() bool {
	return c.Status == GovernanceCaseStatusPending
}

// IsReviewed returns true if the case has been reviewed.
func (c *GovernanceCase) IsReviewed() bool {
	return c.Status != GovernanceCaseStatusPending
}

// IsTerminal returns true if the case is in a terminal state.
func (c *GovernanceCase) IsTerminal() bool {
	return c.Status == GovernanceCaseStatusApproved ||
		c.Status == GovernanceCaseStatusRejected ||
		c.Status == GovernanceCaseStatusEnforced
}

// CanReview returns true if the case can be reviewed.
func (c *GovernanceCase) CanReview() bool {
	return c.Status == GovernanceCaseStatusPending
}

// HasEnforcementActions returns true if the case resulted in enforcement actions.
func (c *GovernanceCase) HasEnforcementActions() bool {
	return c.Status == GovernanceCaseStatusEnforced
}

