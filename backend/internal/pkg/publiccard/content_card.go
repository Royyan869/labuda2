package publiccard

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/mediaref"
)

// ContentCard is the canonical PublicCard exposure for social-content posts
// and requests. It is the LAST major public exposure object to land
// (Batch 2D); together with UserCard / SellerCard / ForSaleCard / AuctionCard
// it gives the platform a complete canonical public-exposure seam over which
// evaluator authority can later be promoted past shadow mode.
//
// PUBLIC BOUNDARY GUARANTEES (verified by go vet + forbidden-field grep):
//   - No IsHidden, DeletedAt, FulfilledBy, FulfilledAt, raw Status enum,
//     raw Visibility enum, OriginalAuthorID, or any internal moderation flag.
//   - Lifecycle is restricted to the coarsened {active, unavailable, removed}
//     vocabulary, ALWAYS sourced from entity.Status.PublicLifecycle() (or
//     the package-level PublicLifecycleFromString helper).
//   - Author identity flows through the canonical UserCard seam (Batch 2A)
//     and inherits its public-safety guarantees — no email / phone /
//     firebase_uid / full_name reads anywhere upstream.
//   - SharedForSale / SharedAuction are reserved for future live-hydrated
//     share embedding. They are always nil today. Once live revalidation
//     lands, these fields will carry canonical ForSaleCard / AuctionCard
//     values built from live commerce truth.
//
// Field nullability:
//   - ID:            required.
//   - Type:          required string ("post", "request", …) — matches the
//     surface's existing entity.Type string emission.
//   - Caption:       nullable; nil when the post carries no body text.
//   - Media:         empty slice when no media — never nil.
//   - Lifecycle:     nullable; nil only when the surface intentionally
//     omits lifecycle (this should never happen in practice
//     — surfaces SHOULD always coarsen and emit).
//   - CreatedAt:     RFC3339 string; required.
//   - Author:        nullable; nil only when author hydration was skipped
//     by the surface. Surfaces SHOULD always hydrate.
//   - SharedForSale: nullable; always nil today.
//   - SharedAuction: nullable; always nil today.
type ContentCard struct {
	ID            uuid.UUID           `json:"id"`
	Type          string              `json:"type"`
	Caption       *string             `json:"caption"`
	Media         []mediaref.MediaRef `json:"media"`
	Lifecycle     *string             `json:"lifecycle"`
	CreatedAt     string              `json:"created_at"`
	Author        *UserCard           `json:"author"`
	SharedForSale *ForSaleCard `json:"shared_for_sale"`
	SharedAuction        *AuctionCard        `json:"shared_auction"`
}

// NewContentCard builds a ContentCard from already-hydrated public-safe
// values. Pass an empty lifecycle string to leave the field nil; pass a nil
// caption to omit the body text; pass nil author when the surface chose not
// to hydrate identity (NOT recommended).
//
// mediaURLs is the surface's existing media-URL string slice; each URL is
// wrapped into a mediaref.MediaRef{URL:u} so the wire shape matches the
// canonical media seam already used by feed / search. Pass nil or an empty
// slice for no media — the resulting card carries an empty (non-nil) Media
// slice.
//
// Builders MUST NOT call this with a raw status enum string. Coarsen via
// entity.Status.PublicLifecycle() or contententity.PublicLifecycleFromString
// first.
func NewContentCard(
	id uuid.UUID,
	contentType string,
	caption *string,
	mediaURLs []string,
	lifecycle string,
	createdAt time.Time,
	author *UserCard,
) ContentCard {
	media := make([]mediaref.MediaRef, 0, len(mediaURLs))
	for _, u := range mediaURLs {
		if u == "" {
			continue
		}
		media = append(media, mediaref.MediaRef{URL: u})
	}
	card := ContentCard{
		ID:        id,
		Type:      contentType,
		Caption:   caption,
		Media:     media,
		CreatedAt: createdAt.Format(time.RFC3339),
		Author:    author,
	}
	if lifecycle != "" {
		card.Lifecycle = &lifecycle
	}
	return card
}


