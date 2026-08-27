package realtime_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/realtime"
	"go.uber.org/zap"
)

// =============================================================================
// MOCK: AccountStatusChecker for unit tests
// =============================================================================

type mockStatusChecker struct {
	status string
	err    error
}

func (m *mockStatusChecker) EnsureActive(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockStatusChecker) GetStatus(_ context.Context, _ uuid.UUID) (string, error) {
	return m.status, m.err
}

func (m *mockStatusChecker) IsBanned(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.status == "banned", nil
}

// =============================================================================
// MOCK: RoomAuthorizer for SubscribeGate construction
// =============================================================================

type mockRoomAuthorizer struct {
	allowed bool
}

func (m *mockRoomAuthorizer) CanSubscribeToRoom(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ realtime.RoomType) bool {
	return m.allowed
}

// =============================================================================
// IsAlive TESTS
// =============================================================================

func TestIsAlive_ActiveUser(t *testing.T) {
	gate := realtime.NewSubscribeGate(
		&mockRoomAuthorizer{allowed: true},
		&mockStatusChecker{status: "active"},
		zap.NewNop(),
	)

	if !gate.IsAlive(context.Background(), uuid.New()) {
		t.Error("active user: IsAlive() = false; want true")
	}
}

func TestIsAlive_RemovedUser(t *testing.T) {
	gate := realtime.NewSubscribeGate(
		&mockRoomAuthorizer{allowed: true},
		&mockStatusChecker{status: "removed"},
		zap.NewNop(),
	)

	if gate.IsAlive(context.Background(), uuid.New()) {
		t.Error("removed user: IsAlive() = true; want false — stale socket must be evicted")
	}
}

func TestIsAlive_SuspendedUser(t *testing.T) {
	gate := realtime.NewSubscribeGate(
		&mockRoomAuthorizer{allowed: true},
		&mockStatusChecker{status: "suspended"},
		zap.NewNop(),
	)

	if gate.IsAlive(context.Background(), uuid.New()) {
		t.Error("suspended user: IsAlive() = true; want false")
	}
}

func TestIsAlive_BannedUser(t *testing.T) {
	gate := realtime.NewSubscribeGate(
		&mockRoomAuthorizer{allowed: true},
		&mockStatusChecker{status: "banned"},
		zap.NewNop(),
	)

	if gate.IsAlive(context.Background(), uuid.New()) {
		t.Error("banned user: IsAlive() = true; want false")
	}
}

func TestIsAlive_DBError_FailClosed(t *testing.T) {
	gate := realtime.NewSubscribeGate(
		&mockRoomAuthorizer{allowed: true},
		&mockStatusChecker{status: "", err: fmt.Errorf("connection refused")},
		zap.NewNop(),
	)

	if gate.IsAlive(context.Background(), uuid.New()) {
		t.Error("DB error: IsAlive() = true; want false (fail-closed)")
	}
}


