//go:build integration

package application

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// ============================================================================
// 3D-1 Integration Tests: Occurrence Batch Read Plumbing
// ============================================================================

type readPlumbingFixture struct {
	svc    *Service
	appDB  *db.DB
	repo   chatRepo.Repository
	sender uuid.UUID
	other  uuid.UUID
	room   uuid.UUID
}

func newReadPlumbingFixture(t *testing.T) *readPlumbingFixture {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())
	repo := chatInfraRepo.NewChatRepository()
	socialRepo := socialInfraRepo.NewSocialRepository()

	fb := NewOccurrenceFallbackBuilders(
		&defaultProfileFallbackBuilder{},
		&defaultContentFallbackBuilder{},
		&defaultFPSFallbackBuilder{},
		&defaultAuctionFallbackBuilder{},
	)

	svc := NewService(
		appDB, repo, socialRepo,
		&countingOutbox{db: appDB},
		rate.NewRateLimiter(),
		nil, nil, nil, nil, nil,
		newProductionAuthorizer(appDB, socialRepo),
		zap.NewNop(),
	)
	svc.fallbackBuilders = fb

	ctx := context.Background()
	sender := insertChatAuthorityUser(t, ctx, appDB, "3d1s-"+uuid.NewString()[:8])
	other := insertChatAuthorityUser(t, ctx, appDB, "3d1o-"+uuid.NewString()[:8])
	room := insertChatAuthorityRoom(t, ctx, appDB, sender, other)

	return &readPlumbingFixture{svc: svc, appDB: appDB, repo: repo, sender: sender, other: other, room: room}
}

func (f *readPlumbingFixture) sendText(t *testing.T, body string) *chatEntity.ChatMessage {
	t.Helper()
	msg, err := f.svc.SendTextMessage(context.Background(), f.room, f.sender, body, uuid.NewString())
	require.NoError(t, err)
	return msg
}

// sendTextBypassRL inserts a message directly into the database, bypassing the
// rate limiter. For use in high-volume test scenarios where the production rate
// limiter would interfere.
func (f *readPlumbingFixture) sendTextBypassRL(t *testing.T, body string) *chatEntity.ChatMessage {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	_, err := f.appDB.Pool().Exec(ctx, `
		INSERT INTO chat_messages (id, room_id, sender_id, message_type, body, idempotency_key, command_fingerprint, created_at)
		VALUES ($1, $2, $3, 'text', $4, $5, '00', NOW())
	`, id, f.room, f.sender, body, uuid.NewString())
	require.NoError(t, err)
	return &chatEntity.ChatMessage{
		ID: id, RoomID: f.room, SenderID: f.sender,
		MessageType: chatEntity.MessageTypeText, Body: &body,
		CreatedAt: time.Now(),
	}
}

// sendWithOccurrenceBypassRL inserts a message + occurrence directly.
func (f *readPlumbingFixture) sendWithOccurrenceBypassRL(t *testing.T, op chatEntity.ResourceOccurrenceOperation, rt chatEntity.ResourceOccurrenceResourceType, rid uuid.UUID) *chatEntity.ChatMessage {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	body := "msg-with-occurrence"
	_, err := f.appDB.Pool().Exec(ctx, `
		INSERT INTO chat_messages (id, room_id, sender_id, message_type, body, idempotency_key, command_fingerprint, created_at)
		VALUES ($1, $2, $3, 'text', $4, $5, '00', NOW())
	`, id, f.room, f.sender, body, uuid.NewString())
	require.NoError(t, err)

	var profileID, contentID, fpsID, auctionID *uuid.UUID
	switch rt {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		profileID = &rid
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		contentID = &rid
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		fpsID = &rid
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		auctionID = &rid
	}
	_, err = f.appDB.Pool().Exec(ctx, `
		INSERT INTO chat_message_resource_occurrences (message_id, operation, profile_source_id, content_source_id, for_sale_source_id, auction_source_id, fallback_snapshot, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, '{}', NOW())
	`, id, string(op), profileID, contentID, fpsID, auctionID)
	require.NoError(t, err)

	return &chatEntity.ChatMessage{
		ID: id, RoomID: f.room, SenderID: f.sender,
		MessageType: chatEntity.MessageTypeText, Body: &body,
		CreatedAt: time.Now(),
	}
}

func (f *readPlumbingFixture) sendWithOccurrence(t *testing.T, op chatEntity.ResourceOccurrenceOperation, rt chatEntity.ResourceOccurrenceResourceType, rid uuid.UUID) *chatEntity.ChatMessage {
	t.Helper()
	body := "msg-with-occurrence"
	key := uuid.NewString()
	msg, err := f.svc.SendMessage(context.Background(), f.room, f.sender, chatEntity.MessageTypeText, &body, nil, nil, nil, key,
		&chatEntity.ResourceOccurrenceIdentity{Operation: op, ResourceType: rt, ResourceID: rid})
	require.NoError(t, err)
	return msg
}

func (f *readPlumbingFixture) seedResource(t *testing.T, resourceType chatEntity.ResourceOccurrenceResourceType) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	switch resourceType {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		// Use a real user (sender already exists; use other for variety)
		return f.other
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		authorID := f.sender
		_, err := f.appDB.Pool().Exec(ctx, `
			INSERT INTO contents (id, author_id, status, visibility, is_hidden, caption, created_at, updated_at)
			VALUES ($1, $2, 'active', 'public', false, 'test content', NOW(), NOW())
			ON CONFLICT (id) DO NOTHING
		`, id, authorID)
		require.NoError(t, err)
		return id
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		sellerID := f.sender
		productID := uuid.New()
		_, err := f.appDB.Pool().Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, variety, media_urls, preparation_time, created_at, updated_at)
			VALUES ($1, $2, 'test fps', '', '', '[]', 'same_day', NOW(), NOW())
			ON CONFLICT (id) DO NOTHING
		`, productID, sellerID)
		require.NoError(t, err)
		_, err = f.appDB.Pool().Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, status, price_per_unit, created_at, updated_at)
			VALUES ($1, $2, $3, 'active', 10000, NOW(), NOW())
			ON CONFLICT (id) DO NOTHING
		`, id, productID, sellerID)
		require.NoError(t, err)
		return id
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		sellerID := f.sender
		productID := uuid.New()
		_, err := f.appDB.Pool().Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, variety, media_urls, preparation_time, created_at, updated_at)
			VALUES ($1, $2, 'test auction', '', '', '[]', 'same_day', NOW(), NOW())
			ON CONFLICT (id) DO NOTHING
		`, productID, sellerID)
		require.NoError(t, err)
		_, err = f.appDB.Pool().Exec(ctx, `
			INSERT INTO auctions (id, product_id, seller_id, status, start_price, bid_increment, start_at, end_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'active', 5000, 1000, NOW(), NOW() + INTERVAL '7 days', NOW(), NOW())
			ON CONFLICT (id) DO NOTHING
		`, id, productID, sellerID)
		require.NoError(t, err)
		return id
	}
	return uuid.Nil
}

// ============================================================================
// Test A: page with normal message only → occurrence map empty
// ============================================================================

func TestReadPlumbing_NormalMessagesOnly_EmptyOccurrenceMap(t *testing.T) {
	f := newReadPlumbingFixture(t)

	// Send 3 plain text messages (no occurrences)
	f.sendText(t, "hello 1")
	f.sendText(t, "hello 2")
	f.sendText(t, "hello 3")

	page, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	require.NotNil(t, page)
	assert.Len(t, page.Messages, 3)
	assert.NotNil(t, page.ResourceOccurrencesByMessageID)
	assert.Empty(t, page.ResourceOccurrencesByMessageID, "no occurrences expected for plain text messages")
}

// ============================================================================
// Test B: Profile occurrence → correct association
// ============================================================================

func TestReadPlumbing_ProfileOccurrence_CorrectAssociation(t *testing.T) {
	f := newReadPlumbingFixture(t)

	profileID := f.other // real user
	msg := f.sendWithOccurrence(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, profileID)

	page, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	require.Len(t, page.Messages, 1)
	assert.Len(t, page.ResourceOccurrencesByMessageID, 1)

	occ, ok := page.ResourceOccurrencesByMessageID[msg.ID]
	require.True(t, ok, "occurrence must be associated with the correct message_id")
	assert.Equal(t, chatEntity.ResourceOccurrenceOperationShareToChat, occ.Operation)
	assert.Equal(t, chatEntity.ResourceOccurrenceResourceTypeProfile, occ.ResourceType())
	assert.NotNil(t, occ.ProfileSourceID)
	assert.Equal(t, profileID, *occ.ProfileSourceID)
	assert.Nil(t, occ.ContentSourceID)
	assert.Nil(t, occ.ForSaleSourceID)
	assert.Nil(t, occ.AuctionSourceID)
	assert.NotEmpty(t, occ.FallbackSnapshot)
}

// ============================================================================
// Test C: Content occurrence → correct association
// ============================================================================

func TestReadPlumbing_ContentOccurrence_CorrectAssociation(t *testing.T) {
	f := newReadPlumbingFixture(t)

	contentID := f.seedResource(t, chatEntity.ResourceOccurrenceResourceTypeContent)
	msg := f.sendWithOccurrence(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)

	page, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	require.Len(t, page.Messages, 1)
	assert.Len(t, page.ResourceOccurrencesByMessageID, 1)

	occ, ok := page.ResourceOccurrencesByMessageID[msg.ID]
	require.True(t, ok)
	assert.Equal(t, chatEntity.ResourceOccurrenceResourceTypeContent, occ.ResourceType())
	assert.NotNil(t, occ.ContentSourceID)
	assert.Equal(t, contentID, *occ.ContentSourceID)
	assert.Nil(t, occ.ProfileSourceID)
	assert.Nil(t, occ.ForSaleSourceID)
	assert.Nil(t, occ.AuctionSourceID)
}

// ============================================================================
// Test D: FPS occurrence → correct association
// ============================================================================

func TestReadPlumbing_FPSOccurrence_CorrectAssociation(t *testing.T) {
	f := newReadPlumbingFixture(t)

	fpsID := f.seedResource(t, chatEntity.ResourceOccurrenceResourceTypeForSale)
	msg := f.sendWithOccurrence(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)

	page, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	require.Len(t, page.Messages, 1)
	assert.Len(t, page.ResourceOccurrencesByMessageID, 1)

	occ, ok := page.ResourceOccurrencesByMessageID[msg.ID]
	require.True(t, ok)
	assert.Equal(t, chatEntity.ResourceOccurrenceResourceTypeForSale, occ.ResourceType())
	assert.NotNil(t, occ.ForSaleSourceID)
	assert.Equal(t, fpsID, *occ.ForSaleSourceID)
	assert.Nil(t, occ.ProfileSourceID)
	assert.Nil(t, occ.ContentSourceID)
	assert.Nil(t, occ.AuctionSourceID)
}

// ============================================================================
// Test E: Auction occurrence → correct association
// ============================================================================

func TestReadPlumbing_AuctionOccurrence_CorrectAssociation(t *testing.T) {
	f := newReadPlumbingFixture(t)

	auctionID := f.seedResource(t, chatEntity.ResourceOccurrenceResourceTypeAuction)
	msg := f.sendWithOccurrence(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)

	page, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	require.Len(t, page.Messages, 1)
	assert.Len(t, page.ResourceOccurrencesByMessageID, 1)

	occ, ok := page.ResourceOccurrencesByMessageID[msg.ID]
	require.True(t, ok)
	assert.Equal(t, chatEntity.ResourceOccurrenceResourceTypeAuction, occ.ResourceType())
	assert.NotNil(t, occ.AuctionSourceID)
	assert.Equal(t, auctionID, *occ.AuctionSourceID)
	assert.Nil(t, occ.ProfileSourceID)
	assert.Nil(t, occ.ContentSourceID)
	assert.Nil(t, occ.ForSaleSourceID)
}

// ============================================================================
// Test F: mixed page — 20 messages, all 4 types, no cross-message leakage
// ============================================================================

func TestReadPlumbing_MixedPage_AllTypes_NoCrossLeakage(t *testing.T) {
	f := newReadPlumbingFixture(t)

	profileID := f.other
	contentID := f.seedResource(t, chatEntity.ResourceOccurrenceResourceTypeContent)
	fpsID := f.seedResource(t, chatEntity.ResourceOccurrenceResourceTypeForSale)
	auctionID := f.seedResource(t, chatEntity.ResourceOccurrenceResourceTypeAuction)

	// Send messages: 5 of each occurrence type + 5 without
	type sentMsg struct {
		msg   *chatEntity.ChatMessage
		rt    chatEntity.ResourceOccurrenceResourceType
		rid   uuid.UUID
		hasOc bool
	}
	var sent []sentMsg

	for i := 0; i < 5; i++ {
		sent = append(sent, sentMsg{msg: f.sendTextBypassRL(t, fmt.Sprintf("plain-%d", i)), hasOc: false})
	}
	for i := 0; i < 5; i++ {
		m := f.sendWithOccurrenceBypassRL(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, profileID)
		sent = append(sent, sentMsg{msg: m, rt: chatEntity.ResourceOccurrenceResourceTypeProfile, rid: profileID, hasOc: true})
	}
	for i := 0; i < 4; i++ {
		m := f.sendWithOccurrenceBypassRL(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)
		sent = append(sent, sentMsg{msg: m, rt: chatEntity.ResourceOccurrenceResourceTypeContent, rid: contentID, hasOc: true})
	}
	for i := 0; i < 3; i++ {
		m := f.sendWithOccurrenceBypassRL(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
		sent = append(sent, sentMsg{msg: m, rt: chatEntity.ResourceOccurrenceResourceTypeForSale, rid: fpsID, hasOc: true})
	}
	for i := 0; i < 3; i++ {
		m := f.sendWithOccurrenceBypassRL(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)
		sent = append(sent, sentMsg{msg: m, rt: chatEntity.ResourceOccurrenceResourceTypeAuction, rid: auctionID, hasOc: true})
	}

	require.Len(t, sent, 20)

	page, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	require.Len(t, page.Messages, 20)

	// 15 of 20 messages have occurrences
	occurrenceCount := len(page.ResourceOccurrencesByMessageID)
	assert.Equal(t, 15, occurrenceCount, "expected 15 messages with occurrences (5 profile + 4 content + 3 FPS + 3 auction)")

	// Verify exact association — no cross-message leakage
	for _, s := range sent {
		occ, hasOcc := page.ResourceOccurrencesByMessageID[s.msg.ID]
		if !s.hasOc {
			assert.False(t, hasOcc, "message %s should NOT have an occurrence", s.msg.ID)
			continue
		}
		require.True(t, hasOcc, "message %s SHOULD have an occurrence", s.msg.ID)
		assert.Equal(t, s.rt, occ.ResourceType(), "wrong resource type for message %s", s.msg.ID)
		assert.Equal(t, s.rid, occ.SourceID(), "wrong source ID for message %s", s.msg.ID)
	}
}

// ============================================================================
// Test G: message without occurrence mixed among occurrence messages
// ============================================================================

func TestReadPlumbing_PlainMessageAmongOccurrences_NoFalseAssociation(t *testing.T) {
	f := newReadPlumbingFixture(t)

	fpsID := f.seedResource(t, chatEntity.ResourceOccurrenceResourceTypeForSale)

	msgWith := f.sendWithOccurrence(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
	msgWithout := f.sendText(t, "plain message")
	msgWith2 := f.sendWithOccurrence(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)

	page, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	require.Len(t, page.Messages, 3)

	// Messages with occurrences
	_, ok := page.ResourceOccurrencesByMessageID[msgWith.ID]
	assert.True(t, ok, "first occurrence message must have occurrence")
	_, ok = page.ResourceOccurrencesByMessageID[msgWith2.ID]
	assert.True(t, ok, "third occurrence message must have occurrence")

	// Plain message must NOT have false association
	_, ok = page.ResourceOccurrencesByMessageID[msgWithout.ID]
	assert.False(t, ok, "plain message must NOT have false occurrence association")
}

// ============================================================================
// Test H: occurrence batch-load infrastructure failure → error propagated
// ============================================================================

// readPlumbingFailingOccurrenceRepo wraps a real Repository and fails on
// GetResourceOccurrencesByMessageIDs.
type readPlumbingFailingOccurrenceRepo struct {
	chatRepo.Repository
}

func (r *readPlumbingFailingOccurrenceRepo) GetResourceOccurrencesByMessageIDs(ctx context.Context, tx interface{}, messageIDs []uuid.UUID) (map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, error) {
	return nil, fmt.Errorf("simulated occurrence batch-load infrastructure failure")
}

func TestReadPlumbing_OccurrenceBatchLoadFailure_PropagatesError(t *testing.T) {
	f := newReadPlumbingFixture(t)

	// Send one message with an occurrence (so there IS an occurrence to load)
	fpsID := f.seedResource(t, chatEntity.ResourceOccurrenceResourceTypeForSale)
	f.sendWithOccurrence(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)

	// Create a service with failing occurrence repo
	appDB := f.appDB
	socialRepo := socialInfraRepo.NewSocialRepository()
	failingRepo := &readPlumbingFailingOccurrenceRepo{Repository: f.repo}

	fb := NewOccurrenceFallbackBuilders(
		&defaultProfileFallbackBuilder{},
		&defaultContentFallbackBuilder{},
		&defaultFPSFallbackBuilder{},
		&defaultAuctionFallbackBuilder{},
	)

	failingSvc := NewService(
		appDB, failingRepo, socialRepo,
		&countingOutbox{db: appDB},
		rate.NewRateLimiter(),
		nil, nil, nil, nil, nil,
		newProductionAuthorizer(appDB, socialRepo),
		zap.NewNop(),
	)
	failingSvc.fallbackBuilders = fb

	// ListMessages must return an error, NOT silently succeed
	page, err := failingSvc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	assert.Error(t, err, "occurrence batch-load failure must propagate as an error")
	assert.Nil(t, page, "page must be nil on error")
	assert.Contains(t, err.Error(), "batch-load resource occurrences failed")
}

// ============================================================================
// Edge case: empty room → empty messages, empty occurrences
// ============================================================================

func TestReadPlumbing_EmptyRoom_EmptyResult(t *testing.T) {
	f := newReadPlumbingFixture(t)

	page, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	require.NotNil(t, page)
	assert.Empty(t, page.Messages)
	assert.NotNil(t, page.ResourceOccurrencesByMessageID)
	assert.Empty(t, page.ResourceOccurrencesByMessageID)
}

// ============================================================================
// Edge case: single message page (pagination still works)
// ============================================================================

func TestReadPlumbing_SingleMessagePage(t *testing.T) {
	f := newReadPlumbingFixture(t)

	f.sendText(t, "msg1")
	f.sendText(t, "msg2")
	contentID := f.seedResource(t, chatEntity.ResourceOccurrenceResourceTypeContent)
	msg3 := f.sendWithOccurrence(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)

	// Fetch with limit=1
	page, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 1)
	require.NoError(t, err)
	require.Len(t, page.Messages, 1)

	// The most recent message (msg3) should have its occurrence
	occ, ok := page.ResourceOccurrencesByMessageID[msg3.ID]
	require.True(t, ok)
	assert.Equal(t, chatEntity.ResourceOccurrenceResourceTypeContent, occ.ResourceType())
}

// ============================================================================
// Query-count proof tests
// ============================================================================

// queryCountingTracer is a TEST-ONLY pgx QueryTracer that atomically counts
// every SQL query execution.
type queryCountingTracer struct {
	count atomic.Int64
}

func (t *queryCountingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	t.count.Add(1)
	return ctx
}

func (t *queryCountingTracer) TraceQueryEnd(_ context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
}

func (t *queryCountingTracer) reset()       { t.count.Store(0) }
func (t *queryCountingTracer) value() int64 { return t.count.Load() }

// tracedReadFixture is backed by a pool with a query-counting tracer.
type tracedReadFixture struct {
	svc    *Service
	tracer *queryCountingTracer
	appDB  *db.DB
	sender uuid.UUID
	other  uuid.UUID
	room   uuid.UUID
}

// newTracedReadFixture creates a query-counting fixture by cloning the pool
// config from an already-set-up DB. It does NOT call testdb.SetupDB — that
// would contend on the advisory migration lock. The caller must already hold
// a valid testdb setup (e.g. via readPlumbingFixture).
func newTracedReadFixture(t *testing.T, seedDB *db.DB) *tracedReadFixture {
	t.Helper()
	ctx := context.Background()

	// Clone pool config from the already-set-up pool — no second SetupDB call.
	baseCfg := seedDB.Pool().Config()
	tracer := &queryCountingTracer{}
	baseCfg.ConnConfig.Tracer = tracer
	tracedPool, err := pgxpool.NewWithConfig(ctx, baseCfg)
	require.NoError(t, err)

	appDB := db.NewFromPool(tracedPool)
	repo := chatInfraRepo.NewChatRepository()
	socialRepo := socialInfraRepo.NewSocialRepository()

	fb := NewOccurrenceFallbackBuilders(
		&defaultProfileFallbackBuilder{},
		&defaultContentFallbackBuilder{},
		&defaultFPSFallbackBuilder{},
		&defaultAuctionFallbackBuilder{},
	)

	svc := NewService(
		appDB, repo, socialRepo,
		&countingOutbox{db: appDB},
		rate.NewRateLimiter(),
		nil, nil, nil, nil, nil,
		newProductionAuthorizer(appDB, socialRepo),
		zap.NewNop(),
	)
	svc.fallbackBuilders = fb

	sender := insertChatAuthorityUser(t, ctx, appDB, "qc-s-"+uuid.NewString()[:8])
	other := insertChatAuthorityUser(t, ctx, appDB, "qc-o-"+uuid.NewString()[:8])
	room := insertChatAuthorityRoom(t, ctx, appDB, sender, other)

	return &tracedReadFixture{svc: svc, tracer: tracer, appDB: appDB, sender: sender, other: other, room: room}
}

func (f *tracedReadFixture) sendTextQC(t *testing.T, body string) {
	t.Helper()
	_, err := f.svc.SendTextMessage(context.Background(), f.room, f.sender, body, uuid.NewString())
	require.NoError(t, err)
}

func (f *tracedReadFixture) sendWithOccurrenceQC(t *testing.T, rt chatEntity.ResourceOccurrenceResourceType, rid uuid.UUID) {
	t.Helper()
	body := "qc-msg"
	key := uuid.NewString()
	_, err := f.svc.SendMessage(context.Background(), f.room, f.sender, chatEntity.MessageTypeText, &body, nil, nil, nil, key,
		&chatEntity.ResourceOccurrenceIdentity{Operation: chatEntity.ResourceOccurrenceOperationShareToChat, ResourceType: rt, ResourceID: rid})
	require.NoError(t, err)
}

func (f *tracedReadFixture) sendTextBypassRL(t *testing.T, body string) {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	_, err := f.appDB.Pool().Exec(ctx, `
		INSERT INTO chat_messages (id, room_id, sender_id, message_type, body, idempotency_key, command_fingerprint, created_at)
		VALUES ($1, $2, $3, 'text', $4, $5, '00', NOW())
	`, id, f.room, f.sender, body, uuid.NewString())
	require.NoError(t, err)
}

func (f *tracedReadFixture) sendWithOccurrenceBypassRL(t *testing.T, rt chatEntity.ResourceOccurrenceResourceType, rid uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	body := "qc-msg"
	_, err := f.appDB.Pool().Exec(ctx, `
		INSERT INTO chat_messages (id, room_id, sender_id, message_type, body, idempotency_key, command_fingerprint, created_at)
		VALUES ($1, $2, $3, 'text', $4, $5, '00', NOW())
	`, id, f.room, f.sender, body, uuid.NewString())
	require.NoError(t, err)

	var profileID, contentID, fpsID, auctionID *uuid.UUID
	switch rt {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		profileID = &rid
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		contentID = &rid
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		fpsID = &rid
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		auctionID = &rid
	}
	_, err = f.appDB.Pool().Exec(ctx, `
		INSERT INTO chat_message_resource_occurrences (message_id, operation, profile_source_id, content_source_id, for_sale_source_id, auction_source_id, fallback_snapshot, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, '{}', NOW())
	`, id, string(chatEntity.ResourceOccurrenceOperationShareToChat), profileID, contentID, fpsID, auctionID)
	require.NoError(t, err)
}

func (f *tracedReadFixture) seedResourceQC(t *testing.T, rt chatEntity.ResourceOccurrenceResourceType) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	switch rt {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		return f.other
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		authorID := f.sender
		_, err := f.appDB.Pool().Exec(ctx,
			`INSERT INTO contents (id, author_id, status, visibility, is_hidden, caption, created_at, updated_at) VALUES ($1,$2,'active','public',false,'qc',NOW(),NOW()) ON CONFLICT DO NOTHING`, id, authorID)
		require.NoError(t, err)
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		sellerID := f.sender
		productID := uuid.New()
		_, err := f.appDB.Pool().Exec(ctx,
			`INSERT INTO products (id, seller_id, title, description, variety, media_urls, preparation_time, created_at, updated_at) VALUES ($1,$2,'qc','','','[]','same_day',NOW(),NOW()) ON CONFLICT DO NOTHING`, productID, sellerID)
		require.NoError(t, err)
		_, err = f.appDB.Pool().Exec(ctx,
			`INSERT INTO for_sales (id, product_id, seller_id, status, price_per_unit, created_at, updated_at) VALUES ($1,$2,$3,'active',10000,NOW(),NOW()) ON CONFLICT DO NOTHING`, id, productID, sellerID)
		require.NoError(t, err)
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		sellerID := f.sender
		productID := uuid.New()
		_, err := f.appDB.Pool().Exec(ctx,
			`INSERT INTO products (id, seller_id, title, description, variety, media_urls, preparation_time, created_at, updated_at) VALUES ($1,$2,'qc','','','[]','same_day',NOW(),NOW()) ON CONFLICT DO NOTHING`, productID, sellerID)
		require.NoError(t, err)
		_, err = f.appDB.Pool().Exec(ctx,
			`INSERT INTO auctions (id, product_id, seller_id, status, start_price, bid_increment, start_at, end_at, created_at, updated_at) VALUES ($1,$2,$3,'active',5000,1000,NOW(),NOW()+INTERVAL '7 days',NOW(),NOW()) ON CONFLICT DO NOTHING`, id, productID, sellerID)
		require.NoError(t, err)
	}
	return id
}

func TestQueryCount_OneNormalMessage_Bounded(t *testing.T) {
	base := newReadPlumbingFixture(t)
	f := newTracedReadFixture(t, base.appDB)
	f.sendTextQC(t, "hello")

	f.tracer.reset()
	_, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	count := f.tracer.value()

	t.Logf("Query count for 1 normal message: %d", count)
	assert.GreaterOrEqual(t, count, int64(3), "at least 3 queries: room + messages + occurrence batch")
}

func TestQueryCount_TwentyMessages_NotMessageDependent(t *testing.T) {
	base := newReadPlumbingFixture(t)
	f := newTracedReadFixture(t, base.appDB)
	for i := 0; i < 20; i++ {
		f.sendTextBypassRL(t, fmt.Sprintf("msg-%d", i))
	}

	f.tracer.reset()
	_, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	count20 := f.tracer.value()

	f2 := newTracedReadFixture(t, base.appDB)
	f2.sendTextQC(t, "single")
	f2.tracer.reset()
	_, err = f2.svc.ListMessages(context.Background(), f2.room, f2.sender, nil, nil, 50)
	require.NoError(t, err)
	count1 := f2.tracer.value()

	t.Logf("Query count: 1 msg=%d, 20 msgs=%d", count1, count20)
	assert.Equal(t, count1, count20, "query count must be constant regardless of message count")
}

func TestQueryCount_OccurrenceLoad_Bounded(t *testing.T) {
	base := newReadPlumbingFixture(t)
	f := newTracedReadFixture(t, base.appDB)

	fpsID := f.seedResourceQC(t, chatEntity.ResourceOccurrenceResourceTypeForSale)
	f.sendWithOccurrenceQC(t, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)

	f.tracer.reset()
	_, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	count1 := f.tracer.value()

	// 20 msgs with mixed occurrences — reuse the same seed DB for the second traced fixture.
	f2 := newTracedReadFixture(t, base.appDB)
	fpsID2 := f2.seedResourceQC(t, chatEntity.ResourceOccurrenceResourceTypeForSale)
	contentID := f2.seedResourceQC(t, chatEntity.ResourceOccurrenceResourceTypeContent)
	for i := 0; i < 10; i++ {
		f2.sendTextBypassRL(t, fmt.Sprintf("p-%d", i))
	}
	for i := 0; i < 5; i++ {
		f2.sendWithOccurrenceBypassRL(t, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID2)
	}
	for i := 0; i < 5; i++ {
		f2.sendWithOccurrenceBypassRL(t, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)
	}

	f2.tracer.reset()
	_, err = f2.svc.ListMessages(context.Background(), f2.room, f2.sender, nil, nil, 50)
	require.NoError(t, err)
	count20 := f2.tracer.value()

	t.Logf("Query count: 1 occurrence msg=%d, 20 mixed msgs=%d", count1, count20)
	diff := count20 - count1
	if diff < 0 {
		diff = -diff
	}
	assert.LessOrEqual(t, diff, int64(2), "query count bounded — not growing with message/occurrence count")
}

func TestQueryCount_NoNPlusOne_AllFourTypes(t *testing.T) {
	base := newReadPlumbingFixture(t)
	f := newTracedReadFixture(t, base.appDB)

	profileID := f.other
	contentID := f.seedResourceQC(t, chatEntity.ResourceOccurrenceResourceTypeContent)
	fpsID := f.seedResourceQC(t, chatEntity.ResourceOccurrenceResourceTypeForSale)
	auctionID := f.seedResourceQC(t, chatEntity.ResourceOccurrenceResourceTypeAuction)

	for i := 0; i < 5; i++ {
		f.sendWithOccurrenceBypassRL(t, chatEntity.ResourceOccurrenceResourceTypeProfile, profileID)
		f.sendWithOccurrenceBypassRL(t, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)
		f.sendWithOccurrenceBypassRL(t, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
		f.sendWithOccurrenceBypassRL(t, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)
	}

	f.tracer.reset()
	_, err := f.svc.ListMessages(context.Background(), f.room, f.sender, nil, nil, 50)
	require.NoError(t, err)
	count := f.tracer.value()

	t.Logf("Query count for 20 msgs with 20 occurrences (4 types): %d", count)
	assert.Less(t, count, int64(15), "N+1 per message would produce >>15 queries; bounded shows <15")
}
