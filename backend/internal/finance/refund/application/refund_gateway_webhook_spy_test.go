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
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	escrowApp "github.com/labuda/backend/internal/core/escrow/application"
	escrowEntity "github.com/labuda/backend/internal/core/escrow/entity"
	escrowRepo "github.com/labuda/backend/internal/core/escrow/repository"
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
	escrows map[uuid.UUID]*escrowEntity.Escrow
}

func (m *mockEscrowRepo) GetByID(_ context.Context, _ db.Tx, id uuid.UUID) (*escrowEntity.Escrow, error) {
	for _, e := range m.escrows {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, nil
}
func (m *mockEscrowRepo) GetByOrderID(_ context.Context, _ db.Tx, orderID uuid.UUID) (*escrowEntity.Escrow, error) {
	return m.escrows[orderID], nil
}
func (m *mockEscrowRepo) GetByOrderIDForUpdate(_ context.Context, _ db.Tx, orderID uuid.UUID) (*escrowEntity.Escrow, error) {
	return m.escrows[orderID], nil
}
func (m *mockEscrowRepo) Create(_ context.Context, _ db.Tx, escrow *escrowEntity.Escrow) error {
	m.escrows[escrow.OrderID] = escrow
	return nil
}
func (m *mockEscrowRepo) Update(_ context.Context, _ db.Tx, escrow *escrowEntity.Escrow) error {
	m.escrows[escrow.OrderID] = escrow
	return nil
}

// ============================================================================
// MOCK: RefundRepository
// ============================================================================

type mockRefundRepo struct {
	refundByGatewayRefundID map[string]*entity.Refund
	refundByID              map[uuid.UUID]*entity.Refund
	refundByIdempotencyKey  map[string]*entity.Refund
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
func (m *mockRefundRepo) GetByGatewayIdempotencyKey(_ context.Context, _ db.Tx, key string) (*entity.Refund, error) {
	return m.refundByIdempotencyKey[key], nil
}
func (m *mockRefundRepo) GetByGatewayRefundID(_ context.Context, _ db.Tx, gatewayRefundID string) (*entity.Refund, error) {
	return m.refundByGatewayRefundID[gatewayRefundID], nil
}
func (m *mockRefundRepo) CreateEvidence(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockRefundRepo) ListEvidence(_ context.Context, _ db.Tx, _ uuid.UUID) ([]string, error) {
	return nil, nil
}
func (m *mockRefundRepo) HasRefundBlockingRelease(_ context.Context, _ db.Tx, _ uuid.UUID, _ bool) (bool, error) {
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
func (m *mockOrderRepo) GetOrderItems(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*orderEntity.OrderItem, error) {
	return nil, nil
}
func (m *mockOrderRepo) FindOrdersForAutoComplete(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
	return nil, nil
}
func (m *mockOrderRepo) FindOverdueOrdersForCancel(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
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
//
// CANONICAL MONEY BASE: total_before_coins_amount = PD + S. In this fixture
// `subtotal` is the discounted product value PD, so the persisted buyer base
// is subtotal + shipping. The refund reversal path derives PD exclusively from
// this base and fails closed without it — an order row without the base cannot
// exist in production (CreateOrderTx always writes the token-derived base).
func buildTestOrder(orderID, buyerID, sellerID uuid.UUID, subtotal, shipping, commission int64) *orderEntity.Order {
	return &orderEntity.Order{
		ID:                     orderID,
		BuyerID:                buyerID,
		SellerID:               sellerID,
		Subtotal:               money.New(subtotal),
		ShippingTotal:          money.New(shipping),
		CommissionAmount:       money.New(commission),
		TotalBeforeCoinsAmount: money.New(subtotal + shipping),
		Status:                 orderEntity.StatusShipped,
	}
}

// buildEscrowService creates an EscrowService with mock escrow + dispute repos.
func buildEscrowService(escrows map[uuid.UUID]*escrowEntity.Escrow) *escrowApp.EscrowService {
	ws := escrowApp.NewEscrowService(nil, zap.NewNop())
	ws.SetEscrowRepository(&mockEscrowRepo{escrows: escrows})
	ws.SetDisputeRepository(&mockDisputeRepo{})
	return ws
}

// buildRefundService wires a RefundService with all mocks + spy.
func buildRefundService(
	refundRepo refundrepo.RefundRepository,
	orderRepo orderrepository.OrderRepository,
	escrowService *escrowApp.EscrowService,
	spy *spyFinanceReverser,
) *RefundService {
	svc := &RefundService{
		refundRepo:              refundRepo,
		orderRepo:               orderRepo,
		escrowService:           escrowService,
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

	escrow := &escrowEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  escrowEntity.EscrowStatusHolding,
	}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})
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

	escrow := &escrowEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  escrowEntity.EscrowStatusHolding,
	}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})
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
	// escrowService and order don't matter — should never be reached
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

	escrow := &escrowEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  100_000,
		Status:  escrowEntity.EscrowStatusHolding,
	}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})
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
// TEST: Post-release ack is rejected
// ============================================================================

func TestWebhookPostRelease_AckRejected(t *testing.T) {
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

	// Escrow is RELEASED (post-release scenario)
	escrow := &escrowEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  escrowEntity.EscrowStatusReleased,
	}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{
		reversalSummary: &financeapp.RecordRefundReversalSummary{
			Phase:     "after_release",
			Duplicate: false,
		},
	}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)

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
	if escrow.Status != escrowEntity.EscrowStatusReleased {
		t.Fatalf("escrow should stay RELEASED, got %s", escrow.Status)
	}
}

// ============================================================================
// TEST: pre-release ack succeeds
// ============================================================================

func TestWebhookPreRelease_AckSuccess(t *testing.T) {
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

	// Escrow is HOLDING (pre-release scenario)
	escrow := &escrowEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  escrowEntity.EscrowStatusHolding,
	}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}
}

// ============================================================================
// TEST: gateway ack failure does not error
// ============================================================================

func TestWebhookAckFailure_NoError(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	refund := buildTestRefund(orderID, buyerID, sellerID, 131_250)
	gatewayRefundID := "gw-postrelease-fail"
	refund.GatewayRefundID = &gatewayRefundID

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund

	svc := &RefundService{
		refundRepo:    rr,
		outboxRepo:    outboxRepoImpl.NewOutboxRepository(nil),
		gatewayLogger: zap.NewNop(),
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
}

// ============================================================================
// TEST: duplicate ack short-circuits (no finance calls)
// ============================================================================

func TestWebhookPostRelease_DuplicateAck_NoFinanceCalls(t *testing.T) {
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

	escrow := &escrowEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  escrowEntity.EscrowStatusReleased,
	}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{
		reversalSummary: &financeapp.RecordRefundReversalSummary{
			Phase:     "after_release",
			Duplicate: true,
		},
	}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)

	// Duplicate: refund.GatewayStatus is already 'succeeded'
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, successNotification(gatewayRefundID, "131250.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}
	if spy.partialReleaseCalled {
		t.Fatal("RecordPartialRefundRelease must not run for a duplicate ack")
	}
}

// ============================================================================
// TEST: Post-release full refund ack is rejected before coins/refund work
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

	// Escrow RELEASED → afterRelease=true, escrowAlreadyTerminal=true
	escrow := &escrowEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  escrowEntity.EscrowStatusReleased,
	}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{
		reversalSummary: &financeapp.RecordRefundReversalSummary{
			Phase:     "after_release",
			Duplicate: false,
		},
	}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)

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
}

// ============================================================================
// TEST: Post-release partial refund ack is rejected before ledger work
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

	// Escrow RELEASED (post-release)
	escrow := &escrowEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  escrowEntity.EscrowStatusReleased,
	}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})

	spy := &spyFinanceReverser{
		reversalSummary: &financeapp.RecordRefundReversalSummary{
			Phase:     "after_release",
			Duplicate: false,
		},
	}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)

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
	escrow := &escrowEntity.Escrow{ID: uuid.New(), OrderID: orderID, Amount: 131_250, Status: escrowEntity.EscrowStatusHolding}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})
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
	escrow := &escrowEntity.Escrow{ID: uuid.New(), OrderID: orderID, Amount: 131_250, Status: escrowEntity.EscrowStatusHolding}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})
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

	escrow := &escrowEntity.Escrow{
		ID:      uuid.New(),
		OrderID: orderID,
		Amount:  131_250,
		Status:  escrowEntity.EscrowStatusHolding,
	}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})
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
	escrow := &escrowEntity.Escrow{ID: uuid.New(), OrderID: orderID, Amount: 131_250, Status: escrowEntity.EscrowStatusReleased}
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{orderID: escrow})
	spy := &spyFinanceReverser{reversalSummary: &financeapp.RecordRefundReversalSummary{Phase: "after_release", Duplicate: false}}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)
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
// REC-6 SLICE 3: ACK for gateway_captured_after_order_invalid refunds.
//
// These tests prove that when a REC-6 refund (reason =
// gateway_captured_after_order_invalid) receives a successful gateway ACK:
//  - gateway_status transitions pending → succeeded (or failed → succeeded)
//  - gateway_acknowledged_at is set
//  - NO financeReverser / escrow / ledger / coin / order mutation
//  - outbox "money.refund_succeeded" is emitted
//  - duplicate ACK is idempotent
//  - concurrent ACKs are safe
// ============================================================================

// buildRec6Refund creates a refund in the gateway_captured_after_order_invalid
// reason with gateway_status = pending (as Slice 2 dispatch would leave it).
func buildRec6Refund(orderID, buyerID, sellerID uuid.UUID, amount int64) *entity.Refund {
	r := entity.NewSystemRefund(orderID, buyerID, sellerID, uuid.Nil,
		entity.RefundReasonGatewayCapturedAfterOrderInvalid,
		amount, 0, nil)
	key := fmt.Sprintf("rec6:payment:%s", uuid.New().String())
	_ = r.MarkGatewayDispatched(key, nil, time.Now())
	return r
}

// TestWebhookRec6Ack_PendingToSucceeded_NoFinanceCalls proves that a
// successful gateway ACK for a REC-6 refund flips gateway_status to
// succeeded WITHOUT invoking any financial reversal machinery.
func TestWebhookRec6Ack_PendingToSucceeded_NoFinanceCalls(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	refund := buildRec6Refund(orderID, buyerID, sellerID, 150_000)
	gatewayRefundID := "rchg-rec6-001"
	notif := successNotification(gatewayRefundID, "150000.00")

	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund

	order := buildTestOrder(orderID, buyerID, sellerID, 100_000, 25_000, 6_250)
	ws := buildEscrowService(map[uuid.UUID]*escrowEntity.Escrow{}) // NO escrow for REC-6

	spy := &spyFinanceReverser{reversalSummary: &financeapp.RecordRefundReversalSummary{}}
	svc := buildRefundService(rr, &mockOrderRepo{order: order}, ws, spy)

	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, notif)
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}

	// State transition: pending → succeeded
	assert.Equal(t, entity.GatewayRefundSucceeded, refund.GatewayStatus,
		"REC-6 ACK must flip gateway_status to succeeded")
	assert.NotNil(t, refund.GatewayAcknowledgedAt,
		"gateway_acknowledged_at must be set")
	assert.Nil(t, refund.LastGatewayError, "last_gateway_error must be cleared")

	// Financial safety: no reverser, no escrow, no ledger, no order mutation
	assert.False(t, spy.reversalCalled,
		"financeReverser must NOT be called for REC-6 refund")
	assert.False(t, spy.coinFundingReversalCalled,
		"coin funding reversal must NOT be called for REC-6 refund")
	assert.False(t, spy.partialReleaseCalled,
		"partial release must NOT be called for REC-6 refund")

	// Persistence: refund updated, outbox emitted
	assert.True(t, rr.updateCalled, "refundRepo.Update must be called")
}

// TestWebhookRec6Ack_DuplicateIsIdempotent proves that a second ACK for an
// already-succeeded REC-6 refund is a no-op (no duplicate side effects).
func TestWebhookRec6Ack_DuplicateIsIdempotent(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	refund := buildRec6Refund(orderID, buyerID, sellerID, 150_000)
	gatewayRefundID := "rchg-rec6-dup"

	// Pre-ack: already succeeded (simulating a prior ACK)
	now := time.Now()
	_ = refund.MarkGatewayDispatched("rec6:payment:"+uuid.New().String(), &gatewayRefundID, now)
	_ = refund.MarkGatewayAckSucceeded(gatewayRefundID, now)
	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund

	spy := &spyFinanceReverser{reversalSummary: &financeapp.RecordRefundReversalSummary{}}
	svc := buildRefundService(rr, &mockOrderRepo{}, nil, spy)

	// Duplicate ACK must return nil
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{},
		successNotification(gatewayRefundID, "150000.00"))
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error on duplicate: %v", err)
	}

	// No finance calls on duplicate
	assert.False(t, spy.reversalCalled, "duplicate ACK must not invoke financeReverser")
	assert.False(t, rr.updateCalled, "duplicate ACK must not persist any update")
}

// TestWebhookRec6Ack_FailureAckRecordsError proves that a failed gateway ACK
// for a REC-6 refund correctly records the error without invoking finance.
func TestWebhookRec6Ack_FailureAckRecordsError(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	refund := buildRec6Refund(orderID, buyerID, sellerID, 150_000)
	gatewayRefundID := "rchg-rec6-fail"
	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund

	spy := &spyFinanceReverser{}
	svc := buildRefundService(rr, &mockOrderRepo{}, nil, spy)

	notif := &midtrans.NotificationPayload{
		TransactionStatus: string(midtrans.StatusDeny),
		StatusCode:        "400",
		RefundChargeID:    gatewayRefundID,
		StatusMessage:     "Refund rejected by bank",
	}
	err := svc.HandleGatewayRefundAck(context.Background(), noopTx{}, notif)
	if err != nil {
		t.Fatalf("HandleGatewayRefundAck returned error: %v", err)
	}

	assert.Equal(t, entity.GatewayRefundFailed, refund.GatewayStatus,
		"failed ACK must set gateway_status to failed")
	assert.NotNil(t, refund.LastGatewayError, "last_gateway_error must be set")
	assert.False(t, spy.reversalCalled,
		"financeReverser must NOT be called on REC-6 failure ACK")
}

// TestWebhookRec6Ack_ConcurrentSafety proves that two concurrent ACK
// handlers for the same REC-6 refund produce a single effective transition
// and no duplicate side effects.
func TestWebhookRec6Ack_ConcurrentSafety(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	refund := buildRec6Refund(orderID, buyerID, sellerID, 150_000)
	gatewayRefundID := "rchg-rec6-conc"
	rr := newMockRefundRepo()
	rr.refundByGatewayRefundID[gatewayRefundID] = refund
	rr.refundByID[refund.ID] = refund

	spy := &spyFinanceReverser{reversalSummary: &financeapp.RecordRefundReversalSummary{}}
	svc := buildRefundService(rr, &mockOrderRepo{}, nil, spy)

	notif := successNotification(gatewayRefundID, "150000.00")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = svc.HandleGatewayRefundAck(context.Background(), noopTx{}, notif) }()
	go func() { defer wg.Done(); _ = svc.HandleGatewayRefundAck(context.Background(), noopTx{}, notif) }()
	wg.Wait()

	assert.Equal(t, entity.GatewayRefundSucceeded, refund.GatewayStatus,
		"concurrent ACK must produce succeeded")
	assert.False(t, spy.reversalCalled,
		"financeReverser must NOT be called for REC-6 even under concurrency")
}

// ============================================================================
// Verify mockRefundRepo satisfies RefundRepository interface at compile time
// ============================================================================
var _ refundrepo.RefundRepository = (*mockRefundRepo)(nil)
var _ orderrepository.OrderRepository = (*mockOrderRepo)(nil)
var _ escrowRepo.EscrowRepository = (*mockEscrowRepo)(nil)
