package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/support/entity"
	"github.com/labuda/backend/pkg/db"
)

// Repository defines the persistence interface for support domain.
//
// DESIGN PRINCIPLES:
// - All mutations use db.Tx for transaction safety
// - Cursor-based pagination only (NO OFFSET)
// - Concurrency-safe with SELECT FOR UPDATE
// - No SQL in service layer
type Repository interface {
	// ========================================================================
	// TICKET OPERATIONS
	// ========================================================================

	// CreateTicket creates a new support ticket.
	// Returns ErrDuplicateOpenTicket if user already has an open ticket.
	CreateTicket(ctx context.Context, tx interface{}, ticket *entity.Ticket) error

	// GetTicketByID retrieves a ticket by ID.
	// Returns ErrTicketNotFound if not found.
	GetTicketByID(ctx context.Context, tx interface{}, ticketID uuid.UUID) (*entity.Ticket, error)

	// GetOpenTicketByUser retrieves the current open ticket for a user.
	// Returns ErrTicketNotFound if no open ticket exists.
	GetOpenTicketByUser(ctx context.Context, tx interface{}, userID uuid.UUID) (*entity.Ticket, error)

	// GetTicketByChatRoomID retrieves the active ticket linked to a chat room.
	// Returns ErrTicketNotFound if no active ticket exists for the room.
	GetTicketByChatRoomID(ctx context.Context, tx interface{}, chatRoomID uuid.UUID) (*entity.Ticket, error)

	// ListTickets lists all tickets with optional filters.
	// Uses cursor-based pagination.
	ListTickets(
		ctx context.Context,
		tx interface{},
		filter *TicketFilter,
		cursorCreatedAt *time.Time,
		cursorID *uuid.UUID,
		limit int,
	) ([]*entity.Ticket, error)

	// CountTickets returns the count of tickets matching the filter.
	CountTickets(ctx context.Context, tx interface{}, filter *TicketFilter) (int64, error)

	// CountActiveTicketsByOrderID returns the count of active (open/in_progress/waiting_user) tickets for an order.
	// Used to block order completion when there's an active support ticket.
	CountActiveTicketsByOrderID(ctx context.Context, tx interface{}, orderID uuid.UUID) (int64, error)

	// ClaimTicket atomically claims an open ticket for an admin.
	// Uses SELECT FOR UPDATE to prevent race conditions.
	// Returns ErrTicketNotFound, ErrTicketAlreadyClaimed, or ErrConcurrentUpdate.
	ClaimTicket(ctx context.Context, tx interface{}, ticketID, adminID uuid.UUID) (*entity.Ticket, error)

	// ResolveTicket updates a ticket status to resolved.
	ResolveTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID, notes *string) error

	// CloseTicket updates a ticket status to closed.
	CloseTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID, reason *string) error

	// ReopenTicket reopens a resolved or closed ticket.
	ReopenTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID) error

	// UpdatePriority updates the ticket priority.
	UpdatePriority(ctx context.Context, tx interface{}, ticketID uuid.UUID, priority entity.Priority) error

	// UpdateCategory updates the ticket category.
	UpdateCategory(ctx context.Context, tx interface{}, ticketID uuid.UUID, category entity.Category) error

	// UpdateStatus updates the ticket status with validation.
	UpdateStatus(ctx context.Context, tx interface{}, ticketID uuid.UUID, status entity.Status) error

	// UpdateEscalation updates the ticket escalation level.
	UpdateEscalation(ctx context.Context, tx interface{}, ticketID uuid.UUID, escalation entity.Escalation) error

	// AssignAdmin directly assigns an admin to a ticket (admin operation).
	AssignAdmin(ctx context.Context, tx interface{}, ticketID, adminID uuid.UUID) error

	// UnassignAdmin removes the admin assignment from a ticket.
	UnassignAdmin(ctx context.Context, tx interface{}, ticketID uuid.UUID) error

	// ========================================================================
	// EVENT OPERATIONS
	// ========================================================================

	// CreateEvent creates a new ticket event.
	CreateEvent(ctx context.Context, tx interface{}, event *entity.Event) error

	// ListEvents lists all events for a ticket.
	ListEvents(ctx context.Context, tx interface{}, ticketID uuid.UUID, limit int) ([]*entity.Event, error)

	// ========================================================================
	// ADMIN OPERATIONS
	// ========================================================================

	// GetAdmin retrieves a support admin by ID.
	// Returns ErrAdminNotFound if not found.
	GetAdmin(ctx context.Context, tx interface{}, adminID uuid.UUID) (*entity.Admin, error)

	// CreateAdmin creates a new support admin record.
	CreateAdmin(ctx context.Context, tx interface{}, admin *entity.Admin) error

	// ListAdmins lists all support admins with optional active filter.
	ListAdmins(ctx context.Context, tx interface{}, isActive *bool) ([]*entity.Admin, error)

	// GetAvailableAdmins returns admins who can take more tickets.
	// Ordered by active_ticket_count ASC, last_assigned_at ASC.
	GetAvailableAdmins(ctx context.Context, tx interface{}, maxConcurrent int, limit int) ([]*entity.Admin, error)

	// IncrementAdminTicketCount increments the active ticket count for an admin.
	IncrementAdminTicketCount(ctx context.Context, tx interface{}, adminID uuid.UUID) error

	// DecrementAdminTicketCount decrements the active ticket count for an admin.
	DecrementAdminTicketCount(ctx context.Context, tx interface{}, adminID uuid.UUID) error

	// SetAdminActive sets the admin's active status.
	SetAdminActive(ctx context.Context, tx interface{}, adminID uuid.UUID, isActive bool) error

	// GetTicketStatistics returns statistics about tickets.
	GetTicketStatistics(ctx context.Context, tx interface{}) (*TicketStatistics, error)

	// ========================================================================
	// SLA OPERATIONS
	// ========================================================================

	// FindTicketsForSLACheck finds tickets that need SLA checking.
	// Returns strongly-typed rows; nullable timestamps are represented as
	// pointers so callers never have to perform unchecked type assertions.
	FindTicketsForSLACheck(ctx context.Context, tx db.Tx, limit int) ([]TicketSLARow, error)

	// FindDisputesForSLACheck finds disputes that need SLA checking.
	// Returns strongly-typed rows; see FindTicketsForSLACheck for nullable
	// timestamp semantics.
	FindDisputesForSLACheck(ctx context.Context, tx db.Tx, limit int) ([]DisputeSLARow, error)
}

// ========================================================================
// DOMAIN ERRORS
// ========================================================================

var (
	// ErrTicketNotFound is returned when a ticket is not found.
	ErrTicketNotFound = errorString("ticket not found")

	// ErrDuplicateOpenTicket is returned when a user already has an open ticket.
	ErrDuplicateOpenTicket = errorString("user already has an open ticket")

	// ErrTicketAlreadyClaimed is returned when attempting to claim an already claimed ticket.
	ErrTicketAlreadyClaimed = errorString("ticket is already claimed")

	// ErrConcurrentUpdate is returned when a concurrent update is detected.
	ErrConcurrentUpdate = errorString("concurrent update detected")

	// ErrInvalidStatusTransition is returned when an invalid status transition is attempted.
	ErrInvalidStatusTransition = errorString("invalid status transition")

	// ErrAdminNotFound is returned when a support admin is not found.
	ErrAdminNotFound = errorString("support admin not found")

	// ErrInvalidPriority is returned when an invalid priority is provided.
	ErrInvalidPriority = errorString("invalid priority")

	// ErrInvalidCategory is returned when an invalid category is provided.
	ErrInvalidCategory = errorString("invalid category")

	// ErrInvalidStatus is returned when an invalid status is provided.
	ErrInvalidStatus = errorString("invalid status")

	// ErrCannotReopenTicket is returned when a ticket cannot be reopened.
	ErrCannotReopenTicket = errorString("ticket cannot be reopened")
)

// errorString is a string type that implements error.
type errorString string

func (e errorString) Error() string {
	return string(e)
}

// ========================================================================
// FILTERS AND DTOs
// ========================================================================

// TicketFilter defines filters for listing tickets.
type TicketFilter struct {
	UserID          *uuid.UUID
	AssignedAdminID *uuid.UUID
	Status          *entity.Status
	Category        *entity.Category
	Priority        *entity.Priority
	LinkedOrderID   *uuid.UUID
	IsOverdue       *bool
	IsUnassigned    *bool
}

// TicketStatistics contains aggregate statistics about tickets.
type TicketStatistics struct {
	TotalTickets       int64
	OpenTickets        int64
	InProgressTickets  int64
	WaitingUserTickets int64
	ResolvedTickets    int64
	ClosedTickets      int64
	UnassignedTickets  int64
}

// TicketSLARow is the projection of a ticket row used by the SLA escalation
// worker. Nullable columns (assigned_at, resolved_at) are pointers; nil means
// "not set". This typed shape replaces the legacy map[string]interface{}
// projection that forced the worker into unchecked type assertions.
type TicketSLARow struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Status     string
	CreatedAt  time.Time
	AssignedAt *time.Time
	ResolvedAt *time.Time
}

// DisputeSLARow is the projection of a dispute row used by the SLA escalation
// worker. ResolvedAt is nullable (nil when the dispute is still open).
type DisputeSLARow struct {
	ID         uuid.UUID
	OrderID    uuid.UUID
	BuyerID    uuid.UUID
	SellerID   uuid.UUID
	Status     string
	OpenedAt   time.Time
	ResolvedAt *time.Time
}


