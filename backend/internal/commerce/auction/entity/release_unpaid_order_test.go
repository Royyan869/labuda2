package entity

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PASS_20B (D2): ReleaseUnpaidOrder clears the auction<->order binding after
// a bound order is cancelled/expired before payment succeeded.

func TestReleaseUnpaidOrder_ClearsMatchingBinding(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusActive
	orderID := uuid.New()
	auction.OrderID = &orderID
	require.NoError(t, auction.End())
	require.Equal(t, StatusEnded, auction.Status)

	err := auction.ReleaseUnpaidOrder(orderID)

	require.NoError(t, err)
	assert.Nil(t, auction.OrderID, "OrderID must be cleared after release")
}

func TestReleaseUnpaidOrder_StatusStaysEnded(t *testing.T) {
	// Ended is a deliberate terminal state (see transitionAllowed) — release
	// must not attempt to reopen the auction for further bids/buy-now.
	auction := createTestDraftAuction()
	auction.Status = StatusActive
	orderID := uuid.New()
	auction.OrderID = &orderID
	require.NoError(t, auction.End())

	require.NoError(t, auction.ReleaseUnpaidOrder(orderID))

	assert.Equal(t, StatusEnded, auction.Status, "auction status must remain Ended, not reopen to Active")
}

func TestReleaseUnpaidOrder_MismatchedOrderRejected(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusActive
	boundOrderID := uuid.New()
	auction.OrderID = &boundOrderID
	require.NoError(t, auction.End())

	otherOrderID := uuid.New()
	err := auction.ReleaseUnpaidOrder(otherOrderID)

	require.ErrorIs(t, err, ErrOrderBindingMismatch)
	require.NotNil(t, auction.OrderID, "a mismatched release must never clear another order's binding")
	assert.Equal(t, boundOrderID, *auction.OrderID)
}

func TestReleaseUnpaidOrder_AlreadyReleased_IdempotentNoOp(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusActive
	orderID := uuid.New()
	auction.OrderID = &orderID
	require.NoError(t, auction.End())
	require.NoError(t, auction.ReleaseUnpaidOrder(orderID))
	require.Nil(t, auction.OrderID)

	// Calling again (e.g. a retried worker pass after a prior partial
	// failure) must be a harmless no-op, not an error.
	err := auction.ReleaseUnpaidOrder(orderID)

	require.NoError(t, err)
	assert.Nil(t, auction.OrderID)
}
