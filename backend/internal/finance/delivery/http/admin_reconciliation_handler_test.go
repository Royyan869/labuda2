package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/finance/entity"
	"github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// TEST DOUBLES (self-contained; distinct names to avoid collisions with
// other _test.go files, including integration-tagged ones, in this package)
// ============================================================================

// fakeReconciliationTransactor runs the callback directly since the fake
// repository below never dereferences tx.
type fakeReconciliationTransactor struct{}

func (fakeReconciliationTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(nil)
}

// fakeReconciliationRepo is an in-memory ReconciliationRepository test double.
// It records whether any write path was invoked, so tests can prove the
// admin visibility handlers never mutate reconciliation state.
type fakeReconciliationRepo struct {
	results         []*entity.ReconciliationResult
	createCalled    bool
	deleteOldCalled bool
}

func (f *fakeReconciliationRepo) Create(ctx context.Context, tx interface{}, result *entity.ReconciliationResult) error {
	f.createCalled = true
	f.results = append(f.results, result)
	return nil
}

func (f *fakeReconciliationRepo) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.ReconciliationResult, error) {
	for _, r := range f.results {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("reconciliation result not found: %s", id)
}

func (f *fakeReconciliationRepo) applyFilters(filters repository.ReconciliationFilters) []*entity.ReconciliationResult {
	var filtered []*entity.ReconciliationResult
	for _, r := range f.results {
		if filters.Severity != nil && r.Severity != *filters.Severity {
			continue
		}
		if filters.ActionTaken != nil && r.ActionTaken != *filters.ActionTaken {
			continue
		}
		if filters.AutoRepaired != nil && r.AutoRepaired != *filters.AutoRepaired {
			continue
		}
		if filters.DateFrom != nil && r.CheckedAt.Before(*filters.DateFrom) {
			continue
		}
		if filters.DateTo != nil && r.CheckedAt.After(*filters.DateTo) {
			continue
		}
		filtered = append(filtered, r)
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].CheckedAt.After(filtered[j].CheckedAt) })
	return filtered
}

func (f *fakeReconciliationRepo) List(ctx context.Context, tx interface{}, filters repository.ReconciliationFilters) ([]*entity.ReconciliationResult, error) {
	filtered := f.applyFilters(filters)

	offset := filters.Offset
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := len(filtered)
	if filters.Limit > 0 && offset+filters.Limit < end {
		end = offset + filters.Limit
	}
	return filtered[offset:end], nil
}

func (f *fakeReconciliationRepo) Count(ctx context.Context, tx interface{}, filters repository.ReconciliationFilters) (int64, error) {
	return int64(len(f.applyFilters(filters))), nil
}

func (f *fakeReconciliationRepo) GetLatest(ctx context.Context, tx interface{}) (*entity.ReconciliationResult, error) {
	if len(f.results) == 0 {
		return nil, nil
	}
	latest := f.results[0]
	for _, r := range f.results {
		if r.CheckedAt.After(latest.CheckedAt) {
			latest = r
		}
	}
	return latest, nil
}

func (f *fakeReconciliationRepo) GetLatestBySeverity(ctx context.Context, tx interface{}, severity entity.ReconcileSeverity) (*entity.ReconciliationResult, error) {
	var latest *entity.ReconciliationResult
	for _, r := range f.results {
		if r.Severity != severity {
			continue
		}
		if latest == nil || r.CheckedAt.After(latest.CheckedAt) {
			latest = r
		}
	}
	return latest, nil
}

func (f *fakeReconciliationRepo) DeleteOld(ctx context.Context, tx interface{}, olderThan time.Duration) (int, error) {
	f.deleteOldCalled = true
	return 0, nil
}

var _ repository.ReconciliationRepository = (*fakeReconciliationRepo)(nil)

// ============================================================================
// TEST HELPERS
// ============================================================================

func newReconciliationTestActor(capabilities []string) *capabilityEntity.Actor {
	return &capabilityEntity.Actor{ID: uuid.New(), Role: "admin", Capabilities: capabilities}
}

// newReconciliationTestRouter wires the real RequireCapability middleware in
// front of the handler, mirroring production routing (routes_core.go), so
// capability enforcement is proven against the real middleware, not assumed.
func newReconciliationTestRouter(actor *capabilityEntity.Actor, handler *AdminReconciliationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := capability.WithActor(c.Request.Context(), actor)
		c.Request = c.Request.WithContext(ctx)
		c.Set("user_id", actor.ID)
		c.Next()
	})

	group := r.Group("/admin")
	group.Use(middleware.RequireCapability("finance.withdraw.read"))
	group.GET("/reconciliation", handler.ListReconciliationResults)
	group.GET("/reconciliation/latest", handler.GetLatestReconciliationResult)
	group.GET("/reconciliation/:id", handler.GetReconciliationResult)
	return r
}

type listRespWire struct {
	Results []reconciliationResultResponse `json:"results"`
	Total   int64                          `json:"total"`
	Limit   int                            `json:"limit"`
	Offset  int                            `json:"offset"`
}

func seedResult(repo *fakeReconciliationRepo, checkedAt time.Time, severity entity.ReconcileSeverity, action entity.ReconcileAction, total, mismatched int) *entity.ReconciliationResult {
	r := entity.NewReconciliationResult(checkedAt, total, mismatched, severity, entity.ReconcileDetails{"note": "test"})
	r.WithAction(action)
	repo.results = append(repo.results, r)
	return r
}

// ============================================================================
// CAPABILITY ENFORCEMENT
// ============================================================================

func TestListReconciliationResults_WithoutCapability_Forbidden(t *testing.T) {
	repo := &fakeReconciliationRepo{}
	handler := NewAdminReconciliationHandler(fakeReconciliationTransactor{}, repo, zap.NewNop())

	actor := newReconciliationTestActor(nil) // no capabilities
	router := newReconciliationTestRouter(actor, handler)

	req, _ := http.NewRequest(http.MethodGet, "/admin/reconciliation", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "actor without finance.withdraw.read must be forbidden")
}

func TestListReconciliationResults_WithCapability_Allowed(t *testing.T) {
	repo := &fakeReconciliationRepo{}
	seedResult(repo, time.Now(), entity.SeverityReconcilePassed, entity.ActionNone, 0, 0)
	handler := NewAdminReconciliationHandler(fakeReconciliationTransactor{}, repo, zap.NewNop())

	actor := newReconciliationTestActor([]string{"finance.withdraw.read"})
	router := newReconciliationTestRouter(actor, handler)

	req, _ := http.NewRequest(http.MethodGet, "/admin/reconciliation", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "actor with finance.withdraw.read must be allowed")
}

// ============================================================================
// LIST: FILTERING + PAGINATION
// ============================================================================

func TestListReconciliationResults_SeverityFilterAndPagination(t *testing.T) {
	repo := &fakeReconciliationRepo{}
	now := time.Now()
	seedResult(repo, now.Add(-4*time.Hour), entity.SeverityReconcilePassed, entity.ActionNone, 0, 0)
	seedResult(repo, now.Add(-3*time.Hour), entity.SeverityReconcileMedium, entity.ActionEscalated, 2, 1)
	seedResult(repo, now.Add(-2*time.Hour), entity.SeverityReconcileHigh, entity.ActionEscalated, 2, 1)
	seedResult(repo, now.Add(-1*time.Hour), entity.SeverityReconcileHigh, entity.ActionEscalated, 2, 1)

	handler := NewAdminReconciliationHandler(fakeReconciliationTransactor{}, repo, zap.NewNop())
	actor := newReconciliationTestActor([]string{"finance.withdraw.read"})
	router := newReconciliationTestRouter(actor, handler)

	// Filter by severity=high should return only the two HIGH runs.
	req, _ := http.NewRequest(http.MethodGet, "/admin/reconciliation?severity=high", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp listRespWire
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.EqualValues(t, 2, resp.Total)
	assert.Len(t, resp.Results, 2)
	for _, row := range resp.Results {
		assert.Equal(t, "high", row.Severity)
	}
	// Newest first.
	assert.True(t, resp.Results[0].CheckedAt > resp.Results[1].CheckedAt)

	// Pagination: limit=1 offset=1 over the unfiltered 4 rows should return
	// exactly the 2nd-newest row while total still reflects the full count.
	req2, _ := http.NewRequest(http.MethodGet, "/admin/reconciliation?limit=1&offset=1", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var resp2 listRespWire
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	assert.EqualValues(t, 4, resp2.Total, "total must reflect full match count, not the page size")
	require.Len(t, resp2.Results, 1)
	assert.Equal(t, 1, resp2.Limit)
	assert.Equal(t, 1, resp2.Offset)
}

func TestListReconciliationResults_InvalidDateFilter_BadRequest(t *testing.T) {
	repo := &fakeReconciliationRepo{}
	handler := NewAdminReconciliationHandler(fakeReconciliationTransactor{}, repo, zap.NewNop())
	actor := newReconciliationTestActor([]string{"finance.withdraw.read"})
	router := newReconciliationTestRouter(actor, handler)

	req, _ := http.NewRequest(http.MethodGet, "/admin/reconciliation?date_from=not-a-date", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// DETAIL
// ============================================================================

func TestGetReconciliationResult_Found(t *testing.T) {
	repo := &fakeReconciliationRepo{}
	seeded := seedResult(repo, time.Now(), entity.SeverityReconcileCritical, entity.ActionEscalated, 3, 2)
	handler := NewAdminReconciliationHandler(fakeReconciliationTransactor{}, repo, zap.NewNop())
	actor := newReconciliationTestActor([]string{"finance.withdraw.read"})
	router := newReconciliationTestRouter(actor, handler)

	req, _ := http.NewRequest(http.MethodGet, "/admin/reconciliation/"+seeded.ID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var row reconciliationResultResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &row))
	assert.Equal(t, seeded.ID.String(), row.ID)
	assert.Equal(t, "critical", row.Severity)
	assert.Equal(t, 3, row.TotalAccounts)
	assert.Equal(t, 2, row.MismatchedAccounts)
}

// PASS_12B: double_check_passed is never set true by the worker (dead field)
// and must not appear in the admin-facing wire response. Unmarshal into a
// raw map — not reconciliationResultResponse — so this proves the JSON key
// is actually absent from the wire format, not just missing from the struct.
func TestGetReconciliationResult_DoubleCheckPassedNotInResponse(t *testing.T) {
	repo := &fakeReconciliationRepo{}
	seeded := seedResult(repo, time.Now(), entity.SeverityReconcileHigh, entity.ActionEscalated, 2, 1)
	handler := NewAdminReconciliationHandler(fakeReconciliationTransactor{}, repo, zap.NewNop())
	actor := newReconciliationTestActor([]string{"finance.withdraw.read"})
	router := newReconciliationTestRouter(actor, handler)

	req, _ := http.NewRequest(http.MethodGet, "/admin/reconciliation/"+seeded.ID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	_, present := raw["double_check_passed"]
	assert.False(t, present, "double_check_passed must not appear in the admin reconciliation response")
}

func TestGetReconciliationResult_NotFound(t *testing.T) {
	repo := &fakeReconciliationRepo{}
	handler := NewAdminReconciliationHandler(fakeReconciliationTransactor{}, repo, zap.NewNop())
	actor := newReconciliationTestActor([]string{"finance.withdraw.read"})
	router := newReconciliationTestRouter(actor, handler)

	req, _ := http.NewRequest(http.MethodGet, "/admin/reconciliation/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ============================================================================
// LATEST — proves a passed/healthy run is inspectable, not just drift
// ============================================================================

func TestGetLatestReconciliationResult_SurfacesPassedRun(t *testing.T) {
	repo := &fakeReconciliationRepo{}
	seedResult(repo, time.Now().Add(-1*time.Hour), entity.SeverityReconcileHigh, entity.ActionEscalated, 2, 1)
	latestPassed := seedResult(repo, time.Now(), entity.SeverityReconcilePassed, entity.ActionNone, 0, 0)

	handler := NewAdminReconciliationHandler(fakeReconciliationTransactor{}, repo, zap.NewNop())
	actor := newReconciliationTestActor([]string{"finance.withdraw.read"})
	router := newReconciliationTestRouter(actor, handler)

	req, _ := http.NewRequest(http.MethodGet, "/admin/reconciliation/latest", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var row reconciliationResultResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &row))
	assert.Equal(t, latestPassed.ID.String(), row.ID)
	assert.Equal(t, "passed", row.Severity, "a healthy run must be visible, not just drift/alert-worthy runs")
}

func TestGetLatestReconciliationResult_NoResultsYet_NotFound(t *testing.T) {
	repo := &fakeReconciliationRepo{}
	handler := NewAdminReconciliationHandler(fakeReconciliationTransactor{}, repo, zap.NewNop())
	actor := newReconciliationTestActor([]string{"finance.withdraw.read"})
	router := newReconciliationTestRouter(actor, handler)

	req, _ := http.NewRequest(http.MethodGet, "/admin/reconciliation/latest", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ============================================================================
// READ-ONLY PROOF — the visibility handlers must never invoke a write path
// ============================================================================

func TestReconciliationHandlers_NeverMutateRepository(t *testing.T) {
	repo := &fakeReconciliationRepo{}
	seedResult(repo, time.Now(), entity.SeverityReconcileHigh, entity.ActionEscalated, 2, 1)
	handler := NewAdminReconciliationHandler(fakeReconciliationTransactor{}, repo, zap.NewNop())
	actor := newReconciliationTestActor([]string{"finance.withdraw.read"})
	router := newReconciliationTestRouter(actor, handler)

	for _, path := range []string{
		"/admin/reconciliation",
		"/admin/reconciliation/latest",
		"/admin/reconciliation/" + repo.results[0].ID.String(),
	} {
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, "path %s should succeed", path)
	}

	assert.False(t, repo.createCalled, "visibility endpoints must never call Create")
	assert.False(t, repo.deleteOldCalled, "visibility endpoints must never call DeleteOld")
}
