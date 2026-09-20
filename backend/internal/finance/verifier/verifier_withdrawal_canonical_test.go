package verifier

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
)

func newWD(id, seller uuid.UUID, amount, fee int64, status string) Withdrawal {
	return Withdrawal{ID: id, SellerID: seller, Amount: amount, FeeAmount: fee, Status: status}
}

// seed funding: BANK_SETTLEMENT -> SELLER_PAYABLE to give seller initial balance
// Returns accounts and the funding transaction+entries with correct BalanceAfter.
// Bank opening 9e15, seller opening 0.
func seedSellerFunds(sellerID uuid.UUID, sellerAccID, bankAccID uuid.UUID, amount int64) (Account, Account, LedgerTransaction, []LedgerEntry) {
	const bankOpen = int64(9_000_000_000_000_000)
	bankAcc := Account{ID: bankAccID, AccountType: finance.AccountBankSettlement, Balance: bankOpen - amount}
	sellerAcc := Account{ID: sellerAccID, UserID: &sellerID, AccountType: finance.AccountSellerPayable, Balance: amount}
	// we will set tx and entries separately; caller will merge
	return bankAcc, sellerAcc, LedgerTransaction{}, nil
}

// Helper to assert verifier result
func assertVerify(t *testing.T, snap *Snapshot, mode Mode, wantFail bool, wantCode string) {
	t.Helper()
	report := Verify(snap, mode)
	hasCode := false
	for _, sec := range report.Sections {
		for _, f := range sec.Findings {
			if f.Code == wantCode {
				hasCode = true
			}
		}
	}
	if wantFail && !report.HasFailures() {
		t.Fatalf("expected failure with code %q but got PASS: %s", wantCode, report.Format("test"))
	}
	if !wantFail && report.HasFailures() {
		t.Fatalf("expected PASS but got FAIL: %s", report.Format("test"))
	}
	if wantCode != "" && wantFail && !hasCode {
		t.Fatalf("expected code %q not found: %s", wantCode, report.Format("test"))
	}
	if wantCode != "" && !wantFail && hasCode {
		t.Fatalf("unexpected code %q found in PASS: %s", wantCode, report.Format("test"))
	}
}

// A: REQUESTED with correct withdrawal_request -> PASS
func TestVerifier_RequestWithCorrectRequest_Pass(t *testing.T) {
	wid := uuid.New()
	sid := uuid.New()
	pendingID := uuid.New()
	committedID := uuid.New()
	bankID := uuid.New()
	pbID := uuid.New()
	sellerAccID := uuid.New()
	gatewayID := uuid.New()

	amount := int64(50000)
	fee := int64(1000)
	net := amount // for request, fee not split yet

	const bankOpen = int64(9_000_000_000_000_000)
	const pbOpen = int64(9_000_000_000_000_000)

	// Funding tx to give seller 50000: BANK_SETTLEMENT debit 50000 -> SELLER_PAYABLE credit 50000
	fundTxID := uuid.New()
	// Request tx: SELLER_PAYABLE debit 50000 -> PENDING credit 50000
	reqTxID := uuid.New()

	snap := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: bankOpen - amount},
			{ID: sellerAccID, UserID: &sid, AccountType: finance.AccountSellerPayable, Balance: 0},
			{ID: pendingID, AccountType: finance.AccountWithdrawalPending, Balance: amount},
			{ID: committedID, AccountType: finance.AccountWithdrawalCommitted, Balance: 0},
			{ID: pbID, AccountType: finance.AccountPlatformBank, Balance: pbOpen},
			{ID: gatewayID, AccountType: finance.AccountGatewayClearing, Balance: 0},
			{ID: uuid.New(), AccountType: finance.AccountPlatformRevenue, Balance: 0},
		},
		Withdrawals: []Withdrawal{newWD(wid, sid, amount, fee, "REQUESTED")},
		Transactions: []LedgerTransaction{
			{ID: fundTxID, ReferenceType: "seed_funding", CreatedAt: 0, TotalDebit: amount, TotalCredit: amount},
			{ID: reqTxID, ReferenceType: "withdrawal_request", ReferenceID: &wid, CreatedAt: 1, TotalDebit: amount, TotalCredit: amount},
		},
		Entries: []LedgerEntry{
			// funding: BANK_SETTLEMENT debit (reserve decreases) -> SELLER_PAYABLE credit (seller increases)
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: bankID, EntryType: "debit", Amount: amount, BalanceAfter: bankOpen - amount, CreatedAt: 0, RowOrder: "(0,1)"},
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: sellerAccID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 0, RowOrder: "(0,2)"},
			// request: SELLER_PAYABLE debit -> PENDING credit
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: sellerAccID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 1, RowOrder: "(0,3)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: pendingID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 1, RowOrder: "(0,4)"},
		},
	}
	// For request, we also need to handle gatewayID etc but not touched
	_ = net
	report := Verify(snap, ModeStrict)
	if report.HasFailures() {
		t.Fatalf("expected PASS for REQUESTED with correct request: %s", report.Format("A"))
	}
}

// B: PROCESSING with correct withdrawal_commit -> PASS
func TestVerifier_ProcessingWithCorrectCommit_Pass(t *testing.T) {
	wid := uuid.New()
	sid := uuid.New()
	pendingID := uuid.New()
	committedID := uuid.New()
	bankID := uuid.New()
	sellerAccID := uuid.New()
	amount := int64(50000)
	fee := int64(1000)
	const bankOpen = int64(9_000_000_000_000_000)
	const pbOpen = int64(9_000_000_000_000_000)
	fundTxID := uuid.New()
	reqTxID := uuid.New()
	commitTxID := uuid.New()
	snap := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: bankOpen - amount},
			{ID: sellerAccID, UserID: &sid, AccountType: finance.AccountSellerPayable, Balance: 0},
			{ID: pendingID, AccountType: finance.AccountWithdrawalPending, Balance: 0},
			{ID: committedID, AccountType: finance.AccountWithdrawalCommitted, Balance: amount},
			{ID: uuid.New(), AccountType: finance.AccountPlatformBank, Balance: pbOpen},
			{ID: uuid.New(), AccountType: finance.AccountGatewayClearing, Balance: 0},
			{ID: uuid.New(), AccountType: finance.AccountPlatformRevenue, Balance: 0},
		},
		Withdrawals: []Withdrawal{newWD(wid, sid, amount, fee, "PROCESSING")},
		Transactions: []LedgerTransaction{
			{ID: fundTxID, ReferenceType: "seed_funding", CreatedAt: 0, TotalDebit: amount, TotalCredit: amount},
			{ID: reqTxID, ReferenceType: "withdrawal_request", ReferenceID: &wid, CreatedAt: 1, TotalDebit: amount, TotalCredit: amount},
			{ID: commitTxID, ReferenceType: "withdrawal_commit", ReferenceID: &wid, CreatedAt: 2, TotalDebit: amount, TotalCredit: amount},
		},
		Entries: []LedgerEntry{
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: bankID, EntryType: "debit", Amount: amount, BalanceAfter: bankOpen - amount, CreatedAt: 0, RowOrder: "(0,1)"},
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: sellerAccID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 0, RowOrder: "(0,2)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: sellerAccID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 1, RowOrder: "(0,3)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: pendingID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 1, RowOrder: "(0,4)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: pendingID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 2, RowOrder: "(0,5)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: committedID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 2, RowOrder: "(0,6)"},
		},
	}
	report := Verify(snap, ModeStrict)
	if report.HasFailures() {
		t.Fatalf("expected PASS for PROCESSING with correct commit: %s", report.Format("B"))
	}
}

// C: SETTLED with correct WITHDRAWAL_SETTLE -> PASS
func TestVerifier_SettledWithCorrectSettle_Pass(t *testing.T) {
	wid := uuid.New()
	sid := uuid.New()
	pendingID := uuid.New()
	committedID := uuid.New()
	bankID := uuid.New()
	pbID := uuid.New()
	prID := uuid.New()
	sellerAccID := uuid.New()
	amount := int64(100000)
	fee := int64(1000)
	net := amount - fee
	const bankOpen = int64(9_000_000_000_000_000)
	const pbOpen = int64(9_000_000_000_000_000)
	fundTxID := uuid.New()
	reqTxID := uuid.New()
	commitTxID := uuid.New()
	settleTxID := uuid.New()
	snap := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: bankOpen - amount},
			{ID: sellerAccID, UserID: &sid, AccountType: finance.AccountSellerPayable, Balance: 0},
			{ID: pendingID, AccountType: finance.AccountWithdrawalPending, Balance: 0},
			{ID: committedID, AccountType: finance.AccountWithdrawalCommitted, Balance: 0},
			{ID: pbID, AccountType: finance.AccountPlatformBank, Balance: pbOpen - net},
			{ID: prID, AccountType: finance.AccountPlatformRevenue, Balance: fee},
			{ID: uuid.New(), AccountType: finance.AccountGatewayClearing, Balance: 0},
		},
		Withdrawals: []Withdrawal{newWD(wid, sid, amount, fee, "SETTLED")},
		Transactions: []LedgerTransaction{
			{ID: fundTxID, ReferenceType: "seed_funding", CreatedAt: 0, TotalDebit: amount, TotalCredit: amount},
			{ID: reqTxID, ReferenceType: "withdrawal_request", ReferenceID: &wid, CreatedAt: 1, TotalDebit: amount, TotalCredit: amount},
			{ID: commitTxID, ReferenceType: "withdrawal_commit", ReferenceID: &wid, CreatedAt: 2, TotalDebit: amount, TotalCredit: amount},
			{ID: settleTxID, ReferenceType: "WITHDRAWAL_SETTLE", ReferenceID: &wid, CreatedAt: 3, TotalDebit: amount, TotalCredit: amount},
		},
		Entries: []LedgerEntry{
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: bankID, EntryType: "debit", Amount: amount, BalanceAfter: bankOpen - amount, CreatedAt: 0, RowOrder: "(0,1)"},
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: sellerAccID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 0, RowOrder: "(0,2)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: sellerAccID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 1, RowOrder: "(0,3)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: pendingID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 1, RowOrder: "(0,4)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: pendingID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 2, RowOrder: "(0,5)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: committedID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 2, RowOrder: "(0,6)"},
			{ID: uuid.New(), TransactionID: settleTxID, AccountID: committedID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 3, RowOrder: "(0,7)"},
			{ID: uuid.New(), TransactionID: settleTxID, AccountID: pbID, EntryType: "credit", Amount: net, BalanceAfter: pbOpen - net, CreatedAt: 3, RowOrder: "(0,8)"},
			{ID: uuid.New(), TransactionID: settleTxID, AccountID: prID, EntryType: "credit", Amount: fee, BalanceAfter: fee, CreatedAt: 3, RowOrder: "(0,9)"},
		},
	}
	report := Verify(snap, ModeStrict)
	if report.HasFailures() {
		t.Fatalf("expected PASS for SETTLED with correct settle: %s", report.Format("C"))
	}
}

// D: SETTLED without WITHDRAWAL_SETTLE -> FAIL
func TestVerifier_SettledWithoutSettle_Fail(t *testing.T) {
	wid := uuid.New()
	sid := uuid.New()
	pendingID := uuid.New()
	committedID := uuid.New()
	bankID := uuid.New()
	sellerAccID := uuid.New()
	amount := int64(50000)
	fee := int64(1000)
	const bankOpen = int64(9_000_000_000_000_000)
	fundTxID := uuid.New()
	reqTxID := uuid.New()
	commitTxID := uuid.New()
	snap := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: bankOpen - amount},
			{ID: sellerAccID, UserID: &sid, AccountType: finance.AccountSellerPayable, Balance: 0},
			{ID: pendingID, AccountType: finance.AccountWithdrawalPending, Balance: 0},
			{ID: committedID, AccountType: finance.AccountWithdrawalCommitted, Balance: amount},
			{ID: uuid.New(), AccountType: finance.AccountPlatformBank, Balance: 9_000_000_000_000_000},
			{ID: uuid.New(), AccountType: finance.AccountPlatformRevenue, Balance: 0},
			{ID: uuid.New(), AccountType: finance.AccountGatewayClearing, Balance: 0},
		},
		Withdrawals: []Withdrawal{newWD(wid, sid, amount, fee, "SETTLED")},
		Transactions: []LedgerTransaction{
			{ID: fundTxID, ReferenceType: "seed_funding", CreatedAt: 0, TotalDebit: amount, TotalCredit: amount},
			{ID: reqTxID, ReferenceType: "withdrawal_request", ReferenceID: &wid, CreatedAt: 1, TotalDebit: amount, TotalCredit: amount},
			{ID: commitTxID, ReferenceType: "withdrawal_commit", ReferenceID: &wid, CreatedAt: 2, TotalDebit: amount, TotalCredit: amount},
		},
		Entries: []LedgerEntry{
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: bankID, EntryType: "debit", Amount: amount, BalanceAfter: bankOpen - amount, CreatedAt: 0, RowOrder: "(0,1)"},
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: sellerAccID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 0, RowOrder: "(0,2)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: sellerAccID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 1, RowOrder: "(0,3)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: pendingID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 1, RowOrder: "(0,4)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: pendingID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 2, RowOrder: "(0,5)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: committedID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 2, RowOrder: "(0,6)"},
		},
	}
	assertVerify(t, snap, ModeStrict, true, "withdrawal_settle_count")
}

// E: SETTLED with inverted settlement direction -> FAIL
func TestVerifier_SettledWithInvertedDirection_Fail(t *testing.T) {
	wid := uuid.New()
	sid := uuid.New()
	pendingID := uuid.New()
	committedID := uuid.New()
	bankID := uuid.New()
	pbID := uuid.New()
	prID := uuid.New()
	sellerAccID := uuid.New()
	amount := int64(100000)
	fee := int64(1000)
	net := amount - fee
	const bankOpen = int64(9_000_000_000_000_000)
	const pbOpen = int64(9_000_000_000_000_000)
	fundTxID := uuid.New()
	reqTxID := uuid.New()
	commitTxID := uuid.New()
	settleTxID := uuid.New()
	// Inverted: WC credit (+amount increases committed), PB debit (+net), PR debit (+fee) — still balanced but wrong direction
	snap := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: bankOpen - amount},
			{ID: sellerAccID, UserID: &sid, AccountType: finance.AccountSellerPayable, Balance: 0},
			{ID: pendingID, AccountType: finance.AccountWithdrawalPending, Balance: 0},
			{ID: committedID, AccountType: finance.AccountWithdrawalCommitted, Balance: amount * 2}, // inverted would make committed 2*amount
			{ID: pbID, AccountType: finance.AccountPlatformBank, Balance: pbOpen + net},
			{ID: prID, AccountType: finance.AccountPlatformRevenue, Balance: fee}, // but direction wrong, stored still fee? actually inverted pr debit would be -fee signed => balance -fee, but we set fee to pass funding? Let's set to make double-entry pass but direction fail
			{ID: uuid.New(), AccountType: finance.AccountGatewayClearing, Balance: 0},
		},
		Withdrawals: []Withdrawal{newWD(wid, sid, amount, fee, "SETTLED")},
		Transactions: []LedgerTransaction{
			{ID: fundTxID, ReferenceType: "seed_funding", CreatedAt: 0, TotalDebit: amount, TotalCredit: amount},
			{ID: reqTxID, ReferenceType: "withdrawal_request", ReferenceID: &wid, CreatedAt: 1, TotalDebit: amount, TotalCredit: amount},
			{ID: commitTxID, ReferenceType: "withdrawal_commit", ReferenceID: &wid, CreatedAt: 2, TotalDebit: amount, TotalCredit: amount},
			{ID: settleTxID, ReferenceType: "WITHDRAWAL_SETTLE", ReferenceID: &wid, CreatedAt: 3, TotalDebit: amount, TotalCredit: amount},
		},
		Entries: []LedgerEntry{
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: bankID, EntryType: "debit", Amount: amount, BalanceAfter: bankOpen - amount, CreatedAt: 0, RowOrder: "(0,1)"},
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: sellerAccID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 0, RowOrder: "(0,2)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: sellerAccID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 1, RowOrder: "(0,3)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: pendingID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 1, RowOrder: "(0,4)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: pendingID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 2, RowOrder: "(0,5)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: committedID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 2, RowOrder: "(0,6)"},
			// inverted settle: WC credit (should be debit), PB debit (should be credit), PR debit (should be credit)
			{ID: uuid.New(), TransactionID: settleTxID, AccountID: committedID, EntryType: "credit", Amount: amount, BalanceAfter: amount * 2, CreatedAt: 3, RowOrder: "(0,7)"},
			{ID: uuid.New(), TransactionID: settleTxID, AccountID: pbID, EntryType: "debit", Amount: net, BalanceAfter: pbOpen + net, CreatedAt: 3, RowOrder: "(0,8)"},
			{ID: uuid.New(), TransactionID: settleTxID, AccountID: prID, EntryType: "debit", Amount: fee, BalanceAfter: fee, CreatedAt: 3, RowOrder: "(0,9)"},
		},
	}
	// We expect verifier to catch direction mismatch via withdrawal_settle_wc_direction
	report := Verify(snap, ModeStrict)
	hasDirection := false
	for _, sec := range report.Sections {
		for _, f := range sec.Findings {
			if f.Code == "withdrawal_settle_wc_direction" || f.Code == "withdrawal_settle_pb_pr_direction" {
				hasDirection = true
			}
		}
	}
	if !hasDirection {
		t.Fatalf("expected direction failure for inverted settle, got: %s", report.Format("E"))
	}
}

// F: FAILED_FINAL with correct WITHDRAWAL_FAIL_RETURN -> PASS
func TestVerifier_FailedFinalWithCorrectReturn_Pass(t *testing.T) {
	wid := uuid.New()
	sid := uuid.New()
	pendingID := uuid.New()
	committedID := uuid.New()
	bankID := uuid.New()
	sellerAccID := uuid.New()
	amount := int64(75000)
	fee := int64(1000)
	const bankOpen = int64(9_000_000_000_000_000)
	fundTxID := uuid.New()
	reqTxID := uuid.New()
	commitTxID := uuid.New()
	failTxID := uuid.New()
	snap := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: bankOpen - amount},
			{ID: sellerAccID, UserID: &sid, AccountType: finance.AccountSellerPayable, Balance: amount},
			{ID: pendingID, AccountType: finance.AccountWithdrawalPending, Balance: 0},
			{ID: committedID, AccountType: finance.AccountWithdrawalCommitted, Balance: 0},
			{ID: uuid.New(), AccountType: finance.AccountPlatformBank, Balance: 9_000_000_000_000_000},
			{ID: uuid.New(), AccountType: finance.AccountPlatformRevenue, Balance: 0},
			{ID: uuid.New(), AccountType: finance.AccountGatewayClearing, Balance: 0},
		},
		Withdrawals: []Withdrawal{newWD(wid, sid, amount, fee, "FAILED_FINAL")},
		Transactions: []LedgerTransaction{
			{ID: fundTxID, ReferenceType: "seed_funding", CreatedAt: 0, TotalDebit: amount, TotalCredit: amount},
			{ID: reqTxID, ReferenceType: "withdrawal_request", ReferenceID: &wid, CreatedAt: 1, TotalDebit: amount, TotalCredit: amount},
			{ID: commitTxID, ReferenceType: "withdrawal_commit", ReferenceID: &wid, CreatedAt: 2, TotalDebit: amount, TotalCredit: amount},
			{ID: failTxID, ReferenceType: "WITHDRAWAL_FAIL_RETURN", ReferenceID: &wid, CreatedAt: 3, TotalDebit: amount, TotalCredit: amount},
		},
		Entries: []LedgerEntry{
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: bankID, EntryType: "debit", Amount: amount, BalanceAfter: bankOpen - amount, CreatedAt: 0, RowOrder: "(0,1)"},
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: sellerAccID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 0, RowOrder: "(0,2)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: sellerAccID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 1, RowOrder: "(0,3)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: pendingID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 1, RowOrder: "(0,4)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: pendingID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 2, RowOrder: "(0,5)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: committedID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 2, RowOrder: "(0,6)"},
			{ID: uuid.New(), TransactionID: failTxID, AccountID: committedID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 3, RowOrder: "(0,7)"},
			{ID: uuid.New(), TransactionID: failTxID, AccountID: sellerAccID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 3, RowOrder: "(0,8)"},
		},
	}
	report := Verify(snap, ModeStrict)
	if report.HasFailures() {
		t.Fatalf("expected PASS for FAILED_FINAL with correct return: %s", report.Format("F"))
	}
}

// G: FAILED_FINAL without WITHDRAWAL_FAIL_RETURN -> FAIL
func TestVerifier_FailedFinalWithoutReturn_Fail(t *testing.T) {
	wid := uuid.New()
	sid := uuid.New()
	pendingID := uuid.New()
	committedID := uuid.New()
	bankID := uuid.New()
	sellerAccID := uuid.New()
	amount := int64(75000)
	fee := int64(1000)
	const bankOpen = int64(9_000_000_000_000_000)
	fundTxID := uuid.New()
	reqTxID := uuid.New()
	commitTxID := uuid.New()
	snap := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: bankOpen - amount},
			{ID: sellerAccID, UserID: &sid, AccountType: finance.AccountSellerPayable, Balance: 0},
			{ID: pendingID, AccountType: finance.AccountWithdrawalPending, Balance: 0},
			{ID: committedID, AccountType: finance.AccountWithdrawalCommitted, Balance: amount},
			{ID: uuid.New(), AccountType: finance.AccountPlatformBank, Balance: 9_000_000_000_000_000},
			{ID: uuid.New(), AccountType: finance.AccountPlatformRevenue, Balance: 0},
			{ID: uuid.New(), AccountType: finance.AccountGatewayClearing, Balance: 0},
		},
		Withdrawals: []Withdrawal{newWD(wid, sid, amount, fee, "FAILED_FINAL")},
		Transactions: []LedgerTransaction{
			{ID: fundTxID, ReferenceType: "seed_funding", CreatedAt: 0, TotalDebit: amount, TotalCredit: amount},
			{ID: reqTxID, ReferenceType: "withdrawal_request", ReferenceID: &wid, CreatedAt: 1, TotalDebit: amount, TotalCredit: amount},
			{ID: commitTxID, ReferenceType: "withdrawal_commit", ReferenceID: &wid, CreatedAt: 2, TotalDebit: amount, TotalCredit: amount},
		},
		Entries: []LedgerEntry{
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: bankID, EntryType: "debit", Amount: amount, BalanceAfter: bankOpen - amount, CreatedAt: 0, RowOrder: "(0,1)"},
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: sellerAccID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 0, RowOrder: "(0,2)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: sellerAccID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 1, RowOrder: "(0,3)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: pendingID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 1, RowOrder: "(0,4)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: pendingID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 2, RowOrder: "(0,5)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: committedID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 2, RowOrder: "(0,6)"},
		},
	}
	assertVerify(t, snap, ModeStrict, true, "withdrawal_fail_return_count")
}

// H: FAILED_FINAL with inverted WC -amount / SP +amount -> FAIL
func TestVerifier_FailedFinalWithInvertedReturn_Fail(t *testing.T) {
	wid := uuid.New()
	sid := uuid.New()
	pendingID := uuid.New()
	committedID := uuid.New()
	bankID := uuid.New()
	sellerAccID := uuid.New()
	amount := int64(75000)
	fee := int64(1000)
	const bankOpen = int64(9_000_000_000_000_000)
	fundTxID := uuid.New()
	reqTxID := uuid.New()
	commitTxID := uuid.New()
	failTxID := uuid.New()
	snap := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: bankOpen - amount},
			{ID: sellerAccID, UserID: &sid, AccountType: finance.AccountSellerPayable, Balance: amount}, // inverted would make payable 0? Let's set to capture direction fail
			{ID: pendingID, AccountType: finance.AccountWithdrawalPending, Balance: 0},
			{ID: committedID, AccountType: finance.AccountWithdrawalCommitted, Balance: amount * 2},
			{ID: uuid.New(), AccountType: finance.AccountPlatformBank, Balance: 9_000_000_000_000_000},
			{ID: uuid.New(), AccountType: finance.AccountPlatformRevenue, Balance: 0},
			{ID: uuid.New(), AccountType: finance.AccountGatewayClearing, Balance: 0},
		},
		Withdrawals: []Withdrawal{newWD(wid, sid, amount, fee, "FAILED_FINAL")},
		Transactions: []LedgerTransaction{
			{ID: fundTxID, ReferenceType: "seed_funding", CreatedAt: 0, TotalDebit: amount, TotalCredit: amount},
			{ID: reqTxID, ReferenceType: "withdrawal_request", ReferenceID: &wid, CreatedAt: 1, TotalDebit: amount, TotalCredit: amount},
			{ID: commitTxID, ReferenceType: "withdrawal_commit", ReferenceID: &wid, CreatedAt: 2, TotalDebit: amount, TotalCredit: amount},
			{ID: failTxID, ReferenceType: "WITHDRAWAL_FAIL_RETURN", ReferenceID: &wid, CreatedAt: 3, TotalDebit: amount, TotalCredit: amount},
		},
		Entries: []LedgerEntry{
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: bankID, EntryType: "debit", Amount: amount, BalanceAfter: bankOpen - amount, CreatedAt: 0, RowOrder: "(0,1)"},
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: sellerAccID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 0, RowOrder: "(0,2)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: sellerAccID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 1, RowOrder: "(0,3)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: pendingID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 1, RowOrder: "(0,4)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: pendingID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 2, RowOrder: "(0,5)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: committedID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 2, RowOrder: "(0,6)"},
			// inverted: WC credit, SP debit — balanced but wrong direction
			{ID: uuid.New(), TransactionID: failTxID, AccountID: committedID, EntryType: "credit", Amount: amount, BalanceAfter: amount * 2, CreatedAt: 3, RowOrder: "(0,7)"},
			{ID: uuid.New(), TransactionID: failTxID, AccountID: sellerAccID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 3, RowOrder: "(0,8)"},
		},
	}
	report := Verify(snap, ModeStrict)
	hasDirection := false
	for _, sec := range report.Sections {
		for _, f := range sec.Findings {
			if f.Code == "withdrawal_fail_return_direction" {
				hasDirection = true
			}
		}
	}
	if !hasDirection {
		t.Fatalf("expected direction failure for inverted FAILED_FINAL, got: %s", report.Format("H"))
	}
}

// I: Correct gateway failure despite two producers sharing same idempotency key -> PASS with exactly one effective transaction
func TestVerifier_FailedFinalSingleTransactionForSharedKey_Pass(t *testing.T) {
	wid := uuid.New()
	sid := uuid.New()
	pendingID := uuid.New()
	committedID := uuid.New()
	bankID := uuid.New()
	sellerAccID := uuid.New()
	amount := int64(80000)
	fee := int64(1000)
	const bankOpen = int64(9_000_000_000_000_000)
	fundTxID := uuid.New()
	reqTxID := uuid.New()
	commitTxID := uuid.New()
	failTxID := uuid.New()
	// Single effective WITHDRAWAL_FAIL_RETURN despite two producers conceptual — verifier counts 1 transaction, passes
	snap := &Snapshot{
		Accounts: []Account{
			{ID: bankID, AccountType: finance.AccountBankSettlement, Balance: bankOpen - amount},
			{ID: sellerAccID, UserID: &sid, AccountType: finance.AccountSellerPayable, Balance: amount},
			{ID: pendingID, AccountType: finance.AccountWithdrawalPending, Balance: 0},
			{ID: committedID, AccountType: finance.AccountWithdrawalCommitted, Balance: 0},
			{ID: uuid.New(), AccountType: finance.AccountPlatformBank, Balance: 9_000_000_000_000_000},
			{ID: uuid.New(), AccountType: finance.AccountPlatformRevenue, Balance: 0},
			{ID: uuid.New(), AccountType: finance.AccountGatewayClearing, Balance: 0},
		},
		Withdrawals: []Withdrawal{newWD(wid, sid, amount, fee, "FAILED_FINAL")},
		Transactions: []LedgerTransaction{
			{ID: fundTxID, ReferenceType: "seed_funding", CreatedAt: 0, TotalDebit: amount, TotalCredit: amount},
			{ID: reqTxID, ReferenceType: "withdrawal_request", ReferenceID: &wid, CreatedAt: 1, TotalDebit: amount, TotalCredit: amount},
			{ID: commitTxID, ReferenceType: "withdrawal_commit", ReferenceID: &wid, CreatedAt: 2, TotalDebit: amount, TotalCredit: amount},
			{ID: failTxID, ReferenceType: "WITHDRAWAL_FAIL_RETURN", ReferenceID: &wid, CreatedAt: 3, TotalDebit: amount, TotalCredit: amount, IdempotencyKey: "withdrawal_gateway_restore_" + wid.String()},
		},
		Entries: []LedgerEntry{
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: bankID, EntryType: "debit", Amount: amount, BalanceAfter: bankOpen - amount, CreatedAt: 0, RowOrder: "(0,1)"},
			{ID: uuid.New(), TransactionID: fundTxID, AccountID: sellerAccID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 0, RowOrder: "(0,2)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: sellerAccID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 1, RowOrder: "(0,3)"},
			{ID: uuid.New(), TransactionID: reqTxID, AccountID: pendingID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 1, RowOrder: "(0,4)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: pendingID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 2, RowOrder: "(0,5)"},
			{ID: uuid.New(), TransactionID: commitTxID, AccountID: committedID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 2, RowOrder: "(0,6)"},
			{ID: uuid.New(), TransactionID: failTxID, AccountID: committedID, EntryType: "debit", Amount: amount, BalanceAfter: 0, CreatedAt: 3, RowOrder: "(0,7)"},
			{ID: uuid.New(), TransactionID: failTxID, AccountID: sellerAccID, EntryType: "credit", Amount: amount, BalanceAfter: amount, CreatedAt: 3, RowOrder: "(0,8)"},
		},
	}
	report := Verify(snap, ModeStrict)
	if report.HasFailures() {
		t.Fatalf("expected PASS for single effective WITHDRAWAL_FAIL_RETURN with shared key: %s", report.Format("I"))
	}
}
