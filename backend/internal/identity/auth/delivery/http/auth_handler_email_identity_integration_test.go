//go:build integration

package http_test

import (
	"bytes"
	"context"
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

func getUserByEmail(t *testing.T, ctx context.Context, pool *testdb.TestDB, email string) (uuid.UUID, string) {
	t.Helper()

	var id uuid.UUID
	var firebaseUID string
	if err := pool.Pool().QueryRow(ctx, `
		SELECT id, firebase_uid
		FROM users
		WHERE LOWER(BTRIM(email)) = LOWER(BTRIM($1))
		  AND deleted_at IS NULL
	`, email).Scan(&id, &firebaseUID); err != nil {
		t.Fatalf("getUserByEmail: %v", err)
	}
	return id, firebaseUID
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

func TestFirebaseAuth_SequentialSameEmailLinksToLatestFirebaseUID(t *testing.T) {
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

	w1 := callFirebaseAuth(t, handler, firstToken)
	if w1.Code != http.StatusOK {
		t.Fatalf("first auth call: got %d, body=%s", w1.Code, w1.Body.String())
	}
	if got := countUsersByEmail(t, ctx, tdb, "caseemail@test.com"); got != 1 {
		t.Fatalf("expected 1 canonical row after first auth, got %d", got)
	}

	w2 := callFirebaseAuth(t, handler, secondToken)
	if w2.Code != http.StatusOK {
		t.Fatalf("second auth call: got %d, body=%s", w2.Code, w2.Body.String())
	}

	id, firebaseUID := getUserByEmail(t, ctx, tdb, "caseemail@test.com")
	if id == uuid.Nil {
		t.Fatal("expected canonical user id")
	}
	if firebaseUID != secondTok.UID {
		t.Fatalf("expected firebase_uid to relink to latest UID %q, got %q", secondTok.UID, firebaseUID)
	}
	if firebaseUID == firstTok.UID {
		t.Fatalf("firebase_uid should have moved from first UID %q to second UID %q", firstTok.UID, secondTok.UID)
	}
}

func TestFirebaseAuth_ConcurrentSameEmailKeepsOneCanonicalRow(t *testing.T) {
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

	for code := range results {
		if code != http.StatusOK {
			t.Fatalf("expected concurrent auth to succeed, got %d", code)
		}
	}

	if got := countUsersByEmail(t, ctx, tdb, "raceemail@test.com"); got != 1 {
		t.Fatalf("expected a single canonical row after concurrent auth, got %d", got)
	}
}
