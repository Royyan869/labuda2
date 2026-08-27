package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	socialRepo "github.com/labuda/backend/internal/social/graph"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"go.uber.org/zap"
)

// ============================================================================
// PASS_8A / F4 — proves the consumer sends proposal messages directly into
// the room referenced by chat_room_id (the buyer/seller's direct room),
// with no separate room_type=negotiation room ever created or resolved.
// ============================================================================

type negotiationFakeChatRepo struct {
	room             *chatEntity.ChatRoom
	createRoomCalls  int
	createMessageArg *chatEntity.ChatMessage
}

func (r *negotiationFakeChatRepo) CreateRoom(context.Context, interface{}, *chatEntity.ChatRoom) error {
	r.createRoomCalls++
	return nil
}
func (r *negotiationFakeChatRepo) GetRoomByID(_ context.Context, _ interface{}, roomID uuid.UUID) (*chatEntity.ChatRoom, error) {
	if r.room == nil || r.room.ID != roomID {
		return nil, chatRepo.ErrRoomNotFound
	}
	return r.room, nil
}
func (r *negotiationFakeChatRepo) GetRoomByIDForUpdate(_ context.Context, _ interface{}, roomID uuid.UUID) (*chatEntity.ChatRoom, error) {
	if r.room == nil || r.room.ID != roomID {
		return nil, chatRepo.ErrRoomNotFound
	}
	return r.room, nil
}
func (r *negotiationFakeChatRepo) GetDirectRoom(context.Context, interface{}, uuid.UUID, uuid.UUID) (*chatEntity.ChatRoom, error) {
	return nil, chatRepo.ErrRoomNotFound
}
func (r *negotiationFakeChatRepo) GetSupportRoom(context.Context, interface{}, uuid.UUID) (*chatEntity.ChatRoom, error) {
	return nil, chatRepo.ErrRoomNotFound
}
func (r *negotiationFakeChatRepo) ListRoomsByUser(context.Context, interface{}, uuid.UUID, *time.Time, *uuid.UUID, int) ([]*chatEntity.ChatRoom, error) {
	return nil, nil
}
func (r *negotiationFakeChatRepo) GetRoomByOrderID(context.Context, interface{}, uuid.UUID) (*chatEntity.ChatRoom, error) {
	return nil, chatRepo.ErrRoomNotFound
}
func (r *negotiationFakeChatRepo) UpdateRoomLastMessageAt(context.Context, interface{}, uuid.UUID, time.Time) error {
	return nil
}
func (r *negotiationFakeChatRepo) UpdateRoomContext(context.Context, interface{}, uuid.UUID, json.RawMessage, uuid.UUID) error {
	return nil
}
func (r *negotiationFakeChatRepo) UpdateRoomLinkedOrderId(context.Context, interface{}, uuid.UUID, *uuid.UUID) error {
	return nil
}
func (r *negotiationFakeChatRepo) CreateMessage(_ context.Context, _ interface{}, message *chatEntity.ChatMessage) error {
	r.createMessageArg = message
	return nil
}
func (r *negotiationFakeChatRepo) GetMessageByID(context.Context, interface{}, uuid.UUID) (*chatEntity.ChatMessage, error) {
	return nil, chatRepo.ErrMessageNotFound
}
func (r *negotiationFakeChatRepo) ListMessagesByRoom(context.Context, interface{}, uuid.UUID, *time.Time, *uuid.UUID, int) ([]*chatEntity.ChatMessage, error) {
	return nil, nil
}
func (r *negotiationFakeChatRepo) GetMessageByIdempotencyKey(context.Context, interface{}, string) (*chatEntity.ChatMessage, error) {
	return nil, chatRepo.ErrMessageNotFound
}
func (r *negotiationFakeChatRepo) SoftHideForModeration(context.Context, interface{}, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (r *negotiationFakeChatRepo) RestoreFromModeration(context.Context, interface{}, uuid.UUID) error {
	return nil
}
func (r *negotiationFakeChatRepo) CreateReadState(context.Context, interface{}, *chatEntity.ChatReadState) error {
	return nil
}
func (r *negotiationFakeChatRepo) GetReadState(context.Context, interface{}, uuid.UUID, uuid.UUID) (*chatEntity.ChatReadState, error) {
	return nil, chatRepo.ErrReadStateNotFound
}
func (r *negotiationFakeChatRepo) UpdateReadState(context.Context, interface{}, *chatEntity.ChatReadState) error {
	return nil
}
func (r *negotiationFakeChatRepo) UpsertReadState(context.Context, interface{}, *chatEntity.ChatReadState) error {
	return nil
}
func (r *negotiationFakeChatRepo) ListReadStatesByRoom(context.Context, interface{}, uuid.UUID) ([]*chatEntity.ChatReadState, error) {
	return nil, nil
}
func (r *negotiationFakeChatRepo) GetUnreadCountByRoomAndUser(context.Context, interface{}, uuid.UUID, uuid.UUID) (int, error) {
	return 0, nil
}

var _ chatRepo.Repository = (*negotiationFakeChatRepo)(nil)

type negotiationFakeSocialRepo struct{}

func (f *negotiationFakeSocialRepo) InsertFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *negotiationFakeSocialRepo) DeleteFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *negotiationFakeSocialRepo) DeleteFollowBothDirections(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *negotiationFakeSocialRepo) ExistsFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (f *negotiationFakeSocialRepo) AcquireFollowLock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *negotiationFakeSocialRepo) IsBlockedBy(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (f *negotiationFakeSocialRepo) ListFollowers(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *negotiationFakeSocialRepo) ListFollowing(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *negotiationFakeSocialRepo) InsertBlock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *negotiationFakeSocialRepo) DeleteBlock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *negotiationFakeSocialRepo) ExistsBlock(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (f *negotiationFakeSocialRepo) ListBlocked(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *negotiationFakeSocialRepo) InsertMute(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *negotiationFakeSocialRepo) DeleteMute(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *negotiationFakeSocialRepo) ExistsMute(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (f *negotiationFakeSocialRepo) ListMuted(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}

var _ socialRepo.SocialRepository = (*negotiationFakeSocialRepo)(nil)

type negotiationFakeOutbox struct{}

func (f *negotiationFakeOutbox) InsertTx(context.Context, db.Tx, string, any, string) error {
	return nil
}

type negotiationFakeTransactor struct{}

func (f *negotiationFakeTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(nil)
}

func buildNegotiationLinkageTestHandler(t *testing.T, repo *negotiationFakeChatRepo) *NegotiationEventHandler {
	t.Helper()
	svc := chatApp.NewService(
		&negotiationFakeTransactor{},
		repo,
		&negotiationFakeSocialRepo{},
		&negotiationFakeOutbox{},
		rate.NewRateLimiter(),
		nil, nil, nil, nil, nil,
		zap.NewNop(),
	)
	return NewNegotiationEventHandler(&negotiationFakeTransactor{}, svc, zap.NewNop())
}

// TestHandleNegotiationStarted_SendsMessageIntoChatRoomID proves the F4 fix:
// the proposal message is sent into the direct room from chat_room_id, and
// no room is ever created (createRoomCalls stays 0).
func TestHandleNegotiationStarted_SendsMessageIntoChatRoomID(t *testing.T) {
	buyerID, sellerID, roomID := uuid.New(), uuid.New(), uuid.New()
	room := &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID}
	repo := &negotiationFakeChatRepo{room: room}
	h := buildNegotiationLinkageTestHandler(t, repo)

	payload, _ := json.Marshal(NegotiationStartedPayload{
		SessionID:    uuid.New(),
		ChatRoomID:   roomID.String(),
		ResourceType: "for_sale",
		ResourceID:   uuid.New(),
		BuyerID:      buyerID,
		SellerID:     sellerID,
		InitialPrice: 400000,
	})

	err := h.HandleNegotiationStarted(context.Background(), uuid.New(), payload)
	if err != nil {
		t.Fatalf("HandleNegotiationStarted() error = %v", err)
	}

	if repo.createRoomCalls != 0 {
		t.Fatalf("createRoomCalls = %d, want 0 — F4 regression: a duplicate negotiation room was created", repo.createRoomCalls)
	}
	if repo.createMessageArg == nil {
		t.Fatal("expected a message to be created")
	}
	if repo.createMessageArg.RoomID != roomID {
		t.Fatalf("message room_id = %s, want %s (the direct room)", repo.createMessageArg.RoomID, roomID)
	}
}

// TestHandleNegotiationMessageSent_SendsMessageIntoChatRoomID mirrors the
// above for counter-offers.
func TestHandleNegotiationMessageSent_SendsMessageIntoChatRoomID(t *testing.T) {
	buyerID, sellerID, roomID := uuid.New(), uuid.New(), uuid.New()
	room := &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID}
	repo := &negotiationFakeChatRepo{room: room}
	h := buildNegotiationLinkageTestHandler(t, repo)

	payload, _ := json.Marshal(NegotiationMessageSentPayload{
		SessionID:        uuid.New(),
		ChatRoomID:       roomID.String(),
		BuyerID:          buyerID,
		SellerID:         sellerID,
		SenderID:         sellerID,
		Price:            380000,
		ProposalSequence: 2,
	})

	err := h.HandleNegotiationMessageSent(context.Background(), uuid.New(), payload)
	if err != nil {
		t.Fatalf("HandleNegotiationMessageSent() error = %v", err)
	}

	if repo.createRoomCalls != 0 {
		t.Fatalf("createRoomCalls = %d, want 0 — F4 regression", repo.createRoomCalls)
	}
	if repo.createMessageArg == nil || repo.createMessageArg.RoomID != roomID {
		t.Fatalf("message room_id mismatch: got %+v, want room %s", repo.createMessageArg, roomID)
	}
}

// TestHandleNegotiationStarted_MissingChatRoomIdFails proves that a missing
// or invalid chat_room_id fails loudly (retryable outbox error) rather than
// silently falling back to creating an orphaned room.
func TestHandleNegotiationStarted_MissingChatRoomIdFails(t *testing.T) {
	h := buildNegotiationLinkageTestHandler(t, &negotiationFakeChatRepo{})

	payload, _ := json.Marshal(NegotiationStartedPayload{
		SessionID: uuid.New(),
		BuyerID:   uuid.New(),
		SellerID:  uuid.New(),
		// ChatRoomID intentionally omitted
	})

	err := h.HandleNegotiationStarted(context.Background(), uuid.New(), payload)
	if err == nil {
		t.Fatal("expected error for missing chat_room_id, got nil")
	}
}

func TestHandleNegotiationMessageSent_MissingChatRoomIdFails(t *testing.T) {
	h := buildNegotiationLinkageTestHandler(t, &negotiationFakeChatRepo{})

	payload, _ := json.Marshal(NegotiationMessageSentPayload{
		SessionID: uuid.New(),
		BuyerID:   uuid.New(),
		SellerID:  uuid.New(),
		SenderID:  uuid.New(),
		// ChatRoomID intentionally omitted
	})

	err := h.HandleNegotiationMessageSent(context.Background(), uuid.New(), payload)
	if err == nil {
		t.Fatal("expected error for missing chat_room_id, got nil")
	}
}
