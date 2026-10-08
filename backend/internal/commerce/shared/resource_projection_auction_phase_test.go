package shared

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/stretchr/testify/require"
)

// The auction conversation projection carries the canonical public PHASE plus
// the minimal outcome discriminator `has_winner`. Both are Commerce-owned and
// read-through by the conversation.
func TestAuctionProjection_PhaseAndHasWinner_Wire(t *testing.T) {
	newProjection := func(phase string, hasWinner *bool) ResourceProjection {
		t.Helper()
		p, err := NewLiveResourceProjection(
			ProjectionResourceTypeAuction,
			uuid.New(),
			AuctionLivePayload{
				Title:     "Lelang Koi",
				EndAt:     "2026-09-10T00:00:00Z",
				Lifecycle: phase,
				HasWinner: hasWinner,
				Seller:    publiccard.NewSellerCardWithUserLifecycle(uuid.New(), "petani", nil, "farm", "active"),
			},
			ProjectionViewerCapabilities{CanView: true, CanInteract: true},
		)
		require.NoError(t, err)
		return p
	}

	auctionOf := func(t *testing.T, p ResourceProjection) map[string]any {
		t.Helper()
		raw, err := json.Marshal(p)
		require.NoError(t, err)
		var m map[string]any
		require.NoError(t, json.Unmarshal(raw, &m))
		auction, ok := m["auction"].(map[string]any)
		require.True(t, ok)
		return auction
	}

	t.Run("ended + winner", func(t *testing.T) {
		winner := true
		auction := auctionOf(t, newProjection("ended", &winner))
		require.Equal(t, "ended", auction["lifecycle"])
		require.Equal(t, true, auction["has_winner"])
	})

	t.Run("ended + no winner", func(t *testing.T) {
		noWinner := false
		auction := auctionOf(t, newProjection("ended", &noWinner))
		require.Equal(t, "ended", auction["lifecycle"])
		require.Equal(t, false, auction["has_winner"])
	})

	t.Run("non-ended phase still carries has_winner", func(t *testing.T) {
		winner := true
		auction := auctionOf(t, newProjection("waiting_settlement", &winner))
		require.Equal(t, "waiting_settlement", auction["lifecycle"])
		require.Equal(t, true, auction["has_winner"])
	})
}
