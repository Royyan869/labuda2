//go:build integration

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appealApp "github.com/labuda/backend/internal/governance/moderation/application"
	appealEntity "github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	outboxInfraRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	contentEntity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockDB is defined in moderation_handler_test.go (same package)

// ============================================================================
// PASS_14D: real fakes for AppealService's dependencies.
//
// GetAppeal (the only method exercised by the IDOR tests below) is a
// passthrough to appealRepo.GetByID alone. The other 4 constructor
// dependencies are never invoked, so they're stubbed as
// panic-if-called doubles: an unexpected call fails the test loudly
// instead of silently returning a zero value.
// ============================================================================

// fakeAppealRepository is an explicit test double for
// moderationrepo.AppealRepository. Only GetByID is given real, per-test
// behavior.
type fakeAppealRepository struct {
	getByIDFunc func(ctx context.Context, tx interface{}, appealID uuid.UUID) (*appealEntity.Appeal, error)
}

func (f *fakeAppealRepository) Create(ctx context.Context, tx interface{}, appeal *appealEntity.Appeal) error {
	panic("fakeAppealRepository.Create: not used by GetAppeal, should not be called")
}
func (f *fakeAppealRepository) CreateWithPendingCheck(ctx context.Context, tx interface{}, appeal *appealEntity.Appeal) error {
	panic("fakeAppealRepository.CreateWithPendingCheck: not used by GetAppeal, should not be called")
}
func (f *fakeAppealRepository) GetByID(ctx context.Context, tx interface{}, appealID uuid.UUID) (*appealEntity.Appeal, error) {
	if f.getByIDFunc == nil {
		panic("fakeAppealRepository.GetByID called without getByIDFunc configured")
	}
	return f.getByIDFunc(ctx, tx, appealID)
}
func (f *fakeAppealRepository) GetForUpdate(ctx context.Context, tx interface{}, appealID uuid.UUID) (*appealEntity.Appeal, error) {
	panic("fakeAppealRepository.GetForUpdate: not used by GetAppeal, should not be called")
}
func (f *fakeAppealRepository) Update(ctx context.Context, tx interface{}, appeal *appealEntity.Appeal) error {
	panic("fakeAppealRepository.Update: not used by GetAppeal, should not be called")
}
func (f *fakeAppealRepository) ListByUser(ctx context.Context, tx interface{}, userID uuid.UUID, limit, offset int) ([]*appealEntity.Appeal, error) {
	panic("fakeAppealRepository.ListByUser: not used by GetAppeal, should not be called")
}
func (f *fakeAppealRepository) ListByCase(ctx context.Context, tx interface{}, caseID uuid.UUID) ([]*appealEntity.Appeal, error) {
	panic("fakeAppealRepository.ListByCase: not used by GetAppeal, should not be called")
}
func (f *fakeAppealRepository) ListAll(ctx context.Context, tx interface{}, statusFilter *appealEntity.AppealStatus, limit, offset int) ([]*appealEntity.Appeal, error) {
	panic("fakeAppealRepository.ListAll: not used by GetAppeal, should not be called")
}
func (f *fakeAppealRepository) ListPending(ctx context.Context, tx interface{}, limit, offset int) ([]*appealEntity.Appeal, error) {
	panic("fakeAppealRepository.ListPending: not used by GetAppeal, should not be called")
}

// unusedModerationRepository satisfies moderationrepo.ModerationRepository
// structurally. GetAppeal never touches it.
type unusedModerationRepository struct{}

func (unusedModerationRepository) Create(ctx context.Context, tx interface{}, caseEntity *appealEntity.GovernanceCase) error {
	panic("unusedModerationRepository.Create: should not be called by GetAppeal")
}
func (unusedModerationRepository) GetByID(ctx context.Context, tx interface{}, caseID uuid.UUID) (*appealEntity.GovernanceCase, error) {
	panic("unusedModerationRepository.GetByID: should not be called by GetAppeal")
}
func (unusedModerationRepository) GetForUpdate(ctx context.Context, tx interface{}, caseID uuid.UUID) (*appealEntity.GovernanceCase, error) {
	panic("unusedModerationRepository.GetForUpdate: should not be called by GetAppeal")
}
func (unusedModerationRepository) Update(ctx context.Context, tx interface{}, caseEntity *appealEntity.GovernanceCase) error {
	panic("unusedModerationRepository.Update: should not be called by GetAppeal")
}
func (unusedModerationRepository) ListPending(ctx context.Context, tx interface{}, limit, offset int) ([]*appealEntity.GovernanceCase, error) {
	panic("unusedModerationRepository.ListPending: should not be called by GetAppeal")
}
func (unusedModerationRepository) ListByResource(ctx context.Context, tx interface{}, resourceType appealEntity.ResourceType, resourceID uuid.UUID) ([]*appealEntity.GovernanceCase, error) {
	panic("unusedModerationRepository.ListByResource: should not be called by GetAppeal")
}
func (unusedModerationRepository) ListByReporter(ctx context.Context, tx interface{}, reporterID uuid.UUID, limit, offset int) ([]*appealEntity.GovernanceCase, error) {
	panic("unusedModerationRepository.ListByReporter: should not be called by GetAppeal")
}
func (unusedModerationRepository) ListWithStatus(ctx context.Context, tx interface{}, statusFilter *appealEntity.GovernanceCaseStatus, resourceTypeFilter *appealEntity.ResourceType, limit, offset int) ([]*appealEntity.GovernanceCase, int64, error) {
	panic("unusedModerationRepository.ListWithStatus: should not be called by GetAppeal")
}
func (unusedModerationRepository) ResourceExists(ctx context.Context, tx interface{}, resourceType appealEntity.ResourceType, resourceID uuid.UUID) (bool, error) {
	panic("unusedModerationRepository.ResourceExists: should not be called by GetAppeal")
}
func (unusedModerationRepository) HasUserReportedResource(ctx context.Context, tx interface{}, reporterID uuid.UUID, resourceType appealEntity.ResourceType, resourceID uuid.UUID) (bool, error) {
	panic("unusedModerationRepository.HasUserReportedResource: should not be called by GetAppeal")
}
func (unusedModerationRepository) ValidateChatMessageReporter(ctx context.Context, tx interface{}, messageID uuid.UUID, reporterID uuid.UUID) (bool, string, error) {
	panic("unusedModerationRepository.ValidateChatMessageReporter: should not be called by GetAppeal")
}

// unusedContentRepository satisfies contentRepo.ContentRepository
// structurally (unrelated social/content domain). GetAppeal never touches it.
type unusedContentRepository struct{}

func (unusedContentRepository) Create(ctx context.Context, tx interface{}, content *contentEntity.Content) error {
	panic("unusedContentRepository.Create: should not be called by GetAppeal")
}
func (unusedContentRepository) CreateMedia(ctx context.Context, tx interface{}, media []*contentEntity.ContentMedia) error {
	panic("unusedContentRepository.CreateMedia: should not be called by GetAppeal")
}
func (unusedContentRepository) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
	panic("unusedContentRepository.GetByID: should not be called by GetAppeal")
}
func (unusedContentRepository) GetForUpdate(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
	panic("unusedContentRepository.GetForUpdate: should not be called by GetAppeal")
}
func (unusedContentRepository) Update(ctx context.Context, tx interface{}, content *contentEntity.Content) error {
	panic("unusedContentRepository.Update: should not be called by GetAppeal")
}
func (unusedContentRepository) ListByAuthor(ctx context.Context, tx interface{}, authorID uuid.UUID, limit int, cursor string) ([]*contentEntity.Content, string, error) {
	panic("unusedContentRepository.ListByAuthor: should not be called by GetAppeal")
}
func (unusedContentRepository) GetMedia(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]*contentEntity.ContentMedia, error) {
	panic("unusedContentRepository.GetMedia: should not be called by GetAppeal")
}
func (unusedContentRepository) GetTagsByContentID(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]string, error) {
	panic("unusedContentRepository.GetTagsByContentID: should not be called by GetAppeal")
}
func (unusedContentRepository) InsertTags(ctx context.Context, tx interface{}, contentID uuid.UUID, tags []string) error {
	panic("unusedContentRepository.InsertTags: should not be called by GetAppeal")
}

// unusedCommentRepository satisfies contentrepository.CommentRepository
// structurally (unrelated social/content domain). GetAppeal never touches it.
type unusedCommentRepository struct{}

func (unusedCommentRepository) Create(ctx context.Context, tx db.Tx, comment *contentEntity.Comment) error {
	panic("unusedCommentRepository.Create: should not be called by GetAppeal")
}
func (unusedCommentRepository) ListByTarget(ctx context.Context, tx db.Tx, targetType contentEntity.CommentTargetType, targetID uuid.UUID, limit int, cursor string) ([]*contentEntity.Comment, string, error) {
	panic("unusedCommentRepository.ListByTarget: should not be called by GetAppeal")
}
func (unusedCommentRepository) GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*contentEntity.Comment, error) {
	panic("unusedCommentRepository.GetForUpdate: should not be called by GetAppeal")
}
func (unusedCommentRepository) Update(ctx context.Context, tx db.Tx, comment *contentEntity.Comment) error {
	panic("unusedCommentRepository.Update: should not be called by GetAppeal")
}
func (unusedCommentRepository) CreateForSaleReferenceComment(ctx context.Context, tx db.Tx, targetID, sellerID, forSaleID uuid.UUID, body *string) error {
	panic("unusedCommentRepository.CreateForSaleReferenceComment: should not be called by GetAppeal")
}
func (unusedCommentRepository) FindTargetIDByForSaleReference(ctx context.Context, tx db.Tx, forSaleID uuid.UUID) (uuid.UUID, error) {
	panic("unusedCommentRepository.FindTargetIDByForSaleReference: should not be called by GetAppeal")
}
func (unusedCommentRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*contentEntity.Comment, error) {
	panic("unusedCommentRepository.GetByID: should not be called by GetAppeal")
}
func (unusedCommentRepository) ListReplies(ctx context.Context, tx db.Tx, parentID uuid.UUID, limit int, cursor string) ([]*contentEntity.Comment, string, error) {
	panic("unusedCommentRepository.ListReplies: should not be called by GetAppeal")
}
func (unusedCommentRepository) GetReplyCount(ctx context.Context, tx db.Tx, commentID uuid.UUID) (int, error) {
	panic("unusedCommentRepository.GetReplyCount: should not be called by GetAppeal")
}
func (unusedCommentRepository) CountTopLevelCommentsByContent(ctx context.Context, tx db.Tx, contentID uuid.UUID) (int, error) {
	panic("unusedCommentRepository.CountTopLevelCommentsByContent: should not be called by GetAppeal")
}
func (unusedCommentRepository) ExistsByTarget(ctx context.Context, tx db.Tx, targetType contentEntity.CommentTargetType, targetID uuid.UUID) (bool, error) {
	panic("unusedCommentRepository.ExistsByTarget: should not be called by GetAppeal")
}
func (unusedCommentRepository) SoftDelete(ctx context.Context, tx db.Tx, id uuid.UUID, deletedAt time.Time) error {
	panic("unusedCommentRepository.SoftDelete: should not be called by GetAppeal")
}
func (unusedCommentRepository) Restore(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	panic("unusedCommentRepository.Restore: should not be called by GetAppeal")
}

// fakeAppealDB implements db.Transactor without a live Postgres connection.
// AppealRepository.GetByID's tx parameter is untyped (interface{}), so a nil
// tx flowing through is safe here (unlike WarningService.IssueWarning, which
// needs a real db.Tx — see fakeWarningDB in warning_handler_test.go).
type fakeAppealDB struct{}

func (fakeAppealDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(nil)
}

func setupAppealRouter(handler *AppealHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// PASS_14B: mirror main.go's production wiring (Recovery first) so a nil
	// dependency panic surfaces as the 500 these tests already assert, instead
	// of crashing the whole test binary uncaught.
	router.Use(gin.Recovery())

	// Add middleware for setting user context
	router.Use(func(c *gin.Context) {
		// Set a test user ID in context
		userID := uuid.New()
		c.Set("user_id", userID)
		c.Set("user_role", "user")
		c.Next()
	})

	return router
}

func TestCreateAppeal(t *testing.T) {
	log := zap.NewNop()

	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	caseID := uuid.New()

	// Create request body
	reqBody := CreateAppealRequest{
		CaseID:  caseID.String(),
		Message: "This is a mistake",
	}
	body, _ := json.Marshal(reqBody)

	router := setupAppealRouter(handler)
	router.POST("/appeals", handler.CreateAppeal)

	req, _ := http.NewRequest("POST", "/appeals", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Service is nil, so expect 500 Internal Server Error
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetAppeal(t *testing.T) {
	log := zap.NewNop()

	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	appealID := uuid.New()

	router := setupAppealRouter(handler)
	router.GET("/appeals/:id", handler.GetAppeal)

	req, _ := http.NewRequest("GET", "/appeals/"+appealID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Service is nil, so expect 500 Internal Server Error
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListMyAppeals(t *testing.T) {
	log := zap.NewNop()

	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	router := setupAppealRouter(handler)
	router.GET("/appeals/me", handler.ListMyAppeals)

	req, _ := http.NewRequest("GET", "/appeals/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Service is nil, so expect 500 Internal Server Error
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAdminListAppeals(t *testing.T) {
	log := zap.NewNop()

	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	// PASS_14B: see setupAppealRouter — mirrors production Recovery wiring so
	// the nil-dependency panic surfaces as the 500 already asserted below.
	router.Use(gin.Recovery())

	// Add admin middleware
	router.Use(func(c *gin.Context) {
		c.Set("user_id", adminID)
		c.Set("user_role", "admin")
		c.Next()
	})

	router.GET("/admin/appeals", handler.AdminListAppeals)

	req, _ := http.NewRequest("GET", "/admin/appeals", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Service is nil, so expect 500 Internal Server Error
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAdminReviewAppeal(t *testing.T) {
	log := zap.NewNop()

	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()
	appealID := uuid.New()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add admin middleware
	router.Use(func(c *gin.Context) {
		c.Set("user_id", adminID)
		c.Set("user_role", "admin")
		c.Next()
	})

	router.PUT("/admin/appeals/:id/review", handler.AdminReviewAppeal)

	// Create request body
	reqBody := ReviewAppealRequest{
		Decision:      "approve",
		AdminResponse: "Appeal granted",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/admin/appeals/"+appealID.String()+"/review", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// No actor in context - expect 403 Forbidden (handler-level defense)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Test appealToResponse tests the response formatter
func TestAppealToResponse(t *testing.T) {
	log := zap.NewNop()
	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	caseID := uuid.New()
	userID := uuid.New()
	appeal := appealEntity.NewAppeal(caseID, userID, "Test appeal message")

	resp := handler.appealToResponse(appeal)

	assert.Equal(t, appeal.ID, resp["id"])
	assert.Equal(t, caseID, resp["case_id"])
	assert.Equal(t, "pending", resp["status"])
	assert.Equal(t, "Test appeal message", resp["message"])
	assert.NotNil(t, resp["created_at"])
}

// ============================================================
// SLICE 7: CAPABILITY-BASED AUTH TESTS FOR AdminReviewAppeal
// ============================================================

// setupAppealCapabilityTestRouter creates a test router with actor injected
func setupAppealCapabilityTestRouter(handler *AppealHandler, actor *capabilityEntity.Actor) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		if actor != nil {
			ctx := capability.WithActor(c.Request.Context(), actor)
			c.Request = c.Request.WithContext(ctx)
			c.Set("user_id", actor.ID)
			c.Set("user_role", actor.Role)
		}
		c.Next()
	})

	return router
}

// TestAdminReviewAppeal_Success_HasCapability tests successful review with capability
func TestAdminReviewAppeal_Success_HasCapability(t *testing.T) {
	log := zap.NewNop()
	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()
	appealID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{capability.CapModerationAppealReview.String()},
	}

	router := setupAppealCapabilityTestRouter(handler, actor)
	router.PUT("/admin/appeals/:id/review", handler.AdminReviewAppeal)

	reqBody := ReviewAppealRequest{
		Decision:      "approve",
		AdminResponse: "Appeal granted",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/admin/appeals/"+appealID.String()+"/review", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Catch the panic that will occur due to nil DB
	// The panic happening AFTER capability check is what we want to verify
	didPanic := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()
		router.ServeHTTP(w, req)
	}()

	// The key assertion: we should NOT get 403 Forbidden in the response
	// (which would mean capability check failed)
	// Either we panic (nil DB) or get 500, but NOT 403
	assert.True(t, didPanic || w.Code == http.StatusInternalServerError, "Expected either panic or 500, got status: %d", w.Code)
	assert.NotContains(t, w.Body.String(), "Insufficient permissions")
}

// TestAdminReviewAppeal_Forbidden_AdminWithoutCapability tests admin without capability is forbidden
func TestAdminReviewAppeal_Forbidden_AdminWithoutCapability(t *testing.T) {
	log := zap.NewNop()
	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()
	appealID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{}, // Empty - no capabilities
	}

	router := setupAppealCapabilityTestRouter(handler, actor)
	router.PUT("/admin/appeals/:id/review", handler.AdminReviewAppeal)

	reqBody := ReviewAppealRequest{
		Decision:      "approve",
		AdminResponse: "Should not work",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/admin/appeals/"+appealID.String()+"/review", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Insufficient permissions")
	assert.Contains(t, w.Body.String(), "moderation.appeal.review")
}

// TestAdminReviewAppeal_Unauthenticated_NoActor tests unauthenticated request is forbidden
func TestAdminReviewAppeal_Unauthenticated_NoActor(t *testing.T) {
	log := zap.NewNop()
	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	appealID := uuid.New()

	router := setupAppealCapabilityTestRouter(handler, nil) // No actor
	router.PUT("/admin/appeals/:id/review", handler.AdminReviewAppeal)

	reqBody := ReviewAppealRequest{
		Decision:      "approve",
		AdminResponse: "Should not work",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/admin/appeals/"+appealID.String()+"/review", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Insufficient permissions")
}

// TestAdminReviewAppeal_NoAdminFallback tests admin role does NOT grant implicit access
func TestAdminReviewAppeal_NoAdminFallback(t *testing.T) {
	// CRITICAL SECURITY TEST: Verify admin role alone does NOT grant access
	log := zap.NewNop()
	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()
	appealID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{"moderation.case.resolve", "governance.audit.read"}, // Has other caps but NOT appeal.review
	}

	router := setupAppealCapabilityTestRouter(handler, actor)
	router.PUT("/admin/appeals/:id/review", handler.AdminReviewAppeal)

	reqBody := ReviewAppealRequest{
		Decision:      "approve",
		AdminResponse: "Should not work",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/admin/appeals/"+appealID.String()+"/review", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Admin without capability must be forbidden
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Insufficient permissions")
	assert.Contains(t, w.Body.String(), "moderation.appeal.review")
}

// TestAdminReviewAppeal_Forbidden_WrongCapability tests wrong capability is forbidden
func TestAdminReviewAppeal_Forbidden_WrongCapability(t *testing.T) {
	log := zap.NewNop()
	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()
	appealID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{capability.CapModerationCaseResolve.String()}, // Has case.resolve but NOT appeal.review
	}

	router := setupAppealCapabilityTestRouter(handler, actor)
	router.PUT("/admin/appeals/:id/review", handler.AdminReviewAppeal)

	reqBody := ReviewAppealRequest{
		Decision:      "approve",
		AdminResponse: "Should not work",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/admin/appeals/"+appealID.String()+"/review", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Insufficient permissions")
	assert.Contains(t, w.Body.String(), "moderation.appeal.review")
}

// TestAdminReviewAppeal_DefenseInDepth tests handler-level defense works even if middleware bypassed
func TestAdminReviewAppeal_DefenseInDepth(t *testing.T) {
	log := zap.NewNop()
	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()
	appealID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{}, // No capabilities
	}

	router := setupAppealCapabilityTestRouter(handler, actor)
	router.PUT("/admin/appeals/:id/review", handler.AdminReviewAppeal)

	reqBody := ReviewAppealRequest{
		Decision:      "approve",
		AdminResponse: "Should not work",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/admin/appeals/"+appealID.String()+"/review", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Handler-level defense must block this
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Insufficient permissions")
	assert.Contains(t, w.Body.String(), "moderation.appeal.review")
}

// TestAdminReviewAppeal_InvalidDecision tests decision validation
func TestAdminReviewAppeal_InvalidDecision(t *testing.T) {
	log := zap.NewNop()
	handler := NewAppealHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()
	appealID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{capability.CapModerationAppealReview.String()},
	}

	router := setupAppealCapabilityTestRouter(handler, actor)
	router.PUT("/admin/appeals/:id/review", handler.AdminReviewAppeal)

	reqBody := ReviewAppealRequest{
		Decision:      "invalid",
		AdminResponse: "Should fail validation",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/admin/appeals/"+appealID.String()+"/review", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid request")
}

// ============================================================
// P1 IDOR FIX: GET /appeals/:id ownership tests
// ============================================================

// setupGetAppealRouterWithUser creates a router that injects a specific userID.
func setupGetAppealRouterWithUser(handler *AppealHandler, userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", "user")
		c.Next()
	})
	return router
}

// newRealAppealServiceForGetAppeal builds a real *application.AppealService
// wired to a controllable fake AppealRepository. The other 3 dependencies
// (ModerationRepository, ContentRepository, CommentRepository) and the
// outbox repository are never invoked by GetAppeal, so they are
// panic-if-called stubs / a zero-value struct — see the fakes above.
func newRealAppealServiceForGetAppeal(getByID func(ctx context.Context, tx interface{}, appealID uuid.UUID) (*appealEntity.Appeal, error)) *appealApp.AppealService {
	repo := &fakeAppealRepository{getByIDFunc: getByID}
	return appealApp.NewAppealService(
		repo,
		unusedModerationRepository{},
		unusedContentRepository{},
		unusedCommentRepository{},
		&outboxInfraRepo.OutboxRepository{},
	)
}

// TestGetAppeal_OwnerCanReadOwnAppeal verifies the appeal owner receives 200
// with the expected response fields.
func TestGetAppeal_OwnerCanReadOwnAppeal(t *testing.T) {
	log := zap.NewNop()

	ownerID := uuid.New()
	appealID := uuid.New()
	existing := appealEntity.NewAppeal(uuid.New(), ownerID, "This is a mistake")
	existing.ID = appealID

	service := newRealAppealServiceForGetAppeal(func(ctx context.Context, tx interface{}, id uuid.UUID) (*appealEntity.Appeal, error) {
		assert.Equal(t, appealID, id)
		return existing, nil
	})
	handler := NewAppealHandler(service, fakeAppealDB{}, log, fakeAdminAuditLogger{})

	router := setupGetAppealRouterWithUser(handler, ownerID)
	router.GET("/appeals/:id", handler.GetAppeal)

	req, _ := http.NewRequest("GET", "/appeals/"+appealID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok, "response envelope must contain a data object")
	appealResp, ok := data["appeal"].(map[string]interface{})
	require.True(t, ok, "response data must contain an appeal object")
	assert.Equal(t, appealID.String(), appealResp["id"])
}

// TestGetAppeal_OtherUserGets404 verifies a different authenticated user gets
// 404 (IDOR prevention), not 403 and not 500.
func TestGetAppeal_OtherUserGets404(t *testing.T) {
	log := zap.NewNop()

	ownerID := uuid.New()
	otherUserID := uuid.New()
	appealID := uuid.New()
	existing := appealEntity.NewAppeal(uuid.New(), ownerID, "This is a mistake")
	existing.ID = appealID

	service := newRealAppealServiceForGetAppeal(func(ctx context.Context, tx interface{}, id uuid.UUID) (*appealEntity.Appeal, error) {
		assert.Equal(t, appealID, id)
		return existing, nil
	})
	handler := NewAppealHandler(service, fakeAppealDB{}, log, fakeAdminAuditLogger{})

	router := setupGetAppealRouterWithUser(handler, otherUserID)
	router.GET("/appeals/:id", handler.GetAppeal)

	req, _ := http.NewRequest("GET", "/appeals/"+appealID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
}
