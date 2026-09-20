package application

import (
	"context"
	"testing"
	"time"

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
// TESTS: RefundReleaseGuard in OrderCompletionService.Complete
// ============================================================================
//
// CANONICAL CONTRACT: the order lifecycle asks ONE question before releasing
// money to the seller — "does this order have a refund that still has to be
// respected?" The answer is composed from two independent facts:
//
//  1. the REFUND domain owns whether a refund's decision is final / whether
//     money owed to the buyer is still settling at the gateway
//     (refundEntity.Refund.BlocksOrderRelease);
//  2. the ORDER domain owns whether the buyer's refund window is still open
//     (Order.IsRefundWindowOpen) — because only the order knows shipped /
//     dispute / auto_release_at.
//
// These tests prove the completion service wires those two facts together and
// fails closed when the guard is absent. The SQL mirror of the predicate lives
// in the refund repository (HasRefundBlockingRelease) and in the auto-complete
// candidate query; the status sets are shared constants asserted by
// TestRefundBlockingReleaseSQL_UsesCanonicalStatusSets.
// ============================================================================

// stubRefundReleaseGuard is a test double for RefundReleaseGuard. It records the
// refundWindowOpen value the completion service supplied so the window wiring is
// provable without a database.
type stubRefundReleaseGuard struct {
	blocks                bool
	err                   error
	called                bool
	observedWindowOpen    bool
	observedWindowWasPass bool
}

func (s *stubRefundReleaseGuard) HasRefundBlockingRelease(
	_ context.Context, _ db.Tx, _ uuid.UUID, refundWindowOpen bool,
) (bool, error) {
	s.called = true
	s.observedWindowWasPass = true
	s.observedWindowOpen = refundWindowOpen
	return s.blocks, s.err
}

// newCompletableOrder returns an order in the canonical auto-complete-eligible
// state, with the refund window explicitly CLOSED (deadline already passed) so
// the guard alone decides.
func newCompletableOrder() *entity.Order {
	order := &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusShipped,
		EscrowStatus: entity.EscrowStatusHolding,
		HasDispute:   false,
	}
	closedAt := time.Now().Add(-time.Minute) // refund window already closed
	order.AutoReleaseAt = &closedAt
	return order
}

// newCompletionServiceWithGuard builds a minimal OrderCompletionService wired
// with the given guard and stubbed order repo. Other deps are nil — the tests
// are designed to hit the refund guard BEFORE downstream calls.
func newCompletionServiceWithGuard(
	guard RefundReleaseGuard,
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
		refundReleaseGuard:   guard,
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

func TestComplete_BlockedByRefundBlockingRelease(t *testing.T) {
	order := newCompletableOrder()
	guard := &stubRefundReleaseGuard{blocks: true}
	svc := newCompletionServiceWithGuard(guard, order, t)

	err := svc.Complete(context.Background(), &refundGuardNoopTx{}, auth.SystemCallerID, order.ID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refund still blocking release")
	assert.True(t, guard.called, "the refund release guard must be consulted")
}

func TestComplete_NotBlockedWhenNoRefundBlocks(t *testing.T) {
	order := newCompletableOrder()
	guard := &stubRefundReleaseGuard{blocks: false}
	svc := newCompletionServiceWithGuard(guard, order, t)

	// Will panic downstream (nil supportRepo etc.) — recover and verify the
	// refund guard was NOT the cause.
	err := safeComplete(svc, order.ID)
	if err != nil {
		assert.NotContains(t, err.Error(), "refund still blocking release")
	}
}

func TestComplete_FailsClosedWhenGuardNil(t *testing.T) {
	// Escrow must not be released without a functioning refund guard.
	order := newCompletableOrder()
	svc := newCompletionServiceWithGuard(nil, order, t)

	err := safeComplete(svc, order.ID)
	if err == nil {
		t.Fatal("expected ErrRefundReleaseGuardNotConfigured, got nil")
	}
	assert.Contains(t, err.Error(), "refund release guard not configured")
}

// TestComplete_PassesOrderRefundWindowToGuard proves the raw predicate is not
// asked in isolation: the ORDER domain's refund-window answer is threaded into
// the guard, so a closed window (order lifecycle owns the outcome) is
// distinguishable from an open one.
func TestComplete_PassesOrderRefundWindowToGuard(t *testing.T) {
	// Window CLOSED: deadline already passed.
	closed := newCompletableOrder()
	closedGuard := &stubRefundReleaseGuard{blocks: false}
	_ = safeComplete(newCompletionServiceWithGuard(closedGuard, closed, t), closed.ID)
	if !closedGuard.observedWindowWasPass || closedGuard.observedWindowOpen {
		t.Fatalf("closed window must be passed as false, got %v (passed=%v)",
			closedGuard.observedWindowOpen, closedGuard.observedWindowWasPass)
	}

	// Window OPEN: still shipped and before the auto-release deadline.
	open := &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusShipped,
		EscrowStatus: entity.EscrowStatusHolding,
	}
	autoRelease := time.Now().Add(time.Hour)
	open.AutoReleaseAt = &autoRelease

	openGuard := &stubRefundReleaseGuard{blocks: false}
	_ = safeComplete(newCompletionServiceWithGuard(openGuard, open, t), open.ID)
	if !openGuard.observedWindowWasPass || !openGuard.observedWindowOpen {
		t.Fatalf("open window must be passed as true, got %v (passed=%v)",
			openGuard.observedWindowOpen, openGuard.observedWindowWasPass)
	}
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
