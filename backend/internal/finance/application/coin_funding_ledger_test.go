package application

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	ledgerepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// ============================================================================
// COIN FUNDING — PLATFORM-OWNED BENEFIT COUNTERPART CONTRACT
//
// Labuda Coins are platform-owned usage rights / loyalty benefits: not money,
// not user wallet, not seller money, not a Labuda payable to users, and their
// consumption moves no cash. Honoring K is therefore a platform-owned benefit
// absorption, so the canonical counterpart is PLATFORM_COIN_BENEFIT — never
// PLATFORM_BANK (cash), PLATFORM_REVENUE (income), PROMOTE_BALANCE /
// PROMOTION_ALLOCATION (seller money), or any user balance.
// ============================================================================

// coinFundingLedgerRepo is a minimal in-package LedgerRepository double: it
// hands out one account ID per account type, records the posted entries, and
// panics on an unbalanced transaction exactly like the real LedgerRepository
// (infrastructure/repository/ledger_repository.go).
type coinFundingLedgerRepo struct {
	accounts map[string]uuid.UUID
	entries  []ledgerepo.Entry
	refType  string
	key      string
}

func newCoinFundingLedgerRepo() *coinFundingLedgerRepo {
	return &coinFundingLedgerRepo{accounts: map[string]uuid.UUID{}}
}

func (r *coinFundingLedgerRepo) accountID(accountType string) uuid.UUID {
	if id, ok := r.accounts[accountType]; ok {
		return id
	}
	id := uuid.New()
	r.accounts[accountType] = id
	return id
}

func (r *coinFundingLedgerRepo) typeOf(id uuid.UUID) string {
	for accountType, accountID := range r.accounts {
		if accountID == id {
			return accountType
		}
	}
	return ""
}

func (r *coinFundingLedgerRepo) amountsByType() map[string]int64 {
	out := map[string]int64{}
	for _, e := range r.entries {
		out[r.typeOf(e.AccountID)] += e.Amount.Int64()
	}
	return out
}

func (r *coinFundingLedgerRepo) CreateTransaction(
	_ context.Context, _ db.Tx,
	idempotencyKey, referenceType string, _ uuid.UUID,
	_, _ *uuid.UUID,
	entries []ledgerepo.Entry,
) error {
	var total int64
	for _, e := range entries {
		total += e.Amount.Int64()
	}
	if total != 0 {
		panic(fmt.Sprintf("ledger: unbalanced transaction, total=%d", total))
	}
	r.entries = append([]ledgerepo.Entry(nil), entries...)
	r.key, r.refType = idempotencyKey, referenceType
	return nil
}

func (r *coinFundingLedgerRepo) GetSystemAccountID(_ context.Context, _ db.Tx, accountType string) (uuid.UUID, error) {
	return r.accountID(accountType), nil
}

func (r *coinFundingLedgerRepo) GetUserAccountID(_ context.Context, _ db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error) {
	return r.accountID(accountType + ":" + userID.String()), nil
}

func (r *coinFundingLedgerRepo) GetOrCreateUserAccount(_ context.Context, _ db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error) {
	return r.accountID(accountType + ":" + userID.String()), nil
}

func (r *coinFundingLedgerRepo) GetAccountBalance(_ context.Context, _ db.Tx, _ uuid.UUID) (money.Money, error) {
	return money.Zero(), nil
}

func (r *coinFundingLedgerRepo) GetAccountBalanceForUpdate(ctx context.Context, tx db.Tx, accountID uuid.UUID) (money.Money, error) {
	return r.GetAccountBalance(ctx, tx, accountID)
}

func (r *coinFundingLedgerRepo) CountTransactionsByEntityID(_ context.Context, _ db.Tx, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (r *coinFundingLedgerRepo) GetTotalCreditToUserAccount(_ context.Context, _ db.Tx, _ string, _ uuid.UUID) (int64, error) {
	return 0, nil
}

var coinFundingForbiddenAccounts = []string{
	finance.AccountPlatformBank,
	finance.AccountPlatformRevenue,
	finance.AccountPromoteBalance,
	finance.AccountPromotionAllocation,
	finance.AccountSellerPayable,
	finance.AccountBuyerRefundable,
	finance.AccountUserServiceCredit,
	finance.AccountWithdrawalPending,
	finance.AccountWithdrawalCommitted,
}

// TestRecordCoinFunding_UsesPlatformCoinBenefitCounterpart locks the funding
// journal: DR PLATFORM_COIN_BENEFIT +K / CR GATEWAY_CLEARING -K, sum = 0, and
// no cash / revenue / seller / user account involved. The ledger double panics
// on an unbalanced set, so a non-zero sum fails the call itself.
func TestRecordCoinFunding_UsesPlatformCoinBenefitCounterpart(t *testing.T) {
	const k = int64(10_000)

	repo := newCoinFundingLedgerRepo()
	svc := &FinanceService{ledgerRepo: repo, logger: zap.NewNop()}

	if err := svc.RecordCoinFunding(context.Background(), nil, uuid.New(), uuid.New(), k); err != nil {
		t.Fatalf("RecordCoinFunding: %v", err)
	}

	if len(repo.entries) != 2 {
		t.Fatalf("coin funding entries = %d, want 2", len(repo.entries))
	}
	if repo.refType != "coin_funding" {
		t.Errorf("reference type = %q, want coin_funding", repo.refType)
	}

	var sum int64
	for _, e := range repo.entries {
		sum += e.Amount.Int64()
	}
	if sum != 0 {
		t.Errorf("coin funding journal sum = %d, want 0", sum)
	}

	amounts := repo.amountsByType()
	if got := amounts[finance.AccountPlatformCoinBenefit]; got != k {
		t.Errorf("PLATFORM_COIN_BENEFIT = %d, want +%d (DR: platform absorbs its own granted usage right)", got, k)
	}
	if got := amounts[finance.AccountGatewayClearing]; got != -k {
		t.Errorf("GATEWAY_CLEARING = %d, want -%d (CR: clearing obligation grows by K)", got, -k)
	}
	for _, forbidden := range coinFundingForbiddenAccounts {
		if _, present := amounts[forbidden]; present {
			t.Errorf("coin funding journal must not touch %s", forbidden)
		}
	}
}

// TestRecordCoinFundingReversal_IsExactInverse locks the reversal journal:
// DR GATEWAY_CLEARING +K / CR PLATFORM_COIN_BENEFIT -K, sum = 0, and an exact
// per-account negation of the funding journal.
func TestRecordCoinFundingReversal_IsExactInverse(t *testing.T) {
	const k = int64(10_000)

	repo := newCoinFundingLedgerRepo()
	svc := &FinanceService{ledgerRepo: repo, logger: zap.NewNop()}
	ctx := context.Background()

	if err := svc.RecordCoinFunding(ctx, nil, uuid.New(), uuid.New(), k); err != nil {
		t.Fatalf("RecordCoinFunding: %v", err)
	}
	funding := repo.amountsByType()

	if err := svc.RecordCoinFundingReversal(ctx, nil, uuid.New(), uuid.New(), k); err != nil {
		t.Fatalf("RecordCoinFundingReversal: %v", err)
	}
	if repo.refType != "coin_funding_reversal" {
		t.Errorf("reference type = %q, want coin_funding_reversal", repo.refType)
	}
	reversal := repo.amountsByType()

	var sum int64
	for _, e := range repo.entries {
		sum += e.Amount.Int64()
	}
	if sum != 0 {
		t.Errorf("coin funding reversal journal sum = %d, want 0", sum)
	}

	if got := reversal[finance.AccountGatewayClearing]; got != k {
		t.Errorf("GATEWAY_CLEARING = %d, want +%d (DR: clearing obligation shrinks as funding leaves)", got, k)
	}
	if got := reversal[finance.AccountPlatformCoinBenefit]; got != -k {
		t.Errorf("PLATFORM_COIN_BENEFIT = %d, want -%d (CR: absorbed benefit released)", got, -k)
	}
	if len(funding) != len(reversal) {
		t.Fatalf("funding touches %d accounts, reversal touches %d", len(funding), len(reversal))
	}
	for accountType, amount := range funding {
		if reversal[accountType] != -amount {
			t.Errorf("reversal %s = %d, want %d (exact inverse of funding)", accountType, reversal[accountType], -amount)
		}
	}
	for _, forbidden := range coinFundingForbiddenAccounts {
		if _, present := reversal[forbidden]; present {
			t.Errorf("coin funding reversal journal must not touch %s", forbidden)
		}
	}
}

// TestPlatformCoinBenefit_IsDebitNormalBenefitAccount locks the account class:
// the platform's absorbed coin benefit must be debit-normal (expense-like), so
// DR +K increases it and CR -K releases it under the class-aware balance
// formula used by the ledger repository.
func TestPlatformCoinBenefit_IsDebitNormalBenefitAccount(t *testing.T) {
	class := finance.AccountClassOf(finance.AccountPlatformCoinBenefit)
	if class != finance.ClassExpense {
		t.Errorf("AccountClassOf(%s) = %v, want ClassExpense", finance.AccountPlatformCoinBenefit, class)
	}
	if !class.DebitIncreasesBalance() {
		t.Errorf("%s must be debit-normal (DR increases the absorbed benefit)", finance.AccountPlatformCoinBenefit)
	}
	if bankClass := finance.AccountClassOf(finance.AccountPlatformBank); bankClass == class {
		t.Errorf("%s and %s must not share an account class", finance.AccountPlatformCoinBenefit, finance.AccountPlatformBank)
	}
}

// TestPlatformCoinBenefit_SingleCanonicalCounterpart proves the bootstrap
// registers exactly one canonical coin-benefit account, with no opening float
// and no competing coin-funding counterpart anywhere in the registry.
func TestPlatformCoinBenefit_SingleCanonicalCounterpart(t *testing.T) {
	config, ok := systemAccountConfigs[finance.AccountPlatformCoinBenefit]
	if !ok {
		t.Fatalf("system account registry has no entry for %s", finance.AccountPlatformCoinBenefit)
	}
	if config.accountType != finance.AccountPlatformCoinBenefit {
		t.Errorf("registered account type = %q, want %q", config.accountType, finance.AccountPlatformCoinBenefit)
	}
	if config.initialBalance != 0 {
		t.Errorf("opening balance = %d, want 0 (the benefit account carries no float)", config.initialBalance)
	}

	var coinAccounts []string
	for accountType := range systemAccountConfigs {
		if strings.Contains(accountType, "COIN") {
			coinAccounts = append(coinAccounts, accountType)
		}
	}
	if len(coinAccounts) != 1 {
		t.Errorf("system accounts claiming a coin role = %v, want exactly [%s]", coinAccounts, finance.AccountPlatformCoinBenefit)
	}
	if !strings.Contains(fmt.Sprintf("%v", coinAccounts), finance.AccountPlatformCoinBenefit) {
		t.Errorf("coin accounts %v do not include %s", coinAccounts, finance.AccountPlatformCoinBenefit)
	}
}

// TestCoinFundingMethods_NoPlatformBankResidue is a source-level guard: the two
// coin-funding methods must post to PLATFORM_COIN_BENEFIT and must not retain
// the retired PLATFORM_BANK counterpart (or any other forbidden counterpart).
func TestCoinFundingMethods_NoPlatformBankResidue(t *testing.T) {
	raw, err := os.ReadFile("finance_service.go")
	if err != nil {
		t.Fatalf("read finance_service.go: %v", err)
	}
	src := string(raw)

	signatures := []string{
		"func (s *FinanceService) RecordCoinFunding(",
		"func (s *FinanceService) RecordCoinFundingReversal(",
	}
	forbidden := []string{
		"AccountPlatformBank",
		"AccountPlatformRevenue",
		"AccountPromoteBalance",
		"AccountPromotionAllocation",
		"AccountSellerPayable",
		"AccountBuyerRefundable",
	}

	for _, signature := range signatures {
		body := coinFundingMethodBody(t, src, signature)
		if !strings.Contains(body, "finance.AccountPlatformCoinBenefit") {
			t.Errorf("RESIDUE: %s must post to finance.AccountPlatformCoinBenefit", signature)
		}
		for _, symbol := range forbidden {
			if strings.Contains(body, symbol) {
				t.Errorf("RESIDUE: %s must not reference %s", signature, symbol)
			}
		}
	}
}

// coinFundingMethodBody returns the source of one method: from its signature up
// to the next top-level function declaration.
func coinFundingMethodBody(t *testing.T, src, signature string) string {
	t.Helper()
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("signature not found in finance_service.go: %s", signature)
	}
	rest := src[start:]
	if end := strings.Index(rest[1:], "\nfunc "); end >= 0 {
		rest = rest[:end+1]
	}
	return rest
}
