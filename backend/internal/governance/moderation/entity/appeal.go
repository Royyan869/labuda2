package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Appeal represents an appeal request for a moderation decision.
//
// Users can appeal moderation decisions that resulted in content removal.
// Appeals are reviewed by admins who can uphold or overturn the decision.
type Appeal struct {
	ID            uuid.UUID
	CaseID       uuid.UUID   // Reference to the original moderation case
	AppealedBy    uuid.UUID   // User who created the appeal
	Status        AppealStatus
	Message       string      // User's explanation for the appeal
	AdminResponse *string     // Admin's response to the appeal
	ReviewedBy    *uuid.UUID  // Admin who reviewed the appeal
	CreatedAt     time.Time
	ReviewedAt    *time.Time
}

// AppealStatus represents the state of an appeal.
type AppealStatus string

const (
	// AppealStatusPending means the appeal is awaiting admin review.
	AppealStatusPending AppealStatus = "pending"

	// AppealStatusApproved means the appeal was approved and the moderation decision was overturned.
	AppealStatusApproved AppealStatus = "approved"

	// AppealStatusRejected means the appeal was rejected and the original moderation decision stands.
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
	AppealID       uuid.UUID
	CurrentStatus  AppealStatus
	TargetStatus   AppealStatus
}

func (e *ErrInvalidAppealTransition) Error() string {
	return fmt.Sprintf("invalid appeal transition: appeal_id=%s, %s -> %s",
		e.AppealID, e.CurrentStatus, e.TargetStatus)
}

// ErrCaseNotFound is returned when attempting to appeal a non-existent moderation case.
type ErrCaseNotFound struct {
	CaseID uuid.UUID
}

func (e *ErrCaseNotFound) Error() string {
	return fmt.Sprintf("moderation case not found: case_id=%s", e.CaseID)
}

// ErrNotResourceOwner is returned when attempting to appeal a resource you don't own.
type ErrNotResourceOwner struct {
	CaseID      uuid.UUID
	ResourceID  uuid.UUID
	UserID      uuid.UUID
	ResourceType string
}

func (e *ErrNotResourceOwner) Error() string {
	return fmt.Sprintf("not resource owner: case_id=%s, resource_type=%s, resource_id=%s, user_id=%s",
		e.CaseID, e.ResourceType, e.ResourceID, e.UserID)
}

// ErrDuplicatePendingAppeal is returned when attempting to create a second pending appeal for the same case.
type ErrDuplicatePendingAppeal struct {
	CaseID uuid.UUID
}

func (e *ErrDuplicatePendingAppeal) Error() string {
	return fmt.Sprintf("duplicate pending appeal: case_id=%s", e.CaseID)
}

// ErrCaseNotAppealable is returned when attempting to appeal a case that is not in a terminal state.
type ErrCaseNotAppealable struct {
	CaseID uuid.UUID
	Status GovernanceCaseStatus
}

func (e *ErrCaseNotAppealable) Error() string {
	return fmt.Sprintf("case not appealable: case_id=%s, status=%s", e.CaseID, e.Status)
}

// ErrUnsupportedResourceType is returned when attempting to appeal a case
// for a resource type that does not support appeals.
// Supported types: content, comment.
type ErrUnsupportedResourceType struct {
	ResourceType string
}

func (e *ErrUnsupportedResourceType) Error() string {
	return fmt.Sprintf("appeals not supported for resource type: %s (supported: content, comment)", e.ResourceType)
}

// ErrRestorationEventFailed is returned when restoration event emission fails.
type ErrRestorationEventFailed struct {
	AppealID uuid.UUID
	Err      error
}

func (e *ErrRestorationEventFailed) Error() string {
	return fmt.Sprintf("restoration event failed: appeal_id=%s, error=%v", e.AppealID, e.Err)
}

// NewAppeal creates a new appeal in pending status.
func NewAppeal(
	caseID uuid.UUID,
	appealedBy uuid.UUID,
	message string,
) *Appeal {
	now := time.Now()
	return &Appeal{
		ID:         uuid.New(),
		CaseID:     caseID,
		AppealedBy: appealedBy,
		Status:     AppealStatusPending,
		Message:    message,
		CreatedAt:  now,
	}
}

// CanTransition checks if a status transition is allowed for appeals.
func CanAppealTransition(from, to AppealStatus) bool {
	// Only pending -> {approved, rejected} transitions allowed
	if from != AppealStatusPending {
		return false
	}
	return to == AppealStatusApproved || to == AppealStatusRejected
}

// Approve transitions the appeal to approved status.
// This means the moderation decision was overturned.
func (a *Appeal) Approve(adminID uuid.UUID, adminResponse *string) error {
	return a.transitionTo(adminID, adminResponse, AppealStatusApproved)
}

// Reject transitions the appeal to rejected status.
// This means the original moderation decision stands.
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
			AppealID:       a.ID,
			CurrentStatus:  a.Status,
			TargetStatus:   targetStatus,
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


