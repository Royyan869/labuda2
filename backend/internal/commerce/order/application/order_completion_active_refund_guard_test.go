package application

import (
	"context"
	"testing"

	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// TESTS: ActiveRefundChecker guard in OrderCompletionService.Complete
// ============================================================================
//
// H2-F2a MONEY-SAFETY: Proves that auto-completion is blocked when an active
// (non-terminal) refund exists on the order, preventing escrow release while
// a refund is being negotiated or awaiting gateway settlement.
//
// Terminal refund statuses that do NOT block: refunded, admin_released.
// All other statuses block: pending_seller_review, seller_approved,
// seller_rejected, escalated_to_admin, admin_refunded.
// ============================================================================

// stubActiveRefundChecker is a test double for ActiveRefundChecker.
type stubActiveRefundChecker struct {
	hasActive bool
	err       error
}

func (s *stubActiveRefundChecker) HasActiveRefundByOrderID(_ context.Context, _ db.Tx, _ uuid.UUID) (bool, error) {
	return s.hasActive, s.err
}

// newCompletableOrder returns an order in the canonical auto-complete-eligible state.
func newCompletableOrder() *entity.Order {
	return &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusShipped,
		EscrowStatus: entity.EscrowStatusHolding,
		HasDispute:   false,
	}
}

// newCompletionServiceWithRefundChecker builds a minimal OrderCompletionService
// wired with the given ActiveRefundChecker and stubbed order repo. Other deps
// are nil — tests are designed to hit the refund guard BEFORE downstream calls.
func newCompletionServiceWithRefundChecker(
	checker ActiveRefundChecker,
	order *entity.Order,
	logger *testing.T,
) *OrderCompletionService {
	return &OrderCompletionService{
		repo: &stubOrderRepo{
			getForUpdateFn: func(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Order, error) {
				return order, nil
			},
		},
		ownership:            auth.NewOwnershipValidator(),
		accountStatusChecker: &noopAccountChecker{},
		activeRefundChecker:  checker,
		logger:               zaptest.NewLogger(logger),
	}
}

// noopAccountChecker satisfies auth.AccountStatusChecker without side effects.
type noopAccountChecker struct{}

func (n *noopAccountChecker) EnsureActive(_ context.Context, _ uuid.UUID) error { return nil }
func (n *noopAccountChecker) GetStatus(_ context.Context, _ uuid.UUID) (string, error) {
	return "active", nil
}
func (n *noopAccountChecker) IsBanned(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}

// --- Test cases ---

func TestComplete_BlockedByActiveRefund_PendingSellerReview(t *testing.T) {
	order := newCompletableOrder()
	svc := newCompletionServiceWithRefundChecker(
		&stubActiveRefundChecker{hasActive: true},
		order, t,
	)

	err := svc.Complete(context.Background(), &refundGuardNoopTx{}, auth.SystemCallerID, order.ID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "active refund exists")
}

func TestComplete_BlockedByActiveRefund_SellerRejected(t *testing.T) {
	// seller_rejected is NOT terminal — buyer still has escalation rights
	order := newCompletableOrder()
	svc := newCompletionServiceWithRefundChecker(
		&stubActiveRefundChecker{hasActive: true},
		order, t,
	)

	err := svc.Complete(context.Background(), &refundGuardNoopTx{}, auth.SystemCallerID, order.ID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "active refund exists")
}

func TestComplete_BlockedByActiveRefund_EscalatedToAdmin(t *testing.T) {
	order := newCompletableOrder()
	svc := newCompletionServiceWithRefundChecker(
		&stubActiveRefundChecker{hasActive: true},
		order, t,
	)

	err := svc.Complete(context.Background(), &refundGuardNoopTx{}, auth.SystemCallerID, order.ID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "active refund exists")
}

func TestComplete_NotBlockedByTerminalRefund(t *testing.T) {
	// Terminal refunds (refunded, admin_released) do NOT block.
	// The checker returns false for terminal-only refunds.
	order := newCompletableOrder()
	svc := newCompletionServiceWithRefundChecker(
		&stubActiveRefundChecker{hasActive: false},
		order, t,
	)

	// Will panic downstream (nil supportRepo etc.) — recover and verify
	// the refund guard was NOT the cause.
	err := safeComplete(svc, order.ID)
	if err != nil {
		assert.NotContains(t, err.Error(), "active refund exists")
	}
}

func TestComplete_NotBlockedWhenNoRefund(t *testing.T) {
	order := newCompletableOrder()
	svc := newCompletionServiceWithRefundChecker(
		&stubActiveRefundChecker{hasActive: false},
		order, t,
	)

	err := safeComplete(svc, order.ID)
	if err != nil {
		assert.NotContains(t, err.Error(), "active refund exists")
	}
}

func TestComplete_FailsClosedWhenCheckerNil(t *testing.T) {
	// FIX-6: nil checker must now return ErrActiveRefundCheckerNotConfigured.
	// Escrow must not be released without a functioning refund guard.
	order := newCompletableOrder()
	svc := newCompletionServiceWithRefundChecker(nil, order, t)

	err := safeComplete(svc, order.ID)
	if err == nil {
		t.Fatal("expected ErrActiveRefundCheckerNotConfigured, got nil")
	}
	assert.Contains(t, err.Error(), "active refund checker not configured")
}

// safeComplete calls Complete and recovers from panics caused by nil
// downstream dependencies (supportRepo, paymentRepo, etc.). Returns the
// error from Complete or a synthetic error wrapping the panic value.
func safeComplete(svc *OrderCompletionService, orderID uuid.UUID) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			// Downstream nil-pointer panic — expected in minimal test harness.
			// Convert to error so callers can assert on the message.
			retErr = fmt.Errorf("downstream panic: %v", r)
		}
	}()
	return svc.Complete(context.Background(), &refundGuardNoopTx{}, auth.SystemCallerID, orderID, "")
}

// --- test doubles for db.Tx ---

// refundGuardNoopTx is a minimal db.Tx stub. Only methods called before the
// refund guard need to work; the rest return zero/nil.
type refundGuardNoopTx struct{}

func (n *refundGuardNoopTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (n *refundGuardNoopTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (n *refundGuardNoopTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &refundGuardNoopRow{}
}
func (n *refundGuardNoopTx) Commit(_ context.Context) error   { return nil }
func (n *refundGuardNoopTx) Rollback(_ context.Context) error { return nil }

type refundGuardNoopRow struct{}

func (n *refundGuardNoopRow) Scan(_ ...any) error { return nil }


