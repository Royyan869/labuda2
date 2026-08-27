package worker

import (
	"testing"

	"github.com/labuda/backend/internal/platform/events"
	"github.com/labuda/backend/internal/presence"
	"go.uber.org/zap/zaptest"
)

func TestOutboxWorker_SetupPresenceLastSeenHandler_RegistersCanonicalEvent(t *testing.T) {
	w := NewOutboxWorker(nil, zaptest.NewLogger(t), DefaultOutboxWorkerConfig())
	svc := presence.NewService(nil, nil, nil, zaptest.NewLogger(t))

	w.SetupPresenceLastSeenHandler(svc)

	handler, ok := w.dispatcher.handlers[events.EventUserPresenceLastSeenRecord]
	if !ok || handler == nil {
		t.Fatal("presence last_seen handler was not registered")
	}
}
