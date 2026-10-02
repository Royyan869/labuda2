package worker

import (
	"testing"
	"time"
)

// TestDefaultOutboxPollInterval_Interactive is the negative contract against a
// batch-latency regression of the outbox dispatch cadence.
//
// RUNTIME EVIDENCE (2026-09-30): a negotiation offer POSTed at 20:05:00 only
// reached chat_messages at 20:05:55 — after the buyer's last GET /messages
// (20:05:53) — because the default poll interval was 1 minute. Every hop of
// the canonical chain (StartNegotiation → outbox → chat consumer → message)
// was correct; only the poll cadence made the offer look lost.
//
// Outbox events are user-facing (chat proposals, notifications, realtime room
// updates), so dispatch must stay interactive. Raise this only with a
// measured throughput requirement, never for convenience.
func TestDefaultOutboxPollInterval_Interactive(t *testing.T) {
	if DefaultOutboxPollInterval > 2*time.Second {
		t.Fatalf(
			"DefaultOutboxPollInterval = %v, want <= 2s: outbox events are "+
				"user-facing and a slow poll makes a successful write look lost",
			DefaultOutboxPollInterval,
		)
	}
	if DefaultOutboxPollInterval <= 0 {
		t.Fatalf("DefaultOutboxPollInterval = %v, want > 0", DefaultOutboxPollInterval)
	}

	cfg := DefaultOutboxWorkerConfig()
	if cfg.PollInterval != DefaultOutboxPollInterval {
		t.Fatalf(
			"DefaultOutboxWorkerConfig().PollInterval = %v, want the canonical default %v",
			cfg.PollInterval,
			DefaultOutboxPollInterval,
		)
	}
}
