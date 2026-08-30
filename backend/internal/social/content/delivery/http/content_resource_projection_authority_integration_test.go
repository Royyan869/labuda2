//go:build integration

package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	contententity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

type contentHTTPResponseEnvelope struct {
	Success   bool           `json:"success"`
	Data      map[string]any `json:"data"`
	Timestamp string         `json:"timestamp"`
}

func TestContentResourceProjectionAuthority_GetContentMatrix(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	gin.SetMode(gin.TestMode)

	t.Run("ordinary content no projection green", func(t *testing.T) {
		authorID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		contentID := createOrdinaryContentRow(t, ctx, tdb, handler, authorID, "plain content")

		data := getContentData(t, handler, viewerContext{userID: authorID}, contentID)
		if _, ok := data["resource_projection"]; ok {
			t.Fatal("resource_projection must be omitted for ordinary content without canonical occurrence")
		}
		if _, ok := data["share_reference"]; ok {
			t.Fatal("share_reference must be omitted for ordinary content without legacy snapshot")
		}
	})

	t.Run("profile live canonical wins over legacy blob", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		targetID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		contentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, viewerID, "profile share", &contententity.ContentResourceOccurrenceIdentity{
			Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
			ResourceType: contententity.ContentResourceOccurrenceResourceTypeProfile,
			ResourceID:   targetID,
		})
		setLegacyShareReference(t, ctx, tdb.Pool(), contentID, contententity.NewShareReferenceFromProfile(
			uuid.NewString(),
			"legacy profile title",
			"https://example.com/legacy-profile.jpg",
			false,
		))

		data := getContentData(t, handler, viewerContext{userID: viewerID}, contentID)
		assertProjectionLive(t, data, contententity.ContentResourceOccurrenceResourceTypeProfile, targetID)
		if _, ok := data["share_reference"]; ok {
			t.Fatal("share_reference must be omitted when canonical resource_projection is present")
		}
	})

	t.Run("profile tombstone beats legacy blob", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		targetID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		contentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, viewerID, "profile tombstone", &contententity.ContentResourceOccurrenceIdentity{
			Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
			ResourceType: contententity.ContentResourceOccurrenceResourceTypeProfile,
			ResourceID:   targetID,
		})
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
				VALUES ($1, $2, NOW())
			`, viewerID, targetID)
			return err
		}))
		setLegacyShareReference(t, ctx, tdb.Pool(), contentID, contententity.NewShareReferenceFromProfile(
			uuid.NewString(),
			"attractive legacy profile",
			"https://example.com/legacy-profile-leak.jpg",
			false,
		))

		data := getContentData(t, handler, viewerContext{userID: viewerID}, contentID)
		assertProjectionTombstone(t, data, contententity.ContentResourceOccurrenceResourceTypeProfile, targetID)
		assertNoLegacyLeak(t, data, "attractive legacy profile", "https://example.com/legacy-profile-leak.jpg")
	})

	t.Run("content live canonical wins over legacy blob", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		targetAuthorID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		targetContentID := createOrdinaryContentRow(t, ctx, tdb, handler, targetAuthorID, "target content")
		contentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, viewerID, "content share", &contententity.ContentResourceOccurrenceIdentity{
			Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
			ResourceType: contententity.ContentResourceOccurrenceResourceTypeContent,
			ResourceID:   targetContentID,
		})
		setLegacyShareReference(t, ctx, tdb.Pool(), contentID, contententity.NewShareReferenceFromContent(
			uuid.NewString(),
			"legacy content title",
			"https://example.com/legacy-content.jpg",
			false,
		))

		data := getContentData(t, handler, viewerContext{userID: viewerID}, contentID)
		assertProjectionLive(t, data, contententity.ContentResourceOccurrenceResourceTypeContent, targetContentID)
		if _, ok := data["share_reference"]; ok {
			t.Fatal("share_reference must be omitted when canonical resource_projection is present")
		}
	})

	t.Run("content tombstone beats legacy blob", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		targetAuthorID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		targetContentID := createOrdinaryContentRow(t, ctx, tdb, handler, targetAuthorID, "blocked target content")
		contentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, viewerID, "content tombstone", &contententity.ContentResourceOccurrenceIdentity{
			Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
			ResourceType: contententity.ContentResourceOccurrenceResourceTypeContent,
			ResourceID:   targetContentID,
		})
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
				VALUES ($1, $2, NOW())
			`, viewerID, targetAuthorID)
			return err
		}))
		setLegacyShareReference(t, ctx, tdb.Pool(), contentID, contententity.NewShareReferenceFromContent(
			uuid.NewString(),
			"content leak title",
			"https://example.com/content-leak.jpg",
			false,
		))

		data := getContentData(t, handler, viewerContext{userID: viewerID}, contentID)
		assertProjectionTombstone(t, data, contententity.ContentResourceOccurrenceResourceTypeContent, targetContentID)
		assertNoLegacyLeak(t, data, "content leak title", "https://example.com/content-leak.jpg")
	})

	t.Run("fixed price sale live canonical wins over legacy blob", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		sellerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		saleID := seedVisibilityHTTPForSale(t, ctx, appDB, sellerID)
		contentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, viewerID, "fps share", &contententity.ContentResourceOccurrenceIdentity{
			Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
			ResourceType: contententity.ContentResourceOccurrenceResourceTypeForSale,
			ResourceID:   saleID,
		})
		setLegacyShareReference(t, ctx, tdb.Pool(), contentID, contententity.NewShareReferenceFromForSale(
			uuid.NewString(),
			"legacy fps title",
			"https://example.com/legacy-fps.jpg",
			true,
			false,
			false,
		))

		data := getContentData(t, handler, viewerContext{userID: viewerID}, contentID)
		assertProjectionLive(t, data, contententity.ContentResourceOccurrenceResourceTypeForSale, saleID)
	})

	t.Run("fixed price sale tombstone beats legacy blob when seller is blocked", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		sellerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		saleID := seedVisibilityHTTPForSale(t, ctx, appDB, sellerID)
		contentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, viewerID, "fps tombstone", &contententity.ContentResourceOccurrenceIdentity{
			Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
			ResourceType: contententity.ContentResourceOccurrenceResourceTypeForSale,
			ResourceID:   saleID,
		})
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
				VALUES ($1, $2, NOW())
			`, viewerID, sellerID)
			return err
		}))
		setLegacyShareReference(t, ctx, tdb.Pool(), contentID, contententity.NewShareReferenceFromForSale(
			uuid.NewString(),
			"legacy fps leak",
			"https://example.com/legacy-fps-leak.jpg",
			true,
			false,
			false,
		))

		data := getContentData(t, handler, viewerContext{userID: viewerID}, contentID)
		assertProjectionTombstone(t, data, contententity.ContentResourceOccurrenceResourceTypeForSale, saleID)
		assertNoLegacyLeak(t, data, "legacy fps leak", "https://example.com/legacy-fps-leak.jpg")
	})

	t.Run("fixed price sale missing source tombstones despite legacy blob", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		contentID, missingSaleID := createContentWithMissingForSaleOccurrence(t, ctx, tdb, handler, viewerID, "fps missing source")
		setLegacyShareReference(t, ctx, tdb.Pool(), contentID, contententity.NewShareReferenceFromForSale(
			uuid.NewString(),
			"legacy fps missing source",
			"https://example.com/legacy-fps-missing.jpg",
			true,
			false,
			false,
		))

		data := getContentData(t, handler, viewerContext{userID: viewerID}, contentID)
		assertProjectionTombstone(t, data, contententity.ContentResourceOccurrenceResourceTypeForSale, missingSaleID)
		assertNoLegacyLeak(t, data, "legacy fps missing source", "https://example.com/legacy-fps-missing.jpg")
	})

	t.Run("auction live canonical wins over legacy blob", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		sellerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		auctionID := seedVisibilityHTTPAuction(t, ctx, appDB, sellerID)
		contentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, viewerID, "auction share", &contententity.ContentResourceOccurrenceIdentity{
			Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
			ResourceType: contententity.ContentResourceOccurrenceResourceTypeAuction,
			ResourceID:   auctionID,
		})
		setLegacyShareReference(t, ctx, tdb.Pool(), contentID, contententity.NewShareReferenceFromAuction(
			uuid.NewString(),
			"legacy auction title",
			"https://example.com/legacy-auction.jpg",
			true,
			false,
			false,
		))

		data := getContentData(t, handler, viewerContext{userID: viewerID}, contentID)
		assertProjectionLive(t, data, contententity.ContentResourceOccurrenceResourceTypeAuction, auctionID)
	})

	t.Run("auction tombstone beats legacy blob when seller is blocked", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		sellerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		auctionID := seedVisibilityHTTPAuction(t, ctx, appDB, sellerID)
		contentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, viewerID, "auction tombstone", &contententity.ContentResourceOccurrenceIdentity{
			Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
			ResourceType: contententity.ContentResourceOccurrenceResourceTypeAuction,
			ResourceID:   auctionID,
		})
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
				VALUES ($1, $2, NOW())
			`, viewerID, sellerID)
			return err
		}))
		setLegacyShareReference(t, ctx, tdb.Pool(), contentID, contententity.NewShareReferenceFromAuction(
			uuid.NewString(),
			"legacy auction leak",
			"https://example.com/legacy-auction-leak.jpg",
			true,
			false,
			false,
		))

		data := getContentData(t, handler, viewerContext{userID: viewerID}, contentID)
		assertProjectionTombstone(t, data, contententity.ContentResourceOccurrenceResourceTypeAuction, auctionID)
		assertNoLegacyLeak(t, data, "legacy auction leak", "https://example.com/legacy-auction-leak.jpg")
	})

	t.Run("viewer-blocked profile tombstones canonically", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		targetID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		contentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, viewerID, "blocked profile", &contententity.ContentResourceOccurrenceIdentity{
			Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
			ResourceType: contententity.ContentResourceOccurrenceResourceTypeProfile,
			ResourceID:   targetID,
		})
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
				VALUES ($1, $2, NOW())
			`, viewerID, targetID)
			return err
		}))

		data := getContentData(t, handler, viewerContext{userID: viewerID}, contentID)
		assertProjectionTombstone(t, data, contententity.ContentResourceOccurrenceResourceTypeProfile, targetID)
	})
}

func TestContentResourceProjectionAuthority_Depth1NestedResource(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)

	viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
	outerAuthorID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
	innerAuthorID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
	deepTargetID := seedVisibilityHTTPUser(t, ctx, appDB, "active")

	deepContentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, innerAuthorID, "deep content", &contententity.ContentResourceOccurrenceIdentity{
		Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
		ResourceType: contententity.ContentResourceOccurrenceResourceTypeProfile,
		ResourceID:   deepTargetID,
	})
	innerContentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, outerAuthorID, "inner content", &contententity.ContentResourceOccurrenceIdentity{
		Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
		ResourceType: contententity.ContentResourceOccurrenceResourceTypeContent,
		ResourceID:   deepContentID,
	})
	outerContentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, viewerID, "outer content", &contententity.ContentResourceOccurrenceIdentity{
		Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
		ResourceType: contententity.ContentResourceOccurrenceResourceTypeContent,
		ResourceID:   innerContentID,
	})

	data := getContentData(t, handler, viewerContext{userID: viewerID}, outerContentID)
	proj := mustMap(t, data, "resource_projection")
	if got := mustString(t, proj, "state"); got != "LIVE" {
		t.Fatalf("outer projection state = %q; want LIVE", got)
	}
	if got := mustString(t, proj, "resource_type"); got != string(contententity.ContentResourceOccurrenceResourceTypeContent) {
		t.Fatalf("outer projection resource_type = %q; want content", got)
	}
	if got := mustString(t, proj, "resource_id"); got != innerContentID.String() {
		t.Fatalf("outer projection resource_id = %q; want %s", got, innerContentID.String())
	}
	payload := mustMap(t, proj, "content")
	nested := mustMap(t, payload, "nested_resource")
	if got := mustString(t, nested, "resource_type"); got != string(contententity.ContentResourceOccurrenceResourceTypeContent) {
		t.Fatalf("nested resource_type = %q; want content", got)
	}
	if got := mustString(t, nested, "resource_id"); got != deepContentID.String() {
		t.Fatalf("nested resource_id = %q; want %s", got, deepContentID.String())
	}
	if _, ok := nested["nested_resource"]; ok {
		t.Fatal("nested_resource must not recurse beyond depth 1")
	}
}

func TestContentResourceProjectionAuthority_CreateAndUpdateResponses(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	gin.SetMode(gin.TestMode)

	t.Run("create response emits canonical resource_projection", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		targetID := seedVisibilityHTTPUser(t, ctx, appDB, "active")

		body := `{
			"caption":"create canonical response",
			"resource_occurrence":{
				"operation":"share_to_feed",
				"resource_type":"profile",
				"resource_id":"` + targetID.String() + `"
			}
		}`
		w := invokeContentRequest(t, handler, http.MethodPost, "/api/v1/contents", body, viewerID, "create-canonical-resource-projection")
		require.Equal(t, http.StatusCreated, w.Code)

		data := decodeContentEnvelope(t, w.Body.Bytes()).Data
		assertProjectionLive(t, data, contententity.ContentResourceOccurrenceResourceTypeProfile, targetID)
		if _, ok := data["share_reference"]; ok {
			t.Fatal("create response must not emit share_reference when canonical resource_projection exists")
		}
	})

	t.Run("update preserves canonical projection and occurrence", func(t *testing.T) {
		viewerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		targetID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
		contentID := createCanonicalContentWithOccurrence(t, ctx, tdb, handler, viewerID, "update target", &contententity.ContentResourceOccurrenceIdentity{
			Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
			ResourceType: contententity.ContentResourceOccurrenceResourceTypeProfile,
			ResourceID:   targetID,
		})

		body := `{"caption":"updated caption"}`
		w := invokeContentRequest(t, handler, http.MethodPut, "/api/v1/contents/"+contentID.String(), body, viewerID, "update-canonical-resource-projection")
		require.Equal(t, http.StatusOK, w.Code)

		data := decodeContentEnvelope(t, w.Body.Bytes()).Data
		assertProjectionLive(t, data, contententity.ContentResourceOccurrenceResourceTypeProfile, targetID)
		if _, ok := data["share_reference"]; ok {
			t.Fatal("update response must not reconstruct legacy share_reference when canonical projection exists")
		}

		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			occ, err := repository.NewContentRepository().GetResourceOccurrenceByContentID(ctx, tx, contentID)
			if err != nil {
				return err
			}
			if occ.SourceID() != targetID {
				t.Fatalf("occurrence source = %s; want %s", occ.SourceID(), targetID)
			}
			return nil
		}))
	})
}

func createOrdinaryContentRow(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	handler *ContentHandler,
	authorID uuid.UUID,
	caption string,
) uuid.UUID {
	t.Helper()

	var contentID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		content, err := handler.contentService.CreateContent(
			ctx,
			tx,
			authorID,
			caption,
			contententity.VisibilityPublic,
			nil,
			nil,
			nil,
			nil,
			nil,
		)
		if err != nil {
			return err
		}
		contentID = content.ID
		return nil
	}))
	return contentID
}

func createCanonicalContentWithOccurrence(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	handler *ContentHandler,
	authorID uuid.UUID,
	caption string,
	occurrence *contententity.ContentResourceOccurrenceIdentity,
) uuid.UUID {
	t.Helper()

	var contentID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {			content, err := handler.contentService.CreateContentWithResourceOccurrence(
				ctx,
				tx,
				authorID,
				caption,
				contententity.VisibilityPublic,
				nil,
				nil,
				occurrence,
				nil,
				nil,
			)
		if err != nil {
			return err
		}
		contentID = content.ID
		return nil
	}))
	return contentID
}

func createContentWithMissingForSaleOccurrence(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	handler *ContentHandler,
	authorID uuid.UUID,
	caption string,
) (uuid.UUID, uuid.UUID) {
	t.Helper()

	var contentID uuid.UUID
	missingSaleID := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		content, err := handler.contentService.CreateContent(
			ctx,
			tx,
			authorID,
			caption,
			contententity.VisibilityPublic,
			nil,
			nil,
			nil,
			nil,
			nil,
		)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role = replica`); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO content_resource_occurrences (
				content_id, actor_id, operation,
				profile_source_id, content_source_id,
				for_sale_source_id, auction_source_id, created_at
			)
			VALUES ($1, $2, 'share_to_feed', NULL, NULL, $3, NULL, NOW())
		`, content.ID, authorID, missingSaleID)
		if err != nil {
			return err
		}
		contentID = content.ID
		return nil
	}))
	return contentID, missingSaleID
}

func setLegacyShareReference(t *testing.T, ctx context.Context, pool *pgxpool.Pool, contentID uuid.UUID, shareRef *contententity.ShareReference) {
	t.Helper()
	_ = contentID
	_ = shareRef

	var shareRefExists bool
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'contents'
			  AND column_name = 'share_reference'
		)
	`).Scan(&shareRefExists))
	require.False(t, shareRefExists, "contents.share_reference must remain absent")
}

func seedVisibilityHTTPAuction(
	t *testing.T,
	ctx context.Context,
	pool *db.DB,
	sellerID uuid.UUID,
) uuid.UUID {
	t.Helper()

	productID := uuid.New()
	auctionID := uuid.New()

	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, productID, sellerID, "Lelang", "Lelang untuk update", `[]`, "kohaku", "immediate")
	require.NoError(t, err)

	_, err = pool.Pool().Exec(ctx, `
		INSERT INTO auctions (
			id, seller_id, product_id,
			start_price, bid_increment, start_at, end_at, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, NOW() - INTERVAL '1 hour', NOW() + INTERVAL '23 hours', 'active', NOW(), NOW())
	`, auctionID, sellerID, productID, int64(250000), int64(50000))
	require.NoError(t, err)

	return auctionID
}

func getContentData(t *testing.T, handler *ContentHandler, vc viewerContext, contentID uuid.UUID) map[string]any {
	t.Helper()

	body := invokeContentRequest(t, handler, http.MethodGet, "/api/v1/contents/"+contentID.String(), "", vc.userID, "")
	if body.Code != http.StatusOK {
		t.Fatalf("get content failed: status=%d body=%s", body.Code, body.Body.String())
	}
	return decodeContentEnvelope(t, body.Body.Bytes()).Data
}

func invokeContentRequest(
	t *testing.T,
	handler *ContentHandler,
	method, path, body string,
	userID uuid.UUID,
	idempotencyKey string,
) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	c.Request = req
	if contentID := pathContentID(path); contentID != nil {
		c.Params = gin.Params{{Key: "id", Value: contentID.String()}}
	}
	if userID != uuid.Nil {
		c.Set("userID", userID)
	}

	switch method {
	case http.MethodGet:
		handler.GetContent(c)
	case http.MethodPost:
		handler.CreateContent(c)
	case http.MethodPut:
		handler.UpdateContent(c)
	default:
		t.Fatalf("unsupported method %s", method)
	}
	return w
}

func pathContentID(path string) *uuid.UUID {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return nil
	}
	raw := parts[len(parts)-1]
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &id
}

func decodeContentEnvelope(t *testing.T, raw []byte) contentHTTPResponseEnvelope {
	t.Helper()

	var envelope contentHTTPResponseEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	return envelope
}

func mustMap(t *testing.T, raw map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := raw[key]
	if !ok {
		t.Fatalf("missing %q", key)
	}
	m, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%q is %T; want map[string]any", key, value)
	}
	return m
}

func mustString(t *testing.T, raw map[string]any, key string) string {
	t.Helper()

	value, ok := raw[key]
	if !ok {
		t.Fatalf("missing %q", key)
	}
	s, ok := value.(string)
	if !ok {
		t.Fatalf("%q is %T; want string", key, value)
	}
	return s
}

func assertProjectionLive(t *testing.T, data map[string]any, wantType contententity.ContentResourceOccurrenceResourceType, wantID uuid.UUID) {
	t.Helper()

	proj := mustMap(t, data, "resource_projection")
	if got := mustString(t, proj, "state"); got != "LIVE" {
		t.Fatalf("resource_projection.state = %q; want LIVE", got)
	}
	if got := mustString(t, proj, "resource_type"); got != string(wantType) {
		t.Fatalf("resource_projection.resource_type = %q; want %q", got, wantType)
	}
	if got := mustString(t, proj, "resource_id"); got != wantID.String() {
		t.Fatalf("resource_projection.resource_id = %q; want %s", got, wantID.String())
	}
	if _, ok := data["share_reference"]; ok {
		t.Fatal("share_reference must be omitted when canonical resource_projection is present")
	}
}

func assertProjectionTombstone(t *testing.T, data map[string]any, wantType contententity.ContentResourceOccurrenceResourceType, wantID uuid.UUID) {
	t.Helper()

	proj := mustMap(t, data, "resource_projection")
	if got := mustString(t, proj, "state"); got != "TOMBSTONE" {
		t.Fatalf("resource_projection.state = %q; want TOMBSTONE", got)
	}
	if got := mustString(t, proj, "resource_type"); got != string(wantType) {
		t.Fatalf("resource_projection.resource_type = %q; want %q", got, wantType)
	}
	if got := mustString(t, proj, "resource_id"); got != wantID.String() {
		t.Fatalf("resource_projection.resource_id = %q; want %s", got, wantID.String())
	}
	if _, ok := proj["profile"]; ok {
		t.Fatal("tombstone projection must not carry profile payload")
	}
	if _, ok := proj["content"]; ok {
		t.Fatal("tombstone projection must not carry content payload")
	}
	if _, ok := proj["for_sale"]; ok {
		t.Fatal("tombstone projection must not carry for_sale payload")
	}
	if _, ok := proj["auction"]; ok {
		t.Fatal("tombstone projection must not carry auction payload")
	}
	if _, ok := data["share_reference"]; ok {
		t.Fatal("share_reference must be omitted when canonical tombstone projection is present")
	}
}

func assertNoLegacyLeak(t *testing.T, data map[string]any, legacyTitle, legacyURL string) {
	t.Helper()

	raw, err := json.Marshal(data)
	require.NoError(t, err)
	body := string(raw)
	if strings.Contains(body, legacyTitle) {
		t.Fatalf("response leaked legacy title %q", legacyTitle)
	}
	if strings.Contains(body, legacyURL) {
		t.Fatalf("response leaked legacy URL %q", legacyURL)
	}
}

type viewerContext struct {
	userID uuid.UUID
}
