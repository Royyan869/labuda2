package entity

import (
	"github.com/google/uuid"
)

// NegotiationMessage represents a price proposal within a negotiation session.
type NegotiationMessage struct {
	ID        uuid.UUID
	SessionID uuid.UUID
	SenderID  uuid.UUID
	Price     int64 // Stored as smallest currency unit (e.g., cents for IDR)
	Note      string
	CreatedAt int64 // Unix timestamp in seconds
}

// NewNegotiationMessage creates a new negotiation message.
//
// Rules:
// - Price must be > 0
// - Sender authorization is validated at service layer
func NewNegotiationMessage(sessionID, senderID uuid.UUID, price int64, note string) (*NegotiationMessage, error) {
	if price <= 0 {
		return nil, &InvalidPriceError{
			SessionID: sessionID,
			Price:     price,
			Reason:    "price must be positive",
		}
	}

	now := currentTimeUnix()

	return &NegotiationMessage{
		ID:        uuid.New(),
		SessionID: sessionID,
		SenderID:  senderID,
		Price:     price,
		Note:      note,
		CreatedAt: now,
	}, nil
}


