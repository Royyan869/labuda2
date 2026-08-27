package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	billingentity "github.com/labuda/backend/internal/finance/billing/entity"
	ledgerepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// mockBillingLedgerRepo is a minimal mock that captures CreateTransaction calls
// and validates the double-entry invariant.
type mockBillingLedgerRepo struct {
	lastEntries       []ledgerepo.Entry
	lastIdempotencyKey string
	createTxCalled    bool
}

func (m *mockBillingLedgerRepo) GetSystemAccountID(_ context.Context, _ db.Tx, accountType string) (uuid.UUID, error) {
	// Return deterministic UUIDs per account type
	switch accountType {
	case ledgerepo.AccountGatewayClearing:
		return uuid.MustParse("00000000-0000-0000-0000-000000000001"), nil
	case ledgerepo.AccountPlatformRevenue:
		return uuid.MustParse("00000000-0000-0000-0000-000000000002"), nil
	case ledgerepo.AccountBankSettlement:
		return uuid.MustParse("00000000-0000-0000-0000-000000000003"), nil
	default:
		return uuid.Nil, nil
	}
}

func (m *mockBillingLedgerRepo) CreateTransaction(
	_ context.Context, _ db.Tx,
	idempotencyKey string, _ string, _ uuid.UUID,
	_ *uuid.UUID, _ *uuid.UUID,
	entries []ledgerepo.Entry,
) error {
	m.createTxCalled = true
	m.lastIdempotencyKey = idempotencyKey
	m.lastEntries = entries
	return nil
}

// Unused interface methods — satisfy LedgerRepository interface.
func (m *mockBillingLedgerRepo) GetAccountBalance(context.Context, db.Tx, uuid.UUID) (money.Money, error) {
	return money.Zero(), nil
}
func (m *mockBillingLedgerRepo) GetAccountBalanceForUpdate(context.Context, db.Tx, uuid.UUID) (money.Money, error) {
	return money.Zero(), nil
}
func (m *mockBillingLedgerRepo) GetUserAccountID(context.Context, db.Tx, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (m *mockBillingLedgerRepo) GetOrCreateUserAccount(context.Context, db.Tx, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (m *mockBillingLedgerRepo) CountTransactionsByEntityID(context.Context, db.Tx, uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockBillingLedgerRepo) GetTotalCreditToUserAccount(context.Context, db.Tx, string, uuid.UUID) (int64, error) {
	return 0, nil
}

// TestRecordBillingServiceRevenue_BalancedEntries verifies that the billing
// ledger transaction produces balanced double-entry (Σ entries = 0).
//
// This is the regression test for the invariant violation where only a single
// CR PLATFORM_REVENUE entry was created, which would panic in
// LedgerRepository.CreateTransaction.
func TestRecordBillingServiceRevenue_BalancedEntries(t *testing.T) {
	mock := &mockBillingLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock}

	billing := &billingentity.BillingTransaction{
		ID:          uuid.New(),
		GrossAmount: money.New(50000), // 50,000 IDR
	}

	err := svc.RecordBillingServiceRevenue(context.Background(), nil, billing)
	if err != nil {
		t.Fatalf("RecordBillingServiceRevenue returned error: %v", err)
	}

	if !mock.createTxCalled {
		t.Fatal("CreateTransaction was not called")
	}

	// Must have exactly 2 entries: DR PLATFORM_REVENUE + CR BANK_SETTLEMENT
	if len(mock.lastEntries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(mock.lastEntries))
	}

	// Verify balance: sum of all entries must equal zero
	total := money.Zero()
	for _, entry := range mock.lastEntries {
		total = total.Add(entry.Amount)
	}
	if !total.IsZero() {
		t.Fatalf("ledger entries unbalanced: sum = %d (must be 0)", total.Int64())
	}

	// Verify DR side: PLATFORM_REVENUE (+50000) — revenue increases
	platformRevenueID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	bankSettlementID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	drEntry := mock.lastEntries[0]
	if drEntry.AccountID != platformRevenueID {
		t.Errorf("entry[0] account: got %s, want PLATFORM_REVENUE %s", drEntry.AccountID, platformRevenueID)
	}
	if drEntry.Amount.Int64() != 50000 {
		t.Errorf("entry[0] amount: got %d, want +50000 (debit)", drEntry.Amount.Int64())
	}

	// Verify CR side: BANK_SETTLEMENT (-50000) — reserve drains
	crEntry := mock.lastEntries[1]
	if crEntry.AccountID != bankSettlementID {
		t.Errorf("entry[1] account: got %s, want BANK_SETTLEMENT %s", crEntry.AccountID, bankSettlementID)
	}
	if crEntry.Amount.Int64() != -50000 {
		t.Errorf("entry[1] amount: got %d, want -50000 (credit)", crEntry.Amount.Int64())
	}

	// Verify idempotency key format
	expectedKey := "billing-" + billing.ID.String()
	if mock.lastIdempotencyKey != expectedKey {
		t.Errorf("idempotency key: got %q, want %q", mock.lastIdempotencyKey, expectedKey)
	}
}

// TestRecordBillingServiceRevenue_ZeroAmount verifies that a zero-amount
// billing transaction produces balanced entries (trivially balanced).
func TestRecordBillingServiceRevenue_ZeroAmount(t *testing.T) {
	mock := &mockBillingLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock}

	billing := &billingentity.BillingTransaction{
		ID:          uuid.New(),
		GrossAmount: money.New(0),
	}

	err := svc.RecordBillingServiceRevenue(context.Background(), nil, billing)
	if err != nil {
		t.Fatalf("RecordBillingServiceRevenue returned error: %v", err)
	}

	if !mock.createTxCalled {
		t.Fatal("CreateTransaction was not called")
	}

	// Sum must be zero
	total := money.Zero()
	for _, entry := range mock.lastEntries {
		total = total.Add(entry.Amount)
	}
	if !total.IsZero() {
		t.Fatalf("ledger entries unbalanced for zero amount: sum = %d", total.Int64())
	}
}

// TestRecordBillingServiceRevenue_LargeAmount verifies balance holds for
// large amounts (near typical promotion package pricing).
func TestRecordBillingServiceRevenue_LargeAmount(t *testing.T) {
	mock := &mockBillingLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock}

	billing := &billingentity.BillingTransaction{
		ID:          uuid.New(),
		GrossAmount: money.New(999_999_999), // ~1B IDR
	}

	err := svc.RecordBillingServiceRevenue(context.Background(), nil, billing)
	if err != nil {
		t.Fatalf("RecordBillingServiceRevenue returned error: %v", err)
	}

	total := money.Zero()
	for _, entry := range mock.lastEntries {
		total = total.Add(entry.Amount)
	}
	if !total.IsZero() {
		t.Fatalf("ledger entries unbalanced for large amount: sum = %d", total.Int64())
	}

	// Verify exact amounts: DR PLATFORM_REVENUE, CR BANK_SETTLEMENT
	if mock.lastEntries[0].Amount.Int64() != 999_999_999 {
		t.Errorf("DR PLATFORM_REVENUE amount: got %d, want 999999999", mock.lastEntries[0].Amount.Int64())
	}
	if mock.lastEntries[1].Amount.Int64() != -999_999_999 {
		t.Errorf("CR BANK_SETTLEMENT amount: got %d, want -999999999", mock.lastEntries[1].Amount.Int64())
	}
}


