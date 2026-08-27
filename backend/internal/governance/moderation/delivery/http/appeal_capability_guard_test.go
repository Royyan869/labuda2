package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// PASS_13B: appeal read/review capability guard — route-level proof.
//
// These tests exercise the real middleware.RequireCapability chain wired the
// same way routes_core.go wires it, proving:
//   - moderation.case.read alone no longer grants access to appeal GET routes
//   - moderation.appeal.read grants access to appeal GET routes
//   - moderation.appeal.read alone does NOT grant access to the review route
//   - moderation.appeal.review grants access to the review route (subject to
//     the handler's own defense-in-depth re-check)
//
// The handler is constructed with a nil *application.AppealService (NewAppealService
// itself panics on nil deps, so we bypass it and pass nil directly — the same
// pattern already used elsewhere in this package's tests). Requests that pass
// the capability gate but then reach real business logic may 500 against the
// nil service; gin.Recovery() converts any resulting panic into a clean 500
// instead of crashing the test process. The only thing under test here is
// whether the gate itself returns 403, not downstream business behavior.
// ============================================================================

type fakeAppealAuditLogger struct{}

func (fakeAppealAuditLogger) LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) {
}

func (fakeAppealAuditLogger) LogTx(context.Context, db.Tx, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) error {
	return nil
}

type noopAppealTransactor struct{}

func (noopAppealTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(nil)
}

func newAppealCapabilityTestActor(capabilities []string) *capabilityEntity.Actor {
	return &capabilityEntity.Actor{ID: uuid.New(), Role: "admin", Capabilities: capabilities}
}

// newAppealCapabilityTestRouter wires the real RequireCapability middleware in
// front of the real AppealHandler, mirroring routes_core.go's production wiring.
func newAppealCapabilityTestRouter(actor *capabilityEntity.Actor) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		ctx := capability.WithActor(c.Request.Context(), actor)
		c.Request = c.Request.WithContext(ctx)
		c.Set("user_id", actor.ID)
		c.Next()
	})

	handler := NewAppealHandler(nil, noopAppealTransactor{}, zap.NewNop(), fakeAppealAuditLogger{})

	admin := r.Group("/admin")
	admin.GET("/appeals", middleware.RequireCapability("moderation.appeal.read"), handler.AdminListAppeals)
	admin.GET("/appeals/pending", middleware.RequireCapability("moderation.appeal.read"), handler.AdminListPendingAppeals)
	admin.GET("/appeals/:id", middleware.RequireCapability("moderation.appeal.read"), handler.AdminGetAppeal)
	admin.PUT("/appeals/:id/review", middleware.RequireCapability("moderation.appeal.review"), handler.AdminReviewAppeal)
	return r
}

// ============================================================================
// GET /admin/appeals
// ============================================================================

func TestAdminListAppeals_NoCapability_Forbidden(t *testing.T) {
	router := newAppealCapabilityTestRouter(newAppealCapabilityTestActor(nil))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/appeals", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminListAppeals_CaseReadOnly_Forbidden(t *testing.T) {
	router := newAppealCapabilityTestRouter(newAppealCapabilityTestActor([]string{"moderation.case.read"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/appeals", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code, "moderation.case.read alone must no longer grant appeal list access")
}

func TestAdminListAppeals_AppealRead_Allowed(t *testing.T) {
	router := newAppealCapabilityTestRouter(newAppealCapabilityTestActor([]string{"moderation.appeal.read"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/appeals", nil)
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusForbidden, w.Code, "moderation.appeal.read must pass the capability gate")
}

// ============================================================================
// GET /admin/appeals/pending
// ============================================================================

func TestAdminListPendingAppeals_CaseReadOnly_Forbidden(t *testing.T) {
	router := newAppealCapabilityTestRouter(newAppealCapabilityTestActor([]string{"moderation.case.read"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/appeals/pending", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code, "moderation.case.read alone must no longer grant pending-appeal access")
}

func TestAdminListPendingAppeals_AppealRead_Allowed(t *testing.T) {
	router := newAppealCapabilityTestRouter(newAppealCapabilityTestActor([]string{"moderation.appeal.read"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/appeals/pending", nil)
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusForbidden, w.Code, "moderation.appeal.read must pass the capability gate")
}

// ============================================================================
// GET /admin/appeals/:id
// ============================================================================

func TestAdminGetAppeal_CaseReadOnly_Forbidden(t *testing.T) {
	router := newAppealCapabilityTestRouter(newAppealCapabilityTestActor([]string{"moderation.case.read"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/appeals/"+uuid.New().String(), nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code, "moderation.case.read alone must no longer grant appeal detail access")
}

func TestAdminGetAppeal_AppealRead_Allowed(t *testing.T) {
	router := newAppealCapabilityTestRouter(newAppealCapabilityTestActor([]string{"moderation.appeal.read"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/appeals/"+uuid.New().String(), nil)
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusForbidden, w.Code, "moderation.appeal.read must pass the capability gate")
}

// ============================================================================
// PUT /admin/appeals/:id/review — must remain gated on moderation.appeal.review,
// NOT satisfied by moderation.appeal.read alone.
// ============================================================================

func TestAdminReviewAppeal_AppealReadOnly_Forbidden(t *testing.T) {
	router := newAppealCapabilityTestRouter(newAppealCapabilityTestActor([]string{"moderation.appeal.read"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/admin/appeals/"+uuid.New().String()+"/review", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code, "moderation.appeal.read must not grant review-mutation access")
}

func TestAdminReviewAppeal_AppealReview_Allowed(t *testing.T) {
	router := newAppealCapabilityTestRouter(newAppealCapabilityTestActor([]string{"moderation.appeal.review"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/admin/appeals/"+uuid.New().String()+"/review", nil)
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusForbidden, w.Code, "moderation.appeal.review must pass the capability gate")
}
