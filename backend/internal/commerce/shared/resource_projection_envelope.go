package shared

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hishumi/backend/internal/pkg/mediaref"
	"github.com/hishumi/backend/internal/pkg/publiccard"
)

// CANONICAL RESOURCE PROJECTION ENVELOPE (scope #3 convergence).
//
// This is the single wire shape for resource projections on EVERY surface:
// content detail, feed, search, comment, chat message attachment, chat room
// list. Before this type existed the chat module and the content module each
// owned a private envelope whose payloads disagreed (price scalar vs money
// object, payload-level can_interact vs envelope-level capabilities,
// image_url vs media[], flat seller vs SellerCard). Two envelopes for one
// resource = dual authority. Both are being killed in favor of this type.
//
// Wire contract (strict, state-specific — see ResourceProjection.Validate):
//
//	LIVE:      {state, resource_type, resource_id, canonical_url,
//	            viewer_capabilities, <payload>}
//	TOMBSTONE: {state, resource_type, resource_id, viewer_capabilities}
//
// Decisions locked during convergence:
//   - resource_id is ALWAYS present (content contract; a tombstone still has
//     to be identifiable for dedup/audit — chat's former ID omission died).
//   - canonical_url is LIVE-only (a URL into a dead resource is a lie).
//   - viewer_capabilities is ENVELOPE-level; the former commerce_actions
//     viewer capability matrix was purged — product attributes (e.g. the
//     for_sale `negotiation_enabled` flag) live on the payload.
//   - price is a money object {amount, currency} (chat's "canonical live
//     money envelope"); the content scalar died.
//   - seller is always publiccard.SellerCard (tier-gated public card);
//     chat's flat ForSaleLiveSeller died.
//   - media is always []mediaref.MediaRef plus optional thumbnail_url;
//     chat's singular image_url died.
//   - Display policy (owner contract): the chat display layer never renders a
//     price; discovery surfaces do. The envelope carries price on LIVE for
//     all surfaces — rendering is not the envelope's concern.

// ProjectionState is the locked lifecycle state of a canonical projection.
type ProjectionState string

const (
	ProjectionStateLive      ProjectionState = "LIVE"
	ProjectionStateTombstone ProjectionState = "TOMBSTONE"
)

// ProjectionResourceType is the canonical resource vocabulary. It replaces the
// two former per-module enums (content ContentResourceOccurrenceResourceType
// and chat ResourceOccurrenceResourceType), whose wire values were identical
// duplicates.
type ProjectionResourceType string

const (
	ProjectionResourceTypeProfile ProjectionResourceType = "profile"
	ProjectionResourceTypeContent ProjectionResourceType = "content"
	ProjectionResourceTypeForSale ProjectionResourceType = "for_sale"
	ProjectionResourceTypeAuction ProjectionResourceType = "auction"
)

// IsValid reports whether t is in the canonical vocabulary.
func (t ProjectionResourceType) IsValid() bool {
	switch t {
	case ProjectionResourceTypeProfile, ProjectionResourceTypeContent,
		ProjectionResourceTypeForSale, ProjectionResourceTypeAuction:
		return true
	default:
		return false
	}
}

// ProjectionViewerCapabilities is the envelope-level viewer truth. It is
// present in BOTH states: in TOMBSTONE it carries the honest
// blocked_by_tombstone=true explanation.
type ProjectionViewerCapabilities struct {
	CanView     bool `json:"can_view"`
	CanInteract bool `json:"can_interact"`
	// CanManage is the canonical Commerce OWNERSHIP capability for the viewer
	// (ViewerCapabilities.Role == "owner", i.e. viewer == product seller),
	// evaluated by the SAME authority that powers the detail wire
	// (EvaluateForSaleViewerCapabilities / EvaluateAuctionViewerCapabilities).
	// It is the sole viewer-scoped authorization a conversation surface may use
	// to offer an owner-only product action; the surface never derives
	// ownership from a message/bubble sender.
	CanManage          bool `json:"can_manage"`
	BlockedByTombstone bool `json:"blocked_by_tombstone"`
}

// LivePrice is the canonical live money envelope {amount, currency}.
type LivePrice struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// LivePriceCurrencyIDR is the single currency of the canonical live money
// envelope — every commerce surface prices in IDR today; the field exists so
// the currency is explicit on the wire instead of implied by convention.
const LivePriceCurrencyIDR = "IDR"

// NestedResourceIndicator captures the depth-1 nested identity (identity only).
type NestedResourceIndicator struct {
	ResourceType ProjectionResourceType `json:"resource_type"`
	ResourceID   uuid.UUID              `json:"resource_id"`
}

// ProfileLivePayload is the LIVE profile payload.
type ProfileLivePayload struct {
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	// StoreName/IsSeller are best-effort: surfaces that cannot source seller
	// identity omit them (absent = unknown, never a fabricated false).
	StoreName *string `json:"store_name,omitempty"`
	IsSeller  bool    `json:"is_seller,omitempty"`
	Lifecycle string  `json:"lifecycle"`
}

// ContentLivePayload is the LIVE content payload.
type ContentLivePayload struct {
	Caption        *string                  `json:"caption"`
	Media          []mediaref.MediaRef      `json:"media"`
	Lifecycle      string                   `json:"lifecycle"`
	CreatedAt      string                   `json:"created_at"`
	Author         publiccard.UserCard      `json:"author"`
	NestedResource *NestedResourceIndicator `json:"nested_resource,omitempty"`
}

// ForSaleLivePayload is the LIVE fixed-price sale payload.
type ForSaleLivePayload struct {
	Title             string              `json:"title"`
	Media             []mediaref.MediaRef `json:"media"`
	ThumbnailURL      *string             `json:"thumbnail_url,omitempty"`
	Price             LivePrice           `json:"price"`
	Status            string              `json:"status"`
	QuantityAvailable int                 `json:"quantity_available"`
	// NegotiationEnabled is the canonical PRODUCT-LEVEL negotiation attribute:
	// the listing is negotiable, active and in stock. It is viewer-independent
	// (never derived from authentication, seller trust, role or a capability
	// evaluation). Surfaces render it as the informational "Nego" attribute.
	NegotiationEnabled bool                  `json:"negotiation_enabled"`
	Seller             publiccard.SellerCard `json:"seller"`
}

// AuctionLivePayload is the LIVE auction payload.
type AuctionLivePayload struct {
	Title        string              `json:"title"`
	Media        []mediaref.MediaRef `json:"media"`
	ThumbnailURL *string             `json:"thumbnail_url,omitempty"`
	CurrentBid   *int64              `json:"current_bid,omitempty"`
	BuyNowPrice  *int64              `json:"buy_now_price,omitempty"`
	EndAt        string              `json:"end_at"`
	// Lifecycle is the canonical public auction PHASE vocabulary:
	// {scheduled, active, waiting_settlement, ended, cancelled}. It is sourced
	// from auctionentity.Status.PublicPhase() — Commerce owns the value.
	// `lapsed` is not public and coarsens to `cancelled`.
	Lifecycle string `json:"lifecycle"`
	// HasWinner is the minimal outcome discriminator for an `ended` phase
	// (ended+winner vs ended+no-winner). Sourced from the canonical winner
	// authority (Auction.WinnerID()). Conversation reads it; it never derives
	// the outcome itself. Nil means the producer did not set it.
	HasWinner *bool                 `json:"has_winner,omitempty"`
	Seller    publiccard.SellerCard `json:"seller"`
}

// ResourceProjection is the sealed canonical projection envelope.
type ResourceProjection struct {
	State        ProjectionState
	ResourceType ProjectionResourceType
	ResourceID   uuid.UUID

	ViewerCapabilities ProjectionViewerCapabilities

	Profile *ProfileLivePayload
	Content *ContentLivePayload
	ForSale *ForSaleLivePayload
	Auction *AuctionLivePayload
}

// CanonicalResourceURL derives the canonical route for a typed identity.
// This is the single route authority for profile/content/for_sale/auction.
func CanonicalResourceURL(rt ProjectionResourceType, id uuid.UUID) (string, error) {
	if id == uuid.Nil {
		return "", fmt.Errorf("shared: cannot derive URL for nil resource ID")
	}
	switch rt {
	case ProjectionResourceTypeProfile:
		return "/user/" + id.String(), nil
	case ProjectionResourceTypeContent:
		return "/content/" + id.String(), nil
	case ProjectionResourceTypeForSale:
		return "/for-sale/" + id.String(), nil
	case ProjectionResourceTypeAuction:
		return "/auction/" + id.String(), nil
	default:
		return "", fmt.Errorf("shared: unknown resource type %q", rt)
	}
}

// NewLiveResourceProjection builds a LIVE envelope and validates it.
func NewLiveResourceProjection(
	rt ProjectionResourceType,
	id uuid.UUID,
	payload any,
	caps ProjectionViewerCapabilities,
) (ResourceProjection, error) {
	p := ResourceProjection{
		State:              ProjectionStateLive,
		ResourceType:       rt,
		ResourceID:         id,
		ViewerCapabilities: caps,
	}
	switch v := payload.(type) {
	case ProfileLivePayload:
		p.Profile = &v
	case *ProfileLivePayload:
		p.Profile = v
	case ContentLivePayload:
		p.Content = &v
	case *ContentLivePayload:
		p.Content = v
	case ForSaleLivePayload:
		p.ForSale = &v
	case *ForSaleLivePayload:
		p.ForSale = v
	case AuctionLivePayload:
		p.Auction = &v
	case *AuctionLivePayload:
		p.Auction = v
	default:
		return ResourceProjection{}, fmt.Errorf("shared: unsupported payload type %T", payload)
	}
	if err := p.Validate(); err != nil {
		return ResourceProjection{}, err
	}
	return p, nil
}

// NewTombstoneResourceProjection builds a TOMBSTONE envelope. The resource ID
// is preserved (canonical contract — identity survives death for dedup/audit);
// canonical_url and any payload are forbidden.
func NewTombstoneResourceProjection(rt ProjectionResourceType, id uuid.UUID) (ResourceProjection, error) {
	p := ResourceProjection{
		State:              ProjectionStateTombstone,
		ResourceType:       rt,
		ResourceID:         id,
		ViewerCapabilities: ProjectionViewerCapabilities{BlockedByTombstone: true},
	}
	if err := p.Validate(); err != nil {
		return ResourceProjection{}, err
	}
	return p, nil
}

// Validate enforces the canonical contract.
func (p ResourceProjection) Validate() error {
	if p.State != ProjectionStateLive && p.State != ProjectionStateTombstone {
		return fmt.Errorf("shared: invalid projection state %q", p.State)
	}
	if !p.ResourceType.IsValid() {
		return fmt.Errorf("shared: invalid resource type %q", p.ResourceType)
	}
	if p.ResourceID == uuid.Nil {
		return fmt.Errorf("shared: %s projection requires resource id", p.State)
	}

	payloadCount := 0
	for _, set := range []bool{
		p.Profile != nil, p.Content != nil, p.ForSale != nil, p.Auction != nil,
	} {
		if set {
			payloadCount++
		}
	}

	switch p.State {
	case ProjectionStateLive:
		if payloadCount != 1 {
			return fmt.Errorf("shared: LIVE projection requires exactly one payload")
		}
		caps := p.ViewerCapabilities
		if !caps.CanView {
			return fmt.Errorf("shared: LIVE projection requires can_view=true")
		}
		if caps.BlockedByTombstone {
			return fmt.Errorf("shared: LIVE projection requires blocked_by_tombstone=false")
		}
		switch p.ResourceType {
		case ProjectionResourceTypeProfile:
			if p.Profile == nil {
				return fmt.Errorf("shared: LIVE profile projection requires profile payload")
			}
			if caps.CanInteract {
				return fmt.Errorf("shared: LIVE profile projection requires can_interact=false")
			}
		case ProjectionResourceTypeContent:
			if p.Content == nil {
				return fmt.Errorf("shared: LIVE content projection requires content payload")
			}
			if caps.CanInteract {
				return fmt.Errorf("shared: LIVE content projection requires can_interact=false")
			}
		case ProjectionResourceTypeForSale:
			if p.ForSale == nil {
				return fmt.Errorf("shared: LIVE for_sale projection requires for_sale payload")
			}
		case ProjectionResourceTypeAuction:
			if p.Auction == nil {
				return fmt.Errorf("shared: LIVE auction projection requires auction payload")
			}
		}
	case ProjectionStateTombstone:
		if payloadCount != 0 {
			return fmt.Errorf("shared: TOMBSTONE projection forbids payloads")
		}
		caps := p.ViewerCapabilities
		if caps.CanView || caps.CanInteract || !caps.BlockedByTombstone {
			return fmt.Errorf("shared: TOMBSTONE projection requires can_view=false, can_interact=false, blocked_by_tombstone=true")
		}
	}
	return nil
}

// MarshalJSON emits the strict state-specific envelope.
func (p ResourceProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	switch p.State {
	case ProjectionStateLive:
		url, err := CanonicalResourceURL(p.ResourceType, p.ResourceID)
		if err != nil {
			return nil, fmt.Errorf("shared: LIVE marshal: %w", err)
		}
		aux := struct {
			State              string                       `json:"state"`
			ResourceType       string                       `json:"resource_type"`
			ResourceID         string                       `json:"resource_id"`
			CanonicalURL       string                       `json:"canonical_url"`
			ViewerCapabilities ProjectionViewerCapabilities `json:"viewer_capabilities"`
			Profile            *ProfileLivePayload          `json:"profile,omitempty"`
			Content            *ContentLivePayload          `json:"content,omitempty"`
			ForSale            *ForSaleLivePayload          `json:"for_sale,omitempty"`
			Auction            *AuctionLivePayload          `json:"auction,omitempty"`
		}{
			State:              string(p.State),
			ResourceType:       string(p.ResourceType),
			ResourceID:         p.ResourceID.String(),
			CanonicalURL:       url,
			ViewerCapabilities: p.ViewerCapabilities,
			Profile:            p.Profile,
			Content:            p.Content,
			ForSale:            p.ForSale,
			Auction:            p.Auction,
		}
		return json.Marshal(aux)
	case ProjectionStateTombstone:
		aux := struct {
			State              string                       `json:"state"`
			ResourceType       string                       `json:"resource_type"`
			ResourceID         string                       `json:"resource_id"`
			ViewerCapabilities ProjectionViewerCapabilities `json:"viewer_capabilities"`
		}{
			State:              string(p.State),
			ResourceType:       string(p.ResourceType),
			ResourceID:         p.ResourceID.String(),
			ViewerCapabilities: p.ViewerCapabilities,
		}
		return json.Marshal(aux)
	default:
		return nil, fmt.Errorf("shared: unreachable projection state %q", p.State)
	}
}
