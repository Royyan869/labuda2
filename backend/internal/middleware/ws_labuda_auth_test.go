package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/identity/auth/application"
	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
)

type wsLookup struct {
	known map[uuid.UUID]bool
}

func (w *wsLookup) GetUserIDByFirebaseUID(ctx context.Context, firebaseUID string) (uuid.UUID, error) {
	return uuid.Nil, context.DeadlineExceeded
}

func (w *wsLookup) GetUserIDByID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	if w.known[userID] {
		return userID, nil
	}
	return uuid.Nil, context.DeadlineExceeded
}

func newTokenServiceForTest() *application.TokenService {
	cfg := &config.JWTConfig{
		Secret:     "test-secret-for-ws-labuda-auth-32bytes!!",
		Expiration: time.Hour,
	}
	return application.NewTokenService(cfg, &logger.Logger{Logger: zap.NewNop()})
}

func TestWS_LabudaAcceptance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTokenServiceForTest()
	userID := uuid.New()
	lookup := &wsLookup{known: map[uuid.UUID]bool{userID: true}}
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	r := gin.New()
	grp := r.Group("/api/v1")
	grp.Use(LabudaAuthMiddleware(svc))
	grp.Use(UserLookupMiddleware(lookup))
	grp.GET("/ws", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	grp.GET("/ws/stats", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	for _, path := range []string{"/api/v1/ws", "/api/v1/ws/stats"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("Labuda valid token should be accepted for %s: got %d body %s", path, w.Code, w.Body.String())
		}
	}
}

func TestWS_FirebaseRejection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTokenServiceForTest()
	lookup := &wsLookup{known: map[uuid.UUID]bool{}}
	r := gin.New()
	grp := r.Group("/api/v1")
	grp.Use(LabudaAuthMiddleware(svc))
	grp.Use(UserLookupMiddleware(lookup))
	grp.GET("/ws", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	firebaseFake := "firebase-id-token-fake-abc123"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	req.Header.Set("Authorization", "Bearer "+firebaseFake)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("Firebase token should be rejected by Labuda WS auth: got %d body %s", w.Code, w.Body.String())
	}
}

func TestWS_InvalidLabudaTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTokenServiceForTest()
	userID := uuid.New()
	lookup := &wsLookup{known: map[uuid.UUID]bool{userID: true}}
	r := gin.New()
	grp := r.Group("/api/v1")
	grp.Use(LabudaAuthMiddleware(svc))
	grp.Use(UserLookupMiddleware(lookup))
	grp.GET("/ws", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	// 1. Expired
	expiredCfg := &config.JWTConfig{Secret: "test-secret-for-ws-labuda-auth-32bytes!!", Expiration: -time.Hour}
	expiredSvc := application.NewTokenService(expiredCfg, &logger.Logger{Logger: zap.NewNop()})
	expiredPair, _ := expiredSvc.GenerateTokenPair(userID, []string{"user"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	req.Header.Set("Authorization", "Bearer "+expiredPair.AccessToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expired Labuda token should be rejected, got %d", w.Code)
	}

	// 2. Tampered signature
	pair, _ := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	tampered := pair.AccessToken + "tamper"
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	req.Header.Set("Authorization", "Bearer "+tampered)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("tampered token should be rejected, got %d", w.Code)
	}

	// 3. Wrong token_use (refresh presented as access)
	refreshToken := pair.RefreshToken
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	req.Header.Set("Authorization", "Bearer "+refreshToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("refresh token presented to WS should be rejected, got %d", w.Code)
	}

	// 4. Unknown user
	unknownID := uuid.New()
	unknownPair, _ := svc.GenerateTokenPair(unknownID, []string{"user"}, nil)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	req.Header.Set("Authorization", "Bearer "+unknownPair.AccessToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("unknown user token should be rejected via USER_NOT_PROVISIONED, got %d body %s", w.Code, w.Body.String())
	}
}

func TestWS_CanonicalUserIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTokenServiceForTest()
	userID := uuid.New()
	lookup := &wsLookup{known: map[uuid.UUID]bool{userID: true}}
	r := gin.New()
	grp := r.Group("/api/v1")
	grp.Use(LabudaAuthMiddleware(svc))
	grp.Use(UserLookupMiddleware(lookup))
	grp.GET("/ws", func(c *gin.Context) {
		uid, err := GetUserIDFromContext(c)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if uid != userID {
			c.JSON(500, gin.H{"error": "user_id mismatch"})
			return
		}
		c.JSON(200, gin.H{"user_id": uid.String()})
	})
	pair, _ := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("canonical identity should resolve: %d %s", w.Code, w.Body.String())
	}
}

func TestWS_NoAuthHeaderRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTokenServiceForTest()
	lookup := &wsLookup{known: map[uuid.UUID]bool{}}
	r := gin.New()
	grp := r.Group("/api/v1")
	grp.Use(LabudaAuthMiddleware(svc))
	grp.Use(UserLookupMiddleware(lookup))
	grp.GET("/ws", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("missing Authorization should be rejected, got %d", w.Code)
	}
}
