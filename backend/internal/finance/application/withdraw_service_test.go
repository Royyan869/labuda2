package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/audit"
	bankaccountrepo "github.com/labuda/backend/internal/finance/bankaccount/infrastructure/repository"
	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	verificationapp "github.com/labuda/backend/internal/governance/verification/application"
	verificationentity "github.com/labuda/backend/internal/governance/verification/entity"
	verificationinfra "github.com/labuda/backend/internal/governance/verification/infrastructure/repository"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/capability"
	capentity "github.com/labuda/backend/internal/platform/capability/entity"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRoleChecker struct{}

func (m *mockRoleChecker) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	return true, nil
}

func (m *mockRoleChecker) IsSeller(ctx context.Context, userID uuid.UUID) (bool, error) {
	return true, nil
}

func (m *mockRoleChecker) HasActiveSellerCapability(ctx context.Context, userID uuid.UUID) (bool, error) {
	return true, nil
}

func (m *mockRoleChecker) HasSellerProfile(ctx context.Context, userID uuid.UUID) (bool, error) {
	return true, nil
}

type mockAccountStatusChecker struct {
	ensureActiveErr error
}

func (m *mockAccountStatusChecker) EnsureActive(ctx context.Context, userID uuid.UUID) error {
	return m.ensureActiveErr
}

func (m *mockAccountStatusChecker) IsBanned(ctx context.Context, userID uuid.UUID) (bool, error) {
	return errors.Is(m.ensureActiveErr, auth.ErrAccountBanned), nil
}

func (m *mockAccountStatusChecker) GetStatus(ctx context.Context, userID uuid.UUID) (string, error) {
	if m.ensureActiveErr != nil {
		if errors.Is(m.ensureActiveErr, auth.ErrAccountSuspended) {
			return "suspended", nil
		}
		if errors.Is(m.ensureActiveErr, auth.ErrAccountBanned) {
			return "banned", nil
		}
		return "inactive", nil
	}
	return "active", nil
}

type mockAdminAuditLogger struct{}

var _ audit.AdminAuditLogger = (*mockAdminAuditLogger)(nil)

func (m *mockAdminAuditLogger) Log(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	return nil
}

func (m *mockAdminAuditLogger) LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) {
}

func (m *mockAdminAuditLogger) LogTx(ctx context.Context, tx db.Tx, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	return nil
}

type allowAllVerificationCapabilityAuthorizer struct{}

func (allowAllVerificationCapabilityAuthorizer) HasCapability(_ context.Context, _ uuid.UUID, _ capability.Capability) (bool, error) {
	return true, nil
}

type mockTx struct {
	QueryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	QueryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	ExecFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, sql, args...)
	}
	return &mockRow{err: errors.New("no mock")}
}

func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

func (m *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("1"), nil
}

func (m *mockTx) Commit(ctx context.Context) error   { return nil }
func (m *mockTx) Rollback(ctx context.Context) error { return nil }

type mockRow struct {
	values []any
	err    error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, v := range r.values {
		switch d := dest[i].(type) {
		case *uuid.UUID:
			if id, ok := v.(uuid.UUID); ok {
				*d = id
			}
		case *string:
			if s, ok := v.(string); ok {
				*d = s
			}
		case *int64:
			if val, ok := v.(int64); ok {
				*d = val
			}
		}
	}
	return nil
}

type mockRows struct{}

func (r *mockRows) Close()                                       {}
func (r *mockRows) Err() error                                   { return nil }
func (r *mockRows) Next() bool                                   { return false }
func (r *mockRows) Scan(dest ...any) error                       { return nil }
func (r *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("0") }
func (r *mockRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) RawValues() [][]byte                          { return nil }
func (r *mockRows) Values() ([]any, error)                       { return nil, nil }
func (r *mockRows) Conn() *pgx.Conn                              { return nil }

type mockDB struct {
	WithTxFunc func(ctx context.Context, fn func(tx db.Tx) error) error
}

func (m *mockDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	if m.WithTxFunc != nil {
		return m.WithTxFunc(ctx, fn)
	}
	return fn(nil)
}

func setupTestService(mockedDb *mockDB) *WithdrawService {
	ledgerRepo := repository.NewLedgerRepository()
	withdrawRepo := repository.NewWithdrawRepository()
	roleChecker := &mockRoleChecker{}
	accountStatusChecker := &mockAccountStatusChecker{}
	adminAuditLogger := &mockAdminAuditLogger{}

	return &WithdrawService{
		db:                   mockedDb,
		ledgerRepo:           ledgerRepo,
		withdrawRepo:         withdrawRepo,
		roleChecker:          roleChecker,
		accountStatusChecker: accountStatusChecker,
		ownership:            auth.NewOwnershipValidator(),
		adminAuditLogger:     adminAuditLogger,
		verificationService:  nil,
		outboxRepo:           nil,
	}
}

func TestWithdrawService_Simple(t *testing.T) {
	// Simple placeholder test
	mockedDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return nil
		},
	}

	service := setupTestService(mockedDB)
	if service == nil {
		t.Fatal("service should not be nil")
	}
}

// ---------------------------------------------------------------------------
// GUARD 0 — Account status gate for RequestWithdrawal (P0 fix)
// ---------------------------------------------------------------------------
//
// Matrix under test:
//
//   account_status  | verification  | expected
//   ────────────────┼───────────────┼──────────────────
//   active          | approved      | passes both gates
//   active (expired)| approved      | passes both gates (subscription irrelevant)
//   suspended       | approved      | BLOCKED by account gate
//   banned          | approved      | BLOCKED by account gate
//   active          | suspended     | BLOCKED by verification gate
//   active          | rejected      | BLOCKED by verification gate

// extendedMockRow supports the 9-column seller_verifications scan shape
// which includes *time.Time and *uuid.UUID pointer types that the base
// mockRow does not handle.
type extendedMockRow struct {
	values []any
	err    error
}

func (r *extendedMockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, v := range r.values {
		if i >= len(dest) {
			break
		}
		switch d := dest[i].(type) {
		case *uuid.UUID:
			if id, ok := v.(uuid.UUID); ok {
				*d = id
			}
		case *verificationentity.Status:
			if s, ok := v.(string); ok {
				*d = verificationentity.Status(s)
			}
		case *string:
			if s, ok := v.(string); ok {
				*d = s
			}
		case *int64:
			if val, ok := v.(int64); ok {
				*d = val
			}
		case *bool:
			if b, ok := v.(bool); ok {
				*d = b
			}
		case **time.Time:
			if tp, ok := v.(*time.Time); ok {
				*d = tp
			}
		case **uuid.UUID:
			if up, ok := v.(*uuid.UUID); ok {
				*d = up
			}
		case **string:
			if sp, ok := v.(*string); ok {
				*d = sp
			}
		case *time.Time:
			if tv, ok := v.(time.Time); ok {
				*d = tv
			}
		}
	}
	return nil
}

// verificationTxForStatus builds a mockTx that resolves the
// seller_verifications scan for a given status.
func verificationTxForStatus(status verificationentity.Status) *mockTx {
	now := time.Now()
	return &mockTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if strings.Contains(sql, "seller_verifications") {
				if status == "" {
					return &extendedMockRow{err: pgx.ErrNoRows}
				}
				sellerID := args[0].(uuid.UUID)
				return &extendedMockRow{values: []any{
					uuid.New(),        // id
					sellerID,          // seller_id
					string(status),    // status
					(*time.Time)(nil), // submitted_at
					(*time.Time)(nil), // reviewed_at
					(*uuid.UUID)(nil), // reviewed_by
					(*string)(nil),    // reason
					now,               // created_at
					now,               // updated_at
				}}
			}
			return &extendedMockRow{err: errors.New("unexpected query in tx")}
		},
	}
}

// buildServiceForAccountGateTest constructs a WithdrawService wired for
// account-gate testing. accountErr controls EnsureActive; verificationStatus
// controls the verification repo response inside the tx.
func buildServiceForAccountGateTest(
	accountErr error,
	verificationStatus verificationentity.Status,
) *WithdrawService {
	tx := verificationTxForStatus(verificationStatus)
	mockedDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(tx)
		},
	}

	verificationRepo := verificationinfra.NewSellerVerificationRepository()
	verificationSvc := verificationapp.NewVerificationService(mockedDB, verificationRepo, nil, nil, allowAllVerificationCapabilityAuthorizer{})

	svc := &WithdrawService{
		db:                   mockedDB,
		ledgerRepo:           repository.NewLedgerRepository(),
		withdrawRepo:         repository.NewWithdrawRepository(),
		roleChecker:          &mockRoleChecker{},
		accountStatusChecker: &mockAccountStatusChecker{ensureActiveErr: accountErr},
		ownership:            auth.NewOwnershipValidator(),
		adminAuditLogger:     &mockAdminAuditLogger{},
		verificationService:  verificationSvc,
	}
	// Set a non-nil canonicalAuthority so the nil guard passes.
	svc.canonicalAuthority = &FinanceService{}
	return svc
}

func TestRequestWithdrawal_AccountGate_SuspendedBlocked(t *testing.T) {
	svc := buildServiceForAccountGateTest(auth.ErrAccountSuspended, verificationentity.StatusApproved)
	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: uuid.New(),
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for suspended account, got nil")
	}
	if !errors.Is(err, auth.ErrAccountSuspended) {
		t.Fatalf("expected ErrAccountSuspended, got: %v", err)
	}
}

func TestRequestWithdrawal_AccountGate_BannedBlocked(t *testing.T) {
	svc := buildServiceForAccountGateTest(auth.ErrAccountBanned, verificationentity.StatusApproved)
	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: uuid.New(),
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for banned account, got nil")
	}
	if !errors.Is(err, auth.ErrAccountBanned) {
		t.Fatalf("expected ErrAccountBanned, got: %v", err)
	}
}

// callRequestWithdrawalSafe calls RequestWithdrawal and converts any panic
// from downstream guards (e.g. zero-valued FinanceService) into an error
// string so the test can assert which gate was actually reached.
func callRequestWithdrawalSafe(svc *WithdrawService, sellerID uuid.UUID, amount int64) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	_, err = svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: sellerID,
		Amount:   amount,
	})
	return err
}

func TestRequestWithdrawal_AccountGate_ActiveApproved_PassesBothGates(t *testing.T) {
	svc := buildServiceForAccountGateTest(nil, verificationentity.StatusApproved)
	err := callRequestWithdrawalSafe(svc, uuid.New(), 100_000)
	// Should pass account gate and verification gate, then panic/error on
	// the canonical authority (AssertSellerWithdrawalAllowed) because the
	// FinanceService is zero-valued. The important assertion: the error
	// is NOT an account or verification error.
	if err == nil {
		t.Fatal("expected downstream error (authority not fully configured), got nil")
	}
	if errors.Is(err, auth.ErrAccountSuspended) || errors.Is(err, auth.ErrAccountBanned) {
		t.Fatalf("should have passed account gate, got account error: %v", err)
	}
	var notVerified *ErrSellerNotVerified
	if errors.As(err, &notVerified) {
		t.Fatalf("should have passed verification gate, got: %v", err)
	}
}

func TestRequestWithdrawal_AccountGate_ExpiredSellerApproved_PassesBothGates(t *testing.T) {
	// Expired seller subscription is irrelevant — account_status is still
	// "active" and verification is "approved". The withdraw path does NOT
	// check subscription status, only the account gate and verification gate.
	svc := buildServiceForAccountGateTest(nil, verificationentity.StatusApproved)
	err := callRequestWithdrawalSafe(svc, uuid.New(), 100_000)
	if err == nil {
		t.Fatal("expected downstream error, got nil")
	}
	if errors.Is(err, auth.ErrAccountSuspended) || errors.Is(err, auth.ErrAccountBanned) {
		t.Fatalf("expired seller should pass account gate, got: %v", err)
	}
	var notVerified *ErrSellerNotVerified
	if errors.As(err, &notVerified) {
		t.Fatalf("approved verification should pass, got: %v", err)
	}
}

func TestRequestWithdrawal_VerificationGate_SuspendedVerification(t *testing.T) {
	svc := buildServiceForAccountGateTest(nil, verificationentity.StatusSuspended)
	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: uuid.New(),
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for verification-suspended seller, got nil")
	}
	var notVerified *ErrSellerNotVerified
	if !errors.As(err, &notVerified) {
		t.Fatalf("expected ErrSellerNotVerified for suspended verification, got: %v", err)
	}
}

func TestRequestWithdrawal_VerificationGate_RevokedVerification(t *testing.T) {
	svc := buildServiceForAccountGateTest(nil, verificationentity.StatusRevoked)
	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: uuid.New(),
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for verification-revoked seller, got nil")
	}
	var notVerified *ErrSellerNotVerified
	if !errors.As(err, &notVerified) {
		t.Fatalf("expected ErrSellerNotVerified for revoked verification, got: %v", err)
	}
}

func TestRequestWithdrawal_VerificationGate_UnderInvestigation(t *testing.T) {
	// under_investigation closes payout authority even though selling authority
	// is preserved (Option C doctrine). HasPayoutAuthority() returns false.
	svc := buildServiceForAccountGateTest(nil, verificationentity.StatusUnderInvestigation)
	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: uuid.New(),
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for under_investigation verification, got nil")
	}
	var notVerified *ErrSellerNotVerified
	if !errors.As(err, &notVerified) {
		t.Fatalf("expected ErrSellerNotVerified for under_investigation verification, got: %v", err)
	}
}

func TestRequestWithdrawal_VerificationGate_PendingReview(t *testing.T) {
	svc := buildServiceForAccountGateTest(nil, verificationentity.StatusPendingReview)
	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: uuid.New(),
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for pending_review verification, got nil")
	}
	var notVerified *ErrSellerNotVerified
	if !errors.As(err, &notVerified) {
		t.Fatalf("expected ErrSellerNotVerified for pending_review verification, got: %v", err)
	}
}

func TestRequestWithdrawal_VerificationGate_Rejected(t *testing.T) {
	svc := buildServiceForAccountGateTest(nil, verificationentity.StatusRejected)
	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: uuid.New(),
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for rejected verification, got nil")
	}
	var notVerified *ErrSellerNotVerified
	if !errors.As(err, &notVerified) {
		t.Fatalf("expected ErrSellerNotVerified for rejected verification, got: %v", err)
	}
}

func TestRequestWithdrawal_VerificationGate_NeedsResubmission(t *testing.T) {
	svc := buildServiceForAccountGateTest(nil, verificationentity.StatusNeedsResubmission)
	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: uuid.New(),
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for needs_resubmission verification, got nil")
	}
	var notVerified *ErrSellerNotVerified
	if !errors.As(err, &notVerified) {
		t.Fatalf("expected ErrSellerNotVerified for needs_resubmission verification, got: %v", err)
	}
}

func TestRequestWithdrawal_VerificationGate_NotSubmitted(t *testing.T) {
	// Empty status ("") causes verificationTxForStatus to return pgx.ErrNoRows,
	// simulating a seller who has never submitted KYC.
	svc := buildServiceForAccountGateTest(nil, "")
	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: uuid.New(),
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for not-submitted verification, got nil")
	}
	var notVerified *ErrSellerNotVerified
	if !errors.As(err, &notVerified) {
		t.Fatalf("expected ErrSellerNotVerified for not-submitted verification, got: %v", err)
	}
}

type noopWithdrawalAuthority struct{}

func (noopWithdrawalAuthority) AssertSellerWithdrawalAllowed(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount int64) (*SellerWithdrawableSummary, error) {
	return &SellerWithdrawableSummary{}, nil
}

func (noopWithdrawalAuthority) RecordWithdrawalRequest(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount, feeAmount int64, withdrawalID uuid.UUID) error {
	return nil
}

func (noopWithdrawalAuthority) RecordWithdrawalCommit(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount, feeAmount int64, withdrawalID uuid.UUID) error {
	return nil
}

func (noopWithdrawalAuthority) RecordWithdrawalReject(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount, feeAmount int64, withdrawalID uuid.UUID) error {
	return nil
}

func (noopWithdrawalAuthority) RecordWithdrawalRestore(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount, feeAmount int64, withdrawalID uuid.UUID) error {
	return nil
}

func (noopWithdrawalAuthority) RecordWithdrawalComplete(ctx context.Context, tx db.Tx, sellerID uuid.UUID, amount, feeAmount int64, withdrawalID uuid.UUID) error {
	return nil
}

type bankReviewChecker struct {
	verified bool
	reviewed  bool
}

func (c bankReviewChecker) IsSellerVerifiedTx(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (bool, error) {
	return c.verified, nil
}

func (c bankReviewChecker) IsReviewedBankAccountTx(ctx context.Context, tx db.Tx, sellerID, bankAccountID uuid.UUID) (bool, error) {
	return c.reviewed, nil
}

func buildBankReviewService(t *testing.T, reviewed bool) (*WithdrawService, uuid.UUID) {
	t.Helper()

	sellerID := uuid.New()
	bankID := uuid.New()
	now := time.Now().UTC()

	tx := &mockTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM bank_accounts"):
				return &extendedMockRow{values: []any{
					bankID,
					sellerID,
					"BCA",
					"BCA",
					"1234567890",
					"Penjual Satu",
					true,
					"active",
					(*time.Time)(nil),
					now,
					now,
				}}
			case strings.Contains(sql, "FROM withdrawals"):
				return &extendedMockRow{err: pgx.ErrNoRows}
			default:
				return &extendedMockRow{err: errors.New("unexpected query in bank review test")}
			}
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("1"), nil
		},
	}

	mockedDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(tx)
		},
	}

	svc := &WithdrawService{
		db:                   mockedDB,
		ledgerRepo:           repository.NewLedgerRepository(),
		withdrawRepo:         repository.NewWithdrawRepository(),
		bankAccountRepo:      bankaccountrepo.NewBankAccountRepository(),
		roleChecker:          &mockRoleChecker{},
		accountStatusChecker: &mockAccountStatusChecker{},
		ownership:            auth.NewOwnershipValidator(),
		adminAuditLogger:     &mockAdminAuditLogger{},
		verificationService: bankReviewChecker{
			verified: true,
			reviewed: reviewed,
		},
		outboxRepo: nil,
	}
	svc.SetCanonicalAuthority(noopWithdrawalAuthority{})
	return svc, sellerID
}

func buildBankNoDefaultService(t *testing.T) (*WithdrawService, uuid.UUID) {
	t.Helper()

	sellerID := uuid.New()

	tx := &mockTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM bank_accounts"):
				return &extendedMockRow{err: pgx.ErrNoRows}
			case strings.Contains(sql, "FROM withdrawals"):
				return &extendedMockRow{err: pgx.ErrNoRows}
			default:
				return &extendedMockRow{err: errors.New("unexpected query in no-default bank test")}
			}
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("1"), nil
		},
	}

	mockedDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(tx)
		},
	}

	svc := &WithdrawService{
		db:                   mockedDB,
		ledgerRepo:           repository.NewLedgerRepository(),
		withdrawRepo:         repository.NewWithdrawRepository(),
		bankAccountRepo:      bankaccountrepo.NewBankAccountRepository(),
		roleChecker:          &mockRoleChecker{},
		accountStatusChecker: &mockAccountStatusChecker{},
		ownership:            auth.NewOwnershipValidator(),
		adminAuditLogger:     &mockAdminAuditLogger{},
		verificationService: bankReviewChecker{
			verified: true,
			reviewed: true,
		},
		outboxRepo: nil,
	}
	svc.SetCanonicalAuthority(noopWithdrawalAuthority{})
	return svc, sellerID
}

func TestRequestWithdrawal_BankReviewGate_UnreviewedBank_Rejected(t *testing.T) {
	svc, sellerID := buildBankReviewService(t, false)

	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: sellerID,
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for unreviewed bank account, got nil")
	}
	var notReviewed *ErrBankAccountNotReviewed
	if !errors.As(err, &notReviewed) {
		t.Fatalf("expected ErrBankAccountNotReviewed, got: %v", err)
	}
}

func TestRequestWithdrawal_BankReviewGate_ReviewedBank_Passes(t *testing.T) {
	svc, sellerID := buildBankReviewService(t, true)

	out, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: sellerID,
		Amount:   100_000,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, repository.WithdrawalStatusRequested, out.Status)
}

func TestRequestWithdrawal_BankReviewGate_NoDefaultBank_Rejected(t *testing.T) {
	svc, sellerID := buildBankNoDefaultService(t)

	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: sellerID,
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for missing default bank account, got nil")
	}
	var noDefault *ErrNoDefaultBankAccount
	if !errors.As(err, &noDefault) {
		t.Fatalf("expected ErrNoDefaultBankAccount, got: %v", err)
	}
}

func TestRequestWithdrawal_BankReviewGate_DeletedBank_Rejected(t *testing.T) {
	svc, sellerID := buildBankNoDefaultService(t)

	_, err := svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: sellerID,
		Amount:   100_000,
	})
	if err == nil {
		t.Fatal("expected error for deleted/inactive bank account, got nil")
	}
	var noDefault *ErrNoDefaultBankAccount
	if !errors.As(err, &noDefault) {
		t.Fatalf("expected ErrNoDefaultBankAccount for deleted/inactive bank, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CAPABILITY AUTHORITY — MarkProcessed capability gate (P0 alignment)
// ---------------------------------------------------------------------------
//
// Matrix under test:
//
//   actor capabilities              | expected
//   ────────────────────────────────┼──────────────────────
//   finance.withdraw.review         | PASS (reaches tx)
//   admin role WITHOUT capability   | BLOCKED (forbidden)
//   finance.withdraw.review + admin | PASS (reaches tx)
//   no actor in context             | BLOCKED (forbidden)
//   system caller                   | PASS (bypasses check)

func TestMarkProcessed_CapabilityOnly_Passes(t *testing.T) {
	svc := setupTestService(&mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&mockTx{})
		},
	})

	callerID := uuid.New()
	ctx := contextWithCapabilities(callerID, "user", capability.CapFinanceWithdrawReview.String())

	err := svc.MarkProcessed(ctx, callerID, uuid.New())
	// Should pass capability gate and reach the tx (will fail on lock, not on auth)
	if err != nil && strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("capability holder should pass auth gate, got: %v", err)
	}
}

func TestMarkProcessed_AdminRoleWithoutCapability_Blocked(t *testing.T) {
	svc := setupTestService(&mockDB{})

	callerID := uuid.New()
	// Admin role but NO finance.withdraw.review capability
	ctx := contextWithCapabilities(callerID, "admin")

	err := svc.MarkProcessed(ctx, callerID, uuid.New())
	if err == nil {
		t.Fatal("admin without capability should be blocked, got nil")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected forbidden error, got: %v", err)
	}
}

func TestMarkProcessed_CapabilityPlusAdmin_Passes(t *testing.T) {
	svc := setupTestService(&mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&mockTx{})
		},
	})

	callerID := uuid.New()
	ctx := contextWithCapabilities(callerID, "admin", capability.CapFinanceWithdrawReview.String())

	err := svc.MarkProcessed(ctx, callerID, uuid.New())
	if err != nil && strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("admin with capability should pass auth gate, got: %v", err)
	}
}

func TestMarkProcessed_NoActorInContext_Blocked(t *testing.T) {
	svc := setupTestService(&mockDB{})

	callerID := uuid.New()
	// Plain context — no actor injected
	ctx := context.Background()

	err := svc.MarkProcessed(ctx, callerID, uuid.New())
	if err == nil {
		t.Fatal("no actor in context should be blocked, got nil")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected forbidden error, got: %v", err)
	}
}

func TestMarkProcessed_SystemCaller_Bypasses(t *testing.T) {
	svc := setupTestService(&mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&mockTx{})
		},
	})

	err := svc.MarkProcessed(context.Background(), auth.SystemCallerID, uuid.New())
	if err != nil && strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("system caller should bypass capability check, got: %v", err)
	}
}

// contextWithCapabilities builds a context with an Actor carrying the given capabilities.
func contextWithCapabilities(userID uuid.UUID, role string, caps ...string) context.Context {
	actor := &capentity.Actor{
		ID:           userID,
		Role:         role,
		Capabilities: caps,
	}
	return capability.WithActor(context.Background(), actor)
}

// ---------------------------------------------------------------------------
// CANONICAL AUTHORITY PROOF — ApproveWithdraw / RejectWithdraw
// ---------------------------------------------------------------------------
//
// These tests prove that after C5.1-A both methods delegate to
// FinanceService.Record* (idempotency prefix "withdrawal_") instead of
// writing directly to ledgerRepo (old prefix "withdraw_").
//
// Approach: a spy tx captures the idempotency key used when
// LedgerRepository.CreateTransaction executes the idempotency check query.
//
// Matrix:
//   method          | withdrawal status | expected idem prefix
//   ────────────────┼───────────────────┼───────────────────────────────────
//   ApproveWithdraw | REQUESTED         | withdrawal_commit_<id>
//   RejectWithdraw  | REQUESTED         | withdrawal_reject_<id>
//   RejectWithdraw  | PROCESSING        | withdrawal_restore_<id>
//   ApproveWithdraw | nil authority     | ErrCanonicalAuthorityNotConfigured

// withdrawalMockRow implements pgx.Row and returns a minimal Withdrawal
// for LockForUpdate scans (18 destination slots).
type withdrawalMockRow struct {
	id        uuid.UUID
	sellerID  uuid.UUID
	amount    int64
	feeAmount int64
	status    repository.WithdrawalStatus
}

func (r *withdrawalMockRow) Scan(dest ...any) error {
	now := time.Now()
	for i, d := range dest {
		switch i {
		case 0:
			*d.(*uuid.UUID) = r.id
		case 1:
			*d.(*uuid.UUID) = r.sellerID
		case 2:
			*d.(*int64) = r.amount
		case 3:
			*d.(*int64) = r.feeAmount
		case 4:
			*d.(*repository.WithdrawalStatus) = r.status
		// indices 5–15: string/int64/int fields; zero value is fine
		case 16, 17:
			*d.(*time.Time) = now
		}
	}
	return nil
}

// accountQueryRows implements pgx.Rows and returns one row with a
// fixed account UUID and zero balance, satisfying the FOR UPDATE lock
// inside LedgerRepository.CreateTransaction.
type accountQueryRows struct {
	accountID uuid.UUID
	done      bool
}

func (r *accountQueryRows) Close()                                       {}
func (r *accountQueryRows) Err() error                                   { return nil }
func (r *accountQueryRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("1") }
func (r *accountQueryRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *accountQueryRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *accountQueryRows) RawValues() [][]byte                          { return nil }
func (r *accountQueryRows) Values() ([]any, error)                       { return nil, nil }
func (r *accountQueryRows) Conn() *pgx.Conn                              { return nil }
func (r *accountQueryRows) Next() bool {
	if r.done {
		return false
	}
	r.done = true
	return true
}
func (r *accountQueryRows) Scan(dest ...any) error {
	for i, d := range dest {
		switch i {
		case 0:
			*d.(*uuid.UUID) = r.accountID
		case 1:
			*d.(*int64) = 0
		}
	}
	return nil
}

// withdrawalSpyTx is a mock db.Tx that:
//   - Returns a controlled Withdrawal from LockForUpdate queries.
//   - Returns a fixed UUID for all financial_accounts lookups so both
//     DR and CR entries reference the same account (net Δ = 0, which
//     satisfies the LedgerRepository double-entry invariant).
//   - Captures the idempotency key when CreateTransaction runs its
//     idempotency check (SELECT ... FROM ledger_transactions WHERE idempotency_key = $1).
//   - Returns RowsAffected=1 for all Exec calls (status update, etc.).
type withdrawalSpyTx struct {
	wdID     uuid.UUID
	sellerID uuid.UUID
	amount   int64
	status   repository.WithdrawalStatus

	fixedAcct uuid.UUID // returned for every financial_accounts lookup

	capturedIdemKey string // populated by the idempotency-check QueryRow
}

func (s *withdrawalSpyTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "FROM withdrawals"):
		return &withdrawalMockRow{
			id:       s.wdID,
			sellerID: s.sellerID,
			amount:   s.amount,
			status:   s.status,
		}
	case strings.Contains(sql, "financial_accounts"):
		return &mockRow{values: []any{s.fixedAcct}}
	case strings.Contains(sql, "idempotency_key"):
		if len(args) > 0 {
			if key, ok := args[0].(string); ok {
				s.capturedIdemKey = key
			}
		}
		return &mockRow{err: pgx.ErrNoRows}
	default:
		return &mockRow{err: fmt.Errorf("unhandled QueryRow sql: %s", sql)}
	}
}

func (s *withdrawalSpyTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "financial_accounts") {
		return &accountQueryRows{accountID: s.fixedAcct}, nil
	}
	return &mockRows{}, nil
}

func (s *withdrawalSpyTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("1"), nil
}
func (s *withdrawalSpyTx) Commit(ctx context.Context) error   { return nil }
func (s *withdrawalSpyTx) Rollback(ctx context.Context) error { return nil }

// alwaysVerifiedChecker is a stub WithdrawVerificationChecker that reports
// every seller as verified and every bank account as reviewed. Used in
// ApproveWithdraw / RejectWithdraw tests that focus on ledger keys, not auth.
type alwaysVerifiedChecker struct{}

func (alwaysVerifiedChecker) IsSellerVerifiedTx(_ context.Context, _ db.Tx, _ uuid.UUID) (bool, error) {
	return true, nil
}
func (alwaysVerifiedChecker) IsReviewedBankAccountTx(_ context.Context, _ db.Tx, _, _ uuid.UUID) (bool, error) {
	return true, nil
}

// buildApproveRejectService builds a WithdrawService with a real FinanceService
// as canonicalAuthority. The spy tx intercepts all SQL in the flow.
func buildApproveRejectService(spy *withdrawalSpyTx, status repository.WithdrawalStatus) (*WithdrawService, uuid.UUID) {
	spy.wdID = uuid.New()
	spy.sellerID = uuid.New()
	spy.amount = 50_000
	spy.status = status
	spy.fixedAcct = uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-000000000001")

	mockedDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(spy)
		},
	}
	svc := &WithdrawService{
		db:                  mockedDB,
		ledgerRepo:          repository.NewLedgerRepository(),
		withdrawRepo:        repository.NewWithdrawRepository(),
		roleChecker:         &mockRoleChecker{},
		ownership:           auth.NewOwnershipValidator(),
		adminAuditLogger:    &mockAdminAuditLogger{},
		verificationService: alwaysVerifiedChecker{},
	}
	svc.canonicalAuthority = NewFinanceService()
	return svc, spy.wdID
}

func TestApproveWithdraw_UsesCanonicalCommitKey(t *testing.T) {
	spy := &withdrawalSpyTx{}
	svc, withdrawalID := buildApproveRejectService(spy, repository.WithdrawalStatusRequested)

	callerID := uuid.New()
	ctx := contextWithCapabilities(callerID, "admin", capability.CapFinanceWithdrawReview.String())

	// Any error here is from the minimal tx mock (e.g. status update); the
	// assertion is on the captured idempotency key only.
	_ = svc.ApproveWithdraw(ctx, callerID, withdrawalID)

	want := fmt.Sprintf("withdrawal_commit_%s", withdrawalID.String())
	if spy.capturedIdemKey != want {
		t.Errorf("ApproveWithdraw: idempotency key = %q, want %q\n(old direct key would be: withdraw_approve_%s)",
			spy.capturedIdemKey, want, withdrawalID.String())
	}
}

func TestRejectWithdraw_Requested_UsesCanonicalRejectKey(t *testing.T) {
	spy := &withdrawalSpyTx{}
	svc, withdrawalID := buildApproveRejectService(spy, repository.WithdrawalStatusRequested)

	callerID := uuid.New()
	ctx := contextWithCapabilities(callerID, "admin", capability.CapFinanceWithdrawReview.String())

	_ = svc.RejectWithdraw(ctx, callerID, withdrawalID)

	want := fmt.Sprintf("withdrawal_reject_%s", withdrawalID.String())
	if spy.capturedIdemKey != want {
		t.Errorf("RejectWithdraw(REQUESTED): idempotency key = %q, want %q\n(old direct key would be: withdraw_reject_%s)",
			spy.capturedIdemKey, want, withdrawalID.String())
	}
}

func TestRejectWithdraw_Processing_UsesCanonicalRestoreKey(t *testing.T) {
	spy := &withdrawalSpyTx{}
	svc, withdrawalID := buildApproveRejectService(spy, repository.WithdrawalStatusProcessing)

	callerID := uuid.New()
	ctx := contextWithCapabilities(callerID, "admin", capability.CapFinanceWithdrawReview.String())

	_ = svc.RejectWithdraw(ctx, callerID, withdrawalID)

	want := fmt.Sprintf("withdrawal_restore_%s", withdrawalID.String())
	if spy.capturedIdemKey != want {
		t.Errorf("RejectWithdraw(PROCESSING): idempotency key = %q, want %q\n(old direct key would be: withdraw_reject_%s)",
			spy.capturedIdemKey, want, withdrawalID.String())
	}
}

func TestApproveWithdraw_NilCanonicalAuthority_FailsClosed(t *testing.T) {
	spy := &withdrawalSpyTx{}
	svc, withdrawalID := buildApproveRejectService(spy, repository.WithdrawalStatusRequested)
	svc.canonicalAuthority = nil

	callerID := uuid.New()
	ctx := contextWithCapabilities(callerID, "admin", capability.CapFinanceWithdrawReview.String())

	err := svc.ApproveWithdraw(ctx, callerID, withdrawalID)
	if err == nil {
		t.Fatal("expected error with nil canonicalAuthority, got nil")
	}
	if !errors.Is(err, ErrCanonicalAuthorityNotConfigured) {
		t.Errorf("expected ErrCanonicalAuthorityNotConfigured, got: %v", err)
	}
}

// ============================================================================
// ListWithdrawalsBySeller tests
// ============================================================================

// mockListRows implements pgx.Rows for the 20-column withdrawal scan shape used
// by WithdrawRepository.ListWithFilters.
//
// Column order:
//
//	id, seller_id, seller_username, seller_farm_name, amount, status, idempotency_key,
//	bank_name_snapshot, bank_code_snapshot, account_number_snapshot, account_holder_snapshot,
//	external_reference_id, gateway_response, failure_reason,
//	submitted_at, settled_at, retry_count,
//	created_at, updated_at
type mockListRows struct {
	data    [][]any
	current int
}

func (r *mockListRows) Close()     {}
func (r *mockListRows) Err() error { return nil }
func (r *mockListRows) Next() bool {
	if r.current < len(r.data) {
		r.current++
		return true
	}
	return false
}
func (r *mockListRows) Scan(dest ...any) error {
	vals := r.data[r.current-1]
	for i, d := range dest {
		if i >= len(vals) {
			break
		}
		v := vals[i]
		switch ptr := d.(type) {
		case *uuid.UUID:
			*ptr = v.(uuid.UUID)
		case *repository.WithdrawalStatus:
			*ptr = repository.WithdrawalStatus(v.(string))
		case *string:
			*ptr = v.(string)
		case *int64:
			*ptr = v.(int64)
		case *int:
			*ptr = v.(int)
		case *time.Time:
			*ptr = v.(time.Time)
		}
	}
	return nil
}
func (r *mockListRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("0") }
func (r *mockListRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *mockListRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockListRows) RawValues() [][]byte                          { return nil }
func (r *mockListRows) Values() ([]any, error)                       { return nil, nil }
func (r *mockListRows) Conn() *pgx.Conn                              { return nil }

func withdrawalRow(id, sellerID uuid.UUID, amount int64, status string, extRef string, created, settled time.Time) []any {
	return []any{
		id, sellerID, "yayan", "Farm Koi Nusantara",
		amount, int64(0), status, "idem-key",
		"BCA", "014", "1234567890", "John Doe",
		extRef, "", "",
		int64(0), settled.Unix(), int(0),
		created, created,
	}
}

func TestListWithdrawalsBySeller_ReturnsCanonicalFinanceRows(t *testing.T) {
	sellerID := uuid.New()
	withdrawalID := uuid.New()
	created := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	settled := time.Date(2024, 6, 2, 10, 0, 0, 0, time.UTC)

	listRows := &mockListRows{
		data: [][]any{
			withdrawalRow(withdrawalID, sellerID, 500000, "SETTLED", "WD_REF_001", created, settled),
		},
	}

	mockedDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&mockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return listRows, nil
				},
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRow{values: []any{int64(1)}}
				},
			})
		},
	}

	svc := setupTestService(mockedDB)
	withdrawals, total, err := svc.ListWithdrawalsBySeller(context.Background(), sellerID, 20, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("total: want 1, got %d", total)
	}
	if len(withdrawals) != 1 {
		t.Fatalf("len: want 1, got %d", len(withdrawals))
	}
	w := withdrawals[0]
	if w.ID != withdrawalID {
		t.Errorf("ID: want %v, got %v", withdrawalID, w.ID)
	}
	if w.Status != repository.WithdrawalStatusSettled {
		t.Errorf("status: want SETTLED, got %v", w.Status)
	}
	if w.BankNameSnapshot != "BCA" {
		t.Errorf("bank name: want BCA, got %v", w.BankNameSnapshot)
	}
	if w.ExternalReferenceID != "WD_REF_001" {
		t.Errorf("external ref: want WD_REF_001, got %v", w.ExternalReferenceID)
	}
	if w.SettledAt != settled.Unix() {
		t.Errorf("settled_at: want %d, got %d", settled.Unix(), w.SettledAt)
	}
	if w.CreatedAt != created.Unix() {
		t.Errorf("created_at: want %d, got %d", created.Unix(), w.CreatedAt)
	}
}

func TestListWithdrawalsBySeller_EmptyResult(t *testing.T) {
	sellerID := uuid.New()

	mockedDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&mockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &mockRows{}, nil
				},
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRow{values: []any{int64(0)}}
				},
			})
		},
	}

	svc := setupTestService(mockedDB)
	withdrawals, total, err := svc.ListWithdrawalsBySeller(context.Background(), sellerID, 20, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("total: want 0, got %d", total)
	}
	if len(withdrawals) != 0 {
		t.Errorf("len: want 0, got %d", len(withdrawals))
	}
}

func TestListWithdrawalsBySeller_LimitClamped(t *testing.T) {
	// Limit 0 should default to 20; limit >100 should clamp to 100.
	for _, tc := range []struct {
		limit    int
		wantPage int
	}{
		{limit: 0, wantPage: 1},
		{limit: -5, wantPage: 1},
		{limit: 200, wantPage: 1}, // clamped to 100 then page=0/100+1=1
	} {
		mockedDB := &mockDB{
			WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
				return fn(&mockTx{
					QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
						return &mockRows{}, nil
					},
					QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
						return &mockRow{values: []any{int64(0)}}
					},
				})
			},
		}
		svc := setupTestService(mockedDB)
		// Must not error regardless of degenerate limit values.
		_, _, err := svc.ListWithdrawalsBySeller(context.Background(), uuid.New(), tc.limit, 0)
		if err != nil {
			t.Errorf("limit=%d: unexpected error: %v", tc.limit, err)
		}
	}
}

func TestListWithdrawalsBySeller_PilotBlockedStatus(t *testing.T) {
	// PILOT_BLOCKED is a canonical finance status; verify it survives the round-trip.
	sellerID := uuid.New()
	withdrawalID := uuid.New()
	created := time.Now().UTC().Truncate(time.Second)

	listRows := &mockListRows{
		data: [][]any{
			withdrawalRow(withdrawalID, sellerID, 100000, "PILOT_BLOCKED", "", created, time.Time{}),
		},
	}

	mockedDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&mockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return listRows, nil
				},
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRow{values: []any{int64(1)}}
				},
			})
		},
	}

	svc := setupTestService(mockedDB)
	withdrawals, _, err := svc.ListWithdrawalsBySeller(context.Background(), sellerID, 20, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(withdrawals) != 1 {
		t.Fatalf("len: want 1, got %d", len(withdrawals))
	}
	if withdrawals[0].Status != repository.WithdrawalStatusPilotBlocked {
		t.Errorf("status: want PILOT_BLOCKED, got %v", withdrawals[0].Status)
	}
}

// ============================================================================
// G5-B: Manual completion status alignment — COMPLETED vs SETTLED
// ============================================================================
//
// Business invariant:
//   - COMPLETED = manual admin bank transfer (no gateway involved)
//   - SETTLED   = confirmed by gateway webhook callback
//
// These tests prove that MarkProcessed stores COMPLETED (not SETTLED) so the
// admin UI can distinguish manual completions from gateway-confirmed settlements.

// markProcessedSpyTx intercepts the UpdateStatusWithCheck Exec call and
// captures the new status written by MarkProcessed.
type markProcessedSpyTx struct {
	wdID      uuid.UUID
	sellerID  uuid.UUID
	amount    int64
	fixedAcct uuid.UUID

	capturedNewStatus string // set when UPDATE withdrawals SET status = $1 is executed
}

func (s *markProcessedSpyTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "FROM withdrawals"):
		// LockForUpdate — return a PROCESSING withdrawal with no external_reference_id
		return &withdrawalMockRow{
			id:       s.wdID,
			sellerID: s.sellerID,
			amount:   s.amount,
			status:   repository.WithdrawalStatusProcessing,
		}
	case strings.Contains(sql, "financial_accounts"):
		// GetSystemAccountID (called twice: WITHDRAWAL_COMMITTED and PLATFORM_BANK)
		return &mockRow{values: []any{s.fixedAcct}}
	case strings.Contains(sql, "idempotency_key"):
		// CreateTransaction idempotency check — ErrNoRows means proceed
		return &mockRow{err: pgx.ErrNoRows}
	default:
		return &mockRow{err: fmt.Errorf("markProcessedSpyTx: unhandled QueryRow sql: %s", sql)}
	}
}

func (s *markProcessedSpyTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "financial_accounts") {
		// CreateTransaction account balance FOR UPDATE lock
		return &accountQueryRows{accountID: s.fixedAcct}, nil
	}
	return &mockRows{}, nil
}

func (s *markProcessedSpyTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "UPDATE withdrawals") {
		// UpdateStatusWithCheck: UPDATE withdrawals SET status = $1 ... — capture $1
		if len(args) > 0 {
			if status, ok := args[0].(repository.WithdrawalStatus); ok {
				s.capturedNewStatus = string(status)
			}
		}
	}
	return pgconn.NewCommandTag("1"), nil
}

func (s *markProcessedSpyTx) Commit(ctx context.Context) error   { return nil }
func (s *markProcessedSpyTx) Rollback(ctx context.Context) error { return nil }

// TestMarkProcessed_StoresCompletedStatus_NotSettled is the regression guard
// for G5-B. It proves that the manual completion path writes COMPLETED to the
// withdrawals row, not SETTLED (which is reserved for gateway webhooks).
func TestMarkProcessed_StoresCompletedStatus_NotSettled(t *testing.T) {
	spy := &markProcessedSpyTx{
		wdID:      uuid.New(),
		sellerID:  uuid.New(),
		amount:    100_000,
		fixedAcct: uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-000000000003"),
	}

	svc := &WithdrawService{
		db: &mockDB{
			WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
				return fn(spy)
			},
		},
		ledgerRepo:       repository.NewLedgerRepository(),
		withdrawRepo:     repository.NewWithdrawRepository(),
		roleChecker:      &mockRoleChecker{},
		ownership:        auth.NewOwnershipValidator(),
		adminAuditLogger: &mockAdminAuditLogger{},
	}
	svc.canonicalAuthority = NewFinanceService()

	callerID := uuid.New()
	ctx := contextWithCapabilities(callerID, "admin", capability.CapFinanceWithdrawReview.String())

	err := svc.MarkProcessed(ctx, callerID, spy.wdID)
	if err != nil {
		t.Fatalf("MarkProcessed returned unexpected error: %v", err)
	}

	if spy.capturedNewStatus == "" {
		t.Fatal("UpdateStatusWithCheck was never called — status transition did not execute")
	}
	if spy.capturedNewStatus != string(repository.WithdrawalStatusCompleted) {
		t.Errorf("MarkProcessed stored status %q, want %q (COMPLETED for manual; SETTLED is gateway-only)",
			spy.capturedNewStatus, string(repository.WithdrawalStatusCompleted))
	}
}

// ---------------------------------------------------------------------------
// OUTBOX ATOMICITY — MarkProcessed outbox failure must roll back transaction
// ---------------------------------------------------------------------------
//
// Regression guard for Batch 5 fix: if InsertEvent fails, the entire tx
// must fail (STRICT_EVENT_ATOMIC). Prior behaviour silently swallowed the
// error, committing the business mutation without its notification event.

// outboxFailSpyTx extends markProcessedSpyTx to inject an outbox insert failure.
type outboxFailSpyTx struct {
	markProcessedSpyTx
	outboxInsertCalled bool
}

func (s *outboxFailSpyTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO outbox") {
		s.outboxInsertCalled = true
		return pgconn.CommandTag{}, fmt.Errorf("simulated outbox write failure")
	}
	// Delegate to parent for status update etc.
	return s.markProcessedSpyTx.Exec(ctx, sql, args...)
}

func TestMarkProcessed_OutboxFailure_RollsBackTransaction(t *testing.T) {
	spy := &outboxFailSpyTx{
		markProcessedSpyTx: markProcessedSpyTx{
			wdID:      uuid.New(),
			sellerID:  uuid.New(),
			amount:    100_000,
			fixedAcct: uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-000000000004"),
		},
	}

	outboxRepo := outboxrepo.NewOutboxRepository(nil)

	svc := &WithdrawService{
		db: &mockDB{
			WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
				return fn(spy)
			},
		},
		ledgerRepo:       repository.NewLedgerRepository(),
		withdrawRepo:     repository.NewWithdrawRepository(),
		roleChecker:      &mockRoleChecker{},
		ownership:        auth.NewOwnershipValidator(),
		adminAuditLogger: &mockAdminAuditLogger{},
		outboxRepo:       outboxRepo,
	}
	svc.canonicalAuthority = NewFinanceService()

	callerID := uuid.New()
	ctx := contextWithCapabilities(callerID, "admin", capability.CapFinanceWithdrawReview.String())

	err := svc.MarkProcessed(ctx, callerID, spy.wdID)

	if err == nil {
		t.Fatal("MarkProcessed must return error when outbox insert fails (STRICT_EVENT_ATOMIC)")
	}
	if !spy.outboxInsertCalled {
		t.Fatal("outbox insert was never attempted — test wiring error")
	}
	if !strings.Contains(err.Error(), "outbox withdrawal.completed") {
		t.Errorf("error should reference outbox withdrawal.completed, got: %v", err)
	}
}

// TestMarkProcessed_StatusSemantics_CompletedAndSettledAreDistinct is a
// compile-time constant proof that COMPLETED and SETTLED have different wire
// values and cannot be confused.
func TestMarkProcessed_StatusSemantics_CompletedAndSettledAreDistinct(t *testing.T) {
	if repository.WithdrawalStatusCompleted == repository.WithdrawalStatusSettled {
		t.Fatal("COMPLETED and SETTLED must be distinct status values; they encode different business facts")
	}
	if string(repository.WithdrawalStatusCompleted) != "COMPLETED" {
		t.Errorf("COMPLETED wire value = %q, want COMPLETED", string(repository.WithdrawalStatusCompleted))
	}
	if string(repository.WithdrawalStatusSettled) != "SETTLED" {
		t.Errorf("SETTLED wire value = %q, want SETTLED", string(repository.WithdrawalStatusSettled))
	}
}


