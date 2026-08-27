package http_test

// auth_handler_refresh_unit_test.go â€” unit-level regression proof for the
// RefreshToken handler. These tests exercise paths that return early before
// any database interaction, so no DB or Firebase client is required.
//
// Invariants covered (matching TASK B matrix):
//   B-5a: access token submitted to /auth/refresh â†’ 401 INVALID_TOKEN
//   B-5b: malformed/tampered JWT â†’ 401 INVALID_TOKEN
//   B-5c: empty body (missing refresh_token) â†’ 400
//   B-5d: empty string refresh_token â†’ 400
//   B-8:  access token type-check is enforced by ValidateRefreshToken
//         (additional handler-level proof; unit proof in token_service_test.go)

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/identity/auth/application"
	authhttp "github.com/labuda/backend/internal/identity/auth/delivery/http"
	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
)

// newRefreshUnitHandler creates an AuthHandler with nil pool and nil Firebase
// client. This is safe only for tests that exit before any pool.Begin() or
// Firebase call (i.e. JWT-validation failures and empty-body failures).
func newRefreshUnitHandler(t *testing.T) *authhttp.AuthHandler {
	t.Helper()
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	log := zap.NewNop()
	// nil pool and nil firebase â€” safe because tested code paths never reach them.
	return authhttp.NewAuthHandler(nil, nil, cfg, log)
}

// mintAccessToken returns a platform access token (typ=access) signed with the
// same secret used by newRefreshUnitHandler. This is the token that must be
// rejected by the refresh endpoint.
func mintAccessToken(t *testing.T) string {
	t.Helper()
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	svc := application.NewTokenService(cfg, &logger.Logger{Logger: zap.NewNop()})
	pair, err := svc.GenerateTokenPair(uuid.New(), []string{"user"}, nil)
	if err != nil {
		t.Fatalf("mintAccessToken: %v", err)
	}
	return pair.AccessToken
}

// mintRestrictedCompletionToken returns the restricted identity-completion
// access token that must be rejected by the refresh endpoint.
func mintRestrictedCompletionToken(t *testing.T) string {
	t.Helper()
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	svc := application.NewTokenService(cfg, &logger.Logger{Logger: zap.NewNop()})
	token, _, err := svc.GenerateRestrictedCompletionToken(uuid.New())
	if err != nil {
		t.Fatalf("mintRestrictedCompletionToken: %v", err)
	}
	return token
}

// callRefreshHandler sends a POST /auth/refresh request to the handler and
// returns the recorded response.
func callRefreshHandler(t *testing.T, h *authhttp.AuthHandler, body any) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var bodyBytes []byte
	var err error
	if body == nil {
		bodyBytes = nil
	} else {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("callRefreshHandler: marshal: %v", err)
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, err = http.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("callRefreshHandler: new request: %v", err)
	}
	c.Request.Header.Set("Content-Type", "application/json")

	h.RefreshToken(c)
	return w
}

// --- B-5a: access token submitted â†’ 401 INVALID_TOKEN ---

func TestRefreshHandler_B5a_AccessTokenRejected(t *testing.T) {
	h := newRefreshUnitHandler(t)
	accessToken := mintAccessToken(t)

	w := callRefreshHandler(t, h, map[string]string{"refresh_token": accessToken})

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	// Backend wraps errors in {success, error: {code, message}}.
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in response, got: %v", resp)
	}
	code, _ := errObj["code"].(string)
	if code != "INVALID_TOKEN" {
		t.Errorf("expected code=INVALID_TOKEN, got %q; full error=%v", code, errObj)
	}
}

// --- B-5b: tampered/malformed JWT â†’ 401 INVALID_TOKEN ---

func TestRefreshHandler_B5b_MalformedJWTRejected(t *testing.T) {
	h := newRefreshUnitHandler(t)

	malformed := "not.a.valid.jwt.at.all"
	w := callRefreshHandler(t, h, map[string]string{"refresh_token": malformed})

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d; body=%s", w.Code, w.Body.String())
	}
}

// --- B-5c: missing body â†’ 400 ---

func TestRefreshHandler_B5c_MissingBodyRejected(t *testing.T) {
	h := newRefreshUnitHandler(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// Request with no body at all.
	req, _ := http.NewRequest(http.MethodPost, "/auth/refresh", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	h.RefreshToken(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing body, got %d; body=%s", w.Code, w.Body.String())
	}
}

// --- B-5d: empty string refresh_token â†’ 400 ---

func TestRefreshHandler_B5d_EmptyRefreshTokenRejected(t *testing.T) {
	h := newRefreshUnitHandler(t)

	w := callRefreshHandler(t, h, map[string]string{"refresh_token": "   "})

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty refresh_token, got %d; body=%s", w.Code, w.Body.String())
	}
}

// --- B-8: ValidateRefreshToken type-check contract enforced at handler boundary ---
//
// This test doubles as a regression lock for the handler calling
// ValidateRefreshToken (not the looser ValidateToken). If someone changes the
// handler to call ValidateToken instead, an access token would reach the DB
// path and the test below would fail with a nil-pool panic rather than 401.

func TestRefreshHandler_B8_HandlerCallsValidateRefreshToken(t *testing.T) {
	h := newRefreshUnitHandler(t)
	accessToken := mintAccessToken(t)

	// If the handler mistakenly calls ValidateToken instead of ValidateRefreshToken,
	// the access token passes JWT validation and the handler proceeds to pool.Begin()
	// which panics on a nil pool. The test would fail with a panic, not 401.
	// The expected path: ValidateRefreshToken rejects typ=access â†’ 401 before pool.
	w := callRefreshHandler(t, h, map[string]string{"refresh_token": accessToken})

	if w.Code != http.StatusUnauthorized {
		t.Errorf("B-8: access token must be rejected before DB interaction; got %d", w.Code)
	}
}

func TestRefreshHandler_RestrictedCompletionTokenRejected(t *testing.T) {
	h := newRefreshUnitHandler(t)
	restrictedToken := mintRestrictedCompletionToken(t)

	w := callRefreshHandler(t, h, map[string]string{"refresh_token": restrictedToken})

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("restricted completion token must be rejected, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in response, got: %v", resp)
	}
	if code, _ := errObj["code"].(string); code != "INVALID_TOKEN" {
		t.Fatalf("unexpected error code for restricted token: %q", code)
	}
}
