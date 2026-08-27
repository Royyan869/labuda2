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

// mockFeeLedgerRepo is a minimal in-memory LedgerRepository fake for testing
// RecordBuyerPaymentFeeRevenue without a real Postgres instance.
type mockFeeLedgerRepo struct {
	lastEntries        []ledgerepo.Entry
	lastIdempotencyKey string
	lastReferenceType  string
	createTxCalled     bool
}

func (m *mockFeeLedgerRepo) GetSystemAccountID(_ context.Context, _ db.Tx, accountType string) (uuid.UUID, error) {
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

func (m *mockFeeLedgerRepo) CreateTransaction(
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

func (m *mockFeeLedgerRepo) GetAccountBalance(context.Context, db.Tx, uuid.UUID) (money.Money, error) {
	return money.Zero(), nil
}
func (m *mockFeeLedgerRepo) GetAccountBalanceForUpdate(context.Context, db.Tx, uuid.UUID) (money.Money, error) {
	return money.Zero(), nil
}
func (m *mockFeeLedgerRepo) GetUserAccountID(context.Context, db.Tx, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (m *mockFeeLedgerRepo) GetOrCreateUserAccount(context.Context, db.Tx, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (m *mockFeeLedgerRepo) CountTransactionsByEntityID(context.Context, db.Tx, uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockFeeLedgerRepo) GetTotalCreditToUserAccount(context.Context, db.Tx, string, uuid.UUID) (int64, error) {
	return 0, nil
}

// TestRecordBuyerPaymentFeeRevenue_BalancedEntries proves the buyer payment
// method fee (PASS_18V) leaves GATEWAY_CLEARING and lands in
// PLATFORM_REVENUE as a balanced double-entry, closing the gap where the
// flat Rp3.000 fee used to sit in GATEWAY_CLEARING forever, unaccounted.
func TestRecordBuyerPaymentFeeRevenue_BalancedEntries(t *testing.T) {
	mock := &mockFeeLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	paymentID := uuid.New()
	orderID := uuid.New()

	if err := svc.RecordBuyerPaymentFeeRevenue(context.Background(), nil, paymentID, orderID, 4987); err != nil {
		t.Fatalf("RecordBuyerPaymentFeeRevenue returned error: %v", err)
	}

	if !mock.createTxCalled {
		t.Fatal("CreateTransaction was not called")
	}
	if mock.lastReferenceType != "payment_fee_revenue" {
		t.Errorf("reference_type = %q, want %q", mock.lastReferenceType, "payment_fee_revenue")
	}

	if len(mock.lastEntries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(mock.lastEntries))
	}

	total := money.Zero()
	for _, e := range mock.lastEntries {
		total = total.Add(e.Amount)
	}
	if !total.IsZero() {
		t.Fatalf("ledger entries unbalanced: sum = %d (must be 0)", total.Int64())
	}

	gatewayClearingID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	platformRevenueID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	cr := mock.lastEntries[0]
	if cr.AccountID != gatewayClearingID {
		t.Errorf("entry[0] account = %s, want GATEWAY_CLEARING %s", cr.AccountID, gatewayClearingID)
	}
	if cr.Amount.Int64() != -4987 {
		t.Errorf("entry[0] amount = %d, want -4987 (credit: fee leaves clearing)", cr.Amount.Int64())
	}

	dr := mock.lastEntries[1]
	if dr.AccountID != platformRevenueID {
		t.Errorf("entry[1] account = %s, want PLATFORM_REVENUE %s", dr.AccountID, platformRevenueID)
	}
	if dr.Amount.Int64() != 4987 {
		t.Errorf("entry[1] amount = %d, want +4987 (debit: fee realized as revenue)", dr.Amount.Int64())
	}

	wantKey := "payment_fee_revenue_" + paymentID.String()
	if mock.lastIdempotencyKey != wantKey {
		t.Errorf("idempotency key = %q, want %q", mock.lastIdempotencyKey, wantKey)
	}
}

// TestRecordBuyerPaymentFeeRevenue_ZeroFeeIsNoOp proves a zero fee (e.g. a
// hypothetical free method) does not create a ledger transaction — there is
// nothing to move.
func TestRecordBuyerPaymentFeeRevenue_ZeroFeeIsNoOp(t *testing.T) {
	mock := &mockFeeLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	if err := svc.RecordBuyerPaymentFeeRevenue(context.Background(), nil, uuid.New(), uuid.New(), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.createTxCalled {
		t.Fatal("CreateTransaction must not be called for a zero fee")
	}
}

// TestRecordBuyerPaymentFeeRevenue_NegativeFeeRejected proves the ledger
// refuses a negative buyer fee rather than silently crediting revenue with
// the wrong sign.
func TestRecordBuyerPaymentFeeRevenue_NegativeFeeRejected(t *testing.T) {
	mock := &mockFeeLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	if err := svc.RecordBuyerPaymentFeeRevenue(context.Background(), nil, uuid.New(), uuid.New(), -100); err == nil {
		t.Fatal("expected error for negative buyer payment fee, got nil")
	}
	if mock.createTxCalled {
		t.Fatal("CreateTransaction must not be called for an invalid negative fee")
	}
}
