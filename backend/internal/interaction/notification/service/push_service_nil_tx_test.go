package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/interaction/notification/service"
	"go.uber.org/zap/zaptest"
)

// emptyPool is a test pool that returns an empty result set for Query
// and success for Exec — satisfies service.PoolQuerier.
type emptyPool struct {
	queryCalls int
}

func (p *emptyPool) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	p.queryCalls++
	return &emptyRows{}, nil
}

func (p *emptyPool) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 0"), nil
}

type emptyRows struct{}

func (r *emptyRows) Next() bool                                     { return false }
func (r *emptyRows) Err() error                                     { return nil }
func (r *emptyRows) Close()                                         {}
func (r *emptyRows) Scan(dest ...any) error                         { return pgx.ErrNoRows }
func (r *emptyRows) CommandTag() pgconn.CommandTag                  { return pgconn.NewCommandTag("SELECT 0") }
func (r *emptyRows) Fields() []pgconn.FieldDescription              { return nil }
func (r *emptyRows) FieldDescriptions() []pgconn.FieldDescription   { return nil }
func (r *emptyRows) RawValues() [][]byte                            { return nil }
func (r *emptyRows) Values() ([]any, error)                         { return nil, nil }
func (r *emptyRows) Conn() *pgx.Conn                                { return nil }

// =============================================================================
// SendNotification nil-tx panic regression tests
// =============================================================================

// TestSendNotification_NilTx_NilPool verifies that passing tx=nil with pool=nil
// does NOT panic and returns nil (best-effort push, warn-logged internally).
func TestSendNotification_NilTx_NilPool(t *testing.T) {
	log := zaptest.NewLogger(t)
	// pool=nil: FCM token lookup returns controlled error, logged as Warn, no panic.
	svc := service.NewPushService(nil, nil, nil, log)

	notif := map[string]interface{}{
		"id":           uuid.New().String(),
		"recipient_id": uuid.New().String(),
		"type":         "order.partially_refunded",
	}

	// Must not panic.
	err := svc.SendNotification(context.Background(), nil, notif, "Test Title", "Test Body")
	if err != nil {
		t.Errorf("expected nil error (best-effort push), got %v", err)
	}
}

// TestSendNotification_NilTx_WithPool verifies that pool fallback is used
// when tx=nil, token lookup returns empty slice, and the call is a clean no-op.
func TestSendNotification_NilTx_WithPool(t *testing.T) {
	log := zaptest.NewLogger(t)
	pool := &emptyPool{}
	// firebaseClient=nil → dev/mock mode: even if tokens were returned,
	// no real push would fire.
	svc := service.NewPushService(nil, pool, nil, log)

	notif := map[string]interface{}{
		"id":           uuid.New().String(),
		"recipient_id": uuid.New().String(),
		"type":         "dispute.resolved",
	}

	err := svc.SendNotification(context.Background(), nil, notif, "Dispute Resolved", "Your dispute has been resolved.")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if pool.queryCalls == 0 {
		t.Error("expected pool to be queried for FCM tokens with nil tx")
	}
}

// TestSendNotification_NilTx_NoMessagingClient verifies that when Firebase
// messaging client is nil (dev/mock mode) the call returns nil (no-op).
// The pool is provided so token lookup succeeds but push is suppressed.
func TestSendNotification_NilTx_NoMessagingClient(t *testing.T) {
	log := zaptest.NewLogger(t)
	pool := &emptyPool{}
	svc := service.NewPushService(nil, pool, nil, log)

	notif := map[string]interface{}{
		"id":           uuid.New().String(),
		"recipient_id": uuid.New().String(),
		"type":         "seller.subscription.expiring",
	}

	err := svc.SendNotification(context.Background(), nil, notif, "Subscription Expiring", "Your subscription expires soon.")
	if err != nil {
		t.Errorf("expected nil (no-op in mock mode), got %v", err)
	}
}

// TestSendNotification_InvalidRecipient verifies that a nil/zero UUID recipient
// returns nil without panicking (early guard path).
func TestSendNotification_InvalidRecipient(t *testing.T) {
	log := zaptest.NewLogger(t)
	svc := service.NewPushService(nil, nil, nil, log)

	notif := map[string]interface{}{
		"id":           uuid.New().String(),
		"recipient_id": uuid.Nil.String(), // zero UUID → guarded
		"type":         "order.partially_refunded",
	}

	err := svc.SendNotification(context.Background(), nil, notif, "T", "B")
	if err != nil {
		t.Errorf("expected nil for nil recipient, got %v", err)
	}
}

// TestSendNotification_UnknownNotificationType verifies that an unrecognised
// notification type returns nil (logged as Warn).
func TestSendNotification_UnknownNotificationType(t *testing.T) {
	log := zaptest.NewLogger(t)
	svc := service.NewPushService(nil, nil, nil, log)

	// Pass a type that is not *entity.Notification or map[string]interface{}
	err := svc.SendNotification(context.Background(), nil, "not-a-valid-type", "T", "B")
	if err != nil {
		t.Errorf("expected nil for unknown notification type, got %v", err)
	}
}
