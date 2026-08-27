package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"go.uber.org/zap"
)

// fakeOrderOwnershipReader is a minimal test double for OrderOwnershipReader.
// PASS_6A / F1: LinkOrderToChat authorization tests.
type fakeOrderOwnershipReader struct {
	buyerID  uuid.UUID
	sellerID uuid.UUID
	err      error
}

func (f *fakeOrderOwnershipReader) GetOrderParticipants(_ context.Context, _ db.Tx, _ uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	if f.err != nil {
		return uuid.Nil, uuid.Nil, f.err
	}
	return f.buyerID, f.sellerID, nil
}

func newLinkOrderTestService(repo *roomUpdatedMockRepo, orderReader OrderOwnershipReader) *Service {
	return &Service{
		db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  &roomUpdatedMockOutbox{},
		rateLimiter: rate.NewRateLimiter(),
		orderReader: orderReader,
		log:         zap.NewNop(),
	}
}

func TestLinkOrderToChat_RejectsNonParticipant(t *testing.T) {
	buyerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sellerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	stranger := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	roomID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &roomUpdatedMockRepo{
		room: &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID},
	}
	service := newLinkOrderTestService(repo, &fakeOrderOwnershipReader{buyerID: buyerID, sellerID: sellerID})

	_, err := service.LinkOrderToChat(context.Background(), roomID, orderID, stranger)
	if !errors.Is(err, chatRepo.ErrParticipantMismatch) {
		t.Fatalf("err=%v want ErrParticipantMismatch", err)
	}
	if repo.updateLinkedOrderIDCalls != 0 {
		t.Fatalf("UpdateRoomLinkedOrderId calls=%d want 0", repo.updateLinkedOrderIDCalls)
	}
}

func TestLinkOrderToChat_RejectsOrderNotFound(t *testing.T) {
	buyerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sellerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	roomID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &roomUpdatedMockRepo{
		room: &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID},
	}
	service := newLinkOrderTestService(repo, &fakeOrderOwnershipReader{err: errors.New("no rows")})

	_, err := service.LinkOrderToChat(context.Background(), roomID, orderID, buyerID)
	if !errors.Is(err, chatRepo.ErrOrderNotFound) {
		t.Fatalf("err=%v want ErrOrderNotFound", err)
	}
	if repo.updateLinkedOrderIDCalls != 0 {
		t.Fatalf("UpdateRoomLinkedOrderId calls=%d want 0", repo.updateLinkedOrderIDCalls)
	}
}

// TestLinkOrderToChat_RejectsOrderOwnershipMismatch proves the F1 hijack is closed:
// a participant of a room they legitimately belong to cannot link an order that
// belongs to two completely unrelated users.
func TestLinkOrderToChat_RejectsOrderOwnershipMismatch(t *testing.T) {
	roomOwnerA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	roomOwnerB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	unrelatedBuyer := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	unrelatedSeller := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	roomID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &roomUpdatedMockRepo{
		room: &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: roomOwnerA, ParticipantB: roomOwnerB},
	}
	service := newLinkOrderTestService(repo, &fakeOrderOwnershipReader{buyerID: unrelatedBuyer, sellerID: unrelatedSeller})

	// roomOwnerA is a legitimate participant of roomID, but is neither the
	// buyer nor seller of orderID.
	_, err := service.LinkOrderToChat(context.Background(), roomID, orderID, roomOwnerA)
	if !errors.Is(err, chatRepo.ErrOrderOwnershipMismatch) {
		t.Fatalf("err=%v want ErrOrderOwnershipMismatch", err)
	}
	if repo.updateLinkedOrderIDCalls != 0 {
		t.Fatalf("UpdateRoomLinkedOrderId calls=%d want 0 (hijack must not persist)", repo.updateLinkedOrderIDCalls)
	}
}

// TestLinkOrderToChat_RejectsRoomParticipantMismatch covers the case where the
// caller genuinely is the order's buyer, but the room's counterparty is some
// other user rather than the order's actual seller.
func TestLinkOrderToChat_RejectsRoomParticipantMismatch(t *testing.T) {
	buyerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	roomCounterparty := uuid.MustParse("33333333-3333-3333-3333-333333333333") // a friend, not the seller
	realSellerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	roomID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &roomUpdatedMockRepo{
		room: &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: roomCounterparty},
	}
	service := newLinkOrderTestService(repo, &fakeOrderOwnershipReader{buyerID: buyerID, sellerID: realSellerID})

	_, err := service.LinkOrderToChat(context.Background(), roomID, orderID, buyerID)
	if !errors.Is(err, chatRepo.ErrOrderRoomParticipantMismatch) {
		t.Fatalf("err=%v want ErrOrderRoomParticipantMismatch", err)
	}
	if repo.updateLinkedOrderIDCalls != 0 {
		t.Fatalf("UpdateRoomLinkedOrderId calls=%d want 0", repo.updateLinkedOrderIDCalls)
	}
}

func TestLinkOrderToChat_BuyerCanLinkOwnOrderToCorrectRoom(t *testing.T) {
	buyerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sellerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	roomID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &roomUpdatedMockRepo{
		room: &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID},
	}
	service := newLinkOrderTestService(repo, &fakeOrderOwnershipReader{buyerID: buyerID, sellerID: sellerID})

	room, err := service.LinkOrderToChat(context.Background(), roomID, orderID, buyerID)
	if err != nil {
		t.Fatalf("LinkOrderToChat failed: %v", err)
	}
	if room.LinkedOrderID == nil || *room.LinkedOrderID != orderID {
		t.Fatalf("linked_order_id=%v want %s", room.LinkedOrderID, orderID)
	}
	if repo.updateLinkedOrderIDCalls != 1 {
		t.Fatalf("UpdateRoomLinkedOrderId calls=%d want 1", repo.updateLinkedOrderIDCalls)
	}
}

func TestLinkOrderToChat_SellerCanLinkOwnOrderToCorrectRoom(t *testing.T) {
	buyerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sellerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	roomID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &roomUpdatedMockRepo{
		room: &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID},
	}
	service := newLinkOrderTestService(repo, &fakeOrderOwnershipReader{buyerID: buyerID, sellerID: sellerID})

	room, err := service.LinkOrderToChat(context.Background(), roomID, orderID, sellerID)
	if err != nil {
		t.Fatalf("LinkOrderToChat failed: %v", err)
	}
	if room.LinkedOrderID == nil || *room.LinkedOrderID != orderID {
		t.Fatalf("linked_order_id=%v want %s", room.LinkedOrderID, orderID)
	}
}

// TestLinkOrderToChat_RejectsDuplicateLinkToAnotherRoom is the hijack
// regression test for GetRoomByOrderID: once an order is linked to its
// canonical room, a different room must not be able to claim the same order.
func TestLinkOrderToChat_RejectsDuplicateLinkToAnotherRoom(t *testing.T) {
	buyerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sellerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	otherRoomID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	roomID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &roomUpdatedMockRepo{
		room: &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID},
		// Order is already linked to a *different* room (otherRoomID).
		getRoomByOrderIDRoom: &chatEntity.ChatRoom{ID: otherRoomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID},
	}
	service := newLinkOrderTestService(repo, &fakeOrderOwnershipReader{buyerID: buyerID, sellerID: sellerID})

	_, err := service.LinkOrderToChat(context.Background(), roomID, orderID, buyerID)
	if !errors.Is(err, chatRepo.ErrOrderAlreadyLinkedElsewhere) {
		t.Fatalf("err=%v want ErrOrderAlreadyLinkedElsewhere", err)
	}
	if repo.updateLinkedOrderIDCalls != 0 {
		t.Fatalf("UpdateRoomLinkedOrderId calls=%d want 0", repo.updateLinkedOrderIDCalls)
	}
}

// TestLinkOrderToChat_IdempotentReLinkToSameRoom proves re-linking the same
// order to the room that already holds it (LATEST ACTIVE ORDER RULE re-affirm)
// is not treated as a conflict.
func TestLinkOrderToChat_IdempotentReLinkToSameRoom(t *testing.T) {
	buyerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sellerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	roomID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &roomUpdatedMockRepo{
		room:                 &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID},
		getRoomByOrderIDRoom: &chatEntity.ChatRoom{ID: roomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: buyerID, ParticipantB: sellerID},
	}
	service := newLinkOrderTestService(repo, &fakeOrderOwnershipReader{buyerID: buyerID, sellerID: sellerID})

	_, err := service.LinkOrderToChat(context.Background(), roomID, orderID, buyerID)
	if err != nil {
		t.Fatalf("LinkOrderToChat failed: %v", err)
	}
	if repo.updateLinkedOrderIDCalls != 1 {
		t.Fatalf("UpdateRoomLinkedOrderId calls=%d want 1", repo.updateLinkedOrderIDCalls)
	}
}

// TestGetRoomByOrderID_NotHijackedAfterFix confirms that once ownership
// validation rejects an unrelated link attempt, GetRoomByOrderID still
// resolves to the legitimate room (never mutated by the rejected attempt).
func TestGetRoomByOrderID_NotHijackedAfterFix(t *testing.T) {
	legitimateBuyer := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	legitimateSeller := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	attacker := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	attackerFriend := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	legitimateRoomID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	attackerRoomID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	legitimateRoom := &chatEntity.ChatRoom{ID: legitimateRoomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: legitimateBuyer, ParticipantB: legitimateSeller}
	repo := &roomUpdatedMockRepo{
		room:                 &chatEntity.ChatRoom{ID: attackerRoomID, RoomType: chatEntity.RoomTypeDirect, ParticipantA: attacker, ParticipantB: attackerFriend},
		getRoomByOrderIDRoom: legitimateRoom,
	}
	service := newLinkOrderTestService(repo, &fakeOrderOwnershipReader{buyerID: legitimateBuyer, sellerID: legitimateSeller})

	// Attacker tries to hijack the order into their own unrelated room.
	_, err := service.LinkOrderToChat(context.Background(), attackerRoomID, orderID, attacker)
	if !errors.Is(err, chatRepo.ErrOrderOwnershipMismatch) {
		t.Fatalf("err=%v want ErrOrderOwnershipMismatch", err)
	}

	// The order must still resolve to the legitimate room.
	resolvedRoom, err := service.GetRoomByOrderID(context.Background(), orderID)
	if err != nil {
		t.Fatalf("GetRoomByOrderID failed: %v", err)
	}
	if resolvedRoom.ID != legitimateRoomID {
		t.Fatalf("resolved room=%s want %s (hijack must not succeed)", resolvedRoom.ID, legitimateRoomID)
	}
}
