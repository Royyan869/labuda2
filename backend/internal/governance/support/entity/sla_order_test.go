package entity

import (
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// ticketWith returns a non-terminal ticket plus the SLA metrics that drive the
// canonical queue ordering. A non-nil FirstResponseTimestamp removes the
// first-response clock, so urgency is controlled exactly by ActiveTime.
func ticketWith(priority Priority, createdAt time.Time, overdue bool, active time.Duration, id uuid.UUID) (*Ticket, SLAMetrics) {
	tk := &Ticket{
		ID:        id,
		Priority:  priority,
		Status:    StatusInProgress,
		CreatedAt: createdAt,
	}
	responded := createdAt.Add(30 * time.Minute)
	return tk, SLAMetrics{
		IsOverdue:              overdue,
		ActiveTime:             active,
		FirstResponseTimestamp: &responded,
	}
}

func orderIDs(tickets []*Ticket, metrics map[uuid.UUID]SLAMetrics, now time.Time) []uuid.UUID {
	out := append([]*Ticket(nil), tickets...)
	sort.SliceStable(out, func(i, j int) bool {
		return SLAQueueLess(out[i], out[j], metrics[out[i].ID], metrics[out[j].ID], now)
	})
	ids := make([]uuid.UUID, len(out))
	for i, t := range out {
		ids[i] = t.ID
	}
	return ids
}

func TestSLAQueueLess_CanonicalOrder(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	metrics := map[uuid.UUID]SLAMetrics{}

	add := func(priority Priority, createdAt time.Time, overdue bool, active time.Duration) *Ticket {
		id := uuid.New()
		tk, m := ticketWith(priority, createdAt, overdue, active, id)
		metrics[id] = m
		return tk
	}

	t.Run("overdue first, then urgency beats priority", func(t *testing.T) {
		overdue := add(PriorityLow, now.Add(-10*time.Hour), true, 1*time.Hour)
		soon := add(PriorityHigh, now.Add(-30*time.Hour), false, ResolutionThreshold-1*time.Hour) // remaining 1h
		later := add(PriorityUrgent, now.Add(-2*time.Hour), false, 1*time.Hour)                   // remaining ~23h

		got := orderIDs([]*Ticket{later, soon, overdue}, metrics, now)
		require.Equal(t, []uuid.UUID{overdue.ID, soon.ID, later.ID}, got)
	})

	t.Run("priority breaks an urgency tie", func(t *testing.T) {
		urgent := add(PriorityUrgent, now.Add(-5*time.Hour), false, 12*time.Hour)
		low := add(PriorityLow, now.Add(-5*time.Hour), false, 12*time.Hour)

		got := orderIDs([]*Ticket{low, urgent}, metrics, now)
		require.Equal(t, []uuid.UUID{urgent.ID, low.ID}, got)
	})

	t.Run("age breaks a priority tie (older first)", func(t *testing.T) {
		older := add(PriorityMedium, now.Add(-5*time.Hour), false, 12*time.Hour)
		newer := add(PriorityMedium, now.Add(-1*time.Hour), false, 12*time.Hour)

		got := orderIDs([]*Ticket{newer, older}, metrics, now)
		require.Equal(t, []uuid.UUID{older.ID, newer.ID}, got)
	})

	t.Run("stable id tie-breaker is deterministic and descending", func(t *testing.T) {
		created := now.Add(-3 * time.Hour)
		hi := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
		lo := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		a, ma := ticketWith(PriorityMedium, created, false, 12*time.Hour, hi)
		b, mb := ticketWith(PriorityMedium, created, false, 12*time.Hour, lo)
		metrics[hi] = ma
		metrics[lo] = mb

		// Regardless of input order, the higher id sorts first.
		require.Equal(t, []uuid.UUID{hi, lo}, orderIDs([]*Ticket{a, b}, metrics, now))
		require.Equal(t, []uuid.UUID{hi, lo}, orderIDs([]*Ticket{b, a}, metrics, now))
	})
}

func TestSLAUrgency_TerminalAndClocks(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	t.Run("terminal tickets are never urgent", func(t *testing.T) {
		tk := &Ticket{ID: uuid.New(), Status: StatusResolved, CreatedAt: now.Add(-10 * time.Hour)}
		require.Equal(t, SLAUrgencyTerminal, SLAUrgency(tk, SLAMetrics{}, now))

		tk.Status = StatusClosed
		require.Equal(t, SLAUrgencyTerminal, SLAUrgency(tk, SLAMetrics{}, now))
	})

	t.Run("first-response clock applies only while unanswered", func(t *testing.T) {
		tk := &Ticket{ID: uuid.New(), Status: StatusOpen, CreatedAt: now.Add(-30 * time.Minute)}
		// No first response, no waiting: remaining first response ~30m, remaining
		// resolution ~23h30m -> urgency is the earliest, ~30m.
		u := SLAUrgency(tk, SLAMetrics{ActiveTime: 0}, now)
		require.InDelta(t, (30 * time.Minute).Seconds(), u.Seconds(), 1.0)

		// Once responded, the first-response clock no longer applies.
		responded := now.Add(-10 * time.Minute)
		u2 := SLAUrgency(tk, SLAMetrics{ActiveTime: 0, FirstResponseTimestamp: &responded}, now)
		require.InDelta(t, ResolutionThreshold.Seconds(), u2.Seconds(), 1.0)
	})

	t.Run("resolution clock uses canonical active time", func(t *testing.T) {
		tk := &Ticket{ID: uuid.New(), Status: StatusInProgress, CreatedAt: now.Add(-10 * time.Hour)}
		responded := now.Add(-9 * time.Hour)
		// Active time 20h -> remaining resolution 4h; first response satisfied.
		u := SLAUrgency(tk, SLAMetrics{ActiveTime: 20 * time.Hour, FirstResponseTimestamp: &responded}, now)
		require.InDelta(t, (4 * time.Hour).Seconds(), u.Seconds(), 1.0)
	})
}
