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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/identity/auth/application"
	authentity "github.com/labuda/backend/internal/identity/auth/entity"
	authrepo "github.com/labuda/backend/internal/identity/auth/infrastructure/repository"
	notificationentity "github.com/labuda/backend/internal/interaction/notification/entity"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type logoutAllUnitTx struct{}

func (logoutAllUnitTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (logoutAllUnitTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (logoutAllUnitTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

func (logoutAllUnitTx) Commit(ctx context.Context) error {
	return nil
}

func (logoutAllUnitTx) Rollback(ctx context.Context) error {
	return nil
}

var _ db.Tx = logoutAllUnitTx{}

type logoutAllUnitTransactor struct {
	withTxCalls int
}

func (t *logoutAllUnitTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	t.withTxCalls++
	return fn(logoutAllUnitTx{})
}

var _ db.Transactor = (*logoutAllUnitTransactor)(nil)

type logoutAllUnitRefreshRepo struct {
	revokeAllForUserCountCalls int
	revokeAllForUserCount      int64
}

func (r *logoutAllUnitRefreshRepo) FindByTokenHash(ctx context.Context, tx db.Tx, tokenHash string) (*authentity.RefreshSession, error) {
	return nil, authrepo.ErrSessionNotFound
}

func (r *logoutAllUnitRefreshRepo) RevokeFamily(ctx context.Context, tx db.Tx, userID uuid.UUID, familyID uuid.UUID) error {
	return nil
}

func (r *logoutAllUnitRefreshRepo) RevokeFamilyCount(ctx context.Context, tx db.Tx, userID uuid.UUID, familyID uuid.UUID) (int64, error) {
	return r.revokeAllForUserCount, nil
}

func (r *logoutAllUnitRefreshRepo) ListActiveByUser(ctx context.Context, tx db.Tx, userID uuid.UUID) ([]*authentity.RefreshSession, error) {
	return []*authentity.RefreshSession{}, nil
}

func (r *logoutAllUnitRefreshRepo) RevokeAllForUserCount(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error) {
	r.revokeAllForUserCountCalls++
	return r.revokeAllForUserCount, nil
}

type logoutAllUnitFCMRepo struct {
	deactivateAllByUserCalls int
	deactivateAllByUserCount int64
	deactivateAllByUserErr   error
}

func (r *logoutAllUnitFCMRepo) DeactivateByToken(ctx context.Context, tx interface{}, tokenString string) error {
	return nil
}

func (r *logoutAllUnitFCMRepo) DeactivateByUserAndDevice(ctx context.Context, tx interface{}, userID uuid.UUID, deviceID string) error {
	return nil
}

func (r *logoutAllUnitFCMRepo) GetActiveTokensByUser(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*notificationentity.FCMToken, error) {
	return []*notificationentity.FCMToken{}, nil
}

func (r *logoutAllUnitFCMRepo) DeactivateByUserAndDeviceCount(ctx context.Context, tx interface{}, userID uuid.UUID, deviceID string) (int64, error) {
	return 0, nil
}

func (r *logoutAllUnitFCMRepo) DeactivateAllByUser(ctx context.Context, tx interface{}, userID uuid.UUID) (int64, error) {
	r.deactivateAllByUserCalls++
	if r.deactivateAllByUserErr != nil {
		return 0, r.deactivateAllByUserErr
	}
	return r.deactivateAllByUserCount, nil
}

func newLogoutAllUnitHandler(t *testing.T) (*AuthHandler, *observer.ObservedLogs, *logoutAllUnitRefreshRepo, *logoutAllUnitFCMRepo, *logoutAllUnitTransactor) {
	t.Helper()

	logCore, observed := observer.New(zap.InfoLevel)
	log := zap.New(logCore)
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	tokenService := application.NewTokenService(cfg, &logger.Logger{Logger: log})
	tx := &logoutAllUnitTransactor{}
	refreshRepo := &logoutAllUnitRefreshRepo{revokeAllForUserCount: 3}
	fcmRepo := &logoutAllUnitFCMRepo{deactivateAllByUserCount: 4}
	svc := application.NewLogoutCurrentSessionService(tx, tokenService, refreshRepo, fcmRepo, log)
	handler := &AuthHandler{
		logoutService: svc,
		log:           log,
		jwtConfig:     cfg,
		tokenService:  tokenService,
	}
	return handler, observed, refreshRepo, fcmRepo, tx
}

func newLogoutUnitHandler(t *testing.T) (*AuthHandler, *observer.ObservedLogs) {
	t.Helper()

	logCore, observed := observer.New(zap.InfoLevel)
	log := zap.New(logCore)
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}

	return NewAuthHandler(nil, nil, cfg, log), observed
}

func mintLogoutUnitTokenPair(t *testing.T, userID uuid.UUID) *application.TokenPair {
	t.Helper()

	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	svc := application.NewTokenService(cfg, &logger.Logger{Logger: zap.NewNop()})
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("mintLogoutUnitTokenPair: %v", err)
	}
	return pair
}

func callLogoutUnitHandler(t *testing.T, h *AuthHandler, userID uuid.UUID, body any) *httptest.ResponseRecorder {
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

func callLogoutAllUnitHandler(t *testing.T, h *AuthHandler, userID uuid.UUID, body any) *httptest.ResponseRecorder {
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
	req, err := http.NewRequest(http.MethodPost, "/api/v1/auth/logout-all", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("user_id", userID)

	h.LogoutAll(c)
	return w
}

func assertLogoutErrorCode(t *testing.T, w *httptest.ResponseRecorder, statusCode int, code string) {
	t.Helper()
	if w.Code != statusCode {
		t.Fatalf("expected %d, got %d; body=%s", statusCode, w.Code, w.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != code {
		t.Fatalf("expected error code %q, got %+v", code, resp.Error)
	}
}

func assertNoRawTokenLeak(t *testing.T, logs *observer.ObservedLogs, token string) {
	t.Helper()
	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, token) {
			t.Fatalf("raw token leaked in log message: %s", entry.Message)
		}
		for _, field := range entry.Context {
			if strings.Contains(field.Key, token) || strings.Contains(field.String, token) {
				t.Fatalf("raw token leaked in log field: %+v", field)
			}
		}
	}
}

func TestLogoutHandlerUnit_MissingRefreshTokenReturns400(t *testing.T) {
	h, _ := newLogoutUnitHandler(t)

	w := callLogoutUnitHandler(t, h, uuid.New(), map[string]string{
		"fcm_token": "fcm-token-a",
	})

	assertLogoutErrorCode(t, w, http.StatusBadRequest, "BAD_REQUEST")
}

func TestLogoutHandlerUnit_AccessTokenRejectedBeforeDBInteraction(t *testing.T) {
	h, logs := newLogoutUnitHandler(t)

	userID := uuid.New()
	pair := mintLogoutUnitTokenPair(t, userID)

	w := callLogoutUnitHandler(t, h, userID, map[string]string{
		"refresh_token": pair.AccessToken,
	})

	assertLogoutErrorCode(t, w, http.StatusUnauthorized, "INVALID_TOKEN")
	assertNoRawTokenLeak(t, logs, pair.AccessToken)
}

func TestLogoutHandlerUnit_MismatchedRefreshTokenRejectedBeforeDBInteraction(t *testing.T) {
	h, logs := newLogoutUnitHandler(t)

	authUserID := uuid.New()
	tokenOwnerID := uuid.New()
	pair := mintLogoutUnitTokenPair(t, tokenOwnerID)

	w := callLogoutUnitHandler(t, h, authUserID, map[string]string{
		"refresh_token": pair.RefreshToken,
	})

	assertLogoutErrorCode(t, w, http.StatusUnauthorized, "TOKEN_USER_MISMATCH")
	assertNoRawTokenLeak(t, logs, pair.RefreshToken)
}

func TestLogoutAllHandlerUnit_DefaultsToDeactivatingFCMTokensAndReturnsCounts(t *testing.T) {
	h, _, refreshRepo, fcmRepo, tx := newLogoutAllUnitHandler(t)

	w := callLogoutAllUnitHandler(t, h, uuid.New(), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map response data, got %#v", resp.Data)
	}
	if got := data["revoked_sessions_count"]; got != float64(3) {
		t.Fatalf("revoked_sessions_count = %v, want 3", got)
	}
	if got := data["deactivated_fcm_tokens_count"]; got != float64(4) {
		t.Fatalf("deactivated_fcm_tokens_count = %v, want 4", got)
	}
	if refreshRepo.revokeAllForUserCountCalls != 1 {
		t.Fatalf("expected one revoke-all call, got %d", refreshRepo.revokeAllForUserCountCalls)
	}
	if fcmRepo.deactivateAllByUserCalls != 1 {
		t.Fatalf("expected one FCM deactivate-all call, got %d", fcmRepo.deactivateAllByUserCalls)
	}
	if tx.withTxCalls != 2 {
		t.Fatalf("expected two transactions, got %d", tx.withTxCalls)
	}
}

func TestLogoutAllHandlerUnit_DisablesFCMDeactivationWhenRequested(t *testing.T) {
	h, _, refreshRepo, fcmRepo, tx := newLogoutAllUnitHandler(t)

	w := callLogoutAllUnitHandler(t, h, uuid.New(), map[string]bool{
		"deactivate_fcm_tokens": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map response data, got %#v", resp.Data)
	}
	if got := data["revoked_sessions_count"]; got != float64(3) {
		t.Fatalf("revoked_sessions_count = %v, want 3", got)
	}
	if got := data["deactivated_fcm_tokens_count"]; got != float64(0) {
		t.Fatalf("deactivated_fcm_tokens_count = %v, want 0", got)
	}
	if refreshRepo.revokeAllForUserCountCalls != 1 {
		t.Fatalf("expected one revoke-all call, got %d", refreshRepo.revokeAllForUserCountCalls)
	}
	if fcmRepo.deactivateAllByUserCalls != 0 {
		t.Fatalf("expected no FCM deactivate-all call, got %d", fcmRepo.deactivateAllByUserCalls)
	}
	if tx.withTxCalls != 1 {
		t.Fatalf("expected one transaction, got %d", tx.withTxCalls)
	}
}

func TestLogoutAllHandlerUnit_MissingUserReturnsUnauthorized(t *testing.T) {
	h, _, _, _, _ := newLogoutAllUnitHandler(t)

	w := callLogoutAllUnitHandler(t, h, uuid.Nil, nil)
	assertLogoutErrorCode(t, w, http.StatusUnauthorized, "UNAUTHORIZED")
}
