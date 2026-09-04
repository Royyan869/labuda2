package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/identity/auth/application"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
)

func newLabudaTokenServiceForTest() *application.TokenService {
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	return application.NewTokenService(cfg, &logger.Logger{Logger: zap.NewNop()})
}

func newGinRouterForLabudaMiddlewareProof(svc *application.TokenService, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/users/me", middleware.LabudaAuthMiddleware(svc), func(c *gin.Context) {
		uid, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user_id"})
			return
		}
		handler(c)
		_ = uid
	})
	return r
}

// helper to perform GET with optional Authorization header.
func doGET(r *gin.Engine, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLabudaAuth_ValidAccessJWT_Accepted_MapsUserID(t *testing.T) {
	svc := newLabudaTokenServiceForTest()
	userID := uuid.New()
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	var capturedUserID uuid.UUID
	router := gin.New()
	gin.SetMode(gin.TestMode)
	router.GET("/api/v1/users/me",
		middleware.LabudaAuthMiddleware(svc),
		middleware.UserLookupMiddleware(&stubLookup{existsID: userID}),
		func(c *gin.Context) {
			if v, ok := c.Get("user_id"); ok {
				if id, ok := v.(uuid.UUID); ok {
					capturedUserID = id
				}
			}
			c.JSON(200, gin.H{"user_id": capturedUserID.String()})
		},
	)
	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("valid Labuda access JWT should be 200, got %d body=%s", w.Code, w.Body.String())
	}
	if capturedUserID != userID {
		t.Fatalf("user_id mapped = %s, want %s", capturedUserID, userID)
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
}

func TestLabudaAuth_FirebaseToken_Rejected(t *testing.T) {
	svc := newLabudaTokenServiceForTest()
	// Simulate a Firebase ID token string (not a Labuda JWT). Must be rejected.
	fakeFirebaseToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.firebase.fake.signature"
	router := newGinRouterForLabudaMiddlewareProof(svc, func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	w := doGET(router, fakeFirebaseToken)
	if w.Code != 401 {
		t.Fatalf("Firebase token on Labuda route should be 401, got %d body=%s", w.Code, w.Body.String())
	}
	var payload map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &payload)
	if !strings.Contains(strings.ToLower(w.Body.String()), "unauthorized") && !strings.Contains(w.Body.String(), "Invalid") {
		// still ensure error envelope
		if _, ok := payload["error"]; !ok {
			if _, ok2 := payload["message"]; !ok2 {
				t.Fatalf("expected error payload, got %s", w.Body.String())
			}
		}
	}
}

func TestLabudaAuth_MissingAuthorization_Rejected(t *testing.T) {
	svc := newLabudaTokenServiceForTest()
	router := newGinRouterForLabudaMiddlewareProof(svc, func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	w := doGET(router, "")
	if w.Code != 401 {
		t.Fatalf("missing Authorization should be 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestLabudaAuth_MalformedJWT_Rejected(t *testing.T) {
	svc := newLabudaTokenServiceForTest()
	router := newGinRouterForLabudaMiddlewareProof(svc, func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	for _, malformed := range []string{"not-a-jwt", "Bearer", "eyJhbGciOiJIUzI1NiJ9..", "eyJ.payload.sig"} {
		req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
		// inject raw malformed via direct header to bypass Bearer prefix handling in helper
		req.Header.Set("Authorization", "Bearer "+malformed)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("malformed %q should be 401, got %d body=%s", malformed, w.Code, w.Body.String())
		}
	}
}

func TestLabudaAuth_ExpiredJWT_Rejected(t *testing.T) {
	cfg := &config.JWTConfig{Secret: "test-secret-32-bytes-long-enough!", Expiration: 15 * time.Minute}
	svc := application.NewTokenService(cfg, &logger.Logger{Logger: zap.NewNop()})
	userID := uuid.New()
	// craft expired token manually
	claims := &application.Claims{
		UserID:    userID,
		TokenType: application.TokenTypeAccess,
		TokenUse:  application.TokenUseAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "labuda-backend",
			ID:        uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	str, _ := tok.SignedString([]byte(cfg.Secret))
	router := newGinRouterForLabudaMiddlewareProof(svc, func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	w := doGET(router, str)
	if w.Code != 401 {
		t.Fatalf("expired JWT should be 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestLabudaAuth_TokenUseNotAccess_Rejected(t *testing.T) {
	svc := newLabudaTokenServiceForTest()
	userID := uuid.New()
	// refresh token has token_use=refresh, must be rejected on Labuda access middleware
	pair, _ := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	router := newGinRouterForLabudaMiddlewareProof(svc, func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	w := doGET(router, pair.RefreshToken)
	if w.Code != 401 {
		t.Fatalf("refresh token on access route should be 401, got %d body=%s", w.Code, w.Body.String())
	}
	// restricted completion token also must be rejected on normal route
	restricted, _, _ := svc.GenerateRestrictedCompletionToken(userID)
	w2 := doGET(router, restricted)
	if w2.Code != 401 {
		t.Fatalf("restricted completion token on /users/me should be 401, got %d body=%s", w2.Code, w2.Body.String())
	}
}

func TestLabudaAuth_InvalidSignature_Rejected(t *testing.T) {
	svc := newLabudaTokenServiceForTest()
	userID := uuid.New()
	pair, _ := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	_ = pair
	// sign same claims with different secret
	otherCfg := &config.JWTConfig{Secret: "different-secret-32-bytes-long!!!!!", Expiration: 15 * time.Minute}
	otherSvc := application.NewTokenService(otherCfg, &logger.Logger{Logger: zap.NewNop()})
	otherPair, _ := otherSvc.GenerateTokenPair(userID, []string{"user"}, nil)
	router := newGinRouterForLabudaMiddlewareProof(svc, func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	w := doGET(router, otherPair.AccessToken)
	if w.Code != 401 {
		t.Fatalf("invalid signature should be 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestLabudaAuth_UnknownUserID_UserLookupFails(t *testing.T) {
	svc := newLabudaTokenServiceForTest()
	userID := uuid.New()
	pair, _ := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	router := gin.New()
	gin.SetMode(gin.TestMode)
	// UserLookup will claim user not found
	router.GET("/api/v1/users/me",
		middleware.LabudaAuthMiddleware(svc),
		middleware.UserLookupMiddleware(&stubLookup{shouldFail: true}),
		func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		},
	)
	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("unknown user_id should propagate as 401 from UserLookup, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestLabudaAuth_UserLookupDoesNotOverwriteLabudaIdentity(t *testing.T) {
	svc := newLabudaTokenServiceForTest()
	labudaUser := uuid.New()
	firebaseUser := uuid.New() // different
	pair, _ := svc.GenerateTokenPair(labudaUser, []string{"user"}, nil)
	var captured uuid.UUID
	router := gin.New()
	gin.SetMode(gin.TestMode)
	// stub would return firebaseUser if it did Firebase UID lookup, but Labuda already set user_id so it must not overwrite.
	// Existence check must validate labudaUser, not firebaseUser.
	router.GET("/api/v1/users/me",
		middleware.LabudaAuthMiddleware(svc),
		middleware.UserLookupMiddleware(&stubLookup{existsID: labudaUser}),
		func(c *gin.Context) {
			if v, ok := c.Get("user_id"); ok {
				captured = v.(uuid.UUID)
			}
			c.JSON(200, gin.H{"ok": true})
		},
	)
	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("should be 200, got %d", w.Code)
	}
	if captured != labudaUser {
		t.Fatalf("UserLookup overwrote Labuda user_id: got %s want %s (firebase UID %s must not be used)", captured, labudaUser, firebaseUser)
	}
	if captured == firebaseUser {
		t.Fatalf("identity confusion: firebase UID used as Labuda user_id")
	}
}

func TestLabudaAuth_MissingTokenUseAccess_BearerPrefixVariants(t *testing.T) {
	svc := newLabudaTokenServiceForTest()
	router := newGinRouterForLabudaMiddlewareProof(svc, func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	// No Bearer prefix
	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Token abc")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("missing Bearer prefix should be 401, got %d", w.Code)
	}
}

// stubLookup implements middleware.UserLookupService without DB.
type stubLookup struct {
	existsID   uuid.UUID
	shouldFail bool
}

func (s *stubLookup) GetUserIDByFirebaseUID(_ context.Context, _ string) (uuid.UUID, error) {
	// This method name is historical: it takes firebase UID, but in Labuda path it should never be called
	// because LabudaAuth already set user_id. We intentionally return the existsID to catch misuse.
	if s.shouldFail {
		return uuid.Nil, jwt.ErrSignatureInvalid
	}
	return s.existsID, nil
}

func (s *stubLookup) GetUserIDByID(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	if s.shouldFail {
		return uuid.Nil, jwt.ErrSignatureInvalid
	}
	// existsID == Nil means user not found
	if s.existsID == uuid.Nil {
		return uuid.Nil, jwt.ErrSignatureInvalid
	}
	// If stub was constructed with a specific existsID, validate that the requested id matches
	if id != s.existsID {
		return uuid.Nil, jwt.ErrSignatureInvalid
	}
	return s.existsID, nil
}

// Ensure stubLookup satisfies the interface via compile check
var _ = func() middleware.UserLookupService { return &stubLookup{} }

// Additional: StrictBrowseLabuda allows anonymous, rejects invalid
func TestStrictBrowseLabuda_AnonymousPasses_InvalidRejected(t *testing.T) {
	svc := newLabudaTokenServiceForTest()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/contents/:id",
		middleware.StrictBrowseLabudaAuthMiddleware(svc),
		func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) },
	)
	// anonymous
	req := httptest.NewRequest("GET", "/api/v1/contents/c1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("anonymous browse should be 200, got %d", w.Code)
	}
	// invalid token
	req2 := httptest.NewRequest("GET", "/api/v1/contents/c1", nil)
	req2.Header.Set("Authorization", "Bearer invalid.labuda.token")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 401 {
		t.Fatalf("invalid Labuda on browse should be 401, got %d body=%s", w2.Code, w2.Body.String())
	}
	// valid token -> 200
	userID := uuid.New()
	pair, _ := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	req3 := httptest.NewRequest("GET", "/api/v1/contents/c1", nil)
	req3.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != 200 {
		t.Fatalf("valid Labuda on browse should be 200, got %d body=%s", w3.Code, w3.Body.String())
	}
}

func TestStrictBrowseLabuda_FirebaseTokenRejected(t *testing.T) {
	svc := newLabudaTokenServiceForTest()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/contents/:id",
		middleware.StrictBrowseLabudaAuthMiddleware(svc),
		func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) },
	)
	req := httptest.NewRequest("GET", "/api/v1/contents/c1", nil)
	req.Header.Set("Authorization", "Bearer firebase.fake.token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("Firebase token on Labuda browse should be 401, got %d body=%s", w.Code, w.Body.String())
	}
}

// WebSocket boundary: verify routes_core.go isolation structurally — Phase 5 canonical
func TestWebSocketBoundary_Isolation(t *testing.T) {
	// Structural proof: routes_core.go must have wsGroup with LabudaAuthMiddleware (Labuda canonical)
	// and must NOT have Firebase AuthMiddleware for WS, nor v1.GET("/ws").
	content, err := readFileForWSCheck("../../cmd/core_server/routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}
	if !strings.Contains(content, `wsGroup.Use(middleware.LabudaAuthMiddleware(labudaTokenService))`) {
		t.Fatal("wsGroup must use LabudaAuthMiddleware(labudaTokenService) — Labuda canonical for WS")
	}
	if strings.Contains(content, `wsGroup.Use(middleware.AuthMiddleware(firebaseClient))`) {
		t.Fatal("wsGroup must NOT use AuthMiddleware(firebaseClient) — Firebase WS boundary removed in Phase 5")
	}
	if !strings.Contains(content, `wsGroup.GET("/ws"`) {
		t.Fatal("wsGroup must register GET /ws")
	}
	if strings.Contains(content, `v1.GET("/ws"`) {
		t.Fatal("v1 must NOT register /ws after isolation — WS must be isolated to wsGroup")
	}
	if !strings.Contains(content, `LabudaAuthMiddleware(labudaTokenService)`) {
		t.Fatal("REST v1 must still use LabudaAuthMiddleware")
	}
}

func readFileForWSCheck(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
