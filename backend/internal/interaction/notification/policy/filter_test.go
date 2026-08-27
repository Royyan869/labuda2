package policy

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// stubStatusChecker implements AccountStatusChecker for filter tests.
type stubStatusChecker struct {
	status string
	err    error
}

func (s *stubStatusChecker) GetStatus(_ context.Context, _ uuid.UUID) (string, error) {
	return s.status, s.err
}

// --- Active ---

func TestShouldDeliver_Active_CommerceCritical_AllowsAll(t *testing.T) {
	f := NewAccountStatusFilter(&stubStatusChecker{status: "active"})
	d := f.ShouldDeliver(context.Background(), uuid.New(), CommerceCritical, "order.completed")
	if !d.AllowDB {
		t.Error("AllowDB = false, want true for active user")
	}
}

// --- Suspended ---

func TestShouldDeliver_Suspended_CommerceCritical_AllowsDB(t *testing.T) {
	f := NewAccountStatusFilter(&stubStatusChecker{status: "suspended"})
	d := f.ShouldDeliver(context.Background(), uuid.New(), CommerceCritical, "order.completed")
	if !d.AllowDB {
		t.Error("AllowDB = false, want true for suspended+CommerceCritical")
	}
}

func TestShouldDeliver_Suspended_Social_BlocksAll(t *testing.T) {
	f := NewAccountStatusFilter(&stubStatusChecker{status: "suspended"})
	d := f.ShouldDeliver(context.Background(), uuid.New(), Social, "content.liked")
	if d.AllowDB || d.AllowPush {
		t.Errorf("AllowDB=%v AllowPush=%v, want both false for suspended+Social", d.AllowDB, d.AllowPush)
	}
}

// --- Banned ---

func TestShouldDeliver_Banned_CommerceCritical_AllowsDB(t *testing.T) {
	f := NewAccountStatusFilter(&stubStatusChecker{status: "banned"})
	d := f.ShouldDeliver(context.Background(), uuid.New(), CommerceCritical, "withdrawal.completed")
	if !d.AllowDB {
		t.Error("AllowDB = false, want true for banned+CommerceCritical")
	}
}

func TestShouldDeliver_Banned_Marketing_BlocksAll(t *testing.T) {
	f := NewAccountStatusFilter(&stubStatusChecker{status: "banned"})
	d := f.ShouldDeliver(context.Background(), uuid.New(), Marketing, "promo.offer")
	if d.AllowDB || d.AllowPush {
		t.Errorf("AllowDB=%v AllowPush=%v, want both false for banned+Marketing", d.AllowDB, d.AllowPush)
	}
}

// --- Removed (soft-deleted) — L-8 regression lock ---

func TestShouldDeliver_Removed_CommerceCritical_BlocksAll(t *testing.T) {
	f := NewAccountStatusFilter(&stubStatusChecker{status: "removed"})
	d := f.ShouldDeliver(context.Background(), uuid.New(), CommerceCritical, "order.completed")
	if d.AllowDB || d.AllowPush {
		t.Errorf("AllowDB=%v AllowPush=%v, want both false for removed+CommerceCritical", d.AllowDB, d.AllowPush)
	}
	if d.Reason != "deleted_user" {
		t.Errorf("Reason = %q, want %q", d.Reason, "deleted_user")
	}
}

func TestShouldDeliver_Removed_Social_BlocksAll(t *testing.T) {
	f := NewAccountStatusFilter(&stubStatusChecker{status: "removed"})
	d := f.ShouldDeliver(context.Background(), uuid.New(), Social, "content.liked")
	if d.AllowDB || d.AllowPush {
		t.Errorf("AllowDB=%v AllowPush=%v, want both false for removed+Social", d.AllowDB, d.AllowPush)
	}
}

func TestShouldDeliver_Removed_Moderation_BlocksAll(t *testing.T) {
	f := NewAccountStatusFilter(&stubStatusChecker{status: "removed"})
	d := f.ShouldDeliver(context.Background(), uuid.New(), Moderation, "moderation.banned")
	if d.AllowDB || d.AllowPush {
		t.Errorf("AllowDB=%v AllowPush=%v, want both false for removed+Moderation", d.AllowDB, d.AllowPush)
	}
}

// --- Error path ---

func TestShouldDeliver_StatusError_FailSafeDBOnly(t *testing.T) {
	f := NewAccountStatusFilter(&stubStatusChecker{err: context.DeadlineExceeded})
	d := f.ShouldDeliver(context.Background(), uuid.New(), CommerceCritical, "order.completed")
	if !d.AllowDB {
		t.Error("AllowDB = false on error, want true (fail-safe DB only)")
	}
	if d.AllowPush {
		t.Error("AllowPush = true on error, want false (fail-safe blocks push)")
	}
}


