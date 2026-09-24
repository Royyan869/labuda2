package entity

import (
	"time"

	"github.com/google/uuid"
)

// FundingIntent is a snapshot funding calculation and payment obligation.
// It captures the promotion parameters, required cost, available PROMOTE_BALANCE,
// and exact shortage at creation time. The payment amount is immutable.
//
// AUTHORITY: this entity is informational only. The ledger is the financial
// truth. This entity does not authorize any money movement.
//
// FundingIntent is NOT:
//   - a promotion contract;
//   - a reservation of PROMOTE_BALANCE;
//   - a binding to a specific future promotion;
//   - a lock on funds.
//
// After settlement, the exact payment amount is credited to PROMOTE_BALANCE
// through the existing canonical settlement path. The resulting balance is
// reusable — it is NOT tied to this FundingIntent. The seller may create
// the original promotion, a modified promotion, another promotion, or wait.
//
// Status is DERIVED from the linked billing transaction:
//   - billing_transaction_id == nil → pending (no payment yet)
//   - billing.status == "paid"      → paid (funding complete)
//   - billing.status == "failed"    → failed
type FundingIntent struct {
	ID                   uuid.UUID
	SellerID             uuid.UUID
	Kind                 string
	BudgetRupiah         int64
	DurationDays         int64
	CityIDs              []string // nullable; empty = nationwide
	ShortageAmount       int64    // exact shortage (always > 0)
	BillingTransactionID *uuid.UUID
	CreatedAt            time.Time
}
