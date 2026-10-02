package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
)

// DEAL BINDING contract: the detail wire exposes `viewer_negotiation_id` only
// for a SETTLEABLE deal (accepted, unexpired, unsettled). Everything else —
// no session, active negotiation, expired deal, already-settled deal — must
// yield NO binding so the CTA never sends a negotiation_id.
func TestViewerNegotiationBinding(t *testing.T) {
	t.Run("no session yields no binding", func(t *testing.T) {
		if got := viewerNegotiationBinding(nil); got != nil {
			t.Errorf("nil session binding = %v, want nil", *got)
		}
	})

	t.Run("accepted deal inside its 24h window binds", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		session := &negotiationEntity.NegotiationSession{
			ID:        uuid.New(),
			Status:    negotiationEntity.NegotiationStatusAccepted,
			ExpiresAt: &future,
		}
		got := viewerNegotiationBinding(session)
		if got == nil || *got != session.ID {
			t.Errorf("binding = %v, want session id %s", got, session.ID)
		}
	})

	t.Run("expired deal yields no binding", func(t *testing.T) {
		past := time.Now().Add(-time.Minute)
		session := &negotiationEntity.NegotiationSession{
			ID:        uuid.New(),
			Status:    negotiationEntity.NegotiationStatusAccepted,
			ExpiresAt: &past,
		}
		if got := viewerNegotiationBinding(session); got != nil {
			t.Errorf("expired deal binding = %v, want nil", *got)
		}
	})

	t.Run("settled deal yields no binding", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		orderID := uuid.New()
		session := &negotiationEntity.NegotiationSession{
			ID:        uuid.New(),
			Status:    negotiationEntity.NegotiationStatusAccepted,
			ExpiresAt: &future,
			OrderID:   &orderID,
		}
		if got := viewerNegotiationBinding(session); got != nil {
			t.Errorf("settled deal binding = %v, want nil", *got)
		}
	})

	t.Run("active (not yet accepted) negotiation yields no binding", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		session := &negotiationEntity.NegotiationSession{
			ID:        uuid.New(),
			Status:    negotiationEntity.NegotiationStatusActive,
			ExpiresAt: &future,
		}
		if got := viewerNegotiationBinding(session); got != nil {
			t.Errorf("active session binding = %v, want nil", *got)
		}
	})
}
