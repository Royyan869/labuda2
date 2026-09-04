//go:build integration

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/internal/identity/auth"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	contentEntity "github.com/labuda/backend/internal/social/content/entity"
	contentRepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/platform/events"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	socialrepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// mentionRoundtripFixture holds production-composition dependencies for the
// full lifecycle test. No mocks — real DB, real repos, real worker handler.
type mentionRoundtripFixture struct {
	appDB         *db.DB
	contentSvc    *contentApp.ContentService
	workerHandler *NotificationEventHandler
}

func setupMentionRoundtripFixture(t *testing.T) *mentionRoundtripFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())

	contentRepository := contentRepo.NewContentRepository()
	outboxRepository := outboxRepo.NewOutboxRepository(appDB)
	blockChecker := &mentionRoundtripBlockChecker{
		db:   appDB,
		repo: socialrepo.NewSocialRepository(),
	}
	accountStatusChecker := auth.NewAccountStatusCheckerDB(appDB)

	contentService := contentApp.NewContentService(
		contentRepository,
		nil,          // likeRepo — not needed
		nil,          // roleChecker — not needed for create
		accountStatusChecker,
		nil, // invariantLogger
	)
	contentService.SetOutboxInserter(outboxRepository)

	workerHandler := NewNotificationEventHandler(
		appDB,
		blockChecker,
		NewNotificationServiceInserter(),
		nil, // pushSender — not needed
		accountStatusChecker,
		zaptest.NewLogger(t),
	)

	return &mentionRoundtripFixture{
		appDB:         appDB,
		contentSvc:    contentService,
		workerHandler: workerHandler,
	}
}

// --- Test doubles ---

type mentionRoundtripBlockChecker struct {
	db   *db.DB
	repo interface {
		ExistsBlock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)
	}
}

func (b *mentionRoundtripBlockChecker) ExistsBlock(ctx context.Context, userA, userB uuid.UUID) (bool, error) {
	var blocked bool
	err := b.db.WithTx(ctx, func(tx db.Tx) error {
		var innerErr error
		blocked, innerErr = b.repo.ExistsBlock(ctx, tx, userA, userB)
		return innerErr
	})
	return blocked, err
}

// --- Helpers ---

func insertRoundtripUser(t *testing.T, ctx context.Context, pool *db.DB, username string) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, phone_verified, account_status,
			email_verified_at, created_at, updated_at, role
		)
		VALUES ($1, $2, $3, true, 'active', NOW(), $4, $4, 'user')
	`, userID, userID.String(), fmt.Sprintf("%s-%s@test.invalid", username, userID.String()), now)
	require.NoError(t, err)

	_, err = pool.Pool().Exec(ctx, `
		INSERT INTO user_profiles (user_id, username, avatar_url, created_at, updated_at)
		VALUES ($1, $2, NULL, $3, $3)
	`, userID, fmt.Sprintf("%s-%s", username, userID.String()), now)
	require.NoError(t, err)

	return userID
}

// ======================================================================
// TEST: Full DB roundtrip
// Create Content → Mention → Outbox → Worker → Notification
// ======================================================================

func TestMentionLifecycle_DBRoundtrip(t *testing.T) {
	fixture := setupMentionRoundtripFixture(t)
	ctx := context.Background()

	// --- SETUP: Create two users ---
	authorID := insertRoundtripUser(t, ctx, fixture.appDB, "author")
	mentionedID := insertRoundtripUser(t, ctx, fixture.appDB, "mentioned")

	// --- STEP 1: Create content with mention through canonical service ---
	var contentID uuid.UUID

	err := fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
		content, err := fixture.contentSvc.CreateContent(
			ctx,
			tx,
			authorID,
			"Hello @mentioned user!",
			contentEntity.VisibilityPublic,
			nil, nil, nil, nil,
			[]uuid.UUID{mentionedID},
		)
		if err != nil {
			return fmt.Errorf("CreateContent failed: %w", err)
		}
		contentID = content.ID
		return nil
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, contentID)

	t.Logf("Content ID:       %s", contentID)
	t.Logf("Author ID:        %s", authorID)
	t.Logf("Mentioned User ID: %s", mentionedID)

	// --- ASSERT PERSISTENCE: content_mentioned_users ---
	var mentionCount int
	err = fixture.appDB.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM content_mentioned_users WHERE content_id = $1 AND user_id = $2`,
		contentID, mentionedID,
	).Scan(&mentionCount)
	require.NoError(t, err)
	require.Equal(t, 1, mentionCount,
		"content_mentioned_users must contain (contentID, mentionedID)")
	t.Log("✅ content_mentioned_users persisted")

	// --- ASSERT OUTBOX: content.mentioned event ---
	var outboxEventType string
	var outboxPayload []byte
	var outboxIdempotencyKey string
	err = fixture.appDB.Pool().QueryRow(ctx,
		`SELECT event_type, payload, idempotency_key
		 FROM outbox
		 WHERE event_type = $1
		   AND idempotency_key LIKE $2
		 ORDER BY created_at DESC LIMIT 1`,
		events.EventContentMentioned,
		fmt.Sprintf("content.mentioned.content.mentioned.%s.%%", contentID.String()),
	).Scan(&outboxEventType, &outboxPayload, &outboxIdempotencyKey)
	require.NoError(t, err)
	require.Equal(t, events.EventContentMentioned, outboxEventType)

	// Parse the outbox payload
	var outboxPayloadParsed map[string]string
	err = json.Unmarshal(outboxPayload, &outboxPayloadParsed)
	require.NoError(t, err)
	require.Equal(t, contentID.String(), outboxPayloadParsed["content_id"])
	require.Equal(t, authorID.String(), outboxPayloadParsed["author_id"])
	require.Equal(t, mentionedID.String(), outboxPayloadParsed["mentioned_user_id"])

	t.Logf("✅ outbox event: type=%s idempotencyKey=%s", outboxEventType, outboxIdempotencyKey)
	t.Logf("   content_id=%s author_id=%s mentioned_user_id=%s",
		outboxPayloadParsed["content_id"],
		outboxPayloadParsed["author_id"],
		outboxPayloadParsed["mentioned_user_id"])

	// --- STEP 2: Process through canonical worker handler ---
	workerEvent := event.OutboxEvent{
		ID:        uuid.New(),
		EventType: events.EventContentMentioned,
		Payload:   outboxPayload,
	}

	err = fixture.workerHandler.Handle(ctx, workerEvent)
	require.NoError(t, err, "worker Handle must succeed")
	t.Log("✅ worker processed content.mentioned event")

	// --- ASSERT NOTIFICATION ---
	var notifRecipientID, notifActorID uuid.UUID
	var notifType string
	var notifEntityID uuid.UUID
	var notifData map[string]interface{}

	err = fixture.appDB.Pool().QueryRow(ctx,
		`SELECT recipient_id, actor_id, type, entity_id, data
		 FROM notifications
		 WHERE type = $1
		   AND entity_id = $2
		   AND actor_id = $3
		 ORDER BY created_at DESC LIMIT 1`,
		events.EventContentMentioned,
		contentID,
		authorID,
	).Scan(&notifRecipientID, &notifActorID, &notifType, &notifEntityID, &notifData)
	require.NoError(t, err, "notification row must exist")

	// Verify canonical notification contract
	require.Equal(t, mentionedID, notifRecipientID,
		"notification recipient must be the mentioned user")
	require.Equal(t, authorID, notifActorID,
		"notification actor must be the content author")
	require.Equal(t, events.EventContentMentioned, notifType,
		"notification type must be content.mentioned")
	require.Equal(t, contentID, notifEntityID,
		"notification entity_id must be the content ID")

	// Canonical data keys
	require.NotNil(t, notifData)
	require.Equal(t, contentID.String(), notifData["targetId"],
		"data[targetId] must be content ID")
	require.Equal(t, "content", notifData["targetType"],
		"data[targetType] must be 'content'")

	// No obsolete keys
	_, hasForSaleId := notifData["forSaleId"]
	_, hasAuctionId := notifData["auctionId"]
	_, hasContentId := notifData["contentId"]
	require.False(t, hasForSaleId, "no obsolete key 'forSaleId'")
	require.False(t, hasAuctionId, "no obsolete key 'auctionId'")
	require.False(t, hasContentId, "no old key 'contentId' — must use 'targetId'")

	t.Log("✅ notification persisted with canonical contract")

	// --- RUNTIME VALUES ---
	t.Log("")
	t.Log("=== RUNTIME VALUES ===")
	t.Logf("contentID:       %s", contentID)
	t.Logf("authorID:        %s", authorID)
	t.Logf("mentionedUserID: %s", mentionedID)
	t.Logf("eventType:       %s", events.EventContentMentioned)
	t.Logf("idempotencyKey:  %s", outboxIdempotencyKey)
	t.Logf("notifType:       %s", notifType)
	t.Logf("targetId:        %s", notifData["targetId"])
	t.Logf("targetType:      %s", notifData["targetType"])
	t.Log("=== END RUNTIME VALUES ===")
}

// ======================================================================
// TEST: Self-mention produces no notification
// ======================================================================

func TestMentionLifecycle_SelfMention_NoNotification(t *testing.T) {
	fixture := setupMentionRoundtripFixture(t)
	ctx := context.Background()

	userA := insertRoundtripUser(t, ctx, fixture.appDB, "selfauthor")

	var contentID uuid.UUID
	err := fixture.appDB.WithTx(ctx, func(tx db.Tx) error {
		content, err := fixture.contentSvc.CreateContent(
			ctx, tx, userA,
			"Self-mention test",
			contentEntity.VisibilityPublic,
			nil, nil, nil, nil,
			[]uuid.UUID{userA},
		)
		if err != nil {
			return err
		}
		contentID = content.ID
		return nil
	})
	require.NoError(t, err)

	// Mention persisted in DB
	var mentionCount int
	err = fixture.appDB.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM content_mentioned_users WHERE content_id = $1 AND user_id = $2`,
		contentID, userA,
	).Scan(&mentionCount)
	require.NoError(t, err)
	require.Equal(t, 1, mentionCount)

	// Outbox event exists
	var outboxCount int
	err = fixture.appDB.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM outbox WHERE event_type = $1 AND idempotency_key LIKE $2`,
		events.EventContentMentioned,
		fmt.Sprintf("content.mentioned.content.mentioned.%s.%%", contentID.String()),
	).Scan(&outboxCount)
	require.NoError(t, err)
	require.Equal(t, 1, outboxCount)

	// Read payload and process through worker
	var payload []byte
	err = fixture.appDB.Pool().QueryRow(ctx,
		`SELECT payload FROM outbox WHERE event_type = $1 AND idempotency_key LIKE $2 LIMIT 1`,
		events.EventContentMentioned,
		fmt.Sprintf("content.mentioned.content.mentioned.%s.%%", contentID.String()),
	).Scan(&payload)
	require.NoError(t, err)

	err = fixture.workerHandler.Handle(ctx, event.OutboxEvent{
		ID: uuid.New(), EventType: events.EventContentMentioned, Payload: payload,
	})
	require.NoError(t, err)

	// No notification
	var notifCount int
	err = fixture.appDB.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE type = $1 AND entity_id = $2`,
		events.EventContentMentioned, contentID,
	).Scan(&notifCount)
	require.NoError(t, err)
	require.Equal(t, 0, notifCount, "self-mention must NOT create notification")

	t.Log("✅ self-mention: DB persisted, outbox exists, worker skips notification")
}
