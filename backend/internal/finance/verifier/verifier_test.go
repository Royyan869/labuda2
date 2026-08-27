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


