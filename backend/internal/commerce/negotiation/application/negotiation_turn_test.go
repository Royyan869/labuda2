package application

import (
	"testing"

	"github.com/google/uuid"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
)

// TURN AUTHORITY (counter side): the party that made the last price change may
// not counter again. This rule used to live only in the mobile card, derived
// from proposal_sequence parity — a double counter by one side inverted that
// parity and disabled the CTA of the party that was actually owed a response
// (runtime proof, 2026-09-30: buyer saw a disabled CTA after the seller's two
// consecutive counters).
func TestCounterTurnDenied(t *testing.T) {
	seller := uuid.New()
	buyer := uuid.New()

	history := func(changedBy uuid.UUID) []*negotiationEntity.NegotiationPriceHistory {
		// Newest-first, matching GetPriceHistoryBySession's ORDER BY created_at DESC.
		return []*negotiationEntity.NegotiationPriceHistory{
			{ChangedByUserID: changedBy},
			{ChangedByUserID: seller},
		}
	}

	cases := []struct {
		name    string
		history []*negotiationEntity.NegotiationPriceHistory
		sender  uuid.UUID
		want    bool
	}{
		{name: "no history yet — first counter allowed", history: nil, sender: buyer, want: false},
		{name: "other side acted last — counter allowed", history: history(buyer), sender: seller, want: false},
		{name: "sender acted last — counter denied", history: history(seller), sender: seller, want: true},
		{name: "buyer counters twice — second denied", history: history(buyer), sender: buyer, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := counterTurnDenied(tc.history, tc.sender); got != tc.want {
				t.Fatalf("counterTurnDenied() = %v, want %v", got, tc.want)
			}
		})
	}
}
