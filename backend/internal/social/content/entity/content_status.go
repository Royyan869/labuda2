package entity

// Status represents the content status state machine.
//
// CONTRACT ALIGNMENT V1:
// Content is a social publishing object, NOT a commerce object.
// Status transitions follow social semantics, not request-closure semantics.
//
// State Transition Rules:
//   - active     -> deleted
//   - deleted    -> (terminal, no transitions allowed)
type Status string

const (
	// StatusActive means content is published and visible.
	// This is the initial state for all newly created content.
	StatusActive Status = "active"

	// StatusDeleted is the terminal state when content is soft-deleted.
	// Once in this state, content cannot transition back.
	StatusDeleted Status = "deleted"
)

// transitionAllowed defines valid state transitions for unified content.
var transitionAllowed = map[Status][]Status{
	StatusActive:  {StatusDeleted},
	StatusDeleted: {},
}

// CanTransition checks if a state transition is allowed.
func CanTransition(from, to Status) bool {
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

// IsTerminal returns true if the status is a terminal state.
func (s Status) IsTerminal() bool {
	return s == StatusDeleted
}

// IsActive returns true if content is in an active state (published).
func (s Status) IsActive() bool {
	return s == StatusActive
}

// PublicLifecycle maps the internal content status onto the coarsened public
// lifecycle vocabulary defined by the public-card-boundary contract
// (docs/contracts/public-card-boundary.md §field-categories: coarsened
// lifecycle is { active, unavailable, removed }).
//
// Internal moderation vocabulary is NEVER leaked to the wire — every public
// response that needs to surface content state MUST go through this projection.
//
//	active     -> "active"
//	deleted    -> "removed"
//	<anything else> -> "removed"  (fail-closed; unknown internal state is
//	                              treated as not-publicly-visible)
func (s Status) PublicLifecycle() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusDeleted:
		return "removed"
	default:
		return "removed"
	}
}

// PublicLifecycleFromString is a string-typed shim for surfaces that carry
// the status as a bare string (e.g. feed projections that decouple from the
// content domain enum). Same coarsening rules as Status.PublicLifecycle.
func PublicLifecycleFromString(raw string) string {
	return Status(raw).PublicLifecycle()
}
