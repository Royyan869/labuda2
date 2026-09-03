package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	socialRepo "github.com/labuda/backend/internal/social/graph"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"go.uber.org/zap"
)

type roomUpdatedMockTx struct{}

func (m *roomUpdatedMockTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *roomUpdatedMockTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *roomUpdatedMockTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return &roomUpdatedMockRow{}
}

func (m *roomUpdatedMockTx) Commit(context.Context) error   { return nil }
func (m *roomUpdatedMockTx) Rollback(context.Context) error { return nil }

type roomUpdatedMockRow struct{}

func (r *roomUpdatedMockRow) Scan(dest ...any) error { return nil }

type roomUpdatedMockTransactor struct {
	tx db.Tx
}

func (m *roomUpdatedMockTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(m.tx)
}

type roomUpdatedOutboxInsert struct {
	eventType string
	payload   map[string]any
	key       string
}

type roomUpdatedMockOutbox struct {
	inserts []roomUpdatedOutboxInsert
}

func (m *roomUpdatedMockOutbox) InsertTx(
	_ context.Context,
	_ db.Tx,
	eventType string,
	payload any,
	idempotencyKey string,
) error {
	data, ok := payload.(map[string]any)
	if !ok {
		return errors.New("payload must be map[string]any")
	}
	m.inserts = append(m.inserts, roomUpdatedOutboxInsert{
		eventType: eventType,
		payload:   data,
		key:       idempotencyKey,
	})
	return nil
}

type roomUpdatedMockSocialRepo struct {
	blocked bool
}

func (m *roomUpdatedMockSocialRepo) InsertFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *roomUpdatedMockSocialRepo) DeleteFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *roomUpdatedMockSocialRepo) DeleteFollowBothDirections(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *roomUpdatedMockSocialRepo) ExistsFollow(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (m *roomUpdatedMockSocialRepo) ListFollowers(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *roomUpdatedMockSocialRepo) ListFollowing(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *roomUpdatedMockSocialRepo) InsertBlock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *roomUpdatedMockSocialRepo) DeleteBlock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *roomUpdatedMockSocialRepo) ExistsBlock(
	context.Context,
	interface{},
	uuid.UUID,
	uuid.UUID,
) (bool, error) {
	return m.blocked, nil
}

func (m *roomUpdatedMockSocialRepo) AcquireFollowLock(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *roomUpdatedMockSocialRepo) IsBlockedBy(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return m.blocked, nil
}

func (m *roomUpdatedMockSocialRepo) InsertMute(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *roomUpdatedMockSocialRepo) DeleteMute(context.Context, interface{}, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *roomUpdatedMockSocialRepo) ExistsMute(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (m *roomUpdatedMockSocialRepo) ListMuted(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *roomUpdatedMockSocialRepo) ListBlocked(context.Context, interface{}, uuid.UUID, int, *time.Time) ([]uuid.UUID, error) {
	return nil, nil
}

type roomUpdatedMockRepo struct {
	room                     *chatEntity.ChatRoom
	unreadCounts             map[uuid.UUID]int
	readState                *chatEntity.ChatReadState
	messages                 []*chatEntity.ChatMessage
	messageByID              *chatEntity.ChatMessage
	createRoomErr            error
	getRoomByIDErr           error
	getDirectRoomErr         error
	getSupportRoomErr        error
	getReadStateErr          error
	listMessagesErr          error
	createMessageCalls       int
	createRoomCalls          int
	updateRoomLastMessageAt  int
	upsertReadStateCalls     int
	softHideCalls            int
	restoreCalls             int
	preferUnreadCounts       bool
	getUnreadCountRequests   []uuid.UUID
	getMessageByKeyRequests  []string
	lastReadStates           []*chatEntity.ChatReadState
	getRoomByOrderIDRoom     *chatEntity.ChatRoom
	getRoomByOrderIDErr      error
	updateLinkedOrderIDCalls int
}

func (r *roomUpdatedMockRepo) CreateRoom(_ context.Context, _ interface{}, room *chatEntity.ChatRoom) error {
	r.createRoomCalls++
	if r.createRoomErr != nil {
		return r.createRoomErr
	}
	r.room = room
	return nil
}

func (r *roomUpdatedMockRepo) GetRoomByID(_ context.Context, _ interface{}, _ uuid.UUID) (*chatEntity.ChatRoom, error) {
	if r.getRoomByIDErr != nil {
		return nil, r.getRoomByIDErr
	}
	return r.room, nil
}

func (r *roomUpdatedMockRepo) GetRoomByIDForUpdate(_ context.Context, _ interface{}, _ uuid.UUID) (*chatEntity.ChatRoom, error) {
	if r.getRoomByIDErr != nil {
		return nil, r.getRoomByIDErr
	}
	return r.room, nil
}

func (r *roomUpdatedMockRepo) GetDirectRoom(context.Context, interface{}, uuid.UUID, uuid.UUID) (*chatEntity.ChatRoom, error) {
	if r.getDirectRoomErr != nil {
		return nil, r.getDirectRoomErr
	}
	return r.room, nil
}

func (r *roomUpdatedMockRepo) GetSupportRoom(context.Context, interface{}, uuid.UUID) (*chatEntity.ChatRoom, error) {
	if r.getSupportRoomErr != nil {
		return nil, r.getSupportRoomErr
	}
	return r.room, nil
}

func (r *roomUpdatedMockRepo) ListRoomsByUser(context.Context, interface{}, uuid.UUID, *time.Time, *uuid.UUID, int) ([]*chatEntity.ChatRoom, error) {
	return nil, nil
}

func (r *roomUpdatedMockRepo) GetRoomByOrderID(context.Context, interface{}, uuid.UUID) (*chatEntity.ChatRoom, error) {
	if r.getRoomByOrderIDErr != nil {
		return nil, r.getRoomByOrderIDErr
	}
	if r.getRoomByOrderIDRoom != nil {
		return r.getRoomByOrderIDRoom, nil
	}
	return nil, chatRepo.ErrRoomNotFound
}

func (r *roomUpdatedMockRepo) UpdateRoomLastMessageAt(context.Context, interface{}, uuid.UUID, time.Time) error {
	r.updateRoomLastMessageAt++
	return nil
}

func (r *roomUpdatedMockRepo) UpdateRoomLinkedOrderId(_ context.Context, _ interface{}, _ uuid.UUID, linkedOrderID *uuid.UUID) error {
	r.updateLinkedOrderIDCalls++
	if r.room != nil {
		r.room.LinkedOrderID = linkedOrderID
		r.room.UpdatedAt = time.Now()
	}
	return nil
}

func (r *roomUpdatedMockRepo) CreateMessage(context.Context, interface{}, *chatEntity.ChatMessage) error {
	r.createMessageCalls++
	return nil
}

func (r *roomUpdatedMockRepo) GetMessageByID(context.Context, interface{}, uuid.UUID) (*chatEntity.ChatMessage, error) {
	if r.messageByID != nil {
		return r.messageByID, nil
	}
	return nil, chatRepo.ErrMessageNotFound
}

func (r *roomUpdatedMockRepo) ListMessagesByRoom(context.Context, interface{}, uuid.UUID, *time.Time, *uuid.UUID, int) ([]*chatEntity.ChatMessage, error) {
	if r.listMessagesErr != nil {
		return nil, r.listMessagesErr
	}
	return r.messages, nil
}

func (r *roomUpdatedMockRepo) GetMessageByIdempotencyKey(context.Context, interface{}, string) (*chatEntity.ChatMessage, error) {
	return nil, chatRepo.ErrMessageNotFound
}

func (r *roomUpdatedMockRepo) SoftHideForModeration(_ context.Context, _ interface{}, messageID uuid.UUID, deletedBy uuid.UUID, reason string) error {
	r.softHideCalls++
	now := time.Now().UTC()
	for _, msg := range r.messages {
		if msg != nil && msg.ID == messageID {
			msg.DeletedAt = &now
			msg.DeletedBy = &deletedBy
			msg.DeletionReason = &reason
		}
	}
	if r.messageByID != nil && r.messageByID.ID == messageID {
		r.messageByID.DeletedAt = &now
		r.messageByID.DeletedBy = &deletedBy
		r.messageByID.DeletionReason = &reason
	}
	return nil
}

func (r *roomUpdatedMockRepo) RestoreFromModeration(_ context.Context, _ interface{}, messageID uuid.UUID) error {
	r.restoreCalls++
	for _, msg := range r.messages {
		if msg != nil && msg.ID == messageID {
			msg.DeletedAt = nil
			msg.DeletedBy = nil
			msg.DeletionReason = nil
		}
	}
	if r.messageByID != nil && r.messageByID.ID == messageID {
		r.messageByID.DeletedAt = nil
		r.messageByID.DeletedBy = nil
		r.messageByID.DeletionReason = nil
	}
	return nil
}

func (r *roomUpdatedMockRepo) CreateReadState(context.Context, interface{}, *chatEntity.ChatReadState) error {
	return nil
}

func (r *roomUpdatedMockRepo) GetReadState(_ context.Context, _ interface{}, roomID, userID uuid.UUID) (*chatEntity.ChatReadState, error) {
	if r.getReadStateErr != nil {
		return nil, r.getReadStateErr
	}
	if r.readState == nil {
		return nil, chatRepo.ErrReadStateNotFound
	}
	if r.readState.RoomID != roomID || r.readState.UserID != userID {
		return nil, chatRepo.ErrReadStateNotFound
	}
	return r.readState, nil
}

func (r *roomUpdatedMockRepo) UpdateReadState(context.Context, interface{}, *chatEntity.ChatReadState) error {
	return nil
}

func (r *roomUpdatedMockRepo) UpsertReadState(_ context.Context, _ interface{}, state *chatEntity.ChatReadState) error {
	r.upsertReadStateCalls++
	r.lastReadStates = append(r.lastReadStates, state)
	r.readState = state
	return nil
}

func (r *roomUpdatedMockRepo) ListReadStatesByRoom(context.Context, interface{}, uuid.UUID) ([]*chatEntity.ChatReadState, error) {
	return nil, nil
}

func (r *roomUpdatedMockRepo) GetUnreadCountByRoomAndUser(
	_ context.Context,
	_ interface{},
	roomID uuid.UUID,
	userID uuid.UUID,
) (int, error) {
	r.getUnreadCountRequests = append(r.getUnreadCountRequests, userID)
	if r.preferUnreadCounts {
		if count, ok := r.unreadCounts[userID]; ok {
			return count, nil
		}
	}
	if len(r.messages) > 0 {
		var lastReadAt time.Time
		if r.readState != nil && r.readState.RoomID == roomID && r.readState.UserID == userID {
			lastReadAt = r.readState.LastReadAt
		}
		count := 0
		for _, msg := range r.messages {
			if msg == nil {
				continue
			}
			if msg.SenderID == userID {
				continue
			}
			if !lastReadAt.IsZero() && !msg.CreatedAt.After(lastReadAt) {
				continue
			}
			count++
		}
		return count, nil
	}
	if count, ok := r.unreadCounts[userID]; ok {
		return count, nil
	}
	return 0, nil
}

var _ db.Tx = (*roomUpdatedMockTx)(nil)
var _ OutboxInserter = (*roomUpdatedMockOutbox)(nil)
var _ socialRepo.SocialRepository = (*roomUpdatedMockSocialRepo)(nil)
var _ chatRepo.Repository = (*roomUpdatedMockRepo)(nil)

func TestSendMessage_EmitsRoomUpdatedOutboxEvents(t *testing.T) {
	senderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	room := &chatEntity.ChatRoom{
		ID:            uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		RoomType:      chatEntity.RoomTypeDirect,
		ParticipantA:  senderID,
		ParticipantB:  recipientID,
		UpdatedAt:     time.Unix(1000, 0).UTC(),
		CreatedAt:     time.Unix(900, 0).UTC(),
		LastMessageAt: time.Unix(900, 0).UTC(),
	}
	linkedOrderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	room.LinkedOrderID = &linkedOrderID

	repo := &roomUpdatedMockRepo{
		room: room,
		unreadCounts: map[uuid.UUID]int{
			senderID:    0,
			recipientID: 1,
		},
	}
	outbox := &roomUpdatedMockOutbox{}
	tx := &roomUpdatedMockTx{}

	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: tx},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{blocked: false},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	body := "hello room"
	msg, err := service.SendMessage(context.Background(), room.ID, senderID, chatEntity.MessageTypeText, &body, nil, "idem-1")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message result")
	}
	if repo.createMessageCalls != 1 {
		t.Fatalf("CreateMessage calls=%d want 1", repo.createMessageCalls)
	}
	if repo.updateRoomLastMessageAt != 1 {
		t.Fatalf("UpdateRoomLastMessageAt calls=%d want 1", repo.updateRoomLastMessageAt)
	}
	if repo.upsertReadStateCalls != 1 {
		t.Fatalf("UpsertReadState calls=%d want 1", repo.upsertReadStateCalls)
	}
	if len(outbox.inserts) != 3 {
		t.Fatalf("outbox inserts=%d want 3", len(outbox.inserts))
	}

	var messageSentFound bool
	var messageCreatedAt string
	roomUpdatedByRecipient := map[string]roomUpdatedOutboxInsert{}
	for _, insert := range outbox.inserts {
		switch insert.eventType {
		case "chat.message.sent":
			messageSentFound = true
			requireOutboxFieldString(t, insert.payload, "recipient_id", recipientID.String())
			requireOutboxFieldString(t, insert.payload, "sender_id", senderID.String())
			messageCreatedAt = requireOutboxFieldStringReturn(t, insert.payload, "created_at")
		case "chat.room.updated":
			roomUpdatedByRecipient[requireOutboxFieldStringReturn(t, insert.payload, "recipient_id")] = insert
		default:
			t.Fatalf("unexpected event type %q", insert.eventType)
		}
	}

	if !messageSentFound {
		t.Fatal("expected chat.message.sent event")
	}
	if len(roomUpdatedByRecipient) != 2 {
		t.Fatalf("room.updated recipients=%d want 2", len(roomUpdatedByRecipient))
	}

	senderUpdate, ok := roomUpdatedByRecipient[senderID.String()]
	if !ok {
		t.Fatalf("missing room.updated event for sender %s", senderID)
	}
	recipientUpdate, ok := roomUpdatedByRecipient[recipientID.String()]
	if !ok {
		t.Fatalf("missing room.updated event for recipient %s", recipientID)
	}

	requireOutboxFieldString(t, senderUpdate.payload, "other_user_id", recipientID.String())
	requireOutboxFieldString(t, recipientUpdate.payload, "other_user_id", senderID.String())
	requireOutboxFieldInt(t, senderUpdate.payload, "unread_count", 0)
	requireOutboxFieldInt(t, recipientUpdate.payload, "unread_count", 1)
	requireOutboxFieldMap(t, senderUpdate.payload, "last_message")
	requireOutboxFieldMap(t, recipientUpdate.payload, "last_message")
	requireOutboxFieldString(t, senderUpdate.payload, "created_at", "1970-01-01T00:15:00Z")
	requireOutboxFieldString(t, recipientUpdate.payload, "created_at", "1970-01-01T00:15:00Z")
	requireOutboxFieldString(t, recipientUpdate.payload, "linked_order_id", linkedOrderID.String())
	requireOutboxFieldString(t, senderUpdate.payload, "updated_at", messageCreatedAt)
	requireOutboxFieldString(t, senderUpdate.payload, "last_message_at", messageCreatedAt)
	requireOutboxFieldString(t, recipientUpdate.payload, "updated_at", messageCreatedAt)
	requireOutboxFieldString(t, recipientUpdate.payload, "last_message_at", messageCreatedAt)

	senderLastMessage := senderUpdate.payload["last_message"].(map[string]any)
	requireMapString(t, senderLastMessage, "body", body)
	requireMapString(t, senderLastMessage, "sender_id", senderID.String())
	requireMapString(t, senderLastMessage, "created_at", messageCreatedAt)
	if _, ok := senderLastMessage["attachment_json"]; ok {
		t.Fatal("did not expect attachment_json for plain text message")
	}
}

func TestSendMessage_AttachmentOnly_EmitsMinimalChatMessageSentOutbox(t *testing.T) {
	senderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	room := &chatEntity.ChatRoom{
		ID:            uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		RoomType:      chatEntity.RoomTypeDirect,
		ParticipantA:  senderID,
		ParticipantB:  recipientID,
		CreatedAt:     time.Unix(900, 0).UTC(),
		UpdatedAt:     time.Unix(1000, 0).UTC(),
		LastMessageAt: time.Unix(900, 0).UTC(),
	}

	repo := &roomUpdatedMockRepo{
		room: room,
		unreadCounts: map[uuid.UUID]int{
			senderID:    0,
			recipientID: 1,
		},
	}
	outbox := &roomUpdatedMockOutbox{}
	tx := &roomUpdatedMockTx{}

	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: tx},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{blocked: false},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	attachmentJSON := map[string]interface{}{
		"type": "image",
		"data": map[string]interface{}{
			"url":      "private://object-key",
			"filename": "secret.jpg",
			"caption":  "leaked caption",
		},
	}

	msg, err := service.SendMessage(
		context.Background(),
		room.ID,
		senderID,
		chatEntity.MessageTypeNegotiationProposal,
		nil,
		attachmentJSON,
		"idem-attachment-only",
	)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message result")
	}
	if len(outbox.inserts) != 3 {
		t.Fatalf("outbox inserts=%d want 3", len(outbox.inserts))
	}

	var messageSentFound bool
	for _, insert := range outbox.inserts {
		switch insert.eventType {
		case "chat.message.sent":
			messageSentFound = true
			requireOutboxFieldString(t, insert.payload, "room_id", room.ID.String())
			requireOutboxFieldString(t, insert.payload, "message_id", msg.ID.String())
			requireOutboxFieldString(t, insert.payload, "sender_id", senderID.String())
			requireOutboxFieldString(t, insert.payload, "recipient_id", recipientID.String())
			requireOutboxFieldString(t, insert.payload, "message_type", string(chatEntity.MessageTypeNegotiationProposal))
			requireOutboxFieldString(t, insert.payload, "created_at", msg.CreatedAt.UTC().Format(time.RFC3339))
			if _, ok := insert.payload["attachment_json"]; ok {
				t.Fatal("did not expect attachment_json on chat.message.sent payload")
			}
			if _, ok := insert.payload["body"]; ok {
				t.Fatal("did not expect body on chat.message.sent payload")
			}
		}
	}

	if !messageSentFound {
		t.Fatal("expected chat.message.sent event")
	}
}

func TestSendMessage_DoesNotEmitRoomUpdatedWhenBlocked(t *testing.T) {
	senderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	room := &chatEntity.ChatRoom{
		ID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		RoomType:     chatEntity.RoomTypeDirect,
		ParticipantA: senderID,
		ParticipantB: recipientID,
	}

	repo := &roomUpdatedMockRepo{room: room}
	outbox := &roomUpdatedMockOutbox{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{blocked: true},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	body := "blocked"
	_, err := service.SendMessage(context.Background(), room.ID, senderID, chatEntity.MessageTypeText, &body, nil, "idem-blocked")
	if !errors.Is(err, chatRepo.ErrUserBlocked) {
		t.Fatalf("SendMessage error=%v want ErrUserBlocked", err)
	}
	if len(outbox.inserts) != 0 {
		t.Fatalf("outbox inserts=%d want 0", len(outbox.inserts))
	}
}

func TestMarkAsRead_EmitsRoomUpdatedForReadingUser(t *testing.T) {
	readerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	room := &chatEntity.ChatRoom{
		ID:            uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		RoomType:      chatEntity.RoomTypeDirect,
		ParticipantA:  readerID,
		ParticipantB:  otherID,
		CreatedAt:     time.Unix(1000, 0).UTC(),
		UpdatedAt:     time.Unix(2000, 0).UTC(),
		LastMessageAt: time.Unix(2000, 0).UTC(),
	}
	firstMessage := &chatEntity.ChatMessage{
		ID:          uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		RoomID:      room.ID,
		SenderID:    otherID,
		MessageType: chatEntity.MessageTypeText,
		Body:        ptrString("first"),
		CreatedAt:   time.Unix(1500, 0).UTC(),
	}
	lastMessage := &chatEntity.ChatMessage{
		ID:          uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		RoomID:      room.ID,
		SenderID:    otherID,
		MessageType: chatEntity.MessageTypeText,
		Body:        ptrString("latest"),
		CreatedAt:   time.Unix(2000, 0).UTC(),
	}

	repo := &roomUpdatedMockRepo{
		room:     room,
		messages: []*chatEntity.ChatMessage{lastMessage, firstMessage},
	}
	outbox := &roomUpdatedMockOutbox{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	markAt := time.Unix(3000, 0).UTC()
	if err := service.MarkAsRead(context.Background(), room.ID, readerID, markAt); err != nil {
		t.Fatalf("MarkAsRead failed: %v", err)
	}
	if repo.upsertReadStateCalls != 1 {
		t.Fatalf("UpsertReadState calls=%d want 1", repo.upsertReadStateCalls)
	}
	if len(outbox.inserts) != 1 {
		t.Fatalf("outbox inserts=%d want 1", len(outbox.inserts))
	}

	insert := outbox.inserts[0]
	if insert.eventType != "chat.room.updated" {
		t.Fatalf("event type=%q want chat.room.updated", insert.eventType)
	}
	requireOutboxFieldString(t, insert.payload, "recipient_id", readerID.String())
	requireOutboxFieldString(t, insert.payload, "other_user_id", otherID.String())
	requireOutboxFieldInt(t, insert.payload, "unread_count", 0)
	requireOutboxFieldString(t, insert.payload, "updated_at", room.UpdatedAt.Format(time.RFC3339))
	requireOutboxFieldString(t, insert.payload, "last_message_at", room.LastMessageAt.Format(time.RFC3339))
	lastMessagePayload := requireOutboxFieldMap(t, insert.payload, "last_message")
	requireMapString(t, lastMessagePayload, "id", lastMessage.ID.String())
	requireMapString(t, lastMessagePayload, "sender_id", otherID.String())
	requireMapString(t, lastMessagePayload, "body", "latest")
}

func TestMarkAsRead_NoOpDoesNotEmitRoomUpdated(t *testing.T) {
	readerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	room := &chatEntity.ChatRoom{
		ID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		RoomType:     chatEntity.RoomTypeDirect,
		ParticipantA: readerID,
		ParticipantB: otherID,
	}
	existingState := &chatEntity.ChatReadState{
		RoomID:     room.ID,
		UserID:     readerID,
		LastReadAt: time.Unix(3000, 0).UTC(),
	}
	repo := &roomUpdatedMockRepo{
		room:      room,
		readState: existingState,
		messages: []*chatEntity.ChatMessage{
			{
				ID:          uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
				RoomID:      room.ID,
				SenderID:    otherID,
				MessageType: chatEntity.MessageTypeText,
				Body:        ptrString("latest"),
				CreatedAt:   time.Unix(2900, 0).UTC(),
			},
		},
	}
	outbox := &roomUpdatedMockOutbox{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	if err := service.MarkAsRead(context.Background(), room.ID, readerID, existingState.LastReadAt); err != nil {
		t.Fatalf("MarkAsRead failed: %v", err)
	}
	if repo.upsertReadStateCalls != 0 {
		t.Fatalf("UpsertReadState calls=%d want 0", repo.upsertReadStateCalls)
	}
	if len(outbox.inserts) != 0 {
		t.Fatalf("outbox inserts=%d want 0", len(outbox.inserts))
	}
}

func TestMarkAsRead_UnauthorizedDoesNotEmitRoomUpdated(t *testing.T) {
	readerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	unauthorizedID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	room := &chatEntity.ChatRoom{
		ID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		RoomType:     chatEntity.RoomTypeDirect,
		ParticipantA: readerID,
		ParticipantB: otherID,
	}
	repo := &roomUpdatedMockRepo{room: room}
	outbox := &roomUpdatedMockOutbox{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	err := service.MarkAsRead(context.Background(), room.ID, unauthorizedID, time.Now().UTC())
	if !errors.Is(err, chatRepo.ErrParticipantMismatch) {
		t.Fatalf("err=%v want ErrParticipantMismatch", err)
	}
	if repo.upsertReadStateCalls != 0 {
		t.Fatalf("UpsertReadState calls=%d want 0", repo.upsertReadStateCalls)
	}
	if len(outbox.inserts) != 0 {
		t.Fatalf("outbox inserts=%d want 0", len(outbox.inserts))
	}
}

func TestMarkAsRead_InvalidRoomDoesNotEmitRoomUpdated(t *testing.T) {
	readerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	roomID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	repo := &roomUpdatedMockRepo{
		getRoomByIDErr: errors.New("room missing"),
	}
	outbox := &roomUpdatedMockOutbox{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	err := service.MarkAsRead(context.Background(), roomID, readerID, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error")
	}
	if repo.upsertReadStateCalls != 0 {
		t.Fatalf("UpsertReadState calls=%d want 0", repo.upsertReadStateCalls)
	}
	if len(outbox.inserts) != 0 {
		t.Fatalf("outbox inserts=%d want 0", len(outbox.inserts))
	}
}

func TestSoftHideForModeration_EmitsRoomUpdatedTombstone(t *testing.T) {
	senderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	room := &chatEntity.ChatRoom{
		ID:            uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		RoomType:      chatEntity.RoomTypeDirect,
		ParticipantA:  senderID,
		ParticipantB:  recipientID,
		CreatedAt:     time.Unix(900, 0).UTC(),
		UpdatedAt:     time.Unix(1000, 0).UTC(),
		LastMessageAt: time.Unix(1100, 0).UTC(),
	}
	body := "hidden body"
	msg := &chatEntity.ChatMessage{
		ID:             uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		RoomID:         room.ID,
		SenderID:       recipientID,
		MessageType:    chatEntity.MessageTypeText,
		Body:           &body,
		CreatedAt:      time.Unix(1100, 0).UTC(),
		IdempotencyKey: "idem-hidden",
	}
	repo := &roomUpdatedMockRepo{
		room:        room,
		messageByID: msg,
		messages:    []*chatEntity.ChatMessage{msg},
		unreadCounts: map[uuid.UUID]int{
			senderID:    0,
			recipientID: 1,
		},
		preferUnreadCounts: true,
	}
	outbox := &roomUpdatedMockOutbox{}
	tx := &roomUpdatedMockTx{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: tx},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	if err := service.SoftHideForModeration(context.Background(), tx, msg.ID, uuid.Nil, "hidden by admin", "case-123"); err != nil {
		t.Fatalf("SoftHideForModeration failed: %v", err)
	}
	if repo.softHideCalls != 1 {
		t.Fatalf("SoftHideForModeration calls=%d want 1", repo.softHideCalls)
	}
	if len(outbox.inserts) != 2 {
		t.Fatalf("outbox inserts=%d want 2", len(outbox.inserts))
	}

	for _, insert := range outbox.inserts {
		if insert.eventType != "chat.room.updated" {
			t.Fatalf("event type=%q want chat.room.updated", insert.eventType)
		}
		recipientIDStr := requireOutboxFieldStringReturn(t, insert.payload, "recipient_id")
		requireOutboxFieldString(t, insert.payload, "room_id", room.ID.String())
		requireOutboxFieldInt(t, insert.payload, "unread_count", map[string]int{
			senderID.String():    0,
			recipientID.String(): 1,
		}[recipientIDStr])
		last := requireOutboxFieldMap(t, insert.payload, "last_message")
		if hidden, ok := last["is_hidden"].(bool); !ok || !hidden {
			t.Fatalf("last_message.is_hidden=%v want true", last["is_hidden"])
		}
		if _, ok := last["body"]; ok {
			t.Fatal("did not expect body on hidden last_message")
		}
		if _, ok := last["attachment_json"]; ok {
			t.Fatal("did not expect attachment_json on hidden last_message")
		}
	}
}

func TestRestoreFromModeration_EmitsRoomUpdatedVisibleMessage(t *testing.T) {
	senderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	room := &chatEntity.ChatRoom{
		ID:            uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		RoomType:      chatEntity.RoomTypeDirect,
		ParticipantA:  senderID,
		ParticipantB:  recipientID,
		CreatedAt:     time.Unix(900, 0).UTC(),
		UpdatedAt:     time.Unix(1000, 0).UTC(),
		LastMessageAt: time.Unix(1100, 0).UTC(),
	}
	body := "restored body"
	deletedAt := time.Unix(1200, 0).UTC()
	deletedBy := uuid.New()
	reason := "hidden before restore"
	msg := &chatEntity.ChatMessage{
		ID:             uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
		RoomID:         room.ID,
		SenderID:       recipientID,
		MessageType:    chatEntity.MessageTypeText,
		Body:           &body,
		CreatedAt:      time.Unix(1100, 0).UTC(),
		DeletedAt:      &deletedAt,
		DeletedBy:      &deletedBy,
		DeletionReason: &reason,
		IdempotencyKey: "idem-restore",
	}
	repo := &roomUpdatedMockRepo{
		room:        room,
		messageByID: msg,
		messages:    []*chatEntity.ChatMessage{msg},
		unreadCounts: map[uuid.UUID]int{
			senderID:    1,
			recipientID: 0,
		},
		preferUnreadCounts: true,
	}
	outbox := &roomUpdatedMockOutbox{}
	tx := &roomUpdatedMockTx{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: tx},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	if err := service.RestoreFromModeration(context.Background(), tx, msg.ID, "appeal-123"); err != nil {
		t.Fatalf("RestoreFromModeration failed: %v", err)
	}
	if repo.restoreCalls != 1 {
		t.Fatalf("RestoreFromModeration calls=%d want 1", repo.restoreCalls)
	}
	if len(outbox.inserts) != 2 {
		t.Fatalf("outbox inserts=%d want 2", len(outbox.inserts))
	}

	for _, insert := range outbox.inserts {
		if insert.eventType != "chat.room.updated" {
			t.Fatalf("event type=%q want chat.room.updated", insert.eventType)
		}
		requireOutboxFieldString(t, insert.payload, "room_id", room.ID.String())
		last := requireOutboxFieldMap(t, insert.payload, "last_message")
		if _, ok := last["is_hidden"]; ok {
			t.Fatal("did not expect is_hidden on restored last_message")
		}
		requireMapString(t, last, "body", body)
	}
}

func TestRestoreFromModeration_VisibleMessageIsDeterministicNoOp(t *testing.T) {
	senderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	room := &chatEntity.ChatRoom{
		ID:            uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"),
		RoomType:      chatEntity.RoomTypeDirect,
		ParticipantA:  senderID,
		ParticipantB:  recipientID,
		CreatedAt:     time.Unix(900, 0).UTC(),
		UpdatedAt:     time.Unix(1000, 0).UTC(),
		LastMessageAt: time.Unix(1100, 0).UTC(),
	}
	body := "already visible body"
	msg := &chatEntity.ChatMessage{
		ID:             uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"),
		RoomID:         room.ID,
		SenderID:       recipientID,
		MessageType:    chatEntity.MessageTypeText,
		Body:           &body,
		CreatedAt:      time.Unix(1200, 0).UTC(),
		IdempotencyKey: "idem-visible-restore",
	}
	repo := &roomUpdatedMockRepo{
		room:        room,
		messageByID: msg,
		messages:    []*chatEntity.ChatMessage{msg},
	}
	outbox := &roomUpdatedMockOutbox{}
	tx := &roomUpdatedMockTx{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: tx},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	if err := service.RestoreFromModeration(context.Background(), tx, msg.ID, "visible-restore"); err != nil {
		t.Fatalf("RestoreFromModeration on visible message failed: %v", err)
	}
	if repo.restoreCalls != 0 {
		t.Fatalf("RestoreFromModeration restore calls=%d want 0", repo.restoreCalls)
	}
	if len(outbox.inserts) != 0 {
		t.Fatalf("outbox inserts=%d want 0 for visible no-op", len(outbox.inserts))
	}
}

func TestRestoreFromModeration_NonexistentMessageReturnsNotFound(t *testing.T) {
	room := &chatEntity.ChatRoom{
		ID:            uuid.MustParse("abababab-abab-abab-abab-abababababab"),
		RoomType:      chatEntity.RoomTypeDirect,
		ParticipantA:  uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ParticipantB:  uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		CreatedAt:     time.Unix(900, 0).UTC(),
		UpdatedAt:     time.Unix(1000, 0).UTC(),
		LastMessageAt: time.Unix(1100, 0).UTC(),
	}
	repo := &roomUpdatedMockRepo{
		room: room,
	}
	outbox := &roomUpdatedMockOutbox{}
	tx := &roomUpdatedMockTx{}
	service := &Service{
		db:          &roomUpdatedMockTransactor{tx: tx},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}

	err := service.RestoreFromModeration(context.Background(), tx, uuid.New(), "missing-restore")
	if !errors.Is(err, chatRepo.ErrMessageNotFound) {
		t.Fatalf("RestoreFromModeration error = %v, want ErrMessageNotFound", err)
	}
	if len(outbox.inserts) != 0 {
		t.Fatalf("outbox inserts=%d want 0 for missing message", len(outbox.inserts))
	}
}

func ptrString(v string) *string {
	return &v
}

func requireOutboxFieldString(t *testing.T, payload map[string]any, key, want string) {
	t.Helper()
	got, ok := payload[key]
	if !ok {
		t.Fatalf("missing field %q", key)
	}
	gotStr, ok := got.(string)
	if !ok {
		t.Fatalf("%s type=%T want string", key, got)
	}
	if gotStr != want {
		t.Fatalf("%s=%q want %q", key, gotStr, want)
	}
}

func requireOutboxFieldStringReturn(t *testing.T, payload map[string]any, key string) string {
	t.Helper()
	got, ok := payload[key]
	if !ok {
		t.Fatalf("missing field %q", key)
	}
	gotStr, ok := got.(string)
	if !ok {
		t.Fatalf("%s type=%T want string", key, got)
	}
	return gotStr
}

func requireOutboxFieldInt(t *testing.T, payload map[string]any, key string, want int) {
	t.Helper()
	got, ok := payload[key]
	if !ok {
		t.Fatalf("missing field %q", key)
	}
	switch v := got.(type) {
	case int:
		if v != want {
			t.Fatalf("%s=%d want %d", key, v, want)
		}
	case int32:
		if int(v) != want {
			t.Fatalf("%s=%d want %d", key, v, want)
		}
	case int64:
		if int(v) != want {
			t.Fatalf("%s=%d want %d", key, v, want)
		}
	case float64:
		if int(v) != want {
			t.Fatalf("%s=%v want %d", key, v, want)
		}
	default:
		t.Fatalf("%s type=%T want integer", key, got)
	}
}

func requireOutboxFieldMap(t *testing.T, payload map[string]any, key string) map[string]any {
	t.Helper()
	got, ok := payload[key]
	if !ok {
		t.Fatalf("missing field %q", key)
	}
	child, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("%s type=%T want map[string]any", key, got)
	}
	return child
}

func requireMapString(t *testing.T, payload map[string]any, key, want string) {
	t.Helper()
	got, ok := payload[key]
	if !ok {
		t.Fatalf("missing field %q", key)
	}
	gotStr, ok := got.(string)
	if !ok {
		t.Fatalf("%s type=%T want string", key, got)
	}
	if gotStr != want {
		t.Fatalf("%s=%q want %q", key, gotStr, want)
	}
}
