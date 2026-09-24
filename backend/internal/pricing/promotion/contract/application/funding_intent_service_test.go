//go:build integration

package application_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	financeapp "github.com/labuda/backend/internal/finance/application"
	billingapp "github.com/labuda/backend/internal/finance/billing/application"
	configapp "github.com/labuda/backend/internal/platform/config/application"
	configrepo "github.com/labuda/backend/internal/platform/config/infrastructure/repository"
	"github.com/labuda/backend/internal/pricing/promotion/contract/application"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	deliveryRepoImpl "github.com/labuda/backend/internal/pricing/promotion/delivery/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// intentHarness wires dependencies for FundingIntentService tests.
type intentHarness struct {
	tdb         *testdb.TestDB
	finance     *financeapp.FinanceService
	billing     *billingapp.BillingService
	svc         *application.FundingIntentService
	config      *configapp.ConfigService
	contractSvc *application.PromotionContractService
}

// stubRoleChecker is a minimal RoleChecker that allows all calls.
type stubRoleChecker struct{}

func (stubRoleChecker) IsAdmin(_ context.Context, _ uuid.UUID) (bool, error) { return false, nil }
func (stubRoleChecker) HasActiveSellerCapability(_ context.Context, _ uuid.UUID) (bool, error) {
	return true, nil
}
func (stubRoleChecker) HasSellerProfile(_ context.Context, _ uuid.UUID) (bool, error) {
	return true, nil
}

// stubAccountStatusChecker allows all accounts.
type stubAccountStatusChecker struct{}

func (stubAccountStatusChecker) EnsureActive(_ context.Context, _ uuid.UUID) error { return nil }
func (stubAccountStatusChecker) GetStatus(_ context.Context, _ uuid.UUID) (string, error) {
	return "active", nil
}
func (stubAccountStatusChecker) IsBanned(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}

func newIntentHarness(t *testing.T) *intentHarness {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	bootstrap := financeapp.NewSystemAccountBootstrapFromPgx(db.NewFromPool(tdb.Pool()))
	_, err := bootstrap.EnsureSystemAccounts(ctx)
	require.NoError(t, err)

	financeSvc := financeapp.NewFinanceService()
	billingSvc := billingapp.NewBillingService(stubRoleChecker{}, stubAccountStatusChecker{})
	cfgSvc := configapp.NewConfigService(configrepo.NewPlatformConfigRepository())

	contractSvc := application.NewPromotionContractService(
		db.NewFromPool(tdb.Pool()),
		financeSvc,
		cfgSvc,
		allowAllGate{},
		deliveryRepoImpl.NewDeliveryRepository(db.NewFromPool(tdb.Pool())),
	)

	intentSvc := application.NewFundingIntentService(contractSvc, billingSvc, nil)

	return &intentHarness{
		tdb:         tdb,
		finance:     financeSvc,
		billing:     billingSvc,
		svc:         intentSvc,
		config:      cfgSvc,
		contractSvc: contractSvc,
	}
}

func (h *intentHarness) seedConfig(t *testing.T, cpm, minDailyBudget int64) {
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

// seedUser inserts a valid user row (required by FK constraint on billing_transactions.payer_id
// and promotion_funding_intents.seller_id).
func (h *intentHarness) seedUser(t *testing.T, seller uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	_, err := h.tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
		ON CONFLICT (id) DO NOTHING
	`, seller, "fb-intent-"+seller.String()[:8], seller.String()+"@test.local")
	require.NoError(t, err)
}

func (h *intentHarness) fundSeller(t *testing.T, seller uuid.UUID, amount int64) {
	t.Helper()
	h.seedUser(t, seller)
	ctx := context.Background()
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return h.finance.RecordPromoteBalanceFunding(ctx, tx, uuid.New(), seller, amount)
	})
	require.NoError(t, err)
}

func (h *intentHarness) balance(t *testing.T, seller uuid.UUID) int64 {
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
// TEST CASE 1: sufficient balance → no payment intent, shortage 0
// ============================================================================

func TestFundingIntent_SufficientBalance(t *testing.T) {
	h := newIntentHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 50_000)

	result, err := h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 30_000,
		DurationDays: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.PaymentRequired, "no payment when balance >= cost")
	assert.Equal(t, int64(0), result.Shortage, "shortage must be 0")
	assert.Equal(t, int64(30_000), result.RequiredCost)
	assert.Equal(t, int64(50_000), result.AvailableFunding)
	assert.Empty(t, result.IntentID, "no intent when no shortage")
	assert.Empty(t, result.BillingID, "no billing when no shortage")

	// Verify balance unchanged
	assert.Equal(t, int64(50_000), h.balance(t, seller))
}

// ============================================================================
// TEST CASE 2: exact balance → no payment intent, shortage 0
// ============================================================================

func TestFundingIntent_ExactBalance(t *testing.T) {
	h := newIntentHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 30_000)

	result, err := h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 30_000,
		DurationDays: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.PaymentRequired, "no payment when balance == cost")
	assert.Equal(t, int64(0), result.Shortage)
	assert.Equal(t, int64(30_000), result.RequiredCost)
	assert.Equal(t, int64(30_000), result.AvailableFunding)
	assert.Empty(t, result.IntentID)
	assert.Empty(t, result.BillingID)
}

// ============================================================================
// TEST CASE 3: insufficient balance → exact shortage payment intent
// ============================================================================

func TestFundingIntent_InsufficientBalance(t *testing.T) {
	h := newIntentHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 10_000)

	result, err := h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 45_000,
		DurationDays: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.PaymentRequired, "payment required when balance < cost")
	assert.Equal(t, int64(35_000), result.Shortage, "shortage = cost - balance = 45k - 10k = 35k")
	assert.Equal(t, int64(45_000), result.RequiredCost)
	assert.Equal(t, int64(10_000), result.AvailableFunding)
	assert.NotEmpty(t, result.IntentID, "intent ID must be present")
	assert.NotEmpty(t, result.BillingID, "billing ID must be present")

	// Verify balance unchanged (no ledger mutation before payment settlement)
	assert.Equal(t, int64(10_000), h.balance(t, seller), "balance must NOT change before payment")

	// Verify funding intent persisted
	ctx := context.Background()
	var intent entity.FundingIntent
	err = h.tdb.Pool().QueryRow(ctx, `
		SELECT id, seller_id, kind, budget_rupiah, duration_days, shortage_amount
		FROM promotion_funding_intents
		WHERE seller_id = $1`, seller,
	).Scan(&intent.ID, &intent.SellerID, &intent.Kind, &intent.BudgetRupiah, &intent.DurationDays, &intent.ShortageAmount)
	require.NoError(t, err)
	assert.Equal(t, int64(35_000), intent.ShortageAmount, "persisted shortage must be exact")
	assert.Equal(t, int64(45_000), intent.BudgetRupiah)

	// Verify billing transaction persisted with correct type and amount
	var billingType string
	var grossAmount int64
	err = h.tdb.Pool().QueryRow(ctx, `
		SELECT type, gross_amount
		FROM billing_transactions
		WHERE id = $1`, result.BillingID,
	).Scan(&billingType, &grossAmount)
	require.NoError(t, err)
	assert.Equal(t, "promote_balance_top_up", billingType, "billing type must be promote_balance_top_up")
	assert.Equal(t, int64(35_000), grossAmount, "billing amount must be exact shortage")
}

// ============================================================================
// TEST CASE 4: zero balance → full cost as shortage
// ============================================================================

func TestFundingIntent_ZeroBalance(t *testing.T) {
	h := newIntentHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.seedUser(t, seller) // No funding, but user must exist for FK

	result, err := h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 15_000,
		DurationDays: 1,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.PaymentRequired)
	assert.Equal(t, int64(15_000), result.Shortage, "shortage = cost when balance = 0")
	assert.Equal(t, int64(15_000), result.RequiredCost)
	assert.Equal(t, int64(0), result.AvailableFunding)
}

// ============================================================================
// TEST CASE 5: insufficient funding does NOT mutate allocation
// ============================================================================

func TestFundingIntent_NoAllocationMutation(t *testing.T) {
	h := newIntentHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 5_000)

	// Count allocation accounts before
	var allocBefore int
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2`,
		finance.AccountPromotionAllocation, seller,
	).Scan(&allocBefore)
	require.NoError(t, err)

	_, err = h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 20_000,
		DurationDays: 2,
	})
	require.NoError(t, err)

	// Verify no allocation account created
	var allocAfter int
	err = h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2`,
		finance.AccountPromotionAllocation, seller,
	).Scan(&allocAfter)
	require.NoError(t, err)
	assert.Equal(t, allocBefore, allocAfter, "must NOT create allocation accounts")

	// Verify no ledger transactions
	var txCount int
	err = h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM ledger_transactions
		WHERE reference_type = 'promotion_allocation'`,
	).Scan(&txCount)
	require.NoError(t, err)
	assert.Equal(t, 0, txCount, "must NOT create ledger transactions")

	// Balance unchanged
	assert.Equal(t, int64(5_000), h.balance(t, seller))
}

// ============================================================================
// TEST CASE 6: sequential idempotent — same params return same intent
// ============================================================================

func TestFundingIntent_Idempotent(t *testing.T) {
	h := newIntentHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 5_000)

	input := application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 40_000,
		DurationDays: 4,
	}

	// First call — creates intent + billing
	result1, err := h.svc.CreateFundingIntent(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, result1.PaymentRequired)
	assert.Equal(t, int64(35_000), result1.Shortage)
	assert.NotEmpty(t, result1.IntentID)
	assert.NotEmpty(t, result1.BillingID)

	// Second call with same params — must return existing intent (idempotent)
	result2, err := h.svc.CreateFundingIntent(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, result2.PaymentRequired)
	assert.Equal(t, result1.IntentID, result2.IntentID, "must return same intent on idempotent call")
	assert.Equal(t, result1.BillingID, result2.BillingID, "must return same billing on idempotent call")
	assert.Equal(t, int64(35_000), result2.Shortage)

	// Verify only ONE billing transaction exists
	var billingCount int
	err = h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM billing_transactions
		WHERE payer_id = $1 AND type = 'promote_balance_top_up'`, seller,
	).Scan(&billingCount)
	require.NoError(t, err)
	assert.Equal(t, 1, billingCount, "must NOT create duplicate billing transactions")
}

// ============================================================================
// TEST CASE 7: concurrent idempotent — race-safe duplicate prevention
// ============================================================================

func TestFundingIntent_ConcurrentIdempotent(t *testing.T) {
	h := newIntentHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 5_000)

	input := application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 40_000,
		DurationDays: 4,
	}

	// Fire N concurrent requests with identical params.
	// The unique index ux_promotion_funding_intents_seller_params ensures
	// at most one intent is created; the rest get the conflict path.
	const concurrency = 10
	results := make([]*application.FundingIntentResult, concurrency)
	errors := make([]error, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errors[idx] = h.svc.CreateFundingIntent(context.Background(), input)
		}(i)
	}
	wg.Wait()

	// All requests must succeed
	for i := 0; i < concurrency; i++ {
		require.NoError(t, errors[i], "concurrent request %d must not error", i)
		require.NotNil(t, results[i], "concurrent request %d must return result", i)
	}

	// ALL results must point to the SAME intent ID
	intentID := results[0].IntentID
	require.NotEmpty(t, intentID, "first result must have intent ID")
	for i := 1; i < concurrency; i++ {
		assert.Equal(t, intentID, results[i].IntentID,
			"concurrent request %d must return same intent ID as request 0", i)
	}

	// ALL results must have the same billing ID
	billingID := results[0].BillingID
	require.NotEmpty(t, billingID, "first result must have billing ID")
	for i := 1; i < concurrency; i++ {
		assert.Equal(t, billingID, results[i].BillingID,
			"concurrent request %d must return same billing ID as request 0", i)
	}

	// Exactly ONE billing transaction must exist
	var billingCount int
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM billing_transactions
		WHERE payer_id = $1 AND type = 'promote_balance_top_up'`, seller,
	).Scan(&billingCount)
	require.NoError(t, err)
	assert.Equal(t, 1, billingCount, "concurrent requests must produce exactly ONE billing transaction")

	// Exactly ONE intent row must exist
	var intentCount int
	err = h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM promotion_funding_intents
		WHERE seller_id = $1`, seller,
	).Scan(&intentCount)
	require.NoError(t, err)
	assert.Equal(t, 1, intentCount, "concurrent requests must produce exactly ONE intent row")
}

// ============================================================================
// TEST CASE 8: budget below minimum → error (same as Create)
// ============================================================================

func TestFundingIntent_BudgetBelowMinimum(t *testing.T) {
	h := newIntentHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 50_000)

	_, err := h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 5_000, // Below 10_000 minimum
		DurationDays: 1,
	})
	require.Error(t, err)

	var budgetErr *application.ErrPromotionBudgetBelowMinimum
	require.ErrorAs(t, err, &budgetErr)
}

// ============================================================================
// TEST CASE 9: invalid inputs → same errors as Create
// ============================================================================

func TestFundingIntent_InvalidInputs(t *testing.T) {
	h := newIntentHarness(t)
	seller := uuid.New()

	// Zero seller
	_, err := h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     uuid.Nil,
		Kind:         entity.KindInternal,
		BudgetRupiah: 10_000,
		DurationDays: 1,
	})
	require.Error(t, err)

	// Invalid kind
	_, err = h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.Kind("bogus"),
		BudgetRupiah: 10_000,
		DurationDays: 1,
	})
	require.ErrorIs(t, err, application.ErrPromotionKindInvalid)

	// Zero budget
	_, err = h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 0,
		DurationDays: 1,
	})
	require.ErrorIs(t, err, application.ErrPromotionBudgetInvalid)

	// Zero duration
	_, err = h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 10_000,
		DurationDays: 0,
	})
	require.ErrorIs(t, err, application.ErrPromotionDurationInvalid)
}
