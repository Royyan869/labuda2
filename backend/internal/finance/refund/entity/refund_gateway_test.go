// Phase-1 tests for the gateway-aware refund sub-state machine (TASK 33).
//
// These tests pin down the GatewayRefundStatus transitions and the
// idempotency / safety invariants the orchestration depends on:
//
//   - duplicate initiate is safe: MarkGatewayDispatched cannot run on a
//     row whose gateway has already succeeded.
//   - duplicate webhook ack is safe: MarkGatewayAckSucceeded returns nil
//     when called against an already-succeeded row.
//   - a synchronous gateway HTTP failure does not consume the row: it
//     transitions to 'failed' so a later retry can re-dispatch.
//   - a webhook 'failed' ack cannot overwrite a 'succeeded' row.
//
// The state machine is the only authority for these invariants — nothing
// in this file touches a database, ledger, escrow, wallet, or order.
package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPendingReviewRefund() *Refund {
	r := NewRefund(
		uuid.New(), uuid.New(), uuid.New(),
		RefundReasonItemNotReceived, nil, 100_00,
	)
	return r
}

func TestNewRefund_GatewayDefaultsAreSafe(t *testing.T) {
	r := newPendingReviewRefund()
	assert.Equal(t, GatewayRefundUnsubmitted, r.GatewayStatus,
		"new refund must start in 'unsubmitted' so the orchestration can dispatch")
	assert.Equal(t, 0, r.GatewayAttempts)
	assert.Nil(t, r.GatewayRefundID)
	assert.Nil(t, r.GatewayIdempotencyKey)
	assert.Nil(t, r.LastGatewayError)
	assert.Nil(t, r.GatewayRequestedAt)
	assert.Nil(t, r.GatewayAcknowledgedAt)
}

func TestMarkGatewayDispatched_TransitionsAndStoresKey(t *testing.T) {
	r := newPendingReviewRefund()
	gwID := "ref-abc-1"
	now := time.Now()

	require.NoError(t, r.MarkGatewayDispatched("idem-1", &gwID, now))

	assert.Equal(t, GatewayRefundPending, r.GatewayStatus)
	assert.Equal(t, 1, r.GatewayAttempts)
	require.NotNil(t, r.GatewayIdempotencyKey)
	assert.Equal(t, "idem-1", *r.GatewayIdempotencyKey)
	require.NotNil(t, r.GatewayRefundID)
	assert.Equal(t, gwID, *r.GatewayRefundID)
	require.NotNil(t, r.GatewayRequestedAt)
	assert.True(t, r.GatewayRequestedAt.Equal(now))
	assert.Nil(t, r.LastGatewayError, "successful dispatch must clear prior errors")
}

func TestMarkGatewayDispatched_RefusesAfterSucceeded(t *testing.T) {
	r := newPendingReviewRefund()
	now := time.Now()
	require.NoError(t, r.MarkGatewayDispatched("idem-1", nil, now))
	require.NoError(t, r.MarkGatewayAckSucceeded("ref-x", now))

	err := r.MarkGatewayDispatched("idem-2", nil, now)

	require.Error(t, err, "must refuse re-dispatch once the gateway acknowledged success")
	var bad *ErrInvalidGatewayTransition
	assert.ErrorAs(t, err, &bad)
}

func TestMarkGatewayRequestFailed_StoresErrorAndAllowsRetry(t *testing.T) {
	r := newPendingReviewRefund()
	now := time.Now()

	require.NoError(t, r.MarkGatewayRequestFailed("circuit breaker open", now))

	assert.Equal(t, GatewayRefundFailed, r.GatewayStatus)
	assert.Equal(t, 1, r.GatewayAttempts)
	require.NotNil(t, r.LastGatewayError)
	assert.Equal(t, "circuit breaker open", *r.LastGatewayError)

	// A retry must be allowed — the 'failed' state is not terminal until
	// the gateway acks one way or another.
	require.NoError(t, r.MarkGatewayDispatched("idem-2", nil, now))
	assert.Equal(t, GatewayRefundPending, r.GatewayStatus)
	assert.Equal(t, 2, r.GatewayAttempts)
	assert.Nil(t, r.LastGatewayError, "successful re-dispatch clears prior error")
}

func TestMarkGatewayAckSucceeded_IsIdempotent(t *testing.T) {
	r := newPendingReviewRefund()
	now := time.Now()
	require.NoError(t, r.MarkGatewayDispatched("idem-1", nil, now))

	require.NoError(t, r.MarkGatewayAckSucceeded("ref-x", now))
	first := r.GatewayAcknowledgedAt

	// Replaying the same ack must be a no-op (no state churn, no error).
	require.NoError(t, r.MarkGatewayAckSucceeded("ref-x", now.Add(time.Hour)))

	assert.Equal(t, GatewayRefundSucceeded, r.GatewayStatus)
	require.NotNil(t, r.GatewayAcknowledgedAt)
	assert.True(t, r.GatewayAcknowledgedAt.Equal(*first),
		"replay must not move acknowledged_at — that would re-emit downstream effects")
}

func TestMarkGatewayAckFailed_RefusesToOverwriteSucceeded(t *testing.T) {
	r := newPendingReviewRefund()
	now := time.Now()
	require.NoError(t, r.MarkGatewayDispatched("idem-1", nil, now))
	require.NoError(t, r.MarkGatewayAckSucceeded("ref-x", now))

	err := r.MarkGatewayAckFailed("late failure ack", now)

	require.Error(t, err)
	assert.Equal(t, GatewayRefundSucceeded, r.GatewayStatus,
		"a stray failure ack must NEVER undo a successful refund")
}

func TestMarkGatewayAckSucceeded_RejectsAckWithoutDispatch(t *testing.T) {
	r := newPendingReviewRefund()
	// gateway_status is still 'unsubmitted' — we never dispatched.

	err := r.MarkGatewayAckSucceeded("ref-x", time.Now())

	require.Error(t, err, "ack must not be honored for a refund we never dispatched")
}

func TestGatewayRefundStatus_IsTerminal(t *testing.T) {
	assert.True(t, GatewayRefundSucceeded.IsTerminal())
	assert.True(t, GatewayRefundFailed.IsTerminal())
	assert.False(t, GatewayRefundPending.IsTerminal())
	assert.False(t, GatewayRefundUnsubmitted.IsTerminal())
}


