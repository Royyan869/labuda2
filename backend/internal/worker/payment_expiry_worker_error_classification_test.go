package worker

import (
	"errors"
	"fmt"
	"testing"

	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
)

// PASS_20B (requirement 26/27): cancelOrderForExpiredPayment used to log
// EVERY error from Expire() at Debug and swallow it as "not critical," which
// masked genuine failures (e.g. the D2 auction-restore bug) as silent,
// endless retries with zero operator visibility. isOrderAlreadyProcessedError
// is the pure classification function that now distinguishes the one
// expected, harmless case (order no longer pending) from everything else
// (a real failure that must be logged loudly and propagated). This test
// exercises that classification directly, without needing a live
// OrderService/DB.

func TestIsOrderAlreadyProcessedError_InvalidTransition_IsExpected(t *testing.T) {
	err := &orderEntity.InvalidTransitionError{
		CurrentStatus: orderEntity.Status("paid"),
		TargetStatus:  orderEntity.Status("expired"),
	}

	if !isOrderAlreadyProcessedError(err) {
		t.Fatal("InvalidTransitionError must be classified as the expected already-processed case")
	}
}

func TestIsOrderAlreadyProcessedError_WrappedInvalidTransition_IsExpected(t *testing.T) {
	inner := &orderEntity.InvalidTransitionError{
		CurrentStatus: orderEntity.Status("paid"),
		TargetStatus:  orderEntity.Status("expired"),
	}
	// errors.As must still unwrap through fmt.Errorf("%w", ...) chains, since
	// cancelOrderForExpiredPayment's caller may wrap errors on the way up.
	wrapped := fmt.Errorf("order expire failed for order %s: %w", "some-id", inner)

	if !isOrderAlreadyProcessedError(wrapped) {
		t.Fatal("a %w-wrapped InvalidTransitionError must still be classified as expected")
	}
}

func TestIsOrderAlreadyProcessedError_GenericError_IsNotExpected(t *testing.T) {
	err := errors.New("failed to lock auction for order release: boom")

	if isOrderAlreadyProcessedError(err) {
		t.Fatal("a generic/unexpected error must NOT be classified as the expected already-processed case")
	}
}

func TestIsOrderAlreadyProcessedError_Nil_IsNotExpected(t *testing.T) {
	if isOrderAlreadyProcessedError(nil) {
		t.Fatal("nil must not be classified as the expected already-processed case")
	}
}
