package http

// admin_me_identity_discovery_test.go — CASE 5 of the admin authority test
// matrix (ADMIN-AUTHORITY scope).
//
// GET /api/v1/admin/me is ADMIN IDENTITY / SESSION DISCOVERY, not a
// privileged internal business action. Its canonical purpose is to answer:
// "Who is the current admin member, and which capabilities does the backend
// currently recognize for this actor?" The admin web client calls it once
// on startup, immediately after the admin-membership boundary has been
// verified, to render capability-aware UI.
//
// Canonical semantics proven here:
//   - the handler performs NO capability check of its own (adding one would
//     make the discovery endpoint unable to report a missing capability);
//   - it grants nothing: it only reads the already-resolved Actor from the
//     request context and reports role, is_admin, and the capability list;
//   - unauthenticated requests (no actor) are rejected with 401.
//
// The coarse admin-membership boundary itself is enforced upstream by
// RequireAdminMiddleware on the admin route group (see
// internal/middleware/admin_authority_matrix_test.go for the full
// membership × capability matrix).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/middleware"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAdminMeTestContext(t *testing.T, actor *capabilityEntity.Actor) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/me", nil)
	if actor != nil {
		req = req.WithContext(middleware.WithActor(req.Context(), actor))
	}
	c.Request = req
	return c, w
}

type adminMeBody struct {
	Success bool `json:"success"`
	Data    struct {
		ID           string   `json:"id"`
		Email        string   `json:"email"`
		Username     string   `json:"username"`
		Role         string   `json:"role"`
		IsAdmin      bool     `json:"is_admin"`
		Capabilities []string `json:"capabilities"`
	} `json:"data"`
}

func TestAdminMe_IdentityDiscovery_AdminActor_ReportsRoleAndCapabilities(t *testing.T) {
	adminID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{"governance.capability.assign", "governance.dashboard.view"},
	}

	h := NewAdminHandler(nil, nil, nil, nil)
	c, w := newAdminMeTestContext(t, actor)

	h.GetAdminMe(c)
	require.Equal(t, http.StatusOK, w.Code)

	var body adminMeBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, adminID.String(), body.Data.ID)
	assert.Equal(t, "admin", body.Data.Role)
	assert.True(t, body.Data.IsAdmin)
	assert.ElementsMatch(t, []string{"governance.capability.assign", "governance.dashboard.view"}, body.Data.Capabilities)
}

// TestAdminMe_IdentityDiscovery_NoCapabilityRequirementByDesign pins the
// discovery semantics: the handler itself must not enforce any capability.
// A canonical capability-free actor still receives its own identity report
// (with is_admin=false and an empty capability list) because discovery must
// be able to REPORT the absence of capabilities, not hide behind it.
// Authorization of business actions stays with the route-level gates.
func TestAdminMe_IdentityDiscovery_NoCapabilityRequirementByDesign(t *testing.T) {
	actor := &capabilityEntity.Actor{
		ID:           uuid.New(),
		Role:         "user",
		Capabilities: []string{},
	}

	h := NewAdminHandler(nil, nil, nil, nil)
	c, w := newAdminMeTestContext(t, actor)

	h.GetAdminMe(c)
	require.Equal(t, http.StatusOK, w.Code)

	var body adminMeBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "user", body.Data.Role)
	assert.False(t, body.Data.IsAdmin)
	assert.Empty(t, body.Data.Capabilities)
}

// TestAdminMe_IdentityDiscovery_Unauthenticated_Rejected pins the 401 edge:
// with no actor in context there is no identity to discover.
func TestAdminMe_IdentityDiscovery_Unauthenticated_Rejected(t *testing.T) {
	h := NewAdminHandler(nil, nil, nil, nil)
	c, w := newAdminMeTestContext(t, nil)

	h.GetAdminMe(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
