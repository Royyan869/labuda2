package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/pkg/rate"
	"go.uber.org/zap"
)

// ============================================================================
// Canonical chat message idempotency authority.
//
// AUTHORITY: (sender_id, idempotency_key) — UNIQUE(sender_id, idempotency_key),
// migration 000032. Replay is decided by command_fingerprint equality.
//
//   Case A: first submission          -> one persisted message
//   Case B: same sender+key+fingerprint -> replay existing
//   Case C: same sender+key, different fingerprint -> conflict, no replay
//   Case D: different sender, same key -> independent message (no cross-read)
//   Case E: concurrent duplicate      -> converge on the single winner
// ============================================================================

func newChatIdempotencyTestService(repo *roomUpdatedMockRepo, outbox *roomUpdatedMockOutbox) *Service {
	return &Service{
		db:          &roomUpdatedMockTransactor{tx: &roomUpdatedMockTx{}},
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}
}

func newDirectRoomFor(sender, other uuid.UUID) *chatEntity.ChatRoom {
	return &chatEntity.ChatRoom{
		ID:           uuid.New(),
		RoomType:     chatEntity.RoomTypeDirect,
		ParticipantA: sender,
		ParticipantB: other,
	}
}

// Case A — first submission creates exactly one message.
func TestChatIdempotency_CaseA_FirstSubmissionCreatesOnce(t *testing.T) {
	sender := uuid.New()
	other := uuid.New()
	room := newDirectRoomFor(sender, other)
	repo := &roomUpdatedMockRepo{room: room}
	outbox := &roomUpdatedMockOutbox{}
	svc := newChatIdempotencyTestService(repo, outbox)

	body := "first submission"
	msg, err := svc.SendMessage(context.Background(), room.ID, sender, chatEntity.MessageTypeText, &body, nil, "key-A")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message result")
	}
	if msg.IdempotencyKey != "key-A" {
		t.Fatalf("idempotency_key=%q want key-A", msg.IdempotencyKey)
	}
	if repo.createMessageCalls != 1 {
		t.Fatalf("CreateMessage calls=%d want 1", repo.createMessageCalls)
	}
	// OWNERSHIP: exactly one realtime durable effect and exactly one notification
	// durable effect per sent message, plus the two viewer-scoped room summaries.
	if len(outbox.inserts) != 4 {
		t.Fatalf("outbox inserts=%d want 4 (1 realtime + 1 notification + 2 room.updated)", len(outbox.inserts))
	}

	var realtimeCount, notificationCount int
	for _, insert := range outbox.inserts {
		switch insert.eventType {
		case "chat.message.sent":
			realtimeCount++
		case "chat.message.notification":
			notificationCount++
		}
	}
	if realtimeCount != 1 || notificationCount != 1 {
		t.Fatalf("realtime events=%d notification events=%d want 1 and 1", realtimeCount, notificationCount)
	}
}

// Case B — same sender + same key + same fingerprint replays the existing message.
func TestChatIdempotency_CaseB_SameCommandReplaysExisting(t *testing.T) {
	sender := uuid.New()
	other := uuid.New()
	room := newDirectRoomFor(sender, other)
	repo := &roomUpdatedMockRepo{room: room}
	outbox := &roomUpdatedMockOutbox{}
	svc := newChatIdempotencyTestService(repo, outbox)

	body := "replay me"
	first, err := svc.SendMessage(context.Background(), room.ID, sender, chatEntity.MessageTypeText, &body, nil, "key-B")
	if err != nil {
		t.Fatalf("first SendMessage failed: %v", err)
	}
	outboxAfterFirst := len(outbox.inserts)

	replayBody := "replay me"
	second, err := svc.SendMessage(context.Background(), room.ID, sender, chatEntity.MessageTypeText, &replayBody, nil, "key-B")
	if err != nil {
		t.Fatalf("replay SendMessage failed: %v", err)
	}
	if second == nil || second.ID != first.ID {
		t.Fatalf("replay returned %v, want same message %s", second, first.ID)
	}
	if repo.createMessageCalls != 1 {
		t.Fatalf("CreateMessage calls=%d want 1 (replay must not insert)", repo.createMessageCalls)
	}
	if len(outbox.inserts) != outboxAfterFirst {
		t.Fatalf("replay emitted %d extra outbox events, want 0", len(outbox.inserts)-outboxAfterFirst)
	}
}

// Case C — same sender + same key + different fingerprint is a conflict, never a replay.
func TestChatIdempotency_CaseC_DifferentCommandConflicts(t *testing.T) {
	sender := uuid.New()
	other := uuid.New()
	room := newDirectRoomFor(sender, other)
	repo := &roomUpdatedMockRepo{room: room}
	outbox := &roomUpdatedMockOutbox{}
	svc := newChatIdempotencyTestService(repo, outbox)

	firstBody := "original command"
	if _, err := svc.SendMessage(context.Background(), room.ID, sender, chatEntity.MessageTypeText, &firstBody, nil, "key-C"); err != nil {
		t.Fatalf("first SendMessage failed: %v", err)
	}
	outboxAfterFirst := len(outbox.inserts)

	differentBody := "a totally different command"
	_, err := svc.SendMessage(context.Background(), room.ID, sender, chatEntity.MessageTypeText, &differentBody, nil, "key-C")
	if !errors.Is(err, chatRepo.ErrIdempotencyKeyConflict) {
		t.Fatalf("err=%v want ErrIdempotencyKeyConflict", err)
	}
	if repo.createMessageCalls != 1 {
		t.Fatalf("CreateMessage calls=%d want 1 (conflict must not insert)", repo.createMessageCalls)
	}
	if len(outbox.inserts) != outboxAfterFirst {
		t.Fatalf("conflict emitted %d extra outbox events, want 0", len(outbox.inserts)-outboxAfterFirst)
	}
}

// Case D — a different sender reusing the same key must not read/replay the
// other sender's message, and must be able to persist its own.
func TestChatIdempotency_CaseD_DifferentSenderIsIsolated(t *testing.T) {
	senderA := uuid.New()
	senderB := uuid.New()
	room := newDirectRoomFor(senderA, senderB)
	repo := &roomUpdatedMockRepo{room: room}
	outbox := &roomUpdatedMockOutbox{}
	svc := newChatIdempotencyTestService(repo, outbox)

	bodyA := "message from A"
	msgA, err := svc.SendMessage(context.Background(), room.ID, senderA, chatEntity.MessageTypeText, &bodyA, nil, "shared-key")
	if err != nil {
		t.Fatalf("sender A SendMessage failed: %v", err)
	}

	bodyB := "message from B"
	msgB, err := svc.SendMessage(context.Background(), room.ID, senderB, chatEntity.MessageTypeText, &bodyB, nil, "shared-key")
	if err != nil {
		t.Fatalf("sender B SendMessage failed: %v", err)
	}
	if msgB == nil {
		t.Fatal("expected sender B message result")
	}
	if msgB.ID == msgA.ID {
		t.Fatalf("sender B received sender A's message %s (cross-actor leak)", msgA.ID)
	}
	if msgB.SenderID != senderB {
		t.Fatalf("sender B message sender_id=%s want %s", msgB.SenderID, senderB)
	}
	if msgB.Body == nil || *msgB.Body != bodyB {
		t.Fatalf("sender B body=%v want %q", msgB.Body, bodyB)
	}
	if repo.createMessageCalls != 2 {
		t.Fatalf("CreateMessage calls=%d want 2 (independent per actor)", repo.createMessageCalls)
	}
	// The lookup must have been issued scoped to each sender — never global.
	expectA := idempotencyIndexKey(senderA, "shared-key")
	expectB := idempotencyIndexKey(senderB, "shared-key")
	if !containsString(repo.getMessageByKeyRequests, expectA) || !containsString(repo.getMessageByKeyRequests, expectB) {
		t.Fatalf("idempotency lookups=%v want to contain %q and %q", repo.getMessageByKeyRequests, expectA, expectB)
	}
}

// Case E — concurrent duplicate (same sender+key+fingerprint): only one message
// is persisted; the loser converges on the winner row with no duplicate side effects.
func TestChatIdempotency_CaseE_ConcurrentDuplicateConverges(t *testing.T) {
	sender := uuid.New()
	other := uuid.New()
	room := newDirectRoomFor(sender, other)

	body := "racing command"
	winner := chatEntity.NewChatMessage(room.ID, sender, chatEntity.MessageTypeText, &body, nil, "key-E")

	repo := &roomUpdatedMockRepo{
		room: room,
		idempotencyIndex: map[string]*chatEntity.ChatMessage{
			idempotencyIndexKey(sender, "key-E"): winner,
		},
		// First lookup misses (winner not yet visible), insert then conflicts,
		// re-lookup finds the winner — the real race shape.
		failFirstIdempotencyLookup: true,
	}
	outbox := &roomUpdatedMockOutbox{}
	svc := newChatIdempotencyTestService(repo, outbox)

	dupBody := "racing command"
	msg, err := svc.SendMessage(context.Background(), room.ID, sender, chatEntity.MessageTypeText, &dupBody, nil, "key-E")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if msg == nil || msg.ID != winner.ID {
		t.Fatalf("converged message=%v want winner %s", msg, winner.ID)
	}
	if repo.createMessageCalls != 1 {
		t.Fatalf("CreateMessage attempts=%d want 1", repo.createMessageCalls)
	}
	if repo.getMessageByKeyCalls < 2 {
		t.Fatalf("idempotency lookups=%d want >=2 (initial miss + race re-resolve)", repo.getMessageByKeyCalls)
	}
	if len(outbox.inserts) != 0 {
		t.Fatalf("loser emitted %d outbox events, want 0", len(outbox.inserts))
	}
}

// Case E (variant) — concurrent duplicate with a different command is a conflict.
func TestChatIdempotency_CaseE_ConcurrentDifferentCommandConflicts(t *testing.T) {
	sender := uuid.New()
	other := uuid.New()
	room := newDirectRoomFor(sender, other)

	winnerBody := "winner command"
	winner := chatEntity.NewChatMessage(room.ID, sender, chatEntity.MessageTypeText, &winnerBody, nil, "key-E2")

	repo := &roomUpdatedMockRepo{
		room: room,
		idempotencyIndex: map[string]*chatEntity.ChatMessage{
			idempotencyIndexKey(sender, "key-E2"): winner,
		},
		failFirstIdempotencyLookup: true,
	}
	outbox := &roomUpdatedMockOutbox{}
	svc := newChatIdempotencyTestService(repo, outbox)

	loserBody := "loser command"
	_, err := svc.SendMessage(context.Background(), room.ID, sender, chatEntity.MessageTypeText, &loserBody, nil, "key-E2")
	if !errors.Is(err, chatRepo.ErrIdempotencyKeyConflict) {
		t.Fatalf("err=%v want ErrIdempotencyKeyConflict", err)
	}
	if len(outbox.inserts) != 0 {
		t.Fatalf("loser emitted %d outbox events, want 0", len(outbox.inserts))
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
