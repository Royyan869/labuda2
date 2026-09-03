//go:build integration

package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatInfraRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/internal/realtime"
	socialInfraRepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type unreadIntegrationFixture struct {
	tdb     *testdb.TestDB
	appDB   *db.DB
	service *chatApp.Service
	handler *Handler
	outbox  *recordingUnreadOutbox
}

type unreadOutboxInsert struct {
	eventType string
	payload   map[string]any
	key       string
}

type recordingUnreadOutbox struct {
	mu      sync.Mutex
	inserts []unreadOutboxInsert
}

func (r *recordingUnreadOutbox) InsertTx(
	_ context.Context,
	_ db.Tx,
	eventType string,
	payload any,
	idempotencyKey string,
) error {
	data, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("payload type %T, want map[string]any", payload)
	}

	cloned := make(map[string]any, len(data))
	for k, v := range data {
		cloned[k] = v
	}

	r.mu.Lock()
	r.inserts = append(r.inserts, unreadOutboxInsert{
		eventType: eventType,
		payload:   cloned,
		key:       idempotencyKey,
	})
	r.mu.Unlock()

	return nil
}

func (r *recordingUnreadOutbox) snapshot() []unreadOutboxInsert {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]unreadOutboxInsert, len(r.inserts))
	copy(out, r.inserts)
	return out
}

type unreadUpsertFailureRepo struct {
	chatRepo.Repository
	err error
}

func (r *unreadUpsertFailureRepo) UpsertReadState(context.Context, interface{}, *chatEntity.ChatReadState) error {
	return r.err
}

func newUnreadIntegrationFixture(t *testing.T) *unreadIntegrationFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())
	outbox := &recordingUnreadOutbox{}

	service := chatApp.NewService(
		appDB,
		chatInfraRepo.NewChatRepository(),
		socialInfraRepo.NewSocialRepository(),
		outbox,
		rate.NewRateLimiter(),
		nil,
		nil,
		nil,
		zap.NewNop(),
	)

	handler := NewHandler(
		service,
		nil,
		nil,
		nil,
		nil,
		appDB,
		zap.NewNop(),
	)

	return &unreadIntegrationFixture{
		tdb:     tdb,
		appDB:   appDB,
		service: service,
		handler: handler,
		outbox:  outbox,
	}
}

func insertUnreadTestUser(t *testing.T, ctx context.Context, pool *db.DB) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, email_verified_at, phone_verified,
			account_status, created_at, updated_at, role
		)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW(), 'user')
	`, userID, userID.String(), userID.String()+"@test.invalid")
	require.NoError(t, err)
	return userID
}

func insertUnreadTestRoom(t *testing.T, ctx context.Context, pool *db.DB, participantA, participantB uuid.UUID) uuid.UUID {
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

func insertUnreadTestMessage(
	t *testing.T,
	ctx context.Context,
	pool *db.DB,
	roomID, senderID uuid.UUID,
	body string,
	createdAt time.Time,
) uuid.UUID {
	t.Helper()

	messageID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO chat_messages (
			id, room_id, sender_id, message_type, body, attachment_json,
			idempotency_key, command_fingerprint, created_at
		)
		VALUES ($1, $2, $3, 'text', $4, NULL, $5, $6, $7)
	`, messageID, roomID, senderID, body, uuid.NewString(), chatEntity.ComputeCommandFingerprint(senderID, chatEntity.MessageTypeText, &body, nil), createdAt)
	require.NoError(t, err)
	return messageID
}

func insertUnreadTestMessageWithAttachment(
	t *testing.T,
	ctx context.Context,
	pool *db.DB,
	roomID, senderID uuid.UUID,
	body *string,
	attachmentJSON string,
	createdAt time.Time,
) uuid.UUID {
	t.Helper()

	messageID := uuid.New()
	var parsedAttachment map[string]interface{}
	if attachmentJSON != "" && attachmentJSON != "null" {
		_ = json.Unmarshal([]byte(attachmentJSON), &parsedAttachment)
	}

	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO chat_messages (
			id, room_id, sender_id, message_type, body, attachment_json,
			idempotency_key, command_fingerprint, created_at
		)
		VALUES ($1, $2, $3, 'text', $4, $5::jsonb, $6, $7, $8)
	`, messageID, roomID, senderID, body, attachmentJSON, uuid.NewString(), chatEntity.ComputeCommandFingerprint(senderID, chatEntity.MessageTypeText, body, parsedAttachment), createdAt)
	require.NoError(t, err)
	return messageID
}

func upsertUnreadTestReadState(t *testing.T, ctx context.Context, pool *db.DB, roomID, userID uuid.UUID, lastReadAt time.Time) {
	t.Helper()

	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO chat_read_states (room_id, user_id, last_read_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (room_id, user_id)
		DO UPDATE SET last_read_at = EXCLUDED.last_read_at
	`, roomID, userID, lastReadAt)
	require.NoError(t, err)
}

func getUnreadHTTPCount(t *testing.T, router *gin.Engine, roomID uuid.UUID) int {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/unread", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected data envelope, got %s", w.Body.String())

	return int(data["unread_count"].(float64))
}

func markReadHTTP(t *testing.T, router *gin.Engine, roomID uuid.UUID, timestamp time.Time) int {
	t.Helper()

	body := fmt.Sprintf(`{"timestamp":"%s"}`, timestamp.UTC().Format(time.RFC3339Nano))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/rooms/"+roomID.String()+"/read", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code
}

func newUnreadAuthRouter(userID uuid.UUID, handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})

	chatRoutes := router.Group("/api/v1/chat")
	chatRoutes.POST("/rooms/:room_id/read", handler.MarkAsRead)
	chatRoutes.GET("/rooms/:room_id/unread", handler.GetUnreadCount)

	return router
}

func newChatHTTPRouter(userID uuid.UUID, handler *Handler) *gin.Engine {
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

	if w.Code != http.StatusOK {
		return w.Code, nil
	}

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return w.Code, resp
}

func assertUnreadParity(
	t *testing.T,
	fixture *unreadIntegrationFixture,
	router *gin.Engine,
	roomID, userID uuid.UUID,
	want int,
) {
	t.Helper()

	gotService, err := fixture.service.GetUnreadCount(context.Background(), roomID, userID)
	require.NoError(t, err)
	require.Equal(t, want, gotService)

	summary, err := fixture.handler.batchUnreadCounts(context.Background(), []uuid.UUID{roomID}, userID)
	require.NoError(t, err)
	require.Equal(t, want, summary[roomID])

	gotHTTP := getUnreadHTTPCount(t, router, roomID)
	require.Equal(t, want, gotHTTP)
}

func latestRoomUpdatedForRecipient(t *testing.T, inserts []unreadOutboxInsert, recipientID uuid.UUID) map[string]any {
	t.Helper()

	for i := len(inserts) - 1; i >= 0; i-- {
		insert := inserts[i]
		if insert.eventType != realtime.EventTypeChatRoomUpdated {
			continue
		}
		if insert.payload["recipient_id"] == recipientID.String() {
			return insert.payload
		}
	}

	t.Fatalf("no chat.room.updated event found for recipient %s", recipientID)
	return nil
}

func TestChatUnreadRuntimeClosure_PostgresBacked(t *testing.T) {
	ctx := context.Background()
	fixture := newUnreadIntegrationFixture(t)

	t.Run("visible incoming message and sender invariant", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)

		message, err := fixture.service.SendTextMessage(ctx, roomID, userB, "hello A", uuid.NewString())
		require.NoError(t, err)

		countA, err := fixture.service.GetUnreadCount(ctx, roomID, userA)
		require.NoError(t, err)
		require.Equal(t, 1, countA)

		countB, err := fixture.service.GetUnreadCount(ctx, roomID, userB)
		require.NoError(t, err)
		require.Equal(t, 0, countB)

		stateB, err := fixture.service.GetReadState(ctx, roomID, userB)
		require.NoError(t, err)
		require.WithinDuration(t, message.CreatedAt, stateB.LastReadAt, time.Microsecond, "sender cursor should advance to the message timestamp")

		events := fixture.outbox.snapshot()
		require.GreaterOrEqual(t, len(events), 3)

		recipientA := latestRoomUpdatedForRecipient(t, events, userA)
		require.Equal(t, 1, recipientA["unread_count"])
		recipientB := latestRoomUpdatedForRecipient(t, events, userB)
		require.Equal(t, 0, recipientB["unread_count"])
	})

	t.Run("sender read-state rollback is atomic", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)

		repo := &unreadUpsertFailureRepo{
			Repository: chatInfraRepo.NewChatRepository(),
			err:        errors.New("injected read-state failure"),
		}

		service := chatApp.NewService(
			fixture.appDB,
			repo,
			socialInfraRepo.NewSocialRepository(),
			&recordingUnreadOutbox{},
			rate.NewRateLimiter(),
			nil,
			nil,
			nil,
			zap.NewNop(),
		)

		_, err := service.SendTextMessage(ctx, roomID, userB, "must rollback", uuid.NewString())
		require.Error(t, err)

		var messageCount, readStateCount int
		require.NoError(t, fixture.appDB.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM chat_messages WHERE room_id = $1`, roomID).Scan(&messageCount))
		require.NoError(t, fixture.appDB.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM chat_read_states WHERE room_id = $1 AND user_id = $2`, roomID, userB).Scan(&readStateCount))
		require.Equal(t, 0, messageCount)
		require.Equal(t, 0, readStateCount)
	})

	t.Run("multiple unread messages and room summary parity", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)
		routerA := newUnreadAuthRouter(userA, fixture.handler)

		_, err := fixture.service.SendTextMessage(ctx, roomID, userB, "one", uuid.NewString())
		require.NoError(t, err)
		_, err = fixture.service.SendTextMessage(ctx, roomID, userB, "two", uuid.NewString())
		require.NoError(t, err)
		_, err = fixture.service.SendTextMessage(ctx, roomID, userB, "three", uuid.NewString())
		require.NoError(t, err)

		assertUnreadParity(t, fixture, routerA, roomID, userA, 3)
	})

	t.Run("mark read is idempotent and monotonic", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)
		routerA := newUnreadAuthRouter(userA, fixture.handler)

		_, err := fixture.service.SendTextMessage(ctx, roomID, userB, "one", uuid.NewString())
		require.NoError(t, err)
		_, err = fixture.service.SendTextMessage(ctx, roomID, userB, "two", uuid.NewString())
		require.NoError(t, err)

		markAt := time.Now().UTC().Truncate(time.Microsecond).Add(1 * time.Second)
		require.Equal(t, http.StatusOK, markReadHTTP(t, routerA, roomID, markAt))

		countAfterFirst, err := fixture.service.GetUnreadCount(ctx, roomID, userA)
		require.NoError(t, err)
		require.Equal(t, 0, countAfterFirst)

		stateAfterFirst, err := fixture.service.GetReadState(ctx, roomID, userA)
		require.NoError(t, err)
		require.WithinDuration(t, markAt, stateAfterFirst.LastReadAt, time.Microsecond)

		eventsBeforeRepeat := len(fixture.outbox.snapshot())
		require.Equal(t, http.StatusOK, markReadHTTP(t, routerA, roomID, markAt))
		eventsAfterRepeat := len(fixture.outbox.snapshot())
		require.Equal(t, eventsBeforeRepeat, eventsAfterRepeat)

		stateAfterRepeat, err := fixture.service.GetReadState(ctx, roomID, userA)
		require.NoError(t, err)
		require.WithinDuration(t, markAt, stateAfterRepeat.LastReadAt, time.Microsecond)

		assertUnreadParity(t, fixture, routerA, roomID, userA, 0)
	})

	t.Run("hidden before read exits unread immediately", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		moderator := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)
		routerA := newUnreadAuthRouter(userA, fixture.handler)

		message, err := fixture.service.SendTextMessage(ctx, roomID, userB, "hide me", uuid.NewString())
		require.NoError(t, err)
		require.Equal(t, 1, mustGetUnreadCountViaService(t, fixture.service, ctx, roomID, userA))

		err = fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
			return fixture.service.SoftHideForModeration(ctx, tx, message.ID, moderator, "moderation hide", "hide-before-read")
		})
		require.NoError(t, err)

		assertUnreadParity(t, fixture, routerA, roomID, userA, 0)

		recipientA := latestRoomUpdatedForRecipient(t, fixture.outbox.snapshot(), userA)
		require.Equal(t, 0, recipientA["unread_count"])
	})

	t.Run("hidden after read keeps the cursor monotonic", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		moderator := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)
		routerA := newUnreadAuthRouter(userA, fixture.handler)

		message, err := fixture.service.SendTextMessage(ctx, roomID, userB, "read then hide", uuid.NewString())
		require.NoError(t, err)

		markAt := message.CreatedAt.Add(1 * time.Microsecond)
		require.Equal(t, http.StatusOK, markReadHTTP(t, routerA, roomID, markAt))

		stateBeforeHide, err := fixture.service.GetReadState(ctx, roomID, userA)
		require.NoError(t, err)

		err = fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
			return fixture.service.SoftHideForModeration(ctx, tx, message.ID, moderator, "moderation hide", "hide-after-read")
		})
		require.NoError(t, err)

		countAfterHide, err := fixture.service.GetUnreadCount(ctx, roomID, userA)
		require.NoError(t, err)
		require.Equal(t, 0, countAfterHide)

		stateAfterHide, err := fixture.service.GetReadState(ctx, roomID, userA)
		require.NoError(t, err)
		require.WithinDuration(t, stateBeforeHide.LastReadAt, stateAfterHide.LastReadAt, time.Microsecond)
	})

	t.Run("restore boundary respects last_read_at equality", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		moderator := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)
		routerA := newUnreadAuthRouter(userA, fixture.handler)

		boundary := time.Date(2026, 7, 14, 10, 0, 0, 123456000, time.UTC)
		upsertUnreadTestReadState(t, ctx, fixture.appDB, roomID, userA, boundary)
		messageID := insertUnreadTestMessage(t, ctx, fixture.appDB, roomID, userB, "boundary", boundary)

		require.Equal(t, 0, mustGetUnreadCountViaService(t, fixture.service, ctx, roomID, userA))

		err := fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
			return fixture.service.SoftHideForModeration(ctx, tx, messageID, moderator, "boundary hide", "boundary-hide")
		})
		require.NoError(t, err)

		err = fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
			return fixture.service.RestoreFromModeration(ctx, tx, messageID, "boundary-restore")
		})
		require.NoError(t, err)

		assertUnreadParity(t, fixture, routerA, roomID, userA, 0)
	})

	t.Run("restore after cursor boundary re-enters unread", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		moderator := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)
		routerA := newUnreadAuthRouter(userA, fixture.handler)

		lastReadAt := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
		upsertUnreadTestReadState(t, ctx, fixture.appDB, roomID, userA, lastReadAt)
		messageID := insertUnreadTestMessage(t, ctx, fixture.appDB, roomID, userB, "later", lastReadAt.Add(1*time.Microsecond))

		err := fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
			return fixture.service.SoftHideForModeration(ctx, tx, messageID, moderator, "later hide", "later-hide")
		})
		require.NoError(t, err)
		require.Equal(t, 0, mustGetUnreadCountViaService(t, fixture.service, ctx, roomID, userA))

		err = fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
			return fixture.service.RestoreFromModeration(ctx, tx, messageID, "later-restore")
		})
		require.NoError(t, err)

		assertUnreadParity(t, fixture, routerA, roomID, userA, 1)
	})

	t.Run("other-room isolation", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		userC := insertUnreadTestUser(t, ctx, fixture.appDB)
		userD := insertUnreadTestUser(t, ctx, fixture.appDB)

		targetRoom := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)
		unrelatedRoom := insertUnreadTestRoom(t, ctx, fixture.appDB, userC, userD)
		routerA := newUnreadAuthRouter(userA, fixture.handler)

		_, err := fixture.service.SendTextMessage(ctx, unrelatedRoom, userC, "noise", uuid.NewString())
		require.NoError(t, err)

		assertUnreadParity(t, fixture, routerA, targetRoom, userA, 0)
	})

	t.Run("authorization guards unread and read-state writes", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		outsider := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)

		_, err := fixture.service.SendTextMessage(ctx, roomID, userB, "secret", uuid.NewString())
		require.NoError(t, err)

		routerOutsider := newUnreadAuthRouter(outsider, fixture.handler)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/rooms/"+roomID.String()+"/read",
			strings.NewReader(fmt.Sprintf(`{"timestamp":"%s"}`, time.Now().UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano))))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		routerOutsider.ServeHTTP(w, req)
		require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

		req = httptest.NewRequest(http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/unread", nil)
		w = httptest.NewRecorder()
		routerOutsider.ServeHTTP(w, req)
		require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

		var readStateCount int
		require.NoError(t, fixture.appDB.Pool().QueryRow(ctx, `
			SELECT COUNT(*)
			FROM chat_read_states
			WHERE room_id = $1 AND user_id = $2
		`, roomID, outsider).Scan(&readStateCount))
		require.Equal(t, 0, readStateCount)
	})

	t.Run("deterministic commit ordering", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)

		roomA := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)
		routerA := newUnreadAuthRouter(userA, fixture.handler)
		_, err := fixture.service.SendTextMessage(ctx, roomA, userB, "committed first", uuid.NewString())
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, markReadHTTP(t, routerA, roomA, time.Now().UTC().Add(1*time.Second)))
		assertUnreadParity(t, fixture, routerA, roomA, userA, 0)

		userC := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomB := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userC)
		routerB := newUnreadAuthRouter(userA, fixture.handler)
		markAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Microsecond)
		require.Equal(t, http.StatusOK, markReadHTTP(t, routerB, roomB, markAt))
		_, err = fixture.service.SendTextMessage(ctx, roomB, userC, "arrives later", uuid.NewString())
		require.NoError(t, err)
		assertUnreadParity(t, fixture, routerB, roomB, userA, 1)
	})

	t.Run("concurrent hide and mark-read interleaving", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		moderator := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)
		routerA := newUnreadAuthRouter(userA, fixture.handler)

		message, err := fixture.service.SendTextMessage(ctx, roomID, userB, "interleave", uuid.NewString())
		require.NoError(t, err)

		hideTx, err := fixture.appDB.BeginTx(ctx)
		require.NoError(t, err)
		defer hideTx.Rollback(ctx)

		hideDone := make(chan error, 1)
		go func() {
			hideDone <- fixture.service.SoftHideForModeration(ctx, hideTx, message.ID, moderator, "interleave hide", "interleave-hide")
		}()
		require.NoError(t, <-hideDone)

		markAt := message.CreatedAt.Add(1 * time.Microsecond)
		require.Equal(t, http.StatusOK, markReadHTTP(t, routerA, roomID, markAt))
		require.NoError(t, hideTx.Commit(ctx))

		first, err := fixture.service.GetUnreadCount(ctx, roomID, userA)
		require.NoError(t, err)
		second, err := fixture.service.GetUnreadCount(ctx, roomID, userA)
		require.NoError(t, err)
		require.Equal(t, 0, first)
		require.Equal(t, 0, second)

		stateAfter, err := fixture.service.GetReadState(ctx, roomID, userA)
		require.NoError(t, err)
		require.WithinDuration(t, markAt, stateAfter.LastReadAt, time.Microsecond)

		assertUnreadParity(t, fixture, routerA, roomID, userA, 0)
	})

	t.Run("postgres timestamp boundary uses microsecond precision", func(t *testing.T) {
		userA := insertUnreadTestUser(t, ctx, fixture.appDB)
		userB := insertUnreadTestUser(t, ctx, fixture.appDB)
		roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)

		lastReadAt := time.Date(2026, 7, 14, 12, 0, 0, 987654000, time.UTC)
		upsertUnreadTestReadState(t, ctx, fixture.appDB, roomID, userA, lastReadAt)

		insertUnreadTestMessage(t, ctx, fixture.appDB, roomID, userB, "boundary", lastReadAt)
		require.Equal(t, 0, mustGetUnreadCountViaService(t, fixture.service, ctx, roomID, userA))

		insertUnreadTestMessage(t, ctx, fixture.appDB, roomID, userB, "after", lastReadAt.Add(1*time.Microsecond))
		require.Equal(t, 1, mustGetUnreadCountViaService(t, fixture.service, ctx, roomID, userA))
	})
}

// TestChatModerationLifecycleRuntimeClosure proves the chat runtime projection
// stays safe across moderation hide/restore transitions: unread state drops
// when a message is hidden, hidden content is not exposed through the chat API,
// and restored messages become visible again without reopening authorization.
func TestChatModerationLifecycleRuntimeClosure(t *testing.T) {
	ctx := context.Background()
	fixture := newUnreadIntegrationFixture(t)

	userA := insertUnreadTestUser(t, ctx, fixture.appDB)
	userB := insertUnreadTestUser(t, ctx, fixture.appDB)
	moderator := insertUnreadTestUser(t, ctx, fixture.appDB)
	roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)
	routerA := newChatHTTPRouter(userA, fixture.handler)

	message, err := fixture.service.SendTextMessage(ctx, roomID, userB, "moderation runtime body", uuid.NewString())
	require.NoError(t, err)
	require.Equal(t, 1, mustGetUnreadCountViaService(t, fixture.service, ctx, roomID, userA))

	err = fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
		return fixture.service.SoftHideForModeration(ctx, tx, message.ID, moderator, "moderation hide", "runtime-hide")
	})
	require.NoError(t, err)

	require.Equal(t, 0, mustGetUnreadCountViaService(t, fixture.service, ctx, roomID, userA))

	status, chatResp := getChatMessagesHTTPResponse(t, routerA, roomID)
	require.Equal(t, http.StatusOK, status)
	chatData, ok := chatResp["data"].(map[string]any)
	require.True(t, ok)
	messages, ok := chatData["data"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)

	hiddenMessage := messages[0].(map[string]any)
	require.Equal(t, true, hiddenMessage["is_hidden"])
	_, hasBody := hiddenMessage["body"]
	_, hasAttachment := hiddenMessage["attachment_json"]
	_, hasSender := hiddenMessage["sender"]
	require.False(t, hasBody)
	require.False(t, hasAttachment)
	require.False(t, hasSender)

	routerOutsider := newChatHTTPRouter(uuid.New(), fixture.handler)
	outsiderStatus, _ := getChatMessagesHTTPResponse(t, routerOutsider, roomID)
	require.Equal(t, http.StatusForbidden, outsiderStatus)

	err = fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
		return fixture.service.RestoreFromModeration(ctx, tx, message.ID, "runtime-restore")
	})
	require.NoError(t, err)

	require.Equal(t, 1, mustGetUnreadCountViaService(t, fixture.service, ctx, roomID, userA))
}

// TestChatRemovedSenderProjectionRuntimeClosure_PostgresBacked proves a soft-
// deleted sender still appears in room history with a coarsened removed
// lifecycle and without exposing private sender fields.
func TestChatRemovedSenderProjectionRuntimeClosure_PostgresBacked(t *testing.T) {
	ctx := context.Background()
	fixture := newUnreadIntegrationFixture(t)

	senderID := insertUnreadTestUser(t, ctx, fixture.appDB)
	recipientID := insertUnreadTestUser(t, ctx, fixture.appDB)
	roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, senderID, recipientID)
	router := newChatHTTPRouter(recipientID, fixture.handler)

	textBody := "sender history body"
	textMessageID := insertUnreadTestMessage(
		t,
		ctx,
		fixture.appDB,
		roomID,
		senderID,
		textBody,
		time.Now().UTC().Truncate(time.Microsecond),
	)
	attachmentBody := "sender attachment body"
	attachmentMessageID := insertUnreadTestMessageWithAttachment(
		t,
		ctx,
		fixture.appDB,
		roomID,
		senderID,
		&attachmentBody,
		`{"type":"image","data":{"url":"private://removed-sender.png","filename":"removed-sender.png"}}`,
		time.Now().UTC().Add(1*time.Microsecond).Truncate(time.Microsecond),
	)

	_, err := fixture.appDB.Pool().Exec(ctx, `
		UPDATE users
		SET deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`, senderID)
	require.NoError(t, err)

	status, chatResp := getChatMessagesHTTPResponse(t, router, roomID)
	require.Equal(t, http.StatusOK, status)

	chatData, ok := chatResp["data"].(map[string]any)
	require.True(t, ok)
	messages, ok := chatData["data"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)

	first := messages[0].(map[string]any)
	second := messages[1].(map[string]any)

	assertRemovedSenderMessage := func(msg map[string]any, wantID string) {
		t.Helper()
		require.Equal(t, wantID, msg["id"])
		require.Equal(t, senderID.String(), msg["sender_id"])
		sender, ok := msg["sender"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "removed", sender["lifecycle"])
		require.Equal(t, senderID.String(), sender["id"])
		require.NotEmpty(t, sender["username"])
		_, hasEmail := sender["email"]
		require.False(t, hasEmail)
		_, hasPhone := sender["phone"]
		require.False(t, hasPhone)
	}

	assertRemovedSenderMessage(first, attachmentMessageID.String())
	_, hasAttachment := first["attachment_json"].(map[string]any)
	require.True(t, hasAttachment)
	require.Equal(t, attachmentBody, first["body"])
	_, hasHidden := first["is_hidden"]
	require.False(t, hasHidden)

	assertRemovedSenderMessage(second, textMessageID.String())
	require.Equal(t, textBody, second["body"])
	_, hasAttachment = second["attachment_json"]
	require.False(t, hasAttachment)
	_, hasHidden = second["is_hidden"]
	require.False(t, hasHidden)
}

func mustGetUnreadCountViaService(t *testing.T, service *chatApp.Service, ctx context.Context, roomID, userID uuid.UUID) int {
	t.Helper()

	count, err := service.GetUnreadCount(ctx, roomID, userID)
	require.NoError(t, err)
	return count
}
