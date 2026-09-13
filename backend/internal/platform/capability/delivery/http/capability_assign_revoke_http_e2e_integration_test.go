//go:build integration

package http

// REAL HTTP E2E PROOF — CAPABILITY ASSIGN / REVOKE
//
// This test drives the two capability endpoints through the canonical
// privileged-action pipeline exactly as routes_core.go mounts it:
//
//	POST   /api/v1/admin/users/:id/capabilities
//	DELETE /api/v1/admin/users/:id/capabilities/:cap
//
//	  user_id (LabudaAuth/UserLookup output)
//	  → ActorContextInject (real resolver: users.role + active capabilities from DB)
//	  → RequireAdminMiddleware (real RoleCheckerDB.IsAdmin)
//	  → RequireCapability("governance.capability.assign")
//	  → CapabilityHandler.AssignCapability / RevokeCapability
//	    → CapabilityService
//	      → CapabilityRepository.CreateGrant / RevokeGuarded (own transaction)
//	        → user_capabilities (real PostgreSQL)
//
// Authentication itself (LabudaAuthMiddleware JWT validation) is stubbed by
// setting the user_id that UserLookupMiddleware would set — the same technique
// the repository's existing middleware-pipeline tests use.
//
// Proven over real HTTP + real DB:
//
//	A. assign  → 200 and the grant is persisted active
//	B. revoke a partial admin → 200 and the grant is no longer active
//	C. revoke from the sole full-access admin → 409 CONFLICT, nothing changes
//	D. revoke from one of two full-access admins → 200, one full-access admin remains
//
// The invariant is asserted through the canonical authority
// (internal/platform/capability/invariant), never a second counter.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityApp "github.com/labuda/backend/internal/platform/capability/application"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	capabilityInfra "github.com/labuda/backend/internal/platform/capability/infrastructure"
	capabilityRepoImpl "github.com/labuda/backend/internal/platform/capability/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/capability/invariant"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// insertAuthorityUser inserts one active user with the given role and active
// capability grants. Grants go through the canonical write path (CreateGrant).
func insertAuthorityUser(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	appDB *db.DB,
	role string,
	caps []string,
) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', $4, NOW(), NOW())
	`, id, id.String(), id.String()+"@capability-http-e2e.test", role)
	require.NoError(t, err, "insert user")

	if len(caps) > 0 {
		repo := capabilityRepoImpl.NewCapabilityRepository(appDB)
		for _, c := range caps {
			require.NoError(t, repo.CreateGrant(ctx, capabilityEntity.NewCapabilityGrant(id, c, nil)),
				"grant %s", c)
		}
	}
	return id
}

// hasActiveCapability reports whether the grant is currently active in the DB.
func hasActiveCapability(t *testing.T, ctx context.Context, tdb *testdb.TestDB, userID uuid.UUID, capStr string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_capabilities
			WHERE user_id = $1 AND capability = $2 AND revoked_at IS NULL
		)
	`, userID, capStr).Scan(&exists))
	return exists
}

// countFullAccessAdmins reuses the canonical invariant authority to read the
// live count. This is an assertion, not a second implementation.
func countFullAccessAdmins(t *testing.T, ctx context.Context, tdb *testdb.TestDB) int {
	t.Helper()
	count, err := invariant.Count(ctx, tdb.Pool())
	require.NoError(t, err)
	return count
}

// buildCapabilityAdminRouter mounts the canonical admin pipeline for the two
// capability endpoints, authenticated as operatorID.
func buildCapabilityAdminRouter(t *testing.T, tdb *testdb.TestDB, appDB *db.DB, operatorID uuid.UUID) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	auditLogger := audit.NewAdminAuditLoggerDB(tdb.Pool())
	capRepo := capabilityRepoImpl.NewCapabilityRepository(appDB)
	svc := capabilityApp.NewCapabilityService(capRepo, auditLogger)
	h := NewCapabilityHandler(svc)

	roleChecker := auth.NewRoleCheckerDB(appDB, auditLogger)
	stateQuerier := capability.NewUserStateQuerierAdapter(appDB)
	resolver := capabilityInfra.NewActorResolver(capRepo, stateQuerier)

	r := gin.New()
	// LabudaAuthMiddleware + UserLookupMiddleware produce user_id.
	r.Use(func(c *gin.Context) { c.Set("user_id", operatorID); c.Next() })
	r.Use(middleware.ActorContextInject(resolver, middleware.ActorContextInjectOptions{}))

	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.RequireAdminMiddleware(roleChecker))
	admin.POST("/users/:id/capabilities",
		middleware.RequireCapability(capability.CapGovernanceCapabilityAssign.String()),
		h.AssignCapability)
	admin.DELETE("/users/:id/capabilities/:cap",
		middleware.RequireCapability(capability.CapGovernanceCapabilityAssign.String()),
		h.RevokeCapability)
	return r
}

// doJSON performs a real HTTP request against the mounted router.
func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCapabilityAssignRevoke_HTTPEndToEnd(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	universe := capability.AllCapabilityStrings()
	assignCap := capability.CapGovernanceCapabilityAssign.String()
	revokedCap := capability.CapFinanceWithdrawRead.String()
	require.Contains(t, universe, revokedCap, "revocation target must be a canonical capability")
	require.Contains(t, universe, assignCap)

	// Scenario isolation without TRUNCATE (fresh-schema reset per scenario is
	// prohibitively slow on this container). We control the invariant count by
	// making previously created full-access admins non-full-access with a fast
	// row update, so each scenario starts from a known count.
	var fullAccessSoFar []uuid.UUID
	demoteFullAccess := func() {
		if len(fullAccessSoFar) == 0 {
			return
		}
		_, err := tdb.Pool().Exec(ctx, `
			UPDATE user_capabilities SET revoked_at = NOW()
			WHERE user_id = ANY($1::uuid[]) AND capability = $2 AND revoked_at IS NULL
		`, fullAccessSoFar, revokedCap)
		require.NoError(t, err)
		fullAccessSoFar = nil
	}

	// ── A. ASSIGN SUCCESS ────────────────────────────────────────────────
	t.Run("A_assign_success", func(t *testing.T) {
		demoteFullAccess()

		operator := insertAuthorityUser(t, ctx, tdb, appDB, capabilityEntity.AdminRole, []string{assignCap})
		target := insertAuthorityUser(t, ctx, tdb, appDB, "user", nil)

		router := buildCapabilityAdminRouter(t, tdb, appDB, operator)
		w := doJSON(t, router, http.MethodPost,
			"/api/v1/admin/users/"+target.String()+"/capabilities",
			gin.H{"capability": revokedCap})

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		require.True(t, hasActiveCapability(t, ctx, tdb, target, revokedCap),
			"grant must be persisted as active")
	})

	// ── B. REVOKE SUCCESS (target is not the last full-access admin) ──────
	t.Run("B_revoke_success", func(t *testing.T) {
		demoteFullAccess()

		// Backstop full-access admin keeps the invariant satisfied; it also acts
		// as the revoking operator.
		operator := insertAuthorityUser(t, ctx, tdb, appDB, capabilityEntity.AdminRole, universe)
		fullAccessSoFar = append(fullAccessSoFar, operator)
		// Partial admin holds the target capability but is not full-access.
		target := insertAuthorityUser(t, ctx, tdb, appDB, capabilityEntity.AdminRole, []string{assignCap, revokedCap})

		router := buildCapabilityAdminRouter(t, tdb, appDB, operator)
		w := doJSON(t, router, http.MethodDelete,
			"/api/v1/admin/users/"+target.String()+"/capabilities/"+revokedCap, nil)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		require.False(t, hasActiveCapability(t, ctx, tdb, target, revokedCap),
			"grant must no longer be active")
		require.Equal(t, 1, countFullAccessAdmins(t, ctx, tdb),
			"backstop full-access admin must still satisfy the invariant")
	})

	// ── C. LAST FULL-ACCESS ADMIN REJECTED (409) ─────────────────────────
	t.Run("C_last_full_access_admin_rejected", func(t *testing.T) {
		demoteFullAccess()

		// The sole full-access admin.
		sole := insertAuthorityUser(t, ctx, tdb, appDB, capabilityEntity.AdminRole, universe)
		fullAccessSoFar = append(fullAccessSoFar, sole)
		// A different admin with only the management capability, so the
		// self-revocation guard cannot be what rejects the request.
		operator := insertAuthorityUser(t, ctx, tdb, appDB, capabilityEntity.AdminRole, []string{assignCap})
		require.Equal(t, 1, countFullAccessAdmins(t, ctx, tdb))

		router := buildCapabilityAdminRouter(t, tdb, appDB, operator)
		w := doJSON(t, router, http.MethodDelete,
			"/api/v1/admin/users/"+sole.String()+"/capabilities/"+revokedCap, nil)

		require.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
		require.Contains(t, w.Body.String(), "full-access",
			"canonical invariant failure must reach the HTTP contract")

		// Rollback proof: the capability is still active and the invariant holds.
		require.True(t, hasActiveCapability(t, ctx, tdb, sole, revokedCap),
			"rejected revocation must not change database state")
		require.Equal(t, 1, countFullAccessAdmins(t, ctx, tdb),
			"invariant must remain satisfied after a rejected revocation")
	})

	// ── D. REVOKE ALLOWED WITH ANOTHER FULL-ACCESS ADMIN ─────────────────
	t.Run("D_revoke_allowed_with_another_full_access_admin", func(t *testing.T) {
		demoteFullAccess()

		// Two independent full-access admins.
		operator := insertAuthorityUser(t, ctx, tdb, appDB, capabilityEntity.AdminRole, universe)
		fullAccessSoFar = append(fullAccessSoFar, operator)
		target := insertAuthorityUser(t, ctx, tdb, appDB, capabilityEntity.AdminRole, universe)
		fullAccessSoFar = append(fullAccessSoFar, target)
		require.Equal(t, 2, countFullAccessAdmins(t, ctx, tdb))

		router := buildCapabilityAdminRouter(t, tdb, appDB, operator)
		w := doJSON(t, router, http.MethodDelete,
			"/api/v1/admin/users/"+target.String()+"/capabilities/"+revokedCap, nil)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		require.False(t, hasActiveCapability(t, ctx, tdb, target, revokedCap),
			"grant must no longer be active")
		require.True(t, hasActiveCapability(t, ctx, tdb, operator, revokedCap),
			"the other full-access admin is untouched")
		require.Equal(t, 1, countFullAccessAdmins(t, ctx, tdb),
			"exactly one full-access admin must remain and the invariant must still hold")
	})
}
