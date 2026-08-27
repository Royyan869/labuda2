// Package mediaref defines the canonical additive MediaRef shape
// emitted across Phase C horizontal-convergence surfaces.
//
// Convergence contract (PHASE C — second horizontal layer, Option D):
//
//   media: [
//     {
//       url:    string,
//       kind:   string|null,
//       width:  int|null,
//       height: int|null,
//     },
//     ...
//   ]
//
// Adoption discipline:
//   - PURELY ADDITIVE. Adopters MUST NOT remove, rename, or reshape
//     existing flat media fields (media_urls, thumbnail_url) when
//     adding the canonical "media" key.
//   - Width / Height are nil when the surface does not currently
//     hydrate dimensions; this layer does NOT introduce dimension
//     inference, remote fetches, or DB schema additions.
//   - Kind is a free-form string ("image", "video", "thumbnail", etc.)
//     reflecting the surface's already-known semantic; nil when the
//     surface has no semantic to mirror.
//   - This package contains exactly one struct and zero methods. It
//     is NOT a builder, framework, or PublicCard abstraction.
//
// Feed surface uses a transitional hybrid shape (legacy
// FeedMedia{url,type,position} extended with Kind/Width/Height
// in-place) rather than emitting MediaRef under a separate key —
// see backend/internal/social/feed/entity/feed_item.go for the
// hybrid struct. The JSON keys "kind"/"width"/"height" overlap
// with this canonical contract; the JSON keys "type"/"position"
// remain for mobile parser compatibility.
package mediaref

// MediaRef is the canonical additive media reference. It is the
// SECOND real horizontal-convergence layer between discovery
// surfaces; the three search surfaces (/search/listings,
// /search/content, /search/auctions) emit a `[]MediaRef` under
// the JSON key "media" alongside their existing flat fields
// (media_urls, thumbnail_url).
//
// Field nullability:
//   - URL:    required (zero-value strings are skipped by adopters).
//   - Kind:   nullable; nil when the surface has no semantic.
//   - Width:  nullable; always nil in this layer (no hydration).
//   - Height: nullable; always nil in this layer (no hydration).
type MediaRef struct {
	URL    string  `json:"url"`
	Kind   *string `json:"kind"`
	Width  *int    `json:"width"`
	Height *int    `json:"height"`
}


