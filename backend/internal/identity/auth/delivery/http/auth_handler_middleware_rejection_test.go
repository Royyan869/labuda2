package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/identity/auth/application"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/logger"
	firebasepkg "github.com/labuda/backend/pkg/firebase"
	"go.uber.org/zap"
)

func testFirebaseAuthClient(t *testing.T) *firebasepkg.Client {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	backendDir := filepath.Dir(file)
	for i := 0; i < 5; i++ {
		backendDir = filepath.Dir(backendDir)
	}

	serviceAccountPath := filepath.Join(backendDir, "configs", "firebase-service-account.json")
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "127.0.0.1:9099")

	client, err := firebasepkg.NewFirebaseClient(&config.FirebaseConfig{
		ProjectID:             "labuda-79de2",
		ServiceAccountKeyPath: serviceAccountPath,
	}, &logger.Logger{Logger: zap.NewNop()})
	if err != nil {
		t.Fatalf("NewFirebaseClient: %v", err)
	}
	return client
}

func mintRestrictedCompletionTokenForMiddlewareTest(t *testing.T) string {
	t.Helper()

	svc := application.NewTokenService(&config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}, &logger.Logger{Logger: zap.NewNop()})

	token, _, err := svc.GenerateRestrictedCompletionToken(uuid.New())
	if err != nil {
		t.Fatalf("mintRestrictedCompletionTokenForMiddlewareTest: %v", err)
	}
	return token
}

func TestAuthMiddleware_RejectsRestrictedCompletionTokenOnAuthenticatedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := testFirebaseAuthClient(t)
	restrictedToken := mintRestrictedCompletionTokenForMiddlewareTest(t)

	router := gin.New()
	called := false
	router.GET(
		"/api/v1/auth/sessions",
		middleware.AuthMiddleware(client),
		func(c *gin.Context) {
			called = true
			c.JSON(http.StatusOK, gin.H{"ok": true})
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+restrictedToken)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusUnauthorized, resp.Body.String())
	}
	if called {
		t.Fatal("authenticated handler must not run when restricted token is rejected")
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	errObj, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("response error payload missing: %v", payload)
	}
	if got, _ := errObj["code"].(string); got != "UNAUTHORIZED" {
		t.Fatalf("error.code = %q, want %q; body=%v", got, "UNAUTHORIZED", payload)
	}
}
