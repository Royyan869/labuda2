// DOMAIN: CHAT
// NOTE: Real-time messaging between platform participants

package entity

import (
	"time"

	"github.com/google/uuid"
)

// ChatRoom represents a conversation space between two participants.
//
// STRICT RULES:
//   - Participants are immutable after creation
//   - participant_a < participant_b is enforced
//   - One room per participant pair per room_type
//   - No financial state in this entity
//   - No coupling to negotiation/trade/financial domains
//   - Room-level commerce context is NOT stored here: commerce/resource
//     references in chat are carried at the message level (attachment_json +
//     chat_message_resource_occurrences). The room carries only identity and the
//     linked_order_id for order ↔ chat continuity.
//   - LinkedOrderId is the order ID for commerce continuity (order ↔ chat alignment)
type ChatRoom struct {
	ID            uuid.UUID
	RoomType      RoomType
	ParticipantA  uuid.UUID
	ParticipantB  uuid.UUID
	LinkedOrderID *uuid.UUID `db:"linked_order_id"` // Order linked to this chat for commerce continuity
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastMessageAt time.Time
}

// NewChatRoom creates a new chat room with sorted participants.
//
// IMPORTANT: Participants are sorted deterministically by UUID.
// This ensures consistent room lookup and unique constraints.
//
// Rules:
// - participantA and participantB must be different
// - participantA is always the smaller UUID (lexicographically)
// - roomType must be valid
func NewChatRoom(roomType RoomType, userA, userB uuid.UUID) *ChatRoom {
	now := time.Now()

	// Sort participants deterministically
	var participantA, participantB uuid.UUID
	if userA.String() < userB.String() {
		participantA = userA
		participantB = userB
	} else {
		participantA = userB
		participantB = userA
	}

	return &ChatRoom{
		ID:            uuid.New(),
		RoomType:      roomType,
		ParticipantA:  participantA,
		ParticipantB:  participantB,
		CreatedAt:     now,
		UpdatedAt:     now,
		LastMessageAt: now,
	}
}

// HasParticipant checks if the given user is a participant in this room.
func (r *ChatRoom) HasParticipant(userID uuid.UUID) bool {
	return r.ParticipantA == userID || r.ParticipantB == userID
}

// OtherParticipant returns the other participant's ID given one participant.
// Returns uuid.Nil if the userID is not a participant.
func (r *ChatRoom) OtherParticipant(userID uuid.UUID) uuid.UUID {
	if r.ParticipantA == userID {
		return r.ParticipantB
	}
	if r.ParticipantB == userID {
		return r.ParticipantA
	}
	return uuid.Nil
}

// UpdateLastMessageTimestamp updates the last_message_at timestamp.
// This is called when a new message is sent in the room.
func (r *ChatRoom) UpdateLastMessageTimestamp() {
	now := time.Now()
	r.LastMessageAt = now
	r.UpdatedAt = now
}

// HasLinkedOrder returns true if the room has a linked order.
func (r *ChatRoom) HasLinkedOrder() bool {
	return r.LinkedOrderID != nil
}

// HasOrderContext returns true if the room has a linked order.
// This is the explicit commerce continuity carve-out for block enforcement:
// parties to an existing order may communicate even if a block exists between them.
// This is the only room-level commerce signal (the room carries no context JSON).
func (r *ChatRoom) HasOrderContext() bool {
	return r.LinkedOrderID != nil
}

// LinkOrder updates the room's linked order ID.
// This is used when an order is created from a chat or when navigating
// from order detail to chat for commerce continuity.
func (r *ChatRoom) LinkOrder(orderID uuid.UUID) {
	r.LinkedOrderID = &orderID
	r.UpdatedAt = time.Now()
}

// UnlinkOrder removes the linked order ID from the room.
func (r *ChatRoom) UnlinkOrder() {
	r.LinkedOrderID = nil
	r.UpdatedAt = time.Now()
}
