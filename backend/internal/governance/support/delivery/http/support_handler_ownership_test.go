package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	supportApp "github.com/labuda/backend/internal/governance/support/application"
	supportEntity "github.com/labuda/backend/internal/governance/support/entity"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Mocks (self-contained — no dependency on integration-tagged test files)
// ---------------------------------------------------------------------------

type ownershipMockTx struct{}

func (m *ownershipMockTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *ownershipMockTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *ownershipMockTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return &ownershipMockRow{}
}
func (m *ownershipMockTx) Commit(ctx context.Context) error   { return nil }
func (m *ownershipMockTx) Rollback(ctx context.Context) error { return nil }

var _ db.Tx = (*ownershipMockTx)(nil)

type ownershipMockRow struct{}

func (m *ownershipMockRow) Scan(dest ...any) error { return nil }

type ownershipMockTransactor struct{}

func (m *ownershipMockTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(&ownershipMockTx{})
}

type ownershipMockRepo struct {
	ticket *supportEntity.Ticket
}

func (m *ownershipMockRepo) CreateTicket(ctx context.Context, tx interface{}, ticket *supportEntity.Ticket) error {
	return nil
}
func (m *ownershipMockRepo) GetTicketByID(ctx context.Context, tx interface{}, ticketID uuid.UUID) (*supportEntity.Ticket, error) {
	if m.ticket != nil {
		return m.ticket, nil
	}
	return nil, supportRepo.ErrTicketNotFound
}
func (m *ownershipMockRepo) GetOpenTicketByUser(ctx context.Context, tx interface{}, userID uuid.UUID) (*supportEntity.Ticket, error) {
	return nil, supportRepo.ErrTicketNotFound
}
func (m *ownershipMockRepo) ListTickets(ctx context.Context, tx interface{}, filter *supportRepo.TicketFilter, cursorCreatedAt *time.Time, cursorID *uuid.UUID, limit int) ([]*supportEntity.Ticket, error) {
	return nil, nil
}
func (m *ownershipMockRepo) CountTickets(ctx context.Context, tx interface{}, filter *supportRepo.TicketFilter) (int64, error) {
	return 0, nil
}
func (m *ownershipMockRepo) ClaimTicket(ctx context.Context, tx interface{}, ticketID, adminID uuid.UUID) (*supportEntity.Ticket, error) {
	return nil, supportRepo.ErrTicketNotFound
}
func (m *ownershipMockRepo) ResolveTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID, notes *string) error {
	return nil
}
func (m *ownershipMockRepo) CloseTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID, reason *string) error {
	return nil
}
func (m *ownershipMockRepo) ReopenTicket(ctx context.Context, tx interface{}, ticketID uuid.UUID) error {
	return nil
}
func (m *ownershipMockRepo) UpdatePriority(ctx context.Context, tx interface{}, ticketID uuid.UUID, priority supportEntity.Priority) error {
	return nil
}
func (m *ownershipMockRepo) UpdateCategory(ctx context.Context, tx interface{}, ticketID uuid.UUID, category supportEntity.Category) error {
	return nil
}
func (m *ownershipMockRepo) UpdateStatus(ctx context.Context, tx interface{}, ticketID uuid.UUID, status supportEntity.Status) error {
	return nil
}
func (m *ownershipMockRepo) UpdateEscalation(ctx context.Context, tx interface{}, ticketID uuid.UUID, escalation supportEntity.Escalation) error {
	return nil
}
func (m *ownershipMockRepo) AssignAdmin(ctx context.Context, tx interface{}, ticketID, adminID uuid.UUID) error {
	return nil
}
func (m *ownershipMockRepo) UnassignAdmin(ctx context.Context, tx interface{}, ticketID uuid.UUID) error {
	return nil
}
func (m *ownershipMockRepo) CreateEvent(ctx context.Context, tx interface{}, event *supportEntity.Event) error {
	return nil
}
func (m *ownershipMockRepo) ListEvents(ctx context.Context, tx interface{}, ticketID uuid.UUID, limit int) ([]*supportEntity.Event, error) {
	return nil, nil
}
func (m *ownershipMockRepo) GetAdmin(ctx context.Context, tx interface{}, adminID uuid.UUID) (*supportEntity.Admin, error) {
	return nil, nil
}
func (m *ownershipMockRepo) CreateAdmin(ctx context.Context, tx interface{}, admin *supportEntity.Admin) error {
	return nil
}
func (m *ownershipMockRepo) ListAdmins(ctx context.Context, tx interface{}, isActive *bool) ([]*supportEntity.Admin, error) {
	return nil, nil
}
func (m *ownershipMockRepo) GetAvailableAdmins(ctx context.Context, tx interface{}, maxConcurrent int, limit int) ([]*supportEntity.Admin, error) {
	return nil, nil
}
func (m *ownershipMockRepo) IncrementAdminTicketCount(ctx context.Context, tx interface{}, adminID uuid.UUID) error {
	return nil
}
func (m *ownershipMockRepo) DecrementAdminTicketCount(ctx context.Context, tx interface{}, adminID uuid.UUID) error {
	return nil
}
func (m *ownershipMockRepo) SetAdminActive(ctx context.Context, tx interface{}, adminID uuid.UUID, isActive bool) error {
	return nil
}
func (m *ownershipMockRepo) GetTicketStatistics(ctx context.Context, tx interface{}) (*supportRepo.TicketStatistics, error) {
	return nil, nil
}
func (m *ownershipMockRepo) CountActiveTicketsByOrderID(ctx context.Context, tx interface{}, orderID uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *ownershipMockRepo) GetTicketByChatRoomID(ctx context.Context, tx interface{}, chatRoomID uuid.UUID) (*supportEntity.Ticket, error) {
	return nil, supportRepo.ErrTicketNotFound
}
func (m *ownershipMockRepo) FindTicketsForSLACheck(ctx context.Context, tx db.Tx, limit int) ([]supportRepo.TicketSLARow, error) {
	return nil, nil
}
func (m *ownershipMockRepo) FindDisputesForSLACheck(ctx context.Context, tx db.Tx, limit int) ([]supportRepo.DisputeSLARow, error) {
	return nil, nil
}

type ownershipMockChatService struct{}

func (m *ownershipMockChatService) CreateSupportTicketRoom(ctx context.Context, userID uuid.UUID) (*chatEntity.ChatRoom, error) {
	return nil, nil
}
func (m *ownershipMockChatService) SendSystemMessage(ctx context.Context, roomID uuid.UUID, body string) error {
	return nil
}

type ownershipMockOutboxInserter struct{}

func (m *ownershipMockOutboxInserter) InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
	return nil
}

type ownershipMockOrderEscrowService struct{}

func (m *ownershipMockOrderEscrowService) GetOrderForValidation(ctx context.Context, orderID uuid.UUID) (buyerID, sellerID uuid.UUID, err error) {
	return uuid.Nil, uuid.Nil, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// setupUserRouter creates a gin router that injects userID into context,
// mimicking the auth middleware for user-facing endpoints.
func setupUserRouter(handler *Handler, userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})

	router.GET("/support/tickets/:id", handler.GetTicket)
	router.GET("/support/tickets/:id/events", handler.ListEvents)
	router.PUT("/support/tickets/:id/reopen", handler.ReopenTicket)

	return router
}

// createOwnershipTestService creates a service backed by ownershipMockRepo.
func createOwnershipTestService(repo *ownershipMockRepo) *supportApp.Service {
	return supportApp.NewService(
		&ownershipMockTransactor{},
		repo,
		&ownershipMockChatService{},
		&ownershipMockOutboxInserter{},
		&ownershipMockOrderEscrowService{},
		nil, // disputeService — not needed for these tests
		zap.NewNop(),
	)
}

// ==========================================================================
// GET /support/tickets/:id — ownership enforcement
// ==========================================================================

func TestHandler_GetTicket_OwnershipEnforcement(t *testing.T) {
	ownerID := uuid.New()
	foreignID := uuid.New()
	ticketID := uuid.New()

	ticket := supportEntity.NewTicket(ownerID, uuid.New(), supportEntity.CategoryPayment, supportEntity.PriorityMedium)
	ticket.ID = ticketID

	t.Run("owner can read their own ticket", func(t *testing.T) {
		repo := &ownershipMockRepo{ticket: ticket}
		service := createOwnershipTestService(repo)
		handler := &Handler{supportService: service, log: zap.NewNop()}
		router := setupUserRouter(handler, ownerID)

		req, _ := http.NewRequest("GET", "/support/tickets/"+ticketID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("foreign user gets 404 for another users ticket", func(t *testing.T) {
		repo := &ownershipMockRepo{ticket: ticket}
		service := createOwnershipTestService(repo)
		handler := &Handler{supportService: service, log: zap.NewNop()}
		router := setupUserRouter(handler, foreignID)

		req, _ := http.NewRequest("GET", "/support/tickets/"+ticketID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("nonexistent ticket returns 404", func(t *testing.T) {
		repo := &ownershipMockRepo{ticket: nil}
		service := createOwnershipTestService(repo)
		handler := &Handler{supportService: service, log: zap.NewNop()}
		router := setupUserRouter(handler, ownerID)

		req, _ := http.NewRequest("GET", "/support/tickets/"+uuid.New().String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// ==========================================================================
// GET /support/tickets/:id/events — ownership enforcement
// ==========================================================================

func TestHandler_ListEvents_OwnershipEnforcement(t *testing.T) {
	ownerID := uuid.New()
	foreignID := uuid.New()
	ticketID := uuid.New()

	ticket := supportEntity.NewTicket(ownerID, uuid.New(), supportEntity.CategoryPayment, supportEntity.PriorityMedium)
	ticket.ID = ticketID

	t.Run("owner can list events for their own ticket", func(t *testing.T) {
		repo := &ownershipMockRepo{ticket: ticket}
		service := createOwnershipTestService(repo)
		handler := &Handler{supportService: service, log: zap.NewNop()}
		router := setupUserRouter(handler, ownerID)

		req, _ := http.NewRequest("GET", "/support/tickets/"+ticketID.String()+"/events", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("foreign user gets 404 for another users ticket events", func(t *testing.T) {
		repo := &ownershipMockRepo{ticket: ticket}
		service := createOwnershipTestService(repo)
		handler := &Handler{supportService: service, log: zap.NewNop()}
		router := setupUserRouter(handler, foreignID)

		req, _ := http.NewRequest("GET", "/support/tickets/"+ticketID.String()+"/events", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("nonexistent ticket returns 404", func(t *testing.T) {
		repo := &ownershipMockRepo{ticket: nil}
		service := createOwnershipTestService(repo)
		handler := &Handler{supportService: service, log: zap.NewNop()}
		router := setupUserRouter(handler, ownerID)

		req, _ := http.NewRequest("GET", "/support/tickets/"+uuid.New().String()+"/events", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// ==========================================================================
// PUT /support/tickets/:id/reopen — ownership enforcement
// ==========================================================================

func TestHandler_ReopenTicket_OwnershipEnforcement(t *testing.T) {
	ownerID := uuid.New()
	foreignID := uuid.New()
	ticketID := uuid.New()

	t.Run("owner can reopen their own resolved ticket", func(t *testing.T) {
		ticket := supportEntity.NewTicket(ownerID, uuid.New(), supportEntity.CategoryPayment, supportEntity.PriorityMedium)
		ticket.ID = ticketID
		ticket.Status = supportEntity.StatusResolved

		repo := &ownershipMockRepo{ticket: ticket}
		service := createOwnershipTestService(repo)
		handler := &Handler{supportService: service, log: zap.NewNop()}
		router := setupUserRouter(handler, ownerID)

		req, _ := http.NewRequest("PUT", "/support/tickets/"+ticketID.String()+"/reopen", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Service-level ownership check passes, repo mock allows reopen.
		// The key assertion is that it does NOT return 404 for the owner.
		assert.NotEqual(t, http.StatusNotFound, w.Code)
	})

	t.Run("foreign user gets 404 when reopening another users ticket", func(t *testing.T) {
		ticket := supportEntity.NewTicket(ownerID, uuid.New(), supportEntity.CategoryPayment, supportEntity.PriorityMedium)
		ticket.ID = ticketID
		ticket.Status = supportEntity.StatusResolved

		repo := &ownershipMockRepo{ticket: ticket}
		service := createOwnershipTestService(repo)
		handler := &Handler{supportService: service, log: zap.NewNop()}
		router := setupUserRouter(handler, foreignID)

		req, _ := http.NewRequest("PUT", "/support/tickets/"+ticketID.String()+"/reopen", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
