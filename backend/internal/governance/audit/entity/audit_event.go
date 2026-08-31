package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuditEvent represents an immutable audit log entry.
//
// DESIGN PRINCIPLES:
// - Immutable: Once created, never modified
// - Append-only: Only new records are added
// - Comprehensive: All critical business actions are logged
// - Non-blocking: Audit failures should not break business flows
type AuditEvent struct {
	ID          uuid.UUID
	EventType   string      // e.g., "order.created", "payment.settled"
	EntityType  string      // e.g., "order", "payment", "user"
	EntityID    uuid.UUID   // ID of the affected entity
	ActorType   string      // e.g., "user", "system", "admin", "worker"
	ActorID     *uuid.UUID  // ID of the actor (NULL for system)
	PayloadJSON interface{} // Additional event metadata (flexible JSON)
	CreatedAt   time.Time   // Timestamp when event occurred
}

// ActorType defines the type of actor that triggered an event.
type ActorType string

const (
	// ActorTypeUser represents a human user action
	ActorTypeUser ActorType = "user"
	// ActorTypeAdmin represents an admin action
	ActorTypeAdmin ActorType = "admin"
	// ActorTypeSystem represents a system-triggered event
	ActorTypeSystem ActorType = "system"
	// ActorTypeWorker represents a background worker action
	ActorTypeWorker ActorType = "worker"
	// ActorTypeAPI represents an API service action
	ActorTypeAPI ActorType = "api"
)

// EntityType defines the type of entity affected by an event.
type EntityType string

const (
	// EntityOrder represents an order entity
	EntityOrder EntityType = "order"
	// EntityPayment represents a payment entity
	EntityPayment EntityType = "payment"
	// EntityCoins represents coins/loyalty points entity
	EntityCoins EntityType = "coins"
	// EntityShipping represents a shipping entity
	EntityShipping EntityType = "shipping"
	// EntityNegotiation represents a negotiation entity
	EntityNegotiation EntityType = "negotiation"
	// EntityDispute represents a dispute entity
	EntityDispute EntityType = "dispute"
	// EntityUser represents a user entity
	EntityUser EntityType = "user"
	// EntityForSale represents a for_sale entity
	EntityForSale EntityType = "for_sale"
	// EntityAuction represents an auction entity
	EntityAuction EntityType = "auction"
	// EntityShippingQuote represents a shipping quote entity
	EntityShippingQuote EntityType = "shipping_quote"

	// Governance entities
	// EntityGovernanceDecision represents a governance decision entity
	EntityGovernanceDecision EntityType = "governance.decision"
)

// EventType defines the type of event that occurred.
type EventType string

// Order events
const (
	// OrderCreated events
	OrderCreated EventType = "order.created"
	// OrderPaid events
	OrderPaid EventType = "order.paid"
	// OrderShipped events
	OrderShipped EventType = "order.shipped"
	// OrderCompleted events
	OrderCompleted EventType = "order.completed"
	// OrderRefunded events
	OrderRefunded EventType = "order.refunded"
)

// Payment events
const (
	// PaymentCreated events
	PaymentCreated EventType = "payment.created"
	// PaymentSettled events
	PaymentSettled EventType = "payment.settled"
	// PaymentFailed events
	PaymentFailed EventType = "payment.failed"
)

// Coins events
const (
	// CoinsEarned events
	CoinsEarned EventType = "coins.earned"
	// CoinsRefunded events
	CoinsRefunded EventType = "coins.refunded"
)

// Shipping events
const (
	// ShippingProofSubmitted events
	ShippingProofSubmitted EventType = "shipment.proof_submitted"
	// ShippingMarkedShipped events
	ShippingMarkedShipped EventType = "shipment.marked_shipped"
)

// Shipping Quote events
const (
	// ShippingQuoteUsed events
	ShippingQuoteUsed EventType = "shipping_quote.used"
	// ShippingQuoteRejected events
	ShippingQuoteRejected EventType = "shipping_quote.rejected"
)

// Negotiation events
const (
	// NegotiationStarted events
	NegotiationStarted EventType = "negotiation.started"
	// NegotiationCountered events
	NegotiationCountered EventType = "negotiation.countered"
	// NegotiationAccepted events
	NegotiationAccepted EventType = "negotiation.accepted"
)

// Dispute events
const (
	// DisputeOpened events
	DisputeOpened EventType = "dispute.opened"
	// DisputeMarkedOverdue events
	DisputeMarkedOverdue EventType = "dispute.marked_overdue"
	// DisputeAutoResolved events
	DisputeAutoResolved EventType = "dispute.auto_resolved"
	// DisputeResolved events
	DisputeResolved EventType = "dispute.resolved"
)

// Governance events
const (
	// GovernanceDecisionCreated events — admin creates a Decision against a Case
	GovernanceDecisionCreated EventType = "governance.decision.created"
)

// String returns the string representation of the EventType.
func (e EventType) String() string {
	return string(e)
}

// String returns the string representation of the EntityType.
func (e EntityType) String() string {
	return string(e)
}

// String returns the string representation of the ActorType.
func (a ActorType) String() string {
	return string(a)
}

// NewAuditEvent creates a new AuditEvent.
//
// Parameters:
//   - eventType: The type of event (e.g., "order.created")
//   - entityType: The type of entity affected (e.g., "order")
//   - entityID: The ID of the entity affected
//   - actorType: The type of actor (e.g., "user", "system")
//   - actorID: The ID of the actor (nil for system)
//   - payload: Additional event metadata (can be nil)
func NewAuditEvent(
	eventType EventType,
	entityType EntityType,
	entityID uuid.UUID,
	actorType ActorType,
	actorID *uuid.UUID,
	payload interface{},
) *AuditEvent {
	return &AuditEvent{
		ID:          uuid.New(),
		EventType:   eventType.String(),
		EntityType:  entityType.String(),
		EntityID:    entityID,
		ActorType:   actorType.String(),
		ActorID:     actorID,
		PayloadJSON: payload,
		CreatedAt:   time.Now(),
	}
}

// NewSystemAuditEvent creates a new AuditEvent for system-triggered actions.
func NewSystemAuditEvent(
	eventType EventType,
	entityType EntityType,
	entityID uuid.UUID,
	payload interface{},
) *AuditEvent {
	return NewAuditEvent(
		eventType,
		entityType,
		entityID,
		ActorTypeSystem,
		nil,
		payload,
	)
}

// NewUserAuditEvent creates a new AuditEvent for user actions.
func NewUserAuditEvent(
	eventType EventType,
	entityType EntityType,
	entityID uuid.UUID,
	actorID uuid.UUID,
	payload interface{},
) *AuditEvent {
	return NewAuditEvent(
		eventType,
		entityType,
		entityID,
		ActorTypeUser,
		&actorID,
		payload,
	)
}
