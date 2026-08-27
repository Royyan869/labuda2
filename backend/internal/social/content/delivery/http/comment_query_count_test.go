//go:build integration

package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	contentapp "github.com/labuda/backend/internal/social/content/application"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

// ── TEST-ONLY query-count tracer ─────────────────────────────────────────

// queryCountingTracer is a TEST-ONLY pgx QueryTracer that atomically counts
// every SQL query execution. It is never referenced from production packages.
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

// ── Test environment ─────────────────────────────────────────────────────

type qcEnv struct {
	t       *testing.T
	cleanup func()
	appDB   *db.DB          // standard pool for seeding
	handler *CommentHandler // handler backed by traced pool
	tracer  *queryCountingTracer
	ctx     context.Context
}

// newQcEnv provisions the database via testdb, then creates a second pool
// with the query-counting tracer attached. The handler uses the traced pool
// so that the ListComments path is measured exactly.
func newQcEnv(t *testing.T) *qcEnv {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	ctx := context.Background()

	// Clone the pool config and attach the tracer for measurement.
	baseCfg := tdb.Pool().Config()
	tracer := &queryCountingTracer{}
	baseCfg.ConnConfig.Tracer = tracer
	tracedPool, err := pgxpool.NewWithConfig(ctx, baseCfg)
	require.NoError(t, err)

	tracedDB := db.NewFromPool(tracedPool)

	contentService := contentapp.NewContentService(
		contentrepo.NewContentRepository(),
		nil,
		qcRoleChecker{}, qcAccountChecker{}, nil,
	)
	commentService := contentapp.NewCommentService(
		contentrepo.NewContentRepository(),
		contentrepo.NewCommentRepository(),
		nil, // fpsValidator
		nil, // auctionValidator
		nil, // visibilityChecker
		nil, // outboxRepo
		nil, // idempotencyRepo
		nil, // blockChecker
		nil, // sellerCapabilityChecker
		nil, // invariantLogger
	)
	handler := NewCommentHandler(commentService, contentService, tracedDB, zap.NewNop())

	env := &qcEnv{
		t: t,
		cleanup: func() {
			tracedPool.Close()
			cleanup()
		},
		appDB:   db.NewFromPool(tdb.Pool()), // use untraced pool for seeding
		handler: handler,
		tracer:  tracer,
		ctx:     ctx,
	}
	return env
}

// qcRoleChecker / qcAccountChecker are no-op implementations.
type qcRoleChecker struct{}

func (qcRoleChecker) IsAdmin(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (qcRoleChecker) HasActiveSellerCapability(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}
func (qcRoleChecker) HasSellerProfile(context.Context, uuid.UUID) (bool, error) { return false, nil }

type qcAccountChecker struct{}

func (qcAccountChecker) EnsureActive(context.Context, uuid.UUID) error        { return nil }
func (qcAccountChecker) GetStatus(context.Context, uuid.UUID) (string, error) { return "active", nil }
func (qcAccountChecker) IsBanned(context.Context, uuid.UUID) (bool, error)    { return false, nil }

// ── Seed helpers (use the untraced appDB so fixture queries don't pollute the count) ──

func (e *qcEnv) seedUser(username string) uuid.UUID {
	userID := uuid.New()
	now := time.Now().UTC()
	err := e.appDB.WithTx(e.ctx, func(tx db.Tx) error {
		_, err := tx.Exec(e.ctx, `
			INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
			VALUES ($1, $2, $3, 'active', $4, $4, 'user')
		`, userID, "fb-"+userID.String(), userID.String()+"@test.local", now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(e.ctx, `
			INSERT INTO user_profiles (id, user_id, username, avatar_url, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
		`, uuid.New(), userID, username, "https://cdn.test/avatar.png", now)
		return err
	})
	require.NoError(e.t, err)
	return userID
}

func (e *qcEnv) seedContent(authorID uuid.UUID) uuid.UUID {
	var contentID uuid.UUID
	err := e.appDB.WithTx(e.ctx, func(tx db.Tx) error {
		content, createErr := e.handler.contentService.CreateContent(
			e.ctx, tx, authorID, "test content",
			contententity.VisibilityPublic, nil, nil, nil, nil, nil,
		)
		if createErr != nil {
			return createErr
		}
		contentID = content.ID
		return nil
	})
	require.NoError(e.t, err)
	return contentID
}

func (e *qcEnv) seedProduct(sellerID uuid.UUID) uuid.UUID {
	productID := uuid.New()
	now := time.Now().UTC()
	_, err := e.appDB.Pool().Exec(e.ctx, `
		INSERT INTO products (id, seller_id, title, description, variety,
			preparation_time, media_urls, created_at, updated_at)
		VALUES ($1, $2, 'Test Product', 'Desc', 'Test Variety',
			'immediate', '[]'::jsonb, $3, $3)
	`, productID, sellerID, now)
	require.NoError(e.t, err)
	return productID
}

func (e *qcEnv) seedFPS(sellerID, productID uuid.UUID) uuid.UUID {
	fpsID := uuid.New()
	_, err := e.appDB.Pool().Exec(e.ctx, `
		INSERT INTO for_sales (id, product_id, seller_id, price_per_unit,
			negotiation_enabled, status, published_at, created_at, updated_at)
		VALUES ($1, $2, $3, 100000, false, 'active', NOW(), NOW(), NOW())
	`, fpsID, productID, sellerID)
	require.NoError(e.t, err)
	return fpsID
}

func (e *qcEnv) seedAuction(sellerID, productID uuid.UUID) uuid.UUID {
	auctionID := uuid.New()
	_, err := e.appDB.Pool().Exec(e.ctx, `
		INSERT INTO auctions (id, seller_id, product_id,
			start_price, bid_increment, start_at, end_at, status, created_at, updated_at)
		VALUES ($1, $2, $3,
			50000, 10000,
			NOW() - INTERVAL '1 hour', NOW() + INTERVAL '23 hours', 'active', NOW(), NOW())
	`, auctionID, sellerID, productID)
	require.NoError(e.t, err)
	return auctionID
}

func (e *qcEnv) seedNormalComment(authorID, contentID uuid.UUID) uuid.UUID {
	commentID := uuid.New()
	now := time.Now().UTC()
	err := e.appDB.WithTx(e.ctx, func(tx db.Tx) error {
		_, err := tx.Exec(e.ctx, `
			INSERT INTO comments (id, author_id, body, target_id, target_type, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'content', $5, $5)
		`, commentID, authorID, "normal comment", contentID, now)
		return err
	})
	require.NoError(e.t, err)
	return commentID
}

func (e *qcEnv) seedFPSCommerceRefComment(authorID, contentID, fpsID uuid.UUID) uuid.UUID {
	commentID := uuid.New()
	now := time.Now().UTC()
	err := e.appDB.WithTx(e.ctx, func(tx db.Tx) error {
		_, err := tx.Exec(e.ctx, `
			INSERT INTO comments (id, author_id, body, target_id, target_type, created_at, updated_at)
			VALUES ($1, $2, 'check this fps', $3, 'content', $4, $4)
		`, commentID, authorID, contentID, now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(e.ctx, `
			INSERT INTO comment_commerce_references (comment_id, for_sale_id, auction_id)
			VALUES ($1, $2, NULL)
		`, commentID, fpsID)
		return err
	})
	require.NoError(e.t, err)
	return commentID
}

func (e *qcEnv) seedAuctionCommerceRefComment(authorID, contentID, auctionID uuid.UUID) uuid.UUID {
	commentID := uuid.New()
	now := time.Now().UTC()
	err := e.appDB.WithTx(e.ctx, func(tx db.Tx) error {
		_, err := tx.Exec(e.ctx, `
			INSERT INTO comments (id, author_id, body, target_id, target_type, created_at, updated_at)
			VALUES ($1, $2, 'check this auction', $3, 'content', $4, $4)
		`, commentID, authorID, contentID, now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(e.ctx, `
			INSERT INTO comment_commerce_references (comment_id, for_sale_id, auction_id)
			VALUES ($1, NULL, $2)
		`, commentID, auctionID)
		return err
	})
	require.NoError(e.t, err)
	return commentID
}

// ── ListComments invocation ──────────────────────────────────────────────

func (e *qcEnv) listComments(contentID, viewerID uuid.UUID) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", viewerID)
		c.Next()
	})
	router.GET("/api/v1/contents/:id/comments", e.handler.ListComments)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/contents/"+contentID.String()+"/comments?limit=20", nil)
	router.ServeHTTP(w, req)
	return w
}

// ── Test cases ───────────────────────────────────────────────────────────

// TestCommentListQueryCount_NormalOnly measures query count for a page with
// only normal (non-commerce-reference) comments.
func TestCommentListQueryCount_NormalOnly(t *testing.T) {
	env := newQcEnv(t)
	defer env.cleanup()

	author := env.seedUser("normal-author")
	content := env.seedContent(author)
	env.seedNormalComment(author, content)
	env.seedNormalComment(author, content)

	env.tracer.reset()
	w := env.listComments(content, author)
	require.Equal(t, http.StatusOK, w.Code)

	count := env.tracer.value()
	t.Logf("NORMAL-ONLY query count: %d", count)
}

// TestCommentListQueryCount_OneFPS measures query count with 1 FPS reference.
func TestCommentListQueryCount_OneFPS(t *testing.T) {
	env := newQcEnv(t)
	defer env.cleanup()

	seller := env.seedUser("fps-seller-1")
	author := env.seedUser("fps-author-1")
	content := env.seedContent(author)
	product := env.seedProduct(seller)
	fps := env.seedFPS(seller, product)

	env.seedNormalComment(author, content)
	env.seedFPSCommerceRefComment(author, content, fps)

	env.tracer.reset()
	w := env.listComments(content, author)
	require.Equal(t, http.StatusOK, w.Code)

	count := env.tracer.value()
	t.Logf("1-FPS query count: %d", count)
}

// TestCommentListQueryCount_TwentyFPS measures query count with 20 FPS refs.
// The count MUST equal the 1-FPS count (batch query invariance).
func TestCommentListQueryCount_TwentyFPS(t *testing.T) {
	env := newQcEnv(t)
	defer env.cleanup()

	seller := env.seedUser("fps-seller-20")
	author := env.seedUser("fps-author-20")
	content := env.seedContent(author)

	env.seedNormalComment(author, content)
	for i := 0; i < 20; i++ {
		product := env.seedProduct(seller)
		fps := env.seedFPS(seller, product)
		env.seedFPSCommerceRefComment(author, content, fps)
	}

	env.tracer.reset()
	w := env.listComments(content, author)
	require.Equal(t, http.StatusOK, w.Code)

	count := env.tracer.value()
	t.Logf("20-FPS query count: %d", count)
}

// TestCommentListQueryCount_OneAuction measures query count with 1 Auction ref.
func TestCommentListQueryCount_OneAuction(t *testing.T) {
	env := newQcEnv(t)
	defer env.cleanup()

	seller := env.seedUser("auc-seller-1")
	author := env.seedUser("auc-author-1")
	content := env.seedContent(author)
	product := env.seedProduct(seller)
	auction := env.seedAuction(seller, product)

	env.seedNormalComment(author, content)
	env.seedAuctionCommerceRefComment(author, content, auction)

	env.tracer.reset()
	w := env.listComments(content, author)
	require.Equal(t, http.StatusOK, w.Code)

	count := env.tracer.value()
	t.Logf("1-AUCTION query count: %d", count)
}

// TestCommentListQueryCount_TwentyAuctions measures query count with 20 Auction refs.
// The count MUST equal the 1-Auction count (batch query invariance).
func TestCommentListQueryCount_TwentyAuctions(t *testing.T) {
	env := newQcEnv(t)
	defer env.cleanup()

	seller := env.seedUser("auc-seller-20")
	author := env.seedUser("auc-author-20")
	content := env.seedContent(author)

	env.seedNormalComment(author, content)
	for i := 0; i < 20; i++ {
		product := env.seedProduct(seller)
		auction := env.seedAuction(seller, product)
		env.seedAuctionCommerceRefComment(author, content, auction)
	}

	env.tracer.reset()
	w := env.listComments(content, author)
	require.Equal(t, http.StatusOK, w.Code)

	count := env.tracer.value()
	t.Logf("20-AUCTION query count: %d", count)
}

// TestCommentListQueryCount_OneFPSOneAuction measures query count with 1 FPS + 1 Auction.
func TestCommentListQueryCount_OneFPSOneAuction(t *testing.T) {
	env := newQcEnv(t)
	defer env.cleanup()

	seller := env.seedUser("mixed-seller-1")
	author := env.seedUser("mixed-author-1")
	content := env.seedContent(author)

	fpsProduct := env.seedProduct(seller)
	fps := env.seedFPS(seller, fpsProduct)
	aucProduct := env.seedProduct(seller)
	auction := env.seedAuction(seller, aucProduct)

	env.seedNormalComment(author, content)
	env.seedFPSCommerceRefComment(author, content, fps)
	env.seedAuctionCommerceRefComment(author, content, auction)

	env.tracer.reset()
	w := env.listComments(content, author)
	require.Equal(t, http.StatusOK, w.Code)

	count := env.tracer.value()
	t.Logf("1-FPS+1-AUCTION query count: %d", count)
}

// TestCommentListQueryCount_TwentyFPSTwentyAuctions measures query count with
// 20 FPS + 20 Auction refs. The count MUST equal the 1+1 mixed count.
func TestCommentListQueryCount_TwentyFPSTwentyAuctions(t *testing.T) {
	env := newQcEnv(t)
	defer env.cleanup()

	seller := env.seedUser("mixed-seller-20")
	author := env.seedUser("mixed-author-20")
	content := env.seedContent(author)

	env.seedNormalComment(author, content)
	for i := 0; i < 20; i++ {
		product := env.seedProduct(seller)
		fps := env.seedFPS(seller, product)
		env.seedFPSCommerceRefComment(author, content, fps)
	}
	for i := 0; i < 20; i++ {
		product := env.seedProduct(seller)
		auction := env.seedAuction(seller, product)
		env.seedAuctionCommerceRefComment(author, content, auction)
	}

	env.tracer.reset()
	w := env.listComments(content, author)
	require.Equal(t, http.StatusOK, w.Code)

	count := env.tracer.value()
	t.Logf("20-FPS+20-AUCTION query count: %d", count)
}

// TestCommentListQueryCount_Invariants verifies the boundedness invariants
// across all measured scenarios in a single test to simplify assertion.
func TestCommentListQueryCount_Invariants(t *testing.T) {
	type scenario struct {
		name string
		fn   func(t *testing.T) int64
	}

	results := make(map[string]int64)

	scenarios := []scenario{
		{"normal-only", func(t *testing.T) int64 {
			env := newQcEnv(t)
			defer env.cleanup()
			a := env.seedUser("inv-a")
			c := env.seedContent(a)
			env.seedNormalComment(a, c)
			env.seedNormalComment(a, c)
			env.tracer.reset()
			w := env.listComments(c, a)
			require.Equal(t, http.StatusOK, w.Code)
			return env.tracer.value()
		}},
		{"1-fps", func(t *testing.T) int64 {
			env := newQcEnv(t)
			defer env.cleanup()
			s := env.seedUser("inv-fs")
			a := env.seedUser("inv-fa")
			c := env.seedContent(a)
			env.seedNormalComment(a, c)
			p := env.seedProduct(s)
			f := env.seedFPS(s, p)
			env.seedFPSCommerceRefComment(a, c, f)
			env.tracer.reset()
			w := env.listComments(c, a)
			require.Equal(t, http.StatusOK, w.Code)
			return env.tracer.value()
		}},
		{"20-fps", func(t *testing.T) int64 {
			env := newQcEnv(t)
			defer env.cleanup()
			s := env.seedUser("inv-fs20")
			a := env.seedUser("inv-fa20")
			c := env.seedContent(a)
			env.seedNormalComment(a, c)
			for i := 0; i < 20; i++ {
				p := env.seedProduct(s)
				f := env.seedFPS(s, p)
				env.seedFPSCommerceRefComment(a, c, f)
			}
			env.tracer.reset()
			w := env.listComments(c, a)
			require.Equal(t, http.StatusOK, w.Code)
			return env.tracer.value()
		}},
		{"1-auction", func(t *testing.T) int64 {
			env := newQcEnv(t)
			defer env.cleanup()
			s := env.seedUser("inv-as")
			a := env.seedUser("inv-aa")
			c := env.seedContent(a)
			env.seedNormalComment(a, c)
			p := env.seedProduct(s)
			auc := env.seedAuction(s, p)
			env.seedAuctionCommerceRefComment(a, c, auc)
			env.tracer.reset()
			w := env.listComments(c, a)
			require.Equal(t, http.StatusOK, w.Code)
			return env.tracer.value()
		}},
		{"20-auctions", func(t *testing.T) int64 {
			env := newQcEnv(t)
			defer env.cleanup()
			s := env.seedUser("inv-as20")
			a := env.seedUser("inv-aa20")
			c := env.seedContent(a)
			env.seedNormalComment(a, c)
			for i := 0; i < 20; i++ {
				p := env.seedProduct(s)
				auc := env.seedAuction(s, p)
				env.seedAuctionCommerceRefComment(a, c, auc)
			}
			env.tracer.reset()
			w := env.listComments(c, a)
			require.Equal(t, http.StatusOK, w.Code)
			return env.tracer.value()
		}},
		{"1-fps+1-auction", func(t *testing.T) int64 {
			env := newQcEnv(t)
			defer env.cleanup()
			s := env.seedUser("inv-ms")
			a := env.seedUser("inv-ma")
			c := env.seedContent(a)
			env.seedNormalComment(a, c)
			p1 := env.seedProduct(s)
			f := env.seedFPS(s, p1)
			env.seedFPSCommerceRefComment(a, c, f)
			p2 := env.seedProduct(s)
			auc := env.seedAuction(s, p2)
			env.seedAuctionCommerceRefComment(a, c, auc)
			env.tracer.reset()
			w := env.listComments(c, a)
			require.Equal(t, http.StatusOK, w.Code)
			return env.tracer.value()
		}},
		{"20-fps+20-auctions", func(t *testing.T) int64 {
			env := newQcEnv(t)
			defer env.cleanup()
			s := env.seedUser("inv-ms20")
			a := env.seedUser("inv-ma20")
			c := env.seedContent(a)
			env.seedNormalComment(a, c)
			for i := 0; i < 20; i++ {
				p := env.seedProduct(s)
				f := env.seedFPS(s, p)
				env.seedFPSCommerceRefComment(a, c, f)
			}
			for i := 0; i < 20; i++ {
				p := env.seedProduct(s)
				auc := env.seedAuction(s, p)
				env.seedAuctionCommerceRefComment(a, c, auc)
			}
			env.tracer.reset()
			w := env.listComments(c, a)
			require.Equal(t, http.StatusOK, w.Code)
			return env.tracer.value()
		}},
	}

	// Execute all scenarios
	for _, sc := range scenarios {
		results[sc.name] = sc.fn(t)
		t.Logf("%s = %d queries", sc.name, results[sc.name])
	}

	// All counts must be bounded (positive and reasonable)
	for name, count := range results {
		require.Greater(t, count, int64(0), "%s: query count must be > 0", name)
		require.LessOrEqual(t, count, int64(30), "%s: query count must be bounded (≤30)", name)
	}

	// Invariant 1: batch invariance — count(1 FPS) == count(20 FPS)
	require.Equal(t, results["1-fps"], results["20-fps"],
		fmt.Sprintf("BATCH INVARIANT VIOLATED: 1-fps=%d != 20-fps=%d", results["1-fps"], results["20-fps"]))

	// Invariant 2: batch invariance — count(1 Auction) == count(20 Auctions)
	require.Equal(t, results["1-auction"], results["20-auctions"],
		fmt.Sprintf("BATCH INVARIANT VIOLATED: 1-auction=%d != 20-auctions=%d", results["1-auction"], results["20-auctions"]))

	// Invariant 3: batch invariance — count(1+1 mixed) == count(20+20 mixed)
	require.Equal(t, results["1-fps+1-auction"], results["20-fps+20-auctions"],
		fmt.Sprintf("BATCH INVARIANT VIOLATED: 1+1-mixed=%d != 20+20-mixed=%d", results["1-fps+1-auction"], results["20-fps+20-auctions"]))

	t.Log("ALL QUERY-COUNT INVARIANTS PASS")
}
