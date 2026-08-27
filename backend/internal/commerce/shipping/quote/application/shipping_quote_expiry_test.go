package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/require"
)

// expiryTestWindow is the tolerance used when asserting that a computed
// ExpiresAt landed "around now + N hours" — generous enough to absorb test
// execution time without being so wide it would miss a real bug (e.g. a
// stray /24 or missing *time.Hour).
const expiryTestWindow = 2 * time.Second

func assertExpiresAtNear(t *testing.T, got *time.Time, wantOffset time.Duration) {
	t.Helper()
	require.NotNil(t, got, "ExpiresAt must never be nil for a newly created quote (PASS_18P)")
	want := time.Now().Add(wantOffset)
	diff := got.Sub(want)
	if diff < 0 {
		diff = -diff
	}
	require.LessOrEqualf(t, diff, expiryTestWindow, "ExpiresAt %v not within %v of expected %v (offset %v)", got, expiryTestWindow, want, wantOffset)
}

// TestCreateShippingQuote_ForSalePath_DefaultsToTwentyFourHourExpiry proves
// scenario 1: a new for_sale shipping quote created without expires_in_hours
// gets ExpiresAt around now + 24h — never nil. This is the PASS_18M bug this
// pass closes: before the fix, an omitted expires_in_hours produced a
// permanently-nil ExpiresAt (an eternal quote).
func TestCreateShippingQuote_ForSalePath_DefaultsToTwentyFourHourExpiry(t *testing.T) {
	sellerID := uuid.New()
	buyerID := uuid.New()
	chatRoom := newAuctionChatRoom(sellerID, buyerID)
	forSaleID := uuid.New()

	quoteRepo := &shippingQuoteRepoStub{}
	svc := newAuctionQuoteService(quoteRepo, chatRoom, &forSaleQuoteRepoStub{sellerID: sellerID}, &auctionQuoteRepoStub{}, &auctionQuoteSender{})

	quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
		ChatID:         chatRoom.ID,
		ProductID:      forSaleID,
		SourceType:     "for_sale",
		SourceID:       forSaleID,
		SellerID:       sellerID,
		Cost:           money.New(15000),
		ExpiresInHours: nil,
	})

	require.NoError(t, err)
	require.NotNil(t, quote)
	assertExpiresAtNear(t, quote.ExpiresAt, DefaultShippingQuoteExpiryHours*time.Hour)
}

// TestCreateShippingQuote_AuctionPath_DefaultsToTwentyFourHourExpiry proves
// scenario 2: the auction shipping quote path gets the same default.
func TestCreateShippingQuote_AuctionPath_DefaultsToTwentyFourHourExpiry(t *testing.T) {
	sellerID := uuid.New()
	winnerID := uuid.New()
	chatRoom := newAuctionChatRoom(sellerID, winnerID)
	auction := newWaitingSettlementAuction(sellerID, winnerID)

	quoteRepo := &shippingQuoteRepoStub{}
	svc := newAuctionQuoteService(quoteRepo, chatRoom, &forSaleQuoteRepoStub{sellerID: sellerID}, &auctionQuoteRepoStub{auction: auction}, &auctionQuoteSender{})

	quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
		ChatID:         chatRoom.ID,
		ProductID:      auction.ProductID,
		SourceType:     "auction",
		SourceID:       auction.ID,
		SellerID:       sellerID,
		AuctionID:      &auction.ID,
		Cost:           money.New(15000),
		ExpiresInHours: nil,
	})

	require.NoError(t, err)
	require.NotNil(t, quote)
	assertExpiresAtNear(t, quote.ExpiresAt, DefaultShippingQuoteExpiryHours*time.Hour)
}

// TestCreateShippingQuote_ExplicitValidExpiry_Accepted proves scenario 3: an
// explicit, in-range expiry (48h) is honored exactly.
func TestCreateShippingQuote_ExplicitValidExpiry_Accepted(t *testing.T) {
	sellerID := uuid.New()
	buyerID := uuid.New()
	chatRoom := newAuctionChatRoom(sellerID, buyerID)
	forSaleID := uuid.New()
	hours := 48

	svc := newAuctionQuoteService(&shippingQuoteRepoStub{}, chatRoom, &forSaleQuoteRepoStub{sellerID: sellerID}, &auctionQuoteRepoStub{}, &auctionQuoteSender{})

	quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
		ChatID:         chatRoom.ID,
		ProductID:      forSaleID,
		SourceType:     "for_sale",
		SourceID:       forSaleID,
		SellerID:       sellerID,
		Cost:           money.New(15000),
		ExpiresInHours: &hours,
	})

	require.NoError(t, err)
	assertExpiresAtNear(t, quote.ExpiresAt, 48*time.Hour)
}

// TestCreateShippingQuote_ExplicitMaxExpiry_Accepted proves scenario 4: the
// maximum allowed expiry (168h / 7 days) is accepted.
func TestCreateShippingQuote_ExplicitMaxExpiry_Accepted(t *testing.T) {
	sellerID := uuid.New()
	buyerID := uuid.New()
	chatRoom := newAuctionChatRoom(sellerID, buyerID)
	forSaleID := uuid.New()
	hours := MaxShippingQuoteExpiryHours

	svc := newAuctionQuoteService(&shippingQuoteRepoStub{}, chatRoom, &forSaleQuoteRepoStub{sellerID: sellerID}, &auctionQuoteRepoStub{}, &auctionQuoteSender{})

	quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
		ChatID:         chatRoom.ID,
		ProductID:      forSaleID,
		SourceType:     "for_sale",
		SourceID:       forSaleID,
		SellerID:       sellerID,
		Cost:           money.New(15000),
		ExpiresInHours: &hours,
	})

	require.NoError(t, err)
	assertExpiresAtNear(t, quote.ExpiresAt, time.Duration(MaxShippingQuoteExpiryHours)*time.Hour)
}

// TestCreateShippingQuote_ExpiryAboveMax_Rejected proves scenario 5: a
// caller requesting more than 168h is rejected, not silently clamped.
func TestCreateShippingQuote_ExpiryAboveMax_Rejected(t *testing.T) {
	sellerID := uuid.New()
	buyerID := uuid.New()
	chatRoom := newAuctionChatRoom(sellerID, buyerID)
	forSaleID := uuid.New()
	hours := MaxShippingQuoteExpiryHours + 1

	quoteRepo := &shippingQuoteRepoStub{}
	svc := newAuctionQuoteService(quoteRepo, chatRoom, &forSaleQuoteRepoStub{sellerID: sellerID}, &auctionQuoteRepoStub{}, &auctionQuoteSender{})

	quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
		ChatID:         chatRoom.ID,
		ProductID:      forSaleID,
		SourceType:     "for_sale",
		SourceID:       forSaleID,
		SellerID:       sellerID,
		Cost:           money.New(15000),
		ExpiresInHours: &hours,
	})

	require.Error(t, err)
	require.Nil(t, quote)
	require.Contains(t, err.Error(), "168")
	require.False(t, quoteRepo.createCalled, "quote must not be persisted when expiry is rejected")
}

// TestCreateShippingQuote_ZeroOrNegativeExpiry_Rejected proves scenario 6:
// zero and negative expires_in_hours values cannot produce an eternal quote
// — they are rejected fail-closed rather than falling back to a nil/eternal
// ExpiresAt (the exact PASS_18M bug: the old code's "> 0" guard silently
// dropped zero/negative values into the nil-expiry branch).
func TestCreateShippingQuote_ZeroOrNegativeExpiry_Rejected(t *testing.T) {
	cases := []struct {
		name  string
		hours int
	}{
		{"zero", 0},
		{"negative_one", -1},
		{"negative_24", -24},
	}
	for _, tc := range cases {
		hours := tc.hours
		t.Run(tc.name, func(t *testing.T) {
			sellerID := uuid.New()
			buyerID := uuid.New()
			chatRoom := newAuctionChatRoom(sellerID, buyerID)
			forSaleID := uuid.New()
			h := hours

			quoteRepo := &shippingQuoteRepoStub{}
			svc := newAuctionQuoteService(quoteRepo, chatRoom, &forSaleQuoteRepoStub{sellerID: sellerID}, &auctionQuoteRepoStub{}, &auctionQuoteSender{})

			quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
				ChatID:         chatRoom.ID,
				ProductID:      forSaleID,
				SourceType:     "for_sale",
				SourceID:       forSaleID,
				SellerID:       sellerID,
				Cost:           money.New(15000),
				ExpiresInHours: &h,
			})

			require.Error(t, err)
			require.Nil(t, quote)
			require.False(t, quoteRepo.createCalled, "quote must not be persisted for non-positive expires_in_hours")
		})
	}
}

// TestCreateShippingQuote_NeverLeavesExpiresAtNil is the PASS_18P headline
// regression test: no normal create path (for_sale or auction, with or
// without an explicit expires_in_hours) may leave ExpiresAt nil. This is the
// exact structural condition PASS_18M identified as making checkout
// expiry validation moot.
func TestCreateShippingQuote_NeverLeavesExpiresAtNil(t *testing.T) {
	t.Run("for_sale path, no explicit expiry", func(t *testing.T) {
		sellerID := uuid.New()
		buyerID := uuid.New()
		chatRoom := newAuctionChatRoom(sellerID, buyerID)
		forSaleID := uuid.New()

		svc := newAuctionQuoteService(&shippingQuoteRepoStub{}, chatRoom, &forSaleQuoteRepoStub{sellerID: sellerID}, &auctionQuoteRepoStub{}, &auctionQuoteSender{})
		quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
			ChatID: chatRoom.ID, ProductID: forSaleID, SourceType: "for_sale",
			SourceID: forSaleID, SellerID: sellerID, Cost: money.New(15000),
		})
		require.NoError(t, err)
		require.NotNil(t, quote.ExpiresAt)
	})

	t.Run("auction path, no explicit expiry", func(t *testing.T) {
		sellerID := uuid.New()
		winnerID := uuid.New()
		chatRoom := newAuctionChatRoom(sellerID, winnerID)
		auction := newWaitingSettlementAuction(sellerID, winnerID)

		svc := newAuctionQuoteService(&shippingQuoteRepoStub{}, chatRoom, &forSaleQuoteRepoStub{sellerID: sellerID}, &auctionQuoteRepoStub{auction: auction}, &auctionQuoteSender{})
		quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
			ChatID: chatRoom.ID, ProductID: auction.ProductID, SourceType: "auction",
			SourceID: auction.ID, SellerID: sellerID, AuctionID: &auction.ID, Cost: money.New(15000),
		})
		require.NoError(t, err)
		require.NotNil(t, quote.ExpiresAt)
	})

	t.Run("for_sale path, explicit valid expiry", func(t *testing.T) {
		sellerID := uuid.New()
		buyerID := uuid.New()
		chatRoom := newAuctionChatRoom(sellerID, buyerID)
		forSaleID := uuid.New()
		hours := 72

		svc := newAuctionQuoteService(&shippingQuoteRepoStub{}, chatRoom, &forSaleQuoteRepoStub{sellerID: sellerID}, &auctionQuoteRepoStub{}, &auctionQuoteSender{})
		quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
			ChatID: chatRoom.ID, ProductID: forSaleID, SourceType: "for_sale",
			SourceID: forSaleID, SellerID: sellerID, Cost: money.New(15000), ExpiresInHours: &hours,
		})
		require.NoError(t, err)
		require.NotNil(t, quote.ExpiresAt)
	})
}
