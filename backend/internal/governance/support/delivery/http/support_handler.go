package http

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	supportApp "github.com/labuda/backend/internal/governance/support/application"
	supportEntity "github.com/labuda/backend/internal/governance/support/entity"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/capability"
	"github.com/labuda/backend/internal/platform/response"
	"go.uber.org/zap"
)

// ChatMessageService is the bounded Support↔Chat message seam.
//
// SUPPORT BOUNDARY: support rooms have no agent chat participant, so agent
// access cannot go through Chat's participant check. Authorization is owned by
// the Support domain (ticket ownership for users, capability for agents) and is
// enforced by this handler BEFORE either method is called. Both methods persist
// and read ordinary chat_messages, reusing the canonical Chat transport.
//
// SendSupportMessage always persists the real authenticated actor as sender —
// human agent replies are never represented as system messages.
type ChatMessageService interface {
	ListSupportMessages(
		ctx context.Context,
		roomID uuid.UUID,
		cursorCreatedAt *time.Time,
		cursorID *uuid.UUID,
		limit int,
	) ([]*chatEntity.ChatMessage, error)

	SendSupportMessage(
		ctx context.Context,
		roomID, senderID uuid.UUID,
		body string,
		idempotencyKey string,
	) (*chatEntity.ChatMessage, error)
}

// Handler handles HTTP requests for support operations.
// W14-B3: Added audit logging for all admin support actions.
type Handler struct {
	supportService     *supportApp.Service
	chatMessageService ChatMessageService
	db                 interface{}
	log                *zap.Logger
	// W14-B3: Audit logger for tracking admin support actions
	adminAuditLogger AdminAuditLogger
}

// AdminAuditLogger defines interface for logging admin actions.
// W14-B3: Extracted from audit package to avoid circular dependency.
type AdminAuditLogger interface {
	LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{})
}

// NewHandler creates a new support handler.
// W14-B3: Added adminAuditLogger parameter for audit logging.
func NewHandler(
	supportService *supportApp.Service,
	chatMessageService ChatMessageService,
	db interface{},
	log *zap.Logger,
	adminAuditLogger AdminAuditLogger,
) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{
		supportService:     supportService,
		chatMessageService: chatMessageService,
		db:                 db,
		log:                log,
		adminAuditLogger:   adminAuditLogger,
	}
}

// ========================================================================
// REQUEST DTOs
// ========================================================================

// CreateTicketRequest holds the request body for creating a ticket.
type CreateTicketRequest struct {
	// Canonical category vocabulary — identical to the persisted DB enum, the
	// admin client and the mobile client. No translation layer exists.
	Category      string  `json:"category" binding:"required,oneof=order_issue payment_issue account_issue listing_issue shipping_issue refund_request dispute technical_issue other"`
	Priority      string  `json:"priority" binding:"omitempty,oneof=low medium high urgent"`
	LinkedOrderID *string `json:"linked_order_id"`
	Subject       *string `json:"subject"`
	Description   *string `json:"description"`
}

// UpdatePriorityRequest holds the request body for updating priority.
type UpdatePriorityRequest struct {
	Priority string `json:"priority" binding:"required,oneof=low medium high urgent"`
}

// UpdateCategoryRequest holds the request body for updating category.
type UpdateCategoryRequest struct {
	Category string `json:"category" binding:"required,oneof=order_issue payment_issue account_issue listing_issue shipping_issue refund_request dispute technical_issue other"`
}

// ResolveTicketRequest holds the request body for resolving a ticket.
type ResolveTicketRequest struct {
	Notes *string `json:"notes"`
}

// CloseTicketRequest holds the request body for closing a ticket.
type CloseTicketRequest struct {
	Reason *string `json:"reason"`
}

// SendMessageRequest holds the request body for sending messages to a ticket.
// Admin/agent endpoint with type parameter for different agent message kinds.
type SendMessageRequest struct {
	// Type: "greeting", "system", "agent"
	Type    string `json:"type" binding:"required,oneof=greeting system agent"`
	Message string `json:"message" binding:"required"`
}

// SendUserMessageRequest holds the request body for a user replying to their
// own support ticket.
//
// The sender is ALWAYS the authenticated ticket owner, so the body carries only
// the message text: there is no sender identity field and no agent message
// "type" selector. A user reply can never be coerced into a system message or
// attributed to someone else.
type SendUserMessageRequest struct {
	Message string `json:"message" binding:"required"`
}

// EscalateToDisputeRequest holds the request body for escalating a ticket to dispute.
type EscalateToDisputeRequest struct {
	Reason      string  `json:"reason" binding:"required"`
	Description *string `json:"description"`
	ReasonCode  string  `json:"reason_code" binding:"required"`
}

// ========================================================================
// TICKET ENDPOINTS (User)
// ========================================================================

// CreateTicket handles POST /api/v1/support/tickets
//
// Creates a new support ticket.
//
// Business rules:
// - User can only have ONE open ticket at a time
// - A chat room is created for the ticket
func (h *Handler) CreateTicket(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Parse priority (default to medium)
	priority := supportEntity.PriorityMedium
	if req.Priority != "" {
		priority = supportEntity.Priority(req.Priority)
	}

	// Parse linked order ID
	var linkedOrderID *uuid.UUID
	if req.LinkedOrderID != nil {
		orderID, err := uuid.Parse(*req.LinkedOrderID)
		if err != nil {
			response.BadRequest(c, "Invalid linked order ID")
			return
		}
		linkedOrderID = &orderID
	}

	// Create ticket request
	createReq := &supportApp.CreateTicketRequest{
		UserID:        userID,
		Category:      supportEntity.Category(req.Category),
		Priority:      priority,
		LinkedOrderID: linkedOrderID,
		Subject:       req.Subject,
		Description:   req.Description,
	}

	ticket, err := h.supportService.CreateTicket(ctx, createReq)
	if err != nil {
		h.log.Error("Failed to create ticket",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to create ticket")
		return
	}

	response.Created(c, ticketToResponse(ticket, nil, nil))
}

// ListMyTickets handles GET /api/v1/support/tickets
//
// Lists tickets for the authenticated user.
func (h *Handler) ListMyTickets(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse cursor
	var cursorCreatedAt *time.Time
	var cursorID *uuid.UUID

	if cursorStr := c.Query("cursor_created_at"); cursorStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, cursorStr); err == nil {
			cursorCreatedAt = &t
		}
	}

	if cursorStr := c.Query("cursor_id"); cursorStr != "" {
		if id, err := uuid.Parse(cursorStr); err == nil {
			cursorID = &id
		}
	}

	// Parse limit
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	tickets, err := h.supportService.ListMyTickets(ctx, userID, cursorCreatedAt, cursorID, limit)
	if err != nil {
		h.log.Error("Failed to list tickets",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve tickets")
		return
	}

	// Batch-fetch status events for SLA computation (single query, no N+1)
	ticketIDs := make([]uuid.UUID, len(tickets))
	for i, t := range tickets {
		ticketIDs[i] = t.ID
	}
	eventsByTicket, err := h.supportService.ListStatusEventsForTickets(ctx, ticketIDs)
	if err != nil {
		// Fallback: use empty events — SLA will show wall-clock (degraded but non-fatal)
		eventsByTicket = make(map[uuid.UUID][]*supportEntity.Event)
	}

	// Batch-fetch first admin response timestamps for SLA-F04 (single query,
	// no N+1). Missing entries mean "no admin response yet".
	firstAdminResponses, err := h.supportService.ListFirstAdminResponsesByTicketIDs(ctx, ticketIDs)
	if err != nil {
		firstAdminResponses = make(map[uuid.UUID]*time.Time)
	}

	// Convert to response
	data := make([]map[string]interface{}, len(tickets))
	for i, ticket := range tickets {
		data[i] = ticketToResponse(ticket, eventsByTicket[ticket.ID], firstAdminResponses[ticket.ID])
	}

	response.Success(c, gin.H{
		"data": data,
	})
}

// GetTicket handles GET /api/v1/support/tickets/:id
//
// Gets a specific ticket by ID.
// SECURITY: Only the ticket owner may access via this user-facing endpoint.
// Admin access uses AdminGetTicket which has its own capability checks.
func (h *Handler) GetTicket(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	ticket, err := h.supportService.GetTicket(ctx, ticketID)
	if err != nil {
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to retrieve ticket")
		return
	}

	// Ownership check: user can only view their own tickets
	if ticket.UserID != userID {
		response.NotFound(c, "Ticket not found")
		return
	}

	// Fetch status events for canonical SLA computation
	events, err := h.supportService.ListEvents(ctx, ticketID, 100)
	if err != nil {
		events = nil // fallback to empty
	}

	// Canonical first-response authority (SLA-F04): batch fetch the first
	// valid admin response message timestamp (single query, no N+1).
	firstAdminResponses, err := h.supportService.ListFirstAdminResponsesByTicketIDs(ctx, []uuid.UUID{ticketID})
	if err != nil {
		firstAdminResponses = make(map[uuid.UUID]*time.Time)
	}

	response.Success(c, ticketToResponse(ticket, events, firstAdminResponses[ticketID]))
}

// ListEvents handles GET /api/v1/support/tickets/:id/events
//
// Lists all events for a ticket.
// SECURITY: Only the ticket owner may access via this user-facing endpoint.
func (h *Handler) ListEvents(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	// Ownership check: verify ticket belongs to caller
	ticket, err := h.supportService.GetTicket(ctx, ticketID)
	if err != nil {
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to retrieve ticket")
		return
	}
	if ticket.UserID != userID {
		response.NotFound(c, "Ticket not found")
		return
	}

	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	events, err := h.supportService.ListEvents(ctx, ticketID, limit)
	if err != nil {
		response.InternalServerError(c, "Failed to retrieve events")
		return
	}

	// Convert to response
	data := make([]map[string]interface{}, len(events))
	for i, event := range events {
		data[i] = eventToResponse(event)
	}

	response.Success(c, gin.H{
		"data": data,
	})
}

// ReopenTicket handles PUT /api/v1/support/tickets/:id/reopen
//
// Reopens a resolved or closed ticket.
func (h *Handler) ReopenTicket(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	req := &supportApp.ReopenTicketRequest{
		TicketID: ticketID,
		ActorID:  userID,
	}

	ticket, err := h.supportService.ReopenTicket(ctx, req)
	if err != nil {
		if err == supportRepo.ErrCannotReopenTicket {
			response.BadRequest(c, "Ticket cannot be reopened")
			return
		}
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to reopen ticket")
		return
	}

	response.Success(c, ticketToResponse(ticket, nil, nil))
}

// AdminReopenTicket handles PUT /api/v1/admin/support/tickets/:id/reopen
//
// Reopens a resolved ticket (resolved -> open) on behalf of the authenticated
// agent. The reopen capability is enforced by middleware; the agent is recorded
// as the event actor. A closed ticket is terminal and is rejected.
func (h *Handler) AdminReopenTicket(c *gin.Context) {
	ctx := c.Request.Context()

	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapSupportTicketResolve.String()) {
		response.Forbidden(c, "Insufficient permissions: support.ticket.resolve required")
		return
	}

	adminIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		response.Unauthorized(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	req := &supportApp.ReopenTicketRequest{
		TicketID: ticketID,
		ActorID:  adminID,
		IsAdmin:  true,
	}

	ticket, err := h.supportService.ReopenTicket(ctx, req)
	if err != nil {
		if err == supportRepo.ErrCannotReopenTicket {
			response.BadRequest(c, "Ticket cannot be reopened")
			return
		}
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to reopen ticket")
		return
	}

	// W14-B3: Log the agent-initiated reopen to the audit trail.
	h.adminAuditLogger.LogSafe(ctx, adminID,
		"support_ticket_reopened", "support_ticket", ticketID,
		map[string]interface{}{
			"new_status": string(ticket.Status),
		},
	)

	response.Success(c, ticketToResponse(ticket, nil, nil))
}

// ========================================================================
// TICKET ENDPOINTS (Admin)
// ========================================================================

// ListAllTickets handles GET /api/v1/support/admin/tickets
//
// Lists all tickets (admin only).
func (h *Handler) ListAllTickets(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse filters
	filter := &supportRepo.TicketFilter{}

	if statusStr := c.Query("status"); statusStr != "" {
		status := supportEntity.Status(statusStr)
		if status.IsValid() {
			filter.Status = &status
		}
	}

	if categoryStr := c.Query("category"); categoryStr != "" {
		category := supportEntity.Category(categoryStr)
		if category.IsValid() {
			filter.Category = &category
		}
	}

	if priorityStr := c.Query("priority"); priorityStr != "" {
		priority := supportEntity.Priority(priorityStr)
		if priority.IsValid() {
			filter.Priority = &priority
		}
	}

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		userID, err := uuid.Parse(userIDStr)
		if err == nil {
			filter.UserID = &userID
		}
	}

	if adminIDStr := c.Query("assigned_admin_id"); adminIDStr != "" {
		adminID, err := uuid.Parse(adminIDStr)
		if err == nil {
			filter.AssignedAdminID = &adminID
		}
	}

	if isOverdueStr := c.Query("is_overdue"); isOverdueStr != "" {
		isOverdue := isOverdueStr == "true"
		filter.IsOverdue = &isOverdue
	}

	if isUnassignedStr := c.Query("is_unassigned"); isUnassignedStr != "" {
		isUnassigned := isUnassignedStr == "true"
		filter.IsUnassigned = &isUnassigned
	}

	// Parse cursor
	var cursorCreatedAt *time.Time
	var cursorID *uuid.UUID

	if cursorStr := c.Query("cursor_created_at"); cursorStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, cursorStr); err == nil {
			cursorCreatedAt = &t
		}
	}

	if cursorStr := c.Query("cursor_id"); cursorStr != "" {
		if id, err := uuid.Parse(cursorStr); err == nil {
			cursorID = &id
		}
	}

	// Parse limit
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	tickets, err := h.supportService.ListTickets(ctx, filter, cursorCreatedAt, cursorID, limit)
	if err != nil {
		response.InternalServerError(c, "Failed to retrieve tickets")
		return
	}

	// Batch-fetch status events for SLA computation (single query, no N+1)
	ticketIDs := make([]uuid.UUID, len(tickets))
	for i, t := range tickets {
		ticketIDs[i] = t.ID
	}
	eventsByTicket, err := h.supportService.ListStatusEventsForTickets(ctx, ticketIDs)
	if err != nil {
		// Fallback: use empty events — SLA will show wall-clock (degraded but non-fatal)
		eventsByTicket = make(map[uuid.UUID][]*supportEntity.Event)
	}

	// Batch-fetch first admin response timestamps for SLA-F04 (single query,
	// no N+1). Missing entries mean "no admin response yet".
	firstAdminResponses, err := h.supportService.ListFirstAdminResponsesByTicketIDs(ctx, ticketIDs)
	if err != nil {
		firstAdminResponses = make(map[uuid.UUID]*time.Time)
	}

	// Convert to response
	data := make([]map[string]interface{}, len(tickets))
	for i, ticket := range tickets {
		data[i] = ticketToResponse(ticket, eventsByTicket[ticket.ID], firstAdminResponses[ticket.ID])
	}

	response.Success(c, gin.H{
		"data": data,
	})
}

// ClaimTicket handles PUT /api/v1/support/tickets/:id/claim
//
// Claims an open ticket (admin only).
//
// M2: MIGRATED to capability-based auth with support.ticket.claim
// DUAL PROTECTION: RequireAdminMiddleware (route level) + handler-level capability check
func (h *Handler) ClaimTicket(c *gin.Context) {
	ctx := c.Request.Context()

	// ============================================================
	// HANDLER-LEVEL DEFENSE: Capability check (defense-in-depth)
	// ============================================================
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapSupportTicketClaim.String()) {
		response.Forbidden(c, "Insufficient permissions: support.ticket.claim required")
		return
	}

	adminIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	req := &supportApp.ClaimTicketRequest{
		TicketID: ticketID,
		AdminID:  adminID,
	}

	ticket, err := h.supportService.ClaimTicket(ctx, req)
	if err != nil {
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		if err == supportRepo.ErrTicketAlreadyClaimed {
			response.BadRequest(c, "Ticket is already claimed")
			return
		}
		response.InternalServerError(c, "Failed to claim ticket")
		return
	}

	// W14-B3: Log ticket claim to audit trail
	h.adminAuditLogger.LogSafe(ctx, adminID,
		"support_ticket_claimed", "support_ticket", ticketID,
		map[string]interface{}{
			"previous_assigned_admin": nil, // Was unassigned
			"new_assigned_admin":      adminID.String(),
		},
	)

	response.Success(c, ticketToResponse(ticket, nil, nil))
}

// ResolveTicket handles PUT /api/v1/support/tickets/:id/resolve
//
// Resolves a ticket (admin only).
//
// SLICE 8: MIGRATED to capability-based auth with support.ticket.resolve
// DUAL PROTECTION: RequireAdminMiddleware (route level) + handler-level capability check
func (h *Handler) ResolveTicket(c *gin.Context) {
	ctx := c.Request.Context()

	// ============================================================
	// HANDLER-LEVEL DEFENSE: Capability check (defense-in-depth)
	// ============================================================
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapSupportTicketResolve.String()) {
		response.Forbidden(c, "Insufficient permissions: support.ticket.resolve required")
		return
	}

	adminIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req ResolveTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resolveReq := &supportApp.ResolveTicketRequest{
		TicketID: ticketID,
		AdminID:  adminID,
		Notes:    req.Notes,
	}

	err = h.supportService.ResolveTicket(ctx, resolveReq)
	if err != nil {
		if err == supportRepo.ErrInvalidStatusTransition {
			response.BadRequest(c, "Ticket cannot be resolved in current state")
			return
		}
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to resolve ticket")
		return
	}

	// W14-B3: Log ticket resolution to audit trail
	h.adminAuditLogger.LogSafe(ctx, adminID,
		"support_ticket_resolved", "support_ticket", ticketID,
		map[string]interface{}{
			"resolution_notes": req.Notes,
		},
	)

	response.SuccessWithMessage(c, "Ticket resolved successfully", gin.H{
		"ticket_id": ticketID,
	})
}

// CloseTicket handles PUT /api/v1/support/tickets/:id/close
//
// Closes a resolved ticket (admin only).
//
// SLICE 8: MIGRATED to capability-based auth with support.ticket.resolve
// DUAL PROTECTION: RequireAdminMiddleware (route level) + handler-level capability check
// Final closure is a resolution action.
func (h *Handler) CloseTicket(c *gin.Context) {
	ctx := c.Request.Context()

	// ============================================================
	// HANDLER-LEVEL DEFENSE: Capability check (defense-in-depth)
	// ============================================================
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapSupportTicketResolve.String()) {
		response.Forbidden(c, "Insufficient permissions: support.ticket.resolve required")
		return
	}

	adminIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req CloseTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	closeReq := &supportApp.CloseTicketRequest{
		TicketID:    ticketID,
		AdminID:     adminID,
		CloseReason: req.Reason,
	}

	err = h.supportService.CloseTicket(ctx, closeReq)
	if err != nil {
		if err == supportRepo.ErrInvalidStatusTransition {
			response.BadRequest(c, "Only resolved tickets can be closed")
			return
		}
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to close ticket")
		return
	}

	// W14-B3: Log ticket closure to audit trail
	h.adminAuditLogger.LogSafe(ctx, adminID,
		"support_ticket_closed", "support_ticket", ticketID,
		map[string]interface{}{
			"close_reason": req.Reason,
		},
	)

	response.SuccessWithMessage(c, "Ticket closed successfully", gin.H{
		"ticket_id": ticketID,
	})
}

// UpdateTicketPriority handles PUT /api/v1/support/tickets/:id/priority
//
// Updates ticket priority (admin only).
func (h *Handler) UpdateTicketPriority(c *gin.Context) {
	ctx := c.Request.Context()

	actorIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	actorID, ok := actorIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req UpdatePriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	err = h.supportService.UpdatePriority(ctx, ticketID, supportEntity.Priority(req.Priority), &actorID)
	if err != nil {
		if err == supportRepo.ErrInvalidPriority {
			response.BadRequest(c, "Invalid priority")
			return
		}
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to update priority")
		return
	}

	// W14-B3: Log priority update to audit trail
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"support_ticket_priority_updated", "support_ticket", ticketID,
		map[string]interface{}{
			"new_priority": req.Priority,
		},
	)

	response.SuccessWithMessage(c, "Priority updated successfully", gin.H{
		"ticket_id": ticketID,
		"priority":  req.Priority,
	})
}

// UpdateTicketCategory handles PUT /api/v1/support/tickets/:id/category
//
// Updates ticket category (admin only).
func (h *Handler) UpdateTicketCategory(c *gin.Context) {
	ctx := c.Request.Context()

	actorIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	actorID, ok := actorIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	err = h.supportService.UpdateCategory(ctx, ticketID, supportEntity.Category(req.Category), &actorID)
	if err != nil {
		if err == supportRepo.ErrInvalidCategory {
			response.BadRequest(c, "Invalid category")
			return
		}
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to update category")
		return
	}

	// W14-B3: Log category update to audit trail
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"support_ticket_category_updated", "support_ticket", ticketID,
		map[string]interface{}{
			"new_category": req.Category,
		},
	)

	response.SuccessWithMessage(c, "Category updated successfully", gin.H{
		"ticket_id": ticketID,
		"category":  req.Category,
	})
}

// SetWaitingForUser handles PUT /api/v1/support/tickets/:id/waiting
//
// Sets ticket status to waiting_user (admin only).
func (h *Handler) SetWaitingForUser(c *gin.Context) {
	ctx := c.Request.Context()

	adminIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	err = h.supportService.SetWaitingForUser(ctx, ticketID, adminID)
	if err != nil {
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to update ticket")
		return
	}

	response.SuccessWithMessage(c, "Ticket status updated", gin.H{
		"ticket_id": ticketID,
		"status":    "waiting_user",
	})
}

// AdminGetTicket handles GET /api/v1/admin/support/tickets/:id
//
// Gets any ticket by ID (admin only).
// Includes order and dispute information for linked orders.
func (h *Handler) AdminGetTicket(c *gin.Context) {
	ctx := c.Request.Context()

	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapSupportTicketRead.String()) {
		response.Forbidden(c, "Insufficient permissions: support.ticket.read required")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	// Get enriched ticket with order and dispute info
	enrichedTicket, err := h.supportService.GetTicketEnriched(ctx, ticketID)
	if err != nil {
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to retrieve ticket")
		return
	}

	// Canonical SLA: fetch status-change events for this ticket and compute
	// with waiting_user pause semantics (same as list/dashboard).
	eventsResp, err := h.supportService.ListStatusEventsForTickets(ctx, []uuid.UUID{ticketID})
	if err != nil {
		eventsResp = make(map[uuid.UUID][]*supportEntity.Event)
	}

	// Canonical first-response authority (SLA-F04): batch fetch the first
	// valid admin response message timestamp (single query, no N+1).
	firstAdminResponses, err := h.supportService.ListFirstAdminResponsesByTicketIDs(ctx, []uuid.UUID{ticketID})
	if err != nil {
		firstAdminResponses = make(map[uuid.UUID]*time.Time)
	}

	resp := enrichedTicketToResponse(enrichedTicket, eventsResp[ticketID], firstAdminResponses[ticketID])
	response.Success(c, resp)
}

// AdminListMessages handles GET /api/v1/admin/support/tickets/:id/messages
//
// Lists all messages in a ticket's chat room (admin only).
// Reuses existing service logic from ListMessages.
func (h *Handler) AdminListMessages(c *gin.Context) {
	ctx := c.Request.Context()

	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapSupportTicketRead.String()) {
		response.Forbidden(c, "Insufficient permissions: support.ticket.read required")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
			limit = l
		}
	}

	ticket, err := h.supportService.GetTicket(ctx, ticketID)
	if err != nil {
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to retrieve ticket")
		return
	}

	if h.chatMessageService != nil {
		// Support-domain read seam: the actor was capability-checked above, so
		// no chat participant check is applied (agents are not chat
		// participants of support rooms).
		messages, err := h.chatMessageService.ListSupportMessages(
			ctx,
			ticket.ChatRoomID,
			nil,
			nil,
			limit,
		)
		if err != nil {
			h.log.Error("Failed to list messages",
				zap.String("ticket_id", ticketID.String()),
				zap.String("chat_room_id", ticket.ChatRoomID.String()),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to retrieve messages")
			return
		}

		data := make([]map[string]interface{}, len(messages))
		for i, msg := range messages {
			data[i] = supportMessageToResponse(msg, ticket.UserID)
		}

		response.Success(c, gin.H{
			"data": data,
		})
		return
	}

	response.Success(c, gin.H{
		"data":         []interface{}{},
		"chat_room_id": ticket.ChatRoomID.String(),
		"message":      "Chat message service not available",
	})
}

// ========================================================================
// ADMIN ENDPOINTS
// ========================================================================

// GetStatistics handles GET /api/v1/support/statistics
//
// Gets support ticket statistics (admin only).
func (h *Handler) GetStatistics(c *gin.Context) {
	ctx := c.Request.Context()

	stats, err := h.supportService.GetStatistics(ctx)
	if err != nil {
		response.InternalServerError(c, "Failed to retrieve statistics")
		return
	}

	response.Success(c, statsToResponse(stats))
}

// ========================================================================
// MESSAGE ENDPOINTS
// ========================================================================

// SendMessage handles POST /api/v1/support/tickets/:id/messages
//
// Unified endpoint for sending messages to a support ticket (admin only).
// Type parameter specifies message kind: "greeting", "system", or "agent"
//
// Body: {"type": "greeting|system|agent", "message": "..."}
//
// M2: MIGRATED to capability-based auth with support.ticket.respond
// DUAL PROTECTION: RequireAdminMiddleware (route level) + handler-level capability check
func (h *Handler) SendMessage(c *gin.Context) {
	ctx := c.Request.Context()

	// ============================================================
	// HANDLER-LEVEL DEFENSE: Capability check (defense-in-depth)
	// ============================================================
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapSupportTicketRespond.String()) {
		response.Forbidden(c, "Insufficient permissions: support.ticket.respond required")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Get ticket to find chat room
	ticket, err := h.supportService.GetTicket(ctx, ticketID)
	if err != nil {
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to retrieve ticket")
		return
	}

	adminIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		response.Unauthorized(c, "Invalid user ID in context")
		return
	}

	if h.chatMessageService == nil {
		response.InternalServerError(c, "Message service unavailable")
		return
	}

	// Admin reply is a REAL admin-authored message: the authenticated agent ID
	// is persisted as sender. It is never represented as a system message.
	if _, err := h.chatMessageService.SendSupportMessage(
		ctx,
		ticket.ChatRoomID,
		adminID,
		req.Message,
		uuid.New().String(),
	); err != nil {
		h.log.Error("Failed to send support message",
			zap.String("ticket_id", ticketID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to send message")
		return
	}

	// Canonical lifecycle: an agent reply means the ticket is waiting for the
	// user. Best-effort — the message is already persisted.
	if ticket.AssignedAdminID != nil && *ticket.AssignedAdminID == adminID && ticket.Status == supportEntity.StatusInProgress {
		if err := h.supportService.SetWaitingForUser(ctx, ticketID, adminID); err != nil {
			h.log.Error("Failed to transition ticket to waiting_user after agent reply",
				zap.String("ticket_id", ticketID.String()),
				zap.Error(err),
			)
		}
	}

	// W14-B3: Log support message sent to audit trail
	h.adminAuditLogger.LogSafe(ctx, adminID,
		"support_message_sent", "support_ticket", ticketID,
		map[string]interface{}{
			"chat_room_id": ticket.ChatRoomID.String(),
		},
	)

	response.SuccessWithMessage(c, "Message sent", gin.H{
		"ticket_id":    ticketID,
		"chat_room_id": ticket.ChatRoomID,
	})
}

// EscalateToDispute handles POST /api/v1/admin/support/tickets/:id/escalate-to-dispute
//
// Escalates a support ticket to a formal dispute (admin only).
//
// Business rules:
// - Ticket must have a linked_order_id
// - Ticket must not already be escalated to dispute
// - Order must not already have an active dispute
func (h *Handler) EscalateToDispute(c *gin.Context) {
	ctx := c.Request.Context()

	// ============================================================
	// HANDLER-LEVEL DEFENSE: Capability check (defense-in-depth)
	// ============================================================
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapSupportTicketEscalate.String()) {
		response.Forbidden(c, "Insufficient permissions: support.ticket.escalate required")
		return
	}

	adminIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req EscalateToDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	escalateReq := &supportApp.EscalateToDisputeRequest{
		TicketID:    ticketID,
		AdminID:     adminID,
		Reason:      req.Reason,
		Description: req.Description,
		ReasonCode:  req.ReasonCode,
	}

	ticket, err := h.supportService.EscalateToDispute(ctx, escalateReq)
	if err != nil {
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		// Check for specific error messages
		if err.Error() == "cannot escalate: ticket has no linked order" {
			response.BadRequest(c, "Cannot escalate: ticket has no linked order")
			return
		}
		if err.Error() == "cannot escalate: ticket already escalated to dispute" {
			response.BadRequest(c, "Cannot escalate: ticket already escalated to dispute")
			return
		}
		if err.Error() == "cannot escalate: order already has an active dispute" {
			response.BadRequest(c, "Cannot escalate: order already has an active dispute")
			return
		}
		h.log.Error("Failed to escalate ticket to dispute",
			zap.String("ticket_id", ticketID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to escalate ticket to dispute")
		return
	}

	// W14-B3: Log escalation to audit trail
	h.adminAuditLogger.LogSafe(ctx, adminID,
		"support_ticket_escalated_to_dispute", "support_ticket", ticketID,
		map[string]interface{}{
			"reason":      req.Reason,
			"reason_code": req.ReasonCode,
		},
	)

	response.SuccessWithMessage(c, "Ticket escalated to dispute successfully", ticketToResponse(ticket, nil, nil))
}

// ListMessages handles GET /api/v1/support/tickets/:id/messages
//
// Lists all messages in a support ticket's chat room.
// Uses the ticket's chat_room_id to retrieve messages from the chat system.
//
// Query parameters:
//   - limit: number of messages to return (default: 100, max: 500)
//
// ListMessages handles GET /api/v1/support/tickets/:id/messages
//
// Returns the conversation for the authenticated ticket owner.
// SECURITY: a user may only read their own ticket's conversation. Ownership is
// verified before the Support read seam is used.
func (h *Handler) ListMessages(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
			limit = l
		}
	}

	ticket, err := h.supportService.GetTicket(ctx, ticketID)
	if err != nil {
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to retrieve ticket")
		return
	}

	// Ownership authority: never leak another user's conversation.
	if ticket.UserID != userID {
		response.NotFound(c, "Ticket not found")
		return
	}

	if h.chatMessageService == nil {
		response.InternalServerError(c, "Message service unavailable")
		return
	}

	messages, err := h.chatMessageService.ListSupportMessages(
		ctx,
		ticket.ChatRoomID,
		nil,
		nil,
		limit,
	)
	if err != nil {
		h.log.Error("Failed to list messages",
			zap.String("ticket_id", ticketID.String()),
			zap.String("chat_room_id", ticket.ChatRoomID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve messages")
		return
	}

	data := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		data[i] = supportMessageToResponse(msg, ticket.UserID)
	}

	response.Success(c, gin.H{
		"data": data,
	})
}

// SendMyMessage handles POST /api/v1/support/tickets/:id/messages (user).
//
// The authenticated ticket owner posts a reply into their own support
// conversation. Messages are ordinary chat_messages — the Support ticket owns
// identity/lifecycle, Chat owns transport.
// SECURITY: ownership is verified before writing.
func (h *Handler) SendMyMessage(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req SendUserMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ticket, err := h.supportService.GetTicket(ctx, ticketID)
	if err != nil {
		if err == supportRepo.ErrTicketNotFound {
			response.NotFound(c, "Ticket not found")
			return
		}
		response.InternalServerError(c, "Failed to retrieve ticket")
		return
	}

	if ticket.UserID != userID {
		response.NotFound(c, "Ticket not found")
		return
	}

	if h.chatMessageService == nil {
		response.InternalServerError(c, "Message service unavailable")
		return
	}

	if _, err := h.chatMessageService.SendSupportMessage(
		ctx,
		ticket.ChatRoomID,
		userID,
		req.Message,
		uuid.New().String(),
	); err != nil {
		h.log.Error("Failed to send user support message",
			zap.String("ticket_id", ticketID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to send message")
		return
	}

	response.SuccessWithMessage(c, "Message sent", gin.H{
		"ticket_id": ticketID,
	})
}

// ========================================================================
// RESPONSE MAPPERS
// ========================================================================

func enrichedTicketToResponse(enriched *supportApp.TicketEnriched, statusEvents []*supportEntity.Event, firstAdminResponseAt *time.Time) map[string]interface{} {
	resp := ticketToResponse(enriched.Ticket, statusEvents, firstAdminResponseAt)

	if enriched.OrderInfo != nil {
		resp["order_info"] = map[string]interface{}{
			"order_id":      enriched.OrderInfo.OrderID,
			"status":        enriched.OrderInfo.Status,
			"escrow_status": enriched.OrderInfo.EscrowStatus,
			"has_dispute":   enriched.OrderInfo.HasDispute,
		}
	}

	if enriched.DisputeInfo != nil {
		resp["dispute_info"] = map[string]interface{}{
			"dispute_id":  enriched.DisputeInfo.DisputeID,
			"status":      enriched.DisputeInfo.Status,
			"opened_at":   enriched.DisputeInfo.OpenedAt.UTC().Format(time.RFC3339),
			"resolved_at": formatTimePtr(enriched.DisputeInfo.ResolvedAt),
			"resolved_by": enriched.DisputeInfo.ResolvedBy,
		}
	}

	return resp
}

func ticketToResponse(ticket *supportEntity.Ticket, statusEvents []*supportEntity.Event, firstAdminResponseAt *time.Time) map[string]interface{} {
	// Canonical SLA: compute from status-change events (excludes waiting_user
	// time) and the first valid admin response message timestamp (SLA-F04).
	slaMetrics := ticket.ComputeSLAMetricsFromEvents(statusEvents, firstAdminResponseAt)

	return map[string]interface{}{
		"id":                ticket.ID,
		"user_id":           ticket.UserID,
		"username":          ticket.Username,
		"seller_farm_name":  ticket.SellerFarmName,
		"chat_room_id":      ticket.ChatRoomID,
		"subject":           ticket.Subject,
		"description":       ticket.Description,
		"category":          ticket.Category.String(),
		"priority":          ticket.Priority.String(),
		"status":            ticket.Status.String(),
		"escalation":        ticket.Escalation.String(),
		"linked_order_id":   ticket.LinkedOrderID,
		"assigned_admin_id": ticket.AssignedAdminID,
		"created_at":        ticket.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":        ticket.UpdatedAt.UTC().Format(time.RFC3339),
		"assigned_at":       formatTimePtr(ticket.AssignedAt),
		"resolved_at":       formatTimePtr(ticket.ResolvedAt),
		"closed_at":         formatTimePtr(ticket.ClosedAt),
		"resolution_notes":  ticket.ResolutionNotes,
		"close_reason":      ticket.CloseReason,
		"metadata":          ticket.Metadata,
		// SLA metrics (canonical — excludes waiting_user time)
		"sla": map[string]interface{}{
			"first_response_time_seconds": formatDurationSeconds(slaMetrics.FirstResponseTime),
			"first_response_overdue":      slaMetrics.FirstResponseOverdue,
			"resolution_time_seconds":     formatDurationSeconds(slaMetrics.ResolutionTime),
			"resolution_overdue":          slaMetrics.ResolutionOverdue,
			"is_overdue":                  slaMetrics.IsOverdue,
			"next_action":                 slaMetrics.NextAction,
			"waiting_time_seconds":        int64(slaMetrics.WaitingTime.Seconds()),
			"active_time_seconds":         int64(slaMetrics.ActiveTime.Seconds()),
		},
	}
}

func eventToResponse(event *supportEntity.Event) map[string]interface{} {
	return map[string]interface{}{
		"id":         event.ID,
		"ticket_id":  event.TicketID,
		"event_type": event.EventType.String(),
		"actor_id":   event.ActorID,
		"old_status": formatStatusPtr(event.OldStatus),
		"new_status": formatStatusPtr(event.NewStatus),
		"notes":      event.Notes,
		"metadata":   event.Metadata,
		"created_at": event.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func statsToResponse(stats *supportRepo.TicketStatistics) map[string]interface{} {
	return map[string]interface{}{
		"total_tickets":        stats.TotalTickets,
		"open_tickets":         stats.OpenTickets,
		"in_progress_tickets":  stats.InProgressTickets,
		"waiting_user_tickets": stats.WaitingUserTickets,
		"resolved_tickets":     stats.ResolvedTickets,
		"closed_tickets":       stats.ClosedTickets,
		"unassigned_tickets":   stats.UnassignedTickets,
	}
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.UTC().Format(time.RFC3339)
	return &formatted
}

func formatStatusPtr(s *supportEntity.Status) *string {
	if s == nil {
		return nil
	}
	str := s.String()
	return &str
}

func formatDurationSeconds(d *time.Duration) *int64 {
	if d == nil {
		return nil
	}
	seconds := int64(d.Seconds())
	return &seconds
}

// supportMessageToResponse converts a chat message to support message response.
// Includes sender context for determining if message is from user, admin, or system.
func supportMessageToResponse(msg *chatEntity.ChatMessage, ticketOwnerID uuid.UUID) map[string]interface{} {
	resp := map[string]interface{}{
		"id":           msg.ID.String(),
		"room_id":      msg.RoomID.String(),
		"sender_id":    msg.SenderID.String(),
		"message_type": string(msg.MessageType),
		"created_at":   msg.CreatedAt.UTC().Format(time.RFC3339),
	}

	// Sender type for the support context. A support conversation has exactly
	// two real parties — the ticket owner and authorised agents — and every
	// message row has a NOT NULL sender FK, so persisted sender identity vs
	// ticket ownership is the complete classification. The client never has to
	// guess.
	if msg.SenderID == ticketOwnerID {
		resp["sender_type"] = "user"
	} else {
		resp["sender_type"] = "admin"
	}

	// Tombstone for moderated/hidden messages on support user-facing surfaces.
	// Preserve timeline structure, suppress content payload.
	if msg.DeletedAt != nil {
		resp["is_hidden"] = true
		return resp
	}

	if msg.Body != nil {
		resp["body"] = *msg.Body
	}

	if msg.AttachmentJSON != nil {
		resp["attachment_json"] = msg.AttachmentJSON
	}

	return resp
}
