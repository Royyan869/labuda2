package publiccard

import (
	"github.com/google/uuid"
)

// ForSaleCard is the canonical PublicCard exposure for a fixed-price sale.
// It collapses the legacy listingref.ListingRef seam onto the PublicCard
// boundary doctrine: identity fields go through SellerCard / UserCard, status
// is the coarsened public lifecycle vocabulary, and no internal stock /
// moderation / finance / shipping state crosses the wire.
//
// PUBLIC BOUNDARY GUARANTEES:
//   - No raw stock quantity / reserved stock / hidden inventory state.
//   - No moderation flags / hidden / shadow-banned indicators.
//   - No reserve price / admin pricing internals.
//   - No shipping configuration / preparation internals beyond what the
//     surface separately emits as additive shipping refs (out of scope here).
//   - Lifecycle is restricted to the coarsened vocabulary {active,
//     unavailable, removed}. Raw enum values (draft, sold, withdrawn, …) are
//     NEVER emitted through this card.
//
// Field nullability:
//   - ID:        required (zero-uuid permitted only when the surface itself
//     has no listing identity).
//   - Title:     required string (empty when not hydrated by the surface).
//   - Thumbnail: nullable; nil when no media is available.
//   - Price:     int64 in minor currency units; mirrors surface truth.
//   - Currency:  nullable; nil when not hydrated by the surface.
//   - Lifecycle: nullable; nil when the surface does not expose lifecycle
//     today. Surfaces SHOULD populate it via the entity's
//     PublicLifecycle() helper rather than emit a raw enum string.
//   - Seller:    nullable; nil when seller identity is not hydrated.
type ForSaleCard struct {
	ID        uuid.UUID   `json:"id"`
	Title     string      `json:"title"`
	Thumbnail *string     `json:"thumbnail_url"`
	Price     int64       `json:"price"`
	Currency  *string     `json:"currency"`
	Lifecycle *string     `json:"lifecycle"`
	Seller    *SellerCard `json:"seller"`
}

// NewForSaleCard builds a ForSaleCard from already-hydrated public-safe values.
// Pass an empty lifecycle string to leave the field nil. Pass nil seller when
// the surface does not hydrate seller identity. Builders MUST NOT call this
// with a raw enum string — coarsen the status via the entity's
// PublicLifecycle() method first.
func NewForSaleCard(
	id uuid.UUID,
	title string,
	thumbnail *string,
	price int64,
	currency *string,
	lifecycle string,
	seller *SellerCard,
) ForSaleCard {
	card := ForSaleCard{
		ID:        id,
		Title:     title,
		Thumbnail: thumbnail,
		Price:     price,
		Currency:  currency,
		Seller:    seller,
	}
	if lifecycle != "" {
		card.Lifecycle = &lifecycle
	}
	return card
}


