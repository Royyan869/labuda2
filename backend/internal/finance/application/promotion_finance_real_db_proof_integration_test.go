//go:build integration

package application

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	repoimpl "github.com/labuda/backend/internal/finance/infrastructure/repository"
	ledgerintf "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/internal/finance/verifier"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// PROMOTION FINANCIAL FOUNDATION — REAL-DB AUTHORITY PROOF
//
// Proves the canonical promotion financial graph against the real PostgreSQL
// ledger (single immutable authority):
//
//	funding    BANK_SETTLEMENT        -> PROMOTE_BALANCE[seller]
//	allocation PROMOTE_BALANCE        -> PROMOTION_ALLOCATION[seller, promo]
//	QI charge  PROMOTION_ALLOCATION   -> PLATFORM_REVENUE   (only QI books
//	                                                         promotion revenue)
//	release    PROMOTION_ALLOCATION   -> PROMOTE_BALANCE    (unused funds)
//
// Required proofs A-H from the Phase 1 scope:
//
//	A. funding credits PROMOTE_BALANCE, balanced ledger, duplicate key no-op
//	B. allocation moves budget Promote Balance -> allocation
//	C. insufficient Promote Balance rejects with NO partial mutation
//	D. QI consumption allocation -> platform revenue, balanced
//	E. duplicate consumption key no-op (sequential + concurrent)
//	F. release returns remaining once; duplicate release no-op
//	G. no path can make PROMOTE_BALANCE / PROMOTION_ALLOCATION negative
//	H. existing finance surface still passes (strict verifier full run)
// ============================================================================

type promotionFinanceHarness struct {
	tdb    *testdb.TestDB
	svc    *FinanceService
	repo   *repoimpl.LedgerRepository
	seller uuid.UUID
}

func newPromotionFinanceHarness(t *testing.T) *promotionFinanceHarness {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// System accounts (BANK_SETTLEMENT reserve, PLATFORM_REVENUE, ...) are
	// created at server boot, not by migrations — bootstrap them like the
	// production boot path does.
	bootstrap := NewSystemAccountBootstrapFromPgx(db.NewFromPool(tdb.Pool()))
	_, err := bootstrap.EnsureSystemAccounts(ctx)
	require.NoError(t, err)

	seller := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
			VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
			ON CONFLICT (id) DO NOTHING
		`, seller, "fb-promo-"+seller.String()[:8], seller.String()+"@test.local")
		return err
	}))

	return &promotionFinanceHarness{
		tdb:    tdb,
		svc:    NewFinanceService(),
		repo:   repoimpl.NewLedgerRepository(),
		seller: seller,
	}
}

// balanceByScope reads a financial_accounts balance by account type, owner and
// holder scope. A nil holder selects the plain user/system account.
func (h *promotionFinanceHarness) balanceByScope(
	t *testing.T,
	ctx context.Context,
	accountType string,
	userID uuid.UUID,
	holder *uuid.UUID,
) (int64, bool) {
	t.Helper()
	var balance int64
	query := `
		SELECT balance FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2`
	args := []interface{}{accountType, userID}
	if holder == nil {
		query += ` AND holder_id IS NULL`
	} else {
		query += ` AND holder_id = $3`
		args = append(args, *holder)
	}
	err := h.tdb.Pool().QueryRow(ctx, query, args...).Scan(&balance)
	if err != nil {
		return 0, false
	}
	return balance, true
}

func (h *promotionFinanceHarness) promoteBalance(t *testing.T, ctx context.Context) (int64, bool) {
	return h.balanceByScope(t, ctx, finance.AccountPromoteBalance, h.seller, nil)
}

func (h *promotionFinanceHarness) allocationBalance(t *testing.T, ctx context.Context, promotionID uuid.UUID) (int64, bool) {
	return h.balanceByScope(t, ctx, finance.AccountPromotionAllocation, h.seller, &promotionID)
}

// allocationAccountID returns the PROMOTION_ALLOCATION financial_accounts id
// for a promotion (user + holder scoped).
func (h *promotionFinanceHarness) allocationAccountID(t *testing.T, ctx context.Context, promotionID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, h.tdb.Pool().QueryRow(ctx, `
		SELECT id FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2 AND holder_id = $3
	`, finance.AccountPromotionAllocation, h.seller, promotionID).Scan(&id))
	return id
}

// seedCanonicalQI creates the canonical Phase 3 aggregate rows (migration
// 000065 / 000066 shape) that the strict verifier's Qualified Impression
// Reconciliation requires: one promotion_contracts row (created once per
// promotion, idempotent), one promotion_delivery_tickets row, and one
// promotion_qualified_impressions row whose charge matches the contract's
// immutable CPM snapshot. The finance proof charges the SAME qi id through
// RecordQualifiedImpression afterwards, exactly like the delivery boundary
// does (QI row + ledger charge in the canonical flow).
func (h *promotionFinanceHarness) seedCanonicalQI(t *testing.T, ctx context.Context, promotionID, allocationID, qiID uuid.UUID, sequenceN, charge int64) {
	t.Helper()
	// Contract row (immutable CPM snapshot 7500 -> charge(1)=7, charge(2)=8).
	_, err := h.tdb.Pool().Exec(ctx, `
		INSERT INTO promotion_contracts (
			id, seller_id, kind, status, budget_rupiah, cpm_rupiah,
			planned_start, planned_finish, allocation_account_id, created_at, updated_at
		) VALUES ($1, $2, 'internal', 'active', 30000, 7500, NOW(), NOW() + INTERVAL '3 days', $3, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, promotionID, h.seller, allocationID)
	require.NoError(t, err)

	ticketID := uuid.New()
	_, err = h.tdb.Pool().Exec(ctx, `
		INSERT INTO promotion_delivery_tickets (
			id, contract_id, target_type, target_id, viewer_id, status,
			issued_at, expires_at, consumed_at, created_at
		) VALUES ($1, $2, 'for_sale', $3, $4, 'consumed', NOW(), NOW() + INTERVAL '15 minutes', NOW(), NOW())
	`, ticketID, promotionID, uuid.New(), h.seller)
	require.NoError(t, err)

	_, err = h.tdb.Pool().Exec(ctx, `
		INSERT INTO promotion_qualified_impressions (
			id, ticket_id, contract_id, allocation_account_id,
			target_type, target_id, sequence_n, charge_rupiah,
			server_occurred_at, created_at
		) VALUES ($1, $2, $3, $4, 'for_sale', $5, $6, $7, NOW(), NOW())
	`, qiID, ticketID, promotionID, allocationID, uuid.New(), sequenceN, charge)
	require.NoError(t, err)
}

func (h *promotionFinanceHarness) systemBalance(t *testing.T, ctx context.Context, accountType string) (int64, bool) {
	t.Helper()
	var balance int64
	err := h.tdb.Pool().QueryRow(ctx,
		`SELECT balance FROM financial_accounts WHERE account_type = $1 AND user_id IS NULL`,
		accountType).Scan(&balance)
	if err != nil {
		return 0, false
	}
	return balance, true
}

func countTxByKeyPrefix(t *testing.T, ctx context.Context, tdb *testdb.TestDB, prefix string) int {
	t.Helper()
	var count int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger_transactions WHERE idempotency_key LIKE $1`, prefix+"%").Scan(&count))
	return count
}

func (h *promotionFinanceHarness) fund(t *testing.T, ctx context.Context, amount int64) uuid.UUID {
	t.Helper()
	fundingID := uuid.New()
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return h.svc.RecordPromoteBalanceFunding(ctx, tx, fundingID, h.seller, amount)
	}))
	return fundingID
}

func (h *promotionFinanceHarness) allocate(t *testing.T, ctx context.Context, promotionID uuid.UUID, budget int64) error {
	_, err := h.allocateID(ctx, promotionID, budget)
	return err
}

func (h *promotionFinanceHarness) allocateID(ctx context.Context, promotionID uuid.UUID, budget int64) (uuid.UUID, error) {
	var id uuid.UUID
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		id, err = h.svc.RecordPromotionAllocation(ctx, tx, promotionID, h.seller, budget)
		return err
	})
	return id, err
}

func (h *promotionFinanceHarness) qualify(t *testing.T, ctx context.Context, promotionID uuid.UUID, qiID uuid.UUID, charge int64) error {
	return h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return h.svc.RecordQualifiedImpression(ctx, tx, qiID, promotionID, h.seller, charge)
	})
}

func (h *promotionFinanceHarness) release(t *testing.T, ctx context.Context, promotionID uuid.UUID, amount int64) error {
	return h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return h.svc.RecordPromotionAllocationRelease(ctx, tx, promotionID, h.seller, amount)
	})
}

// TestPromotionFinance_Funding_Allocation_Idempotency (Proofs A + B)
func TestPromotionFinance_Funding_Allocation_Idempotency(t *testing.T) {
	h := newPromotionFinanceHarness(t)
	ctx := context.Background()

	bankBefore, _ := h.systemBalance(t, ctx, finance.AccountBankSettlement)

	// --- A: verified funding credits PROMOTE_BALANCE -----------------------
	fundingID := h.fund(t, ctx, 50_000)
	require.Equal(t, 1, countTxByKeyPrefix(t, ctx, h.tdb, "promote_balance_funding_"))

	balance, ok := h.promoteBalance(t, ctx)
	require.True(t, ok, "promote balance account must exist after funding")
	require.Equal(t, int64(50_000), balance)

	bankAfter, _ := h.systemBalance(t, ctx, finance.AccountBankSettlement)
	require.Equal(t, bankBefore-50_000, bankAfter, "funding must drain BANK_SETTLEMENT")

	// Duplicate funding (same key) must NOT double credit.
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return h.svc.RecordPromoteBalanceFunding(ctx, tx, fundingID, h.seller, 50_000)
	}))
	balance, _ = h.promoteBalance(t, ctx)
	require.Equal(t, int64(50_000), balance, "duplicate funding key must not double credit")
	require.Equal(t, 1, countTxByKeyPrefix(t, ctx, h.tdb, "promote_balance_funding_"))

	// Platform revenue must stay untouched by a top-up.
	platform, _ := h.systemBalance(t, ctx, finance.AccountPlatformRevenue)
	require.Equal(t, int64(0), platform, "top-up must NEVER book platform revenue")

	// --- B: allocation moves budget Promote Balance -> allocation ----------
	promotionID := uuid.New()
	require.NoError(t, h.allocate(t, ctx, promotionID, 30_000))

	promoteBal, _ := h.promoteBalance(t, ctx)
	require.Equal(t, int64(20_000), promoteBal, "promote balance must decrease by budget")

	allocBal, ok := h.allocationBalance(t, ctx, promotionID)
	require.True(t, ok, "allocation account must exist after allocation")
	require.Equal(t, int64(30_000), allocBal, "allocation must hold the budget")
	require.Equal(t, 1, countTxByKeyPrefix(t, ctx, h.tdb, "promotion_allocation_"))

	// Duplicate allocation (same promotion) must not move money twice.
	require.NoError(t, h.allocate(t, ctx, promotionID, 30_000))
	promoteBal, _ = h.promoteBalance(t, ctx)
	allocBal, _ = h.allocationBalance(t, ctx, promotionID)
	require.Equal(t, int64(20_000), promoteBal)
	require.Equal(t, int64(30_000), allocBal)
	require.Equal(t, 1, countTxByKeyPrefix(t, ctx, h.tdb, "promotion_allocation_"))
}

// TestPromotionFinance_Allocation_InsufficientBalance (Proof C)
func TestPromotionFinance_Allocation_InsufficientBalance(t *testing.T) {
	h := newPromotionFinanceHarness(t)
	ctx := context.Background()

	h.fund(t, ctx, 10_000)
	promotionID := uuid.New()

	err := h.allocate(t, ctx, promotionID, 20_000)
	require.ErrorIs(t, err, ErrPromoteBalanceInsufficient)

	// No partial mutation: balance unchanged, no allocation account row, no
	// allocation transaction.
	bal, _ := h.promoteBalance(t, ctx)
	require.Equal(t, int64(10_000), bal)
	_, found := h.allocationBalance(t, ctx, promotionID)
	require.False(t, found, "failed allocation must not create an allocation account")
	require.Equal(t, 0, countTxByKeyPrefix(t, ctx, h.tdb, "promotion_allocation_"))
}

// TestPromotionFinance_QualifiedImpression_And_Release (Proofs D, E, F)
func TestPromotionFinance_QualifiedImpression_And_Release(t *testing.T) {
	h := newPromotionFinanceHarness(t)
	ctx := context.Background()

	// Canonical cumulative arithmetic at CPM 7500: charge(1)=7, charge(2)=8,
	// charge(3)=7 -> total 22.
	h.fund(t, ctx, 10_000)
	promotionID := uuid.New()
	require.NoError(t, h.allocate(t, ctx, promotionID, 10_000))

	qi1, qi2, qi3 := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, h.qualify(t, ctx, promotionID, qi1, 7))
	require.NoError(t, h.qualify(t, ctx, promotionID, qi2, 8))
	require.NoError(t, h.qualify(t, ctx, promotionID, qi3, 7))

	// D: allocation decreased, platform revenue increased, ledger balanced.
	allocBal, _ := h.allocationBalance(t, ctx, promotionID)
	require.Equal(t, int64(10_000-22), allocBal)
	platform, _ := h.systemBalance(t, ctx, finance.AccountPlatformRevenue)
	require.Equal(t, int64(22), platform, "QI consumption is the ONLY promotion revenue path")
	require.Equal(t, 3, countTxByKeyPrefix(t, ctx, h.tdb, "promotion_qi_"))

	// E: duplicate consumption key must not double charge (sequential).
	require.NoError(t, h.qualify(t, ctx, promotionID, qi2, 8))
	allocBal, _ = h.allocationBalance(t, ctx, promotionID)
	require.Equal(t, int64(10_000-22), allocBal)
	platform, _ = h.systemBalance(t, ctx, finance.AccountPlatformRevenue)
	require.Equal(t, int64(22), platform)
	require.Equal(t, 3, countTxByKeyPrefix(t, ctx, h.tdb, "promotion_qi_"))

	// E (concurrent): two goroutines book the SAME QI -> charged exactly once.
	qiConcurrent := uuid.New()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- h.qualify(t, ctx, promotionID, qiConcurrent, 7)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	allocBal, _ = h.allocationBalance(t, ctx, promotionID)
	require.Equal(t, int64(10_000-29), allocBal, "concurrent duplicate QI must charge exactly once")
	require.Equal(t, 4, countTxByKeyPrefix(t, ctx, h.tdb, "promotion_qi_"))

	// F: release returns the remaining 9971 exactly once.
	remaining := int64(10_000 - 29)
	require.NoError(t, h.release(t, ctx, promotionID, remaining))
	allocBal, _ = h.allocationBalance(t, ctx, promotionID)
	require.Equal(t, int64(0), allocBal)
	promoteBal, _ := h.promoteBalance(t, ctx)
	require.Equal(t, remaining, promoteBal, "unused allocation must return to Promote Balance")
	require.Equal(t, 1, countTxByKeyPrefix(t, ctx, h.tdb, "promotion_allocation_release_"))

	// Duplicate release (same key) must not double credit.
	require.NoError(t, h.release(t, ctx, promotionID, remaining))
	promoteBal, _ = h.promoteBalance(t, ctx)
	allocBal, _ = h.allocationBalance(t, ctx, promotionID)
	require.Equal(t, remaining, promoteBal, "duplicate release must not double credit")
	require.Equal(t, int64(0), allocBal)
	require.Equal(t, 1, countTxByKeyPrefix(t, ctx, h.tdb, "promotion_allocation_release_"))

	// Any later release attempt for the same promotion is a duplicate-key
	// no-op: the finalization release is exact-once PER PROMOTION. A release
	// that fails its balance pre-check (ErrPromotionAllocationInsufficient)
	// does NOT burn the key — a retry with the correct remaining still works.
	require.NoError(t, h.release(t, ctx, promotionID, 1))
	promoteBal, _ = h.promoteBalance(t, ctx)
	allocBal, _ = h.allocationBalance(t, ctx, promotionID)
	require.Equal(t, remaining, promoteBal, "post-release duplicate must not move money")
	require.Equal(t, int64(0), allocBal)
	require.Equal(t, 1, countTxByKeyPrefix(t, ctx, h.tdb, "promotion_allocation_release_"))
}

// TestPromotionFinance_QiCharge_Exceeding_Remaining (Proof G edge + backstop)
func TestPromotionFinance_QiCharge_Exceeding_Remaining(t *testing.T) {
	h := newPromotionFinanceHarness(t)
	ctx := context.Background()

	h.fund(t, ctx, 5_000)
	promotionID := uuid.New()
	require.NoError(t, h.allocate(t, ctx, promotionID, 5_000))

	// Service pre-check rejects the overdraw with a typed error.
	err := h.qualify(t, ctx, promotionID, uuid.New(), 6_000)
	require.ErrorIs(t, err, ErrPromotionAllocationInsufficient)
	allocBal, _ := h.allocationBalance(t, ctx, promotionID)
	require.Equal(t, int64(5_000), allocBal)

	// DB-level backstop: even bypassing the service pre-check, a raw overdraw
	// cannot push the allocation negative — the financial_accounts.balance >= 0
	// CHECK aborts the transaction (rolled back, no mutation).
	rawErr := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		allocationID, err := h.repo.GetHolderAccountID(ctx, tx, finance.AccountPromotionAllocation, h.seller, promotionID)
		if err != nil {
			return err
		}
		platformRevenueID, err := h.repo.GetSystemAccountID(ctx, tx, finance.AccountPlatformRevenue)
		if err != nil {
			return err
		}
		return h.repo.CreateTransaction(
			ctx, tx, "promotion-overdraw-backstop-"+uuid.NewString(), "promotion_qi",
			uuid.New(), nil, nil, []ledgerintf.Entry{
				{AccountID: allocationID, Amount: money.New(-6_000)},
				{AccountID: platformRevenueID, Amount: money.New(6_000)},
			},
		)
	})
	require.Error(t, rawErr, "raw overdraw must be rejected by the DB CHECK backstop")
	allocBal, _ = h.allocationBalance(t, ctx, promotionID)
	require.Equal(t, int64(5_000), allocBal, "rejected overdraw must leave the allocation intact")
}

// TestPromotionFinance_FullLifecycle_VerifierStrict (Proof H)
// Runs the complete canonical lifecycle then runs the strict finance verifier
// against the real DB: every ledger entry replays, every balance reconciles,
// and the Promotion Financial Invariants section passes.
func TestPromotionFinance_FullLifecycle_VerifierStrict(t *testing.T) {
	h := newPromotionFinanceHarness(t)
	ctx := context.Background()

	h.fund(t, ctx, 30_000)
	promotionID := uuid.New()
	require.NoError(t, h.allocate(t, ctx, promotionID, 30_000))

	// The Phase 3 verifier reconciles every promotion_qi ledger charge
	// against a real promotion_qualified_impressions row, so this finance
	// proof seeds the canonical aggregates (contract + ticket + QI) before
	// charging the SAME qi ids — exactly the delivery boundary shape.
	// Two billable QIs at CPM 7500: charge(1)=7, charge(2)=8 -> 15 consumed.
	allocationID := h.allocationAccountID(t, ctx, promotionID)
	qi1, qi2 := uuid.New(), uuid.New()
	h.seedCanonicalQI(t, ctx, promotionID, allocationID, qi1, 1, 7)
	h.seedCanonicalQI(t, ctx, promotionID, allocationID, qi2, 2, 8)
	require.NoError(t, h.qualify(t, ctx, promotionID, qi1, 7))
	require.NoError(t, h.qualify(t, ctx, promotionID, qi2, 8))
	// Zero-charge impression (finance primitive no-op): no money movement,
	// no ledger transaction, not an error.
	require.NoError(t, h.qualify(t, ctx, promotionID, uuid.New(), 0))

	require.NoError(t, h.release(t, ctx, promotionID, 30_000-15))

	snapshot, err := verifier.LoadSnapshot(ctx, h.tdb.Pool())
	require.NoError(t, err)
	report := verifier.Verify(snapshot, verifier.ModeStrict)
	require.False(t, report.HasFailures(), "strict verifier must pass after the canonical lifecycle:\n%s", report.Format("promotion-real-db"))

	promotionSectionPassed := false
	for _, s := range report.Sections {
		if s.Name == "Promotion Financial Invariants" && s.Passed {
			promotionSectionPassed = true
		}
	}
	require.True(t, promotionSectionPassed, "Promotion Financial Invariants section must pass")
}

// TestPromotionFinance_PlatformConfig_Seeds verifies the promotion config
// foundation keys exist with the migration-000064 defaults and that delivery
// is DISABLED by default. Migration seed rows are truncated between tests
// (testdb cleanup), so the seed statement from migration 000064 is re-applied
// idempotently first — exactly as the migration does on a fresh schema.
func TestPromotionFinance_PlatformConfig_Seeds(t *testing.T) {
	h := newPromotionFinanceHarness(t)
	ctx := context.Background()

	// Seed statement copied verbatim from migrations/000064_*.up.sql.
	_, err := h.tdb.Pool().Exec(ctx, `
		INSERT INTO platform_configs (key, value_numeric, value_text, updated_by, updated_at)
		VALUES
			('promotion_cpm', 7500, NULL, NULL, EXTRACT(epoch FROM now())::bigint),
			('promotion_min_daily_budget', 10000, NULL, NULL, EXTRACT(epoch FROM now())::bigint),
			('promotion_delivery_enabled', NULL, 'disabled', NULL, EXTRACT(epoch FROM now())::bigint)
		ON CONFLICT (key) DO NOTHING
	`)
	require.NoError(t, err)

	get := func(key string) (numeric *int64, text string) {
		var n *int64
		var s *string
		err := h.tdb.Pool().QueryRow(ctx,
			`SELECT value_numeric, value_text FROM platform_configs WHERE key = $1`, key).Scan(&n, &s)
		require.NoError(t, err, "config key %s must exist after the migration-000064 seed", key)
		if s != nil {
			text = *s
		}
		return n, text
	}

	cpm, _ := get("promotion_cpm")
	require.NotNil(t, cpm)
	require.Equal(t, int64(7500), *cpm)

	minDaily, _ := get("promotion_min_daily_budget")
	require.NotNil(t, minDaily)
	require.Equal(t, int64(10000), *minDaily)

	_, enabled := get("promotion_delivery_enabled")
	require.Equal(t, "disabled", enabled, "promotion delivery must be DISABLED by default")
}

// TestPromotionFinance_ConcurrentDuplicateReleaseIsSafe proves concurrent
// duplicate releases (worker retry racing a planned finish) never double
// credit the seller.
func TestPromotionFinance_ConcurrentDuplicateReleaseIsSafe(t *testing.T) {
	h := newPromotionFinanceHarness(t)
	ctx := context.Background()

	h.fund(t, ctx, 10_000)
	promotionID := uuid.New()
	require.NoError(t, h.allocate(t, ctx, promotionID, 10_000))

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Duplicate release of the SAME promotion. The loser either no-ops
			// on the duplicate key or observes the drained allocation and
			// fails with the typed error — both are safe (no mutation).
			errs <- h.release(t, ctx, promotionID, 10_000)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, ErrPromotionAllocationInsufficient) {
			t.Fatalf("unexpected release error: %v", err)
		}
	}

	allocBal, _ := h.allocationBalance(t, ctx, promotionID)
	require.Equal(t, int64(0), allocBal, "allocation must be drained exactly once")
	promoteBal, _ := h.promoteBalance(t, ctx)
	require.Equal(t, int64(10_000), promoteBal, "release must credit exactly once, never twice")
	require.Equal(t, 1, countTxByKeyPrefix(t, ctx, h.tdb, "promotion_allocation_release_"))
}

// TestPromotionFinance_ConcurrentFinalBalanceExhaustion_IsSafe proves proof C:
// two DISTINCT Qualified Impressions racing for the FINAL remaining allocation.
// The allocation row FOR UPDATE lock inside RecordQualifiedImpression
// serializes them; exactly one consumes the balance, the loser is rejected by
// the sufficiency pre-check with no mutation. Total charge never exceeds the
// allocation and PLATFORM_REVENUE receives exactly the single successful charge.
func TestPromotionFinance_ConcurrentFinalBalanceExhaustion_IsSafe(t *testing.T) {
	h := newPromotionFinanceHarness(t)
	ctx := context.Background()

	h.fund(t, ctx, 10_000)
	promotionID := uuid.New()
	require.NoError(t, h.allocate(t, ctx, promotionID, 10_000))

	// Two distinct impressions, each asking for the ENTIRE allocation.
	qiA, qiB := uuid.New(), uuid.New()
	book := func(qiID uuid.UUID) error {
		return h.tdb.WithTx(ctx, func(tx db.Tx) error {
			return h.svc.RecordQualifiedImpression(ctx, tx, qiID, promotionID, h.seller, 10_000)
		})
	}

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, qiID := range []uuid.UUID{qiA, qiB} {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()
			results <- book(id)
		}(qiID)
	}
	wg.Wait()
	close(results)

	var wins, rejected int
	for err := range results {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, ErrPromotionAllocationInsufficient):
			rejected++
		default:
			t.Fatalf("unexpected concurrent exhaustion error: %v", err)
		}
	}
	require.Equal(t, 1, wins, "exactly one charge may consume the final allocation")
	require.Equal(t, 1, rejected, "the loser must be rejected with no mutation")

	allocBal, _ := h.allocationBalance(t, ctx, promotionID)
	require.Equal(t, int64(0), allocBal, "allocation drained exactly once, never negative")
	platform, _ := h.systemBalance(t, ctx, finance.AccountPlatformRevenue)
	require.Equal(t, int64(10_000), platform, "revenue equals the single successful charge, no double charge")
	require.Equal(t, 1, countTxByKeyPrefix(t, ctx, h.tdb, "promotion_qi_"), "exactly one ledger transaction")
}
