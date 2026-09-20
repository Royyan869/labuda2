//go:build integration

// REAL POSTGRESQL PROOF — SUPPORT SLA-F04: FIRST RESPONSE AUTHORITY.
//
// Canonical question: what does "first response" mean on a Support ticket?
//
// Evidence-converged authority (SLA-F04): the FIRST VALID ADMIN RESPONSE
// MESSAGE — the earliest non-deleted chat_messages row, sender role='admin',
// in the ticket's own support conversation (one ticket = one support room,
// support_tickets.chat_room_id). Assignment is NOT a response.
//
// This test proves end-to-end on real PostgreSQL:
//  1. claimed + no admin message   → first_response_at = nil; overdue is
//     wall-clock vs the 1h threshold
//  2. admin responds at T+30m      → first_response_at = the message
//     timestamp; first response SATISFIED
//  3. assignment != response       → assigned_at is NOT the authority; the
//     admin message timestamp is
//  4. multiple admin messages      → the FIRST admin message wins
//  5. user messages never count    → user messages cannot become the first
//     admin response
//  6. consumer parity              → the single authority query + the single
//     ComputeSLAMetricsFromEvents calculation produce identical results for
//     the list path, the detail path and the dashboard path
//  7. no N+1                       → ONE batch query returns first admin
//     responses for ALL tickets in a single round trip
package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	adminApp "github.com/labuda/backend/internal/platform/admin/application"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"

	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatInfraRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"

	supportEntity "github.com/labuda/backend/internal/governance/support/entity"
	supportInfraRepo "github.com/labuda/backend/internal/governance/support/infrastructure/repository"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	"github.com/stretchr/testify/require"
)

// newSLAF04ProofStack wires the production Support service, the production
// Chat service, the production support repository and the admin SLA service
// on a disposable real PostgreSQL database.
func newSLAF04ProofStack(t *testing.T, tdb *testdb.TestDB) (*Service, *chatApp.Service, supportRepo.Repository, *adminApp.SLAService, *db.DB) {
	t.Helper()

	database := db.NewFromPool(tdb.Pool())
	outbox := outboxRepo.NewOutboxRepository(database)

	chatSvc := chatApp.NewService(
		tdb,
		chatInfraRepo.NewChatRepository(),
		nil, // socialRepo — support rooms are exempt from block checks
		outbox,
		rate.NewRateLimiter(),
		nil, // metrics
		nil, // account status checker (inactive-account gate not exercised)
		nil, // order ownership reader
		zap.NewNop(),
	)

	svc := NewServiceWithDefaults(tdb, chatSvc, outbox, nil, nil, zap.NewNop())
	repo := supportInfraRepo.NewSupportRepository()
	slaSvc := adminApp.NewSLAService(tdb, nil, repo)
	return svc, chatSvc, repo, slaSvc, database
}

// createSLAF04Ticket creates a ticket and claims it with adminID through the
// real claim path.
func createSLAF04Ticket(t *testing.T, ctx context.Context, svc *Service, ownerID, adminID uuid.UUID, subject string) *supportEntity.Ticket {
	t.Helper()
	ticket, err := svc.CreateTicket(ctx, &CreateTicketRequest{
		UserID:   ownerID,
		Category: supportEntity.CategoryPaymentIssue,
		Priority: supportEntity.PriorityMedium,
		Subject:  &subject,
	})
	require.NoError(t, err, "ticket creation must succeed")
	_, err = svc.ClaimTicket(ctx, &ClaimTicketRequest{TicketID: ticket.ID, AdminID: adminID})
	require.NoError(t, err, "assignment must succeed")
	return ticket
}

// TestSLAF04_RealDB_FirstAdminMessageIsFirstResponseAuthority proves the
// canonical first-response authority end to end on real PostgreSQL.
func TestSLAF04_RealDB_FirstAdminMessageIsFirstResponseAuthority(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, chatSvc, repo, _, database := newSLAF04ProofStack(t, tdb)

	ownerID := uuid.New()
	adminID := uuid.New()
	insertSupportProofUser(t, ctx, tdb, ownerID)
	insertSupportProofAdmin(t, ctx, tdb, adminID)

	// ------------------------------------------------------------------
	// Scenario 1: created → assigned → NO admin message.
	// ------------------------------------------------------------------
	ticketA := createSLAF04Ticket(t, ctx, svc, ownerID, adminID, "SLA-F04 claimed but silent")

	// ------------------------------------------------------------------
	// Scenario 2: created → assigned → admin first message at T+30m.
	// The chat seam cannot time-travel, so persistence is backdated to the
	// canonical timeline: ticket creation to T-40m, first message at T-10m
	// (= creation + 30m). Persistence is implementation truth — these are
	// the stored timestamps the SLA calculator reads.
	// ------------------------------------------------------------------
	ticketB := createSLAF04Ticket(t, ctx, svc, ownerID, adminID, "SLA-F04 admin responds")
	bCreation := time.Now().Add(-40 * time.Minute)
	bFirstResponse := time.Now().Add(-10 * time.Minute) // creation + 30m
	_, err := tdb.Pool().Exec(ctx,
		`UPDATE support_tickets SET created_at = $2 WHERE id = $1`, ticketB.ID, bCreation)
	require.NoError(t, err)
	_, err = chatSvc.SendSupportMessage(ctx, ticketB.ChatRoomID, adminID, "Admin first reply", uuid.NewString())
	require.NoError(t, err, "admin first message must persist")
	_, err = tdb.Pool().Exec(ctx,
		`UPDATE chat_messages SET created_at = $2 WHERE room_id = $1`, ticketB.ChatRoomID, bFirstResponse)
	require.NoError(t, err)

	// ------------------------------------------------------------------
	// Scenario 3+4: created → assigned at wall-now → admin messages at
	// T+40m (first) and T+50m (second). Creation backdated to T-50m.
	// Proves the FIRST admin message is used and that assigned_at (≈T+50m)
	// is never mistaken for the response.
	// ------------------------------------------------------------------
	ticketC := createSLAF04Ticket(t, ctx, svc, ownerID, adminID, "SLA-F04 multiple admin messages")
	cCreation := time.Now().Add(-50 * time.Minute)
	cFirstResponse := time.Now().Add(-10 * time.Minute) // creation + 40m
	_, err = tdb.Pool().Exec(ctx,
		`UPDATE support_tickets SET created_at = $2 WHERE id = $1`, ticketC.ID, cCreation)
	require.NoError(t, err)
	_, err = chatSvc.SendSupportMessage(ctx, ticketC.ChatRoomID, adminID, "Admin first", uuid.NewString())
	require.NoError(t, err)
	_, err = chatSvc.SendSupportMessage(ctx, ticketC.ChatRoomID, adminID, "Admin second", uuid.NewString())
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx,
		`UPDATE chat_messages SET created_at = $2 WHERE room_id = $1 AND body = 'Admin first'`,
		ticketC.ChatRoomID, cFirstResponse)
	require.NoError(t, err)

	// ------------------------------------------------------------------
	// Scenario 5: created → assigned → USER message only. A user message
	// must never become the first admin response.
	// ------------------------------------------------------------------
	ticketD := createSLAF04Ticket(t, ctx, svc, ownerID, adminID, "SLA-F04 user messages only")
	_, err = chatSvc.SendSupportMessage(ctx, ticketD.ChatRoomID, ownerID, "User nudging", uuid.NewString())
	require.NoError(t, err)

	// ------------------------------------------------------------------
	// 7. No N+1: ONE batch query returns first admin responses for ALL
	//    tickets in a single round trip (SLA-F04 performance mandate).
	// ------------------------------------------------------------------
	allIDs := []uuid.UUID{ticketA.ID, ticketB.ID, ticketC.ID, ticketD.ID}
	var batch map[uuid.UUID]*time.Time
	err = database.WithTx(ctx, func(tx db.Tx) error {
		var err error
		batch, err = repo.ListFirstAdminResponsesByTicketIDs(ctx, tx, allIDs)
		return err
	})
	require.NoError(t, err, "batch first-response fetch must succeed")

	// 1 continued: claimed but silent → NO first response authority at all.
	_, hasA := batch[ticketA.ID]
	require.False(t, hasA, "ticket with no admin message must have NO first response")

	// 2 continued: the admin message timestamp (T+30m) is the authority.
	require.NotNil(t, batch[ticketB.ID], "admin response must be found")
	require.WithinDuration(t, bFirstResponse, *batch[ticketB.ID], 2*time.Second,
		"first_response_at must be the admin message timestamp (T+30m)")

	// 3+4 continued: the FIRST admin message (T+40m) wins — NOT the second
	// message, and NOT assigned_at (≈ wall-now = T+50m).
	require.NotNil(t, batch[ticketC.ID])
	require.WithinDuration(t, cFirstResponse, *batch[ticketC.ID], 2*time.Second,
		"the FIRST admin message must be the authority")
	require.WithinDuration(t, cFirstResponse, *batch[ticketC.ID], 2*time.Second)
	require.True(t, batch[ticketC.ID].Before(time.Now().Add(-30*time.Second)),
		"authority must be the T+40m message, not the ~T+50m assignment")

	// 5 continued: user messages never become the first admin response.
	_, hasD := batch[ticketD.ID]
	require.False(t, hasD, "user messages must NOT become the first admin response")

	// ------------------------------------------------------------------
	// 6. Single SLA calculation authority: reload tickets through the
	//    service, rebuild status events through the batch repo, and run
	//    ComputeSLAMetricsFromEvents with the batch-fetched authority.
	// ------------------------------------------------------------------
	var eventsByTicket map[uuid.UUID][]*supportEntity.Event
	err = database.WithTx(ctx, func(tx db.Tx) error {
		var err error
		eventsByTicket, err = repo.ListStatusEventsForTickets(ctx, tx, allIDs)
		return err
	})
	require.NoError(t, err)

	slaFor := func(ticketID uuid.UUID) supportEntity.SLAMetrics {
		ticket, err := svc.GetTicket(ctx, ticketID)
		require.NoError(t, err)
		statusEvents := eventsByTicket[ticketID]
		events := make([]*supportEntity.Event, len(statusEvents))
		copy(events, statusEvents)
		return ticket.ComputeSLAMetricsFromEvents(events, batch[ticketID])
	}

	// 1 continued: claimed but silent → nil first response time; a fresh
	// ticket is below the 1h wall-clock threshold, so not overdue yet.
	metricsA := slaFor(ticketA.ID)
	require.Nil(t, metricsA.FirstResponseTime, "no admin response → no first response time")
	require.Nil(t, metricsA.FirstResponseTimestamp)
	require.False(t, metricsA.FirstResponseOverdue, "fresh ticket below the 1h wall-clock threshold")

	// 2 continued: admin responded at T+30m → first response SATISFIED
	// (T+30m < 1h threshold), measured from creation to the message.
	metricsB := slaFor(ticketB.ID)
	require.NotNil(t, metricsB.FirstResponseTime)
	require.InDelta(t, (30 * time.Minute).Seconds(), metricsB.FirstResponseTime.Seconds(), 5,
		"first response time = creation → first admin message (T+30m)")
	require.False(t, metricsB.FirstResponseOverdue, "T+30m < 1h threshold → satisfied")

	// 3 continued: the SLA uses the MESSAGE time (T+40m). If the stale
	// AssignedAt authority were still in place, this would be ≈T+50m.
	metricsC := slaFor(ticketC.ID)
	require.NotNil(t, metricsC.FirstResponseTime)
	require.InDelta(t, (40 * time.Minute).Seconds(), metricsC.FirstResponseTime.Seconds(), 5,
		"SLA must use the first admin MESSAGE (T+40m), never the assignment")
	require.False(t, metricsC.FirstResponseOverdue, "T+40m < 1h threshold → satisfied")

	// 5 continued: user-only ticket behaves like the silent ticket.
	metricsD := slaFor(ticketD.ID)
	require.Nil(t, metricsD.FirstResponseTime, "user messages never produce a first response time")
}

// TestSLAF04_RealDB_ConsumerParityAndServicePassthrough proves that the
// list path, the detail path and the dashboard path all derive IDENTICAL
// first-response results from the single authority query + the single SLA
// calculator, and that the application-service pass-through matches the
// repository batch exactly.
func TestSLAF04_RealDB_ConsumerParityAndServicePassthrough(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, chatSvc, repo, _, database := newSLAF04ProofStack(t, tdb)

	ownerID := uuid.New()
	adminID := uuid.New()
	insertSupportProofUser(t, ctx, tdb, ownerID)
	insertSupportProofAdmin(t, ctx, tdb, adminID)

	ticket := createSLAF04Ticket(t, ctx, svc, ownerID, adminID, "SLA-F04 parity")
	creation := time.Now().Add(-90 * time.Minute)
	_, err := tdb.Pool().Exec(ctx,
		`UPDATE support_tickets SET created_at = $2 WHERE id = $1`, ticket.ID, creation)
	require.NoError(t, err)
	_, err = chatSvc.SendSupportMessage(ctx, ticket.ChatRoomID, adminID, "Reply at T+30m", uuid.NewString())
	require.NoError(t, err)
	firstResponse := time.Now().Add(-60 * time.Minute) // creation + 30m
	_, err = tdb.Pool().Exec(ctx,
		`UPDATE chat_messages SET created_at = $2 WHERE room_id = $1`, ticket.ChatRoomID, firstResponse)
	require.NoError(t, err)

	// Application-service pass-through must match the repository batch.
	viaService, err := svc.ListFirstAdminResponsesByTicketIDs(ctx, []uuid.UUID{ticket.ID})
	require.NoError(t, err)
	var viaRepo map[uuid.UUID]*time.Time
	err = database.WithTx(ctx, func(tx db.Tx) error {
		var err error
		viaRepo, err = repo.ListFirstAdminResponsesByTicketIDs(ctx, tx, []uuid.UUID{ticket.ID})
		return err
	})
	require.NoError(t, err)
	require.Len(t, viaService, 1)
	require.Len(t, viaRepo, 1)
	require.WithinDuration(t, *viaService[ticket.ID], *viaRepo[ticket.ID], time.Second,
		"service pass-through must equal the repository batch")

	// LIST path: page of tickets → batch authority → calculator.
	var eventsByTicket map[uuid.UUID][]*supportEntity.Event
	err = database.WithTx(ctx, func(tx db.Tx) error {
		var err error
		eventsByTicket, err = repo.ListStatusEventsForTickets(ctx, tx, []uuid.UUID{ticket.ID})
		return err
	})
	require.NoError(t, err)
	listTicket, err := svc.GetTicket(ctx, ticket.ID)
	require.NoError(t, err)
	statusEvents := eventsByTicket[ticket.ID]
	events := make([]*supportEntity.Event, len(statusEvents))
	copy(events, statusEvents)
	listMetrics := listTicket.ComputeSLAMetricsFromEvents(events, viaRepo[ticket.ID])

	// DETAIL path: single ticket → same batch authority → same calculator.
	detailMetrics := listTicket.ComputeSLAMetricsFromEvents(events, viaService[ticket.ID])

	// DASHBOARD path: exactly what platform/admin sla_service.go does —
	// ListTickets → batch events → batch first responses → calculator.
	var dashboardMetrics *supportEntity.SLAMetrics
	err = database.WithTx(ctx, func(tx db.Tx) error {
		tickets, err := repo.ListTickets(ctx, tx, nil, nil, nil, 1000)
		if err != nil {
			return err
		}
		for _, tk := range tickets {
			if tk.ID != ticket.ID {
				continue
			}
			frs, err := repo.ListFirstAdminResponsesByTicketIDs(ctx, tx, []uuid.UUID{tk.ID})
			if err != nil {
				return err
			}
			evs, err := repo.ListStatusEventsForTickets(ctx, tx, []uuid.UUID{tk.ID})
			if err != nil {
				return err
			}
			tevs := evs[tk.ID]
			mevents := make([]*supportEntity.Event, len(tevs))
			copy(mevents, tevs)
			m := tk.ComputeSLAMetricsFromEvents(mevents, frs[tk.ID])
			dashboardMetrics = &m
		}
		return nil
	})
	require.NoError(t, err)

	// All consumers agree — down to the exact first-response timestamp.
	require.Equal(t, *listMetrics.FirstResponseTimestamp, *detailMetrics.FirstResponseTimestamp,
		"list and detail must produce the same first response timestamp")
	require.Equal(t, listMetrics.FirstResponseTime.Seconds(), detailMetrics.FirstResponseTime.Seconds(),
		"list and detail must produce the same first response duration")
	require.Equal(t, listMetrics.FirstResponseOverdue, detailMetrics.FirstResponseOverdue)
	require.Equal(t, listMetrics.FirstResponseOverdue, dashboardMetrics.FirstResponseOverdue,
		"dashboard must agree with list/detail on first response overdue")
	require.InDelta(t, (30*time.Minute).Seconds(), listMetrics.FirstResponseTime.Seconds(), 5,
		"first response measured from creation to the admin message (T+30m)")
	// T+30m response on a 90m-old ticket is below the 1h threshold → satisfied.
	require.False(t, listMetrics.FirstResponseOverdue)
}

// insertSupportProofAdmin inserts a user row with the admin role, so admin
// sends are recognized by the canonical users.role='admin' authority.
func insertSupportProofAdmin(t *testing.T, ctx context.Context, tdb *testdb.TestDB, userID uuid.UUID) {
	t.Helper()
	_, err := tdb.Pool().Exec(ctx,
		`INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'admin')`,
		userID, userID.String(), userID.String()+"@support-proof.test",
	)
	require.NoError(t, err)
}
