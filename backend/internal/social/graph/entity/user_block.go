package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserBlock represents a block relationship between users.
//
// STRICT BOUNDARY RULES:
// - This entity does NOT touch financial state
// - This entity does NOT modify orders, offers, or withdrawals
// - This entity only tracks social graph blocks
// - Block overrides follow (follow cleanup happens in service layer)
// - No outbox events emitted
type UserBlock struct {
	BlockerID uuid.UUID
	BlockedID uuid.UUID
	CreatedAt time.Time
}

// NewUserBlock creates a new block relationship.
func NewUserBlock(blockerID, blockedID uuid.UUID) *UserBlock {
	return &UserBlock{
		BlockerID: blockerID,
		BlockedID: blockedID,
		CreatedAt: time.Now(),
	}
}


