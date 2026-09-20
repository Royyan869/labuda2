package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// TESTS: OrderCompletionService.ReleaseFromDispute vs a FINAL refund decision
// ============================================================================
//
// REAL PROOF tests: they drive the production ReleaseFromDispute and only
// stub the two outbound authorities (order repo + refund decision authority).
//
// Locked business truth being protected:
//
//	Seller ACCEPT / admin buyer-wins / platform-initiated refund
//	  => FINAL refund decision that owes the buyer money.
//
// The order's escrow therefore belongs to the buyer. An admin "seller wins"
// resolution must NOT pay the seller as well (that would move the same money
// twice and contradict a final decision). Fail closed.
// ============================================================================

// stubRefundDecisionAuthority is a refund-decision double that records how it
// was consulted. It only stubs the OUTBOUND authority; the order-side guard
// logic under test is the real production code.
type stubRefundDecisionAuthority struct {
	owedToBuyer   bool
	owedErr       error
	owedCalls     int
	resolveCalls  int
	lastOrderID   uuid.UUID
	resolveRecord bool
}

func (s *stubRefundDecisionAuthority) AdminResolveRefundDecision(
	_ context.Context, _ db.Tx, _ uuid.UUID, _ uuid.UUID, _ bool, _ int64, _ *string,
) (bool, error) {
	s.resolveCalls++
	return s.resolveRecord, nil
}

func (s *stubRefundDecisionAuthority) HasFinalRefundDecisionOwedToBuyer(
	_ context.Context, _ db.Tx, orderID uuid.UUID,
) (bool, error) {
	s.owedCalls++
	s.lastOrderID = orderID
	return s.owedToBuyer, s.owedErr
}

func newDisputeOrderRepo(order *entity.Order) *stubOrderRepo {
	return &stubOrderRepo{
		getForUpdateFn: func(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Order, error) {
			return order, nil
		},
	}
}

func TestReleaseFromDispute_BlockedWhenFinalRefundDecisionOwesBuyer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	order := newOrderForDisputeRelease()
	repo := newDisputeOrderRepo(order)
	authority := &stubRefundDecisionAuthority{owedToBuyer: true}

	svc := &OrderCompletionService{
		repo:               repo,
		logger:             logger,
		refundDecisionAuth: authority,
	}

	err := svc.ReleaseFromDispute(context.Background(), nil, order.ID, uuid.New())

	require.Error(t, err, "a final buyer-wins refund decision must block the seller-wins release")
	assert.ErrorIs(t, err, ErrFinalRefundDecisionOwesBuyer,
		"the failure must name the final refund decision that blocks the release")
	assert.Equal(t, 0, repo.updateStatusCalls,
		"the order must not be completed when the final refund decision owes the buyer")
	assert.Equal(t, 0, authority.resolveCalls,
		"no seller-wins decision may be recorded on top of a final buyer-wins decision")
	assert.Equal(t, 1, authority.owedCalls, "the refund decision authority must be consulted")
	assert.Equal(t, order.ID, authority.lastOrderID)
}

func TestReleaseFromDispute_FailsClosedWhenRefundDecisionAuthorityMissing(t *testing.T) {
	logger := zaptest.NewLogger(t)
	order := newOrderForDisputeRelease()
	repo := newDisputeOrderRepo(order)

	svc := &OrderCompletionService{
		repo:   repo,
		logger: logger,
		// refundDecisionAuth intentionally nil
	}

	err := svc.ReleaseFromDispute(context.Background(), nil, order.ID, uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "refund decision authority not configured")
	assert.Equal(t, 0, repo.updateStatusCalls,
		"escrow must never be released without checking the refund decision")
}

// TestReleaseFromDispute_ContinuesWhenNoFinalRefundOwesBuyer proves the guard
// does not over-block: with no final buyer-owed refund decision the call moves
// past the guard. Execution then reaches the concrete payment/escrow services
// (which the unit harness cannot construct), so any panic past the guard is
// treated as "the guard let it through" — the assertion of interest is that the
// guard was consulted exactly once for this order and did not reject it.
func TestReleaseFromDispute_ContinuesWhenNoFinalRefundOwesBuyer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	order := newOrderForDisputeRelease()
	repo := newDisputeOrderRepo(order)
	authority := &stubRefundDecisionAuthority{owedToBuyer: false, resolveRecord: true}

	svc := &OrderCompletionService{
		repo:               repo,
		logger:             logger,
		refundDecisionAuth: authority,
	}

	var finalityError error
	func() {
		defer func() { _ = recover() }() // panic == execution continued past the guard
		if err := svc.ReleaseFromDispute(context.Background(), nil, order.ID, uuid.New()); err != nil &&
			errors.Is(err, ErrFinalRefundDecisionOwesBuyer) {
			finalityError = err
		}
	}()

	assert.NoError(t, finalityError,
		"the finality guard must not block a release when no final refund decision owes the buyer")
	assert.Equal(t, 1, authority.owedCalls, "the refund decision authority must be consulted exactly once")
	assert.Equal(t, order.ID, authority.lastOrderID)
}
