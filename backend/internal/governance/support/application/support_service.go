package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/governance/support/entity"
	infraRepo "github.com/labuda/backend/internal/governance/support/infrastructure/repository"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
)

const (
	// MaxConcurrentTicketsPerAdmin is the maximum number of active tickets an admin can handle.
	MaxConcurrentTicketsPerAdmin = 10

	// DefaultTicketListLimit is the default limit for listing tickets.
	DefaultTicketListLimit = 50

	// MaxTicketListLimit is the maximum limit for listing tickets.
	MaxTicketListLimit = 100
)

// Service handles support domain business logic.
//
// STRICT BOUNDARY RULES:
// - Integrates with existing chat infrastructure
// - Uses transactions for multi-step operations
// - Emits audit events for all state changes
// - Allows multiple tickets per user for different issues
// - FINANCIAL BOUNDARY: Does NOT mutate escrow (dispute domain handles escrow)
type Service struct {
	db             Transactor
	repo           supportRepo.Repository
	chatService    ChatService
	outboxRepo     OutboxInserter
	orderService   OrderEscrowService
	disputeService DisputeService
	log            *zap.Logger
}

// ChatService defines the seam between Support and Chat.
//
// Support owns the case (identity, lifecycle, assignment, SLA, resolution);
// Chat owns the conversation (room, messages, transport, persistence). Support
// never writes synthetic messages into the conversation — the only messages a
// support room contains are ones a user or a real agent authored.
type ChatService interface {
	// CreateSupportTicketRoom provisions a NEW unique chat room for each
	// support ticket. Each ticket gets its own room to prevent thread mixing.
	//
	// Room-level commerce context is NOT stored on chat rooms (removed schema);
	// ticket linkage is carried by support_tickets.chat_room_id and the
	// ticket's linked_order_id.
	CreateSupportTicketRoom(ctx context.Context, userID uuid.UUID) (*chatEntity.ChatRoom, error)
}

// OutboxInserter defines the interface for inserting outbox events.
type OutboxInserter interface {
	InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}

// OrderEscrowService defines the interface for order validation.
type OrderEscrowService interface {
	// GetOrderForValidation retrieves an order by ID for ownership validation.
	// Returns the buyer and seller IDs, or an error if the order doesn't exist.
	GetOrderForValidation(ctx context.Context, orderID uuid.UUID) (buyerID, sellerID uuid.UUID, err error)
}

// DisputeService defines the interface for dispute operations.
type DisputeService interface {
	// OpenDispute opens a new dispute for an order.
	OpenDispute(
		ctx context.Context,
		tx db.Tx,
		orderID uuid.UUID,
		callerID uuid.UUID,
		input OpenDisputeInput,
	) (*interface{}, error)

	// GetDisputeByOrderID retrieves a dispute by order ID.
	GetDisputeByOrderID(
		ctx context.Context,
		tx db.Tx,
		orderID uuid.UUID,
	) (*interface{}, error)
}

// OpenDisputeInput contains the input for opening a dispute.
type OpenDisputeInput struct {
	Reason      string
	Description *string
	MediaURLs   []string
	VideoURL    *string
	ReasonCode  string
}

// Transactor represents the ability to execute functions within transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// NewService creates a new support service.
func NewService(
	db Transactor,
	repo supportRepo.Repository,
	chatService ChatService,
	outboxRepo OutboxInserter,
	orderService OrderEscrowService,
	disputeService DisputeService,
	log *zap.Logger,
) *Service {
	return &Service{
		db:             db,
		repo:           repo,
		chatService:    chatService,
		outboxRepo:     outboxRepo,
		orderService:   orderService,
		disputeService: disputeService,
		log:            log,
	}
}

// NewServiceWithDefaults creates a support service with the default repository.
func NewServiceWithDefaults(
	db Transactor,
	chatService ChatService,
	outboxRepo OutboxInserter,
	orderService OrderEscrowService,
	disputeService DisputeService,
	log *zap.Logger,
) *Service {
	return NewService(
		db,
		infraRepo.NewSupportRepository(),
		chatService,
		outboxRepo,
		orderService,
		disputeService,
		log,
	)
}

// ========================================================================
// TICKET CREATION
// ========================================================================

// CreateTicketRequest contains the parameters for creating a ticket.
type CreateTicketRequest struct {
	UserID        uuid.UUID
	Category      entity.Category
	Priority      entity.Priority
	LinkedOrderID *uuid.UUID
	Subject       *string
	Description   *string
}

// CreateTicket creates a new support ticket.
//
// Transaction flow:
// 1. Validate order ownership if linked_order_id provided
// 2. Create or get support chat room WITH order context if linked_order_id provided
// 3. Create support ticket
// 4. Create ticket_created event
// 5. Send system message to chat
// 6. Emit outbox event for admin notification
//
// Business rules:
// - Users can have multiple tickets for different issues
// - Ticket must have a linked chat room
// - System message sent to notify user
// - ORDER ↔ CHAT CONTINUITY: linked_order_id is stored in chat room context for mobile retrieval
// - FINANCIAL BOUNDARY: Does NOT freeze escrow (dispute domain handles escrow operations)
func (s *Service) CreateTicket(ctx context.Context, req *CreateTicketRequest) (*entity.Ticket, error) {
	// Validate the canonical taxonomy BEFORE any room is provisioned, so an
	// invalid category/priority can never leave an orphan support room behind.
	// The HTTP layer enforces the same vocabulary via request binding; this is
	// the domain-level guarantee for every caller.
	if !req.Category.IsValid() {
		return nil, supportRepo.ErrInvalidCategory
	}
	priority := req.Priority
	if priority == "" {
		priority = entity.PriorityMedium
	}
	if !priority.IsValid() {
		return nil, supportRepo.ErrInvalidPriority
	}

	// Validate order ownership if linked_order_id is provided
	if req.LinkedOrderID != nil {
		if s.orderService == nil {
			return nil, fmt.Errorf("order service not available")
		}
		buyerID, sellerID, err := s.orderService.GetOrderForValidation(ctx, *req.LinkedOrderID)
		if err != nil {
			return nil, fmt.Errorf("invalid order reference")
		}
		// Check if current user is buyer or seller
		if buyerID != req.UserID && sellerID != req.UserID {
			return nil, fmt.Errorf("invalid order reference")
		}
	}

	var ticket *entity.Ticket

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Create NEW support chat room for this ticket.
		// CRITICAL: Each ticket gets its own unique chat room to prevent thread mixing.
		// Room-level context is not stored; order linkage is carried on the ticket.
		chatRoom, err := s.chatService.CreateSupportTicketRoom(ctx, req.UserID)
		if err != nil {
			return fmt.Errorf("create support ticket room failed: %w", err)
		}

		// Step 3: Create the ticket
		ticket = entity.NewTicket(req.UserID, chatRoom.ID, req.Category, priority)
		if req.LinkedOrderID != nil {
			ticket.LinkedOrderID = req.LinkedOrderID
		}

		// subject/description are first-class ticket fields — the single
		// canonical representation (no metadata duplication).
		ticket.Subject = req.Subject
		ticket.Description = req.Description

		// Step 4: Persist ticket
		if err := s.repo.CreateTicket(ctx, tx, ticket); err != nil {
			return fmt.Errorf("create ticket failed: %w", err)
		}

		// Step 5: Freeze escrow if linked_order_id provided
		// IMPORTANT: This freezes escrow WITHOUT creating a dispute
		// - Only freezes if escrow_status == "holding"
		// - Idempotent: if already frozen, skips silently
		// - Does NOT set HasDispute = true
		// - Does NOT change order status to dispute_open
		// FINANCIAL BOUNDARY: Support domain no longer mutates escrow
		s.log.Info("Support ticket created WITHOUT escrow freeze",
			zap.String("ticket_id", ticket.ID.String()),
			zap.String("user_id", req.UserID.String()),
		)

		// Step 6: Create ticket_created event
		event := entity.NewEvent(ticket.ID, entity.EventTypeTicketCreated, &req.UserID)
		if req.Subject != nil {
			event.WithNotes(*req.Subject)
		}
		if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
			return fmt.Errorf("create event failed: %w", err)
		}

		// NO CONVERSATION SYSTEM MESSAGE. Support lifecycle truth is the ticket
		// state + its event + the canonical support.ticket.* outbox events
		// below — never a synthetic message in the conversation. Chat messages
		// in a support room are authored by the user or by a real agent.

		// Step 7: OUTBOX ATOMIC — emit support.ticket.created in the same transaction.
		// InsertTx failure rolls back the entire transaction — no ticket commits
		// without its critical admin notification event.
		if s.outboxRepo != nil {
			payload := map[string]interface{}{
				"ticket_id":       ticket.ID,
				"user_id":         ticket.UserID,
				"category":        ticket.Category.String(),
				"priority":        ticket.Priority.String(),
				"chat_room_id":    ticket.ChatRoomID,
				"linked_order_id": ticket.LinkedOrderID,
			}
			if err := s.outboxRepo.InsertTx(ctx, tx, "support.ticket.created", payload, ticket.ID.String()); err != nil {
				return fmt.Errorf("outbox support.ticket.created: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return ticket, nil
}

// ========================================================================
// TICKET QUERIES
// ========================================================================

// TicketEnriched contains enriched ticket information with order and dispute details.
type TicketEnriched struct {
	Ticket      *entity.Ticket
	OrderInfo   *OrderInfo
	DisputeInfo *DisputeInfo
}

// OrderInfo contains order information for support ticket context.
type OrderInfo struct {
	OrderID      uuid.UUID
	Status       string
	EscrowStatus string
	HasDispute   bool
}

// DisputeInfo contains dispute information for support ticket context.
type DisputeInfo struct {
	DisputeID  uuid.UUID
	Status     string
	OpenedAt   time.Time
	ResolvedAt *time.Time
	ResolvedBy *uuid.UUID
}

// GetTicket retrieves a ticket by ID.
func (s *Service) GetTicket(ctx context.Context, ticketID uuid.UUID) (*entity.Ticket, error) {
	var ticket *entity.Ticket
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		ticket, err = s.repo.GetTicketByID(ctx, tx, ticketID)
		return err
	})
	return ticket, err
}

// GetTicketEnriched retrieves a ticket with order and dispute information.
// This is used for admin visibility into related order and dispute status.
func (s *Service) GetTicketEnriched(ctx context.Context, ticketID uuid.UUID) (*TicketEnriched, error) {
	var ticket *entity.Ticket
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		ticket, err = s.repo.GetTicketByID(ctx, tx, ticketID)
		return err
	})
	if err != nil {
		return nil, err
	}

	enriched := &TicketEnriched{
		Ticket: ticket,
	}

	// If ticket has a linked order, fetch order and dispute information
	if ticket.LinkedOrderID != nil {
		err := s.db.WithTx(ctx, func(tx db.Tx) error {
			// Query order information
			var orderStatus, orderEscrowStatus string
			var hasDispute bool
			orderErr := tx.QueryRow(ctx, `
				SELECT status, escrow_status, has_dispute
				FROM orders WHERE id = $1
			`, *ticket.LinkedOrderID).Scan(&orderStatus, &orderEscrowStatus, &hasDispute)

			if orderErr == nil {
				enriched.OrderInfo = &OrderInfo{
					OrderID:      *ticket.LinkedOrderID,
					Status:       orderStatus,
					EscrowStatus: orderEscrowStatus,
					HasDispute:   hasDispute,
				}

				// If order has a dispute, fetch dispute information
				if hasDispute {
					var disputeID uuid.UUID
					var disputeStatus string
					var openedAt time.Time
					var resolvedAt *time.Time
					var resolvedBy *uuid.UUID

					disputeErr := tx.QueryRow(ctx, `
						SELECT id, status, opened_at, resolved_at, resolved_by
						FROM disputes WHERE order_id = $1
					`, *ticket.LinkedOrderID).Scan(&disputeID, &disputeStatus, &openedAt, &resolvedAt, &resolvedBy)

					if disputeErr == nil {
						enriched.DisputeInfo = &DisputeInfo{
							DisputeID:  disputeID,
							Status:     disputeStatus,
							OpenedAt:   openedAt,
							ResolvedAt: resolvedAt,
							ResolvedBy: resolvedBy,
						}
					}
				}
			}

			return nil
		})

		if err != nil {
			s.log.Error("Failed to fetch order/dispute info",
				zap.String("ticket_id", ticketID.String()),
				zap.Error(err),
			)
			// Don't fail the request, just return partial data
		}
	}

	return enriched, nil
}

// ListTickets lists tickets with optional filters.
func (s *Service) ListTickets(
	ctx context.Context,
	filter *supportRepo.TicketFilter,
	cursorCreatedAt *time.Time,
	cursorID *uuid.UUID,
	limit int,
) ([]*entity.Ticket, error) {
	if limit <= 0 {
		limit = DefaultTicketListLimit
	}
	if limit > MaxTicketListLimit {
		limit = MaxTicketListLimit
	}

	var tickets []*entity.Ticket
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		tickets, err = s.repo.ListTickets(ctx, tx, filter, cursorCreatedAt, cursorID, limit)
		return err
	})
	return tickets, err
}

// ListMyTickets lists tickets for a specific user.
func (s *Service) ListMyTickets(
	ctx context.Context,
	userID uuid.UUID,
	cursorCreatedAt *time.Time,
	cursorID *uuid.UUID,
	limit int,
) ([]*entity.Ticket, error) {
	filter := &supportRepo.TicketFilter{
		UserID: &userID,
	}
	return s.ListTickets(ctx, filter, cursorCreatedAt, cursorID, limit)
}

// CountTickets returns the count of tickets matching the filter.
func (s *Service) CountTickets(ctx context.Context, filter *supportRepo.TicketFilter) (int64, error) {
	var count int64
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		count, err = s.repo.CountTickets(ctx, tx, filter)
		return err
	})
	return count, err
}

// ========================================================================
// TICKET ACTIONS
// ========================================================================

// ClaimTicketRequest contains the parameters for claiming a ticket.
type ClaimTicketRequest struct {
	TicketID uuid.UUID
	AdminID  uuid.UUID
}

// ClaimTicket allows an admin to claim an open ticket.
//
// Transaction flow:
// 1. Lock ticket row (SELECT FOR UPDATE)
// 2. Verify status is open and not assigned
// 3. Assign admin and update status to in_progress
// 4. Create ticket_claimed event
// 5. Send greeting message to chat
func (s *Service) ClaimTicket(ctx context.Context, req *ClaimTicketRequest) (*entity.Ticket, error) {
	var ticket *entity.Ticket

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error

		// Step 1-3: Claim ticket (with row locking)
		ticket, err = s.repo.ClaimTicket(ctx, tx, req.TicketID, req.AdminID)
		if err != nil {
			return err
		}

		// Step 4: Create ticket_claimed event
		event := entity.NewClaimedEvent(ticket.ID, req.AdminID)
		if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
			return fmt.Errorf("create event failed: %w", err)
		}

		// NO SYNTHETIC GREETING. The authenticating agent's first real reply is
		// their greeting; the claim itself is recorded by the ticket event above.

		return nil
	})

	if err != nil {
		return nil, err
	}

	return ticket, nil
}

// ResolveTicketRequest contains the parameters for resolving a ticket.
type ResolveTicketRequest struct {
	TicketID uuid.UUID
	AdminID  uuid.UUID
	Notes    *string
}

// ResolveTicket resolves a ticket.
//
// Business rules:
// - Only assigned admin can resolve
// - Status must be in_progress or waiting_user
func (s *Service) ResolveTicket(ctx context.Context, req *ResolveTicketRequest) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Verify admin is assigned to this ticket
		ticket, err := s.repo.GetTicketByID(ctx, tx, req.TicketID)
		if err != nil {
			return err
		}

		if ticket.AssignedAdminID == nil || *ticket.AssignedAdminID != req.AdminID {
			return supportRepo.ErrTicketNotFound // Not authorized
		}

		// Resolve the ticket
		if err := s.repo.ResolveTicket(ctx, tx, req.TicketID, req.Notes); err != nil {
			return err
		}

		// Create event
		event := entity.NewResolvedEvent(req.TicketID, &req.AdminID, req.Notes)
		if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
			return fmt.Errorf("create event failed: %w", err)
		}

		// Lifecycle truth is the ticket status + the event above; the
		// conversation carries only what the user and agents actually wrote.

		// OUTBOX ATOMIC — emit support.ticket.resolved in the same transaction.
		// Per-transition key allows reopen→resolve cycles to each emit.
		if s.outboxRepo != nil {
			payload := map[string]interface{}{
				"ticket_id":    ticket.ID.String(),
				"user_id":      ticket.UserID.String(),
				"chat_room_id": ticket.ChatRoomID.String(),
				"status":       "resolved",
			}
			idempotencyKey := fmt.Sprintf("support.ticket.resolved.%s.%d", ticket.ID.String(), time.Now().UnixMilli())
			if err := s.outboxRepo.InsertTx(ctx, tx, "support.ticket.resolved", payload, idempotencyKey); err != nil {
				return fmt.Errorf("outbox support.ticket.resolved: %w", err)
			}
		}

		return nil
	})
}

// CloseTicketRequest contains the parameters for closing a ticket.
type CloseTicketRequest struct {
	TicketID    uuid.UUID
	AdminID     uuid.UUID
	CloseReason *string
}

// CloseTicket closes a resolved ticket.
//
// ASSIGNMENT AUTHORITY: only the assigned support agent may close a ticket.
// A generic resolve capability must not let one agent close another agent's
// ticket.
func (s *Service) CloseTicket(ctx context.Context, req *CloseTicketRequest) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Verify the caller is the assigned agent before mutating.
		existing, err := s.repo.GetTicketByID(ctx, tx, req.TicketID)
		if err != nil {
			return err
		}
		if existing.AssignedAdminID == nil || *existing.AssignedAdminID != req.AdminID {
			return supportRepo.ErrTicketNotFound // Not authorized to close this ticket
		}

		// Close the ticket
		if err := s.repo.CloseTicket(ctx, tx, req.TicketID, req.CloseReason); err != nil {
			return err
		}

		// Get ticket for chat room ID
		ticket, err := s.repo.GetTicketByID(ctx, tx, req.TicketID)
		if err != nil {
			return err
		}

		// Create event
		event := entity.NewClosedEvent(req.TicketID, &req.AdminID, req.CloseReason)
		if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
			return fmt.Errorf("create event failed: %w", err)
		}

		// Lifecycle truth is the ticket status + the event above; the
		// conversation carries only what the user and agents actually wrote.

		// OUTBOX ATOMIC — emit support.ticket.closed in the same transaction.
		// Per-transition key allows reopen→close cycles to each emit.
		if s.outboxRepo != nil {
			payload := map[string]interface{}{
				"ticket_id":    ticket.ID.String(),
				"user_id":      ticket.UserID.String(),
				"chat_room_id": ticket.ChatRoomID.String(),
				"status":       "closed",
			}
			idempotencyKey := fmt.Sprintf("support.ticket.closed.%s.%d", ticket.ID.String(), time.Now().UnixMilli())
			if err := s.outboxRepo.InsertTx(ctx, tx, "support.ticket.closed", payload, idempotencyKey); err != nil {
				return fmt.Errorf("outbox support.ticket.closed: %w", err)
			}
		}

		return nil
	})
}

// ReopenTicketRequest contains the parameters for reopening a ticket.
//
// TWO ACTORS, ONE TRANSITION: the ticket owner may reopen their own ticket,
// and a support agent with the resolve capability may reopen any resolved
// ticket. Both paths run the identical resolved -> open transition, so there is
// exactly one implementation; only the authorization differs.
//
// SECURITY: when IsAdmin is false the ticket owner is verified. When IsAdmin is
// true the HTTP layer has already enforced the resolve capability and the
// ownership check does not apply.
type ReopenTicketRequest struct {
	TicketID uuid.UUID

	// ActorID is the authenticated actor performing the reopen: the ticket
	// owner for a user-initiated reopen, or the support agent for an
	// admin-initiated reopen. It is recorded as the ticket event actor.
	ActorID uuid.UUID

	// IsAdmin marks an agent-initiated reopen. Capability is enforced by the
	// HTTP layer before this request is built.
	IsAdmin bool
}

// ReopenTicket reopens a RESOLVED ticket.
//
// closed is terminal — a closed case is never reopened (the repository encodes
// that invariant). Reopening returns the ticket to `open` with assignment
// cleared, so it re-enters the normal claim flow.
//
// LIFECYCLE TRUTH: the transition is recorded as a ticket event and the status
// change travels through the canonical ticket state. No chat system message is
// produced — Support lifecycle truth never lives in the conversation.
func (s *Service) ReopenTicket(ctx context.Context, req *ReopenTicketRequest) (*entity.Ticket, error) {
	var ticket *entity.Ticket

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		existing, err := s.repo.GetTicketByID(ctx, tx, req.TicketID)
		if err != nil {
			return err
		}

		// Ownership authority for owner-initiated reopen.
		if !req.IsAdmin && existing.UserID != req.ActorID {
			return supportRepo.ErrTicketNotFound
		}

		// Reopen the ticket (resolved -> open; closed is rejected).
		if err := s.repo.ReopenTicket(ctx, tx, req.TicketID); err != nil {
			return err
		}

		// Get updated ticket
		ticket, err = s.repo.GetTicketByID(ctx, tx, req.TicketID)
		if err != nil {
			return err
		}

		// Create event
		event := entity.NewReopenedEvent(req.TicketID, &req.ActorID)
		if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
			return fmt.Errorf("create event failed: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return ticket, nil
}

// EscalateToDisputeRequest contains the parameters for escalating a ticket to a dispute.
type EscalateToDisputeRequest struct {
	TicketID    uuid.UUID
	AdminID     uuid.UUID
	Reason      string
	Description *string
	ReasonCode  string
}

// EscalateToDispute escalates a support ticket to a formal dispute.
//
// Business rules:
// - Ticket must have a linked_order_id
// - Ticket must not already be escalated to dispute
// - Order must not already have an active dispute
// - Opens a new dispute via DisputeService
// - Updates ticket escalation to "dispute"
func (s *Service) EscalateToDispute(ctx context.Context, req *EscalateToDisputeRequest) (*entity.Ticket, error) {
	var ticket *entity.Ticket

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Load ticket
		var err error
		ticket, err = s.repo.GetTicketByID(ctx, tx, req.TicketID)
		if err != nil {
			return fmt.Errorf("ticket not found: %w", err)
		}

		// Step 2: Validate ticket has linked order
		if ticket.LinkedOrderID == nil {
			return fmt.Errorf("cannot escalate: ticket has no linked order")
		}

		// Step 3: Validate ticket is not already escalated to dispute
		if ticket.Escalation == entity.EscalationDispute {
			return fmt.Errorf("cannot escalate: ticket already escalated to dispute")
		}

		// Step 4: Check order has no dispute
		if s.disputeService == nil {
			return fmt.Errorf("dispute service not available")
		}

		_, err = s.disputeService.GetDisputeByOrderID(ctx, tx, *ticket.LinkedOrderID)
		if err == nil {
			// Dispute exists
			return fmt.Errorf("cannot escalate: order already has an active dispute")
		}

		// Step 5: Open dispute via DisputeService
		disputeInput := OpenDisputeInput{
			Reason:      req.Reason,
			Description: req.Description,
			MediaURLs:   []string{},
			VideoURL:    nil,
			ReasonCode:  req.ReasonCode,
		}

		_, err = s.disputeService.OpenDispute(ctx, tx, *ticket.LinkedOrderID, req.AdminID, disputeInput)
		if err != nil {
			return fmt.Errorf("failed to open dispute: %w", err)
		}

		// Step 6: Update ticket escalation to dispute
		if err := s.repo.UpdateEscalation(ctx, tx, req.TicketID, entity.EscalationDispute); err != nil {
			return fmt.Errorf("failed to update ticket escalation: %w", err)
		}

		// Get updated ticket
		ticket, err = s.repo.GetTicketByID(ctx, tx, req.TicketID)
		if err != nil {
			return err
		}

		// Step 7: Create event
		event := entity.NewEvent(ticket.ID, entity.EventTypeTicketEscalated, &req.AdminID)
		event.WithNotes("Escalated to dispute: " + req.Reason)
		if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
			return fmt.Errorf("create event failed: %w", err)
		}

		// Lifecycle truth is the ticket status/escalation + the event above; the
		// conversation carries only what the user and agents actually wrote.

		return nil
	})

	if err != nil {
		return nil, err
	}

	return ticket, nil
}

// ========================================================================
// TICKET UPDATES
// ========================================================================

// UpdatePriority updates the ticket priority.
func (s *Service) UpdatePriority(ctx context.Context, ticketID uuid.UUID, priority entity.Priority, actorID *uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Get current ticket for old priority
		ticket, err := s.repo.GetTicketByID(ctx, tx, ticketID)
		if err != nil {
			return err
		}

		oldPriority := ticket.Priority

		// Update priority
		if err := s.repo.UpdatePriority(ctx, tx, ticketID, priority); err != nil {
			return err
		}

		// Create event
		event := entity.NewPriorityChangedEvent(ticketID, actorID, oldPriority, priority)
		if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
			return fmt.Errorf("create event failed: %w", err)
		}

		return nil
	})
}

// UpdateCategory updates the ticket category.
func (s *Service) UpdateCategory(ctx context.Context, ticketID uuid.UUID, category entity.Category, actorID *uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Get current ticket for old category
		ticket, err := s.repo.GetTicketByID(ctx, tx, ticketID)
		if err != nil {
			return err
		}

		oldCategory := ticket.Category

		// Update category
		if err := s.repo.UpdateCategory(ctx, tx, ticketID, category); err != nil {
			return err
		}

		// Create event
		event := entity.NewCategoryChangedEvent(ticketID, actorID, oldCategory, category)
		if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
			return fmt.Errorf("create event failed: %w", err)
		}

		return nil
	})
}

// SetWaitingForUser changes ticket status to waiting_user.
func (s *Service) SetWaitingForUser(ctx context.Context, ticketID uuid.UUID, adminID uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		ticket, err := s.repo.GetTicketByID(ctx, tx, ticketID)
		if err != nil {
			return err
		}

		if ticket.AssignedAdminID == nil || *ticket.AssignedAdminID != adminID {
			return supportRepo.ErrTicketNotFound
		}

		oldStatus := ticket.Status
		if err := s.repo.UpdateStatus(ctx, tx, ticketID, entity.StatusWaitingUser); err != nil {
			return err
		}

		// Create ticket_waiting_user event (specific event type for this transition)
		event := entity.NewWaitingUserEvent(ticketID, adminID)
		event.OldStatus = &oldStatus
		if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
			return fmt.Errorf("create event failed: %w", err)
		}

		// OUTBOX ATOMIC — emit support.ticket_waiting_user in the same transaction.
		// Per-transition key allows repeated in_progress→waiting_user cycles to each emit.
		if s.outboxRepo != nil {
			// admin_id is the ACTOR of this notification: the human agent whose
			// reply the user is being told about. Carrying it lets the
			// notification consumer persist a real actor identity instead of a
			// uuid.Nil sentinel (which the users FK rejects).
			payload := map[string]interface{}{
				"ticket_id":    ticket.ID.String(),
				"user_id":      ticket.UserID.String(),
				"admin_id":     adminID.String(),
				"chat_room_id": ticket.ChatRoomID.String(),
				"status":       "waiting_user",
			}
			idempotencyKey := fmt.Sprintf("support.ticket_waiting_user.%s.%d", ticket.ID.String(), time.Now().UnixMilli())
			if err := s.outboxRepo.InsertTx(ctx, tx, "support.ticket_waiting_user", payload, idempotencyKey); err != nil {
				return fmt.Errorf("outbox support.ticket_waiting_user: %w", err)
			}
		}

		return nil
	})
}

// ========================================================================
// USER REPLY HOOK
// ========================================================================

// HandleUserReply processes a user sending a message to a support chat room.
// If the ticket is in waiting_user status, transitions to in_progress and
// notifies the assigned admin. No-op if ticket is in any other status.
//
// This is called asynchronously via the outbox worker when a chat message
// is sent to a support room by the ticket owner.
func (s *Service) HandleUserReply(ctx context.Context, chatRoomID uuid.UUID, senderID uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		ticket, err := s.repo.GetTicketByChatRoomID(ctx, tx, chatRoomID)
		if err != nil {
			// No active ticket for this room — no-op (could be closed/resolved)
			if err == supportRepo.ErrTicketNotFound {
				return nil
			}
			return fmt.Errorf("get ticket by chat room failed: %w", err)
		}

		// Only the ticket owner's messages trigger the transition
		if ticket.UserID != senderID {
			return nil
		}

		// Only transition if waiting_user → in_progress
		if ticket.Status != entity.StatusWaitingUser {
			return nil
		}

		oldStatus := ticket.Status
		if err := s.repo.UpdateStatus(ctx, tx, ticket.ID, entity.StatusInProgress); err != nil {
			return fmt.Errorf("update status failed: %w", err)
		}

		// Create event
		event := entity.NewStatusChangeEvent(ticket.ID, &senderID, oldStatus, entity.StatusInProgress)
		event.Notes = strPtr("User replied to support ticket")
		if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
			s.log.Error("failed to create user reply event",
				zap.String("ticket_id", ticket.ID.String()),
				zap.Error(err),
			)
		}

		// OUTBOX ATOMIC — emit support.ticket.user_responded in the same transaction.
		// Uses distinct event type to avoid infinite loop with support.user_replied.
		if s.outboxRepo != nil && ticket.AssignedAdminID != nil {
			payload := map[string]interface{}{
				"ticket_id":    ticket.ID.String(),
				"user_id":      ticket.UserID.String(),
				"admin_id":     ticket.AssignedAdminID.String(),
				"chat_room_id": ticket.ChatRoomID.String(),
				"status":       "in_progress",
			}
			idempotencyKey := fmt.Sprintf("support.ticket.user_responded.%s.%d", ticket.ID.String(), time.Now().UnixMilli())
			if err := s.outboxRepo.InsertTx(ctx, tx, "support.ticket.user_responded", payload, idempotencyKey); err != nil {
				return fmt.Errorf("outbox support.ticket.user_responded: %w", err)
			}
		}

		return nil
	})
}

// strPtr returns a pointer to the given string.
func strPtr(s string) *string {
	return &s
}

// ========================================================================
// STATISTICS
// ========================================================================

// GetStatistics returns ticket statistics.
func (s *Service) GetStatistics(ctx context.Context) (*supportRepo.TicketStatistics, error) {
	var stats *supportRepo.TicketStatistics
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		stats, err = s.repo.GetTicketStatistics(ctx, tx)
		return err
	})
	return stats, err
}

// ========================================================================
// EVENT QUERIES
// ========================================================================

// ListEvents lists all events for a ticket.
func (s *Service) ListEvents(ctx context.Context, ticketID uuid.UUID, limit int) ([]*entity.Event, error) {
	if limit <= 0 {
		limit = 100
	}
	var events []*entity.Event
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		events, err = s.repo.ListEvents(ctx, tx, ticketID, limit)
		return err
	})
	return events, err
}

// ListStatusEventsForTickets batch-fetches status-change events for multiple
// tickets. Returns events keyed by ticket_id. Used by list/dashboard views
// to compute canonical resolution SLA without N+1 queries.
func (s *Service) ListStatusEventsForTickets(ctx context.Context, ticketIDs []uuid.UUID) (map[uuid.UUID][]*entity.Event, error) {
	var result map[uuid.UUID][]*entity.Event
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = s.repo.ListStatusEventsForTickets(ctx, tx, ticketIDs)
		return err
	})
	return result, err
}

// ListFirstAdminResponsesByTicketIDs batch-fetches the first valid admin
// response timestamp for multiple tickets in a single query. Canonical
// first-response authority for SLA-F04: the earliest non-deleted admin-role
// chat message in each ticket's own support conversation. Tickets without any
// admin response are absent from the returned map. Used by list/detail and
// dashboard views to compute first-response SLA without N+1 queries.
func (s *Service) ListFirstAdminResponsesByTicketIDs(ctx context.Context, ticketIDs []uuid.UUID) (map[uuid.UUID]*time.Time, error) {
	var result map[uuid.UUID]*time.Time
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = s.repo.ListFirstAdminResponsesByTicketIDs(ctx, tx, ticketIDs)
		return err
	})
	return result, err
}
