//go:build integration

package application_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	financeapp "github.com/labuda/backend/internal/finance/application"
	configapp "github.com/labuda/backend/internal/platform/config/application"
	configrepo "github.com/labuda/backend/internal/platform/config/infrastructure/repository"
	"github.com/labuda/backend/internal/pricing/promotion/contract/application"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	deliveryRepoImpl "github.com/labuda/backend/internal/pricing/promotion/delivery/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

// previewHarness wires the minimal dependencies for PreviewFunding tests.
type previewHarness struct {
	tdb     *testdb.TestDB
	finance *financeapp.FinanceService
	svc     *application.PromotionContractService
	config  *configapp.ConfigService
}

func newPreviewHarness(t *testing.T) *previewHarness {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	bootstrap := financeapp.NewSystemAccountBootstrapFromPgx(db.NewFromPool(tdb.Pool()))
	_, err := bootstrap.EnsureSystemAccounts(ctx)
	require.NoError(t, err)

	financeSvc := financeapp.NewFinanceService()
	cfgSvc := configapp.NewConfigService(configrepo.NewPlatformConfigRepository())
	svc := application.NewPromotionContractService(
		db.NewFromPool(tdb.Pool()),
		financeSvc,
		cfgSvc,
		allowAllGate{},
		deliveryRepoImpl.NewDeliveryRepository(db.NewFromPool(tdb.Pool())),
	)

	return &previewHarness{
		tdb:     tdb,
		finance: financeSvc,
		svc:     svc,
		config:  cfgSvc,
	}
}

func (h *previewHarness) seedConfig(t *testing.T, cpm, minDailyBudget int64) {
	t.Helper()
	ctx := context.Background()
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, execErr := tx.Exec(ctx, `
			INSERT INTO platform_configs (key, value_numeric, updated_at)
			VALUES ('promotion_cpm', $1, EXTRACT(epoch FROM now())::bigint)
			ON CONFLICT (key) DO UPDATE SET value_numeric = EXCLUDED.value_numeric`, cpm)
		if execErr != nil {
			return execErr
		}
		_, execErr = tx.Exec(ctx, `
			INSERT INTO platform_configs (key, value_numeric, updated_at)
			VALUES ('promotion_min_daily_budget', $1, EXTRACT(epoch FROM now())::bigint)
			ON CONFLICT (key) DO UPDATE SET value_numeric = EXCLUDED.value_numeric`, minDailyBudget)
		return execErr
	})
	require.NoError(t, err)
}

func (h *previewHarness) fundSeller(t *testing.T, seller uuid.UUID, amount int64) {
	t.Helper()
	ctx := context.Background()
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return h.finance.RecordPromoteBalanceFunding(ctx, tx, uuid.New(), seller, amount)
	})
	require.NoError(t, err)
}

func (h *previewHarness) balance(t *testing.T, seller uuid.UUID) int64 {
	t.Helper()
	var balance int64
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COALESCE(balance, 0)
		FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2 AND holder_id IS NULL`,
		finance.AccountPromoteBalance, seller,
	).Scan(&balance)
	require.NoError(t, err)
	return balance
}

// ============================================================================
// TEST CASE 1: required < funding → shortage 0, no payment required
// ============================================================================

func TestPreviewFunding_SufficientBalance(t *testing.T) {
	h := newPreviewHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 50_000)

	preview, err := h.svc.PreviewFunding(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 30_000,
		DurationDays: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, preview)

	// required_cost = 30_000, available = 50_000, shortage = 0
	require.Equal(t, int64(30_000), preview.RequiredCost)
	require.Equal(t, int64(50_000), preview.AvailableFunding)
	require.Equal(t, int64(0), preview.Shortage)
	require.False(t, preview.PaymentRequired)

	// Verify balance unchanged (no mutation)
	require.Equal(t, int64(50_000), h.balance(t, seller))
}

// ============================================================================
// TEST CASE 2: required == funding → shortage 0, no payment required
// ============================================================================

func TestPreviewFunding_ExactBalance(t *testing.T) {
	h := newPreviewHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 30_000)

	preview, err := h.svc.PreviewFunding(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 30_000,
		DurationDays: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, preview)

	// required_cost = 30_000, available = 30_000, shortage = 0
	require.Equal(t, int64(30_000), preview.RequiredCost)
	require.Equal(t, int64(30_000), preview.AvailableFunding)
	require.Equal(t, int64(0), preview.Shortage)
	require.False(t, preview.PaymentRequired)

	// Verify balance unchanged
	require.Equal(t, int64(30_000), h.balance(t, seller))
}

// ============================================================================
// TEST CASE 3: required > funding → exact shortage
// ============================================================================

func TestPreviewFunding_InsufficientBalance(t *testing.T) {
	h := newPreviewHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 10_000)

	preview, err := h.svc.PreviewFunding(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 45_000,
		DurationDays: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, preview)

	// required_cost = 45_000, available = 10_000, shortage = 35_000
	require.Equal(t, int64(45_000), preview.RequiredCost)
	require.Equal(t, int64(10_000), preview.AvailableFunding)
	require.Equal(t, int64(35_000), preview.Shortage)
	require.True(t, preview.PaymentRequired)

	// Verify balance unchanged (no mutation despite insufficient)
	require.Equal(t, int64(10_000), h.balance(t, seller))
}

// ============================================================================
// TEST CASE 4: insufficient funding does not mutate allocation
// ============================================================================

func TestPreviewFunding_NoAllocationMutation(t *testing.T) {
	h := newPreviewHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 5_000)

	// Record allocation count before preview
	var allocBefore int
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2`,
		finance.AccountPromotionAllocation, seller,
	).Scan(&allocBefore)
	require.NoError(t, err)

	// Preview should NOT create any allocation account
	preview, err := h.svc.PreviewFunding(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 20_000,
		DurationDays: 2,
	})
	require.NoError(t, err)
	require.True(t, preview.PaymentRequired)

	// Verify no allocation account was created
	var allocAfter int
	err = h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2`,
		finance.AccountPromotionAllocation, seller,
	).Scan(&allocAfter)
	require.NoError(t, err)
	require.Equal(t, allocBefore, allocAfter, "preview must not create allocation accounts")

	// Verify no ledger transactions were created
	var txCount int
	err = h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM ledger_transactions
		WHERE reference_type = 'promotion_allocation'`,
	).Scan(&txCount)
	require.NoError(t, err)
	require.Equal(t, 0, txCount, "preview must not create ledger transactions")

	// Verify balance unchanged
	require.Equal(t, int64(5_000), h.balance(t, seller))
}

// ============================================================================
// TEST CASE 5: zero balance → full shortage
// ============================================================================

func TestPreviewFunding_ZeroBalance(t *testing.T) {
	h := newPreviewHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New() // No funding at all

	preview, err := h.svc.PreviewFunding(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 15_000,
		DurationDays: 1,
	})
	require.NoError(t, err)
	require.NotNil(t, preview)

	// required_cost = 15_000, available = 0, shortage = 15_000
	require.Equal(t, int64(15_000), preview.RequiredCost)
	require.Equal(t, int64(0), preview.AvailableFunding)
	require.Equal(t, int64(15_000), preview.Shortage)
	require.True(t, preview.PaymentRequired)
}

// ============================================================================
// TEST CASE 6: budget below minimum → error (same as Create)
// ============================================================================

func TestPreviewFunding_BudgetBelowMinimum(t *testing.T) {
	h := newPreviewHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 50_000)

	// 5_000 < 10_000 x 1 = 10_000 minimum
	_, err := h.svc.PreviewFunding(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 5_000,
		DurationDays: 1,
	})
	require.Error(t, err)

	var budgetErr *application.ErrPromotionBudgetBelowMinimum
	require.ErrorAs(t, err, &budgetErr)
}

// ============================================================================
// TEST CASE 7: invalid inputs → same errors as Create
// ============================================================================

func TestPreviewFunding_InvalidInputs(t *testing.T) {
	h := newPreviewHarness(t)
	seller := uuid.New()

	// Zero seller
	_, err := h.svc.PreviewFunding(context.Background(), application.CreatePromotionInput{
		SellerID:     uuid.Nil,
		Kind:         entity.KindInternal,
		BudgetRupiah: 10_000,
		DurationDays: 1,
	})
	require.Error(t, err)

	// Invalid kind
	_, err = h.svc.PreviewFunding(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.Kind("bogus"),
		BudgetRupiah: 10_000,
		DurationDays: 1,
	})
	require.ErrorIs(t, err, application.ErrPromotionKindInvalid)

	// Zero budget
	_, err = h.svc.PreviewFunding(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 0,
		DurationDays: 1,
	})
	require.ErrorIs(t, err, application.ErrPromotionBudgetInvalid)

	// Zero duration
	_, err = h.svc.PreviewFunding(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 10_000,
		DurationDays: 0,
	})
	require.ErrorIs(t, err, application.ErrPromotionDurationInvalid)
}
