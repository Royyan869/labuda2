package application

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type unknownProjectionPayload struct{}

func (unknownProjectionPayload) resourceProjectionPayloadMarker() {}

func validViewerCaps(canInteract bool) ProjectionViewerCapabilities {
	return ProjectionViewerCapabilities{
		CanView:            true,
		CanInteract:        canInteract,
		BlockedByTombstone: false,
	}
}

func tombstoneViewerCaps() ProjectionViewerCapabilities {
	return ProjectionViewerCapabilities{
		CanView:            false,
		CanInteract:        false,
		BlockedByTombstone: true,
	}
}

func validCommerceActions(canInteract bool) *CommerceActionCapabilities {
	if !canInteract {
		return &CommerceActionCapabilities{
			Role:         "owner",
			CanChat:      true,
			CanNegotiate: false,
			CanBuy:       false,
			CanBid:       false,
			CanManage:    true,
		}
	}
	return &CommerceActionCapabilities{
		Role:         "buyer",
		CanChat:      true,
		CanNegotiate: true,
		CanBuy:       true,
		CanBid:       false,
		CanManage:    false,
	}
}

func marshalToMap(t *testing.T, v any) map[string]json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)

	var out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

func requireKeys(t *testing.T, got map[string]json.RawMessage, expected []string) {
	t.Helper()
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sort.Strings(expected)
	require.Equal(t, expected, keys)
}

func mustLiveProjection(
	t *testing.T,
	rt chatEntity.ResourceOccurrenceResourceType,
	payload ResourceProjectionPayload,
	viewerCaps ProjectionViewerCapabilities,
	commerceActions *CommerceActionCapabilities,
) ResourceProjection {
	t.Helper()
	proj, err := NewLiveProjection(rt, uuid.New(), payload, viewerCaps, commerceActions)
	require.NoError(t, err)
	return proj
}

func mustTombstoneProjection(t *testing.T, rt chatEntity.ResourceOccurrenceResourceType) ResourceProjection {
	t.Helper()
	proj, err := NewTombstoneProjection(rt)
	require.NoError(t, err)
	return proj
}

func TestProjectionStateAndIdentityValidation(t *testing.T) {
	t.Run("invalid state rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: "UNKNOWN",
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: validViewerCaps(false),
			Payload:            ProfileLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid projection state")
	})

	t.Run("invalid resource type rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceType("order"),
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: validViewerCaps(false),
			Payload:            ProfileLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid resource type")
	})

	t.Run("live nil payload rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: validViewerCaps(false),
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIVE projection requires non-nil payload")
	})

	t.Run("live wrong payload type rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: validViewerCaps(false),
			Payload:            AuctionLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIVE profile requires ProfileLivePayload")
	})

	t.Run("live uuid nil rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
			},
			ViewerCapabilities: validViewerCaps(false),
			Payload:            ProfileLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIVE projection requires non-nil resource ID")
	})

	t.Run("live can_view false rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: ProjectionViewerCapabilities{
				CanView:            false,
				CanInteract:        false,
				BlockedByTombstone: false,
			},
			Payload: ProfileLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIVE projection requires can_view=true")
	})

	t.Run("live tombstone flag true rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: ProjectionViewerCapabilities{
				CanView:            true,
				CanInteract:        false,
				BlockedByTombstone: true,
			},
			Payload: ProfileLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIVE projection requires blocked_by_tombstone=false")
	})

	t.Run("profile live commerce actions non-nil rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: validViewerCaps(false),
			CommerceActions:    validCommerceActions(false),
			Payload:            ProfileLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIVE profile projection requires nil commerce_actions")
	})

	t.Run("content live commerce actions non-nil rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeContent,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: validViewerCaps(false),
			CommerceActions:    validCommerceActions(false),
			Payload:            ContentLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIVE content projection requires nil commerce_actions")
	})

	t.Run("fps live commerce actions nil rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: validViewerCaps(false),
			Payload:            ForSaleLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIVE FPS projection requires non-nil commerce_actions")
	})

	t.Run("auction live commerce actions nil rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeAuction,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: validViewerCaps(false),
			Payload:            AuctionLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIVE auction projection requires non-nil commerce_actions")
	})

	t.Run("can_interact true plus nil commerce actions rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: validViewerCaps(true),
			Payload:            ForSaleLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires non-nil commerce_actions")
	})

	t.Run("can_interact true plus no actionable flags rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: validViewerCaps(true),
			CommerceActions: &CommerceActionCapabilities{
				Role:         "buyer",
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanManage:    true,
			},
			Payload: ForSaleLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least one actionable commerce flag")
	})

	t.Run("tombstone non-nil payload rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateTombstone,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
			},
			ViewerCapabilities: tombstoneViewerCaps(),
			Payload:            ProfileLivePayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOMBSTONE projection requires nil payload")
	})

	t.Run("tombstone can_view true rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateTombstone,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
			},
			ViewerCapabilities: ProjectionViewerCapabilities{
				CanView:            true,
				CanInteract:        false,
				BlockedByTombstone: true,
			},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOMBSTONE projection requires can_view=false")
	})

	t.Run("tombstone can_interact true rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateTombstone,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
			},
			ViewerCapabilities: ProjectionViewerCapabilities{
				CanView:            false,
				CanInteract:        true,
				BlockedByTombstone: true,
			},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOMBSTONE projection requires can_interact=false")
	})

	t.Run("tombstone blocked_by_tombstone false rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateTombstone,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
			},
			ViewerCapabilities: ProjectionViewerCapabilities{
				CanView:            false,
				CanInteract:        false,
				BlockedByTombstone: false,
			},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOMBSTONE projection requires blocked_by_tombstone=true")
	})

	t.Run("tombstone commerce actions non-nil rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateTombstone,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
			},
			ViewerCapabilities: tombstoneViewerCaps(),
			CommerceActions:    validCommerceActions(false),
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOMBSTONE projection requires nil commerce_actions")
	})

	t.Run("tombstone invalid resource type rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateTombstone,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceType("order"),
			},
			ViewerCapabilities: tombstoneViewerCaps(),
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid resource type")
	})

	t.Run("nested invalid resource type rejected", func(t *testing.T) {
		nested := NestedResourceIndicator{
			ResourceType: chatEntity.ResourceOccurrenceResourceType("order"),
			ResourceID:   uuid.New(),
		}
		_, err := json.Marshal(nested)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid resource type")
	})

	t.Run("nested nil uuid rejected", func(t *testing.T) {
		nested := NestedResourceIndicator{
			ResourceType: chatEntity.ResourceOccurrenceResourceTypeContent,
		}
		_, err := json.Marshal(nested)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil resource ID")
	})

	t.Run("unknown payload implementation rejected", func(t *testing.T) {
		proj := ResourceProjection{
			State: ProjectionStateLive,
			Identity: ResourceProjectionIdentity{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale,
				ResourceID:   uuid.New(),
			},
			ViewerCapabilities: validViewerCaps(true),
			CommerceActions: &CommerceActionCapabilities{
				CanChat:      true,
				CanNegotiate: true,
				CanBuy:       true,
				CanBid:       false,
				CanManage:    false,
			},
			Payload: unknownProjectionPayload{},
		}
		err := proj.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIVE FPS requires ForSaleLivePayload")
	})
}

func TestProjectionCanonicalURL(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	cases := []struct {
		name    string
		rt      chatEntity.ResourceOccurrenceResourceType
		wantURL string
		wantErr string
	}{
		{name: "profile", rt: chatEntity.ResourceOccurrenceResourceTypeProfile, wantURL: "/user/" + id.String()},
		{name: "content", rt: chatEntity.ResourceOccurrenceResourceTypeContent, wantURL: "/content/" + id.String()},
		{name: "fps", rt: chatEntity.ResourceOccurrenceResourceTypeForSale, wantURL: "/for-sale/" + id.String()},
		{name: "auction", rt: chatEntity.ResourceOccurrenceResourceTypeAuction, wantURL: "/auction/" + id.String()},
		{name: "invalid type", rt: chatEntity.ResourceOccurrenceResourceType("order"), wantErr: "unknown resource type"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CanonicalResourceURL(tc.rt, id)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantURL, got)
		})
	}

	_, err := CanonicalResourceURL(chatEntity.ResourceOccurrenceResourceTypeProfile, uuid.Nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil resource ID")
}

func TestNestedResourceIndicatorJSON(t *testing.T) {
	nestedID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	nested := NestedResourceIndicator{
		ResourceType: chatEntity.ResourceOccurrenceResourceTypeContent,
		ResourceID:   nestedID,
	}

	got := marshalToMap(t, nested)
	requireKeys(t, got, []string{"resource_id", "resource_type"})

	var resourceType string
	require.NoError(t, json.Unmarshal(got["resource_type"], &resourceType))
	assert.Equal(t, "content", resourceType)

	var resourceID string
	require.NoError(t, json.Unmarshal(got["resource_id"], &resourceID))
	assert.Equal(t, nestedID.String(), resourceID)

	_, hasNestedResource := got["nested_resource"]
	assert.False(t, hasNestedResource)
}

func TestResourceProjectionValidationAndSerialization(t *testing.T) {
	cases := []struct {
		name                 string
		proj                 ResourceProjection
		wantState            string
		wantResourceType     string
		wantCanonicalURL     string
		wantPayloadKey       string
		wantCommerceActions  bool
		forbidKeys           []string
		wantTopLevelKeyCount int
	}{
		{
			name:                 "live profile",
			proj:                 mustLiveProjection(t, chatEntity.ResourceOccurrenceResourceTypeProfile, ProfileLivePayload{}, validViewerCaps(false), nil),
			wantState:            "LIVE",
			wantResourceType:     "profile",
			wantCanonicalURL:     "",
			wantPayloadKey:       "profile",
			wantCommerceActions:  false,
			forbidKeys:           []string{"commerce_actions", "content", "for_sale", "auction"},
			wantTopLevelKeyCount: 6,
		},
		{
			name:                 "live content",
			proj:                 mustLiveProjection(t, chatEntity.ResourceOccurrenceResourceTypeContent, ContentLivePayload{}, validViewerCaps(false), nil),
			wantState:            "LIVE",
			wantResourceType:     "content",
			wantPayloadKey:       "content",
			wantCommerceActions:  false,
			forbidKeys:           []string{"commerce_actions", "profile", "for_sale", "auction"},
			wantTopLevelKeyCount: 6,
		},
		{
			name:                 "live fps",
			proj:                 mustLiveProjection(t, chatEntity.ResourceOccurrenceResourceTypeForSale, ForSaleLivePayload{}, validViewerCaps(true), validCommerceActions(true)),
			wantState:            "LIVE",
			wantResourceType:     "for_sale",
			wantPayloadKey:       "for_sale",
			wantCommerceActions:  true,
			forbidKeys:           []string{"profile", "content", "auction"},
			wantTopLevelKeyCount: 7,
		},
		{
			name: "live auction",
			proj: mustLiveProjection(t, chatEntity.ResourceOccurrenceResourceTypeAuction, AuctionLivePayload{}, validViewerCaps(true), &CommerceActionCapabilities{
				Role:         "buyer",
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       true,
				CanManage:    false,
			}),
			wantState:            "LIVE",
			wantResourceType:     "auction",
			wantPayloadKey:       "auction",
			wantCommerceActions:  true,
			forbidKeys:           []string{"profile", "content", "for_sale"},
			wantTopLevelKeyCount: 7,
		},
		{
			name:                 "tombstone profile",
			proj:                 mustTombstoneProjection(t, chatEntity.ResourceOccurrenceResourceTypeProfile),
			wantState:            "TOMBSTONE",
			wantResourceType:     "profile",
			wantPayloadKey:       "",
			wantCommerceActions:  false,
			forbidKeys:           []string{"resource_id", "canonical_url", "commerce_actions", "profile", "content", "for_sale", "auction"},
			wantTopLevelKeyCount: 3,
		},
		{
			name:                 "tombstone content",
			proj:                 mustTombstoneProjection(t, chatEntity.ResourceOccurrenceResourceTypeContent),
			wantState:            "TOMBSTONE",
			wantResourceType:     "content",
			wantPayloadKey:       "",
			wantCommerceActions:  false,
			forbidKeys:           []string{"resource_id", "canonical_url", "commerce_actions", "profile", "content", "for_sale", "auction"},
			wantTopLevelKeyCount: 3,
		},
		{
			name:                 "tombstone fps",
			proj:                 mustTombstoneProjection(t, chatEntity.ResourceOccurrenceResourceTypeForSale),
			wantState:            "TOMBSTONE",
			wantResourceType:     "for_sale",
			wantPayloadKey:       "",
			wantCommerceActions:  false,
			forbidKeys:           []string{"resource_id", "canonical_url", "commerce_actions", "profile", "content", "for_sale", "auction"},
			wantTopLevelKeyCount: 3,
		},
		{
			name:                 "tombstone auction",
			proj:                 mustTombstoneProjection(t, chatEntity.ResourceOccurrenceResourceTypeAuction),
			wantState:            "TOMBSTONE",
			wantResourceType:     "auction",
			wantPayloadKey:       "",
			wantCommerceActions:  false,
			forbidKeys:           []string{"resource_id", "canonical_url", "commerce_actions", "profile", "content", "for_sale", "auction"},
			wantTopLevelKeyCount: 3,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			data := marshalProjectionBytes(t, tc.proj)

			var got map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(data, &got))
			require.Len(t, got, tc.wantTopLevelKeyCount)

			require.Equal(t, tc.wantState, mustStringField(t, got, "state"))
			require.Equal(t, tc.wantResourceType, mustStringField(t, got, "resource_type"))

			if tc.wantState != "TOMBSTONE" {
				require.NotEmpty(t, mustStringField(t, got, "resource_id"))
				require.NotEmpty(t, mustStringField(t, got, "canonical_url"))
			}

			if tc.wantState != "TOMBSTONE" {
				require.NotEmpty(t, mustStringField(t, got, "resource_id"))
				if tc.wantCanonicalURL != "" {
					require.Equal(t, tc.wantCanonicalURL, mustStringField(t, got, "canonical_url"))
				} else {
					expectedURL, err := CanonicalResourceURL(tc.proj.Identity.ResourceType, tc.proj.Identity.ResourceID)
					require.NoError(t, err)
					require.Equal(t, expectedURL, mustStringField(t, got, "canonical_url"))
				}
			} else {
				_, hasID := got["resource_id"]
				assert.False(t, hasID)
				_, hasURL := got["canonical_url"]
				assert.False(t, hasURL)
			}

			var viewerCaps ProjectionViewerCapabilities
			require.NoError(t, json.Unmarshal(got["viewer_capabilities"], &viewerCaps))
			if tc.wantState == "TOMBSTONE" {
				assert.Equal(t, tombstoneViewerCaps(), viewerCaps)
			} else {
				if tc.wantCommerceActions {
					assert.True(t, viewerCaps.CanInteract)
				} else {
					assert.False(t, viewerCaps.CanInteract)
				}
				assert.True(t, viewerCaps.CanView)
				assert.False(t, viewerCaps.BlockedByTombstone)
			}

			if tc.wantState != "TOMBSTONE" {
				_, hasResourceID := got["resource_id"]
				assert.True(t, hasResourceID)
				_, hasCanonicalURL := got["canonical_url"]
				assert.True(t, hasCanonicalURL)
			}

			if tc.wantCommerceActions {
				_, ok := got["commerce_actions"]
				assert.True(t, ok)
			} else {
				_, ok := got["commerce_actions"]
				assert.False(t, ok)
			}

			if tc.wantPayloadKey != "" {
				_, ok := got[tc.wantPayloadKey]
				assert.True(t, ok, "expected payload key %s", tc.wantPayloadKey)
			}
			for _, key := range tc.forbidKeys {
				_, ok := got[key]
				assert.False(t, ok, "forbidden key %s must be absent", key)
			}

			switch tc.wantState {
			case "LIVE":
				if tc.wantPayloadKey == "profile" || tc.wantPayloadKey == "content" {
					_, ok := got["commerce_actions"]
					assert.False(t, ok)
				} else {
					_, ok := got["commerce_actions"]
					assert.True(t, ok)
				}
			case "TOMBSTONE":
				_, ok := got["viewer_capabilities"]
				assert.True(t, ok)
				_, ok = got["commerce_actions"]
				assert.False(t, ok)
			}
		})
	}
}

func TestProfileLivePayloadJSONContract(t *testing.T) {
	avatarURL := "https://cdn.example.test/avatar.png"
	storeName := "Labuda Farm"
	payload := ProfileLivePayload{
		Username:  "labuda",
		AvatarURL: &avatarURL,
		StoreName: &storeName,
		IsSeller:  true,
		Lifecycle: "active",
	}

	got := marshalToMap(t, payload)
	requireKeys(t, got, []string{"avatar_url", "is_seller", "lifecycle", "store_name", "username"})

	require.Equal(t, "labuda", mustStringField(t, got, "username"))
	require.Equal(t, "active", mustStringField(t, got, "lifecycle"))

	var isSeller bool
	require.NoError(t, json.Unmarshal(got["is_seller"], &isSeller))
	assert.True(t, isSeller)

	var gotAvatar string
	require.NoError(t, json.Unmarshal(got["avatar_url"], &gotAvatar))
	assert.Equal(t, avatarURL, gotAvatar)

	var gotStore string
	require.NoError(t, json.Unmarshal(got["store_name"], &gotStore))
	assert.Equal(t, storeName, gotStore)
}

func TestContentLivePayloadJSONContract(t *testing.T) {
	caption := "A public post"
	createdAt := "2026-08-08T10:11:12Z"
	avatarURL := "https://cdn.example.test/author.png"
	lifecycle := "active"
	kind := "image"
	payload := ContentLivePayload{
		Caption:   &caption,
		Media:     []mediaref.MediaRef{{URL: "https://cdn.example.test/media-1.jpg", Kind: &kind}},
		Lifecycle: lifecycle,
		CreatedAt: createdAt,
		Author: publiccard.UserCard{
			ID:        uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			Username:  "author_user",
			AvatarURL: &avatarURL,
			Lifecycle: &lifecycle,
		},
	}

	got := marshalToMap(t, payload)
	requireKeys(t, got, []string{"author", "caption", "created_at", "lifecycle", "media"})

	require.Equal(t, caption, mustStringField(t, got, "caption"))
	require.Equal(t, createdAt, mustStringField(t, got, "created_at"))
	require.Equal(t, lifecycle, mustStringField(t, got, "lifecycle"))

	var media []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got["media"], &media))
	require.Len(t, media, 1)

	var mediaURL string
	require.NoError(t, json.Unmarshal(media[0]["url"], &mediaURL))
	assert.Equal(t, "https://cdn.example.test/media-1.jpg", mediaURL)

	var mediaKind string
	require.NoError(t, json.Unmarshal(media[0]["kind"], &mediaKind))
	assert.Equal(t, kind, mediaKind)

	var author map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got["author"], &author))
	requireKeys(t, author, []string{"avatar_url", "id", "lifecycle", "username"})

	var authorUsername string
	require.NoError(t, json.Unmarshal(author["username"], &authorUsername))
	assert.Equal(t, "author_user", authorUsername)

	var authorLifecycle string
	require.NoError(t, json.Unmarshal(author["lifecycle"], &authorLifecycle))
	assert.Equal(t, lifecycle, authorLifecycle)
}

func TestAuctionLivePayloadJSONContract(t *testing.T) {
	title := "Showa Koi 30cm"
	thumbnail := "https://cdn.example.test/auction.jpg"
	currentBid := int64(1425000)
	buyNow := int64(1750000)
	endAt := "2026-08-09T12:34:56Z"
	lifecycle := "active"
	userLifecycle := "active"
	sellerLifecycle := "active"
	sellerFarmName := "Seller Live Farm"
	sellerAvatar := "https://cdn.example.test/seller-avatar.jpg"

	payload := AuctionLivePayload{
		Title:       title,
		Thumbnail:   &thumbnail,
		CurrentBid:  &currentBid,
		BuyNowPrice: &buyNow,
		EndAt:       endAt,
		Lifecycle:   &lifecycle,
		Seller: &publiccard.SellerCard{
			User: publiccard.UserCard{
				ID:        uuid.MustParse("22222222-2222-2222-2222-222222222222"),
				Username:  "seller-live",
				AvatarURL: &sellerAvatar,
				Lifecycle: &userLifecycle,
			},
			FarmName:  &sellerFarmName,
			AvatarURL: &sellerAvatar,
			Lifecycle: &sellerLifecycle,
		},
	}

	got := marshalToMap(t, payload)
	requireKeys(t, got, []string{"buy_now_price", "current_bid", "end_at", "lifecycle", "seller", "thumbnail_url", "title"})

	require.Equal(t, title, mustStringField(t, got, "title"))
	require.Equal(t, endAt, mustStringField(t, got, "end_at"))
	require.Equal(t, lifecycle, mustStringField(t, got, "lifecycle"))

	var gotThumbnail string
	require.NoError(t, json.Unmarshal(got["thumbnail_url"], &gotThumbnail))
	assert.Equal(t, thumbnail, gotThumbnail)

	var gotCurrentBid int64
	require.NoError(t, json.Unmarshal(got["current_bid"], &gotCurrentBid))
	assert.Equal(t, currentBid, gotCurrentBid)

	var gotBuyNow int64
	require.NoError(t, json.Unmarshal(got["buy_now_price"], &gotBuyNow))
	assert.Equal(t, buyNow, gotBuyNow)

	var seller map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got["seller"], &seller))
	requireKeys(t, seller, []string{"avatar_url", "farm_name", "lifecycle", "user"})

	var user map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(seller["user"], &user))
	requireKeys(t, user, []string{"avatar_url", "id", "lifecycle", "username"})

	require.Equal(t, "seller-live", mustStringField(t, user, "username"))
	require.Equal(t, userLifecycle, mustStringField(t, user, "lifecycle"))
	require.Equal(t, sellerLifecycle, mustStringField(t, seller, "lifecycle"))
	require.Equal(t, sellerFarmName, mustStringField(t, seller, "farm_name"))
	require.Equal(t, sellerAvatar, mustStringField(t, seller, "avatar_url"))
}

func marshalProjectionBytes(t *testing.T, proj ResourceProjection) []byte {
	t.Helper()
	data, err := json.Marshal(proj)
	require.NoError(t, err)
	return data
}

func mustStringField(t *testing.T, got map[string]json.RawMessage, key string) string {
	t.Helper()
	var value string
	require.NoError(t, json.Unmarshal(got[key], &value))
	return value
}

func TestProjectionConstructors(t *testing.T) {
	t.Run("live constructor rejects invalid combination", func(t *testing.T) {
		_, err := NewLiveProjection(
			chatEntity.ResourceOccurrenceResourceTypeProfile,
			uuid.New(),
			ProfileLivePayload{},
			validViewerCaps(true),
			validCommerceActions(true),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIVE profile projection requires nil commerce_actions")
	})

	t.Run("tombstone constructor rejects invalid type", func(t *testing.T) {
		_, err := NewTombstoneProjection(chatEntity.ResourceOccurrenceResourceType("order"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid resource type")
	})
}

func TestNewResourceProjectionIdentity(t *testing.T) {
	id := uuid.New()
	identity, err := NewResourceProjectionIdentity(chatEntity.ResourceOccurrenceResourceTypeContent, id)
	require.NoError(t, err)
	assert.Equal(t, chatEntity.ResourceOccurrenceResourceTypeContent, identity.ResourceType)
	assert.Equal(t, id, identity.ResourceID)

	_, err = NewResourceProjectionIdentity(chatEntity.ResourceOccurrenceResourceType("order"), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid resource type")

	_, err = NewResourceProjectionIdentity(chatEntity.ResourceOccurrenceResourceTypeContent, uuid.Nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-nil resource ID")
}

func TestResourceProjectionMarshalValidationErrors(t *testing.T) {
	proj := ResourceProjection{
		State: ProjectionStateLive,
		Identity: ResourceProjectionIdentity{
			ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale,
			ResourceID:   uuid.New(),
		},
		ViewerCapabilities: validViewerCaps(true),
		CommerceActions: &CommerceActionCapabilities{
			CanChat:      true,
			CanNegotiate: true,
			CanBuy:       true,
		},
		Payload: unknownProjectionPayload{},
	}

	_, err := json.Marshal(proj)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LIVE FPS requires ForSaleLivePayload")
}

func ExampleCanonicalResourceURL() {
	id := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	url, _ := CanonicalResourceURL(chatEntity.ResourceOccurrenceResourceTypeAuction, id)
	fmt.Println(url)
	// Output: /auction/77777777-7777-7777-7777-777777777777
}
