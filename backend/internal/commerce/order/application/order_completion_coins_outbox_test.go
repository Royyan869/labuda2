package application

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/internal/commerce/order/entity"
	walletApp "github.com/labuda/backend/internal/core/wallet/application"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// TESTS: coins.refund_required STRICT_EVENT_ATOMIC in Expire
// ============================================================================
//
// Batch 18 regression guard: coins.refund_required outbox insert failure must
// propagate and roll back the transaction. If the outbox write is silently
// swallowed, the order expires but coins are never refunded to the buyer.
//
// CancelOverdue uses the identical pattern but has deeper upstream deps
// (idempotency + gateway refund + wallet escrow). The Expire path is testable
// with minimal mocks because the nil-escrow shortcut skips payment operations.
// ============================================================================

// coinsOutboxFailTx is a mock tx that succeeds for all SQL except
// INSERT INTO outbox containing "coins.refund_required", which returns a
// simulated failure. Other outbox inserts (order.expired etc.) succeed.
type coinsOutboxFailTx struct {
	coinsOutboxCalled bool
	outboxCallCount   int
}

func (t *coinsOutboxFailTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	// Escrow repo GetByOrderID → return ErrNoRows (no escrow for this order)
	return &noRowsMockRow{}
}

func (t *coinsOutboxFailTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (t *coinsOutboxFailTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO outbox") {
		t.outboxCallCount++
		// Check if the event_type arg ($4) is coins.refund_required.
		// InsertEvent passes: id, aggregate_type, aggregate_id, event_type, ...
		for _, arg := range args {
			if s, ok := arg.(string); ok && s == "coins.refund_required" {
				t.coinsOutboxCalled = true
				return pgconn.CommandTag{}, fmt.Errorf("simulated outbox write failure")
			}
		}
	}
	return pgconn.NewCommandTag("1"), nil
}

func (t *coinsOutboxFailTx) Commit(_ context.Context) error   { return nil }
func (t *coinsOutboxFailTx) Rollback(_ context.Context) error { return nil }

// noRowsMockRow returns pgx.ErrNoRows on Scan — simulates "no escrow found".
type noRowsMockRow struct{}

func (r *noRowsMockRow) Scan(_ ...any) error {
	return pgx.ErrNoRows
}

// expireStubOrderRepo extends stubOrderRepo with GetOrderItems for
// restoreForSaleStock (returns empty slice → no stock to restore).
type expireStubOrderRepo struct {
	stubOrderRepo
}

func (r *expireStubOrderRepo) GetOrderItems(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*entity.OrderItem, error) {
	return nil, nil // no items → skip stock restoration loop
}

// newExpirableOrder returns a Pending order with coins used, suitable for
// the Expire path.
func newExpirableOrder() *entity.Order {
	return &entity.Order{
		ID:        uuid.New(),
		BuyerID:   uuid.New(),
		SellerID:  uuid.New(),
		Status:    entity.StatusPending,
		CoinsUsed: 100, // triggers coins.refund_required emission
	}
}

func safeCallPostRelease(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn()
}

func TestExpire_OutboxFailure_RollsBackTransaction(t *testing.T) {
	spy := &coinsOutboxFailTx{}
	order := newExpirableOrder()

	svc := &OrderCompletionService{
		repo: &expireStubOrderRepo{
			stubOrderRepo: stubOrderRepo{
				getForUpdateFn: func(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Order, error) {
					return order, nil
				},
			},
		},
		walletService: walletApp.NewWalletService(nil, zaptest.NewLogger(t)),
		outboxRepo:    outboxRepo.NewOutboxRepository(nil),
		logger:        zaptest.NewLogger(t),
	}

	err := svc.Expire(context.Background(), spy, order.ID)

	require.Error(t, err, "Expire must return error when outbox insert fails (STRICT_EVENT_ATOMIC)")
	assert.True(t, spy.coinsOutboxCalled, "outbox insert was never attempted — test wiring error")
	assert.Contains(t, err.Error(), "outbox coins.refund_required",
		"error should reference the outbox event type")
}

func TestExpire_NoCoins_SkipsOutbox(t *testing.T) {
	spy := &coinsOutboxFailTx{}
	order := newExpirableOrder()
	order.CoinsUsed = 0 // no coins → outbox call skipped

	svc := &OrderCompletionService{
		repo: &expireStubOrderRepo{
			stubOrderRepo: stubOrderRepo{
				getForUpdateFn: func(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Order, error) {
					return order, nil
				},
				updateStatusFn: func(_ context.Context, _ db.Tx, _ *entity.Order) error {
					return nil
				},
			},
		},
		walletService: walletApp.NewWalletService(nil, zaptest.NewLogger(t)),
		outboxRepo:    outboxRepo.NewOutboxRepository(nil),
		logger:        zaptest.NewLogger(t),
	}

	// Will hit downstream nil deps (shippingQuoteService etc.) — catch panic
	err := safeCallPostRelease(func() error {
		return svc.Expire(context.Background(), spy, order.ID)
	})

	// The outbox should NOT have been called (no coins to refund)
	assert.False(t, spy.coinsOutboxCalled, "outbox insert should not be called when CoinsUsed == 0")
	// If there's an error, it should NOT be about the outbox
	if err != nil {
		assert.NotContains(t, err.Error(), "outbox coins.refund_required")
	}
}
