//go:build integration

package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatInfraRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	socialInfraRepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newIdempotencyFixture creates a service wired with real PostgreSQL for
// idempotency security proofs. No mocks — all constraints exercised against
// the actual database.
func newIdempotencyFixture(t *testing.T) (*Service, *db.DB, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())
	repo := chatInfraRepo.NewChatRepository()
	svc := NewService(
		appDB,
		repo,
		socialInfraRepo.NewSocialRepository(),
		noopChatOutbox{},
		rate.NewRateLimiter(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		zap.NewNop(),
	)

	ctx := context.Background()
	senderA := insertChatAuthorityUser(t, ctx, appDB, "sender-a")
	senderB := insertChatAuthorityUser(t, ctx, appDB, "sender-b")
	room := insertChatAuthorityRoom(t, ctx, appDB, senderA, senderB)

	return svc, appDB, room, senderA, senderB
}

// countingOutbox is an OutboxInserter that delegates to a real DB insert
// so that outbox rows are queryable in tests.
type countingOutbox struct {
	db *db.DB
}

func (c *countingOutbox) InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
	fullKey := fmt.Sprintf("%s.%s", eventType, idempotencyKey)
	payloadJSON, _ := json.Marshal(payload)
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox (idempotency_key, event_type, aggregate_type, aggregate_id, payload, created_at, updated_at)
		VALUES ($1, $2, '', $3, $4::jsonb, NOW(), NOW())
		ON CONFLICT (idempotency_key) DO NOTHING
	`, fullKey, eventType, uuid.Nil, string(payloadJSON))
	return err
}

// countOutboxEvents returns the number of outbox rows matching a pattern.
func countOutboxEvents(t *testing.T, appDB *db.DB, eventTypePrefix string) int {
	t.Helper()
	var count int
	err := appDB.Pool().QueryRow(context.Background(),
		`SELECT COUNT(*) FROM outbox WHERE event_type LIKE $1`, eventTypePrefix+"%").Scan(&count)
	require.NoError(t, err)
	return count
}

// newIdempotencyFixtureWithOutbox is like newIdempotencyFixture but uses
// a real outbox inserter so events are queryable.
func newIdempotencyFixtureWithOutbox(t *testing.T) (*Service, *db.DB, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())
	repo := chatInfraRepo.NewChatRepository()
	outbox := &countingOutbox{db: appDB}
	svc := NewService(
		appDB,
		repo,
		socialInfraRepo.NewSocialRepository(),
		outbox,
		rate.NewRateLimiter(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		zap.NewNop(),
	)

	ctx := context.Background()
	senderA := insertChatAuthorityUser(t, ctx, appDB, "sender-a")
	senderB := insertChatAuthorityUser(t, ctx, appDB, "sender-b")
	room := insertChatAuthorityRoom(t, ctx, appDB, senderA, senderB)

	return svc, appDB, room, senderA, senderB
}

// sendText is a helper that sends a text message and fails the test on error.
func sendText(t *testing.T, svc *Service, roomID, senderID uuid.UUID, body string, idempotencyKey string) *chatEntity.ChatMessage {
	t.Helper()
	b := body
	msg, err := svc.SendMessage(context.Background(), roomID, senderID, chatEntity.MessageTypeText, &b, nil, nil, nil, idempotencyKey, nil)
	require.NoError(t, err)
	require.NotNil(t, msg)
	return msg
}

// sendTextExpectError sends a text message and asserts a specific error.
func sendTextExpectError(t *testing.T, svc *Service, roomID, senderID uuid.UUID, body string, idempotencyKey string, expectedErr error) {
	t.Helper()
	b := body
	_, err := svc.SendMessage(context.Background(), roomID, senderID, chatEntity.MessageTypeText, &b, nil, nil, nil, idempotencyKey, nil)
	require.ErrorIs(t, err, expectedErr)
}

// ============================================================================
// PROOF A: Same actor + same key + same command → stable replay
// ============================================================================
func TestIdempotency_SameActorSameKeySameCommand_StableReplay(t *testing.T) {
	svc, _, room, senderA, _ := newIdempotencyFixture(t)

	key := uuid.New().String()
	body := "hello"

	msg1 := sendText(t, svc, room, senderA, body, key)
	require.NotEmpty(t, msg1.ID)
	require.NotEmpty(t, msg1.CommandFingerprint)

	msg2 := sendText(t, svc, room, senderA, body, key)
	assert.Equal(t, msg1.ID, msg2.ID, "replay must return same message ID")
	assert.Equal(t, msg1.CommandFingerprint, msg2.CommandFingerprint)
	assert.Equal(t, *msg1.Body, *msg2.Body)
}

// ============================================================================
// PROOF B: Same actor + same key + exact same command — no duplicate message
// ============================================================================
func TestIdempotency_SameActorSameKey_NoDuplicateMessage(t *testing.T) {
	svc, _, room, senderA, _ := newIdempotencyFixture(t)

	key := uuid.New().String()
	body := "no-duplicates"

	msg1 := sendText(t, svc, room, senderA, body, key)
	msg2 := sendText(t, svc, room, senderA, body, key)

	assert.Equal(t, msg1.ID, msg2.ID)
	assert.Equal(t, msg1.CommandFingerprint, msg2.CommandFingerprint)
	assert.Equal(t, *msg1.Body, *msg2.Body)
}

// ============================================================================
// PROOF C: Same actor + same key + changed body → 409 conflict
// ============================================================================
func TestIdempotency_SameActorSameKey_ChangedBody_Conflict(t *testing.T) {
	svc, _, room, senderA, _ := newIdempotencyFixture(t)

	key := uuid.New().String()
	sendText(t, svc, room, senderA, "original body", key)
	sendTextExpectError(t, svc, room, senderA, "different body", key, chatRepo.ErrIdempotencyConflict)
}

// ============================================================================
// PROOF D: Same actor + same key + changed room → 409 conflict
// ============================================================================
func TestIdempotency_SameActorSameKey_ChangedRoom_Conflict(t *testing.T) {
	svc, appDB, room, senderA, senderB := newIdempotencyFixture(t)

	key := uuid.New().String()

	thirdUser := insertChatAuthorityUser(t, context.Background(), appDB, "third")
	room2 := insertChatAuthorityRoom(t, context.Background(), appDB, senderA, thirdUser)

	sendText(t, svc, room, senderA, "message in room 1", key)
	sendTextExpectError(t, svc, room2, senderA, "message in room 2", key, chatRepo.ErrIdempotencyConflict)

	_ = senderB
}

// ============================================================================
// PROOF E: Same actor + same key + changed attachment → 409 conflict
// ============================================================================
func TestIdempotency_SameActorSameKey_ChangedAttachment_Conflict(t *testing.T) {
	svc, _, room, senderA, _ := newIdempotencyFixture(t)

	key := uuid.New().String()

	attachment1 := map[string]interface{}{
		"type": "image",
		"data": map[string]interface{}{
			"url":      "https://cdn.example.com/first-for-sale.jpg",
			"filename": "first-for-sale.jpg",
			"caption":  "First ForSale",
		},
	}
	body1 := "for_sale shared"
	msg1, err := svc.SendMessage(context.Background(), room, senderA, chatEntity.MessageTypeText, &body1, attachment1, nil, nil, key, nil)
	require.NoError(t, err)
	require.NotNil(t, msg1)

	attachment2 := map[string]interface{}{
		"type": "image",
		"data": map[string]interface{}{
			"url":      "https://cdn.example.com/second-for-sale.jpg",
			"filename": "second-for-sale.jpg",
			"caption":  "Second ForSale",
		},
	}
	body2 := "for_sale shared"
	_, err = svc.SendMessage(context.Background(), room, senderA, chatEntity.MessageTypeText, &body2, attachment2, nil, nil, key, nil)
	require.ErrorIs(t, err, chatRepo.ErrIdempotencyConflict)
}

// ============================================================================
// PROOF F: Different actor + same key → independent messages, zero cross-actor leakage
// ============================================================================
func TestIdempotency_DifferentActorSameKey_IndependentMessages(t *testing.T) {
	svc, _, room, senderA, senderB := newIdempotencyFixture(t)

	key := uuid.New().String()

	msgA := sendText(t, svc, room, senderA, "message from A", key)
	msgB := sendText(t, svc, room, senderB, "message from B", key)

	assert.NotEqual(t, msgA.ID, msgB.ID, "different actors with same key must produce independent messages")
	assert.Equal(t, senderA, msgA.SenderID)
	assert.Equal(t, senderB, msgB.SenderID)
	assert.Equal(t, "message from A", *msgA.Body)
}

// ============================================================================
// PROOF G: Same actor + same key sequential → one artifact
// ============================================================================
func TestIdempotency_SameActorSameKeySequential_OneArtifact(t *testing.T) {
	svc, _, room, senderA, _ := newIdempotencyFixture(t)

	key := uuid.New().String()
	body := "sequential test"

	msg1 := sendText(t, svc, room, senderA, body, key)
	msg2 := sendText(t, svc, room, senderA, body, key)

	assert.Equal(t, msg1.ID, msg2.ID)
	assert.Equal(t, msg1.CommandFingerprint, msg2.CommandFingerprint)
}

// ============================================================================
// PROOF: Empty idempotency key is rejected
// ============================================================================
func TestIdempotency_EmptyKey_Rejected(t *testing.T) {
	svc, _, room, senderA, _ := newIdempotencyFixture(t)

	b := "no key"
	_, err := svc.SendMessage(context.Background(), room, senderA, chatEntity.MessageTypeText, &b, nil, nil, nil, "", nil)
	require.ErrorIs(t, err, chatRepo.ErrInvalidIdempotencyKey)
}

// ============================================================================
// PROOF: DB constraint rejects empty command_fingerprint
// ============================================================================
func TestIdempotency_EmptyFingerprint_RejectedByDBConstraint(t *testing.T) {
	_, appDB, room, senderA, _ := newIdempotencyFixture(t)
	ctx := context.Background()

	// Attempt to INSERT a message with empty fingerprint — must fail
	// because the CHECK (command_fingerprint <> '') constraint rejects it.
	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO chat_messages (id, room_id, sender_id, message_type, body, idempotency_key, command_fingerprint, created_at)
		VALUES ($1, $2, $3, 'text', $4, $5, '', NOW())
	`, uuid.New(), room, senderA, "test body", uuid.New().String())
	require.Error(t, err)
	require.Contains(t, err.Error(), "command_fingerprint_not_empty")
}

// ============================================================================
// PROOF: Fingerprint covers reply target changes
// ============================================================================
func TestIdempotency_SameActorSameKey_ChangedReplyTarget_Conflict(t *testing.T) {
	svc, _, room, senderA, _ := newIdempotencyFixture(t)

	key := uuid.New().String()
	body := "reply message"

	firstMsg := sendText(t, svc, room, senderA, "first message", uuid.New().String())
	secondMsg := sendText(t, svc, room, senderA, "second message", uuid.New().String())

	replyID1 := firstMsg.ID
	msg1, err := svc.SendMessage(context.Background(), room, senderA, chatEntity.MessageTypeText, &body, nil, &replyID1, nil, key, nil)
	require.NoError(t, err)
	require.NotNil(t, msg1)

	replyID2 := secondMsg.ID
	_, err = svc.SendMessage(context.Background(), room, senderA, chatEntity.MessageTypeText, &body, nil, &replyID2, nil, key, nil)
	require.ErrorIs(t, err, chatRepo.ErrIdempotencyConflict)
}

// ============================================================================
// PROOF: Fingerprint covers media asset IDs
// ============================================================================
func TestIdempotency_SameActorSameKey_ChangedMediaAssets_Conflict(t *testing.T) {
	svc, appDB, room, senderA, _ := newIdempotencyFixture(t)
	ctx := context.Background()

	key := uuid.New().String()
	body := "media message"

	assetID1 := uuid.New()
	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO chat_media_assets (id, room_id, uploader_id, media_type, content_type, storage_key,
			byte_size, status, expires_at)
		VALUES ($1, $2, $3, 'image', 'image/png', 'test-key-1', 1024, 'finalized', NOW() + INTERVAL '1 hour')
	`, assetID1, room, senderA)
	require.NoError(t, err)

	assetID2 := uuid.New()
	_, err = appDB.Pool().Exec(ctx, `
		INSERT INTO chat_media_assets (id, room_id, uploader_id, media_type, content_type, storage_key,
			byte_size, status, expires_at)
		VALUES ($1, $2, $3, 'image', 'image/png', 'test-key-2', 2048, 'finalized', NOW() + INTERVAL '1 hour')
	`, assetID2, room, senderA)
	require.NoError(t, err)

	msg1, err := svc.SendMessage(ctx, room, senderA, chatEntity.MessageTypeText, &body, nil, nil, []uuid.UUID{assetID1}, key, nil)
	require.NoError(t, err)
	require.NotNil(t, msg1)

	_, err = svc.SendMessage(ctx, room, senderA, chatEntity.MessageTypeText, &body, nil, nil, []uuid.UUID{assetID2}, key, nil)
	require.ErrorIs(t, err, chatRepo.ErrIdempotencyConflict)
}

// ============================================================================
// CONCURRENCY PROOF 1: Same actor, same key, same command concurrently
// → exactly one message row, both calls see the same message
// ============================================================================
func TestIdempotency_ConcurrentSameActorSameCommand_OneMessageArtifact(t *testing.T) {
	svc, appDB, room, senderA, _ := newIdempotencyFixtureWithOutbox(t)

	key := uuid.New().String()
	body := "concurrent identical"

	var wg sync.WaitGroup
	var msg1, msg2 *chatEntity.ChatMessage
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		b := body
		msg1, err1 = svc.SendMessage(context.Background(), room, senderA, chatEntity.MessageTypeText, &b, nil, nil, nil, key, nil)
	}()
	go func() {
		defer wg.Done()
		b := body
		msg2, err2 = svc.SendMessage(context.Background(), room, senderA, chatEntity.MessageTypeText, &b, nil, nil, nil, key, nil)
	}()
	wg.Wait()

	// Both must succeed
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NotNil(t, msg1)
	require.NotNil(t, msg2)

	// Both must see the SAME message artifact
	assert.Equal(t, msg1.ID, msg2.ID, "concurrent identical commands must produce one artifact")
	assert.Equal(t, msg1.CommandFingerprint, msg2.CommandFingerprint)

	// Outbox: exactly one chat.message.sent event for this message
	sentCount := countOutboxEvents(t, appDB, "chat.message.sent")
	assert.Equal(t, 1, sentCount, "concurrent same command must produce exactly one outbox artifact")
}

// ============================================================================
// CONCURRENCY PROOF 2: Different actors, same key concurrently
// → both succeed independently, zero cross-actor leakage
// ============================================================================
func TestIdempotency_ConcurrentDifferentActorSameKey_IndependentMessages(t *testing.T) {
	svc, appDB, room, senderA, senderB := newIdempotencyFixtureWithOutbox(t)

	key := uuid.New().String()

	var wg sync.WaitGroup
	var msgA, msgB *chatEntity.ChatMessage
	var errA, errB error

	wg.Add(2)
	go func() {
		defer wg.Done()
		b := "message from A"
		msgA, errA = svc.SendMessage(context.Background(), room, senderA, chatEntity.MessageTypeText, &b, nil, nil, nil, key, nil)
	}()
	go func() {
		defer wg.Done()
		b := "message from B"
		msgB, errB = svc.SendMessage(context.Background(), room, senderB, chatEntity.MessageTypeText, &b, nil, nil, nil, key, nil)
	}()
	wg.Wait()

	// Both must succeed independently
	require.NoError(t, errA)
	require.NoError(t, errB)
	require.NotNil(t, msgA)
	require.NotNil(t, msgB)

	// Distinct message artifacts
	assert.NotEqual(t, msgA.ID, msgB.ID, "different actors must produce independent messages")
	assert.Equal(t, senderA, msgA.SenderID)
	assert.Equal(t, senderB, msgB.SenderID)

	// Zero cross-actor leakage: each actor sees only their own content
	assert.Equal(t, "message from A", *msgA.Body)
	assert.Equal(t, "message from B", *msgB.Body)

	// Outbox: exactly two chat.message.sent events, one per actor
	sentCount := countOutboxEvents(t, appDB, "chat.message.sent")
	assert.Equal(t, 2, sentCount, "two actors must produce two independent outbox artifacts")
}

// ============================================================================
// CONCURRENCY PROOF 3: Same actor, same key, DIFFERENT commands concurrently
// → one wins, the other gets 409 ErrIdempotencyConflict
// ============================================================================
func TestIdempotency_ConcurrentSameActorChangedCommand_OneWinsOneConflict(t *testing.T) {
	svc, appDB, room, senderA, _ := newIdempotencyFixtureWithOutbox(t)

	key := uuid.New().String()

	var wg sync.WaitGroup
	var msg1, msg2 *chatEntity.ChatMessage
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		b := "command version alpha"
		msg1, err1 = svc.SendMessage(context.Background(), room, senderA, chatEntity.MessageTypeText, &b, nil, nil, nil, key, nil)
	}()
	go func() {
		defer wg.Done()
		b := "command version beta"
		msg2, err2 = svc.SendMessage(context.Background(), room, senderA, chatEntity.MessageTypeText, &b, nil, nil, nil, key, nil)
	}()
	wg.Wait()

	// Count successes
	successes := 0
	conflicts := 0
	if err1 == nil {
		successes++
		require.NotNil(t, msg1)
	} else if errors.Is(err1, chatRepo.ErrIdempotencyConflict) {
		conflicts++
	}
	if err2 == nil {
		successes++
		require.NotNil(t, msg2)
	} else if errors.Is(err2, chatRepo.ErrIdempotencyConflict) {
		conflicts++
	}

	// Exactly one must succeed, exactly one must conflict
	assert.Equal(t, 1, successes, "exactly one concurrent command must succeed")
	assert.Equal(t, 1, conflicts, "the other concurrent command must get 409 conflict")

	// The successful message must have a real fingerprint
	if msg1 != nil {
		assert.NotEmpty(t, msg1.CommandFingerprint)
	}
	if msg2 != nil {
		assert.NotEmpty(t, msg2.CommandFingerprint)
	}

	// The successful message must NOT leak the conflicting command's content
	if msg1 != nil {
		assert.Equal(t, "command version alpha", *msg1.Body)
	}
	if msg2 != nil {
		assert.Equal(t, "command version beta", *msg2.Body)
	}

	// Outbox: exactly one chat.message.sent event — the losing command
	// must not produce an artifact.
	sentCount := countOutboxEvents(t, appDB, "chat.message.sent")
	assert.Equal(t, 1, sentCount, "changed-command concurrency must produce exactly one outbox artifact")
}

// ============================================================================
// PROOF: No fingerprint bypass — strict comparison always enforced.
// Every message has a real fingerprint (DB CHECK guarantees non-empty),
// and any mismatch triggers ErrIdempotencyConflict.
// ============================================================================
func TestIdempotency_NoSentinelBypass_StrictFingerprintOnly(t *testing.T) {
	svc, _, room, senderA, _ := newIdempotencyFixture(t)

	key := uuid.New().String()

	// Create a message normally (gets a real fingerprint)
	sendText(t, svc, room, senderA, "original body", key)

	// Attempt replay with a different body — MUST be 409.
	// The fingerprint is real, the mismatch is detected, no bypass exists.
	sendTextExpectError(t, svc, room, senderA, "different body", key, chatRepo.ErrIdempotencyConflict)
}
