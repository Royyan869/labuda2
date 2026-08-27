package entity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/verification/entity"
)

func TestNewSellerVerification(t *testing.T) {
	sellerID := uuid.New()
	v := entity.NewSellerVerification(sellerID)

	if v.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}
	if v.SellerID != sellerID {
		t.Errorf("expected SellerID %s, got %s", sellerID, v.SellerID)
	}
	if v.Status != entity.StatusNotSubmitted {
		t.Errorf("expected status %s, got %s", entity.StatusNotSubmitted, v.Status)
	}
	if v.SubmittedAt != nil {
		t.Error("expected SubmittedAt to be nil")
	}
	if v.ReviewedAt != nil {
		t.Error("expected ReviewedAt to be nil")
	}
	if v.ReviewedBy != nil {
		t.Error("expected ReviewedBy to be nil")
	}
	if v.Reason != nil {
		t.Error("expected Reason to be nil")
	}
}

func TestSellerVerification_Submit(t *testing.T) {
	t.Run("submit from not_submitted succeeds", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		err := v.Submit()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Status != entity.StatusPendingReview {
			t.Errorf("expected status %s, got %s", entity.StatusPendingReview, v.Status)
		}
		if v.SubmittedAt == nil {
			t.Error("expected SubmittedAt to be set")
		}
	})

	t.Run("submit from rejected succeeds (resubmit)", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		adminID := uuid.New()
		reason := "invalid document"

		// First submit and reject
		_ = v.Submit()
		_ = v.Reject(adminID, reason)

		// Resubmit
		err := v.Submit()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Status != entity.StatusPendingReview {
			t.Errorf("expected status %s, got %s", entity.StatusPendingReview, v.Status)
		}
		if v.ReviewedAt != nil {
			t.Error("expected ReviewedAt to be cleared on resubmit")
		}
		if v.ReviewedBy != nil {
			t.Error("expected ReviewedBy to be cleared on resubmit")
		}
		if v.Reason != nil {
			t.Error("expected Reason to be cleared on resubmit")
		}
	})

	t.Run("submit from pending fails", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		_ = v.Submit()

		err := v.Submit()
		if err == nil {
			t.Error("expected error, got nil")
		}
		if _, ok := err.(*entity.InvalidTransitionError); !ok {
			t.Errorf("expected InvalidTransitionError, got %T", err)
		}
	})

	t.Run("submit from approved fails", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		adminID := uuid.New()
		_ = v.Submit()
		_ = v.Approve(adminID)

		err := v.Submit()
		if err == nil {
			t.Error("expected error, got nil")
		}
		if _, ok := err.(*entity.InvalidTransitionError); !ok {
			t.Errorf("expected InvalidTransitionError, got %T", err)
		}
	})
}

func TestSellerVerification_Approve(t *testing.T) {
	t.Run("approve from pending succeeds", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		_ = v.Submit()

		adminID := uuid.New()
		err := v.Approve(adminID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Status != entity.StatusApproved {
			t.Errorf("expected status %s, got %s", entity.StatusApproved, v.Status)
		}
		if v.ReviewedAt == nil {
			t.Error("expected ReviewedAt to be set")
		}
		if v.ReviewedBy == nil {
			t.Error("expected ReviewedBy to be set")
		}
		if *v.ReviewedBy != adminID {
			t.Errorf("expected ReviewedBy %s, got %s", adminID, *v.ReviewedBy)
		}
		if v.Reason != nil {
			t.Error("expected Reason to be nil")
		}
	})

	t.Run("approve from not_submitted fails", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		adminID := uuid.New()

		err := v.Approve(adminID)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if _, ok := err.(*entity.WrongStatusError); !ok {
			t.Errorf("expected WrongStatusError, got %T", err)
		}
	})

	t.Run("approve from rejected fails", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		adminID := uuid.New()
		_ = v.Submit()
		_ = v.Reject(adminID, "invalid")

		err := v.Approve(adminID)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if _, ok := err.(*entity.WrongStatusError); !ok {
			t.Errorf("expected WrongStatusError, got %T", err)
		}
	})

	t.Run("approve already approved returns WrongStatusError", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		adminID := uuid.New()
		_ = v.Submit()
		_ = v.Approve(adminID)

		err := v.Approve(adminID)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if _, ok := err.(*entity.WrongStatusError); !ok {
			t.Errorf("expected WrongStatusError, got %T", err)
		}
	})
}

func TestSellerVerification_Reject(t *testing.T) {
	t.Run("reject from pending succeeds", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		_ = v.Submit()

		adminID := uuid.New()
		reason := "document unclear"
		err := v.Reject(adminID, reason)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Status != entity.StatusRejected {
			t.Errorf("expected status %s, got %s", entity.StatusRejected, v.Status)
		}
		if v.ReviewedAt == nil {
			t.Error("expected ReviewedAt to be set")
		}
		if v.ReviewedBy == nil {
			t.Error("expected ReviewedBy to be set")
		}
		if v.Reason == nil {
			t.Error("expected Reason to be set")
		}
		if *v.Reason != reason {
			t.Errorf("expected Reason %s, got %s", reason, *v.Reason)
		}
	})

	t.Run("reject from not_submitted fails", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		adminID := uuid.New()

		err := v.Reject(adminID, "no submission")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if _, ok := err.(*entity.WrongStatusError); !ok {
			t.Errorf("expected WrongStatusError, got %T", err)
		}
	})

	t.Run("reject from approved fails", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		adminID := uuid.New()
		_ = v.Submit()
		_ = v.Approve(adminID)

		err := v.Reject(adminID, "should not reach here")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if _, ok := err.(*entity.WrongStatusError); !ok {
			t.Errorf("expected WrongStatusError, got %T", err)
		}
	})
}

func TestSellerVerification_IsApproved(t *testing.T) {
	t.Run("not_submitted is not approved", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		if v.IsApproved() {
			t.Error("expected IsApproved to return false")
		}
	})

	t.Run("pending is not approved", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		_ = v.Submit()
		if v.IsApproved() {
			t.Error("expected IsApproved to return false")
		}
	})

	t.Run("rejected is not approved", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		_ = v.Submit()
		_ = v.Reject(uuid.New(), "invalid")
		if v.IsApproved() {
			t.Error("expected IsApproved to return false")
		}
	})

	t.Run("approved is approved", func(t *testing.T) {
		v := entity.NewSellerVerification(uuid.New())
		_ = v.Submit()
		_ = v.Approve(uuid.New())
		if !v.IsApproved() {
			t.Error("expected IsApproved to return true")
		}
	})
}

func TestSellerVerification_CanTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     entity.Status
		to       entity.Status
		expected bool
	}{
		{"not_submitted -> pending_review", entity.StatusNotSubmitted, entity.StatusPendingReview, true},
		{"not_submitted -> approved", entity.StatusNotSubmitted, entity.StatusApproved, false},
		{"not_submitted -> rejected", entity.StatusNotSubmitted, entity.StatusRejected, false},
		{"pending_review -> approved", entity.StatusPendingReview, entity.StatusApproved, true},
		{"pending_review -> rejected", entity.StatusPendingReview, entity.StatusRejected, true},
		{"pending_review -> not_submitted", entity.StatusPendingReview, entity.StatusNotSubmitted, false},
		{"rejected -> pending_review", entity.StatusRejected, entity.StatusPendingReview, true},
		{"rejected -> approved", entity.StatusRejected, entity.StatusApproved, false},
		{"approved -> suspended", entity.StatusApproved, entity.StatusSuspended, true},
		{"approved -> revoked", entity.StatusApproved, entity.StatusRevoked, true},
		{"approved -> under_investigation", entity.StatusApproved, entity.StatusUnderInvestigation, true},
		{"approved -> not_submitted", entity.StatusApproved, entity.StatusNotSubmitted, false},
		{"suspended -> approved", entity.StatusSuspended, entity.StatusApproved, true},
		{"suspended -> revoked", entity.StatusSuspended, entity.StatusRevoked, false},
		{"revoked -> approved", entity.StatusRevoked, entity.StatusApproved, false},
		{"under_investigation -> approved", entity.StatusUnderInvestigation, entity.StatusApproved, true},
		{"under_investigation -> suspended", entity.StatusUnderInvestigation, entity.StatusSuspended, true},
		{"under_investigation -> revoked", entity.StatusUnderInvestigation, entity.StatusRevoked, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := entity.CanTransition(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestSellerVerification_FullLifecycle(t *testing.T) {
	sellerID := uuid.New()
	v := entity.NewSellerVerification(sellerID)

	// Initial state
	if v.Status != entity.StatusNotSubmitted {
		t.Errorf("expected not_submitted, got %s", v.Status)
	}

	// Submit
	if err := v.Submit(); err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if v.Status != entity.StatusPendingReview {
		t.Errorf("expected pending_review, got %s", v.Status)
	}

	// Reject
	adminID := uuid.New()
	if err := v.Reject(adminID, "test rejection"); err != nil {
		t.Fatalf("reject failed: %v", err)
	}
	if v.Status != entity.StatusRejected {
		t.Errorf("expected rejected, got %s", v.Status)
	}

	// Resubmit
	if err := v.Submit(); err != nil {
		t.Fatalf("resubmit failed: %v", err)
	}
	if v.Status != entity.StatusPendingReview {
		t.Errorf("expected pending_review after resubmit, got %s", v.Status)
	}

	// Approve
	if err := v.Approve(adminID); err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if v.Status != entity.StatusApproved {
		t.Errorf("expected approved, got %s", v.Status)
	}

	// Approved is NOT terminal in current model — can be suspended/revoked
	// but cannot be rejected directly (wrong transition)
	if err := v.Reject(adminID, "cannot reject approved"); err == nil {
		t.Error("expected error when rejecting approved seller")
	}

	if !v.IsApproved() {
		t.Error("expected IsApproved to be true")
	}
}

func TestSellerVerification_UpdatedAt(t *testing.T) {
	v := entity.NewSellerVerification(uuid.New())
	originalUpdatedAt := v.UpdatedAt

	// Wait a bit to ensure timestamp would change
	time.Sleep(1 * time.Millisecond)

	// Submit
	_ = v.Submit()
	if !v.UpdatedAt.After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to be updated after Submit")
	}

	originalUpdatedAt = v.UpdatedAt
	time.Sleep(1 * time.Millisecond)

	// Approve
	_ = v.Approve(uuid.New())
	if !v.UpdatedAt.After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to be updated after Approve")
	}
}

