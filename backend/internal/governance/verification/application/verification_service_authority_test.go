package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/capability"
)

type reviewCapabilityAuthorizer struct {
	allowed bool
}

func (r reviewCapabilityAuthorizer) HasCapability(_ context.Context, _ uuid.UUID, _ capability.Capability) (bool, error) {
	return r.allowed, nil
}

func TestVerificationService_RejectsWithoutReviewCapability(t *testing.T) {
	denyAuth := reviewCapabilityAuthorizer{allowed: false}
	actorID := uuid.New()
	sellerID := uuid.New()

	svc := NewVerificationService(nil, nil, nil, nil, denyAuth)

	cases := []struct {
		name string
		call func() error
	}{
		{"approve", func() error { return svc.ApproveVerification(context.Background(), sellerID, actorID) }},
		{"reject", func() error { return svc.RejectVerification(context.Background(), sellerID, actorID, "reason") }},
		{"request_resubmission", func() error { return svc.RequestResubmission(context.Background(), sellerID, actorID, "reason") }},
		{"suspend", func() error { return svc.SuspendVerification(context.Background(), sellerID, actorID, "reason") }},
		{"revoke", func() error { return svc.RevokeVerification(context.Background(), sellerID, actorID, "reason") }},
		{"investigate", func() error { return svc.InvestigateVerification(context.Background(), sellerID, actorID, "reason") }},
		{"restore", func() error { return svc.RestoreVerification(context.Background(), sellerID, actorID) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatalf("expected %s to fail without seller.verification.review", tc.name)
			}
			var authErr *ErrVerificationAuthorityRequired
			if !errors.As(err, &authErr) {
				t.Fatalf("expected authority error, got %v", err)
			}
		})
	}
}


