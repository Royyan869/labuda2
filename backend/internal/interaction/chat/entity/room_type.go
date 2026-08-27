package entity

// RoomType represents the type of chat room.
type RoomType string

const (
	// RoomTypeDirect is a direct 1:1 chat between two users.
	RoomTypeDirect RoomType = "direct"

	// RoomTypeNegotiation is a chat linked to a negotiation session.
	RoomTypeNegotiation RoomType = "negotiation"

	// RoomTypeSupport is a chat for customer support.
	RoomTypeSupport RoomType = "support"
)

// String returns the string representation of the room type.
func (r RoomType) String() string {
	return string(r)
}

// IsValid checks if the room type is valid.
func (r RoomType) IsValid() bool {
	switch r {
	case RoomTypeDirect, RoomTypeNegotiation, RoomTypeSupport:
		return true
	default:
		return false
	}
}


