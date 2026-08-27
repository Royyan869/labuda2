package application

// Tests for VerificationService.MarkBankAccountReviewed.
//
// Scenarios covered:
//   A – happy path: approved seller, active unreviewed account → appended, Update called
//   B – idempotent: already-reviewed account → no-op, Update NOT called
//   C – seller not in approved status → ErrMarkReviewedNotApproved
//   D – bank account not found for seller → ErrBankAccountNotFoundForSeller
//   E – no verification record for seller → error (not-found)
//   F – bankAcctReader not wired → configuration error
//   G – entity unit: AppendReviewedBankAccount deduplication

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	bankaccountEntity "github.com/labuda/backend/internal/finance/bankaccount/entity"
	"github.com/labuda/backend/internal/governance/verification/entity"
	"github.com/labuda/backend/internal/governance/verification/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// ---------------------------------------------------------------------------
// Helper: build a VerificationService for MarkBankAccountReviewed tests
// ---------------------------------------------------------------------------

func buildMarkReviewedService(
	txFactory func() db.Tx,
	bankAcctReader BankAccountsReader,
) *VerificationService {
	svc := NewVerificationService(
		&verificationMockDB{txFactory: txFactory},
		repository.NewSellerVerificationRepository(),
		&verificationMockAuditLogger{},
		nil, // outboxRepo — nil-safe in writeAuditTx
		verificationAllowAllAuthorizer{},
	)
	svc.bankAcctReader = bankAcctReader
	return svc
}

// ---------------------------------------------------------------------------
// markReviewedTx — mock db.Tx for MarkBankAccountReviewed
//
// QueryRow returns an approved (or specified-status) SellerVerification row.
// Exec (Update/audit INSERT) always succeeds; updateCalled tracks the call.
// ---------------------------------------------------------------------------

type markReviewedTx struct {
	status       entity.Status
	reviewedIDs  []uuid.UUID
	sellerID     uuid.UUID
	updateCalled bool
}

func (t *markReviewedTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &markReviewedVerifRow{
		status:      t.status,
		reviewedIDs: t.reviewedIDs,
		sellerID:    t.sellerID,
	}
}

func (t *markReviewedTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	// Not used: bank accounts come from staticBankAccountReader, not the tx.
	return nil, nil
}

func (t *markReviewedTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	t.updateCalled = true
	return pgconn.NewCommandTag("1"), nil
}

func (t *markReviewedTx) Commit(_ context.Context) error   { return nil }
func (t *markReviewedTx) Rollback(_ context.Context) error { return nil }

// markReviewedVerifRow satisfies pgx.Row with the 10-column shape of
// SellerVerificationRepository.GetForUpdate:
//
//	0: id, 1: seller_id, 2: status, 3: submitted_at, 4: reviewed_at,
//	5: reviewed_by, 6: reason, 7: reviewed_bank_account_ids (interface{}),
//	8: created_at, 9: updated_at
type markReviewedVerifRow struct {
	status      entity.Status
	reviewedIDs []uuid.UUID
	sellerID    uuid.UUID
}

func (r *markReviewedVerifRow) Scan(dest ...any) error {
	for i, d := range dest {
		switch i {
		case 0:
			*d.(*uuid.UUID) = uuid.New()
		case 1:
			*d.(*uuid.UUID) = r.sellerID
		case 2:
			*d.(*entity.Status) = r.status
		case 7:
			*d.(*interface{}) = r.reviewedIDs
		}
		// positions 3,4,5,6 are nullable pointers: leave zero (nil)
		// positions 8,9 are time.Time: zero value is fine
	}
	return nil
}

// markReviewedNotFoundTx returns pgx.ErrNoRows from GetForUpdate.
type markReviewedNotFoundTx struct{}

func (t *markReviewedNotFoundTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &pgxErrNoRowsRow{}
}
func (t *markReviewedNotFoundTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t *markReviewedNotFoundTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0"), nil
}
func (t *markReviewedNotFoundTx) Commit(_ context.Context) error   { return nil }
func (t *markReviewedNotFoundTx) Rollback(_ context.Context) error { return nil }

type pgxErrNoRowsRow struct{}

func (r *pgxErrNoRowsRow) Scan(_ ...any) error { return pgx.ErrNoRows }

// ---------------------------------------------------------------------------
// staticBankAccountReader — BankAccountsReader backed by a fixed slice.
// ---------------------------------------------------------------------------

type staticBankAccountReader struct {
	accounts []*bankaccountEntity.BankAccount
}

func (r *staticBankAccountReader) ListActiveAccountsBySeller(
	_ context.Context, _ db.Tx, _ uuid.UUID,
) ([]*bankaccountEntity.BankAccount, error) {
	return r.accounts, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// A — Happy path: approved seller, unreviewed account → Update called.
func TestMarkBankAccountReviewed_HappyPath(t *testing.T) {
	ctx := context.Background()
	sellerID := uuid.New()
	bankID := uuid.New()

	ba := &bankaccountEntity.BankAccount{
		ID:                bankID,
		UserID:            sellerID,
		BankName:          "Bank Central Asia",
		BankCode:          "BCA",
		AccountNumber:     "1234567890",
		AccountHolderName: "Penjual Satu",
		Status:            bankaccountEntity.StatusActive,
	}

	tx := &markReviewedTx{
		status:      entity.StatusApproved,
		reviewedIDs: []uuid.UUID{},
		sellerID:    sellerID,
	}
	svc := buildMarkReviewedService(
		func() db.Tx { return tx },
		&staticBankAccountReader{accounts: []*bankaccountEntity.BankAccount{ba}},
	)

	if err := svc.MarkBankAccountReviewed(ctx, sellerID, bankID, uuid.New()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if !tx.updateCalled {
		t.Error("expected repo.Update to be called after appending bank account ID")
	}
}

// B — Idempotent: bank account already in reviewed set → no Update.
func TestMarkBankAccountReviewed_AlreadyReviewed_NoOp(t *testing.T) {
	ctx := context.Background()
	sellerID := uuid.New()
	bankID := uuid.New()

	tx := &markReviewedTx{
		status:      entity.StatusApproved,
		reviewedIDs: []uuid.UUID{bankID}, // already reviewed
		sellerID:    sellerID,
	}
	svc := buildMarkReviewedService(
		func() db.Tx { return tx },
		&staticBankAccountReader{},
	)

	if err := svc.MarkBankAccountReviewed(ctx, sellerID, bankID, uuid.New()); err != nil {
		t.Fatalf("expected nil (idempotent no-op), got %v", err)
	}
	if tx.updateCalled {
		t.Error("repo.Update must NOT be called for already-reviewed account (idempotent)")
	}
}

// C — Seller not in approved status → ErrMarkReviewedNotApproved.
func TestMarkBankAccountReviewed_SellerNotApproved(t *testing.T) {
	ctx := context.Background()
	sellerID := uuid.New()
	bankID := uuid.New()

	nonApproved := []entity.Status{
		entity.StatusPendingReview,
		entity.StatusNeedsResubmission,
		entity.StatusRejected,
		entity.StatusSuspended,
		entity.StatusRevoked,
		entity.StatusUnderInvestigation,
	}

	for _, status := range nonApproved {
		status := status
		tx := &markReviewedTx{status: status, sellerID: sellerID}
		svc := buildMarkReviewedService(func() db.Tx { return tx }, &staticBankAccountReader{})

		err := svc.MarkBankAccountReviewed(ctx, sellerID, bankID, uuid.New())
		if err == nil {
			t.Errorf("status=%s: expected ErrMarkReviewedNotApproved, got nil", status)
			continue
		}
		var target *ErrMarkReviewedNotApproved
		if !errors.As(err, &target) {
			t.Errorf("status=%s: expected *ErrMarkReviewedNotApproved, got %T: %v", status, err, err)
		}
	}
}

// D — Bank account not active or not belonging to seller →
// ErrBankAccountNotFoundForSeller.
func TestMarkBankAccountReviewed_BankAccountNotFound(t *testing.T) {
	ctx := context.Background()
	sellerID := uuid.New()
	bankID := uuid.New()

	tx := &markReviewedTx{status: entity.StatusApproved, sellerID: sellerID}
	// Empty list — the target account is not among active accounts.
	svc := buildMarkReviewedService(func() db.Tx { return tx }, &staticBankAccountReader{accounts: nil})

	err := svc.MarkBankAccountReviewed(ctx, sellerID, bankID, uuid.New())
	if err == nil {
		t.Fatal("expected ErrBankAccountNotFoundForSeller, got nil")
	}
	var target *ErrBankAccountNotFoundForSeller
	if !errors.As(err, &target) {
		t.Errorf("expected *ErrBankAccountNotFoundForSeller, got %T: %v", err, err)
	}
}

// E — No verification record → wrapped error (nil row).
func TestMarkBankAccountReviewed_NoVerificationRecord(t *testing.T) {
	ctx := context.Background()
	svc := buildMarkReviewedService(
		func() db.Tx { return &markReviewedNotFoundTx{} },
		&staticBankAccountReader{},
	)

	err := svc.MarkBankAccountReviewed(ctx, uuid.New(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error for missing verification record, got nil")
	}
}

// F — bankAcctReader not wired → configuration error before entering tx.
func TestMarkBankAccountReviewed_ReaderNotWired(t *testing.T) {
	ctx := context.Background()
	svc := buildMarkReviewedService(func() db.Tx { return nil }, nil)
	svc.bankAcctReader = nil // explicit nil

	err := svc.MarkBankAccountReviewed(ctx, uuid.New(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected configuration error when bankAcctReader is nil, got nil")
	}
}

// ---------------------------------------------------------------------------
// G — Entity unit: AppendReviewedBankAccount deduplication
// ---------------------------------------------------------------------------

func TestAppendReviewedBankAccount_Dedup(t *testing.T) {
	v := entity.NewSellerVerification(uuid.New())
	id1 := uuid.New()
	id2 := uuid.New()

	// Double-append id1 → only one entry.
	v.AppendReviewedBankAccount(id1)
	v.AppendReviewedBankAccount(id1)
	if len(v.ReviewedBankAccountIDs) != 1 {
		t.Errorf("want 1 entry after dup append, got %d", len(v.ReviewedBankAccountIDs))
	}

	// Append distinct id2 → two entries.
	v.AppendReviewedBankAccount(id2)
	if len(v.ReviewedBankAccountIDs) != 2 {
		t.Errorf("want 2 entries after appending distinct id, got %d", len(v.ReviewedBankAccountIDs))
	}

	if !v.HasReviewedBankAccount(id1) {
		t.Error("id1 should be in ReviewedBankAccountIDs")
	}
	if !v.HasReviewedBankAccount(id2) {
		t.Error("id2 should be in ReviewedBankAccountIDs")
	}
	if v.HasReviewedBankAccount(uuid.New()) {
		t.Error("random UUID should NOT be in ReviewedBankAccountIDs")
	}
}


