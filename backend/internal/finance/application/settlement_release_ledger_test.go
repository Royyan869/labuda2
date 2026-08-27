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
// PASS_18Y: settlement/release/refund ledger regression hardening.
//
// PASS_18X audited this flow and found no money-safety bug, but flagged that
// RecordGatewayPaymentSettlement and RecordOrderRelease had zero direct
// automated tests anywhere in the repo. This file closes that gap with:
//   - direct unit tests for both functions (validation + ledger shape)
//   - a stateful in-memory ledger simulator so multi-step scenarios
//     (settlement -> buyer fee sweep -> release -> refund) can assert final
//     account balances, not just individual entry lists
//   - the canonical worked example from the pass instructions:
//   subtotal 100_000, shipping 20_000, commission 5_000,
//   buyer payment fee 4_000, gross 124_000, escrow 120_000,
//   seller payable 115_000, platform revenue after release 9_000
// ============================================================================

// ledgerSim is a stateful in-memory LedgerRepository fake. Unlike the
// single-call mocks elsewhere in this package (mockBillingLedgerRepo et al.),
// it actually applies entries to running balances, mirroring the real
// LedgerRepository's behavior (internal/finance/infrastructure/repository/
// ledger_repository.go): balanced-transaction panic, idempotency-key no-op,
// account balances accumulate across calls. This lets a single test assert
// the FINAL state after a whole settlement->sweep->release->refund sequence.
type ledgerSim struct {
	balances   map[uuid.UUID]int64
	systemAcct map[string]uuid.UUID
	userAcct   map[string]uuid.UUID // accountType_userID -> accountID
	usedKeys   map[string]bool
	txCount    map[uuid.UUID]int // referenceID -> number of CreateTransaction calls
}

func newLedgerSim() *ledgerSim {
	return &ledgerSim{
		balances:   make(map[uuid.UUID]int64),
		systemAcct: make(map[string]uuid.UUID),
		userAcct:   make(map[string]uuid.UUID),
		usedKeys:   make(map[string]bool),
		txCount:    make(map[uuid.UUID]int),
	}
}

func (s *ledgerSim) GetSystemAccountID(_ context.Context, _ db.Tx, accountType string) (uuid.UUID, error) {
	if id, ok := s.systemAcct[accountType]; ok {
		return id, nil
	}
	id := uuid.New()
	s.systemAcct[accountType] = id
	return id, nil
}

func (s *ledgerSim) GetOrCreateUserAccount(_ context.Context, _ db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error) {
	key := accountType + "_" + userID.String()
	if id, ok := s.userAcct[key]; ok {
		return id, nil
	}
	id := uuid.New()
	s.userAcct[key] = id
	return id, nil
}

func (s *ledgerSim) GetUserAccountID(_ context.Context, _ db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error) {
	key := accountType + "_" + userID.String()
	id, ok := s.userAcct[key]
	if !ok {
		return uuid.Nil, fmt.Errorf("account not found: %s", key)
	}
	return id, nil
}

func (s *ledgerSim) GetAccountBalance(_ context.Context, _ db.Tx, accountID uuid.UUID) (money.Money, error) {
	return money.New(s.balances[accountID]), nil
}

func (s *ledgerSim) GetAccountBalanceForUpdate(ctx context.Context, tx db.Tx, accountID uuid.UUID) (money.Money, error) {
	return s.GetAccountBalance(ctx, tx, accountID)
}

func (s *ledgerSim) CountTransactionsByEntityID(_ context.Context, _ db.Tx, entityID uuid.UUID) (int, error) {
	return s.txCount[entityID], nil
}

func (s *ledgerSim) GetTotalCreditToUserAccount(_ context.Context, _ db.Tx, _ string, _ uuid.UUID) (int64, error) {
	return 0, nil
}

// CreateTransaction mirrors the real LedgerRepository: rejects an unbalanced
// entry set (panics, exactly like production — see ledger_repository.go's
// own doc comment), no-ops on a replayed idempotency key, and otherwise
// applies every entry to its account's running balance.
func (s *ledgerSim) CreateTransaction(
	_ context.Context, _ db.Tx,
	idempotencyKey string, _ string, referenceID uuid.UUID,
	_ *uuid.UUID, _ *uuid.UUID,
	entries []ledgerepo.Entry,
) error {
	if s.usedKeys[idempotencyKey] {
		return nil
	}
	total := int64(0)
	for _, e := range entries {
		total += e.Amount.Int64()
	}
	if total != 0 {
		panic(fmt.Sprintf("ledgerSim: unbalanced transaction, total=%d", total))
	}
	for _, e := range entries {
		s.balances[e.AccountID] += e.Amount.Int64()
	}
	s.usedKeys[idempotencyKey] = true
	s.txCount[referenceID]++
	return nil
}

func (s *ledgerSim) balanceOf(accountType string) int64 {
	return s.balances[s.systemAcct[accountType]]
}

func (s *ledgerSim) userBalanceOf(accountType string, userID uuid.UUID) int64 {
	return s.balances[s.userAcct[accountType+"_"+userID.String()]]
}

func newSimService(sim *ledgerSim) *FinanceService {
	return &FinanceService{ledgerRepo: sim, logger: zap.NewNop()}
}

// ============================================================================
// A. RecordGatewayPaymentSettlement — direct tests
// ============================================================================

func TestRecordGatewayPaymentSettlement_CreditsClearingDebitsBankSettlement(t *testing.T) {
	sim := newLedgerSim()
	svc := newSimService(sim)

	paymentID := uuid.New()
	orderID := uuid.New()

	if err := svc.RecordGatewayPaymentSettlement(context.Background(), nil, paymentID, orderID, "txn-1", 124_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := sim.balanceOf(finance.AccountGatewayClearing); got != 124_000 {
		t.Errorf("GATEWAY_CLEARING = %d, want 124000", got)
	}
	if got := sim.balanceOf(finance.AccountBankSettlement); got != -124_000 {
		t.Errorf("BANK_SETTLEMENT = %d, want -124000", got)
	}
}

func TestRecordGatewayPaymentSettlement_IdempotentOnReplay(t *testing.T) {
	sim := newLedgerSim()
	svc := newSimService(sim)

	paymentID := uuid.New()
	orderID := uuid.New()

	if err := svc.RecordGatewayPaymentSettlement(context.Background(), nil, paymentID, orderID, "txn-dup", 100_000); err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Same provider_transaction_id (Midtrans webhook retry) must be a no-op.
	if err := svc.RecordGatewayPaymentSettlement(context.Background(), nil, paymentID, orderID, "txn-dup", 100_000); err != nil {
		t.Fatalf("replay call: %v", err)
	}

	if got := sim.balanceOf(finance.AccountGatewayClearing); got != 100_000 {
		t.Errorf("GATEWAY_CLEARING = %d, want 100000 (replay must not double-credit)", got)
	}
}

func TestRecordGatewayPaymentSettlement_RejectsNonPositiveGross(t *testing.T) {
	svc := newSimService(newLedgerSim())
	cases := []int64{0, -1, -124_000}
	for _, g := range cases {
		if err := svc.RecordGatewayPaymentSettlement(context.Background(), nil, uuid.New(), uuid.New(), "txn", g); err == nil {
			t.Errorf("gross=%d: expected error, got nil", g)
		}
	}
}

func TestRecordGatewayPaymentSettlement_RejectsMissingIdentifiers(t *testing.T) {
	svc := newSimService(newLedgerSim())

	if err := svc.RecordGatewayPaymentSettlement(context.Background(), nil, uuid.Nil, uuid.New(), "txn", 1000); err == nil {
		t.Error("nil payment_id: expected error, got nil")
	}
	if err := svc.RecordGatewayPaymentSettlement(context.Background(), nil, uuid.New(), uuid.Nil, "txn", 1000); err == nil {
		t.Error("nil order_id: expected error, got nil")
	}
	if err := svc.RecordGatewayPaymentSettlement(context.Background(), nil, uuid.New(), uuid.New(), "", 1000); err == nil {
		t.Error("empty provider_transaction_id: expected error, got nil")
	}
}

// ============================================================================
// B. RecordOrderRelease — direct tests
// ============================================================================

func TestRecordOrderRelease_DrainsClearingToSellerAndRevenue(t *testing.T) {
	sim := newLedgerSim()
	svc := newSimService(sim)
	sellerID := uuid.New()

	// Pre-fund GATEWAY_CLEARING as if settlement + fee sweep already ran,
	// leaving exactly the escrow amount (120_000 = BuyerBase) for this order.
	sim.balances[sim.mustSystemAccount(finance.AccountGatewayClearing)] = 120_000

	if err := svc.RecordOrderRelease(context.Background(), nil, uuid.New(), sellerID, 120_000, 5_000, 115_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := sim.balanceOf(finance.AccountGatewayClearing); got != 0 {
		t.Errorf("GATEWAY_CLEARING = %d, want 0 (fully drained for this order)", got)
	}
	if got := sim.userBalanceOf(finance.AccountSellerPayable, sellerID); got != 115_000 {
		t.Errorf("SELLER_PAYABLE = %d, want 115000", got)
	}
	if got := sim.balanceOf(finance.AccountPlatformRevenue); got != 5_000 {
		t.Errorf("PLATFORM_REVENUE = %d, want 5000 (commission only, buyer fee not part of release)", got)
	}
}

func TestRecordOrderRelease_RejectsMismatchedSplit(t *testing.T) {
	svc := newSimService(newLedgerSim())
	err := svc.RecordOrderRelease(context.Background(), nil, uuid.New(), uuid.New(), 120_000, 5_000, 114_000) // 114000+5000 != 120000
	if err == nil {
		t.Fatal("expected error when sellerNet+commission != gross")
	}
}

func TestRecordOrderRelease_RejectsNegativeAmounts(t *testing.T) {
	svc := newSimService(newLedgerSim())
	cases := []struct{ gross, commission, sellerNet int64 }{
		{-1, 0, 0},
		{100, -1, 101},
		{100, 0, -1},
	}
	for _, c := range cases {
		if err := svc.RecordOrderRelease(context.Background(), nil, uuid.New(), uuid.New(), c.gross, c.commission, c.sellerNet); err == nil {
			t.Errorf("gross=%d commission=%d sellerNet=%d: expected error, got nil", c.gross, c.commission, c.sellerNet)
		}
	}
}

func TestRecordOrderRelease_ZeroCommissionAllowed(t *testing.T) {
	sim := newLedgerSim()
	svc := newSimService(sim)
	if err := svc.RecordOrderRelease(context.Background(), nil, uuid.New(), uuid.New(), 50_000, 0, 50_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := sim.balanceOf(finance.AccountPlatformRevenue); got != 0 {
		t.Errorf("PLATFORM_REVENUE = %d, want 0", got)
	}
}

// mustSystemAccount is a tiny test helper so fixtures can pre-seed a system
// account balance without going through CreateTransaction.
func (s *ledgerSim) mustSystemAccount(accountType string) uuid.UUID {
	id, _ := s.GetSystemAccountID(context.Background(), nil, accountType)
	return id
}

// ============================================================================
// C. End-to-end scenario — the pass's canonical worked example
//
//   subtotal=100_000  shipping=20_000  commission=5_000  buyer_fee=4_000
//   escrow (canonical buyer base) = 100_000+20_000 = 120_000  (P+S; C excluded)
//   gross (what buyer pays)       = 120_000+4_000  = 124_000
//   seller payable                = 120_000-5_000  = 115_000
//   platform revenue (final)      = 4_000 (fee) + 5_000 (commission) = 9_000
// ============================================================================

const (
	exSubtotal   = int64(100_000)
	exShipping   = int64(20_000)
	exCommission = int64(5_000)
	exBuyerFee   = int64(4_000)
	exEscrow     = exSubtotal + exShipping // 120_000 (BuyerBase = P+S, C excluded)
	exGross      = exEscrow + exBuyerFee   // 124_000
	exSellerNet  = exEscrow - exCommission // 115_000
)

func TestLedgerScenario_SettlementSweepRelease_MatchesPassExample(t *testing.T) {
	sim := newLedgerSim()
	svc := newSimService(sim)

	paymentID := uuid.New()
	orderID := uuid.New()
	sellerID := uuid.New()

	// 1. Payment settlement: buyer's full gross lands in GATEWAY_CLEARING.
	if err := svc.RecordGatewayPaymentSettlement(context.Background(), nil, paymentID, orderID, "txn-scenario", exGross); err != nil {
		t.Fatalf("settlement: %v", err)
	}
	if got := sim.balanceOf(finance.AccountGatewayClearing); got != exGross {
		t.Fatalf("after settlement: GATEWAY_CLEARING = %d, want %d", got, exGross)
	}
	if got := sim.balanceOf(finance.AccountBankSettlement); got != -exGross {
		t.Fatalf("after settlement: BANK_SETTLEMENT = %d, want %d", got, -exGross)
	}

	// 2. Buyer payment fee revenue sweep: fee leaves clearing immediately.
	if err := svc.RecordBuyerPaymentFeeRevenue(context.Background(), nil, paymentID, orderID, exBuyerFee); err != nil {
		t.Fatalf("fee sweep: %v", err)
	}
	if got := sim.balanceOf(finance.AccountGatewayClearing); got != exEscrow {
		t.Fatalf("after fee sweep: GATEWAY_CLEARING = %d, want %d (escrow-equivalent only)", got, exEscrow)
	}
	if got := sim.balanceOf(finance.AccountPlatformRevenue); got != exBuyerFee {
		t.Fatalf("after fee sweep: PLATFORM_REVENUE = %d, want %d (buyer fee only so far)", got, exBuyerFee)
	}

	// 3. Order release: escrow drains to seller + commission.
	if err := svc.RecordOrderRelease(context.Background(), nil, orderID, sellerID, exEscrow, exCommission, exSellerNet); err != nil {
		t.Fatalf("release: %v", err)
	}

	// --- Final assertions: the exact numbers named in the pass instructions ---
	if got := sim.balanceOf(finance.AccountGatewayClearing); got != 0 {
		t.Errorf("FINAL GATEWAY_CLEARING = %d, want 0 (no buyer-fee residual after sweep+release)", got)
	}
	if got := sim.userBalanceOf(finance.AccountSellerPayable, sellerID); got != 115_000 {
		t.Errorf("FINAL SELLER_PAYABLE = %d, want 115000 (buyer fee never touches seller side)", got)
	}
	if got := sim.balanceOf(finance.AccountPlatformRevenue); got != 9_000 {
		t.Errorf("FINAL PLATFORM_REVENUE = %d, want 9000 (4000 fee + 5000 commission)", got)
	}
}

func TestLedgerScenario_FullRefundBeforeRelease_ExcludesBuyerFee(t *testing.T) {
	sim := newLedgerSim()
	svc := newSimService(sim)

	paymentID := uuid.New()
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	if err := svc.RecordGatewayPaymentSettlement(context.Background(), nil, paymentID, orderID, "txn-refund-before", exGross); err != nil {
		t.Fatalf("settlement: %v", err)
	}
	if err := svc.RecordBuyerPaymentFeeRevenue(context.Background(), nil, paymentID, orderID, exBuyerFee); err != nil {
		t.Fatalf("fee sweep: %v", err)
	}

	// Buyer requests item_not_received (full policy) — refund base is the
	// escrow-equivalent amount ONLY. The buyer payment fee is never part of
	// input.RefundAmount (see refund_policy.OrderSnapshot, which has no fee
	// field at all — PASS_18V's structural guarantee).
	_, err := svc.RecordRefundReversal(context.Background(), nil, RecordRefundReversalInput{
		RefundID:            uuid.New(),
		OrderID:             orderID,
		BuyerID:             buyerID,
		SellerID:            sellerID,
		RefundAmount:        exEscrow,
		SellerComponent:     exEscrow - exCommission,
		CommissionComponent: exCommission,
		OrderGross:          exEscrow,
		CumulativeRefunded:  exEscrow,
		AfterRelease:        false,
	})
	if err != nil {
		t.Fatalf("refund reversal: %v", err)
	}

	if got := sim.userBalanceOf(finance.AccountBuyerRefundable, buyerID); got != exEscrow {
		t.Errorf("BUYER_REFUNDABLE = %d, want %d (escrow only, fee excluded)", got, exEscrow)
	}
	if got := sim.balanceOf(finance.AccountGatewayClearing); got != 0 {
		t.Errorf("GATEWAY_CLEARING = %d, want 0 (escrow refunded out, fee already swept earlier)", got)
	}
	if got := sim.balanceOf(finance.AccountPlatformRevenue); got != exBuyerFee {
		t.Errorf("PLATFORM_REVENUE = %d, want %d (buyer fee kept — non-refundable per PASS_18V policy)", got, exBuyerFee)
	}
}

func TestLedgerScenario_FullRefundAfterRelease_ExcludesBuyerFee(t *testing.T) {
	sim := newLedgerSim()
	svc := newSimService(sim)

	paymentID := uuid.New()
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	if err := svc.RecordGatewayPaymentSettlement(context.Background(), nil, paymentID, orderID, "txn-refund-after", exGross); err != nil {
		t.Fatalf("settlement: %v", err)
	}
	if err := svc.RecordBuyerPaymentFeeRevenue(context.Background(), nil, paymentID, orderID, exBuyerFee); err != nil {
		t.Fatalf("fee sweep: %v", err)
	}
	if err := svc.RecordOrderRelease(context.Background(), nil, orderID, sellerID, exEscrow, exCommission, exSellerNet); err != nil {
		t.Fatalf("release: %v", err)
	}

	_, err := svc.RecordRefundReversal(context.Background(), nil, RecordRefundReversalInput{
		RefundID:            uuid.New(),
		OrderID:             orderID,
		BuyerID:             buyerID,
		SellerID:            sellerID,
		RefundAmount:        exEscrow,
		SellerComponent:     exSellerNet,
		CommissionComponent: exCommission,
		OrderGross:          exEscrow,
		CumulativeRefunded:  exEscrow,
		AfterRelease:        true,
	})
	if err != nil {
		t.Fatalf("refund reversal: %v", err)
	}

	if got := sim.userBalanceOf(finance.AccountBuyerRefundable, buyerID); got != exEscrow {
		t.Errorf("BUYER_REFUNDABLE = %d, want %d", got, exEscrow)
	}
	if got := sim.userBalanceOf(finance.AccountSellerPayable, sellerID); got != 0 {
		t.Errorf("SELLER_PAYABLE = %d, want 0 (120000 released then fully reversed)", got)
	}
	if got := sim.balanceOf(finance.AccountPlatformRevenue); got != exBuyerFee {
		t.Errorf("PLATFORM_REVENUE = %d, want %d (5000 commission reversed, 4000 buyer fee kept)", got, exBuyerFee)
	}
	if got := sim.balanceOf(finance.AccountGatewayClearing); got != 0 {
		t.Errorf("GATEWAY_CLEARING = %d, want 0 (release path never touches clearing after full drain)", got)
	}
}

// ============================================================================
// D. Zombie design guard (scoped to the finance ledger package)
// ============================================================================

// TestFinanceService_NoFlatFeeOrCheckoutfeeResidue is a source-level guard:
// the finance ledger layer must never re-import the deleted checkoutfee
// package, reference the killed BuyerServiceFeeAmount constant, or introduce
// a /100 or *100 minor-unit scaling in a live money path. This complements
// (does not replace) the PASS_18V structural guards in
// internal/pricing/token/application/flat_fee_removed_test.go and
// internal/serverboot/payment_method_default_killed_test.go, which cover the
// checkout/payment-creation layer; this one covers the ledger layer itself.
func TestFinanceService_NoFlatFeeOrCheckoutfeeResidue(t *testing.T) {
	raw, err := os.ReadFile("finance_service.go")
	if err != nil {
		t.Fatalf("read finance_service.go: %v", err)
	}
	src := string(raw)

	// NOTE: "cents"/"sen" are deliberately NOT checked here — the file
	// legitimately contains doctrinal comments like "no cents/sen subunit"
	// explaining the ABSENCE of minor-unit math (e.g. finance_service.go:172,
	// 304, 542). A substring match on those words would false-positive on
	// exactly the comments proving the doctrine is followed. /100 and *100
	// are unambiguous — no legitimate comment needs that exact substring.
	forbidden := []string{"checkoutfee", "BuyerServiceFeeAmount", "/100", "*100"}
	for _, f := range forbidden {
		if strings.Contains(src, f) {
			t.Errorf("REGRESSION: finance_service.go must not contain %q", f)
		}
	}

	required := []string{
		"func (s *FinanceService) RecordGatewayPaymentSettlement(",
		"func (s *FinanceService) RecordBuyerPaymentFeeRevenue(",
		"func (s *FinanceService) RecordOrderRelease(",
		"func (s *FinanceService) RecordRefundReversal(",
	}
	for _, r := range required {
		if !strings.Contains(src, r) {
			t.Errorf("MISSING: finance_service.go must still define %q", r)
		}
	}
}
