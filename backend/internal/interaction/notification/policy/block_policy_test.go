package policy

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// --- test doubles ---

type stubBlockChecker struct {
	blocked bool
	err     error
}

func (s *stubBlockChecker) ExistsBlock(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return s.blocked, s.err
}

// --- A: ShouldApplyBlock reaches the checker and returns correct result ---

func TestShouldApplyBlock_NotBlocked_Delivers(t *testing.T) {
	policy := NewBlockPolicy(&stubBlockChecker{blocked: false})
	action := policy.ShouldApplyBlock(context.Background(), uuid.New(), uuid.New(), Social)

	if !action.Deliver {
		t.Errorf("Deliver = false, want true (no block exists)")
	}
	if action.Anonymize {
		t.Errorf("Anonymize = true, want false (no block exists)")
	}
	if action.Reason != "no_block" {
		t.Errorf("Reason = %q, want %q", action.Reason, "no_block")
	}
}

func TestShouldApplyBlock_Blocked_Social_Drops(t *testing.T) {
	policy := NewBlockPolicy(&stubBlockChecker{blocked: true})
	action := policy.ShouldApplyBlock(context.Background(), uuid.New(), uuid.New(), Social)

	if action.Deliver {
		t.Errorf("Deliver = true, want false (social must respect block)")
	}
	if action.Anonymize {
		t.Errorf("Anonymize = true, want false (drop, not anonymize)")
	}
}

func TestShouldApplyBlock_Blocked_CommerceCritical_AnonymizesNotDrops(t *testing.T) {
	policy := NewBlockPolicy(&stubBlockChecker{blocked: true})
	action := policy.ShouldApplyBlock(context.Background(), uuid.New(), uuid.New(), CommerceCritical)

	if !action.Deliver {
		t.Errorf("Deliver = false, want true (commerce bypasses block)")
	}
	if !action.Anonymize {
		t.Errorf("Anonymize = false, want true (commerce must anonymize on block)")
	}
	if action.Reason != "commerce_bypass_block_anonymized" {
		t.Errorf("Reason = %q, want %q", action.Reason, "commerce_bypass_block_anonymized")
	}
}

func TestShouldApplyBlock_Blocked_Moderation_AnonymizesNotDrops(t *testing.T) {
	policy := NewBlockPolicy(&stubBlockChecker{blocked: true})
	action := policy.ShouldApplyBlock(context.Background(), uuid.New(), uuid.New(), Moderation)

	if !action.Deliver {
		t.Errorf("Deliver = false, want true (moderation bypasses block)")
	}
	if !action.Anonymize {
		t.Errorf("Anonymize = false, want true (moderation must anonymize on block)")
	}
	if action.ActorDisplay != "Admin" {
		t.Errorf("ActorDisplay = %q, want %q", action.ActorDisplay, "Admin")
	}
}

// --- B: nil checker path ---

func TestShouldApplyBlock_NilChecker_Delivers(t *testing.T) {
	policy := NewBlockPolicy(nil)
	action := policy.ShouldApplyBlock(context.Background(), uuid.New(), uuid.New(), Social)

	if !action.Deliver {
		t.Errorf("Deliver = false, want true (nil checker = no filtering)")
	}
	if action.Reason != "no_block_checker" {
		t.Errorf("Reason = %q, want %q", action.Reason, "no_block_checker")
	}
}

// --- C: error paths — no "invalid transaction type" possible since tx is gone ---

func TestShouldApplyBlock_CheckerError_Social_FailClosed(t *testing.T) {
	policy := NewBlockPolicy(&stubBlockChecker{err: fmt.Errorf("db connection failed")})
	action := policy.ShouldApplyBlock(context.Background(), uuid.New(), uuid.New(), Social)

	if action.Deliver {
		t.Errorf("Deliver = true, want false (social fail-closed on error)")
	}
	if action.Reason == "" {
		t.Error("Reason must be non-empty on error path")
	}
}

func TestShouldApplyBlock_CheckerError_Commerce_FailOpenAnonymized(t *testing.T) {
	policy := NewBlockPolicy(&stubBlockChecker{err: fmt.Errorf("db connection failed")})
	action := policy.ShouldApplyBlock(context.Background(), uuid.New(), uuid.New(), CommerceCritical)

	if !action.Deliver {
		t.Errorf("Deliver = false, want true (commerce fail-open on error)")
	}
	if !action.Anonymize {
		t.Errorf("Anonymize = false, want true (safest option on error)")
	}
}

// --- D: regression — "invalid transaction type" must never appear in reason ---

func TestShouldApplyBlock_ReasonNeverContainsInvalidTxType(t *testing.T) {
	cases := []struct {
		name     string
		blocked  bool
		err      error
		category NotificationCategory
	}{
		{"not_blocked_social", false, nil, Social},
		{"not_blocked_commerce", false, nil, CommerceCritical},
		{"blocked_social", true, nil, Social},
		{"blocked_commerce", true, nil, CommerceCritical},
		{"error_social", false, fmt.Errorf("some db error"), Social},
		{"error_commerce", false, fmt.Errorf("some db error"), CommerceCritical},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewBlockPolicy(&stubBlockChecker{blocked: tc.blocked, err: tc.err})
			action := p.ShouldApplyBlock(context.Background(), uuid.New(), uuid.New(), tc.category)
			if action.Reason == "invalid transaction type" {
				t.Errorf("Reason = %q: nil-tx bug still present", action.Reason)
			}
		})
	}
}


