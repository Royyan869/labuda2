package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap/zaptest"

	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
)

// =============================================================================
// MOCK HANDLER
// =============================================================================

// mockEventHandler records calls and can return a configured error.
type mockEventHandler struct {
	calls  []platformevent.OutboxEvent
	err    error
	label  string // for identification in fanout ordering tests
}

func (m *mockEventHandler) Handle(_ context.Context, event platformevent.OutboxEvent) error {
	m.calls = append(m.calls, event)
	return m.err
}

// =============================================================================
// REGISTER DUPLICATE GUARD
// =============================================================================

func TestRegister_DuplicatePanics(t *testing.T) {
	d := NewOutboxDispatcher(zaptest.NewLogger(t))

	h1 := &mockEventHandler{label: "first"}
	h2 := &mockEventHandler{label: "second"}

	d.Register("test.event", h1)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate Register, got nil")
		}
		msg := fmt.Sprintf("%v", r)
		if msg == "" {
			t.Fatal("panic message is empty")
		}
		t.Logf("panic message: %s", msg)
	}()

	d.Register("test.event", h2) // should panic
	t.Fatal("should not reach here")
}

func TestRegister_SameHandlerIdempotent(t *testing.T) {
	d := NewOutboxDispatcher(zaptest.NewLogger(t))

	h := &mockEventHandler{label: "same"}

	d.Register("test.event", h)
	d.Register("test.event", h) // same handler — no panic

	if len(d.handlers) != 1 {
		t.Fatalf("handlers = %d, want 1", len(d.handlers))
	}
}

func TestRegisterMultiple_DuplicatePanics(t *testing.T) {
	d := NewOutboxDispatcher(zaptest.NewLogger(t))

	h1 := &mockEventHandler{label: "first"}
	h2 := &mockEventHandler{label: "second"}

	d.Register("test.event", h1)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate RegisterMultiple, got nil")
		}
	}()

	d.RegisterMultiple([]string{"test.event", "test.other"}, h2) // should panic on "test.event"
}

// =============================================================================
// REGISTER FANOUT
// =============================================================================

func TestRegisterFanout_ExecutesBothHandlers(t *testing.T) {
	d := NewOutboxDispatcher(zaptest.NewLogger(t))

	h1 := &mockEventHandler{label: "domain"}
	h2 := &mockEventHandler{label: "notification"}

	d.RegisterFanout("test.event", h1, h2)

	event := repository.Event{
		ID:        uuid.New(),
		EventType: "test.event",
		Payload:   []byte(`{}`),
	}

	result, err := d.DispatchWithResult(context.Background(), event)
	if err != nil {
		t.Fatalf("DispatchWithResult error = %v", err)
	}
	if result != DispatchResultHandled {
		t.Errorf("result = %s, want %s", result, DispatchResultHandled)
	}

	if len(h1.calls) != 1 {
		t.Errorf("handler1 calls = %d, want 1", len(h1.calls))
	}
	if len(h2.calls) != 1 {
		t.Errorf("handler2 calls = %d, want 1", len(h2.calls))
	}
}

func TestRegisterFanout_PreservesOrder(t *testing.T) {
	d := NewOutboxDispatcher(zaptest.NewLogger(t))

	var order []string
	h1 := &mockEventHandler{label: "first"}
	h2 := &mockEventHandler{label: "second"}

	// Override Handle to track execution order via closure.
	type orderTracker struct {
		EventHandler
		name string
	}
	tracker1 := &orderTrackingHandler{name: "first", order: &order}
	tracker2 := &orderTrackingHandler{name: "second", order: &order}

	_ = h1 // suppress unused
	_ = h2

	d.RegisterFanout("test.event", tracker1, tracker2)

	event := repository.Event{
		ID:        uuid.New(),
		EventType: "test.event",
		Payload:   []byte(`{}`),
	}

	if _, err := d.DispatchWithResult(context.Background(), event); err != nil {
		t.Fatalf("error = %v", err)
	}

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("execution order = %v, want [first second]", order)
	}
}

func TestRegisterFanout_FirstHandlerFails_SecondNotCalled(t *testing.T) {
	d := NewOutboxDispatcher(zaptest.NewLogger(t))

	h1 := &mockEventHandler{label: "failing", err: errors.New("domain error")}
	h2 := &mockEventHandler{label: "notification"}

	d.RegisterFanout("test.event", h1, h2)

	event := repository.Event{
		ID:        uuid.New(),
		EventType: "test.event",
		Payload:   []byte(`{}`),
	}

	_, err := d.DispatchWithResult(context.Background(), event)
	if err == nil {
		t.Fatal("expected error from failing first handler, got nil")
	}

	if len(h1.calls) != 1 {
		t.Errorf("handler1 calls = %d, want 1", len(h1.calls))
	}
	if len(h2.calls) != 0 {
		t.Errorf("handler2 calls = %d, want 0 (should not be called after first fails)", len(h2.calls))
	}
}

func TestRegisterFanout_SecondHandlerFails_ErrorPropagates(t *testing.T) {
	d := NewOutboxDispatcher(zaptest.NewLogger(t))

	h1 := &mockEventHandler{label: "domain"}
	h2 := &mockEventHandler{label: "failing-notif", err: errors.New("notification error")}

	d.RegisterFanout("test.event", h1, h2)

	event := repository.Event{
		ID:        uuid.New(),
		EventType: "test.event",
		Payload:   []byte(`{}`),
	}

	_, err := d.DispatchWithResult(context.Background(), event)
	if err == nil {
		t.Fatal("expected error from failing second handler, got nil")
	}

	// Both handlers should have been called (first succeeded, second failed).
	if len(h1.calls) != 1 {
		t.Errorf("handler1 calls = %d, want 1", len(h1.calls))
	}
	if len(h2.calls) != 1 {
		t.Errorf("handler2 calls = %d, want 1", len(h2.calls))
	}
}

func TestRegisterFanout_SingleHandler_NoPanic(t *testing.T) {
	d := NewOutboxDispatcher(zaptest.NewLogger(t))

	h := &mockEventHandler{label: "only"}

	d.RegisterFanout("test.event", h)

	event := repository.Event{
		ID:        uuid.New(),
		EventType: "test.event",
		Payload:   []byte(`{}`),
	}

	if _, err := d.DispatchWithResult(context.Background(), event); err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(h.calls) != 1 {
		t.Errorf("calls = %d, want 1", len(h.calls))
	}
}

func TestRegisterFanout_EmptyHandlers_NoOp(t *testing.T) {
	d := NewOutboxDispatcher(zaptest.NewLogger(t))
	d.RegisterFanout("test.event") // no handlers

	// Should be no-handler for this event type.
	event := repository.Event{
		ID:        uuid.New(),
		EventType: "test.event",
		Payload:   []byte(`{}`),
	}

	result, err := d.DispatchWithResult(context.Background(), event)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result != DispatchResultNoHandler {
		t.Errorf("result = %s, want %s", result, DispatchResultNoHandler)
	}
}

// =============================================================================
// NO-HANDLER PATH
// =============================================================================

func TestDispatch_NoHandler_ReturnsNoHandlerResult(t *testing.T) {
	d := NewOutboxDispatcher(zaptest.NewLogger(t))

	event := repository.Event{
		ID:        uuid.New(),
		EventType: "unknown.event",
		Payload:   []byte(`{}`),
	}

	result, err := d.DispatchWithResult(context.Background(), event)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result != DispatchResultNoHandler {
		t.Errorf("result = %s, want %s", result, DispatchResultNoHandler)
	}
}

// =============================================================================
// USER.BANNED FANOUT-READY (SetupWSEvictionHandler composability)
// =============================================================================

func TestWSEvictionHandler_ComposesWithExistingHandler(t *testing.T) {
	// Simulate: UserBanEventHandler registered first, then WSEvictionHandler
	// composes via fanout.
	d := NewOutboxDispatcher(zaptest.NewLogger(t))

	existingHandler := &mockEventHandler{label: "ban-handler"}
	d.Register("user.banned", existingHandler)

	// Verify it's registered.
	if _, ok := d.handlers["user.banned"]; !ok {
		t.Fatal("user.banned should be registered")
	}

	// Now simulate what SetupWSEvictionHandler does when it finds existing.
	evictionHandler := &mockEventHandler{label: "ws-eviction"}
	if existing, ok := d.handlers["user.banned"]; ok {
		d.handlers["user.banned"] = &fanoutHandler{
			handlers: []EventHandler{existing, evictionHandler},
		}
	}

	// Dispatch and verify both execute.
	event := repository.Event{
		ID:        uuid.New(),
		EventType: "user.banned",
		Payload:   []byte(`{"actor_id":"` + uuid.New().String() + `","recipient_id":"` + uuid.New().String() + `"}`),
	}

	result, err := d.DispatchWithResult(context.Background(), event)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result != DispatchResultHandled {
		t.Errorf("result = %s, want %s", result, DispatchResultHandled)
	}
	if len(existingHandler.calls) != 1 {
		t.Errorf("ban-handler calls = %d, want 1", len(existingHandler.calls))
	}
	if len(evictionHandler.calls) != 1 {
		t.Errorf("ws-eviction calls = %d, want 1", len(evictionHandler.calls))
	}
}

// =============================================================================
// HELPERS
// =============================================================================

// orderTrackingHandler records execution order via a shared slice.
type orderTrackingHandler struct {
	name  string
	order *[]string
}

func (h *orderTrackingHandler) Handle(_ context.Context, _ platformevent.OutboxEvent) error {
	*h.order = append(*h.order, h.name)
	return nil
}


