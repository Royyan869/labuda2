package http

import (
	"context"
	"encoding/json"
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
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Mock infrastructure for GetMyCases
// =============================================================================

type myCasesMockTx struct{}

var _ db.Tx = myCasesMockTx{}

func (m myCasesMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m myCasesMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m myCasesMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return myCasesNoRows{}
}
func (m myCasesMockTx) Commit(_ context.Context) error   { return nil }
func (m myCasesMockTx) Rollback(_ context.Context) error { return nil }

type myCasesNoRows struct{}

func (myCasesNoRows) Scan(_ ...any) error { return pgx.ErrNoRows }

type myCasesMockTransactor struct {
	tx myCasesMockTx
}

func (m *myCasesMockTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(m.tx)
}

// myCasesMockRepo implements moderationRepo.ModerationRepository with
// configurable ListByReporter behavior.
type myCasesMockRepo struct {
	cases []*entity.GovernanceCase
	total int64
}

func (m *myCasesMockRepo) Create(_ context.Context, _ interface{}, _ *entity.GovernanceCase) error {
	return nil
}
func (m *myCasesMockRepo) GetByID(_ context.Context, _ interface{}, _ uuid.UUID) (*entity.GovernanceCase, error) {
	return nil, nil
}
func (m *myCasesMockRepo) GetForUpdate(_ context.Context, _ interface{}, _ uuid.UUID) (*entity.GovernanceCase, error) {
	return nil, nil
}
func (m *myCasesMockRepo) Update(_ context.Context, _ interface{}, _ *entity.GovernanceCase) error {
	return nil
}
func (m *myCasesMockRepo) ListPending(_ context.Context, _ interface{}, _, _ int) ([]*entity.GovernanceCase, error) {
	return nil, nil
}
func (m *myCasesMockRepo) ListByResource(_ context.Context, _ interface{}, _ entity.ResourceType, _ uuid.UUID) ([]*entity.GovernanceCase, error) {
	return nil, nil
}
func (m *myCasesMockRepo) ListByReporter(_ context.Context, _ interface{}, _ uuid.UUID, _ *entity.GovernanceCaseStatus, _, _ int) ([]*entity.GovernanceCase, int64, error) {
	return m.cases, m.total, nil
}
func (m *myCasesMockRepo) ListWithStatus(_ context.Context, _ interface{}, _ *entity.GovernanceCaseStatus, _ *entity.ResourceType, _, _ int) ([]*entity.GovernanceCase, int64, error) {
	return nil, 0, nil
}
func (m *myCasesMockRepo) ResourceExists(_ context.Context, _ interface{}, _ entity.ResourceType, _ uuid.UUID) (bool, error) {
	return true, nil
}
func (m *myCasesMockRepo) HasUserReportedResource(_ context.Context, _ interface{}, _ uuid.UUID, _ entity.ResourceType, _ uuid.UUID) (bool, error) {
	return false, nil
}
func (m *myCasesMockRepo) ValidateChatMessageReporter(_ context.Context, _ interface{}, _ uuid.UUID, _ uuid.UUID) (bool, string, error) {
	return true, "", nil
}

// Ensure myCasesMockRepo satisfies the interface
var _ moderationRepo.ModerationRepository = (*myCasesMockRepo)(nil)

// =============================================================================
// Helpers
// =============================================================================

func newMyCasesRouter(userID uuid.UUID, handler *ModerationHandler) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	router.GET("/moderation/my-cases", handler.GetMyCases)
	return router
}

func newMyCasesHandler(repo moderationRepo.ModerationRepository) *ModerationHandler {
	service := moderationApp.NewModerationService(&myCasesMockTransactor{}, repo, nil)
	return NewModerationHandler(service, nil, zap.NewNop(), nil)
}

// =============================================================================
// Tests
// =============================================================================

func TestGetMyCases_NonEmptyPage_ReturnsStandardEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reporterID := uuid.New()
	resourceID := uuid.New()
	now := entity.NewGovernanceCase(entity.ResourceTypeContent, resourceID, reporterID, "spam").CreatedAt

	c1 := &entity.GovernanceCase{
		ID:           uuid.New(),
		ResourceType: entity.ResourceTypeContent,
		ResourceID:   resourceID,
		Status:       entity.GovernanceCaseStatusPending,
		ReportedBy:   reporterID,
		Reason:       "spam: repeated posts",
		CreatedAt:    now,
	}

	repo := &myCasesMockRepo{cases: []*entity.GovernanceCase{c1}, total: 5}
	handler := newMyCasesHandler(repo)
	router := newMyCasesRouter(reporterID, handler)

	req := httptest.NewRequest(http.MethodGet, "/moderation/my-cases?page=1&limit=20", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Cases []struct {
				ID           string `json:"id"`
				ResourceType string `json:"resource_type"`
				ResourceID   string `json:"resource_id"`
				Status       string `json:"status"`
				ReportedBy   string `json:"reported_by"`
				Reason       string `json:"reason"`
				CreatedAt    string `json:"created_at"`
			} `json:"cases"`
			Page  int   `json:"page"`
			Limit int   `json:"limit"`
			Count int64 `json:"count"`
		} `json:"data"`
		Timestamp string `json:"timestamp"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp.Success)
	assert.Equal(t, 1, resp.Data.Page)
	assert.Equal(t, 20, resp.Data.Limit)
	assert.Equal(t, int64(5), resp.Data.Count, "count must be total matching rows (5), not page length (1)")
	assert.Len(t, resp.Data.Cases, 1)
	assert.Equal(t, c1.ID.String(), resp.Data.Cases[0].ID)
	assert.Equal(t, "content", resp.Data.Cases[0].ResourceType)
	assert.Equal(t, "pending", resp.Data.Cases[0].Status)
	assert.Equal(t, reporterID.String(), resp.Data.Cases[0].ReportedBy)
	assert.NotEmpty(t, resp.Timestamp)
}

func TestGetMyCases_GenuineEmpty_ReturnsEmptyArray(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reporterID := uuid.New()
	repo := &myCasesMockRepo{cases: nil, total: 0}
	handler := newMyCasesHandler(repo)
	router := newMyCasesRouter(reporterID, handler)

	req := httptest.NewRequest(http.MethodGet, "/moderation/my-cases", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			Cases []interface{} `json:"cases"`
			Count int64         `json:"count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(0), resp.Data.Count)
	assert.Empty(t, resp.Data.Cases)
}

func TestGetMyCases_DefaultPageAndLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reporterID := uuid.New()
	repo := &myCasesMockRepo{cases: []*entity.GovernanceCase{}, total: 0}
	handler := newMyCasesHandler(repo)
	router := newMyCasesRouter(reporterID, handler)

	req := httptest.NewRequest(http.MethodGet, "/moderation/my-cases", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			Page  int `json:"page"`
			Limit int `json:"limit"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Data.Page)
	assert.Equal(t, 20, resp.Data.Limit)
}

func TestGetMyCases_ExplicitPageAndLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reporterID := uuid.New()
	repo := &myCasesMockRepo{cases: []*entity.GovernanceCase{}, total: 0}
	handler := newMyCasesHandler(repo)
	router := newMyCasesRouter(reporterID, handler)

	req := httptest.NewRequest(http.MethodGet, "/moderation/my-cases?page=3&limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			Page  int `json:"page"`
			Limit int `json:"limit"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 3, resp.Data.Page)
	assert.Equal(t, 5, resp.Data.Limit)
}

func TestGetMyCases_LimitCappedAtMaximum(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reporterID := uuid.New()
	repo := &myCasesMockRepo{cases: []*entity.GovernanceCase{}, total: 0}
	handler := newMyCasesHandler(repo)
	router := newMyCasesRouter(reporterID, handler)

	req := httptest.NewRequest(http.MethodGet, "/moderation/my-cases?limit=200", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			Limit int `json:"limit"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 100, resp.Data.Limit, "limit above 100 must clamp to 100")
}

func TestGetMyCases_ValidStatusFilter_Accepted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reporterID := uuid.New()
	c1 := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), reporterID, "test")
	c1.Status = entity.GovernanceCaseStatusEnforced

	repo := &myCasesMockRepo{cases: []*entity.GovernanceCase{c1}, total: 1}
	handler := newMyCasesHandler(repo)
	router := newMyCasesRouter(reporterID, handler)

	req := httptest.NewRequest(http.MethodGet, "/moderation/my-cases?status=enforced", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			Cases []struct {
				Status string `json:"status"`
			} `json:"cases"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Data.Cases, 1)
	assert.Equal(t, "enforced", resp.Data.Cases[0].Status)
}

func TestGetMyCases_InvalidStatus_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reporterID := uuid.New()
	repo := &myCasesMockRepo{cases: nil, total: 0}
	handler := newMyCasesHandler(repo)
	router := newMyCasesRouter(reporterID, handler)

	req := httptest.NewRequest(http.MethodGet, "/moderation/my-cases?status=under_review", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid status filter")
}

func TestGetMyCases_CountExceedsPageLength(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reporterID := uuid.New()
	var cases []*entity.GovernanceCase
	for i := 0; i < 3; i++ {
		c := entity.NewGovernanceCase(entity.ResourceTypeComment, uuid.New(), reporterID, "test")
		cases = append(cases, c)
	}

	// 3 items on this page, but 25 total — proves count ≠ page length
	repo := &myCasesMockRepo{cases: cases, total: 25}
	handler := newMyCasesHandler(repo)
	router := newMyCasesRouter(reporterID, handler)

	req := httptest.NewRequest(http.MethodGet, "/moderation/my-cases?page=1&limit=20", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			Cases []interface{} `json:"cases"`
			Count int64         `json:"count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Data.Cases, 3, "3 items on current page")
	assert.Equal(t, int64(25), resp.Data.Count, "total is 25, not 3")
}

func TestGetMyCases_MissingAuthentication_ReturnsError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &myCasesMockRepo{cases: nil, total: 0}
	handler := newMyCasesHandler(repo)

	// Router WITHOUT the user_id middleware
	router := gin.New()
	router.GET("/moderation/my-cases", handler.GetMyCases)

	req := httptest.NewRequest(http.MethodGet, "/moderation/my-cases", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// MustGetUserIDFromContext returns 401 when user_id is missing
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetMyCases_LimitParsingEdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name      string
		query     string
		wantLimit int
	}{
		{name: "absent", query: "", wantLimit: 20},
		{name: "one", query: "?limit=1", wantLimit: 1},
		{name: "max", query: "?limit=100", wantLimit: 100},
		{name: "over max", query: "?limit=200", wantLimit: 100},
		{name: "malformed", query: "?limit=abc", wantLimit: 20},
		{name: "zero", query: "?limit=0", wantLimit: 20},
		{name: "negative", query: "?limit=-5", wantLimit: 20},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reporterID := uuid.New()
			repo := &myCasesMockRepo{cases: []*entity.GovernanceCase{}, total: 0}
			handler := newMyCasesHandler(repo)
			router := newMyCasesRouter(reporterID, handler)

			req := httptest.NewRequest(http.MethodGet, "/moderation/my-cases"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp struct {
				Data struct {
					Limit int `json:"limit"`
					Page  int `json:"page"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, tc.wantLimit, resp.Data.Limit)
			assert.Equal(t, 1, resp.Data.Page)
		})
	}
}
