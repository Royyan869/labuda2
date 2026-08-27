package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/middleware"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/internal/platform/response"
	promotionApp "github.com/labuda/backend/internal/pricing/promotion/application"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	promotionRepo "github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type analyticsCampaignRepoStub struct {
	summaries map[uuid.UUID]*promotionRepo.PromotionEventAnalyticsSummary
}

var _ promotionRepo.PromotionEventRepository = (*analyticsCampaignRepoStub)(nil)

func (s *analyticsCampaignRepoStub) RecordEvent(context.Context, db.Tx, *promoentity.PromotionEvent) error {
	return nil
}

func (s *analyticsCampaignRepoStub) GetCampaignAnalytics(
	_ context.Context,
	_ db.Tx,
	instanceID uuid.UUID,
	from *time.Time,
	to *time.Time,
) (*promotionRepo.PromotionEventAnalyticsSummary, error) {
	if s == nil || s.summaries == nil {
		return &promotionRepo.PromotionEventAnalyticsSummary{
			InstanceID: instanceID,
			WindowFrom: from,
			WindowTo:   to,
		}, nil
	}
	if summary, ok := s.summaries[instanceID]; ok && summary != nil {
		copy := *summary
		copy.InstanceID = instanceID
		copy.WindowFrom = from
		copy.WindowTo = to
		return &copy, nil
	}
	return &promotionRepo.PromotionEventAnalyticsSummary{
		InstanceID: instanceID,
		WindowFrom: from,
		WindowTo:   to,
	}, nil
}

type analyticsCampaignDB struct {
	existing map[uuid.UUID]bool
}

var _ db.Transactor = (*analyticsCampaignDB)(nil)

func (d *analyticsCampaignDB) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(&analyticsCampaignTx{existing: d.existing})
}

type analyticsCampaignTx struct {
	existing map[uuid.UUID]bool
}

var _ db.Tx = (*analyticsCampaignTx)(nil)

func (t *analyticsCampaignTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 0"), nil
}

func (t *analyticsCampaignTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if len(args) == 1 {
		if id, ok := args[0].(uuid.UUID); ok && t.existing[id] {
			return &analyticsCampaignRow{values: []any{1}}
		}
		return &analyticsCampaignRow{err: pgx.ErrNoRows}
	}
	return &analyticsCampaignRow{err: errors.New("unexpected query")}
}

func (t *analyticsCampaignTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &analyticsCampaignRows{}, nil
}

func (t *analyticsCampaignTx) Commit(context.Context) error   { return nil }
func (t *analyticsCampaignTx) Rollback(context.Context) error { return nil }

type analyticsCampaignRow struct {
	values []any
	err    error
}

func (r *analyticsCampaignRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) != len(dest) {
		return errors.New("scan argument count mismatch")
	}
	for i, value := range r.values {
		switch d := dest[i].(type) {
		case *int:
			*d = value.(int)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

type analyticsCampaignRows struct{}

func (r *analyticsCampaignRows) Next() bool                                   { return false }
func (r *analyticsCampaignRows) Err() error                                   { return nil }
func (r *analyticsCampaignRows) Close()                                       {}
func (r *analyticsCampaignRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *analyticsCampaignRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *analyticsCampaignRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *analyticsCampaignRows) RawValues() [][]byte                          { return nil }
func (r *analyticsCampaignRows) Values() ([]any, error)                       { return nil, nil }
func (r *analyticsCampaignRows) Scan(dest ...any) error                       { return errors.New("no rows") }
func (r *analyticsCampaignRows) Conn() *pgx.Conn                              { return nil }

type analyticsAdminRouteActorResolver struct {
	actor *capabilityEntity.Actor
}

func (m *analyticsAdminRouteActorResolver) ResolveActor(ctx interface{}, userID uuid.UUID) (*capabilityEntity.Actor, error) {
	if m.actor == nil {
		return nil, nil
	}
	return m.actor, nil
}

type analyticsAdminRouteRoleChecker struct {
	isAdmin bool
}

func (m *analyticsAdminRouteRoleChecker) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	return m.isAdmin, nil
}

func (m *analyticsAdminRouteRoleChecker) IsSeller(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *analyticsAdminRouteRoleChecker) HasActiveSellerCapability(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *analyticsAdminRouteRoleChecker) HasSellerProfile(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

var _ capabilityEntity.ActorResolver = (*analyticsAdminRouteActorResolver)(nil)
var _ auth.RoleChecker = (*analyticsAdminRouteRoleChecker)(nil)

type analyticsEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   any             `json:"error"`
}

type analyticsBody struct {
	Analytics CampaignAnalyticsResponse `json:"analytics"`
}

func newTestPromotionAnalyticsHandler(dbtx db.Transactor, repo promotionRepo.PromotionEventRepository) *PromotionHandler {
	return &PromotionHandler{
		promotionService: promotionApp.NewPromotionService(mockOperabilityChecker{}),
		db:               dbtx,
		log:              zap.NewNop(),
		eventRepo:        repo,
	}
}

func decodeAnalyticsBody(t *testing.T, w *httptest.ResponseRecorder) analyticsBody {
	t.Helper()

	var envelope analyticsEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))

	var body analyticsBody
	require.NoError(t, json.Unmarshal(envelope.Data, &body))
	return body
}

func TestAdminGetCampaignAnalytics_InvalidCampaignID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestPromotionAnalyticsHandler(&analyticsCampaignDB{}, &analyticsCampaignRepoStub{})

	router := gin.New()
	router.GET("/api/v1/admin/promotions/campaigns/:id/analytics", handler.AdminGetCampaignAnalytics)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/promotions/campaigns/not-a-uuid/analytics", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
}

func TestAdminGetCampaignAnalytics_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestPromotionAnalyticsHandler(&analyticsCampaignDB{existing: map[uuid.UUID]bool{}}, &analyticsCampaignRepoStub{})

	router := gin.New()
	router.GET("/api/v1/admin/promotions/campaigns/:id/analytics", handler.AdminGetCampaignAnalytics)

	campaignID := uuid.New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/promotions/campaigns/"+campaignID.String()+"/analytics", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), response.ErrCodeNotFound)
}

func TestAdminGetCampaignAnalytics_ReturnsZeroSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	campaignID := uuid.New()
	handler := newTestPromotionAnalyticsHandler(
		&analyticsCampaignDB{existing: map[uuid.UUID]bool{campaignID: true}},
		&analyticsCampaignRepoStub{},
	)

	router := gin.New()
	router.GET("/api/v1/admin/promotions/campaigns/:id/analytics", handler.AdminGetCampaignAnalytics)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/promotions/campaigns/"+campaignID.String()+"/analytics", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeAnalyticsBody(t, w)
	assert.Equal(t, campaignID, body.Analytics.InstanceID)
	assert.Equal(t, 0, body.Analytics.ImpressionsTotal)
	assert.Equal(t, 0, body.Analytics.ClicksTotal)
	assert.Equal(t, 0.0, body.Analytics.CTR)
	assert.Equal(t, 0, body.Analytics.FeedImpressions)
	assert.Equal(t, 0, body.Analytics.FeedClicks)
	assert.Equal(t, 0, body.Analytics.SearchImpressions)
	assert.Equal(t, 0, body.Analytics.SearchClicks)
	assert.Equal(t, 0, body.Analytics.ExploreImpressions)
	assert.Equal(t, 0, body.Analytics.ExploreClicks)
}

func TestAdminCampaignAnalyticsRoute_Guard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	campaignID := uuid.New()
	handler := newTestPromotionAnalyticsHandler(
		&analyticsCampaignDB{existing: map[uuid.UUID]bool{campaignID: true}},
		&analyticsCampaignRepoStub{
			summaries: map[uuid.UUID]*promotionRepo.PromotionEventAnalyticsSummary{
				campaignID: {
					ImpressionsTotal:   10,
					ClicksTotal:        4,
					CTR:                0.4,
					FeedImpressions:    6,
					FeedClicks:         2,
					SearchImpressions:  3,
					SearchClicks:       1,
					ExploreImpressions: 1,
					ExploreClicks:      1,
				},
			},
		},
	)

	router := gin.New()
	userID := uuid.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	router.Use(middleware.ActorContextInject(&analyticsAdminRouteActorResolver{
		actor: &capabilityEntity.Actor{
			ID:           userID,
			Role:         "admin",
			Capabilities: []string{"governance.dashboard.view"},
		},
	}, middleware.ActorContextInjectOptions{}))
	router.Use(middleware.RequireAdminMiddleware(&analyticsAdminRouteRoleChecker{isAdmin: true}))
	router.GET(
		"/api/v1/admin/promotions/campaigns/:id/analytics",
		middleware.RequireCapability("promotion.campaign.view"),
		handler.AdminGetCampaignAnalytics,
	)

	forbidden := httptest.NewRecorder()
	forbiddenReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/promotions/campaigns/"+campaignID.String()+"/analytics", nil)
	router.ServeHTTP(forbidden, forbiddenReq)
	assert.Equal(t, http.StatusForbidden, forbidden.Code)

	allowedRouter := gin.New()
	allowedRouter.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	allowedRouter.Use(middleware.ActorContextInject(&analyticsAdminRouteActorResolver{
		actor: &capabilityEntity.Actor{
			ID:           userID,
			Role:         "admin",
			Capabilities: []string{"governance.dashboard.view", "promotion.campaign.view"},
		},
	}, middleware.ActorContextInjectOptions{}))
	allowedRouter.Use(middleware.RequireAdminMiddleware(&analyticsAdminRouteRoleChecker{isAdmin: true}))
	allowedRouter.GET(
		"/api/v1/admin/promotions/campaigns/:id/analytics",
		middleware.RequireCapability("promotion.campaign.view"),
		handler.AdminGetCampaignAnalytics,
	)

	allowed := httptest.NewRecorder()
	allowedReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/promotions/campaigns/"+campaignID.String()+"/analytics", nil)
	allowedRouter.ServeHTTP(allowed, allowedReq)

	require.Equal(t, http.StatusOK, allowed.Code)
	body := decodeAnalyticsBody(t, allowed)
	assert.Equal(t, 10, body.Analytics.ImpressionsTotal)
	assert.Equal(t, 4, body.Analytics.ClicksTotal)
	assert.InDelta(t, 0.4, body.Analytics.CTR, 0.0001)
	assert.Equal(t, 6, body.Analytics.FeedImpressions)
	assert.Equal(t, 2, body.Analytics.FeedClicks)
	assert.Equal(t, 3, body.Analytics.SearchImpressions)
	assert.Equal(t, 1, body.Analytics.SearchClicks)
	assert.Equal(t, 1, body.Analytics.ExploreImpressions)
	assert.Equal(t, 1, body.Analytics.ExploreClicks)
}
