package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/middleware"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type externalProductAdminRouteActorResolver struct {
	actor *capabilityEntity.Actor
}

func (m *externalProductAdminRouteActorResolver) ResolveActor(ctx interface{}, userID uuid.UUID) (*capabilityEntity.Actor, error) {
	if m.actor == nil {
		return nil, nil
	}
	return m.actor, nil
}

type externalProductAdminRouteRoleChecker struct {
	isAdmin bool
}

func (m *externalProductAdminRouteRoleChecker) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	return m.isAdmin, nil
}

func (m *externalProductAdminRouteRoleChecker) IsSeller(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *externalProductAdminRouteRoleChecker) HasActiveSellerCapability(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *externalProductAdminRouteRoleChecker) HasSellerProfile(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

var _ capabilityEntity.ActorResolver = (*externalProductAdminRouteActorResolver)(nil)
var _ auth.RoleChecker = (*externalProductAdminRouteRoleChecker)(nil)

func TestExternalProductAdminRoute_ForbiddenWithoutReviewCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := buildReviewQueueStore(t)
	handler := newTestExternalProductHandler(store)
	router := gin.New()

	userID := uuid.New()
	actorResolver := &externalProductAdminRouteActorResolver{
		actor: &capabilityEntity.Actor{
			ID:           userID,
			Role:         "admin",
			Capabilities: []string{"governance.dashboard.view"},
		},
	}

	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	router.Use(middleware.ActorContextInject(actorResolver, middleware.ActorContextInjectOptions{}))
	router.Use(middleware.RequireAdminMiddleware(&externalProductAdminRouteRoleChecker{isAdmin: true}))
	router.GET("/api/v1/admin/external-products",
		middleware.RequireCapability("promotion.external_product.review"),
		handler.ListAdminExternalProducts,
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/external-products", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestExternalProductAdminRoute_AllowsWithReviewCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := buildReviewQueueStore(t)
	handler := newTestExternalProductHandler(store)
	router := gin.New()

	userID := uuid.New()
	actorResolver := &externalProductAdminRouteActorResolver{
		actor: &capabilityEntity.Actor{
			ID:           userID,
			Role:         "admin",
			Capabilities: []string{"governance.dashboard.view", "promotion.external_product.review"},
		},
	}

	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	router.Use(middleware.ActorContextInject(actorResolver, middleware.ActorContextInjectOptions{}))
	router.Use(middleware.RequireAdminMiddleware(&externalProductAdminRouteRoleChecker{isAdmin: true}))
	router.GET("/api/v1/admin/external-products",
		middleware.RequireCapability("promotion.external_product.review"),
		handler.ListAdminExternalProducts,
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/external-products", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeAdminExternalProductListResponse(t, w)
	require.Equal(t, 1, resp.Count)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "pending_review", resp.Items[0].ReviewStatus)
}

func TestExternalProductAdminRoute_AllowsApproveWithReviewCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := buildReviewQueueStore(t)
	handler := newTestExternalProductHandler(store)
	router := gin.New()

	userID := uuid.New()
	pendingID := findPendingProductID(t, store)
	actorResolver := &externalProductAdminRouteActorResolver{
		actor: &capabilityEntity.Actor{
			ID:           userID,
			Role:         "admin",
			Capabilities: []string{"promotion.external_product.review"},
		},
	}

	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	router.Use(middleware.ActorContextInject(actorResolver, middleware.ActorContextInjectOptions{}))
	router.Use(middleware.RequireAdminMiddleware(&externalProductAdminRouteRoleChecker{isAdmin: true}))
	router.POST("/api/v1/admin/external-products/:id/approve",
		middleware.RequireCapability("promotion.external_product.review"),
		handler.ApproveExternalProduct,
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/external-products/"+pendingID.String()+"/approve", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, store.products[pendingID])
	assert.Equal(t, entity.ExternalProductReviewStatusApproved, store.products[pendingID].ReviewStatus)
	assert.Len(t, store.history, 1)
}

func buildReviewQueueStore(t *testing.T) *externalProductHandlerStore {
	t.Helper()

	pendingID := uuid.New()
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	pending := &entity.ExternalProduct{
		ID:                    pendingID,
		OwnerUserID:           uuid.New(),
		Title:                 "Pending Promo",
		ExternalURL:           "https://example.com/pending",
		NormalizedExternalURL: "https://example.com/pending",
		ReviewStatus:          entity.ExternalProductReviewStatusPendingReview,
		UnsafeURLFlag:         false,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	mediaID := uuid.New()
	media := &entity.ExternalProductMedia{
		ID:                mediaID,
		ExternalProductID: pendingID,
		MediaType:         entity.ExternalProductMediaTypeImage,
		StorageKey:        "external-products/pending.jpg",
		URL:               "https://cdn.example.com/pending.jpg",
		SortOrder:         0,
		CreatedAt:         now,
	}

	return &externalProductHandlerStore{
		now: now,
		products: map[uuid.UUID]*entity.ExternalProduct{
			pendingID: pending,
		},
		media: map[uuid.UUID]*entity.ExternalProductMedia{
			mediaID: media,
		},
	}
}

func findPendingProductID(t *testing.T, store *externalProductHandlerStore) uuid.UUID {
	t.Helper()

	for id, product := range store.products {
		if product != nil && product.ReviewStatus == entity.ExternalProductReviewStatusPendingReview {
			return id
		}
	}

	t.Fatal("pending product not found in store")
	return uuid.Nil
}
