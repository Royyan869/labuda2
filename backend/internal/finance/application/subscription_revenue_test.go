package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	ledgerepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// TestRecordSubscriptionRevenue_BalancedEntries verifies that the subscription
// revenue ledger transaction produces balanced double-entry (Σ entries = 0).
//
// This is the canonical test for the subscription payment → ledger path.
// RecordSubscriptionRevenue is called by ProcessSuccessfulPayment during
// webhook-triggered subscription activation.
func TestRecordSubscriptionRevenue_BalancedEntries(t *testing.T) {
	mock := &mockBillingLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock}

	paymentID := uuid.New()
	amount := int64(500000) // 500,000 IDR yearly fee
	providerEventID := "midtrans-event-123"

	err := svc.RecordSubscriptionRevenue(context.Background(), nil, paymentID, amount, providerEventID)
	if err != nil {
		t.Fatalf("RecordSubscriptionRevenue returned error: %v", err)
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

	// Verify DR side: PLATFORM_REVENUE (+500000) — revenue increases
	platformRevenueID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	bankSettlementID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	drEntry := mock.lastEntries[0]
	if drEntry.AccountID != platformRevenueID {
		t.Errorf("entry[0] account: got %s, want PLATFORM_REVENUE %s", drEntry.AccountID, platformRevenueID)
	}
	if drEntry.Amount.Int64() != 500000 {
		t.Errorf("entry[0] amount: got %d, want +500000 (debit)", drEntry.Amount.Int64())
	}

	// Verify CR side: BANK_SETTLEMENT (-500000) — reserve drains
	crEntry := mock.lastEntries[1]
	if crEntry.AccountID != bankSettlementID {
		t.Errorf("entry[1] account: got %s, want BANK_SETTLEMENT %s", crEntry.AccountID, bankSettlementID)
	}
	if crEntry.Amount.Int64() != -500000 {
		t.Errorf("entry[1] amount: got %d, want -500000 (credit)", crEntry.Amount.Int64())
	}

	// Verify idempotency key format
	expectedKey := "seller_subscription_payment_" + providerEventID
	if mock.lastIdempotencyKey != expectedKey {
		t.Errorf("idempotency key: got %q, want %q", mock.lastIdempotencyKey, expectedKey)
	}
}

// TestRecordSubscriptionRevenue_ZeroAmount verifies balance holds for zero fee.
func TestRecordSubscriptionRevenue_ZeroAmount(t *testing.T) {
	mock := &mockBillingLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock}

	err := svc.RecordSubscriptionRevenue(context.Background(), nil, uuid.New(), 0, "evt-0")
	if err != nil {
		t.Fatalf("RecordSubscriptionRevenue returned error: %v", err)
	}

	total := money.Zero()
	for _, entry := range mock.lastEntries {
		total = total.Add(entry.Amount)
	}
	if !total.IsZero() {
		t.Fatalf("ledger entries unbalanced for zero amount: sum = %d", total.Int64())
	}
}

// TestRecordSubscriptionRevenue_LargeAmount verifies balance holds for
// large subscription fees.
func TestRecordSubscriptionRevenue_LargeAmount(t *testing.T) {
	mock := &mockBillingLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock}

	amount := int64(10_000_000) // 10M IDR
	err := svc.RecordSubscriptionRevenue(context.Background(), nil, uuid.New(), amount, "evt-large")
	if err != nil {
		t.Fatalf("RecordSubscriptionRevenue returned error: %v", err)
	}

	total := money.Zero()
	for _, entry := range mock.lastEntries {
		total = total.Add(entry.Amount)
	}
	if !total.IsZero() {
		t.Fatalf("ledger entries unbalanced for large amount: sum = %d", total.Int64())
	}

	// Verify exact amounts: DR PLATFORM_REVENUE, CR BANK_SETTLEMENT
	if mock.lastEntries[0].Amount.Int64() != 10_000_000 {
		t.Errorf("DR PLATFORM_REVENUE amount: got %d, want 10000000", mock.lastEntries[0].Amount.Int64())
	}
	if mock.lastEntries[1].Amount.Int64() != -10_000_000 {
		t.Errorf("CR BANK_SETTLEMENT amount: got %d, want -10000000", mock.lastEntries[1].Amount.Int64())
	}
}

// TestRecordSubscriptionRevenue_ReferenceType verifies the reference type
// used in the ledger transaction is correct.
func TestRecordSubscriptionRevenue_ReferenceType(t *testing.T) {
	mock := &mockSubscriptionLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock}

	err := svc.RecordSubscriptionRevenue(context.Background(), nil, uuid.New(), 500000, "evt-ref")
	if err != nil {
		t.Fatalf("RecordSubscriptionRevenue returned error: %v", err)
	}

	if mock.lastReferenceType != "seller_subscription_payment" {
		t.Errorf("reference type: got %q, want %q", mock.lastReferenceType, "seller_subscription_payment")
	}
}

// mockSubscriptionLedgerRepo extends mockBillingLedgerRepo to also capture
// the reference type passed to CreateTransaction.
type mockSubscriptionLedgerRepo struct {
	mockBillingLedgerRepo
	lastReferenceType string
}

func (m *mockSubscriptionLedgerRepo) CreateTransaction(
	_ context.Context, _ db.Tx,
	idempotencyKey string, referenceType string, _ uuid.UUID,
	_ *uuid.UUID, _ *uuid.UUID,
	entries []ledgerepo.Entry,
) error {
	m.createTxCalled = true
	m.lastIdempotencyKey = idempotencyKey
	m.lastEntries = entries
	m.lastReferenceType = referenceType
	return nil
}


