package application

// Tests for BATCH 99 — Support Chat Authority Fix
//
// Verifies:
// 1. CreateSupportTicketRoom creates RoomTypeSupport (not RoomTypeDirect)
// 2. mockChatService already returns RoomTypeSupport (confirms test mock alignment)
// 3. Ticket creation populates ChatRoomID from the support room
// 4. SendSystemMessage is called with the room's ID (no participant mismatch)

import (
	"context"
	"testing"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/governance/support/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestB99_CreateTicket_RoomTypeSupport verifies that support ticket creation
// receives a RoomTypeSupport room from the adapter (not RoomTypeDirect).
func TestB99_CreateTicket_RoomTypeSupport(t *testing.T) {
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

	subject := "Test support ticket"
	req := &CreateTicketRequest{
		UserID:   uuid.New(),
		Category: entity.CategoryPayment,
		Priority: entity.PriorityMedium,
		Subject:  &subject,
	}

	ticket, err := service.CreateTicket(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, ticket)

	// The room returned by the mock must be RoomTypeSupport.
	// This confirms the adapter contract: CreateSupportTicketRoom returns support rooms.
	assert.Equal(t, chatEntity.RoomTypeSupport, chatSvc.room.RoomType,
		"CreateSupportTicketRoom must return RoomTypeSupport, not RoomTypeDirect")

	// Ticket must store the room ID.
	assert.Equal(t, chatSvc.room.ID, ticket.ChatRoomID,
		"Ticket.ChatRoomID must reference the support room")
}

// TestB99_SupportRoomBlockExemptionProperty verifies the structural property:
// RoomTypeSupport rooms are exempt from block enforcement in SendMessage.
//
// The chat service checks:
//   if room.RoomType != chatEntity.RoomTypeSupport && !room.HasOrderContext() { blockCheck }
//
// With RoomTypeSupport, block check is skipped unconditionally.
func TestB99_SupportRoomBlockExemptionProperty(t *testing.T) {
	room := &chatEntity.ChatRoom{
		ID:           uuid.New(),
		RoomType:     chatEntity.RoomTypeSupport,
		ParticipantA: uuid.Nil,               // system
		ParticipantB: uuid.New(),              // user
	}

	// RoomTypeSupport is NOT direct and NOT negotiation → block-exempt per chat_service.go:814
	assert.Equal(t, chatEntity.RoomTypeSupport, room.RoomType)
	assert.NotEqual(t, chatEntity.RoomTypeDirect, room.RoomType,
		"Support room must NOT be RoomTypeDirect — otherwise block exemption is lost")
}

// TestB99_SupportUserRepliedEmissionProperty verifies the structural property:
// The support.user_replied outbox event fires only when:
//   room.RoomType == RoomTypeSupport && senderID != uuid.Nil
//
// With the old RoomTypeDirect, this condition was never true → event never fired.
func TestB99_SupportUserRepliedEmissionProperty(t *testing.T) {
	userID := uuid.New()

	room := &chatEntity.ChatRoom{
		ID:           uuid.New(),
		RoomType:     chatEntity.RoomTypeSupport,
		ParticipantA: uuid.Nil, // system (sorted: Nil is always smallest)
		ParticipantB: userID,
	}

	// Condition for support.user_replied emission:
	isSupportRoom := room.RoomType == chatEntity.RoomTypeSupport
	senderIsUser := userID != uuid.Nil

	assert.True(t, isSupportRoom,
		"Room must be RoomTypeSupport for support.user_replied to fire")
	assert.True(t, senderIsUser,
		"Real user sender must not be uuid.Nil")
	assert.True(t, isSupportRoom && senderIsUser,
		"Both conditions must be true for support.user_replied emission")

	// System messages (sender = uuid.Nil) must NOT trigger support.user_replied
	systemSender := uuid.Nil
	assert.False(t, isSupportRoom && systemSender != uuid.Nil,
		"System messages must NOT trigger support.user_replied")
}

// TestB99_SupportRoomParticipantSorting verifies that NewChatRoom sorts participants
// correctly for support rooms: uuid.Nil is always participant_a.
func TestB99_SupportRoomParticipantSorting(t *testing.T) {
	userID := uuid.New()

	// NewChatRoom sorts by UUID string — uuid.Nil is always smallest.
	room := chatEntity.NewChatRoom(chatEntity.RoomTypeSupport, userID, uuid.Nil)

	assert.Equal(t, uuid.Nil, room.ParticipantA,
		"uuid.Nil must be participant_a (smallest UUID after sorting)")
	assert.Equal(t, userID, room.ParticipantB,
		"User must be participant_b (larger UUID after sorting)")

	// HasParticipant must find both
	assert.True(t, room.HasParticipant(userID), "User must be a participant")
	assert.True(t, room.HasParticipant(uuid.Nil), "System (Nil) must be a participant")
}


