package middleware

// admin_authority_matrix_test.go — behavioral proof of the canonical
// effective predicate for privileged internal business actions:
//
//	authenticated AND admin membership AND explicit required capability
//
// The production chain is composed exactly as routes_core.go mounts it for
// the /api/v1/admin group: UserLookup sets user_id → ActorContextInject
// resolves the Actor (role + capabilities) → RequireAdminMiddleware
// enforces the coarse internal membership boundary (admin group) →
// RequireCapability enforces the explicit required capability.
//
// Required test matrix (ADMIN-AUTHORITY scope):
//
//	CASE 1  normal user  + no capability          → DENIED (401/403)
//	CASE 2  normal user  + required capability    → DENIED (403, admin boundary)
//	CASE 3  admin member + no required capability → DENIED (403, capability gate)
//	CASE 4  admin member + required capability    → ALLOWED (200)
//
// CASE 2 is the invariant most systems get wrong: a capability alone must
// NOT bypass the admin/internal membership boundary.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminMatrixFixture composes the canonical privileged-action pipeline.
type adminMatrixFixture struct {
	router        *gin.Engine
	handlerCalled bool
	recorder      *httptest.ResponseRecorder
}

func newAdminMatrixFixture(t *testing.T, actor *capabilityEntity.Actor) *adminMatrixFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	f := &adminMatrixFixture{
		router:   gin.New(),
		recorder: httptest.NewRecorder(),
	}

	userID := actor.ID
	resolver := &mockActorResolver{actor: actor}

	// Chain mirrored from routes_core.go (v1 group + admin group).
	f.router.Use(func(c *gin.Context) { c.Set("user_id", userID); c.Next() })
	f.router.Use(ActorContextInject(resolver, ActorContextInjectOptions{}))
	f.router.Use(RequireAdminMiddleware(&mockRoleChecker{isAdmin: actor.Role == "admin"}))
	f.router.Use(RequireCapability("finance.withdraw.review"))

	f.router.POST("/privileged-action", func(c *gin.Context) {
		f.handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return f
}

func (f *adminMatrixFixture) fire(t *testing.T) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/privileged-action", nil)
	f.router.ServeHTTP(f.recorder, req)
}

func TestAdminAuthorityMatrix_Case1_NormalUser_NoCapability_Denied(t *testing.T) {
	f := newAdminMatrixFixture(t, &capabilityEntity.Actor{
		ID:           uuid.New(),
		Role:         "user",
		Capabilities: []string{},
	})
	f.fire(t)

	assert.False(t, f.handlerCalled, "privileged action must not execute")
	assert.Equal(t, http.StatusForbidden, f.recorder.Code)
}

func TestAdminAuthorityMatrix_Case2_NormalUser_WithCapability_Denied(t *testing.T) {
	// Invariant 2: capability does NOT allow a normal user to bypass the
	// admin/internal membership boundary.
	f := newAdminMatrixFixture(t, &capabilityEntity.Actor{
		ID:           uuid.New(),
		Role:         "user",
		Capabilities: []string{"finance.withdraw.review"},
	})
	f.fire(t)

	assert.False(t, f.handlerCalled, "capability alone must not bypass the admin membership boundary")
	assert.Equal(t, http.StatusForbidden, f.recorder.Code)
}

func TestAdminAuthorityMatrix_Case3_Admin_NoCapability_Denied(t *testing.T) {
	// Invariant 3: admin membership alone is NOT implicit superuser authority.
	f := newAdminMatrixFixture(t, &capabilityEntity.Actor{
		ID:           uuid.New(),
		Role:         "admin",
		Capabilities: []string{},
	})
	f.fire(t)

	assert.False(t, f.handlerCalled, "admin membership alone must not authorize a privileged business action")
	assert.Equal(t, http.StatusForbidden, f.recorder.Code)
}

func TestAdminAuthorityMatrix_Case4_Admin_WithCapability_Allowed(t *testing.T) {
	f := newAdminMatrixFixture(t, &capabilityEntity.Actor{
		ID:           uuid.New(),
		Role:         "admin",
		Capabilities: []string{"finance.withdraw.review"},
	})
	f.fire(t)

	require.True(t, f.handlerCalled, "admin membership + explicit required capability must be allowed")
	assert.Equal(t, http.StatusOK, f.recorder.Code)
}

// TestAdminAuthorityMatrix_AuthenticationRequired pins the 401 edge: no
// actor in context (unauthenticated) can never pass the capability gate.
func TestAdminAuthorityMatrix_AuthenticationRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	router := gin.New()

	// No user_id set → ActorContextInject injects no actor.
	router.Use(ActorContextInject(&mockActorResolver{}, ActorContextInjectOptions{}))
	router.Use(RequireCapability("finance.withdraw.review"))
	router.POST("/privileged-action", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/privileged-action", nil)
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
