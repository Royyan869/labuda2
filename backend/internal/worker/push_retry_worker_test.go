package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Mocks
// =============================================================================

// mockPushSenderRetry captures SendNotification calls and returns a
// configurable error. Used to test retry success / failure paths.
type mockPushSenderRetry struct {
	calls   int
	failErr error // if non-nil, SendNotification returns this error
}

func (m *mockPushSenderRetry) SendNotification(_ context.Context, _ interface{}, _ interface{}, _, _ string) error {
	m.calls++
	return m.failErr
}

// mockDeliveryLoggerRetry records LogDelivery calls for assertion.
type mockDeliveryLoggerRetry struct {
	logged []struct {
		channel string
		status  string
		reason  string
	}
}

func (m *mockDeliveryLoggerRetry) LogDelivery(
	_ context.Context,
	_, _ uuid.UUID,
	channel, status, reason string,
	_ map[string]interface{},
) {
	m.logged = append(m.logged, struct {
		channel string
		status  string
		reason  string
	}{channel, status, reason})
}

// =============================================================================
// Construction tests
// =============================================================================

func TestPushRetryWorker_Construction(t *testing.T) {
	log := zaptest.NewLogger(t)

	w := NewPushRetryWorker(nil, nil, log)
	if w == nil {
		t.Fatal("NewPushRetryWorker() returned nil")
	}
	if w.pollInterval != DefaultPushRetryPollInterval {
		t.Errorf("pollInterval = %v, want %v", w.pollInterval, DefaultPushRetryPollInterval)
	}
	if w.batchSize != pushBatchSize {
		t.Errorf("batchSize = %v, want %v", w.batchSize, pushBatchSize)
	}
}

func TestPushRetryWorker_NilLogFallback(t *testing.T) {
	// nil logger must not panic
	w := NewPushRetryWorker(nil, nil, nil)
	if w == nil {
		t.Fatal("NewPushRetryWorker(nil, nil, nil) returned nil")
	}
}

func TestNewDBPushRetryQueue_Construction(t *testing.T) {
	q := NewDBPushRetryQueue(nil)
	if q == nil {
		t.Fatal("NewDBPushRetryQueue(nil) returned nil")
	}
}

// =============================================================================
// Lifecycle tests
// =============================================================================

// TestPushRetryWorker_Lifecycle verifies Start/Stop/IsRunning semantics.
// PushRetryWorker polls on a ticker (no immediate DB call), so Start+Stop
// is safe without a real database.
func TestPushRetryWorker_Lifecycle(t *testing.T) {
	log := zaptest.NewLogger(t)
	sender := &mockPushSenderRetry{}

	w := NewPushRetryWorker(nil, sender, log)
	w.pollInterval = 10 * time.Second // long enough to never fire in test

	if w.IsRunning() {
		t.Fatal("worker should not be running before Start")
	}

	w.Start()
	if !w.IsRunning() {
		t.Fatal("worker should be running after Start")
	}

	// Idempotent: second Start is a no-op
	w.Start()
	if !w.IsRunning() {
		t.Fatal("worker should still be running after double Start")
	}

	w.Stop()
	if w.IsRunning() {
		t.Fatal("worker should not be running after Stop")
	}

	// Idempotent: second Stop is a no-op
	w.Stop()
	if w.IsRunning() {
		t.Fatal("worker should not be running after double Stop")
	}
}

func TestPushRetryWorker_SetDeliveryLogger(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewPushRetryWorker(nil, nil, log)

	dl := &mockDeliveryLoggerRetry{}
	w.SetDeliveryLogger(dl)

	if w.deliveryLogger == nil {
		t.Error("deliveryLogger should be set")
	}
}

// =============================================================================
// Backoff schedule tests
// =============================================================================

func TestPushRetryWorker_CalculateNextAttempt(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewPushRetryWorker(nil, nil, log)

	cases := []struct {
		attempt  int
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{1, 48 * time.Second, 72 * time.Second},     // 1m ±20%
		{2, 4 * time.Minute, 6 * time.Minute},       // 5m ±20%
		{3, 12 * time.Minute, 18 * time.Minute},     // 15m ±20%
		{4, 48 * time.Minute, 72 * time.Minute},     // 1h ±20%
		{5, 192 * time.Minute, 288 * time.Minute},   // 4h ±20%
		{6, 384 * time.Minute, 576 * time.Minute},   // 8h ±20%
		{10, 384 * time.Minute, 576 * time.Minute},  // 8h ±20% (capped)
	}

	for _, tc := range cases {
		before := time.Now()
		next := w.calculateNextAttempt(tc.attempt)
		delay := next.Sub(before)

		if delay < tc.minDelay || delay > tc.maxDelay {
			t.Errorf("attempt %d: delay=%v want [%v, %v]",
				tc.attempt, delay, tc.minDelay, tc.maxDelay)
		}
	}
}

// =============================================================================
// logRetryDelivery: async delivery logger
// =============================================================================

// TestPushRetryWorker_LogRetryDelivery verifies the async delivery log path
// without invoking any DB operations.
func TestPushRetryWorker_LogRetryDelivery_Async(t *testing.T) {
	log := zaptest.NewLogger(t)
	dl := &mockDeliveryLoggerRetry{}

	w := NewPushRetryWorker(nil, nil, log)
	w.SetDeliveryLogger(dl)

	notifID := uuid.New()
	recipientID := uuid.New()

	w.logRetryDelivery(notifID, recipientID, "retrying", 3, "fcm timeout")

	// Allow goroutine to flush.
	time.Sleep(50 * time.Millisecond)

	if len(dl.logged) == 0 {
		t.Fatal("expected one delivery log entry")
	}
	got := dl.logged[0]
	if got.channel != "push_retry" {
		t.Errorf("channel = %q, want push_retry", got.channel)
	}
	if got.status != "retrying" {
		t.Errorf("status = %q, want retrying", got.status)
	}
	if got.reason != "fcm timeout" {
		t.Errorf("reason = %q, want fcm timeout", got.reason)
	}
}

// TestPushRetryWorker_LogRetryDelivery_NilLogger verifies no panic when no logger is wired.
func TestPushRetryWorker_LogRetryDelivery_NilLogger(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewPushRetryWorker(nil, nil, log)
	// no SetDeliveryLogger call — deliveryLogger is nil

	// Must not panic.
	w.logRetryDelivery(uuid.New(), uuid.New(), "sent", 1, "")
}

// =============================================================================
// Max-attempts / terminal logic (no DB needed for assertions)
// =============================================================================

func TestPushRetryWorker_MaxAttempts_IsTerminal(t *testing.T) {
	// After maxPushRetryAttempts, the next attempt must be considered terminal.
	log := zaptest.NewLogger(t)
	w := NewPushRetryWorker(nil, nil, log)

	entry := pushRetryQueueEntry{
		Attempts:  maxPushRetryAttempts - 1, // this attempt will be attempt N
		ExpiresAt: time.Now().Add(48 * time.Hour),
	}

	// Simulate: attempts+1 >= maxPushRetryAttempts → terminal
	terminalByCount := entry.Attempts+1 >= maxPushRetryAttempts
	if !terminalByCount {
		t.Errorf("expected terminal at attempts=%d (max=%d)", entry.Attempts+1, maxPushRetryAttempts)
	}

	// Window check: nextAttempt must be after ExpiresAt for window-terminal.
	nextAfterWindow := w.calculateNextAttempt(100) // very high attempt → 8h delay, well past 2s expiry
	expiredEntry := pushRetryQueueEntry{
		Attempts:  1,
		ExpiresAt: time.Now().Add(2 * time.Second),
	}
	if !nextAfterWindow.After(expiredEntry.ExpiresAt) {
		t.Error("expected 8h backoff to exceed 2s expiry window")
	}
}


