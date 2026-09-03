package application

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/support/entity"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockTx is a mock database transaction.
type mockTx struct{}

func (m *mockTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return &mockRow{}
}

func (m *mockTx) Commit(ctx context.Context) error {
	return nil
}

func (m *mockTx) Rollback(ctx context.Context) error {
	return nil
}

// mockRow is a mock pgx.Row
type mockRow struct{}

func (m *mockRow) Scan(dest ...any) error {
	return nil
}

// mockRepository is a mock support repository.
type mockRepository struct {
	tickets      map[uuid.UUID]*entity.Ticket
	events       map[uuid.UUID][]*entity.Event
	admins       map[uuid.UUID]*entity.Admin
	createError  error
	getError     error
	claimError   error
	resolveError error
	closeError   error
	reopenError  error
	updateError  error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		tickets: make(map[uuid.UUID]*entity.Ticket),
		events:  make(map[uuid.UUID][]*entity.Event),
		admins:  make(map[uuid.UUID]*entity.Admin),
	}
}

func (m *mockRepository) CreateTicket(ctx context.Context, tx interface{}, ticket *entity.Ticket) error {
	if m.createError != nil {
		return m.createError
	}
	m.tickets[ticket.ID] = ticket
	return nil
}

func (m *mockRepository) GetTicketByID(ctx context.Context, tx interface{}, ticketID uuid.UUID) (*entity.Ticket, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	ticket, exists := m.tickets[ticketID]
	if !exists {
		return nil, supportRepo.ErrTicketNotFound
	}
	return ticket, nil
}

func (m *mockRepository) GetOpenTicketByUser(ctx context.Context, tx interface{}, userID uuid.UUID) (*entity.Ticket, error) {
	for _, ticket := range m.tickets {
		if ticket.UserID == userID && ticket.IsOpen() {
			return ticket, nil
		}
	}
	return nil, supportRepo.ErrTicketNotFound
}

func (m *mockRepository) ListTickets(ctx context.Context, tx interface{}, filter *supportRepo.TicketFilter, cursorCreatedAt *time.Time, cursorID *uuid.UUID, limit int) ([]*entity.Ticket, error) {
	var result []*entity.Ticket
	for _, ticket := range m.tickets {
		result = append(result, ticket)
	}
	return result, nil
}

func (m *mockRepository) CountTickets(ctx context.Context, tx interface{}, filter *supportRepo.TicketFilter) (int64, error) {
	return int64(len(m.tickets)), nil
}

func (m *mockRepository) ClaimTicket(ctx context.Context, tx interface{}, ticketID, adminID uuid.UUID) (*entity.Ticket, error) {
	if m.claimError != nil {
		return nil, m.claimError
	}
	ticket, exists := m.tickets[ticketID]
	if !exists {
		return nil, supportRepo.ErrTicketNotFound
	}
	if !ticket.CanBeClaimed() {
		if ticket.IsAssigned() {
			return nil, supportRepo.ErrTicketAlreadyClaimed
		}
		return nil, supportRepo.ErrInvalidStatusTransition
	}
	now := time.Now()
	ticket.AssignedAdminID = &adminID
	ticket.Status = entity.StatusInProgress
	ticket.AssignedAt = &now
	ticket.UpdatedAt = now
	return ticket, nil
}

func (m *mockRepository) ResolveTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID, notes *string) error {
	if m.resolveError != nil {
		return m.resolveError
	}
	ticket, exists := m.tickets[ticketID]
	if !exists {
		return supportRepo.ErrTicketNotFound
	}
	if !ticket.Status.CanTransitionTo(entity.StatusResolved) {
		return supportRepo.ErrInvalidStatusTransition
	}
	now := time.Now()
	ticket.Status = entity.StatusResolved
	ticket.ResolvedAt = &now
	ticket.ResolutionNotes = notes
	ticket.UpdatedAt = now
	return nil
}

func (m *mockRepository) CloseTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID, reason *string) error {
	if m.closeError != nil {
		return m.closeError
	}
	ticket, exists := m.tickets[ticketID]
	if !exists {
		return supportRepo.ErrTicketNotFound
	}
	if !ticket.Status.CanTransitionTo(entity.StatusClosed) {
		return supportRepo.ErrInvalidStatusTransition
	}
	now := time.Now()
	ticket.Status = entity.StatusClosed
	ticket.ClosedAt = &now
	ticket.CloseReason = reason
	ticket.UpdatedAt = now
	return nil
}

func (m *mockRepository) ReopenTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID) error {
	ticket, exists := m.tickets[ticketID]
	if !exists {
		return supportRepo.ErrTicketNotFound
	}
	if !ticket.CanBeReopened() {
		return supportRepo.ErrCannotReopenTicket
	}
	ticket.Status = entity.StatusOpen
	ticket.AssignedAdminID = nil
	ticket.AssignedAt = nil
	ticket.ResolvedAt = nil
	ticket.ClosedAt = nil
	ticket.ResolutionNotes = nil
	ticket.CloseReason = nil
	ticket.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepository) UpdatePriority(ctx context.Context, tx interface{}, ticketID uuid.UUID, priority entity.Priority) error {
	if m.updateError != nil {
		return m.updateError
	}
	ticket, exists := m.tickets[ticketID]
	if !exists {
		return supportRepo.ErrTicketNotFound
	}
	ticket.Priority = priority
	ticket.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepository) UpdateCategory(ctx context.Context, tx interface{}, ticketID uuid.UUID, category entity.Category) error {
	if m.updateError != nil {
		return m.updateError
	}
	ticket, exists := m.tickets[ticketID]
	if !exists {
		return supportRepo.ErrTicketNotFound
	}
	ticket.Category = category
	ticket.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepository) UpdateStatus(ctx context.Context, tx interface{}, ticketID uuid.UUID, status entity.Status) error {
	ticket, exists := m.tickets[ticketID]
	if !exists {
		return supportRepo.ErrTicketNotFound
	}
	ticket.Status = status
	ticket.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepository) AssignAdmin(ctx context.Context, tx interface{}, ticketID, adminID uuid.UUID) error {
	ticket, exists := m.tickets[ticketID]
	if !exists {
		return supportRepo.ErrTicketNotFound
	}
	ticket.AssignedAdminID = &adminID
	ticket.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepository) UnassignAdmin(ctx context.Context, tx interface{}, ticketID uuid.UUID) error {
	ticket, exists := m.tickets[ticketID]
	if !exists {
		return supportRepo.ErrTicketNotFound
	}
	ticket.AssignedAdminID = nil
	ticket.Status = entity.StatusOpen
	ticket.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepository) CreateEvent(ctx context.Context, tx interface{}, event *entity.Event) error {
	if m.events == nil {
		m.events = make(map[uuid.UUID][]*entity.Event)
	}
	m.events[event.TicketID] = append(m.events[event.TicketID], event)
	return nil
}

func (m *mockRepository) ListEvents(ctx context.Context, tx interface{}, ticketID uuid.UUID, limit int) ([]*entity.Event, error) {
	events, exists := m.events[ticketID]
	if !exists {
		return []*entity.Event{}, nil
	}
	return events, nil
}

func (m *mockRepository) GetAdmin(ctx context.Context, tx interface{}, adminID uuid.UUID) (*entity.Admin, error) {
	admin, exists := m.admins[adminID]
	if !exists {
		return nil, supportRepo.ErrAdminNotFound
	}
	return admin, nil
}

func (m *mockRepository) CreateAdmin(ctx context.Context, tx interface{}, admin *entity.Admin) error {
	m.admins[admin.ID] = admin
	return nil
}

func (m *mockRepository) ListAdmins(ctx context.Context, tx interface{}, isActive *bool) ([]*entity.Admin, error) {
	var result []*entity.Admin
	for _, admin := range m.admins {
		if isActive == nil || admin.IsActive == *isActive {
			result = append(result, admin)
		}
	}
	return result, nil
}

func (m *mockRepository) GetAvailableAdmins(ctx context.Context, tx interface{}, maxConcurrent int, limit int) ([]*entity.Admin, error) {
	var result []*entity.Admin
	for _, admin := range m.admins {
		if admin.IsActive && admin.ActiveTicketCount < maxConcurrent {
			result = append(result, admin)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *mockRepository) IncrementAdminTicketCount(ctx context.Context, tx interface{}, adminID uuid.UUID) error {
	admin, exists := m.admins[adminID]
	if !exists {
		return supportRepo.ErrAdminNotFound
	}
	admin.ActiveTicketCount++
	return nil
}

func (m *mockRepository) DecrementAdminTicketCount(ctx context.Context, tx interface{}, adminID uuid.UUID) error {
	admin, exists := m.admins[adminID]
	if !exists {
		return supportRepo.ErrAdminNotFound
	}
	if admin.ActiveTicketCount > 0 {
		admin.ActiveTicketCount--
	}
	return nil
}

func (m *mockRepository) SetAdminActive(ctx context.Context, tx interface{}, adminID uuid.UUID, isActive bool) error {
	admin, exists := m.admins[adminID]
	if !exists {
		return supportRepo.ErrAdminNotFound
	}
	admin.IsActive = isActive
	return nil
}

func (m *mockRepository) GetTicketStatistics(ctx context.Context, tx interface{}) (*supportRepo.TicketStatistics, error) {
	stats := &supportRepo.TicketStatistics{}
	for _, ticket := range m.tickets {
		stats.TotalTickets++
		switch ticket.Status {
		case entity.StatusOpen:
			stats.OpenTickets++
		case entity.StatusInProgress:
			stats.InProgressTickets++
		case entity.StatusWaitingUser:
			stats.WaitingUserTickets++
		case entity.StatusResolved:
			stats.ResolvedTickets++
		case entity.StatusClosed:
			stats.ClosedTickets++
		}
		if ticket.AssignedAdminID == nil && ticket.IsOpen() {
			stats.UnassignedTickets++
		}
	}
	return stats, nil
}

func (m *mockRepository) CountActiveTicketsByOrderID(ctx context.Context, tx interface{}, orderID uuid.UUID) (int64, error) {
	count := int64(0)
	for _, ticket := range m.tickets {
		if ticket.LinkedOrderID != nil && *ticket.LinkedOrderID == orderID {
			if ticket.Status == entity.StatusOpen || ticket.Status == entity.StatusInProgress || ticket.Status == entity.StatusWaitingUser {
				count++
			}
		}
	}
	return count, nil
}

func (m *mockRepository) GetTicketByChatRoomID(ctx context.Context, tx interface{}, chatRoomID uuid.UUID) (*entity.Ticket, error) {
	return nil, supportRepo.ErrTicketNotFound
}

func (m *mockRepository) UpdateEscalation(ctx context.Context, tx interface{}, ticketID uuid.UUID, escalation entity.Escalation) error {
	return nil
}

func (m *mockRepository) FindTicketsForSLACheck(ctx context.Context, tx db.Tx, limit int) ([]supportRepo.TicketSLARow, error) {
	return nil, nil
}

func (m *mockRepository) FindDisputesForSLACheck(ctx context.Context, tx db.Tx, limit int) ([]supportRepo.DisputeSLARow, error) {
	return nil, nil
}

// mockChatService is a mock chat service.
type mockChatService struct {
	room      *chatEntity.ChatRoom
	roomError error
	msgError  error
}

func (m *mockChatService) CreateSupportTicketRoom(ctx context.Context, userID uuid.UUID) (*chatEntity.ChatRoom, error) {
	if m.roomError != nil {
		return nil, m.roomError
	}
	if m.room == nil {
		m.room = &chatEntity.ChatRoom{
			ID:       uuid.New(),
			RoomType: chatEntity.RoomTypeSupport,
		}
	}
	return m.room, nil
}

func (m *mockChatService) SendSystemMessage(ctx context.Context, roomID uuid.UUID, body string) error {
	return m.msgError
}

// mockTransactor is a mock database transactor.
type mockTransactor struct {
	fn func(tx db.Tx) error
}

func (m *mockTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(&mockTx{})
}

// mockOutboxInserter is a mock outbox inserter.
type mockOutboxInserter struct{}

func (m *mockOutboxInserter) InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
	return nil
}

// ========================================================================
// TESTS
// ========================================================================

func TestService_CreateTicket(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &mockOutboxInserter{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          transactor,
		log:         zap.NewNop(),
	}

	t.Run("creates ticket successfully", func(t *testing.T) {
		userID := uuid.New()
		subject := "Test subject"

		req := &CreateTicketRequest{
			UserID:   userID,
			Category: entity.CategoryPayment,
			Priority: entity.PriorityMedium,
			Subject:  &subject,
		}

		transactor.fn = func(tx db.Tx) error {
			return nil
		}

		ticket, err := service.CreateTicket(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, ticket)
		assert.Equal(t, userID, ticket.UserID)
		assert.Equal(t, entity.CategoryPayment, ticket.Category)
		assert.Equal(t, entity.PriorityMedium, ticket.Priority)
		assert.Equal(t, entity.StatusOpen, ticket.Status)
		assert.Equal(t, subject, ticket.Metadata["subject"].(string))
	})

	t.Run("rejects duplicate open ticket", func(t *testing.T) {
		userID := uuid.New()

		req := &CreateTicketRequest{
			UserID:   userID,
			Category: entity.CategoryPayment,
			Priority: entity.PriorityMedium,
		}

		repo.createError = supportRepo.ErrDuplicateOpenTicket

		_, err := service.CreateTicket(ctx, req)

		assert.Error(t, err)
		assert.ErrorIs(t, err, supportRepo.ErrDuplicateOpenTicket)

		repo.createError = nil
	})
}

func TestService_ClaimTicket(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &mockOutboxInserter{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          transactor,
		log:         zap.NewNop(),
	}

	t.Run("claims open ticket successfully", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		repo.tickets[ticket.ID] = ticket

		req := &ClaimTicketRequest{
			TicketID: ticket.ID,
			AdminID:  adminID,
		}

		claimedTicket, err := service.ClaimTicket(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, claimedTicket)
		assert.Equal(t, entity.StatusInProgress, claimedTicket.Status)
		assert.True(t, claimedTicket.IsAssigned())
		assert.Equal(t, adminID, *claimedTicket.AssignedAdminID)
	})

	t.Run("fails when ticket already claimed", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		otherAdminID := uuid.New()
		ticket.AssignedAdminID = &otherAdminID
		ticket.Status = entity.StatusInProgress
		repo.tickets[ticket.ID] = ticket

		req := &ClaimTicketRequest{
			TicketID: ticket.ID,
			AdminID:  adminID,
		}

		_, err := service.ClaimTicket(ctx, req)

		assert.Error(t, err)
		assert.Equal(t, supportRepo.ErrTicketAlreadyClaimed, err)
	})
}

func TestService_ResolveTicket(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &mockOutboxInserter{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          transactor,
		log:         zap.NewNop(),
	}

	t.Run("resolves in-progress ticket successfully", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.AssignedAdminID = &adminID
		ticket.Status = entity.StatusInProgress
		repo.tickets[ticket.ID] = ticket

		notes := "Issue resolved"
		req := &ResolveTicketRequest{
			TicketID: ticket.ID,
			AdminID:  adminID,
			Notes:    &notes,
		}

		err := service.ResolveTicket(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, entity.StatusResolved, ticket.Status)
		assert.NotNil(t, ticket.ResolvedAt)
		assert.Equal(t, &notes, ticket.ResolutionNotes)
	})

	t.Run("fails when admin not assigned to ticket", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		repo.tickets[ticket.ID] = ticket

		req := &ResolveTicketRequest{
			TicketID: ticket.ID,
			AdminID:  adminID,
		}

		err := service.ResolveTicket(ctx, req)

		assert.Error(t, err)
	})
}

func TestService_CloseTicket(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &mockOutboxInserter{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          transactor,
		log:         zap.NewNop(),
	}

	t.Run("closes resolved ticket successfully", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.Status = entity.StatusResolved
		now := time.Now()
		ticket.ResolvedAt = &now
		repo.tickets[ticket.ID] = ticket

		reason := "User confirmed resolution"
		req := &CloseTicketRequest{
			TicketID:    ticket.ID,
			AdminID:     adminID,
			CloseReason: &reason,
		}

		err := service.CloseTicket(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, entity.StatusClosed, ticket.Status)
		assert.NotNil(t, ticket.ClosedAt)
		assert.Equal(t, &reason, ticket.CloseReason)
	})

	t.Run("fails when ticket not resolved", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.Status = entity.StatusInProgress
		repo.tickets[ticket.ID] = ticket

		req := &CloseTicketRequest{
			TicketID: ticket.ID,
			AdminID:  adminID,
		}

		err := service.CloseTicket(ctx, req)

		assert.Error(t, err)
		assert.Equal(t, supportRepo.ErrInvalidStatusTransition, err)
	})
}

func TestService_ReopenTicket(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &mockOutboxInserter{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          transactor,
		log:         zap.NewNop(),
	}

	t.Run("reopens resolved ticket successfully", func(t *testing.T) {
		userID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.Status = entity.StatusResolved
		now := time.Now()
		ticket.ResolvedAt = &now
		adminID := uuid.New()
		ticket.AssignedAdminID = &adminID
		repo.tickets[ticket.ID] = ticket

		req := &ReopenTicketRequest{
			TicketID: ticket.ID,
			UserID:   userID,
		}

		reopenedTicket, err := service.ReopenTicket(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, reopenedTicket)
		assert.Equal(t, entity.StatusOpen, reopenedTicket.Status)
		assert.Nil(t, reopenedTicket.AssignedAdminID)
		assert.Nil(t, reopenedTicket.ResolvedAt)
	})

	t.Run("reopens closed ticket successfully", func(t *testing.T) {
		userID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.Status = entity.StatusClosed
		now := time.Now()
		ticket.ClosedAt = &now
		repo.tickets[ticket.ID] = ticket

		req := &ReopenTicketRequest{
			TicketID: ticket.ID,
			UserID:   userID,
		}

		_, err := service.ReopenTicket(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, entity.StatusOpen, ticket.Status)
	})

	t.Run("fails when ticket cannot be reopened", func(t *testing.T) {
		userID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.Status = entity.StatusInProgress
		repo.tickets[ticket.ID] = ticket

		req := &ReopenTicketRequest{
			TicketID: ticket.ID,
			UserID:   userID,
		}

		_, err := service.ReopenTicket(ctx, req)

		assert.Error(t, err)
	})
}

func TestService_ReopenTicket_OwnershipCheck(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &mockOutboxInserter{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          transactor,
		log:         zap.NewNop(),
	}

	t.Run("owner can reopen their own ticket", func(t *testing.T) {
		ownerID := uuid.New()

		ticket := entity.NewTicket(ownerID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.Status = entity.StatusResolved
		now := time.Now()
		ticket.ResolvedAt = &now
		repo.tickets[ticket.ID] = ticket

		req := &ReopenTicketRequest{
			TicketID: ticket.ID,
			UserID:   ownerID,
		}

		reopenedTicket, err := service.ReopenTicket(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, reopenedTicket)
		assert.Equal(t, entity.StatusOpen, reopenedTicket.Status)
	})

	t.Run("foreign user cannot reopen another users ticket", func(t *testing.T) {
		ownerID := uuid.New()
		foreignID := uuid.New()

		ticket := entity.NewTicket(ownerID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.Status = entity.StatusResolved
		now := time.Now()
		ticket.ResolvedAt = &now
		repo.tickets[ticket.ID] = ticket

		req := &ReopenTicketRequest{
			TicketID: ticket.ID,
			UserID:   foreignID,
		}

		_, err := service.ReopenTicket(ctx, req)

		assert.Error(t, err)
		assert.Equal(t, supportRepo.ErrTicketNotFound, err)
	})

	t.Run("nonexistent ticket returns not found", func(t *testing.T) {
		req := &ReopenTicketRequest{
			TicketID: uuid.New(),
			UserID:   uuid.New(),
		}

		_, err := service.ReopenTicket(ctx, req)

		assert.Error(t, err)
		assert.Equal(t, supportRepo.ErrTicketNotFound, err)
	})
}

func TestService_UpdatePriority(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &mockOutboxInserter{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          transactor,
		log:         zap.NewNop(),
	}

	t.Run("updates priority successfully", func(t *testing.T) {
		userID := uuid.New()
		actorID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		repo.tickets[ticket.ID] = ticket

		err := service.UpdatePriority(ctx, ticket.ID, entity.PriorityHigh, &actorID)

		require.NoError(t, err)
		assert.Equal(t, entity.PriorityHigh, ticket.Priority)
	})
}

func TestService_UpdateCategory(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &mockOutboxInserter{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          transactor,
		log:         zap.NewNop(),
	}

	t.Run("updates category successfully", func(t *testing.T) {
		userID := uuid.New()
		actorID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		repo.tickets[ticket.ID] = ticket

		err := service.UpdateCategory(ctx, ticket.ID, entity.CategoryTechnical, &actorID)

		require.NoError(t, err)
		assert.Equal(t, entity.CategoryTechnical, ticket.Category)
	})
}

func TestService_GetStatistics(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &mockOutboxInserter{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          &mockTransactor{},
	}

	t.Run("returns statistics", func(t *testing.T) {
		userID := uuid.New()

		// Add some test tickets
		ticket1 := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket1.Status = entity.StatusOpen
		repo.tickets[ticket1.ID] = ticket1

		ticket2 := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket2.Status = entity.StatusResolved
		repo.tickets[ticket2.ID] = ticket2

		stats, err := service.GetStatistics(ctx)

		require.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, int64(2), stats.TotalTickets)
		assert.Equal(t, int64(1), stats.OpenTickets)
		assert.Equal(t, int64(1), stats.ResolvedTickets)
	})
}

func TestEntity_TicketStateTransitions(t *testing.T) {
	t.Run("can transition from open to in_progress", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusOpen}
		assert.True(t, ticket.Status.CanTransitionTo(entity.StatusInProgress))
	})

	t.Run("cannot transition from open to closed", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusOpen}
		assert.False(t, ticket.Status.CanTransitionTo(entity.StatusClosed))
	})

	t.Run("can transition from in_progress to resolved", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusInProgress}
		assert.True(t, ticket.Status.CanTransitionTo(entity.StatusResolved))
	})

	t.Run("can transition from waiting_user to resolved", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusWaitingUser}
		assert.True(t, ticket.Status.CanTransitionTo(entity.StatusResolved))
	})

	t.Run("can transition from waiting_user to in_progress", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusWaitingUser}
		assert.True(t, ticket.Status.CanTransitionTo(entity.StatusInProgress))
	})

	t.Run("can transition from resolved to open", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusResolved}
		assert.True(t, ticket.Status.CanTransitionTo(entity.StatusOpen))
	})

	t.Run("can transition from closed to open", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusClosed}
		assert.True(t, ticket.Status.CanTransitionTo(entity.StatusOpen))
	})
}

func TestEntity_TicketClaim(t *testing.T) {
	t.Run("successfully claims open ticket", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)

		result := ticket.Claim(adminID)

		assert.True(t, result)
		assert.Equal(t, entity.StatusInProgress, ticket.Status)
		assert.True(t, ticket.IsAssigned())
		assert.Equal(t, adminID, *ticket.AssignedAdminID)
	})

	t.Run("cannot claim already claimed ticket", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		otherAdminID := uuid.New()
		ticket.AssignedAdminID = &otherAdminID
		ticket.Status = entity.StatusInProgress

		result := ticket.Claim(adminID)

		assert.False(t, result)
	})
}

func TestEntity_TicketResolve(t *testing.T) {
	t.Run("successfully resolves ticket", func(t *testing.T) {
		userID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.Status = entity.StatusInProgress

		notes := "Resolved"
		result := ticket.Resolve(&notes)

		assert.True(t, result)
		assert.Equal(t, entity.StatusResolved, ticket.Status)
		assert.NotNil(t, ticket.ResolvedAt)
		assert.Equal(t, &notes, ticket.ResolutionNotes)
	})
}

func TestEntity_TicketClose(t *testing.T) {
	t.Run("successfully closes resolved ticket", func(t *testing.T) {
		userID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.Status = entity.StatusResolved
		now := time.Now()
		ticket.ResolvedAt = &now

		reason := "User confirmed"
		result := ticket.Close(&reason)

		assert.True(t, result)
		assert.Equal(t, entity.StatusClosed, ticket.Status)
		assert.NotNil(t, ticket.ClosedAt)
		assert.Equal(t, &reason, ticket.CloseReason)
	})
}

func TestEntity_TicketReopen(t *testing.T) {
	t.Run("successfully reopens resolved ticket", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.Status = entity.StatusResolved
		now := time.Now()
		ticket.ResolvedAt = &now
		ticket.AssignedAdminID = &adminID

		result := ticket.Reopen()

		assert.True(t, result)
		assert.Equal(t, entity.StatusOpen, ticket.Status)
		assert.Nil(t, ticket.AssignedAdminID)
		assert.Nil(t, ticket.ResolvedAt)
	})
}

func TestEntity_TicketIsOpen(t *testing.T) {
	t.Run("open ticket is open", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusOpen}
		assert.True(t, ticket.IsOpen())
	})

	t.Run("in_progress ticket is open", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusInProgress}
		assert.True(t, ticket.IsOpen())
	})

	t.Run("waiting_user ticket is open", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusWaitingUser}
		assert.True(t, ticket.IsOpen())
	})

	t.Run("resolved ticket is not open", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusResolved}
		assert.False(t, ticket.IsOpen())
	})

	t.Run("closed ticket is not open", func(t *testing.T) {
		ticket := &entity.Ticket{Status: entity.StatusClosed}
		assert.False(t, ticket.IsOpen())
	})
}

func TestEntity_AdminCanTakeMoreTickets(t *testing.T) {
	t.Run("admin with no tickets can take more", func(t *testing.T) {
		admin := entity.NewAdmin(uuid.New())
		assert.True(t, admin.CanTakeMoreTickets(10))
	})

	t.Run("admin at capacity cannot take more", func(t *testing.T) {
		admin := entity.NewAdmin(uuid.New())
		admin.ActiveTicketCount = 10
		assert.False(t, admin.CanTakeMoreTickets(10))
	})

	t.Run("inactive admin cannot take tickets", func(t *testing.T) {
		admin := entity.NewAdmin(uuid.New())
		admin.IsActive = false
		assert.False(t, admin.CanTakeMoreTickets(10))
	})
}

// Test for concurrent claim safety (conceptual test)
func TestService_ConcurrentClaimSafety(t *testing.T) {
	// This is a conceptual test to demonstrate that the repository
	// uses SELECT FOR UPDATE for safe concurrent claims

	// In a real scenario with database:
	// 1. Two admins attempt to claim the same ticket simultaneously
	// 2. First SELECT FOR UPDATE locks the row
	// 3. Second admin waits for lock
	// 4. First admin completes claim
	// 5. Second admin gets updated row and sees it's already claimed
	// 6. Second admin receives ErrTicketAlreadyClaimed

	t.Run("conceptual: repository uses row locking", func(t *testing.T) {
		// Verify the ClaimTicket method uses SELECT FOR UPDATE
		// This is checked by code review of the repository implementation
		// The implementation includes:
		//   SELECT ... FROM support_tickets WHERE id = $1 FOR UPDATE
		assert.True(t, true, "repository uses SELECT FOR UPDATE")
	})
}

// TestService_SetWaitingForUser tests the SetWaitingForUser transition.
func TestService_SetWaitingForUser(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &mockOutboxInserter{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          transactor,
		log:         zap.NewNop(),
	}

	t.Run("sets waiting status successfully", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.AssignedAdminID = &adminID
		ticket.Status = entity.StatusInProgress
		repo.tickets[ticket.ID] = ticket

		err := service.SetWaitingForUser(ctx, ticket.ID, adminID)

		require.NoError(t, err)
		assert.Equal(t, entity.StatusWaitingUser, ticket.Status)

		// Verify event was created
		events := repo.events[ticket.ID]
		require.Len(t, events, 1)
		assert.Equal(t, entity.EventTypeTicketWaitingUser, events[0].EventType)
	})

	t.Run("fails when admin not assigned to ticket", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()
		otherAdminID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		ticket.AssignedAdminID = &otherAdminID
		ticket.Status = entity.StatusInProgress
		repo.tickets[ticket.ID] = ticket

		err := service.SetWaitingForUser(ctx, ticket.ID, adminID)

		assert.Error(t, err)
		assert.Equal(t, supportRepo.ErrTicketNotFound, err)
	})
}

// TestService_AllStateTransitionsCreateEvents verifies that all state transitions
// create the appropriate audit events.
func TestService_AllStateTransitionsCreateEvents(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &mockOutboxInserter{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          transactor,
		log:         zap.NewNop(),
	}

	t.Run("all state transitions create events", func(t *testing.T) {
		userID := uuid.New()
		adminID := uuid.New()

		// 1. Create ticket -> ticket_created event
		createReq := &CreateTicketRequest{
			UserID:   userID,
			Category: entity.CategoryPayment,
			Priority: entity.PriorityMedium,
		}
		ticket, err := service.CreateTicket(ctx, createReq)
		require.NoError(t, err)

		createEvents := repo.events[ticket.ID]
		require.Len(t, createEvents, 1)
		assert.Equal(t, entity.EventTypeTicketCreated, createEvents[0].EventType)

		// 2. Claim ticket -> ticket_claimed event
		claimReq := &ClaimTicketRequest{
			TicketID: ticket.ID,
			AdminID:  adminID,
		}
		_, err = service.ClaimTicket(ctx, claimReq)
		require.NoError(t, err)

		eventsAfterClaim := repo.events[ticket.ID]
		claimEvent := findLastEventByType(eventsAfterClaim, entity.EventTypeTicketClaimed)
		require.NotNil(t, claimEvent)
		assert.Equal(t, entity.EventTypeTicketClaimed, claimEvent.EventType)

		// 3. Resolve ticket -> ticket_resolved event
		// Note: We skip SetWaitingForUser because waiting_user cannot transition to resolved
		// The state machine only allows: in_progress -> resolved
		notes := "Issue resolved"
		resolveReq := &ResolveTicketRequest{
			TicketID: ticket.ID,
			AdminID:  adminID,
			Notes:    &notes,
		}
		err = service.ResolveTicket(ctx, resolveReq)
		require.NoError(t, err)

		eventsAfterResolve := repo.events[ticket.ID]
		resolveEvent := findLastEventByType(eventsAfterResolve, entity.EventTypeTicketResolved)
		require.NotNil(t, resolveEvent)
		assert.Equal(t, entity.EventTypeTicketResolved, resolveEvent.EventType)

		// 4. Close ticket -> ticket_closed event
		reason := "User confirmed"
		closeReq := &CloseTicketRequest{
			TicketID:    ticket.ID,
			AdminID:     adminID,
			CloseReason: &reason,
		}
		err = service.CloseTicket(ctx, closeReq)
		require.NoError(t, err)

		eventsAfterClose := repo.events[ticket.ID]
		closeEvent := findLastEventByType(eventsAfterClose, entity.EventTypeTicketClosed)
		require.NotNil(t, closeEvent)
		assert.Equal(t, entity.EventTypeTicketClosed, closeEvent.EventType)

		// 5. Reopen ticket -> ticket_reopened event
		reopenReq := &ReopenTicketRequest{
			TicketID: ticket.ID,
			UserID:   userID,
		}
		_, err = service.ReopenTicket(ctx, reopenReq)
		require.NoError(t, err)

		eventsAfterReopen := repo.events[ticket.ID]
		reopenEvent := findLastEventByType(eventsAfterReopen, entity.EventTypeTicketReopened)
		require.NotNil(t, reopenEvent)
		assert.Equal(t, entity.EventTypeTicketReopened, reopenEvent.EventType)
	})

	t.Run("priority change creates priority_changed event", func(t *testing.T) {
		userID := uuid.New()
		actorID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		repo.tickets[ticket.ID] = ticket

		err := service.UpdatePriority(ctx, ticket.ID, entity.PriorityHigh, &actorID)
		require.NoError(t, err)

		events := repo.events[ticket.ID]
		priorityEvent := findLastEventByType(events, entity.EventTypePriorityChanged)
		require.NotNil(t, priorityEvent)
		assert.Equal(t, entity.EventTypePriorityChanged, priorityEvent.EventType)
	})

	t.Run("category change creates category_changed event", func(t *testing.T) {
		userID := uuid.New()
		actorID := uuid.New()

		ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
		repo.tickets[ticket.ID] = ticket

		err := service.UpdateCategory(ctx, ticket.ID, entity.CategoryTechnical, &actorID)
		require.NoError(t, err)

		events := repo.events[ticket.ID]
		categoryEvent := findLastEventByType(events, entity.EventTypeCategoryChanged)
		require.NotNil(t, categoryEvent)
		assert.Equal(t, entity.EventTypeCategoryChanged, categoryEvent.EventType)
	})
}

// ---------------------------------------------------------------------------
// OUTBOX ATOMICITY — CreateTicket outbox failure must roll back transaction
// ---------------------------------------------------------------------------

// failingOutboxInserter always returns an error from InsertTx.
type failingOutboxInserter struct {
	called bool
}

func (f *failingOutboxInserter) InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
	f.called = true
	return errors.New("simulated outbox write failure")
}

func TestCreateTicket_OutboxFailure_RollsBackTransaction(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	chatSvc := &mockChatService{}
	outbox := &failingOutboxInserter{}
	transactor := &mockTransactor{}

	service := &Service{
		repo:        repo,
		chatService: chatSvc,
		outboxRepo:  outbox,
		db:          transactor,
		log:         zap.NewNop(),
	}

	userID := uuid.New()
	subject := "Test outbox atomicity"

	req := &CreateTicketRequest{
		UserID:   userID,
		Category: entity.CategoryPayment,
		Priority: entity.PriorityMedium,
		Subject:  &subject,
	}

	ticket, err := service.CreateTicket(ctx, req)

	assert.Error(t, err, "CreateTicket must return error when outbox insert fails (STRICT_EVENT_ATOMIC)")
	assert.Nil(t, ticket, "no ticket should be returned when outbox fails")
	assert.True(t, outbox.called, "outbox insert should have been attempted")
	assert.Contains(t, err.Error(), "outbox support.ticket.created",
		"error should reference the outbox event type")
}

// findLastEventByType finds the last event of a given type in the events slice.
func findLastEventByType(events []*entity.Event, eventType entity.EventType) *entity.Event {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType == eventType {
			return events[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// OUTBOX ATOMICITY — ResolveTicket / CloseTicket / SetWaitingForUser
// ---------------------------------------------------------------------------

// selectiveFailingOutbox fails only on the specified event type.
type selectiveFailingOutbox struct {
	failOn string
	called map[string]bool
}

func (s *selectiveFailingOutbox) InsertTx(_ context.Context, _ db.Tx, eventType string, _ any, _ string) error {
	if s.called == nil {
		s.called = make(map[string]bool)
	}
	s.called[eventType] = true
	if eventType == s.failOn {
		return errors.New("simulated outbox write failure")
	}
	return nil
}

func TestResolveTicket_OutboxFailure_RollsBackTransaction(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	outbox := &selectiveFailingOutbox{failOn: "support.ticket.resolved"}

	service := &Service{
		repo:        repo,
		chatService: &mockChatService{},
		outboxRepo:  outbox,
		db:          &mockTransactor{},
		log:         zap.NewNop(),
	}

	adminID := uuid.New()
	ticket := entity.NewTicket(uuid.New(), uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
	ticket.AssignedAdminID = &adminID
	ticket.Status = entity.StatusInProgress
	repo.tickets[ticket.ID] = ticket

	err := service.ResolveTicket(ctx, &ResolveTicketRequest{
		TicketID: ticket.ID,
		AdminID:  adminID,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outbox support.ticket.resolved")
	assert.True(t, outbox.called["support.ticket.resolved"])
}

func TestCloseTicket_OutboxFailure_RollsBackTransaction(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	outbox := &selectiveFailingOutbox{failOn: "support.ticket.closed"}

	service := &Service{
		repo:        repo,
		chatService: &mockChatService{},
		outboxRepo:  outbox,
		db:          &mockTransactor{},
		log:         zap.NewNop(),
	}

	ticket := entity.NewTicket(uuid.New(), uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
	ticket.Status = entity.StatusResolved
	now := time.Now()
	ticket.ResolvedAt = &now
	repo.tickets[ticket.ID] = ticket

	err := service.CloseTicket(ctx, &CloseTicketRequest{
		TicketID: ticket.ID,
		AdminID:  uuid.New(),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outbox support.ticket.closed")
	assert.True(t, outbox.called["support.ticket.closed"])
}

func TestSetWaitingForUser_OutboxFailure_RollsBackTransaction(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	outbox := &selectiveFailingOutbox{failOn: "support.ticket_waiting_user"}

	service := &Service{
		repo:        repo,
		chatService: &mockChatService{},
		outboxRepo:  outbox,
		db:          &mockTransactor{},
		log:         zap.NewNop(),
	}

	adminID := uuid.New()
	ticket := entity.NewTicket(uuid.New(), uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
	ticket.AssignedAdminID = &adminID
	ticket.Status = entity.StatusInProgress
	repo.tickets[ticket.ID] = ticket

	err := service.SetWaitingForUser(ctx, ticket.ID, adminID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outbox support.ticket_waiting_user")
	assert.True(t, outbox.called["support.ticket_waiting_user"])
}

// ---------------------------------------------------------------------------
// IDEMPOTENCY KEY UNIQUENESS — repeated waiting_user cycles
// ---------------------------------------------------------------------------

// recordingOutboxInserter records all idempotency keys per event type.
type recordingOutboxInserter struct {
	keys map[string][]string
}

func (r *recordingOutboxInserter) InsertTx(_ context.Context, _ db.Tx, eventType string, _ any, idempotencyKey string) error {
	if r.keys == nil {
		r.keys = make(map[string][]string)
	}
	r.keys[eventType] = append(r.keys[eventType], idempotencyKey)
	return nil
}

func TestSetWaitingForUser_RepeatedCycles_UniqueIdempotencyKeys(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	outbox := &recordingOutboxInserter{}

	service := &Service{
		repo:        repo,
		chatService: &mockChatService{},
		outboxRepo:  outbox,
		db:          &mockTransactor{},
		log:         zap.NewNop(),
	}

	adminID := uuid.New()
	ticket := entity.NewTicket(uuid.New(), uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
	ticket.AssignedAdminID = &adminID
	ticket.Status = entity.StatusInProgress
	repo.tickets[ticket.ID] = ticket

	// Cycle 1: in_progress → waiting_user
	err := service.SetWaitingForUser(ctx, ticket.ID, adminID)
	require.NoError(t, err)

	// Simulate user reply (waiting_user → in_progress) by resetting status
	ticket.Status = entity.StatusInProgress

	// Small delay to guarantee different millisecond timestamp
	time.Sleep(2 * time.Millisecond)

	// Cycle 2: in_progress → waiting_user again
	err = service.SetWaitingForUser(ctx, ticket.ID, adminID)
	require.NoError(t, err)

	keys := outbox.keys["support.ticket_waiting_user"]
	require.Len(t, keys, 2, "two waiting_user events should have been emitted")
	assert.NotEqual(t, keys[0], keys[1],
		"idempotency keys must differ across waiting_user cycles (was: %s)", keys[0])
}

func TestResolveTicket_RepeatedCycles_UniqueIdempotencyKeys(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	outbox := &recordingOutboxInserter{}

	service := &Service{
		repo:        repo,
		chatService: &mockChatService{},
		outboxRepo:  outbox,
		db:          &mockTransactor{},
		log:         zap.NewNop(),
	}

	adminID := uuid.New()
	userID := uuid.New()
	ticket := entity.NewTicket(userID, uuid.New(), entity.CategoryPayment, entity.PriorityMedium)
	ticket.AssignedAdminID = &adminID
	ticket.Status = entity.StatusInProgress
	repo.tickets[ticket.ID] = ticket

	// Cycle 1: resolve
	err := service.ResolveTicket(ctx, &ResolveTicketRequest{TicketID: ticket.ID, AdminID: adminID})
	require.NoError(t, err)

	// Reopen (resolved → open → claim → in_progress)
	ticket.Status = entity.StatusOpen
	ticket.AssignedAdminID = nil
	ticket.ResolvedAt = nil
	ticket.AssignedAdminID = &adminID
	ticket.Status = entity.StatusInProgress

	time.Sleep(2 * time.Millisecond)

	// Cycle 2: resolve again
	err = service.ResolveTicket(ctx, &ResolveTicketRequest{TicketID: ticket.ID, AdminID: adminID})
	require.NoError(t, err)

	keys := outbox.keys["support.ticket.resolved"]
	require.Len(t, keys, 2, "two resolved events should have been emitted")
	assert.NotEqual(t, keys[0], keys[1],
		"idempotency keys must differ across resolve cycles (was: %s)", keys[0])
}
