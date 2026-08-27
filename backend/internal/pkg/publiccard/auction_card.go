package publiccard

import (
	"github.com/google/uuid"
)

// AuctionCard is the canonical PublicCard exposure for a commerce auction.
// It collapses the legacy auctionref.AuctionRef seam onto the PublicCard
// boundary doctrine: identity fields flow through SellerCard, status is the
// coarsened public lifecycle vocabulary, and bidding/admin internals never
// cross the wire.
//
// PUBLIC BOUNDARY GUARANTEES:
//   - No bidder identity beyond what other PublicCard surfaces expose.
//   - No anti-sniping internals (extension windows, sniper-watch flags).
//   - No moderation flags / hidden / shadow-banned indicators.
//   - No admin reserve price or hidden minimum semantics.
//   - No raw auction state machine values (waiting_settlement, expired_bnr,
//     scheduled, …) — Lifecycle is always coarsened to {active, unavailable,
//     removed} via entity.Status.PublicLifecycle().
//
// Field nullability:
//   - ID:           required.
//   - Title:        required string (empty when not hydrated).
//   - Thumbnail:    nullable; nil when no media is available.
//   - CurrentBid:   nullable; nil when no bids have been placed yet.
//   - BuyNowPrice:  nullable; nil when the auction has no buy-now option.
//   - EndAt:        RFC3339 string; the auction's scheduled end timestamp.
//   - Lifecycle:    nullable; nil when the surface does not expose lifecycle.
//   - Seller:       nullable; nil when seller identity is not hydrated.
type AuctionCard struct {
	ID          uuid.UUID   `json:"id"`
	Title       string      `json:"title"`
	Thumbnail   *string     `json:"thumbnail_url"`
	CurrentBid  *int64      `json:"current_bid"`
	BuyNowPrice *int64      `json:"buy_now_price"`
	EndAt       string      `json:"end_at"`
	Lifecycle   *string     `json:"lifecycle"`
	Seller      *SellerCard `json:"seller"`
}

// NewAuctionCard builds an AuctionCard from already-hydrated public-safe
// values. Pass an empty lifecycle string to leave the field nil. Builders MUST
// NOT call this with a raw enum status string — coarsen via the entity's
// PublicLifecycle() method first.
func NewAuctionCard(
	id uuid.UUID,
	title string,
	thumbnail *string,
	currentBid *int64,
	buyNowPrice *int64,
	endAt string,
	lifecycle string,
	seller *SellerCard,
) AuctionCard {
	card := AuctionCard{
		ID:          id,
		Title:       title,
		Thumbnail:   thumbnail,
		CurrentBid:  currentBid,
		BuyNowPrice: buyNowPrice,
		EndAt:       endAt,
		Seller:      seller,
	}
	if lifecycle != "" {
		card.Lifecycle = &lifecycle
	}
	return card
}


