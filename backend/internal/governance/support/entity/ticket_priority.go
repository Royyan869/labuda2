package entity

// Priority represents the priority level of a support ticket.
type Priority string

const (
	// PriorityLow is for non-urgent issues.
	PriorityLow Priority = "low"

	// PriorityMedium is the default priority.
	PriorityMedium Priority = "medium"

	// PriorityHigh is for urgent issues.
	PriorityHigh Priority = "high"

	// PriorityUrgent is for critical issues requiring immediate attention.
	PriorityUrgent Priority = "urgent"
)

// String returns the string representation of the priority.
func (p Priority) String() string {
	return string(p)
}

// IsValid checks if the priority is valid.
func (p Priority) IsValid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	default:
		return false
	}
}

// Weight returns a numeric weight for sorting (higher = more urgent).
func (p Priority) Weight() int {
	switch p {
	case PriorityLow:
		return 1
	case PriorityMedium:
		return 2
	case PriorityHigh:
		return 3
	case PriorityUrgent:
		return 4
	default:
		return 0
	}
}


