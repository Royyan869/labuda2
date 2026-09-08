//go:build integration

package http

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	forsalerepo "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	commerceResponse "github.com/labuda/backend/internal/commerce/response"
	contentapp "github.com/labuda/backend/internal/social/content/application"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	idempRepo "github.com/labuda/backend/internal/platform/idempotency/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newIdempotentHandlerFromPool(pool *db.DB) *ContentHandler {
	cs := contentapp.NewContentService(
		contentrepo.NewContentRepository(),
		visibilityHTTPLikeRepository{},
		visibilityHTTPRoleChecker{},
		visibilityHTTPAccountChecker{},
		nil,
	)
	cs.SetCommerceReferenceValidator(
		commerceResponse.NewValidator(
			forsalerepo.NewForSaleRepository(),
			auctionRepo.NewAuctionRepository(),
		),
	)
	cs.SetOutboxInserter(outboxRepo.NewOutboxRepository(pool))
	cs.SetIdempotencyRepository(idempRepo.NewRepository())
	return NewContentHandler(cs, visibilityHTTPRoleChecker{}, pool, zap.NewNop(), nil)
}

func seedIdempotencyUser(t *testing.T, ctx context.Context, pool *db.DB, status string) uuid.UUID {
	t.Helper()
	return seedVisibilityHTTPUser(t, ctx, pool, status)
}

func countContentsForAuthor(t *testing.T, ctx context.Context, pool *db.DB, authorID uuid.UUID) int {
	t.Helper()
	var cnt int
	err := pool.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM contents WHERE author_id=$1`, authorID).Scan(&cnt)
	require.NoError(t, err)
	return cnt
}

func countIdempotencyRecord(t *testing.T, ctx context.Context, pool *db.DB, actorID uuid.UUID, rawKey string) int {
	t.Helper()
	composite := fmt.Sprintf("content.create:%s:%s", actorID.String(), rawKey)
	var cnt int
	err := pool.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM idempotency_records WHERE idempotency_key=$1`, composite).Scan(&cnt)
	require.NoError(t, err)
	return cnt
}

func countOutboxByKeyPrefix(t *testing.T, ctx context.Context, pool *db.DB, prefix string) int {
	t.Helper()
	var cnt int
	// InsertTx prepends eventType: full key = "content.mentioned." + prefix (which already contains "content.mentioned.")
	// So use contains search on idempotency_key.
	err := pool.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE idempotency_key LIKE $1`, "%"+prefix+"%").Scan(&cnt)
	require.NoError(t, err)
	return cnt
}

func countContentMedia(t *testing.T, ctx context.Context, pool *db.DB, contentID uuid.UUID) int {
	t.Helper()
	var cnt int
	err := pool.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM content_media WHERE content_id=$1`, contentID).Scan(&cnt)
	require.NoError(t, err)
	return cnt
}

func TestContentCreateIdempotency_FirstRequestAndSequentialReplay(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newIdempotentHandlerFromPool(appDB)
	authorID := seedIdempotencyUser(t, ctx, appDB, "active")
	rawKey := "test-key-seq-" + uuid.NewString()
	var firstID uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		c, created, err := handler.contentService.CreateContentIdempotent(ctx, tx, authorID, rawKey, "hello seq", contententity.VisibilityPublic, nil, nil, nil, nil, nil)
		if err != nil {
			return err
		}
		if !created {
			return fmt.Errorf("expected created=true on first request")
		}
		firstID = c.ID
		return nil
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, firstID)
	require.Equal(t, 1, countContentsForAuthor(t, ctx, appDB, authorID))
	require.Equal(t, 1, countIdempotencyRecord(t, ctx, appDB, authorID, rawKey))

	var secondID uuid.UUID
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		c, created, err := handler.contentService.CreateContentIdempotent(ctx, tx, authorID, rawKey, "hello seq", contententity.VisibilityPublic, nil, nil, nil, nil, nil)
		if err != nil {
			return err
		}
		if created {
			return fmt.Errorf("expected created=false on replay")
		}
		secondID = c.ID
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, firstID, secondID)
	require.Equal(t, 1, countContentsForAuthor(t, ctx, appDB, authorID), "sequential retry must not create second row")
	require.Equal(t, 1, countIdempotencyRecord(t, ctx, appDB, authorID, rawKey))
}

func TestContentCreateIdempotency_DifferentKeysIndependent(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newIdempotentHandlerFromPool(appDB)
	authorID := seedIdempotencyUser(t, ctx, appDB, "active")
	key1 := "diff-key-1-" + uuid.NewString()
	key2 := "diff-key-2-" + uuid.NewString()
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, _, err := handler.contentService.CreateContentIdempotent(ctx, tx, authorID, key1, "caption 1", contententity.VisibilityPublic, nil, nil, nil, nil, nil)
		return err
	})
	require.NoError(t, err)
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, _, err := handler.contentService.CreateContentIdempotent(ctx, tx, authorID, key2, "caption 2", contententity.VisibilityPublic, nil, nil, nil, nil, nil)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, 2, countContentsForAuthor(t, ctx, appDB, authorID))
}

func TestContentCreateIdempotency_DifferentActorsNoCollision(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newIdempotentHandlerFromPool(appDB)
	actorA := seedIdempotencyUser(t, ctx, appDB, "active")
	actorB := seedIdempotencyUser(t, ctx, appDB, "active")
	rawKey := "shared-raw-key-" + uuid.NewString()
	var idA, idB uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		c, _, err := handler.contentService.CreateContentIdempotent(ctx, tx, actorA, rawKey, "from A", contententity.VisibilityPublic, nil, nil, nil, nil, nil)
		if err != nil {
			return err
		}
		idA = c.ID
		return nil
	})
	require.NoError(t, err)
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		c, _, err := handler.contentService.CreateContentIdempotent(ctx, tx, actorB, rawKey, "from B", contententity.VisibilityPublic, nil, nil, nil, nil, nil)
		if err != nil {
			return err
		}
		idB = c.ID
		return nil
	})
	require.NoError(t, err)
	require.NotEqual(t, idA, idB)
	require.Equal(t, 1, countContentsForAuthor(t, ctx, appDB, actorA))
	require.Equal(t, 1, countContentsForAuthor(t, ctx, appDB, actorB))
	require.Equal(t, 1, countIdempotencyRecord(t, ctx, appDB, actorA, rawKey))
	require.Equal(t, 1, countIdempotencyRecord(t, ctx, appDB, actorB, rawKey))
}

func TestContentCreateIdempotency_SameKeyDifferentPayloadConflict(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newIdempotentHandlerFromPool(appDB)
	authorID := seedIdempotencyUser(t, ctx, appDB, "active")
	rawKey := "conflict-key-" + uuid.NewString()
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, _, err := handler.contentService.CreateContentIdempotent(ctx, tx, authorID, rawKey, "original caption", contententity.VisibilityPublic, nil, nil, nil, nil, nil)
		return err
	})
	require.NoError(t, err)
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, _, err := handler.contentService.CreateContentIdempotent(ctx, tx, authorID, rawKey, "different caption", contententity.VisibilityPublic, nil, nil, nil, nil, nil)
		return err
	})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "idempotency conflict") || strings.Contains(strings.ToLower(err.Error()), "conflict"), "expected conflict got %v", err)
	require.Equal(t, 1, countContentsForAuthor(t, ctx, appDB, authorID))
}

func TestContentCreateIdempotency_ConcurrentRetry(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newIdempotentHandlerFromPool(appDB)
	authorID := seedIdempotencyUser(t, ctx, appDB, "active")
	rawKey := "concurrent-key-" + uuid.NewString()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	ids := make([]uuid.UUID, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := appDB.WithTx(ctx, func(tx db.Tx) error {
				c, _, err := handler.contentService.CreateContentIdempotent(ctx, tx, authorID, rawKey, "concurrent caption", contententity.VisibilityPublic, nil, nil, nil, nil, nil)
				if err != nil {
					return err
				}
				ids[idx] = c.ID
				return nil
			})
			errs[idx] = err
		}(i)
	}
	wg.Wait()
	for i, e := range errs {
		require.NoError(t, e, "concurrent idx %d should not error", i)
		require.NotEqual(t, uuid.Nil, ids[i])
	}
	require.Equal(t, 1, countContentsForAuthor(t, ctx, appDB, authorID), "concurrent retry must create exactly one row")
	require.Equal(t, ids[0], ids[1], "both concurrent callers must resolve to same content ID")
}

func TestContentCreateIdempotency_FailureRollbackNoPoison(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newIdempotentHandlerFromPool(appDB)
	authorID := seedIdempotencyUser(t, ctx, appDB, "active")
	// First attempt with invalid mention (non-existent user) should fail and not poison key
	invalidMention := uuid.New()
	rawKey := "rollback-key-" + uuid.NewString()
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		_, _, err := handler.contentService.CreateContentIdempotent(ctx, tx, authorID, rawKey, "rollback test", contententity.VisibilityPublic, nil, nil, nil, []uuid.UUID{invalidMention}, nil)
		return err
	})
	require.Error(t, err)
	require.Equal(t, 0, countContentsForAuthor(t, ctx, appDB, authorID))
	require.Equal(t, 0, countIdempotencyRecord(t, ctx, appDB, authorID, rawKey), "failed tx must not leave idempotency record")
	// Retry with valid payload and same fingerprint as originally intended (no mention) should succeed
	// Use same caption etc but without invalid mention — fingerprint differs, but since previous failed, it should be allowed
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		_, created, err := handler.contentService.CreateContentIdempotent(ctx, tx, authorID, rawKey, "rollback test", contententity.VisibilityPublic, nil, nil, nil, nil, nil)
		if err != nil {
			return err
		}
		if !created {
			return fmt.Errorf("expected created on retry after rollback")
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, countContentsForAuthor(t, ctx, appDB, authorID))
}

func TestContentCreateIdempotency_SideEffectsNotDuplicated(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newIdempotentHandlerFromPool(appDB)
	authorID := seedIdempotencyUser(t, ctx, appDB, "active")
	mentionedID := seedIdempotencyUser(t, ctx, appDB, "active")
	media := []contentapp.ContentCreateMediaInput{{URL: "https://example.com/a.jpg", Type: contententity.MediaTypeImage}, {URL: "https://example.com/b.mp4", Type: contententity.MediaTypeVideo}}
	rawKey := "sideeffect-key-" + uuid.NewString()
	tags := []string{"golang", "labuda"}
	var firstID uuid.UUID
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		c, _, err := handler.contentService.CreateContentIdempotent(ctx, tx, authorID, rawKey, "side effect cap", contententity.VisibilityPublic, nil, nil, tags, []uuid.UUID{mentionedID}, media)
		if err != nil {
			return err
		}
		firstID = c.ID
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, countContentMedia(t, ctx, appDB, firstID))
	require.Equal(t, 1, countOutboxByKeyPrefix(t, ctx, appDB, "content.mentioned."+firstID.String()))

	// Replay same key same payload
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		c, created, err := handler.contentService.CreateContentIdempotent(ctx, tx, authorID, rawKey, "side effect cap", contententity.VisibilityPublic, nil, nil, tags, []uuid.UUID{mentionedID}, media)
		if err != nil {
			return err
		}
		if created {
			return fmt.Errorf("expected replay")
		}
		if c.ID != firstID {
			return fmt.Errorf("replay ID mismatch")
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, countContentMedia(t, ctx, appDB, firstID), "replay must not duplicate media")
	require.Equal(t, 1, countOutboxByKeyPrefix(t, ctx, appDB, "content.mentioned."+firstID.String()), "replay must not duplicate mention outbox")
	require.Equal(t, 1, countContentsForAuthor(t, ctx, appDB, authorID))
}

func TestContentCreateIdempotency_WithResourceOccurrence(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newIdempotentHandlerFromPool(appDB)
	authorID := seedIdempotencyUser(t, ctx, appDB, "active")
	targetID := seedIdempotencyUser(t, ctx, appDB, "active")
	rawKey := "occ-key-" + uuid.NewString()
	occ := &contententity.ContentResourceOccurrenceIdentity{
		Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
		ResourceType: contententity.ContentResourceOccurrenceResourceTypeProfile,
		ResourceID:   targetID,
	}
	var firstID uuid.UUID
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		c, created, err := handler.contentService.CreateContentWithResourceOccurrenceIdempotent(ctx, tx, authorID, rawKey, "occ caption", contententity.VisibilityPublic, nil, nil, occ, nil, nil, nil)
		if err != nil {
			return err
		}
		if !created {
			return fmt.Errorf("expected created")
		}
		firstID = c.ID
		return nil
	})
	require.NoError(t, err)
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		c, created, err := handler.contentService.CreateContentWithResourceOccurrenceIdempotent(ctx, tx, authorID, rawKey, "occ caption", contententity.VisibilityPublic, nil, nil, occ, nil, nil, nil)
		if err != nil {
			return err
		}
		if created {
			return fmt.Errorf("expected replay")
		}
		if c.ID != firstID {
			return fmt.Errorf("id mismatch")
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, countContentsForAuthor(t, ctx, appDB, authorID))
}
