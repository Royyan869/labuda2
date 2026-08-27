package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	ledgerepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// ============================================================================
// Tests for RecordPartialRefundRelease (H2-A3).
// Validates ledger balance, account routing, idempotency key, and validations.
// ============================================================================

// mockPartialReleaseLedgerRepo extends the base mock with user account support.
type mockPartialReleaseLedgerRepo struct {
	mockBillingLedgerRepo
	lastReferenceType string
	userAccounts      map[string]uuid.UUID // accountType+userID → accountID
}

func newMockPartialReleaseLedgerRepo() *mockPartialReleaseLedgerRepo {
	return &mockPartialReleaseLedgerRepo{
		userAccounts: make(map[string]uuid.UUID),
	}
}

func (m *mockPartialReleaseLedgerRepo) GetOrCreateUserAccount(_ context.Context, _ db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error) {
	key := accountType + "_" + userID.String()
	if id, ok := m.userAccounts[key]; ok {
		return id, nil
	}
	id := uuid.New()
	m.userAccounts[key] = id
	return id, nil
}

func (m *mockPartialReleaseLedgerRepo) CreateTransaction(
	_ context.Context, _ db.Tx,
	idempotencyKey string, referenceType string, _ uuid.UUID,
	_ *uuid.UUID, _ *uuid.UUID,
	entries []ledgerepo.Entry,
) error {
	m.createTxCalled = true
	m.lastIdempotencyKey = idempotencyKey
	m.lastReferenceType = referenceType
	m.lastEntries = entries
	return nil
}

// --- Happy path: product-only refund remainder release ---

func TestRecordPartialRefundRelease_BalancedEntries(t *testing.T) {
	mock := newMockPartialReleaseLedgerRepo()
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	refundID := uuid.New()
	orderID := uuid.New()
	sellerID := uuid.New()

	// Order: Subtotal=100_000, Shipping=25_000, Commission=6_250 (on PD=100_000)
	// Partial product refund: buyer gets 40_000 of the product (PD denominator)
	// Remainder = BuyerBase - cumRefunded = 125_000 - 40_000 = 85_000
	// Remainder product = PD - cumProductAfter = 100_000 - 40_000 = 60_000
	// Commission on remainder = floor(60_000 * 6_250 / 100_000) = 3_750 (PD denominator)
	// Seller net: 85_000 - 3_750 = 81_250
	remainder := int64(85_000)
	commission := int64(3_750)
	sellerNet := int64(81_250)

	dup, err := svc.RecordPartialRefundRelease(context.Background(), nil, RecordPartialRefundReleaseInput{
		RefundID:   refundID,
		OrderID:    orderID,
		SellerID:   sellerID,
		Remainder:  remainder,
		SellerNet:  sellerNet,
		Commission: commission,
	})
	if err != nil {
		t.Fatalf("RecordPartialRefundRelease error: %v", err)
	}
	if dup {
		t.Fatal("expected non-duplicate for first call")
	}
	if !mock.createTxCalled {
		t.Fatal("CreateTransaction was not called")
	}

	// Must have exactly 3 entries
	if len(mock.lastEntries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(mock.lastEntries))
	}

	// Verify balance: sum of all entries must equal zero
	total := money.Zero()
	for _, entry := range mock.lastEntries {
		total = total.Add(entry.Amount)
	}
	if !total.IsZero() {
		t.Fatalf("ledger entries unbalanced: sum = %d (must be 0)", total.Int64())
	}

	// Entry[0]: GATEWAY_CLEARING -= remainder
	gatewayClearingID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	if mock.lastEntries[0].AccountID != gatewayClearingID {
		t.Errorf("entry[0] account: got %s, want GATEWAY_CLEARING", mock.lastEntries[0].AccountID)
	}
	if mock.lastEntries[0].Amount.Int64() != -85_000 {
		t.Errorf("entry[0] amount: got %d, want -85000", mock.lastEntries[0].Amount.Int64())
	}

	// Entry[1]: SELLER_PAYABLE += sellerNet
	if mock.lastEntries[1].Amount.Int64() != 81_250 {
		t.Errorf("entry[1] amount: got %d, want +81250 (seller_net)", mock.lastEntries[1].Amount.Int64())
	}

	// Entry[2]: PLATFORM_REVENUE += commission
	platformRevenueID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	if mock.lastEntries[2].AccountID != platformRevenueID {
		t.Errorf("entry[2] account: got %s, want PLATFORM_REVENUE", mock.lastEntries[2].AccountID)
	}
	if mock.lastEntries[2].Amount.Int64() != 3_750 {
		t.Errorf("entry[2] amount: got %d, want +3750 (commission)", mock.lastEntries[2].Amount.Int64())
	}

	// Verify reference type
	if mock.lastReferenceType != "partial_refund_release" {
		t.Errorf("reference type: got %q, want %q", mock.lastReferenceType, "partial_refund_release")
	}

	// Verify idempotency key
	expectedKey := "partial_release_" + refundID.String()
	if mock.lastIdempotencyKey != expectedKey {
		t.Errorf("idempotency key: got %q, want %q", mock.lastIdempotencyKey, expectedKey)
	}
}

// --- Full gateway clearing drain proof ---

func TestRecordPartialRefundRelease_GatewayClearingDrainsToZero(t *testing.T) {
	// Scenario: order buyer base = 125_000 (P=100_000, S=25_000)
	// Refund reversal already moved: -40_000 from GATEWAY_CLEARING (buyer refund)
	// Remainder release moves:       -85_000 from GATEWAY_CLEARING (seller+platform)
	// Net GATEWAY_CLEARING change:   -(40_000 + 85_000) = -125_000 = -BuyerBase ✓

	mock := newMockPartialReleaseLedgerRepo()
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	// remainder = BuyerBase - cumulativeRefunded = 125_000 - 40_000 = 85_000
	remainder := int64(85_000)
	commission := int64(3_750)
	sellerNet := remainder - commission

	_, err := svc.RecordPartialRefundRelease(context.Background(), nil, RecordPartialRefundReleaseInput{
		RefundID:   uuid.New(),
		OrderID:    uuid.New(),
		SellerID:   uuid.New(),
		Remainder:  remainder,
		SellerNet:  sellerNet,
		Commission: commission,
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Verify GATEWAY_CLEARING is drained by exactly the remainder
	gwEntry := mock.lastEntries[0]
	if gwEntry.Amount.Int64() != -85_000 {
		t.Fatalf("GATEWAY_CLEARING delta: got %d, want -85000", gwEntry.Amount.Int64())
	}
}

// --- Validation: zero remainder ---

func TestRecordPartialRefundRelease_RejectsZeroRemainder(t *testing.T) {
	svc := &FinanceService{ledgerRepo: newMockPartialReleaseLedgerRepo(), logger: zap.NewNop()}
	_, err := svc.RecordPartialRefundRelease(context.Background(), nil, RecordPartialRefundReleaseInput{
		RefundID:   uuid.New(),
		OrderID:    uuid.New(),
		SellerID:   uuid.New(),
		Remainder:  0,
		SellerNet:  0,
		Commission: 0,
	})
	if err == nil {
		t.Fatal("expected error for zero remainder")
	}
}

// --- Validation: unbalanced split ---

func TestRecordPartialRefundRelease_RejectsUnbalancedSplit(t *testing.T) {
	svc := &FinanceService{ledgerRepo: newMockPartialReleaseLedgerRepo(), logger: zap.NewNop()}
	_, err := svc.RecordPartialRefundRelease(context.Background(), nil, RecordPartialRefundReleaseInput{
		RefundID:   uuid.New(),
		OrderID:    uuid.New(),
		SellerID:   uuid.New(),
		Remainder:  100,
		SellerNet:  50,
		Commission: 60, // 50+60=110 ≠ 100
	})
	if err == nil {
		t.Fatal("expected error for unbalanced split")
	}
}

// --- Validation: nil IDs ---

func TestRecordPartialRefundRelease_RejectsNilRefundID(t *testing.T) {
	svc := &FinanceService{ledgerRepo: newMockPartialReleaseLedgerRepo(), logger: zap.NewNop()}
	_, err := svc.RecordPartialRefundRelease(context.Background(), nil, RecordPartialRefundReleaseInput{
		OrderID:    uuid.New(),
		SellerID:   uuid.New(),
		Remainder:  100,
		SellerNet:  90,
		Commission: 10,
	})
	if err == nil {
		t.Fatal("expected error for nil refund_id")
	}
}

// --- Zero commission (no platform fee) ---

func TestRecordPartialRefundRelease_ZeroCommission(t *testing.T) {
	mock := newMockPartialReleaseLedgerRepo()
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	remainder := int64(25_000)
	_, err := svc.RecordPartialRefundRelease(context.Background(), nil, RecordPartialRefundReleaseInput{
		RefundID:   uuid.New(),
		OrderID:    uuid.New(),
		SellerID:   uuid.New(),
		Remainder:  remainder,
		SellerNet:  remainder, // all to seller
		Commission: 0,
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	total := money.Zero()
	for _, entry := range mock.lastEntries {
		total = total.Add(entry.Amount)
	}
	if !total.IsZero() {
		t.Fatalf("ledger unbalanced: sum = %d", total.Int64())
	}
	if mock.lastEntries[1].Amount.Int64() != 25_000 {
		t.Errorf("seller_net: got %d, want 25000", mock.lastEntries[1].Amount.Int64())
	}
	if mock.lastEntries[2].Amount.Int64() != 0 {
		t.Errorf("commission: got %d, want 0", mock.lastEntries[2].Amount.Int64())
	}
}


