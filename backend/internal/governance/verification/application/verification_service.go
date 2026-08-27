package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	bankaccountEntity "github.com/labuda/backend/internal/finance/bankaccount/entity"
	"github.com/labuda/backend/internal/platform/capability"
	"github.com/labuda/backend/internal/governance/verification/entity"
	"github.com/labuda/backend/internal/governance/verification/infrastructure/repository"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// VerificationService handles seller verification lifecycle operations.
//
// Canonical statuses (8) live in entity.Status and follow the doctrine state
// machine in docs/flows/doctrine/verification-review-governance.md. `approved`
// opens payout authority; suspended / under_investigation / revoked are
// admin-initiated downgrades that close the gate without mutating balance,
// history, or active obligations (see Revocable Trust Model).
//
// Invariants:
//   - UNIQUE(seller_id) enforced at DB level
//   - `approved` is NOT terminal; downgrade transitions are part of the machine
//   - Payout authority predicate (HasPayoutAuthority / IsSellerApproved) is the
//     single source of truth consulted by WithdrawService
//   - Every admin transition (approve / reject / request_resubmission) writes
//     an admin_audit_logs row and emits a seller.verification.* outbox event
//     atomically inside the same transaction.
//   - ApproveVerification snapshots the active bank account IDs at approval time
//     into reviewed_bank_account_ids (migration 000208). WithdrawService GUARD 5
//     enforces that only reviewed accounts can receive payouts.
type VerificationService struct {
	db          Transactor
	repo        *repository.SellerVerificationRepository
	auditLogger audit.AdminAuditLogger
	outboxRepo  *outboxrepo.OutboxRepository
	authorizer  CapabilityAuthorizer

	// Optional — wired via SetBankAccountReader after construction.
	// When nil, ApproveVerification stores an empty reviewed_bank_account_ids.
	bankAcctDB     Transactor
	bankAcctReader BankAccountsReader
}

// BankAccountsReader is the minimal interface needed to fetch payout destinations
// for the reviewed-at-approval snapshot. The concrete implementation is
// *bankaccountrepo.BankAccountRepository (the same type used by the admin
// verification handler's BankAccountsReader interface).
type BankAccountsReader interface {
	ListActiveAccountsBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID) ([]*bankaccountEntity.BankAccount, error)
}

// SetBankAccountReader wires the optional bank account reader dependency.
// Must be called in serverboot after NewVerificationService so that
// ApproveVerification can snapshot reviewed bank account IDs at approval time.
// When not wired, approval stores an empty set (backwards-compatible but means
// GUARD 5 blocks all withdrawals for that seller until re-approved).
func (s *VerificationService) SetBankAccountReader(transactor Transactor, reader BankAccountsReader) {
	s.bankAcctDB = transactor
	s.bankAcctReader = reader
}

// CapabilityAuthorizer is the minimal capability lookup surface required by
// admin verification transitions.
type CapabilityAuthorizer interface {
	HasCapability(ctx context.Context, userID uuid.UUID, capability capability.Capability) (bool, error)
}

// ErrVerificationAuthorityRequired is returned when the caller lacks the
// required seller verification review capability.
type ErrVerificationAuthorityRequired struct {
	ActorID    uuid.UUID
	Capability string
	Operation  string
}

func (e *ErrVerificationAuthorityRequired) Error() string {
	return fmt.Sprintf("actor %s lacks capability %s for %s", e.ActorID, e.Capability, e.Operation)
}

// Transactor represents the ability to execute functions within transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// ErrMarkReviewedNotApproved is returned by MarkBankAccountReviewed when the
// seller's verification is not in approved status.
type ErrMarkReviewedNotApproved struct {
	SellerID      uuid.UUID
	CurrentStatus entity.Status
}

func (e *ErrMarkReviewedNotApproved) Error() string {
	return fmt.Sprintf(
		"verification: cannot mark bank account reviewed; seller %s is not approved (status: %s)",
		e.SellerID, e.CurrentStatus,
	)
}

// ErrBankAccountNotFoundForSeller is returned by MarkBankAccountReviewed when
// the requested bank account does not exist, is deleted, or belongs to a
// different seller.
type ErrBankAccountNotFoundForSeller struct {
	SellerID      uuid.UUID
	BankAccountID uuid.UUID
}

func (e *ErrBankAccountNotFoundForSeller) Error() string {
	return fmt.Sprintf(
		"bank account %s not found or not active for seller %s",
		e.BankAccountID, e.SellerID,
	)
}

// Audit / outbox event type constants. Kept here so call sites in tests and
// handlers stay aligned with the worker dispatcher subscriptions.
const (
	// Admin audit action types — written to admin_audit_logs.action_type.
	ActionVerificationApproved             = "seller_verification_approved"
	ActionVerificationRejected             = "seller_verification_rejected"
	ActionVerificationResubmissionRequired = "seller_verification_resubmission_required"
	ActionVerificationSuspended            = "seller_verification_suspended"
	ActionVerificationRevoked              = "seller_verification_revoked"
	ActionVerificationUnderInvestigation   = "seller_verification_under_investigation"
	ActionVerificationRestored             = "seller_verification_restored"
	// ActionBankAccountMarkedReviewed is the audit action for manual post-approval
	// bank account review via POST .../bank-accounts/:id/mark-reviewed.
	ActionBankAccountMarkedReviewed = "seller_bank_account_marked_reviewed"

	// Outbox event types — written to outbox.event_type.
	EventVerificationSubmitted          = "seller.verification.submitted"
	EventVerificationApproved           = "seller.verification.approved"
	EventVerificationRejected           = "seller.verification.rejected"
	EventVerificationNeedsResubmission  = "seller.verification.needs_resubmission"
	EventVerificationSuspended          = "seller.verification.suspended"
	EventVerificationRevoked            = "seller.verification.revoked"
	EventVerificationUnderInvestigation = "seller.verification.under_investigation"
	EventVerificationRestored           = "seller.verification.restored"

	// Audit / outbox target type for seller_verifications.
	TargetTypeSellerVerification = "seller_verification"
)

// NewVerificationService creates a new VerificationService. auditLogger and
// outboxRepo may be nil during local tests; in production they are required
// for doctrine compliance (trust escalation attribution + notification
// continuity).
func NewVerificationService(
	db Transactor,
	repo *repository.SellerVerificationRepository,
	auditLogger audit.AdminAuditLogger,
	outboxRepo *outboxrepo.OutboxRepository,
	authorizer CapabilityAuthorizer,
) *VerificationService {
	return &VerificationService{
		db:          db,
		repo:        repo,
		auditLogger: auditLogger,
		outboxRepo:  outboxRepo,
		authorizer:  authorizer,
	}
}

func (s *VerificationService) requireReviewAuthority(ctx context.Context, adminID uuid.UUID, operation string) error {
	if adminID == uuid.Nil {
		return &ErrVerificationAuthorityRequired{
			ActorID:    adminID,
			Capability: capability.CapSellerVerificationReview.String(),
			Operation:  operation,
		}
	}
	if s.authorizer == nil {
		return &ErrVerificationAuthorityRequired{
			ActorID:    adminID,
			Capability: capability.CapSellerVerificationReview.String(),
			Operation:  operation,
		}
	}
	hasAuthority, err := s.authorizer.HasCapability(ctx, adminID, capability.CapSellerVerificationReview)
	if err != nil {
		return fmt.Errorf("verification: capability check failed: %w", err)
	}
	if !hasAuthority {
		return &ErrVerificationAuthorityRequired{
			ActorID:    adminID,
			Capability: capability.CapSellerVerificationReview.String(),
			Operation:  operation,
		}
	}
	return nil
}

// SubmitVerification submits or resubmits a verification for admin review.
func (s *VerificationService) SubmitVerification(
	ctx context.Context,
	sellerID uuid.UUID,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		v, err := s.repo.GetForUpdate(ctx, tx, sellerID)
		if err != nil {
			return fmt.Errorf("verification: get for update failed: %w", err)
		}

		if v == nil {
			v = entity.NewSellerVerification(sellerID)
			if err := v.Submit(); err != nil {
				return err
			}
			if err := s.repo.Create(ctx, tx, v); err != nil {
				return err
			}
		} else {
			if err := v.Submit(); err != nil {
				return err
			}
			if err := s.repo.Update(ctx, tx, v); err != nil {
				return err
			}
		}

		return s.emitEventTx(ctx, tx, EventVerificationSubmitted, sellerID, map[string]interface{}{
			"seller_id": sellerID.String(),
			"status":    string(v.Status),
		})
	})
}

// ApproveVerification approves a pending_review submission.
//
// In addition to the canonical status flip + audit log + outbox event, this
// method fetches the seller's currently active bank accounts (via the optional
// BankAccountsReader) and stores their IDs as reviewed_bank_account_ids on the
// verification row. WithdrawService GUARD 5 enforces that payout is only allowed
// to accounts in this set.
//
// When BankAccountsReader is not wired (bankAcctReader == nil), the reviewed
// set is left empty — this intentionally blocks all withdrawal until the reader
// is wired and a re-approval is performed.
//
// All mutations — status flip, reviewed IDs, audit log, outbox — are atomic
// in a single transaction (STRICT_EVENT_ATOMIC).
func (s *VerificationService) ApproveVerification(
	ctx context.Context,
	sellerID uuid.UUID,
	adminID uuid.UUID,
) error {
	if err := s.requireReviewAuthority(ctx, adminID, "approve"); err != nil {
		return err
	}

	// Fetch active bank accounts outside the verification transaction so we
	// do not nest transactions. If the reader is unavailable we proceed with
	// an empty set (fail-open on approval, fail-closed on withdrawal).
	var reviewedIDs []uuid.UUID
	if s.bankAcctDB != nil && s.bankAcctReader != nil {
		_ = s.bankAcctDB.WithTx(ctx, func(tx db.Tx) error {
			accounts, err := s.bankAcctReader.ListActiveAccountsBySeller(ctx, tx, sellerID)
			if err != nil {
				// Non-fatal: log via structured log at caller; proceed with empty set.
				return nil
			}
			for _, ba := range accounts {
				reviewedIDs = append(reviewedIDs, ba.ID)
			}
			return nil
		})
	}
	if reviewedIDs == nil {
		reviewedIDs = []uuid.UUID{}
	}

	// Build string slice for audit metadata.
	reviewedIDStrs := make([]string, len(reviewedIDs))
	for i, id := range reviewedIDs {
		reviewedIDStrs[i] = id.String()
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		v, err := s.repo.GetForUpdate(ctx, tx, sellerID)
		if err != nil {
			return fmt.Errorf("verification: get for update failed: %w", err)
		}
		if v == nil {
			return fmt.Errorf("verification: no record found for seller %s", sellerID)
		}

		if err := v.Approve(adminID); err != nil {
			return err
		}
		// Store the reviewed bank account IDs atomically with the approval.
		v.ReviewedBankAccountIDs = reviewedIDs

		if err := s.repo.Update(ctx, tx, v); err != nil {
			return err
		}

		if err := s.writeAuditTx(ctx, tx, adminID, ActionVerificationApproved, sellerID, map[string]interface{}{
			"seller_id":                    sellerID.String(),
			"reviewed_bank_account_ids":    reviewedIDStrs,
			"reviewed_bank_account_count":  len(reviewedIDs),
		}); err != nil {
			return err
		}

		return s.emitEventTx(ctx, tx, EventVerificationApproved, sellerID, map[string]interface{}{
			"seller_id":                 sellerID.String(),
			"approved_by":               adminID.String(),
			"status":                    string(v.Status),
			"reviewed_bank_account_ids": reviewedIDStrs,
		})
	})
}

// RejectVerification rejects a pending_review submission. Reason is mandatory.
func (s *VerificationService) RejectVerification(
	ctx context.Context,
	sellerID uuid.UUID,
	adminID uuid.UUID,
	reason string,
) error {
	if err := s.requireReviewAuthority(ctx, adminID, "reject"); err != nil {
		return err
	}
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		v, err := s.repo.GetForUpdate(ctx, tx, sellerID)
		if err != nil {
			return fmt.Errorf("verification: get for update failed: %w", err)
		}
		if v == nil {
			return fmt.Errorf("verification: no record found for seller %s", sellerID)
		}

		if err := v.Reject(adminID, reason); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, tx, v); err != nil {
			return err
		}

		if err := s.writeAuditTx(ctx, tx, adminID, ActionVerificationRejected, sellerID, map[string]interface{}{
			"seller_id": sellerID.String(),
			"reason":    reason,
		}); err != nil {
			return err
		}

		return s.emitEventTx(ctx, tx, EventVerificationRejected, sellerID, map[string]interface{}{
			"seller_id":   sellerID.String(),
			"rejected_by": adminID.String(),
			"reason":      reason,
			"status":      string(v.Status),
		})
	})
}

// RequestResubmission moves a pending_review submission to needs_resubmission.
// Reason is mandatory.
func (s *VerificationService) RequestResubmission(
	ctx context.Context,
	sellerID uuid.UUID,
	adminID uuid.UUID,
	reason string,
) error {
	if err := s.requireReviewAuthority(ctx, adminID, "request_resubmission"); err != nil {
		return err
	}
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		v, err := s.repo.GetForUpdate(ctx, tx, sellerID)
		if err != nil {
			return fmt.Errorf("verification: get for update failed: %w", err)
		}
		if v == nil {
			return fmt.Errorf("verification: no record found for seller %s", sellerID)
		}

		if err := v.RequestResubmission(adminID, reason); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, tx, v); err != nil {
			return err
		}

		if err := s.writeAuditTx(ctx, tx, adminID, ActionVerificationResubmissionRequired, sellerID, map[string]interface{}{
			"seller_id": sellerID.String(),
			"reason":    reason,
		}); err != nil {
			return err
		}

		return s.emitEventTx(ctx, tx, EventVerificationNeedsResubmission, sellerID, map[string]interface{}{
			"seller_id":    sellerID.String(),
			"requested_by": adminID.String(),
			"reason":       reason,
			"status":       string(v.Status),
		})
	})
}

// SuspendVerification suspends a seller's verification (reversible trust pause).
// Reason is mandatory. Closes both selling and payout authority.
func (s *VerificationService) SuspendVerification(
	ctx context.Context,
	sellerID uuid.UUID,
	adminID uuid.UUID,
	reason string,
) error {
	if err := s.requireReviewAuthority(ctx, adminID, "suspend"); err != nil {
		return err
	}
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		v, err := s.repo.GetForUpdate(ctx, tx, sellerID)
		if err != nil {
			return fmt.Errorf("verification: get for update failed: %w", err)
		}
		if v == nil {
			return fmt.Errorf("verification: no record found for seller %s", sellerID)
		}

		if err := v.Suspend(adminID, reason); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, tx, v); err != nil {
			return err
		}

		if err := s.writeAuditTx(ctx, tx, adminID, ActionVerificationSuspended, sellerID, map[string]interface{}{
			"seller_id": sellerID.String(),
			"reason":    reason,
		}); err != nil {
			return err
		}

		return s.emitEventTx(ctx, tx, EventVerificationSuspended, sellerID, map[string]interface{}{
			"seller_id":    sellerID.String(),
			"suspended_by": adminID.String(),
			"reason":       reason,
			"status":       string(v.Status),
		})
	})
}

// RevokeVerification revokes a seller's verification (terminal trust withdrawal).
// Reason is mandatory. Closes both selling and payout authority permanently.
func (s *VerificationService) RevokeVerification(
	ctx context.Context,
	sellerID uuid.UUID,
	adminID uuid.UUID,
	reason string,
) error {
	if err := s.requireReviewAuthority(ctx, adminID, "revoke"); err != nil {
		return err
	}
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		v, err := s.repo.GetForUpdate(ctx, tx, sellerID)
		if err != nil {
			return fmt.Errorf("verification: get for update failed: %w", err)
		}
		if v == nil {
			return fmt.Errorf("verification: no record found for seller %s", sellerID)
		}

		if err := v.Revoke(adminID, reason); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, tx, v); err != nil {
			return err
		}

		if err := s.writeAuditTx(ctx, tx, adminID, ActionVerificationRevoked, sellerID, map[string]interface{}{
			"seller_id": sellerID.String(),
			"reason":    reason,
		}); err != nil {
			return err
		}

		return s.emitEventTx(ctx, tx, EventVerificationRevoked, sellerID, map[string]interface{}{
			"seller_id":  sellerID.String(),
			"revoked_by": adminID.String(),
			"reason":     reason,
			"status":     string(v.Status),
		})
	})
}

// InvestigateVerification moves a seller to under_investigation.
// Reason is mandatory. Payout authority closed; selling authority preserved
// per Option C doctrine (investigation != conviction).
func (s *VerificationService) InvestigateVerification(
	ctx context.Context,
	sellerID uuid.UUID,
	adminID uuid.UUID,
	reason string,
) error {
	if err := s.requireReviewAuthority(ctx, adminID, "investigate"); err != nil {
		return err
	}
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		v, err := s.repo.GetForUpdate(ctx, tx, sellerID)
		if err != nil {
			return fmt.Errorf("verification: get for update failed: %w", err)
		}
		if v == nil {
			return fmt.Errorf("verification: no record found for seller %s", sellerID)
		}

		if err := v.Investigate(adminID, reason); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, tx, v); err != nil {
			return err
		}

		if err := s.writeAuditTx(ctx, tx, adminID, ActionVerificationUnderInvestigation, sellerID, map[string]interface{}{
			"seller_id": sellerID.String(),
			"reason":    reason,
		}); err != nil {
			return err
		}

		return s.emitEventTx(ctx, tx, EventVerificationUnderInvestigation, sellerID, map[string]interface{}{
			"seller_id":       sellerID.String(),
			"investigated_by": adminID.String(),
			"reason":          reason,
			"status":          string(v.Status),
		})
	})
}

// RestoreVerification restores a seller from suspended or under_investigation
// back to approved. Reason is optional. Reopens both selling and payout authority.
func (s *VerificationService) RestoreVerification(
	ctx context.Context,
	sellerID uuid.UUID,
	adminID uuid.UUID,
) error {
	if err := s.requireReviewAuthority(ctx, adminID, "restore"); err != nil {
		return err
	}
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		v, err := s.repo.GetForUpdate(ctx, tx, sellerID)
		if err != nil {
			return fmt.Errorf("verification: get for update failed: %w", err)
		}
		if v == nil {
			return fmt.Errorf("verification: no record found for seller %s", sellerID)
		}

		if err := v.Restore(adminID); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, tx, v); err != nil {
			return err
		}

		if err := s.writeAuditTx(ctx, tx, adminID, ActionVerificationRestored, sellerID, map[string]interface{}{
			"seller_id": sellerID.String(),
		}); err != nil {
			return err
		}

		return s.emitEventTx(ctx, tx, EventVerificationRestored, sellerID, map[string]interface{}{
			"seller_id":   sellerID.String(),
			"restored_by": adminID.String(),
			"status":      string(v.Status),
		})
	})
}

// MarkBankAccountReviewed appends bankAccountID to the seller's
// reviewed_bank_account_ids without requiring a full re-KYC cycle.
//
// Use this when a seller adds a new bank account post-approval and an admin
// has manually verified that the account belongs to the same identity. This
// unblocks GUARD 5 in WithdrawService for that specific account.
//
// Validations:
//   - Seller must be in approved status (ErrMarkReviewedNotApproved).
//   - Bank account must be active and belong to this seller
//     (ErrBankAccountNotFoundForSeller).
//
// Idempotent: marking an already-reviewed account is a no-op (no audit log
// written on duplicate — the first write already evidences the decision).
//
// All mutations (reviewed IDs update + audit log) are atomic in a single tx.
func (s *VerificationService) MarkBankAccountReviewed(
	ctx context.Context,
	sellerID uuid.UUID,
	bankAccountID uuid.UUID,
	adminID uuid.UUID,
) error {
	if err := s.requireReviewAuthority(ctx, adminID, "mark_bank_account_reviewed"); err != nil {
		return err
	}
	if s.bankAcctReader == nil {
		return fmt.Errorf("verification: bank account reader not wired; cannot validate account ownership")
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		v, err := s.repo.GetForUpdate(ctx, tx, sellerID)
		if err != nil {
			return fmt.Errorf("verification: get for update failed: %w", err)
		}
		if v == nil {
			return fmt.Errorf("verification: no record found for seller %s", sellerID)
		}
		if !v.HasPayoutAuthority() {
			return &ErrMarkReviewedNotApproved{SellerID: sellerID, CurrentStatus: v.Status}
		}

		// Idempotent: already reviewed → no-op (no duplicate audit log).
		if v.HasReviewedBankAccount(bankAccountID) {
			return nil
		}

		// Validate account exists, is active, and belongs to this seller.
		accounts, err := s.bankAcctReader.ListActiveAccountsBySeller(ctx, tx, sellerID)
		if err != nil {
			return fmt.Errorf("verification: fetch bank accounts: %w", err)
		}
		var target *bankaccountEntity.BankAccount
		for _, ba := range accounts {
			if ba.ID == bankAccountID {
				target = ba
				break
			}
		}
		if target == nil {
			return &ErrBankAccountNotFoundForSeller{SellerID: sellerID, BankAccountID: bankAccountID}
		}

		v.AppendReviewedBankAccount(bankAccountID)
		if err := s.repo.Update(ctx, tx, v); err != nil {
			return err
		}

		return s.writeAuditTx(ctx, tx, adminID, ActionBankAccountMarkedReviewed, sellerID, map[string]interface{}{
			"seller_id":           sellerID.String(),
			"bank_account_id":     bankAccountID.String(),
			"bank_name":           target.BankName,
			"bank_code":           target.BankCode,
			"account_holder_name": target.AccountHolderName,
		})
	})
}

// IsReviewedBankAccountTx reports whether bankAccountID was captured in the
// seller's reviewed_bank_account_ids at the most recent KYC approval.
// Satisfies WithdrawVerificationChecker.IsReviewedBankAccountTx (finance domain).
// Returns false when no verification record exists — GUARD 1 handles that case.
func (s *VerificationService) IsReviewedBankAccountTx(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	bankAccountID uuid.UUID,
) (bool, error) {
	v, err := s.repo.GetBySellerID(ctx, tx, sellerID)
	if err != nil {
		return false, err
	}
	if v == nil {
		return false, nil
	}
	return v.HasReviewedBankAccount(bankAccountID), nil
}

// GetVerificationTx returns the full SellerVerification entity using the
// supplied tx. Returns nil when the seller has no record.
func (s *VerificationService) GetVerificationTx(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (*entity.SellerVerification, error) {
	return s.repo.GetBySellerID(ctx, tx, sellerID)
}

// IsSellerVerifiedTx checks whether a seller currently holds payout authority
// inside an existing transaction.
func (s *VerificationService) IsSellerVerifiedTx(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (bool, error) {
	v, err := s.repo.GetBySellerID(ctx, tx, sellerID)
	if err != nil {
		return false, err
	}
	if v == nil {
		return false, nil
	}
	return v.HasPayoutAuthority(), nil
}

// IsSellerVerified is the convenience (non-tx-shared) form of IsSellerVerifiedTx.
func (s *VerificationService) IsSellerVerified(
	ctx context.Context,
	sellerID uuid.UUID,
) (bool, error) {
	var verified bool
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		v, err := s.repo.GetBySellerID(ctx, tx, sellerID)
		if err != nil {
			return err
		}
		if v == nil {
			verified = false
			return nil
		}
		verified = v.HasPayoutAuthority()
		return nil
	})
	if err != nil {
		return false, err
	}
	return verified, nil
}

// GetStatus returns the current verification status for a seller.
func (s *VerificationService) GetStatus(
	ctx context.Context,
	sellerID uuid.UUID,
) (entity.Status, error) {
	var status entity.Status
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		v, err := s.repo.GetBySellerID(ctx, tx, sellerID)
		if err != nil {
			return err
		}
		if v == nil {
			status = entity.StatusNotSubmitted
			return nil
		}
		status = v.Status
		return nil
	})
	if err != nil {
		return "", err
	}
	return status, nil
}

// GetVerification retrieves the full verification record for a seller.
// Returns nil if not found.
func (s *VerificationService) GetVerification(
	ctx context.Context,
	sellerID uuid.UUID,
) (*entity.SellerVerification, error) {
	var v *entity.SellerVerification
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		v, err = s.repo.GetBySellerID(ctx, tx, sellerID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return v, nil
}

// GetVerificationWithUsername retrieves the full verification record together
// with the seller's public username and farm/store name for admin display.
func (s *VerificationService) GetVerificationWithUsername(
	ctx context.Context,
	sellerID uuid.UUID,
) (*repository.VerificationIdentityRow, error) {
	var row *repository.VerificationIdentityRow
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		row, err = s.repo.GetBySellerIDWithUsername(ctx, tx, sellerID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

// ListPendingVerifications retrieves all submissions awaiting admin review.
func (s *VerificationService) ListPendingVerifications(
	ctx context.Context,
) ([]*entity.SellerVerification, error) {
	var verifications []*entity.SellerVerification
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		verifications, err = s.repo.ListByStatus(ctx, tx, entity.StatusPendingReview)
		return err
	})
	if err != nil {
		return nil, err
	}
	return verifications, nil
}

// ListPendingVerificationsWithUsername retrieves all pending_review submissions
// joined with each seller's public username for the admin review queue.
func (s *VerificationService) ListPendingVerificationsWithUsername(
	ctx context.Context,
) ([]repository.PendingVerificationRow, error) {
	var result []repository.PendingVerificationRow
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = s.repo.ListPendingWithUsername(ctx, tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListVerificationsByStatusWithUsername retrieves all verifications matching the
// given status, joined with each seller's public username from user_profiles.
func (s *VerificationService) ListVerificationsByStatusWithUsername(
	ctx context.Context,
	status entity.Status,
) ([]repository.PendingVerificationRow, error) {
	var result []repository.PendingVerificationRow
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = s.repo.ListByStatusWithUsername(ctx, tx, status)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *VerificationService) writeAuditTx(
	ctx context.Context,
	tx db.Tx,
	adminID uuid.UUID,
	action string,
	sellerID uuid.UUID,
	metadata map[string]interface{},
) error {
	if s.auditLogger == nil {
		return nil
	}
	if err := s.auditLogger.LogTx(ctx, tx, adminID, action, TargetTypeSellerVerification, sellerID, metadata); err != nil {
		return fmt.Errorf("verification: audit log failed: %w", err)
	}
	return nil
}

func (s *VerificationService) emitEventTx(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	sellerID uuid.UUID,
	payload map[string]interface{},
) error {
	if s.outboxRepo == nil {
		return nil
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("verification: marshal event payload: %w", err)
	}
	if err := s.outboxRepo.InsertEvent(ctx, tx, eventType, sellerID, payloadBytes); err != nil {
		return fmt.Errorf("outbox %s: %w", eventType, err)
	}
	return nil
}


