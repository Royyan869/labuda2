package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// settlementEligibleSession builds an accepted, open, unexpired, unsettled
// session (the only state where CanSettle must return true).
func settlementEligibleSession() *NegotiationSession {
	exp := time.Now().Add(24 * time.Hour)
	price := int64(5_000_000)
	return &NegotiationSession{
		ID:            uuid.New(),
		Status:        NegotiationStatusAccepted,
		AcceptedPrice: &price,
		ExpiresAt:     &exp,
	}
}

func TestCanSettle(t *testing.T) {
	tests := []struct {
		name string
		s    *NegotiationSession
		want bool
	}{
		{
			name: "accepted + open + unsettled is settlement-eligible",
			s:    settlementEligibleSession(),
			want: true,
		},
		{
			name: "accepted + settled is not settlement-eligible",
			s: func() *NegotiationSession {
				s := settlementEligibleSession()
				oid := uuid.New()
				s.OrderID = &oid
				return s
			}(),
			want: false,
		},
		{
			name: "accepted + order_id = uuid.Nil is not settled (still eligible)",
			s: func() *NegotiationSession {
				s := settlementEligibleSession()
				nilOID := uuid.Nil
				s.OrderID = &nilOID
				return s
			}(),
			want: true,
		},
		{
			name: "accepted + time-expired is not settlement-eligible",
			s: func() *NegotiationSession {
				s := settlementEligibleSession()
				exp := time.Now().Add(-1 * time.Hour)
				s.ExpiresAt = &exp
				return s
			}(),
			want: false,
		},
		{
			name: "active is not settlement-eligible",
			s: func() *NegotiationSession {
				s := settlementEligibleSession()
				s.Status = NegotiationStatusActive
				return s
			}(),
			want: false,
		},
		{
			name: "cancelled is not settlement-eligible",
			s: func() *NegotiationSession {
				s := settlementEligibleSession()
				s.Status = NegotiationStatusCancelled
				return s
			}(),
			want: false,
		},
		{
			name: "status-expired is not settlement-eligible",
			s: func() *NegotiationSession {
				s := settlementEligibleSession()
				s.Status = NegotiationStatusExpired
				return s
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.CanSettle(); got != tt.want {
				t.Errorf("CanSettle() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCanSettle_AcceptedAndTimeExpiredDiffersFromTerminalStatus proves CanSettle
// treats time-expiry (clock passed expires_at) the same as status-terminality:
// neither can proceed to settlement, but neither is a status transition.
func TestCanSettle_AcceptedTimeExpiredIsNotEligibleEvenWhenStatusAccepted(t *testing.T) {
	s := settlementEligibleSession()
	exp := time.Now().Add(-1 * time.Hour)
	s.ExpiresAt = &exp
	if s.Status != NegotiationStatusAccepted {
		t.Fatalf("fixture: expected accepted status")
	}
	if s.CanSettle() {
		t.Fatal("time-expired accepted session must not be settlement-eligible")
	}
	if s.Status.IsTerminal() {
		t.Fatal("time-expiry must not turn the status-machine terminal (orthogonal concepts)")
	}
}
