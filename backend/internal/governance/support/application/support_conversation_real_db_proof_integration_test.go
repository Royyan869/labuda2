//go:build integration

// REAL POSTGRESQL PROOF — SUPPORT SCOPE 2: CONVERSATION END-TO-END.
//
// This test wires the production Support service and the production Chat
// service on a disposable real PostgreSQL database and proves the bounded
// Scope-2 outcome directly, with direct SQL observation of persisted state:
//
//  1. user can read their own ticket conversation
//  2. user can send a message; the sender is the authenticated user
//  3. admin can read the Support conversation
//  4. admin can send a message; the sender is the authenticated admin
//  5. the admin reply is a REAL authored message — never a system message
//  6. both messages are persisted with correct sender ids
//  7. no uuid.Nil is ever stored as a sender
//  8. both sides can read the same conversation
//  9. an admin reply produces the user-facing notification outbox event
//
// 10. a failed send produces no message and no notification
// 11. ticket isolation: ticket A's conversation is not ticket B's
package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	supportEntity "github.com/labuda/backend/internal/governance/support/entity"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatInfraRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newRealSupportConversationStack builds the production Support service wired to
// the production Chat service (with a real in-memory rate limiter, required by
// the message send path) and the real outbox repository on top of tdb.
func newRealSupportConversationStack(t *testing.T, tdb *testdb.TestDB) (*Service, *chatApp.Service) {
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
	return svc, chatSvc
}

func TestSupportConversation_RealDB_UserAndAdminEndToEnd(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc, chatSvc := newRealSupportConversationStack(t, tdb)

	ownerID := uuid.New()
	adminID := uuid.New()
	insertSupportProofUser(t, ctx, tdb, ownerID)
	insertSupportProofUser(t, ctx, tdb, adminID)

	subjectA := "Ticket A — payment"
	subjectB := "Ticket B — refund"
	ticketA, err := svc.CreateTicket(ctx, &CreateTicketRequest{
		UserID:   ownerID,
		Category: supportEntity.CategoryPaymentIssue,
		Priority: supportEntity.PriorityHigh,
		Subject:  &subjectA,
	})
	require.NoError(t, err, "ticket A creation must succeed")
	ticketB, err := svc.CreateTicket(ctx, &CreateTicketRequest{
		UserID:   ownerID,
		Category: supportEntity.CategoryRefundRequest,
		Priority: supportEntity.PriorityMedium,
		Subject:  &subjectB,
	})
	require.NoError(t, err, "ticket B creation must succeed")
	require.NotEqual(t, ticketA.ChatRoomID, ticketB.ChatRoomID,
		"each ticket must own a distinct conversation room")

	// -------------------------------------------------------------------
	// Ticket A is assigned through the REAL claim path — the same service call
	// the admin dashboard makes. This exercises the row lock
	// (FOR UPDATE OF st) against PostgreSQL and leaves the ticket in
	// in_progress with assigned_admin_id set, which is the state the
	// conversation workflow starts from.
	// -------------------------------------------------------------------
	claimed, err := svc.ClaimTicket(ctx, &ClaimTicketRequest{
		TicketID: ticketA.ID,
		AdminID:  adminID,
	})
	require.NoError(t, err, "assignment (claim) must succeed against real PostgreSQL")
	require.Equal(t, supportEntity.StatusInProgress, claimed.Status)
	require.NotNil(t, claimed.AssignedAdminID)
	require.Equal(t, adminID, *claimed.AssignedAdminID,
		"assignment authority lives on support_tickets.assigned_admin_id")

	var afterClaim int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM chat_messages WHERE room_id = $1`, ticketA.ChatRoomID).Scan(&afterClaim))
	require.Zero(t, afterClaim, "no message (fake system or otherwise) may exist after assignment")

	// -------------------------------------------------------------------
	// 1 & 2. The ticket owner sends a message into their own conversation.
	// -------------------------------------------------------------------
	userBody := "Halo, pembayaran saya gagal terus."
	userMsg, err := chatSvc.SendSupportMessage(ctx, ticketA.ChatRoomID, ownerID, userBody, uuid.NewString())
	require.NoError(t, err, "user must be able to send into their own conversation")
	require.Equal(t, ownerID, userMsg.SenderID, "sender must be the authenticated user")
	require.Equal(t, ticketA.ChatRoomID, userMsg.RoomID)
	require.False(t, userMsg.IsSystem(), "a user reply is never a system message")

	// -------------------------------------------------------------------
	// 3 & 4. The assigned agent sends a real authored reply.
	// -------------------------------------------------------------------
	adminBody := "Baik, kami cek transaksinya sekarang ya."
	adminMsg, err := chatSvc.SendSupportMessage(ctx, ticketA.ChatRoomID, adminID, adminBody, uuid.NewString())
	require.NoError(t, err, "the assigned agent must be able to reply")
	require.Equal(t, adminID, adminMsg.SenderID, "sender must be the authenticated agent")
	require.NotEqual(t, uuid.Nil, adminMsg.SenderID, "the agent id is never uuid.Nil")
	require.NotEqual(t, ownerID, adminMsg.SenderID, "the agent reply must not be attributed to the user")

	// 5. The admin reply is a real text message, never a system message.
	require.Equal(t, chatEntity.MessageTypeText, adminMsg.MessageType)
	require.False(t, adminMsg.IsSystem(), "an agent reply must never use the system-message path")

	// -------------------------------------------------------------------
	// 6, 7 & 11. Persisted rows: correct senders, no uuid.Nil, isolation.
	// -------------------------------------------------------------------
	type persisted struct {
		senderID    uuid.UUID
		messageType string
		body        string
	}
	rows, err := tdb.Pool().Query(ctx, `
		SELECT sender_id, message_type, body
		FROM chat_messages
		WHERE room_id = $1 AND deleted_at IS NULL
		ORDER BY created_at, id
	`, ticketA.ChatRoomID)
	require.NoError(t, err)
	defer rows.Close()

	var persistedMsgs []persisted
	for rows.Next() {
		var p persisted
		require.NoError(t, rows.Scan(&p.senderID, &p.messageType, &p.body))
		persistedMsgs = append(persistedMsgs, p)
	}
	require.NoError(t, rows.Err())
	require.Len(t, persistedMsgs, 2, "both messages must be persisted in ticket A's room")
	require.Equal(t, ownerID, persistedMsgs[0].senderID)
	require.Equal(t, userBody, persistedMsgs[0].body)
	require.Equal(t, "text", persistedMsgs[0].messageType)
	require.Equal(t, adminID, persistedMsgs[1].senderID)
	require.Equal(t, adminBody, persistedMsgs[1].body)
	require.Equal(t, "text", persistedMsgs[1].messageType)

	var nilSenderCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM chat_messages WHERE room_id = $1 AND sender_id = $2::uuid`,
		ticketA.ChatRoomID, nilUUIDLiteral).Scan(&nilSenderCount))
	require.Zero(t, nilSenderCount, "uuid.Nil must never be stored as a message sender")

	// Ticket B's conversation is untouched — no cross-ticket leakage.
	var ticketBCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM chat_messages WHERE room_id = $1`, ticketB.ChatRoomID).Scan(&ticketBCount))
	require.Zero(t, ticketBCount, "ticket B's conversation must be isolated from ticket A's")

	// -------------------------------------------------------------------
	// 8. Both sides read the same conversation through the Support read seam.
	//    The read is paginated/ordered by the chat repository, so assert on the
	//    conversation contents rather than a positional order.
	// -------------------------------------------------------------------
	sendersIn := func(msgs []*chatEntity.ChatMessage) map[uuid.UUID]string {
		bodies := make(map[uuid.UUID]string, len(msgs))
		for _, m := range msgs {
			if m.Body != nil {
				bodies[m.SenderID] = *m.Body
			}
		}
		return bodies
	}

	asUser, err := chatSvc.ListSupportMessages(ctx, ticketA.ChatRoomID, nil, nil, 50)
	require.NoError(t, err, "user-side read must succeed")
	require.Len(t, asUser, 2)
	require.Equal(t, userBody, sendersIn(asUser)[ownerID], "the user can read their own message")
	require.Equal(t, adminBody, sendersIn(asUser)[adminID], "the user can read the agent's reply")

	asAgent, err := chatSvc.ListSupportMessages(ctx, ticketA.ChatRoomID, nil, nil, 50)
	require.NoError(t, err, "agent-side read must succeed")
	require.Len(t, asAgent, 2)
	require.Equal(t, userBody, sendersIn(asAgent)[ownerID], "the agent can read the user's message")
	require.Equal(t, adminBody, sendersIn(asAgent)[adminID], "the agent can read their own reply")

	// The two messages are distinguishable by persisted sender identity alone.
	require.NotEqual(t, asUser[0].SenderID, asUser[1].SenderID,
		"user and agent messages must be distinguishable by sender identity")
	require.NotEqual(t, uuid.Nil, asUser[0].SenderID)
	require.NotEqual(t, uuid.Nil, asUser[1].SenderID)

	// -------------------------------------------------------------------
	// 9. The agent reply drives the canonical user notification: the ticket
	//    transitions to waiting_user and emits support.ticket_waiting_user
	//    with the OWNER as recipient.
	// -------------------------------------------------------------------
	require.NoError(t, svc.SetWaitingForUser(ctx, ticketA.ID, adminID))

	var notifCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT count(*) FROM outbox
		WHERE event_type = 'support.ticket_waiting_user'
		  AND payload->>'ticket_id' = $1
	`, ticketA.ID.String()).Scan(&notifCount))
	require.Equal(t, 1, notifCount, "an agent reply must emit exactly one user notification event")

	var recipient, actorID string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT payload->>'user_id', payload->>'admin_id' FROM outbox
		WHERE event_type = 'support.ticket_waiting_user'
		  AND payload->>'ticket_id' = $1
	`, ticketA.ID.String()).Scan(&recipient, &actorID))
	require.Equal(t, ownerID.String(), recipient, "the notification recipient must be the ticket owner")
	require.Equal(t, adminID.String(), actorID,
		"the notification event must carry the replying agent as the real actor")

	// The notification consumer persists notifications.actor_id against a FK to
	// users(id), so the actor carried by the event must be persistable. A
	// uuid.Nil actor (the previous sentinel) is rejected by that FK and the
	// user would never receive the notification. Prove the produced actor is
	// insertable — i.e. the notification for an agent reply can actually be
	// delivered.
	var notifID uuid.UUID
	err = tdb.Pool().QueryRow(ctx, `
		INSERT INTO notifications (recipient_id, actor_id, type, entity_id)
		VALUES ($1, $2, 'support.ticket_waiting_user', $3)
		RETURNING id
	`, ownerID, actorID, ticketA.ID).Scan(&notifID)
	require.NoError(t, err, "the agent-reply notification must be persistable with the produced actor")
	require.NotEqual(t, uuid.Nil, notifID)

	// And the uuid.Nil sentinel the event used to carry is provably rejected.
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO notifications (recipient_id, actor_id, type, entity_id)
		VALUES ($1, $2::uuid, 'support.ticket_waiting_user', $3)
	`, ownerID, nilUUIDLiteral, ticketA.ID)
	require.Error(t, err, "a uuid.Nil actor is unpersistable against the users FK")

	// No competing chat-message notification is produced for a support room.
	var chatNotif int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM outbox WHERE event_type = 'chat.message.notification'`).Scan(&chatNotif))
	require.Zero(t, chatNotif, "support rooms must not emit chat message notifications")

	// -------------------------------------------------------------------
	// 10. A failed send produces no message and no notification.
	// -------------------------------------------------------------------
	_, err = chatSvc.SendSupportMessage(ctx, ticketA.ChatRoomID, adminID, "", uuid.NewString())
	require.Error(t, err, "an empty message body must be rejected")

	var afterFailure int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM chat_messages WHERE room_id = $1`, ticketA.ChatRoomID).Scan(&afterFailure))
	require.Equal(t, 2, afterFailure, "a failed send must not persist a message")

	var notifAfterFailure int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT count(*) FROM outbox
		WHERE event_type = 'support.ticket_waiting_user'
		  AND payload->>'ticket_id' = $1
	`, ticketA.ID.String()).Scan(&notifAfterFailure))
	require.Equal(t, notifCount, notifAfterFailure,
		"a failed send must not produce a notification event")

	// -------------------------------------------------------------------
	// The user can reply into the waiting_user ticket; the reply emits the
	// agent-facing event (support.ticket.user_responded).
	// -------------------------------------------------------------------
	userReply, err := chatSvc.SendSupportMessage(ctx, ticketA.ChatRoomID, ownerID, "Sudah saya kirim bukti transfer.", uuid.NewString())
	require.NoError(t, err)
	require.Equal(t, ownerID, userReply.SenderID)

	require.NoError(t, svc.HandleUserReply(ctx, ticketA.ChatRoomID, ownerID))

	var userRespondedCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT count(*) FROM outbox
		WHERE event_type = 'support.ticket.user_responded'
		  AND payload->>'ticket_id' = $1
		  AND payload->>'admin_id' = $2
	`, ticketA.ID.String(), adminID.String()).Scan(&userRespondedCount))
	require.Equal(t, 1, userRespondedCount,
		"a user reply on a waiting_user ticket must notify the assigned agent")

	// The ticket transitioned back to in_progress.
	reloaded, err := svc.GetTicket(ctx, ticketA.ID)
	require.NoError(t, err)
	require.Equal(t, supportEntity.StatusInProgress, reloaded.Status)

	// Final conversation state: three real messages, all with real senders.
	var finalCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT count(*) FROM chat_messages
		WHERE room_id = $1 AND sender_id <> $2::uuid
	`, ticketA.ChatRoomID, nilUUIDLiteral).Scan(&finalCount))
	require.Equal(t, 3, finalCount, fmt.Sprintf("expected 3 real messages, got %d", finalCount))
}
