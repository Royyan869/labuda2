package entity

import (
	"math"
	"time"
)

// SLAUrgencyTerminal is the urgency assigned to terminal (resolved/closed)
// tickets. They are never urgent and therefore sort to the end of the queue.
const SLAUrgencyTerminal = time.Duration(math.MaxInt64)

// SLAUrgency returns the remaining time until the earliest applicable SLA
// breach for a ticket, using the canonical SLA thresholds and the canonical
// SLAMetrics (active time excluding waiting_user; first response only while
// unanswered). A negative value means the SLA is already breached. Terminal
// tickets always return SLAUrgencyTerminal.
//
// This is derived purely from existing canonical data — no new SLA rule is
// introduced.
func SLAUrgency(t *Ticket, m SLAMetrics, now time.Time) time.Duration {
	if t.IsResolved() || t.IsClosed() {
		return SLAUrgencyTerminal
	}

	urgency := SLAUrgencyTerminal

	// Resolution clock: canonical active time (excludes waiting_user) remaining.
	remainingResolution := ResolutionThreshold - m.ActiveTime
	if remainingResolution < urgency {
		urgency = remainingResolution
	}

	// First-response clock: only while no valid admin response exists yet.
	if m.FirstResponseTimestamp == nil {
		remainingFirstResponse := t.CreatedAt.Add(FirstResponseThreshold).Sub(now)
		if remainingFirstResponse < urgency {
			urgency = remainingFirstResponse
		}
	}

	return urgency
}

// SLAQueueLess reports whether ticket a must be ordered before ticket b in the
// canonical support admin queue. Canonical order (owner-locked):
//
//  1. breached/overdue tickets first
//  2. least remaining SLA time (urgency)
//  3. business priority (urgent > high > medium > low)
//  4. age (older first)
//  5. stable id tie-breaker (descending)
//
// The comparator is a total order: because every tie is broken by the stable
// id, the result is deterministic regardless of the input row order.
func SLAQueueLess(a, b *Ticket, ma, mb SLAMetrics, now time.Time) bool {
	if ma.IsOverdue != mb.IsOverdue {
		return ma.IsOverdue
	}

	ua, ub := SLAUrgency(a, ma, now), SLAUrgency(b, mb, now)
	if ua != ub {
		return ua < ub
	}

	if a.Priority != b.Priority {
		return a.Priority.Weight() > b.Priority.Weight()
	}

	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.Before(b.CreatedAt)
	}

	return a.ID.String() > b.ID.String()
}
