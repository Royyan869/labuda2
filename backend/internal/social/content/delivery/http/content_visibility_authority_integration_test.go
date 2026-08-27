//go:build integration

package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/evaluator"
	contentapp "github.com/labuda/backend/internal/social/content/application"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	likerepo "github.com/labuda/backend/internal/social/like/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type visibilityHTTPRoleChecker struct{}

func (visibilityHTTPRoleChecker) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

func (visibilityHTTPRoleChecker) HasActiveSellerCapability(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

func (visibilityHTTPRoleChecker) HasSellerProfile(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

type visibilityHTTPAccountChecker struct{}

func (visibilityHTTPAccountChecker) EnsureActive(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (visibilityHTTPAccountChecker) GetStatus(ctx context.Context, userID uuid.UUID) (string, error) {
	return "active", nil
}

func (visibilityHTTPAccountChecker) IsBanned(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

type visibilityHTTPLikeRepository struct{}

func (visibilityHTTPLikeRepository) InsertLike(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) error {
	return nil
}

func (visibilityHTTPLikeRepository) DeleteLike(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) error {
	return nil
}

func (visibilityHTTPLikeRepository) ExistsLike(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) (bool, error) {
	return false, nil
}

func (visibilityHTTPLikeRepository) CountLikes(ctx context.Context, tx interface{}, contentID uuid.UUID) (int, error) {
	return 0, nil
}

func (visibilityHTTPLikeRepository) GetLikeCreatedAt(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) (time.Time, error) {
	return time.Time{}, nil
}

func newVisibilityHTTPHandlerFromPool(pool *db.DB) *ContentHandler {
	return NewContentHandler(
		contentapp.NewContentService(
			contentrepo.NewContentRepository(),
			visibilityHTTPLikeRepository{},
			visibilityHTTPRoleChecker{},
			visibilityHTTPAccountChecker{},
			nil,
		),
		visibilityHTTPRoleChecker{},
		pool,
		zap.NewNop(),
		evaluator.NewContentDetailShadowRunner(zap.NewNop()).WithMode(evaluator.ContentDetailEvaluatorModeEnforce),
	)
}

func newProfileEngagementHTTPHandlerFromPool(pool *db.DB) *ContentHandler {
	return NewContentHandler(
		contentapp.NewContentService(
			contentrepo.NewContentRepository(),
			likerepo.NewLikeRepository(),
			visibilityHTTPRoleChecker{},
			visibilityHTTPAccountChecker{},
			nil,
		),
		visibilityHTTPRoleChecker{},
		pool,
		zap.NewNop(),
		evaluator.NewContentDetailShadowRunner(zap.NewNop()).WithMode(evaluator.ContentDetailEvaluatorModeEnforce),
	)
}

func seedVisibilityHTTPUser(t *testing.T, ctx context.Context, pool *db.DB, status string) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, $4, NOW(), NOW())
	`, userID, userID.String(), userID.String()+"@test.invalid", status)
	if err == nil {
		_, err = pool.Pool().Exec(ctx, `
			INSERT INTO user_profiles (id, user_id, username, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`, uuid.New(), userID, "user-"+strings.ReplaceAll(userID.String(), "-", ""))
	}
	require.NoError(t, err)
	return userID
}

func seedVisibilityHTTPUserTx(t *testing.T, ctx context.Context, tx db.Tx, status string) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, $4, NOW(), NOW())
	`, userID, userID.String(), userID.String()+"@test.invalid", status)
	if err == nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO user_profiles (id, user_id, username, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`, uuid.New(), userID, "user-"+strings.ReplaceAll(userID.String(), "-", ""))
	}
	require.NoError(t, err)
	return userID
}

func seedVisibilityHTTPForSale(
	t *testing.T,
	ctx context.Context,
	pool *db.DB,
	sellerID uuid.UUID,
) uuid.UUID {
	t.Helper()

	productID := uuid.New()
	forSaleID := uuid.New()

	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, productID, sellerID, "Produk Dijual", "Produk untuk update", `[]`, "kohaku", "immediate")
	require.NoError(t, err)

	_, err = pool.Pool().Exec(ctx, `
		INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, published_at, quantity_available)
		VALUES ($1, $2, $3, $4, 'active', NOW(), $5)
	`, forSaleID, productID, sellerID, int64(250000), 1)
	require.NoError(t, err)

	return forSaleID
}

func TestGetContent_BlockedFollower_Returns404(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)

	var authorID uuid.UUID
	var viewerID uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		authorID = seedVisibilityHTTPUserTx(t, ctx, tx, "active")
		viewerID = seedVisibilityHTTPUserTx(t, ctx, tx, "active")
		_, err := tx.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewerID, authorID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewerID, authorID)
		return err
	})
	require.NoError(t, err)

	var contentID uuid.UUID
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		content, createErr := handler.contentService.CreateContent(
			ctx,
			tx,
			authorID,
			"blocked detail",
			contententity.VisibilityPublic,
			nil,
			nil,
			nil,
			nil,
			nil,
		)
		if createErr != nil {
			return createErr
		}
		contentID = content.ID
		return nil
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+contentID.String(), nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: contentID.String()}}
	c.Set("userID", viewerID)

	handler.GetContent(c)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "Content not found")
	require.NotContains(t, w.Body.String(), "share_reference")
	require.NotContains(t, w.Body.String(), "media")
	require.NotContains(t, w.Body.String(), "engagement")
}

func TestGetUserContent_BlockedRelationship_Returns403(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)

	var viewerID uuid.UUID
	var targetID uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		viewerID = seedVisibilityHTTPUserTx(t, ctx, tx, "active")
		targetID = seedVisibilityHTTPUserTx(t, ctx, tx, "active")
		_, err := tx.Exec(ctx, `
			INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewerID, targetID)
		return err
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+targetID.String()+"/contents", nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: targetID.String()}}
	c.Set("userID", viewerID)

	handler.GetUserContent(c)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "Content not accessible")
	require.NotContains(t, w.Body.String(), "share_reference")
	require.NotContains(t, w.Body.String(), "media")
}

func TestGetUserContent_EngagementProjectionIncludesLikeCountAndViewerState(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newProfileEngagementHTTPHandlerFromPool(appDB)

	var authorID uuid.UUID
	var viewerID uuid.UUID
	var otherLikerID uuid.UUID
	var firstContentID uuid.UUID
	var secondContentID uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		authorID = seedVisibilityHTTPUserTx(t, ctx, tx, "active")
		viewerID = seedVisibilityHTTPUserTx(t, ctx, tx, "active")
		otherLikerID = seedVisibilityHTTPUserTx(t, ctx, tx, "active")

		firstContent, err := handler.contentService.CreateContent(
			ctx,
			tx,
			authorID,
			"first profile post",
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
		firstContentID = firstContent.ID

		secondContent, err := handler.contentService.CreateContent(
			ctx,
			tx,
			authorID,
			"second profile post",
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
		secondContentID = secondContent.ID

		likeRows := []struct {
			contentID uuid.UUID
			userID    uuid.UUID
		}{
			{contentID: firstContentID, userID: viewerID},
			{contentID: firstContentID, userID: otherLikerID},
		}
		for _, row := range likeRows {
			if _, err := tx.Exec(ctx, `
				INSERT INTO content_likes (content_id, user_id)
				VALUES ($1, $2)
				ON CONFLICT (content_id, user_id) DO NOTHING
			`, row.contentID, row.userID); err != nil {
				return err
			}
		}

		topLevelCommentID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO comments (
				id, author_id, body, target_id, target_type, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, 'content', NOW(), NOW())
		`, topLevelCommentID, otherLikerID, "top level comment", firstContentID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO comments (
				id, author_id, body, target_id, target_type, parent_id, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, 'content', $5, NOW(), NOW())
		`, uuid.New(), viewerID, "reply body", firstContentID, topLevelCommentID); err != nil {
			return err
		}

		return nil
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorID.String()+"/contents", nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: authorID.String()}}
	c.Set("userID", viewerID)

	handler.GetUserContent(c)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Data []struct {
				ID         string `json:"id"`
				Caption    string `json:"caption"`
				Engagement *struct {
					LikeCount    int `json:"likeCount"`
					CommentCount int `json:"commentCount"`
				} `json:"engagement"`
				IsLiked *bool `json:"is_liked"`
			} `json:"data"`
			NextCursor *string `json:"next_cursor"`
			HasMore    bool    `json:"has_more"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Len(t, resp.Data.Data, 2)

	byCaption := map[string]struct {
		engagement *struct {
			LikeCount    int `json:"likeCount"`
			CommentCount int `json:"commentCount"`
		}
		isLiked *bool
	}{}
	for _, item := range resp.Data.Data {
		byCaption[item.Caption] = struct {
			engagement *struct {
				LikeCount    int `json:"likeCount"`
				CommentCount int `json:"commentCount"`
			}
			isLiked *bool
		}{
			engagement: item.Engagement,
			isLiked:    item.IsLiked,
		}
	}

	first := byCaption["first profile post"]
	require.NotNil(t, first.engagement)
	require.Equal(t, 2, first.engagement.LikeCount)
	require.Equal(t, 1, first.engagement.CommentCount)
	require.NotNil(t, first.isLiked)
	require.True(t, *first.isLiked)

	second := byCaption["second profile post"]
	require.NotNil(t, second.engagement)
	require.Equal(t, 0, second.engagement.LikeCount)
	require.Equal(t, 0, second.engagement.CommentCount)
	require.NotNil(t, second.isLiked)
	require.False(t, *second.isLiked)

	// RUNTIME CROSS-CHECK: API engagement.likeCount must equal
	// COUNT(content_likes) for the same content (live-count authority).
	var firstLikeRows int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM content_likes WHERE content_id = $1
	`, firstContentID).Scan(&firstLikeRows))
	require.Equal(t, 2, firstLikeRows, "API likeCount must equal COUNT(content_likes)")

	var viewerLiked int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM content_likes WHERE content_id = $1 AND user_id = $2
	`, firstContentID, viewerID).Scan(&viewerLiked))
	require.Equal(t, 1, viewerLiked, "API viewer isLiked must equal actual current-user like state")

	_ = firstContentID
	_ = secondContentID

	// OWNER PROFILE VIEW: owner sees their own content engagement and their own
	// like state (deterministic; self-likes only if the owner actually liked).
	gin.SetMode(gin.TestMode)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorID.String()+"/contents", nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: authorID.String()}}
	c.Set("userID", authorID)

	handler.GetUserContent(c)

	require.Equal(t, http.StatusOK, w.Code)
	var owner struct {
		Data struct {
			Data []struct {
				ID         string `json:"id"`
				Engagement *struct {
					LikeCount int `json:"likeCount"`
				} `json:"engagement"`
				IsLiked *bool `json:"is_liked"`
			} `json:"data"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &owner))
	require.Len(t, owner.Data.Data, 2)
	for _, it := range owner.Data.Data {
		require.NotNil(t, it.Engagement)
		require.NotNil(t, it.IsLiked, "owner profile view carries per-viewer is_liked deterministically")
	}
}

func TestCreateContent_InvalidVisibilityRejected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	userID := seedVisibilityHTTPUser(t, ctx, appDB, "active")

	cases := []struct {
		name  string
		value string
	}{
		{name: "invalid", value: "not-a-visibility"},
		{name: "empty", value: ""},
	}

	gin.SetMode(gin.TestMode)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			body := `{"caption":"invalid visibility","visibility":"` + tc.value + `"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "visibility-invalid-"+tc.name)
			c.Request = req
			c.Set("userID", userID)

			handler.CreateContent(c)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), "Invalid request")
		})
	}
}

func TestCreateContent_InvalidShareReferenceRejected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	userID := seedVisibilityHTTPUser(t, ctx, appDB, "active")

	cases := []struct {
		name string
		body string
	}{
		{
			name: "invalid_target_id",
			body: `{"caption":"invalid share","share_reference":{"targetType":"for_sale","targetId":"not-a-uuid","preview":{"title":"Produk Dijual","imageUrl":"https://example.com/sale.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}}`,
		},
		{
			name: "invalid_target_type",
			body: `{"caption":"invalid share","share_reference":{"targetType":"broken_type","targetId":"` + uuid.NewString() + `","preview":{"title":"Produk Dijual","imageUrl":"https://example.com/sale.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}}`,
		},
		{
			name: "missing_preview",
			body: `{"caption":"invalid share","share_reference":{"targetType":"for_sale","targetId":"` + uuid.NewString() + `"}}`,
		},
	}

	gin.SetMode(gin.TestMode)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "invalid-share-"+tc.name)
			c.Request = req
			c.Set("userID", userID)

			handler.CreateContent(c)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.NotEmpty(t, w.Body.String())
		})
	}
}

func TestCreateContent_NormalizesMixedMediaOrderAndPersistsPositions(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	userID := seedVisibilityHTTPUser(t, ctx, appDB, "active")

	body := `{
		"caption":"mixed order",
		"media":[
			{"url":"videos/bravo.mp4","type":"video"},
			{"url":"images/alpha.jpg","type":"image"},
			{"url":"videos/alpha.mp4","type":"video"},
			{"url":"images/bravo.jpg","type":"image"}
		]
	}`

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "mixed-order-key")
	c.Request = req
	c.Set("userID", userID)

	handler.CreateContent(c)

	require.Equal(t, http.StatusCreated, w.Code)

	var created struct {
		Success bool `json:"success"`
		Data    struct {
			ID    string `json:"id"`
			Media []struct {
				URL      string `json:"url"`
				Type     string `json:"type"`
				Position int    `json:"position"`
			} `json:"media"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.True(t, created.Success)
	require.Len(t, created.Data.Media, 4)
	require.Contains(t, created.Data.Media[0].URL, "images/alpha.jpg")
	require.Contains(t, created.Data.Media[1].URL, "images/bravo.jpg")
	require.Contains(t, created.Data.Media[2].URL, "videos/bravo.mp4")
	require.Contains(t, created.Data.Media[3].URL, "videos/alpha.mp4")
	require.Equal(t, 0, created.Data.Media[0].Position)
	require.Equal(t, 1, created.Data.Media[1].Position)
	require.Equal(t, 2, created.Data.Media[2].Position)
	require.Equal(t, 3, created.Data.Media[3].Position)
	require.Equal(t, "image", created.Data.Media[0].Type)
	require.Equal(t, "image", created.Data.Media[1].Type)
	require.Equal(t, "video", created.Data.Media[2].Type)
	require.Equal(t, "video", created.Data.Media[3].Type)

	var persisted []struct {
		MediaURL  string
		MediaType string
		Position  int
	}
	rows, err := tdb.Pool().Query(ctx, `
		SELECT media_url, media_type, position
		FROM content_media
		WHERE content_id = $1
		ORDER BY position
	`, created.Data.ID)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var row struct {
			MediaURL  string
			MediaType string
			Position  int
		}
		require.NoError(t, rows.Scan(&row.MediaURL, &row.MediaType, &row.Position))
		persisted = append(persisted, row)
	}
	require.NoError(t, rows.Err())
	require.Len(t, persisted, 4)
	require.Contains(t, persisted[0].MediaURL, "images/alpha.jpg")
	require.Contains(t, persisted[1].MediaURL, "images/bravo.jpg")
	require.Contains(t, persisted[2].MediaURL, "videos/bravo.mp4")
	require.Contains(t, persisted[3].MediaURL, "videos/alpha.mp4")
	require.Equal(t, "image", persisted[0].MediaType)
	require.Equal(t, "image", persisted[1].MediaType)
	require.Equal(t, "video", persisted[2].MediaType)
	require.Equal(t, "video", persisted[3].MediaType)
	require.Equal(t, 0, persisted[0].Position)
	require.Equal(t, 1, persisted[1].Position)
	require.Equal(t, 2, persisted[2].Position)
	require.Equal(t, 3, persisted[3].Position)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+created.Data.ID, nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: created.Data.ID}}
	c.Set("userID", userID)

	handler.GetContent(c)

	require.Equal(t, http.StatusOK, w.Code)

	var detail struct {
		Success bool `json:"success"`
		Data    struct {
			Media []struct {
				URL      string `json:"url"`
				Type     string `json:"type"`
				Position int    `json:"position"`
			} `json:"media"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
	require.True(t, detail.Success)
	require.Len(t, detail.Data.Media, 4)
	require.Equal(t, 0, detail.Data.Media[0].Position)
	require.Equal(t, 1, detail.Data.Media[1].Position)
	require.Equal(t, 2, detail.Data.Media[2].Position)
	require.Equal(t, 3, detail.Data.Media[3].Position)
	require.Equal(t, "image", detail.Data.Media[0].Type)
	require.Equal(t, "video", detail.Data.Media[2].Type)
	require.Contains(t, detail.Data.Media[0].URL, "images/alpha.jpg")
	require.Contains(t, detail.Data.Media[2].URL, "videos/bravo.mp4")
}

func TestCreateContent_InvalidMediaRejectedWithoutPublishing(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	userID := seedVisibilityHTTPUser(t, ctx, appDB, "active")

	body := `{
		"caption":"broken media",
		"media":[
			{"url":"   ","type":"image"},
			{"url":"images/ok.jpg","type":"image"}
		]
	}`

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "broken-media-key")
	c.Request = req
	c.Set("userID", userID)

	handler.CreateContent(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Invalid media")

	var contentCount int
	err := tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM contents
		WHERE author_id = $1 AND caption = 'broken media'
	`, userID).Scan(&contentCount)
	require.NoError(t, err)
	require.Equal(t, 0, contentCount)

	var mediaCount int
	err = tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM content_media cm
		JOIN contents c ON c.id = cm.content_id
		WHERE c.author_id = $1 AND c.caption = 'broken media'
	`, userID).Scan(&mediaCount)
	require.NoError(t, err)
	require.Equal(t, 0, mediaCount)
}

func TestUpdateContent_InvalidVisibilityRejected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	userID := seedVisibilityHTTPUser(t, ctx, appDB, "active")

	cases := []struct {
		name  string
		value string
	}{
		{name: "invalid", value: "not-a-visibility"},
		{name: "empty", value: ""},
	}

	gin.SetMode(gin.TestMode)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			body := `{"caption":"invalid visibility","visibility":"` + tc.value + `"}`
			contentID := uuid.New()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/contents/"+contentID.String(), strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "visibility-update-"+tc.name)
			c.Request = req
			c.Params = gin.Params{{Key: "id", Value: contentID.String()}}
			c.Set("userID", userID)

			handler.UpdateContent(c)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), "Invalid request")
		})
	}
}

func TestUpdateContent_RejectsLegacyShareReferenceAndResourceOccurrence(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	userID := seedVisibilityHTTPUser(t, ctx, appDB, "active")

	cases := []struct {
		name string
		body string
	}{
		{
			name: "share_reference",
			body: `{"caption":"updated caption","share_reference":{"targetType":"for_sale","targetId":"not-a-uuid","preview":{"title":"Produk Dijual","imageUrl":"https://example.com/sale.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}}`,
		},
		{
			name: "resource_occurrence",
			body: `{"caption":"updated caption","resource_occurrence":{"operation":"share_to_feed","resource_type":"profile","resource_id":"` + uuid.NewString() + `"}}`,
		},
	}

	var contentID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		content, err := handler.contentService.CreateContent(
			ctx,
			tx,
			userID,
			"original caption",
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

	gin.SetMode(gin.TestMode)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/contents/"+contentID.String(), strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "invalid-share-update-"+tc.name)
			c.Request = req
			c.Params = gin.Params{{Key: "id", Value: contentID.String()}}
			c.Set("userID", userID)

			handler.UpdateContent(c)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.NotEmpty(t, w.Body.String())

			var caption string
			require.NoError(t, tdb.Pool().QueryRow(ctx, `
				SELECT COALESCE(caption, '')
				FROM contents
				WHERE id = $1
			`, contentID).Scan(&caption))
			require.Equal(t, "original caption", caption)
		})
	}
}

func TestUpdateContent_PreservesCanonicalResourceOccurrenceOnCaptionOnlyUpdate(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	userID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
	targetID := seedVisibilityHTTPUser(t, ctx, appDB, "active")

	var contentID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		content, err := handler.contentService.CreateContentWithResourceOccurrence(
			ctx,
			tx,
			userID,
			"occurrence target",
			contententity.VisibilityPublic,
			nil,
			nil,
			&contententity.ContentResourceOccurrenceIdentity{
				Operation:    contententity.ContentResourceOccurrenceOperationShareToFeed,
				ResourceType: contententity.ContentResourceOccurrenceResourceTypeProfile,
				ResourceID:   targetID,
			},
			nil,
		)
		if err != nil {
			return err
		}
		contentID = content.ID
		return nil
	}))

	gin.SetMode(gin.TestMode)
	body := `{"caption":"updated caption"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/contents/"+contentID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "canonical-caption-update")
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: contentID.String()}}
	c.Set("userID", userID)

	handler.UpdateContent(c)

	require.Equal(t, http.StatusOK, w.Code)

	var caption string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT COALESCE(caption, '')
		FROM contents
		WHERE id = $1
	`, contentID).Scan(&caption))
	require.Equal(t, "updated caption", caption)

	var operation string
	var storedTargetID uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT operation, profile_source_id
		FROM content_resource_occurrences
		WHERE content_id = $1
	`, contentID).Scan(&operation, &storedTargetID))
	require.Equal(t, string(contententity.ContentResourceOccurrenceOperationShareToFeed), operation)
	require.Equal(t, targetID, storedTargetID)
}

func TestCreateContent_AcceptsActiveForSaleOwnedByAnotherSeller(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	authorID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
	sellerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
	forSaleID := seedVisibilityHTTPForSale(t, ctx, appDB, sellerID)

	body := `{
		"caption": "cross seller sale reference",
		"share_reference": {
			"targetType": "for_sale",
			"targetId": "` + forSaleID.String() + `",
			"preview": {
				"title": "Produk Dijual",
				"imageUrl": "https://example.com/sale.jpg",
				"isAvailable": true,
				"isSold": false,
				"isClosed": false,
				"isDeleted": false
			}
		}
	}`

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "cross-seller-sale-reference")
	c.Request = req
	c.Set("userID", authorID)

	handler.CreateContent(c)

	require.Equal(t, http.StatusCreated, w.Code)
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	shareRaw, ok := envelope.Data["share_reference"]
	require.True(t, ok)
	shareMap, ok := shareRaw.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "for_sale", shareMap["targetType"])
	require.Equal(t, forSaleID.String(), shareMap["targetId"])

	var contentID uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT id
		FROM contents
		WHERE author_id = $1
		ORDER BY created_at DESC
		LIMIT 1
		`, authorID).Scan(&contentID))

	var shareType, shareTargetID string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT share_reference->>'targetType', share_reference->>'targetId'
		FROM contents
		WHERE id = $1
	`, contentID).Scan(&shareType, &shareTargetID))
	require.Equal(t, "for_sale", shareType)
	require.Equal(t, forSaleID.String(), shareTargetID)

	var saleSellerID, saleStatus string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT seller_id::text, status
		FROM for_sales
		WHERE id = $1
	`, forSaleID).Scan(&saleSellerID, &saleStatus))
	require.Equal(t, sellerID.String(), saleSellerID)
	require.Equal(t, "active", saleStatus)
}

func TestCreateContent_RejectedForMissingOrWrongSaleTarget(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	authorID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
	sellerID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
	forSaleID := seedVisibilityHTTPForSale(t, ctx, appDB, sellerID)

	var productID string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT product_id::text
		FROM for_sales
		WHERE id = $1
	`, forSaleID).Scan(&productID))

	cases := []struct {
		name string
		body string
	}{
		{
			name: "nonexistent_sale",
			body: `{"caption":"missing sale","share_reference":{"targetType":"for_sale","targetId":"` + uuid.NewString() + `","preview":{"title":"Produk Dijual","imageUrl":"https://example.com/sale.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}}`,
		},
		{
			name: "product_id_as_target",
			body: `{"caption":"wrong id","share_reference":{"targetType":"for_sale","targetId":"` + productID + `","preview":{"title":"Produk Dijual","imageUrl":"https://example.com/sale.jpg","isAvailable":true,"isSold":false,"isClosed":false,"isDeleted":false}}}`,
		},
	}

	gin.SetMode(gin.TestMode)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "missing-or-wrong-"+tc.name)
			c.Request = req
			c.Set("userID", authorID)

			handler.CreateContent(c)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), "not found")
		})
	}
}
