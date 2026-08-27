package event

import (
	"github.com/google/uuid"
)

// OutboxEvent represents an event in the outbox pattern for reliable event delivery.
// This struct is shared across worker and handler packages to ensure consistency.
type OutboxEvent struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       []byte
}


