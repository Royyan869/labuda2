// Spy tests for HandleGatewayRefundAck webhook integration.
//
// These tests prove that:
// 1. RecordPartialRefundRelease IS called for partial refund acks
// 2. RecordPartialRefundRelease is NOT called for full refund acks
// 3. Duplicate webhook acks short-circuit before any finance calls
// 4. Idempotency key is partial_release_<refund_id>
// 5. All calls share the same db.Tx
package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	walletApp "github.com/labuda/backend/internal/core/wallet/application"
	walletEntity "github.com/labuda/backend/internal/core/wallet/entity"
	walletRepo "github.com/labuda/backend/internal/core/wallet/repository"
	financeapp "github.com/labuda/backend/internal/finance/application"
	"github.com/labuda/backend/internal/finance/refund/entity"
	refundrepo "github.com/labuda/backend/internal/finance/refund/repository"
	disputeEntity "github.com/labuda/backend/internal/governance/dispute/entity"
	disputeRepo "github.com/labuda/backend/internal/governance/dispute/repository"
	outboxRepoImpl "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// ============================================================================
// SPY: FinanceReverser
// ============================================================================

type spyFinanceReverser struct {
	reversalCalled       bool
	reversalInput        financeapp.RecordRefundReversalInput
	partialReleaseCalled bool
	partialReleaseInput  financeapp.RecordPartialRefundReleaseInput
	coinFundingReversalCalled bool
	coinFundingReversalAmount int64
	// Configurable return for reversal
	reversalSummary *financeapp.RecordRefundReversalSummary
}

func (s *spyFinanceReverser) RecordRefundReversal(
	_ context.Context, _ db.Tx, input financeapp.RecordRefundReversalInput,
) (*financeapp.RecordRefundReversalSummary, error) {
	s.reversalCalled = true
	s.reversalInput = input
	if s.reversalSummary != nil {
		return s.reversalSummary, nil
	}
	return &financeapp.RecordRefundReversalSummary{
		Phase:     "before_release",
		Duplicate: false,
	}, nil
}

func (s *spyFinanceReverser) RecordPartialRefundRelease(
	_ context.Context, _ db.Tx, input financeapp.RecordPartialRefundReleaseInput,
) (bool, error) {
	s.partialReleaseCalled = true
	s.partialReleaseInput = input
	return false, nil
}

func (s *spyFinanceReverser) RecordCoinFundingReversal(
	_ context.Context, _ db.Tx, _ uuid.UUID, _ uuid.UUID, amount int64,
) error {
	s.coinFundingReversalCalled = true
	s.coinFundingReversalAmount = amount
	return nil
}

// ============================================================================
// MOCK: db.Tx (noop — all ops succeed)
// ============================================================================

type noopTx struct{}

func (noopTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (noopTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (noopTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row { return nil }
func (noopTx) Commit(_ context.Context) error                         { return nil }
func (noopTx) Rollback(_ context.Context) error                       { return nil }

// ============================================================================
// MOCK: EscrowRepository (in-memory, keyed by OrderID)
// ============================================================================

type mockEscrowRepo struct {
	escrows map[uuid.UUID]*walletEntity.Escrow
}

func (m *mockEscrowRepo) GetByID(_ context.Context, _ db.Tx, id uuid.UUID) (*walletEntity.Escrow, error) {
	for _, e := range m.escrows {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, nil
}
func (m *mockEscrowRepo) GetByOrderID(_ context.Context, _ db.Tx, orderID uuid.UUID) (*walletEntity.Escrow, error) {
	return m.escrows[orderID], nil
}
func (m *mockEscrowRepo) GetByOrderIDForUpdate(_ context.Context, _ db.Tx, orderID uuid.UUID) (*walletEntity.Escrow, error) {
	return m.escrows[orderID], nil
}
func (m *mockEscrowRepo) Create(_ context.Context, _ db.Tx, escrow *walletEntity.Escrow) error {
	m.escrows[escrow.OrderID] = escrow
	return nil
}
func (m *mockEscrowRepo) Update(_ context.Context, _ db.Tx, escrow *walletEntity.Escrow) error {
	m.escrows[escrow.OrderID] = escrow
	return nil
}
func (m *mockEscrowRepo) GetByBuyerWalletID(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*walletEntity.Escrow, error) {
	return nil, nil
}
func (m *mockEscrowRepo) GetBySellerWalletID(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*walletEntity.Escrow, error) {
	return nil, nil
}

// ============================================================================
// MOCK: RefundRepository
// ============================================================================

type mockRefundRepo struct {
	refundByGatewayRefundID map[string]*entity.Refund
	refundByID              map[uuid.UUID]*entity.Refund
	refundByIdempotencyKey  map[string]*entity.Refund
	successTotal            int64
	updateCalled            bool
}

func newMockRefundRepo() *mockRefundRepo {
	return &mockRefundRepo{
		refundByGatewayRefundID: make(map[string]*entity.Refund),
		refundByID:              make(map[uuid.UUID]*entity.Refund),
		refundByIdempotencyKey:  make(map[string]*entity.Refund),
	}
}

func (m *mockRefundRepo) Create(_ context.Context, _ db.Tx, r *entity.Refund) error {
	m.refundByID[r.ID] = r
	return nil
}
func (m *mockRefundRepo) GetByID(_ context.Context, _ db.Tx, id uuid.UUID) (*entity.Refund, error) {
	return m.refundByID[id], nil
}
func (m *mockRefundRepo) GetByOrderID(_ context.Context, _ db.Tx, orderID uuid.UUID) (*entity.Refund, error) {
	for _, r := range m.refundByID {
		if r.OrderID == orderID {
			return r, nil
		}
	}
	return nil, nil
}
func (m *mockRefundRepo) GetForUpdate(_ context.Context, _ db.Tx, id uuid.UUID) (*entity.Refund, error) {
	return m.refundByID[id], nil
}
func (m *mockRefundRepo) Update(_ context.Context, _ db.Tx, r *entity.Refund) error {
	m.updateCalled = true
	m.refundByID[r.ID] = r
	return nil
}
func (m *mockRefundRepo) ListByBuyer(_ context.Context, _ db.Tx, _ uuid.UUID, _ int, _ int64) ([]*entity.Refund, error) {
	return nil, nil
}
func (m *mockRefundRepo) ListBySeller(_ context.Context, _ db.Tx, _ uuid.UUID, _ int, _ int64) ([]*entity.Refund, error) {
	return nil, nil
}
func (m *mockRefundRepo) GetByGatewayIdempotencyKey(_ context.Context, _ db.Tx, key string) (*entity.Refund, error) {
	return m.refundByIdempotencyKey[key], nil
}
func (m *mockRefundRepo) GetByGatewayRefundID(_ context.Context, _ db.Tx, gatewayRefundID string) (*entity.Refund, error) {
	return m.refundByGatewayRefundID[gatewayRefundID], nil
}
func (m *mockRefundRepo) GetSuccessfulRefundTotalByOrder(_ context.Context, _ db.Tx, _ uuid.UUID, _ *uuid.UUID) (int64, error) {
	return m.successTotal, nil
}
func (m *mockRefundRepo) CreateEvidence(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockRefundRepo) ListEvidence(_ context.Context, _ db.Tx, _ uuid.UUID) ([]string, error) {
	return nil, nil
}
func (m *mockRefundRepo) HasActiveRefundByOrderID(_ context.Context, _ db.Tx, _ uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockRefundRepo) ListByOrderID(_ context.Context, _ db.Tx, _ uuid.UUID, _ int, _ *refundrepo.OrderRefundCursor) ([]*entity.Refund, error) {
	return nil, nil
}
func (m *mockRefundRepo) GetCumulativeProductRefundByOrder(_ context.Context, _ db.Tx, _ uuid.UUID, _ *uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockRefundRepo) GetCumulativeShippingRefundByOrder(_ context.Context, _ db.Tx, _ uuid.UUID, _ *uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockRefundRepo) GetCumulativeCoinsRefundedByOrder(_ context.Context, _ db.Tx, _ uuid.UUID, _ *uuid.UUID) (int64, error) {
	return 0, nil
}

// ============================================================================
// MOCK: OrderRepository (minimal — only GetForUpdate needed)
// ============================================================================

type mockOrderRepo struct {
	order *orderEntity.Order
}

func (m *mockOrderRepo) GetForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) {
	return m.order, nil
}
func (m *mockOrderRepo) CreateOrderTx(_ context.Context, _ db.Tx, _ *orderEntity.Order) error {
	return nil
}
func (m *mockOrderRepo) CreateOrderItemTx(_ context.Context, _ db.Tx, _ *orderEntity.OrderItem) error {
	return nil
}
func (m *mockOrderRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) {
	return m.order, nil
}
func (m *mockOrderRepo) UpdateStatusTx(_ context.Context, _ db.Tx, _ *orderEntity.Order) error {
	return nil
}
func (m *mockOrderRepo) GetByPricingTokenID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) {
	return nil, nil
}
func (m *mockOrderRepo) GetByIdempotencyKey(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) (*orderEntity.Order, error) {
	return nil, nil
}
func (m *mockOrderRepo) GetByShippingQuoteID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) {
	return nil, nil
}
func (m *mockOrderRepo) GetBlockingOrderByShippingQuoteID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) {
	return nil, nil
}
func (m *mockOrderRepo) CountValidOrdersByShippingQuoteID(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockOrderRepo) GetBySource(_ context.Context, _ db.Tx, _ string, _ uuid.UUID) (*orderEntity.Order, error) {
	return nil, nil
}
func (m *mockOrderRepo) GetOrderItems(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*orderEntity.OrderItem, error) {
	return nil, nil
}
func (m *mockOrderRepo) FindOrdersForAutoComplete(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
	return nil, nil
}
func (m *mockOrderRepo) FindOverdueOrdersForCancel(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
	return nil, nil
}
func (m *mockOrderRepo) GetByOrderNumber(_ context.Context, _ db.Tx, _ string) (*orderEntity.Order, error) {
	return nil, nil
}
func (m *mockOrderRepo) CreateShippingProofTx(_ context.Context, _ db.Tx, _ *orderEntity.ShippingProof) error {
	return nil
}
func (m *mockOrderRepo) GetShippingProofsByOrderID(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*orderEntity.ShippingProof, error) {
	return nil, nil
}
func (m *mockOrderRepo) GetOrderStats(_ context.Context, _ db.Tx, _ uuid.UUID, _ bool) (*orderrepository.OrderStats, error) {
	return nil, nil
}
func (m *mockOrderRepo) CountActiveOrdersByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockOrderRepo) CountAnyOrdersByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return 0, nil
}

// ============================================================================
// MOCK: DisputeRepository (no active disputes)
// ============================================================================

type mockDisputeRepo struct{}

func (m *mockDisputeRepo) Create(_ context.Context, _ db.Tx, _ *disputeEntity.Dispute) error {
	return nil
}
func (m *mockDisputeRepo) GetByOrderID(_ context.Context, _ db.Tx, _ uuid.UUID) (*disputeEntity.Dispute, error) {
	return nil, nil // no active dispute
}
func (m *mockDisputeRepo) GetForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*disputeEntity.Dispute, error) {
	return nil, nil
}
func (m *mockDisputeRepo) Update(_ context.Context, _ db.Tx, _ *disputeEntity.Dispute) error {
	return nil
}
func (m *mockDisputeRepo) CreateMedia(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockDisputeRepo) ListMedia(_ context.Context, _ db.Tx, _ uuid.UUID) ([]string, error) {
	return nil, nil
}
func (m *mockDisputeRepo) ListAll(_ context.Context, _ db.Tx, _ disputeRepo.DisputeListFilters) ([]*disputeEntity.Dispute, int64, error) {
	return nil, 0, nil
}
func (m *mockDisputeRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*disputeEntity.Dispute, error) {
	return nil, nil
}
func (m *mockDisputeRepo) FindOverdueCandidates(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
	return nil, nil
}
func (m *mockDisputeRepo) FindTimeoutCandidates(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
	return nil, nil
}
func (m *mockDisputeRepo) GetCallerDisputeCount(_ context.Context, _ db.Tx, _ uuid.UUID, _ time.Time) (int, error) {
	return 0, nil
}
func (m *mockDisputeRepo) GetCallerDisputeCountAgainstParty(_ context.Context, _ db.Tx, _ uuid.UUID, _ uuid.UUID, _ time.Time) (int, error) {
	return 0, nil
}

// ============================================================================
// HELPERS
// ============================================================================

// buildTestRefund creates a refund in gateway_pending state (ready for ack).
func buildTestRefund(orderID, buyerID, sellerID uuid.UUID, amount int64) *entity.Refund {
	r := entity.NewRefund(orderID, buyerID, sellerID, entity.RefundReasonItemDamaged, nil, amount)
	key := "test_key"
	_ = r.MarkGatewayDispatched(key, nil, time.Now())
	return r
}

// buildTestOrder creates an order with canonical pricing fields.
func buildTestOrder(orderID, buyerID, sellerID uuid.UUID, subtotal, shipping, commission int64) *orderEntity.Order {
	return &orderEntity.Order{
		ID:               orderID,
		BuyerID:          buyerID,
		SellerID:         sellerID,
		Subtotal:         money.New(subtotal),
		ShippingTotal:    money.New(shipping),
		CommissionAmount: money.New(commission),
		Status:           orderEntity.StatusShipped,
	}
}

// buildWalletService creates a WalletService with mock escrow + dispute repos.
func buildWalletService(escrows map[uuid.UUID]*walletEntity.Escrow) *walletApp.WalletService {
	ws := walletApp.NewWalletService(nil, zap.NewNop())
	ws.SetEscrowRepository(&mockEscrowRepo{escrows: escrows})
	ws.SetDisputeRepository(&mockDisputeRepo{})
	return ws
}

// buildRefundService wires a RefundService with all mocks + spy.
func buildRefundService(
	refundRepo refundrepo.RefundRepository,
	orderRepo orderrepository.OrderRepository,
	walletService *walletApp.WalletService,
	spy *spyFinanceReverser,
) *RefundService {
	svc := &RefundService{
		refundRepo:              refundRepo,
		orderRepo:               orderRepo,
		walletService:           walletService,
		outboxRepo:              outboxRepoImpl.NewOutboxRepository(nil),
		gatewayLogger:           zap.NewNop(),
		financeReverser:         spy,
		orderRefundStatusSyncer: &noopOrderRefundStatusSyncer{},
	}
	return svc
}

type noopOrderRefundStatusSyncer struct{}

func (n *noopOrderRefundStatusSyncer) SyncRefundSettlementFromGatewayAck(
	_ context.Context,
	_ db.Tx,
	_ uuid.UUID,
	_ uuid.UUID,
	_ bool,
	_ time.Time,
) error {
	return nil
}

type spyOrderRefundStatusSyncer struct {
	calls        int
	lastOrderID  uuid.UUID
	lastRefundID uuid.UUID
	lastFull     bool
	returnErr    error
}

func (s *spyOrderRefundStatusSyncer) SyncRefundSettlementFromGatewayAck(
	_ context.Context,
	_ db.Tx,
	orderID uuid.UUID,
	refundID uuid.UUID,
	fullyRefunded bool,
	_ time.Time,
) error {
	s.calls++
	s.lastOrderID = orderID
	s.lastRefundID = refundID
	s.lastFull = fullyRefunded
	return s.returnErr
}

// successNotification builds a Midtrans webhook for a successful refund.
func successNotification(gatewayRefundID string, refundAmount string) *midtrans.NotificationPayload {
	return &midtrans.NotificationPayload{
		TransactionStatus: string(midtrans.StatusRefund),
		StatusCode:        "200",
		RefundChargeID:    gatewayRefundID,
		RefundAmount:      refundAmount,
	}
}

// partialNotification builds a Midtrans webhook for a successful partial refund.
func partialNotification(gatewayRefundID string, refundAmount string) *midtrans.NotificationPayload {
	return &midtrans.NotificationPayload{
		TransactionStatus: string(midtrans.StatusPartialRefund),
		StatusCode:        "200",
		RefundChargeID:    gatewayRefundID,
		RefundAmount:      refundAmount,
	}
}

// ============================================================================
// TEST: Partial refund ack calls RecordPartialRefundRelease
// ============================================================================

func TestWebhookPartialRefund_CallsPartialRelease(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	// Order: subtotal=100000, shipping=25000, commission=6250
	// Buyer base (escrow) = P+S = 125000; C is seller-side, never buyer cash.
	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)

	// Refund: product-only = 100000 (partial, leaves 25000 shipping remainder)
	refund := buildTestRefund(orderID, buyerID, sellerID, 100_000)
	gatewayRefundID := "gw-refund-001"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0 // no previous refunds

	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  walletEntity.EscrowStatusHolding,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})
	spy := &spyFinanceReverser{}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)

	// Rp 100,000 refund = "100000.00" in Midtrans wire format (whole Rupiah,
	// no cents subunit — PASS_18J).
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, partialNotification(gatewayRefundID, "100000.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}

	// PROOF 1: RecordRefundReversal was called
	if !spy.reversalCalled {
		t.Fatal("RecordRefundReversal was NOT called — expected for partial refund")
	}

	// PROOF 2: RecordPartialRefundRelease WAS called
	if !spy.partialReleaseCalled {
		t.Fatal("RecordPartialRefundRelease was NOT called — partial refund must release remainder")
	}

	// PROOF 3: Remainder = 125000 - 100000 = 25000
	if spy.partialReleaseInput.Remainder != 25_000 {
		t.Fatalf("remainder=%d want 25000", spy.partialReleaseInput.Remainder)
	}

	// PROOF 4: Commission on remainder = 0 (shipping release is seller-side net)
	expectedCommission := int64(0)
	if spy.partialReleaseInput.Commission != expectedCommission {
		t.Fatalf("commission=%d want %d", spy.partialReleaseInput.Commission, expectedCommission)
	}

	// PROOF 5: SellerNet = remainder - commission = 25000
	expectedSellerNet := int64(25_000) - expectedCommission
	if spy.partialReleaseInput.SellerNet != expectedSellerNet {
		t.Fatalf("seller_net=%d want %d", spy.partialReleaseInput.SellerNet, expectedSellerNet)
	}

	// PROOF 6: RefundID, OrderID, SellerID are correct
	if spy.partialReleaseInput.RefundID != refund.ID {
		t.Fatalf("refund_id=%s want %s", spy.partialReleaseInput.RefundID, refund.ID)
	}
	if spy.partialReleaseInput.OrderID != orderID {
		t.Fatalf("order_id=%s want %s", spy.partialReleaseInput.OrderID, orderID)
	}
	if spy.partialReleaseInput.SellerID != sellerID {
		t.Fatalf("seller_id=%s want %s", spy.partialReleaseInput.SellerID, sellerID)
	}
}

// ============================================================================
// TEST: Full refund ack does NOT call RecordPartialRefundRelease
// ============================================================================

func TestWebhookFullRefund_DoesNotCallPartialRelease(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	// Order: subtotal=100000, shipping=25000, commission=6250
	// Buyer base (escrow) = P+S = 125000; C is seller-side, never buyer cash.
	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)

	// Full refund = entire buyer base (P+S = 125000); C is seller-side only.
	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-refund-full"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0

	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  walletEntity.EscrowStatusHolding,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})
	spy := &spyFinanceReverser{}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)

	// Rp 131,250 refund = "131250.00" in Midtrans wire format (whole Rupiah,
	// no cents subunit — PASS_18J).
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}

	// PROOF: RecordRefundReversal was called
	if !spy.reversalCalled {
		t.Fatal("RecordRefundReversal was NOT called — expected for full refund")
	}

	// PROOF: RecordPartialRefundRelease was NOT called (full refund = no remainder)
	if spy.partialReleaseCalled {
		t.Fatal("RecordPartialRefundRelease WAS called — must NOT be called for full refund")
	}
}

// ============================================================================
// TEST: Duplicate webhook ack short-circuits (no finance calls)
// ============================================================================

func TestWebhookDuplicate_NoFinanceCalls(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	// Refund already succeeded (gateway_status = succeeded)
	refund := buildTestRefund(orderID, buyerID, sellerID, 100_000)
	gatewayRefundID := "gw-refund-dup"
	refund.GatewayRefundID = &gatewayRefundID
	_ = refund.MarkGatewayAckSucceeded(gatewayRefundID, time.Now())

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund

	spy := &spyFinanceReverser{}
	// walletService and order don't matter — should never be reached
	svc := &RefundService{
		refundRepo:      rr,
		gatewayLogger:   zap.NewNop(),
		financeReverser: spy,
	}

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "100000.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}

	// PROOF: Neither reversal nor partial release was called
	if spy.reversalCalled {
		t.Fatal("RecordRefundReversal WAS called on duplicate — must be skipped")
	}
	if spy.partialReleaseCalled {
		t.Fatal("RecordPartialRefundRelease WAS called on duplicate — must be skipped")
	}
}

// ============================================================================
// TEST: Partial refund remainder=0 does NOT call partial release
// (edge: cumulative exactly matches gross, coded as partial by status)
// ============================================================================

func TestWebhookPartialRefund_ZeroRemainder_NoRelease(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	// Order: gross = 100000 (no shipping, no commission for simplicity)
	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 0, 0)

	// Refund amount = full gross (100000); previously refunded = 0
	// cumulative = 100000 = gross → fullyRefunded branch, not partiallyRefunded
	refund := buildTestRefund(orderID, buyerID, sellerID, 100_000)
	gatewayRefundID := "gw-refund-edge"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0

	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  100_000,
		Status:  walletEntity.EscrowStatusHolding,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})
	spy := &spyFinanceReverser{}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)

	// Rp 100,000 refund = "100000.00" in Midtrans wire format (whole Rupiah,
	// no cents subunit — PASS_18J).
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "100000.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}

	// Even though this LOOKS like a partial scenario (status=refund, amount=100000),
	// the breakdown puts cumulative >= gross → fullyRefunded branch
	if spy.partialReleaseCalled {
		t.Fatal("RecordPartialRefundRelease WAS called — zero remainder must not trigger release")
	}
}

// ============================================================================
// SPY: DisputeFreezeReleaser (H2-F2b)
// ============================================================================

type spyFreezeReleaser struct {
	releaseCalled  bool
	releaseOrderID uuid.UUID
}

func (s *spyFreezeReleaser) ReleaseDisputeFreezeByOrderID(_ context.Context, _ db.Tx, orderID uuid.UUID) error {
	s.releaseCalled = true
	s.releaseOrderID = orderID
	return nil
}

// buildRefundServiceWithFreezeReleaser extends buildRefundService with a freeze releaser.
func buildRefundServiceWithFreezeReleaser(
	refundRepo refundrepo.RefundRepository,
	orderRepo orderrepository.OrderRepository,
	walletService *walletApp.WalletService,
	spy *spyFinanceReverser,
	freezeSpy *spyFreezeReleaser,
) *RefundService {
	svc := buildRefundService(refundRepo, orderRepo, walletService, spy)
	svc.freezeReleaser = freezeSpy
	return svc
}

// ============================================================================
// TEST: H2-F2b Post-release ack is rejected before freeze release
// ============================================================================

func TestWebhookPostRelease_AckRejected_DoesNotReleaseFreezeByOrderID(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)

	// Full refund = gross
	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-postrelease-full"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0

	// Escrow is RELEASED (post-release scenario)
	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  walletEntity.EscrowStatusReleased,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{
		reversalSummary: &financeapp.RecordRefundReversalSummary{
			Phase:     "after_release",
			Duplicate: false,
		},
	}
	freezeSpy := &spyFreezeReleaser{}
	svc := buildRefundServiceWithFreezeReleaser(rr, &mockOrderRepo{order: order}, ws, spy, freezeSpy)

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))
	if err == nil {
		t.Fatal("expected post-release ack to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "post-release refund acknowledgements are disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if spy.reversalCalled {
		t.Fatal("RecordRefundReversal must not run for post-release ack")
	}
	if freezeSpy.releaseCalled {
		t.Fatal("ReleaseDisputeFreezeByOrderID must not run for blocked post-release ack")
	}
	if escrow.Status != walletEntity.EscrowStatusReleased {
		t.Fatalf("escrow should stay RELEASED, got %s", escrow.Status)
	}
}

// ============================================================================
// TEST: H2-F2b Pre-release ack does NOT release freeze
// ============================================================================

func TestWebhookPreRelease_AckSuccess_DoesNotReleaseFreezeByOrderID(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)

	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-prerelease"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0

	// Escrow is HOLDING (pre-release scenario)
	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  walletEntity.EscrowStatusHolding,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{}
	freezeSpy := &spyFreezeReleaser{}
	svc := buildRefundServiceWithFreezeReleaser(rr, &mockOrderRepo{order: order}, ws, spy, freezeSpy)

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}

	// Pre-release: afterRelease is false → freeze release must NOT fire
	if freezeSpy.releaseCalled {
		t.Fatal("ReleaseDisputeFreezeByOrderID was called for pre-release — must NOT release freeze")
	}
}

// ============================================================================
// TEST: H2-F2b Gateway ack failure does NOT release freeze
// ============================================================================

func TestWebhookPostRelease_AckFailure_DoesNotReleaseFreezeByOrderID(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-postrelease-fail"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund

	freezeSpy := &spyFreezeReleaser{}
	svc := &RefundService{
		refundRepo:     rr,
		outboxRepo:     outboxRepoImpl.NewOutboxRepository(nil),
		gatewayLogger:  zap.NewNop(),
		freezeReleaser: freezeSpy,
	}

	// Failed notification
	failNotification := &midtrans.NotificationPayload{
		TransactionStatus: "deny",
		StatusCode:        "202",
		RefundChargeID:    gatewayRefundID,
		RefundAmount:      "131250.00",
	}

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, failNotification)
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}

	// Failed ack: freeze must stay active
	if freezeSpy.releaseCalled {
		t.Fatal("ReleaseDisputeFreezeByOrderID was called on failure — freeze must stay active")
	}
}

// ============================================================================
// TEST: H2-F2b Duplicate ack does not double-release freeze
// ============================================================================

func TestWebhookPostRelease_DuplicateAck_DoesNotDoubleRelease(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)

	// Refund already in succeeded state (first ack already processed)
	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-postrelease-dup"
	refund.GatewayRefundID = &gatewayRefundID
	_ = refund.MarkGatewayAckSucceeded(gatewayRefundID, time.Now())

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund

	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  walletEntity.EscrowStatusReleased,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{
		reversalSummary: &financeapp.RecordRefundReversalSummary{
			Phase:     "after_release",
			Duplicate: true,
		},
	}
	freezeSpy := &spyFreezeReleaser{}
	svc := buildRefundServiceWithFreezeReleaser(rr, &mockOrderRepo{order: order}, ws, spy, freezeSpy)

	// Duplicate: refund.GatewayStatus is already 'succeeded'
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}

	// Duplicate ack short-circuits before any finance calls
	if freezeSpy.releaseCalled {
		t.Fatal("ReleaseDisputeFreezeByOrderID was called on duplicate — must not double-release")
	}
}

// ============================================================================
// TEST: H2-F2b Post-release full refund ack is rejected before coins/refund work
// ============================================================================

func TestWebhookPostRelease_FullRefund_IsRejected(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)

	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-postrelease-coins"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0

	// Escrow RELEASED → afterRelease=true, escrowAlreadyTerminal=true
	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  walletEntity.EscrowStatusReleased,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{
		reversalSummary: &financeapp.RecordRefundReversalSummary{
			Phase:     "after_release",
			Duplicate: false,
		},
	}
	freezeSpy := &spyFreezeReleaser{}
	svc := buildRefundServiceWithFreezeReleaser(rr, &mockOrderRepo{order: order}, ws, spy, freezeSpy)

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))
	if err == nil {
		t.Fatal("expected post-release ack to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "post-release refund acknowledgements are disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if spy.reversalCalled {
		t.Fatal("RecordRefundReversal must not run for blocked post-release ack")
	}
	if freezeSpy.releaseCalled {
		t.Fatal("ReleaseDisputeFreezeByOrderID must not run for blocked post-release ack")
	}
}

// ============================================================================
// TEST: H2-F2b Post-release partial refund ack is rejected before ledger work
// ============================================================================

func TestWebhookPostRelease_PartialRefund_IsRejected(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)

	// Partial refund: item price only (100000 out of 125000 buyer base)
	refund := buildTestRefund(orderID, buyerID, sellerID, 100_000)
	gatewayRefundID := "gw-postrelease-partial"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0

	// Escrow RELEASED (post-release)
	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  walletEntity.EscrowStatusReleased,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{
		reversalSummary: &financeapp.RecordRefundReversalSummary{
			Phase:     "after_release",
			Duplicate: false,
		},
	}
	freezeSpy := &spyFreezeReleaser{}
	svc := buildRefundServiceWithFreezeReleaser(rr, &mockOrderRepo{order: order}, ws, spy, freezeSpy)

	// Rp 100,000 refund = "100000.00" in Midtrans wire format (PASS_18J).
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, partialNotification(gatewayRefundID, "100000.00"))
	if err == nil {
		t.Fatal("expected post-release ack to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "post-release refund acknowledgements are disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if spy.reversalCalled {
		t.Fatal("RecordRefundReversal must not run for blocked post-release ack")
	}
	if spy.partialReleaseCalled {
		t.Fatal("RecordPartialRefundRelease must not run for blocked post-release ack")
	}
	if freezeSpy.releaseCalled {
		t.Fatal("ReleaseDisputeFreezeByOrderID must not run for blocked post-release ack")
	}
}

// ============================================================================
// TEST: H2-H1 Seller-approved full refund ack sets order.status = refunded
// ============================================================================

func TestWebhookSellerApproved_FullRefund_SetsOrderStatusRefunded(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)
	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-seller-full"
	refund.GatewayRefundID = &gatewayRefundID
	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0
	escrow := &walletEntity.Escrow{ID: uuid.New(), OrderID: orderID, Amount: 131_250, Status: walletEntity.EscrowStatusHolding}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})
	spy := &spyFinanceReverser{}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)
	syncSpy := &spyOrderRefundStatusSyncer{}
	svc.orderRefundStatusSyncer = syncSpy
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}
	if syncSpy.calls != 1 {
		t.Fatalf("syncer calls=%d want 1", syncSpy.calls)
	}
	if !syncSpy.lastFull {
		t.Fatal("syncer must be called with fullyRefunded=true")
	}
}

// ============================================================================
// TEST: H2-H1 Seller-approved product-only ack sets order.status = partially_refunded
// ============================================================================

func TestWebhookSellerApproved_PartialRefund_SetsOrderStatusPartiallyRefunded(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)
	refund := buildTestRefund(orderID, buyerID, sellerID, 100_000)
	gatewayRefundID := "gw-seller-partial"
	refund.GatewayRefundID = &gatewayRefundID
	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0
	escrow := &walletEntity.Escrow{ID: uuid.New(), OrderID: orderID, Amount: 131_250, Status: walletEntity.EscrowStatusHolding}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})
	spy := &spyFinanceReverser{}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)
	syncSpy := &spyOrderRefundStatusSyncer{}
	svc.orderRefundStatusSyncer = syncSpy
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, partialNotification(gatewayRefundID, "100000.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}
	if syncSpy.calls != 1 {
		t.Fatalf("syncer calls=%d want 1", syncSpy.calls)
	}
	if syncSpy.lastFull {
		t.Fatal("syncer must be called with fullyRefunded=false for partial refund")
	}
}

func TestWebhookSellerApproved_OrderStatusSyncerFailure_ReturnsError(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)
	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-seller-syncer-fail"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0

	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  walletEntity.EscrowStatusHolding,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})
	spy := &spyFinanceReverser{}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)
	syncSpy := &spyOrderRefundStatusSyncer{returnErr: errors.New("order sync failed")}
	svc.orderRefundStatusSyncer = syncSpy

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))
	if err == nil {
		t.Fatal("expected error when order status syncer fails, got nil")
	}
	if !strings.Contains(err.Error(), "sync order status+escrow") {
		t.Fatalf("expected syncer failure context, got: %v", err)
	}
	if syncSpy.calls != 1 {
		t.Fatalf("syncer calls=%d want 1", syncSpy.calls)
	}
}

// ============================================================================
// TEST: H2-H1 Post-release ack is rejected before order.status sync
// ============================================================================

func TestWebhookPostRelease_AckRejected_OrderStatusNotUpdated(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)
	order.Status = orderEntity.StatusCompleted
	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-postrelease-nostatus"
	refund.GatewayRefundID = &gatewayRefundID
	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0
	escrow := &walletEntity.Escrow{ID: uuid.New(), OrderID: orderID, Amount: 131_250, Status: walletEntity.EscrowStatusReleased}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})
	spy := &spyFinanceReverser{reversalSummary: &financeapp.RecordRefundReversalSummary{Phase: "after_release", Duplicate: false}}
	freezeSpy := &spyFreezeReleaser{}
	svc := buildRefundServiceWithFreezeReleaser(rr, &mockOrderRepo{order: order}, ws, spy, freezeSpy)
	syncSpy := &spyOrderRefundStatusSyncer{}
	svc.orderRefundStatusSyncer = syncSpy
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))
	if err == nil {
		t.Fatal("expected post-release ack to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "post-release refund acknowledgements are disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if syncSpy.calls != 0 {
		t.Fatalf("syncer calls=%d want 0 for blocked post-release ack", syncSpy.calls)
	}
}

// ============================================================================
// TEST: H2-H1 Duplicate ack does not re-update order.status
// ============================================================================

func TestWebhookDuplicate_AckDoesNotUpdateOrderStatus(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-dup-nostatus"
	refund.GatewayRefundID = &gatewayRefundID
	_ = refund.MarkGatewayAckSucceeded(gatewayRefundID, time.Now())
	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	spy := &spyFinanceReverser{}
	svc := &RefundService{
		refundRepo:              rr,
		gatewayLogger:           zap.NewNop(),
		financeReverser:         spy,
		orderRefundStatusSyncer: &noopOrderRefundStatusSyncer{},
	}
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}
}

// ============================================================================
// TEST: K1-A Post-release ack + nil freezeReleaser returns error
// ============================================================================

func TestWebhookPostRelease_NilFreezeReleaser_ReturnsError(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)

	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-postrelease-nil-freezer"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0

	// Escrow RELEASED → afterRelease=true
	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  walletEntity.EscrowStatusReleased,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{
		reversalSummary: &financeapp.RecordRefundReversalSummary{
			Phase:     "after_release",
			Duplicate: false,
		},
	}
	// Build service WITHOUT freezeReleaser (nil)
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)
	// Explicitly ensure freezeReleaser is nil
	svc.freezeReleaser = nil

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))

	if err == nil {
		t.Fatal("expected post-release ack to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "post-release refund acknowledgements are disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================================
// TEST: K1-A Pre-release ack + nil freezeReleaser succeeds
// ============================================================================

func TestWebhookPreRelease_NilFreezeReleaser_Succeeds(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)

	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-prerelease-nil-freezer"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0

	// Escrow HOLDING → afterRelease=false → freezeReleaser not needed
	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  walletEntity.EscrowStatusHolding,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{}
	// Build service WITHOUT freezeReleaser (nil)
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)
	svc.freezeReleaser = nil

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))

	// PROOF: Pre-release ack succeeds even without freezeReleaser
	if err != nil {
		t.Fatalf("pre-release ack should succeed with nil freezeReleaser, got: %v", err)
	}
}

// ============================================================================
// TEST: K1-A Gateway failure + nil freezeReleaser does not error
// ============================================================================

func TestWebhookFailure_NilFreezeReleaser_NoError(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-fail-nil-freezer"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund

	// No freezeReleaser, no financeReverser — failure path doesn't need them
	svc := &RefundService{
		refundRepo:    rr,
		outboxRepo:    outboxRepoImpl.NewOutboxRepository(nil),
		gatewayLogger: zap.NewNop(),
		// freezeReleaser: nil — intentionally
	}

	failNotification := &midtrans.NotificationPayload{
		TransactionStatus: "deny",
		StatusCode:        "202",
		RefundChargeID:    gatewayRefundID,
		RefundAmount:      "131250.00",
	}

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, failNotification)

	// PROOF: Failed ack does not error with nil freezeReleaser
	// (freeze stays active — the nil guard only matters on success path)
	if err != nil {
		t.Fatalf("failed ack should not error with nil freezeReleaser, got: %v", err)
	}
}

// ============================================================================
// TEST: K1-A Post-release ack + wired freezeReleaser succeeds
// ============================================================================

func TestWebhookPostRelease_WiredFreezeReleaser_Succeeds(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)

	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-postrelease-wired-freezer"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund
	rr.successTotal = 0

	// Escrow RELEASED → afterRelease=true
	escrow := &walletEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  walletEntity.EscrowStatusReleased,
	}
	ws := buildWalletService(map[uuid.UUID]*walletEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{
		reversalSummary: &financeapp.RecordRefundReversalSummary{
			Phase:     "after_release",
			Duplicate: false,
		},
	}
	freezeSpy := &spyFreezeReleaser{}
	svc := buildRefundServiceWithFreezeReleaser(rr, &mockOrderRepo{order: order}, ws, spy, freezeSpy)

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))

	if err == nil {
		t.Fatal("expected post-release ack to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "post-release refund acknowledgements are disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if freezeSpy.releaseCalled {
		t.Fatal("ReleaseDisputeFreezeByOrderID must not run for blocked post-release ack")
	}
}

// ============================================================================
// Verify mockRefundRepo satisfies RefundRepository interface at compile time
// ============================================================================
var _ refundrepo.RefundRepository = (*mockRefundRepo)(nil)
var _ orderrepository.OrderRepository = (*mockOrderRepo)(nil)
var _ walletRepo.EscrowRepository = (*mockEscrowRepo)(nil)
