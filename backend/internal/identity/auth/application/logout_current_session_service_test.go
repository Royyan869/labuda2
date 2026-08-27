package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/config"
	authentity "github.com/labuda/backend/internal/identity/auth/entity"
	authrepo "github.com/labuda/backend/internal/identity/auth/infrastructure/repository"
	notificationentity "github.com/labuda/backend/internal/interaction/notification/entity"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type fakeTx struct{}

func (fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

func (fakeTx) Commit(ctx context.Context) error {
	return nil
}

func (fakeTx) Rollback(ctx context.Context) error {
	return nil
}

var _ db.Tx = fakeTx{}

type fakeTransactor struct {
	withTxCalls int
}

func (f *fakeTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	f.withTxCalls++
	return fn(fakeTx{})
}

var _ db.Transactor = (*fakeTransactor)(nil)

type fakeRefreshSessionRepo struct {
	sessions          map[string]*authentity.RefreshSession
	findCalls         []string
	revokeFamilyCalls []struct {
		userID   uuid.UUID
		familyID uuid.UUID
	}
	revokeAllForUserCalls      int
	revokeAllForUserCountCalls int
}

func newFakeRefreshSessionRepo() *fakeRefreshSessionRepo {
	return &fakeRefreshSessionRepo{sessions: map[string]*authentity.RefreshSession{}}
}

func (r *fakeRefreshSessionRepo) seed(token string, session *authentity.RefreshSession) {
	r.sessions[token] = session
}

func (r *fakeRefreshSessionRepo) FindByTokenHash(ctx context.Context, tx db.Tx, tokenHash string) (*authentity.RefreshSession, error) {
	r.findCalls = append(r.findCalls, tokenHash)
	session, ok := r.sessions[tokenHash]
	if !ok {
		return nil, authrepo.ErrSessionNotFound
	}
	copySession := *session
	return &copySession, nil
}

func (r *fakeRefreshSessionRepo) RevokeFamily(ctx context.Context, tx db.Tx, userID uuid.UUID, familyID uuid.UUID) error {
	r.revokeFamilyCalls = append(r.revokeFamilyCalls, struct {
		userID   uuid.UUID
		familyID uuid.UUID
	}{userID: userID, familyID: familyID})
	for _, session := range r.sessions {
		if session.UserID == userID && session.FamilyID == familyID && session.Status == authentity.RefreshSessionStatusActive {
			session.Status = authentity.RefreshSessionStatusRevoked
		}
	}
	return nil
}

func (r *fakeRefreshSessionRepo) RevokeFamilyCount(ctx context.Context, tx db.Tx, userID uuid.UUID, familyID uuid.UUID) (int64, error) {
	var count int64
	for _, session := range r.sessions {
		if session.UserID == userID && session.FamilyID == familyID && session.Status == authentity.RefreshSessionStatusActive {
			count++
		}
	}
	if err := r.RevokeFamily(ctx, tx, userID, familyID); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *fakeRefreshSessionRepo) ListActiveByUser(ctx context.Context, tx db.Tx, userID uuid.UUID) ([]*authentity.RefreshSession, error) {
	var sessions []*authentity.RefreshSession
	now := time.Now()
	for _, session := range r.sessions {
		if session == nil || session.UserID != userID || session.Status != authentity.RefreshSessionStatusActive || !session.ExpiresAt.After(now) {
			continue
		}
		copySession := *session
		sessions = append(sessions, &copySession)
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].IssuedAt.Equal(sessions[j].IssuedAt) {
			return sessions[i].FamilyID.String() < sessions[j].FamilyID.String()
		}
		return sessions[i].IssuedAt.After(sessions[j].IssuedAt)
	})
	return sessions, nil
}

func (r *fakeRefreshSessionRepo) RevokeAllForUserCount(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error) {
	r.revokeAllForUserCountCalls++
	var count int64
	for _, session := range r.sessions {
		if session.UserID == userID && session.Status == authentity.RefreshSessionStatusActive {
			session.Status = authentity.RefreshSessionStatusRevoked
			count++
		}
	}
	return count, nil
}

type fakeFCMRepo struct {
	deactivateByTokenCalls         []string
	deactivateByUserAndDeviceCalls []struct {
		userID   uuid.UUID
		deviceID string
	}
	deactivateAllByUserCalls []uuid.UUID
	activeTokensByUser       map[uuid.UUID]int64
	activeTokensByUserTokens map[uuid.UUID][]*notificationentity.FCMToken
	tokenErr                 error
	deviceErr                error
	allErr                   error
}

func (f *fakeFCMRepo) GetActiveTokensByUser(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*notificationentity.FCMToken, error) {
	if f.activeTokensByUserTokens == nil {
		return []*notificationentity.FCMToken{}, nil
	}
	tokens := f.activeTokensByUserTokens[userID]
	out := make([]*notificationentity.FCMToken, 0, len(tokens))
	for _, token := range tokens {
		if token == nil {
			continue
		}
		copyToken := *token
		out = append(out, &copyToken)
	}
	return out, nil
}

func (f *fakeFCMRepo) DeactivateByToken(ctx context.Context, tx interface{}, tokenString string) error {
	f.deactivateByTokenCalls = append(f.deactivateByTokenCalls, tokenString)
	return f.tokenErr
}

func (f *fakeFCMRepo) DeactivateByUserAndDevice(ctx context.Context, tx interface{}, userID uuid.UUID, deviceID string) error {
	f.deactivateByUserAndDeviceCalls = append(f.deactivateByUserAndDeviceCalls, struct {
		userID   uuid.UUID
		deviceID string
	}{userID: userID, deviceID: deviceID})
	return f.deviceErr
}

func (f *fakeFCMRepo) DeactivateByUserAndDeviceCount(ctx context.Context, tx interface{}, userID uuid.UUID, deviceID string) (int64, error) {
	if err := f.DeactivateByUserAndDevice(ctx, tx, userID, deviceID); err != nil {
		return 0, err
	}
	if f.deviceErr != nil {
		return 0, f.deviceErr
	}
	if strings.TrimSpace(deviceID) == "" {
		return 0, nil
	}
	return 1, nil
}

func (f *fakeFCMRepo) DeactivateAllByUser(ctx context.Context, tx interface{}, userID uuid.UUID) (int64, error) {
	f.deactivateAllByUserCalls = append(f.deactivateAllByUserCalls, userID)
	if f.allErr != nil {
		return 0, f.allErr
	}
	if f.activeTokensByUser == nil {
		f.activeTokensByUser = map[uuid.UUID]int64{}
	}
	count := f.activeTokensByUser[userID]
	f.activeTokensByUser[userID] = 0
	return count, nil
}

func newLogoutCoordinatorHarness(t *testing.T) (*LogoutCurrentSessionService, *fakeTransactor, *fakeRefreshSessionRepo, *fakeFCMRepo, *observer.ObservedLogs, *TokenService) {
	t.Helper()
	logCore, observed := observer.New(zap.InfoLevel)
	log := zap.New(logCore)
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	tokenService := NewTokenService(cfg, &logger.Logger{Logger: log})
	tx := &fakeTransactor{}
	refreshRepo := newFakeRefreshSessionRepo()
	fcmRepo := &fakeFCMRepo{}
	svc := NewLogoutCurrentSessionService(tx, tokenService, refreshRepo, fcmRepo, log)
	return svc, tx, refreshRepo, fcmRepo, observed, tokenService
}

func mintLogoutPair(t *testing.T, tokenService *TokenService, userID uuid.UUID, familyID *uuid.UUID) *TokenPair {
	t.Helper()
	pair, err := tokenService.GenerateTokenPair(userID, []string{"user"}, familyID)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	return pair
}

func newStoredSession(t *testing.T, userID uuid.UUID, pair *TokenPair, status authentity.RefreshSessionStatus) *authentity.RefreshSession {
	t.Helper()
	session, err := authentity.NewRefreshSession(
		userID,
		pair.FamilyID,
		pair.RefreshJTI,
		authrepo.HashRefreshToken(pair.RefreshToken),
		pair.RefreshExpiresAt,
	)
	if err != nil {
		t.Fatalf("NewRefreshSession: %v", err)
	}
	session.Status = status
	return session
}

func assertLogoutError(t *testing.T, err error, code string, status int) {
	t.Helper()
	if err == nil {
		t.Fatal("expected logout error, got nil")
	}
	var logoutErr *LogoutCurrentSessionError
	if !errors.As(err, &logoutErr) {
		t.Fatalf("expected LogoutCurrentSessionError, got %T: %v", err, err)
	}
	if logoutErr.Code != code || logoutErr.Status != status {
		t.Fatalf("expected code=%s status=%d, got code=%s status=%d", code, status, logoutErr.Code, logoutErr.Status)
	}
}

func assertNoTokenLeak(t *testing.T, logs *observer.ObservedLogs, token string) {
	t.Helper()
	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, token) {
			t.Fatalf("raw token leaked in log message: %s", entry.Message)
		}
		for _, field := range entry.Context {
			if strings.Contains(field.Key, token) || strings.Contains(field.String, token) || strings.Contains(fmt.Sprint(field.Interface), token) {
				t.Fatalf("raw token leaked in log field: %+v", field)
			}
		}
	}
}

func TestLogoutCurrentSessionService_RevokeOnlyCurrentFamilyAndKeepOthersActive(t *testing.T) {
	svc, tx, refreshRepo, fcmRepo, logs, tokenService := newLogoutCoordinatorHarness(t)
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()

	currentPair := mintLogoutPair(t, tokenService, userA, nil)
	otherFamilyPair := mintLogoutPair(t, tokenService, userA, nil)
	otherUserPair := mintLogoutPair(t, tokenService, userB, nil)

	refreshRepo.seed(authrepo.HashRefreshToken(currentPair.RefreshToken), newStoredSession(t, userA, currentPair, authentity.RefreshSessionStatusActive))
	refreshRepo.seed(authrepo.HashRefreshToken(otherFamilyPair.RefreshToken), newStoredSession(t, userA, otherFamilyPair, authentity.RefreshSessionStatusActive))
	refreshRepo.seed(authrepo.HashRefreshToken(otherUserPair.RefreshToken), newStoredSession(t, userB, otherUserPair, authentity.RefreshSessionStatusActive))

	err := svc.LogoutCurrentSession(ctx, userA, LogoutCurrentSessionRequest{RefreshToken: currentPair.RefreshToken})
	if err != nil {
		t.Fatalf("LogoutCurrentSession: %v", err)
	}

	if got := refreshRepo.sessions[authrepo.HashRefreshToken(currentPair.RefreshToken)].Status; got != authentity.RefreshSessionStatusRevoked {
		t.Fatalf("current family status = %s, want revoked", got)
	}
	if got := refreshRepo.sessions[authrepo.HashRefreshToken(otherFamilyPair.RefreshToken)].Status; got != authentity.RefreshSessionStatusActive {
		t.Fatalf("same-user other family status = %s, want active", got)
	}
	if got := refreshRepo.sessions[authrepo.HashRefreshToken(otherUserPair.RefreshToken)].Status; got != authentity.RefreshSessionStatusActive {
		t.Fatalf("other user status = %s, want active", got)
	}
	if len(refreshRepo.revokeFamilyCalls) != 1 {
		t.Fatalf("expected 1 revoke family call, got %d", len(refreshRepo.revokeFamilyCalls))
	}
	if refreshRepo.revokeAllForUserCalls != 0 {
		t.Fatalf("expected no revoke-all calls, got %d", refreshRepo.revokeAllForUserCalls)
	}
	if len(fcmRepo.deactivateByTokenCalls) != 0 || len(fcmRepo.deactivateByUserAndDeviceCalls) != 0 {
		t.Fatalf("expected no FCM cleanup for empty request, got token=%d device=%d", len(fcmRepo.deactivateByTokenCalls), len(fcmRepo.deactivateByUserAndDeviceCalls))
	}
	assertNoTokenLeak(t, logs, currentPair.RefreshToken)
	if tx.withTxCalls != 1 {
		t.Fatalf("expected 1 tx for logout revoke, got %d", tx.withTxCalls)
	}
}

func TestLogoutCurrentSessionService_RejectsMismatchAccessAndMissingTokens(t *testing.T) {
	svc, _, refreshRepo, _, logs, tokenService := newLogoutCoordinatorHarness(t)
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()

	pairB := mintLogoutPair(t, tokenService, userB, nil)
	refreshRepo.seed(authrepo.HashRefreshToken(pairB.RefreshToken), newStoredSession(t, userB, pairB, authentity.RefreshSessionStatusActive))

	t.Run("missing", func(t *testing.T) {
		err := svc.LogoutCurrentSession(ctx, userA, LogoutCurrentSessionRequest{RefreshToken: "   "})
		assertLogoutError(t, err, "BAD_REQUEST", 400)
	})

	t.Run("access-token", func(t *testing.T) {
		accessPair := mintLogoutPair(t, tokenService, userA, nil)
		err := svc.LogoutCurrentSession(ctx, userA, LogoutCurrentSessionRequest{RefreshToken: accessPair.AccessToken})
		assertLogoutError(t, err, "INVALID_TOKEN", 401)
		assertNoTokenLeak(t, logs, accessPair.AccessToken)
	})

	t.Run("mismatch", func(t *testing.T) {
		err := svc.LogoutCurrentSession(ctx, userA, LogoutCurrentSessionRequest{RefreshToken: pairB.RefreshToken})
		assertLogoutError(t, err, "TOKEN_USER_MISMATCH", 401)
	})
}

func TestLogoutCurrentSessionService_IdempotentRevokedAndRejectsReused(t *testing.T) {
	svc, tx, refreshRepo, _, _, tokenService := newLogoutCoordinatorHarness(t)
	ctx := context.Background()
	userID := uuid.New()

	pair := mintLogoutPair(t, tokenService, userID, nil)
	tokenHash := authrepo.HashRefreshToken(pair.RefreshToken)
	refreshRepo.seed(tokenHash, newStoredSession(t, userID, pair, authentity.RefreshSessionStatusActive))

	if err := svc.LogoutCurrentSession(ctx, userID, LogoutCurrentSessionRequest{RefreshToken: pair.RefreshToken}); err != nil {
		t.Fatalf("first LogoutCurrentSession: %v", err)
	}
	if err := svc.LogoutCurrentSession(ctx, userID, LogoutCurrentSessionRequest{RefreshToken: pair.RefreshToken}); err != nil {
		t.Fatalf("second LogoutCurrentSession should be idempotent: %v", err)
	}
	if got := refreshRepo.sessions[tokenHash].Status; got != authentity.RefreshSessionStatusRevoked {
		t.Fatalf("revoked session status = %s, want revoked", got)
	}
	if len(refreshRepo.revokeFamilyCalls) != 1 {
		t.Fatalf("expected family revoke only once, got %d", len(refreshRepo.revokeFamilyCalls))
	}
	if tx.withTxCalls != 2 {
		t.Fatalf("expected 2 tx calls for two logouts, got %d", tx.withTxCalls)
	}

	reusedPair := mintLogoutPair(t, tokenService, userID, nil)
	reusedHash := authrepo.HashRefreshToken(reusedPair.RefreshToken)
	refreshRepo.seed(reusedHash, newStoredSession(t, userID, reusedPair, authentity.RefreshSessionStatusReused))
	err := svc.LogoutCurrentSession(ctx, userID, LogoutCurrentSessionRequest{RefreshToken: reusedPair.RefreshToken})
	assertLogoutError(t, err, "TOKEN_REUSE", 401)
}

func TestLogoutCurrentSessionService_FCMCleanupAndFailureAreBestEffort(t *testing.T) {
	svc, tx, refreshRepo, fcmRepo, _, tokenService := newLogoutCoordinatorHarness(t)
	ctx := context.Background()
	userID := uuid.New()

	t.Run("token", func(t *testing.T) {
		pair := mintLogoutPair(t, tokenService, userID, nil)
		tokenHash := authrepo.HashRefreshToken(pair.RefreshToken)
		refreshRepo.seed(tokenHash, newStoredSession(t, userID, pair, authentity.RefreshSessionStatusActive))

		err := svc.LogoutCurrentSession(ctx, userID, LogoutCurrentSessionRequest{
			RefreshToken: pair.RefreshToken,
			FCMToken:     "fcm-token-abc",
		})
		if err != nil {
			t.Fatalf("LogoutCurrentSession: %v", err)
		}
		if len(fcmRepo.deactivateByTokenCalls) != 1 {
			t.Fatalf("expected 1 token deactivation, got %d", len(fcmRepo.deactivateByTokenCalls))
		}
		if len(fcmRepo.deactivateByUserAndDeviceCalls) != 0 {
			t.Fatalf("expected no device deactivation, got %d", len(fcmRepo.deactivateByUserAndDeviceCalls))
		}
		if refreshRepo.sessions[tokenHash].Status != authentity.RefreshSessionStatusRevoked {
			t.Fatalf("expected revoked status after token cleanup path")
		}
	})

	t.Run("device-fallback", func(t *testing.T) {
		pair := mintLogoutPair(t, tokenService, userID, nil)
		tokenHash := authrepo.HashRefreshToken(pair.RefreshToken)
		refreshRepo.seed(tokenHash, newStoredSession(t, userID, pair, authentity.RefreshSessionStatusActive))

		err := svc.LogoutCurrentSession(ctx, userID, LogoutCurrentSessionRequest{
			RefreshToken: pair.RefreshToken,
			DeviceID:     "device-xyz",
		})
		if err != nil {
			t.Fatalf("LogoutCurrentSession: %v", err)
		}
		if len(fcmRepo.deactivateByUserAndDeviceCalls) == 0 {
			t.Fatalf("expected device deactivation call")
		}
		last := fcmRepo.deactivateByUserAndDeviceCalls[len(fcmRepo.deactivateByUserAndDeviceCalls)-1]
		if last.userID != userID || last.deviceID != "device-xyz" {
			t.Fatalf("device deactivation call = %+v, want user=%s device=device-xyz", last, userID)
		}
	})

	t.Run("failure-does-not-undo-revoke", func(t *testing.T) {
		fcmRepo.tokenErr = fmt.Errorf("simulated fcm failure")
		pair := mintLogoutPair(t, tokenService, userID, nil)
		tokenHash := authrepo.HashRefreshToken(pair.RefreshToken)
		refreshRepo.seed(tokenHash, newStoredSession(t, userID, pair, authentity.RefreshSessionStatusActive))

		err := svc.LogoutCurrentSession(ctx, userID, LogoutCurrentSessionRequest{
			RefreshToken: pair.RefreshToken,
			FCMToken:     "fcm-token-fail",
		})
		if err != nil {
			t.Fatalf("LogoutCurrentSession should succeed despite FCM failure: %v", err)
		}
		if refreshRepo.sessions[tokenHash].Status != authentity.RefreshSessionStatusRevoked {
			t.Fatalf("expected revoke to stick despite FCM failure, got %s", refreshRepo.sessions[tokenHash].Status)
		}
	})

	if tx.withTxCalls < 4 {
		t.Fatalf("expected multiple tx calls across revoke and cleanup paths, got %d", tx.withTxCalls)
	}
}

func TestLogoutAllSessionsService_RevokesAllActiveSessionsAndDeactivatesAllFCMTokens(t *testing.T) {
	svc, tx, refreshRepo, fcmRepo, _, tokenService := newLogoutCoordinatorHarness(t)
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()

	pairA1 := mintLogoutPair(t, tokenService, userA, nil)
	pairA2 := mintLogoutPair(t, tokenService, userA, nil)
	pairB := mintLogoutPair(t, tokenService, userB, nil)

	refreshRepo.seed(authrepo.HashRefreshToken(pairA1.RefreshToken), newStoredSession(t, userA, pairA1, authentity.RefreshSessionStatusActive))
	refreshRepo.seed(authrepo.HashRefreshToken(pairA2.RefreshToken), newStoredSession(t, userA, pairA2, authentity.RefreshSessionStatusActive))
	refreshRepo.seed(authrepo.HashRefreshToken(pairB.RefreshToken), newStoredSession(t, userB, pairB, authentity.RefreshSessionStatusActive))
	fcmRepo.activeTokensByUser = map[uuid.UUID]int64{
		userA: 3,
		userB: 2,
	}

	result, err := svc.LogoutAllSessions(ctx, userA, LogoutAllSessionsRequest{})
	if err != nil {
		t.Fatalf("LogoutAllSessions: %v", err)
	}
	if result == nil {
		t.Fatal("expected logout-all result")
	}
	if result.RevokedSessionsCount != 2 {
		t.Fatalf("revoked sessions count = %d, want 2", result.RevokedSessionsCount)
	}
	if result.DeactivatedFCMTokensCount != 3 {
		t.Fatalf("deactivated FCM count = %d, want 3", result.DeactivatedFCMTokensCount)
	}
	if result.FCMWarning != nil {
		t.Fatalf("expected no FCM warning, got %v", result.FCMWarning)
	}
	if got := refreshRepo.sessions[authrepo.HashRefreshToken(pairA1.RefreshToken)].Status; got != authentity.RefreshSessionStatusRevoked {
		t.Fatalf("userA session 1 status = %s, want revoked", got)
	}
	if got := refreshRepo.sessions[authrepo.HashRefreshToken(pairA2.RefreshToken)].Status; got != authentity.RefreshSessionStatusRevoked {
		t.Fatalf("userA session 2 status = %s, want revoked", got)
	}
	if got := refreshRepo.sessions[authrepo.HashRefreshToken(pairB.RefreshToken)].Status; got != authentity.RefreshSessionStatusActive {
		t.Fatalf("other user session status = %s, want active", got)
	}
	if len(refreshRepo.revokeFamilyCalls) != 0 {
		t.Fatalf("logout-all must not use revoke-family, got %d calls", len(refreshRepo.revokeFamilyCalls))
	}
	if refreshRepo.revokeAllForUserCountCalls != 1 {
		t.Fatalf("expected 1 revoke-all count call, got %d", refreshRepo.revokeAllForUserCountCalls)
	}
	if len(fcmRepo.deactivateAllByUserCalls) != 1 || fcmRepo.deactivateAllByUserCalls[0] != userA {
		t.Fatalf("expected one FCM deactivate-all call for userA, got %+v", fcmRepo.deactivateAllByUserCalls)
	}
	if tx.withTxCalls != 2 {
		t.Fatalf("expected 2 tx calls for revoke-all and FCM cleanup, got %d", tx.withTxCalls)
	}
}

func TestLogoutAllSessionsService_IdempotentAndBestEffortFCMFailure(t *testing.T) {
	svc, _, refreshRepo, fcmRepo, _, tokenService := newLogoutCoordinatorHarness(t)
	ctx := context.Background()
	userID := uuid.New()

	pair := mintLogoutPair(t, tokenService, userID, nil)
	tokenHash := authrepo.HashRefreshToken(pair.RefreshToken)
	refreshRepo.seed(tokenHash, newStoredSession(t, userID, pair, authentity.RefreshSessionStatusActive))
	fcmRepo.activeTokensByUser = map[uuid.UUID]int64{userID: 1}

	first, err := svc.LogoutAllSessions(ctx, userID, LogoutAllSessionsRequest{})
	if err != nil {
		t.Fatalf("first LogoutAllSessions: %v", err)
	}
	if first.RevokedSessionsCount != 1 {
		t.Fatalf("first revoke count = %d, want 1", first.RevokedSessionsCount)
	}
	if first.DeactivatedFCMTokensCount != 1 {
		t.Fatalf("first fcm count = %d, want 1", first.DeactivatedFCMTokensCount)
	}

	fcmRepo.allErr = fmt.Errorf("simulated fcm failure")
	second, err := svc.LogoutAllSessions(ctx, userID, LogoutAllSessionsRequest{})
	if err != nil {
		t.Fatalf("second LogoutAllSessions should stay successful despite FCM failure: %v", err)
	}
	if second.RevokedSessionsCount != 0 {
		t.Fatalf("second revoke count = %d, want 0", second.RevokedSessionsCount)
	}
	if second.DeactivatedFCMTokensCount != 0 {
		t.Fatalf("second fcm count = %d, want 0 on failure", second.DeactivatedFCMTokensCount)
	}
	if second.FCMWarning == nil {
		t.Fatal("expected FCM warning on failure")
	}
}

func TestSessionManagementService_ListActiveSessionsDedupesFamiliesAndEnrichesDevices(t *testing.T) {
	svc, _, refreshRepo, fcmRepo, _, tokenService := newLogoutCoordinatorHarness(t)
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()

	familyA := uuid.New()
	familyB := uuid.New()

	olderA := mintLogoutPair(t, tokenService, userA, &familyA)
	newerA := mintLogoutPair(t, tokenService, userA, &familyA)
	otherA := mintLogoutPair(t, tokenService, userA, &familyB)
	otherUser := mintLogoutPair(t, tokenService, userB, nil)

	oldSession := newStoredSession(t, userA, olderA, authentity.RefreshSessionStatusActive)
	oldSession.IssuedAt = time.Now().Add(-2 * time.Hour)
	oldSession.ExpiresAt = time.Now().Add(24 * time.Hour)
	oldSession.DeviceID = ptrString("device-a")
	oldSession.DeviceName = ptrString("Pixel 8")
	oldSession.Platform = ptrString("android")
	oldSession.AppVersion = ptrString("1.2.3")

	newSession := newStoredSession(t, userA, newerA, authentity.RefreshSessionStatusActive)
	newSession.IssuedAt = time.Now().Add(-time.Minute)
	newSession.ExpiresAt = time.Now().Add(24 * time.Hour)
	newSession.DeviceID = ptrString("device-a")
	newSession.DeviceName = ptrString("Pixel 8 Pro")
	newSession.Platform = ptrString("android")
	newSession.AppVersion = ptrString("1.2.4")

	otherSession := newStoredSession(t, userA, otherA, authentity.RefreshSessionStatusActive)
	otherSession.IssuedAt = time.Now().Add(-30 * time.Minute)
	otherSession.ExpiresAt = time.Now().Add(24 * time.Hour)
	otherSession.DeviceID = ptrString("device-b")
	otherSession.DeviceName = ptrString("iPhone 15")
	otherSession.Platform = ptrString("ios")
	otherSession.AppVersion = ptrString("2.0.0")

	revokedSession := newStoredSession(t, userA, mintLogoutPair(t, tokenService, userA, nil), authentity.RefreshSessionStatusRevoked)
	reusedSession := newStoredSession(t, userA, mintLogoutPair(t, tokenService, userA, nil), authentity.RefreshSessionStatusReused)
	expiredSession := newStoredSession(t, userA, mintLogoutPair(t, tokenService, userA, nil), authentity.RefreshSessionStatusActive)
	expiredSession.ExpiresAt = time.Now().Add(-time.Hour)
	otherUserSession := newStoredSession(t, userB, otherUser, authentity.RefreshSessionStatusActive)

	refreshRepo.seed(oldSession.TokenHash, oldSession)
	refreshRepo.seed(newSession.TokenHash, newSession)
	refreshRepo.seed(otherSession.TokenHash, otherSession)
	refreshRepo.seed(revokedSession.TokenHash, revokedSession)
	refreshRepo.seed(reusedSession.TokenHash, reusedSession)
	refreshRepo.seed(expiredSession.TokenHash, expiredSession)
	refreshRepo.seed(otherUserSession.TokenHash, otherUserSession)

	fcmRepo.activeTokensByUserTokens = map[uuid.UUID][]*notificationentity.FCMToken{
		userA: {
			{
				ID:         uuid.New(),
				UserID:     userA,
				Token:      "token-a",
				Platform:   notificationentity.FCMPlatformAndroid,
				DeviceID:   ptrString("device-a"),
				DeviceName: ptrString("Pixel 8 Pro"),
				AppVersion: ptrString("1.2.4"),
				IsActive:   true,
				LastUsedAt: ptrTime(time.Now().Add(-10 * time.Minute)),
			},
			{
				ID:         uuid.New(),
				UserID:     userA,
				Token:      "token-b",
				Platform:   notificationentity.FCMPlatformIOS,
				DeviceID:   ptrString("device-b"),
				DeviceName: ptrString("iPhone 15"),
				AppVersion: ptrString("2.0.0"),
				IsActive:   true,
				LastUsedAt: ptrTime(time.Now().Add(-5 * time.Minute)),
			},
		},
	}

	result, err := svc.ListActiveSessions(ctx, userA)
	if err != nil {
		t.Fatalf("ListActiveSessions: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 distinct active families, got %d", len(result))
	}
	if result[0].FamilyID != familyA {
		t.Fatalf("first family = %s, want %s", result[0].FamilyID, familyA)
	}
	if result[1].FamilyID != familyB {
		t.Fatalf("second family = %s, want %s", result[1].FamilyID, familyB)
	}
	if result[0].DeviceName == nil || *result[0].DeviceName != "Pixel 8 Pro" {
		t.Fatalf("expected enriched device name for familyA, got %#v", result[0].DeviceName)
	}
	if result[0].LastUsedAt == nil {
		t.Fatal("expected last_used_at enrichment for familyA")
	}
	if result[1].DeviceName == nil || *result[1].DeviceName != "iPhone 15" {
		t.Fatalf("expected familyB device name, got %#v", result[1].DeviceName)
	}
}

func TestSessionManagementService_RevokeSessionFamilyIdempotentAndScoped(t *testing.T) {
	svc, tx, refreshRepo, fcmRepo, _, tokenService := newLogoutCoordinatorHarness(t)
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()

	familyA := uuid.New()
	familyB := uuid.New()
	otherUserFamily := uuid.New()

	sessionA1 := newStoredSession(t, userA, mintLogoutPair(t, tokenService, userA, &familyA), authentity.RefreshSessionStatusActive)
	sessionA1.DeviceID = ptrString("device-a")
	sessionA2 := newStoredSession(t, userA, mintLogoutPair(t, tokenService, userA, &familyA), authentity.RefreshSessionStatusActive)
	sessionA2.DeviceID = ptrString("device-a")
	sessionB := newStoredSession(t, userA, mintLogoutPair(t, tokenService, userA, &familyB), authentity.RefreshSessionStatusActive)
	sessionOtherUser := newStoredSession(t, userB, mintLogoutPair(t, tokenService, userB, &otherUserFamily), authentity.RefreshSessionStatusActive)

	refreshRepo.seed(sessionA1.TokenHash, sessionA1)
	refreshRepo.seed(sessionA2.TokenHash, sessionA2)
	refreshRepo.seed(sessionB.TokenHash, sessionB)
	refreshRepo.seed(sessionOtherUser.TokenHash, sessionOtherUser)
	fcmRepo.activeTokensByUserTokens = map[uuid.UUID][]*notificationentity.FCMToken{
		userA: {
			{
				ID:         uuid.New(),
				UserID:     userA,
				Token:      "token-a",
				Platform:   notificationentity.FCMPlatformAndroid,
				DeviceID:   ptrString("device-a"),
				IsActive:   true,
				LastUsedAt: ptrTime(time.Now()),
			},
		},
	}

	result, err := svc.RevokeSessionFamily(ctx, userA, familyA)
	if err != nil {
		t.Fatalf("RevokeSessionFamily: %v", err)
	}
	if result == nil {
		t.Fatal("expected revoke result")
	}
	if result.RevokedSessionsCount != 2 {
		t.Fatalf("revoked count = %d, want 2", result.RevokedSessionsCount)
	}
	if result.DeactivatedFCMTokensCount != 1 {
		t.Fatalf("FCM deactivate count = %d, want 1", result.DeactivatedFCMTokensCount)
	}
	if result.FCMWarning != nil {
		t.Fatalf("expected no FCM warning, got %v", result.FCMWarning)
	}
	if refreshRepo.sessions[sessionA1.TokenHash].Status != authentity.RefreshSessionStatusRevoked {
		t.Fatal("sessionA1 should be revoked")
	}
	if refreshRepo.sessions[sessionA2.TokenHash].Status != authentity.RefreshSessionStatusRevoked {
		t.Fatal("sessionA2 should be revoked")
	}
	if refreshRepo.sessions[sessionB.TokenHash].Status != authentity.RefreshSessionStatusActive {
		t.Fatal("same-user other family must stay active")
	}
	if refreshRepo.sessions[sessionOtherUser.TokenHash].Status != authentity.RefreshSessionStatusActive {
		t.Fatal("other user session must stay active")
	}
	if tx.withTxCalls != 2 {
		t.Fatalf("expected 2 tx calls (list + revoke), got %d", tx.withTxCalls)
	}
}

func TestSessionManagementService_RevokeUnknownFamilyIsIdempotent(t *testing.T) {
	svc, _, refreshRepo, _, _, tokenService := newLogoutCoordinatorHarness(t)
	ctx := context.Background()
	userID := uuid.New()

	session := newStoredSession(t, userID, mintLogoutPair(t, tokenService, userID, nil), authentity.RefreshSessionStatusActive)
	refreshRepo.seed(session.TokenHash, session)

	result, err := svc.RevokeSessionFamily(ctx, userID, uuid.New())
	if err != nil {
		t.Fatalf("RevokeSessionFamily unknown family should not error: %v", err)
	}
	if result == nil {
		t.Fatal("expected revoke result")
	}
	if result.RevokedSessionsCount != 0 {
		t.Fatalf("expected 0 revoked rows, got %d", result.RevokedSessionsCount)
	}
}

func ptrString(v string) *string {
	return &v
}

func ptrTime(v time.Time) *time.Time {
	return &v
}
