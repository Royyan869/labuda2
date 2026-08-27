// ⚠️ FINANCIAL RULE:
// All money operations MUST go through WalletService.
// Direct balance mutation is forbidden.
//
// ⚠️ Finance domain is NOT financial authority.
// It is ONLY for billing ledger and reporting.
package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	bankaccountrepo "github.com/labuda/backend/internal/finance/bankaccount/infrastructure/repository"
	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/capability"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// WithdrawVerificationChecker is the minimal interface that WithdrawService
// requires from the verification domain. Using an interface (instead of the
// concrete *VerificationService) keeps the finance domain import-clean and
// makes unit tests trivial to write without a full verification stack.
type WithdrawVerificationChecker interface {
	// IsSellerVerifiedTx checks whether the seller holds payout authority
	// (verified KYC status) inside an existing transaction.
	IsSellerVerifiedTx(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (bool, error)
	// IsReviewedBankAccountTx checks whether bankAccountID was captured in
	// reviewed_bank_account_ids at the most recent KYC approval.
	IsReviewedBankAccountTx(ctx context.Context, tx db.Tx, sellerID, bankAccountID uuid.UUID) (bool, error)
}

// Withdrawal limit constants.
//
// CANONICAL MONEY UNIT (PASS_18H): all amounts here are Rupiah integers.
// There is no cents/sen subunit anywhere in Labuda's money model — 1 unit = Rp 1.
const (
	// MaxWithdrawalAmount is the maximum amount that can be withdrawn in a single request.
	MaxWithdrawalAmount = 50_000_000 // Rp 50,000,000

	// MinWithdrawalAmount is the minimum amount for a withdrawal request.
	MinWithdrawalAmount = 10_000 // Rp 10,000

	// WithdrawalFeeAmount is the fixed seller withdrawal fee, in Rupiah.
	// Owner policy: net_payout = requested_amount - WithdrawalFeeAmount.
	// The fee is deducted FROM the requested amount, never added on top of it.
	WithdrawalFeeAmount int64 = 5_000 // Rp 5,000
)

// withdrawalCanonicalAuthority is the canonical finance surface used by
// WithdrawService. FinanceService implements this interface; tests can inject
// a focused spy without requiring a full ledger stack.
type withdrawalCanonicalAuthority interface {
	AssertSellerWithdrawalAllowed(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount int64) (*SellerWithdrawableSummary, error)
	RecordWithdrawalRequest(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount, feeAmount int64, withdrawalID uuid.UUID) error
	RecordWithdrawalCommit(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount, feeAmount int64, withdrawalID uuid.UUID) error
	RecordWithdrawalReject(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount, feeAmount int64, withdrawalID uuid.UUID) error
	RecordWithdrawalRestore(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount, feeAmount int64, withdrawalID uuid.UUID) error
	RecordWithdrawalComplete(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount, feeAmount int64, withdrawalID uuid.UUID) error
}

// Transactor represents the ability to execute functions within transactions.
// This interface allows both real database.DB and mocks to be used.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// WithdrawService handles seller withdrawal operations.
// Flow: Request → Hold → Approve → Process → Done
//
//	↘ Reject → Funds returned
//
// NOTIFICATION CONTINUITY:
// Emits truthful notification events for withdrawal lifecycle state changes.
type WithdrawService struct {
	db                   Transactor
	ledgerRepo           *repository.LedgerRepository
	withdrawRepo         *repository.WithdrawRepository
	bankAccountRepo      *bankaccountrepo.BankAccountRepository
	roleChecker          auth.RoleChecker
	accountStatusChecker auth.AccountStatusChecker
	ownership            *auth.OwnershipValidator
	adminAuditLogger     audit.AdminAuditLogger
	verificationService  WithdrawVerificationChecker
	outboxRepo           *outboxrepo.OutboxRepository

	// canonicalAuthority owns the freeze-aware withdrawable gate
	// (AssertSellerWithdrawalAllowed) and the canonical request-time ledger
	// reserve (RecordWithdrawalRequest). Wired post-construction via
	// SetCanonicalAuthority. nil → RequestWithdrawal fail-closes.
	canonicalAuthority withdrawalCanonicalAuthority
}

// SetCanonicalAuthority wires the FinanceService that owns the canonical
// withdrawal authority surface. Must be called at boot before
// RequestWithdrawal is exercised; otherwise the request path returns a
// fail-closed configuration error.
func (s *WithdrawService) SetCanonicalAuthority(fs withdrawalCanonicalAuthority) {
	s.canonicalAuthority = fs
}

// NewWithdrawService creates a new WithdrawService.
func NewWithdrawService(
	db Transactor,
	ledgerRepo *repository.LedgerRepository,
	withdrawRepo *repository.WithdrawRepository,
	bankAccountRepo *bankaccountrepo.BankAccountRepository,
	roleChecker auth.RoleChecker,
	accountStatusChecker auth.AccountStatusChecker,
	adminAuditLogger audit.AdminAuditLogger,
	verificationService WithdrawVerificationChecker,
	outboxRepo *outboxrepo.OutboxRepository,
) *WithdrawService {
	return &WithdrawService{
		db:                   db,
		ledgerRepo:           ledgerRepo,
		withdrawRepo:         withdrawRepo,
		bankAccountRepo:      bankAccountRepo,
		roleChecker:          roleChecker,
		accountStatusChecker: accountStatusChecker,
		ownership:            auth.NewOwnershipValidator(),
		adminAuditLogger:     adminAuditLogger,
		verificationService:  verificationService,
		outboxRepo:           outboxRepo,
	}
}

// RequestWithdrawRequest represents a withdrawal request.
type RequestWithdrawRequest struct {
	CallerID uuid.UUID // The user requesting the withdrawal
	Amount   money.Money
}

// RequestWithdrawResponse is the response after creating a withdrawal request.
type RequestWithdrawResponse struct {
	WithdrawalID uuid.UUID
	Status       repository.WithdrawalStatus
}

// RequestWithdraw creates a withdrawal request and holds funds.
// AUTHORIZATION: Only the account owner can request withdrawal from their account.
//
// LEDGER-FIRST ATOMICITY (single db.WithTx):
// 0. VERIFICATION: Check seller is verified (BEFORE any ledger logic)
// 1. Validate amount > 0
// 2. Get PER-USER seller payable account (NOT system account)
// 3. Lock user's SELLER_PAYABLE account with FOR UPDATE
// 4. Check seller payable balance ≥ amount (AFTER lock)
// 5. Check for existing withdrawal (idempotency)
// 6. Fetch default bank account FOR UPDATE (fails if none exists)
// 7. Snapshot bank details (name, account number, holder name)
// 8. Create ledger transaction FIRST: DR user's SELLER_PAYABLE, CR WITHDRAWAL_PENDING
// 9. Insert withdrawal record SECOND with bank snapshots (status = REQUESTED)
// 10. Commit
//
// CRASH SAFETY:
// - If crash before ledger → no withdrawal record, no funds moved (clean retry)
// - If crash after ledger → withdrawal insert fails, transaction rolls back (clean retry)
// - If crash after withdrawal insert → both exist, recovery via RejectWithdraw
//
// ⛔ RequestWithdraw is REMOVED as part of PHASE 3.1 HARDENING
//
// The old split-transaction withdrawal flow has been completely removed.
//
// MIGRATION PATH:
// ❌ OLD: WithdrawService.RequestWithdraw(ctx, withdrawID, req)
// ✅ NEW: WalletService.RequestWithdrawalUnifiedTx(ctx, input)
//
// WHY REMOVED:
// - Old flow used split transactions (wallet + finance separately)
// - This caused wallet-finance drift when transactions partially committed
// - New unified flow ensures atomic wallet+finance mutations in single transaction
//
// ALL WITHDRAWAL REQUESTS MUST NOW GO THROUGH:
// 1. Public API: WalletService.RequestWithdrawalUnifiedTx(ctx, input)
// 2. Internal API: WalletService.RequestWithdrawalUnified(ctx, tx, input)
//
// This is a HARD BLOCK - calling this method will panic with clear migration guidance.
func (s *WithdrawService) RequestWithdraw(
	ctx context.Context,
	withdrawID uuid.UUID,
	req RequestWithdrawRequest,
) (*RequestWithdrawResponse, error) {
	panic(`

════════════════════════════════════════════════════════════════════════════════
🔥 HARD BLOCK: WithdrawService.RequestWithdraw() REMOVED (PHASE 3.1)
════════════════════════════════════════════════════════════════════════════════

The old withdrawal flow has been REMOVED as part of PHASE 3.1 final hardening.

MIGRATION REQUIRED:
  ❌ OLD: WithdrawService.RequestWithdraw(ctx, withdrawID, req)
  ✅ NEW: WalletService.RequestWithdrawalUnifiedTx(ctx, input)

WHERE TO UPDATE:
  - Handler: core/wallet/delivery/http/withdrawal_handler_unified.go (canonical)
  - Tests: Update to use WalletService instead of WithdrawService
  - Workers: Update to use unified flow

WHY REMOVED:
  - Old flow used split transactions (wallet + finance separately)
  - This caused wallet-finance drift when transactions partially committed
  - New unified flow ensures atomic wallet+finance mutations in single transaction

CRITICAL FINANCIAL GUARANTEE:
  - All withdrawal mutations MUST happen in single transaction
  - Wallet deduction + Finance ledger + Withdrawal record = ONE atomic operation
  - No split-brain possible with new flow

════════════════════════════════════════════════════════════════════════════════
`)
}

// ============================================================================
// CANONICAL WITHDRAWAL REQUEST PATH (finance-shape)
// ============================================================================
//
// RequestWithdrawal is the canonical entry point for seller-initiated payout
// requests. It produces a FINANCE-SHAPED withdrawals row (status=REQUESTED,
// seller_id populated, idempotency_key populated, bank snapshots populated)
// that is directly compatible with the downstream admin lifecycle
// (ApproveWithdraw / RejectWithdraw / MarkProcessed), the PayoutWorker
// (polls status='PROCESSING'), and the PayoutWebhookHandler.
//
// All work happens inside a single db.WithTx so the verification check,
// authority gate, duplicate guard, bank lock, row insert, and ledger
// reserve are atomic.

// CanonicalRequestWithdrawalInput is the request payload for the canonical
// finance-shape withdrawal request path.
type CanonicalRequestWithdrawalInput struct {
	SellerID uuid.UUID
	Amount   int64
}

// CanonicalRequestWithdrawalOutput is the response of RequestWithdrawal.
type CanonicalRequestWithdrawalOutput struct {
	WithdrawalID     uuid.UUID
	SellerID         uuid.UUID
	Amount           int64
	FeeAmount        int64
	TotalDebitAmount int64
	Status           repository.WithdrawalStatus
}

// ErrCanonicalAuthorityNotConfigured is returned when SetCanonicalAuthority
// was never called at boot.
var ErrCanonicalAuthorityNotConfigured = fmt.Errorf("withdraw: canonical authority not configured")

// ErrSellerNotVerified is returned when a seller without a verified
// verification record attempts to withdraw.
type ErrSellerNotVerified struct {
	SellerID uuid.UUID
}

func (e *ErrSellerNotVerified) Error() string {
	return fmt.Sprintf("withdraw: seller %s is not verified", e.SellerID)
}

// ErrWithdrawalPendingExists is returned when the seller already has an
// in-flight (non-terminal) withdrawal row. Carries the existing
// withdrawal_id so the API can echo it back to the caller for support /
// retry reconciliation.
type ErrWithdrawalPendingExists struct {
	SellerID             uuid.UUID
	ExistingWithdrawalID uuid.UUID
	ExistingStatus       repository.WithdrawalStatus
}

func (e *ErrWithdrawalPendingExists) Error() string {
	return fmt.Sprintf("withdraw: in-flight withdrawal already exists for seller %s (id=%s status=%s)",
		e.SellerID, e.ExistingWithdrawalID, e.ExistingStatus)
}

// ErrNoDefaultBankAccount is returned when the seller has no default
// bank account configured.
type ErrNoDefaultBankAccount struct {
	SellerID uuid.UUID
}

func (e *ErrNoDefaultBankAccount) Error() string {
	return fmt.Sprintf("withdraw: seller %s has no default bank account", e.SellerID)
}

// ErrBankAccountNotReviewed is returned by GUARD 5 when the seller's default
// bank account was not present at the time of the most recent KYC approval.
// Accounts added or changed post-approval are not payout-eligible until a
// re-approval snapshots them into reviewed_bank_account_ids.
type ErrBankAccountNotReviewed struct {
	SellerID      uuid.UUID
	BankAccountID uuid.UUID
}

func (e *ErrBankAccountNotReviewed) Error() string {
	return fmt.Sprintf(
		"withdraw: bank account %s for seller %s has not been reviewed for payout; "+
			"please use a reviewed account or request re-verification",
		e.BankAccountID, e.SellerID,
	)
}

// ErrWithdrawalAmountOutOfRange is returned when amount < min or > max.
type ErrWithdrawalAmountOutOfRange struct {
	Amount int64
	Min    int64
	Max    int64
}

func (e *ErrWithdrawalAmountOutOfRange) Error() string {
	return fmt.Sprintf("withdraw: amount %d out of range [%d..%d]", e.Amount, e.Min, e.Max)
}

// RequestWithdrawal creates a finance-shape withdrawal row and posts the
// canonical ledger reserve in a single atomic transaction.
//
// MONEY MODEL (PASS_18H, owner-confirmed):
//   - input.Amount is the FULL amount reserved from SELLER_PAYABLE — the fee
//     is deducted FROM it, never added on top. E.g. a Rp100,000 request
//     reserves exactly Rp100,000 (not Rp105,000).
//   - The fee/net split only happens at final settlement (RecordWithdrawalComplete
//     / gateway webhook settlement): net payout = Amount - Fee to the seller's
//     bank, Fee booked as platform revenue.
//   - Every intermediate ledger movement (request/commit/reject/restore) moves
//     the SAME reserved Amount between holding accounts; feeAmount is carried
//     alongside for storage/display and for the final split, not added to the
//     movement itself.
//
// LIFECYCLE WIRING:
//   - Status set to REQUESTED → admin ApproveWithdraw / RejectWithdraw accept it.
//   - idempotency_key set to "withdrawal_request_<id>" → matches the ledger
//     idempotency surface of FinanceService.RecordWithdrawalRequest.
//   - bank_*_snapshot copied from the seller's default bank account FOR UPDATE.
//   - Ledger reservation DR SELLER_PAYABLE / CR WITHDRAWAL_PENDING posted in
//     the same tx so SELLER_PAYABLE can never decrease without the
//     withdrawals row that owns the reservation.
//
// HTTP RETRY NOTE: this API does not currently accept a caller-provided
// idempotency key, so the server-generated key is unique per row. Naive HTTP
// retries are protected against double-spend by the "single in-flight per
// seller" guard which surfaces ErrWithdrawalPendingExists carrying the
// existing withdrawal_id; clients should reconcile against that id.
func (s *WithdrawService) RequestWithdrawal(
	ctx context.Context,
	input CanonicalRequestWithdrawalInput,
) (*CanonicalRequestWithdrawalOutput, error) {
	if s.canonicalAuthority == nil {
		return nil, ErrCanonicalAuthorityNotConfigured
	}
	if input.SellerID == uuid.Nil {
		return nil, fmt.Errorf("withdraw: seller_id required")
	}
	if input.Amount < MinWithdrawalAmount || input.Amount > MaxWithdrawalAmount {
		return nil, &ErrWithdrawalAmountOutOfRange{
			Amount: input.Amount,
			Min:    MinWithdrawalAmount,
			Max:    MaxWithdrawalAmount,
		}
	}

	// GUARD 0 — Account must be active. Suspended, banned, removed, or
	// otherwise inactive accounts MUST NOT initiate withdrawals regardless
	// of verification status. This is the same canonical gate used by all
	// other service-layer mutation surfaces (order, listing, auction, chat).
	// EnsureActive checks both account_status and deleted_at.
	if err := s.accountStatusChecker.EnsureActive(ctx, input.SellerID); err != nil {
		return nil, fmt.Errorf("withdraw: seller account not active: %w", err)
	}

	var output *CanonicalRequestWithdrawalOutput
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		feeAmount := WithdrawalFeeAmount

		// GUARD 1 — Verified seller (tx-aware so it serializes against the
		// SELLER_PAYABLE lock taken in the authority gate below).
		verified, err := s.verificationService.IsSellerVerifiedTx(ctx, tx, input.SellerID)
		if err != nil {
			return fmt.Errorf("check seller verification: %w", err)
		}
		if !verified {
			return &ErrSellerNotVerified{SellerID: input.SellerID}
		}

		// GUARD 2 — Canonical dispute-aware authority. Locks SELLER_PAYABLE
		// FOR UPDATE and subtracts active dispute freezes.
		// Returns ErrWithdrawalBlockedByWithdrawableBalance when over the ceiling.
		// input.Amount is the FULL amount reserved — the fee is deducted from
		// it at settlement, not added on top (see money model note above).
		if _, err := s.canonicalAuthority.AssertSellerWithdrawalAllowed(ctx, tx, input.SellerID, input.Amount); err != nil {
			return err
		}

		// GUARD 3 — Single in-flight withdrawal per seller. Excludes
		// terminal statuses (SETTLED/COMPLETED/FAILED/FAILED_FINAL) so a
		// fresh request after a clean settlement is allowed.
		active, err := s.withdrawRepo.GetActiveBySellerID(ctx, tx, input.SellerID)
		if err != nil {
			return fmt.Errorf("check active withdrawal: %w", err)
		}
		if active != nil {
			return &ErrWithdrawalPendingExists{
				SellerID:             input.SellerID,
				ExistingWithdrawalID: active.ID,
				ExistingStatus:       active.Status,
			}
		}

		// GUARD 4 — Default bank account locked FOR UPDATE. The snapshot
		// is what the payout worker submits to the gateway; it must not
		// drift after request time.
		bank, err := s.bankAccountRepo.GetDefaultBySeller(ctx, tx, input.SellerID)
		if err != nil {
			return &ErrNoDefaultBankAccount{SellerID: input.SellerID}
		}

		// GUARD 5 — Reviewed bank account check (BANK_ACCOUNT_REVIEWED_FOR_PAYOUT_POLICY).
		// The seller's default bank account must appear in the reviewed_bank_account_ids
		// snapshot captured at the most recent KYC approval. Accounts added or changed
		// after KYC approval are not payout-eligible until a re-approval includes them.
		reviewed, err := s.verificationService.IsReviewedBankAccountTx(ctx, tx, input.SellerID, bank.ID)
		if err != nil {
			return fmt.Errorf("check reviewed bank account: %w", err)
		}
		if !reviewed {
			return &ErrBankAccountNotReviewed{
				SellerID:      input.SellerID,
				BankAccountID: bank.ID,
			}
		}

		// CREATE — finance-shape row.
		withdrawalID := uuid.New()
		idemKey := fmt.Sprintf("withdrawal_request_%s", withdrawalID.String())
		if err := s.withdrawRepo.CreateWithIdempotency(
			ctx, tx,
			withdrawalID,
			input.SellerID,
			input.Amount,
			feeAmount,
			repository.WithdrawalStatusRequested,
			idemKey,
			bank.BankName,
			bank.BankCode,
			bank.AccountNumber,
			bank.AccountHolderName,
		); err != nil {
			return fmt.Errorf("create withdrawal: %w", err)
		}

		// LEDGER RESERVE — SELLER_PAYABLE → WITHDRAWAL_PENDING. Idempotency
		// key is withdrawal_request_<id>; the unique constraint on
		// financial_transactions makes this safe under same-tx replay.
		if err := s.canonicalAuthority.RecordWithdrawalRequest(ctx, tx, input.SellerID, input.Amount, feeAmount, withdrawalID); err != nil {
			return fmt.Errorf("record withdrawal request ledger: %w", err)
		}

		// OUTBOX — withdrawal.requested for downstream notification/audit.
		if s.outboxRepo != nil {
			payload := map[string]interface{}{
				"withdrawal_id": withdrawalID.String(),
				"seller_id":     input.SellerID.String(),
				"amount":        input.Amount,
				"fee_amount":    feeAmount,
				"net_payout":    input.Amount - feeAmount,
				"total_debit":   input.Amount,
			}
			payloadBytes, _ := json.Marshal(payload)
			_ = s.outboxRepo.InsertEvent(ctx, tx, "withdrawal.requested", withdrawalID, payloadBytes)
		}

		// TotalDebitAmount == Amount: the full requested amount is what is
		// reserved from SELLER_PAYABLE. The fee/net split happens only at
		// final settlement (see money model note above).
		output = &CanonicalRequestWithdrawalOutput{
			WithdrawalID:     withdrawalID,
			SellerID:         input.SellerID,
			Amount:           input.Amount,
			FeeAmount:        feeAmount,
			TotalDebitAmount: input.Amount,
			Status:           repository.WithdrawalStatusRequested,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return output, nil
}

// ApproveWithdraw approves a pending withdrawal request.
// AUTHORIZATION: Only admin can approve withdrawals.
//
// WITHDRAWAL CONSISTENCY FIX:
// This method now atomically commits the withdrawal in the ledger.
// The PROCESSING state is now ledger-backed by WITHDRAWAL_COMMITTED,
// ensuring that "ready for payout" always has financial state aligned.
//
// Steps:
//  1. Lock withdrawal FOR UPDATE
//  2. Status must be REQUESTED
//  3. Ledger move: DR WITHDRAWAL_PENDING, CR WITHDRAWAL_COMMITTED
//     This transitions funds from "pending review" to "approved for payout"
//  4. Update status → PROCESSING
//  5. Log admin action (ATOMIC - within transaction)
//
// The ledger movement ensures:
// - WITHDRAWAL_PENDING tracks withdrawals awaiting admin approval
// - WITHDRAWAL_COMMITTED tracks admin-approved withdrawals ready for gateway
// - No payout-ready state exists without corresponding ledger movement
func (s *WithdrawService) ApproveWithdraw(
	ctx context.Context,
	callerID uuid.UUID,
	withdrawID uuid.UUID,
) error {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return err
	}

	// 🔥 P0 SECURITY: Capability check for withdrawal approval
	// System caller bypasses capability check
	if !auth.IsSystemCaller(callerID) {
		if !capability.HasCapability(ctx, capability.CapFinanceWithdrawReview.String()) {
			return fmt.Errorf("forbidden: missing capability %s", capability.CapFinanceWithdrawReview.String())
		}
	}

	if s.canonicalAuthority == nil {
		return ErrCanonicalAuthorityNotConfigured
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Lock withdrawal row
		w, err := s.withdrawRepo.LockForUpdate(ctx, tx, withdrawID)
		if err != nil {
			return err
		}

		// Guard: status must be pending
		if w.Status != repository.WithdrawalStatusRequested {
			return fmt.Errorf("withdraw: cannot approve withdrawal with status %s", w.Status)
		}

		// PATCH D — Re-check seller verification inside the approval tx.
		// The seller may have been suspended or revoked after the withdrawal was
		// requested; committing payout for a non-approved seller violates the
		// BANK_ACCOUNT_REVIEWED_FOR_PAYOUT_POLICY. If verification has changed,
		// fail the approval so the admin must reject or wait for re-approval.
		reVerified, err := s.verificationService.IsSellerVerifiedTx(ctx, tx, w.SellerID)
		if err != nil {
			return fmt.Errorf("re-check seller verification: %w", err)
		}
		if !reVerified {
			return &ErrSellerNotVerified{SellerID: w.SellerID}
		}

		// LEDGER CONSISTENCY: Move funds from WITHDRAWAL_PENDING to WITHDRAWAL_COMMITTED.
		// Delegates to FinanceService canonical authority (idem: withdrawal_commit_<id>).
		if err := s.canonicalAuthority.RecordWithdrawalCommit(ctx, tx, w.SellerID, w.Amount, w.FeeAmount, withdrawID); err != nil {
			return fmt.Errorf("commit withdrawal in ledger: %w", err)
		}

		// Update status to approved AFTER ledger succeeds
		rowsAffected, err := s.withdrawRepo.UpdateStatusWithCheck(
			ctx, tx, withdrawID,
			repository.WithdrawalStatusRequested,
			repository.WithdrawalStatusProcessing,
		)
		if err != nil {
			return fmt.Errorf("update status: %w", err)
		}

		// Verify the update actually happened
		if rowsAffected == 0 {
			return fmt.Errorf("withdraw: status transition failed, likely concurrent modification")
		}

		// Log admin action (ATOMIC - within transaction)
		// If audit logging fails, the entire transaction rolls back
		if err := s.adminAuditLogger.LogTx(ctx, tx, callerID,
			audit.ActionWithdrawApproved,
			audit.TargetTypeWithdrawal,
			withdrawID,
			map[string]interface{}{
				"seller_id":  w.SellerID,
				"amount":     w.Amount,
				"fee_amount": w.FeeAmount,
			},
		); err != nil {
			return fmt.Errorf("audit log failed: %w", err)
		}

		// NOTIFICATION CONTINUITY: Emit withdrawal approved event
		// This is non-fatal - if event emission fails, the approval still succeeds
		if s.outboxRepo != nil {
			payload := map[string]interface{}{
				"withdrawal_id": withdrawID.String(),
				"seller_id":     w.SellerID.String(),
				"amount":        w.Amount,
				"fee_amount":    w.FeeAmount,
				"approved_by":   callerID.String(),
			}
			payloadBytes, _ := json.Marshal(payload)
			_ = s.outboxRepo.InsertEvent(ctx, tx, "withdrawal.approved", withdrawID, payloadBytes)
		}

		return nil
	})
}

// RejectWithdraw rejects a withdrawal and returns funds to the seller.
// AUTHORIZATION: Only admin can reject withdrawals.
//
// WITHDRAWAL CONSISTENCY FIX:
// Now handles rejection from both REQUESTED and PROCESSING states,
// returning funds from the correct ledger account.
//
// For REQUESTED withdrawals:
// - Funds are in WITHDRAWAL_PENDING
// - Ledger move: DR WITHDRAWAL_PENDING, CR user's SELLER_PAYABLE
//
// For PROCESSING withdrawals (approved but not submitted):
// - Funds are in WITHDRAWAL_COMMITTED
// - Ledger move: DR WITHDRAWAL_COMMITTED, CR user's SELLER_PAYABLE
//
// Steps:
//  1. Lock withdrawal FOR UPDATE
//  2. Status must be REQUESTED or PROCESSING
//  3. Delegate to FinanceService canonical authority:
//     REQUESTED  → RecordWithdrawalReject  (idem: withdrawal_reject_<id>)
//     PROCESSING → RecordWithdrawalRestore (idem: withdrawal_restore_<id>)
//  4. Update status → FAILED
//  5. Log admin action (ATOMIC - within transaction)
func (s *WithdrawService) RejectWithdraw(
	ctx context.Context,
	callerID uuid.UUID,
	withdrawID uuid.UUID,
) error {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return err
	}

	// 🔥 P0 SECURITY: Capability check for withdrawal rejection
	// System caller bypasses capability check
	if !auth.IsSystemCaller(callerID) {
		if !capability.HasCapability(ctx, capability.CapFinanceWithdrawReview.String()) {
			return fmt.Errorf("forbidden: missing capability %s", capability.CapFinanceWithdrawReview.String())
		}
	}

	if s.canonicalAuthority == nil {
		return ErrCanonicalAuthorityNotConfigured
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Lock withdrawal row
		w, err := s.withdrawRepo.LockForUpdate(ctx, tx, withdrawID)
		if err != nil {
			return err
		}

		// LEDGER CONSISTENCY: Return funds to seller via FinanceService canonical authority.
		// REQUESTED  → RecordWithdrawalReject  (DR WITHDRAWAL_PENDING,   CR SELLER_PAYABLE)
		// PROCESSING → RecordWithdrawalRestore (DR WITHDRAWAL_COMMITTED, CR SELLER_PAYABLE)
		if w.Status == repository.WithdrawalStatusRequested {
			if err := s.canonicalAuthority.RecordWithdrawalReject(ctx, tx, w.SellerID, w.Amount, w.FeeAmount, withdrawID); err != nil {
				return fmt.Errorf("return funds (reject): %w", err)
			}
		} else if w.Status == repository.WithdrawalStatusProcessing {
			if err := s.canonicalAuthority.RecordWithdrawalRestore(ctx, tx, w.SellerID, w.Amount, w.FeeAmount, withdrawID); err != nil {
				return fmt.Errorf("return funds (restore): %w", err)
			}
		} else {
			return fmt.Errorf("withdraw: cannot reject withdrawal with status %s", w.Status)
		}

		// Update status to rejected
		// Handle both REQUESTED and PROCESSING → FAILED transition
		rowsAffected, err := s.withdrawRepo.UpdateStatus(
			ctx, tx, withdrawID,
			repository.WithdrawalStatusFailed,
		)
		if err != nil {
			return fmt.Errorf("update status: %w", err)
		}

		// Verify the update actually happened
		if rowsAffected == 0 {
			return fmt.Errorf("withdraw: status transition failed, likely concurrent modification")
		}

		// Log admin action (ATOMIC - within transaction)
		// If audit logging fails, the entire transaction rolls back
		if err := s.adminAuditLogger.LogTx(ctx, tx, callerID,
			audit.ActionWithdrawRejected,
			audit.TargetTypeWithdrawal,
			withdrawID,
			map[string]interface{}{
				"seller_id":   w.SellerID,
				"amount":      w.Amount,
				"fee_amount":  w.FeeAmount,
				"from_status": string(w.Status),
			},
		); err != nil {
			return fmt.Errorf("audit log failed: %w", err)
		}

		// NOTIFICATION CONTINUITY: Emit withdrawal rejected event
		// This is non-fatal - if event emission fails, the rejection still succeeds
		if s.outboxRepo != nil {
			payload := map[string]interface{}{
				"withdrawal_id": withdrawID.String(),
				"seller_id":     w.SellerID.String(),
				"amount":        w.Amount,
				"fee_amount":    w.FeeAmount,
				"rejected_by":   callerID.String(),
				"from_status":   string(w.Status),
			}
			payloadBytes, _ := json.Marshal(payload)
			_ = s.outboxRepo.InsertEvent(ctx, tx, "withdrawal.rejected", withdrawID, payloadBytes)
		}

		return nil
	})
}

// MarkProcessed marks an approved withdrawal as processed after bank transfer.
// AUTHORIZATION: Only admin or system caller can process withdrawals.
//
// CRITICAL: This method is for MANUAL COMPLETION ONLY, used when:
// 1. External bank transfer was done manually (outside the payment gateway)
// 2. Exceptional recovery requiring admin intervention
//
// GUARD: Cannot be used if withdrawal has already been submitted to a gateway
// (i.e., external_reference_id is set). This prevents bypassing the canonical
// webhook settlement flow which is the source of truth for gateway payouts.
//
// WITHDRAWAL CONSISTENCY FIX:
// Funds are moved from WITHDRAWAL_COMMITTED (where approved withdrawals sit)
// to PLATFORM_BANK, completing the ledger journey.
//
// For automated gateway integration, webhook settlement is the canonical path.
// Steps:
// 1. Lock withdrawal FOR UPDATE
// 2. Status must be PROCESSING
// 3. GUARD: external_reference_id must NOT be set (not submitted to gateway)
// 4. Ledger move: DR WITHDRAWAL_COMMITTED, CR PLATFORM_BANK (funds leave the system)
// 5. Idempotency key: withdraw_manual_complete_<withdrawID>
// 6. Update status → COMPLETED (manual admin completion; gateway path uses SETTLED)
// 7. Log admin action (ATOMIC - within transaction)
func (s *WithdrawService) MarkProcessed(
	ctx context.Context,
	callerID uuid.UUID,
	withdrawID uuid.UUID,
) error {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return err
	}

	// AUTHORIZATION: Capability check for manual withdrawal completion
	// System caller bypasses capability check (workers/webhooks)
	if !auth.IsSystemCaller(callerID) {
		if !capability.HasCapability(ctx, capability.CapFinanceWithdrawReview.String()) {
			return fmt.Errorf("forbidden: missing capability %s", capability.CapFinanceWithdrawReview.String())
		}
	}

	if s.canonicalAuthority == nil {
		return ErrCanonicalAuthorityNotConfigured
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Lock withdrawal row
		w, err := s.withdrawRepo.LockForUpdate(ctx, tx, withdrawID)
		if err != nil {
			return err
		}

		// Guard: status must be approved (PROCESSING)
		if w.Status != repository.WithdrawalStatusProcessing {
			return fmt.Errorf("withdraw: cannot process withdrawal with status %s", w.Status)
		}

		// CRITICAL GUARD: Prevent bypassing canonical webhook settlement
		// If external_reference_id is set, the payout was submitted to a gateway
		// and MUST be settled via webhook callback, not manual completion.
		if w.ExternalReferenceID != "" {
			return fmt.Errorf("withdraw: cannot manually complete - payout already submitted to gateway (external_ref=%s). Use webhook settlement for canonical finality", w.ExternalReferenceID)
		}

		if err := s.canonicalAuthority.RecordWithdrawalComplete(ctx, tx, w.SellerID, w.Amount, w.FeeAmount, withdrawID); err != nil {
			return fmt.Errorf("process withdrawal: %w", err)
		}

		// Update status to COMPLETED (manual admin completion).
		// SETTLED is reserved exclusively for gateway webhook confirmation.
		// COMPLETED marks that an admin performed the bank transfer manually.
		rowsAffected, err := s.withdrawRepo.UpdateStatusWithCheck(
			ctx, tx, withdrawID,
			repository.WithdrawalStatusProcessing,
			repository.WithdrawalStatusCompleted,
		)
		if err != nil {
			return fmt.Errorf("update status: %w", err)
		}

		// Verify the update actually happened
		if rowsAffected == 0 {
			return fmt.Errorf("withdraw: status transition failed, likely concurrent modification")
		}

		// Log admin action (ATOMIC - within transaction)
		// If audit logging fails, the entire transaction rolls back
		if err := s.adminAuditLogger.LogTx(ctx, tx, callerID,
			audit.ActionWithdrawProcessed,
			audit.TargetTypeWithdrawal,
			withdrawID,
			map[string]interface{}{
				"seller_id":         w.SellerID,
				"amount":            w.Amount,
				"fee_amount":        w.FeeAmount,
				"completion_method": "manual",
			},
		); err != nil {
			return fmt.Errorf("audit log failed: %w", err)
		}

		// OUTBOX ATOMIC: Emit withdrawal.completed in the same transaction.
		// InsertEvent failure rolls back the entire transaction — no business
		// mutation commits without its critical notification event.
		if s.outboxRepo != nil {
			payload := map[string]interface{}{
				"withdrawal_id": withdrawID.String(),
				"seller_id":     w.SellerID.String(),
				"amount":        w.Amount,
				"fee_amount":    w.FeeAmount,
			}
			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("marshal withdrawal.completed payload: %w", err)
			}
			if err := s.outboxRepo.InsertEvent(ctx, tx, "withdrawal.completed", withdrawID, payloadBytes); err != nil {
				return fmt.Errorf("outbox withdrawal.completed: %w", err)
			}
		}

		return nil
	})
}

// GetWithdrawal retrieves a withdrawal by ID.
func (s *WithdrawService) GetWithdrawal(
	ctx context.Context,
	withdrawID uuid.UUID,
) (*repository.Withdrawal, error) {
	var w *repository.Withdrawal
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		w, err = s.withdrawRepo.GetByID(ctx, tx, withdrawID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return w, nil
}

// ListWithdrawalsBySeller retrieves paginated withdrawals for a seller from the
// canonical finance withdrawal repository.
//
// limit/offset pagination is converted to the finance repository's 1-based
// page/pageSize format internally.
func (s *WithdrawService) ListWithdrawalsBySeller(
	ctx context.Context,
	sellerID uuid.UUID,
	limit, offset int,
) ([]*repository.Withdrawal, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Convert limit/offset → 1-based page/pageSize.
	// Integer division: page = offset/limit + 1.
	page := 1
	if offset > 0 {
		page = offset/limit + 1
	}

	filters := repository.WithdrawalListFilters{
		SellerID: &sellerID,
		Page:     page,
		PageSize: limit,
	}

	var (
		withdrawals []*repository.Withdrawal
		total       int64
	)
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var listErr error
		withdrawals, listErr = s.withdrawRepo.ListWithFilters(ctx, tx, filters)
		if listErr != nil {
			return listErr
		}
		total, listErr = s.withdrawRepo.CountWithFilters(ctx, tx, filters)
		return listErr
	})
	if err != nil {
		return nil, 0, err
	}
	return withdrawals, total, nil
}
