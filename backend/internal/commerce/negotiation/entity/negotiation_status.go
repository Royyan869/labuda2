package entity

// NegotiationStatus represents the current state of a negotiation session.
type NegotiationStatus string

const (
	// NegotiationStatusActive - Negotiation is in progress, messages can be exchanged
	NegotiationStatusActive NegotiationStatus = "active"

	// NegotiationStatusAccepted - Seller has accepted the price, ready for trade creation
	NegotiationStatusAccepted NegotiationStatus = "accepted"

	// NegotiationStatusCancelled - Buyer cancelled the negotiation
	NegotiationStatusCancelled NegotiationStatus = "cancelled"

	// NegotiationStatusExpired - Negotiation expired (timeout)
	NegotiationStatusExpired NegotiationStatus = "expired"
)

// String returns the string representation of the status.
func (s NegotiationStatus) String() string {
	return string(s)
}

// IsActive returns true if the negotiation is in active state.
func (s NegotiationStatus) IsActive() bool {
	return s == NegotiationStatusActive
}

// IsTerminal returns true if the negotiation is in a terminal state.
// Terminal states: cancelled, expired (accepted can transition to expired)
func (s NegotiationStatus) IsTerminal() bool {
	return s == NegotiationStatusCancelled ||
		s == NegotiationStatusExpired
}

// CanTransition returns true if the transition from current to target status is allowed.
// Rules:
// - active → accepted, cancelled, expired
// - accepted → expired (negotiations can expire even after acceptance)
// - cancelled, expired are truly terminal (no transitions)
//
// NEGOTIATION EXPIRY CONSISTENCY: Accepted negotiations can expire to prevent
// "accepted but expired" state which allows checkout of stale agreements.
func (s NegotiationStatus) CanTransition(target NegotiationStatus) bool {
	switch s {
	case NegotiationStatusActive:
		return target == NegotiationStatusAccepted ||
			target == NegotiationStatusCancelled ||
			target == NegotiationStatusExpired
	case NegotiationStatusAccepted:
		return target == NegotiationStatusExpired // Accepted can expire
	case NegotiationStatusCancelled,
		NegotiationStatusExpired:
		return false // Truly terminal states
	default:
		return false
	}
}


