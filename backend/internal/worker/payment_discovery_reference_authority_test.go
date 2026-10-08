package worker

import (
	"os"
	"strings"
	"testing"
)

// TestDiscoveryRoutesEveryIncomingReferenceType locks that no legitimate
// incoming payment reference type silently falls through the discovery scanner:
// order, subscription, AND billing each have a canonical finalization branch.
func TestDiscoveryRoutesEveryIncomingReferenceType(t *testing.T) {
	src, err := os.ReadFile("payment_discovery_worker.go")
	if err != nil {
		t.Fatalf("read payment_discovery_worker.go: %v", err)
	}
	code := string(src)

	for _, want := range []string{
		"case paymentRepo.ReferenceTypeOrder:",
		"case paymentRepo.ReferenceTypeSubscription:",
		"case paymentRepo.ReferenceTypeBilling:",
		"w.finalizeBillingPayment(",
		"func (w *PaymentDiscoveryWorker) SetBillingPaymentProcessor(",
		"MarkPaidWithPayment(",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("discovery worker missing canonical billing recovery surface: %q", want)
		}
	}
}
