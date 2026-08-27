package entity

// Escalation represents the escalation level of a support ticket.
type Escalation string

const (
	// EscalationNone is the default level for regular support tickets.
	EscalationNone Escalation = "none"

	// EscalationDispute is for tickets escalated to the dispute team.
	EscalationDispute Escalation = "dispute"

	// EscalationFinance is for tickets escalated to the finance team.
	EscalationFinance Escalation = "finance"

	// EscalationOps is for tickets escalated to the operations team.
	EscalationOps Escalation = "ops"
)

// String returns the string representation of the escalation.
func (e Escalation) String() string {
	return string(e)
}

// IsValid checks if the escalation is valid.
func (e Escalation) IsValid() bool {
	switch e {
	case EscalationNone, EscalationDispute, EscalationFinance, EscalationOps:
		return true
	default:
		return false
	}
}

// IsEscalated returns true if the ticket has been escalated beyond default.
func (e Escalation) IsEscalated() bool {
	return e != EscalationNone
}

// CanTransitionTo returns true if the escalation can transition to the target level.
// Business rule: Escalation can only go up, never down (unless admin explicitly de-escalates)
func (e Escalation) CanTransitionTo(new Escalation) bool {
	// Allow same level (no-op)
	if e == new {
		return true
	}

	// Can always escalate to any other level
	// De-escalation requires admin action (enforced at service layer)
	return new.IsValid()
}


