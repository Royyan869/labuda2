//go:build integration

package tests

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// insertOutboxEvent inserts an outbox event directly into the DB for testing.
func insertOutboxEvent(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	status repository.EventStatus,
	eventType string,
	retryCount int,
	nextAttemptAt time.Time,
) uuid.UUID {
	t.Helper()
	id := uuid.New()
	aggID := uuid.New()
	now := time.Now()
	idempotencyKey := eventType + "." + id.String()

	_, err := pool.Exec(ctx, `
		INSERT INTO outbox (
			id, aggregate_type, aggregate_id, event_type, payload,
			status, retry_count, next_attempt_at, created_at, updated_at, idempotency_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, id, "test", aggID, eventType, `{"test":"data"}`,
		status, retryCount, nextAttemptAt, now, now, idempotencyKey)
	require.NoError(t, err)
	return id
}

// getOutboxStatus reads the current status of an outbox event.
func getOutboxStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, eventID uuid.UUID) repository.EventStatus {
	t.Helper()
	var status repository.EventStatus
	err := pool.QueryRow(ctx, `SELECT status FROM outbox WHERE id = $1`, eventID).Scan(&status)
	require.NoError(t, err)
	return status
}

// TestOutboxRetryLifecycle verifies the complete retry lifecycle against real PostgreSQL.
//
// This test proves:
//  1. pending → processing (standard path)
//  2. failed → processing (retry path — the fix)
//  3. dead_letter → cannot be claimed
//  4. race-safe concurrent claims (FOR UPDATE SKIP LOCKED)
func TestOutboxRetryLifecycle(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	appDB := db.NewFromPool(pool)
	outboxRepo := repository.NewOutboxRepository(appDB)

	now := time.Now()

	t.Run("A - pending event can be claimed as processing", func(t *testing.T) {
		eventID := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "test.pending.claim", 0, now)

		err := appDB.WithTx(ctx, func(tx db.Tx) error {
			return outboxRepo.MarkProcessing(ctx, tx, eventID)
		})
		require.NoError(t, err, "MarkProcessing should succeed for pending event")

		status := getOutboxStatus(t, ctx, pool, eventID)
		require.Equal(t, repository.StatusProcessing, status)
	})

	t.Run("B - failed event can be retried (claimed as processing)", func(t *testing.T) {
		eventID := insertOutboxEvent(t, ctx, pool, repository.StatusFailed, "test.failed.retry", 3, now)

		err := appDB.WithTx(ctx, func(tx db.Tx) error {
			return outboxRepo.MarkProcessing(ctx, tx, eventID)
		})
		require.NoError(t, err, "MarkProcessing should succeed for failed event (retry)")

		status := getOutboxStatus(t, ctx, pool, eventID)
		require.Equal(t, repository.StatusProcessing, status)
	})

	t.Run("C - dead_letter event cannot be claimed", func(t *testing.T) {
		eventID := insertOutboxEvent(t, ctx, pool, repository.StatusDeadLetter, "test.dlq.blocked", 20, now)

		err := appDB.WithTx(ctx, func(tx db.Tx) error {
			return outboxRepo.MarkProcessing(ctx, tx, eventID)
		})
		require.ErrorIs(t, err, repository.ErrInvalidStatusTransition, "MarkProcessing should reject dead_letter event")

		status := getOutboxStatus(t, ctx, pool, eventID)
		require.Equal(t, repository.StatusDeadLetter, status, "dead_letter event should remain unchanged")
	})

	t.Run("D - processing event cannot be double-claimed", func(t *testing.T) {
		eventID := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "test.double.claim", 0, now)

		// First claim
		err := appDB.WithTx(ctx, func(tx db.Tx) error {
			return outboxRepo.MarkProcessing(ctx, tx, eventID)
		})
		require.NoError(t, err)

		// Second claim should fail
		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return outboxRepo.MarkProcessing(ctx, tx, eventID)
		})
		require.ErrorIs(t, err, repository.ErrInvalidStatusTransition, "double claim should be rejected")

		status := getOutboxStatus(t, ctx, pool, eventID)
		require.Equal(t, repository.StatusProcessing, status)
	})

	t.Run("E - succeeded event cannot be claimed", func(t *testing.T) {
		eventID := insertOutboxEvent(t, ctx, pool, repository.StatusSucceeded, "test.succeeded.blocked", 0, now)

		err := appDB.WithTx(ctx, func(tx db.Tx) error {
			return outboxRepo.MarkProcessing(ctx, tx, eventID)
		})
		require.ErrorIs(t, err, repository.ErrInvalidStatusTransition)

		status := getOutboxStatus(t, ctx, pool, eventID)
		require.Equal(t, repository.StatusSucceeded, status)
	})

	t.Run("F - FetchPendingBatch returns both pending and failed events", func(t *testing.T) {
		pendingID := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "test.fetch.mixed", 0, now)
		failedID := insertOutboxEvent(t, ctx, pool, repository.StatusFailed, "test.fetch.mixed.failed", 2, now)
		_ = insertOutboxEvent(t, ctx, pool, repository.StatusDeadLetter, "test.fetch.mixed.dlq", 20, now)

		var events []repository.Event
		err := appDB.WithTx(ctx, func(tx db.Tx) error {
			var fetchErr error
			events, fetchErr = outboxRepo.FetchPendingBatch(ctx, tx, 100)
			return fetchErr
		})
		require.NoError(t, err)

		// Should find pending + failed but NOT dead_letter
		foundIDs := make(map[uuid.UUID]bool)
		for _, e := range events {
			foundIDs[e.ID] = true
		}
		require.True(t, foundIDs[pendingID], "pending event should be in batch")
		require.True(t, foundIDs[failedID], "failed event should be in batch")
	})

	t.Run("G - full retry lifecycle: pending → processing → failed → processing → succeeded", func(t *testing.T) {
		eventID := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "test.full.lifecycle", 0, now)

		// Step 1: pending → processing
		err := appDB.WithTx(ctx, func(tx db.Tx) error {
			return outboxRepo.MarkProcessing(ctx, tx, eventID)
		})
		require.NoError(t, err)
		require.Equal(t, repository.StatusProcessing, getOutboxStatus(t, ctx, pool, eventID))

		// Step 2: processing → failed (simulating handler failure)
		nextRetry := time.Now().Add(1 * time.Second)
		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return outboxRepo.MarkFailedWithRetry(ctx, tx, eventID, 1, nextRetry)
		})
		require.NoError(t, err)
		require.Equal(t, repository.StatusFailed, getOutboxStatus(t, ctx, pool, eventID))

		// Step 3: failed → processing (retry — the critical path)
		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return outboxRepo.MarkProcessing(ctx, tx, eventID)
		})
		require.NoError(t, err, "retry: failed → processing should succeed")
		require.Equal(t, repository.StatusProcessing, getOutboxStatus(t, ctx, pool, eventID))

		// Step 4: processing → succeeded
		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return outboxRepo.MarkSucceeded(ctx, tx, eventID)
		})
		require.NoError(t, err)
		require.Equal(t, repository.StatusSucceeded, getOutboxStatus(t, ctx, pool, eventID))
	})
}

// TestOutboxConcurrentClaimRaceSafety proves that two workers cannot both
// claim the same event. Uses FOR UPDATE SKIP LOCKED so exactly one wins.
func TestOutboxConcurrentClaimRaceSafety(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	appDB := db.NewFromPool(pool)
	outboxRepo := repository.NewOutboxRepository(appDB)

	now := time.Now()

	t.Run("exactly one of two concurrent claims succeeds", func(t *testing.T) {
		eventID := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "test.race.claim", 0, now)

		var successCount atomic.Int32
		var wg sync.WaitGroup

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Each goroutine uses its own transaction
				err := appDB.WithTx(ctx, func(tx db.Tx) error {
					return outboxRepo.MarkProcessing(ctx, tx, eventID)
				})
				if err == nil {
					successCount.Add(1)
				}
			}()
		}

		wg.Wait()

		require.Equal(t, int32(1), successCount.Load(), "exactly one concurrent claim should succeed (FOR UPDATE SKIP LOCKED)")

		status := getOutboxStatus(t, ctx, pool, eventID)
		require.Equal(t, repository.StatusProcessing, status, "event should be in processing state")
	})

	t.Run("concurrent claims on different events both succeed", func(t *testing.T) {
		eventID1 := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "test.race.separate1", 0, now)
		eventID2 := insertOutboxEvent(t, ctx, pool, repository.StatusPending, "test.race.separate2", 0, now)

		var wg sync.WaitGroup
		var err1, err2 error

		wg.Add(2)
		go func() {
			defer wg.Done()
			err1 = appDB.WithTx(ctx, func(tx db.Tx) error {
				return outboxRepo.MarkProcessing(ctx, tx, eventID1)
			})
		}()
		go func() {
			defer wg.Done()
			err2 = appDB.WithTx(ctx, func(tx db.Tx) error {
				return outboxRepo.MarkProcessing(ctx, tx, eventID2)
			})
		}()

		wg.Wait()

		require.NoError(t, err1, "claiming event 1 should succeed")
		require.NoError(t, err2, "claiming event 2 should succeed")
		require.Equal(t, repository.StatusProcessing, getOutboxStatus(t, ctx, pool, eventID1))
		require.Equal(t, repository.StatusProcessing, getOutboxStatus(t, ctx, pool, eventID2))
	})
}
