package serverboot

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
)

type profileResourceProjectionResolver interface {
	ResolveProfiles(
		ctx context.Context,
		viewerID uuid.UUID,
		occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
	) (map[uuid.UUID]*chatApp.ResourceProjection, error)
}

type contentResourceProjectionResolver interface {
	ResolveContents(
		ctx context.Context,
		viewerID uuid.UUID,
		occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
	) (map[uuid.UUID]*chatApp.ResourceProjection, error)
}

type forSaleResourceProjectionResolver interface {
	ResolveForSales(
		ctx context.Context,
		viewerID uuid.UUID,
		occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
	) (map[uuid.UUID]*chatApp.ResourceProjection, error)
}

type auctionResourceProjectionResolver interface {
	ResolveAuctions(
		ctx context.Context,
		viewerID uuid.UUID,
		occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
	) (map[uuid.UUID]*chatApp.ResourceProjection, error)
}

// resourceProjectionAggregateResolver composes the four canonical type-specific
// batch resolvers without adding any resource policy of its own.
type resourceProjectionAggregateResolver struct {
	profile        profileResourceProjectionResolver
	content        contentResourceProjectionResolver
	forSale forSaleResourceProjectionResolver
	auction        auctionResourceProjectionResolver
}

func newResourceProjectionAggregateResolver(
	profile profileResourceProjectionResolver,
	content contentResourceProjectionResolver,
	forSale forSaleResourceProjectionResolver,
	auction auctionResourceProjectionResolver,
) *resourceProjectionAggregateResolver {
	return &resourceProjectionAggregateResolver{
		profile:        profile,
		content:        content,
		forSale: forSale,
		auction:        auction,
	}
}

var _ chatApp.ResourceProjectionResolver = (*resourceProjectionAggregateResolver)(nil)

func (r *resourceProjectionAggregateResolver) ResolveResourceProjections(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	if len(occurrences) == 0 {
		return map[uuid.UUID]*chatApp.ResourceProjection{}, nil
	}
	if r == nil {
		return nil, fmt.Errorf("chat: resource projection aggregate resolver not configured")
	}

	profileOccurrences := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence)
	contentOccurrences := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence)
	forSaleOccurrences := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence)
	auctionOccurrences := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence)

	for messageID, occurrence := range occurrences {
		if occurrence == nil {
			return nil, fmt.Errorf("chat: nil occurrence for message %s", messageID)
		}

		resourceType := occurrence.ResourceType()
		if !resourceType.IsValid() {
			return nil, fmt.Errorf("chat: malformed occurrence identity for message %s", messageID)
		}

		switch resourceType {
		case chatEntity.ResourceOccurrenceResourceTypeProfile:
			if occurrence.ProfileSourceID == nil {
				return nil, fmt.Errorf("chat: profile occurrence for message %s requires non-nil profile source id", messageID)
			}
			profileOccurrences[messageID] = occurrence
		case chatEntity.ResourceOccurrenceResourceTypeContent:
			if occurrence.ContentSourceID == nil {
				return nil, fmt.Errorf("chat: content occurrence for message %s requires non-nil content source id", messageID)
			}
			contentOccurrences[messageID] = occurrence
		case chatEntity.ResourceOccurrenceResourceTypeForSale:
			if occurrence.ForSaleSourceID == nil {
				return nil, fmt.Errorf("chat: fixed price sale occurrence for message %s requires non-nil fixed price sale source id", messageID)
			}
			forSaleOccurrences[messageID] = occurrence
		case chatEntity.ResourceOccurrenceResourceTypeAuction:
			if occurrence.AuctionSourceID == nil {
				return nil, fmt.Errorf("chat: auction occurrence for message %s requires non-nil auction source id", messageID)
			}
			auctionOccurrences[messageID] = occurrence
		default:
			return nil, fmt.Errorf("chat: unsupported resource type %q in aggregate resolver", resourceType)
		}
	}

	result := make(map[uuid.UUID]*chatApp.ResourceProjection, len(occurrences))

	if len(profileOccurrences) > 0 {
		if r.profile == nil {
			return nil, fmt.Errorf("chat: profile projection resolver not configured")
		}
		resolved, err := r.profile.ResolveProfiles(ctx, viewerID, profileOccurrences)
		if err != nil {
			return nil, err
		}
		if err := mergeResolvedProjections(result, profileOccurrences, resolved); err != nil {
			return nil, err
		}
	}

	if len(contentOccurrences) > 0 {
		if r.content == nil {
			return nil, fmt.Errorf("chat: content projection resolver not configured")
		}
		resolved, err := r.content.ResolveContents(ctx, viewerID, contentOccurrences)
		if err != nil {
			return nil, err
		}
		if err := mergeResolvedProjections(result, contentOccurrences, resolved); err != nil {
			return nil, err
		}
	}

	if len(forSaleOccurrences) > 0 {
		if r.forSale == nil {
			return nil, fmt.Errorf("chat: fixed price sale projection resolver not configured")
		}
		resolved, err := r.forSale.ResolveForSales(ctx, viewerID, forSaleOccurrences)
		if err != nil {
			return nil, err
		}
		if err := mergeResolvedProjections(result, forSaleOccurrences, resolved); err != nil {
			return nil, err
		}
	}

	if len(auctionOccurrences) > 0 {
		if r.auction == nil {
			return nil, fmt.Errorf("chat: auction projection resolver not configured")
		}
		resolved, err := r.auction.ResolveAuctions(ctx, viewerID, auctionOccurrences)
		if err != nil {
			return nil, err
		}
		if err := mergeResolvedProjections(result, auctionOccurrences, resolved); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func mergeResolvedProjections(
	dst map[uuid.UUID]*chatApp.ResourceProjection,
	expected map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
	resolved map[uuid.UUID]*chatApp.ResourceProjection,
) error {
	if len(expected) == 0 {
		return nil
	}

	for messageID, projection := range resolved {
		occurrence, ok := expected[messageID]
		if !ok {
			return fmt.Errorf("chat: resolver returned unexpected projection for message %s", messageID)
		}
		if projection == nil {
			return fmt.Errorf("chat: resolver returned nil projection for message %s", messageID)
		}
		if err := projection.Validate(); err != nil {
			return fmt.Errorf("chat: invalid projection for message %s: %w", messageID, err)
		}

		if projection.Identity.ResourceType != occurrence.ResourceType() {
			return fmt.Errorf(
				"chat: projection resource type mismatch for message %s: got %s want %s",
				messageID,
				projection.Identity.ResourceType,
				occurrence.ResourceType(),
			)
		}

		switch projection.State {
		case chatApp.ProjectionStateTombstone:
			if projection.Identity.ResourceID != uuid.Nil {
				return fmt.Errorf("chat: tombstone projection leaked resource id for message %s", messageID)
			}
		default:
			if projection.Identity.ResourceID != occurrence.SourceID() {
				return fmt.Errorf(
					"chat: projection resource id mismatch for message %s: got %s want %s",
					messageID,
					projection.Identity.ResourceID,
					occurrence.SourceID(),
				)
			}
		}

		dst[messageID] = projection
	}

	for messageID := range expected {
		if _, ok := resolved[messageID]; !ok {
			return fmt.Errorf("chat: resolver omitted projection for message %s", messageID)
		}
	}

	return nil
}
