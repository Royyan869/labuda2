package serverboot

import (
	"os"
	"testing"

	"go.uber.org/zap/zaptest"
)

// TestModerationEventHandler_DefaultOn verifies the handler is default-ON.
// FIX-O2A: previous defaultOn=false caused silent no-op on all moderation enforcement.
func TestModerationEventHandler_DefaultOn(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Unsetenv("DISABLE_MODERATION_EVENT_HANDLER")

	if !workerEnabled("MODERATION_EVENT_HANDLER", true, log) {
		t.Fatal("MODERATION_EVENT_HANDLER should be enabled by default")
	}
}

// TestModerationEventHandler_DisabledByEnv verifies explicit disable works.
func TestModerationEventHandler_DisabledByEnv(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Setenv("DISABLE_MODERATION_EVENT_HANDLER", "true")
	defer os.Unsetenv("DISABLE_MODERATION_EVENT_HANDLER")

	if workerEnabled("MODERATION_EVENT_HANDLER", true, log) {
		t.Fatal("MODERATION_EVENT_HANDLER should be disabled when DISABLE_MODERATION_EVENT_HANDLER=true")
	}
}

// TestModerationEventHandler_EnabledByEnv verifies explicit enable works.
func TestModerationEventHandler_EnabledByEnv(t *testing.T) {
	log := zaptest.NewLogger(t)
	os.Setenv("DISABLE_MODERATION_EVENT_HANDLER", "false")
	defer os.Unsetenv("DISABLE_MODERATION_EVENT_HANDLER")

	if !workerEnabled("MODERATION_EVENT_HANDLER", true, log) {
		t.Fatal("MODERATION_EVENT_HANDLER should be enabled when DISABLE_MODERATION_EVENT_HANDLER=false")
	}
}


