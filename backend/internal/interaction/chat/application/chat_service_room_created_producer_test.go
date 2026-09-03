package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/pkg/rate"
	"go.uber.org/zap"
)

func TestRoomCreatedProducer_EmitsForNewRooms(t *testing.T) {
	ctx := context.Background()
	type testCase struct {
		name              string
		room              *chatEntity.ChatRoom
		call              func(*Service) (*chatEntity.ChatRoom, error)
		getDirectRoomErr  error
		getSupportRoomErr error
		wantRecipients    []uuid.UUID
	}

	directA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	directB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	supportUser := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	cases := []testCase{
		{
			name: "direct",
			room: &chatEntity.ChatRoom{
				ID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				RoomType:     chatEntity.RoomTypeDirect,
				ParticipantA: directA,
				ParticipantB: directB,
			},
			call: func(s *Service) (*chatEntity.ChatRoom, error) {
				return s.GetOrCreateDirectRoom(ctx, directA, directB)
			},
			getDirectRoomErr: chatRepo.ErrRoomNotFound,
			wantRecipients:   []uuid.UUID{directA, directB},
		},
		{
			name: "support",
			room: &chatEntity.ChatRoom{
				ID:           uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
				RoomType:     chatEntity.RoomTypeSupport,
				ParticipantA: uuid.Nil,
				ParticipantB: supportUser,
			},
			call: func(s *Service) (*chatEntity.ChatRoom, error) {
				return s.GetOrCreateSupportRoom(ctx, supportUser)
			},
			getSupportRoomErr: chatRepo.ErrRoomNotFound,
			wantRecipients:    []uuid.UUID{supportUser},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repo := &roomUpdatedMockRepo{
				room:              tt.room,
				getDirectRoomErr:  tt.getDirectRoomErr,
				getSupportRoomErr: tt.getSupportRoomErr,
			}
			outbox := &roomUpdatedMockOutbox{}
			service := &Service{
				db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
				repo:        repo,
				socialRepo:  &roomUpdatedMockSocialRepo{},
				outboxRepo:  outbox,
				rateLimiter: rate.NewRateLimiter(),
				log:         zap.NewNop(),
			}

			room, err := tt.call(service)
			if err != nil {
				t.Fatalf("call failed: %v", err)
			}
			if room == nil {
				t.Fatal("expected room result")
			}
			if repo.createRoomCalls != 1 {
				t.Fatalf("CreateRoom calls=%d want 1", repo.createRoomCalls)
			}
			if len(outbox.inserts) != len(tt.wantRecipients) {
				t.Fatalf("outbox inserts=%d want %d", len(outbox.inserts), len(tt.wantRecipients))
			}

			gotByRecipient := map[string]roomUpdatedOutboxInsert{}
			for _, insert := range outbox.inserts {
				if insert.eventType != "chat.room.created" {
					t.Fatalf("event type=%q want chat.room.created", insert.eventType)
				}
				recipientID := requireOutboxFieldStringReturn(t, insert.payload, "recipient_id")
				gotByRecipient[recipientID] = insert
			}

			for _, recipientID := range tt.wantRecipients {
				insert, ok := gotByRecipient[recipientID.String()]
				if !ok {
					t.Fatalf("missing room.created event for recipient %s", recipientID)
				}
				requireOutboxFieldString(t, insert.payload, "recipient_id", recipientID.String())
				requireOutboxFieldString(t, insert.payload, "room_id", room.ID.String())
				requireOutboxFieldString(t, insert.payload, "room_type", string(room.RoomType))
				requireOutboxFieldString(t, insert.payload, "other_user_id", room.OtherParticipant(recipientID).String())
				requireOutboxFieldInt(t, insert.payload, "unread_count", 0)
				requireOutboxFieldString(t, insert.payload, "created_at", room.CreatedAt.UTC().Format(time.RFC3339))
				requireOutboxFieldString(t, insert.payload, "updated_at", room.UpdatedAt.UTC().Format(time.RFC3339))
				requireOutboxFieldString(t, insert.payload, "last_message_at", room.LastMessageAt.UTC().Format(time.RFC3339))
				if lastMessage, ok := insert.payload["last_message"]; !ok || lastMessage != nil {
					t.Fatalf("last_message=%v want nil", lastMessage)
				}
			}
		})
	}
}

func TestGetOrCreateDirectRoom_ExistingRoomDoesNotEmitChatRoomCreated(t *testing.T) {
	senderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := &roomUpdatedMockRepo{
		room: &chatEntity.ChatRoom{
			ID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
			RoomType:     chatEntity.RoomTypeDirect,
			ParticipantA: senderID,
			ParticipantB: recipientID,
		},
	}
	outbox := &roomUpdatedMockOutbox{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	room, err := service.GetOrCreateDirectRoom(context.Background(), senderID, recipientID)
	if err != nil {
		t.Fatalf("GetOrCreateDirectRoom failed: %v", err)
	}
	if room == nil {
		t.Fatal("expected room result")
	}
	if repo.createRoomCalls != 0 {
		t.Fatalf("CreateRoom calls=%d want 0", repo.createRoomCalls)
	}
	if len(outbox.inserts) != 0 {
		t.Fatalf("outbox inserts=%d want 0", len(outbox.inserts))
	}
}

func TestGetOrCreateDirectRoom_FailedCreationDoesNotEmitChatRoomCreated(t *testing.T) {
	senderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := &roomUpdatedMockRepo{
		room: &chatEntity.ChatRoom{
			ID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
			RoomType:     chatEntity.RoomTypeDirect,
			ParticipantA: senderID,
			ParticipantB: recipientID,
		},
		getDirectRoomErr: chatRepo.ErrRoomNotFound,
		createRoomErr:    errors.New("create room failed"),
	}
	outbox := &roomUpdatedMockOutbox{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	_, err := service.GetOrCreateDirectRoom(context.Background(), senderID, recipientID)
	if err == nil {
		t.Fatal("expected error")
	}
	if repo.createRoomCalls != 1 {
		t.Fatalf("CreateRoom calls=%d want 1", repo.createRoomCalls)
	}
	if len(outbox.inserts) != 0 {
		t.Fatalf("outbox inserts=%d want 0", len(outbox.inserts))
	}
}

func TestGetOrCreateDirectRoom_BlockedCreationDoesNotEmitChatRoomCreated(t *testing.T) {
	senderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := &roomUpdatedMockRepo{
		room: &chatEntity.ChatRoom{
			ID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
			RoomType:     chatEntity.RoomTypeDirect,
			ParticipantA: senderID,
			ParticipantB: recipientID,
		},
		getDirectRoomErr: chatRepo.ErrRoomNotFound,
	}
	outbox := &roomUpdatedMockOutbox{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{blocked: true},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	_, err := service.GetOrCreateDirectRoom(context.Background(), senderID, recipientID)
	if !errors.Is(err, chatRepo.ErrUserBlocked) {
		t.Fatalf("err=%v want ErrUserBlocked", err)
	}
	if repo.createRoomCalls != 0 {
		t.Fatalf("CreateRoom calls=%d want 0", repo.createRoomCalls)
	}
	if len(outbox.inserts) != 0 {
		t.Fatalf("outbox inserts=%d want 0", len(outbox.inserts))
	}
}

func TestAutoLinkOrderToDirectRoom_NewRoomCarriesLinkedOrderInRoomCreatedPayload(t *testing.T) {
	buyerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sellerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	orderID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	repo := &roomUpdatedMockRepo{
		room: &chatEntity.ChatRoom{
			ID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
			RoomType:     chatEntity.RoomTypeDirect,
			ParticipantA: buyerID,
			ParticipantB: sellerID,
		},
		getDirectRoomErr: chatRepo.ErrRoomNotFound,
	}
	outbox := &roomUpdatedMockOutbox{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	room, err := service.AutoLinkOrderToDirectRoom(context.Background(), buyerID, sellerID, orderID)
	if err != nil {
		t.Fatalf("AutoLinkOrderToDirectRoom failed: %v", err)
	}
	if room == nil {
		t.Fatal("expected room result")
	}
	if len(outbox.inserts) != 2 {
		t.Fatalf("outbox inserts=%d want 2", len(outbox.inserts))
	}
	for _, insert := range outbox.inserts {
		if insert.eventType != "chat.room.created" {
			t.Fatalf("event type=%q want chat.room.created", insert.eventType)
		}
		requireOutboxFieldString(t, insert.payload, "linked_order_id", orderID.String())
		requireOutboxFieldString(t, insert.payload, "room_id", room.ID.String())
		requireOutboxFieldInt(t, insert.payload, "unread_count", 0)
		if insert.payload["last_message"] != nil {
			t.Fatalf("last_message=%v want nil", insert.payload["last_message"])
		}
	}
}
