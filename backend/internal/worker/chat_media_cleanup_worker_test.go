package worker

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// The sweep itself is SQL against chat_media_assets (proven by the chat
// repository/integration suites); these tests pin the worker's lifecycle
// contract — defaults, overrides, and the running flag.

func TestChatMediaCleanupWorker_DefaultInterval(t *testing.T) {
	if DefaultChatMediaCleanupInterval != time.Hour {
		t.Errorf("DefaultChatMediaCleanupInterval = %v, want 1h", DefaultChatMediaCleanupInterval)
	}
}

func TestChatMediaCleanupWorker_Construction(t *testing.T) {
	w := NewChatMediaCleanupWorker(nil, zaptest.NewLogger(t))
	if w == nil {
		t.Fatal("NewChatMediaCleanupWorker() returned nil")
	}
	if w.interval != DefaultChatMediaCleanupInterval {
		t.Errorf("interval = %v, want %v", w.interval, DefaultChatMediaCleanupInterval)
	}
	if w.IsRunning() {
		t.Error("a freshly constructed worker must not report running")
	}
}

func TestChatMediaCleanupWorker_NilLogFallback(t *testing.T) {
	w := NewChatMediaCleanupWorker(nil, nil)
	if w.log == nil {
		t.Fatal("nil logger must fall back to a no-op logger")
	}
}

func TestChatMediaCleanupWorker_SetInterval(t *testing.T) {
	w := NewChatMediaCleanupWorker(nil, zaptest.NewLogger(t))

	w.SetInterval(30 * time.Minute)
	if w.interval != 30*time.Minute {
		t.Errorf("interval = %v, want 30m", w.interval)
	}

	// A non-positive override must not disable the loop by accident.
	w.SetInterval(0)
	if w.interval != 30*time.Minute {
		t.Errorf("interval = %v, want the previous 30m", w.interval)
	}
}

func TestChatMediaCleanupWorker_StopWithoutStartIsNoop(t *testing.T) {
	w := NewChatMediaCleanupWorker(nil, zaptest.NewLogger(t))
	w.Stop()
	if w.IsRunning() {
		t.Error("Stop() without Start() must leave the worker stopped")
	}
}
