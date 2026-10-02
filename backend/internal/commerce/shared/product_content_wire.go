package shared

// ProductContentWireKeys is the canonical Product content projection key set.
//
// CANONICAL TRUTH (owner-locked):
//   - Product is the single content authority (title, description, media,
//     fish attributes, farm address, preparation).
//   - BOTH sale channels (for_sale + auction) emit the SAME key set.
//   - BOTH payload classes (list/search + detail) emit the SAME key set.
//
// What legitimately differs per channel is the economics slot (price/quantity/
// negotiation vs start_price/bid_increment/current_bid/end_at) and what
// legitimately differs per payload class is viewer_capabilities (detail-only).
//
// Every producer test asserts membership of this list; a channel that drops a
// content key breaks the contract instead of silently rendering an empty card.
var ProductContentWireKeys = []string{
	"title",
	"description",
	"media",
	"media_urls",
	"variety",
	"size_cm",
	"age_months",
	"gender",
	"breeder",
	"bloodline",
	"certificates",
	"farm_address_id",
	"preparation_time",
}
