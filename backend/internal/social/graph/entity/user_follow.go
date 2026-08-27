// DOMAIN: SOCIAL
// NOTE: User relationship tracking (follow, block, mute)

package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserFollow represents a follow relationship between users.
//
// STRICT BOUNDARY RULES:
// - This entity does NOT touch financial state
// - This entity does NOT modify orders, offers, or withdrawals
// - This entity only tracks social graph relationships
// - No outbox events emitted
type UserFollow struct {
	FollowerID  uuid.UUID
	FollowingID uuid.UUID
	CreatedAt   time.Time
}

// NewUserFollow creates a new follow relationship.
func NewUserFollow(followerID, followingID uuid.UUID) *UserFollow {
	return &UserFollow{
		FollowerID:  followerID,
		FollowingID: followingID,
		CreatedAt:   time.Now(),
	}
}


