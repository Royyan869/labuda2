package entity

// EventType represents the type of support ticket event.
type EventType string

const (
	// EventTypeTicketCreated is emitted when a ticket is created.
	EventTypeTicketCreated EventType = "ticket_created"

	// EventTypeTicketClaimed is emitted when an admin claims a ticket.
	EventTypeTicketClaimed EventType = "ticket_claimed"

	// EventTypeTicketWaitingUser is emitted when ticket status changes to waiting_user.
	EventTypeTicketWaitingUser EventType = "ticket_waiting_user"

	// EventTypeStatusChanged is emitted when ticket status changes.
	EventTypeStatusChanged EventType = "status_changed"

	// EventTypePriorityChanged is emitted when ticket priority changes.
	EventTypePriorityChanged EventType = "priority_changed"

	// EventTypeCategoryChanged is emitted when ticket category changes.
	EventTypeCategoryChanged EventType = "category_changed"

	// EventTypeTicketResolved is emitted when a ticket is resolved.
	EventTypeTicketResolved EventType = "ticket_resolved"

	// EventTypeTicketClosed is emitted when a ticket is closed.
	EventTypeTicketClosed EventType = "ticket_closed"

	// EventTypeTicketReopened is emitted when a ticket is reopened.
	EventTypeTicketReopened EventType = "ticket_reopened"

	// EventTypeAdminAssigned is emitted when an admin is assigned.
	EventTypeAdminAssigned EventType = "admin_assigned"

	// EventTypeAdminUnassigned is emitted when an admin is unassigned.
	EventTypeAdminUnassigned EventType = "admin_unassigned"

	// EventTypeTicketEscalated is emitted when a ticket is escalated to dispute.
	EventTypeTicketEscalated EventType = "ticket_escalated"
)

// String returns the string representation of the event type.
func (e EventType) String() string {
	return string(e)
}

// IsValid checks if the event type is valid.
func (e EventType) IsValid() bool {
	switch e {
	case EventTypeTicketCreated, EventTypeTicketClaimed, EventTypeTicketWaitingUser, EventTypeStatusChanged,
		EventTypePriorityChanged, EventTypeCategoryChanged, EventTypeTicketResolved,
		EventTypeTicketClosed, EventTypeTicketReopened, EventTypeAdminAssigned,
		EventTypeAdminUnassigned, EventTypeTicketEscalated:
		return true
	default:
		return false
	}
}


