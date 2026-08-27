//go:build integration

// This is the direct regression test for the admin Support Overview and
// Support Tickets page HTTP 500s reported in PASS_20F. Every read-only
// Service method here used to call its repository with a literal `nil` for
// the transaction argument (GetTicket, GetMyOpenTicket, ListTickets,
// CountTickets, GetAdmin, ListAdmins, GetAvailableAdmins, GetStatistics,
// ListEvents), but SupportRepositoryImpl's toTx() helper does a type
// assertion `tx.(db.Tx)` that panics on a nil interface — so every one of
// these calls panicked on every request, not just on empty data. Fixed by
// opening a real transaction via the Service's own Transactor (s.db.WithTx)
// instead of passing nil straight through.
package application

import (
	"context"
	"testing"

	infraRepo "github.com/labuda/backend/internal/governance/support/infrastructure/repository"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

func setupSupportServiceTest(t *testing.T) (*Service, func()) {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	svc := NewService(tdb, infraRepo.NewSupportRepository(), nil, nil, nil, nil, zap.NewNop())
	return svc, cleanup
}

func TestService_GetStatistics_EmptyDBDoesNotPanic(t *testing.T) {
	svc, cleanup := setupSupportServiceTest(t)
	defer cleanup()

	stats, err := svc.GetStatistics(context.Background())
	if err != nil {
		t.Fatalf("GetStatistics: %v (nil-tx panic regression if this mentions 'invalid transaction type')", err)
	}
	if stats.TotalTickets != 0 {
		t.Fatalf("expected 0 total tickets on empty DB, got %d", stats.TotalTickets)
	}
}

func TestService_ListTickets_EmptyDBDoesNotPanic(t *testing.T) {
	svc, cleanup := setupSupportServiceTest(t)
	defer cleanup()

	tickets, err := svc.ListTickets(context.Background(), &supportRepo.TicketFilter{}, nil, nil, 20)
	if err != nil {
		t.Fatalf("ListTickets: %v (nil-tx panic regression if this mentions 'invalid transaction type')", err)
	}
	if len(tickets) != 0 {
		t.Fatalf("expected 0 tickets on empty DB, got %d", len(tickets))
	}
}

func TestService_CountTickets_EmptyDBDoesNotPanic(t *testing.T) {
	svc, cleanup := setupSupportServiceTest(t)
	defer cleanup()

	count, err := svc.CountTickets(context.Background(), &supportRepo.TicketFilter{})
	if err != nil {
		t.Fatalf("CountTickets: %v (nil-tx panic regression if this mentions 'invalid transaction type')", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0 on empty DB, got %d", count)
	}
}

func TestService_ListAdmins_EmptyDBDoesNotPanic(t *testing.T) {
	svc, cleanup := setupSupportServiceTest(t)
	defer cleanup()

	admins, err := svc.ListAdmins(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListAdmins: %v (nil-tx panic regression if this mentions 'invalid transaction type')", err)
	}
	if len(admins) != 0 {
		t.Fatalf("expected 0 admins on empty DB, got %d", len(admins))
	}
}

func TestService_GetAvailableAdmins_EmptyDBDoesNotPanic(t *testing.T) {
	svc, cleanup := setupSupportServiceTest(t)
	defer cleanup()

	admins, err := svc.GetAvailableAdmins(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetAvailableAdmins: %v (nil-tx panic regression if this mentions 'invalid transaction type')", err)
	}
	if len(admins) != 0 {
		t.Fatalf("expected 0 available admins on empty DB, got %d", len(admins))
	}
}
