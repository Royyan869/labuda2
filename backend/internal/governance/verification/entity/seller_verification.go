package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Status represents the canonical seller verification lifecycle state.
// Eight states; approved is NOT terminal — trust may be downgraded per
// the Revocable Trust Model.
type Status string

const (
	StatusNotSubmitted       Status = "not_submitted"
	StatusPendingReview      Status = "pending_review"
	StatusNeedsResubmission  Status = "needs_resubmission"
	StatusApproved           Status = "approved"
	StatusRejected           Status = "rejected"
	StatusSuspended          Status = "suspended"
	StatusRevoked            Status = "revoked"
	StatusUnderInvestigation Status = "under_investigation"
)

var transitionAllowed = map[Status][]Status{
	StatusNotSubmitted:       {StatusPendingReview},
	StatusPendingReview:      {StatusApproved, StatusRejected, StatusNeedsResubmission},
	StatusNeedsResubmission:  {StatusPendingReview},
	StatusRejected:           {StatusPendingReview},
	StatusApproved:           {StatusSuspended, StatusRevoked, StatusUnderInvestigation},
	StatusSuspended:          {StatusApproved},
	StatusUnderInvestigation: {StatusApproved, StatusSuspended, StatusRevoked, StatusNeedsResubmission},
	StatusRevoked:            {},
}

func CanTransition(from, to Status) bool {
	allowed, exists := transitionAllowed[from]
	if !exists {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// SellerVerification represents a seller's verification lifecycle row.
// Single record per seller enforced by UNIQUE(seller_id) at the DB level.
type SellerVerification struct {
	ID          uuid.UUID
	SellerID    uuid.UUID
	Status      Status
	SubmittedAt *time.Time
	ReviewedAt  *time.Time
	ReviewedBy  *uuid.UUID
	Reason      *string
	// ReviewedBankAccountIDs holds the IDs of bank accounts visible to the admin
	// at the time of the most recent approval. Withdrawal is only permitted to
	// accounts in this set (GUARD 5 in WithdrawService).
	// Persisted in seller_verifications.reviewed_bank_account_ids (migration 000208).
	ReviewedBankAccountIDs []uuid.UUID
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type InvalidTransitionError struct {
	CurrentStatus Status
	TargetStatus  Status
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("verification: invalid status transition: %s -> %s", e.CurrentStatus, e.TargetStatus)
}

type WrongStatusError struct {
	CurrentStatus  Status
	RequiredStatus Status
	Operation      string
}

func (e *WrongStatusError) Error() string {
	return fmt.Sprintf("verification: cannot %s from status %s (requires %s)", e.Operation, e.CurrentStatus, e.RequiredStatus)
}

type MissingReasonError struct {
	Operation string
}

func (e *MissingReasonError) Error() string {
	return fmt.Sprintf("verification: %s requires a reason", e.Operation)
}

func NewSellerVerification(sellerID uuid.UUID) *SellerVerification {
	now := time.Now()
	return &SellerVerification{
		ID:                     uuid.New(),
		SellerID:               sellerID,
		Status:                 StatusNotSubmitted,
		ReviewedBankAccountIDs: []uuid.UUID{},
		CreatedAt:              now,
		UpdatedAt:              now,
	}
}

func (v *SellerVerification) Submit() error {
	if !CanTransition(v.Status, StatusPendingReview) {
		return &InvalidTransitionError{CurrentStatus: v.Status, TargetStatus: StatusPendingReview}
	}
	now := time.Now()
	v.Status = StatusPendingReview
	v.SubmittedAt = &now
	v.UpdatedAt = now
	v.ReviewedAt = nil
	v.ReviewedBy = nil
	v.Reason = nil
	return nil
}

func (v *SellerVerification) Approve(adminID uuid.UUID) error {
	if !CanTransition(v.Status, StatusApproved) {
		return &WrongStatusError{CurrentStatus: v.Status, RequiredStatus: StatusPendingReview, Operation: "approve"}
	}
	now := time.Now()
	v.Status = StatusApproved
	v.ReviewedAt = &now
	v.ReviewedBy = &adminID
	v.Reason = nil
	v.UpdatedAt = now
	return nil
}

func (v *SellerVerification) Reject(adminID uuid.UUID, reason string) error {
	if reason == "" {
		return &MissingReasonError{Operation: "reject"}
	}
	if !CanTransition(v.Status, StatusRejected) {
		return &WrongStatusError{CurrentStatus: v.Status, RequiredStatus: StatusPendingReview, Operation: "reject"}
	}
	now := time.Now()
	v.Status = StatusRejected
	v.ReviewedAt = &now
	v.ReviewedBy = &adminID
	v.Reason = &reason
	v.UpdatedAt = now
	return nil
}

func (v *SellerVerification) RequestResubmission(adminID uuid.UUID, reason string) error {
	if reason == "" {
		return &MissingReasonError{Operation: "needs_resubmission"}
	}
	if !CanTransition(v.Status, StatusNeedsResubmission) {
		return &WrongStatusError{CurrentStatus: v.Status, RequiredStatus: StatusPendingReview, Operation: "needs_resubmission"}
	}
	now := time.Now()
	v.Status = StatusNeedsResubmission
	v.ReviewedAt = &now
	v.ReviewedBy = &adminID
	v.Reason = &reason
	v.UpdatedAt = now
	return nil
}

func (v *SellerVerification) Suspend(adminID uuid.UUID, reason string) error {
	if reason == "" {
		return &MissingReasonError{Operation: "suspend"}
	}
	if !CanTransition(v.Status, StatusSuspended) {
		return &InvalidTransitionError{CurrentStatus: v.Status, TargetStatus: StatusSuspended}
	}
	now := time.Now()
	v.Status = StatusSuspended
	v.ReviewedAt = &now
	v.ReviewedBy = &adminID
	v.Reason = &reason
	v.UpdatedAt = now
	return nil
}

func (v *SellerVerification) Revoke(adminID uuid.UUID, reason string) error {
	if reason == "" {
		return &MissingReasonError{Operation: "revoke"}
	}
	if !CanTransition(v.Status, StatusRevoked) {
		return &InvalidTransitionError{CurrentStatus: v.Status, TargetStatus: StatusRevoked}
	}
	now := time.Now()
	v.Status = StatusRevoked
	v.ReviewedAt = &now
	v.ReviewedBy = &adminID
	v.Reason = &reason
	v.UpdatedAt = now
	return nil
}

func (v *SellerVerification) Investigate(adminID uuid.UUID, reason string) error {
	if reason == "" {
		return &MissingReasonError{Operation: "investigate"}
	}
	if !CanTransition(v.Status, StatusUnderInvestigation) {
		return &InvalidTransitionError{CurrentStatus: v.Status, TargetStatus: StatusUnderInvestigation}
	}
	now := time.Now()
	v.Status = StatusUnderInvestigation
	v.ReviewedAt = &now
	v.ReviewedBy = &adminID
	v.Reason = &reason
	v.UpdatedAt = now
	return nil
}

func (v *SellerVerification) Restore(adminID uuid.UUID) error {
	if !CanTransition(v.Status, StatusApproved) {
		return &InvalidTransitionError{CurrentStatus: v.Status, TargetStatus: StatusApproved}
	}
	now := time.Now()
	v.Status = StatusApproved
	v.ReviewedAt = &now
	v.ReviewedBy = &adminID
	v.Reason = nil
	v.UpdatedAt = now
	return nil
}

// HasPayoutAuthority returns true only when the seller is in the approved state.
// Any downgrade (suspended / under_investigation / revoked) closes the payout gate.
func (v *SellerVerification) HasPayoutAuthority() bool {
	return v.Status == StatusApproved
}

// IsApproved is a synonym for HasPayoutAuthority.
func (v *SellerVerification) IsApproved() bool {
	return v.Status == StatusApproved
}

func (v *SellerVerification) IsPendingReview() bool {
	return v.Status == StatusPendingReview
}

func (v *SellerVerification) IsRejected() bool {
	return v.Status == StatusRejected
}

// HasReviewedBankAccount returns true when bankAccountID is in ReviewedBankAccountIDs.
// Used by WithdrawService GUARD 5 to enforce that withdrawal only goes to bank accounts
// reviewed during KYC approval.
func (v *SellerVerification) HasReviewedBankAccount(bankAccountID uuid.UUID) bool {
	for _, id := range v.ReviewedBankAccountIDs {
		if id == bankAccountID {
			return true
		}
	}
	return false
}

// AppendReviewedBankAccount idempotently appends bankAccountID to ReviewedBankAccountIDs.
// No-op when the ID is already present. The caller is responsible for verifying that
// the seller is in approved status before calling this method.
func (v *SellerVerification) AppendReviewedBankAccount(bankAccountID uuid.UUID) {
	if v.HasReviewedBankAccount(bankAccountID) {
		return
	}
	v.ReviewedBankAccountIDs = append(v.ReviewedBankAccountIDs, bankAccountID)
	v.UpdatedAt = time.Now()
}


