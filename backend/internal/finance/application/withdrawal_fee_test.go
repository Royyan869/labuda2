package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	ledgerepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type capturedLedgerTransaction struct {
	idempotencyKey string
	referenceType  string
	entries        []ledgerepo.Entry
}

type feeLedgerRepo struct {
	systemAccounts map[string]uuid.UUID
	userAccounts   map[string]uuid.UUID
	balance        int64

	transactions []capturedLedgerTransaction
	seen         map[string]bool
}

func newFeeLedgerRepo(balance int64) *feeLedgerRepo {
	return &feeLedgerRepo{
		systemAccounts: map[string]uuid.UUID{
			finance.AccountWithdrawalPending:   uuid.New(),
			finance.AccountWithdrawalCommitted: uuid.New(),
			finance.AccountPlatformBank:        uuid.New(),
			finance.AccountPlatformRevenue:     uuid.New(),
			finance.AccountSellerPayable:       uuid.New(),
		},
		userAccounts: map[string]uuid.UUID{},
		balance:      balance,
		seen:         map[string]bool{},
	}
}

func (r *feeLedgerRepo) CreateTransaction(
	_ context.Context,
	_ db.Tx,
	idempotencyKey string,
	referenceType string,
	_ uuid.UUID,
	_ *uuid.UUID,
	_ *uuid.UUID,
	entries []ledgerepo.Entry,
) error {
	if r.seen[idempotencyKey] {
		return nil
	}
	r.seen[idempotencyKey] = true
	copied := append([]ledgerepo.Entry(nil), entries...)
	r.transactions = append(r.transactions, capturedLedgerTransaction{
		idempotencyKey: idempotencyKey,
		referenceType:  referenceType,
		entries:        copied,
	})
	return nil
}

func (r *feeLedgerRepo) GetAccountBalance(_ context.Context, _ db.Tx, _ uuid.UUID) (money.Money, error) {
	return money.New(r.balance), nil
}

func (r *feeLedgerRepo) GetAccountBalanceForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (money.Money, error) {
	return money.New(r.balance), nil
}

func (r *feeLedgerRepo) GetSystemAccountID(_ context.Context, _ db.Tx, accountType string) (uuid.UUID, error) {
	return r.systemAccounts[accountType], nil
}

func (r *feeLedgerRepo) GetUserAccountID(_ context.Context, _ db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error) {
	key := accountType + ":" + userID.String()
	if id, ok := r.userAccounts[key]; ok {
		return id, nil
	}
	return uuid.Nil, nil
}

func (r *feeLedgerRepo) GetOrCreateUserAccount(_ context.Context, _ db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error) {
	key := accountType + ":" + userID.String()
	if id, ok := r.userAccounts[key]; ok {
		return id, nil
	}
	id := uuid.New()
	r.userAccounts[key] = id
	return id, nil
}

func (r *feeLedgerRepo) CountTransactionsByEntityID(_ context.Context, _ db.Tx, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (r *feeLedgerRepo) GetTotalCreditToUserAccount(_ context.Context, _ db.Tx, _ string, _ uuid.UUID) (int64, error) {
	return 0, nil
}

var _ ledgerepo.LedgerRepository = (*feeLedgerRepo)(nil)

// TestWithdrawalFeeAmount_IsFiveThousandRupiah locks the PASS_18H-corrected
// fee constant. MU1 regression guard: the fee was previously 500_000 (100x
// too large under the Rupiah-integer canonical unit).
func TestWithdrawalFeeAmount_IsFiveThousandRupiah(t *testing.T) {
	require.Equal(t, int64(5_000), int64(WithdrawalFeeAmount))
}

// PASS_18H MONEY MODEL: the fee is deducted FROM the requested amount, never
// added on top. AssertSellerWithdrawalAllowed checks the balance against the
// requested amount only — the fee is not an additional debit.
func TestAssertSellerWithdrawalAllowed_ChecksRequestedAmountOnly(t *testing.T) {
	sellerID := uuid.New()
	repo := newFeeLedgerRepo(100_000)
	svc := &FinanceService{ledgerRepo: repo, logger: zap.NewNop()}

	summary, err := svc.AssertSellerWithdrawalAllowed(context.Background(), nil, sellerID, 100_000)
	require.NoError(t, err)
	require.NotNil(t, summary)
	require.Equal(t, int64(100_000), summary.Withdrawable)

	// Rp99,999 balance cannot fund a Rp100,000 withdrawal request.
	repo.balance = 99_999
	_, err = svc.AssertSellerWithdrawalAllowed(context.Background(), nil, sellerID, 100_000)
	require.Error(t, err)
	var blocked *ErrWithdrawalBlockedByWithdrawableBalance
	require.True(t, errors.As(err, &blocked))

	// Rp100,000 balance can fund a Rp100,000 withdrawal request.
	repo.balance = 100_000
	_, err = svc.AssertSellerWithdrawalAllowed(context.Background(), nil, sellerID, 100_000)
	require.NoError(t, err)
}

// TestWithdrawalRequestAndCompletionSplitFeeFromAmount proves the canonical
// subtractive money model end-to-end: request Rp100,000 -> seller payable
// debited Rp100,000 (not Rp105,000) -> on completion, platform bank receives
// the net payout Rp95,000 and platform revenue receives the Rp5,000 fee.
func TestWithdrawalRequestAndCompletionSplitFeeFromAmount(t *testing.T) {
	sellerID := uuid.New()
	withdrawalID := uuid.New()
	repo := newFeeLedgerRepo(10_000_000)
	svc := &FinanceService{ledgerRepo: repo, logger: zap.NewNop()}

	const requestedAmount = 100_000
	const feeAmount = 5_000
	const netPayout = requestedAmount - feeAmount // 95_000

	err := svc.RecordWithdrawalRequest(context.Background(), nil, sellerID, requestedAmount, feeAmount, withdrawalID)
	require.NoError(t, err)
	require.Len(t, repo.transactions, 1)
	require.Equal(t, "withdrawal_request", repo.transactions[0].referenceType)
	require.Len(t, repo.transactions[0].entries, 2)
	require.Equal(t, int64(-requestedAmount), repo.transactions[0].entries[0].Amount.Int64())
	require.Equal(t, int64(requestedAmount), repo.transactions[0].entries[1].Amount.Int64())

	err = svc.RecordWithdrawalCommit(context.Background(), nil, sellerID, requestedAmount, feeAmount, withdrawalID)
	require.NoError(t, err)
	require.Len(t, repo.transactions, 2)
	require.Equal(t, "withdrawal_commit", repo.transactions[1].referenceType)
	require.Equal(t, int64(-requestedAmount), repo.transactions[1].entries[0].Amount.Int64())
	require.Equal(t, int64(requestedAmount), repo.transactions[1].entries[1].Amount.Int64())

	err = svc.RecordWithdrawalComplete(context.Background(), nil, sellerID, requestedAmount, feeAmount, withdrawalID)
	require.NoError(t, err)
	require.Len(t, repo.transactions, 3)
	require.Equal(t, "withdrawal_complete", repo.transactions[2].referenceType)
	require.Len(t, repo.transactions[2].entries, 3)
	// committed account: -requestedAmount (full reservation drained)
	require.Equal(t, int64(-requestedAmount), repo.transactions[2].entries[0].Amount.Int64())
	// platform bank: +netPayout (Rp95,000 — what actually reaches the seller's bank)
	require.Equal(t, int64(netPayout), repo.transactions[2].entries[1].Amount.Int64())
	// platform revenue: +feeAmount (Rp5,000 — the platform's cut)
	require.Equal(t, int64(feeAmount), repo.transactions[2].entries[2].Amount.Int64())

	// Duplicate success should not double-book because the idempotency key
	// collapses the repeated transaction into a no-op.
	err = svc.RecordWithdrawalComplete(context.Background(), nil, sellerID, requestedAmount, feeAmount, withdrawalID)
	require.NoError(t, err)
	require.Len(t, repo.transactions, 3)
}

// TestWithdrawalFailureRestoresRequestedAmountOnly proves that reject/restore
// paths return exactly the reserved requested amount to the seller payable
// account — never requestedAmount+fee.
func TestWithdrawalFailureRestoresRequestedAmountOnly(t *testing.T) {
	sellerID := uuid.New()
	withdrawalID := uuid.New()
	repo := newFeeLedgerRepo(10_000_000)
	svc := &FinanceService{ledgerRepo: repo, logger: zap.NewNop()}

	const requestedAmount = 100_000
	const feeAmount = 5_000

	err := svc.RecordWithdrawalReject(context.Background(), nil, sellerID, requestedAmount, feeAmount, withdrawalID)
	require.NoError(t, err)
	require.Len(t, repo.transactions, 1)
	require.Equal(t, int64(-requestedAmount), repo.transactions[0].entries[0].Amount.Int64())
	require.Equal(t, int64(requestedAmount), repo.transactions[0].entries[1].Amount.Int64())

	err = svc.RecordWithdrawalRestore(context.Background(), nil, sellerID, requestedAmount, feeAmount, withdrawalID)
	require.NoError(t, err)
	require.Len(t, repo.transactions, 2)
	require.Equal(t, int64(-requestedAmount), repo.transactions[1].entries[0].Amount.Int64())
	require.Equal(t, int64(requestedAmount), repo.transactions[1].entries[1].Amount.Int64())
}

// TestRecordWithdrawalComplete_RejectsFeeGreaterOrEqualToAmount guards the
// invariant that the fee can never consume the entire (or more than the)
// requested amount.
func TestRecordWithdrawalComplete_RejectsFeeGreaterOrEqualToAmount(t *testing.T) {
	sellerID := uuid.New()
	withdrawalID := uuid.New()
	repo := newFeeLedgerRepo(10_000_000)
	svc := &FinanceService{ledgerRepo: repo, logger: zap.NewNop()}

	err := svc.RecordWithdrawalComplete(context.Background(), nil, sellerID, 5_000, 5_000, withdrawalID)
	require.Error(t, err)
}
