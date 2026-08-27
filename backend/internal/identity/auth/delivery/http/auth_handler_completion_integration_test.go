//go:build integration

package http_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func callCompleteProfile(t *testing.T, h *authhttp.AuthHandler, token, username string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	body := []byte(`{"username":"` + username + `"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/auth/complete-profile", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Body = io.NopCloser(bytes.NewReader(body))
	c.Request = req

	h.CompleteProfile(c)
	return w
}

type exchangeResponseEnvelope struct {
	Success bool           `json:"success"`
	Data    map[string]any `json:"data"`
	Error   map[string]any `json:"error"`
}

func newCompletionTokenService() *application.TokenService {
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	return application.NewTokenService(cfg, &logger.Logger{Logger: zap.NewNop()})
}

func parseExchangeResponse(t *testing.T, body []byte) exchangeResponseEnvelope {
	t.Helper()

	var resp exchangeResponseEnvelope
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal exchange response: %v", err)
	}
	return resp
}

func TestFirebaseExchange_CompleteProfileRoundTripAndRecovery(t *testing.T) {
	tdb, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	runPrefix := uuid.NewString()
	shortSuffix := strings.ReplaceAll(runPrefix, "-", "")[:8]
	firebaseToken := "rt-" + shortSuffix
	username := "roundtrip_" + shortSuffix

	exchange := callFirebaseAuth(t, handler, firebaseToken)
	if exchange.Code != http.StatusOK {
		t.Fatalf("exchange status = %d, body=%s", exchange.Code, exchange.Body.String())
	}

	resp := parseExchangeResponse(t, exchange.Body.Bytes())
	data := resp.Data
	if got := data["requires_profile_completion"]; got != true {
		t.Fatalf("requires_profile_completion = %v, want true", got)
	}
	if _, ok := data["refresh_token"]; ok {
		t.Fatalf("incomplete exchange must not return refresh_token: %v", data)
	}
	userID, ok := data["user_id"].(string)
	if !ok || userID == "" {
		t.Fatalf("user_id missing from incomplete exchange response: %v", data)
	}
	restrictedToken, ok := data["access_token"].(string)
	if !ok || restrictedToken == "" {
		t.Fatalf("restricted access_token missing from incomplete exchange response: %v", data)
	}

	svc := newCompletionTokenService()
	claims, err := svc.ValidateRestrictedCompletionToken(restrictedToken)
	if err != nil {
		t.Fatalf("ValidateRestrictedCompletionToken: %v", err)
	}
	if claims.UserID.String() != userID {
		t.Fatalf("restricted token user_id = %s, want %s", claims.UserID, userID)
	}
	if claims.TokenUse != application.TokenUseIdentityCompletion {
		t.Fatalf("restricted token_use = %q, want %q", claims.TokenUse, application.TokenUseIdentityCompletion)
	}
	if claims.Scope != application.ScopeIdentityComplete {
		t.Fatalf("restricted token scope = %q, want %q", claims.Scope, application.ScopeIdentityComplete)
	}
	if claims.ID != "" {
		t.Fatalf("restricted token jti = %q, want empty", claims.ID)
	}
	if claims.FamilyID != "" {
		t.Fatalf("restricted token family_id = %q, want empty", claims.FamilyID)
	}

	var storedUsername sql.NullString
	if err := tdb.Pool().QueryRow(ctx, `
		SELECT username
		FROM user_profiles
		WHERE user_id = $1
	`, claims.UserID).Scan(&storedUsername); err != nil {
		t.Fatalf("query incomplete username: %v", err)
	}
	if storedUsername.Valid {
		t.Fatalf("expected username to be absent before completion, got %q", storedUsername.String)
	}

	completion := callCompleteProfile(t, handler, restrictedToken, username)
	if completion.Code != http.StatusOK {
		t.Fatalf("complete-profile status = %d, body=%s", completion.Code, completion.Body.String())
	}

	var completionResp exchangeResponseEnvelope
	if err := json.Unmarshal(completion.Body.Bytes(), &completionResp); err != nil {
		t.Fatalf("unmarshal completion response: %v", err)
	}
	completionData := completionResp.Data
	if got := completionData["requires_profile_completion"]; got != false {
		t.Fatalf("completion requires_profile_completion = %v, want false", got)
	}
	if _, ok := completionData["refresh_token"].(string); !ok {
		t.Fatalf("completion must return refresh_token: %v", completionData)
	}

	var completedUsername string
	if err := tdb.Pool().QueryRow(ctx, `
		SELECT username
		FROM user_profiles
		WHERE user_id = $1
	`, claims.UserID).Scan(&completedUsername); err != nil {
		t.Fatalf("query completed username: %v", err)
	}
	if completedUsername != username {
		t.Fatalf("completed username = %q, want %q", completedUsername, username)
	}

	repeat := callCompleteProfile(t, handler, restrictedToken, username)
	if repeat.Code != http.StatusConflict {
		t.Fatalf("repeat completion status = %d, body=%s", repeat.Code, repeat.Body.String())
	}
	var repeatResp exchangeResponseEnvelope
	if err := json.Unmarshal(repeat.Body.Bytes(), &repeatResp); err != nil {
		t.Fatalf("unmarshal repeat response: %v", err)
	}
	if repeatResp.Error["code"] != "PROFILE_ALREADY_COMPLETED" {
		t.Fatalf("repeat completion error code = %v, want PROFILE_ALREADY_COMPLETED", repeatResp.Error["code"])
	}

	recovery := callFirebaseAuth(t, handler, firebaseToken)
	if recovery.Code != http.StatusOK {
		t.Fatalf("recovery exchange status = %d, body=%s", recovery.Code, recovery.Body.String())
	}
	var recoveryResp exchangeResponseEnvelope
	if err := json.Unmarshal(recovery.Body.Bytes(), &recoveryResp); err != nil {
		t.Fatalf("unmarshal recovery response: %v", err)
	}
	recoveryData := recoveryResp.Data
	if got := recoveryData["requires_profile_completion"]; got != false {
		t.Fatalf("recovery requires_profile_completion = %v, want false", got)
	}
	if _, ok := recoveryData["refresh_token"].(string); !ok {
		t.Fatalf("recovery exchange must return full session refresh_token: %v", recoveryData)
	}
}

func TestCompleteProfile_ConcurrentDuplicateSubmission_OneSucceeds(t *testing.T) {
	_, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	runPrefix := uuid.NewString()
	shortSuffix := strings.ReplaceAll(runPrefix, "-", "")[:8]
	for i := 0; i < 20; i++ {
		firebaseToken := fmt.Sprintf("cc-%s-%02d", shortSuffix, i)
		username := fmt.Sprintf("concurrent_%s_%02d", shortSuffix, i)

		exchange := callFirebaseAuth(t, handler, firebaseToken)
		if exchange.Code != http.StatusOK {
			t.Fatalf("iteration %d exchange status = %d, body=%s", i, exchange.Code, exchange.Body.String())
		}

		resp := parseExchangeResponse(t, exchange.Body.Bytes())
		restrictedToken, ok := resp.Data["access_token"].(string)
		if !ok || restrictedToken == "" {
			t.Fatalf("iteration %d restricted access_token missing from incomplete exchange response: %v", i, resp.Data)
		}

		start := make(chan struct{})
		results := make(chan *httptest.ResponseRecorder, 2)
		var wg sync.WaitGroup
		for j := 0; j < 2; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				results <- callCompleteProfile(t, handler, restrictedToken, username)
			}()
		}

		close(start)
		wg.Wait()
		close(results)

		successes := 0
		conflicts := 0
		for w := range results {
			switch w.Code {
			case http.StatusOK:
				successes++
			case http.StatusConflict:
				conflicts++
				var conflictResp exchangeResponseEnvelope
				if err := json.Unmarshal(w.Body.Bytes(), &conflictResp); err != nil {
					t.Fatalf("iteration %d unmarshal conflict response: %v", i, err)
				}
				if conflictResp.Error["code"] != "PROFILE_ALREADY_COMPLETED" {
					t.Fatalf("iteration %d unexpected conflict error code: %v", i, conflictResp.Error["code"])
				}
			default:
				t.Fatalf("iteration %d unexpected completion status %d; body=%s", i, w.Code, w.Body.String())
			}
		}

		if successes != 1 || conflicts != 1 {
			t.Fatalf("iteration %d expected one success and one conflict, got success=%d conflict=%d", i, successes, conflicts)
		}
	}
}
