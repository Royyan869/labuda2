package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	negotiationEntity "github.com/hishumi/backend/internal/commerce/negotiation/entity"
)

// CANONICAL NEGOTIATION ACTIONABILITY PROJECTION.
//
// CanViewerAct is the single Commerce-provided answer to "may this viewer act
// on the current proposal". It must be derived ONLY from canonical facts:
// participation, lifecycle, and the authoritative last price-change author.
// It must NOT depend on proposal_sequence parity (the removed client-side
// reconstruction).
func TestCanViewerAct(t *testing.T) {
	buyer := uuid.New()
	seller := uuid.New()
	stranger := uuid.New()
	orderID := uuid.New()
	expiredAt := time.Now().Add(-time.Hour)

	activeSession := func() *negotiationEntity.NegotiationSession {
		return &negotiationEntity.NegotiationSession{
			ID:        uuid.New(),
			BuyerID:   buyer,
			SellerID:  seller,
			Status:    negotiationEntity.NegotiationStatusActive,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	historyBy := func(userID uuid.UUID) []*negotiationEntity.NegotiationPriceHistory {
		return []*negotiationEntity.NegotiationPriceHistory{
			{ChangedByUserID: userID},
			{ChangedByUserID: seller},
		}
	}

	cases := []struct {
		name    string
		session *negotiationEntity.NegotiationSession
		history []*negotiationEntity.NegotiationPriceHistory
		viewer  uuid.UUID
		want    bool
	}{
		{
			name:    "other side acted last — viewer can act",
			session: activeSession(),
			history: historyBy(seller),
			viewer:  buyer,
			want:    true,
		},
		{
			name:    "viewer acted last — viewer cannot act",
			session: activeSession(),
			history: historyBy(buyer),
			viewer:  buyer,
			want:    false,
		},
		{
			name:    "participant with empty history may act",
			session: activeSession(),
			history: nil,
			viewer:  seller,
			want:    true,
		},
		{
			name:    "non-participant can never act",
			session: activeSession(),
			history: historyBy(seller),
			viewer:  stranger,
			want:    false,
		},
		{
			name: "expired session is not actionable",
			session: func() *negotiationEntity.NegotiationSession {
				s := activeSession()
				s.ExpiresAt = &expiredAt
				return s
			}(),
			history: historyBy(seller),
			viewer:  buyer,
			want:    false,
		},
		{
			name: "settled session is not actionable",
			session: func() *negotiationEntity.NegotiationSession {
				s := activeSession()
				s.OrderID = &orderID
				return s
			}(),
			history: historyBy(seller),
			viewer:  buyer,
			want:    false,
		},
		{
			name: "accepted session is not actionable",
			session: func() *negotiationEntity.NegotiationSession {
				s := activeSession()
				s.Status = negotiationEntity.NegotiationStatusAccepted
				return s
			}(),
			history: historyBy(seller),
			viewer:  buyer,
			want:    false,
		},
		{
			name: "cancelled session is not actionable",
			session: func() *negotiationEntity.NegotiationSession {
				s := activeSession()
				s.Status = negotiationEntity.NegotiationStatusCancelled
				return s
			}(),
			history: historyBy(seller),
			viewer:  buyer,
			want:    false,
		},
		{
			// PROOF: decision follows the authoritative history author, not
			// proposal_sequence parity. Sequence 4 is even (parity would claim
			// the seller made the last offer); the server says the BUYER did.
			// The seller must therefore be able to act.
			name: "parity is not the authority (even sequence, buyer last)",
			session: func() *negotiationEntity.NegotiationSession {
				s := activeSession()
				s.ProposalSequence = 4
				return s
			}(),
			history: historyBy(buyer),
			viewer:  seller,
			want:    true,
		},
		{
			name:    "nil session is never actionable",
			session: nil,
			history: historyBy(seller),
			viewer:  buyer,
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanViewerAct(tc.session, tc.history, tc.viewer); got != tc.want {
				t.Fatalf("CanViewerAct() = %v, want %v", got, tc.want)
			}
		})
	}
}
