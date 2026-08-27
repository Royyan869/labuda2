// PASS_17B: locks the classification of account-status lookup errors.
// A missing user row (pgx.ErrNoRows) means the account no longer exists and
// must classify as ErrAccountRemoved (fail-closed 4xx downstream), while any
// other lookup failure must remain a wrapped internal error (500 downstream).
package auth

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestClassifyAccountStatusLookupError_NoRows_ReturnsAccountRemoved(t *testing.T) {
	err := classifyAccountStatusLookupError(pgx.ErrNoRows)
	if !errors.Is(err, ErrAccountRemoved) {
		t.Fatalf("expected ErrAccountRemoved, got: %v", err)
	}
}

func TestClassifyAccountStatusLookupError_WrappedNoRows_ReturnsAccountRemoved(t *testing.T) {
	err := classifyAccountStatusLookupError(fmt.Errorf("scan: %w", pgx.ErrNoRows))
	if !errors.Is(err, ErrAccountRemoved) {
		t.Fatalf("expected ErrAccountRemoved, got: %v", err)
	}
}

func TestClassifyAccountStatusLookupError_OtherError_RemainsInternal(t *testing.T) {
	cause := errors.New("connection refused")
	err := classifyAccountStatusLookupError(cause)
	if errors.Is(err, ErrAccountRemoved) ||
		errors.Is(err, ErrAccountSuspended) ||
		errors.Is(err, ErrAccountBanned) ||
		errors.Is(err, ErrAccountInactive) {
		t.Fatalf("internal error must not classify as an account state, got: %v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("internal error must wrap the original cause, got: %v", err)
	}
}
