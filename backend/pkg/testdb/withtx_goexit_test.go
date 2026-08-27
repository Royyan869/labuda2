//go:build integration

package testdb

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/require"
)

// fakeFailNowT is a minimal require.TestingT that mimics testify's real
// failure path (Errorf then FailNow, exactly like *testing.T) without
// touching the real *testing.T — so triggering it inside a goroutine aborts
// only that goroutine via runtime.Goexit, instead of contaminating this
// test's own pass/fail state the way a failing t.Run subtest would.
type fakeFailNowT struct{ failed bool }

func (f *fakeFailNowT) Errorf(format string, args ...interface{}) { f.failed = true }
func (f *fakeFailNowT) FailNow()                                  { runtime.Goexit() }

// TestWithTx_GoexitDuringClosure_ReleasesConnection (PASS_19D) is the
// regression guard for the connection-leak hang found in
// TestForSaleStockRoundTrip_MultiQty: a closure passed to TestDB.WithTx that
// calls testify's require.* (or t.Fatal) aborts via runtime.Goexit rather
// than a normal return. Before this pass, WithTx's deferred rollback was
// guarded by checking the outer error variable that "err = fn(tx)" would
// have set — but that assignment never executes when fn exits via Goexit,
// so the rollback never fired and the transaction's connection stayed
// checked out forever, hanging any later pgxpool.Pool.Close() in test
// cleanup (observed as a 600s test timeout).
//
// This test reproduces the exact pattern (require.NoError failing inside a
// WithTx closure) using fakeFailNowT in its own goroutine — Goexit only
// terminates that goroutine, not this outer test — and then proves the pool
// still has a usable connection immediately after, with a bounded wait so a
// regression fails fast instead of hanging the whole suite again.
func TestWithTx_GoexitDuringClosure_ReleasesConnection(t *testing.T) {
	testDB, cleanup := SetupDB(t)
	defer cleanup()

	failingClosureDone := make(chan struct{})
	go func() {
		defer close(failingClosureDone)
		ft := &fakeFailNowT{}
		_ = testDB.WithTx(context.Background(), func(tx db.Tx) error {
			require.NoError(ft, errors.New("boom: simulated closure failure"))
			return nil // unreachable — Goexit already unwound this goroutine
		})
	}()
	<-failingClosureDone

	// If WithTx leaked the transaction's connection above, this call blocks
	// forever waiting for the pool to free one up (bounded MaxConns in
	// tests). It must complete promptly instead of hanging.
	done := make(chan error, 1)
	go func() {
		done <- testDB.WithTx(context.Background(), func(tx db.Tx) error {
			_, err := tx.Exec(context.Background(), "SELECT 1")
			return err
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err, "connection must be usable after the Goexit-triggered closure failure above")
	case <-time.After(15 * time.Second):
		t.Fatal("REGRESSION: WithTx leaked a connection after a Goexit-triggered closure failure — pool exhausted")
	}
}
