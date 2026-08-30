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
	commerceResponse "github.com/labuda/backend/internal/commerce/response"
	shippingrepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	idempotencyRepo "github.com/labuda/backend/internal/platform/idempotency/repository"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// testCommentDisplayabilityHandler builds a production-shaped handler wired
// with the canonical commerce response reference validator (existence +
// displayability only). Any user may reference any displayable commerce
// resource — no ownership or seller-capability gate is enforced.
func testCommentDisplayabilityHandler(appDB *db.DB) *CommentHandler {
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
		nil, // blockChecker
		nil, // invariantLogger
	)
	// Wire the canonical commerce response reference validator.
	// This validates existence + displayability only — no ownership or
	// seller capability check.
	commentService.SetCommerceReferenceValidator(
		commerceResponse.NewValidator(forSaleSvc, nil), // nil AuctionGetter: ForSale-only test
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

// TestCommentCommerceReference_AnyUserCanReferenceAnyDisplayableForSale proves
// that ANY authenticated user can create a commerce reference comment pointing
// at ANY displayable ForSale, regardless of who owns the ForSale or whether
// the caller has seller capability.
//
// BUSINESS TRUTH:
// - Content/Comment/Chat are display/reference surfaces, NOT commerce authority.
// - Commerce resource ownership and seller capability are irrelevant for references.
// - Only existence + displayability matter.
func TestCommentCommerceReference_AnyUserCanReferenceAnyDisplayableForSale(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	appDB := db.NewFromPool(tdb.Pool())
	ctx := context.Background()

	// Create two distinct users: one owns the ForSale, the other references it.
	contentAuthor := seedCommentListHTTPUser(t, ctx, appDB, "display-author")
	sellerWhoOwnsFPS := seedCommentListHTTPUser(t, ctx, appDB, "display-seller")
	nonSellerWhoReferences := seedCommentListHTTPUser(t, ctx, appDB, "display-ref-user")

	handler := testCommentDisplayabilityHandler(appDB)
	contentID := seedCommentListHTTPContent(t, ctx, appDB, handler, contentAuthor)

	// Seed an ACTIVE ForSale owned by sellerWhoOwnsFPS.
	fpsID := seedCommentCommerceFPS(t, ctx, appDB, sellerWhoOwnsFPS, "active")

	// nonSellerWhoReferences creates a commerce reference comment pointing at
	// the ForSale owned by sellerWhoOwnsFPS. This MUST succeed because:
	// 1. The ForSale exists
	// 2. The ForSale is displayable (active)
	// 3. Ownership is NOT checked
	// 4. Seller capability is NOT checked
	router := gin.New()
	router.POST("/contents/:id/comments/reference", func(c *gin.Context) {
		c.Set("userID", nonSellerWhoReferences)
		handler.CreateCommerceReferenceComment(c)
	})

	reqBody := map[string]any{
		"resource_reference": map[string]any{
			"resource_type": "for_sale",
			"resource_id":   fpsID.String(),
		},
		"body": "check this listing",
	}
	raw, err := json.Marshal(reqBody)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/contents/"+contentID.String()+"/comments/reference", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "display-key-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code,
		"non-owner non-seller MUST be able to reference any displayable ForSale; body=%s", w.Body.String())

	data := decodeWireCreateResponse(t, w)
	require.Equal(t, "commerce_reference", data["type"])
	ref := data["reference"].(map[string]any)
	require.Equal(t, "for_sale", ref["targetType"])
	require.Equal(t, fpsID.String(), ref["targetId"])
}

// TestCommentCommerceReference_RejectedForNonDisplayableForSale proves that
// referencing a non-displayable (withdrawn) ForSale is rejected, with the
// rejection coming from displayability validation, NOT ownership.
func TestCommentCommerceReference_RejectedForNonDisplayableForSale(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	appDB := db.NewFromPool(tdb.Pool())
	ctx := context.Background()

	contentAuthor := seedCommentListHTTPUser(t, ctx, appDB, "nd-author")
	seller := seedCommentListHTTPUser(t, ctx, appDB, "nd-seller")
	anyUser := seedCommentListHTTPUser(t, ctx, appDB, "nd-anyuser")

	handler := testCommentDisplayabilityHandler(appDB)
	contentID := seedCommentListHTTPContent(t, ctx, appDB, handler, contentAuthor)

	// Seed a WITHDRAWN ForSale — not displayable.
	withdrawnFPS := seedCommentCommerceFPS(t, ctx, appDB, seller, "withdrawn")

	router := gin.New()
	router.POST("/contents/:id/comments/reference", func(c *gin.Context) {
		c.Set("userID", anyUser)
		handler.CreateCommerceReferenceComment(c)
	})

	reqBody := map[string]any{
		"resource_reference": map[string]any{
			"resource_type": "for_sale",
			"resource_id":   withdrawnFPS.String(),
		},
	}
	raw, err := json.Marshal(reqBody)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/contents/"+contentID.String()+"/comments/reference", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "nd-key-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.NotEqual(t, http.StatusCreated, w.Code,
		"withdrawn ForSale MUST be rejected (not displayable)")
}

// TestCommentCommerceReference_RejectedForNonExistentForSale proves that
// referencing a non-existent ForSale is rejected.
func TestCommentCommerceReference_RejectedForNonExistentForSale(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	appDB := db.NewFromPool(tdb.Pool())
	ctx := context.Background()

	contentAuthor := seedCommentListHTTPUser(t, ctx, appDB, "ne-author")
	anyUser := seedCommentListHTTPUser(t, ctx, appDB, "ne-anyuser")

	handler := testCommentDisplayabilityHandler(appDB)
	contentID := seedCommentListHTTPContent(t, ctx, appDB, handler, contentAuthor)

	router := gin.New()
	router.POST("/contents/:id/comments/reference", func(c *gin.Context) {
		c.Set("userID", anyUser)
		handler.CreateCommerceReferenceComment(c)
	})

	reqBody := map[string]any{
		"resource_reference": map[string]any{
			"resource_type": "for_sale",
			"resource_id":   uuid.New().String(), // non-existent ID
		},
	}
	raw, err := json.Marshal(reqBody)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/contents/"+contentID.String()+"/comments/reference", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "ne-key-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.NotEqual(t, http.StatusCreated, w.Code,
		"non-existent ForSale MUST be rejected")
}
