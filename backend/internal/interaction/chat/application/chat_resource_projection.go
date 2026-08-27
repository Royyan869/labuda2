package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/pkg/publiccard"
)

// ProjectionState is the locked lifecycle state of a resource projection.
type ProjectionState string

const (
	ProjectionStateLive      ProjectionState = "LIVE"
	ProjectionStateTombstone ProjectionState = "TOMBSTONE"
)

// ProjectionViewerCapabilities captures the generic viewer-facing envelope.
type ProjectionViewerCapabilities struct {
	CanView            bool `json:"can_view"`
	CanInteract        bool `json:"can_interact"`
	BlockedByTombstone bool `json:"blocked_by_tombstone"`
}

// CommerceActionCapabilities captures FPS/Auction-specific interactive actions.
type CommerceActionCapabilities struct {
	Role         string `json:"role,omitempty"`
	CanChat      bool   `json:"can_chat"`
	CanNegotiate bool   `json:"can_negotiate"`
	CanBuy       bool   `json:"can_buy"`
	CanBid       bool   `json:"can_bid"`
	CanManage    bool   `json:"can_manage"`
}

// ResourceProjectionIdentity is the typed internal identity for a projection.
type ResourceProjectionIdentity struct {
	ResourceType chatEntity.ResourceOccurrenceResourceType
	ResourceID   uuid.UUID
}

// NewResourceProjectionIdentity validates a canonical identity for LIVE projections.
func NewResourceProjectionIdentity(
	rt chatEntity.ResourceOccurrenceResourceType,
	id uuid.UUID,
) (ResourceProjectionIdentity, error) {
	if id == uuid.Nil {
		return ResourceProjectionIdentity{}, fmt.Errorf("chat: identity requires non-nil resource ID")
	}
	if !rt.IsValid() {
		return ResourceProjectionIdentity{}, fmt.Errorf("chat: invalid resource type %q", rt)
	}
	return ResourceProjectionIdentity{ResourceType: rt, ResourceID: id}, nil
}

// CanonicalResourceURL derives the canonical URL path for a typed identity.
func CanonicalResourceURL(rt chatEntity.ResourceOccurrenceResourceType, id uuid.UUID) (string, error) {
	if id == uuid.Nil {
		return "", fmt.Errorf("chat: cannot derive URL for nil resource ID")
	}
	switch rt {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		return "/user/" + id.String(), nil
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		return "/content/" + id.String(), nil
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		return "/for-sale/" + id.String(), nil
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		return "/auction/" + id.String(), nil
	default:
		return "", fmt.Errorf("chat: unknown resource type %q", rt)
	}
}

// ResourceProjectionPayload is the sealed interface for typed projection payloads.
type ResourceProjectionPayload interface {
	resourceProjectionPayloadMarker()
}

// LIVE payloads.
type ProfileLivePayload struct {
	resourceProjectionPayloadMarkerImpl `json:"-"`
	Username                            string  `json:"username"`
	AvatarURL                           *string `json:"avatar_url,omitempty"`
	StoreName                           *string `json:"store_name,omitempty"`
	IsSeller                            bool    `json:"is_seller"`
	Lifecycle                           string  `json:"lifecycle"`
}

type ContentLivePayload struct {
	resourceProjectionPayloadMarkerImpl `json:"-"`
	Caption                             *string                  `json:"caption"`
	Media                               []mediaref.MediaRef      `json:"media"`
	Lifecycle                           string                   `json:"lifecycle"`
	CreatedAt                           string                   `json:"created_at"`
	Author                              publiccard.UserCard      `json:"author"`
	NestedResource                      *NestedResourceIndicator `json:"nested_resource,omitempty"`
}

// ForSaleLivePrice is the canonical live money envelope.
type ForSaleLivePrice struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// ForSaleLiveSeller is the canonical live seller identity envelope.
type ForSaleLiveSeller struct {
	ID         uuid.UUID `json:"id"`
	StoreName  string    `json:"store_name"`
	StoreImage *string   `json:"store_image,omitempty"`
	Username   string    `json:"username"`
	Lifecycle  string    `json:"lifecycle"`
}

type ForSaleLivePayload struct {
	resourceProjectionPayloadMarkerImpl `json:"-"`
	Title                               string            `json:"title"`
	ImageURL                            *string           `json:"image_url,omitempty"`
	Price                               ForSaleLivePrice  `json:"price"`
	Status                              string            `json:"status"`
	Seller                              ForSaleLiveSeller `json:"seller"`
	QuantityAvailable                   int               `json:"quantity_available"`
}

type AuctionLivePayload struct {
	resourceProjectionPayloadMarkerImpl `json:"-"`
	Title                               string                 `json:"title"`
	Thumbnail                           *string                `json:"thumbnail_url,omitempty"`
	CurrentBid                          *int64                 `json:"current_bid,omitempty"`
	BuyNowPrice                         *int64                 `json:"buy_now_price,omitempty"`
	EndAt                               string                 `json:"end_at"`
	Lifecycle                           *string                `json:"lifecycle,omitempty"`
	Seller                              *publiccard.SellerCard `json:"seller,omitempty"`
}

type resourceProjectionPayloadMarkerImpl struct{}

func (resourceProjectionPayloadMarkerImpl) resourceProjectionPayloadMarker() {}

func (ProfileLivePayload) resourceProjectionPayloadMarker() {}
func (ContentLivePayload) resourceProjectionPayloadMarker() {}
func (ForSaleLivePayload) resourceProjectionPayloadMarker() {}
func (AuctionLivePayload) resourceProjectionPayloadMarker() {}

// NestedResourceIndicator captures a depth-1 nested resource identity.
type NestedResourceIndicator struct {
	ResourceType chatEntity.ResourceOccurrenceResourceType
	ResourceID   uuid.UUID
}

// MarshalJSON emits only the typed nested identity envelope.
func (n NestedResourceIndicator) MarshalJSON() ([]byte, error) {
	if !n.ResourceType.IsValid() {
		return nil, fmt.Errorf("chat: nested indicator has invalid resource type %q", n.ResourceType)
	}
	if n.ResourceID == uuid.Nil {
		return nil, fmt.Errorf("chat: nested indicator has nil resource ID")
	}
	return json.Marshal(struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
	}{
		ResourceType: string(n.ResourceType),
		ResourceID:   n.ResourceID.String(),
	})
}

// ResourceProjection is the sealed projection envelope used by the chat app.
type ResourceProjection struct {
	State              ProjectionState
	Identity           ResourceProjectionIdentity
	ViewerCapabilities ProjectionViewerCapabilities
	CommerceActions    *CommerceActionCapabilities
	Payload            ResourceProjectionPayload
}

// NewLiveProjection builds a LIVE projection and validates the invariants.
func NewLiveProjection(
	rt chatEntity.ResourceOccurrenceResourceType,
	id uuid.UUID,
	payload ResourceProjectionPayload,
	viewerCaps ProjectionViewerCapabilities,
	commerceActions *CommerceActionCapabilities,
) (ResourceProjection, error) {
	identity, err := NewResourceProjectionIdentity(rt, id)
	if err != nil {
		return ResourceProjection{}, err
	}
	p := ResourceProjection{
		State:              ProjectionStateLive,
		Identity:           identity,
		ViewerCapabilities: viewerCaps,
		CommerceActions:    commerceActions,
		Payload:            payload,
	}
	if err := p.Validate(); err != nil {
		return ResourceProjection{}, err
	}
	return p, nil
}

// NewTombstoneProjection builds a TOMBSTONE projection.
func NewTombstoneProjection(rt chatEntity.ResourceOccurrenceResourceType) (ResourceProjection, error) {
	if !rt.IsValid() {
		return ResourceProjection{}, fmt.Errorf("chat: invalid resource type %q", rt)
	}
	p := ResourceProjection{
		State: ProjectionStateTombstone,
		Identity: ResourceProjectionIdentity{
			ResourceType: rt,
		},
		ViewerCapabilities: ProjectionViewerCapabilities{
			CanView:            false,
			CanInteract:        false,
			BlockedByTombstone: true,
		},
	}
	if err := p.Validate(); err != nil {
		return ResourceProjection{}, err
	}
	return p, nil
}

// Validate checks the strict projection invariants.
func (p ResourceProjection) Validate() error {
	switch p.State {
	case ProjectionStateLive, ProjectionStateTombstone:
	default:
		return fmt.Errorf("chat: invalid projection state %q", p.State)
	}

	if !p.Identity.ResourceType.IsValid() {
		return fmt.Errorf("chat: invalid resource type %q", p.Identity.ResourceType)
	}

	switch p.State {
	case ProjectionStateLive:
		if p.Identity.ResourceID == uuid.Nil {
			return fmt.Errorf("chat: %s projection requires non-nil resource ID", p.State)
		}
	case ProjectionStateTombstone:
		// ResourceID is intentionally omitted from TOMBSTONE serialization.
	}

	switch p.State {
	case ProjectionStateLive:
		if p.Payload == nil {
			return fmt.Errorf("chat: LIVE projection requires non-nil payload")
		}
		if err := validateLivePayloadType(p.Identity.ResourceType, p.Payload); err != nil {
			return err
		}
		if err := validateLiveCommerceActions(p.Identity.ResourceType, p.CommerceActions); err != nil {
			return err
		}
	case ProjectionStateTombstone:
		if p.Payload != nil {
			return fmt.Errorf("chat: TOMBSTONE projection requires nil payload")
		}
		if p.CommerceActions != nil {
			return fmt.Errorf("chat: TOMBSTONE projection requires nil commerce_actions")
		}
	}

	if err := validateViewerCapabilities(p.State, p.Identity.ResourceType, p.ViewerCapabilities, p.CommerceActions); err != nil {
		return err
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
		return marshalLiveJSON(p)
	case ProjectionStateTombstone:
		return marshalTombstoneJSON(p)
	default:
		return nil, fmt.Errorf("chat: unreachable projection state %q", p.State)
	}
}

func validateLivePayloadType(rt chatEntity.ResourceOccurrenceResourceType, payload ResourceProjectionPayload) error {
	switch rt {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		switch payload.(type) {
		case ProfileLivePayload, *ProfileLivePayload:
			return nil
		default:
			return fmt.Errorf("chat: LIVE profile requires ProfileLivePayload, got %T", payload)
		}
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		switch payload.(type) {
		case ContentLivePayload, *ContentLivePayload:
			return nil
		default:
			return fmt.Errorf("chat: LIVE content requires ContentLivePayload, got %T", payload)
		}
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		switch payload.(type) {
		case ForSaleLivePayload, *ForSaleLivePayload:
			return nil
		default:
			return fmt.Errorf("chat: LIVE FPS requires ForSaleLivePayload, got %T", payload)
		}
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		switch payload.(type) {
		case AuctionLivePayload, *AuctionLivePayload:
			return nil
		default:
			return fmt.Errorf("chat: LIVE auction requires AuctionLivePayload, got %T", payload)
		}
	default:
		return fmt.Errorf("chat: invalid resource type %q", rt)
	}
}

func validateLiveCommerceActions(
	rt chatEntity.ResourceOccurrenceResourceType,
	commerceActions *CommerceActionCapabilities,
) error {
	switch rt {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		if commerceActions != nil {
			return fmt.Errorf("chat: LIVE profile projection requires nil commerce_actions")
		}
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		if commerceActions != nil {
			return fmt.Errorf("chat: LIVE content projection requires nil commerce_actions")
		}
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		if commerceActions == nil {
			return fmt.Errorf("chat: LIVE FPS projection requires non-nil commerce_actions")
		}
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		if commerceActions == nil {
			return fmt.Errorf("chat: LIVE auction projection requires non-nil commerce_actions")
		}
	default:
		return fmt.Errorf("chat: invalid resource type %q", rt)
	}
	return nil
}

func validateViewerCapabilities(
	state ProjectionState,
	rt chatEntity.ResourceOccurrenceResourceType,
	caps ProjectionViewerCapabilities,
	commerceActions *CommerceActionCapabilities,
) error {
	switch state {
	case ProjectionStateLive:
		if !caps.CanView {
			return fmt.Errorf("chat: LIVE projection requires can_view=true")
		}
		if caps.BlockedByTombstone {
			return fmt.Errorf("chat: LIVE projection requires blocked_by_tombstone=false")
		}
		switch rt {
		case chatEntity.ResourceOccurrenceResourceTypeProfile, chatEntity.ResourceOccurrenceResourceTypeContent:
			if caps.CanInteract {
				return fmt.Errorf("chat: LIVE %s projection requires can_interact=false", rt)
			}
		case chatEntity.ResourceOccurrenceResourceTypeForSale, chatEntity.ResourceOccurrenceResourceTypeAuction:
			if caps.CanInteract {
				if commerceActions == nil {
					return fmt.Errorf("chat: can_interact=true requires commerce_actions")
				}
				if !commerceActions.CanBuy && !commerceActions.CanBid && !commerceActions.CanNegotiate {
					return fmt.Errorf("chat: can_interact=true requires at least one actionable commerce flag")
				}
			}
		default:
			return fmt.Errorf("chat: invalid resource type %q", rt)
		}
	case ProjectionStateTombstone:
		if caps.CanView {
			return fmt.Errorf("chat: TOMBSTONE projection requires can_view=false")
		}
		if caps.CanInteract {
			return fmt.Errorf("chat: TOMBSTONE projection requires can_interact=false")
		}
		if !caps.BlockedByTombstone {
			return fmt.Errorf("chat: TOMBSTONE projection requires blocked_by_tombstone=true")
		}
	default:
		return fmt.Errorf("chat: invalid projection state %q", state)
	}

	return nil
}

func marshalLiveJSON(p ResourceProjection) ([]byte, error) {
	url, err := CanonicalResourceURL(p.Identity.ResourceType, p.Identity.ResourceID)
	if err != nil {
		return nil, fmt.Errorf("chat: LIVE marshal: %w", err)
	}

	aux := struct {
		State              string                       `json:"state"`
		ResourceType       string                       `json:"resource_type"`
		ResourceID         string                       `json:"resource_id"`
		CanonicalURL       string                       `json:"canonical_url"`
		ViewerCapabilities ProjectionViewerCapabilities `json:"viewer_capabilities"`
		CommerceActions    *CommerceActionCapabilities  `json:"commerce_actions,omitempty"`
		Profile            *ProfileLivePayload          `json:"profile,omitempty"`
		Content            *ContentLivePayload          `json:"content,omitempty"`
		ForSale            *ForSaleLivePayload          `json:"for_sale,omitempty"`
		Auction            *AuctionLivePayload          `json:"auction,omitempty"`
	}{
		State:              string(p.State),
		ResourceType:       string(p.Identity.ResourceType),
		ResourceID:         p.Identity.ResourceID.String(),
		CanonicalURL:       url,
		ViewerCapabilities: p.ViewerCapabilities,
		CommerceActions:    p.CommerceActions,
	}

	switch payload := p.Payload.(type) {
	case ProfileLivePayload:
		aux.Profile = &payload
	case *ProfileLivePayload:
		aux.Profile = payload
	case ContentLivePayload:
		aux.Content = &payload
	case *ContentLivePayload:
		aux.Content = payload
	case ForSaleLivePayload:
		aux.ForSale = &payload
	case *ForSaleLivePayload:
		aux.ForSale = payload
	case AuctionLivePayload:
		aux.Auction = &payload
	case *AuctionLivePayload:
		aux.Auction = payload
	default:
		return nil, fmt.Errorf("chat: LIVE marshal received unknown payload type %T", p.Payload)
	}

	return json.Marshal(aux)
}

// ResourceProjectionResolver resolves occurrence rows into viewer-aware
// resource projections.
//
// The resolver is batch-oriented: all occurrences for a message page are
// resolved together. Implementation lives outside the application package.
type ResourceProjectionResolver interface {
	ResolveResourceProjections(
		ctx context.Context,
		viewerID uuid.UUID,
		occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
	) (map[uuid.UUID]*ResourceProjection, error)
}

func marshalTombstoneJSON(p ResourceProjection) ([]byte, error) {
	aux := struct {
		State              string                       `json:"state"`
		ResourceType       string                       `json:"resource_type"`
		ViewerCapabilities ProjectionViewerCapabilities `json:"viewer_capabilities"`
	}{
		State:              string(p.State),
		ResourceType:       string(p.Identity.ResourceType),
		ViewerCapabilities: p.ViewerCapabilities,
	}
	return json.Marshal(aux)
}
