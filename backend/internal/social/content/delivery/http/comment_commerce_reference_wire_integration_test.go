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
	idempotencyRepo "github.com/labuda/backend/internal/platform/idempotency/repository"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// commentCommerceCapChecker grants active seller capability for the
// commerce-reference create path (CREATE-time market authority gate).
type commentCommerceCapChecker struct{}

func (commentCommerceCapChecker) HasActiveSellerCapability(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}

// newCommentCommerceWireHandler wires a production-shaped CommentHandler over
// a real DB: real forSaleService (live for_sale previews), real idempotency,
// no-op outbox, granted seller capability. contentService is the canonical
// real ContentService instance (visibility gate dependency).
func newCommentCommerceWireHandler(appDB *db.DB) *CommentHandler {
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
		nil,                     // visibilityChecker (falls back to contentRepo)
		commentWireTestOutbox{}, // outboxRepo (no-op)
		idempotencyRepo.NewRepository(),
		nil,                         // blockChecker
		commentCommerceCapChecker{}, // sellerCapabilityChecker
		nil,                         // invariantLogger
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

// seedCommentCommerceFPS seeds a product + fixed-price-sale row and returns
// the fixed-price-sale ID.
func seedCommentCommerceFPS(t *testing.T, ctx context.Context, appDB *db.DB, sellerID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	productID := uuid.New()
	fpsID := uuid.New()
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
			VALUES ($1, $2, 'Wire Product', 'desc', $3, 'kohaku', 'immediate')
		`, productID, sellerID, `["https://example.com/wire.jpg"]`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, published_at, quantity_available)
			VALUES ($1, $2, $3, $4, $5, NOW(), 1)
		`, fpsID, productID, sellerID, int64(100000), status)
		return err
	})
	require.NoError(t, err)
	return fpsID
}

// seedCommentCommerceRefRow inserts a commerce-reference comment row +
// association directly (used to model targets the create gate would reject,
// e.g. a withdrawn for_sale already on the wire).
func seedCommentCommerceRefRow(t *testing.T, ctx context.Context, appDB *db.DB, authorID, contentID, fpsID uuid.UUID) {
	t.Helper()
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		var commentID uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO comments (id, author_id, body, target_id, target_type, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, 'withdrawn ref', $2, 'content', NOW(), NOW())
			RETURNING id
		`, authorID, contentID).Scan(&commentID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO comment_commerce_references (comment_id, for_sale_id)
			VALUES ($1, $2)
		`, commentID, fpsID)
		return err
	})
	require.NoError(t, err)
}

func TestCommentCommerceReferenceWire_CreateListShape_SurvivesReload_NoLeak(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	appDB := db.NewFromPool(tdb.Pool())
	ctx := context.Background()

	authorID := seedCommentListHTTPUser(t, ctx, appDB, "cr-author")
	sellerID := seedCommentListHTTPUser(t, ctx, appDB, "cr-seller")

	handler := newCommentCommerceWireHandler(appDB)
	contentID := seedCommentListHTTPContent(t, ctx, appDB, handler, authorID)
	fpsID := seedCommentCommerceFPS(t, ctx, appDB, sellerID, "active")

	router := gin.New()
	router.POST("/contents/:id/comments/reference", func(c *gin.Context) {
		c.Set("userID", sellerID)
		handler.CreateCommerceReferenceComment(c)
	})
	router.GET("/contents/:id/comments", handler.ListComments)

	// C1 — create a commerce-reference comment through the real HTTP stack.
	reqBody := map[string]any{
		"resource_reference": map[string]any{
			"resource_type": "for_sale",
			"resource_id":   fpsID.String(),
		},
		"body": "check this for_sale",
	}
	raw, err := json.Marshal(reqBody)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/contents/"+contentID.String()+"/comments/reference", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "cr-key-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())

	data := decodeWireCreateResponse(t, w)
	require.Equal(t, "commerce_reference", data["type"])
	ref := data["reference"].(map[string]any)
	require.Equal(t, "for_sale", ref["targetType"])
	require.Equal(t, fpsID.String(), ref["targetId"])
	preview := ref["preview"].(map[string]any)
	require.NotEmpty(t, preview["title"], "create-time snapshot must carry the for_sale title")

	// Live for_sale preview attached to the create response.
	forSale := data["forSale"].(map[string]any)
	require.Equal(t, "Wire Product", forSale["title"])
	require.Equal(t, float64(100000), forSale["price"])
	require.Equal(t, "active", forSale["status"])

	// C2/C3 — GET list. Reference identity survives the store/reload round
	// trip; the snapshot preview is empty (only the linkage is persisted);
	// the live `forSale` preview is re-hydrated by the handler.
	wList := performWireListComments(t, router, contentID.String(), "", 20)
	require.Equal(t, http.StatusOK, wList.Code, "body=%s", wList.Body.String())
	var env struct {
		Data struct {
			Comments []map[string]any `json:"comments"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(wList.Body.Bytes(), &env))
	require.Len(t, env.Data.Comments, 1)

	first := env.Data.Comments[0]
	require.Equal(t, "commerce_reference", first["type"])
	refList := first["reference"].(map[string]any)
	require.Equal(t, "for_sale", refList["targetType"])
	require.Equal(t, fpsID.String(), refList["targetId"])
	previewList := refList["preview"].(map[string]any)
	require.Equal(t, "", previewList["title"], "reference.preview is not persisted; empty on the list surface")
	forSaleList := first["forSale"].(map[string]any)
	require.Equal(t, "Wire Product", forSaleList["title"])
	require.Equal(t, "active", forSaleList["status"])

	// C6 — a withdrawn (inaccessible/hidden) commerce target on the list still
	// carries only its canonical identity + status; the create gate plus FK
	// guarantees a comment can never reference a nonexistent/foreign for_sale.
	withdrawnFPS := seedCommentCommerceFPS(t, ctx, appDB, sellerID, "withdrawn")
	seedCommentCommerceRefRow(t, ctx, appDB, sellerID, contentID, withdrawnFPS)

	wList2 := performWireListComments(t, router, contentID.String(), "", 20)
	require.Equal(t, http.StatusOK, wList2.Code)
	var env2 struct {
		Data struct {
			Comments []map[string]any `json:"comments"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(wList2.Body.Bytes(), &env2))
	require.Len(t, env2.Data.Comments, 2)

	var withdrawnRow map[string]any
	for c := range env2.Data.Comments {
		row := env2.Data.Comments[c]
		if row["type"] == "commerce_reference" {
			if r, ok := row["reference"].(map[string]any); ok && r["targetId"] == withdrawnFPS.String() {
				withdrawnRow = row
			}
		}
	}
	require.NotNil(t, withdrawnRow, "withdrawn-ref comment must appear on the list")
	refW := withdrawnRow["reference"].(map[string]any)
	require.Equal(t, "for_sale", refW["targetType"])
	require.Equal(t, withdrawnFPS.String(), refW["targetId"])
	require.Equal(t, "", refW["preview"].(map[string]any)["title"])
	forSaleW := withdrawnRow["forSale"].(map[string]any)
	require.Equal(t, "withdrawn", forSaleW["status"], "inaccessible for_sale surfaces status so the UI fails closed")
}
