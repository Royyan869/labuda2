// DOMAIN: Moderation Domain (governance/moderation/)
// RESPONSIBILITY: Appeal entity — user challenge of a governance Decision
//
// SLICE A: Canonical alignment — Appeal → Decision (NOT Case, NOT Report).
// Appeal targets a Decision via decision_id FK.
// Appeal review produces Decision #2 (deferred to Slice B).
//
// Canonical reference: LABUDA — CANONICAL MODERATION BUSINESS TRUTH v1 §24-25
// Canonical reference: LABUDA — CANONICAL MODERATION DESIGN v1 §23-25

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Appeal represents an appeal request against a governance Decision.
//
// Canonical: Appeal → Decision (BT §24, Design §4.6).
// Users can appeal Decisions that produced consequences against them.
// Appeals are reviewed by admins who produce Decision #2 (deferred to Slice B).
type Appeal struct {
	ID            uuid.UUID
	DecisionID    uuid.UUID   // FK → decisions.id (canonical: Appeal → Decision)
	AppealedBy    uuid.UUID   // User who created the appeal (affected party)
	Status        AppealStatus
	Message       string      // User's explanation for the appeal
	AdminResponse *string     // Admin's response to the appeal
	ReviewedBy    *uuid.UUID  // Admin who reviewed the appeal
	CreatedAt     time.Time
	ReviewedAt    *time.Time
}

// AppealStatus represents the state of an appeal request.
// Note: The canonical state vocabulary is not explicitly locked in Business Truth.
// These values match the existing DB constraint (migration 000055):
//   CHECK (status IN ('pending', 'approved', 'rejected'))
type AppealStatus string

const (
	// AppealStatusPending means the appeal is awaiting admin review.
	AppealStatusPending AppealStatus = "pending"

	// AppealStatusApproved means the appeal was approved and the governance
	// Decision was overturned. Decision #2 will be created (Slice B).
	AppealStatusApproved AppealStatus = "approved"

	// AppealStatusRejected means the appeal was rejected and the original
	// governance Decision stands.
	AppealStatusRejected AppealStatus = "rejected"
)

// IsPending returns true if the appeal is awaiting review.
func (s AppealStatus) IsPending() bool {
	return s == AppealStatusPending
}

// IsReviewed returns true if the appeal has been reviewed.
func (s AppealStatus) IsReviewed() bool {
	return s != AppealStatusPending
}

// ============================================================================
// DOMAIN ERRORS
// ============================================================================

// ErrAppealNotFound is returned when an appeal cannot be found.
type ErrAppealNotFound struct {
	AppealID uuid.UUID
}

func (e *ErrAppealNotFound) Error() string {
	return fmt.Sprintf("appeal not found: %s", e.AppealID)
}

// ErrAppealAlreadyReviewed is returned when attempting to review an already reviewed appeal.
type ErrAppealAlreadyReviewed struct {
	AppealID uuid.UUID
	Status   AppealStatus
}

func (e *ErrAppealAlreadyReviewed) Error() string {
	return fmt.Sprintf("appeal already reviewed: appeal_id=%s, status=%s", e.AppealID, e.Status)
}

// ErrInvalidAppealTransition is returned when attempting an invalid status transition.
type ErrInvalidAppealTransition struct {
	AppealID      uuid.UUID
	CurrentStatus AppealStatus
	TargetStatus  AppealStatus
}

func (e *ErrInvalidAppealTransition) Error() string {
	return fmt.Sprintf("invalid appeal transition: appeal_id=%s, %s -> %s",
		e.AppealID, e.CurrentStatus, e.TargetStatus)
}

// ErrDecisionNotFound is returned when attempting to appeal a non-existent Decision.
type ErrDecisionNotFound struct {
	DecisionID uuid.UUID
}

func (e *ErrDecisionNotFound) Error() string {
	return fmt.Sprintf("decision not found: %s", e.DecisionID)
}

// ErrDecisionNotAppealable is returned when attempting to appeal a Decision
// that does not produce consequences (e.g. no_violation).
// Design §23: "Tidak ada appeal terhadap pure rejection/no-action."
type ErrDecisionNotAppealable struct {
	DecisionID uuid.UUID
	Outcome    DecisionOutcome
}

func (e *ErrDecisionNotAppealable) Error() string {
	return fmt.Sprintf("decision not appealable: decision_id=%s, outcome=%s", e.DecisionID, e.Outcome)
}

// ErrNotResourceOwner is returned when attempting to appeal a resource you don't own.
type ErrNotResourceOwner struct {
	DecisionID   uuid.UUID
	ResourceID   uuid.UUID
	UserID       uuid.UUID
	ResourceType string
}

func (e *ErrNotResourceOwner) Error() string {
	return fmt.Sprintf("not resource owner: decision_id=%s, resource_type=%s, resource_id=%s, user_id=%s",
		e.DecisionID, e.ResourceType, e.ResourceID, e.UserID)
}

// ErrDuplicatePendingAppeal is returned when attempting to create a second
// pending appeal for the same Decision.
type ErrDuplicatePendingAppeal struct {
	DecisionID uuid.UUID
}

func (e *ErrDuplicatePendingAppeal) Error() string {
	return fmt.Sprintf("duplicate pending appeal: decision_id=%s", e.DecisionID)
}

// ErrUnsupportedResourceType is returned when attempting to appeal a Decision
// for a resource type that does not support appeals.
// Supported types: content, comment, for_sale, auction, user.
type ErrUnsupportedResourceType struct {
	ResourceType string
}

func (e *ErrUnsupportedResourceType) Error() string {
	return fmt.Sprintf("appeals not supported for resource type: %s (supported: content, comment, for_sale, auction, user)", e.ResourceType)
}

// ============================================================================
// CONSTRUCTORS AND STATE MACHINE
// ============================================================================

// NewAppeal creates a new appeal in pending status.
//
// Business rules:
//   - Initial status is always "pending"
//   - Appeal targets a specific Decision (canonical: Appeal → Decision)
func NewAppeal(
	decisionID uuid.UUID,
	appealedBy uuid.UUID,
	message string,
) *Appeal {
	now := time.Now()
	return &Appeal{
		ID:         uuid.New(),
		DecisionID: decisionID,
		AppealedBy: appealedBy,
		Status:     AppealStatusPending,
		Message:    message,
		CreatedAt:  now,
	}
}

// CanAppealTransition checks if a status transition is allowed for appeals.
func CanAppealTransition(from, to AppealStatus) bool {
	// Only pending -> {approved, rejected} transitions allowed
	if from != AppealStatusPending {
		return false
	}
	return to == AppealStatusApproved || to == AppealStatusRejected
}

// Approve transitions the appeal to approved status.
// This means the governance Decision will be overturned (Decision #2 in Slice B).
func (a *Appeal) Approve(adminID uuid.UUID, adminResponse *string) error {
	return a.transitionTo(adminID, adminResponse, AppealStatusApproved)
}

// Reject transitions the appeal to rejected status.
// This means the original governance Decision stands.
func (a *Appeal) Reject(adminID uuid.UUID, adminResponse *string) error {
	return a.transitionTo(adminID, adminResponse, AppealStatusRejected)
}

// transitionTo is the internal state transition method.
func (a *Appeal) transitionTo(adminID uuid.UUID, adminResponse *string, targetStatus AppealStatus) error {
	// Guard: Already reviewed?
	if a.Status != AppealStatusPending {
		return &ErrAppealAlreadyReviewed{
			AppealID: a.ID,
			Status:   a.Status,
		}
	}

	// Validate transition
	if !CanAppealTransition(a.Status, targetStatus) {
		return &ErrInvalidAppealTransition{
			AppealID:      a.ID,
			CurrentStatus: a.Status,
			TargetStatus:  targetStatus,
		}
	}

	// Apply transition
	now := time.Now()
	a.Status = targetStatus
	a.ReviewedBy = &adminID
	a.AdminResponse = adminResponse
	a.ReviewedAt = &now

	return nil
}
