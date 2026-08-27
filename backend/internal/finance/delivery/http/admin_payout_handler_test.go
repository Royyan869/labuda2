//go:build integration

// Package http provides tests for admin payout handler capability-based authorization.
//
// SLICE 3: FINANCE WITHDRAWAL REVIEW MIGRATION
//
// This test file verifies that the withdrawal approve/reject endpoints
// properly enforce capability-based authorization using finance.withdraw.review.
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
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/finance/application"
	withdrawrepo "github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"

	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockWithdrawService is a mock for WithdrawService
type MockWithdrawService struct {
	mock.Mock
}

func (m *MockWithdrawService) RequestWithdraw(ctx context.Context, withdrawID uuid.UUID, req application.RequestWithdrawRequest) (*application.RequestWithdrawResponse, error) {
	args := m.Called(ctx, withdrawID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*application.RequestWithdrawResponse), args.Error(1)
}

func (m *MockWithdrawService) ApproveWithdraw(ctx context.Context, adminID uuid.UUID, withdrawID uuid.UUID) error {
	args := m.Called(ctx, adminID, withdrawID)
	return args.Error(0)
}

func (m *MockWithdrawService) RejectWithdraw(ctx context.Context, adminID uuid.UUID, withdrawID uuid.UUID) error {
	args := m.Called(ctx, adminID, withdrawID)
	return args.Error(0)
}

func (m *MockWithdrawService) GetWithdrawalForUpdate(ctx context.Context, tx db.Tx, withdrawID uuid.UUID) (*withdrawrepo.Withdrawal, error) {
	args := m.Called(ctx, tx, withdrawID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*withdrawrepo.Withdrawal), args.Error(1)
}

// MockDB is a mock database for testing
type MockDB struct {
	mock.Mock
}

func (m *MockDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	args := m.Called(ctx, fn)
	if args.Get(0) != nil {
		return args.Get(0).(error)
	}
	// Execute the function directly for testing
	return fn(nil)
}

// MockAuditLogger is a mock for audit logging
type MockAuditLogger struct{}

func (m *MockAuditLogger) Log(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	return nil
}

func (m *MockAuditLogger) LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) {
	// No-op for tests
}

func (m *MockAuditLogger) LogTx(ctx context.Context, tx db.Tx, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	return nil
}

// Compile-time assertion
var _ audit.AdminAuditLogger = (*MockAuditLogger)(nil)

// ============================================================================
// TEST HELPERS
// ============================================================================

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// createTestActor creates an Actor for testing with specified capabilities
func createTestActor(id uuid.UUID, role string, capabilities []string) *capabilityEntity.Actor {
	return &capabilityEntity.Actor{
		ID:           id,
		Role:         role,
		Capabilities: capabilities,
	}
}

// injectActorIntoContext injects an actor into the gin context
func injectActorIntoContext(c *gin.Context, actor *capabilityEntity.Actor) {
	ctx := capability.WithActor(c.Request.Context(), actor)
	c.Request = c.Request.WithContext(ctx)
	// Also set user_id for compatibility with existing middleware
	c.Set("user_id", actor.ID)
	c.Set("userID", actor.ID)
}

// createHandler creates a test handler with minimal real dependencies
// Note: This creates real service instances with nil/optional dependencies
// The handler will work but actual service calls may fail
func createHandler() *AdminPayoutHandler {
	log := zap.NewNop()
	auditLogger := &MockAuditLogger{}

	// Create a minimal WithdrawService with nil dependencies
	// The service will be created but many operations will fail
	withdrawService := application.NewWithdrawService(
		nil, // db - will cause WithTx to fail, but handler may not call it
		nil, // ledgerRepo
		nil, // withdrawRepo
		nil, // bankAccountRepo
		nil, // roleChecker
		nil, // accountStatusChecker
		auditLogger,
		nil, // verificationService
		nil, // outboxRepo
	)

	return NewAdminPayoutHandler(withdrawService, nil, auditLogger, log, nil)
}

// ============================================================================
// TEST 1: SUCCESS CASE - User WITH capability can approve/reject
// ============================================================================

func TestApproveWithdrawal_WithCapability_Success(t *testing.T) {
	handler := createHandler()

	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		// Inject an actor WITH finance.withdraw.review capability
		adminID := uuid.New()
		actor := createTestActor(adminID, "admin", []string{"finance.withdraw.review"})
		injectActorIntoContext(c, actor)
		c.Next()
	})
	router.POST("/admin/payouts/withdrawals/:id/approve", handler.ApproveWithdrawal)

	withdrawalID := uuid.New()
	reqBody := ApproveWithdrawalRequest{Notes: "Approved"}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/payouts/withdrawals/"+withdrawalID.String()+"/approve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should succeed - actor has the capability
	// Note: May return 500 if mockDB doesn't properly simulate the withdrawal lookup
	// The key is that it should NOT return 403 Forbidden
	assert.NotEqual(t, http.StatusForbidden, w.Code, "Request with capability should not be forbidden")
}

func TestRejectWithdrawal_WithCapability_Success(t *testing.T) {
	handler := createHandler()

	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		// Inject an actor WITH finance.withdraw.review capability
		adminID := uuid.New()
		actor := createTestActor(adminID, "admin", []string{"finance.withdraw.review"})
		injectActorIntoContext(c, actor)
		c.Next()
	})
	router.POST("/admin/payouts/withdrawals/:id/reject", handler.RejectWithdrawal)

	withdrawalID := uuid.New()

	reqBody := RejectWithdrawalRequest{Reason: "Insufficient funds"}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/payouts/withdrawals/"+withdrawalID.String()+"/reject", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should succeed - actor has the capability
	// Note: May return 500 if mockDB doesn't properly simulate the withdrawal lookup
	// The key is that it should NOT return 403 Forbidden
	assert.NotEqual(t, http.StatusForbidden, w.Code, "Request with capability should not be forbidden")
}

// ============================================================================
// TEST 2: FORBIDDEN CASE - Admin WITHOUT capability cannot approve/reject
// ============================================================================

func TestApproveWithdrawal_AdminWithoutCapability_Forbidden(t *testing.T) {
	handler := createHandler()

	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		// Inject an admin actor WITHOUT finance.withdraw.review capability
		adminID := uuid.New()
		actor := createTestActor(adminID, "admin", []string{}) // No capabilities!
		injectActorIntoContext(c, actor)
		c.Next()
	})
	router.POST("/admin/payouts/withdrawals/:id/approve", handler.ApproveWithdrawal)

	withdrawalID := uuid.New()
	reqBody := ApproveWithdrawalRequest{Notes: "Should fail"}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/payouts/withdrawals/"+withdrawalID.String()+"/approve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// MUST return 403 Forbidden - admin without capability
	assert.Equal(t, http.StatusForbidden, w.Code, "Admin without capability must be forbidden")

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp, "error")
}

func TestRejectWithdrawal_AdminWithoutCapability_Forbidden(t *testing.T) {
	handler := createHandler()

	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		// Inject an admin actor WITHOUT finance.withdraw.review capability
		adminID := uuid.New()
		actor := createTestActor(adminID, "admin", []string{}) // No capabilities!
		injectActorIntoContext(c, actor)
		c.Next()
	})
	router.POST("/admin/payouts/withdrawals/:id/reject", handler.RejectWithdrawal)

	withdrawalID := uuid.New()
	reqBody := RejectWithdrawalRequest{Reason: "Should fail"}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/payouts/withdrawals/"+withdrawalID.String()+"/reject", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// MUST return 403 Forbidden - admin without capability
	assert.Equal(t, http.StatusForbidden, w.Code, "Admin without capability must be forbidden")

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp, "error")
}

// ============================================================================
// TEST 3: UNAUTHENTICATED CASE - No actor in context
// ============================================================================

func TestApproveWithdrawal_NoActor_Unauthorized(t *testing.T) {
	handler := createHandler()

	router := setupTestRouter()
	// No middleware that injects actor - simulates unauthenticated request
	router.POST("/admin/payouts/withdrawals/:id/approve", handler.ApproveWithdrawal)

	withdrawalID := uuid.New()
	reqBody := ApproveWithdrawalRequest{Notes: "Should fail"}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/payouts/withdrawals/"+withdrawalID.String()+"/approve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// MUST return 401 Unauthorized - no actor in context
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Unauthenticated request must be unauthorized")

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp, "error")
}

func TestRejectWithdrawal_NoActor_Unauthorized(t *testing.T) {
	handler := createHandler()

	router := setupTestRouter()
	// No middleware that injects actor - simulates unauthenticated request
	router.POST("/admin/payouts/withdrawals/:id/reject", handler.RejectWithdrawal)

	withdrawalID := uuid.New()
	reqBody := RejectWithdrawalRequest{Reason: "Should fail"}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/payouts/withdrawals/"+withdrawalID.String()+"/reject", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// MUST return 401 Unauthorized - no actor in context
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Unauthenticated request must be unauthorized")
}

// ============================================================================
// TEST 4: DEFENSE-IN-DEPTH - Handler-level check works without middleware
// ============================================================================

func TestApproveWithdrawal_HandlerLevelDefense_WithoutCapability(t *testing.T) {
	handler := createHandler()

	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		// Inject user WITHOUT capability - bypassing any middleware checks
		// This tests that the handler-level defense works independently
		userID := uuid.New()
		actor := createTestActor(userID, "admin", []string{}) // No finance.withdraw.review
		injectActorIntoContext(c, actor)
		c.Next()
	})
	// No RequireCapability middleware - direct handler call
	router.POST("/admin/payouts/withdrawals/:id/approve", handler.ApproveWithdrawal)

	withdrawalID := uuid.New()
	reqBody := ApproveWithdrawalRequest{Notes: "Should fail at handler level"}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/payouts/withdrawals/"+withdrawalID.String()+"/approve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Handler-level defense MUST reject
	assert.Equal(t, http.StatusForbidden, w.Code, "Handler-level defense must reject without capability")

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp, "error")
	errorMsg, _ := resp["error"].(string)
	assert.Contains(t, errorMsg, "finance.withdraw.review", "Error message should mention required capability")
}

// ============================================================================
// TEST 5: NO ADMIN FALLBACK - Explicit admin role without capability fails
// ============================================================================

func TestApproveWithdrawal_AdminRoleNoCapability_NoFallback(t *testing.T) {
	handler := createHandler()

	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		// Explicitly create an admin with OTHER capabilities but NOT finance.withdraw.review
		// This verifies no implicit fallback from admin role
		adminID := uuid.New()
		actor := createTestActor(adminID, "admin", []string{
			"governance.audit.read",   // Has some capabilities
			"governance.user.suspend", // but NOT finance.withdraw.review
		})
		injectActorIntoContext(c, actor)
		c.Next()
	})
	router.POST("/admin/payouts/withdrawals/:id/approve", handler.ApproveWithdrawal)

	withdrawalID := uuid.New()
	reqBody := ApproveWithdrawalRequest{Notes: "Admin role should not grant implicit access"}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/payouts/withdrawals/"+withdrawalID.String()+"/approve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// MUST fail - admin role does NOT provide implicit access
	assert.Equal(t, http.StatusForbidden, w.Code, "Admin role without capability must be forbidden - no fallback")

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp, "error")
}

// ============================================================================
// TEST 6: Capability string validation
// ============================================================================

func TestApproveWithdrawal_WrongCapability_Forbidden(t *testing.T) {
	handler := createHandler()

	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		// Inject actor with WRONG capability (similar but not the same)
		adminID := uuid.New()
		actor := createTestActor(adminID, "admin", []string{
			"finance.withdraw.read", // Has READ but not REVIEW
		})
		injectActorIntoContext(c, actor)
		c.Next()
	})
	router.POST("/admin/payouts/withdrawals/:id/approve", handler.ApproveWithdrawal)

	withdrawalID := uuid.New()
	reqBody := ApproveWithdrawalRequest{Notes: "Read capability should not allow approve"}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/admin/payouts/withdrawals/"+withdrawalID.String()+"/approve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// MUST fail - read capability doesn't grant review access
	assert.Equal(t, http.StatusForbidden, w.Code, "Read capability must not allow approve action")
}

// ============================================================================
// TEST 7: Multiple actors can have the capability
// ============================================================================

func TestApproveWithdrawal_MultipleActorsWithCapability_Allowed(t *testing.T) {
	handler := createHandler()

	testCases := []struct {
		name         string
		role         string
		capabilities []string
	}{
		{
			name:         "Admin with finance capability",
			role:         "admin",
			capabilities: []string{"finance.withdraw.review"},
		},
		{
			name:         "Finance staff with capability",
			role:         "finance_staff",
			capabilities: []string{"finance.withdraw.review"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router := setupTestRouter()
			router.Use(func(c *gin.Context) {
				adminID := uuid.New()
				actor := createTestActor(adminID, tc.role, tc.capabilities)
				injectActorIntoContext(c, actor)
				c.Next()
			})
			router.POST("/admin/payouts/withdrawals/:id/approve", handler.ApproveWithdrawal)

			withdrawalID := uuid.New()

			reqBody := ApproveWithdrawalRequest{Notes: tc.name}
			body, _ := json.Marshal(reqBody)

			req, _ := http.NewRequest("POST", "/admin/payouts/withdrawals/"+withdrawalID.String()+"/approve", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should NOT return 403 Forbidden
			assert.NotEqual(t, http.StatusForbidden, w.Code,
				"%s: User with capability should not be forbidden", tc.name)
		})
	}
}


