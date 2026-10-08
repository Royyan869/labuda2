package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/governance/support/entity"
)

func orderedIDs(tickets []*entity.Ticket) []uuid.UUID {
	ids := make([]uuid.UUID, len(tickets))
	for i, t := range tickets {
		ids[i] = t.ID
	}
	return ids
}

// TestService_ListTicketsOrdered_GlobalOrder proves the admin queue ordering is
// computed on the SERVER over the full filtered set, so a page is a slice of
// one global canonical order. The frontend must never re-sort.
func TestService_ListTicketsOrdered_GlobalOrder(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	service := &Service{repo: repo, db: &mockTransactor{}, log: zap.NewNop()}

	now := time.Now()
	add := func(priority entity.Priority, createdAt time.Time) *entity.Ticket {
		tk := entity.NewTicket(uuid.New(), uuid.New(), entity.CategoryOther, priority)
		tk.CreatedAt = createdAt
		tk.Status = entity.StatusOpen
		repo.tickets[tk.ID] = tk
		return tk
	}

	// Unanswered old ticket -> first response overdue.
	overdue := add(entity.PriorityLow, now.Add(-2*time.Hour))
	// Answered recent ticket -> urgency ~ remaining resolution only.
	soon := add(entity.PriorityLow, now.Add(-30*time.Minute))
	later := add(entity.PriorityUrgent, now.Add(-5*time.Minute))

	// Mark soon/later as already responded so urgency is driven by active time;
	// leave overdue unanswered so it is SLA-overdue.
	// (The mock returns no first admin responses by default, so only 'overdue'
	// is past the 1h first-response threshold.)

	page, total, err := service.ListTicketsOrdered(ctx, nil, 10, 0)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	// overdue (SLA breached) first; soon (remaining ~30m) before later (~55m)
	// even though later is priority=urgent.
	require.Equal(t, []uuid.UUID{overdue.ID, soon.ID, later.ID}, orderedIDs(page))

	// Pagination boundary preserves the global order: page 2 of size 1 is 'soon'.
	page2, total2, err := service.ListTicketsOrdered(ctx, nil, 1, 1)
	require.NoError(t, err)
	require.Equal(t, int64(3), total2)
	require.Equal(t, []uuid.UUID{soon.ID}, orderedIDs(page2))

	// Empty page beyond the end is empty (truthful total unchanged).
	page3, total3, err := service.ListTicketsOrdered(ctx, nil, 5, 99)
	require.NoError(t, err)
	require.Equal(t, int64(3), total3)
	require.Empty(t, page3)
}

// TestService_ListTicketsOrdered_Empty ensures an empty filtered set yields an
// empty page and a truthful zero total.
func TestService_ListTicketsOrdered_Empty(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	service := &Service{repo: repo, db: &mockTransactor{}, log: zap.NewNop()}

	page, total, err := service.ListTicketsOrdered(ctx, nil, 20, 0)
	require.NoError(t, err)
	require.Empty(t, page)
	require.Equal(t, int64(0), total)
}
