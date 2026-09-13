//go:build integration

package http

// DERIVED FULL-ACCESS IN THE ADMIN USER LIST — REAL DB PROOF
//
// GET /api/v1/admin/users must expose, per user, the derived authority state
//
//	full_access = role == "admin" AND active coverage of capability.AllCapabilities()
//
// Full access is never stored. This test drives the real handler → real
// AdminService → real repository against real PostgreSQL so it proves the
// whole chain: the page's active capabilities are batch-loaded (no per-user
// query) and the canonical authority (capability.IsFullAccessAdmin) derives the
// field that reaches the wire.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	adminApp "github.com/labuda/backend/internal/platform/admin/application"
	adminInfra "github.com/labuda/backend/internal/platform/admin/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	capabilityRepoImpl "github.com/labuda/backend/internal/platform/capability/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

type adminUsersListBody struct {
	Data struct {
		Users []struct {
			ID         uuid.UUID `json:"id"`
			Role       string    `json:"role"`
			FullAccess bool      `json:"full_access"`
		} `json:"users"`
	} `json:"data"`
}

func insertListUser(
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
	`, id, id.String(), id.String()+"@list-full-access.test", role)
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

func TestAdminListUsers_ExposesDerivedFullAccess(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	all := capability.AllCapabilityStrings()
	partial := all[:len(all)-1] // every canonical capability except one

	ordinary := insertListUser(t, ctx, tdb, appDB, "user", nil)
	ordinaryAllCaps := insertListUser(t, ctx, tdb, appDB, "user", all)
	partialAdmin := insertListUser(t, ctx, tdb, appDB, "admin", partial)
	fullAdmin := insertListUser(t, ctx, tdb, appDB, "admin", all)

	svc := adminApp.NewAdminService(appDB, adminInfra.NewAdminRepository(), &mockAuditLogger{}, nil)
	h := NewAdminHandler(svc, &mockAuditLogger{}, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=1&page_size=50", nil)
	setAdminContext(c, uuid.New())

	h.ListUsers(c)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "full_access", "list response must expose the field")

	var body adminUsersListBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	got := make(map[uuid.UUID]bool, len(body.Data.Users))
	for _, u := range body.Data.Users {
		got[u.ID] = u.FullAccess
	}

	require.Contains(t, got, ordinary)
	require.False(t, got[ordinary], "ordinary user must not have full access")
	require.False(t, got[ordinaryAllCaps], "non-admin with full capability coverage must not have full access")
	require.False(t, got[partialAdmin], "admin missing one capability must not have full access")
	require.True(t, got[fullAdmin], "admin with the whole canonical universe must have full access")
}
