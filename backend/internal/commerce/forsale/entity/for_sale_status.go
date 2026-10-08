package entity

import (
	"errors"
	"fmt"
)

// ErrLiveImmutable is the canonical seller-edit rejection for a live ForSale.
// Seller-controlled product definition is immutable once the for_sale exists:
// create = publish, so every persisted for_sale is live.
var ErrLiveImmutable = errors.New("LIVE_IMMUTABLE: for_sale is live, seller edit forbidden")

// ForSaleStatus represents the lifecycle status of a for_sale.
//
// ═══════════════════════════════════════════════════════════════════════════════
// CANONICAL LIFECYCLE:
// ═══════════════════════════════════════════════════════════════════════════════
// active (published at create) -> sold/withdrawn
//
// THERE IS NO DRAFT STATE (owner decision, Oct 2026): create = publish. A
// seller who completes the create form is publishing; the surface is active
// and public from the moment it exists.
//
// ═══════════════════════════════════════════════════════════════════════════════
// STATUS MEANINGS:
// ═══════════════════════════════════════════════════════════════════════════════
// - active:    Published and market-ready. ALWAYS PUBLIC (enforced invariant).
//              Visible to buyers if stock > 0. Purchasable when all conditions met.
//              Creating one requires an active seller subscription (market
//              authority) — checked inside the create transaction.
//
// - sold:      Terminal state. All stock sold out. NOT editable, NOT purchasable.
//
// - withdrawn: Terminal state. Seller removed from sale. NOT editable, NOT purchasable.
//
// ═══════════════════════════════════════════════════════════════════════════════
// HARD INVARIANT: STATUS → VISIBILITY MAPPING
// ═══════════════════════════════════════════════════════════════════════════════
// - active    → MUST BE public (market-visible, enforces ACTIVE = PUBLIC ONLY)
// - sold      → visibility irrelevant (terminal state)
// - withdrawn → visibility irrelevant (terminal state)
//
// A for_sale can be:
// - active + public:    full market for_sale (the ONLY valid active state;
//                       the constructor sets visibility=public unconditionally)
// - active + private:   INVALID (never constructed)
//
// ═══════════════════════════════════════════════════════════════════════════════
// RESERVATION & STOCK:
// ═══════════════════════════════════════════════════════════════════════════════
// Reservation is handled by Order entity, not ForSale.
// When an order is created, for_sale stock is reduced immediately.
// If order fails/cancels, stock is restored via OrderService.
type ForSaleStatus string

const (
	// ForSaleStatusActive is the published state — the INITIAL state of every
	// created for_sale (create = publish). Visible to buyers if
	// visibility=public AND stock > 0.
	ForSaleStatusActive ForSaleStatus = "active"

	// ForSaleStatusSold is when for_sale has been successfully sold out.
	// Ordinarily terminal for seller actions. Can return to active ONLY through
	// stock restoration (order cancellation/expiration) — never through seller action.
	ForSaleStatusSold ForSaleStatus = "sold"

	// ForSaleStatusWithdrawn is when seller removes the for_sale from sale.
	// Ordinarily terminal for seller actions. Can return to active ONLY through
	// moderation restoration (governance override) — never through seller action.
	ForSaleStatusWithdrawn ForSaleStatus = "withdrawn"
)

// transitionAllowed defines ordinary (non-governed) state transitions.
// This is the primary transition graph for seller-initiated actions.
// ─────────────────────────────────────────────────────────────────────────────
// ACTIVE can transition to: sold (stock exhaustion), withdrawn (seller withdrawal)
// SOLD → active: ONLY through stock restoration (order cancel/expire) — not here
// WITHDRAWN → active: ONLY through moderation restoration — not here
var transitionAllowed = map[ForSaleStatus][]ForSaleStatus{
	ForSaleStatusActive:    {ForSaleStatusSold, ForSaleStatusWithdrawn},
	ForSaleStatusSold:      {}, // Seller-terminal; governed reversal via RestoreQuantity only
	ForSaleStatusWithdrawn: {}, // Seller-terminal; governed reversal via Moderation only
}

// CanTransition checks if a state transition is allowed.
func CanTransition(from, to ForSaleStatus) bool {
	allowed, exists := transitionAllowed[from]
	if !exists {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// InvalidTransitionError is returned when attempting an invalid state transition.
type InvalidTransitionError struct {
	CurrentStatus ForSaleStatus
	TargetStatus  ForSaleStatus
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid for_sale status transition: %s -> %s", e.CurrentStatus, e.TargetStatus)
}

// IsValid checks if the for_sale status is valid.
func (s ForSaleStatus) IsValid() bool {
	switch s {
	case ForSaleStatusActive, ForSaleStatusSold, ForSaleStatusWithdrawn:
		return true
	default:
		return false
	}
}

// String returns the string representation of the for_sale status.
func (s ForSaleStatus) String() string {
	return string(s)
}

// IsRepostable returns true if the for_sale is in a state where social reposts
// are permitted.
//
// REPOST POLICY: Only active for_sales can be reposted. Sold, withdrawn and
// any unknown status are not repostable.
//
// This is the single source of truth for the repost creation gate
// (commerceResponse.Validator via content_service.validateCommerceReference) and the read-side governance filter
// (feed/search SQL NOT EXISTS checks for targetType='for_sale').
func (s ForSaleStatus) IsRepostable() bool {
	return s == ForSaleStatusActive
}

// PublicLifecycle returns the coarsened public lifecycle string for this
// for_sale status. The public vocabulary is intentionally narrow:
//
//	active       — buyable now
//	sold         — sold (honest buyer-facing outcome; owner decision: the
//	               fact that an item sold is public business truth, while
//	               the reason an item was pulled stays private)
//	unavailable  — not buyable, reason withheld (withdrawn or any unknown state)
//	removed      — reserved for moderation/hard-delete; ForSaleStatus does not
//	               model these today so this method never returns "removed".
//
// Internal enum values (withdrawn, …) MUST NOT cross the public
// boundary. Public surfaces should call this method and emit the result instead
// of String() / raw enum text.
func (s ForSaleStatus) PublicLifecycle() string {
	switch s {
	case ForSaleStatusActive:
		return "active"
	case ForSaleStatusSold:
		return "sold"
	case ForSaleStatusWithdrawn:
		return "unavailable"
	default:
		return "unavailable"
	}
}
