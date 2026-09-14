//go:build integration

package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	authEntity "github.com/labuda/backend/internal/identity/auth/entity"
	authRefreshRepo "github.com/labuda/backend/internal/identity/auth/infrastructure/repository"
	userApp "github.com/labuda/backend/internal/identity/user/application"
	userRepoImpl "github.com/labuda/backend/internal/identity/user/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestSelfDelete_RevokesActiveSessions(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	var currentDB string
	tdb.Pool().QueryRow(ctx, "SELECT current_database()").Scan(&currentDB)
	if currentDB == "labuda" {
		t.Fatalf("SAFETY FAIL: connected to main DB")
	}
	userID := uuid.New()
	firebaseUID := "self-delete-a-" + uuid.NewString()
	email := "selfdeleteA@test.com"
	if _, err := tdb.Pool().Exec(ctx, `INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at) VALUES ($1,$2,$3,'active',NOW(),NOW())`, userID, firebaseUID, email); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := tdb.Pool().Exec(ctx, `INSERT INTO user_profiles (user_id, username, created_at, updated_at) VALUES ($1,$2,NOW(),NOW())`, userID, "selfdeletea"); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	refreshRepo := authRefreshRepo.NewRefreshSessionRepository()
	familyID := uuid.New()
	jti := uuid.New()
	tokenHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	expires := time.Now().Add(30 * 24 * time.Hour)
	session, err := authEntity.NewRefreshSession(userID, familyID, jti, tokenHash, expires)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	tx, _ := tdb.Pool().Begin(ctx)
	wrappedTx := &testTx{tx: tx}
	if err := refreshRepo.Create(ctx, wrappedTx, session); err != nil {
		tx.Rollback(ctx)
		t.Fatalf("create session: %v", err)
	}
	tx.Commit(ctx)
	var before int
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='active'`, userID).Scan(&before)
	if before != 1 {
		t.Fatalf("expected 1 active before, got %d", before)
	}
	dbWrapper := db.NewFromPool(tdb.Pool())
	userRepo := userRepoImpl.NewUserRepository(dbWrapper)
	outbox := outboxRepo.NewOutboxRepository(dbWrapper)
	service := userApp.NewUserProfileService(userRepo, nil, nil, outbox, nil, dbWrapper)
	if err := service.SelfDeleteAccount(ctx, userID); err != nil {
		t.Fatalf("SelfDeleteAccount: %v", err)
	}
	var deletedAt *time.Time
	tdb.Pool().QueryRow(ctx, `SELECT deleted_at FROM users WHERE id=$1`, userID).Scan(&deletedAt)
	if deletedAt == nil {
		t.Fatal("expected deleted_at not null")
	}
	var after int
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='active'`, userID).Scan(&after)
	if after != 0 {
		t.Fatalf("expected 0 active after, got %d", after)
	}
	var revoked int
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='revoked'`, userID).Scan(&revoked)
	if revoked != 1 {
		t.Fatalf("expected 1 revoked, got %d", revoked)
	}
	var outboxCount int
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE event_type='user.deleted' AND aggregate_id=$1`, userID).Scan(&outboxCount)
	if outboxCount != 1 {
		t.Fatalf("expected 1 outbox, got %d", outboxCount)
	}
}

func TestSelfDelete_OldRefreshCannotRefresh(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	userID := uuid.New()
	firebaseUID := "self-delete-b-" + uuid.NewString()
	email := "selfdeleteB@test.com"
	tdb.Pool().Exec(ctx, `INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at) VALUES ($1,$2,$3,'active',NOW(),NOW())`, userID, firebaseUID, email)
	tdb.Pool().Exec(ctx, `INSERT INTO user_profiles (user_id, username, created_at, updated_at) VALUES ($1,$2,NOW(),NOW())`, userID, "selfdeleteb")
	refreshRepo := authRefreshRepo.NewRefreshSessionRepository()
	familyID := uuid.New()
	jti := uuid.New()
	tokenHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	s, _ := authEntity.NewRefreshSession(userID, familyID, jti, tokenHash, time.Now().Add(30*24*time.Hour))
	tx, _ := tdb.Pool().Begin(ctx)
	refreshRepo.Create(ctx, &testTx{tx: tx}, s)
	tx.Commit(ctx)
	dbWrapper := db.NewFromPool(tdb.Pool())
	userRepo := userRepoImpl.NewUserRepository(dbWrapper)
	outbox := outboxRepo.NewOutboxRepository(dbWrapper)
	service := userApp.NewUserProfileService(userRepo, nil, nil, outbox, nil, dbWrapper)
	if err := service.SelfDeleteAccount(ctx, userID); err != nil {
		t.Fatalf("self delete: %v", err)
	}
	tx2, _ := tdb.Pool().Begin(ctx)
	_, err := refreshRepo.FindActiveByTokenHash(ctx, &testTx{tx: tx2}, tokenHash)
	tx2.Rollback(ctx)
	if err == nil {
		t.Fatal("expected FindActiveByTokenHash to fail for revoked")
	}
	var dummy string
	err = tdb.Pool().QueryRow(ctx, `SELECT account_status FROM users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&dummy)
	if err == nil {
		t.Fatal("expected user lookup to fail for deleted")
	}
}

func TestSelfDelete_MultipleActiveSessionsAllRevoked(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	userID := uuid.New()
	firebaseUID := "self-delete-c-" + uuid.NewString()
	email := "selfdeleteC@test.com"
	tdb.Pool().Exec(ctx, `INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at) VALUES ($1,$2,$3,'active',NOW(),NOW())`, userID, firebaseUID, email)
	tdb.Pool().Exec(ctx, `INSERT INTO user_profiles (user_id, username, created_at, updated_at) VALUES ($1,$2,NOW(),NOW())`, userID, "selfdeletec")
	refreshRepo := authRefreshRepo.NewRefreshSessionRepository()
	for i := 0; i < 3; i++ {
		familyID := uuid.New()
		jti := uuid.New()
		hash := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
		b := []byte(hash)
		b[63] = byte('0' + i)
		hash = string(b)
		s, _ := authEntity.NewRefreshSession(userID, familyID, jti, hash, time.Now().Add(30*24*time.Hour))
		tx, _ := tdb.Pool().Begin(ctx)
		refreshRepo.Create(ctx, &testTx{tx: tx}, s)
		tx.Commit(ctx)
	}
	var before int
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='active'`, userID).Scan(&before)
	if before != 3 {
		t.Fatalf("expected 3 before, got %d", before)
	}
	dbWrapper := db.NewFromPool(tdb.Pool())
	userRepo := userRepoImpl.NewUserRepository(dbWrapper)
	outbox := outboxRepo.NewOutboxRepository(dbWrapper)
	service := userApp.NewUserProfileService(userRepo, nil, nil, outbox, nil, dbWrapper)
	if err := service.SelfDeleteAccount(ctx, userID); err != nil {
		t.Fatalf("self delete: %v", err)
	}
	var after int
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='active'`, userID).Scan(&after)
	if after != 0 {
		t.Fatalf("expected 0 after, got %d", after)
	}
	var revoked int
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='revoked'`, userID).Scan(&revoked)
	if revoked != 3 {
		t.Fatalf("expected 3 revoked, got %d", revoked)
	}
}

func TestSelfDelete_HistoricalSessionsUntouched(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	userID := uuid.New()
	firebaseUID := "self-delete-d-" + uuid.NewString()
	email := "selfdeleteD@test.com"
	tdb.Pool().Exec(ctx, `INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at) VALUES ($1,$2,$3,'active',NOW(),NOW())`, userID, firebaseUID, email)
	tdb.Pool().Exec(ctx, `INSERT INTO user_profiles (user_id, username, created_at, updated_at) VALUES ($1,$2,NOW(),NOW())`, userID, "selfdeleted")
	activeID := uuid.New()
	consumedID := uuid.New()
	revokedID := uuid.New()
	reusedID := uuid.New()
	expiredID := uuid.New()
	now := time.Now()
	tdb.Pool().Exec(ctx, `INSERT INTO auth_refresh_sessions (id, user_id, family_id, jti, token_hash, status, issued_at, expires_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,'active',NOW(),$6,NOW(),NOW())`, activeID, userID, uuid.New(), uuid.New(), "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", now.Add(30*24*time.Hour))
	tdb.Pool().Exec(ctx, `INSERT INTO auth_refresh_sessions (id, user_id, family_id, jti, token_hash, status, issued_at, expires_at, consumed_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,'consumed',NOW(),$6,NOW(),NOW(),NOW())`, consumedID, userID, uuid.New(), uuid.New(), "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", now.Add(30*24*time.Hour))
	tdb.Pool().Exec(ctx, `INSERT INTO auth_refresh_sessions (id, user_id, family_id, jti, token_hash, status, issued_at, expires_at, revoked_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,'revoked',NOW(),$6,NOW(),NOW(),NOW())`, revokedID, userID, uuid.New(), uuid.New(), "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", now.Add(30*24*time.Hour))
	tdb.Pool().Exec(ctx, `INSERT INTO auth_refresh_sessions (id, user_id, family_id, jti, token_hash, status, issued_at, expires_at, reuse_detected_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,'reused',NOW(),$6,NOW(),NOW(),NOW())`, reusedID, userID, uuid.New(), uuid.New(), "1111111111111111111111111111111111111111111111111111111111111111", now.Add(30*24*time.Hour))
	tdb.Pool().Exec(ctx, `INSERT INTO auth_refresh_sessions (id, user_id, family_id, jti, token_hash, status, issued_at, expires_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,'active',NOW(),$6,NOW(),NOW())`, expiredID, userID, uuid.New(), uuid.New(), "2222222222222222222222222222222222222222222222222222222222222222", now.Add(-1*time.Hour))
	dbWrapper := db.NewFromPool(tdb.Pool())
	outbox := outboxRepo.NewOutboxRepository(dbWrapper)
	userRepo := userRepoImpl.NewUserRepository(dbWrapper)
	service := userApp.NewUserProfileService(userRepo, nil, nil, outbox, nil, dbWrapper)
	if err := service.SelfDeleteAccount(ctx, userID); err != nil {
		t.Fatalf("self delete: %v", err)
	}
	var active, consumed, revoked, reused, expiredRevoked int
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='active' AND expires_at > NOW()`, userID).Scan(&active)
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='consumed'`, userID).Scan(&consumed)
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='revoked'`, userID).Scan(&revoked)
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='reused'`, userID).Scan(&reused)
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE id=$1 AND status='revoked'`, expiredID).Scan(&expiredRevoked)
	if active != 0 {
		t.Fatalf("expected 0 active future, got %d", active)
	}
	if consumed != 1 {
		t.Fatalf("expected 1 consumed, got %d", consumed)
	}
	if reused != 1 {
		t.Fatalf("expected 1 reused, got %d", reused)
	}
	if revoked != 3 {
		t.Fatalf("expected 3 revoked, got %d", revoked)
	}
	if expiredRevoked != 1 {
		t.Fatalf("expected expired active revoked, got %d", expiredRevoked)
	}
}

func TestConcurrency_RefreshBeforeDelete_Serializes(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	var curDB string
	tdb.Pool().QueryRow(ctx, "SELECT current_database()").Scan(&curDB)
	if curDB == "labuda" {
		t.Fatalf("SAFETY: main DB")
	}
	userID := uuid.New()
	firebaseUID := "conc-serial-a-" + uuid.NewString()
	email := "concSerialA@test.com"
	tdb.Pool().Exec(ctx, `INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at) VALUES ($1,$2,$3,'active',NOW(),NOW())`, userID, firebaseUID, email)
	tdb.Pool().Exec(ctx, `INSERT INTO user_profiles (user_id, username, created_at, updated_at) VALUES ($1,$2,NOW(),NOW())`, userID, "concseria")
	refreshRepo := authRefreshRepo.NewRefreshSessionRepository()
	familyID := uuid.New()
	jtiOld := uuid.New()
	hashOld := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sOld, _ := authEntity.NewRefreshSession(userID, familyID, jtiOld, hashOld, time.Now().Add(30*24*time.Hour))
	tx0, _ := tdb.Pool().Begin(ctx)
	refreshRepo.Create(ctx, &testTx{tx: tx0}, sOld)
	tx0.Commit(ctx)

	refreshLocked := make(chan struct{})
	refreshCanCommit := make(chan struct{})
	deleteDone := make(chan struct{})
	var refreshErr error

	go func() {
		tx, _ := tdb.Pool().Begin(ctx)
		// FindActive
		_, err := refreshRepo.FindActiveByTokenHash(ctx, &testTx{tx: tx}, hashOld)
		if err != nil {
			refreshErr = err
			close(refreshLocked)
			tx.Rollback(ctx)
			return
		}
		// LOCK users row FOR UPDATE (hardening)
		var deletedAt *time.Time
		var accountStatus, role string
		err = tx.QueryRow(ctx, `SELECT deleted_at, account_status, role FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&deletedAt, &accountStatus, &role)
		if err != nil {
			refreshErr = err
			close(refreshLocked)
			tx.Rollback(ctx)
			return
		}
		if deletedAt != nil {
			refreshErr = err
			close(refreshLocked)
			tx.Rollback(ctx)
			return
		}
		close(refreshLocked)
		<-refreshCanCommit
		jtiNew := uuid.New()
		hashNew := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		sNew, _ := authEntity.NewRefreshSession(userID, familyID, jtiNew, hashNew, time.Now().Add(30*24*time.Hour))
		if err := refreshRepo.ConsumeAndReplace(ctx, &testTx{tx: tx}, jtiOld, sNew); err != nil {
			refreshErr = err
			tx.Rollback(ctx)
			return
		}
		tx.Commit(ctx)
	}()

	<-refreshLocked
	go func() {
		dbWrapper := db.NewFromPool(tdb.Pool())
		userRepo := userRepoImpl.NewUserRepository(dbWrapper)
		outbox := outboxRepo.NewOutboxRepository(dbWrapper)
		service := userApp.NewUserProfileService(userRepo, nil, nil, outbox, nil, dbWrapper)
		service.SelfDeleteAccount(ctx, userID)
		close(deleteDone)
	}()
	time.Sleep(200 * time.Millisecond)
	close(refreshCanCommit)
	select {
	case <-deleteDone:
	case <-time.After(10 * time.Second):
		t.Fatal("self-delete did not complete after refresh commit (deadlock?)")
	}
	var deletedAt *time.Time
	tdb.Pool().QueryRow(ctx, `SELECT deleted_at FROM users WHERE id=$1`, userID).Scan(&deletedAt)
	if deletedAt == nil {
		t.Fatal("expected deleted_at")
	}
	var active int
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='active'`, userID).Scan(&active)
	if active != 0 {
		t.Fatalf("expected 0 active after refresh-before-delete, got %d (new session should have been revoked)", active)
	}
	if refreshErr != nil {
		t.Fatalf("refresh err: %v", refreshErr)
	}
}

func TestConcurrency_DeleteBeforeRefresh_Serializes(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	var curDB string
	tdb.Pool().QueryRow(ctx, "SELECT current_database()").Scan(&curDB)
	if curDB == "labuda" {
		t.Fatalf("SAFETY: main DB")
	}
	userID := uuid.New()
	firebaseUID := "conc-serial-b-" + uuid.NewString()
	email := "concSerialB@test.com"
	tdb.Pool().Exec(ctx, `INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at) VALUES ($1,$2,$3,'active',NOW(),NOW())`, userID, firebaseUID, email)
	tdb.Pool().Exec(ctx, `INSERT INTO user_profiles (user_id, username, created_at, updated_at) VALUES ($1,$2,NOW(),NOW())`, userID, "concserib")
	refreshRepo := authRefreshRepo.NewRefreshSessionRepository()
	familyID := uuid.New()
	jtiOld := uuid.New()
	hashOld := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	sOld, _ := authEntity.NewRefreshSession(userID, familyID, jtiOld, hashOld, time.Now().Add(30*24*time.Hour))
	tx0, _ := tdb.Pool().Begin(ctx)
	refreshRepo.Create(ctx, &testTx{tx: tx0}, sOld)
	tx0.Commit(ctx)

	// Hold users lock via self-delete style UPDATE in a transaction
	deleteLocked := make(chan struct{})
	deleteCanCommit := make(chan struct{})
	go func() {
		tx, _ := tdb.Pool().Begin(ctx)
		tx.Exec(ctx, `UPDATE users SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, userID)
		close(deleteLocked)
		<-deleteCanCommit
		tx.Exec(ctx, `UPDATE auth_refresh_sessions SET status='revoked', revoked_at=NOW(), updated_at=NOW() WHERE user_id=$1 AND status='active'`, userID)
		tx.Exec(ctx, `INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload, status, retry_count, next_attempt_at, created_at, updated_at, idempotency_key) VALUES ($1,'user',$2,'user.deleted','{}','pending',0,NOW(),NOW(),NOW(),'user.deleted.`+userID.String()+`') ON CONFLICT (idempotency_key) DO NOTHING`, uuid.New(), userID)
		tx.Commit(ctx)
	}()
	<-deleteLocked
	refreshDone := make(chan error, 1)
	go func() {
		tx, _ := tdb.Pool().Begin(ctx)
		_, err := refreshRepo.FindActiveByTokenHash(ctx, &testTx{tx: tx}, hashOld)
		if err != nil {
			refreshDone <- err
			tx.Rollback(ctx)
			return
		}
		var deletedAt *time.Time
		var accountStatus, role string
		err = tx.QueryRow(ctx, `SELECT deleted_at, account_status, role FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&deletedAt, &accountStatus, &role)
		if err != nil {
			refreshDone <- err
			tx.Rollback(ctx)
			return
		}
		if deletedAt != nil {
			tx.Rollback(ctx)
			refreshDone <- errDeletedSentinel
			return
		}
		tx.Rollback(ctx)
		refreshDone <- nil
	}()
	time.Sleep(200 * time.Millisecond)
	close(deleteCanCommit)
	select {
	case err := <-refreshDone:
		if err != errDeletedSentinel {
			t.Fatalf("expected deleted rejection, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("refresh did not complete after delete commit")
	}
	var deletedAt *time.Time
	tdb.Pool().QueryRow(ctx, `SELECT deleted_at FROM users WHERE id=$1`, userID).Scan(&deletedAt)
	if deletedAt == nil {
		t.Fatal("expected deleted")
	}
	var active int
	tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM auth_refresh_sessions WHERE user_id=$1 AND status='active'`, userID).Scan(&active)
	if active != 0 {
		t.Fatalf("expected 0 active, got %d", active)
	}
}

var errDeletedSentinel = context.Canceled // sentinel for test

type testTx struct{ tx pgx.Tx }

func (d *testTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return d.tx.QueryRow(ctx, sql, args...)
}
func (d *testTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return d.tx.Exec(ctx, sql, args...)
}
func (d *testTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return d.tx.Query(ctx, sql, args...)
}
func (d *testTx) Commit(ctx context.Context) error { return d.tx.Commit(ctx) }
func (d *testTx) Rollback(ctx context.Context) error { return d.tx.Rollback(ctx) }
