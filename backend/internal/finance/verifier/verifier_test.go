package verifier

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
)

func TestOpeningBalanceMetadataForBankSettlement(t *testing.T) {
	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	counterID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	txID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	report := Verify(&Snapshot{
		Accounts: []Account{
			{ID: accountID, AccountType: finance.AccountBankSettlement, Balance: 8_999_999_999_000_000},
			{ID: counterID, AccountType: finance.AccountGatewayClearing, Balance: 1_000_000},
		},
		Transactions: []LedgerTransaction{
			{ID: txID, ReferenceType: "payment_settlement", CreatedAt: 10},
		},
		Entries: []LedgerEntry{
			{
				ID:            uuid.MustParse("00000000-0000-0000-0000-000000000003"),
				TransactionID: txID,
				AccountID:     accountID,
				EntryType:     "credit",
				Amount:        1_000_000,
				BalanceAfter:  8_999_999_999_000_000,
				CreatedAt:     10,
				RowOrder:      "(0,1)",
			},
			{
				ID:            uuid.MustParse("00000000-0000-0000-0000-000000000011"),
				TransactionID: txID,
				AccountID:     counterID,
				EntryType:     "debit",
				Amount:        1_000_000,
				BalanceAfter:  1_000_000,
				CreatedAt:     10,
				RowOrder:      "(0,2)",
			},
		},
	}, ModeStrict)
	if report.HasFailures() {
		t.Fatalf("expected opening-balance metadata to stabilize bank settlement replay, got report:\n%s", report.Format("test"))
	}
}

func TestAmbiguousOrderingStrictFailsButForensicWarns(t *testing.T) {
	accountID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	counterID := uuid.MustParse("10000000-0000-0000-0000-000000000010")
	tx1 := uuid.MustParse("10000000-0000-0000-0000-000000000002")
	tx2 := uuid.MustParse("10000000-0000-0000-0000-000000000003")
	snapshot := &Snapshot{
		Accounts: []Account{
			{ID: accountID, AccountType: finance.AccountGatewayClearing, Balance: 0},
			{ID: counterID, AccountType: finance.AccountBankSettlement, Balance: 9_000_000_000_000_000},
		},
		Transactions: []LedgerTransaction{
			{ID: tx1, ReferenceType: "payment_settlement", CreatedAt: 10},
			{ID: tx2, ReferenceType: "order_release", CreatedAt: 10},
		},
		Entries: []LedgerEntry{
			{ID: uuid.MustParse("10000000-0000-0000-0000-000000000004"), TransactionID: tx1, AccountID: accountID, EntryType: "debit", Amount: 100, BalanceAfter: 100, CreatedAt: 10, RowOrder: "(0,1)"},
			{ID: uuid.MustParse("10000000-0000-0000-0000-000000000006"), TransactionID: tx1, AccountID: counterID, EntryType: "credit", Amount: 100, BalanceAfter: 8_999_999_999_999_900, CreatedAt: 10, RowOrder: "(0,1)"},
			{ID: uuid.MustParse("10000000-0000-0000-0000-000000000005"), TransactionID: tx2, AccountID: accountID, EntryType: "credit", Amount: 100, BalanceAfter: 0, CreatedAt: 10, RowOrder: "(0,2)"},
			{ID: uuid.MustParse("10000000-0000-0000-0000-000000000007"), TransactionID: tx2, AccountID: counterID, EntryType: "debit", Amount: 100, BalanceAfter: 9_000_000_000_000_000, CreatedAt: 10, RowOrder: "(0,2)"},
		},
	}
	strict := Verify(snapshot, ModeStrict)
	if !strict.HasFailures() {
		t.Fatalf("expected strict mode to fail on missing ordering primitive")
	}
	forensic := Verify(snapshot, ModeForensic)
	if forensic.HasFailures() {
		t.Fatalf("expected forensic mode to downgrade ambiguous-ordering issue, got:\n%s", forensic.Format("test"))
	}
}

func TestPaymentSettlementResidueStrictFailsForensicWarns(t *testing.T) {
	paymentID := uuid.MustParse("20000000-0000-0000-0000-000000000001")
	orderID := uuid.MustParse("20000000-0000-0000-0000-000000000002")
	cutoverTx := uuid.MustParse("20000000-0000-0000-0000-000000000003")
	counterA := uuid.MustParse("20000000-0000-0000-0000-000000000004")
	counterB := uuid.MustParse("20000000-0000-0000-0000-000000000005")
	snapshot := &Snapshot{
		Accounts: []Account{
			{ID: counterA, AccountType: finance.AccountGatewayClearing, Balance: 100},
			{ID: counterB, AccountType: finance.AccountBankSettlement, Balance: 8_999_999_999_999_900},
		},
		Payments: []Payment{
			{ID: paymentID, ReferenceType: "order", ReferenceID: orderID, Status: "settlement", GrossAmount: 100, CreatedAt: time.Unix(5, 0)},
		},
		Transactions: []LedgerTransaction{
			{ID: cutoverTx, ReferenceType: "payment_settlement", CreatedAt: 10},
		},
		Entries: []LedgerEntry{
			{ID: uuid.MustParse("20000000-0000-0000-0000-000000000006"), TransactionID: cutoverTx, AccountID: counterA, EntryType: "debit", Amount: 100, BalanceAfter: 100, CreatedAt: 10, RowOrder: "(0,1)"},
			{ID: uuid.MustParse("20000000-0000-0000-0000-000000000007"), TransactionID: cutoverTx, AccountID: counterB, EntryType: "credit", Amount: 100, BalanceAfter: 8_999_999_999_999_900, CreatedAt: 10, RowOrder: "(0,2)"},
		},
	}
	if !Verify(snapshot, ModeStrict).HasFailures() {
		t.Fatalf("expected strict mode to fail on missing historical payment settlement")
	}
	if Verify(snapshot, ModeForensic).HasFailures() {
		t.Fatalf("expected forensic mode to classify pre-cutover payment settlement gap as residue")
	}
}

// TestDoubleEntryImbalanceFailsStrict verifies that a transaction whose debit
// and credit amounts differ is always caught in strict mode. This is a
// CI-safe fixture test (no DB required).
func TestDoubleEntryImbalanceFailsStrict(t *testing.T) {
	acctA := uuid.MustParse("50000000-0000-0000-0000-000000000001")
	acctB := uuid.MustParse("50000000-0000-0000-0000-000000000002")
	txID := uuid.MustParse("50000000-0000-0000-0000-000000000003")
	snapshot := &Snapshot{
		Accounts: []Account{
			{ID: acctA, AccountType: finance.AccountGatewayClearing, Balance: 300},
			{ID: acctB, AccountType: finance.AccountBankSettlement, Balance: 8_999_999_999_999_700},
		},
		Transactions: []LedgerTransaction{
			{ID: txID, ReferenceType: "payment_settlement", CreatedAt: 1},
		},
		Entries: []LedgerEntry{
			// Debit 300, credit 200 — intentional imbalance
			{ID: uuid.MustParse("50000000-0000-0000-0000-000000000004"), TransactionID: txID, AccountID: acctA, EntryType: "debit", Amount: 300, BalanceAfter: 300, CreatedAt: 1, RowOrder: "(0,1)"},
			{ID: uuid.MustParse("50000000-0000-0000-0000-000000000005"), TransactionID: txID, AccountID: acctB, EntryType: "credit", Amount: 200, BalanceAfter: 8_999_999_999_999_700, CreatedAt: 1, RowOrder: "(0,2)"},
		},
	}
	if !Verify(snapshot, ModeStrict).HasFailures() {
		t.Fatal("expected strict verifier to flag debit/credit imbalance")
	}
	if !Verify(snapshot, ModeForensic).HasFailures() {
		t.Fatal("expected forensic verifier to flag debit/credit imbalance (not downgraded)")
	}
}

func TestPayoutOutboxCorrelationIsOptional(t *testing.T) {
	withdrawalID := uuid.MustParse("40000000-0000-0000-0000-000000000001")
	sellerID := uuid.MustParse("40000000-0000-0000-0000-000000000002")
	bankID := uuid.MustParse("40000000-0000-0000-0000-000000000008")
	pendingID := uuid.MustParse("40000000-0000-0000-0000-000000000003")
	committedID := uuid.MustParse("40000000-0000-0000-0000-000000000004")
	requestTxID := uuid.MustParse("40000000-0000-0000-0000-000000000005")
	commitTxID := uuid.MustParse("40000000-0000-0000-0000-000000000009")
	snapshot := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: 8_999_999_999_999_900},
			{ID: pendingID, AccountType: finance.AccountWithdrawalPending, Balance: 0},
			{ID: committedID, AccountType: finance.AccountWithdrawalCommitted, Balance: 100},
		},
		Withdrawals: []Withdrawal{
			{ID: withdrawalID, SellerID: sellerID, Amount: 100, Status: "approved"},
		},
		Transactions: []LedgerTransaction{
			{ID: requestTxID, ReferenceType: "withdrawal_request", ReferenceID: &withdrawalID, CreatedAt: 9},
			{ID: commitTxID, ReferenceType: "withdrawal_commit", ReferenceID: &withdrawalID, CreatedAt: 10},
		},
		Entries: []LedgerEntry{
			{ID: uuid.MustParse("40000000-0000-0000-0000-000000000006"), TransactionID: requestTxID, AccountID: bankID, EntryType: "credit", Amount: 100, BalanceAfter: 8_999_999_999_999_900, CreatedAt: 9, RowOrder: "(0,1)"},
			{ID: uuid.MustParse("40000000-0000-0000-0000-000000000007"), TransactionID: requestTxID, AccountID: pendingID, EntryType: "debit", Amount: 100, BalanceAfter: 100, CreatedAt: 9, RowOrder: "(0,2)"},
			{ID: uuid.MustParse("40000000-0000-0000-0000-000000000010"), TransactionID: commitTxID, AccountID: pendingID, EntryType: "credit", Amount: 100, BalanceAfter: 0, CreatedAt: 10, RowOrder: "(0,3)"},
			{ID: uuid.MustParse("40000000-0000-0000-0000-000000000011"), TransactionID: commitTxID, AccountID: committedID, EntryType: "debit", Amount: 100, BalanceAfter: 100, CreatedAt: 10, RowOrder: "(0,4)"},
		},
	}
	if Verify(snapshot, ModeStrict).HasFailures() {
		t.Fatalf("expected payout outbox to remain optional when no observed contract exists")
	}
}

// TestPromotionFinancialInvariants_PositiveLifecycle is a CI-safe fixture
// proving the verifier understands the full promotion ledger lifecycle
// (PROMOTION_FINANCIAL_FOUNDATION):
//
//	funding 30000        BANK_SETTLEMENT -> PROMOTE_BALANCE
//	allocation 30000     PROMOTE_BALANCE -> PROMOTION_ALLOCATION
//	QI x2 (7 + 8)        PROMOTION_ALLOCATION -> PLATFORM_REVENUE  (CPM 7500)
//	release 29985        PROMOTION_ALLOCATION -> PROMOTE_BALANCE
func TestPromotionFinancialInvariants_PositiveLifecycle(t *testing.T) {
	bankID := uuid.MustParse("60000000-0000-0000-0000-000000000001")
	promoteID := uuid.MustParse("60000000-0000-0000-0000-000000000002")
	allocID := uuid.MustParse("60000000-0000-0000-0000-000000000003")
	platformID := uuid.MustParse("60000000-0000-0000-0000-000000000004")
	sellerID := uuid.MustParse("60000000-0000-0000-0000-000000000005")
	promotionID := uuid.MustParse("60000000-0000-0000-0000-000000000006")
	fundingID := uuid.MustParse("60000000-0000-0000-0000-000000000007")
	qi1 := uuid.MustParse("60000000-0000-0000-0000-000000000008")
	qi2 := uuid.MustParse("60000000-0000-0000-0000-000000000009")

	const bankOpen = int64(9_000_000_000_000_000)
	const afterFunding = bankOpen - 30_000 // 8_999_999_999_970_000
	holderType := "promotion"

	snapshot := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: afterFunding},
			{ID: promoteID, UserID: &sellerID, AccountType: finance.AccountPromoteBalance, Balance: 29_985},
			{ID: allocID, UserID: &sellerID, AccountType: finance.AccountPromotionAllocation, HolderType: &holderType, HolderID: &promotionID, Balance: 0},
			{ID: platformID, AccountType: finance.AccountPlatformRevenue, Balance: 15},
		},
		Transactions: []LedgerTransaction{
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000010"), ReferenceType: "promote_balance_funding", ReferenceID: &fundingID, CreatedAt: 1},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000011"), ReferenceType: "promotion_allocation", ReferenceID: &promotionID, CreatedAt: 2},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000012"), ReferenceType: "promotion_qi", ReferenceID: &qi1, CreatedAt: 3},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000013"), ReferenceType: "promotion_qi", ReferenceID: &qi2, CreatedAt: 4},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000014"), ReferenceType: "promotion_allocation_release", ReferenceID: &promotionID, CreatedAt: 5},
		},
		Entries: []LedgerEntry{
			// funding: BANK_SETTLEMENT -30000 -> PROMOTE_BALANCE +30000
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000020"), TransactionID: uuid.MustParse("60000000-0000-0000-0000-000000000010"), AccountID: bankID, EntryType: "credit", Amount: 30_000, BalanceAfter: afterFunding, CreatedAt: 1, RowOrder: "(0,1)"},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000021"), TransactionID: uuid.MustParse("60000000-0000-0000-0000-000000000010"), AccountID: promoteID, EntryType: "debit", Amount: 30_000, BalanceAfter: 30_000, CreatedAt: 1, RowOrder: "(0,2)"},
			// allocation: PROMOTE_BALANCE -30000 -> PROMOTION_ALLOCATION +30000
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000022"), TransactionID: uuid.MustParse("60000000-0000-0000-0000-000000000011"), AccountID: promoteID, EntryType: "credit", Amount: 30_000, BalanceAfter: 0, CreatedAt: 2, RowOrder: "(0,1)"},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000023"), TransactionID: uuid.MustParse("60000000-0000-0000-0000-000000000011"), AccountID: allocID, EntryType: "debit", Amount: 30_000, BalanceAfter: 30_000, CreatedAt: 2, RowOrder: "(0,2)"},
			// QI #1: charge 7
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000024"), TransactionID: uuid.MustParse("60000000-0000-0000-0000-000000000012"), AccountID: allocID, EntryType: "credit", Amount: 7, BalanceAfter: 29_993, CreatedAt: 3, RowOrder: "(0,1)"},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000025"), TransactionID: uuid.MustParse("60000000-0000-0000-0000-000000000012"), AccountID: platformID, EntryType: "debit", Amount: 7, BalanceAfter: 7, CreatedAt: 3, RowOrder: "(0,2)"},
			// QI #2: charge 8
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000026"), TransactionID: uuid.MustParse("60000000-0000-0000-0000-000000000013"), AccountID: allocID, EntryType: "credit", Amount: 8, BalanceAfter: 29_985, CreatedAt: 4, RowOrder: "(0,1)"},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000027"), TransactionID: uuid.MustParse("60000000-0000-0000-0000-000000000013"), AccountID: platformID, EntryType: "debit", Amount: 8, BalanceAfter: 15, CreatedAt: 4, RowOrder: "(0,2)"},
			// release: remaining 29985 back to PROMOTE_BALANCE
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000028"), TransactionID: uuid.MustParse("60000000-0000-0000-0000-000000000014"), AccountID: allocID, EntryType: "credit", Amount: 29_985, BalanceAfter: 0, CreatedAt: 5, RowOrder: "(0,1)"},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000029"), TransactionID: uuid.MustParse("60000000-0000-0000-0000-000000000014"), AccountID: promoteID, EntryType: "debit", Amount: 29_985, BalanceAfter: 29_985, CreatedAt: 5, RowOrder: "(0,2)"},
		},
		// Phase 3 canonical facts backing the promotion_qi ledger charges:
		// the Qualified Impression Reconciliation section requires every
		// billable QI charge to map to a real promotion_qualified_impressions
		// row whose charge matches the contract's immutable CPM snapshot
		// (CPM 7500 -> charge(1)=7, charge(2)=8).
		PromotionContracts: []PromotionContract{
			{ID: promotionID, SellerID: sellerID, Status: "active", CPMRupiah: 7500, AllocationAccountID: allocID},
		},
		QualifiedImpressions: []QualifiedImpression{
			{ID: qi1, TicketID: uuid.MustParse("60000000-0000-0000-0000-00000000000A"), ContractID: promotionID, SequenceN: 1, ChargeRupiah: 7, ServerOccurredAt: time.Unix(3, 0)},
			{ID: qi2, TicketID: uuid.MustParse("60000000-0000-0000-0000-00000000000B"), ContractID: promotionID, SequenceN: 2, ChargeRupiah: 8, ServerOccurredAt: time.Unix(4, 0)},
		},
	}
	report := Verify(snapshot, ModeStrict)
	if report.HasFailures() {
		t.Fatalf("expected clean promotion lifecycle snapshot to pass, got report:\n%s", report.Format("promotion-positive"))
	}
}

// TestPromotionFinancialInvariants_TopUpMustNotBookRevenue is a CI-safe
// negative fixture: a promote_balance_funding transaction that credits
// PLATFORM_REVENUE (the exact legacy behavior the canonical contract
// prohibits — a top-up is NOT platform revenue) must fail the promotion
// section regardless of ledger balance.
func TestPromotionFinancialInvariants_TopUpMustNotBookRevenue(t *testing.T) {
	bankID := uuid.MustParse("70000000-0000-0000-0000-000000000001")
	platformID := uuid.MustParse("70000000-0000-0000-0000-000000000002")
	fundingID := uuid.MustParse("70000000-0000-0000-0000-000000000003")
	txID := uuid.MustParse("70000000-0000-0000-0000-000000000004")

	const bankOpen = int64(9_000_000_000_000_000)
	snapshot := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: bankOpen - 100},
			{ID: platformID, AccountType: finance.AccountPlatformRevenue, Balance: 100},
		},
		Transactions: []LedgerTransaction{
			{ID: txID, ReferenceType: "promote_balance_funding", ReferenceID: &fundingID, CreatedAt: 1},
		},
		Entries: []LedgerEntry{
			{ID: uuid.MustParse("70000000-0000-0000-0000-000000000005"), TransactionID: txID, AccountID: bankID, EntryType: "credit", Amount: 100, BalanceAfter: bankOpen - 100, CreatedAt: 1, RowOrder: "(0,1)"},
			{ID: uuid.MustParse("70000000-0000-0000-0000-000000000006"), TransactionID: txID, AccountID: platformID, EntryType: "debit", Amount: 100, BalanceAfter: 100, CreatedAt: 1, RowOrder: "(0,2)"},
		},
	}
	report := Verify(snapshot, ModeStrict)
	if !report.HasFailures() {
		t.Fatal("expected illegal top-up->platform revenue flow to fail the promotion section")
	}
	if !reportHasCode(report, "promotion_reference_illegal_account") {
		t.Fatalf("expected promotion_reference_illegal_account finding, got report:\n%s", report.Format("promotion-negative"))
	}
}

func reportHasCode(report Report, code string) bool {
	for _, s := range report.Sections {
		for _, f := range s.Findings {
			if f.Code == code {
				return true
			}
		}
	}
	return false
}
