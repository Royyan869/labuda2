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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	warningApp "github.com/labuda/backend/internal/governance/moderation/application"
	warningEntity "github.com/labuda/backend/internal/governance/moderation/entity"
	userentity "github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// PASS_14D: real fakes for WarningService's dependencies.
//
// WarningService (unlike AppealService) takes 3 small interfaces with no
// nil-panic guards in its constructor, so a fully real service can be built
// directly — no live Postgres, no cross-domain stubs required.
// ============================================================================

// fakeWarningRow is a minimal pgx.Row satisfying value; never expected to be
// scanned by these tests since QueryRow is never invoked through the fakes
// below, but returning a valid Row (rather than nil) avoids a latent nil
// panic if that ever changes.
type fakeWarningRow struct{}

func (fakeWarningRow) Scan(_ ...any) error { return nil }

// fakeWarningTx is a minimal no-op db.Tx. WarningService.IssueWarning does a
// runtime tx.(db.Tx) assertion, so a real (non-nil) db.Tx value must flow
// through fakeWarningDB.WithTx for the success path to be reachable.
type fakeWarningTx struct{}

func (fakeWarningTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (fakeWarningTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (fakeWarningTx) QueryRow(context.Context, string, ...any) pgx.Row       { return fakeWarningRow{} }
func (fakeWarningTx) Commit(context.Context) error                          { return nil }
func (fakeWarningTx) Rollback(context.Context) error                        { return nil }

var _ db.Tx = fakeWarningTx{}

// fakeWarningDB implements db.Transactor without a live Postgres connection.
type fakeWarningDB struct{}

func (fakeWarningDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(fakeWarningTx{})
}

var _ db.Transactor = fakeWarningDB{}

// fakeWarningRepository is an explicit test double for repository.WarningRepository.
// Only methods a given test configures behave meaningfully; any unconfigured
// method panics loudly so an unexpected call fails the test clearly instead
// of silently returning a zero value.
type fakeWarningRepository struct {
	createFunc           func(ctx context.Context, tx interface{}, warning *warningEntity.UserWarning) error
	getByIDFunc          func(ctx context.Context, tx interface{}, warningID uuid.UUID) (*warningEntity.UserWarning, error)
	getForUpdateFunc     func(ctx context.Context, tx interface{}, warningID uuid.UUID) (*warningEntity.UserWarning, error)
	updateFunc           func(ctx context.Context, tx interface{}, warning *warningEntity.UserWarning) error
	listByUserFunc       func(ctx context.Context, tx interface{}, userID uuid.UUID, limit, offset int) ([]*warningEntity.UserWarning, error)
	listActiveByUserFunc func(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*warningEntity.UserWarning, error)
	listAllFunc          func(ctx context.Context, tx interface{}, userID *uuid.UUID, isActive *bool, limit, offset int) ([]*warningEntity.UserWarning, int64, error)
}

func (f *fakeWarningRepository) Create(ctx context.Context, tx interface{}, warning *warningEntity.UserWarning) error {
	if f.createFunc == nil {
		panic("fakeWarningRepository.Create called without createFunc configured")
	}
	return f.createFunc(ctx, tx, warning)
}

func (f *fakeWarningRepository) GetByID(ctx context.Context, tx interface{}, warningID uuid.UUID) (*warningEntity.UserWarning, error) {
	if f.getByIDFunc == nil {
		panic("fakeWarningRepository.GetByID called without getByIDFunc configured")
	}
	return f.getByIDFunc(ctx, tx, warningID)
}

func (f *fakeWarningRepository) GetForUpdate(ctx context.Context, tx interface{}, warningID uuid.UUID) (*warningEntity.UserWarning, error) {
	if f.getForUpdateFunc == nil {
		panic("fakeWarningRepository.GetForUpdate called without getForUpdateFunc configured")
	}
	return f.getForUpdateFunc(ctx, tx, warningID)
}

func (f *fakeWarningRepository) Update(ctx context.Context, tx interface{}, warning *warningEntity.UserWarning) error {
	if f.updateFunc == nil {
		panic("fakeWarningRepository.Update called without updateFunc configured")
	}
	return f.updateFunc(ctx, tx, warning)
}

func (f *fakeWarningRepository) ListByUser(ctx context.Context, tx interface{}, userID uuid.UUID, limit, offset int) ([]*warningEntity.UserWarning, error) {
	if f.listByUserFunc == nil {
		panic("fakeWarningRepository.ListByUser called without listByUserFunc configured")
	}
	return f.listByUserFunc(ctx, tx, userID, limit, offset)
}

func (f *fakeWarningRepository) ListActiveByUser(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*warningEntity.UserWarning, error) {
	if f.listActiveByUserFunc == nil {
		panic("fakeWarningRepository.ListActiveByUser called without listActiveByUserFunc configured")
	}
	return f.listActiveByUserFunc(ctx, tx, userID)
}

func (f *fakeWarningRepository) ListAll(ctx context.Context, tx interface{}, userID *uuid.UUID, isActive *bool, limit, offset int) ([]*warningEntity.UserWarning, int64, error) {
	if f.listAllFunc == nil {
		panic("fakeWarningRepository.ListAll called without listAllFunc configured")
	}
	return f.listAllFunc(ctx, tx, userID, isActive, limit, offset)
}

// fakeWarningUserLookup satisfies WarningService's unexported
// warningTargetUserLookup interface structurally (GetByID only).
type fakeWarningUserLookup struct {
	user *userentity.User
	err  error
}

func (f fakeWarningUserLookup) GetByID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*userentity.User, error) {
	return f.user, f.err
}

// fakeWarningOutbox satisfies WarningService's unexported warningOutboxWriter
// interface structurally (InsertEvent only). Always a no-op: no real outbox
// writes happen in these tests.
type fakeWarningOutbox struct{}

func (fakeWarningOutbox) InsertEvent(ctx context.Context, tx db.Tx, eventType string, entityID uuid.UUID, payload []byte) error {
	return nil
}

// setupWarningRouter creates a test router with an explicit, caller-known
// user_id in context (needed to exercise ownership checks deterministically).
func setupWarningRouter(handler *WarningHandler, userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", "user")
		c.Next()
	})
	return router
}

func decodeResponseData(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &resp))
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok, "response envelope must contain a data object, got: %s", string(body))
	return data
}

func TestAdminIssueWarning(t *testing.T) {
	log := zap.NewNop()

	targetUserID := uuid.New()
	adminID := uuid.New()
	expiresAt := int64(time.Now().Add(30 * 24 * time.Hour).Unix())

	repo := &fakeWarningRepository{
		createFunc: func(ctx context.Context, tx interface{}, warning *warningEntity.UserWarning) error {
			assert.Equal(t, targetUserID, warning.UserID)
			assert.Equal(t, adminID, warning.IssuedBy)
			return nil
		},
	}
	service := warningApp.NewWarningService(repo, fakeWarningUserLookup{user: &userentity.User{ID: targetUserID}}, fakeWarningOutbox{})
	handler := NewWarningHandler(service, fakeWarningDB{}, log, fakeAdminAuditLogger{})

	reqBody := CreateWarningRequest{
		UserID:    targetUserID.String(),
		Level:     "warning",
		Reason:    "Policy violation",
		ExpiresAt: &expiresAt,
	}
	body, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", adminID)
		c.Set("user_role", "admin")
		c.Next()
	})
	router.POST("/warnings", handler.AdminIssueWarning)

	req, _ := http.NewRequest("POST", "/warnings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	data := decodeResponseData(t, w.Body.Bytes())
	assert.Equal(t, targetUserID.String(), data["user_id"])
	assert.Equal(t, "warning", data["level"])
	assert.Equal(t, "Policy violation", data["reason"])
	assert.Equal(t, true, data["is_active"])
}

func TestGetWarning(t *testing.T) {
	log := zap.NewNop()

	ownerID := uuid.New()
	warningID := uuid.New()
	existing := warningEntity.NewWarning(ownerID, uuid.New(), warningEntity.WarningLevelWarning, "Policy violation", nil)
	existing.ID = warningID

	repo := &fakeWarningRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*warningEntity.UserWarning, error) {
			assert.Equal(t, warningID, id)
			return existing, nil
		},
	}
	service := warningApp.NewWarningService(repo, fakeWarningUserLookup{}, fakeWarningOutbox{})
	handler := NewWarningHandler(service, fakeWarningDB{}, log, fakeAdminAuditLogger{})

	router := setupWarningRouter(handler, ownerID)
	router.GET("/warnings/:id", handler.GetWarning)

	req, _ := http.NewRequest("GET", "/warnings/"+warningID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	data := decodeResponseData(t, w.Body.Bytes())
	warningResp, ok := data["warning"].(map[string]interface{})
	require.True(t, ok, "response data must contain a warning object")
	assert.Equal(t, warningID.String(), warningResp["id"])
	assert.Equal(t, "active", warningResp["status"])
}

func TestListWarnings(t *testing.T) {
	log := zap.NewNop()

	ownerID := uuid.New()
	w1 := warningEntity.NewWarning(ownerID, uuid.New(), warningEntity.WarningLevelInfo, "First", nil)
	w2 := warningEntity.NewWarning(ownerID, uuid.New(), warningEntity.WarningLevelSevere, "Second", nil)

	repo := &fakeWarningRepository{
		listByUserFunc: func(ctx context.Context, tx interface{}, userID uuid.UUID, limit, offset int) ([]*warningEntity.UserWarning, error) {
			assert.Equal(t, ownerID, userID)
			assert.Equal(t, 20, limit)
			assert.Equal(t, 0, offset)
			return []*warningEntity.UserWarning{w1, w2}, nil
		},
	}
	service := warningApp.NewWarningService(repo, fakeWarningUserLookup{}, fakeWarningOutbox{})
	handler := NewWarningHandler(service, fakeWarningDB{}, log, fakeAdminAuditLogger{})

	router := setupWarningRouter(handler, ownerID)
	router.GET("/warnings", handler.ListWarnings)

	req, _ := http.NewRequest("GET", "/warnings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	data := decodeResponseData(t, w.Body.Bytes())
	assert.EqualValues(t, 2, data["count"])
	warnings, ok := data["warnings"].([]interface{})
	require.True(t, ok)
	assert.Len(t, warnings, 2)
}

func TestGetActiveWarnings(t *testing.T) {
	log := zap.NewNop()

	ownerID := uuid.New()
	active := warningEntity.NewWarning(ownerID, uuid.New(), warningEntity.WarningLevelSevere, "Still active", nil)

	repo := &fakeWarningRepository{
		listActiveByUserFunc: func(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*warningEntity.UserWarning, error) {
			assert.Equal(t, ownerID, userID)
			return []*warningEntity.UserWarning{active}, nil
		},
	}
	service := warningApp.NewWarningService(repo, fakeWarningUserLookup{}, fakeWarningOutbox{})
	handler := NewWarningHandler(service, fakeWarningDB{}, log, fakeAdminAuditLogger{})

	router := setupWarningRouter(handler, ownerID)
	router.GET("/users/:id/warnings/active", handler.GetActiveWarnings)

	req, _ := http.NewRequest("GET", "/users/"+ownerID.String()+"/warnings/active", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	data := decodeResponseData(t, w.Body.Bytes())
	assert.EqualValues(t, 1, data["count"])
}

func TestAdminRevokeWarning(t *testing.T) {
	log := zap.NewNop()

	adminID := uuid.New()
	warningID := uuid.New()
	existing := warningEntity.NewWarning(uuid.New(), uuid.New(), warningEntity.WarningLevelWarning, "Policy violation", nil)
	existing.ID = warningID

	repo := &fakeWarningRepository{
		getForUpdateFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*warningEntity.UserWarning, error) {
			assert.Equal(t, warningID, id)
			return existing, nil
		},
		updateFunc: func(ctx context.Context, tx interface{}, warning *warningEntity.UserWarning) error {
			assert.False(t, warning.IsActive, "warning must be revoked before persisting")
			return nil
		},
	}
	service := warningApp.NewWarningService(repo, fakeWarningUserLookup{}, fakeWarningOutbox{})
	handler := NewWarningHandler(service, fakeWarningDB{}, log, fakeAdminAuditLogger{})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", adminID)
		c.Set("user_role", "admin")
		c.Next()
	})
	router.DELETE("/warnings/:id/revoke", handler.AdminRevokeWarning)

	req, _ := http.NewRequest("DELETE", "/warnings/"+warningID.String()+"/revoke", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	data := decodeResponseData(t, w.Body.Bytes())
	assert.Equal(t, false, data["is_active"])
}

// Test warningToResponse tests the response formatter
func TestWarningToResponse(t *testing.T) {
	log := zap.NewNop()
	handler := NewWarningHandler(nil, nil, log, fakeAdminAuditLogger{})

	userID := uuid.New()
	adminID := uuid.New()
	warning := warningEntity.NewWarning(userID, adminID, warningEntity.WarningLevelWarning, "Policy violation", nil)

	resp := handler.warningToResponse(warning)

	assert.Equal(t, warning.ID, resp["id"])
	assert.Equal(t, userID, resp["user_id"])
	assert.Equal(t, "warning", resp["level"])
	assert.Equal(t, "Policy violation", resp["reason"])
	assert.Equal(t, true, resp["is_active"])
	assert.Equal(t, "active", resp["status"])
	assert.NotNil(t, resp["created_at"])
}

// TestWarningContractSafety validates the warning request/response contract
// This test ensures that Flutter clients can correctly parse warning responses
// and that requests match backend validation requirements.
func TestWarningContractSafety(t *testing.T) {
	t.Run("valid warning levels", func(t *testing.T) {
		validLevels := []string{"info", "warning", "severe"}
		for _, level := range validLevels {
			entityLevel := warningEntity.WarningLevel(level)
			assert.True(t, entityLevel.IsValid(), "level %s should be valid", level)
		}
	})

	t.Run("invalid warning levels are rejected", func(t *testing.T) {
		invalidLevels := []string{"first", "second", "final_", "critical", "ban"}
		for _, level := range invalidLevels {
			entityLevel := warningEntity.WarningLevel(level)
			assert.False(t, entityLevel.IsValid(), "level %s should be invalid", level)
		}
	})

	t.Run("warning status values", func(t *testing.T) {
		validStatuses := []string{"active", "revoked", "expired"}
		for _, status := range validStatuses {
			entityStatus := warningEntity.WarningStatus(status)
			// Verify status is a valid WarningStatus value
			assert.Contains(t, []warningEntity.WarningStatus{
				warningEntity.WarningStatusActive,
				warningEntity.WarningStatusRevoked,
				warningEntity.WarningStatusExpired,
			}, entityStatus, "status %s should be valid", status)
		}
	})

	t.Run("invalid status values are not recognized", func(t *testing.T) {
		invalidStatuses := []string{"acknowledged", "pending", "under_review"}
		for _, status := range invalidStatuses {
			entityStatus := warningEntity.WarningStatus(status)
			// These should not match any valid status
			assert.NotEqual(t, warningEntity.WarningStatusActive, entityStatus)
			assert.NotEqual(t, warningEntity.WarningStatusRevoked, entityStatus)
			assert.NotEqual(t, warningEntity.WarningStatusExpired, entityStatus)
		}
	})

	t.Run("response contains all required fields", func(t *testing.T) {
		log := zap.NewNop()
		handler := NewWarningHandler(nil, nil, log, fakeAdminAuditLogger{})

		userID := uuid.New()
		adminID := uuid.New()

		// Test with no expiration
		warning := warningEntity.NewWarning(userID, adminID, warningEntity.WarningLevelWarning, "Test reason", nil)
		resp := handler.warningToResponse(warning)

		// Required fields for Flutter client
		requiredFields := []string{"id", "user_id", "level", "reason", "is_active", "status", "created_at"}
		for _, field := range requiredFields {
			assert.Contains(t, resp, field, "response must contain field: %s", field)
		}

		// Verify field types match contract
		assert.IsType(t, uuid.UUID{}, resp["id"], "id must be UUID")
		assert.IsType(t, uuid.UUID{}, resp["user_id"], "user_id must be UUID")
		assert.IsType(t, string(""), resp["level"], "level must be string")
		assert.IsType(t, string(""), resp["reason"], "reason must be string")
		assert.IsType(t, false, resp["is_active"], "is_active must be bool")
		assert.IsType(t, string(""), resp["status"], "status must be string")

		// Test with expiration
		expiresAt := time.Now().Add(30 * 24 * time.Hour)
		warningWithExpiry := warningEntity.NewWarning(userID, adminID, warningEntity.WarningLevelSevere, "Severe violation", &expiresAt)
		respWithExpiry := handler.warningToResponse(warningWithExpiry)

		assert.Contains(t, respWithExpiry, "expires_at", "response with expiry must contain expires_at")

		// Test revoked warning
		err := warning.Revoke(adminID)
		assert.NoError(t, err)
		respRevoked := handler.warningToResponse(warning)

		assert.Contains(t, respRevoked, "revoked_at", "revoked warning must contain revoked_at")
		assert.Contains(t, respRevoked, "revoked_by", "revoked warning must contain revoked_by")
		assert.Equal(t, false, respRevoked["is_active"], "revoked warning must have is_active=false")
	})
}
