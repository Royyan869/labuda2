//go:build integration

// PHASE 4A — CANONICAL CONTRACT RUNTIME COMPOSITION (SERVICE LEVEL)
//
// Proves against the real PostgreSQL ledger + the REAL seller-capability
// authority (auth.RoleCheckerDB.HasActiveSellerCapability) that the canonical
// PromotionContractService is production-truthful for Create / List / Get /
// Pause / Resume / Finalize, with:
//   - NEGATIVE 1: no contract outcome without financial allocation truth
//   - NEGATIVE 2: no cross-owner lifecycle mutation
//   - NEGATIVE 3: no seller-eligibility bypass (real authority, no mocks)
//   - NEGATIVE 4: no legacy money / state effect
//   - NEGATIVE 5: no delivery activation (no ticket, no QI, no QI revenue)
package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	financeapp "github.com/labuda/backend/internal/finance/application"
	"github.com/labuda/backend/internal/identity/auth"
	configapp "github.com/labuda/backend/internal/platform/config/application"
	configrepo "github.com/labuda/backend/internal/platform/config/infrastructure/repository"
	contractapp "github.com/labuda/backend/internal/pricing/promotion/contract/application"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	deliveryRepoImpl "github.com/labuda/backend/internal/pricing/promotion/delivery/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

type phase4AHarness struct {
	tdb         *testdb.TestDB
	finance     *financeapp.FinanceService
	contracts   *contractapp.PromotionContractService
	roleChecker *auth.RoleCheckerDB
}

func newPhase4AHarness(t *testing.T) *phase4AHarness {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	_, err := financeapp.NewSystemAccountBootstrapFromPgx(db.NewFromPool(tdb.Pool())).EnsureSystemAccounts(ctx)
	require.NoError(t, err)

	// Seed the canonical promotion pricing config (idempotent upsert).
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO platform_configs (key, value_numeric, value_text, updated_by, updated_at)
		VALUES
			('promotion_cpm', 7500, NULL, NULL, EXTRACT(epoch FROM now())::bigint),
			('promotion_min_daily_budget', 10000, NULL, NULL, EXTRACT(epoch FROM now())::bigint),
			('promotion_delivery_enabled', NULL, 'disabled', NULL, EXTRACT(epoch FROM now())::bigint)
		ON CONFLICT (key) DO UPDATE
			SET value_numeric = EXCLUDED.value_numeric,
			    value_text = EXCLUDED.value_text,
			    updated_at = EXTRACT(epoch FROM now())::bigint
	`)
	require.NoError(t, err)

	financeSvc := financeapp.NewFinanceService()
	cfgSvc := configapp.NewConfigService(configrepo.NewPlatformConfigRepository())
	deliveryRepo := deliveryRepoImpl.NewDeliveryRepository(db.NewFromPool(tdb.Pool()))
	roleChecker := auth.NewRoleCheckerDB(db.NewFromPool(tdb.Pool()), nil)
	gate := contractapp.NewRoleCheckerSellerEligibilityGate(roleChecker)
	contractSvc := contractapp.NewPromotionContractService(
		db.NewFromPool(tdb.Pool()),
		financeSvc,
		cfgSvc,
		gate,
		deliveryRepo,
	)
	return &phase4AHarness{
		tdb:         tdb,
		finance:     financeSvc,
		contracts:   contractSvc,
		roleChecker: roleChecker,
	}
}

// newSeller inserts a canonical user. Eligible sellers also get a seller
// profile and an active subscription so the REAL RoleCheckerDB gate passes;
// ineligible sellers get neither.
func (h *phase4AHarness) newSeller(t *testing.T, eligible bool, fundRupiah int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	seller := uuid.New()
	_, err := h.tdb.Pool().Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, account_status, email_verified_at, created_at, updated_at, role
		)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), NOW(), 'user')
	`, seller, "fb-p4a-"+seller.String()[:8], seller.String()+"@p4a.local")
	require.NoError(t, err)

	if eligible {
		_, err = h.tdb.Pool().Exec(ctx, `
			INSERT INTO seller_profiles (id, user_id, store_name, tier, status, created_at, updated_at)
			VALUES ($1, $2, 'P4A Store', 'basic', 'active', NOW(), NOW())
		`, uuid.New(), seller)
		require.NoError(t, err)

		now := time.Now().UTC()
		_, err = h.tdb.Pool().Exec(ctx, `
			INSERT INTO seller_subscriptions (
				id, user_id, status, started_at, expires_at,
				duration_days, amount_paid, currency, payment_id, created_at, updated_at
			)
			VALUES ($1, $2, 'active', $3, $4, 365, 0, 'IDR', $5, NOW(), NOW())
		`, uuid.New(), seller, now.Add(-24*time.Hour), now.Add(24*time.Hour), uuid.New())
		require.NoError(t, err)
	}

	if fundRupiah > 0 {
		require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
			return h.finance.RecordPromoteBalanceFunding(ctx, tx, uuid.New(), seller, fundRupiah)
		}))
	}
	return seller
}

// --- read helpers -----------------------------------------------------------

func (h *phase4AHarness) contractRowCount(t *testing.T) int64 {
	t.Helper()
	var n int64
	err := h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM promotion_contracts`).Scan(&n)
	require.NoError(t, err)
	return n
}

func (h *phase4AHarness) promoteBalance(t *testing.T, seller uuid.UUID) int64 {
	t.Helper()
	var balance int64
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT balance FROM financial_accounts
		WHERE user_id = $1 AND account_type = 'PROMOTE_BALANCE'
	`, seller).Scan(&balance)
	require.NoError(t, err)
	return balance
}

func (h *phase4AHarness) platformRevenue(t *testing.T) int64 {
	t.Helper()
	var total int64
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COALESCE(SUM(balance), 0) FROM financial_accounts
		WHERE user_id IS NULL AND account_type = 'PLATFORM_REVENUE'
	`).Scan(&total)
	require.NoError(t, err)
	return total
}

func (h *phase4AHarness) countRows(t *testing.T, table string) int64 {
	t.Helper()
	var n int64
	err := h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM `+table).Scan(&n)
	require.NoError(t, err)
	return n
}

func (h *phase4AHarness) promotionQiLedgerCount(t *testing.T) int64 {
	t.Helper()
	var n int64
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM ledger_transactions WHERE reference_type = 'promotion_qi'
	`).Scan(&n)
	require.NoError(t, err)
	return n
}

// ============================================================================
// MAIN PROOF — full canonical lifecycle under the REAL eligibility authority
// ============================================================================

func TestPhase4A_ContractLifecycleRealAuthority_RealDB(t *testing.T) {
	h := newPhase4AHarness(t)
	ctx := context.Background()

	const funded = int64(100_000)
	const budget = int64(50_000)
	sellerA := h.newSeller(t, true, funded)

	// Create: eligible seller, funded balance.
	c, err := h.contracts.Create(ctx, contractapp.CreatePromotionInput{
		SellerID:     sellerA,
		Kind:         entity.KindInternal,
		BudgetRupiah: budget,
		DurationDays: 3,
	})
	require.NoError(t, err)
	require.Equal(t, entity.StatusActive, c.Status)
	require.Equal(t, int64(7500), c.CPMRupiah, "immutable pricing snapshot")
	require.Equal(t, int64(1), h.contractRowCount(t))

	// Financial allocation truth: PROMOTE_BALANCE debited exactly the budget,
	// PROMOTION_ALLOCATION credited exactly the budget.
	require.Equal(t, funded-budget, h.promoteBalance(t, sellerA), "promote balance must drop by the exact allocation")
	var allocBalance int64
	err = h.tdb.Pool().QueryRow(ctx, `SELECT balance FROM financial_accounts WHERE id = $1`, c.AllocationAccountID).Scan(&allocBalance)
	require.NoError(t, err)
	require.Equal(t, budget, allocBalance, "allocation account must hold the exact budget")

	// NEGATIVE 1 — no contract without allocation truth: a create whose
	// allocation would exceed the remaining balance must fail atomically.
	_, err = h.contracts.Create(ctx, contractapp.CreatePromotionInput{
		SellerID:     sellerA,
		Kind:         entity.KindExternal, // separate slot; failure must be financial, not slot
		BudgetRupiah: funded,              // > remaining 50k
		DurationDays: 1,
	})
	require.ErrorIs(t, err, financeapp.ErrPromoteBalanceInsufficient)
	require.Equal(t, int64(1), h.contractRowCount(t), "failed create must not persist a contract")
	require.Equal(t, funded-budget, h.promoteBalance(t, sellerA), "failed create must not move money")

	// List (owner only sees own contracts) + cross-owner read denied.
	listA, err := h.contracts.List(ctx, sellerA)
	require.NoError(t, err)
	require.Len(t, listA, 1)
	require.Equal(t, c.ID, listA[0].ID)

	sellerB := h.newSeller(t, true, funded)
	listB, err := h.contracts.List(ctx, sellerB)
	require.NoError(t, err)
	require.Len(t, listB, 0, "seller B must never see seller A contracts")

	// NEGATIVE 2 — cross-owner lifecycle mutation denied.
	_, err = h.contracts.Get(ctx, sellerB, c.ID)
	require.ErrorIs(t, err, contractapp.ErrPromotionContractNotOwned)
	require.ErrorIs(t, h.contracts.Pause(ctx, contractapp.PausePromotionInput{SellerID: sellerB, ContractID: c.ID}), contractapp.ErrPromotionContractNotOwned)
	require.ErrorIs(t, h.contracts.Resume(ctx, contractapp.ResumePromotionInput{SellerID: sellerB, ContractID: c.ID}), contractapp.ErrPromotionContractNotOwned)
	require.ErrorIs(t, h.contracts.Finalize(ctx, contractapp.FinalizePromotionInput{SellerID: sellerB, ContractID: c.ID}), contractapp.ErrPromotionContractNotOwned)

	// Pause does NOT shift planned_finish.
	before := c.PlannedFinish
	require.NoError(t, h.contracts.Pause(ctx, contractapp.PausePromotionInput{SellerID: sellerA, ContractID: c.ID}))
	paused, err := h.contracts.Get(ctx, sellerA, c.ID)
	require.NoError(t, err)
	require.Equal(t, entity.StatusPaused, paused.Status)
	require.Equal(t, before, paused.PlannedFinish, "pause alone must not shift planned finish")

	// Resume shifts planned_finish by the measured pause duration.
	time.Sleep(1100 * time.Millisecond) // measurable DB-time pause
	require.NoError(t, h.contracts.Resume(ctx, contractapp.ResumePromotionInput{SellerID: sellerA, ContractID: c.ID}))
	resumed, err := h.contracts.Get(ctx, sellerA, c.ID)
	require.NoError(t, err)
	require.Equal(t, entity.StatusActive, resumed.Status)
	require.True(t, resumed.PlannedFinish.After(paused.PlannedFinish), "resume must extend planned finish by the pause duration")

	// Finalize: exact remainder (full allocation — no QI ever charged) released.
	require.NoError(t, h.contracts.Finalize(ctx, contractapp.FinalizePromotionInput{SellerID: sellerA, ContractID: c.ID}))
	finalized, err := h.contracts.Get(ctx, sellerA, c.ID)
	require.NoError(t, err)
	require.Equal(t, entity.StatusFinalized, finalized.Status)
	require.Equal(t, funded, h.promoteBalance(t, sellerA), "full unused allocation must return to promote balance")

	// NEGATIVE 5 — no delivery activation by any contract operation.
	require.Zero(t, h.countRows(t, "promotion_delivery_tickets"), "no ticket may exist")
	require.Zero(t, h.countRows(t, "promotion_qualified_impressions"), "no QI may exist")
	require.Zero(t, h.promotionQiLedgerCount(t), "no promotion_qi ledger transaction")
	require.Zero(t, h.platformRevenue(t), "no platform revenue from QI")

	// NEGATIVE 4 — no legacy promotion state or money effect.
	require.Zero(t, h.countRows(t, "promotion_ownerships"), "no legacy ownership write")
	require.Zero(t, h.countRows(t, "promotion_instances"), "no legacy instance write")
	require.Zero(t, h.countRows(t, "promotion_packages"), "no legacy package write")
}

// ============================================================================
// NEGATIVE 3 — ineligible seller cannot create (REAL authority, not a mock)
// ============================================================================

func TestPhase4A_IneligibleSellerCannotCreate_RealDB(t *testing.T) {
	h := newPhase4AHarness(t)
	ctx := context.Background()

	// Ineligible: no seller profile, no active subscription.
	ghost := h.newSeller(t, false, 0)

	_, err := h.contracts.Create(ctx, contractapp.CreatePromotionInput{
		SellerID:     ghost,
		Kind:         entity.KindInternal,
		BudgetRupiah: 50_000,
		DurationDays: 1,
	})
	require.ErrorIs(t, err, auth.ErrMarketAuthorityRequired, "ineligible seller must be rejected by the canonical authority")
	require.Zero(t, h.contractRowCount(t), "no contract may be created by an ineligible seller")
}
