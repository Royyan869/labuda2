//go:build integration

package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chathttp "github.com/labuda/backend/internal/interaction/chat/delivery/http"
	chatInfraRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	notificationpkg "github.com/labuda/backend/internal/interaction/notification"
	notificationhttp "github.com/labuda/backend/internal/interaction/notification/delivery/http"
	notificationentity "github.com/labuda/backend/internal/interaction/notification/entity"
	notificationrepoimpl "github.com/labuda/backend/internal/interaction/notification/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/platform/events"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	socialrepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	likeApp "github.com/labuda/backend/internal/social/like/application"
	likerepo "github.com/labuda/backend/internal/social/like/infrastructure/repository"
	worker "github.com/labuda/backend/internal/worker"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type recordedPushCall struct {
	notification map[string]any
	title        string
	body         string
}

type recordingPushSender struct {
	mu    sync.Mutex
	calls []recordedPushCall
}

type panicRecordingPushSender struct {
	mu    sync.Mutex
	calls []recordedPushCall
}

func (r *recordingPushSender) SendNotification(ctx context.Context, tx interface{}, notification interface{}, title, body string) error {
	callMap, ok := notification.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected notification type %T", notification)
	}

	cloned := make(map[string]any, len(callMap))
	for k, v := range callMap {
		cloned[k] = v
	}

	r.mu.Lock()
	r.calls = append(r.calls, recordedPushCall{
		notification: cloned,
		title:        title,
		body:         body,
	})
	r.mu.Unlock()
	return nil
}

func (r *recordingPushSender) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *recordingPushSender) latest() recordedPushCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return recordedPushCall{}
	}
	return r.calls[len(r.calls)-1]
}

func (r *panicRecordingPushSender) SendNotification(ctx context.Context, tx interface{}, notification interface{}, title, body string) error {
	callMap, ok := notification.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected notification type %T", notification)
	}

	cloned := make(map[string]any, len(callMap))
	for k, v := range callMap {
		cloned[k] = v
	}

	r.mu.Lock()
	r.calls = append(r.calls, recordedPushCall{
		notification: cloned,
		title:        title,
		body:         body,
	})
	r.mu.Unlock()
	panic("forced notification push failure")
}

func (r *panicRecordingPushSender) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *panicRecordingPushSender) latest() recordedPushCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return recordedPushCall{}
	}
	return r.calls[len(r.calls)-1]
}

type notificationBlockCheckerAdapter struct {
	db   *db.DB
	repo interface {
		ExistsBlock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)
	}
}

func (a *notificationBlockCheckerAdapter) ExistsBlock(ctx context.Context, userA, userB uuid.UUID) (bool, error) {
	var blocked bool
	err := a.db.WithTx(ctx, func(tx db.Tx) error {
		var innerErr error
		blocked, innerErr = a.repo.ExistsBlock(ctx, tx, userA, userB)
		return innerErr
	})
	return blocked, err
}

type notificationLifecycleFixture struct {
	appDB            *db.DB
	notificationRepo notificationpkg.Repository
	handler          *worker.NotificationEventHandler
	notificationHTTP *notificationhttp.NotificationHandler
	chatService      *chatApp.Service
	chatHTTP         *chathttp.Handler
	pushSender       *recordingPushSender
}

type noopOutbox struct{}

func (n *noopOutbox) InsertTx(context.Context, db.Tx, string, any, string) error {
	return nil
}

type postCommitFailureTransactor struct {
	db       *db.DB
	failOnce bool
}

func (t *postCommitFailureTransactor) WithTx(ctx context.Context, fn func(db.Tx) error) error {
	tx, err := t.db.BeginTx(ctx)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if t.failOnce {
		t.failOnce = false
		return fmt.Errorf("forced post-commit failure")
	}

	return nil
}

func newNotificationLifecycleFixture(t *testing.T) *notificationLifecycleFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())
	pushSender := &recordingPushSender{}
	blockChecker := &notificationBlockCheckerAdapter{
		db:   appDB,
		repo: socialrepo.NewSocialRepository(),
	}
	accountStatusChecker := auth.NewAccountStatusCheckerDB(appDB)
	handler := worker.NewNotificationEventHandler(
		appDB,
		blockChecker,
		worker.NewNotificationServiceInserter(),
		pushSender,
		accountStatusChecker,
		zap.NewNop(),
	)
	chatService := chatApp.NewService(
		appDB,
		chatInfraRepo.NewChatRepository(),
		socialrepo.NewSocialRepository(),
		&noopOutbox{},
		rate.NewRateLimiter(),
		nil,
		nil,
		nil,
		nil,
		nil,
		zap.NewNop(),
	)

	return &notificationLifecycleFixture{
		appDB:            appDB,
		notificationRepo: notificationrepoimpl.NewNotificationRepository(),
		handler:          handler,
		notificationHTTP: notificationhttp.NewNotificationHandlerWithDefaults(appDB, zap.NewNop()),
		chatService:      chatService,
		chatHTTP:         chathttp.NewHandler(chatService, nil, nil, nil, nil, appDB, zap.NewNop()),
		pushSender:       pushSender,
	}
}

func insertNotificationUser(t *testing.T, ctx context.Context, pool *db.DB, username string, status string, deleted bool) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	var deletedAt any
	if deleted {
		deletedAt = now
	}

	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, phone_verified, account_status,
			email_verified_at, created_at, updated_at, deleted_at, role
		)
		VALUES ($1, $2, $3, true, $4, NOW(), $5, $5, $6, 'user')
	`, userID, userID.String(), fmt.Sprintf("%s-%s@test.invalid", username, userID.String()), status, now, deletedAt)
	require.NoError(t, err)

	_, err = pool.Pool().Exec(ctx, `
		INSERT INTO user_profiles (
			user_id, username, avatar_url, created_at, updated_at
		)
		VALUES ($1, $2, NULL, $3, $3)
	`, userID, fmt.Sprintf("%s-%s", username, userID.String()), now)
	require.NoError(t, err)

	return userID
}

func insertNotificationRoom(t *testing.T, ctx context.Context, pool *db.DB, participantA, participantB uuid.UUID) uuid.UUID {
	t.Helper()

	if participantA.String() > participantB.String() {
		participantA, participantB = participantB, participantA
	}

	roomID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO chat_rooms (
			id, room_type, participant_a, participant_b, created_at, updated_at, last_message_at
		)
		VALUES ($1, 'direct', $2, $3, $4, $4, $4)
	`, roomID, participantA, participantB, now)
	require.NoError(t, err)
	return roomID
}

func insertNotificationMessage(
	t *testing.T,
	ctx context.Context,
	pool *db.DB,
	roomID, senderID uuid.UUID,
	body string,
	attachmentJSON string,
	createdAt time.Time,
) uuid.UUID {
	return insertNotificationMessageWithType(t, ctx, pool, roomID, senderID, "text", &body, attachmentJSON, createdAt)
}

func insertNotificationMessageWithType(
	t *testing.T,
	ctx context.Context,
	pool *db.DB,
	roomID, senderID uuid.UUID,
	messageType string,
	body *string,
	attachmentJSON string,
	createdAt time.Time,
) uuid.UUID {
	t.Helper()

	messageID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO chat_messages (
			id, room_id, sender_id, message_type, body, attachment_json,
			idempotency_key, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
	`, messageID, roomID, senderID, messageType, body, attachmentJSON, uuid.NewString(), createdAt)
	require.NoError(t, err)
	return messageID
}

func emitChatMessageNotificationWithType(
	t *testing.T,
	ctx context.Context,
	handler *worker.NotificationEventHandler,
	senderID, recipientID, roomID, messageID uuid.UUID,
	messageType string,
) {
	t.Helper()

	payload, err := json.Marshal(worker.ChatMessagePayload{
		RoomID:      roomID.String(),
		MessageID:   messageID.String(),
		SenderID:    senderID.String(),
		RecipientID: recipientID.String(),
		MessageType: messageType,
	})
	require.NoError(t, err)

	err = handler.Handle(ctx, event.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "chat_room",
		AggregateID:   roomID,
		EventType:     "chat.message.sent",
		Payload:       payload,
	})
	require.NoError(t, err)
}

func emitChatMessageNotification(t *testing.T, ctx context.Context, handler *worker.NotificationEventHandler, senderID, recipientID, roomID, messageID uuid.UUID) {
	emitChatMessageNotificationWithType(t, ctx, handler, senderID, recipientID, roomID, messageID, "text")
}

func emitUnknownModerationChatEvent(t *testing.T, ctx context.Context, handler *worker.NotificationEventHandler, roomID uuid.UUID, payload []byte, eventType string) {
	t.Helper()

	err := handler.Handle(ctx, event.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "chat_message",
		AggregateID:   roomID,
		EventType:     eventType,
		Payload:       payload,
	})
	require.NoError(t, err)
}

func fetchNotificationCount(t *testing.T, ctx context.Context, pool *db.DB, recipientID, actorID, entityID uuid.UUID) int {
	t.Helper()

	var count int
	err := pool.Pool().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM notifications
		WHERE recipient_id = $1
		  AND actor_id = $2
		  AND type = 'chat_message'
		  AND entity_id = $3
	`, recipientID, actorID, entityID).Scan(&count)
	require.NoError(t, err)
	return count
}

func fetchNotificationRow(t *testing.T, ctx context.Context, repo notificationpkg.Repository, pool *db.DB, recipientID, actorID, entityID uuid.UUID) *notificationentity.Notification {
	t.Helper()

	var id uuid.UUID
	err := pool.Pool().QueryRow(ctx, `
		SELECT id
		FROM notifications
		WHERE recipient_id = $1
		  AND actor_id = $2
		  AND type = 'chat_message'
		  AND entity_id = $3
		ORDER BY created_at DESC
		LIMIT 1
	`, recipientID, actorID, entityID).Scan(&id)
	require.NoError(t, err)

	var n notificationentity.Notification
	err = pool.WithTx(ctx, func(tx db.Tx) error {
		row, err := repo.GetByID(ctx, tx, id)
		if err != nil {
			return err
		}
		n = *row
		return nil
	})
	require.NoError(t, err)
	return &n
}

func getNotificationHTTPResponse(t *testing.T, router *gin.Engine, userID uuid.UUID) map[string]any {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func newNotificationHTTPRouter(userID uuid.UUID, handler *notificationhttp.NotificationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})

	notifications := router.Group("/api/v1/notifications")
	notifications.GET("", handler.GetNotifications)
	return router
}

func newChatHTTPRouter(userID uuid.UUID, handler *chathttp.Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})

	chat := router.Group("/api/v1/chat/rooms")
	chat.GET("/:room_id/messages", handler.ListMessages)
	return router
}

func getChatMessagesHTTPResponse(t *testing.T, router *gin.Engine, roomID uuid.UUID) (int, map[string]any) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return w.Code, resp
}

func TestChatNotificationLifecycle_PostgresBacked(t *testing.T) {
	ctx := context.Background()
	fixture := newNotificationLifecycleFixture(t)

	t.Run("active chat message inserts minimal row, projects cleanly, and dedupes", func(t *testing.T) {
		baseline := fixture.pushSender.count()
		senderID := insertNotificationUser(t, ctx, fixture.appDB, "chat-sender", "active", false)
		recipientID := insertNotificationUser(t, ctx, fixture.appDB, "chat-recipient", "active", false)
		roomID := insertNotificationRoom(t, ctx, fixture.appDB, senderID, recipientID)
		messageID := insertNotificationMessage(
			t,
			ctx,
			fixture.appDB,
			roomID,
			senderID,
			"secret body that must not leak",
			`{"kind":"image","url":"private-attachment"}`,
			time.Now().UTC().Truncate(time.Microsecond),
		)

		emitChatMessageNotification(t, ctx, fixture.handler, senderID, recipientID, roomID, messageID)

		require.Eventually(t, func() bool {
			return fixture.pushSender.count() == baseline+1
		}, 5*time.Second, 20*time.Millisecond)

		require.Equal(t, 1, fetchNotificationCount(t, ctx, fixture.appDB, recipientID, senderID, roomID))

		row := fetchNotificationRow(t, ctx, fixture.notificationRepo, fixture.appDB, recipientID, senderID, roomID)
		require.Equal(t, recipientID, row.RecipientID)
		require.Equal(t, senderID, row.ActorID)
		require.Equal(t, notificationentity.TypeChatMessage, row.Type)
		require.Equal(t, roomID, row.EntityID)
		require.Equal(t, map[string]any{
			"chatId":    roomID.String(),
			"messageId": messageID.String(),
		}, row.Data)
		require.False(t, row.IsRead)

		pushCall := fixture.pushSender.latest()
		require.Equal(t, "New Message", pushCall.title)
		require.Equal(t, "You received a new message", pushCall.body)
		require.Len(t, pushCall.notification, 4)
		require.Equal(t, senderID.String(), pushCall.notification["actor_id"])
		require.Equal(t, recipientID.String(), pushCall.notification["recipient_id"])
		require.Equal(t, "chat_message", pushCall.notification["type"])
		_, err := uuid.Parse(pushCall.notification["id"].(string))
		require.NoError(t, err)

		router := newNotificationHTTPRouter(recipientID, fixture.notificationHTTP)
		resp := getNotificationHTTPResponse(t, router, recipientID)

		data, ok := resp["data"].(map[string]any)
		require.True(t, ok)
		notifications, ok := data["notifications"].([]any)
		require.True(t, ok)
		require.Len(t, notifications, 1)

		first := notifications[0].(map[string]any)
		require.Equal(t, "chat_message", first["type"])
		require.Equal(t, "New Message", first["title"])
		require.Equal(t, "You received a new message", first["body"])

		payloadData, ok := first["data"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, roomID.String(), payloadData["chatId"])
		require.Equal(t, messageID.String(), payloadData["messageId"])
		require.Equal(t, senderID.String(), payloadData["actor_id"])
		require.Equal(t, roomID.String(), payloadData["entity_id"])

		actor, ok := first["actor"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, senderID.String(), actor["id"])
		require.Equal(t, fmt.Sprintf("chat-sender-%s", senderID.String()), actor["username"])

		emitChatMessageNotification(t, ctx, fixture.handler, senderID, recipientID, roomID, messageID)
		time.Sleep(200 * time.Millisecond)
		require.Equal(t, baseline+1, fixture.pushSender.count())
		require.Equal(t, 1, fetchNotificationCount(t, ctx, fixture.appDB, recipientID, senderID, roomID))
	})

	t.Run("sender never receives self notification", func(t *testing.T) {
		baseline := fixture.pushSender.count()
		senderID := insertNotificationUser(t, ctx, fixture.appDB, "chat-self", "active", false)
		roomID := uuid.New()
		messageID := uuid.New()

		emitChatMessageNotification(t, ctx, fixture.handler, senderID, senderID, roomID, messageID)
		time.Sleep(200 * time.Millisecond)

		require.Equal(t, baseline, fixture.pushSender.count())
		require.Equal(t, 0, fetchNotificationCount(t, ctx, fixture.appDB, senderID, senderID, roomID))
	})

	t.Run("recipient blocked or suspended does not create chat notification", func(t *testing.T) {
		cases := []struct {
			name             string
			recipientStatus  string
			recipientDeleted bool
			blocked          bool
		}{
			{name: "blocked relationship", recipientStatus: "active", recipientDeleted: false, blocked: true},
			{name: "suspended recipient", recipientStatus: "suspended", recipientDeleted: false, blocked: false},
			{name: "banned recipient", recipientStatus: "banned", recipientDeleted: false, blocked: false},
			{name: "removed recipient", recipientStatus: "active", recipientDeleted: true, blocked: false},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				baseline := fixture.pushSender.count()
				senderID := insertNotificationUser(t, ctx, fixture.appDB, "chat-sender-"+tc.name, "active", false)
				recipientID := insertNotificationUser(t, ctx, fixture.appDB, "chat-recipient-"+tc.name, tc.recipientStatus, tc.recipientDeleted)
				roomID := insertNotificationRoom(t, ctx, fixture.appDB, senderID, recipientID)
				messageID := insertNotificationMessage(
					t,
					ctx,
					fixture.appDB,
					roomID,
					senderID,
					"recipient gate body",
					`{"kind":"image","url":"recipient-gate-attachment"}`,
					time.Now().UTC().Truncate(time.Microsecond),
				)

				if tc.blocked {
					err := fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
						_, err := tx.Exec(ctx, `
							INSERT INTO user_blocks (blocker_id, blocked_id)
							VALUES ($1, $2)
							ON CONFLICT (blocker_id, blocked_id) DO NOTHING
						`, recipientID, senderID)
						return err
					})
					require.NoError(t, err)
				}

				emitChatMessageNotification(t, ctx, fixture.handler, senderID, recipientID, roomID, messageID)
				time.Sleep(200 * time.Millisecond)
				require.Equal(t, baseline, fixture.pushSender.count())
				require.Equal(t, 0, fetchNotificationCount(t, ctx, fixture.appDB, recipientID, senderID, roomID))
			})
		}
	})

	t.Run("sender suspended keeps the row but suppresses push", func(t *testing.T) {
		baseline := fixture.pushSender.count()
		senderID := insertNotificationUser(t, ctx, fixture.appDB, "chat-sender-suspended", "suspended", false)
		recipientID := insertNotificationUser(t, ctx, fixture.appDB, "chat-recipient-active", "active", false)
		roomID := insertNotificationRoom(t, ctx, fixture.appDB, senderID, recipientID)
		messageID := insertNotificationMessage(
			t,
			ctx,
			fixture.appDB,
			roomID,
			senderID,
			"sender suspended body",
			`{"kind":"file","url":"sender-suspended-attachment"}`,
			time.Now().UTC().Truncate(time.Microsecond),
		)

		emitChatMessageNotification(t, ctx, fixture.handler, senderID, recipientID, roomID, messageID)
		time.Sleep(200 * time.Millisecond)

		require.Equal(t, baseline, fixture.pushSender.count())
		require.Equal(t, 1, fetchNotificationCount(t, ctx, fixture.appDB, recipientID, senderID, roomID))

		var row notificationentity.Notification
		err := fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
			got, err := fixture.notificationRepo.ListByRecipient(ctx, tx, recipientID, 10, 0)
			if err != nil {
				return err
			}
			require.Len(t, got, 1)
			row = *got[0]
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, senderID, row.ActorID)
		require.Equal(t, map[string]any{
			"chatId":    roomID.String(),
			"messageId": messageID.String(),
		}, row.Data)
	})

	t.Run("sender banned or removed drops the notification entirely", func(t *testing.T) {
		cases := []struct {
			name          string
			senderStatus  string
			senderDeleted bool
		}{
			{name: "banned sender", senderStatus: "banned", senderDeleted: false},
			{name: "removed sender", senderStatus: "active", senderDeleted: true},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				baseline := fixture.pushSender.count()
				senderID := insertNotificationUser(t, ctx, fixture.appDB, "chat-sender-"+tc.name, tc.senderStatus, tc.senderDeleted)
				recipientID := insertNotificationUser(t, ctx, fixture.appDB, "chat-recipient-"+tc.name, "active", false)
				roomID := insertNotificationRoom(t, ctx, fixture.appDB, senderID, recipientID)
				messageID := insertNotificationMessage(
					t,
					ctx,
					fixture.appDB,
					roomID,
					senderID,
					"sender gate body",
					`{"kind":"image","url":"sender-gate-attachment"}`,
					time.Now().UTC().Truncate(time.Microsecond),
				)

				emitChatMessageNotification(t, ctx, fixture.handler, senderID, recipientID, roomID, messageID)
				time.Sleep(200 * time.Millisecond)

				require.Equal(t, baseline, fixture.pushSender.count())
				require.Equal(t, 0, fetchNotificationCount(t, ctx, fixture.appDB, recipientID, senderID, roomID))
			})
		}
	})

	t.Run("hidden and restored moderation events do not create chat notifications", func(t *testing.T) {
		baseline := fixture.pushSender.count()
		senderID := insertNotificationUser(t, ctx, fixture.appDB, "chat-moderated-sender", "active", false)
		recipientID := insertNotificationUser(t, ctx, fixture.appDB, "chat-moderated-recipient", "active", false)
		roomID := insertNotificationRoom(t, ctx, fixture.appDB, senderID, recipientID)
		messageID := insertNotificationMessage(
			t,
			ctx,
			fixture.appDB,
			roomID,
			senderID,
			"moderated body",
			`{"kind":"image","url":"moderated-attachment"}`,
			time.Now().UTC().Truncate(time.Microsecond),
		)

		payload, err := json.Marshal(worker.ChatMessagePayload{
			RoomID:      roomID.String(),
			MessageID:   messageID.String(),
			SenderID:    senderID.String(),
			RecipientID: recipientID.String(),
			MessageType: "text",
		})
		require.NoError(t, err)

		emitUnknownModerationChatEvent(t, ctx, fixture.handler, roomID, payload, "moderation.chat_message.hidden")
		emitUnknownModerationChatEvent(t, ctx, fixture.handler, roomID, payload, "moderation.chat_message.restored")
		time.Sleep(200 * time.Millisecond)

		require.Equal(t, baseline, fixture.pushSender.count())
		require.Equal(t, 0, fetchNotificationCount(t, ctx, fixture.appDB, recipientID, senderID, roomID))
	})

	t.Run("attachment-only chat message stays generic in notification row, push, and API", func(t *testing.T) {
		baseline := fixture.pushSender.count()
		senderID := insertNotificationUser(t, ctx, fixture.appDB, "chat-attachment-sender", "active", false)
		recipientID := insertNotificationUser(t, ctx, fixture.appDB, "chat-attachment-recipient", "active", false)
		roomID := insertNotificationRoom(t, ctx, fixture.appDB, senderID, recipientID)
		messageID := insertNotificationMessageWithType(
			t,
			ctx,
			fixture.appDB,
			roomID,
			senderID,
			"negotiation_proposal",
			nil,
			`{"type":"image","data":{"url":"private://object-key","filename":"secret.jpg","caption":"leaked caption"}}`,
			time.Now().UTC().Truncate(time.Microsecond),
		)

		emitChatMessageNotificationWithType(
			t,
			ctx,
			fixture.handler,
			senderID,
			recipientID,
			roomID,
			messageID,
			"negotiation_proposal",
		)

		require.Eventually(t, func() bool {
			return fixture.pushSender.count() == baseline+1
		}, 5*time.Second, 20*time.Millisecond)

		require.Equal(t, 1, fetchNotificationCount(t, ctx, fixture.appDB, recipientID, senderID, roomID))

		row := fetchNotificationRow(t, ctx, fixture.notificationRepo, fixture.appDB, recipientID, senderID, roomID)
		require.Equal(t, map[string]any{
			"chatId":    roomID.String(),
			"messageId": messageID.String(),
		}, row.Data)

		pushCall := fixture.pushSender.latest()
		require.Equal(t, "New Message", pushCall.title)
		require.Equal(t, "You received a new message", pushCall.body)
		require.Len(t, pushCall.notification, 4)
		require.Equal(t, senderID.String(), pushCall.notification["actor_id"])
		require.Equal(t, recipientID.String(), pushCall.notification["recipient_id"])
		require.Equal(t, "chat_message", pushCall.notification["type"])

		router := newNotificationHTTPRouter(recipientID, fixture.notificationHTTP)
		resp := getNotificationHTTPResponse(t, router, recipientID)
		data, ok := resp["data"].(map[string]any)
		require.True(t, ok)
		notifications, ok := data["notifications"].([]any)
		require.True(t, ok)
		require.Len(t, notifications, 1)

		first := notifications[0].(map[string]any)
		require.Equal(t, "New Message", first["title"])
		require.Equal(t, "You received a new message", first["body"])
		payloadData, ok := first["data"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, roomID.String(), payloadData["chatId"])
		require.Equal(t, messageID.String(), payloadData["messageId"])
		require.Equal(t, senderID.String(), payloadData["actor_id"])
		require.Equal(t, roomID.String(), payloadData["entity_id"])

		require.Equal(t, baseline+1, fixture.pushSender.count())
	})

	t.Run("hidden after row exists stays generic and hides chat content on tap", func(t *testing.T) {
		senderID := insertNotificationUser(t, ctx, fixture.appDB, "chat-hide-sender", "active", false)
		recipientID := insertNotificationUser(t, ctx, fixture.appDB, "chat-hide-recipient", "active", false)
		moderatorID := insertNotificationUser(t, ctx, fixture.appDB, "chat-moderator", "active", false)
		roomID := insertNotificationRoom(t, ctx, fixture.appDB, senderID, recipientID)
		secretBody := "hidden body that must not leak"
		messageID := insertNotificationMessage(
			t,
			ctx,
			fixture.appDB,
			roomID,
			senderID,
			secretBody,
			`{"type":"image","data":{"url":"private://attachment","filename":"hidden.png","caption":"secret caption"}}`,
			time.Now().UTC().Truncate(time.Microsecond),
		)

		emitChatMessageNotification(t, ctx, fixture.handler, senderID, recipientID, roomID, messageID)
		require.Eventually(t, func() bool {
			return fixture.pushSender.count() >= 1
		}, 5*time.Second, 20*time.Millisecond)

		notificationBeforeHide := fetchNotificationRow(t, ctx, fixture.notificationRepo, fixture.appDB, recipientID, senderID, roomID)
		require.Equal(t, map[string]any{
			"chatId":    roomID.String(),
			"messageId": messageID.String(),
		}, notificationBeforeHide.Data)

		err := fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
			return fixture.chatService.SoftHideForModeration(ctx, tx, messageID, moderatorID, "moderation hide", "hidden-after-row")
		})
		require.NoError(t, err)

		routerRecipient := newChatHTTPRouter(recipientID, fixture.chatHTTP)
		status, chatResp := getChatMessagesHTTPResponse(t, routerRecipient, roomID)
		require.Equal(t, http.StatusOK, status)

		chatData, ok := chatResp["data"].(map[string]any)
		require.True(t, ok)
		data, ok := chatData["data"].([]any)
		require.True(t, ok)
		require.Len(t, data, 1)
		hiddenMessage := data[0].(map[string]any)
		require.Equal(t, true, hiddenMessage["is_hidden"])
		_, hasBody := hiddenMessage["body"]
		_, hasAttachment := hiddenMessage["attachment_json"]
		_, hasSender := hiddenMessage["sender"]
		require.False(t, hasBody)
		require.False(t, hasAttachment)
		require.False(t, hasSender)

		routerOutsider := newChatHTTPRouter(uuid.New(), fixture.chatHTTP)
		outsiderStatus, _ := getChatMessagesHTTPResponse(t, routerOutsider, roomID)
		require.Equal(t, http.StatusForbidden, outsiderStatus)

		err = fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
			return fixture.chatService.RestoreFromModeration(ctx, tx, messageID, "restored-after-row")
		})
		require.NoError(t, err)

		status, chatResp = getChatMessagesHTTPResponse(t, routerRecipient, roomID)
		require.Equal(t, http.StatusOK, status)
		chatData, ok = chatResp["data"].(map[string]any)
		require.True(t, ok)
		data, ok = chatData["data"].([]any)
		require.True(t, ok)
		require.Len(t, data, 1)
		restoredMessage := data[0].(map[string]any)
		require.Equal(t, secretBody, restoredMessage["body"])
		_, hasHidden := restoredMessage["is_hidden"]
		require.False(t, hasHidden)
		require.Equal(t, 1, fetchNotificationCount(t, ctx, fixture.appDB, recipientID, senderID, roomID))
	})

	t.Run("retry after push-boundary panic stays idempotent", func(t *testing.T) {
		transactor := &postCommitFailureTransactor{db: fixture.appDB, failOnce: true}
		panicSender := &recordingPushSender{}
		handler := worker.NewNotificationEventHandler(
			transactor,
			&notificationBlockCheckerAdapter{
				db:   fixture.appDB,
				repo: socialrepo.NewSocialRepository(),
			},
			worker.NewNotificationServiceInserter(),
			panicSender,
			auth.NewAccountStatusCheckerDB(fixture.appDB),
			zap.NewNop(),
		)

		senderID := insertNotificationUser(t, ctx, fixture.appDB, "chat-retry-sender", "active", false)
		recipientID := insertNotificationUser(t, ctx, fixture.appDB, "chat-retry-recipient", "active", false)
		roomID := insertNotificationRoom(t, ctx, fixture.appDB, senderID, recipientID)
		messageID := insertNotificationMessage(
			t,
			ctx,
			fixture.appDB,
			roomID,
			senderID,
			"retry body",
			`{"kind":"image","url":"retry-attachment"}`,
			time.Now().UTC().Truncate(time.Microsecond),
		)

		payloadEvent := worker.ChatMessagePayload{
			RoomID:      roomID.String(),
			MessageID:   messageID.String(),
			SenderID:    senderID.String(),
			RecipientID: recipientID.String(),
			MessageType: "text",
		}
		payload, err := json.Marshal(payloadEvent)
		require.NoError(t, err)

		err = handler.Handle(ctx, event.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "chat_room",
			AggregateID:   roomID,
			EventType:     "chat.message.sent",
			Payload:       payload,
		})
		require.Error(t, err)
		require.Equal(t, 0, panicSender.count())
		require.Equal(t, 1, fetchNotificationCount(t, ctx, fixture.appDB, recipientID, senderID, roomID))

		err = handler.Handle(ctx, event.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "chat_room",
			AggregateID:   roomID,
			EventType:     "chat.message.sent",
			Payload:       payload,
		})
		require.NoError(t, err)
		require.Equal(t, 0, panicSender.count())
		require.Equal(t, 1, fetchNotificationCount(t, ctx, fixture.appDB, recipientID, senderID, roomID))
		row := fetchNotificationRow(t, ctx, fixture.notificationRepo, fixture.appDB, recipientID, senderID, roomID)
		require.Equal(t, map[string]any{
			"chatId":    roomID.String(),
			"messageId": messageID.String(),
		}, row.Data)
	})
}

// =============================================================================
// LIKES DOMAIN — like → unlike → like OCCURRENCE LIFECYCLE (DB-backed proof)
// =============================================================================

// contentLikeLifecycleFixture wires the authoritative LikeService (real
// repos, real outbox, real notification scrubber) against the same database
// the notification worker consumes, proving like occurrence semantics
// end-to-end: content_likes row → content.liked outbox event → worker →
// notifications.
type contentLikeLifecycleFixture struct {
	appDB       *db.DB
	likeService *likeApp.Service
	handler     *worker.NotificationEventHandler
	pushSender  *recordingPushSender
}

func newContentLikeLifecycleFixture(t *testing.T) *contentLikeLifecycleFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())
	pushSender := &recordingPushSender{}
	blockChecker := &notificationBlockCheckerAdapter{
		db:   appDB,
		repo: socialrepo.NewSocialRepository(),
	}
	handler := worker.NewNotificationEventHandler(
		appDB,
		blockChecker,
		worker.NewNotificationServiceInserter(),
		pushSender,
		auth.NewAccountStatusCheckerDB(appDB),
		zap.NewNop(),
	)
	likeService := likeApp.NewService(
		appDB,
		contentrepo.NewContentRepository(),
		likerepo.NewLikeRepository(),
		outboxRepo.NewOutboxRepository(appDB),
		socialrepo.NewSocialRepository(),
		nil,
		&notificationrepoimpl.NotificationRepository{},
	)

	return &contentLikeLifecycleFixture{
		appDB:       appDB,
		likeService: likeService,
		handler:     handler,
		pushSender:  pushSender,
	}
}

func insertLikeContent(t *testing.T, ctx context.Context, pool *db.DB, authorID uuid.UUID, caption string) uuid.UUID {
	t.Helper()
	contentID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO contents (id, author_id, caption, visibility, created_at, updated_at)
		VALUES ($1, $2, $3, 'public', NOW(), NOW())
	`, contentID, authorID, caption)
	require.NoError(t, err)
	return contentID
}

func countLikeRows(t *testing.T, ctx context.Context, pool *db.DB, contentID, userID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, pool.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM content_likes WHERE content_id = $1 AND user_id = $2
	`, contentID, userID).Scan(&n))
	return n
}

func countPendingContentLikedEvents(t *testing.T, ctx context.Context, pool *db.DB, contentID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, pool.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM outbox WHERE event_type = 'content.liked' AND status = 'pending' AND payload->>'content_id' = $1
	`, contentID.String()).Scan(&n))
	return n
}

func countContentLikedNotifications(t *testing.T, ctx context.Context, pool *db.DB, recipientID, likerID, contentID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, pool.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM notifications
		WHERE recipient_id = $1 AND actor_id = $2 AND type = 'content.liked' AND entity_id = $3
	`, recipientID, likerID, contentID).Scan(&n))
	return n
}

// deliverPendingLikes mimics the outbox worker: fetch pending content.liked
// events in created order, deliver through the notification handler, then mark
// each succeeded. Returns the number of events fetched (delivered or skipped).
func deliverPendingLikes(t *testing.T, ctx context.Context, fixture *contentLikeLifecycleFixture, contentID uuid.UUID) int {
	t.Helper()

	type pendingEvent struct {
		id          uuid.UUID
		eventType   string
		payload     []byte
		aggregate   string
		aggregateID uuid.UUID
	}
	var events []pendingEvent

	rows, err := fixture.appDB.Pool().Query(ctx, `
		SELECT id, aggregate_type, aggregate_id, event_type, payload
		FROM outbox
		WHERE event_type = 'content.liked' AND status = 'pending' AND payload->>'content_id' = $1
		ORDER BY created_at ASC
	`, contentID.String())
	require.NoError(t, err)
	for rows.Next() {
		var ev pendingEvent
		require.NoError(t, rows.Scan(&ev.id, &ev.aggregate, &ev.aggregateID, &ev.eventType, &ev.payload))
		events = append(events, ev)
	}
	require.NoError(t, rows.Err())
	rows.Close()

	for _, ev := range events {
		err := fixture.handler.Handle(ctx, event.OutboxEvent{
			ID:            ev.id,
			AggregateType: ev.aggregate,
			AggregateID:   ev.aggregateID,
			EventType:     ev.eventType,
			Payload:       ev.payload,
		})
		require.NoError(t, err)
		_, err = fixture.appDB.Pool().Exec(ctx, `
			UPDATE outbox SET status = 'succeeded', updated_at = NOW() WHERE id = $1
		`, ev.id)
		require.NoError(t, err)
	}
	return len(events)
}

// fetchFirstContentLikedEvent returns the FIRST (oldest) content.liked outbox
// event payload for a content, regardless of status (used to replay a stale
// event through the worker to prove the occurrence guard).
func fetchContentLikedEventPayloads(t *testing.T, ctx context.Context, pool *db.DB, contentID uuid.UUID) [][]byte {
	t.Helper()
	var payloads [][]byte
	rows, err := pool.Pool().Query(ctx, `
		SELECT payload FROM outbox
		WHERE event_type = 'content.liked' AND payload->>'content_id' = $1
		ORDER BY created_at ASC
	`, contentID.String())
	require.NoError(t, err)
	for rows.Next() {
		var p []byte
		require.NoError(t, rows.Scan(&p))
		payloads = append(payloads, p)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	return payloads
}

func TestContentLike_ReLikeAfterUnlike_DeliversNewNotification(t *testing.T) {
	fixture := newContentLikeLifecycleFixture(t)
	ctx := context.Background()

	authorID := insertNotificationUser(t, ctx, fixture.appDB, "like-cycle-author", "active", false)
	likerID := insertNotificationUser(t, ctx, fixture.appDB, "like-cycle-liker", "active", false)
	contentID := insertLikeContent(t, ctx, fixture.appDB, authorID, "like cycle post")

	assertCounts := func(likes, notifs, pending int) {
		t.Helper()
		require.Equal(t, likes, countLikeRows(t, ctx, fixture.appDB, contentID, likerID), "content_likes rows")
		require.Equal(t, notifs, countContentLikedNotifications(t, ctx, fixture.appDB, authorID, likerID, contentID), "notifications rows")
		require.Equal(t, pending, countPendingContentLikedEvents(t, ctx, fixture.appDB, contentID), "pending content.liked outbox rows")
	}

	assertCounts(0, 0, 0)

	// --- FIRST LIKE (new occurrence) ---
	res, err := fixture.likeService.ToggleContentLike(ctx, contentID, likerID)
	require.NoError(t, err)
	require.True(t, res.Liked)
	require.Equal(t, 1, res.Count)
	assertCounts(1, 0, 1)
	t.Logf("RUNTIME PHASE=first-like likes=%d notifications=%d pending_events=%d", countLikeRows(t, ctx, fixture.appDB, contentID, likerID), countContentLikedNotifications(t, ctx, fixture.appDB, authorID, likerID, contentID), countPendingContentLikedEvents(t, ctx, fixture.appDB, contentID))

	// duplicate/retry first LIKE (idempotent like path) must NOT add a row or event
	require.NoError(t, fixture.likeService.Like(ctx, contentID, likerID))
	require.NoError(t, fixture.likeService.Like(ctx, contentID, likerID))
	assertCounts(1, 0, 1)
	t.Logf("RUNTIME PHASE=first-like-retry likes=%d pending_events=%d (deduped)", countLikeRows(t, ctx, fixture.appDB, contentID, likerID), countPendingContentLikedEvents(t, ctx, fixture.appDB, contentID))

	// worker delivers → exactly one notification
	require.Equal(t, 1, deliverPendingLikes(t, ctx, fixture, contentID))
	assertCounts(1, 1, 0)
	t.Logf("RUNTIME PHASE=first-like-delivered notifications=%d", countContentLikedNotifications(t, ctx, fixture.appDB, authorID, likerID, contentID))

	// --- UNLIKE (removes row + scrubs notification, releases occurrence) ---
	res, err = fixture.likeService.ToggleContentLike(ctx, contentID, likerID)
	require.NoError(t, err)
	require.False(t, res.Liked)
	require.Equal(t, 0, res.Count)
	assertCounts(0, 0, 0)
	t.Logf("RUNTIME PHASE=unlike likes=%d notifications=%d (scrubbed)", countLikeRows(t, ctx, fixture.appDB, contentID, likerID), countContentLikedNotifications(t, ctx, fixture.appDB, authorID, likerID, contentID))

	// --- LIKE AGAIN (new occurrence; must NOT be suppressed by history) ---
	res, err = fixture.likeService.ToggleContentLike(ctx, contentID, likerID)
	require.NoError(t, err)
	require.True(t, res.Liked)
	require.Equal(t, 1, res.Count)
	assertCounts(1, 0, 1)
	t.Logf("RUNTIME PHASE=re-like likes=%d pending_events=%d (new key)", countLikeRows(t, ctx, fixture.appDB, contentID, likerID), countPendingContentLikedEvents(t, ctx, fixture.appDB, contentID))

	// Stale replay of the FIRST (unliked) occurrence's event while the current
	// like is a NEW occurrence must be skipped by the occurrence guard — the
	// event's created_at no longer matches the current content_likes row.
	stalePayloads := fetchContentLikedEventPayloads(t, ctx, fixture.appDB, contentID)
	require.Len(t, stalePayloads, 2, "two occurrence events exist (T1 and T2)")
	var cp worker.ContentLikedPayload
	require.NoError(t, json.Unmarshal(stalePayloads[0], &cp))
	require.NoError(t, fixture.handler.Handle(ctx, event.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "content",
		AggregateID:   contentID,
		EventType:     events.EventContentLiked,
		Payload:       stalePayloads[0],
	}))
	assertCounts(1, 0, 1)

	// delivery of the NEW occurrence produces exactly one fresh notification
	require.Equal(t, 1, deliverPendingLikes(t, ctx, fixture, contentID))
	assertCounts(1, 1, 0)
	t.Logf("RUNTIME PHASE=re-like-delivered notifications=%d (NEW, was not suppressed)", countContentLikedNotifications(t, ctx, fixture.appDB, authorID, likerID, contentID))

	// retry second LIKE: idempotent, no new rows/events/notifications
	require.NoError(t, fixture.likeService.Like(ctx, contentID, likerID))
	require.NoError(t, fixture.likeService.Like(ctx, contentID, likerID))
	require.Equal(t, 0, deliverPendingLikes(t, ctx, fixture, contentID))
	assertCounts(1, 1, 0)
	t.Logf("RUNTIME PHASE=re-like-retry likes=%d notifications=%d (stable, deduped)", countLikeRows(t, ctx, fixture.appDB, contentID, likerID), countContentLikedNotifications(t, ctx, fixture.appDB, authorID, likerID, contentID))
}

func TestContentLike_ConcurrentDoubleToggle_NeverDuplicatesBusinessState(t *testing.T) {
	fixture := newContentLikeLifecycleFixture(t)
	ctx := context.Background()

	authorID := insertNotificationUser(t, ctx, fixture.appDB, "like-conc-author", "active", false)
	likerID := insertNotificationUser(t, ctx, fixture.appDB, "like-conc-liker", "active", false)
	contentID := insertLikeContent(t, ctx, fixture.appDB, authorID, "concurrent toggle post")

	const attempts = 8
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = fixture.likeService.ToggleContentLike(ctx, contentID, likerID)
		}()
	}
	wg.Wait()

	likes := countLikeRows(t, ctx, fixture.appDB, contentID, likerID)
	pending := countPendingContentLikedEvents(t, ctx, fixture.appDB, contentID)
	require.LessOrEqual(t, likes, 1, "concurrent toggles must never leave >1 like row")
	require.LessOrEqual(t, pending, 1, "concurrent toggles must never leave >1 pending content.liked event")

	deliverPendingLikes(t, ctx, fixture, contentID)
	notifs := countContentLikedNotifications(t, ctx, fixture.appDB, authorID, likerID, contentID)
	require.LessOrEqual(t, notifs, 1, "no duplicate notification under concurrent toggles")
}
