package entity

// EscrowStatus represents the state of escrow funds for an order.
//
// CRITICAL: This is a READ-ONLY projection of Wallet.Escrow.Status.
// MUST match wallet/entity/escrow.go exactly.
//
// Valid states (mirrored from Wallet):
// - "holding": Funds held in escrow awaiting completion
// - "released": Funds released to seller
// - "refunded": Funds refunded to buyer
//
// STATES REMOVED (no longer derivable from Wallet):
// - "none": Use Order.Status = "pending_payment" instead
// - "frozen": Use Order.HasDispute = true instead
// - "partially_refunded": Use Order.Status + separate tracking
// - "partially_released": Use Order.Status + separate tracking
//
// RULE: EscrowStatus can ONLY be set by deriving from Wallet.Escrow.Status.
// NEVER set independently based on business logic.
type EscrowStatus string

const (
	// EscrowStatusHolding means funds are held in escrow awaiting completion.
	// MIRRORED FROM: wallet/entity/escrow.go EscrowStatusHolding
	// NOTE: Database uses "holding", not "held" - MUST MATCH
	EscrowStatusHolding EscrowStatus = "holding"
	// EscrowStatusReleased means funds have been released to seller.
	// MIRRORED FROM: wallet/entity/escrow.go EscrowStatusReleased
	EscrowStatusReleased EscrowStatus = "released"
	// EscrowStatusRefunded means funds have been refunded to buyer.
	// MIRRORED FROM: wallet/entity/escrow.go EscrowStatusRefunded
	EscrowStatusRefunded EscrowStatus = "refunded"
)

// escrowTransitionAllowed defines valid escrow state transitions.
// Matches database constraint logic.
// MIRRORED FROM: wallet/entity/escrow.go (Wallet.Escrow state machine)
var escrowTransitionAllowed = map[EscrowStatus][]EscrowStatus{
	EscrowStatusHolding:  {EscrowStatusReleased, EscrowStatusRefunded}, // holding → released/refunded
	EscrowStatusReleased: {},                                           // Terminal state
	EscrowStatusRefunded: {},                                           // Terminal state
}

// canEscrowTransition checks if an escrow state transition is allowed.
func canEscrowTransition(from, to EscrowStatus) bool {
	allowed, exists := escrowTransitionAllowed[from]
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


