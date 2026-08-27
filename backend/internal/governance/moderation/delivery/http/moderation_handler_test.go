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
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockModerationService is a mock for testing
type MockModerationService struct {
	mock.Mock
}

func (m *MockModerationService) CreateCase(ctx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID, userID uuid.UUID, reason string) (*entity.GovernanceCase, error) {
	args := m.Called(ctx, resourceType, resourceID, userID, reason)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.GovernanceCase), args.Error(1)
}

func (m *MockModerationService) GetCasesByUser(ctx interface{}, userID uuid.UUID, limit, offset int) ([]*entity.GovernanceCase, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.GovernanceCase), args.Error(1)
}

func (m *MockModerationService) ListCases(ctx interface{}, statusFilter *entity.GovernanceCaseStatus, resourceTypeFilter *entity.ResourceType, limit, offset int) ([]*entity.GovernanceCase, int64, error) {
	args := m.Called(ctx, statusFilter, resourceTypeFilter, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*entity.GovernanceCase), args.Get(1).(int64), args.Error(2)
}

func (m *MockModerationService) GetCase(ctx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
	args := m.Called(ctx, caseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.GovernanceCase), args.Error(1)
}

func (m *MockModerationService) ReviewCase(ctx interface{}, caseID uuid.UUID, adminID uuid.UUID, decision entity.Decision, notes *string) (*entity.GovernanceCase, error) {
	args := m.Called(ctx, caseID, adminID, decision, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.GovernanceCase), args.Error(1)
}

// MockDB is a mock database for testing. Implements db.Transactor.
type MockDB struct {
	mock.Mock
}

func (m *MockDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	args := m.Called(ctx, fn)
	if args.Get(0) != nil {
		return args.Get(0).(error)
	}
	// Execute the function directly for testing (passes nil tx to inner fn)
	return fn(nil)
}

// fakeAdminAuditLogger is a shared no-op test double for this package's
// integration-tagged handler tests (appeal, moderation, warning). It
// implements LogSafe (the local AdminAuditLogger interface used by
// AppealHandler/ModerationHandler) plus Log and LogTx (needed by the shared
// audit.AdminAuditLogger interface WarningHandler depends on), so it
// satisfies whichever shape a given constructor expects without writing to
// any real audit store. PASS_14B: used in place of a literal nil so that any
// future test exercising a success path that reaches the audit call does not
// panic on a nil interface.
type fakeAdminAuditLogger struct{}

func (fakeAdminAuditLogger) Log(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	return nil
}

func (fakeAdminAuditLogger) LogTx(ctx context.Context, tx db.Tx, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	return nil
}

func (fakeAdminAuditLogger) LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) {
}

func setupTestRouter(handler *ModerationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

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


// ============================================================
// SLICE 6: CAPABILITY-BASED AUTH TESTS
// ============================================================

// setupCapabilityTestRouter creates a test router with actor injection
func setupCapabilityTestRouter(handler *ModerationHandler, actor *capabilityEntity.Actor) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Inject actor into context
	router.Use(func(c *gin.Context) {
		if actor != nil {
			ctx := capability.WithActor(c.Request.Context(), actor)
			c.Request = c.Request.WithContext(ctx)
		}
		// Also set user_id for backward compatibility with existing middleware
		if actor != nil {
			c.Set("user_id", actor.ID)
			c.Set("user_role", actor.Role)
		}
		c.Next()
	})

	return router
}

// TestApplyAction_WithCapability_Success tests successful resolution with capability
func TestApplyAction_WithCapability_Success(t *testing.T) {
	log := zap.NewNop()

	handler := NewModerationHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()
	caseID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{capability.CapModerationCaseResolve.String()},
	}

	router := setupCapabilityTestRouter(handler, actor)
	router.Use(func(c *gin.Context) {
		// Recover from panic caused by nil service/db
		defer func() {
			if r := recover(); r != nil {
				// If we get here, it means capability check passed (no 403)
				// but service is nil which causes panic - this is expected
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	})
	router.POST("/admin/moderation/cases/:id/action", handler.ApplyAction)

	reqBody := ApplyActionRequest{
		Action: "approve",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/moderation/cases/"+caseID.String()+"/action", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// The key assertion: we should NOT get 403 Forbidden
	// If we get 500 or anything else except 403, capability check passed
	assert.NotEqual(t, http.StatusForbidden, w.Code, "Should not get Forbidden when capability is present")
}

// TestApplyAction_WithoutCapability_Forbidden tests that admin without capability is rejected
func TestApplyAction_WithoutCapability_Forbidden(t *testing.T) {
	log := zap.NewNop()

	handler := NewModerationHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()
	caseID := uuid.New()

	// Actor is admin but does NOT have moderation.case.resolve capability
	actor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{"moderation.content.view"}, // Only view, not resolve
	}

	router := setupCapabilityTestRouter(handler, actor)
	router.POST("/admin/moderation/cases/:id/action", handler.ApplyAction)

	reqBody := ApplyActionRequest{
		Action: "approve",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/moderation/cases/"+caseID.String()+"/action", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Handler-level defense should reject
	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	errInfo := resp["error"].(map[string]interface{})
	assert.Equal(t, "Insufficient permissions: moderation.case.resolve capability required", errInfo["message"])
}

// TestApplyAction_NoActor_Unauthorized tests that unauthenticated request is rejected
func TestApplyAction_NoActor_Unauthorized(t *testing.T) {
	log := zap.NewNop()

	handler := NewModerationHandler(nil, nil, log, fakeAdminAuditLogger{})

	caseID := uuid.New()

	// No actor at all
	router := setupCapabilityTestRouter(handler, nil)
	router.POST("/admin/moderation/cases/:id/action", handler.ApplyAction)

	reqBody := ApplyActionRequest{
		Action: "approve",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/moderation/cases/"+caseID.String()+"/action", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	errInfo := resp["error"].(map[string]interface{})
	assert.Equal(t, "Insufficient permissions: moderation.case.resolve capability required", errInfo["message"])
}

// TestApplyAction_AdminRoleNoCapability_Forbidden tests NO admin fallback
func TestApplyAction_AdminRoleNoCapability_Forbidden(t *testing.T) {
    // CRITICAL SECURITY TEST: Ensures admin role does NOT grant implicit capability
    log := zap.NewNop()

	handler := NewModerationHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()
	caseID := uuid.New()

	// Admin role but NO capabilities
	actor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{}, // Empty - no explicit capabilities
	}

	router := setupCapabilityTestRouter(handler, actor)
	router.POST("/admin/moderation/cases/:id/action", handler.ApplyAction)

	reqBody := ApplyActionRequest{
		Action: "approve",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/moderation/cases/"+caseID.String()+"/action", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Admin role alone should NOT grant access
	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

    errInfo := resp["error"].(map[string]interface{})
    assert.Equal(t, "Insufficient permissions: moderation.case.resolve capability required", errInfo["message"])
}

func TestApplyAction_AuthorityMatrix_DeniedWithoutResolveCapability(t *testing.T) {
    log := zap.NewNop()
    handler := NewModerationHandler(nil, nil, log, fakeAdminAuditLogger{})

    tests := []struct {
            name  string
            actor *capabilityEntity.Actor
    }{
            {
                    name: "sender",
                    actor: &capabilityEntity.Actor{ID: uuid.New(), Role: "sender"},
            },
            {
                    name: "ordinary participant",
                    actor: &capabilityEntity.Actor{ID: uuid.New(), Role: "participant"},
            },
            {
                    name: "outsider",
                    actor: &capabilityEntity.Actor{ID: uuid.New(), Role: "outsider"},
            },
            {
                    name: "support-only actor",
                    actor: &capabilityEntity.Actor{
                            ID:           uuid.New(),
                            Role:         "support",
                            Capabilities: []string{"support.tier.read"},
                    },
            },
            {
                    name: "generic admin without moderation authority",
                    actor: &capabilityEntity.Actor{
                            ID:           uuid.New(),
                            Role:         "admin",
                            Capabilities: []string{"moderation.case.read"},
                    },
            },
    }

    for _, tc := range tests {
            t.Run(tc.name, func(t *testing.T) {
                    router := setupCapabilityTestRouter(handler, tc.actor)
                    router.POST("/admin/moderation/cases/:id/action", handler.ApplyAction)

                    body, _ := json.Marshal(ApplyActionRequest{Action: "approve"})
                    req, _ := http.NewRequest(http.MethodPost, "/admin/moderation/cases/"+uuid.New().String()+"/action", bytes.NewBuffer(body))
                    req.Header.Set("Content-Type", "application/json")
                    w := httptest.NewRecorder()

                    router.ServeHTTP(w, req)

                    assert.Equal(t, http.StatusForbidden, w.Code)
                    assert.Contains(t, w.Body.String(), "moderation.case.resolve")
            })
    }
}

// TestApplyAction_InvalidDecision_BadRequest tests invalid decision is rejected
func TestApplyAction_InvalidDecision_BadRequest(t *testing.T) {
    log := zap.NewNop()

	handler := NewModerationHandler(nil, nil, log, fakeAdminAuditLogger{})

	adminID := uuid.New()
	caseID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{capability.CapModerationCaseResolve.String()},
	}

	router := setupCapabilityTestRouter(handler, actor)
	router.POST("/admin/moderation/cases/:id/action", handler.ApplyAction)

	// Invalid action - should be caught by binding validation
	reqBody := map[string]interface{}{
		"action": "invalid_action",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/moderation/cases/"+caseID.String()+"/action", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}


