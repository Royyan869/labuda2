//go:build integration

package serverboot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

type profileProjectionFixture struct {
	appDB    *db.DB
	traced   *db.DB
	tracer   *queryCountingTracer
	resolver *profileProjectionBatchResolver
	cleanup  func()
}

func newProfileProjectionFixture(t *testing.T) *profileProjectionFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	ctx := context.Background()

	baseCfg := *tdb.Pool().Config()
	tracer := &queryCountingTracer{}
	baseCfg.ConnConfig.Tracer = tracer

	tracedPool, err := pgxpool.NewWithConfig(ctx, &baseCfg)
	require.NoError(t, err)

	fx := &profileProjectionFixture{
		appDB:    db.NewFromPool(tdb.Pool()),
		traced:   db.NewFromPool(tracedPool),
		tracer:   tracer,
		resolver: newProfileProjectionBatchResolver(db.NewFromPool(tracedPool)),
		cleanup: func() {
			tracedPool.Close()
			cleanup()
		},
	}

	t.Cleanup(fx.cleanup)
	return fx
}

func (f *profileProjectionFixture) seedUser(t *testing.T, accountStatus string, deletedAt *time.Time) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO users (id, firebase_uid, email, account_status, deleted_at, created_at, updated_at, role)
		VALUES ($1, $2, $3, $4, $5, $6, $6, 'user')
	`, id, id.String(), id.String()+"@test.local", accountStatus, deletedAt, now)
	require.NoError(t, err)
	return id
}

func (f *profileProjectionFixture) seedProfile(t *testing.T, userID uuid.UUID, username string, avatarURL *string) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO user_profiles (id, user_id, username, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, uuid.New(), userID, username, avatarURL)
	require.NoError(t, err)
}

func (f *profileProjectionFixture) seedSellerProfile(t *testing.T, userID uuid.UUID, storeName string) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO seller_profiles (id, user_id, store_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
	`, uuid.New(), userID, storeName)
	require.NoError(t, err)
}

func (f *profileProjectionFixture) seedBlock(t *testing.T, blockerID, blockedID uuid.UUID) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, NOW())
	`, blockerID, blockedID)
	require.NoError(t, err)
}

func newProfileOccurrence(messageID, profileID uuid.UUID) *chatEntity.ChatMessageResourceOccurrence {
	return chatEntity.NewChatMessageResourceOccurrence(
		messageID,
		chatEntity.ResourceOccurrenceOperationShareToChat,
		chatEntity.ResourceOccurrenceResourceTypeProfile,
		profileID,
		json.RawMessage(`{}`),
	)
}

func (f *profileProjectionFixture) resolve(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return f.resolver.ResolveProfiles(ctx, viewerID, occurrences)
}

func requireLiveProfileProjection(t *testing.T, proj *chatApp.ResourceProjection) chatApp.ProfileLivePayload {
	t.Helper()
	require.NotNil(t, proj)
	require.Equal(t, chatApp.ProjectionStateLive, proj.State)
	require.Equal(t, chatEntity.ResourceOccurrenceResourceTypeProfile, proj.Identity.ResourceType)
	require.NotNil(t, proj.Payload)

	payload, ok := proj.Payload.(chatApp.ProfileLivePayload)
	require.True(t, ok, "expected ProfileLivePayload, got %T", proj.Payload)
	return payload
}

func requireTombstoneProfileProjection(t *testing.T, proj *chatApp.ResourceProjection) {
	t.Helper()
	require.NotNil(t, proj)
	require.Equal(t, chatApp.ProjectionStateTombstone, proj.State)
	require.Equal(t, chatEntity.ResourceOccurrenceResourceTypeProfile, proj.Identity.ResourceType)
	require.Nil(t, proj.Payload)
	require.Equal(t, uuid.Nil, proj.Identity.ResourceID)
	assert.True(t, proj.ViewerCapabilities.BlockedByTombstone)
	assert.False(t, proj.ViewerCapabilities.CanView)
	assert.False(t, proj.ViewerCapabilities.CanInteract)
}

func TestProfileProjectionResolver_MixedStatesAndPayloadContract(t *testing.T) {
	fx := newProfileProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil)
	viewerAvatar := "https://cdn.example.test/viewer.png"
	fx.seedProfile(t, viewerID, "viewer", &viewerAvatar)

	activeID := fx.seedUser(t, "active", nil)
	activeAvatar := "https://cdn.example.test/active.png"
	fx.seedProfile(t, activeID, "active-user", &activeAvatar)

	blockedByViewerID := fx.seedUser(t, "active", nil)
	fx.seedProfile(t, blockedByViewerID, "blocked-by-viewer", nil)
	fx.seedBlock(t, viewerID, blockedByViewerID)

	blocksViewerID := fx.seedUser(t, "active", nil)
	fx.seedProfile(t, blocksViewerID, "blocks-viewer", nil)
	fx.seedBlock(t, blocksViewerID, viewerID)

	suspendedID := fx.seedUser(t, "suspended", nil)
	fx.seedProfile(t, suspendedID, "suspended-user", nil)

	bannedID := fx.seedUser(t, "banned", nil)
	fx.seedProfile(t, bannedID, "banned-user", nil)

	removedAt := time.Now().UTC()
	removedID := fx.seedUser(t, "active", &removedAt)
	fx.seedProfile(t, removedID, "removed-user", nil)

	sellerID := fx.seedUser(t, "active", nil)
	fx.seedProfile(t, sellerID, "seller-user", nil)
	fx.seedSellerProfile(t, sellerID, "Labuda Farm")

	plainID := fx.seedUser(t, "active", nil)
	fx.seedProfile(t, plainID, "plain-user", nil)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newProfileOccurrence(uuid.New(), activeID),
		uuid.New(): newProfileOccurrence(uuid.New(), viewerID),
		uuid.New(): newProfileOccurrence(uuid.New(), blockedByViewerID),
		uuid.New(): newProfileOccurrence(uuid.New(), blocksViewerID),
		uuid.New(): newProfileOccurrence(uuid.New(), suspendedID),
		uuid.New(): newProfileOccurrence(uuid.New(), bannedID),
		uuid.New(): newProfileOccurrence(uuid.New(), removedID),
		uuid.New(): newProfileOccurrence(uuid.New(), sellerID),
		uuid.New(): newProfileOccurrence(uuid.New(), plainID),
	}

	// Re-key the map with stable message IDs for assertions.
	stable := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, len(occurrences))
	for msgID, occ := range occurrences {
		stable[msgID] = occ
	}

	projections, err := fx.resolve(ctx, viewerID, stable)
	require.NoError(t, err)
	require.Len(t, projections, len(stable))

	for msgID, occ := range stable {
		proj, ok := projections[msgID]
		require.True(t, ok, "missing projection for message %s", msgID)
		require.Equal(t, chatEntity.ResourceOccurrenceResourceTypeProfile, occ.ResourceType())

		sourceID := occ.SourceID()
		switch sourceID {
		case activeID:
			payload := requireLiveProfileProjection(t, proj)
			assert.Equal(t, "active-user", payload.Username)
			require.NotNil(t, payload.AvatarURL)
			assert.Equal(t, activeAvatar, *payload.AvatarURL)
			assert.Nil(t, payload.StoreName)
			assert.False(t, payload.IsSeller)
			assert.Equal(t, "active", payload.Lifecycle)
			assert.Equal(t, sourceID, proj.Identity.ResourceID)
		case viewerID:
			payload := requireLiveProfileProjection(t, proj)
			assert.Equal(t, "viewer", payload.Username)
			require.NotNil(t, payload.AvatarURL)
			assert.Equal(t, viewerAvatar, *payload.AvatarURL)
			assert.Nil(t, payload.StoreName)
			assert.False(t, payload.IsSeller)
			assert.Equal(t, "active", payload.Lifecycle)
			assert.Equal(t, sourceID, proj.Identity.ResourceID)
		case blockedByViewerID, blocksViewerID, suspendedID, bannedID, removedID:
			requireTombstoneProfileProjection(t, proj)
		case sellerID:
			payload := requireLiveProfileProjection(t, proj)
			assert.Equal(t, "seller-user", payload.Username)
			assert.Nil(t, payload.AvatarURL)
			require.NotNil(t, payload.StoreName)
			assert.Equal(t, "Labuda Farm", *payload.StoreName)
			assert.True(t, payload.IsSeller)
			assert.Equal(t, "active", payload.Lifecycle)
			assert.Equal(t, sourceID, proj.Identity.ResourceID)
		case plainID:
			payload := requireLiveProfileProjection(t, proj)
			assert.Equal(t, "plain-user", payload.Username)
			assert.Nil(t, payload.AvatarURL)
			assert.Nil(t, payload.StoreName)
			assert.False(t, payload.IsSeller)
			assert.Equal(t, "active", payload.Lifecycle)
			assert.Equal(t, sourceID, proj.Identity.ResourceID)
		default:
			t.Fatalf("unexpected source id %s", sourceID)
		}
	}
}

func TestProfileProjectionResolver_QueryCount_BoundedByDistinctProfiles(t *testing.T) {
	fx := newProfileProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil)
	fx.seedProfile(t, viewerID, "viewer", nil)

	sharedProfileID := fx.seedUser(t, "active", nil)
	fx.seedProfile(t, sharedProfileID, "shared", nil)

	one := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newProfileOccurrence(uuid.New(), sharedProfileID),
	}

	twentySame := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		twentySame[uuid.New()] = newProfileOccurrence(uuid.New(), sharedProfileID)
	}

	twentyDistinct := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		id := fx.seedUser(t, "active", nil)
		fx.seedProfile(t, id, fmt.Sprintf("user-%02d", i), nil)
		twentyDistinct[uuid.New()] = newProfileOccurrence(uuid.New(), id)
	}

	fx.tracer.reset()
	_, err := fx.resolve(ctx, viewerID, one)
	require.NoError(t, err)
	countOne := fx.tracer.value()

	fx.tracer.reset()
	_, err = fx.resolve(ctx, viewerID, twentySame)
	require.NoError(t, err)
	countSame := fx.tracer.value()

	fx.tracer.reset()
	_, err = fx.resolve(ctx, viewerID, twentyDistinct)
	require.NoError(t, err)
	countDistinct := fx.tracer.value()

	t.Logf("query counts: one=%d same=%d distinct=%d", countOne, countSame, countDistinct)
	require.Equal(t, countOne, countSame)
	require.Equal(t, countOne, countDistinct)
}

func TestProfileProjectionResolver_MissingSourceRow_IsIntegrityError(t *testing.T) {
	fx := newProfileProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil)
	fx.seedProfile(t, viewerID, "viewer", nil)

	missingProfileUserID := fx.seedUser(t, "active", nil)
	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newProfileOccurrence(uuid.New(), missingProfileUserID),
	}

	projections, err := fx.resolve(ctx, viewerID, occurrences)
	require.Error(t, err)
	require.Nil(t, projections)
	assert.Contains(t, err.Error(), "profile source row missing")
}

type profileProjectionFailingDB struct {
	base        *db.DB
	failOnQuery int
	err         error
}

func (d *profileProjectionFailingDB) WithTx(ctx context.Context, fn func(db.Tx) error) error {
	return d.base.WithTx(ctx, func(tx db.Tx) error {
		return fn(&profileProjectionFailingTx{
			Tx:          tx,
			failOnQuery: d.failOnQuery,
			err:         d.err,
		})
	})
}

type profileProjectionFailingTx struct {
	db.Tx
	failOnQuery int
	queryCount  int
	err         error
}

func (t *profileProjectionFailingTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.queryCount++
	if t.queryCount == t.failOnQuery {
		return nil, t.err
	}
	return t.Tx.Query(ctx, sql, args...)
}

var _ profileProjectionDB = (*profileProjectionFailingDB)(nil)

func TestProfileProjectionResolver_QueryFailures_Propagate(t *testing.T) {
	fx := newProfileProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil)
	fx.seedProfile(t, viewerID, "viewer", nil)
	targetID := fx.seedUser(t, "active", nil)
	fx.seedProfile(t, targetID, "target", nil)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newProfileOccurrence(uuid.New(), targetID),
	}

	t.Run("profile source query failure", func(t *testing.T) {
		resolver := newProfileProjectionBatchResolver(&profileProjectionFailingDB{
			base:        fx.appDB,
			failOnQuery: 1,
			err:         errors.New("source query boom"),
		})

		projections, err := resolver.ResolveProfiles(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "profile source batch query failed")
	})

	t.Run("block query failure", func(t *testing.T) {
		resolver := newProfileProjectionBatchResolver(&profileProjectionFailingDB{
			base:        fx.appDB,
			failOnQuery: 2,
			err:         errors.New("block query boom"),
		})

		projections, err := resolver.ResolveProfiles(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "profile block batch query failed")
	})
}
