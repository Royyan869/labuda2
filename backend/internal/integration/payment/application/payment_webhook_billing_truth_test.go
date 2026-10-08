package application

import (
	"os"
	"strings"
	"testing"
)

// TestWebhookBillingBranchSettlesPaymentTruth locks the Owner decision that a
// successful billing/promotion payment must SETTLE the payment row, not merely
// mark the billing transaction paid. Settlement must run BEFORE the billing
// domain is finalized so an unsettled payment can never credit Promote Balance.
func TestWebhookBillingBranchSettlesPaymentTruth(t *testing.T) {
	src, err := os.ReadFile("payment_webhook.go")
	if err != nil {
		t.Fatalf("read payment_webhook.go: %v", err)
	}
	code := string(src)

	billingStart := strings.Index(code, `payment.ReferenceType == "billing"`)
	if billingStart < 0 {
		t.Fatal("billing branch not found in payment_webhook.go")
	}
	// Bound the billing branch at the next reference-type branch.
	rest := code[billingStart:]
	end := strings.Index(rest, "ReferenceTypeSubscription")
	if end < 0 {
		end = len(rest)
	}
	branch := rest[:end]

	settle := strings.Index(branch, "settlementService.SettlePaymentByID(")
	capture := strings.Index(branch, "paymentRepo.MarkAsCapture(")
	markPaid := strings.Index(branch, "MarkPaidWithPayment(")

	if settle < 0 {
		t.Fatal("billing branch must settle the payment row (SettlePaymentByID)")
	}
	if capture < 0 {
		t.Fatal("billing branch must capture the payment row (MarkAsCapture)")
	}
	if markPaid < 0 {
		t.Fatal("billing branch must finalize the billing domain (MarkPaidWithPayment)")
	}
	if settle > markPaid {
		t.Fatal("settlement must run BEFORE MarkPaidWithPayment credits Promote Balance")
	}
	if capture > markPaid {
		t.Fatal("capture must run BEFORE MarkPaidWithPayment credits Promote Balance")
	}
}
