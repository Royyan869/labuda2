//go:build integration

package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// ============================================================================
// ORDER-A1 INVARIANT (DB-backed): a paid order cannot reach
// `status = cancelled` + `escrow_status = holding` through the normal
// Cancel path.
// ============================================================================
//
// The seeded order is promoted to the paid state (escrow holding) — the exact
// state the buyer cancel endpoint must never be able to cancel without moving
// money. Cancel() must be rejected and leave the row untouched.
func TestCancelPaidOrder_RejectedWithoutRefund(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID, buyerID := uuid.New(), uuid.New()
	insertOrderTestUsers(t, ctx, tdb, sellerID, buyerID)
	orderID := seedFPS002PendingOrder(t, ctx, tdb, sellerID, buyerID)

	// Promote the pending order to paid with escrow held.
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE orders
			SET status = 'paid',
			    escrow_status = 'holding',
			    ready_to_ship_by = NOW() + INTERVAL '3 days'
			WHERE id = $1
		`, orderID)
		return err
	}))

	svc := newTestOrderCompletionService(t)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return svc.Cancel(ctx, tx, orderID, "order-a1-cancel-paid", buyerID)
	})
	require.Error(t, err, "cancelling a paid order must be rejected")

	// INVARIANT: still paid, escrow still held.
	assertFPS002Status(t, ctx, tdb, orderID, orderentity.StatusPaid)

	var escrowStatus string
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT escrow_status FROM orders WHERE id = $1`, orderID).Scan(&escrowStatus)
	}))
	require.Equal(t, "holding", escrowStatus)
}
