package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth/application"
	authhttp "github.com/labuda/backend/internal/identity/auth/delivery/http"
)

func callCompleteProfileUnitHandler(t *testing.T, h *authhttp.AuthHandler, token, username string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	body := []byte(`{"username":"` + username + `"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/auth/complete-profile", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	c.Request = req

	h.CompleteProfile(c)
	return w
}

func mintCompletionTokenWithClaims(
	t *testing.T,
	tokenUse string,
	scope string,
	expiry time.Time,
) string {
	t.Helper()

	const secret = "test-secret-32-bytes-long-enough!"
	claims := &application.Claims{
		UserID:    uuid.New(),
		TokenType: application.TokenTypeAccess,
		TokenUse:  tokenUse,
		Scope:     scope,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign completion token: %v", err)
	}
	return tokenString
}

func assertCompletionErrorCode(t *testing.T, w *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()

	if w.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, wantStatus, w.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	errObj, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error payload missing: %v", payload)
	}
	if got, _ := errObj["code"].(string); got != wantCode {
		t.Fatalf("error.code = %q, want %q; body=%v", got, wantCode, payload)
	}
}

func TestCompleteProfile_RejectsFullAccessToken(t *testing.T) {
	h := newRefreshUnitHandler(t)
	accessToken := mintAccessToken(t)

	w := callCompleteProfileUnitHandler(t, h, accessToken, "anyusername")
	assertCompletionErrorCode(t, w, http.StatusForbidden, "INVALID_SCOPE")
}

func TestCompleteProfile_RejectsWrongTokenUse(t *testing.T) {
	h := newRefreshUnitHandler(t)
	token := mintCompletionTokenWithClaims(
		t,
		application.TokenUseAccess,
		application.ScopeIdentityComplete,
		time.Now().Add(15*time.Minute),
	)

	w := callCompleteProfileUnitHandler(t, h, token, "anyusername")
	assertCompletionErrorCode(t, w, http.StatusForbidden, "INVALID_SCOPE")
}

func TestCompleteProfile_RejectsWrongScope(t *testing.T) {
	h := newRefreshUnitHandler(t)
	token := mintCompletionTokenWithClaims(
		t,
		application.TokenUseIdentityCompletion,
		"identity.wrong_scope",
		time.Now().Add(15*time.Minute),
	)

	w := callCompleteProfileUnitHandler(t, h, token, "anyusername")
	assertCompletionErrorCode(t, w, http.StatusForbidden, "INVALID_SCOPE")
}

func TestCompleteProfile_RejectsExpiredRestrictedToken(t *testing.T) {
	h := newRefreshUnitHandler(t)
	token := mintCompletionTokenWithClaims(
		t,
		application.TokenUseIdentityCompletion,
		application.ScopeIdentityComplete,
		time.Now().Add(-1*time.Minute),
	)

	w := callCompleteProfileUnitHandler(t, h, token, "anyusername")
	assertCompletionErrorCode(t, w, http.StatusUnauthorized, "INVALID_RESTRICTED_TOKEN")
}
