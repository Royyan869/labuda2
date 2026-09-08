// Package entity holds the canonical Promotion Contract aggregate types.
//
// The Promotion Contract is the lifecycle authority for one Promotion. It is
// NOT a financial authority: money authority stays in the immutable finance
// ledger (FinanceService). The contract carries the immutable pricing
// snapshot and the pacing boundary (planned_start / planned_finish).
package entity

import (
	"time"

	"github.com/google/uuid"
)

// Kind identifies the promotion subject domain.
type Kind string

const (
	// KindInternal promotes For Sale / Auction targets.
	KindInternal Kind = "internal"
	// KindExternal promotes external subjects (Event / Business).
	KindExternal Kind = "external"
)

// IsValid reports whether k is a canonical contract kind.
func (k Kind) IsValid() bool {
	switch k {
	case KindInternal, KindExternal:
		return true
	default:
		return false
	}
}

// Status is the canonical contract lifecycle state.
type Status string

const (
	// StatusPrepared means the contract is created but not yet delivering.
	// V1 has no future-start feature: canonical creation activates the
	// contract immediately, so 'prepared' is reserved for the future-start
	// case and still occupies the seller slot.
	StatusPrepared Status = "prepared"
	// StatusActive means the contract is delivering (or ready to deliver).
	StatusActive Status = "active"
	// StatusPaused means an EXPLICIT seller pause stopped ordinary delivery.
	// Allocation stays held and the seller slot stays occupied.
	StatusPaused Status = "paused"
	// StatusFinalizing means canonical financial finalization is in flight
	// (outstanding delivery settlement). In the current phase — before the
	// Delivery Ticket domain exists — finalizing is transient within the
	// finalization transaction.
	StatusFinalizing Status = "finalizing"
	// StatusFinalized is the terminal state. The seller slot is freed and
	// remaining allocation has been released exactly once.
	StatusFinalized Status = "finalized"
)

// IsNonFinalized reports whether the status still occupies the seller slot.
func (s Status) IsNonFinalized() bool {
	switch s {
	case StatusPrepared, StatusActive, StatusPaused, StatusFinalizing:
		return true
	default:
		return false
	}
}

// IsFinalized reports whether the status is terminal.
func (s Status) IsFinalized() bool {
	return s == StatusFinalized
}

// Contract is the canonical promotion contract aggregate.
//
// Field immutability: budget_rupiah, cpm_rupiah (pricing snapshot),
// planned_start, kind and the holder scope of allocation_account_id are
// immutable after creation — no update path rewrites them.
type Contract struct {
	ID                  uuid.UUID
	SellerID            uuid.UUID
	Kind                Kind
	Status              Status
	BudgetRupiah        int64
	CPMRupiah           int64 // immutable pricing snapshot at creation
	PlannedStart        time.Time
	PlannedFinish       time.Time
	AllocationAccountID uuid.UUID // financial_accounts PROMOTION_ALLOCATION row
	PausedAt            *time.Time
	FinalizedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
