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
	"github.com/jackc/pgx/v5/pgxpool"
	sellerrepository "github.com/labuda/backend/internal/commerce/seller/infrastructure/repository"
	subscriptionrepository "github.com/labuda/backend/internal/commerce/subscription/infrastructure/repository"
	"github.com/labuda/backend/internal/config"
	userApp "github.com/labuda/backend/internal/identity/user/application"
	userrepoimpl "github.com/labuda/backend/internal/identity/user/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// USERNAME IMMUTABILITY GUARDRAIL — PASS_10B REGRESSION LOCK
//
// Username can only be established once (at signup or first profile
// completion). PATCH /api/v1/users/me/profile must reject any attempt to
// change an already-set username with 409 USERNAME_ALREADY_SET. This is the
// entire reason PASS_10A found no active P0/P1: if this guard regresses,
// the dormant traceability gap (no username_history) becomes live.
//
// This is a real-DB integration test (requires -tags integration and a live
// Postgres test database per this repo's pkg/testdb convention).
// ============================================================================

func setupUsernameImmutabilityTest(t *testing.T) (*testdb.TestDB, *UserHandler, func()) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)

	cfg, err := config.Load()
	require.NoError(t, err)

	appDB, err := db.New(context.Background(), db.Config{
		ConnString: cfg.Database.GetTestDSN(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { appDB.Close() })

	userRepo := userrepoimpl.NewUserRepository(appDB)
	sellerRepo := sellerrepository.NewSellerRepository()
	subscriptionRepo := subscriptionrepository.NewSellerSubscriptionRepository()

	// outboxRepo and firebaseClient are not reached by GetMyProfile/UpdateMyProfile
	// (firebaseClient is explicitly nil-checked; outboxRepo is unused in this path).
	profileService := userApp.NewUserProfileService(userRepo, sellerRepo, subscriptionRepo, nil, nil, appDB)

	handler := NewUserHandler(profileService, appDB, zap.NewNop())
	return tdb, handler, cleanup
}

func insertImmutabilityTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, username *string) uuid.UUID {
	t.Helper()
	uid := uuid.New()

	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
	`, uid, uid.String(), uid.String()+"@test.invalid")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO user_profiles (user_id, username, followers_count, following_count, created_at, updated_at)
		VALUES ($1, $2, 0, 0, NOW(), NOW())
	`, uid, username)
	require.NoError(t, err)

	return uid
}

func callUpdateMyProfile(t *testing.T, h *UserHandler, userID uuid.UUID, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPatch, "/api/v1/users/me/profile", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("userID", userID)

	h.UpdateMyProfile(c)
	return w
}

func queryPersistedUsername(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) string {
	t.Helper()
	var username *string
	err := pool.QueryRow(ctx, `SELECT username FROM user_profiles WHERE user_id = $1`, userID).Scan(&username)
	require.NoError(t, err)
	if username == nil {
		return ""
	}
	return *username
}

// 1. First-time username establishment works.
func TestUpdateMyProfile_FirstTimeUsernameEstablishment_Succeeds(t *testing.T) {
	tdb, h, cleanup := setupUsernameImmutabilityTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := insertImmutabilityTestUser(t, ctx, tdb.Pool(), nil)

	w := callUpdateMyProfile(t, h, userID, map[string]interface{}{
		"username": "  NewHandle_1  ",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].(map[string]interface{})
	profile, _ := data["profile"].(map[string]interface{})
	if profile["username"] != "newhandle_1" {
		t.Fatalf("expected normalized username 'newhandle_1' in response, got %+v", profile["username"])
	}

	saved := queryPersistedUsername(t, ctx, tdb.Pool(), userID)
	if saved != "newhandle_1" {
		t.Fatalf("expected normalized username persisted, got %q", saved)
	}
}

// 2. Rename is rejected.
func TestUpdateMyProfile_RenameAttempt_RejectedWithUsernameAlreadySet(t *testing.T) {
	tdb, h, cleanup := setupUsernameImmutabilityTest(t)
	defer cleanup()

	ctx := context.Background()
	existing := "handle_a"
	userID := insertImmutabilityTestUser(t, ctx, tdb.Pool(), &existing)

	w := callUpdateMyProfile(t, h, userID, map[string]interface{}{
		"username": "handle_b",
	})

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	errObj, _ := resp["error"].(map[string]interface{})
	if errObj["code"] != "USERNAME_ALREADY_SET" {
		t.Fatalf("expected error code USERNAME_ALREADY_SET, got %+v", resp)
	}

	saved := queryPersistedUsername(t, ctx, tdb.Pool(), userID)
	if saved != existing {
		t.Fatalf("username must remain %q after a rejected rename attempt, got %q", existing, saved)
	}
}

// 3. Same-username resubmission is not treated as a rename; other fields still update.
func TestUpdateMyProfile_SameUsernameResubmitted_NotTreatedAsRename(t *testing.T) {
	tdb, h, cleanup := setupUsernameImmutabilityTest(t)
	defer cleanup()

	ctx := context.Background()
	existing := "handle_c"
	userID := insertImmutabilityTestUser(t, ctx, tdb.Pool(), &existing)

	w := callUpdateMyProfile(t, h, userID, map[string]interface{}{
		"username": "HANDLE_C", // normalizes to the same value as `existing`
		"bio":      "updated bio",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (same username is a no-op, not a rename), got %d; body=%s", w.Code, w.Body.String())
	}

	saved := queryPersistedUsername(t, ctx, tdb.Pool(), userID)
	if saved != existing {
		t.Fatalf("username must remain %q, got %q", existing, saved)
	}

	var bio *string
	err := tdb.Pool().QueryRow(ctx, `SELECT bio FROM user_profiles WHERE user_id = $1`, userID).Scan(&bio)
	require.NoError(t, err)
	if bio == nil || *bio != "updated bio" {
		t.Fatalf("expected bio to update alongside the no-op username resubmission, got %v", bio)
	}
}

// 4. CheckUsername remains a pure availability check — it must never mutate state.
func TestCheckUsername_DoesNotMutateAnyState(t *testing.T) {
	tdb, h, cleanup := setupUsernameImmutabilityTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := insertImmutabilityTestUser(t, ctx, tdb.Pool(), nil)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodGet, "/api/v1/users/check-username?username=freshhandle", nil)
	require.NoError(t, err)
	c.Request = req
	c.Set("userID", userID)

	h.CheckUsername(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	saved := queryPersistedUsername(t, ctx, tdb.Pool(), userID)
	if saved != "" {
		t.Fatalf("CheckUsername must never mutate username, got %q", saved)
	}
}
