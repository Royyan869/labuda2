package http

// SUPPORT SCOPE 2 — canonical Support conversation HTTP contract.
//
// Proves the conversation endpoints:
//   - resolve ticket → ticket-specific room;
//   - enforce ticket ownership for users (read AND write);
//   - enforce Support capability for agents;
//   - persist the authenticated actor as sender (never client-supplied,
//     never uuid.Nil, never a system message for a human reply);
//   - expose a canonical sender_type (user/admin/system) on the response;
//   - emit the user notification event when an agent replies.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	supportApp "github.com/labuda/backend/internal/governance/support/application"
	supportEntity "github.com/labuda/backend/internal/governance/support/entity"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type sentSupportMessage struct {
	roomID   uuid.UUID
	senderID uuid.UUID
	body     string
}

type recordingChatMessageService struct {
	messages []*chatEntity.ChatMessage
	sent     []sentSupportMessage
}

func (m *recordingChatMessageService) ListSupportMessages(
	ctx context.Context,
	roomID uuid.UUID,
	cursorCreatedAt *time.Time,
	cursorID *uuid.UUID,
	limit int,
) ([]*chatEntity.ChatMessage, error) {
	return m.messages, nil
}

func (m *recordingChatMessageService) SendSupportMessage(
	ctx context.Context,
	roomID, senderID uuid.UUID,
	body string,
	idempotencyKey string,
) (*chatEntity.ChatMessage, error) {
	m.sent = append(m.sent, sentSupportMessage{roomID: roomID, senderID: senderID, body: body})
	return &chatEntity.ChatMessage{
		ID:          uuid.New(),
		RoomID:      roomID,
		SenderID:    senderID,
		MessageType: chatEntity.MessageTypeText,
		Body:        &body,
	}, nil
}

type recordingOutboxInserterSupport struct {
	events []string
}

func (m *recordingOutboxInserterSupport) InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
	m.events = append(m.events, eventType)
	return nil
}

type noopAdminAuditLogger struct{}

func (noopAdminAuditLogger) LogSafe(ctx context.Context, actorID uuid.UUID, actionType, targetType string, targetID uuid.UUID, metadata map[string]interface{}) {
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type conversationFixture struct {
	handler    *Handler
	repo       *ownershipMockRepo
	chatMsgSvc *recordingChatMessageService
	outbox     *recordingOutboxInserterSupport
	service    *supportApp.Service
	ownerID    uuid.UUID
	adminID    uuid.UUID
	ticket     *supportEntity.Ticket
}

func newConversationFixture(t *testing.T) *conversationFixture {
	t.Helper()

	ownerID := uuid.New()
	adminID := uuid.New()
	ticket := supportEntity.NewTicket(ownerID, uuid.New(), supportEntity.CategoryOrderIssue, supportEntity.PriorityMedium)
	ticket.ID = uuid.New()
	// Assigned + in_progress so an agent reply drives the canonical
	// waiting_user notification transition.
	ticket.AssignedAdminID = &adminID
	ticket.Status = supportEntity.StatusInProgress

	repo := &ownershipMockRepo{ticket: ticket}
	chatMsgSvc := &recordingChatMessageService{}
	outbox := &recordingOutboxInserterSupport{}

	svc := supportApp.NewService(
		&ownershipMockTransactor{},
		repo,
		&ownershipMockChatService{},
		outbox,
		&ownershipMockOrderEscrowService{},
		nil,
		zap.NewNop(),
	)

	handler := &Handler{
		supportService:     svc,
		chatMessageService: chatMsgSvc,
		log:                zap.NewNop(),
		adminAuditLogger:   noopAdminAuditLogger{},
	}

	return &conversationFixture{
		handler:    handler,
		repo:       repo,
		chatMsgSvc: chatMsgSvc,
		outbox:     outbox,
		service:    svc,
		ownerID:    ownerID,
		adminID:    adminID,
		ticket:     ticket,
	}
}

func conversationUserRouter(handler *Handler, userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})
	router.GET("/support/tickets/:id/messages", handler.ListMessages)
	router.POST("/support/tickets/:id/messages", handler.SendMyMessage)
	return router
}

func conversationAdminRouter(handler *Handler, adminID uuid.UUID, caps ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		actor := &capabilityEntity.Actor{ID: adminID, Role: "admin", Capabilities: caps}
		c.Request = c.Request.WithContext(capability.WithActor(c.Request.Context(), actor))
		c.Set("userID", adminID)
		c.Next()
	})
	router.GET("/admin/support/tickets/:id/messages", handler.AdminListMessages)
	router.POST("/admin/support/tickets/:id/messages", handler.SendMessage)
	return router
}

func decodeMessageEnvelope(t *testing.T, body []byte) []map[string]interface{} {
	t.Helper()
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Data []map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Data.Data
}

func newTextMessage(roomID, senderID uuid.UUID, body string) *chatEntity.ChatMessage {
	msg := chatEntity.NewChatMessage(roomID, senderID, chatEntity.MessageTypeText, &body, nil, uuid.New().String())
	msg.ID = uuid.New()
	return msg
}

// ---------------------------------------------------------------------------
// User read
// ---------------------------------------------------------------------------

func TestConversation_UserRead_OwnershipEnforced(t *testing.T) {
	f := newConversationFixture(t)
	f.chatMsgSvc.messages = []*chatEntity.ChatMessage{
		newTextMessage(f.ticket.ChatRoomID, f.ownerID, "user hello"),
		newTextMessage(f.ticket.ChatRoomID, f.adminID, "agent reply"),
	}

	t.Run("owner reads own conversation with canonical sender types", func(t *testing.T) {
		router := conversationUserRouter(f.handler, f.ownerID)
		req, _ := http.NewRequest("GET", "/support/tickets/"+f.ticket.ID.String()+"/messages", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		messages := decodeMessageEnvelope(t, w.Body.Bytes())
		require.Len(t, messages, 2)
		assert.Equal(t, "user", messages[0]["sender_type"])
		assert.Equal(t, "admin", messages[1]["sender_type"])
		assert.Equal(t, f.ownerID.String(), messages[0]["sender_id"])
		assert.Equal(t, f.adminID.String(), messages[1]["sender_id"])
	})

	t.Run("foreign user cannot read someone else's conversation", func(t *testing.T) {
		router := conversationUserRouter(f.handler, uuid.New())
		req, _ := http.NewRequest("GET", "/support/tickets/"+f.ticket.ID.String()+"/messages", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("unauthenticated read is rejected", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/support/tickets/:id/messages", f.handler.ListMessages)
		req, _ := http.NewRequest("GET", "/support/tickets/"+f.ticket.ID.String()+"/messages", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// ---------------------------------------------------------------------------
// User send
// ---------------------------------------------------------------------------

func TestConversation_UserSend_UsesAuthSender(t *testing.T) {
	f := newConversationFixture(t)

	t.Run("owner reply is persisted with the authenticated sender", func(t *testing.T) {
		router := conversationUserRouter(f.handler, f.ownerID)
		req, _ := http.NewRequest("POST", "/support/tickets/"+f.ticket.ID.String()+"/messages",
			strings.NewReader(`{"message":"sudah saya coba"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Len(t, f.chatMsgSvc.sent, 1)
		assert.Equal(t, f.ticket.ChatRoomID, f.chatMsgSvc.sent[0].roomID)
		assert.Equal(t, f.ownerID, f.chatMsgSvc.sent[0].senderID,
			"sender must come from the auth context")
		assert.Equal(t, "sudah saya coba", f.chatMsgSvc.sent[0].body)
	})

	t.Run("client-supplied sender identity is ignored", func(t *testing.T) {
		f := newConversationFixture(t)
		router := conversationUserRouter(f.handler, f.ownerID)
		req, _ := http.NewRequest("POST", "/support/tickets/"+f.ticket.ID.String()+"/messages",
			strings.NewReader(`{"message":"hi","sender_id":"`+f.adminID.String()+`","senderId":"`+f.adminID.String()+`"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Len(t, f.chatMsgSvc.sent, 1)
		assert.Equal(t, f.ownerID, f.chatMsgSvc.sent[0].senderID,
			"a forged sender identity must never be honored")
	})

	t.Run("foreign user cannot write to someone else's conversation", func(t *testing.T) {
		f := newConversationFixture(t)
		router := conversationUserRouter(f.handler, uuid.New())
		req, _ := http.NewRequest("POST", "/support/tickets/"+f.ticket.ID.String()+"/messages",
			strings.NewReader(`{"message":"intrusion"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Empty(t, f.chatMsgSvc.sent)
	})

	t.Run("empty message is rejected", func(t *testing.T) {
		f := newConversationFixture(t)
		router := conversationUserRouter(f.handler, f.ownerID)
		req, _ := http.NewRequest("POST", "/support/tickets/"+f.ticket.ID.String()+"/messages",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, f.chatMsgSvc.sent)
	})
}

// ---------------------------------------------------------------------------
// Admin read
// ---------------------------------------------------------------------------

func TestConversation_AdminRead_RequiresCapability(t *testing.T) {
	f := newConversationFixture(t)
	f.chatMsgSvc.messages = []*chatEntity.ChatMessage{
		newTextMessage(f.ticket.ChatRoomID, f.ownerID, "user hello"),
	}

	t.Run("agent with support.ticket.read can read the conversation", func(t *testing.T) {
		router := conversationAdminRouter(f.handler, f.adminID, capability.CapSupportTicketRead.String())
		req, _ := http.NewRequest("GET", "/admin/support/tickets/"+f.ticket.ID.String()+"/messages", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		messages := decodeMessageEnvelope(t, w.Body.Bytes())
		require.Len(t, messages, 1)
		assert.Equal(t, "user", messages[0]["sender_type"])
	})

	t.Run("agent without capability is forbidden", func(t *testing.T) {
		router := conversationAdminRouter(f.handler, f.adminID)
		req, _ := http.NewRequest("GET", "/admin/support/tickets/"+f.ticket.ID.String()+"/messages", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// ---------------------------------------------------------------------------
// Admin send
// ---------------------------------------------------------------------------

func TestConversation_AdminSend_RealAgentSenderAndNotification(t *testing.T) {
	t.Run("agent reply persists the authenticated agent as sender", func(t *testing.T) {
		f := newConversationFixture(t)
		router := conversationAdminRouter(f.handler, f.adminID, capability.CapSupportTicketRespond.String())
		req, _ := http.NewRequest("POST", "/admin/support/tickets/"+f.ticket.ID.String()+"/messages",
			strings.NewReader(`{"type":"agent","message":"kami cek dulu ya"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Len(t, f.chatMsgSvc.sent, 1)
		assert.Equal(t, f.adminID, f.chatMsgSvc.sent[0].senderID,
			"the agent's authenticated id is the sender — never a system message")
		assert.NotEqual(t, uuid.Nil, f.chatMsgSvc.sent[0].senderID)
		assert.Equal(t, "kami cek dulu ya", f.chatMsgSvc.sent[0].body)
	})

	t.Run("agent without respond capability is forbidden and sends nothing", func(t *testing.T) {
		f := newConversationFixture(t)
		router := conversationAdminRouter(f.handler, f.adminID, capability.CapSupportTicketRead.String())
		req, _ := http.NewRequest("POST", "/admin/support/tickets/"+f.ticket.ID.String()+"/messages",
			strings.NewReader(`{"type":"agent","message":"nope"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Empty(t, f.chatMsgSvc.sent)
	})

	t.Run("agent reply emits the user notification event", func(t *testing.T) {
		f := newConversationFixture(t)
		router := conversationAdminRouter(f.handler, f.adminID, capability.CapSupportTicketRespond.String())
		req, _ := http.NewRequest("POST", "/admin/support/tickets/"+f.ticket.ID.String()+"/messages",
			strings.NewReader(`{"type":"agent","message":"balasan agent"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, f.outbox.events, "support.ticket_waiting_user",
			"an agent reply must produce the user-facing notification event")
	})
}
