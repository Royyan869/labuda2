package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/internal/identity/auth"
	idempotencyRepo "github.com/labuda/backend/internal/platform/idempotency/repository"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// REGRESSION LOCK: a PAID order must never cancel through the normal Cancel path
// ============================================================================
//
// ORDER-A1 finding: POST /orders/:id/cancel routed paid-but-not-yet-overdue
// orders into OrderService.Cancel -> OrderCompletionService.Cancel ->
// Order.Cancel. The state machine allowed paid -> cancelled, and Cancel performs
// no refund and no escrow flip, so a paid order could become
// `status = cancelled` + `escrow_status = holding` with the buyer's money
// frozen and the selling-surface stock already restored.
//
// CANONICAL: Cancel is the PRE-PAYMENT transition (pending_payment -> cancelled).
// A paid order's terminal exits are shipped, refunded (refund domain) and
// cancelled_timeout (CancelOverdue: fulfillment deadline, gateway refund +
// escrow flip). This test proves the rejection happens at the state-machine
// layer and that nothing is persisted.
// ============================================================================

func TestCancel_RejectsPaidOrderWithoutRefund(t *testing.T) {
	buyerID := uuid.New()
	order := &entity.Order{
		ID:           uuid.New(),
		BuyerID:      buyerID,
		SellerID:     uuid.New(),
		Status:       entity.StatusPaid,
		EscrowStatus: entity.EscrowStatusHolding,
	}

	repo := &stubOrderRepo{
		getForUpdateFn: func(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Order, error) {
			return order, nil
		},
	}

	// Minimal wiring: the paid-order rejection happens before any stock,
	// shipping-quote or outbox dependency is touched.
	svc := &OrderCompletionService{
		repo:            repo,
		ownership:       auth.NewOwnershipValidator(),
		idempotencyRepo: idempotencyRepo.NewRepository(),
	}

	err := svc.Cancel(context.Background(), &cancelRejectTx{}, order.ID, "order-a1-cancel-paid", buyerID)

	// REJECTED, not silently swallowed.
	require.Error(t, err, "cancelling a paid order must be rejected")
	var invalidTransition *entity.InvalidTransitionError
	require.ErrorAs(t, err, &invalidTransition)
	assert.Equal(t, entity.StatusPaid, invalidTransition.CurrentStatus)
	assert.Equal(t, entity.StatusCancelled, invalidTransition.TargetStatus)

	// INVARIANT: no mutation was persisted; the order stayed paid with escrow held.
	assert.Zero(t, repo.updateStatusCalls, "no order status write may be persisted")
	assert.Equal(t, entity.StatusPaid, order.Status)
	assert.Equal(t, entity.EscrowStatusHolding, order.EscrowStatus)
}

// cancelRejectTx is a minimal db.Tx double. Exec returns a non-zero
// RowsAffected so the idempotency TryInsert records a fresh operation instead of
// short-circuiting as a duplicate.
type cancelRejectTx struct{}

func (cancelRejectTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (cancelRejectTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (cancelRejectTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return cancelRejectRow{}
}
func (cancelRejectTx) Commit(_ context.Context) error   { return nil }
func (cancelRejectTx) Rollback(_ context.Context) error { return nil }

type cancelRejectRow struct{}

func (cancelRejectRow) Scan(_ ...any) error { return nil }
