package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	ledgerepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// ============================================================================
// PROMOTION FINANCIAL FOUNDATION (PHASE 1)
// ============================================================================
//
// Canonical financial graph (single authority path — the immutable
// double-entry ledger owned by FinanceService):
//
//	Top-up verified (canonical payment webhook):
//	    BANK_SETTLEMENT        -amount
//	    PROMOTE_BALANCE[seller] +amount
//
//	Create Promotion allocation:
//	    PROMOTE_BALANCE[seller]            -budget
//	    PROMOTION_ALLOCATION[seller,promo] +budget
//
//	Qualified Impression (billable charge):
//	    PROMOTION_ALLOCATION[seller,promo] -charge
//	    PLATFORM_REVENUE                   +charge
//
//	Finalization — unused allocation release:
//	    PROMOTION_ALLOCATION[seller,promo] -remaining
//	    PROMOTE_BALANCE[seller]            +remaining
//
// Rules honored:
//   - PLATFORM_REVENUE receives promotion revenue ONLY from Qualified
//     Impressions. A top-up NEVER books platform revenue.
//   - PROMOTE_BALANCE is a seller-owned usage balance; no direct wallet, no
//     second ledger, no coin-reservation money model.
//   - No mutable remaining_budget counter and no ticket reservation of money:
//     allocation availability is the PROMOTION_ALLOCATION ledger balance.
//   - Idempotency keys make every operation replay-safe; duplicate calls are
//     no-ops at the ledger layer (UNIQUE on ledger_transactions.idempotency_key).
//   - The DB CHECK financial_accounts.balance >= 0 is the fail-closed backstop:
//     PROMOTE_BALANCE and PROMOTION_ALLOCATION can never go negative. Balance
//     pre-checks below surface insufficiency as typed errors first.
//
// A zero-charge Qualified Impression (cumulative rounding, see
// finance.PromotionCharge) performs NO money movement and is not an error.
// ============================================================================

// Promotion ledger errors surfaced by the promotion financial operations.
var (
	// ErrPromoteBalanceInsufficient is returned when an allocation would draw
	// more than the seller's available PROMOTE_BALANCE. No partial financial
	// mutation occurs: the caller must abort the whole transaction.
	ErrPromoteBalanceInsufficient = errors.New("promote balance insufficient")

	// ErrPromotionAllocationInsufficient is returned when a Qualified
	// Impression charge or a release would draw more than the remaining
	// PROMOTION_ALLOCATION balance. No partial financial mutation occurs.
	ErrPromotionAllocationInsufficient = errors.New("promotion allocation insufficient")
)

// promotionLedgerCapabilities is the holder-scoped + replay-guard surface the
// promotion operations need on top of the base LedgerRepository. It is
// satisfied by the concrete *LedgerRepository; fakes used by unrelated
// FinanceService unit tests are untouched (no interface expansion).
type promotionLedgerCapabilities interface {
	GetHolderAccountID(ctx context.Context, tx db.Tx, accountType string, userID, holderID uuid.UUID) (uuid.UUID, error)
	GetOrCreateHolderAccount(ctx context.Context, tx db.Tx, accountType string, holderType string, userID, holderID uuid.UUID) (uuid.UUID, error)
	TransactionExistsByKey(ctx context.Context, tx db.Tx, idempotencyKey string) (bool, error)
}

// promotionHolderType is the holder_type stored on PROMOTION_ALLOCATION
// accounts. The holder is the promotion contract that owns the allocation.
const promotionHolderType = "promotion"

func (s *FinanceService) promotionRepo() (promotionLedgerCapabilities, error) {
	pr, ok := s.ledgerRepo.(promotionLedgerCapabilities)
	if !ok {
		return nil, fmt.Errorf("finance: ledger repository does not implement promotion holder accounts")
	}
	return pr, nil
}

// ============================================================================
// PROMOTE BALANCE FUNDING (verified top-up)
// ============================================================================

// RecordPromoteBalanceFunding books the canonical ledger transaction for a
// VERIFIED Promote Balance top-up (seller pays through the canonical payment
// flow; the payment gateway webhook confirmed it).
//
// Caller responsibilities:
//   - payment row already settled/verified in the SAME db.Tx (the webhook
//     handler owns the payment state transition)
//   - fundingID is the canonical reference for this top-up (the settled
//     payment or top-up id) — used for idempotency
//   - amount is a positive Rupiah integer (Labuda canonical money unit)
//
// Ledger movements (Σ entries = 0):
//   - BANK_SETTLEMENT        -amount   (real gateway money leaves the reserve)
//   - PROMOTE_BALANCE[seller] +amount   (seller usage balance credited)
//
// PLATFORM_REVENUE is NOT touched: a top-up is not platform revenue.
// Only Qualified Impression consumption books promotion revenue.
//
// IDEMPOTENCY: idempotency_key = "promote_balance_funding_<funding_id>".
// Duplicate webhook / retry calls are no-ops at the ledger layer.
func (s *FinanceService) RecordPromoteBalanceFunding(
	ctx context.Context,
	tx db.Tx,
	fundingID uuid.UUID,
	sellerID uuid.UUID,
	amount int64,
) error {
	if fundingID == uuid.Nil {
		return fmt.Errorf("RecordPromoteBalanceFunding: funding_id required")
	}
	if sellerID == uuid.Nil {
		return fmt.Errorf("RecordPromoteBalanceFunding: seller_id required")
	}
	if amount <= 0 {
		return fmt.Errorf("RecordPromoteBalanceFunding: amount must be positive (got %d)", amount)
	}

	pr, err := s.promotionRepo()
	if err != nil {
		return err
	}

	idempotencyKey := fmt.Sprintf("promote_balance_funding_%s", fundingID.String())
	exists, err := pr.TransactionExistsByKey(ctx, tx, idempotencyKey)
	if err != nil {
		return fmt.Errorf("promote balance funding duplicate-check: %w", err)
	}
	if exists {
		s.logger.Info("promote_balance_funding_duplicate_ignored",
			zap.String("funding_id", fundingID.String()),
			zap.String("seller_id", sellerID.String()),
			zap.Int64("amount", amount),
			zap.String("idempotency_key", idempotencyKey),
		)
		return nil
	}

	bankSettlementID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountBankSettlement)
	if err != nil {
		return fmt.Errorf("get bank settlement account: %w", err)
	}
	promoteBalanceID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountPromoteBalance, sellerID)
	if err != nil {
		return fmt.Errorf("get/create promote balance account: %w", err)
	}

	// CANONICAL SIGN: BS (liab) DR decreases, MB (liab) CR increases.
	entries := []ledgerepo.Entry{
		{AccountID: bankSettlementID, Amount: money.New(amount)},   // DR: reserve decreases
		{AccountID: promoteBalanceID, Amount: money.New(-amount)}, // CR: promote balance increases
	}
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idempotencyKey, "promote_balance_funding", fundingID, nil, nil, entries,
	); err != nil {
		return fmt.Errorf("record promote balance funding ledger: %w", err)
	}

	s.logger.Info("promote_balance_funding_recorded",
		zap.String("funding_id", fundingID.String()),
		zap.String("seller_id", sellerID.String()),
		zap.Int64("amount", amount),
		zap.String("idempotency_key", idempotencyKey),
	)
	return nil
}

// ============================================================================
// PROMOTION ALLOCATION (budget move)
// ============================================================================

// RecordPromotionAllocation books the budget move that funds a promotion's
// allocation account from the seller's Promote Balance.
//
// Caller responsibilities:
//   - promotion row already created/locked in the SAME db.Tx
//   - promotionID is the promotion contract id — it becomes the allocation
//     account's holder scope
//   - budget is a positive Rupiah integer
//
// Ledger movements (Σ entries = 0):
//   - PROMOTE_BALANCE[seller]            -budget
//   - PROMOTION_ALLOCATION[seller,promo] +budget
//
// The allocation account is created on first allocation (user + holder
// scope). Insufficient Promote Balance returns ErrPromoteBalanceInsufficient
// and performs NO partial financial mutation.
//
// IDEMPOTENCY: idempotency_key = "promotion_allocation_<promotion_id>".
func (s *FinanceService) RecordPromotionAllocation(
	ctx context.Context,
	tx db.Tx,
	promotionID uuid.UUID,
	sellerID uuid.UUID,
	budget int64,
) error {
	if promotionID == uuid.Nil {
		return fmt.Errorf("RecordPromotionAllocation: promotion_id required")
	}
	if sellerID == uuid.Nil {
		return fmt.Errorf("RecordPromotionAllocation: seller_id required")
	}
	if budget <= 0 {
		return fmt.Errorf("RecordPromotionAllocation: budget must be positive (got %d)", budget)
	}

	if _, err := s.RecordCanonicalPromotionAllocation(ctx, tx, promotionID, sellerID, budget); err != nil {
		return err
	}
	return nil
}

// RecordCanonicalPromotionAllocation books a canonical promotion allocation without asserting legacy promotion identity rules.
func (s *FinanceService) RecordCanonicalPromotionAllocation(
	ctx context.Context,
	tx db.Tx,
	promotionID uuid.UUID,
	sellerID uuid.UUID,
	budget int64,
) (uuid.UUID, error) {
	if promotionID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("RecordCanonicalPromotionAllocation: promotion_id required")
	}
	if sellerID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("RecordCanonicalPromotionAllocation: seller_id required")
	}
	if budget <= 0 {
		return uuid.Nil, fmt.Errorf("RecordCanonicalPromotionAllocation: budget must be positive (got %d)", budget)
	}

	pr, err := s.promotionRepo()
	if err != nil {
		return uuid.Nil, err
	}

	idempotencyKey := fmt.Sprintf("promotion_allocation_%s", promotionID.String())
	exists, err := pr.TransactionExistsByKey(ctx, tx, idempotencyKey)
	if err != nil {
		return uuid.Nil, fmt.Errorf("promotion allocation duplicate-check: %w", err)
	}
	if exists {
		s.logger.Info("promotion_allocation_duplicate_ignored",
			zap.String("promotion_id", promotionID.String()),
			zap.String("seller_id", sellerID.String()),
			zap.Int64("budget", budget),
			zap.String("idempotency_key", idempotencyKey),
		)
		existingID, lookupErr := pr.GetHolderAccountID(ctx, tx, finance.AccountPromotionAllocation, sellerID, promotionID)
		if lookupErr != nil {
			return uuid.Nil, fmt.Errorf("get existing allocation account: %w", lookupErr)
		}
		return existingID, nil
	}

	promoteBalanceID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountPromoteBalance, sellerID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get/create promote balance account: %w", err)
	}
	// Lock the seller's PROMOTE_BALANCE row and verify sufficiency. The row
	// lock serializes concurrent allocations against the same balance.
	promoteBal, err := s.ledgerRepo.GetAccountBalanceForUpdate(ctx, tx, promoteBalanceID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("lock promote balance: %w", err)
	}
	if promoteBal.Int64() < budget {
		s.logger.Warn("promotion_allocation_promote_balance_insufficient",
			zap.String("promotion_id", promotionID.String()),
			zap.String("seller_id", sellerID.String()),
			zap.Int64("promote_balance", promoteBal.Int64()),
			zap.Int64("required_budget", budget),
		)
		return uuid.Nil, ErrPromoteBalanceInsufficient
	}

	allocationID, err := pr.GetOrCreateHolderAccount(ctx, tx, finance.AccountPromotionAllocation, promotionHolderType, sellerID, promotionID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get/create promotion allocation account: %w", err)
	}

	// CANONICAL SIGN: MB (liab) DR decreases, MA (liab) CR increases.
	entries := []ledgerepo.Entry{
		{AccountID: promoteBalanceID, Amount: money.New(budget)},    // DR: promote balance decreases
		{AccountID: allocationID, Amount: money.New(-budget)},      // CR: allocation increases
	}
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idempotencyKey, "promotion_allocation", promotionID, nil, nil, entries,
	); err != nil {
		return uuid.Nil, fmt.Errorf("record promotion allocation ledger: %w", err)
	}

	s.logger.Info("promotion_allocation_recorded",
		zap.String("promotion_id", promotionID.String()),
		zap.String("seller_id", sellerID.String()),
		zap.Int64("budget", budget),
		zap.String("allocation_account", allocationID.String()),
		zap.String("idempotency_key", idempotencyKey),
	)
	return allocationID, nil
}

// ============================================================================
// QUALIFIED IMPRESSION CONSUMPTION
// ============================================================================

// RecordQualifiedImpression books the ledger charge for ONE billable
// Qualified Impression against the promotion's allocation account and
// realizes it as platform revenue.
//
// Caller responsibilities:
//   - the Qualified Impression was already server-qualified and its identity
//     (qualifiedImpressionID) is unique per billable delivery in the SAME
//     db.Tx — finance-layer idempotency is the second line of defense
//   - charge is the exact integer charge for this impression from
//     finance.PromotionCharge(N, CPM); the domain computes N cumulatively
//   - the promotion must already have an allocation (created by
//     RecordPromotionAllocation) — otherwise the account is not found
//
// A charge of 0 (cumulative rounding means this impression is not yet
// billable) is a documented no-op: no money movement, not an error.
//
// Ledger movements (Σ entries = 0):
//   - PROMOTION_ALLOCATION[seller,promo] -charge
//   - PLATFORM_REVENUE                   +charge   (promotion revenue)
//
// IDEMPOTENCY: idempotency_key = "promotion_qi_<qualified_impression_id>".
// A duplicate callback for the same Qualified Impression is a no-op — the
// same impression can never be double-charged.
func (s *FinanceService) RecordQualifiedImpression(
	ctx context.Context,
	tx db.Tx,
	qualifiedImpressionID uuid.UUID,
	promotionID uuid.UUID,
	sellerID uuid.UUID,
	charge int64,
) error {
	if qualifiedImpressionID == uuid.Nil {
		return fmt.Errorf("RecordQualifiedImpression: qualified_impression_id required")
	}
	if promotionID == uuid.Nil {
		return fmt.Errorf("RecordQualifiedImpression: promotion_id required")
	}
	if sellerID == uuid.Nil {
		return fmt.Errorf("RecordQualifiedImpression: seller_id required")
	}
	if charge < 0 {
		return fmt.Errorf("RecordQualifiedImpression: charge must not be negative (got %d)", charge)
	}
	if charge == 0 {
		// Not yet billable under cumulative rounding — no money movement.
		return nil
	}

	pr, err := s.promotionRepo()
	if err != nil {
		return err
	}

	idempotencyKey := fmt.Sprintf("promotion_qi_%s", qualifiedImpressionID.String())
	exists, err := pr.TransactionExistsByKey(ctx, tx, idempotencyKey)
	if err != nil {
		return fmt.Errorf("promotion qi duplicate-check: %w", err)
	}
	if exists {
		s.logger.Info("promotion_qi_duplicate_ignored",
			zap.String("qualified_impression_id", qualifiedImpressionID.String()),
			zap.String("promotion_id", promotionID.String()),
			zap.String("seller_id", sellerID.String()),
			zap.Int64("charge", charge),
			zap.String("idempotency_key", idempotencyKey),
		)
		return nil
	}

	allocationID, err := pr.GetHolderAccountID(ctx, tx, finance.AccountPromotionAllocation, sellerID, promotionID)
	if err != nil {
		return fmt.Errorf("get promotion allocation account (promotion must be allocated first): %w", err)
	}
	allocationBal, err := s.ledgerRepo.GetAccountBalanceForUpdate(ctx, tx, allocationID)
	if err != nil {
		return fmt.Errorf("lock promotion allocation balance: %w", err)
	}
	if allocationBal.Int64() < charge {
		s.logger.Warn("promotion_qi_allocation_insufficient",
			zap.String("qualified_impression_id", qualifiedImpressionID.String()),
			zap.String("promotion_id", promotionID.String()),
			zap.String("seller_id", sellerID.String()),
			zap.Int64("allocation_balance", allocationBal.Int64()),
			zap.Int64("required_charge", charge),
		)
		return ErrPromotionAllocationInsufficient
	}

	platformRevenueID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountPlatformRevenue)
	if err != nil {
		return fmt.Errorf("get platform revenue account: %w", err)
	}

	// CANONICAL SIGN: MA (liab) DR decreases, PR (rev) CR increases.
	entries := []ledgerepo.Entry{
		{AccountID: allocationID, Amount: money.New(charge)},       // DR: allocation decreases
		{AccountID: platformRevenueID, Amount: money.New(-charge)}, // CR: revenue increases
	}
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idempotencyKey, "promotion_qi", qualifiedImpressionID, nil, nil, entries,
	); err != nil {
		return fmt.Errorf("record promotion qi ledger: %w", err)
	}

	s.logger.Info("promotion_qi_recorded",
		zap.String("qualified_impression_id", qualifiedImpressionID.String()),
		zap.String("promotion_id", promotionID.String()),
		zap.String("seller_id", sellerID.String()),
		zap.Int64("charge", charge),
		zap.String("idempotency_key", idempotencyKey),
	)
	return nil
}

// ============================================================================
// PROMOTION ALLOCATION RELEASE (finalization)
// ============================================================================

// RecordPromotionAllocationRelease books the return of a promotion's unused
// allocation to the seller's Promote Balance at canonical finalization.
//
// Caller responsibilities:
//   - promotion finalization already locked/validated in the SAME db.Tx
//   - amount is the remaining allocation (computed from the allocation
//     account balance after the last Qualified Impression charge), so the
//     release is exact-once per finalization
//
// Ledger movements (Σ entries = 0):
//   - PROMOTION_ALLOCATION[seller,promo] -amount
//   - PROMOTE_BALANCE[seller]            +amount
//
// Duplicate release attempts (worker retry, race between stop and planned
// finish) are prevented by the idempotency key
// "promotion_allocation_release_<promotion_id>" — a finalization that already
// released cannot release again. A second call with amount 0 is also a
// no-op. The allocation account can never go negative (DB CHECK + pre-check).
func (s *FinanceService) RecordPromotionAllocationRelease(
	ctx context.Context,
	tx db.Tx,
	promotionID uuid.UUID,
	sellerID uuid.UUID,
	amount int64,
) error {
	if promotionID == uuid.Nil {
		return fmt.Errorf("RecordPromotionAllocationRelease: promotion_id required")
	}
	if sellerID == uuid.Nil {
		return fmt.Errorf("RecordPromotionAllocationRelease: seller_id required")
	}
	if amount < 0 {
		return fmt.Errorf("RecordPromotionAllocationRelease: amount must not be negative (got %d)", amount)
	}
	if amount == 0 {
		// Nothing left to release — no money movement.
		return nil
	}

	pr, err := s.promotionRepo()
	if err != nil {
		return err
	}

	idempotencyKey := fmt.Sprintf("promotion_allocation_release_%s", promotionID.String())
	exists, err := pr.TransactionExistsByKey(ctx, tx, idempotencyKey)
	if err != nil {
		return fmt.Errorf("promotion allocation release duplicate-check: %w", err)
	}
	if exists {
		s.logger.Info("promotion_allocation_release_duplicate_ignored",
			zap.String("promotion_id", promotionID.String()),
			zap.String("seller_id", sellerID.String()),
			zap.Int64("amount", amount),
			zap.String("idempotency_key", idempotencyKey),
		)
		return nil
	}

	allocationID, err := pr.GetHolderAccountID(ctx, tx, finance.AccountPromotionAllocation, sellerID, promotionID)
	if err != nil {
		return fmt.Errorf("get promotion allocation account (promotion must be allocated first): %w", err)
	}
	allocationBal, err := s.ledgerRepo.GetAccountBalanceForUpdate(ctx, tx, allocationID)
	if err != nil {
		return fmt.Errorf("lock promotion allocation balance: %w", err)
	}
	if allocationBal.Int64() < amount {
		s.logger.Warn("promotion_allocation_release_exceeds_remaining",
			zap.String("promotion_id", promotionID.String()),
			zap.String("seller_id", sellerID.String()),
			zap.Int64("allocation_balance", allocationBal.Int64()),
			zap.Int64("release_amount", amount),
		)
		return ErrPromotionAllocationInsufficient
	}

	promoteBalanceID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountPromoteBalance, sellerID)
	if err != nil {
		return fmt.Errorf("get/create promote balance account: %w", err)
	}

	// CANONICAL SIGN: MA (liab) DR decreases, MB (liab) CR increases.
	entries := []ledgerepo.Entry{
		{AccountID: allocationID, Amount: money.New(amount)},       // DR: allocation decreases
		{AccountID: promoteBalanceID, Amount: money.New(-amount)}, // CR: promote balance increases
	}
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idempotencyKey, "promotion_allocation_release", promotionID, nil, nil, entries,
	); err != nil {
		return fmt.Errorf("record promotion allocation release ledger: %w", err)
	}

	s.logger.Info("promotion_allocation_release_recorded",
		zap.String("promotion_id", promotionID.String()),
		zap.String("seller_id", sellerID.String()),
		zap.Int64("amount", amount),
		zap.String("idempotency_key", idempotencyKey),
	)
	return nil
}
