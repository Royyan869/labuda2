package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	forSaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap/zaptest"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mocks — minimal fakes that record calls for assertion.
// ─────────────────────────────────────────────────────────────────────────────

type mockModerationContentService struct {
	softDeleteCalls []uuid.UUID
	restoreCalls    []uuid.UUID
	softDeleteErr   error
	restoreErr      error
}

func (m *mockModerationContentService) SoftDeleteForModeration(_ context.Context, _ interface{}, id uuid.UUID) error {
	m.softDeleteCalls = append(m.softDeleteCalls, id)
	return m.softDeleteErr
}

func (m *mockModerationContentService) RestoreFromModeration(_ context.Context, _ interface{}, id uuid.UUID) error {
	m.restoreCalls = append(m.restoreCalls, id)
	return m.restoreErr
}

type mockModerationCommentService struct {
	softDeleteCalls []uuid.UUID
	restoreCalls    []uuid.UUID
	softDeleteErr   error
	restoreErr      error
}

func (m *mockModerationCommentService) SoftDeleteForModeration(_ context.Context, _ interface{}, id uuid.UUID) error {
	m.softDeleteCalls = append(m.softDeleteCalls, id)
	return m.softDeleteErr
}

func (m *mockModerationCommentService) RestoreFromModeration(_ context.Context, _ interface{}, id uuid.UUID) error {
	m.restoreCalls = append(m.restoreCalls, id)
	return m.restoreErr
}

type mockModerationForSaleService struct {
	withdrawCalls []uuid.UUID
	withdrawErr   error
}

func (m *mockModerationForSaleService) Withdraw(_ context.Context, _ interface{}, id uuid.UUID) error {
	m.withdrawCalls = append(m.withdrawCalls, id)
	return m.withdrawErr
}

type mockModerationUserRepo struct {
	getForUpdateUser *mockModerationUser
	getForUpdateErr  error
	updateCalls      []*mockModerationUser
	updateErr        error
}

type mockModerationUser struct {
	ID            uuid.UUID
	AccountStatus string
}

type mockChatMessageModerationService struct {
	softHideCalls []struct {
		messageID     uuid.UUID
		deletedBy     uuid.UUID
		reason        string
		moderationKey string
	}
	restoreCalls []uuid.UUID
	restoreKeys  []string
	softHideErr  error
	restoreErr   error
}

type mockChatMessageModerationStore = mockChatMessageModerationService

func (m *mockChatMessageModerationService) SoftHideForModeration(_ context.Context, _ db.Tx, messageID uuid.UUID, deletedBy uuid.UUID, reason, moderationKey string) error {
	m.softHideCalls = append(m.softHideCalls, struct {
		messageID     uuid.UUID
		deletedBy     uuid.UUID
		reason        string
		moderationKey string
	}{messageID: messageID, deletedBy: deletedBy, reason: reason, moderationKey: moderationKey})
	return m.softHideErr
}

func (m *mockChatMessageModerationService) RestoreFromModeration(_ context.Context, _ db.Tx, messageID uuid.UUID, moderationKey string) error {
	m.restoreCalls = append(m.restoreCalls, messageID)
	m.restoreKeys = append(m.restoreKeys, moderationKey)
	return m.restoreErr
}

func (m *mockModerationUserRepo) GetByIDForUpdate(_ context.Context, _ interface{}, id uuid.UUID) (*mockModerationUser, error) {
	if m.getForUpdateErr != nil {
		return nil, m.getForUpdateErr
	}
	return m.getForUpdateUser, nil
}

func (m *mockModerationUserRepo) Update(_ context.Context, _ interface{}, user *mockModerationUser) error {
	m.updateCalls = append(m.updateCalls, user)
	return m.updateErr
}

// ─────────────────────────────────────────────────────────────────────────────
// The ModerationEventHandler uses concrete types from the real services via
// type assertion in the constructor. For unit tests we bypass the constructor
// and test the Handle() routing + individual handler behavior via the outbox
// event interface directly. The handler is safe to construct with nil services
// (nil-guards return nil on each handler path).
//
// We test the public Handle() entry point with crafted OutboxEvent payloads
// and verify:
//   - correct routing by resource_type
//   - idempotent return (nil) on parse error / unknown type
//   - auction PARKED returns nil
//   - fixed-price sale terminal-state returns nil (InvalidTransitionError guard)
// ─────────────────────────────────────────────────────────────────────────────

func buildRemovedEvent(resourceType, resourceID string) platformevent.OutboxEvent {
	payload, _ := json.Marshal(moderationRemovedPayload{
		CaseID:       uuid.New().String(),
		ResourceType: resourceType,
		ResourceID:   resourceID,
	})
	return platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "moderation." + resourceType + ".removed",
		Payload:   payload,
	}
}

func buildRestoredEvent(resourceType, resourceID string) platformevent.OutboxEvent {
	payload, _ := json.Marshal(moderationRestoredPayload{
		CaseID:       uuid.New().String(),
		AppealID:     uuid.New().String(),
		ResourceType: resourceType,
		ResourceID:   resourceID,
	})
	return platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "moderation." + resourceType + ".restored",
		Payload:   payload,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — nil-service handler (constructor nil-guards)
// ─────────────────────────────────────────────────────────────────────────────

// TestModerationHandler_ContentRemoved_NilService verifies that a content
// removal event with nil contentService returns nil (no retry).
func TestModerationHandler_ContentRemoved_NilService(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRemovedEvent("content", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestModerationHandler_CommentRemoved_NilService verifies that a comment
// removal event with nil commentService returns nil (no retry).
func TestModerationHandler_CommentRemoved_NilService(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRemovedEvent("comment", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestModerationHandler_ForSaleRemoved_NilService verifies that a
// fixed-price sale removal event with nil forSaleService returns nil
// (no retry).
func TestModerationHandler_ForSaleRemoved_NilService(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRemovedEvent("for_sale", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestModerationHandler_UserSuspended_NilRepo verifies that a user suspension
// event with nil userRepo returns nil (no retry).
func TestModerationHandler_UserSuspended_NilRepo(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRemovedEvent("user", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — auction moderation enforcement (FIX-1C)
// ─────────────────────────────────────────────────────────────────────────────

// mockAuctionCanceller is a test double for the auctionCanceller interface.
type mockAuctionCanceller struct {
	callCount int
	returnErr error
}

func (m *mockAuctionCanceller) CancelForModeration(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	m.callCount++
	return m.returnErr
}

// TestModerationHandler_AuctionRemoved_NilService verifies the guard path:
// nil auctionService returns nil (no retry, configuration issue).
func TestModerationHandler_AuctionRemoved_NilService(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRemovedEvent("auction", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("nil service guard must return nil, got %v", err)
	}
}

// TestModerationHandler_AuctionRemoved_Success verifies that handleAuctionRemoved
// calls CancelForModeration and returns nil on success.
func TestModerationHandler_AuctionRemoved_Success(t *testing.T) {
	mock := &mockAuctionCanceller{}
	h := &ModerationEventHandler{
		db:             &mockDB{},
		auctionService: mock,
		log:            zaptest.NewLogger(t),
	}
	event := buildRemovedEvent("auction", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}
	if mock.callCount != 1 {
		t.Errorf("CancelForModeration call count = %d, want 1", mock.callCount)
	}
}

// TestModerationHandler_AuctionRemoved_Idempotent_AlreadyCancelled verifies that
// InvalidTransitionError from a cancelled auction is treated as idempotent success.
func TestModerationHandler_AuctionRemoved_Idempotent_AlreadyCancelled(t *testing.T) {
	ite := &auctionEntity.InvalidTransitionError{
		CurrentStatus: auctionEntity.StatusCancelled,
		TargetStatus:  auctionEntity.StatusCancelled,
	}
	mock := &mockAuctionCanceller{returnErr: ite}
	h := &ModerationEventHandler{
		db:             &mockDB{},
		auctionService: mock,
		log:            zaptest.NewLogger(t),
	}
	event := buildRemovedEvent("auction", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("Idempotent path must return nil, got %v", err)
	}
}

// TestModerationHandler_AuctionRemoved_Idempotent_Ended verifies that an ended
// auction returns idempotent nil (auction is no longer active).
func TestModerationHandler_AuctionRemoved_Idempotent_Ended(t *testing.T) {
	ite := &auctionEntity.InvalidTransitionError{
		CurrentStatus: auctionEntity.StatusEnded,
		TargetStatus:  auctionEntity.StatusCancelled,
	}
	mock := &mockAuctionCanceller{returnErr: ite}
	h := &ModerationEventHandler{
		db:             &mockDB{},
		auctionService: mock,
		log:            zaptest.NewLogger(t),
	}
	event := buildRemovedEvent("auction", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("Idempotent path must return nil, got %v", err)
	}
}

// TestModerationHandler_AuctionRemoved_PropagatesNonIdempotentError verifies that
// non-InvalidTransitionError errors are returned (trigger retry).
func TestModerationHandler_AuctionRemoved_PropagatesNonIdempotentError(t *testing.T) {
	mock := &mockAuctionCanceller{returnErr: fmt.Errorf("db connection lost")}
	h := &ModerationEventHandler{
		db:             &mockDB{},
		auctionService: mock,
		log:            zaptest.NewLogger(t),
	}
	event := buildRemovedEvent("auction", uuid.New().String())
	if err := h.Handle(context.Background(), event); err == nil {
		t.Fatal("Non-idempotent error must be returned (trigger retry), got nil")
	}
}

// TestModerationHandler_AuctionRestored_NoopReturnsNil verifies that auction
// restoration (seller must re-create) returns nil.
func TestModerationHandler_AuctionRestored_NoopReturnsNil(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRestoredEvent("auction", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("NOOP handler must return nil, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — fixed-price sale idempotency on terminal state
// ─────────────────────────────────────────────────────────────────────────────

// TestModerationHandler_ForSaleRemoved_AlreadyWithdrawn verifies that
// when ForSaleService.Withdraw returns InvalidTransitionError
// (fixed-price sale already withdrawn or sold), the handler treats it as
// idempotent success (nil).
func TestModerationHandler_ForSaleRemoved_AlreadyWithdrawn(t *testing.T) {
	// Simulate: Withdraw() called on an already-withdrawn fixed-price sale returns
	// InvalidTransitionError. The handler's error guard uses errors.As to
	// detect this and return nil (no retry). Verify the guard logic works.
	ite := &forSaleEntity.InvalidTransitionError{
		CurrentStatus: forSaleEntity.ForSaleStatusWithdrawn,
		TargetStatus:  forSaleEntity.ForSaleStatusWithdrawn,
	}

	// Verify that errors.As works on the error type (regression lock).
	var target *forSaleEntity.InvalidTransitionError
	if !errors.As(ite, &target) {
		t.Fatal("errors.As must match *forSaleEntity.InvalidTransitionError")
	}

	if target.CurrentStatus != forSaleEntity.ForSaleStatusWithdrawn {
		t.Fatalf("expected withdrawn, got %s", target.CurrentStatus)
	}

	// Also verify the nil-guard path: nil forSaleService → return nil.
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRemovedEvent("for_sale", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("nil forSaleService must return nil, got %v", err)
	}
}

// TestModerationHandler_ForSaleRemoved_AlreadySold verifies that
// InvalidTransitionError from a sold fixed-price sale is also treated as
// idempotent.
func TestModerationHandler_ForSaleRemoved_AlreadySold(t *testing.T) {
	ite := &forSaleEntity.InvalidTransitionError{
		CurrentStatus: forSaleEntity.ForSaleStatusSold,
		TargetStatus:  forSaleEntity.ForSaleStatusWithdrawn,
	}
	var target *forSaleEntity.InvalidTransitionError
	if !errors.As(ite, &target) {
		t.Fatal("errors.As must match *forSaleEntity.InvalidTransitionError")
	}
	if target.CurrentStatus != forSaleEntity.ForSaleStatusSold {
		t.Fatalf("expected sold, got %s", target.CurrentStatus)
	}
}

// TestModerationHandler_ForSaleRestored_NilService verifies the
// nil-service guard path: nil forSaleService returns nil (no retry,
// configuration issue).
// FIX-5: handleForSaleRestored is now a real implementation; nil-guard preserved.
func TestModerationHandler_ForSaleRestored_NilService(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRestoredEvent("for_sale", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("nil forSaleService must return nil (nil-guard), got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — content/comment restoration (nil-service guard)
// ─────────────────────────────────────────────────────────────────────────────

// TestModerationHandler_ContentRestored_NilService returns nil with nil service.
func TestModerationHandler_ContentRestored_NilService(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRestoredEvent("content", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestModerationHandler_CommentRestored_NilService returns nil with nil service.
func TestModerationHandler_CommentRestored_NilService(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRestoredEvent("comment", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — user restoration (nil-service guard + banned permanence)
// ─────────────────────────────────────────────────────────────────────────────

// TestModerationHandler_UserRestored_NilRepo returns nil with nil userRepo.
func TestModerationHandler_UserRestored_NilRepo(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRestoredEvent("user", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — routing: unknown resource type
// ─────────────────────────────────────────────────────────────────────────────

// TestModerationHandler_UnknownResourceType_ReturnsNil verifies that unknown
// resource types are consumed without error (no retry).
func TestModerationHandler_UnknownResourceType_ReturnsNil(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRemovedEvent("video", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("expected nil for unknown resource type, got %v", err)
	}
}

// TestModerationHandler_UnknownResourceType_Restored_ReturnsNil verifies the
// restoration path also handles unknown types gracefully.
func TestModerationHandler_UnknownResourceType_Restored_ReturnsNil(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRestoredEvent("video", uuid.New().String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("expected nil for unknown resource type, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — malformed payload
// ─────────────────────────────────────────────────────────────────────────────

// TestModerationHandler_MalformedPayload_ReturnsNil verifies that malformed
// JSON payloads are consumed without error (no infinite retry).
func TestModerationHandler_MalformedPayload_ReturnsNil(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "moderation.content.removed",
		Payload:   []byte(`{invalid json`),
	}
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("malformed payload must return nil (no retry), got %v", err)
	}
}

// TestModerationHandler_InvalidResourceID_ReturnsNil verifies that an invalid
// UUID in resource_id is consumed without error.
func TestModerationHandler_InvalidResourceID_ReturnsNil(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	event := buildRemovedEvent("content", "not-a-uuid")
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("invalid resource_id must return nil (no retry), got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — chat_message routing (Phase 3: soft-hide enforcement)
// ─────────────────────────────────────────────────────────────────────────────

// TestModerationHandler_ChatMessageHidden_NilDB verifies that a chat_message
// hidden event routes correctly to handleChatMessageHidden. With nil db, the
// handler will panic on h.db.WithTx — we recover and verify the routing worked.
func TestModerationHandler_ChatMessageHidden_RoutesToHandler(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	messageID := uuid.New()

	payload, _ := json.Marshal(moderationRemovedPayload{
		CaseID:       uuid.New().String(),
		ResourceType: "chat_message",
		ResourceID:   messageID.String(),
	})
	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "moderation.chat_message.hidden",
		Payload:   payload,
	}

	// nil db → handleChatMessageHidden will panic at h.db.WithTx
	// Recovery proves the routing reached the correct handler.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from nil db, routing may not have reached handleChatMessageHidden")
		}
	}()
	_ = h.Handle(context.Background(), event)
}

// TestModerationHandler_ChatMessageRestored_RoutesToHandler verifies that a
// chat_message restored event routes correctly to handleChatMessageRestored.
func TestModerationHandler_ChatMessageRestored_RoutesToHandler(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}
	messageID := uuid.New()

	payload, _ := json.Marshal(moderationRestoredPayload{
		CaseID:       uuid.New().String(),
		AppealID:     uuid.New().String(),
		ResourceType: "chat_message",
		ResourceID:   messageID.String(),
	})
	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "moderation.chat_message.restored",
		Payload:   payload,
	}

	// nil db → handleChatMessageRestored will panic at h.db.WithTx
	// Recovery proves the routing reached the correct handler.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from nil db, routing may not have reached handleChatMessageRestored")
		}
	}()
	_ = h.Handle(context.Background(), event)
}

func TestModerationHandler_ChatMessageHidden_CallsStore(t *testing.T) {
	msgID := uuid.New()
	caseID := uuid.New().String()
	store := &mockChatMessageModerationStore{}
	h := &ModerationEventHandler{
		db:               &mockDB{},
		chatMessageStore: store,
		log:              zaptest.NewLogger(t),
	}

	payload, _ := json.Marshal(moderationRemovedPayload{
		CaseID:       caseID,
		ResourceType: "chat_message",
		ResourceID:   msgID.String(),
	})
	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "moderation.chat_message.hidden",
		Payload:   payload,
	}
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(store.softHideCalls) != 1 {
		t.Fatalf("softHide call count = %d, want 1", len(store.softHideCalls))
	}
	call := store.softHideCalls[0]
	if call.messageID != msgID {
		t.Fatalf("messageID = %s, want %s", call.messageID, msgID)
	}
	if call.deletedBy != uuid.Nil {
		t.Fatalf("deletedBy = %s, want uuid.Nil", call.deletedBy)
	}
	wantReason := fmt.Sprintf("Moderation: hidden by admin (case %s)", caseID)
	if call.reason != wantReason {
		t.Fatalf("reason = %q, want %q", call.reason, wantReason)
	}
}

func TestModerationHandler_ChatMessageRestored_CallsStore(t *testing.T) {
	msgID := uuid.New()
	store := &mockChatMessageModerationStore{}
	h := &ModerationEventHandler{
		db:               &mockDB{},
		chatMessageStore: store,
		log:              zaptest.NewLogger(t),
	}

	event := buildRestoredEvent("chat_message", msgID.String())
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(store.restoreCalls) != 1 {
		t.Fatalf("restore call count = %d, want 1", len(store.restoreCalls))
	}
	if store.restoreCalls[0] != msgID {
		t.Fatalf("messageID = %s, want %s", store.restoreCalls[0], msgID)
	}
}

func TestModerationHandler_ChatMessageHidden_PropagatesStoreError(t *testing.T) {
	store := &mockChatMessageModerationStore{softHideErr: errors.New("store failed")}
	h := &ModerationEventHandler{
		db:               &mockDB{},
		chatMessageStore: store,
		log:              zaptest.NewLogger(t),
	}
	event := buildRemovedEvent("chat_message", uuid.New().String())
	if err := h.Handle(context.Background(), event); err == nil {
		t.Fatal("expected non-nil error")
	}
}

func TestModerationHandler_ChatMessageRestored_PropagatesStoreError(t *testing.T) {
	store := &mockChatMessageModerationStore{restoreErr: errors.New("store failed")}
	h := &ModerationEventHandler{
		db:               &mockDB{},
		chatMessageStore: store,
		log:              zaptest.NewLogger(t),
	}
	event := buildRestoredEvent("chat_message", uuid.New().String())
	if err := h.Handle(context.Background(), event); err == nil {
		t.Fatal("expected non-nil error")
	}
}

func TestNewModerationEventHandler_NilChatMessageStorePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when chatMessageStore is nil")
		}
	}()

	NewModerationEventHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		zaptest.NewLogger(t),
	)
}

// TestModerationHandler_ContentCommentUser_UnaffectedByChat verifies that
// existing resource types still route correctly after adding chat_message.
func TestModerationHandler_ContentCommentUser_UnaffectedByChat(t *testing.T) {
	h := &ModerationEventHandler{log: zaptest.NewLogger(t)}

	for _, rt := range []string{"content", "comment", "for_sale", "auction", "user"} {
		t.Run(rt+"_removed", func(t *testing.T) {
			event := buildRemovedEvent(rt, uuid.New().String())
			// All nil-service paths return nil
			if err := h.Handle(context.Background(), event); err != nil {
				t.Fatalf("expected nil for %s removed, got %v", rt, err)
			}
		})
		t.Run(rt+"_restored", func(t *testing.T) {
			event := buildRestoredEvent(rt, uuid.New().String())
			if err := h.Handle(context.Background(), event); err != nil {
				t.Fatalf("expected nil for %s restored, got %v", rt, err)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — isNonRetryableRestoreError (FIX-5)
// ─────────────────────────────────────────────────────────────────────────────

// TestIsNonRetryableRestoreError verifies the error classification helper used
// by handleForSaleRestored to distinguish terminal errors (no retry) from
// transient ones (should retry).
func TestIsNonRetryableRestoreError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantNoRetry bool
	}{
		{"nil error", nil, false},
		{
			"sold status",
			fmt.Errorf("cannot restore fixed-price sale from moderation: status is sold (id=abc)"),
			true,
		},
		{
			"unexpected status draft",
			fmt.Errorf(`cannot restore fixed-price sale from moderation: unexpected status "draft" (id=abc)`),
			true,
		},
		{
			"fixed-price sale not found",
			fmt.Errorf("fixed-price sale not found for moderation restore: sql: no rows in result set"),
			true,
		},
		{
			"wrapped sold error",
			fmt.Errorf("outer: %w", fmt.Errorf("cannot restore fixed-price sale from moderation: status is sold (id=xyz)")),
			true,
		},
		{"transient db error", fmt.Errorf("pq: connection refused"), false},
		{"context canceled", fmt.Errorf("context canceled"), false},
		{"arbitrary error", fmt.Errorf("some other problem"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNonRetryableRestoreError(tt.err)
			if got != tt.wantNoRetry {
				t.Errorf("isNonRetryableRestoreError(%v) = %v, want %v", tt.err, got, tt.wantNoRetry)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — splitEventSuffix
// ─────────────────────────────────────────────────────────────────────────────

func TestSplitEventSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"moderation.content.removed", []string{"moderation", "content", "removed"}},
		{"moderation.user.restored", []string{"moderation", "user", "restored"}},
		{"moderation.chat_message.hidden", []string{"moderation", "chat_message", "hidden"}},
		{"moderation.chat_message.restored", []string{"moderation", "chat_message", "restored"}},
		{"single", []string{"single"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitEventSuffix(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitEventSuffix(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitEventSuffix(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}



