//go:build integration

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	forsaleApp "github.com/labuda/backend/internal/commerce/forsale/application"
	shippingrepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	"github.com/labuda/backend/internal/identity/auth"
	idempotencyRepo "github.com/labuda/backend/internal/platform/idempotency/repository"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// seedSellerCapability seeds seller_profiles + a single seller_subscriptions row
// mirroring the canonical RoleCheckerDB gates (profile exists + active time-bounded
// subscription). status "active" grants capability; anything else does not.
func seedSellerCapability(t *testing.T, ctx context.Context, appDB *db.DB, userID uuid.UUID, status string) {
	t.Helper()
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO seller_profiles (id, user_id, store_name, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
			ON CONFLICT DO NOTHING
		`, uuid.New(), userID, "store-"+userID.String()[:8]); err != nil {
			return err
		}
		var interval string
		if status == "active" {
			interval = "NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days'"
		} else {
			interval = "NOW() - INTERVAL '60 days', NOW() - INTERVAL '30 days'"
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO seller_subscriptions (id, user_id, status, started_at, expires_at, duration_days, amount_paid, payment_id, created_at, updated_at)
			VALUES ($1, $2, $3, `+interval+`, 30, 0, $4, NOW(), NOW())
		`, uuid.New(), userID, status, uuid.New())
		return err
	})
	require.NoError(t, err)
}

// testCommentCommerceCapabilityHandler builds the production-shaped handler with
// the REAL canonical auth.RoleCheckerDB wired as CommentService.sellerCapabilityChecker
// (the exact instance production InitServices now receives).
func testCommentCommerceCapabilityHandler(appDB *db.DB) *CommentHandler {
	contentService := contentApp.NewContentService(
		contentrepo.NewContentRepository(),
		nil,
		commentListHTTPRoleChecker{},
		commentListHTTPAccountChecker{},
		nil,
	)
	forSaleSvc := forsaleApp.NewForSaleService(
		nil, // outboxRepo
		nil, // roleChecker
		nil, // actorResolver
		nil,
		shippingrepo.NewProductShippingOptionRepository(nil),
		nil, // coverageRepo
		nil, // shippingQuoteRepo
		nil, // addressRepo
	)
	commentService := contentApp.NewCommentService(
		contentrepo.NewContentRepository(),
		contentrepo.NewCommentRepository(),
		forSaleSvc,
		nil,                     // auctionValidator
		nil,                     // visibilityChecker
		commentWireTestOutbox{}, // outboxRepo
		idempotencyRepo.NewRepository(),
		nil,                               // blockChecker
		auth.NewRoleCheckerDB(appDB, nil), // sellerCapabilityChecker — canonical authority
		nil,                               // invariantLogger
	)
	return NewCommentHandler(
		commentService,
		contentService,
		forSaleSvc,
		commentListHTTPRoleChecker{},
		appDB,
		zap.NewNop(),
	)
}

func TestCommentCommerceReferenceWire_RealSellerCapabilityGate(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	appDB := db.NewFromPool(tdb.Pool())
	ctx := context.Background()

	authorID := seedCommentListHTTPUser(t, ctx, appDB, "cap-author")
	capableSeller := seedCommentListHTTPUser(t, ctx, appDB, "cap-seller")
	expiredSeller := seedCommentListHTTPUser(t, ctx, appDB, "cap-expired")
	seedSellerCapability(t, ctx, appDB, capableSeller, "active")
	seedSellerCapability(t, ctx, appDB, expiredSeller, "expired")

	handler := testCommentCommerceCapabilityHandler(appDB)
	contentID := seedCommentListHTTPContent(t, ctx, appDB, handler, authorID)
	fpsID := seedCommentCommerceFPS(t, ctx, appDB, capableSeller, "active")

	curUser := capableSeller
	router := gin.New()
	router.POST("/contents/:id/comments/reference", func(c *gin.Context) {
		c.Set("userID", curUser)
		handler.CreateCommerceReferenceComment(c)
	})

	reqBody := map[string]any{
		"resource_reference": map[string]any{
			"resource_type": "for_sale",
			"resource_id":   fpsID.String(),
		},
		"body": "real capability gate",
	}
	performCreate := func(key string) *httptest.ResponseRecorder {
		raw, err := json.Marshal(reqBody)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/contents/"+contentID.String()+"/comments/reference", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	// 1. Capable seller succeeds (real RoleCheckerDB gate passes).
	curUser = capableSeller
	w := performCreate("cap-key-1")
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	data := decodeWireCreateResponse(t, w)
	require.Equal(t, "for_sale", data["reference"].(map[string]any)["targetType"])
	firstID := data["id"]

	// 4. Persisted state correct: comment + commerce-reference association row.
	var persisted bool
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM comments c
				JOIN comment_commerce_references ccr ON ccr.comment_id = c.id
				WHERE c.author_id = $1 AND ccr.for_sale_id = $2
			)
		`, capableSeller, fpsID).Scan(&persisted)
	})
	require.NoError(t, err)
	require.True(t, persisted, "commerce-reference comment + association must be persisted")

	// 5. Idempotency intact: same key + same payload replays, no new row.
	wReplay := performCreate("cap-key-1")
	require.Equal(t, http.StatusCreated, wReplay.Code)
	require.Equal(t, firstID, decodeWireCreateResponse(t, wReplay)["id"])
	var commentCount int
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM comments
			WHERE author_id = $1 AND target_id = $2
		`, capableSeller, contentID).Scan(&commentCount)
	}))
	require.Equal(t, 1, commentCount)

	// 2. Expired seller is rejected by the canonical capability rule (no capability).
	curUser = expiredSeller
	wExpired := performCreate("cap-key-expired")
	require.NotEqual(t, http.StatusCreated, wExpired.Code,
		"expired seller must NOT create a commerce-reference comment")
	// Existing handler only maps expected sentinel errors; market-authority denial
	// surfaces as 500. The rule rejection is what matters here (no 201, no row).
	require.Equal(t, http.StatusInternalServerError, wExpired.Code, "body=%s", wExpired.Body.String())

	var failedCount int
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM comments
			WHERE author_id = $1 AND target_id = $2
		`, expiredSeller, contentID).Scan(&failedCount)
	}))
	require.Zero(t, failedCount, "rejected actor must leave no comment row")
}
