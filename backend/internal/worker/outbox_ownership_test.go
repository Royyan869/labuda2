package worker

import (
	"testing"

	"go.uber.org/zap"

	"github.com/labuda/backend/internal/realtime"
)

// newTestOutboxWorker builds an OutboxWorker for ownership assertions. The
// constructor performs no database IO, so a nil DB is sufficient to inspect the
// claim scope the worker is actually wired with.
func newTestOutboxWorker(t *testing.T) *OutboxWorker {
	t.Helper()
	return NewOutboxWorker(nil, zap.NewNop(), DefaultOutboxWorkerConfig())
}

// TestOutboxWorker_OwnershipScopeExcludesRealtimeOwnedEvents proves the outbox
// worker is the default owner of every event type EXCEPT the realtime-owned set:
// it uses an exclude scope containing exactly the realtime declaration, so it can
// never claim a realtime event (one event type = one owning consumer).
func TestOutboxWorker_OwnershipScopeExcludesRealtimeOwnedEvents(t *testing.T) {
	w := newTestOutboxWorker(t)
	owned := realtime.OwnedOutboxEventTypes

	if len(w.ownershipScope.Exclude) == 0 {
		t.Fatal("outbox worker has no ownership scope: it could claim another consumer's events")
	}
	if len(w.ownershipScope.Include) != 0 {
		t.Fatalf("outbox worker is the default owner and must use an exclude scope, got include %v", w.ownershipScope.Include)
	}

	excluded := toSet(w.ownershipScope.Exclude)
	if len(excluded) != len(owned) {
		t.Fatalf("outbox ownership scope has %d entries, want %d", len(excluded), len(owned))
	}
	for _, eventType := range owned {
		if _, ok := excluded[eventType]; !ok {
			t.Errorf("outbox worker could claim realtime-owned event %q", eventType)
		}
	}
}

// TestOwnership_EveryProducedEventHasExactlyOneClaimant is the partition proof of
// the ownership invariant: for every produced event type, exactly one consumer's
// claim scope selects it. There is no event both workers can claim (the
// competing-claim defect) and no event neither can claim (a lost event).
func TestOwnership_EveryProducedEventHasExactlyOneClaimant(t *testing.T) {
	w := newTestOutboxWorker(t)

	realtimeOwned := toSet(realtime.OwnedOutboxEventTypes)
	excludedFromOutbox := toSet(w.ownershipScope.Exclude)

	for _, eventType := range knownProducedEvents {
		_, claimedByRealtime := realtimeOwned[eventType]
		_, skippedByOutbox := excludedFromOutbox[eventType]
		claimedByOutbox := !skippedByOutbox

		if claimedByRealtime == claimedByOutbox {
			t.Errorf(
				"event %q must be claimable by exactly one consumer (realtime=%v outbox=%v)",
				eventType, claimedByRealtime, claimedByOutbox,
			)
		}
	}
}
