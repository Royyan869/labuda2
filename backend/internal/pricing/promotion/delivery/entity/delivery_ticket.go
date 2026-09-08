// Package entity holds the canonical Phase 3 Delivery Ticket and Qualified
// Impression aggregates.
//
// The Delivery Ticket is a server-side delivery AUTHORIZATION for one
// potential promotion delivery. It is NOT a financial authority: issuing a
// ticket moves no money, reserves no allocation, and creates no ledger
// transaction. Only a server-validated Qualified Impression moves money
// (PROMOTION_ALLOCATION -> PLATFORM_REVENUE via FinanceService).
package entity

import (
	"time"

	"github.com/google/uuid"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
)

// TicketStatus is the canonical Delivery Ticket lifecycle.
//
//	issued → consumed          (one Qualified Impression was produced)
//	issued → invalidated       (contract finalized; ticket can never qualify)
//
// A ticket that is not in the 'issued' state MUST NOT produce a Qualified
// Impression.
type TicketStatus string

const (
	// TicketStatusIssued means the ticket is a live delivery authorization.
	TicketStatusIssued TicketStatus = "issued"
	// TicketStatusConsumed means the ticket produced exactly one Qualified
	// Impression and can never qualify again.
	TicketStatusConsumed TicketStatus = "consumed"
	// TicketStatusInvalidated means the ticket was frozen (contract
	// finalization) before qualification and can never qualify.
	TicketStatusInvalidated TicketStatus = "invalidated"
)

// IsValid reports whether s is a canonical ticket status.
func (s TicketStatus) IsValid() bool {
	switch s {
	case TicketStatusIssued, TicketStatusConsumed, TicketStatusInvalidated:
		return true
	default:
		return false
	}
}

// CanQualify reports whether the ticket is still an issued, qualifiable
// authorization.
func (s TicketStatus) CanQualify() bool {
	return s == TicketStatusIssued
}

// DeliveryTicket is the canonical server-side delivery authorization.
//
// Money boundary: issuance performs NO ledger transaction, NO allocation
// reservation, NO balance mutation. The ticket is authorization/state only.
type DeliveryTicket struct {
	ID         uuid.UUID
	ContractID uuid.UUID
	// TargetType / TargetID identify the promoted target. The qualifier MUST
	// re-read the canonical commerce target state at qualification time — the
	// ticket snapshot is never trusted for billing.
	TargetType promoentity.TargetType
	TargetID   uuid.UUID
	// ViewerID binds the audience identity so seller self-delivery is
	// structurally detectable (viewer == contract seller is not billable).
	ViewerID uuid.UUID
	Status   TicketStatus
	// IssuedAt / ExpiresAt come from the DB clock (server time authority);
	// client timestamps are never trusted.
	IssuedAt   time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}
