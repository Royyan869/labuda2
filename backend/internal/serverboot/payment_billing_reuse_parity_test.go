package serverboot

import (
	"os"
	"strings"
	"testing"
)

// TestInitiateBillingPayment_MethodAwareReuse is a source contract for F1.
//
// The canonical billing→payment engine must never silently reuse a pending
// billing payment created for a DIFFERENT payment method: a method switch
// terminalizes the superseded attempt and creates a fresh canonical payment,
// mirroring the canonical seller-subscription lifecycle. The reuse candidate
// must come from the deterministic, active-only lookup (F2), never the
// unscoped GetPaymentByReference.
func TestInitiateBillingPayment_MethodAwareReuse(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	body := funcBodyFrom(t, string(src), "func (h *CorePaymentHandler) InitiateBillingPayment(")

	// F2: reuse uses the active-only, deterministic billing lookup.
	if !strings.Contains(body, "FindPendingBillingPayment(") {
		t.Fatal("InitiateBillingPayment must reuse through FindPendingBillingPayment (active-only)")
	}
	// F1: reuse must never use the unscoped reference lookup.
	if strings.Contains(body, "GetPaymentByReference(") {
		t.Fatal("InitiateBillingPayment must not use the unscoped GetPaymentByReference for reuse")
	}
	// F1: method identity is compared before any reuse.
	if !strings.Contains(body, "*existing.PaymentMethodCode == method.Code") {
		t.Fatal("InitiateBillingPayment must compare the requested method identity before reuse")
	}
	// F1: a different method supersedes the stale pending payment.
	for _, want := range []string{
		"MarkAsFailed(ctx, tx, existing.ID, repository.PaymentStatusCancel)",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("InitiateBillingPayment must supersede a stale pending payment (%q)", want)
		}
	}
}
