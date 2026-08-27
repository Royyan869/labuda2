package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/pkg/rate"
	"go.uber.org/zap"
)

type roomEventResourceAuthorizerStub struct{}

func (roomEventResourceAuthorizerStub) AuthorizeShare(context.Context, interface{}, uuid.UUID, chatEntity.ResourceOccurrenceResourceType, uuid.UUID) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func (roomEventResourceAuthorizerStub) AuthorizeDirect(context.Context, interface{}, uuid.UUID, chatEntity.ResourceOccurrenceResourceType, uuid.UUID) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func (roomEventResourceAuthorizerStub) BuildFallback(context.Context, interface{}, chatEntity.ResourceOccurrenceResourceType, uuid.UUID) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

type roomEventProjectionResolverStub struct {
	project func(context.Context, uuid.UUID, *chatEntity.ChatMessageResourceOccurrence) (*ResourceProjection, error)
}

func (r roomEventProjectionResolverStub) ResolveResourceProjections(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*ResourceProjection, error) {
	out := make(map[uuid.UUID]*ResourceProjection, len(occurrences))
	for messageID, occurrence := range occurrences {
		if occurrence == nil || r.project == nil {
			continue
		}
		projection, err := r.project(ctx, viewerID, occurrence)
		if err != nil {
			return nil, err
		}
		if projection != nil {
			out[messageID] = projection
		}
	}
	return out, nil
}

func TestSendMessage_EmitsRoomUpdatedResourceProjectionForCanonicalOccurrence(t *testing.T) {
	senderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	room := &chatEntity.ChatRoom{
		ID:            uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		RoomType:      chatEntity.RoomTypeDirect,
		ParticipantA:  senderID,
		ParticipantB:  recipientID,
		CreatedAt:     time.Unix(900, 0).UTC(),
		UpdatedAt:     time.Unix(1000, 0).UTC(),
		LastMessageAt: time.Unix(900, 0).UTC(),
	}

	cases := []struct {
		name         string
		resourceType chatEntity.ResourceOccurrenceResourceType
		resourceID   uuid.UUID
		build        func(t *testing.T, viewerID uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *ResourceProjection
	}{
		{
			name:         "profile",
			resourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
			resourceID:   uuid.MustParse("33333333-3333-3333-3333-333333333333"),
			build: func(t *testing.T, viewerID uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *ResourceProjection {
				t.Helper()
				proj, err := NewLiveProjection(
					occurrence.ResourceType(),
					occurrence.SourceID(),
					ProfileLivePayload{
						Username:  "alice",
						Lifecycle: "active",
					},
					ProjectionViewerCapabilities{
						CanView:            true,
						CanInteract:        false,
						BlockedByTombstone: false,
					},
					nil,
				)
				if err != nil {
					t.Fatalf("NewLiveProjection profile failed: %v", err)
				}
				return &proj
			},
		},
		{
			name:         "content",
			resourceType: chatEntity.ResourceOccurrenceResourceTypeContent,
			resourceID:   uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			build: func(t *testing.T, viewerID uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *ResourceProjection {
				t.Helper()
				proj, err := NewLiveProjection(
					occurrence.ResourceType(),
					occurrence.SourceID(),
					ContentLivePayload{
						Caption:   ptrString("caption"),
						Media:     nil,
						Lifecycle: "active",
						CreatedAt: time.Unix(100, 0).UTC().Format(time.RFC3339),
						Author:    publiccard.New(uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"), "author", nil),
					},
					ProjectionViewerCapabilities{
						CanView:            true,
						CanInteract:        false,
						BlockedByTombstone: false,
					},
					nil,
				)
				if err != nil {
					t.Fatalf("NewLiveProjection content failed: %v", err)
				}
				return &proj
			},
		},
		{
			name:         "for_sale",
			resourceType: chatEntity.ResourceOccurrenceResourceTypeForSale,
			resourceID:   uuid.MustParse("55555555-5555-5555-5555-555555555555"),
			build: func(t *testing.T, viewerID uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *ResourceProjection {
				t.Helper()
				proj, err := NewLiveProjection(
					occurrence.ResourceType(),
					occurrence.SourceID(),
					ForSaleLivePayload{
						Title:             "sale title",
						Price:             ForSaleLivePrice{Amount: 12345, Currency: "IDR"},
						Status:            "active",
						Seller:            ForSaleLiveSeller{ID: viewerID, StoreName: "Store", Username: "seller", Lifecycle: "active"},
						QuantityAvailable: 1,
					},
					ProjectionViewerCapabilities{
						CanView:            true,
						CanInteract:        true,
						BlockedByTombstone: false,
					},
					&CommerceActionCapabilities{
						Role:         "buyer",
						CanChat:      true,
						CanNegotiate: true,
						CanBuy:       true,
					},
				)
				if err != nil {
					t.Fatalf("NewLiveProjection fixed price sale failed: %v", err)
				}
				return &proj
			},
		},
		{
			name:         "auction",
			resourceType: chatEntity.ResourceOccurrenceResourceTypeAuction,
			resourceID:   uuid.MustParse("66666666-6666-6666-6666-666666666666"),
			build: func(t *testing.T, viewerID uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) *ResourceProjection {
				t.Helper()
				proj, err := NewLiveProjection(
					occurrence.ResourceType(),
					occurrence.SourceID(),
					AuctionLivePayload{
						Title:     "auction title",
						EndAt:     time.Unix(200, 0).UTC().Format(time.RFC3339),
						Lifecycle: ptrString("active"),
					},
					ProjectionViewerCapabilities{
						CanView:            true,
						CanInteract:        true,
						BlockedByTombstone: false,
					},
					&CommerceActionCapabilities{
						Role:    "bidder",
						CanChat: true,
						CanBid:  true,
					},
				)
				if err != nil {
					t.Fatalf("NewLiveProjection auction failed: %v", err)
				}
				return &proj
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repo := &roomUpdatedMockRepo{room: room}
			outbox := &roomUpdatedMockOutbox{}
			service := &Service{
				db:                 &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
				repo:               repo,
				socialRepo:         &roomUpdatedMockSocialRepo{},
				outboxRepo:         outbox,
				rateLimiter:        rate.NewRateLimiter(),
				resourceAuthorizer: roomEventResourceAuthorizerStub{},
				log:                zap.NewNop(),
			}
			service.SetResourceProjectionResolver(roomEventProjectionResolverStub{
				project: func(ctx context.Context, viewerID uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) (*ResourceProjection, error) {
					return tt.build(t, viewerID, occurrence), nil
				},
			})

			body := "resource message"
			resourceOccurrence := &chatEntity.ResourceOccurrenceIdentity{
				Operation:    chatEntity.ResourceOccurrenceOperationShareToChat,
				ResourceType: tt.resourceType,
				ResourceID:   tt.resourceID,
			}
			msg, err := service.SendMessage(
				context.Background(),
				room.ID,
				senderID,
				chatEntity.MessageTypeText,
				&body,
				nil,
				nil,
				nil,
				uuid.NewString(),
				resourceOccurrence,
			)
			if err != nil {
				t.Fatalf("SendMessage failed: %v", err)
			}
			if msg == nil {
				t.Fatal("expected message result")
			}
			if len(repo.getResourceOccurrenceRequests) != 1 {
				t.Fatalf("resource occurrence lookups=%d want 1", len(repo.getResourceOccurrenceRequests))
			}
			if got := repo.getResourceOccurrenceRequests[0]; len(got) != 1 || got[0] != msg.ID {
				t.Fatalf("resource occurrence lookup=%v want [%s]", got, msg.ID)
			}

			roomUpdatedByRecipient := map[string]roomUpdatedOutboxInsert{}
			for _, insert := range outbox.inserts {
				switch insert.eventType {
				case "chat.room.updated":
					roomUpdatedByRecipient[requireOutboxFieldStringReturn(t, insert.payload, "recipient_id")] = insert
				case "chat.message.sent":
					if _, ok := insert.payload["resource_projection"]; ok {
						t.Fatal("did not expect resource_projection on chat.message.sent")
					}
				}
			}

			if len(roomUpdatedByRecipient) != 2 {
				t.Fatalf("room.updated recipients=%d want 2", len(roomUpdatedByRecipient))
			}
			for recipientID, insert := range roomUpdatedByRecipient {
				last := requireOutboxFieldMap(t, insert.payload, "last_message")
				rawProjection, ok := last["resource_projection"]
				if !ok {
					t.Fatalf("recipient %s: expected resource_projection", recipientID)
				}
				projection, ok := rawProjection.(ResourceProjection)
				if !ok {
					t.Fatalf("recipient %s: resource_projection type=%T want ResourceProjection", recipientID, rawProjection)
				}
				if projection.State != ProjectionStateLive {
					t.Fatalf("recipient %s: state=%s want LIVE", recipientID, projection.State)
				}
				if projection.Identity.ResourceType != tt.resourceType {
					t.Fatalf("recipient %s: resource_type=%s want %s", recipientID, projection.Identity.ResourceType, tt.resourceType)
				}
				if projection.Identity.ResourceID != tt.resourceID {
					t.Fatalf("recipient %s: resource_id=%s want %s", recipientID, projection.Identity.ResourceID, tt.resourceID)
				}
			}
		})
	}
}

func TestSendMessage_EmitsRoomUpdatedProjectionPerRecipientViewer(t *testing.T) {
	senderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	resourceID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	room := &chatEntity.ChatRoom{
		ID:            uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		RoomType:      chatEntity.RoomTypeDirect,
		ParticipantA:  senderID,
		ParticipantB:  recipientID,
		CreatedAt:     time.Unix(900, 0).UTC(),
		UpdatedAt:     time.Unix(1000, 0).UTC(),
		LastMessageAt: time.Unix(900, 0).UTC(),
	}

	repo := &roomUpdatedMockRepo{room: room}
	outbox := &roomUpdatedMockOutbox{}
	service := &Service{
		db:                 &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:               repo,
		socialRepo:         &roomUpdatedMockSocialRepo{},
		outboxRepo:         outbox,
		rateLimiter:        rate.NewRateLimiter(),
		resourceAuthorizer: roomEventResourceAuthorizerStub{},
		log:                zap.NewNop(),
	}
	service.SetResourceProjectionResolver(roomEventProjectionResolverStub{
		project: func(ctx context.Context, viewerID uuid.UUID, occurrence *chatEntity.ChatMessageResourceOccurrence) (*ResourceProjection, error) {
			if viewerID == senderID {
				proj, err := NewTombstoneProjection(occurrence.ResourceType())
				if err != nil {
					return nil, err
				}
				return &proj, nil
			}

			proj, err := NewLiveProjection(
				occurrence.ResourceType(),
				occurrence.SourceID(),
				ProfileLivePayload{
					Username:  "recipient-view",
					Lifecycle: "active",
				},
				ProjectionViewerCapabilities{
					CanView:            true,
					CanInteract:        false,
					BlockedByTombstone: false,
				},
				nil,
			)
			if err != nil {
				return nil, err
			}
			return &proj, nil
		},
	})

	body := "viewer-specific resource"
	resourceOccurrence := &chatEntity.ResourceOccurrenceIdentity{
		Operation:    chatEntity.ResourceOccurrenceOperationShareToChat,
		ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
		ResourceID:   resourceID,
	}

	msg, err := service.SendMessage(
		context.Background(),
		room.ID,
		senderID,
		chatEntity.MessageTypeText,
		&body,
		nil,
		nil,
		nil,
		uuid.NewString(),
		resourceOccurrence,
	)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message result")
	}
	if len(repo.getResourceOccurrenceRequests) != 1 {
		t.Fatalf("resource occurrence lookups=%d want 1", len(repo.getResourceOccurrenceRequests))
	}

	roomUpdatedByRecipient := map[string]roomUpdatedOutboxInsert{}
	for _, insert := range outbox.inserts {
		if insert.eventType == "chat.room.updated" {
			roomUpdatedByRecipient[requireOutboxFieldStringReturn(t, insert.payload, "recipient_id")] = insert
		}
	}
	if len(roomUpdatedByRecipient) != 2 {
		t.Fatalf("room.updated recipients=%d want 2", len(roomUpdatedByRecipient))
	}

	senderUpdate := roomUpdatedByRecipient[senderID.String()]
	recipientUpdate := roomUpdatedByRecipient[recipientID.String()]

	senderProjection := requireOutboxFieldMap(t, senderUpdate.payload, "last_message")["resource_projection"].(ResourceProjection)
	recipientProjection := requireOutboxFieldMap(t, recipientUpdate.payload, "last_message")["resource_projection"].(ResourceProjection)

	if senderProjection.State != ProjectionStateTombstone {
		t.Fatalf("sender projection state=%s want TOMBSTONE", senderProjection.State)
	}
	if senderProjection.Identity.ResourceID != uuid.Nil {
		t.Fatalf("sender tombstone leaked resource_id=%s", senderProjection.Identity.ResourceID)
	}
	if recipientProjection.State != ProjectionStateLive {
		t.Fatalf("recipient projection state=%s want LIVE", recipientProjection.State)
	}
	if recipientProjection.Identity.ResourceID != resourceID {
		t.Fatalf("recipient projection resource_id=%s want %s", recipientProjection.Identity.ResourceID, resourceID)
	}
}
