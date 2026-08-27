package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/admin/repository"
	"github.com/labuda/backend/internal/platform/capability"
	"github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
)

// ---------------------------------------------------------------------------
// BAN PERMANENCE ALIGNMENT TESTS
// ---------------------------------------------------------------------------
//
// Matrix under test:
//
//   operation       | current_status | expected
//   ────────────────┼────────────────┼──────────────────────────
//   ActivateUser    | suspended      | PASS → active
//   ActivateUser    | banned         | BLOCKED (ErrCannotActivateBannedUser)
//   UnbanUser       | banned         | PASS → active
//   UnbanUser       | suspended      | BLOCKED (ErrCannotUnbanNonBannedUser)
//   UnbanUser       | active         | BLOCKED (ErrCannotUnbanNonBannedUser)
//   BanUser         | active         | PASS → banned (existing behavior)
//   BanUser         | banned         | PASS (idempotent noop)

// --- mocks ---

type mockAdminRepo struct {
	userStatus    string
	updateCalled  bool
	updatedStatus string
}

func (m *mockAdminRepo) GetUserStatus(ctx context.Context, tx interface{}, userID uuid.UUID) (string, error) {
	return m.userStatus, nil
}

func (m *mockAdminRepo) UpdateUserStatus(ctx context.Context, tx interface{}, userID uuid.UUID, status string) error {
	m.updateCalled = true
	m.updatedStatus = status
	return nil
}

func (m *mockAdminRepo) ListUsers(ctx context.Context, tx interface{}, filters repository.UserListFilters) ([]repository.UserSummary, error) {
	return nil, nil
}
func (m *mockAdminRepo) CountUsers(ctx context.Context, tx interface{}, filters repository.UserListFilters) (int, error) {
	return 0, nil
}
func (m *mockAdminRepo) GetUserDetails(ctx context.Context, tx interface{}, userID uuid.UUID) (*repository.UserDetails, error) {
	return nil, nil
}
func (m *mockAdminRepo) GetDashboardMetrics(ctx context.Context, tx interface{}) (*repository.DashboardMetrics, error) {
	return nil, nil
}
func (m *mockAdminRepo) ListAuditLogs(ctx context.Context, tx interface{}, filters repository.AuditLogFilters) ([]repository.AuditLogEntry, error) {
	return nil, nil
}
func (m *mockAdminRepo) CountAuditLogs(ctx context.Context, tx interface{}, filters repository.AuditLogFilters) (int, error) {
	return 0, nil
}

// Compile-time check: mockAdminRepo satisfies AdminRepository
var _ repository.AdminRepository = (*mockAdminRepo)(nil)

type mockAdminDB struct{}

func (m *mockAdminDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(nil)
}

type mockAuditLogger struct {
	lastAction string
}

func (m *mockAuditLogger) Log(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	m.lastAction = actionType
	return nil
}

func (m *mockAuditLogger) LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) {
	m.lastAction = actionType
}

func (m *mockAuditLogger) LogTx(ctx context.Context, tx db.Tx, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	m.lastAction = actionType
	return nil
}

// capabilityContext injects an Actor with the given capability into context.
func capabilityContext(cap string) context.Context {
	actor := &entity.Actor{
		ID:           uuid.New(),
		Role:         "admin",
		Capabilities: []string{cap},
	}
	return capability.WithActor(context.Background(), actor)
}

// --- tests ---

func TestActivateUser_Suspended_Passes(t *testing.T) {
	repo := &mockAdminRepo{userStatus: "suspended"}
	audit := &mockAuditLogger{}
	svc := NewAdminService(&mockAdminDB{}, repo, audit, nil)

	ctx := capabilityContext(capability.CapGovernanceUserActivate.String())
	err := svc.ActivateUser(ctx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("activate suspended should pass, got: %v", err)
	}
	if !repo.updateCalled || repo.updatedStatus != "active" {
		t.Fatal("expected repo.UpdateUserStatus to be called with 'active'")
	}
	if audit.lastAction != "user_activated" {
		t.Fatalf("expected audit action 'user_activated', got: %s", audit.lastAction)
	}
}

func TestActivateUser_Banned_Blocked(t *testing.T) {
	repo := &mockAdminRepo{userStatus: "banned"}
	audit := &mockAuditLogger{}
	svc := NewAdminService(&mockAdminDB{}, repo, audit, nil)

	ctx := capabilityContext(capability.CapGovernanceUserActivate.String())
	err := svc.ActivateUser(ctx, uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("activate banned should be blocked, got nil")
	}
	var bannedErr *ErrCannotActivateBannedUser
	if !errors.As(err, &bannedErr) {
		t.Fatalf("expected ErrCannotActivateBannedUser, got: %v", err)
	}
	if repo.updateCalled {
		t.Fatal("repo.UpdateUserStatus should NOT be called for banned user")
	}
}

func TestUnbanUser_Banned_Passes(t *testing.T) {
	repo := &mockAdminRepo{userStatus: "banned"}
	audit := &mockAuditLogger{}
	svc := NewAdminService(&mockAdminDB{}, repo, audit, nil)

	ctx := capabilityContext(capability.CapGovernanceUserUnban.String())
	err := svc.UnbanUser(ctx, uuid.New(), uuid.New(), UnbanUserRequest{Reason: "appeal approved"})
	if err != nil {
		t.Fatalf("unban banned should pass, got: %v", err)
	}
	if !repo.updateCalled || repo.updatedStatus != "active" {
		t.Fatal("expected repo.UpdateUserStatus to be called with 'active'")
	}
	if audit.lastAction != "user_unbanned" {
		t.Fatalf("expected audit action 'user_unbanned', got: %s", audit.lastAction)
	}
}

func TestUnbanUser_Suspended_Blocked(t *testing.T) {
	repo := &mockAdminRepo{userStatus: "suspended"}
	audit := &mockAuditLogger{}
	svc := NewAdminService(&mockAdminDB{}, repo, audit, nil)

	ctx := capabilityContext(capability.CapGovernanceUserUnban.String())
	err := svc.UnbanUser(ctx, uuid.New(), uuid.New(), UnbanUserRequest{Reason: "test"})
	if err == nil {
		t.Fatal("unban suspended should be blocked, got nil")
	}
	var notBannedErr *ErrCannotUnbanNonBannedUser
	if !errors.As(err, &notBannedErr) {
		t.Fatalf("expected ErrCannotUnbanNonBannedUser, got: %v", err)
	}
	if repo.updateCalled {
		t.Fatal("repo.UpdateUserStatus should NOT be called for non-banned user")
	}
}

func TestUnbanUser_Active_Blocked(t *testing.T) {
	repo := &mockAdminRepo{userStatus: "active"}
	audit := &mockAuditLogger{}
	svc := NewAdminService(&mockAdminDB{}, repo, audit, nil)

	ctx := capabilityContext(capability.CapGovernanceUserUnban.String())
	err := svc.UnbanUser(ctx, uuid.New(), uuid.New(), UnbanUserRequest{Reason: "test"})
	if err == nil {
		t.Fatal("unban active should be blocked, got nil")
	}
	var notBannedErr *ErrCannotUnbanNonBannedUser
	if !errors.As(err, &notBannedErr) {
		t.Fatalf("expected ErrCannotUnbanNonBannedUser, got: %v", err)
	}
}

func TestUnbanUser_MissingCapability_Forbidden(t *testing.T) {
	repo := &mockAdminRepo{userStatus: "banned"}
	audit := &mockAuditLogger{}
	svc := NewAdminService(&mockAdminDB{}, repo, audit, nil)

	// Context with activate capability but NOT unban
	ctx := capabilityContext(capability.CapGovernanceUserActivate.String())
	err := svc.UnbanUser(ctx, uuid.New(), uuid.New(), UnbanUserRequest{Reason: "test"})
	if err == nil {
		t.Fatal("unban without capability should be forbidden, got nil")
	}
	if repo.updateCalled {
		t.Fatal("repo.UpdateUserStatus should NOT be called without unban capability")
	}
}

func TestBanUser_Active_Passes(t *testing.T) {
	repo := &mockAdminRepo{userStatus: "active"}
	audit := &mockAuditLogger{}
	svc := NewAdminService(&mockAdminDB{}, repo, audit, nil)

	actorID := uuid.New()
	targetID := uuid.New()
	ctx := capabilityContext(capability.CapGovernanceUserBan.String())
	err := svc.BanUser(ctx, actorID, targetID, BanUserRequest{Reason: "violation"})
	if err != nil {
		t.Fatalf("ban active should pass, got: %v", err)
	}
	if !repo.updateCalled || repo.updatedStatus != "banned" {
		t.Fatal("expected repo.UpdateUserStatus to be called with 'banned'")
	}
	if audit.lastAction != "user_banned" {
		t.Fatalf("expected audit action 'user_banned', got: %s", audit.lastAction)
	}
}

func TestBanUser_AlreadyBanned_Idempotent(t *testing.T) {
	repo := &mockAdminRepo{userStatus: "banned"}
	audit := &mockAuditLogger{}
	svc := NewAdminService(&mockAdminDB{}, repo, audit, nil)

	actorID := uuid.New()
	targetID := uuid.New()
	ctx := capabilityContext(capability.CapGovernanceUserBan.String())
	err := svc.BanUser(ctx, actorID, targetID, BanUserRequest{Reason: "re-ban"})
	if err != nil {
		t.Fatalf("ban already-banned should be idempotent, got: %v", err)
	}
	if repo.updateCalled {
		t.Fatal("already-banned should not trigger update (idempotent)")
	}
}

func TestCapGovernanceUserUnban_IsValid(t *testing.T) {
	if !capability.IsValid(capability.CapGovernanceUserUnban.String()) {
		t.Fatal("CapGovernanceUserUnban should be a valid capability")
	}
}

func TestUserBannedTransitionIdempotencyKey_DistinctPerTransition(t *testing.T) {
	targetUserID := uuid.New()
	transitionA := uuid.New()
	transitionB := uuid.New()

	keyA := userBannedTransitionIdempotencyKey(targetUserID, transitionA)
	keyB := userBannedTransitionIdempotencyKey(targetUserID, transitionB)

	if keyA == keyB {
		t.Fatalf("expected distinct keys per transition, got same key: %s", keyA)
	}
}

func TestUserBannedTransitionIdempotencyKey_DeterministicForSameInputs(t *testing.T) {
	targetUserID := uuid.New()
	transitionID := uuid.New()

	key1 := userBannedTransitionIdempotencyKey(targetUserID, transitionID)
	key2 := userBannedTransitionIdempotencyKey(targetUserID, transitionID)

	if key1 != key2 {
		t.Fatalf("expected deterministic key, got %s and %s", key1, key2)
	}
}


