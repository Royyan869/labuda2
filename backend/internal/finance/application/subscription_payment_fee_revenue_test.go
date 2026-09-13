package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// PMF-02: RecordSubscriptionPaymentFeeRevenue books the payment-method fee of a
// seller subscription payment as platform revenue. These tests give the method
// the same authority proof the other fee-ledger paths already have
// (RecordBuyerPaymentFeeRevenue, RecordBillingPaymentFeeRevenue).

// TestRecordSubscriptionPaymentFeeRevenue_BalancedEntries proves F > 0 moves the
// fee from BANK_SETTLEMENT into PLATFORM_REVENUE as a balanced double-entry —
// subscription payments never flow through GATEWAY_CLEARING.
func TestRecordSubscriptionPaymentFeeRevenue_BalancedEntries(t *testing.T) {
	mock := &mockFeeLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	paymentID := uuid.New()
	subID := uuid.New()
	const fee = int64(3750)

	if err := svc.RecordSubscriptionPaymentFeeRevenue(context.Background(), nil, paymentID, subID, fee); err != nil {
		t.Fatalf("RecordSubscriptionPaymentFeeRevenue returned error: %v", err)
	}
	if !mock.createTxCalled {
		t.Fatal("CreateTransaction was not called")
	}
	if mock.lastReferenceType != "subscription_fee_revenue" {
		t.Errorf("reference_type = %q, want %q", mock.lastReferenceType, "subscription_fee_revenue")
	}
	if want := "subscription_fee_revenue_" + paymentID.String(); mock.lastIdempotencyKey != want {
		t.Errorf("idempotency_key = %q, want %q", mock.lastIdempotencyKey, want)
	}
	if len(mock.lastEntries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(mock.lastEntries))
	}

	total := money.Zero()
	movements := map[uuid.UUID]int64{}
	for _, e := range mock.lastEntries {
		total = total.Add(e.Amount)
		movements[e.AccountID] += e.Amount.Int64()
	}
	if !total.IsZero() {
		t.Fatalf("ledger entries unbalanced: sum = %d (must be 0)", total.Int64())
	}

	platformRevenue := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	bankSettlement := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	if movements[platformRevenue] != fee {
		t.Errorf("PLATFORM_REVENUE movement = %d, want +%d", movements[platformRevenue], fee)
	}
	if movements[bankSettlement] != -fee {
		t.Errorf("BANK_SETTLEMENT movement = %d, want -%d", movements[bankSettlement], fee)
	}
}

// TestRecordSubscriptionPaymentFeeRevenue_ZeroFeeIsNoOp proves a zero-fee method
// never creates a useless ledger transaction.
func TestRecordSubscriptionPaymentFeeRevenue_ZeroFeeIsNoOp(t *testing.T) {
	mock := &mockFeeLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	if err := svc.RecordSubscriptionPaymentFeeRevenue(context.Background(), nil, uuid.New(), uuid.New(), 0); err != nil {
		t.Fatalf("zero fee must be a no-op, got error: %v", err)
	}
	if mock.createTxCalled {
		t.Fatal("zero fee must not create a ledger transaction")
	}
}

// TestRecordSubscriptionPaymentFeeRevenue_NegativeFeeRejected proves a negative
// fee can never book negative revenue.
func TestRecordSubscriptionPaymentFeeRevenue_NegativeFeeRejected(t *testing.T) {
	mock := &mockFeeLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	if err := svc.RecordSubscriptionPaymentFeeRevenue(context.Background(), nil, uuid.New(), uuid.New(), -100); err == nil {
		t.Fatal("negative fee must be rejected")
	}
	if mock.createTxCalled {
		t.Fatal("rejected fee must not reach the ledger")
	}
}

// TestRecordSubscriptionPaymentFeeRevenue_RequiresIdentities proves the method
// fails closed without a payment id or a subscription reference.
func TestRecordSubscriptionPaymentFeeRevenue_RequiresIdentities(t *testing.T) {
	mock := &mockFeeLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	if err := svc.RecordSubscriptionPaymentFeeRevenue(context.Background(), nil, uuid.Nil, uuid.New(), 1000); err == nil {
		t.Error("nil payment id must be rejected")
	}
	if err := svc.RecordSubscriptionPaymentFeeRevenue(context.Background(), nil, uuid.New(), uuid.Nil, 1000); err == nil {
		t.Error("nil subscription id must be rejected")
	}
	if mock.createTxCalled {
		t.Fatal("rejected calls must not reach the ledger")
	}
}

// TestRecordSubscriptionPaymentFeeRevenue_IdempotencyKeyIsStable proves the
// dedup identity is derived from the payment id alone, so a webhook replay
// resolves to the same ledger transaction instead of booking the fee twice.
func TestRecordSubscriptionPaymentFeeRevenue_IdempotencyKeyIsStable(t *testing.T) {
	mock := &mockFeeLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	paymentID := uuid.New()
	first := uuid.New()
	second := uuid.New()

	if err := svc.RecordSubscriptionPaymentFeeRevenue(context.Background(), nil, paymentID, first, 3750); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	firstKey := mock.lastIdempotencyKey

	// Same payment, different provider-event reference and subscription id.
	if err := svc.RecordSubscriptionPaymentFeeRevenue(context.Background(), nil, paymentID, second, 3750); err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if mock.lastIdempotencyKey != firstKey {
		t.Fatalf("idempotency key changed across replay: %q -> %q", firstKey, mock.lastIdempotencyKey)
	}
	if want := "subscription_fee_revenue_" + paymentID.String(); firstKey != want {
		t.Errorf("idempotency_key = %q, want %q", firstKey, want)
	}
}

// TestRecordSubscriptionPaymentFeeRevenue_EntryShape locks the exact account
// pair: BANK_SETTLEMENT (credit) then PLATFORM_REVENUE (debit).
func TestRecordSubscriptionPaymentFeeRevenue_EntryShape(t *testing.T) {
	mock := &mockFeeLedgerRepo{}
	svc := &FinanceService{ledgerRepo: mock, logger: zap.NewNop()}

	if err := svc.RecordSubscriptionPaymentFeeRevenue(context.Background(), nil, uuid.New(), uuid.New(), 2500); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if len(mock.lastEntries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(mock.lastEntries))
	}

	bankSettlement := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	platformRevenue := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	if mock.lastEntries[0].AccountID != bankSettlement || mock.lastEntries[0].Amount.Int64() != -2500 {
		t.Errorf("entry[0] = (%s, %d), want BANK_SETTLEMENT credit -2500",
			mock.lastEntries[0].AccountID, mock.lastEntries[0].Amount.Int64())
	}
	if mock.lastEntries[1].AccountID != platformRevenue || mock.lastEntries[1].Amount.Int64() != 2500 {
		t.Errorf("entry[1] = (%s, %d), want PLATFORM_REVENUE debit +2500",
			mock.lastEntries[1].AccountID, mock.lastEntries[1].Amount.Int64())
	}
	if mock.lastEntries[0].AccountID == mock.lastEntries[1].AccountID {
		t.Fatal("fee revenue must not use GATEWAY_CLEARING as both sides")
	}
}
