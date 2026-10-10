//go:build integration

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hishumi/backend/internal/identity/auth"
	notificationRepoImpl "github.com/hishumi/backend/internal/interaction/notification/infrastructure/repository"
	"github.com/hishumi/backend/internal/platform/event"
	"github.com/hishumi/backend/internal/platform/events"
	outboxRepoPkg "github.com/hishumi/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/hishumi/backend/internal/realtime"
	dbpkg "github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TASK 5 — NOTIFICATION REALTIME DELIVERY (end-to-end integration).
//
// Proves the full committed-creation → user-targeted WebSocket chain through
// the REAL infrastructure: real Postgres, real notification repository, real
// outbox repository, the real NotificationEventHandler creation path, the
// real realtime Worker + Dispatcher + Hub.
//
// Chain under test:
//
//	chat.message.notification (outbox business event)
//	  → handler policy + notification INSERT
//	  → SAME TX: outbox row notification.created (only when a row was written)
//	  → realtime Worker claims it (ownership) → Dispatcher → Hub
//	  → recipient connection(s) receive the canonical WS envelope

type notifRealtimeFixture struct {
	tdb        *testdb.TestDB
	appDB      *dbpkg.DB
	handler    *NotificationEventHandler
	outboxRepo *outboxRepoPkg.OutboxRepository
	hub        *realtime.Hub
	rtWorker   *realtime.Worker
	recipient  uuid.UUID
	sender     uuid.UUID
	bystander  uuid.UUID
}

// failingRealtimeOutbox forces the outbox half of the combined operation to
// fail so rollback of the notification half can be proven (Case 5).
type failingRealtimeOutbox struct {
	err error
}

func (f *failingRealtimeOutbox) InsertTx(context.Context, dbpkg.Tx, string, any, string) error {
	return f.err
}

// testRealtimeOutboxAdapter mirrors serverboot's realtimeOutboxRepositoryAdapter
// (test-local copy of the same 15-line conversion).
type testRealtimeOutboxAdapter struct {
	repo *outboxRepoPkg.OutboxRepository
}

func (a *testRealtimeOutboxAdapter) FetchPendingBatch(
	ctx context.Context, tx dbpkg.Tx, limit int, ownedEventTypes []string,
) ([]realtime.Event, error) {
	events, err := a.repo.FetchPendingBatch(ctx, tx, limit, outboxRepoPkg.EventOwnershipScope{Include: ownedEventTypes})
	if err != nil {
		return nil, err
	}
	result := make([]realtime.Event, len(events))
	for i, e := range events {
		result[i] = realtime.Event{
			ID:            e.ID,
			AggregateType: e.AggregateType,
			AggregateID:   e.AggregateID,
			EventType:     e.EventType,
			Payload:       e.Payload,
			Status:        string(e.Status),
			RetryCount:    e.RetryCount,
			NextAttemptAt: e.NextAttemptAt,
		}
	}
	return result, nil
}

func (a *testRealtimeOutboxAdapter) MarkProcessing(ctx context.Context, tx dbpkg.Tx, eventID uuid.UUID) error {
	return a.repo.MarkProcessing(ctx, tx, eventID)
}

func (a *testRealtimeOutboxAdapter) MarkSucceeded(ctx context.Context, tx dbpkg.Tx, eventID uuid.UUID) error {
	return a.repo.MarkSucceeded(ctx, tx, eventID)
}

func (a *testRealtimeOutboxAdapter) MarkFailedWithRetry(ctx context.Context, tx dbpkg.Tx, eventID uuid.UUID, retryCount int, nextAttemptAt time.Time) error {
	return a.repo.MarkFailedWithRetry(ctx, tx, eventID, retryCount, nextAttemptAt)
}

func newNotifRealtimeFixture(t *testing.T) *notifRealtimeFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := dbpkg.NewFromPool(tdb.Pool())
	notifRepo := notificationRepoImpl.NewNotificationRepository()
	outboxRepository := outboxRepoPkg.NewOutboxRepository(appDB)
	statusChecker := auth.NewAccountStatusCheckerDB(appDB)

	handler := NewNotificationEventHandler(appDB, nil, notifRepo, nil, nil, zap.NewNop())
	handler.SetRealtimeOutboxInserter(outboxRepository)
	handler.SetUnreadCounter(notifRepo)

	hub := realtime.NewHub(zap.NewNop())
	rtWorker := realtime.NewWorker(
		appDB,
		&testRealtimeOutboxAdapter{repo: outboxRepository},
		hub,
		statusChecker,
		zap.NewNop(),
		realtime.DefaultWorkerConfig(),
	)

	f := &notifRealtimeFixture{
		tdb:        tdb,
		appDB:      appDB,
		handler:    handler,
		outboxRepo: outboxRepository,
		hub:        hub,
		rtWorker:   rtWorker,
		recipient:  notifRealtimeSeedUser(t, appDB),
		sender:     notifRealtimeSeedUser(t, appDB),
		bystander:  notifRealtimeSeedUser(t, appDB),
	}
	return f
}

func notifRealtimeSeedUser(t *testing.T, appDB *dbpkg.DB) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	_, err := appDB.Pool().Exec(context.Background(),
		`INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), 'active', NOW(), NOW())`,
		userID, "fb-"+userID.String(), userID.String()+"@test.invalid",
	)
	require.NoError(t, err)
	return userID
}

// registerConn attaches a fake connection (buffered outbound channel) for a
// user, exactly like an authenticated device on the hub.
func registerConn(t *testing.T, f *notifRealtimeFixture, userID uuid.UUID) *realtime.Connection {
	t.Helper()

	conn := &realtime.Connection{
		ID:     uuid.NewString(),
		UserID: userID,
		Send:   make(chan []byte, 16),
	}
	f.hub.Register(conn)
	t.Cleanup(func() { f.hub.Unregister(conn) })
	return conn
}

// emitChatNotification drives the REAL creation path: the chat-message
// notification business event through NotificationEventHandler.Handle.
func emitChatNotification(t *testing.T, f *notifRealtimeFixture, senderID, recipientID, roomID uuid.UUID) error {
	t.Helper()

	messageID := uuid.New()
	payload, err := json.Marshal(ChatMessagePayload{
		RoomID:      roomID.String(),
		MessageID:   messageID.String(),
		SenderID:    senderID.String(),
		RecipientID: recipientID.String(),
		MessageType: "text",
	})
	require.NoError(t, err)

	return f.handler.Handle(context.Background(), event.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "chat_room",
		AggregateID:   roomID,
		EventType:     events.EventChatMessageNotification,
		Payload:       payload,
	})
}

func countNotificationsFor(t *testing.T, f *notifRealtimeFixture, recipientID uuid.UUID) int {
	t.Helper()

	var n int
	err := f.appDB.Pool().QueryRow(context.Background(),
		"SELECT COUNT(*) FROM notifications WHERE recipient_id = $1", recipientID,
	).Scan(&n)
	require.NoError(t, err)
	return n
}

func countRealtimeCreatedEvents(t *testing.T, f *notifRealtimeFixture) int {
	t.Helper()

	var n int
	err := f.appDB.Pool().QueryRow(context.Background(),
		`SELECT COUNT(*) FROM outbox WHERE event_type = 'notification.created'`,
	).Scan(&n)
	require.NoError(t, err)
	return n
}

type wsEnvelope struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

func drainConn(t *testing.T, conn *realtime.Connection) []wsEnvelope {
	t.Helper()

	var out []wsEnvelope
	for {
		select {
		case raw := <-conn.Send:
			var env wsEnvelope
			require.NoError(t, json.Unmarshal(raw, &env))
			out = append(out, env)
		default:
			return out
		}
	}
}

// Case 1 + Case 2 + Case 3 — created event delivered to every recipient
// connection and to nobody else.
func TestNotificationRealtime_CreatedEventDeliversToRecipientConnectionsOnly(t *testing.T) {
	f := newNotifRealtimeFixture(t)
	roomID := uuid.New()

	connA := registerConn(t, f, f.recipient) // device 1
	connB := registerConn(t, f, f.recipient) // device 2
	connOther := registerConn(t, f, f.bystander)

	require.NoError(t, emitChatNotification(t, f, f.sender, f.recipient, roomID))

	// Persistence + durable realtime outbox row committed together.
	require.Equal(t, 1, countNotificationsFor(t, f, f.recipient))
	require.Equal(t, 1, countRealtimeCreatedEvents(t, f))

	var storedNotificationID string
	err := f.appDB.Pool().QueryRow(context.Background(),
		"SELECT id::text FROM notifications WHERE recipient_id = $1", f.recipient,
	).Scan(&storedNotificationID)
	require.NoError(t, err)

	var rawPayload string
	err = f.appDB.Pool().QueryRow(context.Background(),
		`SELECT payload FROM outbox WHERE event_type = 'notification.created'`,
	).Scan(&rawPayload)
	require.NoError(t, err)
	var outboxPayload map[string]any
	require.NoError(t, json.Unmarshal([]byte(rawPayload), &outboxPayload))
	require.Equal(t, f.recipient.String(), outboxPayload["recipient_id"])
	require.Equal(t, storedNotificationID, outboxPayload["notification_id"])
	require.Equal(t, float64(1), outboxPayload["unread_count"],
		"payload unread_count must be the canonical post-insert count")

	// Realtime worker claims and delivers.
	require.NoError(t, f.rtWorker.ManualProcess(context.Background()))

	for name, conn := range map[string]*realtime.Connection{"device1": connA, "device2": connB} {
		frames := drainConn(t, conn)
		require.Len(t, frames, 1, "%s must receive exactly one frame", name)
		require.Equal(t, realtime.EventTypeNotificationCreated, frames[0].Type)
		require.Equal(t, storedNotificationID, frames[0].Data["notification_id"])
		require.Equal(t, "chat_message", frames[0].Data["type"])
		require.Equal(t, float64(1), frames[0].Data["unread_count"])
	}

	require.Empty(t, drainConn(t, connOther), "a non-recipient must never receive the event")
}

// Case 4 — dedup: a replayed creation emits no second notification and no
// second notification.created event.
func TestNotificationRealtime_DedupEmitsNoSecondEvent(t *testing.T) {
	f := newNotifRealtimeFixture(t)
	roomID := uuid.New()
	conn := registerConn(t, f, f.recipient)

	require.NoError(t, emitChatNotification(t, f, f.sender, f.recipient, roomID))
	// Same (recipient, actor, type, entity) tuple → canonical dedup no-op.
	require.NoError(t, emitChatNotification(t, f, f.sender, f.recipient, roomID))

	require.Equal(t, 1, countNotificationsFor(t, f, f.recipient), "dedup must keep one row")
	require.Equal(t, 1, countRealtimeCreatedEvents(t, f), "dedup must not emit a second event")

	require.NoError(t, f.rtWorker.ManualProcess(context.Background()))
	require.Len(t, drainConn(t, conn), 1, "exactly one realtime frame for one created row")

	// A second processing pass must not resurrect anything.
	require.NoError(t, f.rtWorker.ManualProcess(context.Background()))
	require.Empty(t, drainConn(t, conn))
}

// Case 5 — rollback: when the combined operation fails, neither the
// notification nor its realtime event survives.
func TestNotificationRealtime_InsertRollbackEmitsNoEvent(t *testing.T) {
	f := newNotifRealtimeFixture(t)
	roomID := uuid.New()
	registerConn(t, f, f.recipient)

	f.handler.SetRealtimeOutboxInserter(&failingRealtimeOutbox{
		err: fmt.Errorf("forced outbox insert failure"),
	})

	err := emitChatNotification(t, f, f.sender, f.recipient, roomID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "forced outbox insert failure")

	require.Equal(t, 0, countNotificationsFor(t, f, f.recipient),
		"a failed combined operation must not persist the notification")
	require.Equal(t, 0, countRealtimeCreatedEvents(t, f),
		"a failed combined operation must not emit the realtime event")
}

// Case 6 — disconnected recipient: delivery finds no connection, yet the
// notification and its durable event remain; the worker marks delivery done
// (existing drop semantics — polling/reconnect reconcile later).
func TestNotificationRealtime_DisconnectedRecipientKeepsPersistence(t *testing.T) {
	f := newNotifRealtimeFixture(t)
	roomID := uuid.New()

	require.NoError(t, emitChatNotification(t, f, f.sender, f.recipient, roomID))
	require.Equal(t, 1, countNotificationsFor(t, f, f.recipient))

	require.NoError(t, f.rtWorker.ManualProcess(context.Background()))

	require.Equal(t, 1, countNotificationsFor(t, f, f.recipient),
		"WebSocket unavailability must never roll back the notification")

	var status string
	err := f.appDB.Pool().QueryRow(context.Background(),
		`SELECT status::text FROM outbox WHERE event_type = 'notification.created'`,
	).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "succeeded", status,
		"hub delivery with no live connection is a canonical drop, not a retry")
}
