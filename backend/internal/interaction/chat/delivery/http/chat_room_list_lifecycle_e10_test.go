package http

// E10 — Chat room list participant lifecycle convergence tests.
//
// These tests complement chat_lifecycle_test.go (which covers the pure
// card-from-row coarsening contract) with two additional cases that pin
// the support-room / non-participant exclusion gate in hydrateRoomParticipants.
//
// The gate is the `if other == uuid.Nil { continue }` check at the top of
// the id-collection loop. Because OtherParticipant returns uuid.Nil for:
//   a) support rooms where the admin side is stored as uuid.Nil
//   b) rooms where the caller is not a participant at all
//
// …those rooms never contribute an ID to the lifecycle batch query.
// ChatCard._buildSupportChatCard is reached directly for support rooms,
// bypassing the participant lifecycle render gate entirely.
//
// The coarsening contract (active / unavailable / removed vocabulary,
// slot-persistence, rollback-safe nil) is fully covered in
// chat_lifecycle_test.go — nothing is duplicated here.

import (
	"testing"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
)

// TestOtherParticipant_SupportRoomNilAdminExcluded verifies that a support
// room whose "admin side" is stored as uuid.Nil produces uuid.Nil from
// OtherParticipant — which hydrateRoomParticipants then skips via the
// `if other == uuid.Nil { continue }` gate, preventing any lifecycle hydration
// attempt for the nil placeholder.
func TestOtherParticipant_SupportRoomNilAdminExcluded(t *testing.T) {
	userID := uuid.New()
	// Support room: caller is ParticipantA, admin side is uuid.Nil placeholder.
	room := &chatEntity.ChatRoom{
		ID:           uuid.New(),
		RoomType:     chatEntity.RoomTypeSupport,
		ParticipantA: userID,
		ParticipantB: uuid.Nil,
	}
	other := room.OtherParticipant(userID)
	if other != uuid.Nil {
		t.Errorf("support room OtherParticipant = %v; want uuid.Nil (excluded from lifecycle hydration)", other)
	}
}

// TestOtherParticipant_NonParticipantRoomExcluded verifies that a room where
// the caller is not a participant produces uuid.Nil — same exclusion path as
// the support room case.
func TestOtherParticipant_NonParticipantRoomExcluded(t *testing.T) {
	userID := uuid.New()
	strangerA := uuid.New()
	strangerB := uuid.New()
	room := &chatEntity.ChatRoom{
		ID:           uuid.New(),
		RoomType:     chatEntity.RoomTypeDirect,
		ParticipantA: strangerA,
		ParticipantB: strangerB,
	}
	other := room.OtherParticipant(userID)
	if other != uuid.Nil {
		t.Errorf("non-participant room OtherParticipant = %v; want uuid.Nil", other)
	}
}

// TestOtherParticipant_DirectRoom verifies the happy path: a direct room
// correctly identifies the other participant. This confirms the same pure
// function used by hydrateRoomParticipants to collect IDs.
func TestOtherParticipant_DirectRoom(t *testing.T) {
	userID := uuid.New()
	otherID := uuid.New()
	room := &chatEntity.ChatRoom{
		ID:           uuid.New(),
		RoomType:     chatEntity.RoomTypeDirect,
		ParticipantA: userID,
		ParticipantB: otherID,
	}
	other := room.OtherParticipant(userID)
	if other != otherID {
		t.Errorf("direct room OtherParticipant = %v; want %v", other, otherID)
	}
}


