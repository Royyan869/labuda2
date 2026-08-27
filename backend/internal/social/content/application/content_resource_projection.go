package application

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	contententity "github.com/labuda/backend/internal/social/content/entity"
)

// ProjectionState is the locked lifecycle state for a canonical resource
// projection envelope.
type ProjectionState string

const (
	ProjectionStateLive      ProjectionState = "LIVE"
	ProjectionStateTombstone ProjectionState = "TOMBSTONE"
)

// NestedResourceIndicator captures the single depth-1 nested identity for a
// content projection. It intentionally carries identity only.
type NestedResourceIndicator struct {
	ResourceType contententity.ContentResourceOccurrenceResourceType `json:"resource_type"`
	ResourceID   uuid.UUID                                           `json:"resource_id"`
}

// ProfileLivePayload is the live payload for a canonical profile projection.
type ProfileLivePayload struct {
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	Lifecycle string  `json:"lifecycle"`
}

// ContentLivePayload is the live payload for a canonical content projection.
type ContentLivePayload struct {
	Caption        *string                  `json:"caption"`
	Media          []mediaref.MediaRef      `json:"media"`
	Lifecycle      string                   `json:"lifecycle"`
	CreatedAt      string                   `json:"created_at"`
	Author         publiccard.UserCard      `json:"author"`
	NestedResource *NestedResourceIndicator `json:"nested_resource,omitempty"`
}

// ForSaleLivePayload is the live payload for a canonical fixed-price sale
// projection.
type ForSaleLivePayload struct {
	Title             string                `json:"title"`
	Media             []mediaref.MediaRef   `json:"media"`
	ThumbnailURL      *string               `json:"thumbnail_url,omitempty"`
	Price             int64                 `json:"price"`
	Status            string                `json:"status"`
	QuantityAvailable int                   `json:"quantity_available"`
	CanInteract       bool                  `json:"can_interact"`
	Seller            publiccard.SellerCard `json:"seller"`
}

// AuctionLivePayload is the live payload for a canonical auction projection.
type AuctionLivePayload struct {
	Title        string                `json:"title"`
	Media        []mediaref.MediaRef   `json:"media"`
	ThumbnailURL *string               `json:"thumbnail_url,omitempty"`
	CurrentBid   *int64                `json:"current_bid,omitempty"`
	BuyNowPrice  *int64                `json:"buy_now_price,omitempty"`
	EndAt        string                `json:"end_at"`
	Lifecycle    string                `json:"lifecycle"`
	CanInteract  bool                  `json:"can_interact"`
	Seller       publiccard.SellerCard `json:"seller"`
}

// ContentResourceProjection is the canonical public projection envelope used by
// content detail, feed, search, create, and update responses.
type ContentResourceProjection struct {
	State        ProjectionState                                     `json:"state"`
	ResourceType contententity.ContentResourceOccurrenceResourceType `json:"resource_type"`
	ResourceID   uuid.UUID                                           `json:"resource_id"`

	Profile        *ProfileLivePayload        `json:"profile,omitempty"`
	Content        *ContentLivePayload        `json:"content,omitempty"`
	ForSale *ForSaleLivePayload `json:"for_sale,omitempty"`
	Auction        *AuctionLivePayload        `json:"auction,omitempty"`
}

// NewTombstoneContentResourceProjection builds a safe tombstone envelope.
func NewTombstoneContentResourceProjection(rt contententity.ContentResourceOccurrenceResourceType, resourceID uuid.UUID) (ContentResourceProjection, error) {
	if !rt.IsValid() {
		return ContentResourceProjection{}, fmt.Errorf("content: invalid resource type %q", rt)
	}
	if resourceID == uuid.Nil {
		return ContentResourceProjection{}, fmt.Errorf("content: tombstone projection requires resource id")
	}
	p := ContentResourceProjection{
		State:        ProjectionStateTombstone,
		ResourceType: rt,
		ResourceID:   resourceID,
	}
	if err := p.Validate(); err != nil {
		return ContentResourceProjection{}, err
	}
	return p, nil
}

// NewLiveContentResourceProjection builds a LIVE envelope.
func NewLiveContentResourceProjection(rt contententity.ContentResourceOccurrenceResourceType, resourceID uuid.UUID, payload any) (ContentResourceProjection, error) {
	p := ContentResourceProjection{
		State:        ProjectionStateLive,
		ResourceType: rt,
		ResourceID:   resourceID,
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
		return ContentResourceProjection{}, fmt.Errorf("content: unsupported payload type %T", payload)
	}
	if err := p.Validate(); err != nil {
		return ContentResourceProjection{}, err
	}
	return p, nil
}

// Validate enforces the canonical contract.
func (p ContentResourceProjection) Validate() error {
	if !p.ResourceType.IsValid() {
		return fmt.Errorf("content: invalid resource type %q", p.ResourceType)
	}
	switch p.State {
	case ProjectionStateLive:
		if p.ResourceID == uuid.Nil {
			return fmt.Errorf("content: LIVE projection requires resource id")
		}
		switch p.ResourceType {
		case contententity.ContentResourceOccurrenceResourceTypeProfile:
			if p.Profile == nil || p.Content != nil || p.ForSale != nil || p.Auction != nil {
				return fmt.Errorf("content: LIVE profile projection requires profile payload only")
			}
		case contententity.ContentResourceOccurrenceResourceTypeContent:
			if p.Content == nil || p.Profile != nil || p.ForSale != nil || p.Auction != nil {
				return fmt.Errorf("content: LIVE content projection requires content payload only")
			}
		case contententity.ContentResourceOccurrenceResourceTypeForSale:
			if p.ForSale == nil || p.Profile != nil || p.Content != nil || p.Auction != nil {
				return fmt.Errorf("content: LIVE fixed price sale projection requires for_sale payload only")
			}
		case contententity.ContentResourceOccurrenceResourceTypeAuction:
			if p.Auction == nil || p.Profile != nil || p.Content != nil || p.ForSale != nil {
				return fmt.Errorf("content: LIVE auction projection requires auction payload only")
			}
		default:
			return fmt.Errorf("content: invalid resource type %q", p.ResourceType)
		}
	case ProjectionStateTombstone:
		if p.ResourceID == uuid.Nil {
			return fmt.Errorf("content: TOMBSTONE projection requires resource id")
		}
		if p.Profile != nil || p.Content != nil || p.ForSale != nil || p.Auction != nil {
			return fmt.Errorf("content: TOMBSTONE projection must not carry payload")
		}
	default:
		return fmt.Errorf("content: invalid projection state %q", p.State)
	}
	return nil
}

// MarshalJSON emits the strict state-specific envelope.
func (p ContentResourceProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	switch p.State {
	case ProjectionStateLive:
		aux := struct {
			State          string                     `json:"state"`
			ResourceType   string                     `json:"resource_type"`
			ResourceID     string                     `json:"resource_id"`
			Profile        *ProfileLivePayload        `json:"profile,omitempty"`
			Content        *ContentLivePayload        `json:"content,omitempty"`
			ForSale *ForSaleLivePayload `json:"for_sale,omitempty"`
			Auction        *AuctionLivePayload        `json:"auction,omitempty"`
		}{
			State:          string(p.State),
			ResourceType:   string(p.ResourceType),
			ResourceID:     p.ResourceID.String(),
			Profile:        p.Profile,
			Content:        p.Content,
			ForSale: p.ForSale,
			Auction:        p.Auction,
		}
		return json.Marshal(aux)
	case ProjectionStateTombstone:
		aux := struct {
			State        string `json:"state"`
			ResourceType string `json:"resource_type"`
			ResourceID   string `json:"resource_id"`
		}{
			State:        string(p.State),
			ResourceType: string(p.ResourceType),
			ResourceID:   p.ResourceID.String(),
		}
		return json.Marshal(aux)
	default:
		return nil, fmt.Errorf("content: unreachable projection state %q", p.State)
	}
}

// buildPublicUserCard builds a public-safe user card from raw truth.
func buildPublicUserCard(id uuid.UUID, username string, avatarURL *string, accountStatus string, deleted bool) publiccard.UserCard {
	lifecycle := string(viewercontext.CoarsenLifecycle(accountStatus, deleted))
	card := publiccard.UserCard{ID: id, Username: username, AvatarURL: avatarURL}
	if lifecycle != "" {
		card.Lifecycle = &lifecycle
	}
	return card
}

func resolveMediaRefs(urls []string) []mediaref.MediaRef {
	refs := make([]mediaref.MediaRef, 0, len(urls))
	for _, raw := range urls {
		if trimmed := resolveMediaReference(raw); trimmed != "" {
			refs = append(refs, mediaref.MediaRef{URL: trimmed})
		}
	}
	return refs
}

func firstResolvedMediaURL(urls []string) *string {
	for _, raw := range urls {
		if trimmed := resolveMediaReference(raw); trimmed != "" {
			v := trimmed
			return &v
		}
	}
	return nil
}

func resolveMediaReference(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		if resolved, err := mediaresolve.ResolveMediaReadURL(trimmed); err == nil {
			return resolved
		}
		return trimmed
	}
	return ""
}
