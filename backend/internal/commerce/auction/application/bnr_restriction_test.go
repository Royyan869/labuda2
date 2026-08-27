package application

import (
	"testing"
	"time"
)

// =============================================================================
// BNR RESTRICTION EVALUATE TESTS
// =============================================================================
//
// Tests cover the owner-approved punishment table:
//   0 strikes → allow
//   1 strike  → allow + warning
//   2 strikes → deny if last_struck_at + 14d > now (within window)
//   2 strikes → allow if last_struck_at + 14d <= now (window expired)
//   3 strikes → deny if last_struck_at + 90d > now (within window)
//   3 strikes → allow if last_struck_at + 90d <= now (window expired)
//   4+ strikes → deny permanent

func TestBNRRestriction_ZeroStrikes_Allowed(t *testing.T) {
	c := NewBNRStrikeChecker()
	result := c.evaluate(0, nil, time.Now())

	if !result.Allowed {
		t.Fatal("0 strikes should be allowed")
	}
	if result.ActiveStrikes != 0 {
		t.Errorf("ActiveStrikes = %d, want 0", result.ActiveStrikes)
	}
	if result.Warning != "" {
		t.Errorf("Warning should be empty, got %q", result.Warning)
	}
	if result.PermanentBan {
		t.Error("PermanentBan should be false")
	}
	if result.RestrictionUntil != nil {
		t.Error("RestrictionUntil should be nil")
	}
}

func TestBNRRestriction_OneStrike_AllowedWithWarning(t *testing.T) {
	c := NewBNRStrikeChecker()
	struck := time.Now().Add(-1 * time.Hour)
	result := c.evaluate(1, &struck, time.Now())

	if !result.Allowed {
		t.Fatal("1 strike should be allowed")
	}
	if result.ActiveStrikes != 1 {
		t.Errorf("ActiveStrikes = %d, want 1", result.ActiveStrikes)
	}
	if result.Warning == "" {
		t.Error("1 strike should produce a warning")
	}
	if result.PermanentBan {
		t.Error("PermanentBan should be false")
	}
}

func TestBNRRestriction_TwoStrikes_WithinWindow_Blocked(t *testing.T) {
	c := NewBNRStrikeChecker()
	// Last strike was 5 days ago — within 14-day window
	struck := time.Now().Add(-5 * 24 * time.Hour)
	result := c.evaluate(2, &struck, time.Now())

	if result.Allowed {
		t.Fatal("2 strikes within 14d window should be blocked")
	}
	if result.ActiveStrikes != 2 {
		t.Errorf("ActiveStrikes = %d, want 2", result.ActiveStrikes)
	}
	if result.RestrictionUntil == nil {
		t.Fatal("RestrictionUntil should be set")
	}
	expectedUntil := struck.Add(14 * 24 * time.Hour)
	if !result.RestrictionUntil.Equal(expectedUntil) {
		t.Errorf("RestrictionUntil = %v, want %v", result.RestrictionUntil, expectedUntil)
	}
	if result.PermanentBan {
		t.Error("PermanentBan should be false for 2 strikes")
	}
}

func TestBNRRestriction_TwoStrikes_WindowExpired_Allowed(t *testing.T) {
	c := NewBNRStrikeChecker()
	// Last strike was 15 days ago — 14-day window expired
	struck := time.Now().Add(-15 * 24 * time.Hour)
	result := c.evaluate(2, &struck, time.Now())

	if !result.Allowed {
		t.Fatal("2 strikes after 14d window should be allowed")
	}
	if result.ActiveStrikes != 2 {
		t.Errorf("ActiveStrikes = %d, want 2", result.ActiveStrikes)
	}
	if result.Warning == "" {
		t.Error("should produce a warning after restriction expires")
	}
}

func TestBNRRestriction_ThreeStrikes_WithinWindow_Blocked(t *testing.T) {
	c := NewBNRStrikeChecker()
	// Last strike was 30 days ago — within 90-day window
	struck := time.Now().Add(-30 * 24 * time.Hour)
	result := c.evaluate(3, &struck, time.Now())

	if result.Allowed {
		t.Fatal("3 strikes within 90d window should be blocked")
	}
	if result.ActiveStrikes != 3 {
		t.Errorf("ActiveStrikes = %d, want 3", result.ActiveStrikes)
	}
	if result.RestrictionUntil == nil {
		t.Fatal("RestrictionUntil should be set")
	}
	expectedUntil := struck.Add(90 * 24 * time.Hour)
	if !result.RestrictionUntil.Equal(expectedUntil) {
		t.Errorf("RestrictionUntil = %v, want %v", result.RestrictionUntil, expectedUntil)
	}
	if result.PermanentBan {
		t.Error("PermanentBan should be false for 3 strikes")
	}
}

func TestBNRRestriction_ThreeStrikes_WindowExpired_Allowed(t *testing.T) {
	c := NewBNRStrikeChecker()
	// Last strike was 91 days ago — 90-day window expired
	struck := time.Now().Add(-91 * 24 * time.Hour)
	result := c.evaluate(3, &struck, time.Now())

	if !result.Allowed {
		t.Fatal("3 strikes after 90d window should be allowed")
	}
	if result.ActiveStrikes != 3 {
		t.Errorf("ActiveStrikes = %d, want 3", result.ActiveStrikes)
	}
	if result.Warning == "" {
		t.Error("should produce a warning after restriction expires")
	}
}

func TestBNRRestriction_FourStrikes_PermanentBan(t *testing.T) {
	c := NewBNRStrikeChecker()
	struck := time.Now().Add(-1 * time.Hour)
	result := c.evaluate(4, &struck, time.Now())

	if result.Allowed {
		t.Fatal("4 strikes should be permanently banned")
	}
	if result.ActiveStrikes != 4 {
		t.Errorf("ActiveStrikes = %d, want 4", result.ActiveStrikes)
	}
	if !result.PermanentBan {
		t.Error("PermanentBan should be true")
	}
	if result.RestrictionUntil != nil {
		t.Error("RestrictionUntil should be nil for permanent ban")
	}
}

func TestBNRRestriction_FiveStrikes_PermanentBan(t *testing.T) {
	c := NewBNRStrikeChecker()
	struck := time.Now().Add(-1000 * 24 * time.Hour) // even very old
	result := c.evaluate(5, &struck, time.Now())

	if result.Allowed {
		t.Fatal("5 strikes should be permanently banned regardless of time")
	}
	if !result.PermanentBan {
		t.Error("PermanentBan should be true")
	}
}

func TestBNRRestriction_TwoStrikes_ExactBoundary_Blocked(t *testing.T) {
	c := NewBNRStrikeChecker()
	// Exactly at the 14-day mark — time.Before returns false when equal,
	// so this should be allowed (boundary = restriction expired).
	now := time.Now()
	struck := now.Add(-14 * 24 * time.Hour)
	result := c.evaluate(2, &struck, now)

	// At exactly 14d, now.Before(until) = now.Before(now) = false → allowed
	if !result.Allowed {
		t.Fatal("2 strikes at exact 14d boundary should be allowed (window expired)")
	}
}

func TestBNRRestriction_ThreeStrikes_ExactBoundary_Blocked(t *testing.T) {
	c := NewBNRStrikeChecker()
	// Exactly at 90-day mark
	now := time.Now()
	struck := now.Add(-90 * 24 * time.Hour)
	result := c.evaluate(3, &struck, now)

	// At exactly 90d, now.Before(until) = false → allowed
	if !result.Allowed {
		t.Fatal("3 strikes at exact 90d boundary should be allowed (window expired)")
	}
}

func TestBNRRestriction_TwoStrikes_OneSecondBefore_Blocked(t *testing.T) {
	c := NewBNRStrikeChecker()
	// 1 second before the 14-day mark — should still be blocked
	now := time.Now()
	struck := now.Add(-14*24*time.Hour + 1*time.Second)
	result := c.evaluate(2, &struck, now)

	if result.Allowed {
		t.Fatal("2 strikes 1s before 14d boundary should still be blocked")
	}
}


