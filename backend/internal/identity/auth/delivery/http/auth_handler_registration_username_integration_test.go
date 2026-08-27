//go:build integration

package http_test

// Stage 1A — Backend Registration Username Contract.
//
// Proves the canonical behavior of POST /api/v1/auth/firebase/exchange when a
// registration username is submitted (or omitted) at identity resolution:
//
//  1. NEW USER + valid username + empty profile  → username assigned, full session
//  2. NEW USER + invalid format                  → 400 USERNAME_INVALID_FORMAT
//  3. NEW USER + reserved username               → 400 USERNAME_RESERVED
//  4. USERNAME already owned by another user     → 409 USERNAME_TAKEN
//  5. EXISTING USER + different username         → 409 USERNAME_IMMUTABLE
//  6. EXISTING USER + same username              → idempotent success/no-op
//  7. EXISTING/LOGIN without username            → username not modified
//
// All tests go through the real database path (testdb + canonical migrations),
// so uniqueness authority is exercised via the user_profiles.username UNIQUE
// index — the same authority the backend relies on in production.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	authhttp "github.com/labuda/backend/internal/identity/auth/delivery/http"
)

// callFirebaseAuthWithUsername exchanges a firebase token with an optional
// registration username in the request body. A nil username omits the field
// entirely, matching the login / Google-first-sync request shape.
func callFirebaseAuthWithUsername(t *testing.T, h *authhttp.AuthHandler, token string, username *string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var body string
	if username == nil {
		body = fmt.Sprintf(`{"firebase_id_token":%q}`, token)
	} else {
		body = fmt.Sprintf(`{"firebase_id_token":%q,"username":%q}`, token, *username)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/auth/firebase/exchange", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	h.FirebaseExchange(c)
	return w
}

func exchangeUsernameData(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var resp exchangeResponseEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal exchange response: %v", err)
	}
	return resp.Data
}

func exchangeUsernameErrorCode(t *testing.T, w *httptest.ResponseRecorder) (int, string) {
	t.Helper()
	var resp exchangeResponseEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal exchange response: %v", err)
	}
	code, _ := resp.Error["code"].(string)
	return w.Code, code
}

func TestRegistrationUsername_ValidUsernameAssignedOnNewUser(t *testing.T) {
	tdb, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	runPrefix := uuid.NewString()
	shortSuffix := strings.ReplaceAll(runPrefix, "-", "")[:8]
	token := "ru-valid-" + shortSuffix
	username := "alice_" + shortSuffix

	w := callFirebaseAuthWithUsername(t, handler, token, &username)
	if w.Code != http.StatusOK {
		t.Fatalf("exchange status = %d, body=%s", w.Code, w.Body.String())
	}

	data := exchangeUsernameData(t, w)
	if got := data["requires_profile_completion"]; got != false {
		t.Fatalf("requires_profile_completion = %v, want false (assigned username completes the profile), body=%v", got, data)
	}
	if _, ok := data["refresh_token"].(string); !ok {
		t.Fatalf("assigned-username exchange must return full session refresh_token: %v", data)
	}

	userID, ok := data["user_id"].(string)
	if !ok || userID == "" {
		t.Fatalf("user_id missing: %v", data)
	}

	var stored string
	if err := tdb.Pool().QueryRow(ctx, `
		SELECT username
		FROM user_profiles
		WHERE user_id = $1
	`, userID).Scan(&stored); err != nil {
		t.Fatalf("query stored username: %v", err)
	}
	if stored != username {
		t.Fatalf("stored username = %q, want %q", stored, username)
	}
}

func TestRegistrationUsername_InvalidFormatRejected(t *testing.T) {
	_, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	runPrefix := uuid.NewString()
	shortSuffix := strings.ReplaceAll(runPrefix, "-", "")[:8]
	token := "ru-invalid-" + shortSuffix
	badUsername := "UPPER!" + shortSuffix // uppercase + '!' fails the [a-z0-9_] pattern

	w := callFirebaseAuthWithUsername(t, handler, token, &badUsername)
	status, code := exchangeUsernameErrorCode(t, w)
	if status != http.StatusBadRequest || code != "USERNAME_INVALID_FORMAT" {
		t.Fatalf("got status=%d code=%q, want 400 USERNAME_INVALID_FORMAT; body=%s", status, code, w.Body.String())
	}
}

func TestRegistrationUsername_ReservedRejected(t *testing.T) {
	_, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	runPrefix := uuid.NewString()
	shortSuffix := strings.ReplaceAll(runPrefix, "-", "")[:8]
	token := "ru-reserved-" + shortSuffix
	reserved := "admin"

	w := callFirebaseAuthWithUsername(t, handler, token, &reserved)
	status, code := exchangeUsernameErrorCode(t, w)
	if status != http.StatusBadRequest || code != "USERNAME_RESERVED" {
		t.Fatalf("got status=%d code=%q, want 400 USERNAME_RESERVED; body=%s", status, code, w.Body.String())
	}
}

func TestRegistrationUsername_UsernameTakenByAnotherUser(t *testing.T) {
	tdb, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	runPrefix := uuid.NewString()
	shortSuffix := strings.ReplaceAll(runPrefix, "-", "")[:8]
	username := "taken_" + shortSuffix

	// User A claims the username.
	firstToken := "ru-taken-a-" + shortSuffix
	wA := callFirebaseAuthWithUsername(t, handler, firstToken, &username)
	if wA.Code != http.StatusOK {
		t.Fatalf("first user exchange status = %d, body=%s", wA.Code, wA.Body.String())
	}

	// User B (different token → different email/UID) tries to claim it.
	secondToken := "ru-taken-b-" + shortSuffix
	wB := callFirebaseAuthWithUsername(t, handler, secondToken, &username)
	status, code := exchangeUsernameErrorCode(t, wB)
	if status != http.StatusConflict || code != "USERNAME_TAKEN" {
		t.Fatalf("got status=%d code=%q, want 409 USERNAME_TAKEN; body=%s", status, code, wB.Body.String())
	}

	// The canonical username must remain owned by user A.
	var ownerCount int
	if err := tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM user_profiles WHERE username = $1
	`, username).Scan(&ownerCount); err != nil {
		t.Fatalf("count username owners: %v", err)
	}
	if ownerCount != 1 {
		t.Fatalf("username %q owned by %d profiles, want 1", username, ownerCount)
	}
}

func TestRegistrationUsername_ImmutableOnDifferentUsername(t *testing.T) {
	tdb, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	runPrefix := uuid.NewString()
	shortSuffix := strings.ReplaceAll(runPrefix, "-", "")[:8]
	token := "ru-immutable-" + shortSuffix
	firstUsername := "fixed_" + shortSuffix
	otherUsername := "other_" + shortSuffix

	w1 := callFirebaseAuthWithUsername(t, handler, token, &firstUsername)
	if w1.Code != http.StatusOK {
		t.Fatalf("first exchange status = %d, body=%s", w1.Code, w1.Body.String())
	}

	// Same user submits a DIFFERENT username → immutable conflict.
	w2 := callFirebaseAuthWithUsername(t, handler, token, &otherUsername)
	status, code := exchangeUsernameErrorCode(t, w2)
	if status != http.StatusConflict || code != "USERNAME_IMMUTABLE" {
		t.Fatalf("got status=%d code=%q, want 409 USERNAME_IMMUTABLE; body=%s", status, code, w2.Body.String())
	}

	// Stored username must be unchanged.
	userID := exchangeUsernameData(t, w1)["user_id"].(string)
	var stored string
	if err := tdb.Pool().QueryRow(ctx, `
		SELECT username
		FROM user_profiles
		WHERE user_id = $1
	`, userID).Scan(&stored); err != nil {
		t.Fatalf("query stored username: %v", err)
	}
	if stored != firstUsername {
		t.Fatalf("stored username = %q, want immutable %q", stored, firstUsername)
	}
}

func TestRegistrationUsername_SameUsernameIdempotent(t *testing.T) {
	tdb, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	runPrefix := uuid.NewString()
	shortSuffix := strings.ReplaceAll(runPrefix, "-", "")[:8]
	token := "ru-idem-" + shortSuffix
	username := "stable_" + shortSuffix

	w1 := callFirebaseAuthWithUsername(t, handler, token, &username)
	if w1.Code != http.StatusOK {
		t.Fatalf("first exchange status = %d, body=%s", w1.Code, w1.Body.String())
	}
	userID := exchangeUsernameData(t, w1)["user_id"].(string)

	// Same user re-submits the SAME username → idempotent success, no conflict.
	w2 := callFirebaseAuthWithUsername(t, handler, token, &username)
	if w2.Code != http.StatusOK {
		t.Fatalf("idempotent exchange status = %d, body=%s", w2.Code, w2.Body.String())
	}

	var stored string
	if err := tdb.Pool().QueryRow(ctx, `
		SELECT username
		FROM user_profiles
		WHERE user_id = $1
	`, userID).Scan(&stored); err != nil {
		t.Fatalf("query stored username: %v", err)
	}
	if stored != username {
		t.Fatalf("stored username = %q, want %q", stored, username)
	}
}

func TestRegistrationUsername_LoginWithoutUsernameDoesNotModify(t *testing.T) {
	tdb, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	runPrefix := uuid.NewString()
	shortSuffix := strings.ReplaceAll(runPrefix, "-", "")[:8]
	token := "ru-login-" + shortSuffix
	username := "login_user_" + shortSuffix

	w1 := callFirebaseAuthWithUsername(t, handler, token, &username)
	if w1.Code != http.StatusOK {
		t.Fatalf("register exchange status = %d, body=%s", w1.Code, w1.Body.String())
	}
	userID := exchangeUsernameData(t, w1)["user_id"].(string)

	// Login (no username in body) → must succeed and must NOT modify the username.
	w2 := callFirebaseAuth(t, handler, token)
	if w2.Code != http.StatusOK {
		t.Fatalf("login exchange status = %d, body=%s", w2.Code, w2.Body.String())
	}

	var stored string
	if err := tdb.Pool().QueryRow(ctx, `
		SELECT username
		FROM user_profiles
		WHERE user_id = $1
	`, userID).Scan(&stored); err != nil {
		t.Fatalf("query stored username: %v", err)
	}
	if stored != username {
		t.Fatalf("stored username = %q after login, want %q", stored, username)
	}
}
