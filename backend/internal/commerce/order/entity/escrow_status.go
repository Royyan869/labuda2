package entity

// EscrowStatus represents the state of escrow funds for an order.
//
// Valid states:
// - "holding": Funds held in escrow awaiting completion
// - "released": Funds released to seller
// - "refunded": Funds refunded to buyer
//
// RULE: EscrowStatus is the canonical operational state, managed by EscrowService.
type EscrowStatus string

const (
	// EscrowStatusHolding means funds are held in escrow awaiting completion.
	EscrowStatusHolding EscrowStatus = "holding"
	// EscrowStatusReleased means funds have been released to seller.
	EscrowStatusReleased EscrowStatus = "released"
	// EscrowStatusRefunded means funds have been refunded to buyer.
	EscrowStatusRefunded EscrowStatus = "refunded"
)

// escrowTransitionAllowed defines valid escrow state transitions.
// Matches database constraint logic.
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
