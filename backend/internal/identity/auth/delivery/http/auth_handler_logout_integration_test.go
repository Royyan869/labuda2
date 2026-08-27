//go:build integration

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/identity/auth/application"
	authentity "github.com/labuda/backend/internal/identity/auth/entity"
	authrepo "github.com/labuda/backend/internal/identity/auth/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func setupLogoutHandlerTest(t *testing.T) (*testdb.TestDB, *AuthHandler, *observer.ObservedLogs, func()) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	logCore, observed := observer.New(zap.InfoLevel)
	log := zap.New(logCore)
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}

	handler := NewAuthHandler(tdb.Pool(), nil, cfg, log)
	return tdb, handler, observed, cleanup
}

func insertLogoutTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
	`, uid, uid.String(), uid.String()+"@test.invalid")
	if err != nil {
		t.Fatalf("insertLogoutTestUser: %v", err)
	}
	return uid
}

func mintLogoutTokenPair(t *testing.T, userID uuid.UUID) (*application.TokenPair, *application.TokenService) {
	t.Helper()
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	svc := application.NewTokenService(cfg, &logger.Logger{Logger: zap.NewNop()})
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("mintLogoutTokenPair: %v", err)
	}
	return pair, svc
}

func createLogoutSession(t *testing.T, ctx context.Context, repo *authrepo.RefreshSessionRepository, tx db.Tx, userID uuid.UUID, pair *application.TokenPair) *authentity.RefreshSession {
	t.Helper()
	session, err := authentity.NewRefreshSession(
		userID,
		pair.FamilyID,
		pair.RefreshJTI,
		authrepo.HashRefreshToken(pair.RefreshToken),
		pair.RefreshExpiresAt,
	)
	if err != nil {
		t.Fatalf("createLogoutSession: %v", err)
	}
	if err := repo.Create(ctx, tx, session); err != nil {
		t.Fatalf("createLogoutSession create: %v", err)
	}
	return session
}

func insertActiveFCMToken(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, token string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO fcm_tokens (
			id, user_id, token, platform, device_id, device_name, app_version,
			is_active, last_used_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, true, NOW(), NOW(), NOW())
	`, uuid.New(), userID, token, "android", token, "test-device", "1.0.0")
	if err != nil {
		t.Fatalf("insertActiveFCMToken: %v", err)
	}
}

func callLogoutHandler(t *testing.T, h *AuthHandler, userID uuid.UUID, body any) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("user_id", userID)

	h.Logout(c)
	return w
}

func assertLogoutSuccess(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if success, _ := resp["success"].(bool); !success {
		t.Fatalf("expected success response, got %v", resp)
	}
}

func TestLogoutHandler_RevokeOnlyCurrentFamilyAndKeepOthersActive(t *testing.T) {
	tdb, h, observed, cleanup := setupLogoutHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	userA := insertLogoutTestUser(t, ctx, tdb.Pool())
	userB := insertLogoutTestUser(t, ctx, tdb.Pool())

	var currentPair *application.TokenPair
	var otherFamilyPair *application.TokenPair
	var otherUserPair *application.TokenPair

	withTx := func(fn func(tx db.Tx)) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			fn(tx)
			return nil
		})
		if err != nil {
			t.Fatalf("tx: %v", err)
		}
	}

	withTx(func(tx db.Tx) {
		currentPair, _ = mintLogoutTokenPair(t, userA)
		otherFamilyPair, _ = mintLogoutTokenPair(t, userA)
		otherUserPair, _ = mintLogoutTokenPair(t, userB)

		createLogoutSession(t, ctx, h.refreshSessionRepo, tx, userA, currentPair)
		createLogoutSession(t, ctx, h.refreshSessionRepo, tx, userA, otherFamilyPair)
		createLogoutSession(t, ctx, h.refreshSessionRepo, tx, userB, otherUserPair)

		insertActiveFCMToken(t, ctx, tdb.Pool(), userA, "fcm-token-a")
	})

	// FCM deactivation is handled by the logout service (wired in NewAuthHandler).
	// Integration test: verify via DB state, not stub call counts.

	w := callLogoutHandler(t, h, userA, map[string]string{
		"refresh_token": currentPair.RefreshToken,
		"fcm_token":     "fcm-token-a",
		"device_id":     "fcm-token-a",
	})
	assertLogoutSuccess(t, w)

	var currentStatus, otherFamilyStatus, otherUserStatus string
	err := tdb.Pool().QueryRow(ctx, `SELECT status FROM auth_refresh_sessions WHERE jti = $1`, currentPair.RefreshJTI).Scan(&currentStatus)
	if err != nil {
		t.Fatalf("current status: %v", err)
	}
	err = tdb.Pool().QueryRow(ctx, `SELECT status FROM auth_refresh_sessions WHERE jti = $1`, otherFamilyPair.RefreshJTI).Scan(&otherFamilyStatus)
	if err != nil {
		t.Fatalf("other family status: %v", err)
	}
	err = tdb.Pool().QueryRow(ctx, `SELECT status FROM auth_refresh_sessions WHERE jti = $1`, otherUserPair.RefreshJTI).Scan(&otherUserStatus)
	if err != nil {
		t.Fatalf("other user status: %v", err)
	}

	if currentStatus != string(authentity.RefreshSessionStatusRevoked) {
		t.Fatalf("current family session must be revoked, got %q", currentStatus)
	}
	if otherFamilyStatus != string(authentity.RefreshSessionStatusActive) {
		t.Fatalf("same-user other family must remain active, got %q", otherFamilyStatus)
	}
	if otherUserStatus != string(authentity.RefreshSessionStatusActive) {
		t.Fatalf("other user session must remain active, got %q", otherUserStatus)
	}

	var tokenActive bool
	err = tdb.Pool().QueryRow(ctx, `SELECT is_active FROM fcm_tokens WHERE token = $1`, "fcm-token-a").Scan(&tokenActive)
	if err != nil {
		t.Fatalf("fcm token lookup: %v", err)
	}
	if tokenActive {
		t.Fatalf("expected FCM token to be deactivated")
	}

	for _, entry := range observed.All() {
		if strings.Contains(entry.Message, currentPair.RefreshToken) {
			t.Fatalf("raw refresh token leaked to logs: %s", entry.Message)
		}
	}
}

func TestLogoutHandler_RejectedWhenRefreshTokenBelongsToDifferentUser(t *testing.T) {
	tdb, h, _, cleanup := setupLogoutHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	userA := insertLogoutTestUser(t, ctx, tdb.Pool())
	userB := insertLogoutTestUser(t, ctx, tdb.Pool())

	withTx := func(fn func(tx db.Tx)) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			fn(tx)
			return nil
		})
		if err != nil {
			t.Fatalf("tx: %v", err)
		}
	}

	var pairA *application.TokenPair
	withTx(func(tx db.Tx) {
		pairA, _ = mintLogoutTokenPair(t, userA)
		createLogoutSession(t, ctx, h.refreshSessionRepo, tx, userA, pairA)
	})

	w := callLogoutHandler(t, h, userB, map[string]string{
		"refresh_token": pairA.RefreshToken,
	})

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != "TOKEN_USER_MISMATCH" {
		t.Fatalf("expected TOKEN_USER_MISMATCH, got %+v", resp.Error)
	}
}

func TestLogoutHandler_RejectedWhenAccessTokenUsedAsRefreshToken(t *testing.T) {
	_, h, _, cleanup := setupLogoutHandlerTest(t)
	defer cleanup()

	userID := uuid.New()
	pair, svc := mintLogoutTokenPair(t, userID)
	accessToken := pair.AccessToken
	_ = svc

	w := callLogoutHandler(t, h, userID, map[string]string{
		"refresh_token": accessToken,
	})

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestLogoutHandler_MissingRefreshTokenReturnsBadRequest(t *testing.T) {
	_, h, _, cleanup := setupLogoutHandlerTest(t)
	defer cleanup()

	w := callLogoutHandler(t, h, uuid.New(), map[string]string{
		"fcm_token": "fcm-token-a",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestLogoutHandler_IdempotentForAlreadyRevokedSession(t *testing.T) {
	tdb, h, _, cleanup := setupLogoutHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := insertLogoutTestUser(t, ctx, tdb.Pool())

	var pair *application.TokenPair
	withTx := func(fn func(tx db.Tx)) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			fn(tx)
			return nil
		})
		if err != nil {
			t.Fatalf("tx: %v", err)
		}
	}

	withTx(func(tx db.Tx) {
		pair, _ = mintLogoutTokenPair(t, userID)
		createLogoutSession(t, ctx, h.refreshSessionRepo, tx, userID, pair)
	})

	first := callLogoutHandler(t, h, userID, map[string]string{
		"refresh_token": pair.RefreshToken,
	})
	assertLogoutSuccess(t, first)

	second := callLogoutHandler(t, h, userID, map[string]string{
		"refresh_token": pair.RefreshToken,
	})
	assertLogoutSuccess(t, second)
}

func TestLogoutHandler_FCMFailureDoesNotCorruptSessionRevocation(t *testing.T) {
	tdb, h, _, cleanup := setupLogoutHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := insertLogoutTestUser(t, ctx, tdb.Pool())

	var pair *application.TokenPair
	withTx := func(fn func(tx db.Tx)) {
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			fn(tx)
			return nil
		})
		if err != nil {
			t.Fatalf("tx: %v", err)
		}
	}

	withTx(func(tx db.Tx) {
		pair, _ = mintLogoutTokenPair(t, userID)
		createLogoutSession(t, ctx, h.refreshSessionRepo, tx, userID, pair)
	})

	w := callLogoutHandler(t, h, userID, map[string]string{
		"refresh_token": pair.RefreshToken,
		"fcm_token":     "fcm-token-failure",
	})
	assertLogoutSuccess(t, w)

	var status string
	err := tdb.Pool().QueryRow(ctx, `SELECT status FROM auth_refresh_sessions WHERE jti = $1`, pair.RefreshJTI).Scan(&status)
	if err != nil {
		t.Fatalf("status query: %v", err)
	}
	if status != string(authentity.RefreshSessionStatusRevoked) {
		t.Fatalf("refresh session must stay revoked despite FCM failure, got %q", status)
	}
}
