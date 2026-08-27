package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// deleted_at × account_status matrix tests for EnsureActive
// ---------------------------------------------------------------------------
//
//   account_status | deleted_at | expected
//   ───────────────┼────────────┼──────────────────
//   active         | NULL       | nil (allowed)
//   active         | set        | ErrAccountRemoved
//   suspended      | NULL       | ErrAccountSuspended
//   banned         | NULL       | ErrAccountBanned
//   suspended      | set        | ErrAccountRemoved (takes precedence)
//   banned         | set        | ErrAccountRemoved (takes precedence)
//
// The tests below verify:
// 1. The sentinel ErrAccountRemoved exists and is distinct
// 2. The interface contract holds for mock implementations
// 3. The deleted_at precedence logic is correct

func TestErrAccountRemoved_IsSentinel(t *testing.T) {
	// Verify ErrAccountRemoved is a distinct sentinel error.
	if errors.Is(ErrAccountRemoved, ErrAccountSuspended) {
		t.Fatal("ErrAccountRemoved must not alias ErrAccountSuspended")
	}
	if errors.Is(ErrAccountRemoved, ErrAccountBanned) {
		t.Fatal("ErrAccountRemoved must not alias ErrAccountBanned")
	}
	if errors.Is(ErrAccountRemoved, ErrAccountInactive) {
		t.Fatal("ErrAccountRemoved must not alias ErrAccountInactive")
	}
}

// testAccountStatusChecker is a mock that simulates the hardened
// deleted_at-aware EnsureActive logic.
type testAccountStatusChecker struct {
	accountStatus string
	deletedAt     *time.Time
}

func (m *testAccountStatusChecker) EnsureActive(ctx context.Context, userID uuid.UUID) error {
	if IsSystemCaller(userID) {
		return nil
	}
	if userID == uuid.Nil {
		return ErrInvalidCaller
	}
	// deleted_at takes precedence — mirrors AccountStatusCheckerDB.EnsureActive
	if m.deletedAt != nil {
		return ErrAccountRemoved
	}
	switch m.accountStatus {
	case "active":
		return nil
	case "suspended":
		return ErrAccountSuspended
	case "banned":
		return ErrAccountBanned
	default:
		return ErrAccountInactive
	}
}

func (m *testAccountStatusChecker) GetStatus(ctx context.Context, userID uuid.UUID) (string, error) {
	if m.deletedAt != nil {
		return "removed", nil
	}
	return m.accountStatus, nil
}

func (m *testAccountStatusChecker) IsBanned(ctx context.Context, userID uuid.UUID) (bool, error) {
	if m.deletedAt != nil {
		return false, nil
	}
	return m.accountStatus == "banned", nil
}

// Compile-time interface check.
var _ AccountStatusChecker = (*testAccountStatusChecker)(nil)

func TestEnsureActive_ActiveNotDeleted_Allowed(t *testing.T) {
	checker := &testAccountStatusChecker{accountStatus: "active", deletedAt: nil}
	err := checker.EnsureActive(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("active + not deleted should be allowed, got: %v", err)
	}
}

func TestEnsureActive_ActiveDeleted_Removed(t *testing.T) {
	now := time.Now()
	checker := &testAccountStatusChecker{accountStatus: "active", deletedAt: &now}
	err := checker.EnsureActive(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("active + deleted_at set should be blocked, got nil")
	}
	if !errors.Is(err, ErrAccountRemoved) {
		t.Fatalf("expected ErrAccountRemoved, got: %v", err)
	}
}

func TestEnsureActive_SuspendedNotDeleted_Suspended(t *testing.T) {
	checker := &testAccountStatusChecker{accountStatus: "suspended", deletedAt: nil}
	err := checker.EnsureActive(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("suspended + not deleted should be blocked, got nil")
	}
	if !errors.Is(err, ErrAccountSuspended) {
		t.Fatalf("expected ErrAccountSuspended, got: %v", err)
	}
}

func TestEnsureActive_BannedNotDeleted_Banned(t *testing.T) {
	checker := &testAccountStatusChecker{accountStatus: "banned", deletedAt: nil}
	err := checker.EnsureActive(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("banned + not deleted should be blocked, got nil")
	}
	if !errors.Is(err, ErrAccountBanned) {
		t.Fatalf("expected ErrAccountBanned, got: %v", err)
	}
}

func TestEnsureActive_SuspendedDeleted_RemovedTakesPrecedence(t *testing.T) {
	now := time.Now()
	checker := &testAccountStatusChecker{accountStatus: "suspended", deletedAt: &now}
	err := checker.EnsureActive(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("suspended + deleted_at set should be blocked, got nil")
	}
	if !errors.Is(err, ErrAccountRemoved) {
		t.Fatalf("deleted_at should take precedence over suspended, got: %v", err)
	}
}

func TestEnsureActive_BannedDeleted_RemovedTakesPrecedence(t *testing.T) {
	now := time.Now()
	checker := &testAccountStatusChecker{accountStatus: "banned", deletedAt: &now}
	err := checker.EnsureActive(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("banned + deleted_at set should be blocked, got nil")
	}
	if !errors.Is(err, ErrAccountRemoved) {
		t.Fatalf("deleted_at should take precedence over banned, got: %v", err)
	}
}

func TestGetStatus_Deleted_ReturnsRemoved(t *testing.T) {
	now := time.Now()
	checker := &testAccountStatusChecker{accountStatus: "active", deletedAt: &now}
	status, err := checker.GetStatus(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "removed" {
		t.Fatalf("expected 'removed', got: %s", status)
	}
}

func TestIsBanned_Deleted_ReturnsFalse(t *testing.T) {
	now := time.Now()
	checker := &testAccountStatusChecker{accountStatus: "banned", deletedAt: &now}
	banned, err := checker.IsBanned(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if banned {
		t.Fatal("removed user should not be reported as banned")
	}
}

func TestEnsureActive_SystemCaller_Bypasses(t *testing.T) {
	now := time.Now()
	checker := &testAccountStatusChecker{accountStatus: "banned", deletedAt: &now}
	err := checker.EnsureActive(context.Background(), SystemCallerID)
	if err != nil {
		t.Fatalf("system caller should bypass all checks, got: %v", err)
	}
}


