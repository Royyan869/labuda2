//go:build integration

package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/config"
	identityusername "github.com/labuda/backend/internal/identity/username"
	userApp "github.com/labuda/backend/internal/identity/user/application"
	userrepoimpl "github.com/labuda/backend/internal/identity/user/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// C1B3 — BACKEND RESERVED-NAME RUNTIME PROOF
//
// Proves the production CheckUsername handler path:
//
//   Normalize → ValidateFormat → IsReserved → response (no DB call)
//
// These tests must pass against a live Postgres test database
// (-tags=integration). A real DB is required because the handler
// constructor needs *db.DB, but for reserved names the handler
// returns before touching the database.

func setupReservedUsernameTest(t *testing.T) (*UserHandler, func()) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)

	cfg, err := config.Load()
	require.NoError(t, err)

	appDB, dbErr := db.New(t.Context(), db.Config{
		ConnString: cfg.Database.GetTestDSN(),
	})
	require.NoError(t, dbErr)
	t.Cleanup(func() { appDB.Close() })

	userRepo := userrepoimpl.NewUserRepository(appDB)
	profileService := userApp.NewUserProfileService(userRepo, nil, nil, nil, nil, appDB)
	handler := NewUserHandler(profileService, appDB, zap.NewNop())

	_ = tdb // used only for DB lifecycle; handler owns its own DB

	return handler, cleanup
}

// Helper: perform a CheckUsername request and decode the response.
func doCheckUsername(t *testing.T, handler *UserHandler, username string, userID uuid.UUID) checkUsernameResponse {
	t.Helper()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqURL := "/api/v1/users/check-username?username=" + url.QueryEscape(username)
	req := httptest.NewRequest(http.MethodGet, reqURL, nil)
	c.Request = req

	// Set the authenticated user ID so currentUserID succeeds.
	c.Set("userID", userID)

	handler.CheckUsername(c)

	require.Equal(t, http.StatusOK, w.Code)

	var envelope struct {
		Success bool                  `json:"success"`
		Data    checkUsernameResponse `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &envelope)
	require.NoError(t, err)
	require.True(t, envelope.Success)

	return envelope.Data
}

// =============================================================================
// TESTS
// =============================================================================

func TestCheckUsername_ReservedUsername_ReturnsUnavailable(t *testing.T) {
	handler, cleanup := setupReservedUsernameTest(t)
	defer cleanup()

	// "labuda" is in the production reserved list (validation.go).
	resp := doCheckUsername(t, handler, "labuda", uuid.New())

	require.False(t, resp.Available)
	require.Equal(t, "USERNAME_RESERVED", resp.Reason)
}

func TestCheckUsername_ReservedUsername_MixedCaseNormalized(t *testing.T) {
	handler, cleanup := setupReservedUsernameTest(t)
	defer cleanup()

	// "  LaBuDa  " must normalize to "labuda" and be rejected.
	resp := doCheckUsername(t, handler, "  LaBuDa  ", uuid.New())

	require.False(t, resp.Available)
	require.Equal(t, "USERNAME_RESERVED", resp.Reason)
}

func TestCheckUsername_ValidNonReserved_ProceedsToUniquenessCheck(t *testing.T) {
	handler, cleanup := setupReservedUsernameTest(t)
	defer cleanup()

	// "moderator" is not in the backend reserved list.
	// It must pass format + reserved checks and reach the DB.
	resp := doCheckUsername(t, handler, "moderator", uuid.New())

	// The DB has no user with this username → available.
	require.True(t, resp.Available)
	require.Empty(t, resp.Reason)
}

func TestCheckUsername_InvalidFormat_ReturnsInvalidFormat(t *testing.T) {
	handler, cleanup := setupReservedUsernameTest(t)
	defer cleanup()

	// "john-doe" contains hyphens → invalid format.
	resp := doCheckUsername(t, handler, "john-doe", uuid.New())

	require.False(t, resp.Available)
	require.Equal(t, "USERNAME_INVALID_FORMAT", resp.Reason)
}

// =============================================================================
// SANITY: prove IsReserved is called on the production package
// =============================================================================

func TestIsReserved_Labuda_IsReserved(t *testing.T) {
	require.True(t, identityusername.IsReserved("labuda"))
}

func TestIsReserved_Moderator_IsNotReserved(t *testing.T) {
	require.False(t, identityusername.IsReserved("moderator"))
}

func TestIsReserved_MixedCase_IsReserved(t *testing.T) {
	require.True(t, identityusername.IsReserved("  LaBuDa  "))
}
