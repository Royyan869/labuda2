package worker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap/zaptest"

	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/interaction/notification/policy"
	"github.com/labuda/backend/internal/platform/events"
	dbpkg "github.com/labuda/backend/pkg/db"
)

func TestP1_PushPayload_StringContract(t *testing.T) {
	push := &capturePushPayloadSender{}
	push.wg.Add(1)

	h := NewNotificationEventHandler(
		&mockDBForNotification{},
		&mockBlockCheckerForNotification{},
		NewNotificationServiceInserter(),
		push,
		&mockAccountStatusCheckerForNotification{},
		zaptest.NewLogger(t),
	)

	info := notificationInfo{
		notificationID: uuid.New(),
		inserted:       true,
		recipientID:    uuid.New(),
		actorID:        uuid.New(),
		notifyType:     "withdrawal.requested",
		allowPush:      true,
	}
	h.sendPushAsync(context.Background(), info)
	push.wg.Wait()

	payload := push.lastPayload()
	if payload == nil {
		t.Fatal("expected push payload map, got nil")
	}
	if _, ok := payload["id"].(string); !ok {
		t.Errorf("payload id type = %T, want string", payload["id"])
	}
	if _, ok := payload["recipient_id"].(string); !ok {
		t.Errorf("payload recipient_id type = %T, want string", payload["recipient_id"])
	}
	if _, ok := payload["actor_id"].(string); !ok {
		t.Errorf("payload actor_id type = %T, want string", payload["actor_id"])
	}
}

// TestP1_DedupReplay_NoDuplicatePush proves deduped replay is handled successfully
// but does not send push again.
func TestP1_DedupReplay_NoDuplicatePush(t *testing.T) {
	sellerID := uuid.New()
	withdrawalID := uuid.New()
	insertCalls := 0

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					insertCalls++
					if insertCalls == 1 {
						return &mockRowForNotification{scanValue: uuid.New()}
					}
					return &mockRowForNotification{err: pgx.ErrNoRows}
				},
			}
			return fn(tx)
		},
	}

	push := &pushCountSender{}
	push.wg.Add(1) // first delivery only
	h := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerForNotification{},
		NewNotificationServiceInserter(),
		push,
		&mockAccountStatusCheckerForNotification{},
		zaptest.NewLogger(t),
	)

	payload, _ := json.Marshal(WithdrawalPayload{
		WithdrawalID: withdrawalID.String(),
		SellerID:     sellerID.String(),
		Amount:       50000,
	})
	ev := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "withdrawal.requested",
		Payload:   payload,
	}

	if err := h.Handle(context.Background(), ev); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := h.Handle(context.Background(), ev); err != nil {
		t.Fatalf("replay Handle() error = %v", err)
	}

	push.wg.Wait()
	if got := push.pushCount(); got != 1 {
		t.Errorf("push count = %d, want 1 (replay dedup must not push)", got)
	}
}

func TestO3_HandlePanic_ReturnsError(t *testing.T) {
	log := zaptest.NewLogger(t)
	mockDB := &mockDBForNotification{}

	// Use a panicking inserter so that the handler panics mid-execution.
	handler := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerForNotification{},
		&panickingInserter{},
		nil, // no push
		&mockAccountStatusCheckerForNotification{},
		log,
	)

	actorID := uuid.New()
	recipientID := uuid.New()
	payload, _ := json.Marshal(map[string]interface{}{
		"actor_id":     actorID.String(),
		"recipient_id": recipientID.String(),
	})

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		AggregateID: uuid.New(),
		EventType:   events.EventUserFollowed,
		Payload:     payload,
	}

	err := handler.Handle(context.Background(), event)

	// Must return error, NOT propagate panic
	if err == nil {
		t.Fatal("expected error from panicking handler, got nil")
	}
	if !strings.Contains(err.Error(), "notification handler panic") {
		t.Errorf("error = %q, want substring %q", err, "notification handler panic")
	}
}

// TestO3_HandleNormal_StillSucceeds verifies that panic recovery does not affect
// the normal (non-panicking) code path. Uses an unknown event type which returns
// nil without touching the DB — isolating the recovery wrapper from DB mocking.
func TestO3_HandleNormal_StillSucceeds(t *testing.T) {
	log := zaptest.NewLogger(t)
	mockDB := &mockDBForNotification{}
	inserter := NewNotificationServiceInserter()

	handler := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerForNotification{},
		inserter,
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerForNotification{},
		log,
	)

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		AggregateID: uuid.New(),
		EventType:   "test.unknown_event_for_o3",
		Payload:     []byte(`{}`),
	}

	// Unknown event returns nil — proves recovery wrapper is transparent on normal path
	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("expected nil error for normal handler, got: %v", err)
	}
}

// TestO3_SendPushAsyncPanic_DoesNotCrash verifies that a panic inside
// sendPushAsync() is recovered (goroutine doesn't crash the worker).
func TestO3_SendPushAsyncPanic_DoesNotCrash(t *testing.T) {
	log := zaptest.NewLogger(t)

	handler := &NotificationEventHandler{
		pushSender: &panickingPushSender{},
		log:        log,
	}

	info := notificationInfo{
		recipientID:    uuid.New(),
		notifyType:     "test",
		notificationID: uuid.New(),
		allowPush:      true,
		title:          "Test",
		body:           "Test body",
	}

	// Run synchronously to verify recovery. If no recovery, this test crashes.
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.sendPushAsync(context.Background(), info)
	}()

	<-done
	// If we reach here, the goroutine recovered from the panic successfully.
}

// TestN4A3_FinalBypassCount_Zero proves:
//   - Across all notification_worker*.go source files there is exactly 1
//     InsertNotification call site: the internal DB write inside
//     insertNotificationWithPolicy.
//   - Any additional occurrence = regression (a new direct bypass was introduced).
func TestN4A3_FinalBypassCount_Zero(t *testing.T) {
	files, err := filepath.Glob("notification_worker*.go")
	if err != nil {
		t.Fatalf("glob notification_worker*.go: %v", err)
	}
	// Count direct call-site pattern across all domain split files.
	// Interface definitions and method signatures use a different form and are
	// not matched by this substring.
	const pattern = "h.notificationInserter.InsertNotification("
	total := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, readErr := os.ReadFile(f)
		if readErr != nil {
			t.Fatalf("read %s: %v", f, readErr)
		}
		total += strings.Count(string(src), pattern)
	}
	if total != 1 {
		t.Errorf("direct InsertNotification bypass count = %d, want 1 "+
			"(only the internal write inside insertNotificationWithPolicy); "+
			"extra occurrences are ungoverneed bypass regressions",
			total)
	}
}

// =============================================================================
// G1: DISPUTE AGING ADMIN NOTIFICATION TESTS
// =============================================================================

func makeDisputeOverduePayload(disputeID, orderID, buyerID, sellerID uuid.UUID, daysOpen int) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"dispute_id": disputeID.String(),
		"order_id":   orderID.String(),
		"buyer_id":   buyerID.String(),
		"seller_id":  sellerID.String(),
		"days_open":  daysOpen,
		"reason":     "Dispute has exceeded escalation threshold (3 days)",
	})
	return b
}

func makeDisputeTimeoutEscalationPayload(disputeID, orderID, buyerID, sellerID uuid.UUID, daysOpen, timeoutDays int) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"dispute_id":       disputeID.String(),
		"order_id":         orderID.String(),
		"buyer_id":         buyerID.String(),
		"seller_id":        sellerID.String(),
		"days_open":        daysOpen,
		"timeout_days":     timeoutDays,
		"escalation_level": "critical",
		"reason":           "Dispute exceeded timeout period - requires immediate admin review",
		"policy":           "escalate",
	})
	return b
}

func TestModerationPushPolicy_SuspensionAndRestorationRequirePush(t *testing.T) {
	pushTypes := []string{
		"moderation.user.suspended",
		"moderation.warning.issued",
		"moderation.content.restored",
		"moderation.for_sale.restored",
	}
	for _, typ := range pushTypes {
		if !policy.RequiresPushByType(typ) {
			t.Errorf("RequiresPushByType(%q) = false, want true", typ)
		}
	}
}

func TestModerationPushPolicy_RemovalTypesInAppOnly(t *testing.T) {
	inAppOnly := []string{
		"moderation.content.removed",
		"moderation.comment.removed",
		"moderation.for_sale.removed",
		"moderation.comment.restored",
		"moderation.user.restored",
	}
	for _, typ := range inAppOnly {
		if policy.RequiresPushByType(typ) {
			t.Errorf("RequiresPushByType(%q) = true, want false", typ)
		}
	}
}


