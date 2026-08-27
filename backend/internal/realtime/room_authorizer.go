package realtime

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// RoomType represents the type of room.
type RoomType string

const (
	RoomTypeChat    RoomType = "chat"    // Chat room between users
	RoomTypeOrder   RoomType = "order"   // Order room (buyer/seller)
	RoomTypeAuction RoomType = "auction" // Auction room (public)
)

// RoomAuthorizer validates whether a user is allowed to subscribe to a room.
//
// Authorization rules by room type:
// - Chat: user must be a participant in the conversation
// - Order: user must be buyer or seller
// - Auction: any authenticated user may join (public)
//
// The canonical implementation is DatabaseRoomAuthorizer.
type RoomAuthorizer interface {
	CanSubscribeToRoom(ctx context.Context, userID, roomID uuid.UUID, roomType RoomType) bool
}

// RoomAccessDeniedError is returned when a user is not allowed to access a room.
type RoomAccessDeniedError struct {
	UserID   uuid.UUID
	RoomID   uuid.UUID
	RoomType RoomType
}

func (e *RoomAccessDeniedError) Error() string {
	return fmt.Sprintf("user %s cannot access %s room %s",
		e.UserID, e.RoomType, e.RoomID)
}

// FormatRoomID formats a room ID with its type prefix.
func FormatRoomID(roomID uuid.UUID, roomType RoomType) string {
	return fmt.Sprintf("%s:%s", roomType, roomID.String())
}


