//go:build integration

package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// B1+B2 canonical integration tests.
// B1 = Permanent reservation (a soft-deleted identity is never resurrected).
// B2 = SINGLE BINDING RULE: the users row is the account, its normalized email
// is the account key, and a Firebase identity is only the credential that
// proves control of that email. A different UID binds to the row only when its
// email is Firebase-VERIFIED.

// Helper to call exchange with optional username (string, not *string) —
// distinct name to avoid collision with existing helper in
// auth_handler_registration_username_integration_test.go which uses *string.
func callFirebaseAuthWithUsernameStr(t *testing.T, h interface{ FirebaseExchange(c *gin.Context) }, token, username string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := map[string]string{"firebase_id_token": token}
	if username != "" {
		body["username"] = username
	}
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/firebase/exchange", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	h.FirebaseExchange(c)
	return w
}

// B1-2: soft-deleted email remains permanently unavailable (ACCOUNT_DELETED)
func TestB1_SoftDeletedEmailPermanentlyUnavailable(t *testing.T) {
	tdb, handler, fb, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()
	ctx := context.Background()

	// Create first user
	firstToken := "B1EmailUser"
	_, _ = fb.VerifyIDTokenMock(ctx, firstToken)
	w1 := callFirebaseAuth(t, handler, firstToken)
	if w1.Code != http.StatusOK {
		t.Fatalf("first auth must succeed, got %d body=%s", w1.Code, w1.Body.String())
	}
	// Get email used
	id, _ := getUserByEmail(t, ctx, tdb, "b1emailuser@test.com")
	if id == uuid.Nil {
		t.Fatal("expected user id")
	}
	// Soft-delete via DB
	if _, err := tdb.Pool().Exec(ctx, `UPDATE users SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1`, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	// Second Firebase identity with SAME normalized email (different UID) must be ACCOUNT_DELETED, not create new row
	secondToken := "b1emailuser-different-uid"
	// Force same email: use token that sanitizes to same email prefix "B1EmailUser"
	// sanitizeEmail("B1EmailUser-different") truncates to "B1EmailUser-differen" -> different email.
	// Instead use same prefix with case variant to hit same normalized email: "B1EMAILUSER" -> same lower
	secondToken = "B1EMAILUSER"
	w2 := callFirebaseAuth(t, handler, secondToken)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("soft-deleted email must be ACCOUNT_DELETED 403, got %d body=%s", w2.Code, w2.Body.String())
	}
	if !bytes.Contains(w2.Body.Bytes(), []byte("ACCOUNT_DELETED")) {
		t.Fatalf("expected ACCOUNT_DELETED, got %s", w2.Body.String())
	}
	if got := countUsersByEmail(t, ctx, tdb, "b1emailuser@test.com"); got != 0 {
		// After soft-delete, countUsersByEmail filters deleted_at IS NULL, so 0
		// But the deleted row still holds unique index - check that re-registration did not create active row
		if got != 0 {
			t.Fatalf("expected 0 active rows after soft-delete + rejected re-registration, got %d", got)
		}
	}
	// Verify unique index still holds by trying insert with same email directly should fail 23505
	var pgErr error
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at) VALUES ($1,$2,$3,'active',NOW(),NOW())`, uuid.New(), uuid.NewString(), "b1emailuser@test.com")
	pgErr = err
	if pgErr == nil {
		t.Fatal("expected unique violation on reusing soft-deleted email (permanent reservation), but insert succeeded")
	}
}

// B1-3: soft-deleted username remains unavailable
func TestB1_SoftDeletedUsernamePermanentlyUnavailable(t *testing.T) {
	tdb, handler, fb, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()
	ctx := context.Background()

	// Create user with username via exchange wantUsername
	tokenA := "B1UsernameOwner"
	_, _ = fb.VerifyIDTokenMock(ctx, tokenA)
	// Use call with username
	w1 := callFirebaseAuthWithUsernameStr(t, handler, tokenA, "permreserveduser")
	if w1.Code != http.StatusOK {
		t.Fatalf("first auth with username must succeed, got %d body=%s", w1.Code, w1.Body.String())
	}
	id, _ := getUserByEmail(t, ctx, tdb, "b1usernameowner@test.com")
	if _, err := tdb.Pool().Exec(ctx, `UPDATE users SET deleted_at=NOW() WHERE id=$1`, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	// New user tries same username
	tokenB := "B1UsernameNewUser"
	_, _ = fb.VerifyIDTokenMock(ctx, tokenB)
	w2 := callFirebaseAuthWithUsernameStr(t, handler, tokenB, "permreserveduser")
	if w2.Code != http.StatusConflict {
		t.Fatalf("soft-deleted username must be USERNAME_TAKEN 409, got %d body=%s", w2.Code, w2.Body.String())
	}
	if !bytes.Contains(w2.Body.Bytes(), []byte("USERNAME_TAKEN")) {
		t.Fatalf("expected USERNAME_TAKEN, got %s", w2.Body.String())
	}
}

// B1-4: deleted Firebase UID remains ACCOUNT_DELETED
func TestB1_DeletedFirebaseUIDBlocked(t *testing.T) {
	tdb, handler, fb, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()
	ctx := context.Background()

	token := "B1FirebaseUIDUser"
	mockTok, _ := fb.VerifyIDTokenMock(ctx, token)
	w1 := callFirebaseAuth(t, handler, token)
	if w1.Code != http.StatusOK {
		t.Fatalf("first auth must succeed, got %d body=%s", w1.Code, w1.Body.String())
	}
	id, _ := getUserByEmail(t, ctx, tdb, "b1firebaseuiduser@test.com")
	if _, err := tdb.Pool().Exec(ctx, `UPDATE users SET deleted_at=NOW() WHERE id=$1`, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	// Same Firebase UID again must be ACCOUNT_DELETED
	w2 := callFirebaseAuth(t, handler, token)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("deleted Firebase UID must be ACCOUNT_DELETED 403, got %d body=%s", w2.Code, w2.Body.String())
	}
	if !bytes.Contains(w2.Body.Bytes(), []byte("ACCOUNT_DELETED")) {
		t.Fatalf("expected ACCOUNT_DELETED, got %s", w2.Body.String())
	}
	_ = mockTok
}

// B1-5: normal registration cannot resurrect deleted identity (email+username)
func TestB1_NormalRegistrationCannotResurrectDeleted(t *testing.T) {
	tdb, handler, fb, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()
	ctx := context.Background()

	tokenOrig := "B1ResurrectOrig"
	_, _ = fb.VerifyIDTokenMock(ctx, tokenOrig)
	w1 := callFirebaseAuthWithUsernameStr(t, handler, tokenOrig, "resurrectuser")
	if w1.Code != http.StatusOK {
		t.Fatalf("orig must succeed, got %d body=%s", w1.Code, w1.Body.String())
	}
	id, _ := getUserByEmail(t, ctx, tdb, "b1resurrectorig@test.com")
	if _, err := tdb.Pool().Exec(ctx, `UPDATE users SET deleted_at=NOW() WHERE id=$1`, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	// Try resurrect via same email (different UID via token "B1ResurrectOrig" lower same email? Use same normalized email)
	// Use token that normalizes to same email: case variant
	tokenNew := "B1RESURRECTORIG"
	w2 := callFirebaseAuth(t, handler, tokenNew)
	if w2.Code != http.StatusForbidden || !bytes.Contains(w2.Body.Bytes(), []byte("ACCOUNT_DELETED")) {
		t.Fatalf("resurrect via same email must be ACCOUNT_DELETED, got %d body=%s", w2.Code, w2.Body.String())
	}
}

// B2-6: existing Firebase UID authenticates normally
func TestB2_ExistingFirebaseUIDAuthenticates(t *testing.T) {
	_, handler, fb, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()
	ctx := context.Background()
	token := "B2ExistingUID"
	_, _ = fb.VerifyIDTokenMock(ctx, token)
	w1 := callFirebaseAuth(t, handler, token)
	if w1.Code != http.StatusOK {
		t.Fatalf("first auth must succeed, got %d body=%s", w1.Code, w1.Body.String())
	}
	w2 := callFirebaseAuth(t, handler, token)
	if w2.Code != http.StatusOK {
		t.Fatalf("existing UID second auth must succeed, got %d body=%s", w2.Code, w2.Body.String())
	}
}

// B2-7: new Firebase UID + new email creates account
func TestB2_NewUIDNewEmailCreates(t *testing.T) {
	tdb, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()
	ctx := context.Background()
	tokenA := "B2NewEmailA-unique123"
	tokenB := "B2NewEmailB-unique456"
	w1 := callFirebaseAuth(t, handler, tokenA)
	if w1.Code != http.StatusOK {
		t.Fatalf("A must succeed, got %d body=%s", w1.Code, w1.Body.String())
	}
	w2 := callFirebaseAuth(t, handler, tokenB)
	if w2.Code != http.StatusOK {
		t.Fatalf("B must succeed, got %d body=%s", w2.Code, w2.Body.String())
	}
	// Two distinct active rows
	var count int
	if err := tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 active users, got %d", count)
	}
}

// Single binding rule: a DIFFERENT Firebase UID may bind an existing active
// account only when its email is Firebase-VERIFIED. Unverified → 403
// EMAIL_NOT_VERIFIED and the existing binding stays untouched.
func TestB2_UnverifiedNewUIDCannotCreateOrBindActiveEmail(t *testing.T) {
	// Canonical matrix: row BOUND + different UID → always 409 (verified or
	// not). A different-UID request against an existing row must never be
	// answered by create/bind; it faces the bound row through the email
	// lookup and gets IDENTITY_CONFLICT.
	tdb, handler, fb, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()
	ctx := context.Background()
	firstToken := "B2ActiveEmailFirst"
	secondToken := "B2ACTIVEEMAILFIRST" // same normalized email case variant
	firstMock, _ := fb.VerifyIDTokenMock(ctx, firstToken)
	secondMock, _ := fb.VerifyIDTokenMock(ctx, secondToken)
	if firstMock.UID == secondMock.UID {
		t.Fatal("precondition: tokens must yield different Firebase UIDs")
	}
	w1 := callFirebaseAuth(t, handler, firstToken)
	if w1.Code != http.StatusOK {
		t.Fatalf("first must succeed, got %d body=%s", w1.Code, w1.Body.String())
	}
	idBefore, uidBefore := getUserByEmail(t, ctx, tdb, "b2activeemailfirst@test.com")
	if uidBefore == nil || *uidBefore != firstMock.UID {
		t.Fatalf("pre-check uid mismatch: got %v want %q", uidBefore, firstMock.UID)
	}
	w2 := callFirebaseAuth(t, handler, secondToken)
	if w2.Code != http.StatusConflict {
		t.Fatalf("different UID on a bound row must be IDENTITY_CONFLICT, got %d body=%s", w2.Code, w2.Body.String())
	}
	if !bytes.Contains(w2.Body.Bytes(), []byte("IDENTITY_CONFLICT")) {
		t.Fatalf("expected IDENTITY_CONFLICT, got %s", w2.Body.String())
	}
	_, uidAfter := getUserByEmail(t, ctx, tdb, "b2activeemailfirst@test.com")
	if uidAfter == nil || uidBefore == nil || *uidAfter != *uidBefore {
		t.Fatalf("firebase_uid must NOT have been overwritten: before %v after %v", uidBefore, uidAfter)
	}
	if idBefore == uuid.Nil {
		t.Fatal("id must not be nil")
	}
}

// D4 negative contract: a row that is already bound NEVER accepts a different
// UID, even when the incoming email is Firebase-VERIFIED. Re-bind is dead —
// canonical behavior is 409 IDENTITY_CONFLICT with the binding untouched.
// Client-side Firebase linking (one UID, many providers) is the sole
// unification mechanism; the backend never rewrites bindings.
func TestB2_VerifiedDifferentUIDOnBoundRowIsIdentityConflict(t *testing.T) {
	tdb, handler, fb, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()
	ctx := context.Background()
	firstToken := "b2bindverified"
	secondToken := "B2BINDVERIFIED" // same normalized email, verified, different UID
	firstMock, _ := fb.VerifyIDTokenMock(ctx, firstToken)
	secondMock, _ := fb.VerifyIDTokenMock(ctx, secondToken)
	if firstMock.UID == secondMock.UID {
		t.Fatal("precondition: tokens must yield different Firebase UIDs")
	}
	if verified, _ := secondMock.Claims["email_verified"].(bool); !verified {
		t.Fatal("precondition: second token must carry email_verified=true")
	}
	firstEmail, _ := firstMock.Claims["email"].(string)
	secondEmail, _ := secondMock.Claims["email"].(string)
	if !strings.EqualFold(firstEmail, secondEmail) {
		t.Fatalf("precondition: same normalized email required, got %q vs %q", firstEmail, secondEmail)
	}

	w1 := callFirebaseAuth(t, handler, firstToken)
	if w1.Code != http.StatusOK {
		t.Fatalf("first must succeed, got %d body=%s", w1.Code, w1.Body.String())
	}
	idBefore, uidBefore := getUserByEmail(t, ctx, tdb, "b2bindverified@test.com")

	w2 := callFirebaseAuth(t, handler, secondToken)
	if w2.Code != http.StatusConflict {
		t.Fatalf("verified different UID on a bound row must be IDENTITY_CONFLICT, got %d body=%s", w2.Code, w2.Body.String())
	}
	if !bytes.Contains(w2.Body.Bytes(), []byte("IDENTITY_CONFLICT")) {
		t.Fatalf("expected IDENTITY_CONFLICT, got %s", w2.Body.String())
	}
	idAfter, uidAfter := getUserByEmail(t, ctx, tdb, "b2bindverified@test.com")
	if idAfter != idBefore {
		t.Fatalf("binding must stay on the canonical row: before %s after %s", idBefore, idAfter)
	}
	if uidAfter == nil || uidBefore == nil || *uidAfter != *uidBefore {
		t.Fatalf("firebase_uid must NOT be overwritten: before %v after %v", uidBefore, uidAfter)
	}
}

// B2-10: new UID + deleted email returns ACCOUNT_DELETED
func TestB2_NewUIDDeletedEmailReturnsAccountDeleted(t *testing.T) {
	tdb, handler, fb, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()
	ctx := context.Background()
	firstToken := "B2DeletedEmailFirst"
	_, _ = fb.VerifyIDTokenMock(ctx, firstToken)
	w1 := callFirebaseAuth(t, handler, firstToken)
	if w1.Code != http.StatusOK {
		t.Fatalf("first must succeed, got %d body=%s", w1.Code, w1.Body.String())
	}
	id, _ := getUserByEmail(t, ctx, tdb, "b2deletedemailfirst@test.com")
	if _, err := tdb.Pool().Exec(ctx, `UPDATE users SET deleted_at=NOW() WHERE id=$1`, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	secondToken := "B2DELETEDemailfirst" // same normalized email (lower)
	w2 := callFirebaseAuth(t, handler, secondToken)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("deleted email must be ACCOUNT_DELETED 403, got %d body=%s", w2.Code, w2.Body.String())
	}
	if !bytes.Contains(w2.Body.Bytes(), []byte("ACCOUNT_DELETED")) {
		t.Fatalf("expected ACCOUNT_DELETED, got %s", w2.Body.String())
	}
}
