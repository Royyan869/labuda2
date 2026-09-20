package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/internal/commerce/order/entity"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// TESTS: OrderCompletionService.ReleaseFromDispute (gateway-aware)
// ============================================================================
//
// Protects the dispute seller-favor release contract:
//
//  1. Uses paymentService.ReleaseGatewayEscrowToSeller (the canonical
//     gateway-funded release path; there is no balance-hold release method
//     was demolished).
//  2. Sets order.Status=completed, order.EscrowStatus=released,
//     order.CompletedAt=now, order.UpdatedAt=now and persists via UpdateStatusTx.
//  3. Emits order.completed and money.released (with gross/commission/seller_net/
//     newly_released/released_at).
//  4. Does NOT grant loyalty points.
//  5. Does NOT fulfill content/request automatically.
//  6. Rejects orders that have no open dispute.
//  7. Rejects orders whose status is not dispute_open.
//
// Guards that return early (no payment service / escrow service required) are
// covered by real unit tests below. Happy-path / "does not call legacy" / replay
// idempotency assertions are documented as integration scenarios because
// OrderCompletionService is constructed with concrete struct dependencies
// (*OrderPaymentService, *escrowApp.EscrowService, *outboxRepo.OutboxRepository,
// *paymentRepo.PaymentRepository, ...) — full mocking would require a larger
// new test framework which is explicitly out of scope for this patch.
// ============================================================================

// stubOrderRepo is a minimal OrderRepository fake supporting only the methods
// touched by ReleaseFromDispute's guard checks: GetForUpdate and (optionally)
// UpdateStatusTx. All other interface methods return zero values.
type stubOrderRepo struct {
	orderrepository.OrderRepository

	getForUpdateFn   func(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Order, error)
	updateStatusFn   func(ctx context.Context, tx db.Tx, order *entity.Order) error
	updateStatusCalls int
}

func (s *stubOrderRepo) GetForUpdate(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*entity.Order, error) {
	return s.getForUpdateFn(ctx, tx, orderID)
}

func (s *stubOrderRepo) UpdateStatusTx(ctx context.Context, tx db.Tx, order *entity.Order) error {
	s.updateStatusCalls++
	if s.updateStatusFn != nil {
		return s.updateStatusFn(ctx, tx, order)
	}
	return nil
}

// newOrderForDisputeRelease constructs an order in the canonical pre-release
// dispute state.
func newOrderForDisputeRelease() *entity.Order {
	return &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusDisputeOpen,
		EscrowStatus: entity.EscrowStatusHolding,
		HasDispute:   true,
	}
}

func TestReleaseFromDispute_RejectsOrderWithoutDispute(t *testing.T) {
	logger := zaptest.NewLogger(t)

	order := newOrderForDisputeRelease()
	order.HasDispute = false // dispute guard should fire

	repo := &stubOrderRepo{
		getForUpdateFn: func(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Order, error) {
			return order, nil
		},
	}

	svc := &OrderCompletionService{
		repo:   repo,
		logger: logger,
	}

	err := svc.ReleaseFromDispute(context.Background(), nil, order.ID, uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "open dispute")
	assert.Equal(t, 0, repo.updateStatusCalls,
		"order must not be persisted when dispute guard rejects the call")
}

func TestReleaseFromDispute_RejectsWrongStatus(t *testing.T) {
	logger := zaptest.NewLogger(t)

	order := newOrderForDisputeRelease()
	order.Status = entity.StatusShipped // any non-dispute_open status

	repo := &stubOrderRepo{
		getForUpdateFn: func(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Order, error) {
			return order, nil
		},
	}

	svc := &OrderCompletionService{
		repo:   repo,
		logger: logger,
	}

	err := svc.ReleaseFromDispute(context.Background(), nil, order.ID, uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state")
	assert.Equal(t, 0, repo.updateStatusCalls,
		"order must not be persisted when status guard rejects the call")
}

// TestReleaseFromDispute_GatewayReleaseHappyPath documents the canonical
// gateway-funded happy path. Pure unit-mocking is not feasible because
// OrderCompletionService depends on concrete *OrderPaymentService /
// *escrowApp.EscrowService / *outboxRepo.OutboxRepository (no interfaces);
// covering this path requires an integration harness with a real test DB.
//
// Expected behavior (must be exercised by the integration suite):
//   - paymentService.ReleaseGatewayEscrowToSeller is invoked exactly once,
//     flipping escrow to "released" and writing the finance ledger
//     (idempotency_key="order_release_<order_id>").
//   - order.Status=completed, order.EscrowStatus=released,
//     order.CompletedAt!=nil, order.UpdatedAt!=nil are persisted via UpdateStatusTx.
//   - One outbox row with event_type="order.completed".
//   - One outbox row with event_type="money.released" whose payload includes
//     order_id, seller_id, gross, commission, seller_net, newly_released,
//     released_at.
//   - No loyalty points granted (coinsService.EarnPointsForOrderCompletion
//     must NOT be called).
//   - fulfillRequestsFromOrder must NOT be called.
// SKIPPED, NOT PASSING: this test asserts nothing. It previously reported a
// green PASS while only logging the expectations, which is FALSE CONFIDENCE —
// the canonical happy path stays unproven until it is exercised against a real
// database (escrow flip + finance ledger + order.completed + money.released).
func TestReleaseFromDispute_GatewayReleaseHappyPath(t *testing.T) {
	t.Skip("UNPROVEN: requires an integration harness (real DB) — escrow flip, finance ledger, order.completed + money.released")
}

// TestReleaseFromDispute_UsesGatewayReleaseOnly documents that the
// canonical gateway release is the only available path; the legacy
// balance-hold release methods have been demolished from the codebase.
// SKIPPED, NOT PASSING: asserts nothing today (was a false PASS). The claim
// that ReleaseGatewayEscrowToSeller is the only release path is proven by
// absence of the legacy methods (compile-time) — not by this test.
func TestReleaseFromDispute_UsesGatewayReleaseOnly(t *testing.T) {
	t.Skip("UNPROVEN: no call spy available; legacy methods are absent at compile time instead")
}

// TestReleaseFromDispute_IdempotentReplay documents the replay-safe contract.
// The escrow flip and finance ledger write are both idempotent (UNIQUE
// idempotency_key="order_release_<order_id>"), and the money.released outbox
// row is deduped on idempotency_key="money.released.<order_id>". A repeated
// call on an already-released dispute must therefore be a no-op rather than an
// error or a double release.
// SKIPPED, NOT PASSING: idempotent replay needs the real escrow/ledger/outbox
// constraints (UNIQUE idempotency_key) to be meaningful; logging it was a false PASS.
func TestReleaseFromDispute_IdempotentReplay(t *testing.T) {
	t.Skip("UNPROVEN: requires an integration harness (real DB) to exercise release idempotency")
}


