package application

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	bankaccountrepo "github.com/labuda/backend/internal/finance/bankaccount/infrastructure/repository"
	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type requestWithdrawalIdempotencyTx struct {
	sellerID          uuid.UUID
	bankID            uuid.UUID
	bankName          string
	bankCode          string
	accountNumber     string
	accountHolderName string
	createdWithdrawal *repository.Withdrawal
	createCount       int
}

func (t *requestWithdrawalIdempotencyTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "idempotency_key"):
		if t.createdWithdrawal == nil {
			return &mockRow{err: pgx.ErrNoRows}
		}
		return &idempotencyWithdrawalRow{withdrawal: t.createdWithdrawal}
	case strings.Contains(sql, "FROM withdrawals"):
		return &mockRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "FROM bank_accounts"):
		now := time.Now()
		return &extendedMockRow{values: []any{
			t.bankID,
			t.sellerID,
			t.bankName,
			t.bankCode,
			t.accountNumber,
			t.accountHolderName,
			true,
			"active",
			(*time.Time)(nil),
			now,
			now,
		}}
	default:
		return &mockRow{err: fmt.Errorf("unexpected query in idempotency tx: %s", sql)}
	}
}

func (t *requestWithdrawalIdempotencyTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &mockRows{}, nil
}

func (t *requestWithdrawalIdempotencyTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO withdrawals") {
		t.createCount++
		now := time.Now()
		t.createdWithdrawal = &repository.Withdrawal{
			ID:                    args[0].(uuid.UUID),
			SellerID:              args[1].(uuid.UUID),
			SellerUsername:        "seller",
			SellerFarmName:        "farm",
			Amount:                args[2].(int64),
			FeeAmount:             args[3].(int64),
			Status:                args[4].(repository.WithdrawalStatus),
			IdempotencyKey:        args[5].(string),
			BankNameSnapshot:      args[6].(string),
			BankCodeSnapshot:      args[7].(string),
			AccountNumberSnapshot: args[8].(string),
			AccountHolderSnapshot: args[9].(string),
			CreatedAt:             now.Unix(),
			UpdatedAt:             now.Unix(),
		}
	}
	return pgconn.NewCommandTag("1"), nil
}

func (t *requestWithdrawalIdempotencyTx) Commit(ctx context.Context) error   { return nil }
func (t *requestWithdrawalIdempotencyTx) Rollback(ctx context.Context) error { return nil }

type idempotencyWithdrawalRow struct {
	withdrawal *repository.Withdrawal
}

func (r *idempotencyWithdrawalRow) Scan(dest ...any) error {
	if r.withdrawal == nil {
		return pgx.ErrNoRows
	}
	createdAt := time.Unix(r.withdrawal.CreatedAt, 0)
	updatedAt := time.Unix(r.withdrawal.UpdatedAt, 0)

	for i, d := range dest {
		switch ptr := d.(type) {
		case *uuid.UUID:
			switch i {
			case 0:
				*ptr = r.withdrawal.ID
			case 1:
				*ptr = r.withdrawal.SellerID
			}
		case *string:
			switch i {
			case 2:
				*ptr = r.withdrawal.SellerUsername
			case 3:
				*ptr = r.withdrawal.SellerFarmName
			case 7:
				*ptr = r.withdrawal.IdempotencyKey
			case 8:
				*ptr = r.withdrawal.BankNameSnapshot
			case 9:
				*ptr = r.withdrawal.BankCodeSnapshot
			case 10:
				*ptr = r.withdrawal.AccountNumberSnapshot
			case 11:
				*ptr = r.withdrawal.AccountHolderSnapshot
			case 12:
				*ptr = r.withdrawal.ExternalReferenceID
			case 13:
				*ptr = r.withdrawal.GatewayResponse
			case 14:
				*ptr = r.withdrawal.FailureReason
			}
		case *int64:
			switch i {
			case 4:
				*ptr = r.withdrawal.Amount
			case 5:
				*ptr = r.withdrawal.FeeAmount
			case 15:
				*ptr = r.withdrawal.SubmittedAt
			case 16:
				*ptr = r.withdrawal.SettledAt
			}
		case *repository.WithdrawalStatus:
			if i == 6 {
				*ptr = r.withdrawal.Status
			}
		case *int:
			if i == 17 {
				*ptr = r.withdrawal.RetryCount
			}
		case *time.Time:
			switch i {
			case 18:
				*ptr = createdAt
			case 19:
				*ptr = updatedAt
			}
		}
	}
	return nil
}

func buildIdempotencyService(tx *requestWithdrawalIdempotencyTx) *WithdrawService {
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
	return svc
}

func TestRequestWithdrawal_IdempotencyKey_RetriesReturnSameWithdrawal(t *testing.T) {
	tx := &requestWithdrawalIdempotencyTx{
		sellerID:          uuid.New(),
		bankID:            uuid.New(),
		bankName:          "BCA",
		bankCode:          "BCA",
		accountNumber:     "1234567890",
		accountHolderName: "Budi Santoso",
	}
	svc := buildIdempotencyService(tx)
	input := CanonicalRequestWithdrawalInput{
		SellerID: tx.sellerID,
		Amount:   100_000,
	}

	first, err := svc.RequestWithdrawal(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NotNil(t, tx.createdWithdrawal)
	assert.Equal(t, 1, tx.createCount)
	assert.Equal(t, tx.createdWithdrawal.ID, first.WithdrawalID)
	assert.Equal(t, repository.WithdrawalStatusRequested, first.Status)

	// Retry with the same payload must not create a second row; the
	// single-in-flight-per-seller guard (ErrWithdrawalPendingExists) rejects
	// the duplicate while the original withdrawal remains active.
	_, err = svc.RequestWithdrawal(context.Background(), input)
	require.Error(t, err)
	var pending *ErrWithdrawalPendingExists
	require.ErrorAs(t, err, &pending)
	assert.Equal(t, tx.createdWithdrawal.ID, pending.ExistingWithdrawalID)
	assert.Equal(t, 1, tx.createCount, "retry must not insert a second withdrawal row")
}

func TestRequestWithdrawal_IdempotencyKey_DifferentAmountConflicts(t *testing.T) {
	tx := &requestWithdrawalIdempotencyTx{
		sellerID:          uuid.New(),
		bankID:            uuid.New(),
		bankName:          "BCA",
		bankCode:          "BCA",
		accountNumber:     "1234567890",
		accountHolderName: "Budi Santoso",
	}
	svc := buildIdempotencyService(tx)
	firstInput := CanonicalRequestWithdrawalInput{
		SellerID: tx.sellerID,
		Amount:   100_000,
	}

	_, err := svc.RequestWithdrawal(context.Background(), firstInput)
	require.NoError(t, err)
	assert.Equal(t, 1, tx.createCount)

	// A different amount while the first withdrawal is still in flight is
	// rejected by the single-in-flight-per-seller guard.
	_, err = svc.RequestWithdrawal(context.Background(), CanonicalRequestWithdrawalInput{
		SellerID: tx.sellerID,
		Amount:   120_000,
	})
	require.Error(t, err)

	var pending *ErrWithdrawalPendingExists
	require.ErrorAs(t, err, &pending)
	assert.Equal(t, tx.createdWithdrawal.ID, pending.ExistingWithdrawalID)
	assert.Equal(t, 1, tx.createCount, "conflict must not insert a second withdrawal row")
}
