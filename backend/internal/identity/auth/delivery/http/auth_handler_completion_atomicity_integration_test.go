//go:build integration

package http_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	authhttp "github.com/labuda/backend/internal/identity/auth/delivery/http"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/pkg/firebase"
	"go.uber.org/zap"
)

// We want to force a session creation failure to prove that the profile update
// rolls back in the same transaction. We do this by dropping the refresh_sessions
// table temporarily in a subtest.
func TestCompleteProfile_Atomicity(t *testing.T) {
	tdb, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	pool := tdb.Pool()

	ctx := context.Background()

	// 1. Create incomplete user
	firebaseToken := "atomic-test-" + uuid.NewString()[:8]
	exchange := callFirebaseAuth(t, handler, firebaseToken)
	if exchange.Code != http.StatusOK {
		t.Fatalf("exchange failed: %s", exchange.Body.String())
	}
	resp := parseExchangeResponse(t, exchange.Body.Bytes())
	userID := resp.Data["user_id"].(string)
	restrictedToken := resp.Data["access_token"].(string)

	// Verify profile is currently empty
	var username *string
	err := pool.QueryRow(ctx, "SELECT username FROM user_profiles WHERE user_id = $1", userID).Scan(&username)
	if err != nil {
		t.Fatalf("verify empty profile error: %v", err)
	}
	if username != nil && *username != "" {
		t.Fatalf("expected empty username, got %v", *username)
	}

	// Wait a moment to ensure no background operations are still locking the table
	time.Sleep(50 * time.Millisecond)

	// Build a separate mock client and handler just for the sabotage
	log := zap.NewNop()
	mockFirebase := firebase.NewMockClient(&logger.Logger{Logger: log})
	jwtCfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	sabotagedHandler := authhttp.NewAuthHandler(pool, mockFirebase, jwtCfg, log)

	// 2. Sabotage the refresh session table to force session creation failure
	_, err = pool.Exec(ctx, "ALTER TABLE auth_refresh_sessions RENAME TO auth_refresh_sessions_broken")
	if err != nil {
		t.Fatalf("sabotage db: %v", err)
	}
	
	defer func() {
		// Restore table unconditionally to not break other tests
		_, _ = pool.Exec(context.Background(), "ALTER TABLE auth_refresh_sessions_broken RENAME TO auth_refresh_sessions")
	}()

	// 3. Attempt completion
	targetUsername := "atomic_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	completion := callCompleteProfile(t, sabotagedHandler, restrictedToken, targetUsername)
	
	if completion.Code == http.StatusOK {
		t.Fatalf("expected completion to fail due to broken table, but got 200 OK")
	}

	// Restore table so we can query
	_, _ = pool.Exec(context.Background(), "ALTER TABLE auth_refresh_sessions_broken RENAME TO auth_refresh_sessions")

	// 4. Verify transaction rollback - profile should STILL be empty
	err = pool.QueryRow(ctx, "SELECT username FROM user_profiles WHERE user_id = $1", userID).Scan(&username)
	if err != nil {
		t.Fatalf("verify profile after rollback error: %v", err)
	}
	if username != nil && *username != "" {
		t.Fatalf("TRANSACTION ATOMICITY FAILURE! Expected empty username after session failure, but got %v", *username)
	}
}
