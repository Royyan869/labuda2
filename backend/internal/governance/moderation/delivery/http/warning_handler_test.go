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
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// PASS_14D: real fakes for WarningService's dependencies.
// ============================================================================

type fakeWarningRow struct{}

func (fakeWarningRow) Scan(_ ...any) error { return nil }

type fakeWarningTx struct{}

func (fakeWarningTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (fakeWarningTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (fakeWarningTx) QueryRow(context.Context, string, ...any) pgx.Row       { return fakeWarningRow{} }
func (fakeWarningTx) Commit(context.Context) error                          { return nil }
func (fakeWarningTx) Rollback(context.Context) error                        { return nil }

var _ db.Tx = fakeWarningTx{}

type fakeWarningDB struct{}

func (fakeWarningDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(fakeWarningTx{})
}

var _ db.Transactor = fakeWarningDB{}

type fakeWarningRepository struct {
	getByIDFunc          func(ctx context.Context, tx interface{}, warningID uuid.UUID) (*warningEntity.UserWarning, error)
	getForUpdateFunc     func(ctx context.Context, tx interface{}, warningID uuid.UUID) (*warningEntity.UserWarning, error)
	updateFunc           func(ctx context.Context, tx interface{}, warning *warningEntity.UserWarning) error
	listByUserFunc       func(ctx context.Context, tx interface{}, userID uuid.UUID, limit, offset int) ([]*warningEntity.UserWarning, error)
	listActiveByUserFunc func(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*warningEntity.UserWarning, error)
	listAllFunc          func(ctx context.Context, tx interface{}, userID *uuid.UUID, isActive *bool, limit, offset int) ([]*warningEntity.UserWarning, int64, error)
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

func TestGetWarning(t *testing.T) {
	log := zap.NewNop()

	ownerID := uuid.New()
	warningID := uuid.New()
	existing := &warningEntity.UserWarning{UserID: ownerID, IssuedBy: uuid.New(), Level: warningEntity.WarningLevelWarning, Reason: "Policy violation", IsActive: true}
	existing.ID = warningID

	repo := &fakeWarningRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*warningEntity.UserWarning, error) {
			assert.Equal(t, warningID, id)
			return existing, nil
		},
	}
	service := warningApp.NewWarningService(repo)
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
	w1 := &warningEntity.UserWarning{UserID: ownerID, IssuedBy: uuid.New(), Level: warningEntity.WarningLevelInfo, Reason: "First", IsActive: true}
	w2 := &warningEntity.UserWarning{UserID: ownerID, IssuedBy: uuid.New(), Level: warningEntity.WarningLevelSevere, Reason: "Second", IsActive: true}

	repo := &fakeWarningRepository{
		listByUserFunc: func(ctx context.Context, tx interface{}, userID uuid.UUID, limit, offset int) ([]*warningEntity.UserWarning, error) {
			assert.Equal(t, ownerID, userID)
			assert.Equal(t, 20, limit)
			assert.Equal(t, 0, offset)
			return []*warningEntity.UserWarning{w1, w2}, nil
		},
	}
	service := warningApp.NewWarningService(repo)
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
	active := &warningEntity.UserWarning{UserID: ownerID, IssuedBy: uuid.New(), Level: warningEntity.WarningLevelSevere, Reason: "Still active", IsActive: true}

	repo := &fakeWarningRepository{
		listActiveByUserFunc: func(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*warningEntity.UserWarning, error) {
			assert.Equal(t, ownerID, userID)
			return []*warningEntity.UserWarning{active}, nil
		},
	}
	service := warningApp.NewWarningService(repo)
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
	existing := &warningEntity.UserWarning{UserID: uuid.New(), IssuedBy: uuid.New(), Level: warningEntity.WarningLevelWarning, Reason: "Policy violation", IsActive: true}
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
	service := warningApp.NewWarningService(repo)
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
	warning := &warningEntity.UserWarning{
			UserID:   userID,
			IssuedBy: adminID,
			Level:    warningEntity.WarningLevelWarning,
			Reason:   "Policy violation",
			IsActive: true,
			ExpiresAt: nil,
		}

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
		warning := &warningEntity.UserWarning{
			UserID:   userID,
			IssuedBy: adminID,
			Level:    warningEntity.WarningLevelWarning,
			Reason:   "Test reason",
			IsActive: true,
			ExpiresAt: nil,
		}
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
		warningWithExpiry := &warningEntity.UserWarning{
			UserID:   userID,
			IssuedBy: adminID,
			Level:    warningEntity.WarningLevelSevere,
			Reason:   "Severe violation",
			IsActive: true,
			ExpiresAt: &expiresAt,
		}
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
