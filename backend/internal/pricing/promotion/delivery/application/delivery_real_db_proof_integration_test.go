//go:build integration

package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	financeapp "github.com/labuda/backend/internal/finance/application"
	"github.com/labuda/backend/internal/finance/verifier"
	configapp "github.com/labuda/backend/internal/platform/config/application"
	configrepo "github.com/labuda/backend/internal/platform/config/infrastructure/repository"
	legacyapp "github.com/labuda/backend/internal/pricing/promotion/application"
	contractapp "github.com/labuda/backend/internal/pricing/promotion/contract/application"
	contractentity "github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	contractRepoImpl "github.com/labuda/backend/internal/pricing/promotion/contract/infrastructure/repository"
	deliveryapp "github.com/labuda/backend/internal/pricing/promotion/delivery/application"
	deliveryentity "github.com/labuda/backend/internal/pricing/promotion/delivery/entity"
	deliveryRepoImpl "github.com/labuda/backend/internal/pricing/promotion/delivery/infrastructure/repository"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// PROMOTION PHASE 3 — DELIVERY TICKET + QUALIFIED IMPRESSION REAL-DB PROOFS
//
// Proves against the real PostgreSQL ledger:
//   A. ticket issuance moves NO money (Model A)
//   B. valid QI charges exactly once (allocation debit + revenue credit)
//   C. sequential cumulative CPM arithmetic (S(N)-S(N-1), no drift)
//   D. duplicate same-ticket qualification -> exactly one charge
//   E. concurrent same-ticket qualification -> exactly one winner
//   F. concurrent different tickets same contract -> unique N + exact charges
//   G. expired ticket -> rejected, no charge
//   H. ineligible target (canonical commerce authority) -> rejected, no charge
//   I. seller self-delivery -> rejected, no charge
//   J. qualify vs finalize race -> only legal outcomes A/B
//   K. full lifecycle -> strict finance verifier PASS incl. QI reconciliation
// ============================================================================

type allowAllGate struct{}

func (allowAllGate) EnsureCanPromote(context.Context, db.Tx, uuid.UUID) error { return nil }

type deliveryHarness struct {
	tdb       *testdb.TestDB
	finance   *financeapp.FinanceService
	contracts *contractapp.PromotionContractService
	delivery  *deliveryapp.DeliveryService
}

func newDeliveryHarness(t *testing.T) *deliveryHarness {
	t.Helper()
	tdb, financeSvc, contractSvc, deliveryRepo := setupDeliveryHarnessBase(t)
	// Canonical target eligibility authority: the existing OperabilityChecker
	// reads real For Sale / Auction state (reused, not duplicated).
	checker := legacyapp.NewOperabilityCheckerImpl(db.NewFromPool(tdb.Pool()), nil)
	return newDeliveryHarnessWithEligibility(t, tdb, financeSvc, contractSvc, deliveryRepo, checker)
}

// setupDeliveryHarnessBase wires the real-DB services shared by every
// delivery harness (system accounts, finance, contract, delivery repo).
func setupDeliveryHarnessBase(t *testing.T) (*testdb.TestDB, *financeapp.FinanceService, *contractapp.PromotionContractService, *deliveryRepoImpl.DeliveryRepositoryImpl) {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	bootstrap := financeapp.NewSystemAccountBootstrapFromPgx(db.NewFromPool(tdb.Pool()))
	_, err := bootstrap.EnsureSystemAccounts(ctx)
	require.NoError(t, err)

	financeSvc := financeapp.NewFinanceService()
	cfgSvc := configapp.NewConfigService(configrepo.NewPlatformConfigRepository())
	deliveryRepo := deliveryRepoImpl.NewDeliveryRepository(db.NewFromPool(tdb.Pool()))
	contractSvc := contractapp.NewPromotionContractService(
		db.NewFromPool(tdb.Pool()),
		financeSvc,
		cfgSvc,
		allowAllGate{},
		deliveryRepo,
	)
	// Phase 4C: Enable delivery in config for canonical enabled-path tests.
	// Migration 000064 seeds this as 'disabled' by default.
	_, err = tdb.Pool().Exec(ctx, `UPDATE platform_configs SET value_text = 'enabled' WHERE key = 'promotion_delivery_enabled'`)
	require.NoError(t, err)

	return tdb, financeSvc, contractSvc, deliveryRepo
}

// newDeliveryHarnessWithEligibility wires a delivery service with a
// caller-provided target eligibility (real canonical checker or a stub that
// controls the pre-flight gate and the transaction-boundary gate
// independently).
func newDeliveryHarnessWithEligibility(
	t *testing.T,
	tdb *testdb.TestDB,
	financeSvc *financeapp.FinanceService,
	contractSvc *contractapp.PromotionContractService,
	deliveryRepo *deliveryRepoImpl.DeliveryRepositoryImpl,
	eligibility deliveryapp.TargetEligibility,
) *deliveryHarness {
	t.Helper()
	cfgSvc := configapp.NewConfigService(configrepo.NewPlatformConfigRepository())
	deliverySvc := deliveryapp.NewDeliveryService(
		db.NewFromPool(tdb.Pool()),
		contractRepoImpl.NewContractRepository(),
		deliveryRepo,
		financeSvc,
		cfgSvc,
		eligibility,
	)
	return &deliveryHarness{tdb: tdb, finance: financeSvc, contracts: contractSvc, delivery: deliverySvc}
}

// stubEligibility lets a proof drive the pool-based pre-flight gate and the
// transaction-boundary revalidation gate independently, proving the billing
// boundary re-validates eligibility even when the pre-flight gate passed.
type stubEligibility struct {
	preflightOperable  bool
	txBoundaryOperable bool
}

func (s *stubEligibility) CheckOperability(context.Context, promoentity.TargetType, *uuid.UUID) (bool, string, error) {
	return s.preflightOperable, "", nil
}

func (s *stubEligibility) CheckOperabilityTx(context.Context, db.Tx, time.Time, promoentity.TargetType, *uuid.UUID) (bool, string, error) {
	if !s.txBoundaryOperable {
		return false, "for_sale_sold", nil
	}
	return true, "", nil
}

func (h *deliveryHarness) seedConfig(t *testing.T, cpm, minDaily int64) {
	t.Helper()
	_, err := h.tdb.Pool().Exec(context.Background(), `
		INSERT INTO platform_configs (key, value_numeric, value_text, updated_by, updated_at)
		VALUES
			('promotion_cpm', $1, NULL, NULL, EXTRACT(epoch FROM now())::bigint),
			('promotion_min_daily_budget', $2, NULL, NULL, EXTRACT(epoch FROM now())::bigint)
		ON CONFLICT (key) DO UPDATE
			SET value_numeric = EXCLUDED.value_numeric,
			    updated_at = EXTRACT(epoch FROM now())::bigint
	`, cpm, minDaily)
	require.NoError(t, err)
}

// newSeller creates an active seller with an active subscription and funds
// their Promote Balance (canonical funding path).
func (h *deliveryHarness) newSeller(t *testing.T, fundRupiah int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	seller := uuid.New()
	_, err := h.tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
		ON CONFLICT (id) DO NOTHING
	`, seller, "fb-delivery-"+seller.String()[:8], seller.String()+"@delivery.local")
	require.NoError(t, err)
	_, err = h.tdb.Pool().Exec(ctx, `
		INSERT INTO seller_subscriptions (
			id, user_id, status, started_at, expires_at,
			duration_days, amount_paid, payment_id
		) VALUES ($1, $2, 'active', $3, $4, 365, 0, $5)
	`, uuid.New(), seller, time.Now(), time.Now().Add(365*24*time.Hour), uuid.New())
	require.NoError(t, err)
	if fundRupiah > 0 {
		require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
			return h.finance.RecordPromoteBalanceFunding(ctx, tx, uuid.New(), seller, fundRupiah)
		}))
	}
	return seller
}

func (h *deliveryHarness) newViewer(t *testing.T) uuid.UUID {
	t.Helper()
	viewer := uuid.New()
	_, err := h.tdb.Pool().Exec(context.Background(), `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
		ON CONFLICT (id) DO NOTHING
	`, viewer, "fb-viewer-"+viewer.String()[:8], viewer.String()+"@viewer.local")
	require.NoError(t, err)
	return viewer
}

// newForSale creates a canonical active, published, in-stock for_sale owned
// by sellerID (eligible per the OperabilityChecker authority).
func (h *deliveryHarness) newForSale(t *testing.T, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	productID := uuid.New()
	forSaleID := uuid.New()
	_, err := h.tdb.Pool().Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, productID, sellerID, "Sanke Koi", "A fine sanke", `["https://cdn.example.com/sanke.jpg"]`, "sanke", "immediate")
	require.NoError(t, err)
	_, err = h.tdb.Pool().Exec(ctx, `
		INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, published_at, quantity_available)
		VALUES ($1, $2, $3, $4, 'active', NOW(), $5)
	`, forSaleID, productID, sellerID, int64(200_000), 1)
	require.NoError(t, err)
	return forSaleID
}

// newAuction creates a canonical active auction owned by sellerID.
func (h *deliveryHarness) newAuction(t *testing.T, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	productID := uuid.New()
	auctionID := uuid.New()
	_, err := h.tdb.Pool().Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, productID, sellerID, "Kohaku Koi", "A fine kohaku", `["https://cdn.example.com/kohaku.jpg"]`, "kohaku", "immediate")
	require.NoError(t, err)
	_, err = h.tdb.Pool().Exec(ctx, `
		INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, start_at, end_at, status, created_at, updated_at)
		VALUES ($1, $2, $3, 100000, 10000, NOW(), NOW() + INTERVAL '2 days', 'active', NOW(), NOW())
	`, auctionID, sellerID, productID)
	require.NoError(t, err)
	return auctionID
}

func (h *deliveryHarness) createContract(t *testing.T, seller uuid.UUID, kind contractentity.Kind, budget, days int64) *contractentity.Contract {
	t.Helper()
	c, err := h.contracts.Create(context.Background(), contractapp.CreatePromotionInput{
		SellerID:     seller,
		Kind:         kind,
		BudgetRupiah: budget,
		DurationDays: days,
	})
	require.NoError(t, err)
	return c
}

func (h *deliveryHarness) issue(t *testing.T, c *contractentity.Contract, targetType promoentity.TargetType, targetID, viewer uuid.UUID, ttl time.Duration) *deliveryentity.DeliveryTicket {
	t.Helper()
	ticket, err := h.delivery.IssueTicket(context.Background(), deliveryapp.IssueTicketInput{
		ContractID: c.ID,
		TargetType: targetType,
		TargetID:   targetID,
		ViewerID:   viewer,
		TTL:        ttl,
	})
	require.NoError(t, err)
	return ticket
}

// qualify is goroutine-safe: no require inside; callers assert on the error.
func (h *deliveryHarness) qualify(ticketID uuid.UUID) (*deliveryentity.QualifiedImpression, error) {
	return h.delivery.QualifyTicket(context.Background(), deliveryapp.QualifyTicketInput{TicketID: ticketID})
}

// --- read helpers -----------------------------------------------------------

func (h *deliveryHarness) promoteBalance(t *testing.T, seller uuid.UUID) int64 {
	t.Helper()
	var balance int64
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT balance FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2 AND holder_id IS NULL
	`, finance.AccountPromoteBalance, seller).Scan(&balance)
	require.NoError(t, err)
	return balance
}

func (h *deliveryHarness) allocationBalance(t *testing.T, seller, contractID uuid.UUID) int64 {
	t.Helper()
	var balance int64
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT balance FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2 AND holder_id = $3
	`, finance.AccountPromotionAllocation, seller, contractID).Scan(&balance)
	require.NoError(t, err)
	return balance
}

func (h *deliveryHarness) platformRevenue(t *testing.T) int64 {
	t.Helper()
	var balance int64
	err := h.tdb.Pool().QueryRow(context.Background(), `
		SELECT balance FROM financial_accounts WHERE account_type = $1 AND user_id IS NULL
	`, finance.AccountPlatformRevenue).Scan(&balance)
	require.NoError(t, err)
	return balance
}

func (h *deliveryHarness) countQI(t *testing.T, contractID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM promotion_qualified_impressions WHERE contract_id = $1
	`, contractID).Scan(&count))
	return count
}

// qiLedgerTxCountForContract counts promotion_qi ledger transactions whose
// reference is one of the contract's Qualified Impressions.
func (h *deliveryHarness) qiLedgerTxCountForContract(t *testing.T, contractID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM ledger_transactions
		WHERE reference_type = 'promotion_qi'
		  AND reference_id IN (SELECT id FROM promotion_qualified_impressions WHERE contract_id = $1)
	`, contractID).Scan(&count))
	return count
}

func (h *deliveryHarness) releaseTxCountForContract(t *testing.T, contractID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM ledger_transactions WHERE idempotency_key LIKE $1
	`, "promotion_allocation_release_"+contractID.String()+"%").Scan(&count))
	return count
}

func (h *deliveryHarness) ticketStatus(t *testing.T, ticketID uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `
		SELECT status::text FROM promotion_delivery_tickets WHERE id = $1
	`, ticketID).Scan(&status))
	return status
}

func (h *deliveryHarness) qiBySequence(t *testing.T, contractID uuid.UUID) map[int64]int64 {
	t.Helper()
	rows, err := h.tdb.Pool().Query(context.Background(), `
		SELECT sequence_n, charge_rupiah FROM promotion_qualified_impressions
		WHERE contract_id = $1 ORDER BY sequence_n
	`, contractID)
	require.NoError(t, err)
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var n, charge int64
		require.NoError(t, rows.Scan(&n, &charge))
		out[n] = charge
	}
	require.NoError(t, rows.Err())
	return out
}

// ============================================================================

func TestPromotionDelivery_Canonical_RealDB(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()

	t.Run("A_ticket_issuance_no_money_movement", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 30_000, 3)
		target := h.newForSale(t, seller)
		viewer := h.newViewer(t)

		allocBefore := h.allocationBalance(t, seller, c.ID)
		promoteBefore := h.promoteBalance(t, seller)
		revenueBefore := h.platformRevenue(t)

		ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, viewer, 15*time.Minute)
		require.Equal(t, deliveryentity.TicketStatusIssued, ticket.Status)
		require.True(t, ticket.ExpiresAt.After(ticket.IssuedAt), "server-derived expiry")

		// Money boundary (Model A): issuance changes NOTHING financially.
		require.Equal(t, allocBefore, h.allocationBalance(t, seller, c.ID), "allocation must be unchanged")
		require.Equal(t, promoteBefore, h.promoteBalance(t, seller), "promote balance must be unchanged")
		require.Equal(t, revenueBefore, h.platformRevenue(t), "platform revenue must be unchanged")
		require.Equal(t, 0, h.qiLedgerTxCountForContract(t, c.ID), "no consumption ledger transaction")
		require.Equal(t, 0, h.countQI(t, c.ID), "no qualified impression")
		require.Equal(t, 0, h.releaseTxCountForContract(t, c.ID), "no release transaction")
	})

	t.Run("B_valid_qi_charges_exactly_once", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000) // CPM 7500: S(1)=7
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
		target := h.newForSale(t, seller)
		viewer := h.newViewer(t)
		revenueBefore := h.platformRevenue(t)

		ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, viewer, 15*time.Minute)
		qi, err := h.qualify(ticket.ID)
		require.NoError(t, err)
		require.Equal(t, int64(1), qi.SequenceN)
		require.Equal(t, int64(7), qi.ChargeRupiah)
		require.Equal(t, ticket.ID, qi.TicketID)
		require.Equal(t, c.ID, qi.ContractID)

		require.Equal(t, 1, h.countQI(t, c.ID), "exactly one qualified impression")
		require.Equal(t, 1, h.qiLedgerTxCountForContract(t, c.ID), "exactly one ledger transaction")
		require.Equal(t, int64(50_000-7), h.allocationBalance(t, seller, c.ID), "allocation decreased by charge")
		require.Equal(t, revenueBefore+7, h.platformRevenue(t), "platform revenue increased by charge")
		require.Equal(t, int64(50_000), h.promoteBalance(t, seller), "promote balance untouched by consumption")
		require.Equal(t, "consumed", h.ticketStatus(t, ticket.ID), "ticket consumed exactly once")
	})

	t.Run("B2_auction_target_qualifies_via_canonical_authority", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
		auctionTarget := h.newAuction(t, seller)
		viewer := h.newViewer(t)

		ticket := h.issue(t, c, promoentity.TargetTypeAuction, auctionTarget, viewer, 15*time.Minute)
		qi, err := h.qualify(ticket.ID)
		require.NoError(t, err)
		require.Equal(t, int64(7), qi.ChargeRupiah)
		require.Equal(t, int64(50_000-7), h.allocationBalance(t, seller, c.ID))
	})

	t.Run("C_sequential_cumulative_cpm_arithmetic", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000) // CPM 7500 -> charges 7,8,7 (S(3)=22)
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
		target := h.newForSale(t, seller)
		revenueBefore := h.platformRevenue(t)

		expected := []struct {
			n      int64
			charge int64
		}{
			{1, 7}, {2, 8}, {3, 7},
		}
		for _, e := range expected {
			ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, h.newViewer(t), 15*time.Minute)
			qi, err := h.qualify(ticket.ID)
			require.NoError(t, err)
			require.Equal(t, e.n, qi.SequenceN)
			require.Equal(t, e.charge, qi.ChargeRupiah, "charge(N) = S(N)-S(N-1)")
		}

		got := h.qiBySequence(t, c.ID)
		require.Equal(t, map[int64]int64{1: 7, 2: 8, 3: 7}, got)
		require.Equal(t, 3, h.qiLedgerTxCountForContract(t, c.ID))
		require.Equal(t, int64(50_000-22), h.allocationBalance(t, seller, c.ID), "cumulative spend = S(3) = 22")
		require.Equal(t, revenueBefore+22, h.platformRevenue(t), "sum of charges = S(3) exactly, no drift")
	})

	t.Run("D_duplicate_same_ticket_sequential", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
		target := h.newForSale(t, seller)
		revenueBefore := h.platformRevenue(t)

		ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, h.newViewer(t), 15*time.Minute)
		_, err := h.qualify(ticket.ID)
		require.NoError(t, err)

		// Same ticket again: consumed -> rejected, no second charge.
		_, err = h.qualify(ticket.ID)
		require.ErrorIs(t, err, deliveryapp.ErrTicketNotIssued)

		require.Equal(t, 1, h.countQI(t, c.ID), "1 QI")
		require.Equal(t, 1, h.qiLedgerTxCountForContract(t, c.ID), "1 charge")
		require.Equal(t, revenueBefore+7, h.platformRevenue(t), "no double charge")
		require.Equal(t, int64(50_000-7), h.allocationBalance(t, seller, c.ID))
	})

	t.Run("E_concurrent_same_ticket_qualification", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
		target := h.newForSale(t, seller)
		revenueBefore := h.platformRevenue(t)

		ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, h.newViewer(t), 15*time.Minute)

		const workers = 8
		var wg sync.WaitGroup
		errs := make(chan error, workers)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := h.qualify(ticket.ID)
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		var wins, rejected int
		for err := range errs {
			switch {
			case err == nil:
				wins++
			case errors.Is(err, deliveryapp.ErrTicketNotIssued):
				rejected++
			default:
				t.Fatalf("unexpected concurrent qualification error: %v", err)
			}
		}
		require.Equal(t, 1, wins, "exactly one concurrent qualification must win")
		require.Equal(t, workers-1, rejected, "all losers observe the consumed ticket")
		require.Equal(t, 1, h.countQI(t, c.ID), "exactly one QI")
		require.Equal(t, 1, h.qiLedgerTxCountForContract(t, c.ID), "exactly one charge")
		require.Equal(t, revenueBefore+7, h.platformRevenue(t))
		require.Equal(t, int64(50_000-7), h.allocationBalance(t, seller, c.ID))
	})

	t.Run("F_concurrent_different_tickets_same_contract", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000) // CPM 7500 -> S(6)=45, charges 7,8,7,8,7,8
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
		target := h.newForSale(t, seller)
		revenueBefore := h.platformRevenue(t)

		const tickets = 6
		ids := make([]uuid.UUID, 0, tickets)
		for i := 0; i < tickets; i++ {
			ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, h.newViewer(t), 15*time.Minute)
			ids = append(ids, ticket.ID)
		}

		var wg sync.WaitGroup
		errs := make(chan error, tickets)
		for _, id := range ids {
			wg.Add(1)
			go func(ticketID uuid.UUID) {
				defer wg.Done()
				_, err := h.qualify(ticketID)
				errs <- err
			}(id)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err, "every distinct ticket must qualify")
		}

		got := h.qiBySequence(t, c.ID)
		require.Equal(t, map[int64]int64{1: 7, 2: 8, 3: 7, 4: 8, 5: 7, 6: 8}, got,
			"unique sequential N with deterministic cumulative CPM charges")
		require.Equal(t, tickets, h.countQI(t, c.ID))
		require.Equal(t, tickets, h.qiLedgerTxCountForContract(t, c.ID))
		require.Equal(t, int64(50_000-45), h.allocationBalance(t, seller, c.ID), "cumulative = S(6) = 45, no allocation drift")
		require.Equal(t, revenueBefore+45, h.platformRevenue(t))
	})

	t.Run("G_expired_ticket_rejected", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
		target := h.newForSale(t, seller)
		revenueBefore := h.platformRevenue(t)

		ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, h.newViewer(t), 15*time.Minute)
		// Force the whole validity window into the past (issued < expires
		// stays true for the DB CHECK, but expires_at is before DB now, so
		// qualification must reject on the DB clock authority).
		_, err := h.tdb.Pool().Exec(ctx, `
			UPDATE promotion_delivery_tickets
			SET issued_at = NOW() - INTERVAL '2 minutes',
			    expires_at = NOW() - INTERVAL '1 minute'
			WHERE id = $1
		`, ticket.ID)
		require.NoError(t, err)

		_, err = h.qualify(ticket.ID)
		require.ErrorIs(t, err, deliveryapp.ErrTicketExpired)
		require.Equal(t, 0, h.countQI(t, c.ID), "no QI")
		require.Equal(t, 0, h.qiLedgerTxCountForContract(t, c.ID), "no charge")
		require.Equal(t, revenueBefore, h.platformRevenue(t))
		require.Equal(t, "issued", h.ticketStatus(t, ticket.ID), "ticket remains issued (not consumed)")
	})

	t.Run("H_ineligible_target_rejected", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
		target := h.newForSale(t, seller)
		revenueBefore := h.platformRevenue(t)

		ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, h.newViewer(t), 15*time.Minute)
		// Target loses canonical eligibility AFTER issuance (seller withdraws).
		_, err := h.tdb.Pool().Exec(ctx, `UPDATE for_sales SET status = 'withdrawn' WHERE id = $1`, target)
		require.NoError(t, err)

		_, err = h.qualify(ticket.ID)
		var ineligible *deliveryapp.ErrTargetIneligible
		require.ErrorAs(t, err, &ineligible, "must be rejected by canonical target authority")
		require.Equal(t, "for_sale_hidden", ineligible.Reason)
		require.Equal(t, 0, h.countQI(t, c.ID), "no QI")
		require.Equal(t, 0, h.qiLedgerTxCountForContract(t, c.ID), "no charge")
		require.Equal(t, revenueBefore, h.platformRevenue(t))
		require.Equal(t, "issued", h.ticketStatus(t, ticket.ID))
	})

	t.Run("H2_auction_ended_rejected", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
		auctionTarget := h.newAuction(t, seller)

		ticket := h.issue(t, c, promoentity.TargetTypeAuction, auctionTarget, h.newViewer(t), 15*time.Minute)
		_, err := h.tdb.Pool().Exec(ctx, `UPDATE auctions SET status = 'ended' WHERE id = $1`, auctionTarget)
		require.NoError(t, err)

		_, err = h.qualify(ticket.ID)
		var ineligible *deliveryapp.ErrTargetIneligible
		require.ErrorAs(t, err, &ineligible)
		require.Equal(t, "auction_ended", ineligible.Reason)
		require.Equal(t, 0, h.countQI(t, c.ID))
		require.Equal(t, 0, h.qiLedgerTxCountForContract(t, c.ID))
	})

	t.Run("I_seller_self_delivery_rejected", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
		target := h.newForSale(t, seller)
		revenueBefore := h.platformRevenue(t)

		// Issuance rejects a self-viewed ticket structurally.
		_, err := h.delivery.IssueTicket(ctx, deliveryapp.IssueTicketInput{
			ContractID: c.ID,
			TargetType: promoentity.TargetTypeForSale,
			TargetID:   target,
			ViewerID:   seller, // the seller is the viewer
			TTL:        15 * time.Minute,
		})
		require.ErrorIs(t, err, deliveryapp.ErrTicketSelfDelivery)

		// Qualification-time defense: even a ticket that bypassed issuance
		// (direct row insert) is rejected at qualification.
		selfTicketID := uuid.New()
		_, err = h.tdb.Pool().Exec(ctx, `
			INSERT INTO promotion_delivery_tickets (
				id, contract_id, target_type, target_id, viewer_id,
				status, issued_at, expires_at, created_at
			) VALUES ($1, $2, 'for_sale', $3, $4, 'issued', NOW(), NOW() + INTERVAL '15 minutes', NOW())
		`, selfTicketID, c.ID, target, seller)
		require.NoError(t, err)

		_, err = h.qualify(selfTicketID)
		require.ErrorIs(t, err, deliveryapp.ErrTicketSelfDelivery)
		require.Equal(t, 0, h.countQI(t, c.ID), "no QI")
		require.Equal(t, 0, h.qiLedgerTxCountForContract(t, c.ID), "no charge")
		require.Equal(t, revenueBefore, h.platformRevenue(t))
	})

	t.Run("J_qualify_vs_finalize_race", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)

		// --- Deterministic Outcome A: qualify wins, finalize releases remainder ---
		t.Run("A_qualify_then_finalize", func(t *testing.T) {
			seller := h.newSeller(t, 100_000)
			c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
			target := h.newForSale(t, seller)
			revenueBefore := h.platformRevenue(t)
			ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, h.newViewer(t), 15*time.Minute)

			_, err := h.qualify(ticket.ID)
			require.NoError(t, err)
			require.NoError(t, h.contracts.Finalize(ctx, contractapp.FinalizePromotionInput{SellerID: seller, ContractID: c.ID}))

			require.Equal(t, int64(0), h.allocationBalance(t, seller, c.ID), "allocation drained")
			require.Equal(t, int64(99_993), h.promoteBalance(t, seller), "release = exact remainder 49,993")
			require.Equal(t, 1, h.releaseTxCountForContract(t, c.ID), "released exactly once")
			require.Equal(t, revenueBefore+7, h.platformRevenue(t), "charge committed before finalize survives")
		})

		// --- Deterministic Outcome B: finalize wins, old ticket never charges ---
		t.Run("B_finalize_then_qualify", func(t *testing.T) {
			seller := h.newSeller(t, 100_000)
			c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
			target := h.newForSale(t, seller)
			revenueBefore := h.platformRevenue(t)
			ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, h.newViewer(t), 15*time.Minute)

			require.NoError(t, h.contracts.Finalize(ctx, contractapp.FinalizePromotionInput{SellerID: seller, ContractID: c.ID}))

			_, err := h.qualify(ticket.ID)
			require.Error(t, err, "a ticket of a finalized contract must never qualify")
			require.True(t, errors.Is(err, deliveryapp.ErrContractNotActive) || errors.Is(err, deliveryapp.ErrTicketNotIssued),
				"rejection must be a post-finalization guard (contract not active or ticket invalidated), got: %v", err)
			require.Equal(t, 0, h.countQI(t, c.ID), "no QI")
			require.Equal(t, 0, h.qiLedgerTxCountForContract(t, c.ID), "no charge")
			require.Equal(t, revenueBefore, h.platformRevenue(t), "no post-finalization charge")
			require.Equal(t, "invalidated", h.ticketStatus(t, ticket.ID), "outstanding ticket invalidated at finalize")
			require.Equal(t, int64(0), h.allocationBalance(t, seller, c.ID))
			require.Equal(t, int64(100_000), h.promoteBalance(t, seller), "full allocation released")
			require.Equal(t, 1, h.releaseTxCountForContract(t, c.ID))
		})

		// --- True concurrent race: only the two legal outcomes may occur ---
		t.Run("C_concurrent", func(t *testing.T) {
			for iter := 0; iter < 4; iter++ {
				seller := h.newSeller(t, 100_000)
				c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
				target := h.newForSale(t, seller)
				ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, h.newViewer(t), 15*time.Minute)
				revenueBefore := h.platformRevenue(t)

				var wg sync.WaitGroup
				qualifyErrs := make(chan error, 1)
				finalizeErrs := make(chan error, 1)
				wg.Add(2)
				go func() {
					defer wg.Done()
					_, err := h.qualify(ticket.ID)
					qualifyErrs <- err
				}()
				go func() {
					defer wg.Done()
					finalizeErrs <- h.contracts.Finalize(ctx, contractapp.FinalizePromotionInput{SellerID: seller, ContractID: c.ID})
				}()
				wg.Wait()
				close(qualifyErrs)
				close(finalizeErrs)
				require.NoError(t, <-finalizeErrs, "finalize must always succeed on a fresh contract")
				qualifyErr := <-qualifyErrs
				require.True(t, qualifyErr == nil ||
					errors.Is(qualifyErr, deliveryapp.ErrContractNotActive) ||
					errors.Is(qualifyErr, deliveryapp.ErrTicketNotIssued),
					"qualify must either win or be rejected by a post-finalization guard, got: %v", qualifyErr)

				qiCount := h.countQI(t, c.ID)
				allocAfter := h.allocationBalance(t, seller, c.ID)
				promoteAfter := h.promoteBalance(t, seller)
				revenueAfter := h.platformRevenue(t)
				releaseCount := h.releaseTxCountForContract(t, c.ID)
				ticketState := h.ticketStatus(t, ticket.ID)

				require.Equal(t, int64(0), allocAfter, "allocation must be drained exactly once in either outcome")
				require.Equal(t, 1, releaseCount, "exactly one release, never double")

				switch {
				case qiCount == 1 && ticketState == "consumed" && revenueAfter == revenueBefore+7 && promoteAfter == 99_993:
					// Outcome A: charge won, finalize released the exact remainder.
					require.Equal(t, 1, h.qiLedgerTxCountForContract(t, c.ID))
				case qiCount == 0 && ticketState == "invalidated" && revenueAfter == revenueBefore && promoteAfter == 100_000:
					// Outcome B: finalize won, ticket never charged.
					require.Equal(t, 0, h.qiLedgerTxCountForContract(t, c.ID))
				default:
					t.Fatalf("iteration %d: illegal race outcome: qi=%d alloc=%d promote=%d revenue_delta=%d release=%d ticket=%s",
						iter, qiCount, allocAfter, promoteAfter, revenueAfter-revenueBefore, releaseCount, ticketState)
				}
			}
		})
	})

	t.Run("K_full_lifecycle_verifier_strict", func(t *testing.T) {
		h.seedConfig(t, 7500, 10_000)
		seller := h.newSeller(t, 100_000)
		c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
		target := h.newForSale(t, seller)

		// Issue + qualify 3 impressions (charges 7 + 8 + 7 = 22).
		for i := 0; i < 3; i++ {
			ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, h.newViewer(t), 15*time.Minute)
			_, err := h.qualify(ticket.ID)
			require.NoError(t, err)
		}
		require.NoError(t, h.contracts.Finalize(ctx, contractapp.FinalizePromotionInput{SellerID: seller, ContractID: c.ID}))

		snapshot, err := verifier.LoadSnapshot(ctx, h.tdb.Pool())
		require.NoError(t, err)
		report := verifier.Verify(snapshot, verifier.ModeStrict)
		require.False(t, report.HasFailures(), "strict verifier must pass after the full lifecycle:\n%s", report.Format("promotion-delivery-real-db"))

		sections := map[string]bool{}
		for _, s := range report.Sections {
			if s.Passed {
				sections[s.Name] = true
			}
		}
		require.True(t, sections["Qualified Impression Reconciliation"], "QI reconciliation section must pass")
		require.True(t, sections["Promotion Financial Invariants"], "promotion financial invariants must pass")
		require.True(t, sections["Account Balance Integrity"], "account balance integrity must pass")
	})
}

// TestPromotionDelivery_TxBoundaryEligibilityRevalidation_RealDB proves the
// transaction-boundary eligibility revalidation closes the pre-flight TOCTOU
// window (canonical §13/§19): even when the pre-flight gate passes, a target
// that is ineligible at the transaction boundary can never produce a billable
// Qualified Impression. A stub eligibility drives the two gates
// independently, so the proof is deterministic (no timing races): the
// pre-flight gate says eligible, the transaction-boundary gate says the
// target just became ineligible (e.g. for_sale sold between the two reads).
//
// This is a separate top-level test (not a subtest of the canonical suite)
// because it needs its own test database harness — the test-db lifecycle
// advisory lock is held per top-level SetupDB.
func TestPromotionDelivery_TxBoundaryEligibilityRevalidation_RealDB(t *testing.T) {
	tdb, financeSvc, contractSvc, deliveryRepo := setupDeliveryHarnessBase(t)
	h := newDeliveryHarnessWithEligibility(t, tdb, financeSvc, contractSvc, deliveryRepo,
		&stubEligibility{preflightOperable: true, txBoundaryOperable: false})
	h.seedConfig(t, 7500, 10_000)
	seller := h.newSeller(t, 100_000)
	c := h.createContract(t, seller, contractentity.KindInternal, 50_000, 3)
	target := h.newForSale(t, seller)
	revenueBefore := h.platformRevenue(t)

	ticket := h.issue(t, c, promoentity.TargetTypeForSale, target, h.newViewer(t), 15*time.Minute)

	_, err := h.qualify(ticket.ID)
	var ineligible *deliveryapp.ErrTargetIneligible
	require.ErrorAs(t, err, &ineligible, "stale target must be rejected at the transaction boundary")
	require.Equal(t, "for_sale_sold", ineligible.Reason)
	require.Equal(t, 0, h.countQI(t, c.ID), "no QI")
	require.Equal(t, 0, h.qiLedgerTxCountForContract(t, c.ID), "no charge")
	require.Equal(t, revenueBefore, h.platformRevenue(t), "platform revenue untouched")
	require.Equal(t, "issued", h.ticketStatus(t, ticket.ID), "ticket stays issued (whole tx rolled back)")
}
