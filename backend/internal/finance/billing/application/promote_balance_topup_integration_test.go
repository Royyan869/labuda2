//go:build integration

package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	financeapp "github.com/labuda/backend/internal/finance/application"
	billingentity "github.com/labuda/backend/internal/finance/billing/entity"
	billingrepo "github.com/labuda/backend/internal/finance/billing/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CANONICAL PROMOTE BALANCE TOP-UP — REAL-DB PROOF (PHASE 2)
//
// The billing payment pipeline (verified gateway webhook -> MarkPaid) routes
// billing type 'promote_balance_top_up' through
// FinanceService.RecordPromoteBalanceFunding:
//
//	BANK_SETTLEMENT -> PROMOTE_BALANCE[seller]
//
// A top-up is FUNDING, not revenue: PLATFORM_REVENUE must stay untouched and a
// top-up must not carry a platform fee.
// ============================================================================

func TestPromoteBalanceTopUp_MarkPaid_CreditsPromoteBalanceNotRevenue(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	bootstrap := financeapp.NewSystemAccountBootstrapFromPgx(db.NewFromPool(tdb.Pool()))
	_, err := bootstrap.EnsureSystemAccounts(ctx)
	require.NoError(t, err)

	seller := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
	`, seller, "fb-topup-"+seller.String()[:8], seller.String()+"@test.local")
	require.NoError(t, err)

	repo := billingrepo.NewBillingRepository()
	svc := &BillingService{
		billingRepo:          repo,
		financeService:       financeapp.NewFinanceService(),
		roleChecker:          nil,
		accountStatusChecker: nil,
		ownership:            nil,
	}

	revenueBefore := systemBalance(t, tdb, finance.AccountPlatformRevenue)
	bankBefore := systemBalance(t, tdb, finance.AccountBankSettlement)

	billing, err := billingentity.NewBillingTransaction(
		seller, uuid.New(), billingentity.TypePromoteBalanceTopUp, money.New(50_000), 0,
	)
	require.NoError(t, err)

	// Verified webhook settlement: mark paid inside one tx.
	newlyPaid := false
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := repo.CreateBillingTransaction(ctx, tx, billing); err != nil {
			return err
		}
		newlyPaid, err = svc.MarkPaid(ctx, tx, billing.ID)
		return err
	}))
	require.True(t, newlyPaid)

	// PROMOTE_BALANCE credited exactly the gross; PLATFORM_REVENUE untouched;
	// BANK_SETTLEMENT drained by the same amount.
	require.Equal(t, int64(50_000), userBalance(t, tdb, finance.AccountPromoteBalance, seller))
	require.Equal(t, revenueBefore, systemBalance(t, tdb, finance.AccountPlatformRevenue),
		"a top-up must NEVER book platform revenue")
	require.Equal(t, bankBefore-50_000, systemBalance(t, tdb, finance.AccountBankSettlement))

	// Duplicate webhook: MarkPaid returns newlyPaid=false; no double credit.
	newlyPaid2 := false
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		newlyPaid2, err = svc.MarkPaid(ctx, tx, billing.ID)
		return err
	}))
	require.False(t, newlyPaid2, "duplicate webhook must not re-run post-payment side effects")
	require.Equal(t, int64(50_000), userBalance(t, tdb, finance.AccountPromoteBalance, seller))
}

func TestPromoteBalanceTopUp_RejectsPlatformFee(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	bootstrap := financeapp.NewSystemAccountBootstrapFromPgx(db.NewFromPool(tdb.Pool()))
	_, err := bootstrap.EnsureSystemAccounts(ctx)
	require.NoError(t, err)

	seller := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
	`, seller, "fb-topup-fee-"+seller.String()[:8], seller.String()+"@test.local")
	require.NoError(t, err)

	repo := billingrepo.NewBillingRepository()
	svc := &BillingService{
		billingRepo:          repo,
		financeService:       financeapp.NewFinanceService(),
		roleChecker:          nil,
		accountStatusChecker: nil,
		ownership:            nil,
	}

	// A top-up carrying a platform fee would let funding leak into revenue —
	// fail closed at MarkPaid. The billing row is created in its own committed
	// transaction; only the (failing) payment settlement is rolled back.
	billing, err := billingentity.NewBillingTransaction(
		seller, uuid.New(), billingentity.TypePromoteBalanceTopUp, money.New(10_000), 5,
	)
	require.NoError(t, err)
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.CreateBillingTransaction(ctx, tx, billing)
	}))

	paidErr := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := svc.MarkPaid(ctx, tx, billing.ID)
		return err
	})
	require.Error(t, paidErr)
	require.Contains(t, paidErr.Error(), "must not carry a platform fee")

	// Nothing was credited and the billing row is still pending.
	var balances int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM financial_accounts WHERE account_type = $1 AND user_id = $2
	`, finance.AccountPromoteBalance, seller).Scan(&balances))
	require.Equal(t, int64(0), balances)

	var status string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status::text FROM billing_transactions WHERE id = $1`, billing.ID).Scan(&status))
	require.Equal(t, string(billingentity.StatusPending), status)
}

func userBalance(t *testing.T, tdb *testdb.TestDB, accountType string, userID uuid.UUID) int64 {
	t.Helper()
	var balance int64
	err := tdb.Pool().QueryRow(context.Background(), `
		SELECT balance FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2 AND holder_id IS NULL
	`, accountType, userID).Scan(&balance)
	require.NoError(t, err)
	return balance
}

func systemBalance(t *testing.T, tdb *testdb.TestDB, accountType string) int64 {
	t.Helper()
	var balance int64
	err := tdb.Pool().QueryRow(context.Background(), `
		SELECT balance FROM financial_accounts WHERE account_type = $1 AND user_id IS NULL
	`, accountType).Scan(&balance)
	require.NoError(t, err)
	return balance
}
