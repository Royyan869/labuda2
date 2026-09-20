//go:build integration

package tests

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/internal/realtime"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// realtimeOwnershipScope is the claim scope of the realtime worker.
func realtimeOwnershipScope() repository.EventOwnershipScope {
	return repository.EventOwnershipScope{Include: realtime.OwnedOutboxEventTypes}
}

// outboxOwnershipScope is the claim scope of the default owner (outbox worker).
func outboxOwnershipScope() repository.EventOwnershipScope {
	return repository.EventOwnershipScope{Exclude: realtime.OwnedOutboxEventTypes}
}

// fetchOwned loads the events a consumer may claim, against real PostgreSQL.
func fetchOwned(
	t *testing.T,
	ctx context.Context,
	appDB *db.DB,
	outboxRepo *repository.OutboxRepository,
	scope repository.EventOwnershipScope,
) []uuid.UUID {
	t.Helper()

	var ids []uuid.UUID
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		events, fetchErr := outboxRepo.FetchPendingBatch(ctx, tx, 100, scope)
		for _, e := range events {
			ids = append(ids, e.ID)
		}
		return fetchErr
	})
	require.NoError(t, err)
	return ids
}

func containsID(ids []uuid.UUID, want uuid.UUID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// TestOutboxOwnership_ScopesPartitionClaimableEvents proves against real
// PostgreSQL that the two production claim scopes are disjoint: each event type
// is claimable by exactly one consumer, and neither consumer can select the
// other's rows. This is the durable ownership enforcement — no process-local
// coordination is involved.
func TestOutboxOwnership_ScopesPartitionClaimableEvents(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	appDB := db.NewFromPool(pool)
	outboxRepo := repository.NewOutboxRepository(appDB)

	now := time.Now()
	realtimeEventID := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "chat.message.sent", 0, now)
	notificationEventID := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "chat.message.notification", 0, now)

	realtimeClaimable := fetchOwned(t, ctx, appDB, outboxRepo, realtimeOwnershipScope())
	require.True(t, containsID(realtimeClaimable, realtimeEventID),
		"realtime worker must be able to claim its own chat.message.sent event")
	require.False(t, containsID(realtimeClaimable, notificationEventID),
		"realtime worker must NOT be able to claim the outbox-owned chat.message.notification event")

	outboxClaimable := fetchOwned(t, ctx, appDB, outboxRepo, outboxOwnershipScope())
	require.True(t, containsID(outboxClaimable, notificationEventID),
		"outbox worker must be able to claim its own chat.message.notification event")
	require.False(t, containsID(outboxClaimable, realtimeEventID),
		"outbox worker must NOT be able to claim the realtime-owned chat.message.sent event")
}

// TestOutboxOwnership_RealtimeRoomEventsAreNotOutboxClaimable proves the second
// half of the known defect is gone: chat.room.created / chat.room.updated can no
// longer fall to the outbox worker (which has no handler for them and would have
// marked them succeeded, silently dropping the WebSocket frame).
func TestOutboxOwnership_RealtimeRoomEventsAreNotOutboxClaimable(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	appDB := db.NewFromPool(pool)
	outboxRepo := repository.NewOutboxRepository(appDB)

	now := time.Now()
	createdID := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "chat.room.created", 0, now)
	updatedID := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "chat.room.updated", 0, now)

	outboxClaimable := fetchOwned(t, ctx, appDB, outboxRepo, outboxOwnershipScope())
	require.False(t, containsID(outboxClaimable, createdID),
		"outbox worker must NOT be able to claim chat.room.created")
	require.False(t, containsID(outboxClaimable, updatedID),
		"outbox worker must NOT be able to claim chat.room.updated")

	realtimeClaimable := fetchOwned(t, ctx, appDB, outboxRepo, realtimeOwnershipScope())
	require.True(t, containsID(realtimeClaimable, createdID), "realtime worker owns chat.room.created")
	require.True(t, containsID(realtimeClaimable, updatedID), "realtime worker owns chat.room.updated")
}

// TestOutboxOwnership_ConcurrentClaimsHaveSingleWinner proves the claim model
// under real concurrency: many racing claimants on one row yield exactly one
// successful claim and the row is never double-delivered.
func TestOutboxOwnership_ConcurrentClaimsHaveSingleWinner(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	appDB := db.NewFromPool(pool)
	outboxRepo := repository.NewOutboxRepository(appDB)

	eventID := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "chat.message.notification", 0, time.Now())

	const racers = 8
	var wg sync.WaitGroup
	var wins int32

	var mu sync.Mutex
	var unexpected []error

	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := appDB.WithTx(ctx, func(tx db.Tx) error {
				return outboxRepo.MarkProcessing(ctx, tx, eventID)
			})
			switch {
			case err == nil:
				atomic.AddInt32(&wins, 1)
			case err == repository.ErrInvalidStatusTransition:
				// expected for every loser
			default:
				mu.Lock()
				unexpected = append(unexpected, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	require.Empty(t, unexpected, "only ErrInvalidStatusTransition is acceptable for losing claimants")
	require.Equal(t, int32(1), atomic.LoadInt32(&wins), "exactly one claimant may win")

	status := getOutboxStatus(t, ctx, pool, eventID)
	require.Equal(t, repository.StatusProcessing, status)
}

// TestOutboxOwnership_ConcurrentScopedClaimsNeverCrossOwners runs both consumers
// concurrently over a mixed pending set and proves that every claimed event was
// claimed by its owner, and that no event was left behind or double-claimed.
func TestOutboxOwnership_ConcurrentScopedClaimsNeverCrossOwners(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	appDB := db.NewFromPool(pool)
	outboxRepo := repository.NewOutboxRepository(appDB)

	now := time.Now()
	const perOwner = 5

	realtimeEvents := make(map[uuid.UUID]struct{}, perOwner)
	notificationEvents := make(map[uuid.UUID]struct{}, perOwner)

	for i := 0; i < perOwner; i++ {
		realtimeEvents[insertOutboxEvent(t, ctx, pool, repository.StatusPending, "chat.message.sent", 0, now)] = struct{}{}
		notificationEvents[insertOutboxEvent(t, ctx, pool, repository.StatusPending, "chat.message.notification", 0, now)] = struct{}{}
	}

	claimedBy := func(scope repository.EventOwnershipScope) map[uuid.UUID]struct{} {
		claimed := make(map[uuid.UUID]struct{})
		for attempt := 0; attempt < 10; attempt++ {
			err := appDB.WithTx(ctx, func(tx db.Tx) error {
				events, fetchErr := outboxRepo.FetchPendingBatch(ctx, tx, 100, scope)
				if fetchErr != nil {
					return fetchErr
				}
				for _, e := range events {
					if markErr := outboxRepo.MarkProcessing(ctx, tx, e.ID); markErr != nil {
						return markErr
					}
					claimed[e.ID] = struct{}{}
				}
				return nil
			})
			require.NoError(t, err)
		}
		return claimed
	}

	var wg sync.WaitGroup
	var realtimeClaimed, outboxClaimed map[uuid.UUID]struct{}

	wg.Add(2)
	go func() {
		defer wg.Done()
		realtimeClaimed = claimedBy(realtimeOwnershipScope())
	}()
	go func() {
		defer wg.Done()
		outboxClaimed = claimedBy(outboxOwnershipScope())
	}()
	wg.Wait()

	for id := range realtimeEvents {
		_, claimedByRealtime := realtimeClaimed[id]
		_, claimedByOutbox := outboxClaimed[id]
		require.True(t, claimedByRealtime, "realtime-owned event %s must be claimed by the realtime consumer", id)
		require.False(t, claimedByOutbox, "realtime-owned event %s must never be claimed by the outbox consumer", id)
	}

	for id := range notificationEvents {
		_, claimedByRealtime := realtimeClaimed[id]
		_, claimedByOutbox := outboxClaimed[id]
		require.True(t, claimedByOutbox, "outbox-owned event %s must be claimed by the outbox consumer", id)
		require.False(t, claimedByRealtime, "outbox-owned event %s must never be claimed by the realtime consumer", id)
	}
}
