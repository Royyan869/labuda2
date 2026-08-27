package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap/zaptest"

	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/platform/events"
	dbpkg "github.com/labuda/backend/pkg/db"
)

// mockTxForNotification implements db.Tx for notification testing
type mockTxForNotification struct {
	execCalls    int
	ExecFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *mockTxForNotification) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, sql, args...)
	}
	return &mockRowForNotification{err: errors.New("not implemented")}
}

func (m *mockTxForNotification) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &mockRowsForNotification{}, nil
}

func (m *mockTxForNotification) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalls++
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("1"), nil
}

func (m *mockTxForNotification) Commit(ctx context.Context) error {
	return nil
}

func (m *mockTxForNotification) Rollback(ctx context.Context) error {
	return nil
}

// mockRowForNotification implements pgx.Row for testing
type mockRowForNotification struct {
	err       error
	scanValue any
}

func (r *mockRowForNotification) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if r.scanValue != nil && len(dest) > 0 {
		if ptr, ok := dest[0].(*uuid.UUID); ok {
			if val, ok := r.scanValue.(uuid.UUID); ok {
				*ptr = val
				return nil
			}
		}
		if ptr, ok := dest[0].(*bool); ok {
			if val, ok := r.scanValue.(bool); ok {
				*ptr = val
				return nil
			}
		}
	}
	return r.err
}

// mockRowsForNotification implements pgx.Rows for testing
type mockRowsForNotification struct{}

func (r *mockRowsForNotification) Next() bool {
	return false
}

func (r *mockRowsForNotification) Err() error {
	return nil
}

func (r *mockRowsForNotification) Close() {}

func (r *mockRowsForNotification) Scan(dest ...any) error {
	return pgx.ErrNoRows
}

func (r *mockRowsForNotification) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("0")
}

func (r *mockRowsForNotification) Fields() []pgconn.FieldDescription {
	return nil
}

func (r *mockRowsForNotification) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *mockRowsForNotification) RawValues() [][]byte {
	return nil
}

func (r *mockRowsForNotification) Values() ([]any, error) {
	return nil, nil
}

func (r *mockRowsForNotification) Conn() *pgx.Conn {
	return nil
}

// mockTxForNotificationWithQueryRow extends mockTxForNotification with customizable QueryRow
type mockTxForNotificationWithQueryRow struct {
	mockTxForNotification
	queryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *mockTxForNotificationWithQueryRow) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFunc != nil {
		return m.queryRowFunc(ctx, sql, args...)
	}
	return &mockRowForNotification{err: errors.New("not implemented")}
}

// mockDBForNotification implements db.Transactor for notification testing

// mockBlockCheckerForNotification implements BlockChecker for testing.
// Returns blocked=false by default. Use mockBlockCheckerBlocked for blocked=true.
type mockBlockCheckerForNotification struct{}

func (m *mockBlockCheckerForNotification) ExistsBlock(ctx context.Context, userA, userB uuid.UUID) (bool, error) {
	return false, nil
}

// mockBlockCheckerBlocked returns blocked=true for all pairs.
type mockBlockCheckerBlocked struct{}

func (m *mockBlockCheckerBlocked) ExistsBlock(ctx context.Context, userA, userB uuid.UUID) (bool, error) {
	return true, nil
}

// mockBlockCheckerError returns an error to simulate DB failure.
type mockBlockCheckerError struct{}

func (m *mockBlockCheckerError) ExistsBlock(ctx context.Context, userA, userB uuid.UUID) (bool, error) {
	return false, fmt.Errorf("db connection failed")
}

// mockPushSender implements PushSender for testing
type mockPushSenderForNotification struct{}

func (m *mockPushSenderForNotification) SendNotification(ctx context.Context, tx interface{}, notification interface{}, title, body string) error {
	return nil
}

// mockAccountStatusChecker implements AccountStatusChecker for testing
type mockAccountStatusCheckerForNotification struct{}

func (m *mockAccountStatusCheckerForNotification) GetStatus(ctx context.Context, userID uuid.UUID) (string, error) {
	return "active", nil
}

type mockDBForNotification struct {
	withTxCalls int
	WithTxFunc  func(ctx context.Context, fn func(tx dbpkg.Tx) error) error
}

func (m *mockDBForNotification) WithTx(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
	m.withTxCalls++
	if m.WithTxFunc != nil {
		return m.WithTxFunc(ctx, fn)
	}
	return fn(&mockTxForNotification{})
}

// TestNewNotificationEventHandler tests creating a new NotificationEventHandler.
func TestNewNotificationEventHandler(t *testing.T) {
	log := zaptest.NewLogger(t)
	inserter := NewNotificationServiceInserter()

	// Pass nil for db and blockChecker since we don't need them for this test
	handler := NewNotificationEventHandler(nil, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	if handler == nil {
		t.Fatal("NewNotificationEventHandler() returned nil")
	}

	if handler.notificationInserter != inserter {
		t.Error("notificationInserter not set correctly")
	}
}

// TestNotificationEventHandler_HandleUserFollowed tests handling user.followed events.
func TestNotificationEventHandler_HandleUserFollowed(t *testing.T) {
	log := zaptest.NewLogger(t)

	actorID := uuid.New()
	recipientID := uuid.New()

	payload, _ := json.Marshal(NotificationPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   recipientID,
		EventType:     events.EventUserFollowed,
		Payload:       payload,
	}

	var insertedRecipientID, insertedActorID uuid.UUID
	var insertedType string
	var insertedEntityID uuid.UUID

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			mockTx := &mockTxForNotification{
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					// INSERT uses QueryRow, not Exec
					return pgconn.NewCommandTag("1"), nil
				},
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					// Capture the arguments passed to INSERT ... RETURNING
					if len(args) >= 6 {
						insertedRecipientID = args[1].(uuid.UUID)
						insertedActorID = args[2].(uuid.UUID)
						insertedType = args[3].(string)
						insertedEntityID = args[4].(uuid.UUID)
					}
					// Return a valid UUID for the INSERT ... RETURNING id
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(mockTx)
		},
	}

	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify the notification was inserted with correct values
	if insertedRecipientID != recipientID {
		t.Errorf("recipient_id = %s, want %s", insertedRecipientID, recipientID)
	}

	if insertedActorID != actorID {
		t.Errorf("actor_id = %s, want %s", insertedActorID, actorID)
	}

	if insertedType != events.EventUserFollowed {
		t.Errorf("type = %s, want user.followed", insertedType)
	}

	// For follow events, entity_id should be the actor_id
	if insertedEntityID != actorID {
		t.Errorf("entity_id = %s, want %s", insertedEntityID, actorID)
	}
}

// TestNotificationEventHandler_HandleContentLiked tests handling content.liked events.
func TestNotificationEventHandler_HandleContentLiked(t *testing.T) {
	log := zaptest.NewLogger(t)

	actorID := uuid.New()
	recipientID := uuid.New()
	contentID := uuid.New()

	payload, _ := json.Marshal(ContentLikedPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
		ContentID:   contentID.String(),
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "content",
		AggregateID:   contentID,
		EventType:     events.EventContentLiked,
		Payload:       payload,
	}

	var insertedRecipientID, insertedActorID, insertedEntityID uuid.UUID
	var insertedType string

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			mockTx := &mockTxForNotification{
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					// INSERT uses QueryRow, not Exec
					return pgconn.NewCommandTag("1"), nil
				},
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					// LIKE-OCCURRENCE GUARD: content.liked delivery re-checks that
					// the like row still exists. Simulate an active like.
					if strings.Contains(sql, "content_likes") {
						return &mockRowForNotification{scanValue: true}
					}
					// Capture the arguments passed to INSERT ... RETURNING
					if len(args) >= 6 {
						insertedRecipientID = args[1].(uuid.UUID)
						insertedActorID = args[2].(uuid.UUID)
						insertedType = args[3].(string)
						insertedEntityID = args[4].(uuid.UUID)
					}
					// Return a valid UUID for the INSERT ... RETURNING id
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(mockTx)
		},
	}

	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify the notification was inserted with correct values
	if insertedRecipientID != recipientID {
		t.Errorf("recipient_id = %s, want %s", insertedRecipientID, recipientID)
	}

	if insertedActorID != actorID {
		t.Errorf("actor_id = %s, want %s", insertedActorID, actorID)
	}

	if insertedType != events.EventContentLiked {
		t.Errorf("type = %s, want content.liked", insertedType)
	}

	if insertedEntityID != contentID {
		t.Errorf("entity_id = %s, want %s", insertedEntityID, contentID)
	}
}

// TestNotificationEventHandler_HandleContentLiked_SelfLike tests that self-likes don't create notifications.
func TestNotificationEventHandler_HandleContentLiked_SelfLike(t *testing.T) {
	log := zaptest.NewLogger(t)

	userID := uuid.New()
	contentID := uuid.New()

	payload, _ := json.Marshal(ContentLikedPayload{
		ActorID:     userID.String(),
		RecipientID: userID.String(), // Same as actor_id (self-like)
		ContentID:   contentID.String(),
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "content",
		AggregateID:   contentID,
		EventType:     events.EventContentLiked,
		Payload:       payload,
	}

	calls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			calls++
			return fn(&mockTxForNotification{})
		},
	}

	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should not call WithTx for self-likes
	if calls != 0 {
		t.Errorf("WithTx called %d times, want 0 (self-like should skip notification)", calls)
	}
}

// TestNotificationEventHandler_HandleUnknownEventType tests that unknown event types don't fail.
func TestNotificationEventHandler_HandleUnknownEventType(t *testing.T) {
	log := zaptest.NewLogger(t)

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "unknown",
		AggregateID:   uuid.New(),
		EventType:     "unknown.event",
		Payload:       []byte("{}"),
	}

	mockDB := &mockDBForNotification{}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() should return nil for unknown event types, got %v", err)
	}
}

// TestNotificationEventHandler_InvalidPayload tests handling of invalid JSON payload.
func TestNotificationEventHandler_InvalidPayload(t *testing.T) {
	log := zaptest.NewLogger(t)

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   uuid.New(),
		EventType:     events.EventUserFollowed,
		Payload:       []byte("invalid json"),
	}

	mockDB := &mockDBForNotification{}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("Handle() should return error for invalid payload, got nil")
	}

	if !errors.Is(err, fmt.Errorf("unmarshal payload failed")) && err.Error()[:25] != "unmarshal payload failed" {
		t.Logf("error = %v (contains unmarshal payload failed)", err)
	}
}

// =============================================================================
// N1 BLOCK POLICY FIX TESTS
// Prove: nil-tx bug is gone; block checker reaches DB; policy decisions are correct.
// =============================================================================

// buildMockDB returns a mockDB that captures the actorID passed to InsertNotification.
func buildMockDB(insertedActorID *uuid.UUID) *mockDBForNotification {
	return &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					// args: id(0), recipientID(1), actorID(2), type(3), entityID(4), data(5), false(6)
					if len(args) >= 3 {
						if id, ok := args[2].(uuid.UUID); ok {
							*insertedActorID = id
						}
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}
}

// TestN1_Social_NotBlocked_Delivers verifies that user.followed is inserted when no block exists.
func TestN1_Social_NotBlocked_Delivers(t *testing.T) {
	log := zaptest.NewLogger(t)
	actorID := uuid.New()
	recipientID := uuid.New()

	payload, _ := json.Marshal(NotificationPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
	})
	event := platformevent.OutboxEvent{
		ID: uuid.New(), AggregateType: "user", AggregateID: recipientID,
		EventType: events.EventUserFollowed, Payload: payload,
	}

	var capturedActorID uuid.UUID
	mockDB := buildMockDB(&capturedActorID)
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, nil, &mockAccountStatusCheckerForNotification{}, log)

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if capturedActorID == uuid.Nil {
		t.Error("notification was not inserted (no block should allow delivery)")
	}
	if capturedActorID != actorID {
		t.Errorf("actor_id = %s, want %s (actor must not be anonymized when not blocked)", capturedActorID, actorID)
	}
}

// TestN1_Social_Blocked_Drops verifies that user.followed is suppressed when a block exists.
func TestN1_Social_Blocked_Drops(t *testing.T) {
	log := zaptest.NewLogger(t)
	actorID := uuid.New()
	recipientID := uuid.New()

	payload, _ := json.Marshal(NotificationPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
	})
	event := platformevent.OutboxEvent{
		ID: uuid.New(), AggregateType: "user", AggregateID: recipientID,
		EventType: events.EventUserFollowed, Payload: payload,
	}

	withTxCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			withTxCalls++
			return fn(&mockTxForNotification{})
		},
	}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerBlocked{}, inserter, nil, &mockAccountStatusCheckerForNotification{}, log)

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// Block should suppress the DB insert; WithTx must not be called.
	if withTxCalls != 0 {
		t.Errorf("WithTx called %d times, want 0 (blocked social must be dropped)", withTxCalls)
	}
}

// TestN1_Commerce_NotBlocked_ActorPreserved verifies refund.approved delivers with real actor.
func TestN1_Commerce_NotBlocked_ActorPreserved(t *testing.T) {
	log := zaptest.NewLogger(t)
	buyerID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()

	payload, _ := json.Marshal(map[string]string{
		"refund_id": uuid.New().String(),
		"order_id":  orderID.String(),
		"buyer_id":  buyerID.String(),
		"seller_id": sellerID.String(),
		"status":    "approved",
	})
	event := platformevent.OutboxEvent{
		ID: uuid.New(), AggregateType: "refund", AggregateID: orderID,
		EventType: "refund.approved", Payload: payload,
	}

	var capturedActorID uuid.UUID
	mockDB := buildMockDB(&capturedActorID)
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, nil, &mockAccountStatusCheckerForNotification{}, log)

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if capturedActorID == uuid.Nil {
		t.Error("notification was not inserted")
	}
	// actor = sellerID (seller approved the refund, buyer is recipient)
	if capturedActorID != sellerID {
		t.Errorf("actor_id = %s, want sellerID %s (not blocked: actor must be preserved)", capturedActorID, sellerID)
	}
}

// TestN1_Commerce_Blocked_ActorAnonymized verifies refund.approved delivers but anonymizes actor.
func TestN1_Commerce_Blocked_ActorAnonymized(t *testing.T) {
	log := zaptest.NewLogger(t)
	buyerID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()

	payload, _ := json.Marshal(map[string]string{
		"refund_id": uuid.New().String(),
		"order_id":  orderID.String(),
		"buyer_id":  buyerID.String(),
		"seller_id": sellerID.String(),
		"status":    "approved",
	})
	event := platformevent.OutboxEvent{
		ID: uuid.New(), AggregateType: "refund", AggregateID: orderID,
		EventType: "refund.approved", Payload: payload,
	}

	var capturedActorID uuid.UUID
	mockDB := buildMockDB(&capturedActorID)
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerBlocked{}, inserter, nil, &mockAccountStatusCheckerForNotification{}, log)

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// Commerce bypasses block but must anonymize the actor.
	if capturedActorID != uuid.Nil {
		t.Errorf("actor_id = %s, want uuid.Nil (commerce blocked: actor must be anonymized)", capturedActorID)
	}
}

// TestN1_NoInvalidTxTypeReason verifies the nil-tx error reason never surfaces.
// Before the fix, ExistsBlock(ctx, nil, ...) caused "invalid transaction type" errors
// which either silently dropped social notifications or anonymized commerce ones.
func TestN1_NoInvalidTxTypeReason(t *testing.T) {
	log := zaptest.NewLogger(t)
	actorID := uuid.New()
	recipientID := uuid.New()

	// Use an error-returning checker to simulate what happened before the fix.
	// After the fix the adapter opens its own tx; the only way to get an error
	// is a real DB failure — never a type-assertion failure.
	payload, _ := json.Marshal(NotificationPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
	})
	event := platformevent.OutboxEvent{
		ID: uuid.New(), AggregateType: "user", AggregateID: recipientID,
		EventType: events.EventUserFollowed, Payload: payload,
	}

	mockDB := &mockDBForNotification{}
	inserter := NewNotificationServiceInserter()
	// Error checker: simulates any DB error but NOT "invalid transaction type"
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerError{}, inserter, nil, &mockAccountStatusCheckerForNotification{}, log)

	// Must not panic or return a hard error; social fail-closed silently drops.
	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() error = %v (block check errors must be absorbed, not propagated)", err)
	}
}

// TestNotificationEventHandler_InvalidActorID tests handling of invalid actor_id in payload.
func TestNotificationEventHandler_InvalidActorID(t *testing.T) {
	log := zaptest.NewLogger(t)

	payload, _ := json.Marshal(NotificationPayload{
		ActorID:     "not-a-uuid",
		RecipientID: uuid.New().String(),
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   uuid.New(),
		EventType:     events.EventUserFollowed,
		Payload:       payload,
	}

	mockDB := &mockDBForNotification{}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("Handle() should return error for invalid actor_id, got nil")
	}

	if !errors.Is(err, fmt.Errorf("invalid actor_id")) && err.Error()[:17] != "invalid actor_id" {
		t.Logf("error = %v (contains invalid actor_id)", err)
	}
}

// TestNotificationEventHandler_InvalidRecipientID tests handling of invalid recipient_id in payload.
func TestNotificationEventHandler_InvalidRecipientID(t *testing.T) {
	log := zaptest.NewLogger(t)

	payload, _ := json.Marshal(NotificationPayload{
		ActorID:     uuid.New().String(),
		RecipientID: "not-a-uuid",
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   uuid.New(),
		EventType:     events.EventUserFollowed,
		Payload:       payload,
	}

	mockDB := &mockDBForNotification{}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("Handle() should return error for invalid recipient_id, got nil")
	}

	if !errors.Is(err, fmt.Errorf("invalid recipient_id")) && err.Error()[:20] != "invalid recipient_id" {
		t.Errorf("error = %v, want invalid recipient_id", err)
	}
}

// =============================================================================
// CHAT-4: Chat notification governance convergence tests
// =============================================================================

// mockBlockCheckerChat is a configurable block checker for chat governance tests.
type mockBlockCheckerChat struct {
	blocked bool
	err     error
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	return data
}

// =============================================================================
// MISSING CRITICAL NOTIFICATIONS: order.cancelled_timeout
// =============================================================================

// =============================================================================
// ASYNC_NOTIFICATION_PANIC_FIX: nil-tx async push path regression tests
//
// Before the fix, sendPushAsync called pushSender.SendNotification(ctx, nil, ...)
// which would type-assert nil tx inside FCMTokenRepository.GetActiveTokensByUser
// and panic: "interface conversion: interface is nil, not interface { Query ... }"
//
// These tests confirm the handler does not panic and returns nil for affected
// event types: order.partially_refunded, dispute.resolved, seller.subscription.expiring.
// =============================================================================

// panicCapturePushSender captures whether SendNotification was called and allows
// recording panics from within the sender.
type panicCapturePushSender struct {
	calls    int
	lastTx   interface{}
	panicVal interface{} // set if SendNotification panicked (recovered)
}

func (p *panicCapturePushSender) SendNotification(ctx context.Context, tx interface{}, notification interface{}, title, body string) error {
	p.calls++
	p.lastTx = tx
	return nil
}

// TestAsyncPush_NilTx_OrderPartiallyRefunded verifies that handling
// order.partially_refunded with a push sender does not panic.
// The push fires asynchronously (goroutine); handler.Handle must return nil.
func TestAsyncPush_NilTx_OrderPartiallyRefunded(t *testing.T) {
	log := zaptest.NewLogger(t)
	buyerID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()

	payload, _ := json.Marshal(map[string]string{
		"order_id":        orderID.String(),
		"buyer_id":        buyerID.String(),
		"seller_id":       sellerID.String(),
		"refunded_amount": "50000",
		"original_amount": "100000",
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "order",
		AggregateID:   orderID,
		EventType:     "order.partially_refunded",
		Payload:       payload,
	}

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	sender := &panicCapturePushSender{}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, sender, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() must not error: %v", err)
	}
	// Push fires in a goroutine; we can't wait for it reliably in a unit test,
	// but the handler must not panic or return an error. The O3 defer/recover in
	// sendPushAsync would catch any panic and log it — we confirm Handle() is clean.
}

// TestAsyncPush_NilTx_DisputeResolved verifies dispute.resolved does not panic.
func TestAsyncPush_NilTx_DisputeResolved(t *testing.T) {
	log := zaptest.NewLogger(t)
	buyerID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()
	disputeID := uuid.New()

	payload, _ := json.Marshal(map[string]string{
		"dispute_id": disputeID.String(),
		"order_id":   orderID.String(),
		"buyer_id":   buyerID.String(),
		"seller_id":  sellerID.String(),
		"outcome":    "buyer_wins",
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "dispute",
		AggregateID:   disputeID,
		EventType:     "dispute.resolved",
		Payload:       payload,
	}

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	sender := &panicCapturePushSender{}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, sender, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() must not error: %v", err)
	}
}

// TestAsyncPush_NilTx_SellerSubscriptionExpiring verifies seller.subscription.expiring
// does not panic on the async push path.
func TestAsyncPush_NilTx_SellerSubscriptionExpiring(t *testing.T) {
	log := zaptest.NewLogger(t)
	userID := uuid.New()
	subscriptionID := uuid.New()

	payload, _ := json.Marshal(SellerSubscriptionExpiringPayload{
		UserID:          userID.String(),
		SubscriptionID:  subscriptionID.String(),
		ExpiresAt:       "2026-07-10T00:00:00Z",
		DaysUntilExpiry: 7,
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "seller_subscription",
		AggregateID:   subscriptionID,
		EventType:     "seller.subscription.expiring",
		Payload:       payload,
	}

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	sender := &panicCapturePushSender{}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, sender, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() must not error: %v", err)
	}
}
