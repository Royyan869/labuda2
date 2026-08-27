//go:build integration

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupFCMTokenHandlerIntegrationTest(t *testing.T) (*testdb.TestDB, *FCMTokenHandler, func()) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	handler := NewFCMTokenHandler(db.NewFromPool(tdb.Pool()), zap.NewNop())
	return tdb, handler, cleanup
}

func insertFCMTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, account_status, role, created_at, updated_at
		)
		VALUES ($1, $2, $3, 'active', 'user', NOW(), NOW())
	`, userID, userID.String(), userID.String()+"@test.invalid")
	require.NoError(t, err)
	return userID
}

func callRegisterFCMToken(t *testing.T, h *FCMTokenHandler, userID uuid.UUID, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	w, err := performRegisterFCMToken(h, userID, body)
	require.NoError(t, err)
	return w
}

func performRegisterFCMToken(h *FCMTokenHandler, userID uuid.UUID, body map[string]any) (*httptest.ResponseRecorder, error) {

	gin.SetMode(gin.TestMode)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/notifications/fcm-token", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("userID", userID)

	h.RegisterToken(c)
	return w, nil
}

func callUnregisterFCMToken(t *testing.T, h *FCMTokenHandler, userID uuid.UUID, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	w, err := performUnregisterFCMToken(h, userID, body)
	require.NoError(t, err)
	return w
}

func performUnregisterFCMToken(h *FCMTokenHandler, userID uuid.UUID, body map[string]any) (*httptest.ResponseRecorder, error) {

	gin.SetMode(gin.TestMode)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodDelete, "/api/v1/notifications/fcm-token", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("userID", userID)

	h.UnregisterToken(c)
	return w, nil
}

func assertHTTPSuccess(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, true, payload["success"])
}

func insertFCMTokenOwnerFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	return insertFCMTestUser(t, ctx, pool), insertFCMTestUser(t, ctx, pool)
}

func TestFCMTokenHandler_RegisterToken_CanonicalUpsertAndOwnershipTransfer(t *testing.T) {
	tdb, handler, cleanup := setupFCMTokenHandlerIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	userA, userB := insertFCMTokenOwnerFixture(t, ctx, tdb.Pool())
	token := "fcm-token-canonical-ownership"

	baseBody := map[string]any{
		"token":    token,
		"platform": "android",
	}

	assertHTTPSuccess(t, callRegisterFCMToken(t, handler, userA, baseBody))
	assertHTTPSuccess(t, callRegisterFCMToken(t, handler, userA, baseBody))

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w, err := performRegisterFCMToken(handler, userA, baseBody)
			if err != nil {
				errCh <- err
				return
			}
			if w.Code != http.StatusOK {
				errCh <- fmt.Errorf("concurrent register returned %d: %s", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	assertHTTPSuccess(t, callRegisterFCMToken(t, handler, userB, baseBody))

	var rowCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM fcm_tokens
		WHERE token = $1
	`, token).Scan(&rowCount))
	require.Equal(t, 1, rowCount)

	var ownerID uuid.UUID
	var storedDeviceID string
	var isActive bool
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT user_id, device_id, is_active
		FROM fcm_tokens
		WHERE token = $1
	`, token).Scan(&ownerID, &storedDeviceID, &isActive))
	require.Equal(t, userB, ownerID)
	require.Equal(t, token, storedDeviceID)
	require.True(t, isActive)
}

func TestFCMTokenHandler_UnregisterToken_IsUserScopedAfterTransfer(t *testing.T) {
	tdb, handler, cleanup := setupFCMTokenHandlerIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	userA, userB := insertFCMTokenOwnerFixture(t, ctx, tdb.Pool())
	token := "fcm-token-user-scope"

	body := map[string]any{
		"token":    token,
		"platform": "android",
	}

	assertHTTPSuccess(t, callRegisterFCMToken(t, handler, userA, body))
	assertHTTPSuccess(t, callRegisterFCMToken(t, handler, userB, body))

	assertHTTPSuccess(t, callUnregisterFCMToken(t, handler, userA, map[string]any{"token": token}))

	var activeAfterOldLogout bool
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT is_active
		FROM fcm_tokens
		WHERE token = $1
	`, token).Scan(&activeAfterOldLogout))
	require.True(t, activeAfterOldLogout, "old user's logout must not deactivate transferred token")

	assertHTTPSuccess(t, callUnregisterFCMToken(t, handler, userB, map[string]any{"token": token}))

	var activeAfterNewLogout bool
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT is_active
		FROM fcm_tokens
		WHERE token = $1
	`, token).Scan(&activeAfterNewLogout))
	require.False(t, activeAfterNewLogout, "current owner's logout should deactivate the token")
}

func TestFCMTokenHandler_TokenRotationKeepsNewTokenActive(t *testing.T) {
	tdb, handler, cleanup := setupFCMTokenHandlerIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := insertFCMTestUser(t, ctx, tdb.Pool())
	oldToken := "fcm-token-rotation-old"
	newToken := "fcm-token-rotation-new"

	assertHTTPSuccess(t, callRegisterFCMToken(t, handler, userID, map[string]any{
		"token":    oldToken,
		"platform": "android",
	}))
	assertHTTPSuccess(t, callRegisterFCMToken(t, handler, userID, map[string]any{
		"token":    newToken,
		"platform": "android",
	}))

	assertHTTPSuccess(t, callUnregisterFCMToken(t, handler, userID, map[string]any{"token": oldToken}))

	var oldActive, newActive bool
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT is_active
		FROM fcm_tokens
		WHERE token = $1
	`, oldToken).Scan(&oldActive))
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT is_active
		FROM fcm_tokens
		WHERE token = $1
	`, newToken).Scan(&newActive))
	require.False(t, oldActive, "rotated old token should be deactivated")
	require.True(t, newActive, "new token should remain active")
}

func TestFCMTokenSchema_DeviceIDIsNotNull(t *testing.T) {
	tdb, handler, cleanup := setupFCMTokenHandlerIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := insertFCMTestUser(t, ctx, tdb.Pool())
	token := "fcm-token-device-id-not-null"

	assertHTTPSuccess(t, callRegisterFCMToken(t, handler, userID, map[string]any{
		"token":    token,
		"platform": "android",
	}))

	var isNullable string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'fcm_tokens'
		  AND column_name = 'device_id'
	`).Scan(&isNullable))
	require.Equal(t, "NO", isNullable)

	var storedDeviceID string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT device_id
		FROM fcm_tokens
		WHERE token = $1
	`, token).Scan(&storedDeviceID))
	require.Equal(t, token, storedDeviceID)
}
