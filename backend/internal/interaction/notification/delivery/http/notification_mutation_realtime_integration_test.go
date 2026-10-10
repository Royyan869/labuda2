//go:build integration

package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hishumi/backend/internal/identity/auth"
	chatApp "github.com/hishumi/backend/internal/interaction/chat/application"
	chatEntity "github.com/hishumi/backend/internal/interaction/chat/entity"
	chatInfraRepo "github.com/hishumi/backend/internal/interaction/chat/infrastructure/repository"
	notificationpkg "github.com/hishumi/backend/internal/interaction/notification"
	notificationEntity "github.com/hishumi/backend/internal/interaction/notification/entity"
	notificationRepoImpl "github.com/hishumi/backend/internal/interaction/notification/infrastructure/repository"
	outboxRepoPkg "github.com/hishumi/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/hishumi/backend/internal/realtime"
	socialInfraRepo "github.com/hishumi/backend/internal/social/graph/infrastructure/repository"
	dbpkg "github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/rate"
	"github.com/hishumi/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TASK_4.2 — NOTIFICATION MUTATION REALTIME (end-to-end integration).
//
// Proves the full committed-mutation → notification.updated → user-targeted
// WebSocket chain through REAL infrastructure: real Postgres, real repos, the
// real HTTP mutation handlers, the real ChatRoomReadSyncer, the real realtime
// Worker/Dispatcher/Hub.
//
// Gates: A1 mark-one · A2 mark-all (one event, not N) · A3 delete ·
// A4 chat-room-read sync · A5 no-op mutations · A6 rollback ·
// A7 target isolation (A×2 devices receive, B does not) · A8 duplicates.

type mutationRealtimeFixture struct {
	tdb        *testdb.TestDB
	appDB      *dbpkg.DB
	repo       notificationpkg.Repository
	outboxRepo *outboxRepoPkg.OutboxRepository
	handler    *NotificationHandler
	hub        *realtime.Hub
	rtWorker   *realtime.Worker
	emitter    *notificationpkg.MutationRealtimeEmitter
	userA      uuid.UUID
	userB      uuid.UUID
	sender     uuid.UUID
}

type mutationRealtimeOutbox struct{}

func (mutationRealtimeOutbox) InsertTx(context.Context, dbpkg.Tx, string, any, string) error {
	return nil
}

// mutationRealtimeFailingOutbox forces the emitter's outbox insert to fail so
// rollback of the mutation half can be proven (Gate A6).
type mutationRealtimeFailingOutbox struct{ err error }

func (f *mutationRealtimeFailingOutbox) InsertTx(context.Context, dbpkg.Tx, string, any, string) error {
	return f.err
}

// mutationRealtimeRTAdapter mirrors serverboot's realtimeOutboxRepositoryAdapter
// (test-local copy of the same conversion).
type mutationRealtimeRTAdapter struct {
	repo *outboxRepoPkg.OutboxRepository
}

func (a *mutationRealtimeRTAdapter) FetchPendingBatch(
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

func (a *mutationRealtimeRTAdapter) MarkProcessing(ctx context.Context, tx dbpkg.Tx, eventID uuid.UUID) error {
	return a.repo.MarkProcessing(ctx, tx, eventID)
}

func (a *mutationRealtimeRTAdapter) MarkSucceeded(ctx context.Context, tx dbpkg.Tx, eventID uuid.UUID) error {
	return a.repo.MarkSucceeded(ctx, tx, eventID)
}

func (a *mutationRealtimeRTAdapter) MarkFailedWithRetry(ctx context.Context, tx dbpkg.Tx, eventID uuid.UUID, retryCount int, nextAttemptAt time.Time) error {
	return a.repo.MarkFailedWithRetry(ctx, tx, eventID, retryCount, nextAttemptAt)
}

func newMutationRealtimeFixture(t *testing.T) *mutationRealtimeFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := dbpkg.NewFromPool(tdb.Pool())
	notifRepo := notificationRepoImpl.NewNotificationRepository()
	outboxRepository := outboxRepoPkg.NewOutboxRepository(appDB)
	statusChecker := auth.NewAccountStatusCheckerDB(appDB)
	emitter := &notificationpkg.MutationRealtimeEmitter{
		Outbox:  outboxRepository,
		Counter: notifRepo,
	}

	handler := NewNotificationHandlerWithDefaults(appDB, zap.NewNop())
	handler.SetMutationRealtimeEmitter(emitter)

	hub := realtime.NewHub(zap.NewNop())
	rtWorker := realtime.NewWorker(
		appDB,
		&mutationRealtimeRTAdapter{repo: outboxRepository},
		hub,
		statusChecker,
		zap.NewNop(),
		realtime.DefaultWorkerConfig(),
	)

	return &mutationRealtimeFixture{
		tdb:        tdb,
		appDB:      appDB,
		repo:       notifRepo,
		outboxRepo: outboxRepository,
		handler:    handler,
		hub:        hub,
		rtWorker:   rtWorker,
		emitter:    emitter,
		userA:      mutationRealtimeSeedUser(t, appDB),
		userB:      mutationRealtimeSeedUser(t, appDB),
		sender:     mutationRealtimeSeedUser(t, appDB),
	}
}

func mutationRealtimeSeedUser(t *testing.T, appDB *dbpkg.DB) uuid.UUID {
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

func mutationRealtimeConn(t *testing.T, f *mutationRealtimeFixture, userID uuid.UUID) *realtime.Connection {
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

func mutationRealtimeRouter(userID uuid.UUID, handler *NotificationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})

	notifications := router.Group("/api/v1/notifications")
	notifications.POST("/:id/read", handler.MarkNotificationAsRead)
	notifications.POST("/read-all", handler.MarkAllAsRead)
	notifications.DELETE("/:id", handler.DeleteNotification)
	notifications.GET("/unread-count", handler.GetUnreadCount)
	return router
}

func mutationRealtimePerform(
	t *testing.T,
	router *gin.Engine,
	method, path string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func mutationRealtimeSeedNotification(
	t *testing.T,
	f *mutationRealtimeFixture,
	recipientID, actorID uuid.UUID,
	notifyType notificationEntity.NotificationType,
	entityID uuid.UUID,
) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := f.appDB.WithTx(context.Background(), func(tx dbpkg.Tx) error {
		var insertErr error
		id, _, insertErr = f.repo.Insert(context.Background(), tx,
			notificationEntity.NewNotification(
				recipientID,
				notificationEntity.UserActor(actorID),
				notifyType,
				entityID,
				map[string]interface{}{},
			),
		)
		return insertErr
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)
	return id
}

func mutationRealtimeCountUpdatedEvents(t *testing.T, f *mutationRealtimeFixture) int {
	t.Helper()

	var n int
	err := f.appDB.Pool().QueryRow(context.Background(),
		`SELECT COUNT(*) FROM outbox WHERE event_type = 'notification.updated'`,
	).Scan(&n)
	require.NoError(t, err)
	return n
}

func mutationRealtimeIsRead(t *testing.T, f *mutationRealtimeFixture, notificationID uuid.UUID) bool {
	t.Helper()

	var isRead bool
	err := f.appDB.Pool().QueryRow(context.Background(),
		"SELECT is_read FROM notifications WHERE id = $1", notificationID,
	).Scan(&isRead)
	require.NoError(t, err)
	return isRead
}

type mutationRealtimeEnvelope struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

func mutationRealtimeDrain(t *testing.T, conn *realtime.Connection) []mutationRealtimeEnvelope {
	t.Helper()

	var out []mutationRealtimeEnvelope
	for {
		select {
		case raw := <-conn.Send:
			var env mutationRealtimeEnvelope
			require.NoError(t, json.Unmarshal(raw, &env))
			out = append(out, env)
		default:
			return out
		}
	}
}

// Gate A1 + A7 + A8: mark one read → ONE event → both of A's devices receive,
// B does not; already-read repeat emits nothing (Gate A5 mark-one part).
func TestNotificationMutationRealtime_MarkOneRead(t *testing.T) {
	f := newMutationRealtimeFixture(t)
	router := mutationRealtimeRouter(f.userA, f.handler)

	connA1 := mutationRealtimeConn(t, f, f.userA)
	connA2 := mutationRealtimeConn(t, f, f.userA)
	connB := mutationRealtimeConn(t, f, f.userB)

	notificationID := mutationRealtimeSeedNotification(
		t, f, f.userA, f.sender, notificationEntity.TypeChatMessage, uuid.New(),
	)

	w := mutationRealtimePerform(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/%s/read", notificationID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, mutationRealtimeIsRead(t, f, notificationID))
	require.Equal(t, 1, mutationRealtimeCountUpdatedEvents(t, f),
		"one committed state change → exactly one outbox event")

	require.NoError(t, f.rtWorker.ManualProcess(context.Background()))

	for name, conn := range map[string]*realtime.Connection{"device1": connA1, "device2": connA2} {
		frames := mutationRealtimeDrain(t, conn)
		require.Len(t, frames, 1, "%s must receive exactly one frame", name)
		require.Equal(t, realtime.EventTypeNotificationUpdated, frames[0].Type)
		require.Equal(t, float64(0), frames[0].Data["unread_count"])
	}
	require.Empty(t, mutationRealtimeDrain(t, connB),
		"a different user must never receive the event (target isolation)")

	// Gate A5: marking the already-read notification again → no state change,
	// no second event.
	w = mutationRealtimePerform(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/%s/read", notificationID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, mutationRealtimeCountUpdatedEvents(t, f),
		"marking an already-read notification must not emit a second event")
	require.NoError(t, f.rtWorker.ManualProcess(context.Background()))
	require.Empty(t, mutationRealtimeDrain(t, connA1),
		"no extra frame after the no-op mutation")
}

// Gate A2: mark-all emits ONE user-level event regardless of N notifications.
func TestNotificationMutationRealtime_MarkAllRead(t *testing.T) {
	f := newMutationRealtimeFixture(t)
	router := mutationRealtimeRouter(f.userA, f.handler)
	connA := mutationRealtimeConn(t, f, f.userA)

	for i := 0; i < 3; i++ {
		mutationRealtimeSeedNotification(
			t, f, f.userA, f.sender, notificationEntity.TypeChatMessage, uuid.New(),
		)
	}

	w := mutationRealtimePerform(t, router, http.MethodPost, "/api/v1/notifications/read-all")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, mutationRealtimeCountUpdatedEvents(t, f),
		"mark-all is ONE logical state change → exactly one event, never N")

	require.NoError(t, f.rtWorker.ManualProcess(context.Background()))
	frames := mutationRealtimeDrain(t, connA)
	require.Len(t, frames, 1)
	require.Equal(t, realtime.EventTypeNotificationUpdated, frames[0].Type)
	require.Equal(t, float64(0), frames[0].Data["unread_count"])

	// Gate A5: mark-all over an already-cleared inbox → no event.
	w = mutationRealtimePerform(t, router, http.MethodPost, "/api/v1/notifications/read-all")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, mutationRealtimeCountUpdatedEvents(t, f),
		"mark-all with no unread must not emit an event")
}

// Gate A3: delete emits an event; deleting a missing notification emits none.
func TestNotificationMutationRealtime_Delete(t *testing.T) {
	f := newMutationRealtimeFixture(t)
	router := mutationRealtimeRouter(f.userA, f.handler)
	connA := mutationRealtimeConn(t, f, f.userA)

	notificationID := mutationRealtimeSeedNotification(
		t, f, f.userA, f.sender, notificationEntity.TypeChatMessage, uuid.New(),
	)

	w := mutationRealtimePerform(t, router, http.MethodDelete,
		fmt.Sprintf("/api/v1/notifications/%s", notificationID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, mutationRealtimeCountUpdatedEvents(t, f))

	require.NoError(t, f.rtWorker.ManualProcess(context.Background()))
	frames := mutationRealtimeDrain(t, connA)
	require.Len(t, frames, 1)
	require.Equal(t, realtime.EventTypeNotificationUpdated, frames[0].Type)
	require.Equal(t, float64(0), frames[0].Data["unread_count"])

	// Gate A5: delete of an already-absent notification → 404, no event.
	w = mutationRealtimePerform(t, router, http.MethodDelete,
		fmt.Sprintf("/api/v1/notifications/%s", notificationID))
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.Equal(t, 1, mutationRealtimeCountUpdatedEvents(t, f),
		"deleting an absent notification must not emit an event")
}

// Gate A4: chat room read → the notification sync flips chat notifications
// → exactly ONE notification.updated; a second read with no state change
// emits nothing.
func TestNotificationMutationRealtime_ChatRoomRead(t *testing.T) {
	f := newMutationRealtimeFixture(t)
	connA := mutationRealtimeConn(t, f, f.userA)

	room1 := uuid.New()
	room2 := uuid.New()

	chatNotifRoom1 := mutationRealtimeSeedNotification(
		t, f, f.userA, f.sender, notificationEntity.TypeChatMessage, room1,
	)
	chatNotifRoom2 := mutationRealtimeSeedNotification(
		t, f, f.userA, f.sender, notificationEntity.TypeChatMessage, room2,
	)
	paymentNotif := mutationRealtimeSeedNotification(
		t, f, f.userA, f.sender, notificationEntity.TypeOrderCreated, uuid.New(),
	)

	chatService := chatApp.NewService(
		f.appDB,
		chatInfraRepo.NewChatRepository(),
		socialInfraRepo.NewSocialRepository(),
		mutationRealtimeOutbox{},
		rate.NewRateLimiter(),
		nil,
		nil,
		nil,
		zap.NewNop(),
	)
	chatService.SetChatNotificationReadSyncer(&notificationpkg.ChatRoomReadSyncer{
		Repo:            f.repo,
		MutationEmitter: f.emitter,
	})

	// Seed a message + room so the chat read has a real participant context.
	// Direct-room rows require lexicographically sorted participants
	// (chat_rooms partial CHECK — same rule the Task 3 fixture honors).
	participantA, participantB := f.userA, f.sender
	if participantA.String() > participantB.String() {
		participantA, participantB = participantB, participantA
	}
	require.NoError(t, f.appDB.WithTx(context.Background(), func(tx dbpkg.Tx) error {
		_, err := f.appDB.Pool().Exec(context.Background(), `
			INSERT INTO chat_rooms (id, room_type, participant_a, participant_b, created_at, updated_at, last_message_at)
			VALUES ($1, 'direct', $2, $3, NOW(), NOW(), NOW())`,
			room1, participantA, participantB,
		)
		return err
	}))
	body := "hello"
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO chat_messages (id, room_id, sender_id, message_type, body, attachment_json, idempotency_key, command_fingerprint, created_at)
		VALUES ($1, $2, $3, 'text', $4, '{}'::jsonb, $5, $6, NOW())`,
		uuid.New(), room1, f.sender, body, uuid.NewString(),
		chatEntity.ComputeCommandFingerprint(f.sender, chatEntity.MessageTypeText, &body, map[string]interface{}{}, nil),
	)
	require.NoError(t, err)

	require.NoError(t, chatService.MarkAsRead(
		context.Background(), room1, f.userA, time.Now().UTC().Add(time.Minute),
	))

	// Chat authority advanced; notification state changed for room1 only.
	require.True(t, mutationRealtimeIsRead(t, f, chatNotifRoom1))
	require.False(t, mutationRealtimeIsRead(t, f, chatNotifRoom2),
		"another room's chat notification must stay unread")
	require.False(t, mutationRealtimeIsRead(t, f, paymentNotif),
		"non-chat notifications must stay unread")

	require.Equal(t, 1, mutationRealtimeCountUpdatedEvents(t, f),
		"chat-room-read that flips notification state emits ONE user-level event")

	require.NoError(t, f.rtWorker.ManualProcess(context.Background()))
	frames := mutationRealtimeDrain(t, connA)
	require.Len(t, frames, 1)
	require.Equal(t, realtime.EventTypeNotificationUpdated, frames[0].Type)

	// Second chat read of the same room with no remaining unread chat
	// notifications → no state change → no second event.
	require.NoError(t, chatService.MarkAsRead(
		context.Background(), room1, f.userA, time.Now().UTC().Add(2*time.Minute),
	))
	require.Equal(t, 1, mutationRealtimeCountUpdatedEvents(t, f),
		"a no-change chat-room-read must not emit a second event")
}

// Gate A6: a failed mutation transaction rolls back BOTH the mutation and
// the outbox event — no phantom realtime signal.
func TestNotificationMutationRealtime_RollbackEmitsNoEvent(t *testing.T) {
	f := newMutationRealtimeFixture(t)
	router := mutationRealtimeRouter(f.userA, f.handler)
	mutationRealtimeConn(t, f, f.userA)

	notificationID := mutationRealtimeSeedNotification(
		t, f, f.userA, f.sender, notificationEntity.TypeChatMessage, uuid.New(),
	)

	f.handler.SetMutationRealtimeEmitter(&notificationpkg.MutationRealtimeEmitter{
		Outbox:  &mutationRealtimeFailingOutbox{err: fmt.Errorf("forced outbox failure")},
		Counter: f.repo,
	})

	w := mutationRealtimePerform(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/%s/read", notificationID))
	// The handler maps any transaction failure to an error status (existing
	// contract); the invariant under proof is the atomic rollback, not the
	// status code.
	require.NotEqual(t, http.StatusOK, w.Code, w.Body.String())

	require.False(t, mutationRealtimeIsRead(t, f, notificationID),
		"a failed combined operation must not commit the mutation")
	require.Equal(t, 0, mutationRealtimeCountUpdatedEvents(t, f),
		"a failed combined operation must not leave a phantom event")

	// Recovery: restore the real emitter — the retry succeeds atomically.
	f.handler.SetMutationRealtimeEmitter(f.emitter)
	w = mutationRealtimePerform(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/%s/read", notificationID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, mutationRealtimeIsRead(t, f, notificationID))
	require.Equal(t, 1, mutationRealtimeCountUpdatedEvents(t, f))
}
