package serverboot

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/stretchr/testify/require"
)

type profileProjectionResolverStub struct {
	calls   int
	last    map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence
	outputs map[uuid.UUID]*chatApp.ResourceProjection
	err     error
}

func (s *profileProjectionResolverStub) ResolveProfiles(
	_ context.Context,
	_ uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return resolveProjectionStub(&s.calls, &s.last, s.outputs, s.err, occurrences)
}

type contentProjectionResolverStub struct {
	calls   int
	last    map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence
	outputs map[uuid.UUID]*chatApp.ResourceProjection
	err     error
}

func (s *contentProjectionResolverStub) ResolveContents(
	_ context.Context,
	_ uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return resolveProjectionStub(&s.calls, &s.last, s.outputs, s.err, occurrences)
}

type forSaleProjectionResolverStub struct {
	calls   int
	last    map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence
	outputs map[uuid.UUID]*chatApp.ResourceProjection
	err     error
}

func (s *forSaleProjectionResolverStub) ResolveForSales(
	_ context.Context,
	_ uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return resolveProjectionStub(&s.calls, &s.last, s.outputs, s.err, occurrences)
}

type auctionProjectionResolverStub struct {
	calls   int
	last    map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence
	outputs map[uuid.UUID]*chatApp.ResourceProjection
	err     error
}

func (s *auctionProjectionResolverStub) ResolveAuctions(
	_ context.Context,
	_ uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return resolveProjectionStub(&s.calls, &s.last, s.outputs, s.err, occurrences)
}

type aggregateHarness struct {
	resolver *resourceProjectionAggregateResolver
	profile  *profileProjectionResolverStub
	content  *contentProjectionResolverStub
	fps      *forSaleProjectionResolverStub
	auction  *auctionProjectionResolverStub
}

func newAggregateHarness() *aggregateHarness {
	profile := &profileProjectionResolverStub{}
	content := &contentProjectionResolverStub{}
	fps := &forSaleProjectionResolverStub{}
	auction := &auctionProjectionResolverStub{}

	return &aggregateHarness{
		resolver: newResourceProjectionAggregateResolver(profile, content, fps, auction),
		profile:  profile,
		content:  content,
		fps:      fps,
		auction:  auction,
	}
}

func (h *aggregateHarness) resolve(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return h.resolver.ResolveResourceProjections(ctx, viewerID, occurrences)
}

func resolveProjectionStub(
	calls *int,
	last *map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
	outputs map[uuid.UUID]*chatApp.ResourceProjection,
	err error,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	*calls = *calls + 1
	*last = cloneOccurrenceMap(occurrences)
	if err != nil {
		return nil, err
	}
	return cloneProjectionMap(outputs), nil
}

func cloneOccurrenceMap(in map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence) map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence {
	out := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneProjectionMap(in map[uuid.UUID]*chatApp.ResourceProjection) map[uuid.UUID]*chatApp.ResourceProjection {
	out := make(map[uuid.UUID]*chatApp.ResourceProjection, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func newOccurrence(
	messageID uuid.UUID,
	resourceType chatEntity.ResourceOccurrenceResourceType,
	resourceID uuid.UUID,
) *chatEntity.ChatMessageResourceOccurrence {
	return chatEntity.NewChatMessageResourceOccurrence(
		messageID,
		chatEntity.ResourceOccurrenceOperationShareToChat,
		resourceType,
		resourceID,
		nil,
	)
}

func buildOccurrences(
	resourceType chatEntity.ResourceOccurrenceResourceType,
	resourceIDs []uuid.UUID,
) map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence {
	occurrences := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		messageID := uuid.New()
		occurrences[messageID] = newOccurrence(messageID, resourceType, resourceID)
	}
	return occurrences
}

func buildRepeatedOccurrences(
	resourceType chatEntity.ResourceOccurrenceResourceType,
	resourceID uuid.UUID,
	count int,
) map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence {
	resourceIDs := make([]uuid.UUID, count)
	for i := range resourceIDs {
		resourceIDs[i] = resourceID
	}
	return buildOccurrences(resourceType, resourceIDs)
}

func combineOccurrenceMaps(maps ...map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence) map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence {
	total := 0
	for _, m := range maps {
		total += len(m)
	}
	out := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, total)
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

func projectMapFromOccurrences(
	t *testing.T,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
	build func(messageID uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection,
) map[uuid.UUID]*chatApp.ResourceProjection {
	t.Helper()
	out := make(map[uuid.UUID]*chatApp.ResourceProjection, len(occurrences))
	for messageID, occurrence := range occurrences {
		out[messageID] = build(messageID, occurrence)
	}
	return out
}

func mustProfileLiveProjection(t *testing.T, resourceID uuid.UUID) *chatApp.ResourceProjection {
	t.Helper()
	proj, err := chatApp.NewLiveProjection(
		chatEntity.ResourceOccurrenceResourceTypeProfile,
		resourceID,
		chatApp.ProfileLivePayload{
			Username:  "profile-" + resourceID.String()[:8],
			Lifecycle: "active",
		},
		chatApp.ProjectionViewerCapabilities{
			CanView:            true,
			CanInteract:        false,
			BlockedByTombstone: false,
		},
		nil,
	)
	require.NoError(t, err)
	return &proj
}

func mustProfileTombstoneProjection(t *testing.T) *chatApp.ResourceProjection {
	t.Helper()
	proj, err := chatApp.NewTombstoneProjection(chatEntity.ResourceOccurrenceResourceTypeProfile)
	require.NoError(t, err)
	return &proj
}

func mustContentLiveProjection(t *testing.T, resourceID uuid.UUID) *chatApp.ResourceProjection {
	t.Helper()
	proj, err := chatApp.NewLiveProjection(
		chatEntity.ResourceOccurrenceResourceTypeContent,
		resourceID,
		chatApp.ContentLivePayload{
			Caption:   nil,
			Media:     []mediaref.MediaRef{},
			Lifecycle: "active",
			CreatedAt: time.Date(2026, time.August, 9, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			Author:    publiccard.NewWithLifecycle(uuid.New(), "author", nil, "active"),
		},
		chatApp.ProjectionViewerCapabilities{
			CanView:            true,
			CanInteract:        false,
			BlockedByTombstone: false,
		},
		nil,
	)
	require.NoError(t, err)
	return &proj
}

func mustContentTombstoneProjection(t *testing.T) *chatApp.ResourceProjection {
	t.Helper()
	proj, err := chatApp.NewTombstoneProjection(chatEntity.ResourceOccurrenceResourceTypeContent)
	require.NoError(t, err)
	return &proj
}

func mustForSaleLiveProjection(t *testing.T, resourceID uuid.UUID) *chatApp.ResourceProjection {
	t.Helper()
	sellerID := uuid.New()
	seller := chatApp.ForSaleLiveSeller{
		ID:        sellerID,
		StoreName: "store-" + sellerID.String()[:8],
		Username:  "seller-" + sellerID.String()[:8],
		Lifecycle: "active",
	}
	proj, err := chatApp.NewLiveProjection(
		chatEntity.ResourceOccurrenceResourceTypeForSale,
		resourceID,
		chatApp.ForSaleLivePayload{
			Title:             "sale-" + resourceID.String()[:8],
			Price:             chatApp.ForSaleLivePrice{Amount: 1000, Currency: "IDR"},
			Status:            "active",
			Seller:            seller,
			QuantityAvailable: 1,
		},
		chatApp.ProjectionViewerCapabilities{
			CanView:            true,
			CanInteract:        true,
			BlockedByTombstone: false,
		},
		&chatApp.CommerceActionCapabilities{
			CanChat:      true,
			CanNegotiate: true,
			CanBuy:       true,
			CanBid:       false,
			CanManage:    false,
		},
	)
	require.NoError(t, err)
	return &proj
}

func mustForSaleTombstoneProjection(t *testing.T) *chatApp.ResourceProjection {
	t.Helper()
	proj, err := chatApp.NewTombstoneProjection(chatEntity.ResourceOccurrenceResourceTypeForSale)
	require.NoError(t, err)
	return &proj
}

func mustAuctionLiveProjection(t *testing.T, resourceID uuid.UUID) *chatApp.ResourceProjection {
	t.Helper()
	sellerID := uuid.New()
	seller := publiccard.NewSellerCardWithUserLifecycle(sellerID, "seller-"+sellerID.String()[:8], nil, "farm-"+sellerID.String()[:8], "active")
	proj, err := chatApp.NewLiveProjection(
		chatEntity.ResourceOccurrenceResourceTypeAuction,
		resourceID,
		chatApp.AuctionLivePayload{
			Title:     "auction-" + resourceID.String()[:8],
			EndAt:     time.Date(2026, time.August, 9, 1, 0, 0, 0, time.UTC).Format(time.RFC3339),
			Lifecycle: strPtr("active"),
			Seller:    &seller,
		},
		chatApp.ProjectionViewerCapabilities{
			CanView:            true,
			CanInteract:        true,
			BlockedByTombstone: false,
		},
		&chatApp.CommerceActionCapabilities{
			CanChat:      true,
			CanNegotiate: false,
			CanBuy:       false,
			CanBid:       true,
			CanManage:    false,
		},
	)
	require.NoError(t, err)
	return &proj
}

func mustAuctionTombstoneProjection(t *testing.T) *chatApp.ResourceProjection {
	t.Helper()
	proj, err := chatApp.NewTombstoneProjection(chatEntity.ResourceOccurrenceResourceTypeAuction)
	require.NoError(t, err)
	return &proj
}

func strPtr(v string) *string {
	return &v
}

func expectCalls(t *testing.T, h *aggregateHarness, profile, content, fps, auction int) {
	t.Helper()
	require.Equal(t, profile, h.profile.calls, "profile resolver call count")
	require.Equal(t, content, h.content.calls, "content resolver call count")
	require.Equal(t, fps, h.fps.calls, "fixed price sale resolver call count")
	require.Equal(t, auction, h.auction.calls, "auction resolver call count")
}

func assertProjectionMatchesOccurrence(
	t *testing.T,
	projection *chatApp.ResourceProjection,
	occurrence *chatEntity.ChatMessageResourceOccurrence,
) {
	t.Helper()
	require.NotNil(t, projection)
	require.NoError(t, projection.Validate())
	require.Equal(t, occurrence.ResourceType(), projection.Identity.ResourceType)
	switch projection.State {
	case chatApp.ProjectionStateTombstone:
		require.Equal(t, uuid.Nil, projection.Identity.ResourceID)
		require.Nil(t, projection.Payload)
		require.Nil(t, projection.CommerceActions)
		require.False(t, projection.ViewerCapabilities.CanView)
		require.False(t, projection.ViewerCapabilities.CanInteract)
		require.True(t, projection.ViewerCapabilities.BlockedByTombstone)
	default:
		require.Equal(t, occurrence.SourceID(), projection.Identity.ResourceID)
	}
}

func TestResourceProjectionAggregateResolver_Matrix(t *testing.T) {
	ctx := context.Background()
	viewerID := uuid.New()

	t.Run("G1 empty occurrences", func(t *testing.T) {
		h := newAggregateHarness()
		got, err := h.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{})
		require.NoError(t, err)
		require.Empty(t, got)
		expectCalls(t, h, 0, 0, 0, 0)
	})

	t.Run("G2 one profile", func(t *testing.T) {
		h := newAggregateHarness()
		profileSourceID := uuid.New()
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, []uuid.UUID{profileSourceID})
		h.profile.outputs = projectMapFromOccurrences(t, occurrences, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustProfileLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, 1)
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		expectCalls(t, h, 1, 0, 0, 0)
	})

	t.Run("G3 twenty profiles", func(t *testing.T) {
		h := newAggregateHarness()
		resourceIDs := make([]uuid.UUID, 20)
		for i := range resourceIDs {
			resourceIDs[i] = uuid.New()
		}
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, resourceIDs)
		h.profile.outputs = projectMapFromOccurrences(t, occurrences, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustProfileLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, len(occurrences))
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		expectCalls(t, h, 1, 0, 0, 0)
	})

	t.Run("G4 one content", func(t *testing.T) {
		h := newAggregateHarness()
		resourceID := uuid.New()
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, []uuid.UUID{resourceID})
		h.content.outputs = projectMapFromOccurrences(t, occurrences, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustContentLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, 1)
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		expectCalls(t, h, 0, 1, 0, 0)
	})

	t.Run("G5 twenty contents", func(t *testing.T) {
		h := newAggregateHarness()
		resourceIDs := make([]uuid.UUID, 20)
		for i := range resourceIDs {
			resourceIDs[i] = uuid.New()
		}
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, resourceIDs)
		h.content.outputs = projectMapFromOccurrences(t, occurrences, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustContentLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, len(occurrences))
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		expectCalls(t, h, 0, 1, 0, 0)
	})

	t.Run("G6 one fixed price sale", func(t *testing.T) {
		h := newAggregateHarness()
		resourceID := uuid.New()
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, []uuid.UUID{resourceID})
		h.fps.outputs = projectMapFromOccurrences(t, occurrences, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustForSaleLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, 1)
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		expectCalls(t, h, 0, 0, 1, 0)
	})

	t.Run("G7 twenty fixed price sales", func(t *testing.T) {
		h := newAggregateHarness()
		resourceIDs := make([]uuid.UUID, 20)
		for i := range resourceIDs {
			resourceIDs[i] = uuid.New()
		}
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, resourceIDs)
		h.fps.outputs = projectMapFromOccurrences(t, occurrences, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustForSaleLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, len(occurrences))
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		expectCalls(t, h, 0, 0, 1, 0)
	})

	t.Run("G8 one auction", func(t *testing.T) {
		h := newAggregateHarness()
		resourceID := uuid.New()
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeAuction, []uuid.UUID{resourceID})
		h.auction.outputs = projectMapFromOccurrences(t, occurrences, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustAuctionLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, 1)
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		expectCalls(t, h, 0, 0, 0, 1)
	})

	t.Run("G9 twenty auctions", func(t *testing.T) {
		h := newAggregateHarness()
		resourceIDs := make([]uuid.UUID, 20)
		for i := range resourceIDs {
			resourceIDs[i] = uuid.New()
		}
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeAuction, resourceIDs)
		h.auction.outputs = projectMapFromOccurrences(t, occurrences, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustAuctionLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, len(occurrences))
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		expectCalls(t, h, 0, 0, 0, 1)
	})

	t.Run("G10 one of each four types", func(t *testing.T) {
		h := newAggregateHarness()
		profileOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, []uuid.UUID{uuid.New()})
		contentOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, []uuid.UUID{uuid.New()})
		fpsOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, []uuid.UUID{uuid.New()})
		auctionOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeAuction, []uuid.UUID{uuid.New()})
		occurrences := combineOccurrenceMaps(profileOcc, contentOcc, fpsOcc, auctionOcc)

		h.profile.outputs = projectMapFromOccurrences(t, profileOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustProfileLiveProjection(t, occurrence.SourceID())
		})
		h.content.outputs = projectMapFromOccurrences(t, contentOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustContentLiveProjection(t, occurrence.SourceID())
		})
		h.fps.outputs = projectMapFromOccurrences(t, fpsOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustForSaleLiveProjection(t, occurrence.SourceID())
		})
		h.auction.outputs = projectMapFromOccurrences(t, auctionOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustAuctionLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, len(occurrences))
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		expectCalls(t, h, 1, 1, 1, 1)
	})

	t.Run("G11 large mixed page with all four types", func(t *testing.T) {
		h := newAggregateHarness()
		profileOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, makeUUIDs(8))
		contentOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, makeUUIDs(8))
		fpsOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, makeUUIDs(8))
		auctionOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeAuction, makeUUIDs(8))
		occurrences := combineOccurrenceMaps(profileOcc, contentOcc, fpsOcc, auctionOcc)

		h.profile.outputs = projectMapFromOccurrences(t, profileOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustProfileLiveProjection(t, occurrence.SourceID())
		})
		h.content.outputs = projectMapFromOccurrences(t, contentOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustContentLiveProjection(t, occurrence.SourceID())
		})
		h.fps.outputs = projectMapFromOccurrences(t, fpsOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustForSaleLiveProjection(t, occurrence.SourceID())
		})
		h.auction.outputs = projectMapFromOccurrences(t, auctionOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustAuctionLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, len(occurrences))
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		expectCalls(t, h, 1, 1, 1, 1)
	})

	t.Run("G12 same source referenced by multiple messages", func(t *testing.T) {
		h := newAggregateHarness()
		sharedSourceID := uuid.New()
		occurrences := buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, sharedSourceID, 20)
		h.profile.outputs = projectMapFromOccurrences(t, occurrences, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustProfileLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, len(occurrences))
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
			require.Equal(t, sharedSourceID, projection.Identity.ResourceID)
		}
		expectCalls(t, h, 1, 0, 0, 0)
	})

	t.Run("G13 mix live and tombstone", func(t *testing.T) {
		h := newAggregateHarness()
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, []uuid.UUID{uuid.New(), uuid.New()})
		h.content.outputs = map[uuid.UUID]*chatApp.ResourceProjection{}
		idx := 0
		for messageID, occurrence := range occurrences {
			if idx == 0 {
				h.content.outputs[messageID] = mustContentLiveProjection(t, occurrence.SourceID())
			} else {
				h.content.outputs[messageID] = mustContentTombstoneProjection(t)
			}
			idx++
		}

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, len(occurrences))
		liveCount := 0
		tombstoneCount := 0
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
			switch projection.State {
			case chatApp.ProjectionStateLive:
				liveCount++
			case chatApp.ProjectionStateTombstone:
				tombstoneCount++
			}
		}
		require.Equal(t, 1, liveCount)
		require.Equal(t, 1, tombstoneCount)
		expectCalls(t, h, 0, 1, 0, 0)
	})

	t.Run("G14 profile resolver failure", func(t *testing.T) {
		h := newAggregateHarness()
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, []uuid.UUID{uuid.New()})
		h.profile.err = errors.New("profile boom")

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, got)
		expectCalls(t, h, 1, 0, 0, 0)
	})

	t.Run("G15 content resolver failure", func(t *testing.T) {
		h := newAggregateHarness()
		profileOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, []uuid.UUID{uuid.New()})
		contentOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, []uuid.UUID{uuid.New()})
		occurrences := combineOccurrenceMaps(profileOcc, contentOcc)
		h.profile.outputs = projectMapFromOccurrences(t, profileOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustProfileLiveProjection(t, occurrence.SourceID())
		})
		h.content.err = errors.New("content boom")

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, got)
		expectCalls(t, h, 1, 1, 0, 0)
	})

	t.Run("G16 fixed price sale resolver failure", func(t *testing.T) {
		h := newAggregateHarness()
		profileOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, []uuid.UUID{uuid.New()})
		contentOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, []uuid.UUID{uuid.New()})
		fpsOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, []uuid.UUID{uuid.New()})
		occurrences := combineOccurrenceMaps(profileOcc, contentOcc, fpsOcc)
		h.profile.outputs = projectMapFromOccurrences(t, profileOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustProfileLiveProjection(t, occurrence.SourceID())
		})
		h.content.outputs = projectMapFromOccurrences(t, contentOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustContentLiveProjection(t, occurrence.SourceID())
		})
		h.fps.err = errors.New("fps boom")

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, got)
		expectCalls(t, h, 1, 1, 1, 0)
	})

	t.Run("G17 auction resolver failure", func(t *testing.T) {
		h := newAggregateHarness()
		profileOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, []uuid.UUID{uuid.New()})
		contentOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, []uuid.UUID{uuid.New()})
		fpsOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, []uuid.UUID{uuid.New()})
		auctionOcc := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeAuction, []uuid.UUID{uuid.New()})
		occurrences := combineOccurrenceMaps(profileOcc, contentOcc, fpsOcc, auctionOcc)
		h.profile.outputs = projectMapFromOccurrences(t, profileOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustProfileLiveProjection(t, occurrence.SourceID())
		})
		h.content.outputs = projectMapFromOccurrences(t, contentOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustContentLiveProjection(t, occurrence.SourceID())
		})
		h.fps.outputs = projectMapFromOccurrences(t, fpsOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustForSaleLiveProjection(t, occurrence.SourceID())
		})
		h.auction.err = errors.New("auction boom")

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, got)
		expectCalls(t, h, 1, 1, 1, 1)
	})

	t.Run("G18 child omits requested message", func(t *testing.T) {
		h := newAggregateHarness()
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, []uuid.UUID{uuid.New(), uuid.New()})
		h.profile.outputs = map[uuid.UUID]*chatApp.ResourceProjection{}
		for messageID, occurrence := range occurrences {
			h.profile.outputs[messageID] = mustProfileLiveProjection(t, occurrence.SourceID())
			break
		}

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, got)
		require.Contains(t, err.Error(), "omitted projection")
		expectCalls(t, h, 1, 0, 0, 0)
	})

	t.Run("G19 child returns unexpected message id", func(t *testing.T) {
		h := newAggregateHarness()
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, []uuid.UUID{uuid.New()})
		h.profile.outputs = projectMapFromOccurrences(t, occurrences, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustProfileLiveProjection(t, occurrence.SourceID())
		})
		h.profile.outputs[uuid.New()] = mustProfileLiveProjection(t, uuid.New())

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, got)
		require.Contains(t, err.Error(), "unexpected projection")
		expectCalls(t, h, 1, 0, 0, 0)
	})

	t.Run("G20 child returns wrong live resource type", func(t *testing.T) {
		h := newAggregateHarness()
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, []uuid.UUID{uuid.New()})
		messageID := onlyMessageID(t, occurrences)
		h.profile.outputs = map[uuid.UUID]*chatApp.ResourceProjection{
			messageID: mustContentLiveProjection(t, uuid.New()),
		}

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, got)
		require.Contains(t, err.Error(), "resource type mismatch")
		expectCalls(t, h, 1, 0, 0, 0)
	})

	t.Run("G21 child returns wrong live source id", func(t *testing.T) {
		h := newAggregateHarness()
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, []uuid.UUID{uuid.New()})
		messageID := onlyMessageID(t, occurrences)
		h.fps.outputs = map[uuid.UUID]*chatApp.ResourceProjection{
			messageID: mustForSaleLiveProjection(t, uuid.New()),
		}

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, got)
		require.Contains(t, err.Error(), "resource id mismatch")
		expectCalls(t, h, 0, 0, 1, 0)
	})

	t.Run("G22 tombstone resource type mismatch", func(t *testing.T) {
		h := newAggregateHarness()
		occurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeAuction, []uuid.UUID{uuid.New()})
		messageID := onlyMessageID(t, occurrences)
		h.auction.outputs = map[uuid.UUID]*chatApp.ResourceProjection{
			messageID: mustForSaleTombstoneProjection(t),
		}

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, got)
		require.Contains(t, err.Error(), "resource type mismatch")
		expectCalls(t, h, 0, 0, 0, 1)
	})

	t.Run("G23 unsupported resource type", func(t *testing.T) {
		h := newAggregateHarness()
		messageID := uuid.New()
		occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			messageID: &chatEntity.ChatMessageResourceOccurrence{},
		}

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, got)
		require.Contains(t, err.Error(), "malformed occurrence identity")
		expectCalls(t, h, 0, 0, 0, 0)
	})

	t.Run("G24 mixed repeated sources across types", func(t *testing.T) {
		h := newAggregateHarness()
		sharedProfileID := uuid.New()
		sharedContentID := uuid.New()
		sharedFPSID := uuid.New()
		sharedAuctionID := uuid.New()

		profileOcc := buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, sharedProfileID, 2)
		contentOcc := buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, sharedContentID, 2)
		fpsOcc := buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, sharedFPSID, 2)
		auctionOcc := buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeAuction, sharedAuctionID, 2)
		occurrences := combineOccurrenceMaps(profileOcc, contentOcc, fpsOcc, auctionOcc)

		h.profile.outputs = projectMapFromOccurrences(t, profileOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustProfileLiveProjection(t, occurrence.SourceID())
		})
		h.content.outputs = projectMapFromOccurrences(t, contentOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustContentLiveProjection(t, occurrence.SourceID())
		})
		h.fps.outputs = projectMapFromOccurrences(t, fpsOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustForSaleLiveProjection(t, occurrence.SourceID())
		})
		h.auction.outputs = projectMapFromOccurrences(t, auctionOcc, func(_ uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *chatApp.ResourceProjection {
			return mustAuctionLiveProjection(t, occurrence.SourceID())
		})

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, len(occurrences))
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		require.Equal(t, sharedProfileID, projectionByTypeAndState(t, got, chatEntity.ResourceOccurrenceResourceTypeProfile, chatApp.ProjectionStateLive).Identity.ResourceID)
		require.Equal(t, sharedContentID, projectionByTypeAndState(t, got, chatEntity.ResourceOccurrenceResourceTypeContent, chatApp.ProjectionStateLive).Identity.ResourceID)
		require.Equal(t, sharedFPSID, projectionByTypeAndState(t, got, chatEntity.ResourceOccurrenceResourceTypeForSale, chatApp.ProjectionStateLive).Identity.ResourceID)
		require.Equal(t, sharedAuctionID, projectionByTypeAndState(t, got, chatEntity.ResourceOccurrenceResourceTypeAuction, chatApp.ProjectionStateLive).Identity.ResourceID)
		expectCalls(t, h, 1, 1, 1, 1)
	})

	t.Run("G25 ordering and pagination independent association", func(t *testing.T) {
		h := newAggregateHarness()
		entries := []struct {
			resourceType chatEntity.ResourceOccurrenceResourceType
			resourceID   uuid.UUID
		}{
			{chatEntity.ResourceOccurrenceResourceTypeAuction, uuid.New()},
			{chatEntity.ResourceOccurrenceResourceTypeForSale, uuid.New()},
			{chatEntity.ResourceOccurrenceResourceTypeContent, uuid.New()},
			{chatEntity.ResourceOccurrenceResourceTypeProfile, uuid.New()},
		}

		occurrences := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, len(entries))
		for i := len(entries) - 1; i >= 0; i-- {
			entry := entries[i]
			messageID := uuid.New()
			occurrences[messageID] = newOccurrence(messageID, entry.resourceType, entry.resourceID)
		}
		for messageID, occurrence := range occurrences {
			switch occurrence.ResourceType() {
			case chatEntity.ResourceOccurrenceResourceTypeProfile:
				h.profile.outputs = ensureProjectionMap(t, h.profile.outputs)
				h.profile.outputs[messageID] = mustProfileLiveProjection(t, occurrence.SourceID())
			case chatEntity.ResourceOccurrenceResourceTypeContent:
				h.content.outputs = ensureProjectionMap(t, h.content.outputs)
				h.content.outputs[messageID] = mustContentLiveProjection(t, occurrence.SourceID())
			case chatEntity.ResourceOccurrenceResourceTypeForSale:
				h.fps.outputs = ensureProjectionMap(t, h.fps.outputs)
				h.fps.outputs[messageID] = mustForSaleLiveProjection(t, occurrence.SourceID())
			case chatEntity.ResourceOccurrenceResourceTypeAuction:
				h.auction.outputs = ensureProjectionMap(t, h.auction.outputs)
				h.auction.outputs[messageID] = mustAuctionLiveProjection(t, occurrence.SourceID())
			}
		}

		got, err := h.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err)
		require.Len(t, got, len(occurrences))
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		expectCalls(t, h, 1, 1, 1, 1)
	})
}

func makeUUIDs(count int) []uuid.UUID {
	out := make([]uuid.UUID, count)
	for i := range out {
		out[i] = uuid.New()
	}
	return out
}

func onlyMessageID(t *testing.T, occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence) uuid.UUID {
	t.Helper()
	require.Len(t, occurrences, 1)
	for messageID := range occurrences {
		return messageID
	}
	t.Fatalf("missing message id")
	return uuid.Nil
}

func ensureProjectionMap(
	t *testing.T,
	m map[uuid.UUID]*chatApp.ResourceProjection,
) map[uuid.UUID]*chatApp.ResourceProjection {
	t.Helper()
	if m == nil {
		return map[uuid.UUID]*chatApp.ResourceProjection{}
	}
	return m
}

func projectionByTypeAndState(
	t *testing.T,
	projections map[uuid.UUID]*chatApp.ResourceProjection,
	resourceType chatEntity.ResourceOccurrenceResourceType,
	state chatApp.ProjectionState,
) *chatApp.ResourceProjection {
	t.Helper()
	for _, projection := range projections {
		if projection.Identity.ResourceType == resourceType && projection.State == state {
			return projection
		}
	}
	t.Fatalf("missing projection for type %s and state %s", resourceType, state)
	return nil
}
