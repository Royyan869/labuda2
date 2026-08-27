package http

import (
	"os"
	"strings"
	"testing"
)

// =============================================================================
// REGRESSION LOCK: BUG-P7-2 — Fulfillment role errors must use errors.Is, not
// string comparison. The old code checked err.Error() == "seller authority required"
// and err.Error() == "buyer role required" — neither string ever matched the
// actual sentinel errors, so wrong-actor requests got HTTP 500 instead of 403.
// =============================================================================

// TestMarkShipped_AuthErrorMapping proves the MarkShipped handler uses
// errors.Is(err, auth.ErrSellerRequired) to map seller-only auth errors to 403.
func TestMarkShipped_AuthErrorMapping(t *testing.T) {
	src, err := os.ReadFile("order_handler.go")
	if err != nil {
		t.Fatalf("read order_handler.go: %v", err)
	}
	source := string(src)

	// Must NOT use fragile string comparison for seller authority error
	if strings.Contains(source, `err.Error() == "seller authority required"`) {
		t.Fatal("MarkShipped must NOT use err.Error()==\"seller authority required\" — " +
			"this never matches auth.ErrSellerRequired; wrong-actor gets HTTP 500 instead of 403. " +
			"Fix: errors.Is(err, auth.ErrSellerRequired)")
	}
	if strings.Contains(source, `strings.Contains(err.Error(), "seller authority required")`) {
		t.Fatal("MarkShipped must NOT use strings.Contains for seller authority error — " +
			"use errors.Is(err, auth.ErrSellerRequired) instead")
	}

	// Must use errors.Is with the sentinel
	if !strings.Contains(source, "errors.Is(err, auth.ErrSellerRequired)") {
		t.Fatal("MarkShipped MUST use errors.Is(err, auth.ErrSellerRequired) to detect " +
			"seller-only auth failures and return HTTP 403")
	}
}

// TestCompleteOrder_AuthErrorMapping proves the CompleteOrder handler uses
// errors.Is(err, auth.ErrBuyerRequired) to map buyer-only auth errors to 403.
func TestCompleteOrder_AuthErrorMapping(t *testing.T) {
	src, err := os.ReadFile("order_handler.go")
	if err != nil {
		t.Fatalf("read order_handler.go: %v", err)
	}
	source := string(src)

	// Must NOT use fragile string comparison for buyer role error
	if strings.Contains(source, `err.Error() == "buyer role required"`) {
		t.Fatal("CompleteOrder must NOT use err.Error()==\"buyer role required\" — " +
			"this never matches auth.ErrBuyerRequired; wrong-actor gets HTTP 500 instead of 403. " +
			"Fix: errors.Is(err, auth.ErrBuyerRequired)")
	}
	if strings.Contains(source, `strings.Contains(err.Error(), "buyer role required")`) {
		t.Fatal("CompleteOrder must NOT use strings.Contains for buyer role error — " +
			"use errors.Is(err, auth.ErrBuyerRequired) instead")
	}

	// Must use errors.Is with the sentinel
	if !strings.Contains(source, "errors.Is(err, auth.ErrBuyerRequired)") {
		t.Fatal("CompleteOrder MUST use errors.Is(err, auth.ErrBuyerRequired) to detect " +
			"buyer-only auth failures and return HTTP 403")
	}
}

// TestFulfillment_SentinelErrorsImported proves auth sentinel errors are
// imported and referenced correctly (not duplicated inline).
func TestFulfillment_SentinelErrorsImported(t *testing.T) {
	src, err := os.ReadFile("order_handler.go")
	if err != nil {
		t.Fatalf("read order_handler.go: %v", err)
	}
	source := string(src)

	// auth package must be imported
	if !strings.Contains(source, `"github.com/labuda/backend/internal/identity/auth"`) {
		t.Fatal("order_handler.go must import the auth package to use auth.ErrSellerRequired " +
			"and auth.ErrBuyerRequired sentinels")
	}

	// errors package must be imported (for errors.Is)
	if !strings.Contains(source, `"errors"`) {
		t.Fatal("order_handler.go must import \"errors\" to use errors.Is")
	}
}
