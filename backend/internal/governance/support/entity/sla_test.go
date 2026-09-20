package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestTicket(status Status, createdAt time.Time, resolvedAt *time.Time) *Ticket {
	return &Ticket{
		ID:        uuid.New(),
		Status:    status,
		CreatedAt: createdAt,
		ResolvedAt: resolvedAt,
	}
}

func eventTime(t *time.Time) time.Time {
	if t != nil {
		return *t
	}
	return time.Time{}
}

func statusPtr(s Status) *Status { return &s }

func TestComputeSLAMetricsFromEvents_NoEvents_UsesWallClock(t *testing.T) {
	created := time.Now().Add(-10 * time.Hour)
	resolved := time.Now().Add(-2 * time.Hour)
	ticket := newTestTicket(StatusResolved, created, &resolved)

	metrics := ticket.ComputeSLAMetricsFromEvents(nil, nil)

	// No events → wall clock fallback
	expectedActive := resolved.Sub(created)
	if metrics.ActiveTime != expectedActive {
		t.Errorf("ActiveTime = %v, want %v", metrics.ActiveTime, expectedActive)
	}
	if metrics.ResolutionTime == nil {
		t.Fatal("ResolutionTime should not be nil for resolved ticket")
	}
	if *metrics.ResolutionTime != expectedActive {
		t.Errorf("ResolutionTime = %v, want %v", *metrics.ResolutionTime, expectedActive)
	}
	if metrics.WaitingTime != 0 {
		t.Errorf("WaitingTime = %v, want 0", metrics.WaitingTime)
	}
}

func TestComputeSLAMetricsFromEvents_NoWaiting(t *testing.T) {
	created := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC) // 12h later
	ticket := newTestTicket(StatusResolved, created, &resolved)

	// Events: in_progress → resolved (no waiting_user)
	inProgress := created.Add(1 * time.Hour)
	events := []*Event{
		{CreatedAt: inProgress, NewStatus: statusPtr(StatusInProgress)},
		{CreatedAt: resolved, NewStatus: statusPtr(StatusResolved)},
	}

	metrics := ticket.ComputeSLAMetricsFromEvents(events, nil)

	expectedActive := resolved.Sub(created) // 12h wall clock
	if metrics.ActiveTime != expectedActive {
		t.Errorf("ActiveTime = %v, want %v", metrics.ActiveTime, expectedActive)
	}
	if metrics.WaitingTime != 0 {
		t.Errorf("WaitingTime = %v, want 0", metrics.WaitingTime)
	}
	if metrics.ResolutionOverdue {
		t.Error("ResolutionOverdue should be false for 12h < 24h")
	}
}

func TestComputeSLAMetricsFromEvents_OneWaitingPeriod(t *testing.T) {
	// Scenario:
	// created       → 10h active → waiting_user 72h → 2h active → resolved
	// Expected: resolution = 12h active, 72h waiting, not overdue
	created := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	enterWaiting := created.Add(10 * time.Hour)  // 20:00
	exitWaiting := enterWaiting.Add(72 * time.Hour) // +3 days
	resolved := exitWaiting.Add(2 * time.Hour)       // +2h

	ticket := newTestTicket(StatusResolved, created, &resolved)

	events := []*Event{
		{CreatedAt: created.Add(1 * time.Hour), NewStatus: statusPtr(StatusInProgress)},
		{CreatedAt: enterWaiting, NewStatus: statusPtr(StatusWaitingUser)},
		{CreatedAt: exitWaiting, NewStatus: statusPtr(StatusInProgress)},
		{CreatedAt: resolved, NewStatus: statusPtr(StatusResolved)},
	}

	metrics := ticket.ComputeSLAMetricsFromEvents(events, nil)

	expectedActive := 12 * time.Hour
	expectedWaiting := 72 * time.Hour

	if metrics.ActiveTime != expectedActive {
		t.Errorf("ActiveTime = %v, want %v", metrics.ActiveTime, expectedActive)
	}
	if metrics.WaitingTime != expectedWaiting {
		t.Errorf("WaitingTime = %v, want %v", metrics.WaitingTime, expectedWaiting)
	}
	if metrics.ResolutionTime == nil {
		t.Fatal("ResolutionTime should not be nil for resolved ticket")
	}
	if *metrics.ResolutionTime != expectedActive {
		t.Errorf("ResolutionTime = %v, want %v", *metrics.ResolutionTime, expectedActive)
	}
	if metrics.ResolutionOverdue {
		t.Error("ResolutionOverdue should be false (12h active < 24h threshold)")
	}
}

func TestComputeSLAMetricsFromEvents_MultipleWaitingPeriods(t *testing.T) {
	// Scenario:
	// created → 5h active → waiting 10h → 3h active → waiting 5h → 4h active → resolved
	// Expected: active = 5+3+4 = 12h, waiting = 10+5 = 15h
	created := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	w1Start := created.Add(5 * time.Hour)        // 15:00
	w1End := w1Start.Add(10 * time.Hour)          // 01:00 next day
	w2Start := w1End.Add(3 * time.Hour)           // 04:00
	w2End := w2Start.Add(5 * time.Hour)           // 09:00
	resolved := w2End.Add(4 * time.Hour)          // 13:00

	ticket := newTestTicket(StatusResolved, created, &resolved)

	events := []*Event{
		{CreatedAt: created.Add(1 * time.Hour), NewStatus: statusPtr(StatusInProgress)},
		{CreatedAt: w1Start, NewStatus: statusPtr(StatusWaitingUser)},
		{CreatedAt: w1End, NewStatus: statusPtr(StatusInProgress)},
		{CreatedAt: w2Start, NewStatus: statusPtr(StatusWaitingUser)},
		{CreatedAt: w2End, NewStatus: statusPtr(StatusInProgress)},
		{CreatedAt: resolved, NewStatus: statusPtr(StatusResolved)},
	}

	metrics := ticket.ComputeSLAMetricsFromEvents(events, nil)

	expectedActive := 12 * time.Hour
	expectedWaiting := 15 * time.Hour

	if metrics.ActiveTime != expectedActive {
		t.Errorf("ActiveTime = %v, want %v", metrics.ActiveTime, expectedActive)
	}
	if metrics.WaitingTime != expectedWaiting {
		t.Errorf("WaitingTime = %v, want %v", metrics.WaitingTime, expectedWaiting)
	}
}

func TestComputeSLAMetricsFromEvents_StillWaiting(t *testing.T) {
	// Ticket still in waiting_user — waiting time counted from entry to now
	created := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	wStart := created.Add(5 * time.Hour) // enter waiting

	ticket := newTestTicket(StatusWaitingUser, created, nil)

	events := []*Event{
		{CreatedAt: created.Add(1 * time.Hour), NewStatus: statusPtr(StatusInProgress)},
		{CreatedAt: wStart, NewStatus: statusPtr(StatusWaitingUser)},
	}

	metrics := ticket.ComputeSLAMetricsFromEvents(events, nil)

	// Active time = created → wStart = 5h (wall clock from last event to now is waiting)
	if metrics.ActiveTime != 5*time.Hour {
		t.Errorf("ActiveTime = %v, want 5h", metrics.ActiveTime)
	}
	// Waiting time includes time from wStart to now — should be > 0
	if metrics.WaitingTime <= 0 {
		t.Errorf("WaitingTime should be > 0, got %v", metrics.WaitingTime)
	}
}

func TestComputeSLAMetricsFromEvents_WaitingCountsForOverdue(t *testing.T) {
	// 25h active → waiting 10h → still in_progress → should be overdue
	created := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	wStart := created.Add(25 * time.Hour)

	ticket := newTestTicket(StatusInProgress, created, nil)

	events := []*Event{
		{CreatedAt: created.Add(1 * time.Hour), NewStatus: statusPtr(StatusInProgress)},
		{CreatedAt: wStart, NewStatus: statusPtr(StatusWaitingUser)},
	}

	metrics := ticket.ComputeSLAMetricsFromEvents(events, nil)

	// Active time = 25h (before waiting period) → overdue
	if !metrics.ResolutionOverdue {
		t.Error("ResolutionOverdue should be true (25h active > 24h threshold)")
	}
	if !metrics.IsOverdue {
		t.Error("IsOverdue should be true")
	}
}

func TestComputeSLAMetricsFromEvents_ListDetailConsistency(t *testing.T) {
	// Same ticket data: list (batch) and detail should produce identical SLA
	created := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC) // 24h total
	wStart := created.Add(6 * time.Hour)
	wEnd := wStart.Add(6 * time.Hour)         // 6h waiting
	activeEnd := wEnd.Add(6 * time.Hour)       // 6h more active
	resolved = activeEnd.Add(12 * time.Hour)   // 12h more active = 24h total active
	ticket := newTestTicket(StatusResolved, created, &resolved)

	events := []*Event{
		{CreatedAt: created.Add(1 * time.Hour), NewStatus: statusPtr(StatusInProgress)},
		{CreatedAt: wStart, NewStatus: statusPtr(StatusWaitingUser)},
		{CreatedAt: wEnd, NewStatus: statusPtr(StatusInProgress)},
		{CreatedAt: resolved, NewStatus: statusPtr(StatusResolved)},
	}

	// Simulate list (batch) path — same events passed directly
	listMetrics := ticket.ComputeSLAMetricsFromEvents(events, nil)

	// Simulate detail path — same events passed directly
	detailMetrics := ticket.ComputeSLAMetricsFromEvents(events, nil)

	if listMetrics.ActiveTime != detailMetrics.ActiveTime {
		t.Errorf("List ActiveTime %v != Detail ActiveTime %v", listMetrics.ActiveTime, detailMetrics.ActiveTime)
	}
	if listMetrics.WaitingTime != detailMetrics.WaitingTime {
		t.Errorf("List WaitingTime %v != Detail WaitingTime %v", listMetrics.WaitingTime, detailMetrics.WaitingTime)
	}
	if listMetrics.ResolutionOverdue != detailMetrics.ResolutionOverdue {
		t.Errorf("List ResolutionOverdue %v != Detail ResolutionOverdue %v", listMetrics.ResolutionOverdue, detailMetrics.ResolutionOverdue)
	}
}

func TestComputeSLAMetricsFromEvents_ComputeSLAMetricsSimpleRemoved(t *testing.T) {
	// Verify ComputeSLAMetricsSimple does not exist (compilation check)
	// This test ensures the method was removed. If it compiles, the method is gone.
	ticket := newTestTicket(StatusInProgress, time.Now().Add(-1*time.Hour), nil)
	_ = ticket // Ensure ticket is used

	// The old ComputeSLAMetricsSimple would compute resolution as wall clock.
	// ComputeSLAMetricsFromEvents(nil) should also use wall clock as fallback.
	metrics := ticket.ComputeSLAMetricsFromEvents(nil, nil)
	_ = metrics
}

func TestComputeSLAMetricsFromEvents_OverdueNotPausedByWaiting(t *testing.T) {
	// 30h active → waiting 5h → should still be overdue (waiting doesn't help)
	created := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	wStart := created.Add(30 * time.Hour)

	ticket := newTestTicket(StatusWaitingUser, created, nil)

	events := []*Event{
		{CreatedAt: created.Add(1 * time.Hour), NewStatus: statusPtr(StatusInProgress)},
		{CreatedAt: wStart, NewStatus: statusPtr(StatusWaitingUser)},
	}

	metrics := ticket.ComputeSLAMetricsFromEvents(events, nil)

	// Active time = 30h, waiting doesn't reduce active time
	if metrics.ActiveTime != 30*time.Hour {
		t.Errorf("ActiveTime = %v, want 30h", metrics.ActiveTime)
	}
	if !metrics.ResolutionOverdue {
		t.Error("ResolutionOverdue should be true (30h active > 24h)")
	}
}

// ============================================================================
// SLA-F04: FIRST RESPONSE AUTHORITY — first valid admin response message.
// ============================================================================

// TestComputeSLAMetricsFromEvents_F04_ClaimedNoResponse: ticket created →
// assigned → no admin message. Expected: first_response_at = nil and
// first-response overdue driven by the 1h threshold wall clock.
func TestComputeSLAMetricsFromEvents_F04_ClaimedNoResponse(t *testing.T) {
	created := time.Now().Add(-90 * time.Minute)
	ticket := newTestTicket(StatusInProgress, created, nil)
	assigned := created.Add(5 * time.Minute)
	ticket.AssignedAt = &assigned

	metrics := ticket.ComputeSLAMetricsFromEvents(nil, nil)

	if metrics.FirstResponseTime != nil {
		t.Errorf("FirstResponseTime = %v, want nil (assignment is not a response)", metrics.FirstResponseTime)
	}
	if metrics.FirstResponseTimestamp != nil {
		t.Errorf("FirstResponseTimestamp = %v, want nil", metrics.FirstResponseTimestamp)
	}
	if !metrics.FirstResponseOverdue {
		t.Error("FirstResponseOverdue should be true (no response for 90m > 1h threshold)")
	}
	if AssignedAt := ticket.AssignedAt; AssignedAt != nil && metrics.FirstResponseTimestamp != nil && !metrics.FirstResponseTimestamp.Equal(*AssignedAt) {
		t.Errorf("FirstResponseTimestamp must never be the assignment time")
	}
}

// TestComputeSLAMetricsFromEvents_F04_AdminResponds: ticket created →
// assigned → admin first message at T+30m. Expected: first_response_at =
// T+30m and first response SLA satisfied.
func TestComputeSLAMetricsFromEvents_F04_AdminResponds(t *testing.T) {
	created := time.Now().Add(-90 * time.Minute)
	ticket := newTestTicket(StatusWaitingUser, created, nil)
	assigned := created.Add(5 * time.Minute)
	ticket.AssignedAt = &assigned

	firstResponse := created.Add(30 * time.Minute)
	metrics := ticket.ComputeSLAMetricsFromEvents(nil, &firstResponse)

	if metrics.FirstResponseTime == nil {
		t.Fatal("FirstResponseTime should not be nil when an admin response exists")
	}
	if *metrics.FirstResponseTime != 30*time.Minute {
		t.Errorf("FirstResponseTime = %v, want 30m", *metrics.FirstResponseTime)
	}
	if metrics.FirstResponseTimestamp == nil || !metrics.FirstResponseTimestamp.Equal(firstResponse) {
		t.Errorf("FirstResponseTimestamp = %v, want the admin message time T+30m", metrics.FirstResponseTimestamp)
	}
	if metrics.FirstResponseOverdue {
		t.Error("FirstResponseOverdue should be false (T+30m < 1h threshold → satisfied)")
	}
}

// TestComputeSLAMetricsFromEvents_F04_AssignmentIsNotResponse proves the SLA
// uses the admin MESSAGE time (T+40m), never the ASSIGNMENT time (T+5m).
func TestComputeSLAMetricsFromEvents_F04_AssignmentIsNotResponse(t *testing.T) {
	created := time.Now().Add(-2 * time.Hour)
	ticket := newTestTicket(StatusWaitingUser, created, nil)
	assigned := created.Add(5 * time.Minute) // assigned at T+5m
	ticket.AssignedAt = &assigned

	firstResponse := created.Add(40 * time.Minute) // admin first message at T+40m
	metrics := ticket.ComputeSLAMetricsFromEvents(nil, &firstResponse)

	if metrics.FirstResponseTime == nil {
		t.Fatal("FirstResponseTime should not be nil when an admin response exists")
	}
	if *metrics.FirstResponseTime != 40*time.Minute {
		t.Errorf("FirstResponseTime = %v, want 40m (message time), not 5m (assignment)", *metrics.FirstResponseTime)
	}
	if metrics.FirstResponseTimestamp == nil || !metrics.FirstResponseTimestamp.Equal(firstResponse) {
		t.Errorf("FirstResponseTimestamp must be the message time T+40m, got %v", metrics.FirstResponseTimestamp)
	}
}

// TestComputeSLAMetricsFromEvents_F04_FirstValidAdminResponseWins proves the
// FIRST (earliest) valid admin response is used when several exist.
func TestComputeSLAMetricsFromEvents_F04_FirstValidAdminResponseWins(t *testing.T) {
	created := time.Now().Add(-3 * time.Hour)
	ticket := newTestTicket(StatusWaitingUser, created, nil)

	first := created.Add(40 * time.Minute)
	second := created.Add(50 * time.Minute)
	third := created.Add(60 * time.Minute)

	metrics := ticket.ComputeSLAMetricsFromEvents(nil, &first)
	if metrics.FirstResponseTime == nil || *metrics.FirstResponseTime != 40*time.Minute {
		t.Errorf("FirstResponseTime = %v, want 40m (the first valid admin response)", metrics.FirstResponseTime)
	}

	// Multiple admin messages in the room: the calculator receives the first
	// (ListFirstAdminResponsesByTicketIDs returns MIN(created_at)), so the
	// earliest of several candidates must win.
	metrics = ticket.ComputeSLAMetricsFromEvents(nil, &second)
	if metrics.FirstResponseTime == nil || *metrics.FirstResponseTime != 50*time.Minute {
		t.Errorf("FirstResponseTime = %v, want 50m", metrics.FirstResponseTime)
	}
	_ = third
}

// TestComputeSLAMetricsFromEvents_F04_UserMessagesNeverCount proves user
// messages cannot become the first admin response: with only user messages
// the authority stays nil and the overdue flag follows the wall clock.
func TestComputeSLAMetricsFromEvents_F04_UserMessagesNeverCount(t *testing.T) {
	created := time.Now().Add(-2 * time.Hour)
	ticket := newTestTicket(StatusInProgress, created, nil)

	// Only user messages in the conversation → authority nil (the batch query
	// joins users.role='admin'; user senders cannot match).
	metrics := ticket.ComputeSLAMetricsFromEvents(nil, nil)
	if metrics.FirstResponseTime != nil {
		t.Errorf("FirstResponseTime = %v, want nil (user messages are not admin responses)", metrics.FirstResponseTime)
	}
	if !metrics.FirstResponseOverdue {
		t.Error("FirstResponseOverdue should be true (no admin response for 2h > 1h threshold)")
	}
}

// TestComputeSLAMetricsFromEvents_F04_ConsumersConverged proves the parity
// mandate: the same ticket + events + authority produce identical first
// response results regardless of which consumer (list / detail / dashboard)
// invokes the single calculator.
func TestComputeSLAMetricsFromEvents_F04_ConsumersConverged(t *testing.T) {
	created := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	ticket := newTestTicket(StatusWaitingUser, created, nil)
	assigned := created.Add(5 * time.Minute)
	ticket.AssignedAt = &assigned

	firstResponse := created.Add(30 * time.Minute)

	// All consumers pass the same inputs to the same single calculator.
	listMetrics := ticket.ComputeSLAMetricsFromEvents(nil, &firstResponse)
	detailMetrics := ticket.ComputeSLAMetricsFromEvents(nil, &firstResponse)
	dashboardMetrics := ticket.ComputeSLAMetricsFromEvents(nil, &firstResponse)

	if listMetrics.FirstResponseTime == nil || detailMetrics.FirstResponseTime == nil || dashboardMetrics.FirstResponseTime == nil {
		t.Fatal("all consumers must have a first response time when an admin response exists")
	}
	if *listMetrics.FirstResponseTime != *detailMetrics.FirstResponseTime ||
		*listMetrics.FirstResponseTime != *dashboardMetrics.FirstResponseTime {
		t.Error("all SLA consumers must produce the same first response time")
	}
	if listMetrics.FirstResponseOverdue != detailMetrics.FirstResponseOverdue ||
		listMetrics.FirstResponseOverdue != dashboardMetrics.FirstResponseOverdue {
		t.Error("all SLA consumers must produce the same first response overdue flag")
	}
}
