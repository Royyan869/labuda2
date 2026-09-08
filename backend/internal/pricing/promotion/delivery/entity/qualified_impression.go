package entity

import (
	"time"

	"github.com/google/uuid"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
)

// QualifiedImpression is the canonical immutable billable financial fact of
// the promotion domain.
//
// A Qualified Impression exists ONLY after server-side validation of a
// Delivery Ticket (issued, unexpired, unconsumed, contract active, target
// canonically eligible, viewer not the seller). One ticket produces at most
// one Qualified Impression — enforced structurally by the UNIQUE(ticket_id)
// database constraint, never merely by application pre-checks.
//
// The ledger charge for a QI equals finance.PromotionCharge(N, contract CPM
// snapshot) where N is the canonical per-contract sequential impression
// index. The row records the exact charge actually used so the finance
// verifier can reconcile QI -> CPM arithmetic -> ledger charge ->
// PROMOTION_ALLOCATION movement.
type QualifiedImpression struct {
	ID         uuid.UUID
	TicketID   uuid.UUID
	ContractID uuid.UUID
	// AllocationAccountID is the PROMOTION_ALLOCATION financial_accounts row
	// debited for this impression (the contract's holder-scoped allocation).
	AllocationAccountID uuid.UUID
	TargetType          promoentity.TargetType
	TargetID            uuid.UUID
	// SequenceN is the canonical sequential impression index for the
	// contract (1-based, unique per contract). Computed under the contract
	// row lock so concurrent qualifications cannot produce duplicate N.
	SequenceN int64
	// ChargeRupiah is the exact integer ledger charge S(N)-S(N-1) from the
	// canonical cumulative CPM arithmetic. May be 0 (Phase 1 zero-charge
	// case: a legitimate QI with NO money movement).
	ChargeRupiah int64
	// ServerOccurredAt is the DB-clock time the QI was validated.
	ServerOccurredAt time.Time
	CreatedAt        time.Time
}
