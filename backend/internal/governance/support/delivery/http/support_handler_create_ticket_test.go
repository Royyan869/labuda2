package http

// SUPPORT SCOPE 1 — canonical Support ticket creation HTTP contract.
//
// Proves the user-facing creation endpoint:
//   - rejects unauthenticated callers,
//   - enforces the canonical category taxonomy (legacy values rejected),
//   - accepts every canonical category and persists it verbatim,
//   - binds subject/description and the linked order reference,
//   - refuses a linked order the caller does not participate in.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	supportApp "github.com/labuda/backend/internal/governance/support/application"
	supportEntity "github.com/labuda/backend/internal/governance/support/entity"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// createTicketRecordingRepo records the ticket handed to CreateTicket.
type createTicketRecordingRepo struct {
	ownershipMockRepo
	created *supportEntity.Ticket
}

func (m *createTicketRecordingRepo) CreateTicket(ctx context.Context, tx interface{}, ticket *supportEntity.Ticket) error {
	m.created = ticket
	return nil
}

// createTicketChatService always provisions a fresh support room.
type createTicketChatService struct {
	room *chatEntity.ChatRoom
}

func (m *createTicketChatService) CreateSupportTicketRoom(ctx context.Context, userID uuid.UUID) (*chatEntity.ChatRoom, error) {
	m.room = &chatEntity.ChatRoom{
		ID:           uuid.New(),
		RoomType:     chatEntity.RoomTypeSupport,
		ParticipantA: userID,
	}
	return m.room, nil
}

func (m *createTicketChatService) SendSystemMessage(ctx context.Context, roomID uuid.UUID, body string) error {
	return nil
}

// fixedOrderEscrowService returns a canned buyer/seller pair for order checks.
type fixedOrderEscrowService struct {
	buyerID  uuid.UUID
	sellerID uuid.UUID
	err      error
}

func (m *fixedOrderEscrowService) GetOrderForValidation(ctx context.Context, orderID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	if m.err != nil {
		return uuid.Nil, uuid.Nil, m.err
	}
	return m.buyerID, m.sellerID, nil
}

func setupCreateTicketRouter(handler *Handler, userID *uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if userID != nil {
		uid := *userID
		router.Use(func(c *gin.Context) {
			c.Set("userID", uid)
			c.Next()
		})
	}
	router.POST("/support/tickets", handler.CreateTicket)
	return router
}

func newCreateTicketHandler(repo supportRepo.Repository, orderService supportApp.OrderEscrowService) *Handler {
	svc := supportApp.NewService(
		&ownershipMockTransactor{},
		repo,
		&createTicketChatService{},
		&ownershipMockOutboxInserter{},
		orderService,
		nil, // dispute service
		zap.NewNop(),
	)
	return &Handler{supportService: svc, log: zap.NewNop()}
}

func TestHandler_CreateTicket_RequiresAuthentication(t *testing.T) {
	repo := &createTicketRecordingRepo{}
	handler := newCreateTicketHandler(repo, &ownershipMockOrderEscrowService{})
	router := setupCreateTicketRouter(handler, nil)

	req, _ := http.NewRequest("POST", "/support/tickets",
		strings.NewReader(`{"category":"order_issue"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Nil(t, repo.created, "no ticket may be created without authentication")
}

func TestHandler_CreateTicket_RejectsLegacyCategory(t *testing.T) {
	// Every legacy (pre-canonical) category value must be rejected by the
	// request binding, never silently translated.
	for _, legacy := range []string{"payment", "order", "technical", "account", "general"} {
		t.Run(legacy, func(t *testing.T) {
			userID := uuid.New()
			repo := &createTicketRecordingRepo{}
			handler := newCreateTicketHandler(repo, &ownershipMockOrderEscrowService{})
			router := setupCreateTicketRouter(handler, &userID)

			req, _ := http.NewRequest("POST", "/support/tickets",
				strings.NewReader(`{"category":"`+legacy+`"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code,
				"legacy category %q must be rejected", legacy)
			assert.Nil(t, repo.created)
		})
	}
}

func TestHandler_CreateTicket_RejectsMissingCategory(t *testing.T) {
	userID := uuid.New()
	repo := &createTicketRecordingRepo{}
	handler := newCreateTicketHandler(repo, &ownershipMockOrderEscrowService{})
	router := setupCreateTicketRouter(handler, &userID)

	req, _ := http.NewRequest("POST", "/support/tickets", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Nil(t, repo.created)
}

func TestHandler_CreateTicket_AcceptsEveryCanonicalCategory(t *testing.T) {
	for _, category := range supportEntity.AllCategories {
		t.Run(category.String(), func(t *testing.T) {
			userID := uuid.New()
			repo := &createTicketRecordingRepo{}
			handler := newCreateTicketHandler(repo, &ownershipMockOrderEscrowService{})
			router := setupCreateTicketRouter(handler, &userID)

			body := `{"category":"` + category.String() + `","priority":"high",` +
				`"subject":"Cannot checkout","description":"Payment hangs"}`
			req, _ := http.NewRequest("POST", "/support/tickets", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusCreated, w.Code, "response: %s", w.Body.String())
			require.NotNil(t, repo.created, "ticket must be created")
			assert.Equal(t, category, repo.created.Category)
			assert.Equal(t, supportEntity.PriorityHigh, repo.created.Priority)
			assert.Equal(t, supportEntity.StatusOpen, repo.created.Status)
			assert.Equal(t, userID, repo.created.UserID, "owner comes from auth context")
			require.NotNil(t, repo.created.Subject)
			assert.Equal(t, "Cannot checkout", *repo.created.Subject)
			require.NotNil(t, repo.created.Description)
			assert.Equal(t, "Payment hangs", *repo.created.Description)
			assert.NotEqual(t, uuid.Nil, repo.created.ChatRoomID,
				"ticket must reference a ticket-specific conversation")
		})
	}
}

func TestHandler_CreateTicket_LinkedOrderOwnershipEnforced(t *testing.T) {
	userID := uuid.New()
	orderID := uuid.New()

	t.Run("linked order owned by the caller is accepted", func(t *testing.T) {
		repo := &createTicketRecordingRepo{}
		orderService := &fixedOrderEscrowService{buyerID: userID, sellerID: uuid.New()}
		handler := newCreateTicketHandler(repo, orderService)
		router := setupCreateTicketRouter(handler, &userID)

		body := `{"category":"order_issue","linked_order_id":"` + orderID.String() + `"}`
		req, _ := http.NewRequest("POST", "/support/tickets", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code, "response: %s", w.Body.String())
		require.NotNil(t, repo.created)
		require.NotNil(t, repo.created.LinkedOrderID)
		assert.Equal(t, orderID, *repo.created.LinkedOrderID)
	})

	t.Run("linked order owned by someone else is refused", func(t *testing.T) {
		repo := &createTicketRecordingRepo{}
		orderService := &fixedOrderEscrowService{buyerID: uuid.New(), sellerID: uuid.New()}
		handler := newCreateTicketHandler(repo, orderService)
		router := setupCreateTicketRouter(handler, &userID)

		body := `{"category":"order_issue","linked_order_id":"` + orderID.String() + `"}`
		req, _ := http.NewRequest("POST", "/support/tickets", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusCreated, w.Code)
		assert.Nil(t, repo.created, "no ticket may be created for an order the caller does not own")
	})
}

// guard against accidental removal of the wire-level lifecycle of a created
// ticket: a freshly created ticket must always be in the canonical `open` state.
func TestHandler_CreateTicket_PersistsCanonicalLifecycle(t *testing.T) {
	userID := uuid.New()
	repo := &createTicketRecordingRepo{}
	handler := newCreateTicketHandler(repo, &ownershipMockOrderEscrowService{})
	router := setupCreateTicketRouter(handler, &userID)

	req, _ := http.NewRequest("POST", "/support/tickets",
		strings.NewReader(`{"category":"other"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, repo.created)
	assert.Equal(t, supportEntity.StatusOpen, repo.created.Status)
	assert.WithinDuration(t, time.Now(), repo.created.CreatedAt, 5*time.Second)
}
