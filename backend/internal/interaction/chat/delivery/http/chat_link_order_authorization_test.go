package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	socialRepo "github.com/labuda/backend/internal/social/graph"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"go.uber.org/zap"
)

// ========================================================================
// PASS_6A / F1 + F3 — HTTP-boundary tests for LinkOrderToChat authorization.
//
// These fakes are local to this test file (delivery/http package) and are
// intentionally minimal — only the methods LinkOrderToChat actually
// exercises are implemented with real behavior; everything else is a no-op
// satisfying the interface.
// ========================================================================

type linkOrderFakeRepo struct {
	room                 *chatEntity.ChatRoom
	getRoomByOrderIDRoom *chatEntity.ChatRoom
	updateCalls          int
}

func (r *linkOrderFakeRepo) CreateRoom(context.Context, interface{}, *chatEntity.ChatRoom) error {
	return nil
}
func (r *linkOrderFakeRepo) GetRoomByID(context.Context, interface{}, uuid.UUID) (*chatEntity.ChatRoom, error) {
	if r.room == nil {
		return nil, chatRepo.ErrRoomNotFound
	}
	return r.room, nil
}
func (r *linkOrderFakeRepo) GetRoomByIDForUpdate(context.Context, interface{}, uuid.UUID) (*chatEntity.ChatRoom, error) {
	if r.room == nil {
		return nil, chatRepo.ErrRoomNotFound
	}
	return r.room, nil
}
func (r *linkOrderFakeRepo) GetDirectRoom(context.Context, interface{}, uuid.UUID, uuid.UUID) (*chatEntity.ChatRoom, error) {
	return nil, chatRepo.ErrRoomNotFound
}
func (r *linkOrderFakeRepo) GetSupportRoom(context.Context, interface{}, uuid.UUID) (*chatEntity.ChatRoom, error) {
	return nil, chatRepo.ErrRoomNotFound
}
func (r *linkOrderFakeRepo) ListRoomsByUser(context.Context, interface{}, uuid.UUID, *time.Time, *uuid.UUID, int) ([]*chatEntity.ChatRoom, error) {
	return nil, nil
}
func (r *linkOrderFakeRepo) GetRoomByOrderID(context.Context, interface{}, uuid.UUID) (*chatEntity.ChatRoom, error) {
	if r.getRoomByOrderIDRoom != nil {
		return r.getRoomByOrderIDRoom, nil
	}
	return nil, chatRepo.ErrRoomNotFound
}
func (r *linkOrderFakeRepo) UpdateRoomLastMessageAt(context.Context, interface{}, uuid.UUID, time.Time) error {
	return nil
}
func (r *linkOrderFakeRepo) UpdateRoomContext(context.Context, interface{}, uuid.UUID, json.RawMessage, uuid.UUID) error {
	return nil
}
func (r *linkOrderFakeRepo) UpdateRoomLinkedOrderId(_ context.Context, _ interface{}, _ uuid.UUID, linkedOrderID *uuid.UUID) error {
	r.updateCalls++
	if r.room != nil {
		r.room.LinkedOrderID = linkedOrderID
	}
	return nil
}
func (r *linkOrderFakeRepo) CreateMessage(context.Context, interface{}, *chatEntity.ChatMessage) error {
	return nil
}
func (r *linkOrderFakeRepo) GetMessageByID(context.Context, interface{}, uuid.UUID) (*chatEntity.ChatMessage, error) {
	return nil, chatRepo.ErrMessageNotFound
}
func (r *linkOrderFakeRepo) ListMessagesByRoom(context.Context, interface{}, uuid.UUID, *time.Time, *uuid.UUID, int) ([]*chatEntity.ChatMessage, error) {
	return nil, nil
}
func (r *linkOrderFakeRepo) GetMessageByIdempotencyKey(context.Context, interface{}, string) (*chatEntity.ChatMessage, error) {
	return nil, chatRepo.ErrMessageNotFound
}
func (r *linkOrderFakeRepo) SoftHideForModeration(context.Context, interface{}, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (r *linkOrderFakeRepo) RestoreFromModeration(context.Context, interface{}, uuid.UUID) error {
	return nil
}
func (r *linkOrderFakeRepo) CreateReadState(context.Context, interface{}, *chatEntity.ChatReadState) error {
	return nil
}
func (r *linkOrderFakeRepo) GetReadState(context.Context, interface{}, uuid.UUID, uuid.UUID) (*chatEntity.ChatReadState, error) {
	return nil, chatRepo.ErrReadStateNotFound
}
func (r *linkOrderFakeRepo) UpdateReadState(context.Context, interface{}, *chatEntity.ChatReadState) error {
	return nil
}
func (r *linkOrderFakeRepo) UpsertReadState(context.Context, interface{}, *chatEntity.ChatReadState) error {
	return nil
}
func (r *linkOrderFakeRepo) ListReadStatesByRoom(context.Context, interface{}, uuid.UUID) ([]*chatEntity.ChatReadState, error) {
	return nil, nil
}
func (r *linkOrderFakeRepo) GetUnreadCountByRoomAndUser(context.Context, interface{}, uuid.UUID, uuid.UUID) (int, error) {
	return 0, nil
}

var _ chatRepo.Repository = (*linkOrderFakeRepo)(nil)

type linkOrderFakeSocialRepo struct{}

func (f *linkOrderFakeSocialRepo) InsertFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *linkOrderFakeSocialRepo) DeleteFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *linkOrderFakeSocialRepo) DeleteFollowBothDirections(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *linkOrderFakeSocialRepo) ExistsFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (f *linkOrderFakeSocialRepo) AcquireFollowLock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *linkOrderFakeSocialRepo) IsBlockedBy(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (f *linkOrderFakeSocialRepo) ListFollowers(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *linkOrderFakeSocialRepo) ListFollowing(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *linkOrderFakeSocialRepo) InsertBlock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *linkOrderFakeSocialRepo) DeleteBlock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *linkOrderFakeSocialRepo) ExistsBlock(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (f *linkOrderFakeSocialRepo) ListBlocked(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *linkOrderFakeSocialRepo) InsertMute(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *linkOrderFakeSocialRepo) DeleteMute(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *linkOrderFakeSocialRepo) ExistsMute(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (f *linkOrderFakeSocialRepo) ListMuted(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}

var _ socialRepo.SocialRepository = (*linkOrderFakeSocialRepo)(nil)

type linkOrderFakeOutbox struct{}

func (f *linkOrderFakeOutbox) InsertTx(context.Context, db.Tx, string, any, string) error {
	return nil
}

type linkOrderFakeTransactor struct{}

func (f *linkOrderFakeTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(nil)
}

type linkOrderFakeOrderReader struct {
	buyerID  uuid.UUID
	sellerID uuid.UUID
	err      error
}

func (f *linkOrderFakeOrderReader) GetOrderParticipants(context.Context, db.Tx, uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	if f.err != nil {
		return uuid.Nil, uuid.Nil, f.err
	}
	return f.buyerID, f.sellerID, nil
}

func newLinkOrderTestHandler(repo *linkOrderFakeRepo, orderReader chatApp.OrderOwnershipReader) *Handler {
	svc := chatApp.NewService(
		&linkOrderFakeTransactor{},
		repo,
		&linkOrderFakeSocialRepo{},
		&linkOrderFakeOutbox{},
		rate.NewRateLimiter(),
		nil,
		nil,
		orderReader,
		zap.NewNop(),
	)
	return &Handler{chatService: svc, log: zap.NewNop()}
}

func makeChatTestContext(method, target, body string, userID uuid.UUID, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if body != "" {
		c.Request = httptest.NewRequest(method, target, bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")
	} else {
		c.Request = httptest.NewRequest(method, target, nil)
	}
	c.Set("userID", userID)
	c.Params = params
	return c, w
}

func TestLinkOrderToChatHTTP_NonParticipantForbidden(t *testing.T) {
	roomID := uuid.New()
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	stranger := uuid.New()

	repo := &linkOrderFakeRepo{
		room: &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID},
	}
	h := newLinkOrderTestHandler(repo, &linkOrderFakeOrderReader{buyerID: buyerID, sellerID: sellerID})

	body, _ := json.Marshal(map[string]string{"order_id": orderID.String()})
	c, w := makeChatTestContext("PUT", "/chat/rooms/"+roomID.String()+"/link-order", string(body), stranger, gin.Params{
		{Key: "room_id", Value: roomID.String()},
	})

	h.LinkOrderToChat(c)

	if w.Code != 403 {
		t.Fatalf("non-participant: status=%d want 403, body=%s", w.Code, w.Body.String())
	}
	if repo.updateCalls != 0 {
		t.Fatalf("non-participant: UpdateRoomLinkedOrderId calls=%d want 0", repo.updateCalls)
	}
}

func TestLinkOrderToChatHTTP_UnrelatedOrderForbidden(t *testing.T) {
	roomID := uuid.New()
	orderID := uuid.New()
	roomOwnerA := uuid.New()
	roomOwnerB := uuid.New()
	unrelatedBuyer := uuid.New()
	unrelatedSeller := uuid.New()

	repo := &linkOrderFakeRepo{
		room: &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: roomOwnerA, ParticipantB: roomOwnerB},
	}
	h := newLinkOrderTestHandler(repo, &linkOrderFakeOrderReader{buyerID: unrelatedBuyer, sellerID: unrelatedSeller})

	body, _ := json.Marshal(map[string]string{"order_id": orderID.String()})
	c, w := makeChatTestContext("PUT", "/chat/rooms/"+roomID.String()+"/link-order", string(body), roomOwnerA, gin.Params{
		{Key: "room_id", Value: roomID.String()},
	})

	h.LinkOrderToChat(c)

	if w.Code != 403 {
		t.Fatalf("unrelated order: status=%d want 403, body=%s", w.Code, w.Body.String())
	}
	if repo.updateCalls != 0 {
		t.Fatalf("unrelated order: UpdateRoomLinkedOrderId calls=%d want 0 (hijack must not persist)", repo.updateCalls)
	}
}

func TestLinkOrderToChatHTTP_AlreadyLinkedElsewhereConflict(t *testing.T) {
	roomID := uuid.New()
	otherRoomID := uuid.New()
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	repo := &linkOrderFakeRepo{
		room:                 &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID},
		getRoomByOrderIDRoom: &chatEntity.ChatRoom{ID: otherRoomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID},
	}
	h := newLinkOrderTestHandler(repo, &linkOrderFakeOrderReader{buyerID: buyerID, sellerID: sellerID})

	body, _ := json.Marshal(map[string]string{"order_id": orderID.String()})
	c, w := makeChatTestContext("PUT", "/chat/rooms/"+roomID.String()+"/link-order", string(body), buyerID, gin.Params{
		{Key: "room_id", Value: roomID.String()},
	})

	h.LinkOrderToChat(c)

	if w.Code != 409 {
		t.Fatalf("already linked elsewhere: status=%d want 409, body=%s", w.Code, w.Body.String())
	}
}

func TestMarkAsReadHTTP_RemainsParticipantScopedAfterFix(t *testing.T) {
	roomID := uuid.New()
	userID := uuid.New()
	other := uuid.New()
	stranger := uuid.New()

	repo := &linkOrderFakeRepo{
		room: &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: userID, ParticipantB: other},
	}
	h := newLinkOrderTestHandler(repo, &linkOrderFakeOrderReader{})

	body, _ := json.Marshal(map[string]string{"timestamp": "2026-01-01T00:00:00Z"})
	c, w := makeChatTestContext("POST", "/chat/rooms/"+roomID.String()+"/read", string(body), stranger, gin.Params{
		{Key: "room_id", Value: roomID.String()},
	})

	h.MarkAsRead(c)

	if w.Code != 403 {
		t.Fatalf("non-participant mark-as-read: status=%d want 403, body=%s", w.Code, w.Body.String())
	}
}
