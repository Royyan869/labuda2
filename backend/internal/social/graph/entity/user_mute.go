package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserMute represents a mute relationship between users.
//
// STRICT BOUNDARY RULES:
// - This entity does NOT touch financial state
// - This entity does NOT modify orders, offers, or withdrawals
// - This entity only tracks social graph mutes
// - Mute hides content but does NOT block interactions
// - No outbox events emitted
type UserMute struct {
	MuterID uuid.UUID
	MutedID  uuid.UUID
	CreatedAt time.Time
}

// NewUserMute creates a new mute relationship.
func NewUserMute(muterID, mutedID uuid.UUID) *UserMute {
	return &UserMute{
		MuterID: muterID,
		MutedID:  mutedID,
		CreatedAt: time.Now(),
	}
}


