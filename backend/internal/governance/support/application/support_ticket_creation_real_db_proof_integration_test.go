//go:build integration

// REAL POSTGRESQL PROOF — SUPPORT SCOPE 1: TICKET CREATION END-TO-END.
//
// This test wires the production Support service to the production Chat
// service on a disposable real PostgreSQL database and proves the bounded
// Scope-1 outcome directly:
//
//  1. create ticket succeeds
//  2. the support_tickets row really exists
//  3. a chat_rooms row really exists
//  4. the ticket ↔ room relationship is correct (ticket.chat_room_id)
//  5. a second ticket for the same user provisions a DIFFERENT room
//  6. no uuid.Nil is stored as a fake participant
//  7. support rooms are never reused
//  8. the ticket can be read back through the service (API read path)
//
// Room-level proof is done with direct SQL so the test observes the real
// persisted state rather than a projection of it.
package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	supportEntity "github.com/labuda/backend/internal/governance/support/entity"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatInfraRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const nilUUIDLiteral = "00000000-0000-0000-0000-000000000000"

// newRealSupportTicketStack builds the production Support service wired to the
// production Chat service and the real outbox repository on top of tdb.
func newRealSupportTicketStack(t *testing.T, tdb *testdb.TestDB) *Service {
	t.Helper()

	database := db.NewFromPool(tdb.Pool())
	outbox := outboxRepo.NewOutboxRepository(database)

	chatSvc := chatApp.NewService(
		tdb,
		chatInfraRepo.NewChatRepository(),
		nil, // socialRepo not exercised by support room creation
		outbox,
		nil, // rate limiter
		nil, // metrics
		nil, // account status checker
		nil, // order ownership reader
		zap.NewNop(),
	)

	// orderService and disputeService are not exercised by unlinked tickets.
	return NewServiceWithDefaults(tdb, chatSvc, outbox, nil, nil, zap.NewNop())
}

func countSupportRooms(t *testing.T, ctx context.Context, tdb *testdb.TestDB) int {
	t.Helper()
	var n int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM chat_rooms WHERE room_type = 'support'`).Scan(&n))
	return n
}

func insertSupportProofUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB, userID uuid.UUID) {
	t.Helper()
	_, err := tdb.Pool().Exec(ctx,
		`INSERT INTO users (id, firebase_uid, email) VALUES ($1, $2, $3)`,
		userID, userID.String(), userID.String()+"@support-proof.test",
	)
	require.NoError(t, err)
}

func TestSupportTicketCreation_RealDB_CanonicalProof(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	svc := newRealSupportTicketStack(t, tdb)

	ownerID := uuid.New()
	insertSupportProofUser(t, ctx, tdb, ownerID)

	subject := "Cannot complete checkout"
	description := "Payment step hangs forever"

	// -------------------------------------------------------------------
	// 1. Create ticket succeeds.
	// -------------------------------------------------------------------
	ticket, err := svc.CreateTicket(ctx, &CreateTicketRequest{
		UserID:      ownerID,
		Category:    supportEntity.CategoryPaymentIssue,
		Priority:    supportEntity.PriorityHigh,
		Subject:     &subject,
		Description: &description,
	})
	require.NoError(t, err, "ticket creation must succeed")
	require.NotNil(t, ticket)
	require.NotEqual(t, uuid.Nil, ticket.ID)
	require.False(t, ticket.ChatRoomID == uuid.Nil, "ticket must reference a real room")

	// -------------------------------------------------------------------
	// 2. support_tickets row really exists and is canonical.
	// -------------------------------------------------------------------
	var (
		category    string
		priority    string
		status      string
		dbSubject   string
		dbDesc      string
		chatRoomID  uuid.UUID
		dbUserID    uuid.UUID
		assignedNil bool
	)
	err = tdb.Pool().QueryRow(ctx, `
		SELECT user_id, category, priority, status, subject, description,
		       chat_room_id, assigned_admin_id IS NULL
		FROM support_tickets WHERE id = $1
	`, ticket.ID).Scan(&dbUserID, &category, &priority, &status, &dbSubject, &dbDesc, &chatRoomID, &assignedNil)
	require.NoError(t, err, "support_tickets row must be persisted")
	require.Equal(t, ownerID, dbUserID)
	require.Equal(t, "payment_issue", category, "canonical category must be persisted verbatim")
	require.Equal(t, "high", priority)
	require.Equal(t, "open", status)
	require.Equal(t, subject, dbSubject)
	require.Equal(t, description, dbDesc)
	require.True(t, assignedNil, "unassigned ticket has no assigned_admin_id")

	// -------------------------------------------------------------------
	// 3. chat_rooms row really exists, is a support room and has NULL agent.
	// -------------------------------------------------------------------
	var (
		roomType     string
		participantA uuid.UUID
		participantB *uuid.UUID
	)
	err = tdb.Pool().QueryRow(ctx, `
		SELECT room_type, participant_a, participant_b
		FROM chat_rooms WHERE id = $1
	`, ticket.ChatRoomID).Scan(&roomType, &participantA, &participantB)
	require.NoError(t, err, "support chat room must be persisted")
	require.Equal(t, "support", roomType)
	require.Equal(t, ownerID, participantA, "ticket owner is the sole human participant")
	require.Nil(t, participantB, "support room must NOT store a fake agent participant")

	// -------------------------------------------------------------------
	// 4. ticket ↔ room relationship is correct.
	// -------------------------------------------------------------------
	require.Equal(t, ticket.ChatRoomID, chatRoomID,
		"the persisted support_tickets.chat_room_id must equal the provisioned room")

	// The room must be reachable from the ticket through the chat repository too.
	var roomTicketCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM support_tickets WHERE chat_room_id = $1`, ticket.ChatRoomID).
		Scan(&roomTicketCount))
	require.Equal(t, 1, roomTicketCount, "exactly one ticket is bound to the room (1:1)")

	// -------------------------------------------------------------------
	// 5. A second ticket for the SAME user provisions a DIFFERENT room.
	// -------------------------------------------------------------------
	subject2 := "Refund not received"
	ticket2, err := svc.CreateTicket(ctx, &CreateTicketRequest{
		UserID:   ownerID,
		Category: supportEntity.CategoryRefundRequest,
		Priority: supportEntity.PriorityMedium,
		Subject:  &subject2,
	})
	require.NoError(t, err, "second ticket creation must succeed")
	require.NotEqual(t, ticket.ID, ticket2.ID)
	require.NotEqual(t, ticket.ChatRoomID, ticket2.ChatRoomID,
		"a second ticket must never reuse the first ticket's room")

	// -------------------------------------------------------------------
	// 6 & 7. No uuid.Nil fake participant, no reused room.
	// -------------------------------------------------------------------
	var supportRooms, nilParticipantRooms, distinctRooms int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM chat_rooms WHERE room_type = 'support'`).Scan(&supportRooms))
	require.Equal(t, 2, supportRooms, "exactly one support room per ticket")

	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT count(*) FROM chat_rooms
		WHERE room_type = 'support'
		  AND (participant_a = $1::uuid OR participant_b = $1::uuid)
	`, nilUUIDLiteral).Scan(&nilParticipantRooms))
	require.Zero(t, nilParticipantRooms, "uuid.Nil must never be stored as a support participant")

	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT count(DISTINCT id) FROM chat_rooms
		WHERE id IN ($1, $2)
	`, ticket.ChatRoomID, ticket2.ChatRoomID).Scan(&distinctRooms))
	require.Equal(t, 2, distinctRooms, "the two tickets must own two distinct rooms")

	// -------------------------------------------------------------------
	// 8. Ticket is readable back through the service (API read path).
	// -------------------------------------------------------------------
	readBack, err := svc.GetTicket(ctx, ticket.ID)
	require.NoError(t, err, "created ticket must be readable via the service")
	require.Equal(t, ticket.ID, readBack.ID)
	require.Equal(t, ownerID, readBack.UserID)
	require.Equal(t, ticket.ChatRoomID, readBack.ChatRoomID)
	require.Equal(t, supportEntity.CategoryPaymentIssue, readBack.Category)
	require.Equal(t, supportEntity.PriorityHigh, readBack.Priority)
	require.Equal(t, supportEntity.StatusOpen, readBack.Status)
	require.NotNil(t, readBack.Subject)
	require.Equal(t, subject, *readBack.Subject)
	require.NotNil(t, readBack.Description)
	require.Equal(t, description, *readBack.Description)

	// The ticket is also listed for its owner.
	listed, err := svc.ListMyTickets(ctx, ownerID, nil, nil, 50)
	require.NoError(t, err)
	require.Len(t, listed, 2, "both tickets must be listed for the owner")

	// The outbox event for each created ticket is committed atomically.
	var outboxCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM outbox WHERE event_type = 'support.ticket.created'`).
		Scan(&outboxCount))
	require.Equal(t, 2, outboxCount, "support.ticket.created must be emitted once per ticket")
}

// TestSupportTicketCreation_RealDB_CategoryEnumRejectsLegacyValues proves the
// database enum and the entity share the single canonical taxonomy: legacy
// (divergent) category values are rejected and never silently translated.
func TestSupportTicketCreation_RealDB_CategoryEnumRejectsLegacyValues(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	// Every canonical value must exist in the database enum, in order.
	rows, err := tdb.Pool().Query(ctx, `
		SELECT enumlabel FROM pg_enum e
		JOIN pg_type t ON t.oid = e.enumtypid
		WHERE t.typname = 'ticket_category_enum'
		ORDER BY enumsortorder
	`)
	require.NoError(t, err)
	defer rows.Close()

	var dbValues []string
	for rows.Next() {
		var v string
		require.NoError(t, rows.Scan(&v))
		dbValues = append(dbValues, v)
	}
	require.NoError(t, rows.Err())

	var entityValues []string
	for _, c := range supportEntity.AllCategories {
		entityValues = append(entityValues, c.String())
	}
	require.Equal(t, entityValues, dbValues,
		"DB enum and canonical entity taxonomy must be identical and ordered the same")

	// Legacy taxonomy values must not be valid in the entity.
	for _, legacy := range []supportEntity.Category{"payment", "order", "technical", "general", "account"} {
		require.False(t, legacy.IsValid(), "legacy category %q must not be canonical", legacy)
	}
	require.False(t, supportEntity.Category("").IsValid())

	// The database enum itself rejects a legacy value: an insert with 'payment'
	// must fail rather than silently store a competing vocabulary.
	ownerID := uuid.New()
	insertSupportProofUser(t, ctx, tdb, ownerID)

	roomID := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO chat_rooms (id, room_type, participant_a) VALUES ($1, 'support', $2)
	`, roomID, ownerID)
	require.NoError(t, err)

	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO support_tickets (id, user_id, chat_room_id, category, priority, status, escalation)
		VALUES ($1, $2, $3, 'payment', 'medium', 'open', 'none')
	`, uuid.New(), ownerID, roomID)
	require.Error(t, err, "legacy category value must be rejected by the DB enum")
	require.Contains(t, err.Error(), "ticket_category_enum")

	// The service must reject a legacy category BEFORE provisioning a room, so
	// an invalid request can never leave an orphan support room behind.
	svc := newRealSupportTicketStack(t, tdb)
	roomsBefore := countSupportRooms(t, ctx, tdb)
	_, err = svc.CreateTicket(ctx, &CreateTicketRequest{
		UserID:   ownerID,
		Category: supportEntity.Category("payment"),
		Priority: supportEntity.PriorityMedium,
	})
	require.ErrorIs(t, err, supportRepo.ErrInvalidCategory)
	require.Equal(t, roomsBefore, countSupportRooms(t, ctx, tdb),
		"an invalid category must not provision a support room")
}
