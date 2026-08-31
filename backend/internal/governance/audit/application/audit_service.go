package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auditentity "github.com/labuda/backend/internal/governance/audit/entity"
	auditrepo "github.com/labuda/backend/internal/governance/audit/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// AuditService handles audit event logging for critical business events.
//
// DESIGN PRINCIPLES:
// - SAFETY FIRST: Audit failures never break business flows
// - IMMUTABLE: Audit records are never modified or deleted
// - COMPREHENSIVE: All critical actions are logged
// - ASYNC-FRIENDLY: Can work with or without transactions
type AuditService struct {
	repo   auditrepo.AuditEventRepository
	db     *db.DB
	logger *zap.Logger
}

// NewAuditService creates a new AuditService.
//
// Parameters:
//   - repo: The audit event repository for persistence
//   - database: The database for creating transactions when needed
//   - logger: Logger for error reporting (required for safety)
func NewAuditService(repo auditrepo.AuditEventRepository, database *db.DB, logger *zap.Logger) *AuditService {
	return &AuditService{
		repo:   repo,
		db:     database,
		logger: logger,
	}
}

// EmitOptions holds optional parameters for audit event emission.
type EmitOptions struct {
	// Payload contains additional event metadata
	Payload interface{}
}

// Emit safely logs an audit event.
//
// CRITICAL SAFETY: This method is designed to NEVER break business flows.
// All errors are logged but not returned to the caller.
//
// This method can be called in two ways:
// 1. Within an existing transaction: tx is provided, event is part of the transaction
// 2. Without transaction: tx is nil, event is logged in its own transaction
//
// Parameters:
//   - ctx: The context for the operation
//   - tx: Optional transaction (nil for standalone logging)
//   - eventType: The type of event (e.g., "order.created")
//   - entityType: The type of entity affected (e.g., "order")
//   - entityID: The ID of the entity affected
//   - actorType: The type of actor (e.g., "user", "system")
//   - actorID: The ID of the actor (nil for system)
//   - options: Optional parameters (payload)
//
// Example Usage:
//
//	// Within a transaction
//	err := s.db.WithTx(ctx, func(tx db.Tx) error {
//	    // ... business logic ...
//	    audit.Emit(ctx, tx, "order.created", "order", orderID, "user", userID, nil)
//	    return nil
//	})
//
//	// Without transaction (standalone)
//	audit.Emit(ctx, nil, "order.shipped", "order", orderID, "system", nil, nil)
func (s *AuditService) Emit(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	entityType string,
	entityID uuid.UUID,
	actorType string,
	actorID *uuid.UUID,
	options *EmitOptions,
) {
	event := auditentity.NewAuditEvent(
		auditentity.EventType(eventType),
		auditentity.EntityType(entityType),
		entityID,
		auditentity.ActorType(actorType),
		actorID,
		nil,
	)

	// Build payload with actor_name for immutability
	// actor_name is captured at event time and never changes, ensuring
	// timeline display remains stable even if user changes their name
	payload := make(map[string]interface{})

	// Resolve actor name for user/admin types (immutable capture)
	if actorID != nil && (actorType == string(auditentity.ActorTypeUser) || actorType == string(auditentity.ActorTypeAdmin)) {
		if resolvedName := s.resolveActorName(ctx, tx, *actorID); resolvedName != "" {
			payload["actor_name"] = resolvedName
		}
	}

	// Apply options payload
	if options != nil && options.Payload != nil {
		if p, ok := options.Payload.(map[string]interface{}); ok {
			for k, v := range p {
				payload[k] = v
			}
		} else {
			payload["data"] = options.Payload
		}
	}

	if len(payload) > 0 {
		event.PayloadJSON = payload
	}

	// Emit the event (with error handling)
	if err := s.emitWithTx(ctx, tx, event); err != nil {
		s.logError(event, err)
	}
}

// EmitUser is a convenience method for logging user-initiated events.
func (s *AuditService) EmitUser(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	entityType string,
	entityID uuid.UUID,
	userID uuid.UUID,
	payload interface{},
) {
	options := &EmitOptions{Payload: payload}
	s.Emit(ctx, tx, eventType, entityType, entityID, string(auditentity.ActorTypeUser), &userID, options)
}

// EmitSystem is a convenience method for logging system-initiated events.
func (s *AuditService) EmitSystem(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	entityType string,
	entityID uuid.UUID,
	payload interface{},
) {
	options := &EmitOptions{Payload: payload}
	s.Emit(ctx, tx, eventType, entityType, entityID, string(auditentity.ActorTypeSystem), nil, options)
}

// EmitAdmin is a convenience method for logging admin-initiated events.
func (s *AuditService) EmitAdmin(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	entityType string,
	entityID uuid.UUID,
	adminID uuid.UUID,
	payload interface{},
) {
	options := &EmitOptions{Payload: payload}
	s.Emit(ctx, tx, eventType, entityType, entityID, string(auditentity.ActorTypeAdmin), &adminID, options)
}

// EmitWorker is a convenience method for logging worker-initiated events.
func (s *AuditService) EmitWorker(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	entityType string,
	entityID uuid.UUID,
	workerID string,
	payload interface{},
) {
	options := &EmitOptions{Payload: payload}
	s.Emit(ctx, tx, eventType, entityType, entityID, string(auditentity.ActorTypeWorker), nil, options)
}

// emitWithTx emits an event with the given transaction or creates a new one.
func (s *AuditService) emitWithTx(ctx context.Context, tx db.Tx, event *auditentity.AuditEvent) error {
	// If we have a transaction, use it
	if tx != nil {
		return s.repo.Emit(ctx, tx, event)
	}

	// Otherwise, create a new transaction for the audit event
	// This ensures atomicity even for standalone audit calls
	return s.db.WithTx(ctx, func(auditTx db.Tx) error {
		return s.repo.Emit(ctx, auditTx, event)
	})
}

// logError logs an audit error without breaking the flow.
func (s *AuditService) logError(event *auditentity.AuditEvent, err error) {
	if s.logger == nil {
		return // No logger configured, fail silently
	}

	s.logger.Error("audit event emission failed",
		zap.String("event_type", event.EventType),
		zap.String("entity_type", event.EntityType),
		zap.String("entity_id", event.EntityID.String()),
		zap.String("actor_type", event.ActorType),
		zap.Error(err),
	)
}

// resolveActorName looks up the actor's username at event time for immutability.
// Returns empty string if not found (graceful degradation).
func (s *AuditService) resolveActorName(ctx context.Context, tx db.Tx, actorID uuid.UUID) string {
	var username string
	var err error

	// Use provided tx or pool for standalone lookup
	if tx != nil {
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(username, '') FROM user_profiles WHERE user_id = $1
		`, actorID).Scan(&username)
	} else {
		// Use pool directly for standalone lookup
		err = s.db.Pool().QueryRow(ctx, `
			SELECT COALESCE(username, '') FROM user_profiles WHERE user_id = $1
		`, actorID).Scan(&username)
	}

	// Silently return empty string on error (audit should never break flow)
	if err != nil {
		return ""
	}
	return username
}

// =============================================================================
// QUERY METHODS
// =============================================================================

// GetByEntity retrieves audit events for a specific entity.
// Useful for viewing the complete history of an entity.
func (s *AuditService) GetByEntity(ctx context.Context, entityType string, entityID uuid.UUID, limit int) ([]*auditentity.AuditEvent, error) {
	return s.repo.GetByEntity(ctx, nil, entityType, entityID, limit)
}

// GetByActor retrieves audit events performed by a specific actor.
// Useful for auditing user/admin activity.
func (s *AuditService) GetByActor(ctx context.Context, actorType string, actorID uuid.UUID, limit int) ([]*auditentity.AuditEvent, error) {
	return s.repo.GetByActor(ctx, nil, actorType, actorID, limit)
}

// GetByEventType retrieves audit events of a specific type.
// Useful for analyzing specific event patterns.
func (s *AuditService) GetByEventType(ctx context.Context, eventType string, limit int) ([]*auditentity.AuditEvent, error) {
	return s.repo.GetByEventType(ctx, nil, eventType, limit)
}

// GetByEntityIDs retrieves audit events for multiple entity IDs of the same type.
// Useful for fetching audit trail across related entities (e.g., all decisions for a case).
func (s *AuditService) GetByEntityIDs(ctx context.Context, entityType string, entityIDs []uuid.UUID, limit int) ([]*auditentity.AuditEvent, error) {
	return s.repo.GetByEntityIDs(ctx, nil, entityType, entityIDs, limit)
}

// =============================================================================
// DOMAIN-SPECIFIC CONVENIENCE METHODS
// =============================================================================
//
// These methods provide type-safe ways to emit common audit events.
// They handle the event type strings and payload structures automatically.

// =============================================================================
// ORDER EVENTS
// =============================================================================

// OrderCreated emits an audit event when an order is created.
func (s *AuditService) OrderCreated(ctx context.Context, tx db.Tx, orderID, buyerID, sellerID uuid.UUID, amount int64) {
	s.EmitUser(ctx, tx,
		string(auditentity.OrderCreated),
		string(auditentity.EntityOrder),
		orderID,
		buyerID,
		map[string]interface{}{
			"seller_id": sellerID.String(),
			"amount":    amount,
		},
	)
}

// OrderPaid emits an audit event when an order is paid.
func (s *AuditService) OrderPaid(ctx context.Context, tx db.Tx, orderID, buyerID, paymentID uuid.UUID, amount int64) {
	s.EmitUser(ctx, tx,
		string(auditentity.OrderPaid),
		string(auditentity.EntityOrder),
		orderID,
		buyerID,
		map[string]interface{}{
			"payment_id": paymentID.String(),
			"amount":     amount,
		},
	)
}

// OrderShipped emits an audit event when an order is shipped.
func (s *AuditService) OrderShipped(ctx context.Context, tx db.Tx, orderID, sellerID uuid.UUID) {
	s.EmitUser(ctx, tx,
		string(auditentity.OrderShipped),
		string(auditentity.EntityOrder),
		orderID,
		sellerID,
		nil,
	)
}

// OrderCompleted emits an audit event when an order is completed.
func (s *AuditService) OrderCompleted(ctx context.Context, tx db.Tx, orderID uuid.UUID, completedBy *uuid.UUID) {
	if completedBy != nil {
		s.EmitUser(ctx, tx,
			string(auditentity.OrderCompleted),
			string(auditentity.EntityOrder),
			orderID,
			*completedBy,
			nil,
		)
	} else {
		s.EmitSystem(ctx, tx,
			string(auditentity.OrderCompleted),
			string(auditentity.EntityOrder),
			orderID,
			map[string]interface{}{
				"trigger": "auto_release",
			},
		)
	}
}

// OrderRefunded emits an audit event when an order is refunded.
func (s *AuditService) OrderRefunded(ctx context.Context, tx db.Tx, orderID, refundID uuid.UUID, amount int64, reason string) {
	s.EmitSystem(ctx, tx,
		string(auditentity.OrderRefunded),
		string(auditentity.EntityOrder),
		orderID,
		map[string]interface{}{
			"refund_id": refundID.String(),
			"amount":    amount,
			"reason":    reason,
		},
	)
}

// =============================================================================
// PAYMENT EVENTS
// =============================================================================

// PaymentCreated emits an audit event when a payment is created.
func (s *AuditService) PaymentCreated(ctx context.Context, tx db.Tx, paymentID, userID uuid.UUID, amount int64) {
	s.EmitUser(ctx, tx,
		string(auditentity.PaymentCreated),
		string(auditentity.EntityPayment),
		paymentID,
		userID,
		map[string]interface{}{
			"amount": amount,
		},
	)
}

// PaymentSettled emits an audit event when a payment is settled.
func (s *AuditService) PaymentSettled(ctx context.Context, tx db.Tx, paymentID uuid.UUID, amount int64) {
	s.EmitSystem(ctx, tx,
		string(auditentity.PaymentSettled),
		string(auditentity.EntityPayment),
		paymentID,
		map[string]interface{}{
			"amount": amount,
		},
	)
}

// PaymentFailed emits an audit event when a payment fails.
func (s *AuditService) PaymentFailed(ctx context.Context, tx db.Tx, paymentID uuid.UUID, reason string) {
	s.EmitSystem(ctx, tx,
		string(auditentity.PaymentFailed),
		string(auditentity.EntityPayment),
		paymentID,
		map[string]interface{}{
			"reason": reason,
		},
	)
}

// CoinsEarned emits an audit event when coins are earned.
func (s *AuditService) CoinsEarned(ctx context.Context, tx db.Tx, userID uuid.UUID, amount int64, referenceType string, referenceID *uuid.UUID) {
	s.EmitUser(ctx, tx,
		string(auditentity.CoinsEarned),
		string(auditentity.EntityCoins),
		userID,
		userID,
		map[string]interface{}{
			"amount":         amount,
			"reference_type": referenceType,
			"reference_id":   referenceID.String(),
		},
	)
}

// CoinsRefunded emits an audit event when coins are refunded.
func (s *AuditService) CoinsRefunded(ctx context.Context, tx db.Tx, userID uuid.UUID, amount int64, reason string) {
	s.EmitSystem(ctx, tx,
		string(auditentity.CoinsRefunded),
		string(auditentity.EntityCoins),
		userID,
		map[string]interface{}{
			"user_id": userID.String(),
			"amount":  amount,
			"reason":  reason,
		},
	)
}

// =============================================================================
// SHIPPING EVENTS
// =============================================================================

// ShippingProofSubmitted emits an audit event when shipping proof is submitted.
func (s *AuditService) ShippingProofSubmitted(ctx context.Context, tx db.Tx, orderID, sellerID uuid.UUID, trackingNumber string) {
	s.EmitUser(ctx, tx,
		string(auditentity.ShippingProofSubmitted),
		string(auditentity.EntityShipping),
		orderID,
		sellerID,
		map[string]interface{}{
			"tracking_number": trackingNumber,
		},
	)
}

// ShippingMarkedShipped emits an audit event when shipment is marked as shipped.
func (s *AuditService) ShippingMarkedShipped(ctx context.Context, tx db.Tx, orderID, sellerID uuid.UUID) {
	s.EmitUser(ctx, tx,
		string(auditentity.ShippingMarkedShipped),
		string(auditentity.EntityShipping),
		orderID,
		sellerID,
		nil,
	)
}

// =============================================================================
// SHIPPING QUOTE EVENTS
// =============================================================================

// ShippingQuoteUsed emits an audit event when a shipping quote is successfully used in an order.
// FINAL SAFETY: Logs successful quote usage for audit trail and fraud detection.
func (s *AuditService) ShippingQuoteUsed(ctx context.Context, tx db.Tx, quoteID, buyerID, sellerID uuid.UUID, cost int64) {
	s.EmitUser(ctx, tx,
		string(auditentity.ShippingQuoteUsed),
		string(auditentity.EntityShippingQuote),
		quoteID,
		buyerID,
		map[string]interface{}{
			"seller_id": sellerID,
			"cost":      cost,
			"status":    "used",
		},
	)
}

// ShippingQuoteRejected emits an audit event when a shipping quote is rejected during validation.
// FINAL SAFETY: Logs rejection reason for security monitoring and debugging.
//
// Rejection reasons:
// - "fetch_failed": Database error during quote retrieval
// - "not_found": Quote does not exist in database
// - "invalid_status": Quote status is not ACTIVE (USED, EXPIRED, INVALID)
// - "expired": Quote expiration time has passed
// - "chat_mismatch": Quote chat_id does not match order chat_id (quote theft)
// - "seller_mismatch": Quote seller_id does not match for_sale seller_id (impersonation)
// - "auction_mismatch": Quote auction_id does not match order auction_id (cross-item usage)
// - "listing_mismatch": Quote for_sale_id does not match order for_sale_id (cross-item usage)
// - "buyer_mismatch": Quote buyer_id does not match order buyer_id (quote theft)
// - "address_mismatch": Checkout address does not match locked destination
// - "mark_used_failed": Database error during status update
func (s *AuditService) ShippingQuoteRejected(ctx context.Context, tx db.Tx, quoteID, buyerID uuid.UUID, reason, details string) {
	s.EmitUser(ctx, tx,
		string(auditentity.ShippingQuoteRejected),
		string(auditentity.EntityShippingQuote),
		quoteID,
		buyerID,
		map[string]interface{}{
			"reason":  reason,
			"details": details,
			"status":  "rejected",
		},
	)
}

// =============================================================================
// NEGOTIATION EVENTS
// =============================================================================

// NegotiationStarted emits an audit event when a negotiation starts.
func (s *AuditService) NegotiationStarted(ctx context.Context, tx db.Tx, negotiationID, buyerID, sellerID uuid.UUID, forSaleID *uuid.UUID) {
	s.EmitUser(ctx, tx,
		string(auditentity.NegotiationStarted),
		string(auditentity.EntityNegotiation),
		negotiationID,
		buyerID,
		map[string]interface{}{
			"seller_id":   sellerID.String(),
			"for_sale_id": forSaleID.String(),
		},
	)
}

// NegotiationCountered emits an audit event when a negotiation is countered.
func (s *AuditService) NegotiationCountered(ctx context.Context, tx db.Tx, negotiationID uuid.UUID, counteredBy uuid.UUID, newPrice int64) {
	s.EmitUser(ctx, tx,
		string(auditentity.NegotiationCountered),
		string(auditentity.EntityNegotiation),
		negotiationID,
		counteredBy,
		map[string]interface{}{
			"new_price": newPrice,
		},
	)
}

// NegotiationAccepted emits an audit event when a negotiation is accepted.
func (s *AuditService) NegotiationAccepted(ctx context.Context, tx db.Tx, negotiationID uuid.UUID, acceptedBy uuid.UUID) {
	s.EmitUser(ctx, tx,
		string(auditentity.NegotiationAccepted),
		string(auditentity.EntityNegotiation),
		negotiationID,
		acceptedBy,
		nil,
	)
}

// =============================================================================
// DISPUTE EVENTS
// =============================================================================

// DisputeOpened emits an audit event when a dispute is opened.
func (s *AuditService) DisputeOpened(ctx context.Context, tx db.Tx, disputeID, orderID, buyerID, sellerID uuid.UUID, reason string) {
	s.EmitUser(ctx, tx,
		string(auditentity.DisputeOpened),
		string(auditentity.EntityDispute),
		disputeID,
		buyerID,
		map[string]interface{}{
			"order_id":  orderID.String(),
			"seller_id": sellerID.String(),
			"reason":    reason,
		},
	)
}

// DisputeMarkedOverdue emits an audit event when a dispute is marked as overdue.
func (s *AuditService) DisputeMarkedOverdue(ctx context.Context, tx db.Tx, disputeID, orderID uuid.UUID, daysOpen int) {
	s.EmitSystem(ctx, tx,
		string(auditentity.DisputeMarkedOverdue),
		string(auditentity.EntityDispute),
		disputeID,
		map[string]interface{}{
			"order_id":  orderID.String(),
			"days_open": daysOpen,
			"reason":    "Dispute exceeded escalation threshold",
		},
	)
}

// DisputeAutoResolved emits an audit event when a dispute is auto-resolved by timeout.
func (s *AuditService) DisputeAutoResolved(ctx context.Context, tx db.Tx, disputeID, orderID uuid.UUID, resolution string, daysOpen int, timeoutDays int) {
	s.EmitSystem(ctx, tx,
		string(auditentity.DisputeAutoResolved),
		string(auditentity.EntityDispute),
		disputeID,
		map[string]interface{}{
			"order_id":     orderID.String(),
			"resolution":   resolution,
			"days_open":    daysOpen,
			"timeout_days": timeoutDays,
			"reason":       "Dispute exceeded timeout period - auto-resolved by system",
		},
	)
}

// DisputeResolved emits an audit event when a dispute is resolved by admin.
func (s *AuditService) DisputeResolved(ctx context.Context, tx db.Tx, disputeID, orderID uuid.UUID, resolution string, adminID uuid.UUID) {
	s.EmitAdmin(ctx, tx,
		string(auditentity.DisputeResolved),
		string(auditentity.EntityDispute),
		disputeID,
		adminID,
		map[string]interface{}{
			"order_id":   orderID.String(),
			"resolution": resolution,
		},
	)
}

// =============================================================================
// GOVERNANCE EVENTS
// =============================================================================

// GovernanceDecisionCreated emits an audit event when an admin creates a Decision.
// This is the primary governance audit event — it captures the admin's governance
// action with full provenance (who decided what, against which case, with which outcome).
//
// MUST be called within the same transaction as the Decision creation.
// Actor: admin (the decided_by user).
//
// IMPORTANT: Unlike Emit/EmitAdmin, this method RETURNS an error because
// governance Decision audit is MANDATORY. If the audit INSERT fails, the caller
// must propagate the error to roll back the containing transaction.
// This bypasses the Emit error-swallowing behavior for mandatory governance audit.
func (s *AuditService) GovernanceDecisionCreated(
	ctx context.Context,
	tx db.Tx,
	decisionID, caseID, adminID uuid.UUID,
	outcome string,
	payload map[string]interface{},
) error {
	event := auditentity.NewAuditEvent(
		auditentity.GovernanceDecisionCreated,
		auditentity.EntityGovernanceDecision,
		decisionID,
		auditentity.ActorTypeAdmin,
		&adminID,
		payload,
	)

	// Resolve actor name for immutability
	if resolvedName := s.resolveActorName(ctx, tx, adminID); resolvedName != "" {
		if p, ok := event.PayloadJSON.(map[string]interface{}); ok {
			p["actor_name"] = resolvedName
		}
	}

	// Direct repository call — error is NOT swallowed.
	// The caller (DecisionService) propagates this error to roll back the TX.
	return s.repo.Emit(ctx, tx, event)
}

// =============================================================================
// AUDIT TRAIL QUERY
// =============================================================================

// GetAuditTrailForEntity returns a formatted audit trail for an entity.
// Useful for displaying entity history in admin panels.
func (s *AuditService) GetAuditTrailForEntity(ctx context.Context, entityType string, entityID uuid.UUID, limit int) ([]map[string]interface{}, error) {
	events, err := s.GetByEntity(ctx, entityType, entityID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit trail: %w", err)
	}

	trail := make([]map[string]interface{}, len(events))
	for i, event := range events {
		trail[i] = map[string]interface{}{
			"id":         event.ID.String(),
			"event_type": event.EventType,
			"actor_type": event.ActorType,
			"actor_id":   event.ActorID,
			"payload":    event.PayloadJSON,
			"created_at": event.CreatedAt,
		}
	}

	return trail, nil
}
