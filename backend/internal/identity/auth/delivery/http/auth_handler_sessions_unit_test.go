package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/identity/auth/application"
	authentity "github.com/labuda/backend/internal/identity/auth/entity"
	authrepo "github.com/labuda/backend/internal/identity/auth/infrastructure/repository"
	notificationentity "github.com/labuda/backend/internal/interaction/notification/entity"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

type sessionsUnitTx struct{}

func (sessionsUnitTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (sessionsUnitTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (sessionsUnitTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

func (sessionsUnitTx) Commit(ctx context.Context) error {
	return nil
}

func (sessionsUnitTx) Rollback(ctx context.Context) error {
	return nil
}

var _ db.Tx = sessionsUnitTx{}

type sessionsUnitTransactor struct {
	withTxCalls int
}

func (t *sessionsUnitTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	t.withTxCalls++
	return fn(sessionsUnitTx{})
}

var _ db.Transactor = (*sessionsUnitTransactor)(nil)

type sessionsUnitRefreshRepo struct {
	sessions map[string]*authentity.RefreshSession
}

func (r *sessionsUnitRefreshRepo) FindByTokenHash(ctx context.Context, tx db.Tx, tokenHash string) (*authentity.RefreshSession, error) {
	if session, ok := r.sessions[tokenHash]; ok {
		copySession := *session
		return &copySession, nil
	}
	return nil, authrepo.ErrSessionNotFound
}

func (r *sessionsUnitRefreshRepo) RevokeFamily(ctx context.Context, tx db.Tx, userID uuid.UUID, familyID uuid.UUID) error {
	return nil
}

func (r *sessionsUnitRefreshRepo) RevokeFamilyCount(ctx context.Context, tx db.Tx, userID uuid.UUID, familyID uuid.UUID) (int64, error) {
	var count int64
	for _, session := range r.sessions {
		if session != nil && session.UserID == userID && session.FamilyID == familyID && session.Status == authentity.RefreshSessionStatusActive {
			count++
			session.Status = authentity.RefreshSessionStatusRevoked
		}
	}
	return count, nil
}

func (r *sessionsUnitRefreshRepo) ListActiveByUser(ctx context.Context, tx db.Tx, userID uuid.UUID) ([]*authentity.RefreshSession, error) {
	sessions := make([]*authentity.RefreshSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		if session == nil || session.UserID != userID || session.Status != authentity.RefreshSessionStatusActive || !session.ExpiresAt.After(time.Now()) {
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

func (r *sessionsUnitRefreshRepo) RevokeAllForUserCount(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error) {
	return 0, nil
}

type sessionsUnitFCMRepo struct {
	tokens map[uuid.UUID][]*notificationentity.FCMToken
}

func (r *sessionsUnitFCMRepo) GetActiveTokensByUser(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*notificationentity.FCMToken, error) {
	return r.tokens[userID], nil
}

func (r *sessionsUnitFCMRepo) DeactivateByToken(ctx context.Context, tx interface{}, tokenString string) error {
	return nil
}

func (r *sessionsUnitFCMRepo) DeactivateByUserAndDevice(ctx context.Context, tx interface{}, userID uuid.UUID, deviceID string) error {
	return nil
}

func (r *sessionsUnitFCMRepo) DeactivateByUserAndDeviceCount(ctx context.Context, tx interface{}, userID uuid.UUID, deviceID string) (int64, error) {
	return 1, nil
}

func (r *sessionsUnitFCMRepo) DeactivateAllByUser(ctx context.Context, tx interface{}, userID uuid.UUID) (int64, error) {
	return 0, nil
}

func newSessionsUnitHandler(t *testing.T) (*AuthHandler, *sessionsUnitRefreshRepo, *sessionsUnitFCMRepo, *sessionsUnitTransactor) {
	t.Helper()
	log := zap.NewNop()
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	tokenService := application.NewTokenService(cfg, &logger.Logger{Logger: log})
	tx := &sessionsUnitTransactor{}
	refreshRepo := &sessionsUnitRefreshRepo{sessions: map[string]*authentity.RefreshSession{}}
	fcmRepo := &sessionsUnitFCMRepo{tokens: map[uuid.UUID][]*notificationentity.FCMToken{}}
	svc := application.NewLogoutCurrentSessionService(tx, tokenService, refreshRepo, fcmRepo, log)
	h := &AuthHandler{
		logoutService: svc,
		log:           log,
		jwtConfig:     cfg,
		tokenService:  tokenService,
	}
	return h, refreshRepo, fcmRepo, tx
}

func callSessionsListHandler(t *testing.T, h *AuthHandler, userID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	c.Request = req
	c.Set("user_id", userID)

	h.ListSessions(c)
	return w
}

func callSessionsRevokeHandler(t *testing.T, h *AuthHandler, userID uuid.UUID, familyID string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodDelete, "/api/v1/auth/sessions/"+familyID, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	c.Request = req
	c.Params = gin.Params{{Key: "family_id", Value: familyID}}
	c.Set("user_id", userID)

	h.RevokeSession(c)
	return w
}

func TestListSessionsHandler_ReturnsSafeDedupedSessions(t *testing.T) {
	h, refreshRepo, fcmRepo, _ := newSessionsUnitHandler(t)
	userID := uuid.New()
	familyA := uuid.New()
	familyB := uuid.New()

	older := authentity.RefreshSession{
		ID:         uuid.New(),
		UserID:     userID,
		FamilyID:   familyA,
		JTI:        uuid.New(),
		TokenHash:  authrepo.HashRefreshToken("older"),
		Status:     authentity.RefreshSessionStatusActive,
		IssuedAt:   time.Now().Add(-2 * time.Hour),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		DeviceID:   ptrStringLocal("device-a"),
		DeviceName: ptrStringLocal("Old device"),
		Platform:   ptrStringLocal("android"),
		AppVersion: ptrStringLocal("1.0.0"),
	}
	newer := authentity.RefreshSession{
		ID:         uuid.New(),
		UserID:     userID,
		FamilyID:   familyA,
		JTI:        uuid.New(),
		TokenHash:  authrepo.HashRefreshToken("newer"),
		Status:     authentity.RefreshSessionStatusActive,
		IssuedAt:   time.Now().Add(-time.Minute),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		DeviceID:   ptrStringLocal("device-a"),
		DeviceName: ptrStringLocal("New device"),
		Platform:   ptrStringLocal("android"),
		AppVersion: ptrStringLocal("1.0.1"),
	}
	other := authentity.RefreshSession{
		ID:         uuid.New(),
		UserID:     userID,
		FamilyID:   familyB,
		JTI:        uuid.New(),
		TokenHash:  authrepo.HashRefreshToken("other"),
		Status:     authentity.RefreshSessionStatusActive,
		IssuedAt:   time.Now().Add(-30 * time.Minute),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		DeviceID:   ptrStringLocal("device-b"),
		DeviceName: ptrStringLocal("Tablet"),
		Platform:   ptrStringLocal("ios"),
		AppVersion: ptrStringLocal("2.0.0"),
	}
	refreshRepo.sessions[older.TokenHash] = &older
	refreshRepo.sessions[newer.TokenHash] = &newer
	refreshRepo.sessions[other.TokenHash] = &other

	fcmRepo.tokens[userID] = []*notificationentity.FCMToken{
		{
			ID:         uuid.New(),
			UserID:     userID,
			Token:      "fcm-a",
			Platform:   notificationentity.FCMPlatformAndroid,
			DeviceID:   ptrStringLocal("device-a"),
			DeviceName: ptrStringLocal("Pixel"),
			AppVersion: ptrStringLocal("1.0.2"),
			IsActive:   true,
			LastUsedAt: ptrTimeLocal(time.Now().Add(-10 * time.Minute)),
		},
		{
			ID:         uuid.New(),
			UserID:     userID,
			Token:      "fcm-b",
			Platform:   notificationentity.FCMPlatformIOS,
			DeviceID:   ptrStringLocal("device-b"),
			DeviceName: ptrStringLocal("iPhone"),
			AppVersion: ptrStringLocal("2.0.1"),
			IsActive:   true,
			LastUsedAt: ptrTimeLocal(time.Now().Add(-5 * time.Minute)),
		},
	}

	w := callSessionsListHandler(t, h, userID)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected response data map, got %#v", resp.Data)
	}
	sessions, ok := data["sessions"].([]any)
	if !ok {
		t.Fatalf("expected sessions array, got %#v", data["sessions"])
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	first := sessions[0].(map[string]any)
	if first["family_id"] != familyA.String() {
		t.Fatalf("first family_id = %v, want %s", first["family_id"], familyA)
	}
	if first["device_name"] != "Pixel" {
		t.Fatalf("first device_name = %v, want Pixel", first["device_name"])
	}
	if _, ok := first["jti"]; ok {
		t.Fatal("jti must not be exposed")
	}
	if _, ok := first["token_hash"]; ok {
		t.Fatal("token_hash must not be exposed")
	}
	second := sessions[1].(map[string]any)
	if second["family_id"] != familyB.String() {
		t.Fatalf("second family_id = %v, want %s", second["family_id"], familyB)
	}
	if second["fcm_token_active"] != true {
		t.Fatalf("expected FCM active flag, got %#v", second["fcm_token_active"])
	}
}

func TestRevokeSessionHandler_ReturnsCountsAndScopesToFamily(t *testing.T) {
	h, refreshRepo, fcmRepo, _ := newSessionsUnitHandler(t)
	userID := uuid.New()
	familyA := uuid.New()
	familyB := uuid.New()

	first := authentity.RefreshSession{
		ID:        uuid.New(),
		UserID:    userID,
		FamilyID:  familyA,
		JTI:       uuid.New(),
		TokenHash: authrepo.HashRefreshToken("family-a-1"),
		Status:    authentity.RefreshSessionStatusActive,
		IssuedAt:  time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		DeviceID:  ptrStringLocal("device-a"),
	}
	second := authentity.RefreshSession{
		ID:        uuid.New(),
		UserID:    userID,
		FamilyID:  familyA,
		JTI:       uuid.New(),
		TokenHash: authrepo.HashRefreshToken("family-a-2"),
		Status:    authentity.RefreshSessionStatusActive,
		IssuedAt:  time.Now().Add(-time.Minute),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		DeviceID:  ptrStringLocal("device-a"),
	}
	otherFamily := authentity.RefreshSession{
		ID:        uuid.New(),
		UserID:    userID,
		FamilyID:  familyB,
		JTI:       uuid.New(),
		TokenHash: authrepo.HashRefreshToken("family-b"),
		Status:    authentity.RefreshSessionStatusActive,
		IssuedAt:  time.Now().Add(-30 * time.Minute),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		DeviceID:  ptrStringLocal("device-b"),
	}
	refreshRepo.sessions[first.TokenHash] = &first
	refreshRepo.sessions[second.TokenHash] = &second
	refreshRepo.sessions[otherFamily.TokenHash] = &otherFamily
	fcmRepo.tokens[userID] = []*notificationentity.FCMToken{
		{
			ID:         uuid.New(),
			UserID:     userID,
			Token:      "token-a",
			Platform:   notificationentity.FCMPlatformAndroid,
			DeviceID:   ptrStringLocal("device-a"),
			IsActive:   true,
			LastUsedAt: ptrTimeLocal(time.Now()),
		},
	}

	w := callSessionsRevokeHandler(t, h, userID, familyA.String())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data := resp.Data.(map[string]any)
	if data["revoked_sessions_count"] != float64(2) {
		t.Fatalf("revoked_sessions_count = %#v, want 2", data["revoked_sessions_count"])
	}
	if data["deactivated_fcm_tokens_count"] != float64(1) {
		t.Fatalf("deactivated_fcm_tokens_count = %#v, want 1", data["deactivated_fcm_tokens_count"])
	}
	if refreshRepo.sessions[first.TokenHash].Status != authentity.RefreshSessionStatusRevoked {
		t.Fatal("familyA first session should be revoked")
	}
	if refreshRepo.sessions[second.TokenHash].Status != authentity.RefreshSessionStatusRevoked {
		t.Fatal("familyA second session should be revoked")
	}
	if refreshRepo.sessions[otherFamily.TokenHash].Status != authentity.RefreshSessionStatusActive {
		t.Fatal("other family should remain active")
	}
}

func TestRevokeSessionHandler_InvalidFamilyReturns400(t *testing.T) {
	h, _, _, _ := newSessionsUnitHandler(t)

	w := callSessionsRevokeHandler(t, h, uuid.New(), "not-a-uuid")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func ptrStringLocal(v string) *string {
	return &v
}

func ptrTimeLocal(v time.Time) *time.Time {
	return &v
}


