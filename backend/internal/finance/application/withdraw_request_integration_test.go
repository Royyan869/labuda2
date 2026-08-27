//go:build integration

package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	financerepo "github.com/labuda/backend/internal/finance/infrastructure/repository"
	bankaccountrepo "github.com/labuda/backend/internal/finance/bankaccount/infrastructure/repository"
	ledgerentryrepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

type withdrawalIntegrationAuthority struct {
	ledgerRepo           *financerepo.LedgerRepository
	sellerPayableID      uuid.UUID
	withdrawalPendingID   uuid.UUID
	withdrawable         int64
	activeDisputeFreeze  int64
}

func (a *withdrawalIntegrationAuthority) AssertSellerWithdrawalAllowed(
	_ context.Context,
	_ db.Tx,
	sellerID uuid.UUID,
	amount int64,
) (*SellerWithdrawableSummary, error) {
	summary := &SellerWithdrawableSummary{
		PayableBalance:      a.withdrawable + a.activeDisputeFreeze,
		ActiveDisputeFreeze: a.activeDisputeFreeze,
		Withdrawable:        a.withdrawable,
	}
	if amount > a.withdrawable {
		return nil, &ErrWithdrawalBlockedByWithdrawableBalance{
			SellerID:            sellerID,
			RequestedAmount:     amount,
			PayableBalance:      summary.PayableBalance,
			ActiveDisputeFreeze: summary.ActiveDisputeFreeze,
			Withdrawable:        summary.Withdrawable,
		}
	}
	return summary, nil
}

func (a *withdrawalIntegrationAuthority) RecordWithdrawalRequest(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	amount, feeAmount int64,
	withdrawalID uuid.UUID,
) error {
	return a.ledgerRepo.CreateTransaction(
		ctx,
		tx,
		fmt.Sprintf("withdrawal_request_%s", withdrawalID.String()),
		"withdrawal_request",
		withdrawalID,
		nil,
		nil,
		[]ledgerentryrepo.Entry{
			{AccountID: a.sellerPayableID, Amount: money.New(-amount)},
			{AccountID: a.withdrawalPendingID, Amount: money.New(amount)},
		},
	)
}

func (a *withdrawalIntegrationAuthority) RecordWithdrawalCommit(
	context.Context,
	db.Tx,
	uuid.UUID,
	int64,
	int64,
	uuid.UUID,
) error {
	return nil
}

func (a *withdrawalIntegrationAuthority) RecordWithdrawalReject(
	context.Context,
	db.Tx,
	uuid.UUID,
	int64,
	int64,
	uuid.UUID,
) error {
	return nil
}

func (a *withdrawalIntegrationAuthority) RecordWithdrawalRestore(
	context.Context,
	db.Tx,
	uuid.UUID,
	int64,
	int64,
	uuid.UUID,
) error {
	return nil
}

func (a *withdrawalIntegrationAuthority) RecordWithdrawalComplete(
	context.Context,
	db.Tx,
	uuid.UUID,
	int64,
	int64,
	uuid.UUID,
) error {
	return nil
}

type withdrawalVerificationGate struct {
	verified bool
	reviewed bool
}

func (g withdrawalVerificationGate) IsSellerVerifiedTx(_ context.Context, _ db.Tx, _ uuid.UUID) (bool, error) {
	return g.verified, nil
}

func (g withdrawalVerificationGate) IsReviewedBankAccountTx(_ context.Context, _ db.Tx, _, _ uuid.UUID) (bool, error) {
	return g.reviewed, nil
}

func setupWithdrawalIntegrationService(
	t *testing.T,
	withdrawable int64,
) (*testdb.TestDB, *WithdrawService, uuid.UUID, uuid.UUID, func()) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	sellerID := uuid.New()
	bankID := uuid.New()
	sellerPayableID := uuid.New()
	withdrawalPendingID := uuid.New()

	err := tdb.WithTx(context.Background(), func(tx db.Tx) error {
		now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
		_, err := tx.Exec(context.Background(), `
			INSERT INTO users (
				id, firebase_uid, email, account_status, created_at, updated_at, role
			)
			VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
		`, sellerID, sellerID.String(), sellerID.String()+"@example.invalid")
		require.NoError(t, err)
		_, err = tx.Exec(context.Background(), `
			INSERT INTO bank_accounts (
				id, user_id, bank_name, bank_code, account_number, account_holder_name,
				is_default, status, deleted_at, created_at, updated_at
			)
			VALUES ($1, $2, 'Bank Central Asia', 'BCA', '1234567890', 'Seller One', true, 'active', NULL, $3, $3)
		`, bankID, sellerID, now)
		require.NoError(t, err)

		_, err = tx.Exec(context.Background(), `
			INSERT INTO financial_accounts (
				id, user_id, account_type, balance, currency, name, is_active, created_at, updated_at
			)
			VALUES
				($1, $2, 'SELLER_PAYABLE', 1000000, 'IDR', 'Seller Payable', true, $3, $3),
				($4, $2, 'WITHDRAWAL_PENDING', 0, 'IDR', 'Withdrawal Pending', true, $3, $3)
		`, sellerPayableID, sellerID, now, withdrawalPendingID)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	authority := &withdrawalIntegrationAuthority{
		ledgerRepo:          financerepo.NewLedgerRepository(),
		sellerPayableID:     sellerPayableID,
		withdrawalPendingID:  withdrawalPendingID,
		withdrawable:        withdrawable,
	}

	svc := &WithdrawService{
		db:                  tdb,
		ledgerRepo:          financerepo.NewLedgerRepository(),
		withdrawRepo:        financerepo.NewWithdrawRepository(),
		bankAccountRepo:     bankaccountrepo.NewBankAccountRepository(),
		roleChecker:         &mockRoleChecker{},
		accountStatusChecker: &mockAccountStatusChecker{},
		ownership:           auth.NewOwnershipValidator(),
		adminAuditLogger:    &mockAdminAuditLogger{},
		verificationService: withdrawalVerificationGate{
			verified: true,
			reviewed: true,
		},
		outboxRepo: nil,
	}
	svc.SetCanonicalAuthority(authority)

	return tdb, svc, sellerID, bankID, cleanup
}

func countWithdrawalsBySeller(t *testing.T, ctx context.Context, tx db.Tx, sellerID uuid.UUID) int64 {
	t.Helper()

	var count int64
	require.NoError(t, tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM withdrawals
		WHERE seller_id = $1
	`, sellerID).Scan(&count))
	return count
}

func countLedgerTransactionsByPrefix(t *testing.T, ctx context.Context, tx db.Tx, prefix string) int64 {
	t.Helper()

	var count int64
	require.NoError(t, tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM ledger_transactions
		WHERE idempotency_key LIKE $1
	`, prefix+"%").Scan(&count))
	return count
}

func countLedgerEntries(t *testing.T, ctx context.Context, tx db.Tx) int64 {
	t.Helper()

	var count int64
	require.NoError(t, tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM ledger_entries
	`).Scan(&count))
	return count
}

func seedWithdrawalRowFromService(
	t *testing.T,
	ctx context.Context,
	svc *WithdrawService,
	sellerID uuid.UUID,
	amount int64,
) (*CanonicalRequestWithdrawalOutput, error) {
	t.Helper()

	return svc.RequestWithdrawal(ctx, CanonicalRequestWithdrawalInput{
		SellerID: sellerID,
		Amount:   amount,
	})
}

func TestRequestWithdrawal_Idempotency_ConcurrentSameSellerCreatesOneWithdrawalAndOneLedgerReservation(t *testing.T) {
	tdb, svc, sellerID, _, cleanup := setupWithdrawalIntegrationService(t, 500000)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const amount = int64(100000)

	var wg sync.WaitGroup
	startCh := make(chan struct{})
	errCh := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			_, err := seedWithdrawalRowFromService(t, ctx, svc, sellerID, amount)
			errCh <- err
		}()
	}

	close(startCh)
	wg.Wait()
	close(errCh)

	// Exactly one request succeeds; the concurrent duplicate is rejected by
	// the single-in-flight-per-seller guard (ErrWithdrawalPendingExists).
	var succeeded, rejected int
	for err := range errCh {
		if err == nil {
			succeeded++
			continue
		}
		var pending *ErrWithdrawalPendingExists
		if errors.As(err, &pending) {
			rejected++
			continue
		}
		t.Fatalf("unexpected error: %v", err)
	}
	require.Equal(t, 1, succeeded, "exactly one concurrent request must succeed")
	require.Equal(t, 1, rejected, "exactly one concurrent duplicate must be rejected")

	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	require.Equal(t, int64(1), countWithdrawalsBySeller(t, ctx, tx, sellerID))
	require.Equal(t, int64(1), countLedgerTransactionsByPrefix(t, ctx, tx, "withdrawal_request_"))
	require.Equal(t, int64(2), countLedgerEntries(t, ctx, tx))
}

func TestRequestWithdrawal_IdempotencyRetry_SameSellerActiveWithdrawalRejected(t *testing.T) {
	tdb, svc, sellerID, _, cleanup := setupWithdrawalIntegrationService(t, 500000)
	defer cleanup()

	ctx := context.Background()
	const amount = int64(100000)

	first, err := seedWithdrawalRowFromService(t, ctx, svc, sellerID, amount)
	require.NoError(t, err)
	require.NotNil(t, first)

	// A retry while the first withdrawal is still in flight is rejected by the
	// single-in-flight-per-seller guard; the original withdrawal is returned
	// for reconciliation.
	_, err = seedWithdrawalRowFromService(t, ctx, svc, sellerID, amount)
	require.Error(t, err)
	var pending *ErrWithdrawalPendingExists
	require.True(t, errors.As(err, &pending))
	require.Equal(t, first.WithdrawalID, pending.ExistingWithdrawalID)

	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	require.Equal(t, int64(1), countWithdrawalsBySeller(t, ctx, tx, sellerID))
	require.Equal(t, int64(1), countLedgerTransactionsByPrefix(t, ctx, tx, "withdrawal_request_"))
	require.Equal(t, int64(2), countLedgerEntries(t, ctx, tx))
}

func TestRequestWithdrawal_IdempotencyConflict_SameSellerDifferentAmountRejected(t *testing.T) {
	tdb, svc, sellerID, _, cleanup := setupWithdrawalIntegrationService(t, 500000)
	defer cleanup()

	ctx := context.Background()

	_, err := seedWithdrawalRowFromService(t, ctx, svc, sellerID, 100000)
	require.NoError(t, err)

	// A different amount while the first withdrawal is still in flight is
	// rejected by the single-in-flight-per-seller guard.
	_, err = seedWithdrawalRowFromService(t, ctx, svc, sellerID, 120000)
	require.Error(t, err)
	var pending *ErrWithdrawalPendingExists
	require.True(t, errors.As(err, &pending))

	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	require.Equal(t, int64(1), countWithdrawalsBySeller(t, ctx, tx, sellerID))
	require.Equal(t, int64(1), countLedgerTransactionsByPrefix(t, ctx, tx, "withdrawal_request_"))
	require.Equal(t, int64(2), countLedgerEntries(t, ctx, tx))
}

func TestRequestWithdrawal_AfterTerminalWithdrawalCreatesANewRow(t *testing.T) {
	tdb, svc, sellerID, _, cleanup := setupWithdrawalIntegrationService(t, 500000)
	defer cleanup()

	ctx := context.Background()
	const amount = int64(100000)

	first, err := seedWithdrawalRowFromService(t, ctx, svc, sellerID, amount)
	require.NoError(t, err)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, updateErr := svc.withdrawRepo.UpdateStatus(ctx, tx, first.WithdrawalID, financerepo.WithdrawalStatusFailed)
		return updateErr
	})
	require.NoError(t, err)

	// Once the first withdrawal is terminal, a fresh request is allowed.
	second, err := seedWithdrawalRowFromService(t, ctx, svc, sellerID, amount)
	require.NoError(t, err)
	require.NotEqual(t, first.WithdrawalID, second.WithdrawalID)

	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	require.Equal(t, int64(2), countWithdrawalsBySeller(t, ctx, tx, sellerID))
	require.Equal(t, int64(2), countLedgerTransactionsByPrefix(t, ctx, tx, "withdrawal_request_"))
	require.Equal(t, int64(4), countLedgerEntries(t, ctx, tx))
}

func TestRequestWithdrawal_InsufficientBalanceRejectedWithoutWritingRows(t *testing.T) {
	tdb, svc, sellerID, _, cleanup := setupWithdrawalIntegrationService(t, 50000)
	defer cleanup()

	ctx := context.Background()
	_, err := seedWithdrawalRowFromService(t, ctx, svc, sellerID, 100000)
	require.Error(t, err)

	var blocked *ErrWithdrawalBlockedByWithdrawableBalance
	require.True(t, errors.As(err, &blocked))

	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	require.Equal(t, int64(0), countWithdrawalsBySeller(t, ctx, tx, sellerID))
	require.Equal(t, int64(0), countLedgerTransactionsByPrefix(t, ctx, tx, "withdrawal_request_"))
	require.Equal(t, int64(0), countLedgerEntries(t, ctx, tx))
}
