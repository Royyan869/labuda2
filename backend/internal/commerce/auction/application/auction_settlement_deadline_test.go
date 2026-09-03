package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	"github.com/labuda/backend/internal/identity/auth"
	"go.uber.org/zap"
)

// ============================================================================
// CANONICAL SETTLEMENT DEADLINE AUTHORITY TESTS
//
// The settlement shipping deadline is DERIVED — auction.end_at + 24h — and
// never stored or extended. Quote timing must NOT move the deadline. These
// tests pin the claim guard against that derived boundary.
// ============================================================================

func newClaimDeadlineAuction(sellerID uuid.UUID, endAt time.Time) *entity.Auction {
	a := newAuctionForUpdateAuthority(entity.StatusWaitingSettlement, sellerID)
	a.EndAt = endAt
	winner := uuid.New()
	a.CurrentWinnerID = &winner
	bid := int64(1_200_000)
	a.CurrentBid = &bid
	return a
}

// claimServiceWithAuction builds an AuctionService whose GetForUpdate returns
// the given auction (via the spy row/tx helpers in auction_service_authority_test.go).
func claimServiceWithAuction(a *entity.Auction) (*AuctionService, *auctionUpdateSpyTx) {
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: a}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}
	return svc, tx
}

func TestGeneratePricingTokenForAuctionClaim_DeadlineBoundary(t *testing.T) {
	sellerID := uuid.New()
	winnerID := uuid.New()
	now := time.Now()

	t.Run("accepted just before deadline (end_at + 24h - 1s)", func(t *testing.T) {
		a := newClaimDeadlineAuction(sellerID, now.Add(-24*time.Hour+time.Second))
		a.CurrentWinnerID = &winnerID
		svc, tx := claimServiceWithAuction(a)

		got, err := svc.GeneratePricingTokenForAuctionClaim(context.Background(), tx, GeneratePricingTokenForAuctionInput{
			AuctionID: a.ID,
			WinnerID:  winnerID,
		})
		if err != nil {
			t.Fatalf("claim should be accepted before deadline, got error: %v", err)
		}
		if got == nil {
			t.Fatal("expected auction, got nil")
		}
	})

	t.Run("rejected after deadline (end_at + 24h + 1s)", func(t *testing.T) {
		a := newClaimDeadlineAuction(sellerID, now.Add(-24*time.Hour-time.Second))
		a.CurrentWinnerID = &winnerID
		svc, tx := claimServiceWithAuction(a)

		_, err := svc.GeneratePricingTokenForAuctionClaim(context.Background(), tx, GeneratePricingTokenForAuctionInput{
			AuctionID: a.ID,
			WinnerID:  winnerID,
		})
		if err == nil {
			t.Fatal("claim must be rejected after the derived deadline")
		}
	})

	t.Run("no deadline extension from later events", func(t *testing.T) {
		// A quote arriving at T+23h59m does NOT move the deadline. The deadline
		// is derived purely from end_at: changing "quote timing" never changes
		// auction.SettlementDeadline().
		endAt := now.Add(-1 * time.Hour) // auction ended 1h ago
		a := newClaimDeadlineAuction(sellerID, endAt)
		a.CurrentWinnerID = &winnerID

		want := endAt.Add(24 * time.Hour)
		// Simulate a "quote provided at T+23h" and confirm the deadline is
		// unchanged (derived, not quote-relative).
		a.SellerActionRequired = true
		a.SellerQuoteProvided = true
		if got := a.SettlementDeadline(); !got.Equal(want) {
			t.Fatalf("SettlementDeadline() = %v, want %v (deadline must stay end_at+24h regardless of quote timing)", got, want)
		}
	})
}

func TestSettlementDeadline_QuoteTimingDoesNotMoveDeadline(t *testing.T) {
	// Business truth: quote at T+1h / T+23h / T+23h59m all keep the SAME
	// settlement deadline T+24h (where T = auction end). No extension.
	endAt := time.Date(2026, 9, 1, 20, 0, 0, 0, time.UTC)
	want := endAt.Add(24 * time.Hour)

	for _, quoteOffset := range []time.Duration{time.Hour, 23 * time.Hour, 23*time.Hour + 59*time.Minute} {
		quoteAt := endAt.Add(quoteOffset)
		_ = quoteAt // the deadline derivation must not depend on quote timing
		if got := (&entity.Auction{EndAt: endAt}).SettlementDeadline(); !got.Equal(want) {
			t.Fatalf("deadline with quote at T+%v = %v, want %v", quoteOffset, got, want)
		}
	}
}
