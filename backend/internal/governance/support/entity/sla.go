package entity

import (
	"time"
)

// SLA defines Service Level Agreement thresholds for support tickets.
type SLA struct {
	FirstResponse time.Duration
	Resolution     time.Duration
}

// Default SLA thresholds.
const (
	// FirstResponseThreshold is the maximum time to first respond to a ticket.
	FirstResponseThreshold = 1 * time.Hour

	// ResolutionThreshold is the maximum time to resolve a ticket.
	ResolutionThreshold = 24 * time.Hour
)

// SLAMetrics contains computed SLA metrics for a ticket.
type SLAMetrics struct {
	// First response metrics
	FirstResponseTime       *time.Duration
	FirstResponseOverdue    bool
	FirstResponseTimestamp  *time.Time

	// Resolution metrics (excluding waiting periods)
	ResolutionTime          *time.Duration
	ResolutionOverdue       bool
	ResolutionTimestamp     *time.Time

	// Waiting time tracking
	WaitingTime             time.Duration
	ActiveTime              time.Duration

	// Overall status
	IsOverdue               bool

	// Next action hint
	NextAction              string
}

// Next action constants
const (
	NextActionReply    = "reply"     // Admin should reply to user
	NextActionWait     = "wait"      // Waiting for user response
	NextActionResolve  = "resolve"   // Admin should resolve ticket
	NextActionNone     = "none"      // Ticket resolved/closed
)

// GetDefaultSLA returns the default SLA configuration.
func GetDefaultSLA() SLA {
	return SLA{
		FirstResponse: FirstResponseThreshold,
		Resolution:     ResolutionThreshold,
	}
}

// ComputeSLAMetricsSimple computes simplified SLA metrics without message data.
// Used for list views where fetching messages would be too expensive.
//
// Simplified rules:
// - First response: based on assigned_at (may not be accurate but fast)
// - Resolution: based on created_at to resolved_at (includes waiting time)
// - Next action: based on status only
func (t *Ticket) ComputeSLAMetricsSimple() SLAMetrics {
	metrics := SLAMetrics{
		FirstResponseTime:      nil,
		FirstResponseOverdue:   false,
		FirstResponseTimestamp: nil,
		ResolutionTime:         nil,
		ResolutionOverdue:      false,
		ResolutionTimestamp:    nil,
		WaitingTime:            0,
		ActiveTime:             0,
		IsOverdue:              false,
		NextAction:             t.computeSimpleNextAction(),
	}

	// Compute first response time (simplified - using assigned_at)
	if t.AssignedAt != nil {
		firstResponseDuration := t.AssignedAt.Sub(t.CreatedAt)
		metrics.FirstResponseTime = &firstResponseDuration
		metrics.FirstResponseTimestamp = t.AssignedAt
		metrics.FirstResponseOverdue = firstResponseDuration > FirstResponseThreshold
	} else {
		// Not yet assigned - check if overdue based on current time
		timeSinceCreation := time.Since(t.CreatedAt)
		metrics.FirstResponseOverdue = timeSinceCreation > FirstResponseThreshold
	}

	// Compute resolution time (simplified - includes waiting time)
	if t.ResolvedAt != nil {
		resolutionDuration := t.ResolvedAt.Sub(t.CreatedAt)
		metrics.ResolutionTime = &resolutionDuration
		metrics.ResolutionTimestamp = t.ResolvedAt
		metrics.ResolutionOverdue = resolutionDuration > ResolutionThreshold
		metrics.ActiveTime = resolutionDuration
	} else if !t.IsClosed() {
		// Not yet resolved - check based on current time
		timeSinceCreation := time.Since(t.CreatedAt)
		metrics.ResolutionOverdue = timeSinceCreation > ResolutionThreshold
		metrics.ActiveTime = timeSinceCreation
	}

	// Compute overall overdue status
	metrics.IsOverdue = metrics.FirstResponseOverdue || metrics.ResolutionOverdue

	return metrics
}

// computeSimpleNextAction determines next action without message data.
func (t *Ticket) computeSimpleNextAction() string {
	// If resolved or closed, no action needed
	if t.IsResolved() || t.IsClosed() {
		return NextActionNone
	}

	// Based on status
	switch t.Status {
	case StatusOpen:
		return NextActionReply
	case StatusInProgress:
		return NextActionResolve
	case StatusWaitingUser:
		return NextActionWait
	default:
		return NextActionResolve
	}
}

// MessageEvent represents a message for SLA calculation.
type MessageEvent struct {
	Timestamp   time.Time
	SenderID    string
	IsAdmin     bool
	MessageType string
}

// ComputeSLAMetrics computes SLA metrics for a ticket given its message history.
//
// Rules:
// - First response time: time from CreatedAt to FIRST ADMIN MESSAGE (not assigned_at)
// - Resolution time: time from CreatedAt to ResolvedAt, EXCLUDING waiting_user periods
// - Overdue: separate flags for first_response and resolution
// - next_action: computed based on status and last message
func (t *Ticket) ComputeSLAMetrics(messages []MessageEvent) SLAMetrics {
	metrics := SLAMetrics{
		FirstResponseTime:      nil,
		FirstResponseOverdue:   false,
		FirstResponseTimestamp: nil,
		ResolutionTime:         nil,
		ResolutionOverdue:      false,
		ResolutionTimestamp:    nil,
		WaitingTime:            0,
		ActiveTime:             0,
		IsOverdue:              false,
		NextAction:             NextActionNone,
	}

	// Find first admin message timestamp
	var firstAdminMessageTime *time.Time
	for _, msg := range messages {
		if msg.IsAdmin {
			if firstAdminMessageTime == nil || msg.Timestamp.Before(*firstAdminMessageTime) {
				firstAdminMessageTime = &msg.Timestamp
			}
		}
	}

	// Compute first response time
	if firstAdminMessageTime != nil {
		firstResponseDuration := firstAdminMessageTime.Sub(t.CreatedAt)
		metrics.FirstResponseTime = &firstResponseDuration
		metrics.FirstResponseTimestamp = firstAdminMessageTime
		metrics.FirstResponseOverdue = firstResponseDuration > FirstResponseThreshold
	} else {
		// No admin message yet - check if overdue based on current time
		timeSinceCreation := time.Since(t.CreatedAt)
		metrics.FirstResponseOverdue = timeSinceCreation > FirstResponseThreshold
	}

	// Compute resolution time (excluding waiting periods)
	if t.ResolvedAt != nil {
		// Calculate active time (excluding waiting_user status periods)
		activeTime := t.calculateActiveTime(messages)
		metrics.ActiveTime = activeTime
		metrics.ResolutionTime = &activeTime
		metrics.ResolutionTimestamp = t.ResolvedAt
		metrics.ResolutionOverdue = activeTime > ResolutionThreshold
	} else if !t.IsClosed() {
		// Not yet resolved - check current active time
		activeTime := t.calculateActiveTime(messages)
		metrics.ActiveTime = activeTime

		// Check if overdue based on current active time
		metrics.ResolutionOverdue = activeTime > ResolutionThreshold
	}

	// Calculate total waiting time
	metrics.WaitingTime = t.calculateWaitingTime(messages)

	// Compute overall overdue status
	metrics.IsOverdue = metrics.FirstResponseOverdue || metrics.ResolutionOverdue

	// Compute next action
	metrics.NextAction = t.computeNextAction(messages)

	return metrics
}

// calculateActiveTime calculates the actual working time on a ticket, excluding waiting periods.
func (t *Ticket) calculateActiveTime(messages []MessageEvent) time.Duration {
	if t.ResolvedAt != nil {
		return t.calculateActiveTimeUntil(messages, *t.ResolvedAt)
	}

	// Not resolved - calculate active time until now
	return t.calculateActiveTimeUntil(messages, time.Now())
}

// calculateActiveTimeUntil calculates active time up to a given timestamp.
func (t *Ticket) calculateActiveTimeUntil(messages []MessageEvent, until time.Time) time.Duration {
	var activeTime time.Duration

	// Start from ticket creation

	// Sort messages by timestamp
	sortedMessages := make([]MessageEvent, len(messages))
	copy(sortedMessages, messages)

	// Simple bubble sort by timestamp
	for i := 0; i < len(sortedMessages)-1; i++ {
		for j := 0; j < len(sortedMessages)-i-1; j++ {
			if sortedMessages[j].Timestamp.After(sortedMessages[j+1].Timestamp) {
				sortedMessages[j], sortedMessages[j+1] = sortedMessages[j+1], sortedMessages[j]
			}
		}
	}

	// This is a simplified calculation. In production, you'd track status changes from events.
	// For now, we'll use a heuristic: waiting_user status excludes time after last admin message
	// if no user response followed.

	var lastAdminMessageTime *time.Time
	var lastUserMessageTime *time.Time

	for _, msg := range sortedMessages {
		if msg.Timestamp.After(until) {
			break
		}

		if msg.IsAdmin {
			lastAdminMessageTime = &msg.Timestamp
		} else {
			lastUserMessageTime = &msg.Timestamp
		}
	}

	// If we have a last admin message but no user response after, that period is waiting time
	if lastAdminMessageTime != nil {
		if lastUserMessageTime != nil && lastUserMessageTime.After(*lastAdminMessageTime) {
			// User responded - time is active
			activeTime = until.Sub(t.CreatedAt)
		} else if t.Status == StatusWaitingUser {
			// Currently waiting - exclude time after last admin message
			waitingPeriod := until.Sub(*lastAdminMessageTime)
			activeTime = until.Sub(t.CreatedAt) - waitingPeriod
		} else {
			// In progress - all time is active
			activeTime = until.Sub(t.CreatedAt)
		}
	} else {
		// No admin messages yet
		activeTime = 0
	}

	// Don't return negative time
	if activeTime < 0 {
		activeTime = 0
	}

	return activeTime
}

// calculateWaitingTime calculates total time spent in waiting_user status.
func (t *Ticket) calculateWaitingTime(messages []MessageEvent) time.Duration {
	// Simplified: if status is waiting_user, calculate time from last admin message
	if t.Status != StatusWaitingUser {
		return 0
	}

	// Find last admin message
	var lastAdminMessageTime *time.Time
	for _, msg := range messages {
		if msg.IsAdmin {
			if lastAdminMessageTime == nil || msg.Timestamp.After(*lastAdminMessageTime) {
				lastAdminMessageTime = &msg.Timestamp
			}
		}
	}

	if lastAdminMessageTime != nil {
		return time.Since(*lastAdminMessageTime)
	}

	return 0
}

// computeNextAction determines what the admin should do next.
func (t *Ticket) computeNextAction(messages []MessageEvent) string {
	// If resolved or closed, no action needed
	if t.IsResolved() || t.IsClosed() {
		return NextActionNone
	}

	// Check if there's a recent user message waiting for response
	var lastUserMessageTime *time.Time
	var lastAdminMessageTime *time.Time

	for _, msg := range messages {
		if msg.IsAdmin {
			if lastAdminMessageTime == nil || msg.Timestamp.After(*lastAdminMessageTime) {
				lastAdminMessageTime = &msg.Timestamp
			}
		} else {
			if lastUserMessageTime == nil || msg.Timestamp.After(*lastUserMessageTime) {
				lastUserMessageTime = &msg.Timestamp
			}
		}
	}

	// If user sent the last message, admin should reply
	if lastUserMessageTime != nil {
		if lastAdminMessageTime == nil || lastUserMessageTime.After(*lastAdminMessageTime) {
			return NextActionReply
		}
	}

	// If admin sent the last message, waiting for user
	if lastAdminMessageTime != nil {
		if lastUserMessageTime == nil || lastAdminMessageTime.After(*lastUserMessageTime) {
			return NextActionWait
		}
	}

	// No messages yet - first response needed
	if lastAdminMessageTime == nil {
		return NextActionReply
	}

	// In progress but no recent messages
	return NextActionResolve
}

// IsFirstResponseOverdue returns true if first response is overdue.
func (t *Ticket) IsFirstResponseOverdue(firstAdminMessageTime *time.Time) bool {
	if firstAdminMessageTime != nil {
		return firstAdminMessageTime.Sub(t.CreatedAt) > FirstResponseThreshold
	}
	return time.Since(t.CreatedAt) > FirstResponseThreshold
}

// IsResolutionOverdue returns true if resolution is overdue (excluding waiting time).
func (t *Ticket) IsResolutionOverdue(activeTime time.Duration) bool {
	if t.ResolvedAt != nil {
		return activeTime > ResolutionThreshold
	}
	if t.IsClosed() {
		return false
	}
	return activeTime > ResolutionThreshold
}


