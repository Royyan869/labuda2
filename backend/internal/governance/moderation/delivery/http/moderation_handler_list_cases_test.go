package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	moderationApp "github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	moderationRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type listCasesMockTx struct{}

var _ db.Tx = (*listCasesMockTx)(nil)

func (m *listCasesMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *listCasesMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *listCasesMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return nil
}

func (m *listCasesMockTx) Commit(_ context.Context) error   { return nil }
func (m *listCasesMockTx) Rollback(_ context.Context) error { return nil }

type listCasesMockTransactor struct {
	tx db.Tx
}

func (m *listCasesMockTransactor) WithTx(_ context.Context, fn func(tx db.Tx) error) error {
	if m.tx == nil {
		m.tx = &listCasesMockTx{}
	}
	return fn(m.tx)
}

type listCasesMockRepository struct {
	t                    *testing.T
	expectedStatus       *entity.GovernanceCaseStatus
	expectedResourceType *entity.ResourceType
	expectedCases        []*entity.GovernanceCase
	expectedTotal        int64
}

var _ moderationRepo.ModerationRepository = (*listCasesMockRepository)(nil)

func (m *listCasesMockRepository) Create(context.Context, interface{}, *entity.GovernanceCase) error {
	return nil
}

func (m *listCasesMockRepository) GetByID(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error) {
	return nil, nil
}

func (m *listCasesMockRepository) GetForUpdate(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error) {
	return nil, nil
}

func (m *listCasesMockRepository) Update(context.Context, interface{}, *entity.GovernanceCase) error {
	return nil
}

func (m *listCasesMockRepository) ListPending(context.Context, interface{}, int, int) ([]*entity.GovernanceCase, error) {
	return nil, nil
}

func (m *listCasesMockRepository) ListByResource(context.Context, interface{}, entity.ResourceType, uuid.UUID) ([]*entity.GovernanceCase, error) {
	return nil, nil
}

func (m *listCasesMockRepository) ListByReporter(context.Context, interface{}, uuid.UUID, int, int) ([]*entity.GovernanceCase, error) {
	return nil, nil
}

func (m *listCasesMockRepository) ListWithStatus(
	_ context.Context,
	_ interface{},
	statusFilter *entity.GovernanceCaseStatus,
	resourceTypeFilter *entity.ResourceType,
	limit, offset int,
) ([]*entity.GovernanceCase, int64, error) {
	if m.expectedStatus == nil {
		assert.Nil(m.t, statusFilter)
	} else if assert.NotNil(m.t, statusFilter) {
		assert.Equal(m.t, *m.expectedStatus, *statusFilter)
	}

	if m.expectedResourceType == nil {
		assert.Nil(m.t, resourceTypeFilter)
	} else if assert.NotNil(m.t, resourceTypeFilter) {
		assert.Equal(m.t, *m.expectedResourceType, *resourceTypeFilter)
	}

	assert.Equal(m.t, 20, limit)
	assert.Equal(m.t, 0, offset)

	return m.expectedCases, m.expectedTotal, nil
}

func (m *listCasesMockRepository) ResourceExists(context.Context, interface{}, entity.ResourceType, uuid.UUID) (bool, error) {
	return false, nil
}

func (m *listCasesMockRepository) HasUserReportedResource(context.Context, interface{}, uuid.UUID, entity.ResourceType, uuid.UUID) (bool, error) {
	return false, nil
}

func (m *listCasesMockRepository) ValidateChatMessageReporter(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, string, error) {
	return false, "", nil
}

func newListCasesTestHandler(t *testing.T, expectedStatus *entity.GovernanceCaseStatus, expectedResourceType *entity.ResourceType, expectedCases []*entity.GovernanceCase, expectedTotal int64) *ModerationHandler {
	t.Helper()

	repo := &listCasesMockRepository{
		t:                    t,
		expectedStatus:       expectedStatus,
		expectedResourceType: expectedResourceType,
		expectedCases:        expectedCases,
		expectedTotal:        expectedTotal,
	}
	service := moderationApp.NewModerationService(&listCasesMockTransactor{}, repo, nil)
	return NewModerationHandler(service, nil, zap.NewNop(), nil)
}

func newListCasesTestRouter(actor *capabilityEntity.Actor, handler *ModerationHandler) *gin.Engine {
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
	router.GET("/admin/moderation/cases", handler.ListCases)
	return router
}

func TestListCases_InvalidResourceTypeFilter_ReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewModerationHandler(nil, nil, zap.NewNop(), nil)
	router := newListCasesTestRouter(&capabilityEntity.Actor{
		ID:           uuid.New(),
		Role:         "admin",
		Capabilities: []string{capability.CapModerationCaseRead.String()},
	}, handler)

	req := httptest.NewRequest(http.MethodGet, "/admin/moderation/cases?resource_type=invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid resource_type filter")
}

func TestListCases_ResourceTypeFilter_ForwardsToService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	status := entity.GovernanceCaseStatusPending
	resourceType := entity.ResourceTypeForSale
	expectedCases := []*entity.GovernanceCase{
		entity.NewGovernanceCase(entity.ResourceTypeForSale, uuid.New(), uuid.New(), "fixed-price sale case"),
	}

	handler := newListCasesTestHandler(t, &status, &resourceType, expectedCases, int64(1))
	router := newListCasesTestRouter(&capabilityEntity.Actor{
		ID:           uuid.New(),
		Role:         "admin",
		Capabilities: []string{capability.CapModerationCaseRead.String()},
	}, handler)

	req := httptest.NewRequest(http.MethodGet, "/admin/moderation/cases?status=pending&resource_type=for_sale", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"count":1`)
}

func TestListCases_OmittedResourceType_PreservesBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)

	status := entity.GovernanceCaseStatusPending
	expectedCases := []*entity.GovernanceCase{
		entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "content case"),
	}

	handler := newListCasesTestHandler(t, &status, nil, expectedCases, int64(1))
	router := newListCasesTestRouter(&capabilityEntity.Actor{
		ID:           uuid.New(),
		Role:         "admin",
		Capabilities: []string{capability.CapModerationCaseRead.String()},
	}, handler)

	req := httptest.NewRequest(http.MethodGet, "/admin/moderation/cases?status=pending", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"count":1`)
}


