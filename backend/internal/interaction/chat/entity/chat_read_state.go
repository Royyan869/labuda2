package entity

import (
	"time"

	"github.com/google/uuid"
)

// ChatReadState tracks the last read timestamp for a user in a room.
//
// IMPORTANT: No unread_count column.
// Unread counts are computed on read by comparing last_read_at with message timestamps.
//
// STRICT RULES:
// - One read state per (room_id, user_id) pair
// - last_read_at is the timestamp of the last message the user has read
// - No financial state in this entity
type ChatReadState struct {
	RoomID     uuid.UUID
	UserID     uuid.UUID
	LastReadAt time.Time
}

// NewChatReadState creates a new read state with the current timestamp.
func NewChatReadState(roomID, userID uuid.UUID) *ChatReadState {
	return &ChatReadState{
		RoomID:     roomID,
		UserID:     userID,
		LastReadAt: time.Now(),
	}
}

// NewChatReadStateWithTimestamp creates a new read state with a specific timestamp.
func NewChatReadStateWithTimestamp(roomID, userID uuid.UUID, lastReadAt time.Time) *ChatReadState {
	return &ChatReadState{
		RoomID:     roomID,
		UserID:     userID,
		LastReadAt: lastReadAt,
	}
}

// UpdateLastReadAt updates the last read timestamp.
func (s *ChatReadState) UpdateLastReadAt(timestamp time.Time) {
	s.LastReadAt = timestamp
}


