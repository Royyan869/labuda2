package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/support/entity"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	"github.com/labuda/backend/pkg/db"
)

// SupportRepositoryImpl implements the support repository using pgx.
type SupportRepositoryImpl struct{}

// NewSupportRepository creates a new SupportRepository.
func NewSupportRepository() supportRepo.Repository {
	return &SupportRepositoryImpl{}
}

// toTx wraps the transaction interface for type casting.
func toTx(tx interface{}) db.Tx {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		panic(fmt.Sprintf("invalid transaction type: %T", tx))
	}
	return dbTx
}

// ========================================================================
// TICKET OPERATIONS
// ========================================================================

// CreateTicket creates a new support ticket.
func (r *SupportRepositoryImpl) CreateTicket(ctx context.Context, tx interface{}, ticket *entity.Ticket) error {
	query := `
		INSERT INTO support_tickets (
			id, user_id, chat_room_id, category, priority, status, escalation,
			subject, description,
			linked_order_id, assigned_admin_id, created_at, updated_at,
			assigned_at, resolved_at, closed_at, resolution_notes, close_reason, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
	`

	_, err := toTx(tx).Exec(ctx, query,
		ticket.ID, ticket.UserID, ticket.ChatRoomID, ticket.Category, ticket.Priority, ticket.Status, ticket.Escalation,
		ticket.Subject, ticket.Description,
		ticket.LinkedOrderID, ticket.AssignedAdminID, ticket.CreatedAt, ticket.UpdatedAt,
		ticket.AssignedAt, ticket.ResolvedAt, ticket.ClosedAt, ticket.ResolutionNotes, ticket.CloseReason,
		ticket.GetMetadataJSON(),
	)

	if err != nil {
		return fmt.Errorf("create ticket failed: %w", err)
	}

	return nil
}

// GetTicketByID retrieves a ticket by ID.
func (r *SupportRepositoryImpl) GetTicketByID(ctx context.Context, tx interface{}, ticketID uuid.UUID) (*entity.Ticket, error) {
	query := `
		SELECT st.id, st.user_id,
		       COALESCE(up.username, '') AS username,
		       COALESCE(sp.store_name, '') AS seller_farm_name,
		       st.chat_room_id, st.category, st.priority, st.status, st.escalation,
		       st.subject, st.description,
		       st.linked_order_id, st.assigned_admin_id, st.created_at, st.updated_at,
		       st.assigned_at, st.resolved_at, st.closed_at, st.resolution_notes, st.close_reason, st.metadata
		FROM support_tickets st
		LEFT JOIN user_profiles up ON up.user_id = st.user_id
		LEFT JOIN seller_profiles sp ON sp.user_id = st.user_id
		WHERE st.id = $1
	`

	var ticket entity.Ticket
	var metadataJSON []byte

	err := toTx(tx).QueryRow(ctx, query, ticketID).Scan(
		&ticket.ID, &ticket.UserID, &ticket.Username, &ticket.SellerFarmName, &ticket.ChatRoomID, &ticket.Category, &ticket.Priority, &ticket.Status, &ticket.Escalation,
		&ticket.Subject, &ticket.Description,
		&ticket.LinkedOrderID, &ticket.AssignedAdminID, &ticket.CreatedAt, &ticket.UpdatedAt,
		&ticket.AssignedAt, &ticket.ResolvedAt, &ticket.ClosedAt, &ticket.ResolutionNotes, &ticket.CloseReason,
		&metadataJSON,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, supportRepo.ErrTicketNotFound
		}
		return nil, fmt.Errorf("get ticket by id failed: %w", err)
	}

	if metadataJSON != nil {
		ticket.SetMetadataFromJSON(metadataJSON)
	}

	return &ticket, nil
}

// GetTicketByChatRoomID retrieves the active ticket linked to a chat room.
// Used by the user-reply hook to transition ticket status when a user
// sends a message in a support chat room.
func (r *SupportRepositoryImpl) GetTicketByChatRoomID(ctx context.Context, tx interface{}, chatRoomID uuid.UUID) (*entity.Ticket, error) {
	query := `
		SELECT st.id, st.user_id,
		       COALESCE(up.username, '') AS username,
		       COALESCE(sp.store_name, '') AS seller_farm_name,
		       st.chat_room_id, st.category, st.priority, st.status, st.escalation,
		       st.subject, st.description,
		       st.linked_order_id, st.assigned_admin_id, st.created_at, st.updated_at,
		       st.assigned_at, st.resolved_at, st.closed_at, st.resolution_notes, st.close_reason, st.metadata
		FROM support_tickets st
		LEFT JOIN user_profiles up ON up.user_id = st.user_id
		LEFT JOIN seller_profiles sp ON sp.user_id = st.user_id
		WHERE st.chat_room_id = $1 AND st.status IN ('open', 'in_progress', 'waiting_user')
		ORDER BY st.created_at DESC
		LIMIT 1
	`

	var ticket entity.Ticket
	var metadataJSON []byte

	err := toTx(tx).QueryRow(ctx, query, chatRoomID).Scan(
		&ticket.ID, &ticket.UserID, &ticket.Username, &ticket.SellerFarmName, &ticket.ChatRoomID, &ticket.Category, &ticket.Priority, &ticket.Status, &ticket.Escalation,
		&ticket.Subject, &ticket.Description,
		&ticket.LinkedOrderID, &ticket.AssignedAdminID, &ticket.CreatedAt, &ticket.UpdatedAt,
		&ticket.AssignedAt, &ticket.ResolvedAt, &ticket.ClosedAt, &ticket.ResolutionNotes, &ticket.CloseReason,
		&metadataJSON,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, supportRepo.ErrTicketNotFound
		}
		return nil, fmt.Errorf("get ticket by chat room id failed: %w", err)
	}

	if metadataJSON != nil {
		ticket.SetMetadataFromJSON(metadataJSON)
	}

	return &ticket, nil
}

// ListTickets lists all tickets with optional filters.
//
// PRIORITY SORTING: Orders by priority DESC, then created_at ASC
// - Urgent tickets first
// - Then high, medium, low
// - Within same priority: oldest first (created_at ASC)
func (r *SupportRepositoryImpl) ListTickets(
	ctx context.Context,
	tx interface{},
	filter *supportRepo.TicketFilter,
	cursorCreatedAt *time.Time,
	cursorID *uuid.UUID,
	limit int,
) ([]*entity.Ticket, error) {
	baseQuery := `
		SELECT st.id, st.user_id,
		       COALESCE(up.username, '') AS username,
		       COALESCE(sp.store_name, '') AS seller_farm_name,
		       st.chat_room_id, st.category, st.priority, st.status, st.escalation,
		       st.subject, st.description,
		       st.linked_order_id, st.assigned_admin_id, st.created_at, st.updated_at,
		       st.assigned_at, st.resolved_at, st.closed_at, st.resolution_notes, st.close_reason, st.metadata
		FROM support_tickets st
		LEFT JOIN user_profiles up ON up.user_id = st.user_id
		LEFT JOIN seller_profiles sp ON sp.user_id = st.user_id
		WHERE 1=1
	`

	args := []interface{}{}
	argIdx := 1

	// Apply filters
	if filter != nil {
		if filter.UserID != nil {
			baseQuery += fmt.Sprintf(" AND st.user_id = $%d", argIdx)
			args = append(args, *filter.UserID)
			argIdx++
		}
		if filter.AssignedAdminID != nil {
			baseQuery += fmt.Sprintf(" AND st.assigned_admin_id = $%d", argIdx)
			args = append(args, *filter.AssignedAdminID)
			argIdx++
		}
		if filter.Status != nil {
			baseQuery += fmt.Sprintf(" AND st.status = $%d", argIdx)
			args = append(args, *filter.Status)
			argIdx++
		}
		if filter.Category != nil {
			baseQuery += fmt.Sprintf(" AND st.category = $%d", argIdx)
			args = append(args, *filter.Category)
			argIdx++
		}
		if filter.Priority != nil {
			baseQuery += fmt.Sprintf(" AND st.priority = $%d", argIdx)
			args = append(args, *filter.Priority)
			argIdx++
		}
		if filter.LinkedOrderID != nil {
			baseQuery += fmt.Sprintf(" AND st.linked_order_id = $%d", argIdx)
			args = append(args, *filter.LinkedOrderID)
			argIdx++
		}
		if filter.IsUnassigned != nil {
			if *filter.IsUnassigned {
				baseQuery += " AND st.assigned_admin_id IS NULL"
			} else {
				baseQuery += " AND st.assigned_admin_id IS NOT NULL"
			}
		}
		if filter.IsOverdue != nil {
			// SLA-F04 canonical first-response authority: a ticket has been
			// first-responded when an admin-role message exists in its support
			// conversation (EXISTS subquery, same authority as
			// ListFirstAdminResponsesByTicketIDs). assigned_at is deliberately
			// NOT a response — assignment only. Unresponded tickets are first
			// response overdue after 1h wall clock; resolution overdue after 24h.
			firstResponseOverdue := `(NOT EXISTS (
				SELECT 1 FROM chat_rooms cr
				JOIN chat_messages cm ON cm.room_id = cr.id
				JOIN users u ON u.id = cm.sender_id AND u.role = 'admin'
				WHERE cr.id = st.chat_room_id AND cr.room_type = 'support'
				AND cm.deleted_at IS NULL AND cm.created_at >= st.created_at
			) AND st.created_at < NOW() - INTERVAL '1 hour')`
			resolutionOverdue := "((st.resolved_at IS NULL AND st.status NOT IN ('resolved', 'closed') AND st.created_at < NOW() - INTERVAL '24 hours') OR (st.resolved_at IS NOT NULL AND st.resolved_at - st.created_at > INTERVAL '24 hours'))"
			if *filter.IsOverdue {
				// Overdue: first response overdue OR resolution overdue
				baseQuery += " AND ((" + firstResponseOverdue + ") OR (" + resolutionOverdue + "))"
			} else {
				baseQuery += " AND NOT ((" + firstResponseOverdue + ") OR (" + resolutionOverdue + "))"
			}
		}
	}

	// Add cursor conditions for pagination
	if cursorCreatedAt != nil && cursorID != nil {
		baseQuery += fmt.Sprintf(" AND (st.priority, st.created_at, st.id) < ($%d, $%d, $%d)", argIdx, argIdx+1, argIdx+2)
		args = append(args, *cursorCreatedAt, *cursorID)
		argIdx += 3
	}

	// PRIORITY SORTING: Order by priority DESC, then created_at ASC, then id DESC
	// This ensures urgent tickets are first, and within same priority, oldest tickets are first
	baseQuery += fmt.Sprintf(" ORDER BY st.priority DESC, st.created_at ASC, st.id DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := toTx(tx).Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("list tickets failed: %w", err)
	}
	defer rows.Close()

	var tickets []*entity.Ticket
	for rows.Next() {
		var ticket entity.Ticket
		var metadataJSON []byte

		err := rows.Scan(
			&ticket.ID, &ticket.UserID, &ticket.Username, &ticket.SellerFarmName, &ticket.ChatRoomID, &ticket.Category, &ticket.Priority, &ticket.Status, &ticket.Escalation,
			&ticket.Subject, &ticket.Description,
			&ticket.LinkedOrderID, &ticket.AssignedAdminID, &ticket.CreatedAt, &ticket.UpdatedAt,
			&ticket.AssignedAt, &ticket.ResolvedAt, &ticket.ClosedAt, &ticket.ResolutionNotes, &ticket.CloseReason,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ticket failed: %w", err)
		}

		if metadataJSON != nil {
			ticket.SetMetadataFromJSON(metadataJSON)
		}

		tickets = append(tickets, &ticket)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate tickets failed: %w", rows.Err())
	}

	return tickets, nil
}

// CountTickets returns the count of tickets matching the filter.
func (r *SupportRepositoryImpl) CountTickets(ctx context.Context, tx interface{}, filter *supportRepo.TicketFilter) (int64, error) {
	baseQuery := `SELECT COUNT(*) FROM support_tickets WHERE 1=1`

	args := []interface{}{}
	argIdx := 1

	if filter != nil {
		if filter.UserID != nil {
			baseQuery += fmt.Sprintf(" AND user_id = $%d", argIdx)
			args = append(args, *filter.UserID)
			argIdx++
		}
		if filter.AssignedAdminID != nil {
			baseQuery += fmt.Sprintf(" AND assigned_admin_id = $%d", argIdx)
			args = append(args, *filter.AssignedAdminID)
			argIdx++
		}
		if filter.Status != nil {
			baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
			args = append(args, *filter.Status)
			argIdx++
		}
		if filter.Category != nil {
			baseQuery += fmt.Sprintf(" AND category = $%d", argIdx)
			args = append(args, *filter.Category)
			argIdx++
		}
		if filter.Priority != nil {
			baseQuery += fmt.Sprintf(" AND priority = $%d", argIdx)
			args = append(args, *filter.Priority)
			argIdx++
		}
		if filter.LinkedOrderID != nil {
			baseQuery += fmt.Sprintf(" AND linked_order_id = $%d", argIdx)
			args = append(args, *filter.LinkedOrderID)
			argIdx++
		}
	}

	var count int64
	err := toTx(tx).QueryRow(ctx, baseQuery, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count tickets failed: %w", err)
	}

	return count, nil
}

// CountActiveTicketsByOrderID returns the count of active (open/in_progress/waiting_user) tickets for an order.
func (r *SupportRepositoryImpl) CountActiveTicketsByOrderID(ctx context.Context, tx interface{}, orderID uuid.UUID) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM support_tickets
		WHERE linked_order_id = $1
		  AND status IN ('open', 'in_progress', 'waiting_user')
	`

	var count int64
	err := toTx(tx).QueryRow(ctx, query, orderID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active tickets by order id failed: %w", err)
	}

	return count, nil
}

// ClaimTicket atomically claims an open ticket for an admin using SELECT FOR UPDATE.
//
// LOCK SCOPE: the row lock targets ONLY support_tickets (`FOR UPDATE OF st`).
// The username / seller farm name come from LEFT JOINed profile tables, and a
// bare FOR UPDATE is rejected by PostgreSQL on the nullable side of an outer
// join ("FOR UPDATE cannot be applied to the nullable side of an outer join").
// Locking just the ticket row keeps the claim atomic without locking profiles.
func (r *SupportRepositoryImpl) ClaimTicket(ctx context.Context, tx interface{}, ticketID, adminID uuid.UUID) (*entity.Ticket, error) {
	// First, lock the ticket row for update
	query := `
		SELECT st.id, st.user_id,
		       COALESCE(up.username, '') AS username,
		       COALESCE(sp.store_name, '') AS seller_farm_name,
		       st.chat_room_id, st.category, st.priority, st.status, st.escalation,
		       st.subject, st.description,
		       st.linked_order_id, st.assigned_admin_id, st.created_at, st.updated_at,
		       st.assigned_at, st.resolved_at, st.closed_at, st.resolution_notes, st.close_reason, st.metadata
		FROM support_tickets st
		LEFT JOIN user_profiles up ON up.user_id = st.user_id
		LEFT JOIN seller_profiles sp ON sp.user_id = st.user_id
		WHERE st.id = $1
		FOR UPDATE OF st
	`

	var ticket entity.Ticket
	var metadataJSON []byte

	err := toTx(tx).QueryRow(ctx, query, ticketID).Scan(
		&ticket.ID, &ticket.UserID, &ticket.Username, &ticket.SellerFarmName, &ticket.ChatRoomID, &ticket.Category, &ticket.Priority, &ticket.Status, &ticket.Escalation,
		&ticket.Subject, &ticket.Description,
		&ticket.LinkedOrderID, &ticket.AssignedAdminID, &ticket.CreatedAt, &ticket.UpdatedAt,
		&ticket.AssignedAt, &ticket.ResolvedAt, &ticket.ClosedAt, &ticket.ResolutionNotes, &ticket.CloseReason,
		&metadataJSON,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, supportRepo.ErrTicketNotFound
		}
		return nil, fmt.Errorf("lock ticket failed: %w", err)
	}

	if metadataJSON != nil {
		ticket.SetMetadataFromJSON(metadataJSON)
	}

	// Check if ticket can be claimed
	if !ticket.CanBeClaimed() {
		if ticket.IsAssigned() {
			return nil, supportRepo.ErrTicketAlreadyClaimed
		}
		return nil, supportRepo.ErrInvalidStatusTransition
	}

	// Update the ticket
	now := time.Now()
	updateQuery := `
		UPDATE support_tickets
		SET assigned_admin_id = $1,
		    status = 'in_progress',
		    assigned_at = $2,
		    updated_at = $2
		WHERE id = $3
	`

	_, err = toTx(tx).Exec(ctx, updateQuery, adminID, now, ticketID)
	if err != nil {
		return nil, fmt.Errorf("claim ticket update failed: %w", err)
	}

	// Update the in-memory ticket
	ticket.AssignedAdminID = &adminID
	ticket.Status = entity.StatusInProgress
	ticket.AssignedAt = &now
	ticket.UpdatedAt = now

	return &ticket, nil
}

// ResolveTicket updates a ticket status to resolved.
func (r *SupportRepositoryImpl) ResolveTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID, notes *string) error {
	query := `
		UPDATE support_tickets
		SET status = 'resolved',
		    resolved_at = now(),
		    updated_at = now(),
		    resolution_notes = $1
		WHERE id = $2 AND status IN ('in_progress', 'waiting_user')
	`

	result, err := toTx(tx).Exec(ctx, query, notes, ticketID)
	if err != nil {
		return fmt.Errorf("resolve ticket failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return supportRepo.ErrInvalidStatusTransition
	}

	return nil
}

// CloseTicket updates a ticket status to closed.
func (r *SupportRepositoryImpl) CloseTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID, reason *string) error {
	query := `
		UPDATE support_tickets
		SET status = 'closed',
		    closed_at = now(),
		    updated_at = now(),
		    close_reason = $1
		WHERE id = $2 AND status = 'resolved'
	`

	result, err := toTx(tx).Exec(ctx, query, reason, ticketID)
	if err != nil {
		return fmt.Errorf("close ticket failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return supportRepo.ErrInvalidStatusTransition
	}

	return nil
}

// ReopenTicket reopens a RESOLVED ticket.
//
// closed is TERMINAL: a closed case is never reopened (a new problem is a new
// ticket). The WHERE clause encodes that lifecycle invariant, so a reopen
// attempt on a closed ticket affects no rows and returns ErrCannotReopenTicket.
func (r *SupportRepositoryImpl) ReopenTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID) error {
	query := `
		UPDATE support_tickets
		SET status = 'open',
		    assigned_admin_id = NULL,
		    assigned_at = NULL,
		    resolved_at = NULL,
		    closed_at = NULL,
		    resolution_notes = NULL,
		    close_reason = NULL,
		    updated_at = now()
		WHERE id = $1 AND status = 'resolved'
	`

	result, err := toTx(tx).Exec(ctx, query, ticketID)
	if err != nil {
		return fmt.Errorf("reopen ticket failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return supportRepo.ErrCannotReopenTicket
	}

	return nil
}

// UpdatePriority updates the ticket priority.
func (r *SupportRepositoryImpl) UpdatePriority(ctx context.Context, tx interface{}, ticketID uuid.UUID, priority entity.Priority) error {
	if !priority.IsValid() {
		return supportRepo.ErrInvalidPriority
	}

	query := `
		UPDATE support_tickets
		SET priority = $1, updated_at = now()
		WHERE id = $2
	`

	_, err := toTx(tx).Exec(ctx, query, priority, ticketID)
	if err != nil {
		return fmt.Errorf("update priority failed: %w", err)
	}

	return nil
}

// UpdateCategory updates the ticket category.
func (r *SupportRepositoryImpl) UpdateCategory(ctx context.Context, tx interface{}, ticketID uuid.UUID, category entity.Category) error {
	if !category.IsValid() {
		return supportRepo.ErrInvalidCategory
	}

	query := `
		UPDATE support_tickets
		SET category = $1, updated_at = now()
		WHERE id = $2
	`

	_, err := toTx(tx).Exec(ctx, query, category, ticketID)
	if err != nil {
		return fmt.Errorf("update category failed: %w", err)
	}

	return nil
}

// UpdateStatus updates the ticket status with validation.
func (r *SupportRepositoryImpl) UpdateStatus(ctx context.Context, tx interface{}, ticketID uuid.UUID, status entity.Status) error {
	if !status.IsValid() {
		return supportRepo.ErrInvalidStatus
	}

	query := `
		UPDATE support_tickets
		SET status = $1, updated_at = now()
		WHERE id = $2
	`

	_, err := toTx(tx).Exec(ctx, query, status, ticketID)
	if err != nil {
		return fmt.Errorf("update status failed: %w", err)
	}

	return nil
}

// UpdateEscalation updates the ticket escalation level.
func (r *SupportRepositoryImpl) UpdateEscalation(ctx context.Context, tx interface{}, ticketID uuid.UUID, escalation entity.Escalation) error {
	if !escalation.IsValid() {
		return fmt.Errorf("invalid escalation")
	}

	query := `
		UPDATE support_tickets
		SET escalation = $1, updated_at = now()
		WHERE id = $2
	`

	_, err := toTx(tx).Exec(ctx, query, escalation, ticketID)
	if err != nil {
		return fmt.Errorf("update escalation failed: %w", err)
	}

	return nil
}

// ========================================================================
// EVENT OPERATIONS
// ========================================================================

// CreateEvent creates a new ticket event.
func (r *SupportRepositoryImpl) CreateEvent(ctx context.Context, tx interface{}, event *entity.Event) error {
	query := `
		INSERT INTO support_ticket_events (
			id, ticket_id, event_type, actor_id, old_status, new_status, notes, metadata, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := toTx(tx).Exec(ctx, query,
		event.ID, event.TicketID, event.EventType, event.ActorID,
		event.OldStatus, event.NewStatus, event.Notes,
		event.GetMetadataJSON(), event.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("create event failed: %w", err)
	}

	return nil
}

// ListEvents lists all events for a ticket.
func (r *SupportRepositoryImpl) ListEvents(ctx context.Context, tx interface{}, ticketID uuid.UUID, limit int) ([]*entity.Event, error) {
	query := `
		SELECT id, ticket_id, event_type, actor_id, old_status, new_status, notes, metadata, created_at
		FROM support_ticket_events
		WHERE ticket_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := toTx(tx).Query(ctx, query, ticketID, limit)
	if err != nil {
		return nil, fmt.Errorf("list events failed: %w", err)
	}
	defer rows.Close()

	var events []*entity.Event
	for rows.Next() {
		var event entity.Event
		var metadataJSON []byte

		err := rows.Scan(
			&event.ID, &event.TicketID, &event.EventType, &event.ActorID,
			&event.OldStatus, &event.NewStatus, &event.Notes,
			&metadataJSON, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan event failed: %w", err)
		}

		if metadataJSON != nil {
			event.Metadata = make(map[string]interface{})
			if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal event metadata failed: %w", err)
			}
		}

		events = append(events, &event)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate events failed: %w", rows.Err())
	}

	return events, nil
}

// ListStatusEventsForTickets batch-fetches status-change events for multiple
// tickets in a single query. Returns events keyed by ticket_id.
func (r *SupportRepositoryImpl) ListStatusEventsForTickets(ctx context.Context, tx interface{}, ticketIDs []uuid.UUID) (map[uuid.UUID][]*entity.Event, error) {
	if len(ticketIDs) == 0 {
		return map[uuid.UUID][]*entity.Event{}, nil
	}

	query := `
		SELECT id, ticket_id, event_type, actor_id, old_status, new_status, notes, metadata, created_at
		FROM support_ticket_events
		WHERE ticket_id = ANY($1::uuid[])
		AND event_type IN ('status_changed', 'ticket_waiting_user')
		ORDER BY ticket_id, created_at ASC
	`

	rows, err := toTx(tx).Query(ctx, query, ticketIDs)
	if err != nil {
		return nil, fmt.Errorf("list status events for tickets failed: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]*entity.Event)
	for rows.Next() {
		var event entity.Event
		var metadataJSON []byte

		err := rows.Scan(
			&event.ID, &event.TicketID, &event.EventType, &event.ActorID,
			&event.OldStatus, &event.NewStatus, &event.Notes,
			&metadataJSON, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan status event failed: %w", err)
		}

		if metadataJSON != nil {
			event.Metadata = make(map[string]interface{})
			if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal event metadata failed: %w", err)
			}
		}

		result[event.TicketID] = append(result[event.TicketID], &event)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate status events failed: %w", rows.Err())
	}

	return result, nil
}

// ListFirstAdminResponsesByTicketIDs batch-fetches the first valid admin
// response timestamp for multiple tickets in a single query.
//
// CANONICAL AUTHORITY (SLA-F04): the earliest non-deleted chat_messages row
// in the ticket's own support conversation (one ticket = one support room,
// support_tickets.chat_room_id) whose sender has role = 'admin'. System
// messages carry sender_id NULL and cannot match the join. No heuristic —
// assigned_at/claimed events are deliberately NOT treated as a response.
//
// Single query via support_tickets ⋈ chat_rooms ⋈ chat_messages; index
// idx_chat_messages_room_id (room_id, created_at) serves MIN(created_at)
// per room. Tickets without any admin response are absent from the map.
func (r *SupportRepositoryImpl) ListFirstAdminResponsesByTicketIDs(ctx context.Context, tx interface{}, ticketIDs []uuid.UUID) (map[uuid.UUID]*time.Time, error) {
	if len(ticketIDs) == 0 {
		return map[uuid.UUID]*time.Time{}, nil
	}

	query := `
		SELECT st.id, MIN(cm.created_at) AS first_admin_response_at
		FROM support_tickets st
		JOIN chat_rooms cr ON cr.id = st.chat_room_id AND cr.room_type = 'support'
		JOIN chat_messages cm ON cm.room_id = cr.id
		JOIN users u ON u.id = cm.sender_id AND u.role = 'admin'
		WHERE st.id = ANY($1::uuid[])
		AND cm.deleted_at IS NULL
		AND cm.created_at >= st.created_at
		GROUP BY st.id
	`

	rows, err := toTx(tx).Query(ctx, query, ticketIDs)
	if err != nil {
		return nil, fmt.Errorf("list first admin responses for tickets failed: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*time.Time)
	for rows.Next() {
		var ticketID uuid.UUID
		var firstAdminResponseAt time.Time

		if err := rows.Scan(&ticketID, &firstAdminResponseAt); err != nil {
			return nil, fmt.Errorf("scan first admin response failed: %w", err)
		}

		result[ticketID] = &firstAdminResponseAt
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate first admin responses failed: %w", rows.Err())
	}

	return result, nil
}

// GetTicketStatistics returns statistics about tickets.
func (r *SupportRepositoryImpl) GetTicketStatistics(ctx context.Context, tx interface{}) (*supportRepo.TicketStatistics, error) {
	query := `
		SELECT
			COUNT(*) as total_tickets,
			COUNT(*) FILTER (WHERE status = 'open') as open_tickets,
			COUNT(*) FILTER (WHERE status = 'in_progress') as in_progress_tickets,
			COUNT(*) FILTER (WHERE status = 'waiting_user') as waiting_user_tickets,
			COUNT(*) FILTER (WHERE status = 'resolved') as resolved_tickets,
			COUNT(*) FILTER (WHERE status = 'closed') as closed_tickets,
			COUNT(*) FILTER (WHERE assigned_admin_id IS NULL AND status IN ('open', 'in_progress', 'waiting_user')) as unassigned_tickets
		FROM support_tickets
	`

	var stats supportRepo.TicketStatistics
	err := toTx(tx).QueryRow(ctx, query).Scan(
		&stats.TotalTickets, &stats.OpenTickets, &stats.InProgressTickets,
		&stats.WaitingUserTickets, &stats.ResolvedTickets, &stats.ClosedTickets, &stats.UnassignedTickets,
	)

	if err != nil {
		return nil, fmt.Errorf("get statistics failed: %w", err)
	}

	return &stats, nil
}

// FindTicketsForSLACheck finds tickets that need SLA checking.
func (r *SupportRepositoryImpl) FindTicketsForSLACheck(ctx context.Context, tx db.Tx, limit int) ([]supportRepo.TicketSLARow, error) {
	query := `
		SELECT
			id,
			user_id,
			status,
			created_at,
			assigned_at,
			resolved_at
		FROM support_tickets
		WHERE status IN ('open', 'in_progress', 'waiting_user')
		ORDER BY created_at ASC
		LIMIT $1
	`

	dbTx := toTx(tx)
	rows, err := dbTx.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []supportRepo.TicketSLARow
	for rows.Next() {
		var row supportRepo.TicketSLARow
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.Status, &row.CreatedAt,
			&row.AssignedAt, &row.ResolvedAt,
		); err != nil {
			return nil, err
		}
		tickets = append(tickets, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tickets, nil
}

// FindDisputesForSLACheck finds disputes that need SLA checking.
func (r *SupportRepositoryImpl) FindDisputesForSLACheck(ctx context.Context, tx db.Tx, limit int) ([]supportRepo.DisputeSLARow, error) {
	query := `
		SELECT
			id,
			order_id,
			buyer_id,
			seller_id,
			status,
			opened_at,
			resolved_at
		FROM disputes
		WHERE status = 'under_review'
		ORDER BY opened_at ASC
		LIMIT $1
	`

	dbTx := toTx(tx)
	rows, err := dbTx.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var disputes []supportRepo.DisputeSLARow
	for rows.Next() {
		var row supportRepo.DisputeSLARow
		if err := rows.Scan(
			&row.ID, &row.OrderID, &row.BuyerID, &row.SellerID,
			&row.Status, &row.OpenedAt, &row.ResolvedAt,
		); err != nil {
			return nil, err
		}
		disputes = append(disputes, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return disputes, nil
}


