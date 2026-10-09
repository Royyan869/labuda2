//go:build integration

package http

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatInfraRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	notificationpkg "github.com/labuda/backend/internal/interaction/notification"
	notificationEntity "github.com/labuda/backend/internal/interaction/notification/entity"
	notificationRepoImpl "github.com/labuda/backend/internal/interaction/notification/infrastructure/repository"
	socialInfraRepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TASK 3 — CHAT READ COMPLETES CHAT NOTIFICATIONS (one transaction).
//
// Proves that POST /chat/rooms/:id/read (service MarkAsRead) marks ONLY the
// chat_message notifications of the room being read, atomically with the
// chat read-state upsert:
//   - other rooms' chat notifications stay unread;
//   - non-chat notifications stay unread;
//   - a failing notification sync rolls back the chat read state too.

type notifSyncFixture struct {
	tdb       *testdb.TestDB
	appDB     *db.DB
	service   *chatApp.Service
	chatRepo  chatRepo.Repository
	notifRepo notificationpkg.Repository
	syncer    *notificationpkg.ChatRoomReadSyncer

	recipient uuid.UUID // u1 — the viewer who reads rooms
	senderA   uuid.UUID // u2 — other participant of room1
	senderB   uuid.UUID // u3 — other participant of room2

	room1 uuid.UUID // u1 <-> u2
	room2 uuid.UUID // u1 <-> u3
}

type notifSyncRecordingOutbox struct{}

func (notifSyncRecordingOutbox) InsertTx(context.Context, db.Tx, string, any, string) error {
	return nil
}

// notifSyncFailingSyncer forces the notification half of the combined
// operation to fail so rollback of the chat half can be proven.
type notifSyncFailingSyncer struct {
	err error
}

func (f *notifSyncFailingSyncer) MarkChatRoomNotificationsRead(
	context.Context, db.Tx, uuid.UUID, uuid.UUID,
) error {
	return f.err
}

func newNotifSyncFixture(t *testing.T) *notifSyncFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())
	chatRepository := chatInfraRepo.NewChatRepository()
	notifRepository := notificationRepoImpl.NewNotificationRepository()
	syncer := &notificationpkg.ChatRoomReadSyncer{Repo: notifRepository}

	service := chatApp.NewService(
		appDB,
		chatRepository,
		socialInfraRepo.NewSocialRepository(),
		notifSyncRecordingOutbox{},
		rate.NewRateLimiter(),
		nil,
		nil,
		nil,
		zap.NewNop(),
	)
	service.SetChatNotificationReadSyncer(syncer)

	f := &notifSyncFixture{
		tdb:       tdb,
		appDB:     appDB,
		service:   service,
		chatRepo:  chatRepository,
		notifRepo: notifRepository,
		syncer:    syncer,
		recipient: notifSyncSeedUser(t, appDB),
		senderA:   notifSyncSeedUser(t, appDB),
		senderB:   notifSyncSeedUser(t, appDB),
	}
	f.room1 = notifSyncSeedRoom(t, appDB, f.recipient, f.senderA)
	f.room2 = notifSyncSeedRoom(t, appDB, f.recipient, f.senderB)
	return f
}

func notifSyncSeedUser(t *testing.T, appDB *db.DB) uuid.UUID {
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

func notifSyncSeedRoom(t *testing.T, appDB *db.DB, a, b uuid.UUID) uuid.UUID {
	t.Helper()

	if a.String() > b.String() {
		a, b = b, a
	}
	roomID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := appDB.Pool().Exec(context.Background(), `
		INSERT INTO chat_rooms (
			id, room_type, participant_a, participant_b, created_at, updated_at, last_message_at
		)
		VALUES ($1, 'direct', $2, $3, $4, $4, $4)
	`, roomID, a, b, now)
	require.NoError(t, err)
	return roomID
}

func notifSyncSeedMessage(t *testing.T, appDB *db.DB, roomID, senderID uuid.UUID) uuid.UUID {
	t.Helper()

	messageID := uuid.New()
	body := "hello"
	attachment := "{}"
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := appDB.Pool().Exec(context.Background(), `
		INSERT INTO chat_messages (
			id, room_id, sender_id, message_type, body, attachment_json,
			idempotency_key, command_fingerprint, created_at
		)
		VALUES ($1, $2, $3, 'text', $4, $5::jsonb, $6, $7, $8)
	`,
		messageID, roomID, senderID, body, attachment,
		uuid.NewString(),
		chatEntity.ComputeCommandFingerprint(senderID, chatEntity.MessageTypeText, &body, map[string]interface{}{}, nil),
		now,
	)
	require.NoError(t, err)
	return messageID
}

// notifSyncSeedChatNotification writes the exact row the production producer
// (handleChatMessage) writes: type=chat_message, entity_id=room.
func notifSyncSeedChatNotification(
	t *testing.T,
	f *notifSyncFixture,
	recipientID, senderID, roomID uuid.UUID,
) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := f.appDB.WithTx(context.Background(), func(tx db.Tx) error {
		var insertErr error
		id, _, insertErr = f.notifRepo.Insert(context.Background(), tx,
			notificationEntity.NewNotification(
				recipientID,
				notificationEntity.UserActor(senderID),
				notificationEntity.TypeChatMessage,
				roomID,
				map[string]interface{}{},
			),
		)
		return insertErr
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)
	return id
}

func notifSyncSeedNonChatNotification(
	t *testing.T,
	f *notifSyncFixture,
	recipientID, actorID uuid.UUID,
) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := f.appDB.WithTx(context.Background(), func(tx db.Tx) error {
		var insertErr error
		id, _, insertErr = f.notifRepo.Insert(context.Background(), tx,
			notificationEntity.NewNotification(
				recipientID,
				notificationEntity.UserActor(actorID),
				notificationEntity.TypeOrderCreated,
				uuid.New(), // order id — unrelated entity
				map[string]interface{}{},
			),
		)
		return insertErr
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)
	return id
}

func notifSyncIsRead(t *testing.T, f *notifSyncFixture, notificationID uuid.UUID) bool {
	t.Helper()

	var isRead bool
	err := f.appDB.Pool().QueryRow(context.Background(),
		"SELECT is_read FROM notifications WHERE id = $1", notificationID,
	).Scan(&isRead)
	require.NoError(t, err)
	return isRead
}

func notifSyncHasReadState(t *testing.T, f *notifSyncFixture, roomID, userID uuid.UUID) bool {
	t.Helper()

	var exists bool
	err := f.appDB.Pool().QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM chat_read_states WHERE room_id = $1 AND user_id = $2)",
		roomID, userID,
	).Scan(&exists)
	require.NoError(t, err)
	return exists
}

// TestChatRead_CompletesOnlyThatRoomsChatNotifications covers Cases 1-3:
// reading room1 marks room1's chat notification read, while room2's chat
// notification and the non-chat notification stay unread.
func TestChatRead_CompletesOnlyThatRoomsChatNotifications(t *testing.T) {
	f := newNotifSyncFixture(t)
	ctx := context.Background()

	notifSyncSeedMessage(t, f.appDB, f.room1, f.senderA)
	notifSyncSeedMessage(t, f.appDB, f.room2, f.senderB)

	notifRoom1 := notifSyncSeedChatNotification(t, f, f.recipient, f.senderA, f.room1)
	notifRoom2 := notifSyncSeedChatNotification(t, f, f.recipient, f.senderB, f.room2)
	notifPayment := notifSyncSeedNonChatNotification(t, f, f.recipient, f.senderA)

	require.False(t, notifSyncIsRead(t, f, notifRoom1))
	require.False(t, notifSyncIsRead(t, f, notifRoom2))
	require.False(t, notifSyncIsRead(t, f, notifPayment))

	readAt := time.Now().UTC().Add(time.Minute)
	require.NoError(t, f.service.MarkAsRead(ctx, f.room1, f.recipient, readAt))

	// Chat authority: read state advanced for room1.
	require.True(t, notifSyncHasReadState(t, f, f.room1, f.recipient))

	// Case 1: room1's chat notification completed.
	require.True(t, notifSyncIsRead(t, f, notifRoom1),
		"the read room's chat notification must be marked read")

	// Case 2: the OTHER room's chat notification stays unread.
	require.False(t, notifSyncIsRead(t, f, notifRoom2),
		"a different room's chat notification must stay unread")

	// Case 3: the non-chat notification stays unread.
	require.False(t, notifSyncIsRead(t, f, notifPayment),
		"a non-chat notification must stay unread")
}

// TestChatRead_CompletesEveryChatNotificationForTheRoom (Case 4): the chat
// read semantic is ROOM-level, so every chat_message notification for
// (recipient, room) completes — regardless of which actor produced it.
// (The producer's per-(sender,room) dedup usually caps this at one row; the
// sync must still be room-scoped, not row-scoped.)
func TestChatRead_CompletesEveryChatNotificationForTheRoom(t *testing.T) {
	f := newNotifSyncFixture(t)
	ctx := context.Background()

	notifSyncSeedMessage(t, f.appDB, f.room1, f.senderA)

	notifFromSender := notifSyncSeedChatNotification(t, f, f.recipient, f.senderA, f.room1)

	// Second chat_message row for the SAME room under a different actor key.
	var notifFromSystem uuid.UUID
	err := f.appDB.WithTx(ctx, func(tx db.Tx) error {
		id, _, insertErr := f.notifRepo.Insert(ctx, tx,
			notificationEntity.NewNotification(
				f.recipient,
				notificationEntity.SystemActor(),
				notificationEntity.TypeChatMessage,
				f.room1,
				map[string]interface{}{},
			),
		)
		notifFromSystem = id
		return insertErr
	})
	require.NoError(t, err)
	require.False(t, notifSyncIsRead(t, f, notifFromSender))
	require.False(t, notifSyncIsRead(t, f, notifFromSystem))

	require.NoError(t, f.service.MarkAsRead(ctx, f.room1, f.recipient, time.Now().UTC().Add(time.Minute)))

	require.True(t, notifSyncIsRead(t, f, notifFromSender))
	require.True(t, notifSyncIsRead(t, f, notifFromSystem),
		"every chat_message notification of the read room must complete")
}

// TestChatRead_NotificationSyncFailureRollsBackChatRead (Case 5): one failed
// combined operation leaves BOTH authorities unchanged — no partial commit.
func TestChatRead_NotificationSyncFailureRollsBackChatRead(t *testing.T) {
	f := newNotifSyncFixture(t)
	ctx := context.Background()

	notifSyncSeedMessage(t, f.appDB, f.room1, f.senderA)
	notifRoom1 := notifSyncSeedChatNotification(t, f, f.recipient, f.senderA, f.room1)

	// Force the notification half to fail inside the same transaction.
	f.service.SetChatNotificationReadSyncer(&notifSyncFailingSyncer{
		err: errors.New("forced notification sync failure"),
	})

	readAt := time.Now().UTC().Add(time.Minute)
	err := f.service.MarkAsRead(ctx, f.room1, f.recipient, readAt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "forced notification sync failure")

	// Chat read state rolled back.
	require.False(t, notifSyncHasReadState(t, f, f.room1, f.recipient),
		"a failed combined operation must not leave chat read committed")

	// Notification read state rolled back (never marked).
	require.False(t, notifSyncIsRead(t, f, notifRoom1),
		"a failed combined operation must not leave the notification read")

	// Recovery: restore the real syncer — the retry succeeds atomically.
	f.service.SetChatNotificationReadSyncer(f.syncer)
	require.NoError(t, f.service.MarkAsRead(ctx, f.room1, f.recipient, readAt))
	require.True(t, notifSyncHasReadState(t, f, f.room1, f.recipient))
	require.True(t, notifSyncIsRead(t, f, notifRoom1))
}

// TestChatRead_NilSyncerKeepsChatOnlyBehavior: services wired without the
// syncer (existing tests, minimal embeddings) keep pure chat-read behavior —
// the optional dependency never breaks legacy wiring.
func TestChatRead_NilSyncerKeepsChatOnlyBehavior(t *testing.T) {
	f := newNotifSyncFixture(t)
	ctx := context.Background()

	notifSyncSeedMessage(t, f.appDB, f.room1, f.senderA)
	notifRoom1 := notifSyncSeedChatNotification(t, f, f.recipient, f.senderA, f.room1)

	f.service.SetChatNotificationReadSyncer(nil)
	require.NoError(t, f.service.MarkAsRead(ctx, f.room1, f.recipient, time.Now().UTC().Add(time.Minute)))

	require.True(t, notifSyncHasReadState(t, f, f.room1, f.recipient),
		"chat read must still work without the syncer")
	require.False(t, notifSyncIsRead(t, f, notifRoom1),
		"without the syncer the notification domain is untouched (legacy behavior)")
}
