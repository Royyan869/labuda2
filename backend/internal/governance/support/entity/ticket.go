package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Ticket represents a support ticket.
//
// Business Rules:
// - Users can have multiple tickets for different issues
// - Each ticket must have a chat_room_id for communication
// - Admins must claim tickets before handling them
// - Resolved tickets must have resolved_at timestamp
// - Closed tickets must have closed_at timestamp
// - Escalation level for routing to specialized teams
type Ticket struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Username        string
	SellerFarmName  string
	ChatRoomID      uuid.UUID
	Category        Category
	Priority        Priority
	Status          Status
	Escalation      Escalation
	LinkedOrderID   *uuid.UUID
	AssignedAdminID *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
	AssignedAt      *time.Time
	ResolvedAt      *time.Time
	ClosedAt        *time.Time
	ResolutionNotes *string
	CloseReason     *string
	Metadata        map[string]interface{}
}

// NewTicket creates a new support ticket.
//
// Rules:
// - userID must be valid
// - chatRoomID must be valid
// - category must be valid
// - status defaults to open
// - priority defaults to medium
// - escalation defaults to none
func NewTicket(userID, chatRoomID uuid.UUID, category Category, priority Priority) *Ticket {
	now := time.Now()

	metadata := make(map[string]interface{})

	return &Ticket{
		ID:         uuid.New(),
		UserID:     userID,
		ChatRoomID: chatRoomID,
		Category:   category,
		Priority:   priority,
		Status:     StatusOpen,
		Escalation: EscalationNone,
		CreatedAt:  now,
		UpdatedAt:  now,
		Metadata:   metadata,
	}
}

// IsOpen returns true if the ticket is in an open state.
func (t *Ticket) IsOpen() bool {
	return t.Status.IsOpen()
}

// IsAssigned returns true if an admin is assigned to this ticket.
func (t *Ticket) IsAssigned() bool {
	return t.AssignedAdminID != nil && *t.AssignedAdminID != uuid.Nil
}

// IsResolved returns true if the ticket is resolved.
func (t *Ticket) IsResolved() bool {
	return t.Status == StatusResolved
}

// IsClosed returns true if the ticket is closed.
func (t *Ticket) IsClosed() bool {
	return t.Status == StatusClosed
}

// CanBeClaimed returns true if the ticket can be claimed by an admin.
func (t *Ticket) CanBeClaimed() bool {
	return t.Status == StatusOpen && !t.IsAssigned()
}

// CanBeReopened returns true if the ticket can be reopened.
func (t *Ticket) CanBeReopened() bool {
	return t.Status == StatusResolved || t.Status == StatusClosed
}

// Claim assigns an admin to the ticket and changes status to in_progress.
// Returns false if the ticket cannot be claimed.
func (t *Ticket) Claim(adminID uuid.UUID) bool {
	if !t.CanBeClaimed() {
		return false
	}

	now := time.Now()
	t.AssignedAdminID = &adminID
	t.AssignedAt = &now
	t.Status = StatusInProgress
	t.UpdatedAt = now

	return true
}

// Resolve changes the ticket status to resolved.
func (t *Ticket) Resolve(notes *string) bool {
	if !t.Status.CanTransitionTo(StatusResolved) {
		return false
	}

	now := time.Now()
	t.Status = StatusResolved
	t.ResolvedAt = &now
	t.ResolutionNotes = notes
	t.UpdatedAt = now

	return true
}

// Close changes the ticket status to closed.
func (t *Ticket) Close(reason *string) bool {
	if !t.Status.CanTransitionTo(StatusClosed) {
		return false
	}

	now := time.Now()
	t.Status = StatusClosed
	t.ClosedAt = &now
	t.CloseReason = reason
	t.UpdatedAt = now

	return true
}

// Reopen changes the ticket status from resolved/closed back to open.
func (t *Ticket) Reopen() bool {
	if !t.CanBeReopened() {
		return false
	}

	t.Status = StatusOpen
	t.AssignedAdminID = nil
	t.AssignedAt = nil
	t.ResolvedAt = nil
	t.ClosedAt = nil
	t.ResolutionNotes = nil
	t.CloseReason = nil
	t.UpdatedAt = time.Now()

	return true
}

// UpdatePriority updates the ticket priority.
func (t *Ticket) UpdatePriority(priority Priority) bool {
	if !priority.IsValid() {
		return false
	}

	t.Priority = priority
	t.UpdatedAt = time.Now()

	return true
}

// UpdateCategory updates the ticket category.
func (t *Ticket) UpdateCategory(category Category) bool {
	if !category.IsValid() {
		return false
	}

	t.Category = category
	t.UpdatedAt = time.Now()

	return true
}

// TransitionTo attempts to transition the ticket to a new status.
func (t *Ticket) TransitionTo(newStatus Status) bool {
	if !t.Status.CanTransitionTo(newStatus) {
		return false
	}

	t.Status = newStatus
	t.UpdatedAt = time.Now()

	// Set timestamps for terminal states
	if newStatus == StatusResolved && t.ResolvedAt == nil {
		now := time.Now()
		t.ResolvedAt = &now
	}

	if newStatus == StatusClosed && t.ClosedAt == nil {
		now := time.Now()
		t.ClosedAt = &now
	}

	return true
}

// GetMetadataJSON returns the metadata as JSON bytes.
func (t *Ticket) GetMetadataJSON() []byte {
	if t.Metadata == nil {
		return nil
	}
	data, err := json.Marshal(t.Metadata)
	if err != nil {
		return nil
	}
	return data
}

// SetMetadataFromJSON sets the metadata from JSON bytes.
func (t *Ticket) SetMetadataFromJSON(data []byte) error {
	if t.Metadata == nil {
		t.Metadata = make(map[string]interface{})
	}
	return json.Unmarshal(data, &t.Metadata)
}


