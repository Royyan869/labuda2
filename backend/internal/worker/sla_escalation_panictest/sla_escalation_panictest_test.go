package slaescalationpanictest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	"github.com/labuda/backend/internal/worker"
	"github.com/labuda/backend/pkg/db"
)

// =============================================================================
// SLA ESCALATION WORKER — PANIC HARDENING REGRESSION TESTS
// =============================================================================
//
// Before the typed-row migration, the worker consumed []map[string]interface{}
// rows produced by the repository. The repository scanned IDs into uuid.UUID
// and nullable timestamps into *time.Time, then stuffed them into untyped
// maps. The worker reached back in with naked type assertions:
//
//   ticket["id"].(string)             // panic: uuid.UUID is not string
//   ticket["assigned_at"].(time.Time) // panic: *time.Time is not time.Time
//   dispute["id"].(string)            // panic on line 426: uuid.UUID is not string
//
// These tests pin the new typed contract so that panic class cannot regress.
// =============================================================================

// --- minimal transactor / row plumbing -------------------------------------

type fakeTx struct{}

func (fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return fakeRow{}
}
func (fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("1"), nil
}
func (fakeTx) Commit(_ context.Context) error   { return nil }
func (fakeTx) Rollback(_ context.Context) error { return nil }

// fakeRow.Scan returns (zero, nil) so eventExists() observes exists=false
// and the worker proceeds to emit events under test.
type fakeRow struct{}

func (fakeRow) Scan(dest ...any) error {
	for _, d := range dest {
		if b, ok := d.(*bool); ok {
			*b = false
		}
	}
	return nil
}

type fakeDB struct{}

func (fakeDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(fakeTx{})
}

// --- repo + outbox stubs ---------------------------------------------------

type fakeRepo struct {
	tickets    []supportRepo.TicketSLARow
	disputes   []supportRepo.DisputeSLARow
	ticketErr  error
	disputeErr error
}

func (r *fakeRepo) FindTicketsForSLACheck(_ context.Context, _ db.Tx, _ int) ([]supportRepo.TicketSLARow, error) {
	return r.tickets, r.ticketErr
}
func (r *fakeRepo) FindDisputesForSLACheck(_ context.Context, _ db.Tx, _ int) ([]supportRepo.DisputeSLARow, error) {
	return r.disputes, r.disputeErr
}

type recordedEvent struct {
	eventType      string
	idempotencyKey string
}

type fakeOutbox struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (o *fakeOutbox) InsertTx(_ context.Context, _ db.Tx, eventType string, _ any, idempotencyKey string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, recordedEvent{eventType, idempotencyKey})
	return nil
}
func (o *fakeOutbox) snapshot() []recordedEvent {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]recordedEvent, len(o.events))
	copy(out, o.events)
	return out
}

func newRig(repo worker.SLAEscalationRepository) (*worker.SLAEscalationWorker, *fakeOutbox) {
	out := &fakeOutbox{}
	w := worker.NewSLAEscalationWorker(fakeDB{}, repo, out, zap.NewNop())
	return w, out
}

// --- regression tests ------------------------------------------------------

// TestSLAWorker_TypedRows_NoPanicOnUUIDColumn pins the fix for the original
// crash at sla_escalation_worker.go:426 — formerly
//
//	uuid.Parse(dispute["id"].(string))
//
// which panicked because the underlying value was uuid.UUID, not string.
// With the typed contract the ID is a uuid.UUID field; no assertion needed.
func TestSLAWorker_TypedRows_NoPanicOnUUIDColumn(t *testing.T) {
	repo := &fakeRepo{
		disputes: []supportRepo.DisputeSLARow{
			{
				ID:         uuid.New(),
				OrderID:    uuid.New(),
				BuyerID:    uuid.New(),
				SellerID:   uuid.New(),
				Status:     "under_review",
				OpenedAt:   time.Now().Add(-3 * time.Hour), // past breach threshold
				ResolvedAt: nil,
			},
		},
	}
	w, out := newRig(repo)

	if err := w.ProcessDisputesSLA(context.Background()); err != nil {
		t.Fatalf("ProcessDisputesSLA returned error: %v", err)
	}
	if len(out.snapshot()) == 0 {
		t.Fatalf("expected at least one SLA event for a 3h-old dispute, got 0")
	}
}

// TestSLAWorker_TicketWithNilTimestamps verifies the worker handles tickets
// whose assigned_at/resolved_at are NULL. Pre-fix the worker did
// assignedAt.(time.Time) against an interface holding a typed-nil
// *time.Time, which would panic regardless of the `assignedAt != nil` guard
// (because an interface wrapping a typed-nil pointer is not the nil
// interface).
func TestSLAWorker_TicketWithNilTimestamps(t *testing.T) {
	repo := &fakeRepo{
		tickets: []supportRepo.TicketSLARow{
			{
				ID:         uuid.New(),
				UserID:     uuid.New(),
				Status:     "open",
				CreatedAt:  time.Now().Add(-90 * time.Minute), // past breach threshold
				AssignedAt: nil,                               // never responded
				ResolvedAt: nil,                               // never resolved
			},
		},
	}
	w, out := newRig(repo)

	if err := w.ProcessTicketsSLA(context.Background()); err != nil {
		t.Fatalf("ProcessTicketsSLA returned error: %v", err)
	}
	if len(out.snapshot()) == 0 {
		t.Fatalf("expected first-response breach event for unassigned 90min-old ticket")
	}
}

// TestSLAWorker_TicketWithPopulatedTimestamps verifies the worker treats a
// ticket as having a first response when assigned_at is set and as resolved
// when resolved_at is set — using actual *time.Time pointers.
func TestSLAWorker_TicketWithPopulatedTimestamps(t *testing.T) {
	assigned := time.Now().Add(-30 * time.Minute)
	resolved := time.Now().Add(-1 * time.Minute)

	repo := &fakeRepo{
		tickets: []supportRepo.TicketSLARow{
			{
				ID:         uuid.New(),
				UserID:     uuid.New(),
				Status:     "resolved",
				CreatedAt:  time.Now().Add(-2 * time.Hour),
				AssignedAt: &assigned,
				ResolvedAt: &resolved,
			},
		},
	}
	w, out := newRig(repo)

	if err := w.ProcessTicketsSLA(context.Background()); err != nil {
		t.Fatalf("ProcessTicketsSLA returned error: %v", err)
	}
	if got := len(out.snapshot()); got != 0 {
		t.Fatalf("expected 0 events for already-resolved ticket, got %d", got)
	}
}

// TestSLAWorker_NilUUIDGuard ensures that if a malformed row arrives with a
// zero UUID — e.g. a scan-drift bug upstream — the worker bails out
// deterministically with a per-row error instead of emitting events keyed
// to uuid.Nil. ProcessXxxSLA swallows per-row errors and continues, so the
// public-facing invariant is: no panic, no events emitted.
func TestSLAWorker_NilUUIDGuard(t *testing.T) {
	repo := &fakeRepo{
		disputes: []supportRepo.DisputeSLARow{
			{
				ID:         uuid.Nil, // forensic invariant violation
				OrderID:    uuid.New(),
				BuyerID:    uuid.New(),
				SellerID:   uuid.New(),
				Status:     "under_review",
				OpenedAt:   time.Now().Add(-3 * time.Hour),
				ResolvedAt: nil,
			},
		},
		tickets: []supportRepo.TicketSLARow{
			{
				ID:         uuid.Nil, // forensic invariant violation
				UserID:     uuid.New(),
				Status:     "open",
				CreatedAt:  time.Now().Add(-90 * time.Minute),
				AssignedAt: nil,
				ResolvedAt: nil,
			},
		},
	}
	w, out := newRig(repo)

	if err := w.ProcessDisputesSLA(context.Background()); err != nil {
		t.Fatalf("ProcessDisputesSLA must not surface per-row error: %v", err)
	}
	if err := w.ProcessTicketsSLA(context.Background()); err != nil {
		t.Fatalf("ProcessTicketsSLA must not surface per-row error: %v", err)
	}
	if got := len(out.snapshot()); got != 0 {
		t.Fatalf("expected 0 events for nil-UUID rows, got %d", got)
	}
}

// TestSLAWorker_EmptyResultSet ensures the worker is a no-op when the
// repository returns no rows.
func TestSLAWorker_EmptyResultSet(t *testing.T) {
	w, out := newRig(&fakeRepo{})

	if err := w.ProcessTicketsSLA(context.Background()); err != nil {
		t.Fatalf("ProcessTicketsSLA error: %v", err)
	}
	if err := w.ProcessDisputesSLA(context.Background()); err != nil {
		t.Fatalf("ProcessDisputesSLA error: %v", err)
	}
	if got := len(out.snapshot()); got != 0 {
		t.Fatalf("expected no events on empty result set, got %d", got)
	}
}


