//go:build integration

package application

import (
	"context"
	"encoding/json"
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

// ============================================================================
// Production-grade test authorizer using real domain services
// ============================================================================

// productionAuthorizer is a test-accessible composition that mirrors the
// production chatResourceAuthorizerAdapter using the test DB.
type productionAuthorizer struct {
	fb         *OccurrenceFallbackBuilders
	socialRepo blockChecker
	db         *db.DB
}

func newProductionAuthorizer(db *db.DB, socialRepo blockChecker) *productionAuthorizer {
	return &productionAuthorizer{
		fb: NewOccurrenceFallbackBuilders(
			&defaultProfileFallbackBuilder{},
			&defaultContentFallbackBuilder{},
			&defaultFPSFallbackBuilder{},
			&defaultAuctionFallbackBuilder{},
		),
		socialRepo: socialRepo,
		db:         db,
	}
}

func (a *productionAuthorizer) AuthorizeShare(ctx context.Context, tx interface{}, viewerID uuid.UUID, rt chatEntity.ResourceOccurrenceResourceType, rid uuid.UUID) (json.RawMessage, error) {
	dtx := tx.(db.Tx)
	switch rt {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		var deletedAt *interface{}
		if err := dtx.QueryRow(ctx, `SELECT deleted_at FROM users WHERE id=$1`, rid).Scan(&deletedAt); err != nil {
			return nil, chatRepo.ErrResourceNotFound
		}
		if deletedAt != nil {
			return nil, chatRepo.ErrResourceNotFound
		}
		if viewerID != rid {
			blocked, _ := a.socialRepo.ExistsBlock(ctx, tx, viewerID, rid)
			if blocked {
				return nil, chatRepo.ErrResourceNotAccessible
			}
		}
		return a.fb.BuildFallback(ctx, dtx, rt, rid)

	case chatEntity.ResourceOccurrenceResourceTypeContent:
		var authorID uuid.UUID
		var visibility string
		var isHidden bool
		var deletedAt *interface{}
		if err := dtx.QueryRow(ctx, `SELECT author_id, visibility, is_hidden, deleted_at FROM contents WHERE id=$1`, rid).Scan(&authorID, &visibility, &isHidden, &deletedAt); err != nil {
			return nil, chatRepo.ErrResourceNotFound
		}
		if deletedAt != nil || isHidden {
			return nil, chatRepo.ErrResourceNotAccessible
		}
		// Block check applies to all visibility levels
		if authorID != viewerID {
			blocked, _ := a.socialRepo.ExistsBlock(ctx, tx, viewerID, authorID)
			if blocked {
				return nil, chatRepo.ErrResourceNotAccessible
			}
		}
		switch visibility {
		case "public":
		case "private":
			if authorID != viewerID {
				return nil, chatRepo.ErrResourceNotAccessible
			}
		case "followers_only":
			if authorID != viewerID {
				var follows bool
				_ = dtx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_follows WHERE follower_id=$1 AND following_id=$2)`, viewerID, authorID).Scan(&follows)
				if !follows {
					return nil, chatRepo.ErrResourceNotAccessible
				}
			}
		default:
			return nil, chatRepo.ErrResourceNotAccessible
		}
		return a.fb.BuildFallback(ctx, dtx, rt, rid)

	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		var found uuid.UUID
		if err := dtx.QueryRow(ctx, `SELECT id FROM for_sales WHERE id=$1`, rid).Scan(&found); err != nil {
			return nil, chatRepo.ErrResourceNotFound
		}
		return a.fb.BuildFallback(ctx, dtx, rt, rid)

	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		var found uuid.UUID
		if err := dtx.QueryRow(ctx, `SELECT id FROM auctions WHERE id=$1`, rid).Scan(&found); err != nil {
			return nil, chatRepo.ErrResourceNotFound
		}
		return a.fb.BuildFallback(ctx, dtx, rt, rid)
	}
	return nil, chatRepo.ErrResourceNotFound
}

func (a *productionAuthorizer) AuthorizeDirect(ctx context.Context, tx interface{}, actorID uuid.UUID, rt chatEntity.ResourceOccurrenceResourceType, rid uuid.UUID) (json.RawMessage, error) {
	dtx := tx.(db.Tx)
	switch rt {
	case chatEntity.ResourceOccurrenceResourceTypeProfile, chatEntity.ResourceOccurrenceResourceTypeContent:
		return nil, fmt.Errorf("direct_commerce_insert_chat not valid for profile or content")
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		var sellerID uuid.UUID
		var status string
		if err := dtx.QueryRow(ctx, `SELECT seller_id, status FROM for_sales WHERE id=$1`, rid).Scan(&sellerID, &status); err != nil {
			return nil, chatRepo.ErrResourceNotFound
		}
		if sellerID != actorID {
			return nil, chatRepo.ErrNotResourceOwner
		}
		if status != "active" {
			return nil, chatRepo.ErrResourceNotPromotable
		}
		// Market capability: seller must have active subscription
		if !a.hasMarketCapability(ctx, sellerID) {
			return nil, chatRepo.ErrMarketAuthorityRequired
		}
		return a.fb.BuildFallback(ctx, dtx, rt, rid)
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		var sellerID uuid.UUID
		var status string
		if err := dtx.QueryRow(ctx, `SELECT seller_id, status FROM auctions WHERE id=$1`, rid).Scan(&sellerID, &status); err != nil {
			return nil, chatRepo.ErrResourceNotFound
		}
		if sellerID != actorID {
			return nil, chatRepo.ErrNotResourceOwner
		}
		if status != "scheduled" && status != "active" {
			return nil, chatRepo.ErrResourceNotPromotable
		}
		// Market capability: seller must have active subscription
		if !a.hasMarketCapability(ctx, sellerID) {
			return nil, chatRepo.ErrMarketAuthorityRequired
		}
		return a.fb.BuildFallback(ctx, dtx, rt, rid)
	}
	return nil, chatRepo.ErrResourceNotFound
}

func (a *productionAuthorizer) BuildFallback(ctx context.Context, tx interface{}, rt chatEntity.ResourceOccurrenceResourceType, rid uuid.UUID) (json.RawMessage, error) {
	return a.fb.BuildFallback(ctx, tx.(db.Tx), rt, rid)
}

// hasMarketCapability checks seller subscription status via the DB pool
// (same authority as production auth.RoleCheckerDB.HasActiveSellerCapability).
func (a *productionAuthorizer) hasMarketCapability(ctx context.Context, sellerID uuid.UUID) bool {
	var status string
	err := a.db.Pool().QueryRow(ctx, `SELECT status FROM seller_subscriptions WHERE user_id=$1 ORDER BY created_at DESC LIMIT 1`, sellerID).Scan(&status)
	if err != nil {
		return false
	}
	return status == "active" || status == "trial"
}

type blockChecker interface {
	ExistsBlock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)
}

// ============================================================================
// E2E Fixture
// ============================================================================

type occurrenceE2EFixture struct {
	svc    *Service
	appDB  *db.DB
	sender uuid.UUID
	other  uuid.UUID
	room   uuid.UUID
}

func newOccurrenceE2EFixture(t *testing.T) *occurrenceE2EFixture {
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
	sender := insertChatAuthorityUser(t, ctx, appDB, "e2es-"+uuid.NewString()[:8])
	other := insertChatAuthorityUser(t, ctx, appDB, "e2eo-"+uuid.NewString()[:8])
	room := insertChatAuthorityRoom(t, ctx, appDB, sender, other)

	return &occurrenceE2EFixture{svc: svc, appDB: appDB, sender: sender, other: other, room: room}
}

func (f *occurrenceE2EFixture) send(t *testing.T, op chatEntity.ResourceOccurrenceOperation, rt chatEntity.ResourceOccurrenceResourceType, rid uuid.UUID) (*chatEntity.ChatMessage, error) {
	t.Helper()
	body := "e2e"
	key := uuid.NewString()
	return f.svc.SendMessage(context.Background(), f.room, f.sender, chatEntity.MessageTypeText, &body, nil, nil, nil, key,
		&chatEntity.ResourceOccurrenceIdentity{Operation: op, ResourceType: rt, ResourceID: rid})
}

func (f *occurrenceE2EFixture) sendWithKey(t *testing.T, op chatEntity.ResourceOccurrenceOperation, rt chatEntity.ResourceOccurrenceResourceType, rid uuid.UUID, key string) (*chatEntity.ChatMessage, error) {
	t.Helper()
	body := "e2e"
	return f.svc.SendMessage(context.Background(), f.room, f.sender, chatEntity.MessageTypeText, &body, nil, nil, nil, key,
		&chatEntity.ResourceOccurrenceIdentity{Operation: op, ResourceType: rt, ResourceID: rid})
}

func (f *occurrenceE2EFixture) assertOccurrence(t *testing.T, msgID uuid.UUID, op chatEntity.ResourceOccurrenceOperation, rt chatEntity.ResourceOccurrenceResourceType, sourceID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var dbOp string
	var p, c, fp, a *uuid.UUID
	var fallback []byte
	err := f.appDB.Pool().QueryRow(ctx,
		`SELECT operation, profile_source_id, content_source_id, for_sale_source_id, auction_source_id, fallback_snapshot FROM chat_message_resource_occurrences WHERE message_id=$1`, msgID,
	).Scan(&dbOp, &p, &c, &fp, &a, &fallback)
	require.NoError(t, err)
	assert.Equal(t, string(op), dbOp)

	switch rt {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		require.NotNil(t, p)
		assert.Equal(t, sourceID, *p)
		assert.Nil(t, c)
		assert.Nil(t, fp)
		assert.Nil(t, a)
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		require.NotNil(t, c)
		assert.Equal(t, sourceID, *c)
		assert.Nil(t, p)
		assert.Nil(t, fp)
		assert.Nil(t, a)
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		require.NotNil(t, fp)
		assert.Equal(t, sourceID, *fp)
		assert.Nil(t, p)
		assert.Nil(t, c)
		assert.Nil(t, a)
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		require.NotNil(t, a)
		assert.Equal(t, sourceID, *a)
		assert.Nil(t, p)
		assert.Nil(t, c)
		assert.Nil(t, fp)
	}
	assert.NotEmpty(t, fallback)
	assert.NotEqual(t, "{}", string(fallback))
	var fb map[string]interface{}
	require.NoError(t, json.Unmarshal(fallback, &fb))
	for _, prohibited := range []string{"price", "current_bid", "quantity", "status", "phase", "is_available", "is_sold", "is_closed"} {
		_, has := fb[prohibited]
		assert.False(t, has, "fallback must not contain %s", prohibited)
	}
}

func (f *occurrenceE2EFixture) cntMsg(t *testing.T) int {
	var c int
	require.NoError(t, f.appDB.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM chat_messages`).Scan(&c))
	return c
}
func (f *occurrenceE2EFixture) cntOcc(t *testing.T) int {
	var c int
	require.NoError(t, f.appDB.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM chat_message_resource_occurrences`).Scan(&c))
	return c
}
func (f *occurrenceE2EFixture) cntOutbox(t *testing.T) int {
	var c int
	require.NoError(t, f.appDB.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM outbox WHERE event_type='chat.message.sent'`).Scan(&c))
	return c
}

// ============================================================================
// Test seed helpers
// ============================================================================

func seedProfile(t *testing.T, f *occurrenceE2EFixture, name string) uuid.UUID {
	return insertChatAuthorityUser(t, context.Background(), f.appDB, name)
}

func seedContent(t *testing.T, f *occurrenceE2EFixture, authorID uuid.UUID, visibility string, hidden, deleted bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	_, err := f.appDB.Pool().Exec(ctx,
		`INSERT INTO contents (id, author_id, caption, visibility, is_hidden, status, created_at, updated_at) VALUES ($1,$2,'test',$3::content_visibility_enum,$4,'active',NOW(),NOW())`,
		id, authorID, visibility, hidden)
	require.NoError(t, err)
	if deleted {
		f.appDB.Pool().Exec(ctx, `UPDATE contents SET deleted_at=NOW() WHERE id=$1`, id)
	}
	return id
}

func seedFollow(t *testing.T, f *occurrenceE2EFixture, follower, followed uuid.UUID) {
	_, err := f.appDB.Pool().Exec(context.Background(), `INSERT INTO user_follows (follower_id, following_id, created_at) VALUES ($1,$2,NOW()) ON CONFLICT DO NOTHING`, follower, followed)
	require.NoError(t, err)
}

func seedBlock(t *testing.T, f *occurrenceE2EFixture, blocker, blocked uuid.UUID) {
	_, err := f.appDB.Pool().Exec(context.Background(), `INSERT INTO user_blocks (blocker_id, blocked_id, created_at) VALUES ($1,$2,NOW()) ON CONFLICT DO NOTHING`, blocker, blocked)
	require.NoError(t, err)
}

func seedFPS(t *testing.T, f *occurrenceE2EFixture, sellerID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	pid := uuid.New()
	_, err := f.appDB.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'p','','','same_day',NOW(),NOW())`, pid, sellerID)
	require.NoError(t, err)
	id := uuid.New()
	_, err = f.appDB.Pool().Exec(ctx, `INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, created_at, updated_at) VALUES ($1,$2,$3,10000,$4::for_sale_status_enum,NOW(),NOW())`, id, pid, sellerID, status)
	require.NoError(t, err)
	return id
}

func seedSellerSub(t *testing.T, f *occurrenceE2EFixture, sellerID uuid.UUID, status string) {
	t.Helper()
	pid := uuid.New()
	// Insert a minimal payment row for FK
	_, _ = f.appDB.Pool().Exec(context.Background(), `INSERT INTO payments (id, user_id, payment_number, midtrans_order_id, gross_amount, service_fee_amount, coin_discount_amount, coins_to_use, reference_type, reference_id, expired_at, created_at, updated_at) VALUES ($1,$2,'sub-1','mid-1',0,0,0,0,'subscription',$3,NOW()+INTERVAL'1y',NOW(),NOW()) ON CONFLICT DO NOTHING`, pid, sellerID, uuid.New())
	_, err := f.appDB.Pool().Exec(context.Background(), `INSERT INTO seller_subscriptions (id, user_id, status, started_at, expires_at, duration_days, amount_paid, payment_id, created_at, updated_at) VALUES ($1,$2,$3,NOW(),NOW()+INTERVAL'1y',365,0,$4,NOW(),NOW())`, uuid.New(), sellerID, status, pid)
	require.NoError(t, err)
}

func seedAuction(t *testing.T, f *occurrenceE2EFixture, sellerID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	pid := uuid.New()
	_, err := f.appDB.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'ap','','','same_day',NOW(),NOW())`, pid, sellerID)
	require.NoError(t, err)
	id := uuid.New()
	_, err = f.appDB.Pool().Exec(ctx, `INSERT INTO auctions (id, product_id, seller_id, start_price, bid_increment, start_at, end_at, status, created_at, updated_at) VALUES ($1,$2,$3,10000,1000,NOW(),NOW()+INTERVAL'7d',$4::auction_status_enum,NOW(),NOW())`, id, pid, sellerID, status)
	require.NoError(t, err)
	return id
}

// ============================================================================
// SHARE E2E Matrix
// ============================================================================

func TestOccurrenceE2E_Share_Profile_Success(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	pid := seedProfile(t, f, "sp")
	msg, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, pid)
	require.NoError(t, err)
	f.assertOccurrence(t, msg.ID, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, pid)
	assert.Equal(t, 1, f.cntOcc(t))
	assert.Equal(t, 1, f.cntOutbox(t))
}

func TestOccurrenceE2E_Share_Profile_Blocked_Rejected(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	pid := seedProfile(t, f, "bp")
	seedBlock(t, f, f.sender, pid)
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, pid)
	require.Error(t, err)
	assert.Equal(t, 0, f.cntOcc(t))
}

func TestOccurrenceE2E_Share_Content_Public(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	cid := seedContent(t, f, f.sender, "public", false, false)
	msg, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, cid)
	require.NoError(t, err)
	f.assertOccurrence(t, msg.ID, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, cid)
}

func TestOccurrenceE2E_Share_Content_Private_Author(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	cid := seedContent(t, f, f.sender, "private", false, false)
	msg, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, cid)
	require.NoError(t, err)
	f.assertOccurrence(t, msg.ID, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, cid)
}

func TestOccurrenceE2E_Share_Content_Private_NonAuthor(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	oa := seedProfile(t, f, "oa")
	cid := seedContent(t, f, oa, "private", false, false)
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, cid)
	require.Error(t, err)
	assert.Equal(t, 0, f.cntOcc(t))
}

func TestOccurrenceE2E_Share_Content_Followers_Follower(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	oa := seedProfile(t, f, "fa")
	seedFollow(t, f, f.sender, oa)
	cid := seedContent(t, f, oa, "followers_only", false, false)
	msg, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, cid)
	require.NoError(t, err)
	f.assertOccurrence(t, msg.ID, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, cid)
}

func TestOccurrenceE2E_Share_Content_Followers_NonFollower(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	oa := seedProfile(t, f, "nf")
	cid := seedContent(t, f, oa, "followers_only", false, false)
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, cid)
	require.Error(t, err)
	assert.Equal(t, 0, f.cntOcc(t))
}

func TestOccurrenceE2E_Share_Content_Hidden(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	cid := seedContent(t, f, f.sender, "public", true, false)
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, cid)
	require.Error(t, err)
	assert.Equal(t, 0, f.cntOcc(t))
}

func TestOccurrenceE2E_Share_Content_Deleted(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	cid := seedContent(t, f, f.sender, "public", false, true)
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, cid)
	require.Error(t, err)
	assert.Equal(t, 0, f.cntOcc(t))
}

func TestOccurrenceE2E_Share_Content_Blocked(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	oa := seedProfile(t, f, "ba")
	seedBlock(t, f, f.sender, oa)
	cid := seedContent(t, f, oa, "public", false, false)
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, cid)
	require.Error(t, err)
	assert.Equal(t, 0, f.cntOcc(t))
}

func TestOccurrenceE2E_Share_FPS(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	fpsID := seedFPS(t, f, f.sender, "active")
	msg, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
	require.NoError(t, err)
	f.assertOccurrence(t, msg.ID, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
}

func TestOccurrenceE2E_Share_Auction(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	aucID := seedAuction(t, f, f.sender, "active")
	msg, err := f.send(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, aucID)
	require.NoError(t, err)
	f.assertOccurrence(t, msg.ID, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, aucID)
}

// ============================================================================
// DIRECT E2E Matrix
// ============================================================================

func TestOccurrenceE2E_Direct_FPS_Success(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	seedSellerSub(t, f, f.sender, "active")
	fpsID := seedFPS(t, f, f.sender, "active")
	msg, err := f.send(t, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
	require.NoError(t, err)
	f.assertOccurrence(t, msg.ID, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
}

func TestOccurrenceE2E_Direct_Auction_Scheduled(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	seedSellerSub(t, f, f.sender, "active")
	aucID := seedAuction(t, f, f.sender, "scheduled")
	msg, err := f.send(t, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeAuction, aucID)
	require.NoError(t, err)
	f.assertOccurrence(t, msg.ID, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeAuction, aucID)
}

func TestOccurrenceE2E_Direct_Auction_Active(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	seedSellerSub(t, f, f.sender, "active")
	aucID := seedAuction(t, f, f.sender, "active")
	msg, err := f.send(t, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeAuction, aucID)
	require.NoError(t, err)
	f.assertOccurrence(t, msg.ID, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeAuction, aucID)
}

func TestOccurrenceE2E_Direct_ForeignFPS(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	fpsID := seedFPS(t, f, f.other, "active")
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
	require.Error(t, err)
	assert.ErrorIs(t, err, chatRepo.ErrNotResourceOwner)
	assert.Equal(t, 0, f.cntOcc(t))
}

func TestOccurrenceE2E_Direct_ForeignAuction(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	aucID := seedAuction(t, f, f.other, "active")
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeAuction, aucID)
	require.Error(t, err)
	assert.ErrorIs(t, err, chatRepo.ErrNotResourceOwner)
	assert.Equal(t, 0, f.cntOcc(t))
}

func TestOccurrenceE2E_Direct_NonRepostableFPS(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	fpsID := seedFPS(t, f, f.sender, "draft") // draft is not repostable
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
	require.Error(t, err)
	assert.ErrorIs(t, err, chatRepo.ErrResourceNotPromotable)
	assert.Equal(t, 0, f.cntOcc(t))
}

func TestOccurrenceE2E_Direct_NonRepostableAuction(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	aucID := seedAuction(t, f, f.sender, "draft") // draft is not repostable
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeAuction, aucID)
	require.Error(t, err)
	assert.ErrorIs(t, err, chatRepo.ErrResourceNotPromotable)
	assert.Equal(t, 0, f.cntOcc(t))
}

func TestOccurrenceE2E_Direct_NoMarketCapability_Rejected(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	// Sender has no seller subscription — market capability check should fail
	fpsID := seedFPS(t, f, f.sender, "active")
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
	require.Error(t, err)
	assert.Equal(t, 0, f.cntOcc(t))
}

func TestOccurrenceE2E_Direct_Profile_Rejected(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	pid := seedProfile(t, f, "dp")
	_, err := f.send(t, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeProfile, pid)
	require.Error(t, err)
	assert.Equal(t, 0, f.cntOcc(t))
}

// ============================================================================
// Idempotency A-E
// ============================================================================

func TestOccurrenceE2E_Idempotency_A_SameActorKeySameOccurrence(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	fpsID := seedFPS(t, f, f.sender, "active")
	key := uuid.NewString()
	msg1, err := f.sendWithKey(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID, key)
	require.NoError(t, err)
	bm, bo, bs := f.cntMsg(t), f.cntOcc(t), f.cntOutbox(t)
	msg2, err := f.sendWithKey(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID, key)
	require.NoError(t, err)
	assert.Equal(t, msg1.ID, msg2.ID)
	assert.Equal(t, bm, f.cntMsg(t))
	assert.Equal(t, bo, f.cntOcc(t))
	assert.Equal(t, bs, f.cntOutbox(t))
}

func TestOccurrenceE2E_Idempotency_B_DifferentResource(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	fps1 := seedFPS(t, f, f.sender, "active")
	fps2 := seedFPS(t, f, f.sender, "active")
	key := uuid.NewString()
	_, err := f.sendWithKey(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fps1, key)
	require.NoError(t, err)
	bo := f.cntOcc(t)
	_, err = f.sendWithKey(t, chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fps2, key)
	require.Error(t, err)
	assert.ErrorIs(t, err, chatRepo.ErrIdempotencyConflict)
	assert.Equal(t, bo, f.cntOcc(t))
}

func TestOccurrenceE2E_Idempotency_C_DifferentOperation(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	fpsID := seedFPS(t, f, f.sender, "active")
	key := uuid.NewString()
	occShare := &chatEntity.ResourceOccurrenceIdentity{Operation: chatEntity.ResourceOccurrenceOperationShareToChat, ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale, ResourceID: fpsID}
	occDirect := &chatEntity.ResourceOccurrenceIdentity{Operation: chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale, ResourceID: fpsID}
	body := "e2e"
	_, err := f.svc.SendMessage(context.Background(), f.room, f.sender, chatEntity.MessageTypeText, &body, nil, nil, nil, key, occShare)
	require.NoError(t, err)
	bo := f.cntOcc(t)
	_, err = f.svc.SendMessage(context.Background(), f.room, f.sender, chatEntity.MessageTypeText, &body, nil, nil, nil, key, occDirect)
	require.Error(t, err)
	assert.ErrorIs(t, err, chatRepo.ErrIdempotencyConflict)
	assert.Equal(t, bo, f.cntOcc(t))
}

func TestOccurrenceE2E_Idempotency_D_Concurrent(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	fpsID := seedFPS(t, f, f.sender, "active")
	key := uuid.NewString()
	occ := &chatEntity.ResourceOccurrenceIdentity{Operation: chatEntity.ResourceOccurrenceOperationShareToChat, ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale, ResourceID: fpsID}
	body := "e2e"
	var wg sync.WaitGroup
	var m1, m2 *chatEntity.ChatMessage
	var e1, e2 error
	wg.Add(2)
	go func() {
		defer wg.Done()
		b := body
		m1, e1 = f.svc.SendMessage(context.Background(), f.room, f.sender, chatEntity.MessageTypeText, &b, nil, nil, nil, key, occ)
	}()
	go func() {
		defer wg.Done()
		b := body
		m2, e2 = f.svc.SendMessage(context.Background(), f.room, f.sender, chatEntity.MessageTypeText, &b, nil, nil, nil, key, occ)
	}()
	wg.Wait()
	require.NoError(t, e1)
	require.NoError(t, e2)
	assert.Equal(t, m1.ID, m2.ID)
	assert.Equal(t, 1, f.cntOcc(t))
	assert.Equal(t, 1, f.cntOutbox(t))
}

func TestOccurrenceE2E_Idempotency_E_DifferentActors(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	fps1 := seedFPS(t, f, f.sender, "active")
	fps2 := seedFPS(t, f, f.other, "active")
	key := uuid.NewString()
	occ1 := &chatEntity.ResourceOccurrenceIdentity{Operation: chatEntity.ResourceOccurrenceOperationShareToChat, ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale, ResourceID: fps1}
	occ2 := &chatEntity.ResourceOccurrenceIdentity{Operation: chatEntity.ResourceOccurrenceOperationShareToChat, ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale, ResourceID: fps2}
	body := "e2e"
	var wg sync.WaitGroup
	var mA, mB *chatEntity.ChatMessage
	var eA, eB error
	wg.Add(2)
	go func() {
		defer wg.Done()
		b := body
		mA, eA = f.svc.SendMessage(context.Background(), f.room, f.sender, chatEntity.MessageTypeText, &b, nil, nil, nil, key, occ1)
	}()
	go func() {
		defer wg.Done()
		b := body
		mB, eB = f.svc.SendMessage(context.Background(), f.room, f.other, chatEntity.MessageTypeText, &b, nil, nil, nil, key, occ2)
	}()
	wg.Wait()
	require.NoError(t, eA)
	require.NoError(t, eB)
	assert.NotEqual(t, mA.ID, mB.ID)
	assert.Equal(t, 2, f.cntOcc(t))
	assert.Equal(t, 2, f.cntOutbox(t))
	assert.Equal(t, f.sender, mA.SenderID)
	assert.Equal(t, f.other, mB.SenderID)
}

// ============================================================================
// Replay after authority change
// ============================================================================

func TestOccurrenceE2E_ReplayAfterLifecycleChange(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	seedSellerSub(t, f, f.sender, "active")
	fpsID := seedFPS(t, f, f.sender, "active")
	key := uuid.NewString()
	occ := &chatEntity.ResourceOccurrenceIdentity{Operation: chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale, ResourceID: fpsID}
	body := "replay-lifecycle"

	msg1, err := f.svc.SendMessage(context.Background(), f.room, f.sender, chatEntity.MessageTypeText, &body, nil, nil, nil, key, occ)
	require.NoError(t, err)
	mid, mc, oc, sc := msg1.ID, f.cntMsg(t), f.cntOcc(t), f.cntOutbox(t)

	// Mutate FPS to non-repostable state — a NEW direct command would now fail
	ctx := context.Background()
	_, err = f.appDB.Pool().Exec(ctx, `UPDATE for_sales SET status='draft' WHERE id=$1`, fpsID)
	require.NoError(t, err)

	// Verify a new command would be rejected
	_, err = f.sendWithKey(t, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID, uuid.NewString())
	require.Error(t, err, "new command after lifecycle change must fail")

	// Replay exact original — must return original message, not re-authorize
	msg2, err := f.svc.SendMessage(context.Background(), f.room, f.sender, chatEntity.MessageTypeText, &body, nil, nil, nil, key, occ)
	require.NoError(t, err)
	assert.Equal(t, mid, msg2.ID, "replay must return same message")
	assert.Equal(t, mc, f.cntMsg(t), "message count unchanged")
	assert.Equal(t, oc, f.cntOcc(t), "occurrence count unchanged")
	assert.Equal(t, sc, f.cntOutbox(t), "outbox count unchanged")
}

func TestOccurrenceE2E_LegacyReferenceRejectedBeforePersistence(t *testing.T) {
	f := newOccurrenceE2EFixture(t)
	fpsID := seedFPS(t, f, f.sender, "active")

	body := "canonical-first"
	key := uuid.NewString()
	_, err := f.svc.SendMessage(
		context.Background(),
		f.room,
		f.sender,
		chatEntity.MessageTypeText,
		&body,
		nil,
		nil,
		nil,
		key,
		&chatEntity.ResourceOccurrenceIdentity{
			Operation:    chatEntity.ResourceOccurrenceOperationShareToChat,
			ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale,
			ResourceID:   fpsID,
		},
	)
	require.NoError(t, err)

	msgBefore, occBefore, sentBefore := f.cntMsg(t), f.cntOcc(t), f.cntOutbox(t)
	var outboxBefore int
	require.NoError(t, f.appDB.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM outbox`).Scan(&outboxBefore))

	legacyBody := "legacy-reference"
	legacyKey := uuid.NewString()
	_, err = f.svc.SendMessage(
		context.Background(),
		f.room,
		f.sender,
		chatEntity.MessageTypeText,
		&legacyBody,
		map[string]interface{}{
			"type": "reference",
			"data": map[string]interface{}{
				"target_type": "for_sale",
				"target_id":   fpsID.String(),
				"preview": map[string]interface{}{
					"title": "Legacy for_sale",
				},
			},
		},
		nil,
		nil,
		legacyKey,
		nil,
	)
	require.ErrorIs(t, err, chatRepo.ErrInvalidAttachment)

	var outboxAfter int
	require.NoError(t, f.appDB.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM outbox`).Scan(&outboxAfter))
	assert.Equal(t, msgBefore, f.cntMsg(t), "chat_messages must not change")
	assert.Equal(t, occBefore, f.cntOcc(t), "chat_message_resource_occurrences must not change")
	assert.Equal(t, sentBefore, f.cntOutbox(t), "chat.message.sent must not change")
	assert.Equal(t, outboxBefore, outboxAfter, "total outbox rows must not change")
}

// ============================================================================
// Atomic rollback
// ============================================================================

type failingOccurrenceRepo struct {
	chatRepo.Repository
	failCount int
}

func (r *failingOccurrenceRepo) CreateResourceOccurrence(ctx context.Context, tx interface{}, occ *chatEntity.ChatMessageResourceOccurrence) error {
	if r.failCount > 0 {
		r.failCount--
		return fmt.Errorf("injected occurrence failure")
	}
	return r.Repository.CreateResourceOccurrence(ctx, tx, occ)
}

func TestOccurrenceE2E_AtomicRollback(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	appDB := db.NewFromPool(tdb.Pool())
	realRepo := chatInfraRepo.NewChatRepository()
	failRepo := &failingOccurrenceRepo{Repository: realRepo, failCount: 1}
	svc := NewService(appDB, failRepo, socialInfraRepo.NewSocialRepository(), &countingOutbox{db: appDB}, rate.NewRateLimiter(), nil, nil, nil, nil, nil, &productionAuthorizer{fb: NewOccurrenceFallbackBuilders(&defaultProfileFallbackBuilder{}, &defaultContentFallbackBuilder{}, &defaultFPSFallbackBuilder{}, &defaultAuctionFallbackBuilder{}), socialRepo: socialInfraRepo.NewSocialRepository()}, zap.NewNop())
	svc.fallbackBuilders = NewOccurrenceFallbackBuilders(&defaultProfileFallbackBuilder{}, &defaultContentFallbackBuilder{}, &defaultFPSFallbackBuilder{}, &defaultAuctionFallbackBuilder{})

	ctx := context.Background()
	sender := insertChatAuthorityUser(t, ctx, appDB, "ar-s")
	other := insertChatAuthorityUser(t, ctx, appDB, "ar-o")
	room := insertChatAuthorityRoom(t, ctx, appDB, sender, other)

	fpsID := seedFPS(t, &occurrenceE2EFixture{appDB: appDB}, sender, "active")
	body := "rollback"
	key := uuid.NewString()
	occ := &chatEntity.ResourceOccurrenceIdentity{Operation: chatEntity.ResourceOccurrenceOperationShareToChat, ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale, ResourceID: fpsID}

	_, err := svc.SendMessage(ctx, room, sender, chatEntity.MessageTypeText, &body, nil, nil, nil, key, occ)
	require.Error(t, err)
	require.Contains(t, err.Error(), "injected occurrence failure")

	var msgCnt, occCnt, outboxCnt int
	appDB.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM chat_messages WHERE room_id=$1`, room).Scan(&msgCnt)
	appDB.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM chat_message_resource_occurrences`).Scan(&occCnt)
	appDB.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE event_type='chat.message.sent'`).Scan(&outboxCnt)
	assert.Equal(t, 0, msgCnt)
	assert.Equal(t, 0, occCnt)
	assert.Equal(t, 0, outboxCnt)
}
