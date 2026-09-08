//go:build integration

package application_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

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

// ============================================================================
// CANONICAL PROMOTION CONTRACT — REAL-DB AUTHORITY PROOF (PHASE 2)
//
// Proves (contract + allocation atomicity, seller slot DB authority, minimum
// daily budget, immutable CPM snapshot, pause/resume DB-time finish shift,
// exact-once finalization release) against the real PostgreSQL ledger.
// ============================================================================

type allowAllGate struct{}

func (allowAllGate) EnsureCanPromote(context.Context, db.Tx, uuid.UUID) error { return nil }

type contractHarness struct {
	tdb     *testdb.TestDB
	finance *financeapp.FinanceService
	svc     *application.PromotionContractService
}

func newContractHarness(t *testing.T) *contractHarness {
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
	return &contractHarness{tdb: tdb, finance: financeSvc, svc: svc}
}

// seedConfig applies the migration-000064 seed defaults (rows are truncated
// between binaries) and lets a subtest set the CPM / minimum daily budget.
func (h *contractHarness) seedConfig(t *testing.T, cpm, minDaily int64) {
	t.Helper()
	_, err := h.tdb.Pool().Exec(context.Background(), `
		INSERT INTO platform_configs (key, value_numeric, value_text, updated_by, updated_at)
		VALUES
			('promotion_cpm', $1, NULL, NULL, EXTRACT(epoch FROM now())::bigint),
			('promotion_min_daily_budget', $2, NULL, NULL, EXTRACT(epoch FROM now())::bigint),
			('promotion_delivery_enabled', NULL, 'disabled', NULL, EXTRACT(epoch FROM now())::bigint)
		ON CONFLICT (key) DO UPDATE
			SET value_numeric = EXCLUDED.value_numeric,
			    value_text = EXCLUDED.value_text,
			    updated_at = EXTRACT(epoch FROM now())::bigint
	`, cpm, minDaily)
	require.NoError(t, err)
}

func (h *contractHarness) newSeller(t *testing.T, fundRupiah int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	seller := uuid.New()
	_, err := h.tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
		ON CONFLICT (id) DO NOTHING
	`, seller, "fb-contract-"+seller.String()[:8], seller.String()+"@test.local")
	require.NoError(t, err)
	if fundRupiah > 0 {
		fundingID := uuid.New()
		require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
			return h.finance.RecordPromoteBalanceFunding(ctx, tx, fundingID, seller, fundRupiah)
		}))
	}
	return seller
}

func (h *contractHarness) promoteBalance(t *testing.T, seller uuid.UUID) int64 {
	t.Helper()
	var balance int64
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT balance FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2 AND holder_id IS NULL
	`, finance.AccountPromoteBalance, seller).Scan(&balance)
	require.NoError(t, err)
	return balance
}

func (h *contractHarness) allocationBalance(t *testing.T, seller, contractID uuid.UUID) (int64, bool) {
	t.Helper()
	var balance int64
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT balance FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2 AND holder_id = $3
	`, finance.AccountPromotionAllocation, seller, contractID).Scan(&balance)
	if err != nil {
		return 0, false
	}
	return balance, true
}

func (h *contractHarness) platformRevenue(t *testing.T) int64 {
	t.Helper()
	var balance int64
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT balance FROM financial_accounts WHERE account_type = $1 AND user_id IS NULL
	`, finance.AccountPlatformRevenue).Scan(&balance)
	require.NoError(t, err)
	return balance
}

func (h *contractHarness) dbNow(t *testing.T) time.Time {
	t.Helper()
	var now time.Time
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT NOW()`).Scan(&now))
	return now
}

func (h *contractHarness) countContracts(t *testing.T, seller uuid.UUID, kind entity.Kind) int {
	t.Helper()
	var count int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM promotion_contracts WHERE seller_id = $1 AND kind = $2
	`, seller, string(kind)).Scan(&count))
	return count
}

func (h *contractHarness) countAllocationAccounts(t *testing.T, seller uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2
	`, finance.AccountPromotionAllocation, seller).Scan(&count))
	return count
}

func (h *contractHarness) releaseTxCount(t *testing.T, contractID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM ledger_transactions WHERE idempotency_key LIKE $1
	`, "promotion_allocation_release_"+contractID.String()+"%").Scan(&count))
	return count
}

func (h *contractHarness) consumeQI(t *testing.T, seller, contractID uuid.UUID, charge int64) error {
	return h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return h.finance.RecordQualifiedImpression(context.Background(), tx, uuid.New(), contractID, seller, charge)
	})
}

func (h *contractHarness) create(t *testing.T, seller uuid.UUID, kind entity.Kind, budget int64, durationDays int64) (*entity.Contract, error) {
	return h.svc.Create(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         kind,
		BudgetRupiah: budget,
		DurationDays: durationDays,
	})
}

func TestPromotionContractCanonical_RealDB(t *testing.T) {
	h := newContractHarness(t)
	ctx := context.Background()

	t.Run("A_happy_path_contract_created_active_and_allocated", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)

		c, err := h.create(t, seller, entity.KindInternal, 30_000, 3)
		require.NoError(t, err)
		require.Equal(t, entity.StatusActive, c.Status)
		require.Equal(t, int64(30_000), c.BudgetRupiah)
		require.Equal(t, int64(7500), c.CPMRupiah)
		require.Equal(t, entity.KindInternal, c.Kind)

		// planned_finish - planned_start == duration (pacing boundary).
		require.Equal(t, 3*24*time.Hour, c.PlannedFinish.Sub(c.PlannedStart).Round(time.Second))
		require.True(t, c.PlannedStart.After(time.Now().Add(-10*time.Second)), "planned_start must be ~DB now")

		// Allocation ledger balance equals budget; promote balance reduced.
		allocBal, ok := h.allocationBalance(t, seller, c.ID)
		require.True(t, ok)
		require.Equal(t, int64(30_000), allocBal)
		require.Equal(t, int64(70_000), h.promoteBalance(t, seller))
		// The contract row references the holder-scoped allocation account.
		require.Equal(t, c.AllocationAccountID.String(), contractAllocationAccountID(t, h, c.ID))
	})

	t.Run("B_atomic_failure_insufficient_balance_no_contract_no_orphan_no_ledger", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 5_000)

		_, err := h.create(t, seller, entity.KindInternal, 30_000, 3)
		require.Error(t, err)
		require.ErrorIs(t, err, financeapp.ErrPromoteBalanceInsufficient)

		// No contract survives, no allocation account row survives (no orphan),
		// no ledger transaction survives, balance untouched.
		require.Equal(t, 0, h.countContracts(t, seller, entity.KindInternal))
		var allocAccounts int
		require.NoError(t, h.tdb.Pool().QueryRow(ctx, `
			SELECT COUNT(*) FROM financial_accounts
			WHERE account_type = $1 AND user_id = $2
		`, finance.AccountPromotionAllocation, seller).Scan(&allocAccounts))
		require.Equal(t, 0, allocAccounts, "no orphan allocation account may survive a failed create")
		require.Equal(t, int64(5_000), h.promoteBalance(t, seller), "failed create must not move money")
	})

	t.Run("C_minimum_daily_budget", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000) // configured minimum daily budget Rp10.000
		cases := []struct {
			name      string
			budget    int64
			days      int64
			wantError bool
		}{
			{"10k_1day_pass", 10_000, 1, false},
			{"10k_2day_reject", 10_000, 2, true},
			{"30k_3day_pass", 30_000, 3, false},
			{"20k_3day_reject", 20_000, 3, true},
			{"30k_30day_reject", 30_000, 30, true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				seller := h.newSeller(t, 1_000_000)
				_, err := h.create(t, seller, entity.KindInternal, tc.budget, tc.days)
				if tc.wantError {
					require.Error(t, err)
					var belowMin *application.ErrPromotionBudgetBelowMinimum
					require.ErrorAs(t, err, &belowMin, "must be rejected by the minimum daily budget rule")
					require.Equal(t, tc.budget, belowMin.Budget)
					require.Equal(t, int64(10_000)*tc.days, belowMin.Required)
					require.Equal(t, 0, h.countContracts(t, seller, entity.KindInternal))
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("C2_minimum_budget_rejection_no_partial_state", func(t *testing.T) {
		// Negative proof C: duration_days=2 / budget 10.000 must fail with
		// Required=20.000 and leave NO partial contract, NO partial
		// allocation account, and NO ledger movement.
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 1_000_000)

		_, err := h.create(t, seller, entity.KindInternal, 10_000, 2)
		require.Error(t, err)
		var belowMin *application.ErrPromotionBudgetBelowMinimum
		require.ErrorAs(t, err, &belowMin, "must be rejected by the minimum daily budget rule")
		require.Equal(t, int64(10_000), belowMin.Budget)
		require.Equal(t, int64(20_000), belowMin.Required)

		require.Equal(t, 0, h.countContracts(t, seller, entity.KindInternal), "no partial contract")
		require.Equal(t, 0, h.countAllocationAccounts(t, seller), "no partial allocation account")
		require.Equal(t, int64(1_000_000), h.promoteBalance(t, seller), "no partial ledger movement")
	})

	t.Run("D_immutable_cpm_snapshot", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 1_000_000)

		first, err := h.create(t, seller, entity.KindInternal, 30_000, 3)
		require.NoError(t, err)
		require.Equal(t, int64(7500), first.CPMRupiah)

		// Admin changes CPM -> future contracts use the new price...
		h.seedConfig(t, 9999, 10_000)
		second, err := h.create(t, seller, entity.KindExternal, 30_000, 3)
		require.NoError(t, err)
		require.Equal(t, int64(9999), second.CPMRupiah)

		// ...but the existing contract keeps its immutable snapshot.
		got, err := h.svc.Get(ctx, seller, first.ID)
		require.NoError(t, err)
		require.Equal(t, int64(7500), got.CPMRupiah, "existing contract must never be repriced")
		require.Equal(t, int64(7500), first.CPMRupiah)

		h.seedConfig(t, 7500, 10_000) // restore for other subtests
	})

	t.Run("E_seller_concurrency_db_final_authority", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 1_000_000)

		// Concurrent internal creates: exactly one wins.
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := h.create(t, seller, entity.KindInternal, 30_000, 3)
				results <- err
			}()
		}
		wg.Wait()
		close(results)
		var wins, slotErrors int
		for err := range results {
			switch {
			case err == nil:
				wins++
			default:
				var occupied *application.ErrPromotionSellerSlotOccupied
				if errors.As(err, &occupied) {
					slotErrors++
				} else {
					t.Fatalf("unexpected concurrent create error: %v", err)
				}
			}
		}
		require.Equal(t, 1, wins, "exactly one concurrent internal create must win")
		require.Equal(t, 1, slotErrors, "the loser must receive ErrPromotionSellerSlotOccupied")
		require.Equal(t, 1, h.countContracts(t, seller, entity.KindInternal))

		// Internal + external may coexist.
		_, err := h.create(t, seller, entity.KindExternal, 30_000, 3)
		require.NoError(t, err)

		// A second internal while the first is active must fail.
		_, err = h.create(t, seller, entity.KindInternal, 30_000, 3)
		var occupied *application.ErrPromotionSellerSlotOccupied
		require.ErrorAs(t, err, &occupied)

		// Paused still counts: pause the internal, retry -> still blocked.
		internalID := firstContractID(t, h, seller, entity.KindInternal)
		require.NoError(t, h.svc.Pause(ctx, application.PausePromotionInput{SellerID: seller, ContractID: internalID}))
		_, err = h.create(t, seller, entity.KindInternal, 30_000, 3)
		require.ErrorAs(t, err, &occupied, "paused contracts still occupy the seller slot")

		// Finalized frees the slot.
		require.NoError(t, h.svc.Finalize(ctx, application.FinalizePromotionInput{SellerID: seller, ContractID: internalID}))
		_, err = h.create(t, seller, entity.KindInternal, 30_000, 3)
		require.NoError(t, err, "finalized contract must free the seller slot")
	})

	t.Run("F_pause_resume_planned_finish_shift", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c, err := h.create(t, seller, entity.KindInternal, 30_000, 3)
		require.NoError(t, err)
		originalFinish := c.PlannedFinish

		// Pause (DB time) must NOT shift planned_finish by itself.
		require.NoError(t, h.svc.Pause(ctx, application.PausePromotionInput{SellerID: seller, ContractID: c.ID}))
		paused := h.getContract(t, seller, c.ID)
		require.Equal(t, entity.StatusPaused, paused.Status)
		require.NotNil(t, paused.PausedAt)
		require.Equal(t, originalFinish, paused.PlannedFinish, "pause alone must not shift planned_finish")

		// Measured explicit pause interval (DB time based).
		time.Sleep(2 * time.Second)
		pausedAt := *paused.PausedAt
		resumeAt := h.dbNow(t)

		require.NoError(t, h.svc.Resume(ctx, application.ResumePromotionInput{SellerID: seller, ContractID: c.ID}))
		resumed := h.getContract(t, seller, c.ID)
		require.Equal(t, entity.StatusActive, resumed.Status)
		require.Nil(t, resumed.PausedAt)

		expectedShift := resumeAt.Sub(pausedAt)
		actualShift := resumed.PlannedFinish.Sub(originalFinish)
		require.GreaterOrEqual(t, actualShift, expectedShift,
			"finish must shift by at least the measured explicit pause duration")
		require.LessOrEqual(t, actualShift-expectedShift, time.Second,
			"finish shift must equal the explicit pause duration (DB time), not wall-clock guesses")
		require.GreaterOrEqual(t, actualShift, 2*time.Second)

		// Repeated pause/resume accumulates on planned_finish.
		require.NoError(t, h.svc.Pause(ctx, application.PausePromotionInput{SellerID: seller, ContractID: c.ID}))
		time.Sleep(2 * time.Second)
		require.NoError(t, h.svc.Resume(ctx, application.ResumePromotionInput{SellerID: seller, ContractID: c.ID}))
		resumed2 := h.getContract(t, seller, c.ID)
		secondShift := resumed2.PlannedFinish.Sub(resumed.PlannedFinish)
		require.GreaterOrEqual(t, secondShift, 2*time.Second)
		require.Greater(t, resumed2.PlannedFinish.Sub(originalFinish), 4*time.Second,
			"repeated explicit pauses must accumulate on planned_finish")

		// Lifecycle misuse is rejected: Resume on an active contract fails, and
		// pausing an already-paused contract fails.
		require.ErrorIs(t, h.svc.Resume(ctx, application.ResumePromotionInput{SellerID: seller, ContractID: c.ID}), application.ErrPromotionResumeNotAllowed)
		require.NoError(t, h.svc.Pause(ctx, application.PausePromotionInput{SellerID: seller, ContractID: c.ID}), "an active contract may pause again")
		require.ErrorIs(t, h.svc.Pause(ctx, application.PausePromotionInput{SellerID: seller, ContractID: c.ID}), application.ErrPromotionAlreadyPaused)
	})

	t.Run("G_stop_finalization_exact_release", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c, err := h.create(t, seller, entity.KindInternal, 50_000, 3)
		require.NoError(t, err)

		// Consume 15 Rupiah via two Qualified Impressions (CPM 7500: 7 + 8).
		require.NoError(t, h.consumeQI(t, seller, c.ID, 7))
		require.NoError(t, h.consumeQI(t, seller, c.ID, 8))
		allocBefore, _ := h.allocationBalance(t, seller, c.ID)
		require.Equal(t, int64(50_000-15), allocBefore)
		promoteBefore := h.promoteBalance(t, seller)
		revenueBefore := h.platformRevenue(t) // 15 from the QIs above

		// Finalize: release amount comes from the ACTUAL allocation balance.
		require.NoError(t, h.svc.Finalize(ctx, application.FinalizePromotionInput{SellerID: seller, ContractID: c.ID}))

		finalized := h.getContract(t, seller, c.ID)
		require.Equal(t, entity.StatusFinalized, finalized.Status)
		require.NotNil(t, finalized.FinalizedAt)
		allocAfter, _ := h.allocationBalance(t, seller, c.ID)
		require.Equal(t, int64(0), allocAfter, "allocation must be zero after release")
		require.Equal(t, promoteBefore+allocBefore, h.promoteBalance(t, seller),
			"remaining allocation credited back to PROMOTE_BALANCE")
		require.Equal(t, revenueBefore, h.platformRevenue(t),
			"finalization release must never book platform revenue")
		require.Equal(t, 1, h.releaseTxCount(t, c.ID))

		// Duplicate finalization: no double release.
		err = h.svc.Finalize(ctx, application.FinalizePromotionInput{SellerID: seller, ContractID: c.ID})
		require.ErrorIs(t, err, application.ErrPromotionAlreadyFinalized)
		require.Equal(t, promoteBefore+allocBefore, h.promoteBalance(t, seller))
		require.Equal(t, 1, h.releaseTxCount(t, c.ID))
	})

	t.Run("H_planned_finish_non_seller_completion_no_shift", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c, err := h.create(t, seller, entity.KindInternal, 30_000, 3)
		require.NoError(t, err)
		originalFinish := c.PlannedFinish

		// Finalizing with remaining budget (e.g. delivery-independent planned
		// completion) never shifts planned_finish — it only releases. The
		// only code path that mutates planned_finish is Resume.
		require.NoError(t, h.svc.Finalize(ctx, application.FinalizePromotionInput{SellerID: seller, ContractID: c.ID}))
		got, err := h.svc.Get(ctx, seller, c.ID)
		require.NoError(t, err)
		require.Equal(t, entity.StatusFinalized, got.Status)
		require.Equal(t, originalFinish, got.PlannedFinish, "planned_finish must be untouched by finalization")
		require.Equal(t, int64(100_000), h.promoteBalance(t, seller),
			"the full under-delivered budget returns to Promote Balance — never forfeited by time alone")
	})

	t.Run("J_migration_schema_present", func(t *testing.T) {
		// promotion_contracts table exists with enum-backed kind/status.
		var tableCount int
		require.NoError(t, h.tdb.Pool().QueryRow(ctx, `
			SELECT COUNT(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'promotion_contracts'
		`).Scan(&tableCount))
		require.Equal(t, 1, tableCount)

		var statuses []string
		rows, err := h.tdb.Pool().Query(ctx, `
			SELECT enumlabel FROM pg_enum
			JOIN pg_type ON pg_type.oid = pg_enum.enumtypid
			WHERE pg_type.typname = 'promotion_contract_status_enum'
			ORDER BY enumsortorder
		`)
		require.NoError(t, err)
		defer rows.Close()
		for rows.Next() {
			var s string
			require.NoError(t, rows.Scan(&s))
			statuses = append(statuses, s)
		}
		require.NoError(t, rows.Err())
		require.Equal(t, []string{"prepared", "active", "paused", "finalizing", "finalized"}, statuses)

		var kinds int
		require.NoError(t, h.tdb.Pool().QueryRow(ctx, `
			SELECT COUNT(*) FROM pg_enum
			JOIN pg_type ON pg_type.oid = pg_enum.enumtypid
			WHERE pg_type.typname = 'promotion_contract_kind_enum'
		`).Scan(&kinds))
		require.Equal(t, 2, kinds)

		// Seller-slot partial unique indexes exist (DB final authority).
		for _, name := range []string{
			"ux_promotion_contracts_one_nonfinalized_internal_per_seller",
			"ux_promotion_contracts_one_nonfinalized_external_per_seller",
		} {
			var idxCount int
			require.NoError(t, h.tdb.Pool().QueryRow(ctx, `
				SELECT COUNT(*) FROM pg_indexes WHERE indexname = $1
			`, name).Scan(&idxCount))
			require.Equal(t, 1, idxCount, "missing unique index %s", name)
		}

		// planned_finish > planned_start remains DB-enforced (schema CHECK).
		var checkCount int
		require.NoError(t, h.tdb.Pool().QueryRow(ctx, `
			SELECT COUNT(*) FROM pg_constraint
			WHERE conname = 'promotion_contracts_finish_after_start'
			  AND conrelid = 'promotion_contracts'::regclass
		`).Scan(&checkCount))
		require.Equal(t, 1, checkCount, "planned_finish > planned_start CHECK must remain enforced")
	})

	t.Run("K_no_arbitrary_3650_product_cap", func(t *testing.T) {
		// Negative proof E: the invented 3650-day product cap is gone. The
		// only remaining bound is the technical time.Duration overflow guard
		// (~292 years). A SAFE duration far above 3650 days must be accepted;
		// rejection here would mean an arbitrary product maximum survived.
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 1_000_000_000)
		const days = 4000 // > 3650 (~11 years), safe for time.Duration + timestamptz

		c, err := h.create(t, seller, entity.KindInternal, 10_000*days, days)
		require.NoError(t, err, "duration above 3650 days must not be rejected by an arbitrary cap")
		require.Equal(t, int64(10_000*days), c.BudgetRupiah)
		require.Equal(t, days*24*time.Hour, c.PlannedFinish.Sub(c.PlannedStart).Round(time.Second),
			"the server-derived finish must still follow duration x 24h")
	})

	t.Run("L_duration_overflow_fail_closed", func(t *testing.T) {
		// Negative proof F (time branch): duration_days one day past the
		// technical time.Duration bound must be rejected with
		// ErrPromotionDurationInvalid BEFORE any contract, allocation
		// account, or ledger movement — the guard is a technical overflow
		// constraint, and overflow can never wrap into a persisted contract.
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 1_000_000)

		const technicalBound = math.MaxInt64 / int64(24*time.Hour) // 106,751 days
		_, err := h.create(t, seller, entity.KindInternal, 30_000, technicalBound+1)
		require.ErrorIs(t, err, application.ErrPromotionDurationInvalid)
		require.Equal(t, 0, h.countContracts(t, seller, entity.KindInternal), "no contract may survive")
		require.Equal(t, 0, h.countAllocationAccounts(t, seller), "no allocation account may survive")
		require.Equal(t, int64(1_000_000), h.promoteBalance(t, seller), "no ledger movement may survive")
	})

	t.Run("M_min_budget_multiplication_overflow_fail_closed", func(t *testing.T) {
		// Negative proof F (budget branch): min_daily_budget x duration_days
		// overflowing int64 must fail closed with the requirement reported as
		// unrepresentable (math.MaxInt64) — before any contract, allocation
		// account, or ledger movement.
		h.seedConfig(t, 7500, math.MaxInt64/2) // admin misconfigures an astronomically large minimum
		seller := h.newSeller(t, 1_000_000)

		_, err := h.create(t, seller, entity.KindInternal, 30_000, 3)
		require.Error(t, err)
		var belowMin *application.ErrPromotionBudgetBelowMinimum
		require.ErrorAs(t, err, &belowMin, "overflowing requirement must be rejected by the minimum-budget rule")
		require.Equal(t, int64(30_000), belowMin.Budget)
		require.Equal(t, int64(math.MaxInt64), belowMin.Required, "unrepresentable requirement must be surfaced, never wrapped")
		require.Equal(t, 0, h.countContracts(t, seller, entity.KindInternal), "no contract may survive")
		require.Equal(t, 0, h.countAllocationAccounts(t, seller), "no allocation account may survive")
		require.Equal(t, int64(1_000_000), h.promoteBalance(t, seller), "no ledger movement may survive")
	})

	t.Run("N_one_day_creation_exact_24h_finish", func(t *testing.T) {
		// Negative proof A + D: duration_days=1 / budget 10.000 passes, and
		// the persisted contract's planned_finish is exactly planned_start +
		// 24h. Combined with the structural guard
		// (TestCreatePromotionInput_HasSingleDurationAuthority) this proves
		// the system cannot create duration_days=1 with a 30-day finish: no
		// canonical creation input can carry a finish, and the service
		// derives the finish from the SAME DurationDays used for the
		// minimum-budget branch.
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)

		c, err := h.create(t, seller, entity.KindInternal, 10_000, 1)
		require.NoError(t, err)
		require.Equal(t, 24*time.Hour, c.PlannedFinish.Sub(c.PlannedStart).Round(time.Second))

		// Read back from the DB: the persisted row agrees.
		got := h.getContract(t, seller, c.ID)
		require.Equal(t, 24*time.Hour, got.PlannedFinish.Sub(got.PlannedStart).Round(time.Second),
			"persisted planned_finish must equal planned_start + 24h")
	})
}

// --- helpers ---------------------------------------------------------------

func (h *contractHarness) getContract(t *testing.T, seller, contractID uuid.UUID) *entity.Contract {
	t.Helper()
	c, err := h.svc.Get(context.Background(), seller, contractID)
	require.NoError(t, err)
	return c
}

func contractAllocationAccountID(t *testing.T, h *contractHarness, contractID uuid.UUID) string {
	t.Helper()
	var id string
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(),
		`SELECT allocation_account_id::text FROM promotion_contracts WHERE id = $1`, contractID).Scan(&id))
	return id
}

func firstContractID(t *testing.T, h *contractHarness, seller uuid.UUID, kind entity.Kind) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(),
		`SELECT id FROM promotion_contracts WHERE seller_id = $1 AND kind = $2 ORDER BY created_at ASC LIMIT 1`,
		seller, string(kind)).Scan(&id))
	return id
}
