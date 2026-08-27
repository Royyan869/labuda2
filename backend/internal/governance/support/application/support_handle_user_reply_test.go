package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/support/entity"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// replyMockRepository wraps mockRepository with room-based ticket lookup.
type replyMockRepository struct {
	mockRepository
	ticketsByRoom map[uuid.UUID]*entity.Ticket
}

func newReplyMockRepository() *replyMockRepository {
	return &replyMockRepository{
		mockRepository: *newMockRepository(),
		ticketsByRoom:  make(map[uuid.UUID]*entity.Ticket),
	}
}

func (m *replyMockRepository) GetTicketByChatRoomID(ctx context.Context, tx interface{}, chatRoomID uuid.UUID) (*entity.Ticket, error) {
	ticket, exists := m.ticketsByRoom[chatRoomID]
	if !exists {
		return nil, supportRepo.ErrTicketNotFound
	}
	return ticket, nil
}

// addTicketWithRoom adds a ticket to both the ID and room-based indexes.
func (m *replyMockRepository) addTicketWithRoom(ticket *entity.Ticket) {
	m.tickets[ticket.ID] = ticket
	m.ticketsByRoom[ticket.ChatRoomID] = ticket
}

// outboxSpy records emitted events for assertions.
type outboxSpy struct {
	events []outboxSpyEvent
}

type outboxSpyEvent struct {
	EventType      string
	IdempotencyKey string
}

func (m *outboxSpy) InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
	m.events = append(m.events, outboxSpyEvent{EventType: eventType, IdempotencyKey: idempotencyKey})
	return nil
}

func makeWaitingTicket(userID, roomID uuid.UUID, adminID *uuid.UUID) *entity.Ticket {
	ticket := entity.NewTicket(userID, roomID, entity.CategoryPayment, entity.PriorityMedium)
	ticket.Status = entity.StatusWaitingUser
	ticket.AssignedAdminID = adminID
	return ticket
}

// TestHandleUserReply_WaitingUser_TransitionsToInProgress verifies the happy path:
// a ticket in waiting_user becomes in_progress when the ticket owner replies.
func TestHandleUserReply_WaitingUser_TransitionsToInProgress(t *testing.T) {
	ctx := context.Background()
	repo := newReplyMockRepository()
	outbox := &outboxSpy{}
	transactor := &mockTransactor{}

	userID := uuid.New()
	roomID := uuid.New()
	adminID := uuid.New()

	ticket := makeWaitingTicket(userID, roomID, &adminID)
	repo.addTicketWithRoom(ticket)

	service := &Service{
		repo:       repo,
		outboxRepo: outbox,
		db:         transactor,
		log:        zap.NewNop(),
	}

	err := service.HandleUserReply(ctx, roomID, userID)
	require.NoError(t, err)

	// Ticket should now be in_progress.
	assert.Equal(t, entity.StatusInProgress, ticket.Status)

	// Event should have been created.
	events := repo.events[ticket.ID]
	require.Len(t, events, 1)
	assert.Equal(t, entity.EventTypeStatusChanged, events[0].EventType)
	assert.NotNil(t, events[0].OldStatus)
	assert.Equal(t, entity.StatusWaitingUser, *events[0].OldStatus)
	assert.NotNil(t, events[0].NewStatus)
	assert.Equal(t, entity.StatusInProgress, *events[0].NewStatus)

	// Outbox should emit support.ticket.user_responded.
	require.Len(t, outbox.events, 1)
	assert.Equal(t, "support.ticket.user_responded", outbox.events[0].EventType)
}

// TestHandleUserReply_NonWaiting_NoOp verifies that tickets NOT in
// waiting_user status are left unchanged.
func TestHandleUserReply_NonWaiting_NoOp(t *testing.T) {
	statuses := []entity.Status{
		entity.StatusOpen,
		entity.StatusInProgress,
		entity.StatusResolved,
		entity.StatusClosed,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			ctx := context.Background()
			repo := newReplyMockRepository()
			outbox := &outboxSpy{}
			transactor := &mockTransactor{}

			userID := uuid.New()
			roomID := uuid.New()
			adminID := uuid.New()

			ticket := makeWaitingTicket(userID, roomID, &adminID)
			ticket.Status = status
			repo.addTicketWithRoom(ticket)

			service := &Service{
				repo:       repo,
				outboxRepo: outbox,
				db:         transactor,
				log:        zap.NewNop(),
			}

			err := service.HandleUserReply(ctx, roomID, userID)
			require.NoError(t, err)

			// Status should remain unchanged.
			assert.Equal(t, status, ticket.Status)

			// No events emitted.
			assert.Empty(t, repo.events[ticket.ID])
			assert.Empty(t, outbox.events)
		})
	}
}

// TestHandleUserReply_NonOwner_NoOp verifies that messages from non-owners
// (e.g., admin) do not trigger the transition.
func TestHandleUserReply_NonOwner_NoOp(t *testing.T) {
	ctx := context.Background()
	repo := newReplyMockRepository()
	outbox := &outboxSpy{}
	transactor := &mockTransactor{}

	userID := uuid.New()
	roomID := uuid.New()
	adminID := uuid.New()
	otherSender := uuid.New()

	ticket := makeWaitingTicket(userID, roomID, &adminID)
	repo.addTicketWithRoom(ticket)

	service := &Service{
		repo:       repo,
		outboxRepo: outbox,
		db:         transactor,
		log:        zap.NewNop(),
	}

	err := service.HandleUserReply(ctx, roomID, otherSender)
	require.NoError(t, err)

	// Status should remain waiting_user.
	assert.Equal(t, entity.StatusWaitingUser, ticket.Status)
	assert.Empty(t, outbox.events)
}

// TestHandleUserReply_NoTicket_NoOp verifies that a reply in a room with
// no active ticket is a silent no-op (e.g., ticket already closed).
func TestHandleUserReply_NoTicket_NoOp(t *testing.T) {
	ctx := context.Background()
	repo := newReplyMockRepository()
	outbox := &outboxSpy{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:       repo,
		outboxRepo: outbox,
		db:         transactor,
		log:        zap.NewNop(),
	}

	err := service.HandleUserReply(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, outbox.events)
}

// TestHandleUserReply_NoAdmin_SkipsOutbox verifies that when no admin is
// assigned, the transition still happens but no outbox event is emitted
// (no one to notify).
func TestHandleUserReply_NoAdmin_SkipsOutbox(t *testing.T) {
	ctx := context.Background()
	repo := newReplyMockRepository()
	outbox := &outboxSpy{}
	transactor := &mockTransactor{}

	userID := uuid.New()
	roomID := uuid.New()

	ticket := makeWaitingTicket(userID, roomID, nil) // no admin
	repo.addTicketWithRoom(ticket)

	service := &Service{
		repo:       repo,
		outboxRepo: outbox,
		db:         transactor,
		log:        zap.NewNop(),
	}

	err := service.HandleUserReply(ctx, roomID, userID)
	require.NoError(t, err)

	// Transition should still happen.
	assert.Equal(t, entity.StatusInProgress, ticket.Status)

	// But no outbox event (no admin to notify).
	assert.Empty(t, outbox.events)
}


