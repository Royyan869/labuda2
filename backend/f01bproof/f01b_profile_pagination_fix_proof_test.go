//go:build integration

package f01bproof

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentEntity "github.com/labuda/backend/internal/social/content/entity"

	feedApp "github.com/labuda/backend/internal/social/feed/application"
	feedRepo "github.com/labuda/backend/internal/social/feed/infrastructure/repository"
	feedHTTP "github.com/labuda/backend/internal/social/feed/delivery/http"

	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

// ======================================================================
// SECTION 3: POSTGRESQL REGRESSION — 6 items, limit=3, all traversed
// ======================================================================

func TestPaginationFix_6ItemsAllTraversed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	authorID := seedPU(t, ctx, appDB, "pgfix_author")
	now := time.Now().UTC()

	// 6 contents with distinct timestamps (1-minute intervals)
	contentIDs := make([]uuid.UUID, 6)
	for i := 0; i < 6; i++ {
		contentIDs[i] = seedPContent(t, ctx, appDB, authorID,
			fmt.Sprintf("PGFIX Content %d", i), "public", now.Add(-time.Duration(i)*time.Minute))
	}

	t.Logf("=== FIXTURE: 6 content IDs ===")
	for i, cid := range contentIDs {
		t.Logf("  [%d] %s (created_at = %s)", i, cid, now.Add(-time.Duration(i)*time.Minute).Format(time.RFC3339))
	}

	svc := newProfileContentService()

	// ===== PAGE 1: limit=3 =====
	var page1 []*contentEntity.Content
	var cursor1 string
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		page1, cursor1, err = svc.ListByAuthor(ctx, tx, authorID, authorID, 3, "")
		return err
	}))

	t.Logf("=== PAGE 1 ===")
	t.Logf("  Items: %d, cursor: %s", len(page1), cursor1)
	require.Equal(t, 3, len(page1), "page 1 must return exactly 3 items")
	require.NotEmpty(t, cursor1, "page 1 must have cursor (has_more)")

	// Verify Page 1 contains the 3 most recent
	page1IDs := make(map[uuid.UUID]bool)
	for _, c := range page1 {
		page1IDs[c.ID] = true
	}
	assert.True(t, page1IDs[contentIDs[0]], "Page 1 must contain content 0 (most recent)")
	assert.True(t, page1IDs[contentIDs[1]], "Page 1 must contain content 1")
	assert.True(t, page1IDs[contentIDs[2]], "Page 1 must contain content 2")

	// ===== PAGE 2: limit=3, cursor from page 1 =====
	var page2 []*contentEntity.Content
	var cursor2 string
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		page2, cursor2, err = svc.ListByAuthor(ctx, tx, authorID, authorID, 3, cursor1)
		return err
	}))

	t.Logf("=== PAGE 2 ===")
	t.Logf("  Items: %d, cursor: '%s'", len(page2), cursor2)
	require.Equal(t, 3, len(page2), "page 2 must return exactly 3 items (FIX VERIFICATION)")
	assert.Empty(t, cursor2, "page 2 must be terminal")

	page2IDs := make(map[uuid.UUID]bool)
	for _, c := range page2 {
		page2IDs[c.ID] = true
	}

	// Verify Page 2 contains the 3 oldest
	assert.True(t, page2IDs[contentIDs[3]], "Page 2 must contain content 3")
	assert.True(t, page2IDs[contentIDs[4]], "Page 2 must contain content 4")
	assert.True(t, page2IDs[contentIDs[5]], "Page 2 must contain content 5")

	// ===== NO OVERLAP =====
	overlap := 0
	for id := range page1IDs {
		if page2IDs[id] {
			overlap++
		}
	}
	assert.Equal(t, 0, overlap, "zero overlap between pages")
	t.Log("  ✓ Zero overlap")

	// ===== COMPLETE COVERAGE =====
	totalIDs := make(map[uuid.UUID]bool)
	for id := range page1IDs {
		totalIDs[id] = true
	}
	for id := range page2IDs {
		totalIDs[id] = true
	}
	for i, cid := range contentIDs {
		assert.True(t, totalIDs[cid], "content %d (%s) must be found", i, cid)
	}
	t.Log("  ✓ All 6 content IDs found across both pages")

	// ===== ORDERING =====
	for i := 0; i < len(page1)-1; i++ {
		assert.False(t, page1[i].CreatedAt.Before(page1[i+1].CreatedAt),
			"page 1 must be created_at DESC")
	}
	for i := 0; i < len(page2)-1; i++ {
		assert.False(t, page2[i].CreatedAt.Before(page2[i+1].CreatedAt),
			"page 2 must be created_at DESC")
	}
	// Page 1 last item must be newer than Page 2 first item
	assert.True(t, page1[len(page1)-1].CreatedAt.After(page2[0].CreatedAt) ||
		page1[len(page1)-1].CreatedAt.Equal(page2[0].CreatedAt),
		"page 1 last must be >= page 2 first")
	t.Log("  ✓ Ordering: created_at DESC across pages")

	t.Log("")
	t.Log("=== FIX VERIFIED: 6/6 items traversed, 0 missing, 0 duplicate ===")
}

// ======================================================================
// SECTION 4: BOUNDARY TESTS
// ======================================================================

func TestPaginationFix_LessThanLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	authorID := seedPU(t, ctx, appDB, "pgfix_less")
	now := time.Now().UTC()
	id1 := seedPContent(t, ctx, appDB, authorID, "Less 1", "public", now)
	id2 := seedPContent(t, ctx, appDB, authorID, "Less 2", "public", now.Add(-1*time.Minute))

	svc := newProfileContentService()
	var items []*contentEntity.Content
	var cursor string
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		items, cursor, err = svc.ListByAuthor(ctx, tx, authorID, authorID, 3, "")
		return err
	}))

	t.Logf("2 items, limit=3: %d items, cursor='%s'", len(items), cursor)
	assert.Equal(t, 2, len(items), "must return 2 items")
	assert.Empty(t, cursor, "terminal — no cursor")

	ids := make(map[uuid.UUID]bool)
	for _, c := range items {
		ids[c.ID] = true
	}
	assert.True(t, ids[id1])
	assert.True(t, ids[id2])
	t.Log("  ✓ Less than limit: 2 items, terminal")
}

func TestPaginationFix_ExactlyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	authorID := seedPU(t, ctx, appDB, "pgfix_exact")
	now := time.Now().UTC()
	ids := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		ids[i] = seedPContent(t, ctx, appDB, authorID, fmt.Sprintf("Exact %d", i), "public", now.Add(-time.Duration(i)*time.Minute))
	}

	svc := newProfileContentService()
	var items []*contentEntity.Content
	var cursor string
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		items, cursor, err = svc.ListByAuthor(ctx, tx, authorID, authorID, 3, "")
		return err
	}))

	t.Logf("3 items, limit=3: %d items, cursor='%s'", len(items), cursor)
	// When result == limit, there may or may not be more (the LIMIT+1 probe
	// determines this). With exactly 3 items and limit=3, the probe fetches 4,
	// which returns only 3. So has_more=false, cursor empty.
	assert.Equal(t, 3, len(items), "must return 3 items")
	assert.Empty(t, cursor, "terminal — no cursor when exactly limit items")
	t.Log("  ✓ Exactly limit: 3 items, terminal")
}

func TestPaginationFix_MoreThanLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	authorID := seedPU(t, ctx, appDB, "pgfix_more")
	now := time.Now().UTC()
	ids := make([]uuid.UUID, 4)
	for i := 0; i < 4; i++ {
		ids[i] = seedPContent(t, ctx, appDB, authorID, fmt.Sprintf("More %d", i), "public", now.Add(-time.Duration(i)*time.Minute))
	}

	svc := newProfileContentService()

	// Page 1
	var page1 []*contentEntity.Content
	var cursor1 string
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		page1, cursor1, err = svc.ListByAuthor(ctx, tx, authorID, authorID, 3, "")
		return err
	}))
	t.Logf("Page 1: %d items, cursor present=%v", len(page1), cursor1 != "")
	assert.Equal(t, 3, len(page1))
	assert.NotEmpty(t, cursor1)

	// Page 2
	var page2 []*contentEntity.Content
	var cursor2 string
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		page2, cursor2, err = svc.ListByAuthor(ctx, tx, authorID, authorID, 3, cursor1)
		return err
	}))
	t.Logf("Page 2: %d items, cursor='%s'", len(page2), cursor2)
	assert.Equal(t, 1, len(page2), "page 2 must have exactly 1 remaining item")
	assert.Empty(t, cursor2, "terminal")

	// Total
	totalIDs := make(map[uuid.UUID]bool)
	for _, c := range page1 {
		totalIDs[c.ID] = true
	}
	for _, c := range page2 {
		totalIDs[c.ID] = true
	}
	for i, id := range ids {
		assert.True(t, totalIDs[id], "content %d must be found", i)
	}
	t.Log("  ✓ More than limit: 3+1=4, all found")
}

func TestPaginationFix_MultiPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	authorID := seedPU(t, ctx, appDB, "pgfix_multi")
	now := time.Now().UTC()

	// 7 items
	ids := make([]uuid.UUID, 7)
	for i := 0; i < 7; i++ {
		ids[i] = seedPContent(t, ctx, appDB, authorID, fmt.Sprintf("Multi %d", i), "public", now.Add(-time.Duration(i)*time.Minute))
	}

	svc := newProfileContentService()

	// Traverse all pages
	allIDs := make(map[uuid.UUID]bool)
	var cursor string
	pageNum := 0
	for {
		var items []*contentEntity.Content
		require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
			var err error
			items, cursor, err = svc.ListByAuthor(ctx, tx, authorID, authorID, 3, cursor)
			return err
		}))
		pageNum++
		t.Logf("Page %d: %d items, has_more=%v", pageNum, len(items), cursor != "")
		require.Greater(t, len(items), 0, "page %d must not be empty", pageNum)
		for _, c := range items {
			allIDs[c.ID] = true
		}
		if cursor == "" {
			break
		}
	}

	// All 7 found
	for i, id := range ids {
		assert.True(t, allIDs[id], "content %d must be found", i)
	}
	assert.Equal(t, 7, len(allIDs), "must have exactly 7 unique IDs")
	t.Logf("  ✓ Multi-page: 7 items across %d pages, all found, %d unique IDs", pageNum, len(allIDs))
}

// ======================================================================
// SECTION 5: SAME-TIMESTAMP SAFETY
// ======================================================================

func TestPaginationFix_SameTimestamp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	authorID := seedPU(t, ctx, appDB, "pgfix_same_ts")
	sameTime := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)

	// 4 items with identical created_at, different IDs
	ids := make([]uuid.UUID, 4)
	for i := 0; i < 4; i++ {
		ids[i] = seedPContent(t, ctx, appDB, authorID, fmt.Sprintf("SameTS %d", i), "public", sameTime)
	}

	svc := newProfileContentService()

	// Page 1: limit=2
	var page1 []*contentEntity.Content
	var cursor1 string
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		page1, cursor1, err = svc.ListByAuthor(ctx, tx, authorID, authorID, 2, "")
		return err
	}))
	t.Logf("Same-timestamp page 1: %d items, cursor present=%v", len(page1), cursor1 != "")

	// Page 1 must return exactly 2
	require.Equal(t, 2, len(page1))

	// Page 2: using cursor
	if cursor1 != "" {
		var page2 []*contentEntity.Content
		var cursor2 string
		require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
			var err error
			page2, cursor2, err = svc.ListByAuthor(ctx, tx, authorID, authorID, 2, cursor1)
			return err
		}))
		t.Logf("Same-timestamp page 2: %d items, has_more=%v", len(page2), cursor2 != "")

		// Collect all IDs
		allIDs := make(map[uuid.UUID]bool)
		for _, c := range page1 {
			allIDs[c.ID] = true
		}
		for _, c := range page2 {
			allIDs[c.ID] = true
		}


		// With composite cursor (created_at|id), all same-timestamp items
		// must be found deterministically.
		assert.Equal(t, 4, len(allIDs), "must find all 4 same-timestamp items")
		t.Logf("  ✓ All 4 same-timestamp items traversed")
	} else {
		// All 4 returned in one page (possible if DB sorts them together)
		t.Logf("  All items in page 1 (cursor empty)")
	}
}

// ======================================================================
// SECTION 6: HTTP PAGE 1 → PAGE 2
// ======================================================================

func TestPaginationFix_HTTPPage1ToPage2(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	authorID := seedPU(t, ctx, appDB, "pgfix_http")
	viewerID := seedPU(t, ctx, appDB, "pgfix_http_viewer")
	seedPFollow(t, ctx, appDB, viewerID, authorID)

	now := time.Now().UTC()
	contentIDs := make([]uuid.UUID, 6)
	for i := 0; i < 6; i++ {
		contentIDs[i] = seedPContent(t, ctx, appDB, authorID,
			fmt.Sprintf("HTTP PGFIX %d", i), "public", now.Add(-time.Duration(i)*time.Minute))
	}

	handler := newProfileHandler(appDB)
	router := gin.New()
	router.GET("/api/v1/users/:id/contents", func(c *gin.Context) {
		c.Set("user_id", viewerID)
		handler.GetUserContent(c)
	})

	// ===== HTTP PAGE 1 =====
	t.Log("=== HTTP PAGE 1 ===")
	req1 := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/users/%s/contents?limit=3", authorID), nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	var body1 map[string]interface{}
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &body1))
	data1 := body1["data"].(map[string]interface{})
	items1 := data1["data"].([]interface{})
	hasMore1, _ := data1["has_more"].(bool)
	cursor1, _ := data1["next_cursor"].(string)

	t.Logf("  Items: %d, has_more: %v", len(items1), hasMore1)
	assert.Equal(t, 3, len(items1), "page 1 must have 3 items")
	assert.True(t, hasMore1, "page 1 must have more")
	require.NotEmpty(t, cursor1, "page 1 must have cursor")

	page1IDs := make(map[string]bool)
	for _, raw := range items1 {
		item := raw.(map[string]interface{})
		page1IDs[item["id"].(string)] = true
		t.Logf("  [%s] type=%s author_id=%s", item["id"], item["type"], item["author_id"])
	}

	// ===== HTTP PAGE 2 =====
	t.Log("=== HTTP PAGE 2 ===")
	encodedCursor := url.QueryEscape(cursor1)
	req2 := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/users/%s/contents?limit=3&cursor=%s", authorID, encodedCursor), nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var body2 map[string]interface{}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &body2))
	data2 := body2["data"].(map[string]interface{})
	items2 := data2["data"].([]interface{})
	hasMore2, _ := data2["has_more"].(bool)

	t.Logf("  Items: %d, has_more: %v", len(items2), hasMore2)
	assert.Equal(t, 3, len(items2), "page 2 must have 3 items (FIX VERIFICATION)")
	assert.False(t, hasMore2, "page 2 must be terminal")

	page2IDs := make(map[string]bool)
	for _, raw := range items2 {
		item := raw.(map[string]interface{})
		page2IDs[item["id"].(string)] = true
		t.Logf("  [%s] type=%s", item["id"], item["type"])
	}

	// ===== CROSS-PAGE =====
	overlap := 0
	for id := range page1IDs {
		if page2IDs[id] {
			overlap++
		}
	}
	assert.Equal(t, 0, overlap, "no overlap")
	t.Logf("  Overlap: %d", overlap)

	totalIDs := make(map[string]bool)
	for id := range page1IDs {
		totalIDs[id] = true
	}
	for id := range page2IDs {
		totalIDs[id] = true
	}
	for _, cid := range contentIDs {
		assert.True(t, totalIDs[cid.String()], "HTTP: content %s must be found", cid)
	}

	t.Log("  ✓ HTTP Page 1 + Page 2: all 6 content IDs found")
	t.Log("  ✓ Fix verified via actual HTTP responses")
}

// ======================================================================
// SECTION 7: HOME ↔ PROFILE SMOKE REGRESSION
// ======================================================================

func TestPaginationFix_HomeProfileSmoke(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	viewerID := seedPU(t, ctx, appDB, "pgfix_smoke_viewer")
	authorID := seedPU(t, ctx, appDB, "pgfix_smoke_author")
	seedSellerProfile(t, ctx, appDB, authorID, "Smoke Farm")
	seedSellerSubscription(t, ctx, appDB, authorID)
	seedPFollow(t, ctx, appDB, viewerID, authorID)

	now := time.Now().UTC()
	content1 := seedPContent(t, ctx, appDB, authorID, "Smoke Content 1", "public", now.Add(-1*time.Minute))
	content2 := seedPContent(t, ctx, appDB, authorID, "Smoke Content 2", "public", now.Add(-2*time.Minute))

	// Home Feed
	feedService := feedApp.NewFeedService(feedRepo.NewFeedRepository())
	feedHandler := feedHTTP.NewFeedHandler(feedService, appDB, zap.NewNop(), nil, nil)
	feedRouter := gin.New()
	feedRouter.GET("/api/v1/feed", func(c *gin.Context) {
		c.Set("user_id", viewerID)
		feedHandler.GetFeed(c)
	})
	fRec := httptest.NewRecorder()
	feedRouter.ServeHTTP(fRec, httptest.NewRequest(http.MethodGet, "/api/v1/feed?limit=20", nil))
	var fBody map[string]interface{}
	require.NoError(t, json.Unmarshal(fRec.Body.Bytes(), &fBody))
	fItems := fBody["data"].(map[string]interface{})["data"].([]interface{})
	fIDs := make(map[string]bool)
	for _, raw := range fItems {
		if id, ok := raw.(map[string]interface{})["id"].(string); ok {
			fIDs[id] = true
		}
	}

	// Author Profile (page 1)
	profileHandler := newProfileHandler(appDB)
	pRouter := gin.New()
	pRouter.GET("/api/v1/users/:id/contents", func(c *gin.Context) {
		c.Set("user_id", viewerID)
		profileHandler.GetUserContent(c)
	})
	pRec := httptest.NewRecorder()
	pRouter.ServeHTTP(pRec, httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/users/%s/contents?limit=20", authorID), nil))
	var pBody map[string]interface{}
	require.NoError(t, json.Unmarshal(pRec.Body.Bytes(), &pBody))
	pItems := pBody["data"].(map[string]interface{})["data"].([]interface{})
	pIDs := make(map[string]bool)
	for _, raw := range pItems {
		if id, ok := raw.(map[string]interface{})["id"].(string); ok {
			pIDs[id] = true
		}
	}

	// Verify same IDs
	for _, cid := range []uuid.UUID{content1, content2} {
		assert.True(t, fIDs[cid.String()], "Home Feed: content %s present", cid)
		assert.True(t, pIDs[cid.String()], "Profile: content %s present", cid)
	}
	t.Log("  ✓ Home ↔ Profile: same Content IDs after pagination fix")
	t.Log("  ✓ Pagination fix does not affect Content authority")
}
