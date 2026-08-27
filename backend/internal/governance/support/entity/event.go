package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event represents an event in the ticket's audit trail.
// Events are created for every state transition and action on a ticket.
type Event struct {
	ID        uuid.UUID
	TicketID  uuid.UUID
	EventType EventType
	ActorID   *uuid.UUID
	OldStatus *Status
	NewStatus *Status
	Notes     *string
	Metadata  map[string]interface{}
	CreatedAt time.Time
}

// NewEvent creates a new ticket event.
func NewEvent(ticketID uuid.UUID, eventType EventType, actorID *uuid.UUID) *Event {
	now := time.Now()
	metadata := make(map[string]interface{})

	return &Event{
		ID:        uuid.New(),
		TicketID:  ticketID,
		EventType: eventType,
		ActorID:   actorID,
		CreatedAt: now,
		Metadata:  metadata,
	}
}

// NewStatusChangeEvent creates an event for a status change.
func NewStatusChangeEvent(ticketID uuid.UUID, actorID *uuid.UUID, oldStatus, newStatus Status) *Event {
	event := NewEvent(ticketID, EventTypeStatusChanged, actorID)
	event.OldStatus = &oldStatus
	event.NewStatus = &newStatus
	return event
}

// NewClaimedEvent creates an event when an admin claims a ticket.
func NewClaimedEvent(ticketID uuid.UUID, adminID uuid.UUID) *Event {
	event := NewEvent(ticketID, EventTypeTicketClaimed, &adminID)
	event.NewStatus = func() *Status { s := StatusInProgress; return &s }()
	return event
}

// NewWaitingUserEvent creates an event when ticket status changes to waiting_user.
func NewWaitingUserEvent(ticketID uuid.UUID, adminID uuid.UUID) *Event {
	event := NewEvent(ticketID, EventTypeTicketWaitingUser, &adminID)
	event.OldStatus = func() *Status { s := StatusInProgress; return &s }()
	event.NewStatus = func() *Status { s := StatusWaitingUser; return &s }()
	return event
}

// NewResolvedEvent creates an event when a ticket is resolved.
func NewResolvedEvent(ticketID uuid.UUID, actorID *uuid.UUID, notes *string) *Event {
	event := NewEvent(ticketID, EventTypeTicketResolved, actorID)
	event.Notes = notes
	event.NewStatus = func() *Status { s := StatusResolved; return &s }()
	return event
}

// NewClosedEvent creates an event when a ticket is closed.
func NewClosedEvent(ticketID uuid.UUID, actorID *uuid.UUID, reason *string) *Event {
	event := NewEvent(ticketID, EventTypeTicketClosed, actorID)
	event.Notes = reason
	event.NewStatus = func() *Status { s := StatusClosed; return &s }()
	return event
}

// NewReopenedEvent creates an event when a ticket is reopened.
func NewReopenedEvent(ticketID uuid.UUID, actorID *uuid.UUID) *Event {
	event := NewEvent(ticketID, EventTypeTicketReopened, actorID)
	event.OldStatus = func() *Status { s := StatusClosed; return &s }()
	event.NewStatus = func() *Status { s := StatusOpen; return &s }()
	return event
}

// NewPriorityChangedEvent creates an event when priority is changed.
func NewPriorityChangedEvent(ticketID uuid.UUID, actorID *uuid.UUID, oldPriority, newPriority Priority) *Event {
	event := NewEvent(ticketID, EventTypePriorityChanged, actorID)
	event.Metadata["old_priority"] = oldPriority.String()
	event.Metadata["new_priority"] = newPriority.String()
	return event
}

// NewCategoryChangedEvent creates an event when category is changed.
func NewCategoryChangedEvent(ticketID uuid.UUID, actorID *uuid.UUID, oldCategory, newCategory Category) *Event {
	event := NewEvent(ticketID, EventTypeCategoryChanged, actorID)
	event.Metadata["old_category"] = oldCategory.String()
	event.Metadata["new_category"] = newCategory.String()
	return event
}

// WithNotes adds notes to the event.
func (e *Event) WithNotes(notes string) *Event {
	e.Notes = &notes
	return e
}

// WithMetadata adds metadata to the event.
func (e *Event) WithMetadata(key string, value interface{}) *Event {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// GetMetadataJSON returns the metadata as JSON bytes.
func (e *Event) GetMetadataJSON() []byte {
	if e.Metadata == nil {
		return nil
	}
	data, err := json.Marshal(e.Metadata)
	if err != nil {
		return nil
	}
	return data
}


