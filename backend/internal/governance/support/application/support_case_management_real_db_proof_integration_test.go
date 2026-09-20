//go:build integration

// REAL POSTGRESQL PROOF — SUPPORT CASE MANAGEMENT (user + admin).
//
// This test drives the production Support service against a disposable real
// PostgreSQL database and proves the case-management outcome directly, with
// direct SQL observation of persisted state:
//
//  1. multiple tickets per user, each owning exactly one distinct room
//  2. the user's ticket list is the Support authority for their own cases
//  3. agent ticket list sees the queue
//  4. ASSIGNMENT works against PostgreSQL (real claim path, row lock)
//  5. a claimed ticket cannot be claimed twice
//  6. an unassignable ticket status is rejected
//  7. an invalid lifecycle transition is rejected
//  8. resolved -> open reopen works (agent actor)
//  9. resolved -> closed works, and closed is TERMINAL
//
// 10. waiting_user reflects "SLA is waiting on the user"
// 11. lifecycle changes write NO synthetic conversation messages
// 12. conversation isolation between tickets is preserved
package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	supportEntity "github.com/labuda/backend/internal/governance/support/entity"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

func TestSupportCaseManagement_RealDB_UserAndAdminEndToEnd(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, chatSvc := newRealSupportConversationStack(t, tdb)

	ownerID := uuid.New()
	otherUserID := uuid.New()
	adminID := uuid.New()
	insertSupportProofUser(t, ctx, tdb, ownerID)
	insertSupportProofUser(t, ctx, tdb, otherUserID)
	insertSupportProofUser(t, ctx, tdb, adminID)

	// -------------------------------------------------------------------
	// 1. Multiple tickets per user are allowed, and each owns its own room.
	// -------------------------------------------------------------------
	subjectA := "Ticket A — payment"
	subjectB := "Ticket B — refund"
	descriptionA := "Payment step hangs forever"
	ticketA, err := svc.CreateTicket(ctx, &CreateTicketRequest{
		UserID:      ownerID,
		Category:    supportEntity.CategoryPaymentIssue,
		Priority:    supportEntity.PriorityHigh,
		Subject:     &subjectA,
		Description: &descriptionA,
	})
	require.NoError(t, err, "first ticket for a user must be created")
	ticketB, err := svc.CreateTicket(ctx, &CreateTicketRequest{
		UserID:   ownerID,
		Category: supportEntity.CategoryRefundRequest,
		Priority: supportEntity.PriorityMedium,
		Subject:  &subjectB,
	})
	require.NoError(t, err, "a second ticket for the SAME user must be allowed")
	require.NotEqual(t, ticketA.ID, ticketB.ID)
	require.NotEqual(t, ticketA.ChatRoomID, ticketB.ChatRoomID,
		"each ticket must own a distinct conversation room")

	var roomCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(DISTINCT chat_room_id) FROM support_tickets WHERE user_id = $1`,
		ownerID).Scan(&roomCount))
	require.Equal(t, 2, roomCount, "1 ticket = 1 room; no room sharing")

	// -------------------------------------------------------------------
	// 2 & 3. List authority: the user sees only their own tickets; the agent
	//        list sees the queue.
	// -------------------------------------------------------------------
	mine, err := svc.ListMyTickets(ctx, ownerID, nil, nil, 50)
	require.NoError(t, err)
	require.Len(t, mine, 2, "the owner's Support list is their two cases")
	for _, tk := range mine {
		require.Equal(t, ownerID, tk.UserID, "a user's list never contains another user's case")
	}

	strangerTickets, err := svc.ListMyTickets(ctx, otherUserID, nil, nil, 50)
	require.NoError(t, err)
	require.Empty(t, strangerTickets, "another user sees none of these cases")

	queue, err := svc.ListTickets(ctx, &supportRepo.TicketFilter{}, nil, nil, 50)
	require.NoError(t, err)
	require.Len(t, queue, 2, "the agent queue lists the cases")

	// -------------------------------------------------------------------
	// 4. ASSIGNMENT. The real claim path runs a locked read against
	//    PostgreSQL — this is what makes the admin dashboard usable.
	// -------------------------------------------------------------------
	claimed, err := svc.ClaimTicket(ctx, &ClaimTicketRequest{
		TicketID: ticketA.ID,
		AdminID:  adminID,
	})
	require.NoError(t, err, "claim (assignment) must work against real PostgreSQL")
	require.Equal(t, supportEntity.StatusInProgress, claimed.Status)
	require.NotNil(t, claimed.AssignedAdminID)
	require.Equal(t, adminID, *claimed.AssignedAdminID,
		"assignment authority is support_tickets.assigned_admin_id")
	require.NotNil(t, claimed.AssignedAt)

	var persistedStatus string
	var persistedAdmin *uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status, assigned_admin_id FROM support_tickets WHERE id = $1`,
		ticketA.ID).Scan(&persistedStatus, &persistedAdmin))
	require.Equal(t, "in_progress", persistedStatus, "assignment is persisted")
	require.NotNil(t, persistedAdmin)
	require.Equal(t, adminID, *persistedAdmin)

	// 5. A claimed ticket cannot be claimed again.
	_, err = svc.ClaimTicket(ctx, &ClaimTicketRequest{TicketID: ticketA.ID, AdminID: otherUserID})
	require.Error(t, err, "a claimed ticket must not be reassigned by claim")
	require.Equal(t, supportRepo.ErrTicketAlreadyClaimed, err)

	// 6. An unassignable status is rejected (ticket B is still open, so claim
	//    it first to reach a non-claimable status, then try again).
	_, err = svc.ClaimTicket(ctx, &ClaimTicketRequest{TicketID: ticketB.ID, AdminID: adminID})
	require.NoError(t, err)
	_, err = svc.ClaimTicket(ctx, &ClaimTicketRequest{TicketID: ticketB.ID, AdminID: adminID})
	require.Error(t, err)

	// 7. An invalid lifecycle transition is rejected: resolving an OPEN ticket
	//    is not a legal transition.
	subjectC := "Ticket C — open ticket resolve attempt"
	ticketC, err := svc.CreateTicket(ctx, &CreateTicketRequest{
		UserID:   ownerID,
		Category: supportEntity.CategoryOrderIssue,
		Priority: supportEntity.PriorityLow,
		Subject:  &subjectC,
	})
	require.NoError(t, err)
	err = svc.ResolveTicket(ctx, &ResolveTicketRequest{TicketID: ticketC.ID, AdminID: adminID})
	require.Error(t, err, "resolve on an open ticket must be rejected")

	// 10. waiting_user is the "SLA is waiting on the user" state: the next
	//     action is WAIT and the waiting time is tracked separately.
	require.NoError(t, svc.SetWaitingForUser(ctx, ticketA.ID, adminID))
	waitingTicket, err := svc.GetTicket(ctx, ticketA.ID)
	require.NoError(t, err)
	require.Equal(t, supportEntity.StatusWaitingUser, waitingTicket.Status)
	sla := waitingTicket.ComputeSLAMetricsFromEvents(nil, nil)
	require.Equal(t, supportEntity.NextActionWait, sla.NextAction,
		"waiting_user means the SLA is waiting for the user")

	// A real agent reply, then the message-aware SLA separates waiting from
	// active time (the SLA clock is not "working" while it waits on the user).
	adminBody := "Kami sudah cek, mohon konfirmasi ya."
	_, err = chatSvc.SendSupportMessage(ctx, ticketA.ChatRoomID, adminID, adminBody, uuid.NewString())
	require.NoError(t, err)
	conversation, err := chatSvc.ListSupportMessages(ctx, ticketA.ChatRoomID, nil, nil, 50)
	require.NoError(t, err)
	var events []supportEntity.MessageEvent
	for _, m := range conversation {
		events = append(events, supportEntity.MessageEvent{
			Timestamp:   m.CreatedAt,
			SenderID:    m.SenderID.String(),
			IsAdmin:     m.SenderID != ownerID,
			MessageType: string(m.MessageType),
		})
	}
	full := waitingTicket.ComputeSLAMetrics(events)
	require.GreaterOrEqual(t, full.WaitingTime, time.Duration(0))
	require.Equal(t, supportEntity.NextActionWait, full.NextAction)

	// 8. The user reply moves waiting_user -> in_progress (canonical hook).
	_, err = chatSvc.SendSupportMessage(ctx, ticketA.ChatRoomID, ownerID, "Sudah saya cek, masih gagal.", uuid.NewString())
	require.NoError(t, err)
	require.NoError(t, svc.HandleUserReply(ctx, ticketA.ChatRoomID, ownerID))
	afterReply, err := svc.GetTicket(ctx, ticketA.ID)
	require.NoError(t, err)
	require.Equal(t, supportEntity.StatusInProgress, afterReply.Status,
		"a user reply returns the case to in_progress")

	// 9. resolved -> open, driven by an AGENT actor.
	require.NoError(t, svc.ResolveTicket(ctx, &ResolveTicketRequest{TicketID: ticketA.ID, AdminID: adminID}))
	reopened, err := svc.ReopenTicket(ctx, &ReopenTicketRequest{
		TicketID: ticketA.ID,
		ActorID:  adminID,
		IsAdmin:  true,
	})
	require.NoError(t, err, "an agent must be able to reopen a resolved case")
	require.Equal(t, supportEntity.StatusOpen, reopened.Status)
	require.Nil(t, reopened.AssignedAdminID, "reopen clears assignment")
	require.Nil(t, reopened.ResolvedAt)

	// Reopen is owner-authorised too, and it is still owner-scoped.
	_, err = svc.ClaimTicket(ctx, &ClaimTicketRequest{TicketID: ticketA.ID, AdminID: adminID})
	require.NoError(t, err)
	require.NoError(t, svc.ResolveTicket(ctx, &ResolveTicketRequest{TicketID: ticketA.ID, AdminID: adminID}))
	_, err = svc.ReopenTicket(ctx, &ReopenTicketRequest{TicketID: ticketA.ID, ActorID: otherUserID})
	require.Error(t, err, "a foreign user must not reopen someone else's case")
	require.Equal(t, supportRepo.ErrTicketNotFound, err)
	_, err = svc.ReopenTicket(ctx, &ReopenTicketRequest{TicketID: ticketA.ID, ActorID: ownerID})
	require.NoError(t, err, "the owner reopens their own resolved case")

	// resolved -> closed, then closed is TERMINAL.
	_, err = svc.ClaimTicket(ctx, &ClaimTicketRequest{TicketID: ticketA.ID, AdminID: adminID})
	require.NoError(t, err)
	require.NoError(t, svc.ResolveTicket(ctx, &ResolveTicketRequest{TicketID: ticketA.ID, AdminID: adminID}))
	reason := "user confirmed"
	require.NoError(t, svc.CloseTicket(ctx, &CloseTicketRequest{
		TicketID:    ticketA.ID,
		AdminID:     adminID,
		CloseReason: &reason,
	}))

	_, err = svc.ReopenTicket(ctx, &ReopenTicketRequest{
		TicketID: ticketA.ID,
		ActorID:  adminID,
		IsAdmin:  true,
	})
	require.Error(t, err, "a CLOSED case is terminal — never reopened")
	require.Equal(t, supportRepo.ErrCannotReopenTicket, err)

	finalTicket, err := svc.GetTicket(ctx, ticketA.ID)
	require.NoError(t, err)
	require.Equal(t, supportEntity.StatusClosed, finalTicket.Status,
		"the rejected reopen must not have mutated the case")
	require.NotNil(t, finalTicket.ClosedAt)
	// Closing retains the handling agent as the historical record of who
	// resolved the case (assignment is only cleared by reopen).
	require.NotNil(t, finalTicket.AssignedAdminID)
	require.Equal(t, adminID, *finalTicket.AssignedAdminID)

	// 11. Lifecycle changes write NO synthetic conversation messages: the only
	//     rows in ticket A's room are the two the user and the agent wrote.
	var messageCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM chat_messages WHERE room_id = $1`, ticketA.ChatRoomID).Scan(&messageCount))
	require.Equal(t, 2, messageCount,
		"lifecycle truth lives in ticket state/events, never in the conversation")

	var nilSenderCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM chat_messages WHERE sender_id = $1::uuid`,
		nilUUIDLiteral).Scan(&nilSenderCount))
	require.Zero(t, nilSenderCount, "uuid.Nil must never be stored as a sender")

	// 12. Ticket B's conversation stays isolated from ticket A's lifecycle.
	var roomBCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM chat_messages WHERE room_id = $1`, ticketB.ChatRoomID).Scan(&roomBCount))
	require.Zero(t, roomBCount, "ticket B's conversation must stay isolated")
}
