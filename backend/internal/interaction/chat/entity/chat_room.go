// DOMAIN: CHAT
// NOTE: Real-time messaging between platform participants

package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ChatRoom represents a conversation space between two participants.
//
// STRICT RULES:
// - Participants are immutable after creation
// - participant_a < participant_b is enforced
// - One room per participant pair per room_type
// - No financial state in this entity
// - No coupling to negotiation/trade/financial domains
// - Context is optional commerce metadata (forSale, auction, etc.)
// - LinkedOrderId is the order ID for commerce continuity (order ↔ chat alignment)
type ChatRoom struct {
	ID           uuid.UUID
	RoomType     RoomType
	ParticipantA uuid.UUID
	ParticipantB uuid.UUID
	// ContextJSON is optional commerce context for UI display only.
	// UI HINT ONLY - DO NOT USE FOR BUSINESS LOGIC.
	// This field is for rendering context hints in the chat UI (forSale preview, auction info, etc.).
	// All pricing, validation, and business logic MUST use authoritative domain entities.
	ContextJSON   json.RawMessage `db:"context_json"`
	ContextSetBy  *uuid.UUID      `db:"context_set_by"`  // User who set the context, nil when not applicable
	LinkedOrderID *uuid.UUID      `db:"linked_order_id"` // Order linked to this chat for commerce continuity
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastMessageAt time.Time
}

// HasContext returns true if the room has a context set.
func (r *ChatRoom) HasContext() bool {
	return len(r.ContextJSON) > 0
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

// NewChatRoomWithContext creates a new chat room with commerce context.
//
// Context is used to associate the room with a forSale, auction, or other
// commerce entity for features like seller quotes, negotiations, etc.
//
// Parameters:
// - roomType: Type of room (direct, negotiation, support)
// - userA, userB: The two participants (will be sorted deterministically)
// - contextJSON: Commerce context as JSON (forSale attachment, etc.)
// - contextSetBy: User ID who is setting the context
func NewChatRoomWithContext(
	roomType RoomType,
	userA, userB uuid.UUID,
	contextJSON json.RawMessage,
	contextSetBy uuid.UUID,
) *ChatRoom {
	room := NewChatRoom(roomType, userA, userB)
	room.ContextJSON = contextJSON
	if contextSetBy != uuid.Nil {
		room.ContextSetBy = &contextSetBy
	}
	return room
}

// SetContext updates the room's context.
//
// This allows adding context to an existing room, for example when a user
// opens a chat from a forSale after the room was already created.
func (r *ChatRoom) SetContext(contextJSON json.RawMessage, contextSetBy uuid.UUID) {
	r.ContextJSON = contextJSON
	if contextSetBy == uuid.Nil {
		r.ContextSetBy = nil
	} else {
		r.ContextSetBy = &contextSetBy
	}
	r.UpdatedAt = time.Now()
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
// Distinct from HasContext() (which checks ContextJSON UI hints — not a block bypass).
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


