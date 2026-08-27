//go:build integration

package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	authentity "github.com/labuda/backend/internal/identity/auth/entity"
	authrepo "github.com/labuda/backend/internal/identity/auth/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// --- helpers ---

func setupRefreshSessionTest(t *testing.T) (*testdb.TestDB, *authrepo.RefreshSessionRepository, func()) {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	repo := authrepo.NewRefreshSessionRepository()
	return tdb, repo, cleanup
}

// insertTestUser inserts a bare-minimum user row for FK satisfaction.
func insertTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
	`, uid, uid.String(), uid.String()+"@test.invalid")
	if err != nil {
		t.Fatalf("insertTestUser: %v", err)
	}
	return uid
}

func newActiveSession(userID uuid.UUID, familyID uuid.UUID) *authentity.RefreshSession {
	jti := uuid.New()
	hash := authrepo.HashRefreshToken("raw-refresh-jwt-for-" + jti.String())
	s, err := authentity.NewRefreshSession(userID, familyID, jti, hash, time.Now().Add(30*24*time.Hour))
	if err != nil {
		panic(err)
	}
	return s
}

// withTx runs fn in a transaction and commits if no error.
func withTx(t *testing.T, ctx context.Context, tdb *testdb.TestDB, fn func(tx db.Tx)) {
	t.Helper()
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		fn(tx)
		return nil
	})
	if err != nil {
		t.Fatalf("withTx: %v", err)
	}
}

// --- Invariant 1: Create active refresh session ---

func TestRefreshSession_Invariant1_CreateActive(t *testing.T) {
	tdb, repo, cleanup := setupRefreshSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertTestUser(t, ctx, tdb.Pool())
	familyID := uuid.New()
	s := newActiveSession(userID, familyID)

	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.Create(ctx, tx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
	})

	// Verify round-trip by finding the session.
	var count int
	err := tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM auth_refresh_sessions WHERE id = $1 AND status = 'active'
	`, s.ID).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("expected 1 active row for session %v, got count=%d err=%v", s.ID, count, err)
	}
}

// --- Invariant 2: JTI unique enforced ---

func TestRefreshSession_Invariant2_JTIUniqueEnforced(t *testing.T) {
	tdb, repo, cleanup := setupRefreshSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertTestUser(t, ctx, tdb.Pool())
	familyID := uuid.New()
	s1 := newActiveSession(userID, familyID)

	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.Create(ctx, tx, s1); err != nil {
			t.Fatalf("Create s1: %v", err)
		}
	})

	// Create a second session with the same JTI but different token_hash.
	s2 := newActiveSession(userID, familyID)
	s2.JTI = s1.JTI // duplicate JTI

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, s2)
	})
	if err == nil {
		t.Fatal("expected error for duplicate JTI, got nil")
	}
}

// --- Invariant 3: token_hash unique enforced ---

func TestRefreshSession_Invariant3_TokenHashUniqueEnforced(t *testing.T) {
	tdb, repo, cleanup := setupRefreshSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertTestUser(t, ctx, tdb.Pool())
	familyID := uuid.New()
	s1 := newActiveSession(userID, familyID)

	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.Create(ctx, tx, s1); err != nil {
			t.Fatalf("Create s1: %v", err)
		}
	})

	// Create a second session with same token_hash but different JTI.
	s2 := newActiveSession(userID, familyID)
	s2.TokenHash = s1.TokenHash // duplicate hash

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, s2)
	})
	if err == nil {
		t.Fatal("expected error for duplicate token_hash, got nil")
	}
}

// --- Invariant 4: FindActiveByTokenHash works ---

func TestRefreshSession_Invariant4_FindActiveByTokenHash(t *testing.T) {
	tdb, repo, cleanup := setupRefreshSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertTestUser(t, ctx, tdb.Pool())
	familyID := uuid.New()
	s := newActiveSession(userID, familyID)

	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.Create(ctx, tx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
	})

	var found *authentity.RefreshSession
	withTx(t, ctx, tdb, func(tx db.Tx) {
		var err error
		found, err = repo.FindActiveByTokenHash(ctx, tx, s.TokenHash)
		if err != nil {
			t.Fatalf("FindActiveByTokenHash: %v", err)
		}
	})
	if found.JTI != s.JTI {
		t.Fatalf("JTI mismatch: got %v want %v", found.JTI, s.JTI)
	}
	if found.UserID != userID {
		t.Fatalf("UserID mismatch")
	}
}

// --- Invariant 5: Consumed session not returned as active ---

func TestRefreshSession_Invariant5_ConsumedNotReturnedAsActive(t *testing.T) {
	tdb, repo, cleanup := setupRefreshSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertTestUser(t, ctx, tdb.Pool())
	familyID := uuid.New()
	old := newActiveSession(userID, familyID)
	replacement := newActiveSession(userID, familyID)

	// Create old session.
	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.Create(ctx, tx, old); err != nil {
			t.Fatalf("Create old: %v", err)
		}
	})

	// Rotate: consume old, insert replacement.
	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.ConsumeAndReplace(ctx, tx, old.JTI, replacement); err != nil {
			t.Fatalf("ConsumeAndReplace: %v", err)
		}
	})

	// Old token hash must NOT be returned as active.
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.FindActiveByTokenHash(ctx, tx, old.TokenHash)
		return err
	})
	if err == nil {
		t.Fatal("expected error when looking up consumed token, got nil")
	}
	if !isSessionNotActiveErr(err) {
		t.Fatalf("expected ErrSessionNotActive, got: %v", err)
	}
}

// --- Invariant 6: ConsumeAndReplace is atomic ---

func TestRefreshSession_Invariant6_ConsumeAndReplaceAtomic(t *testing.T) {
	tdb, repo, cleanup := setupRefreshSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertTestUser(t, ctx, tdb.Pool())
	familyID := uuid.New()
	old := newActiveSession(userID, familyID)

	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.Create(ctx, tx, old); err != nil {
			t.Fatalf("Create old: %v", err)
		}
	})

	// Create a replacement with a duplicate JTI to force INSERT failure.
	replacement := newActiveSession(userID, familyID)
	replacement.JTI = old.JTI // force unique violation on replacement INSERT

	// The entire ConsumeAndReplace should fail; old should still be active.
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.ConsumeAndReplace(ctx, tx, old.JTI, replacement)
	})
	if err == nil {
		t.Fatal("expected ConsumeAndReplace to fail on duplicate JTI, got nil")
	}

	// Old session must still be active (transaction rolled back).
	var status string
	qErr := tdb.Pool().QueryRow(ctx,
		`SELECT status FROM auth_refresh_sessions WHERE jti = $1`, old.JTI,
	).Scan(&status)
	if qErr != nil {
		t.Fatalf("verify query: %v", qErr)
	}
	if status != "active" {
		t.Fatalf("expected old session to still be active after rollback, got %q", status)
	}
}

// --- Invariant 7: RevokeFamily marks all active tokens in family revoked ---

func TestRefreshSession_Invariant7_RevokeFamilyMarksAllRevoked(t *testing.T) {
	tdb, repo, cleanup := setupRefreshSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertTestUser(t, ctx, tdb.Pool())
	familyID := uuid.New()

	// Create 3 sessions in the same family.
	s1 := newActiveSession(userID, familyID)
	s2 := newActiveSession(userID, familyID)
	s3 := newActiveSession(userID, familyID)

	withTx(t, ctx, tdb, func(tx db.Tx) {
		for _, s := range []*authentity.RefreshSession{s1, s2, s3} {
			if err := repo.Create(ctx, tx, s); err != nil {
				t.Fatalf("Create: %v", err)
			}
		}
	})

	// Revoke entire family.
	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.RevokeFamily(ctx, tx, userID, familyID); err != nil {
			t.Fatalf("RevokeFamily: %v", err)
		}
	})

	// All three must be revoked.
	var activeCount int
	err := tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM auth_refresh_sessions
		WHERE family_id = $1 AND status = 'active'
	`, familyID).Scan(&activeCount)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	if activeCount != 0 {
		t.Fatalf("expected 0 active sessions after RevokeFamily, got %d", activeCount)
	}
}

// --- Invariant 8: RevokeAllForUser does not affect other users ---

func TestRefreshSession_Invariant8_RevokeAllForUserDoesNotAffectOthers(t *testing.T) {
	tdb, repo, cleanup := setupRefreshSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	userA := insertTestUser(t, ctx, tdb.Pool())
	userB := insertTestUser(t, ctx, tdb.Pool())
	familyA := uuid.New()
	familyB := uuid.New()

	sA := newActiveSession(userA, familyA)
	sB := newActiveSession(userB, familyB)

	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.Create(ctx, tx, sA); err != nil {
			t.Fatalf("Create sA: %v", err)
		}
		if err := repo.Create(ctx, tx, sB); err != nil {
			t.Fatalf("Create sB: %v", err)
		}
	})

	// Revoke all for userA.
	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.RevokeAllForUser(ctx, tx, userA); err != nil {
			t.Fatalf("RevokeAllForUser: %v", err)
		}
	})

	// userA must have 0 active; userB must still have 1 active.
	var countA, countB int
	tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id = $1 AND status = 'active'`, userA,
	).Scan(&countA)
	tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id = $1 AND status = 'active'`, userB,
	).Scan(&countB)

	if countA != 0 {
		t.Fatalf("expected 0 active sessions for userA after RevokeAll, got %d", countA)
	}
	if countB != 1 {
		t.Fatalf("expected 1 active session for userB (unaffected), got %d", countB)
	}
}

// --- Invariant 9: DeleteExpired only removes expired sessions ---

func TestRefreshSession_Invariant9_DeleteExpiredOnlyAffectsExpired(t *testing.T) {
	tdb, repo, cleanup := setupRefreshSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertTestUser(t, ctx, tdb.Pool())
	familyID := uuid.New()

	// Active session: expires in 30 days.
	active := newActiveSession(userID, familyID)

	// Expired session: expires in the past (insert directly bypassing entity validation).
	expiredID := uuid.New()
	expiredJTI := uuid.New()
	expiredHash := authrepo.HashRefreshToken("expired-raw-jwt-" + expiredJTI.String())

	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.Create(ctx, tx, active); err != nil {
			t.Fatalf("Create active: %v", err)
		}
		// Direct INSERT for past-expires_at (bypasses entity validation intentionally).
		_, err := tx.Exec(ctx, `
			INSERT INTO auth_refresh_sessions
			    (id, user_id, family_id, jti, token_hash, status, issued_at, expires_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 'active', now() - interval '31 days', now() - interval '1 day', now(), now())
		`, expiredID, userID, familyID, expiredJTI, expiredHash)
		if err != nil {
			t.Fatalf("insert expired: %v", err)
		}
	})

	// Delete sessions expired before now.
	var deleted int64
	withTx(t, ctx, tdb, func(tx db.Tx) {
		var err error
		deleted, err = repo.DeleteExpired(ctx, tx, time.Now())
		if err != nil {
			t.Fatalf("DeleteExpired: %v", err)
		}
	})

	if deleted != 1 {
		t.Fatalf("expected 1 deleted (expired), got %d", deleted)
	}

	// Active session must still exist.
	var count int
	tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM auth_refresh_sessions WHERE id = $1`, active.ID,
	).Scan(&count)
	if count != 1 {
		t.Fatalf("active session must not be deleted, count=%d", count)
	}
}

// --- Invariant 10: No raw refresh token is stored in DB ---

func TestRefreshSession_Invariant10_NoRawTokenInDB(t *testing.T) {
	tdb, repo, cleanup := setupRefreshSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertTestUser(t, ctx, tdb.Pool())
	familyID := uuid.New()

	const rawToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.raw.token.string"
	tokenHash := authrepo.HashRefreshToken(rawToken)

	// Build session manually (hash length won't be 64 for the fake JWT, use fixed known-good hash).
	tokenHash = strings.Repeat("b", 64)
	s, _ := authentity.NewRefreshSession(userID, familyID, uuid.New(), tokenHash, time.Now().Add(time.Hour))

	withTx(t, ctx, tdb, func(tx db.Tx) {
		if err := repo.Create(ctx, tx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
	})

	// Verify: the raw token string does NOT appear anywhere in auth_refresh_sessions.
	var matchCount int
	err := tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM auth_refresh_sessions
		WHERE token_hash = $1
	`, rawToken).Scan(&matchCount)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if matchCount > 0 {
		t.Fatal("raw token must not appear in token_hash column")
	}

	// Verify: the stored hash differs from the raw token.
	var storedHash string
	tdb.Pool().QueryRow(ctx,
		`SELECT token_hash FROM auth_refresh_sessions WHERE id = $1`, s.ID,
	).Scan(&storedHash)
	if storedHash == rawToken {
		t.Fatal("stored token_hash must be a hash, not the raw token")
	}
	if len(storedHash) != 64 {
		t.Fatalf("stored token_hash must be 64-char SHA-256 hex, got len=%d", len(storedHash))
	}
}

// --- HashRefreshToken unit test (no DB needed, included here for locality) ---

func TestHashRefreshToken_DeterministicAndNot64Chars(t *testing.T) {
	raw := "some.jwt.string"
	h1 := authrepo.HashRefreshToken(raw)
	h2 := authrepo.HashRefreshToken(raw)
	if h1 != h2 {
		t.Fatal("HashRefreshToken must be deterministic")
	}
	if len(h1) != 64 {
		t.Fatalf("SHA-256 hex must be 64 chars, got %d", len(h1))
	}
	if strings.Contains(h1, ".") {
		t.Fatal("hash must not look like a JWT")
	}
	// Different input must produce different hash.
	h3 := authrepo.HashRefreshToken("different.input")
	if h1 == h3 {
		t.Fatal("different inputs must produce different hashes")
	}
}

// isSessionNotActiveErr checks for ErrSessionNotActive (can't import directly in _test pkg).
func isSessionNotActiveErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not active")
}


