//go:build integration

package tests

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestFPS002_ConcurrentCancelVsExpire(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID, buyerID := uuid.New(), uuid.New()
	insertOrderTestUsers(t, ctx, tdb, sellerID, buyerID)
	orderID := seedFPS002PendingOrder(t, ctx, tdb, sellerID, buyerID)
	svc := newTestOrderCompletionService(t)

	start := make(chan struct{})
	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		results <- result{"cancel", tdb.WithTx(ctx, func(tx db.Tx) error {
			return svc.Cancel(ctx, tx, orderID, "fps-002-concurrent-cancel", buyerID)
		})}
	}()
	go func() {
		defer wg.Done()
		<-start
		results <- result{"expire", tdb.WithTx(ctx, func(tx db.Tx) error {
			return svc.Expire(ctx, tx, orderID)
		})}
	}()
	close(start)
	wg.Wait()
	first, second := <-results, <-results
	require.NotEqual(t, first.err == nil, second.err == nil, "%s=%v, %s=%v", first.name, first.err, second.name, second.err)

	var want orderentity.Status
	if first.err == nil {
		if first.name == "cancel" {
			want = orderentity.StatusCancelled
		} else {
			want = orderentity.StatusExpired
		}
	} else if second.name == "cancel" {
		want = orderentity.StatusCancelled
	} else {
		want = orderentity.StatusExpired
	}
	assertFPS002Status(t, ctx, tdb, orderID, want)
	assertFPS002Quantity(t, ctx, tdb, orderID, 1)
}

func TestFPS002_CompletionRollbackThenRetry(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID, buyerID := uuid.New(), uuid.New()
	insertOrderTestUsers(t, ctx, tdb, sellerID, buyerID)
	orderID := seedFPS002PendingOrder(t, ctx, tdb, sellerID, buyerID)
	svc := newTestOrderCompletionService(t)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		require.NoError(t, svc.Cancel(ctx, tx, orderID, "fps-002-rollback", buyerID))
		return errors.New("inject outer transaction rollback")
	})
	require.EqualError(t, err, "inject outer transaction rollback")
	assertFPS002Status(t, ctx, tdb, orderID, orderentity.StatusPending)
	assertFPS002Quantity(t, ctx, tdb, orderID, 0)

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return svc.Cancel(ctx, tx, orderID, "fps-002-retry", buyerID)
	}))
	assertFPS002Status(t, ctx, tdb, orderID, orderentity.StatusCancelled)
	assertFPS002Quantity(t, ctx, tdb, orderID, 1)
}
