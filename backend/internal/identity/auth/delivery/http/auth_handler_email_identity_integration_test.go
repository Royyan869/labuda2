//go:build integration

package http_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/config"
	authhttp "github.com/labuda/backend/internal/identity/auth/delivery/http"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/pkg/firebase"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

func setupEmailIdentityHandlerTest(t *testing.T) (*testdb.TestDB, *authhttp.AuthHandler, *firebase.Client, func()) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	log := zap.NewNop()
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}

	fb := firebase.NewMockClient(&logger.Logger{Logger: log})
	handler := authhttp.NewAuthHandler(tdb.Pool(), fb, cfg, log)
	return tdb, handler, fb, cleanup
}

func callFirebaseAuth(t *testing.T, h *authhttp.AuthHandler, token string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	reqBody := []byte(`{"firebase_id_token":"` + token + `"}`)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/auth/firebase/exchange", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	h.FirebaseExchange(c)
	return w
}

func countUsersByEmail(t *testing.T, ctx context.Context, pool *testdb.TestDB, email string) int {
	t.Helper()

	var count int
	if err := pool.Pool().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM users
		WHERE LOWER(BTRIM(email)) = LOWER(BTRIM($1))
		  AND deleted_at IS NULL
	`, email).Scan(&count); err != nil {
		t.Fatalf("countUsersByEmail: %v", err)
	}
	return count
}

// getUserByEmail returns the canonical row id plus its credential binding.
// A nil binding means the account row is unbound (firebase_uid IS NULL).
func getUserByEmail(t *testing.T, ctx context.Context, pool *testdb.TestDB, email string) (uuid.UUID, *string) {
	t.Helper()

	var id uuid.UUID
	var firebaseUID sql.NullString
	if err := pool.Pool().QueryRow(ctx, `
		SELECT id, firebase_uid
		FROM users
		WHERE LOWER(BTRIM(email)) = LOWER(BTRIM($1))
		  AND deleted_at IS NULL
	`, email).Scan(&id, &firebaseUID); err != nil {
		t.Fatalf("getUserByEmail: %v", err)
	}
	if !firebaseUID.Valid {
		return id, nil
	}
	return id, &firebaseUID.String
}

func TestFirebaseAuth_RejectsDuplicateNormalizedEmailInDatabase(t *testing.T) {
	tdb, _, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	firstID := uuid.New()
	secondID := uuid.New()
	email := "Duplicate@Test.Com"

	if _, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
	`, firstID, firstID.String(), email); err != nil {
		t.Fatalf("insert first user: %v", err)
	}

	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
	`, secondID, secondID.String(), "duplicate@test.com")
	if err == nil {
		t.Fatal("expected duplicate normalized email insert to fail")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected pg error, got %T: %v", err, err)
	}
	if pgErr.Code != "23505" {
		t.Fatalf("expected unique violation 23505, got %s", pgErr.Code)
	}
}

func TestFirebaseAuth_DifferentUIDToBoundAccountIsIdentityConflict(t *testing.T) {
	// SINGLE BINDING RULE (canonical exchange matrix): the users row IS the
	// account and its normalized email is the account key; a Firebase identity
	// is only the credential that proves control of that email. A row that is
	// ALREADY BOUND never accepts a different UID — verified or not. Two
	// Firebase identities claiming one account is a canonical anomaly:
	// 409 IDENTITY_CONFLICT, binding untouched (D4 — no re-bind, ever).
	// Client-side Firebase linking unifies providers under ONE UID upstream.
	tdb, handler, fb, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	firstToken := "CaseEmail"
	secondToken := "caseemail"
	firstTok, err := fb.VerifyIDTokenMock(ctx, firstToken)
	if err != nil {
		t.Fatalf("first token mock: %v", err)
	}
	secondTok, err := fb.VerifyIDTokenMock(ctx, secondToken)
	if err != nil {
		t.Fatalf("second token mock: %v", err)
	}
	if firstTok.UID == secondTok.UID {
		t.Fatal("precondition: the two tokens must yield different Firebase UIDs")
	}

	w1 := callFirebaseAuth(t, handler, firstToken)
	if w1.Code != http.StatusOK {
		t.Fatalf("first auth call: got %d, body=%s", w1.Code, w1.Body.String())
	}
	if got := countUsersByEmail(t, ctx, tdb, "caseemail@test.com"); got != 1 {
		t.Fatalf("expected 1 canonical row after first auth, got %d", got)
	}

	w2 := callFirebaseAuth(t, handler, secondToken)
	if w2.Code != http.StatusConflict {
		t.Fatalf("different UID on a bound row must be IDENTITY_CONFLICT: got %d, body=%s", w2.Code, w2.Body.String())
	}
	if !bytes.Contains(w2.Body.Bytes(), []byte("IDENTITY_CONFLICT")) {
		t.Fatalf("expected IDENTITY_CONFLICT code in body, got %s", w2.Body.String())
	}

	id, boundUID := getUserByEmail(t, ctx, tdb, "caseemail@test.com")
	if id == uuid.Nil {
		t.Fatal("expected canonical user id")
	}
	if boundUID == nil || *boundUID != firstTok.UID {
		t.Fatalf("expected firebase_uid to remain the original UID %q, got %v", firstTok.UID, boundUID)
	}
}

func TestFirebaseAuth_UnverifiedEmailCannotBindUnboundRow(t *testing.T) {
	// Matrix row: row UNBOUND + unverified token → 403 EMAIL_NOT_VERIFIED.
	// The binding stays NULL — an unverified email proves nothing.
	tdb, handler, fb, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	fixtureID := uuid.New()
	if _, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at)
		VALUES ($1, NULL, $2, 'active', NOW(), NOW())
	`, fixtureID, "unboundmail@test.com"); err != nil {
		t.Fatalf("insert unbound fixture: %v", err)
	}

	// Mock convention: tokens WITHOUT the "verified" substring are unverified.
	token := "unboundmail"
	tok, err := fb.VerifyIDTokenMock(ctx, token)
	if err != nil {
		t.Fatalf("mock token: %v", err)
	}
	if verified, _ := tok.Claims["email_verified"].(bool); verified {
		t.Fatal("precondition: mock token must be unverified")
	}

	w := callFirebaseAuth(t, handler, token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("unverified email must not bind an unbound row: got %d, body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("EMAIL_NOT_VERIFIED")) {
		t.Fatalf("expected EMAIL_NOT_VERIFIED code in body, got %s", w.Body.String())
	}

	id, boundUID := getUserByEmail(t, ctx, tdb, "unboundmail@test.com")
	if id != fixtureID {
		t.Fatalf("expected the fixture row to be kept: want %s got %s", fixtureID, id)
	}
	if boundUID != nil {
		t.Fatalf("expected firebase_uid to stay NULL, got %q", *boundUID)
	}
}

func TestFirebaseAuth_VerifiedEmailBindsUnboundFixtureAccount(t *testing.T) {
	// Dev/seed fixtures exist as UNBOUND account rows (firebase_uid IS NULL)
	// carrying role/capability/profile. The first login with a
	// Firebase-VERIFIED email binds that row — there is no second linking path
	// and no fabricated UID anywhere in the flow.
	tdb, handler, fb, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	fixtureID := uuid.New()
	if _, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at)
		VALUES ($1, NULL, $2, 'active', NOW(), NOW())
	`, fixtureID, "fixtureverified@test.com"); err != nil {
		t.Fatalf("insert unbound fixture: %v", err)
	}

	token := "fixtureverified"
	tok, err := fb.VerifyIDTokenMock(ctx, token)
	if err != nil {
		t.Fatalf("mock token: %v", err)
	}
	if verified, _ := tok.Claims["email_verified"].(bool); !verified {
		t.Fatal("precondition: mock token must carry email_verified=true")
	}
	if got := tok.Claims["email"]; got != "fixtureverified@test.com" {
		t.Fatalf("precondition: mock email must match the fixture, got %v", got)
	}

	w := callFirebaseAuth(t, handler, token)
	if w.Code != http.StatusOK {
		t.Fatalf("verified login must bind the unbound fixture: got %d, body=%s", w.Code, w.Body.String())
	}

	id, boundUID := getUserByEmail(t, ctx, tdb, "fixtureverified@test.com")
	if id != fixtureID {
		t.Fatalf("binding must reuse the canonical fixture row: want %s got %s", fixtureID, id)
	}
	if boundUID == nil || *boundUID != tok.UID {
		t.Fatalf("expected firebase_uid bound to %q, got %v", tok.UID, boundUID)
	}
}

func TestFirebaseAuth_ConcurrentSameEmailKeepsOneCanonicalRow(t *testing.T) {
	// Concurrent same-email logins with different Firebase UIDs must NEVER
	// create duplicate Labuda accounts and MUST NOT last-writer-wins. Under the
	// single binding rule exactly one succeeds (it creates and binds the row)
	// and the other is rejected with IDENTITY_CONFLICT: the row it now faces is
	// already bound to a different UID. The advisory lock serializes them.
	tdb, handler, _, cleanup := setupEmailIdentityHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	firstToken := "Race/Email"
	secondToken := "RaceEmail"

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan int, 2)

	for _, token := range []string{firstToken, secondToken} {
		wg.Add(1)
		go func(tok string) {
			defer wg.Done()
			<-start
			w := callFirebaseAuth(t, handler, tok)
			results <- w.Code
		}(token)
	}

	close(start)
	wg.Wait()
	close(results)

	var okCount, conflictCount int
	for code := range results {
		switch code {
		case http.StatusOK:
			okCount++
		case http.StatusConflict:
			conflictCount++
		default:
			t.Fatalf("expected concurrent auth to be 200 or 409, got %d", code)
		}
	}
	if okCount != 1 || conflictCount != 1 {
		t.Fatalf("expected exactly 1 success and 1 IDENTITY_CONFLICT, got ok=%d conflict=%d", okCount, conflictCount)
	}

	if got := countUsersByEmail(t, ctx, tdb, "raceemail@test.com"); got != 1 {
		t.Fatalf("expected a single canonical row after concurrent auth, got %d", got)
	}
}
