package entity

// Status represents the status of a support ticket.
type Status string

const (
	// StatusOpen is the initial status when a ticket is created.
	StatusOpen Status = "open"

	// StatusInProgress means an admin has claimed the ticket.
	StatusInProgress Status = "in_progress"

	// StatusWaitingUser means admin is waiting for user response.
	StatusWaitingUser Status = "waiting_user"

	// StatusResolved means the ticket has been resolved.
	StatusResolved Status = "resolved"

	// StatusClosed means the ticket is permanently closed.
	StatusClosed Status = "closed"
)

// String returns the string representation of the status.
func (s Status) String() string {
	return string(s)
}

// IsValid checks if the status is valid.
func (s Status) IsValid() bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusWaitingUser, StatusResolved, StatusClosed:
		return true
	default:
		return false
	}
}

// IsOpen returns true if the ticket is in an "open" state.
// Open states are: open, in_progress, waiting_user
func (s Status) IsOpen() bool {
	return s == StatusOpen || s == StatusInProgress || s == StatusWaitingUser
}

// CanTransitionTo returns true if the status can transition to the target status.
// State machine:
// open -> in_progress (claim)
// in_progress -> waiting_user (waiting for user)
// waiting_user -> in_progress (user replied)
// in_progress -> resolved (resolve)
// waiting_user -> resolved (admin resolves while waiting)
// resolved -> closed (close)
// resolved -> open (reopen)
// closed -> open (reopen)
func (s Status) CanTransitionTo(new Status) bool {
	transitions := map[Status][]Status{
		StatusOpen:        {StatusInProgress},
		StatusInProgress:  {StatusWaitingUser, StatusResolved},
		StatusWaitingUser: {StatusInProgress, StatusResolved},
		StatusResolved:    {StatusClosed, StatusOpen},
		StatusClosed:      {StatusOpen},
	}

	allowed, exists := transitions[s]
	if !exists {
		return false
	}

	for _, allowedStatus := range allowed {
		if allowedStatus == new {
			return true
		}
	}

	return false
}


