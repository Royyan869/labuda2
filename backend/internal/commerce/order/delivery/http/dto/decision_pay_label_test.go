package dto

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
)

func strPtr(s string) *string { return &s }

// TestSelectPayActionLabelKey_AllPaymentStates locks the canonical label_key
// selection for every payment state the pending-buyer pay CTA can encounter.
// The pay action itself is only exposed while the order's payment window is
// open (see buildDecisionV2ForOrder) — this function maps payment-row state
// to wording within that open window. The old "pending row past its
// payments.expired_at" branch is gone: a closed window offers NO pay action
// at all (canonical expiry source: orders.payment_expires_at).
func TestSelectPayActionLabelKey_AllPaymentStates(t *testing.T) {
	cases := []struct {
		name          string
		paymentStatus *string
		want          string
	}{
		{"no payment row", nil, "action.pay_now"},
		{"active pending payment", strPtr("pending"), "action.payment_continue"},
		{"challenge", strPtr("challenge"), "action.payment_check_status"},
		{"settlement while order pending", strPtr("settlement"), "action.payment_check_status"},
		{"capture while order pending", strPtr("capture"), "action.payment_check_status"},
		{"deny", strPtr("deny"), "action.pay_again"},
		{"cancel", strPtr("cancel"), "action.pay_again"},
		{"expire", strPtr("expire"), "action.pay_again"},
		{"unrecognized status falls back safely", strPtr("unknown_status"), "action.pay_now"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectPayActionLabelKey(tc.paymentStatus)
			if got != tc.want {
				t.Errorf("selectPayActionLabelKey(%v) = %q, want %q",
					tc.paymentStatus, got, tc.want)
			}
		})
	}
}

// pendingBuyerOrder returns a pending order whose payment window is still
// open — the canonical precondition for any pay-action exposure.
func pendingBuyerOrder() *entity.Order {
	return &entity.Order{
		ID:               uuid.New(),
		BuyerID:          uuid.New(),
		SellerID:         uuid.New(),
		Status:           entity.StatusPending,
		PaymentExpiresAt: time.Now().Add(30 * time.Minute),
		CreatedAt:        time.Now().Add(-1 * time.Hour),
		UpdatedAt:        time.Now().Add(-1 * time.Hour),
	}
}

// TestBuildDecisionV2ForOrder_PendingBuyer_LabelVariesByPaymentState verifies
// the primary "pay" action's label_key reflects payment state end-to-end,
// while the action type, endpoint, and order_id input stay constant.
func TestBuildDecisionV2ForOrder_PendingBuyer_LabelVariesByPaymentState(t *testing.T) {
	order := pendingBuyerOrder()

	cases := []struct {
		name          string
		paymentStatus *string
		wantLabel     string
	}{
		{"no payment", nil, "action.pay_now"},
		{"active pending", strPtr("pending"), "action.payment_continue"},
		{"settlement lag", strPtr("settlement"), "action.payment_check_status"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := buildDecisionV2ForOrder(order, "buyer", false, nil, tc.paymentStatus, false)

			if decision.PrimaryAction == nil {
				t.Fatal("expected a primary action for pending buyer order with open payment window")
			}
			if decision.PrimaryAction.Type != ActionPay {
				t.Errorf("expected action type %q, got %q", ActionPay, decision.PrimaryAction.Type)
			}
			if decision.PrimaryAction.LabelKey != tc.wantLabel {
				t.Errorf("expected label_key %q, got %q", tc.wantLabel, decision.PrimaryAction.LabelKey)
			}
			if decision.PrimaryAction.Endpoint != "/api/v1/payments" {
				t.Errorf("expected endpoint /api/v1/payments, got %q", decision.PrimaryAction.Endpoint)
			}
			if decision.PrimaryAction.Method != "POST" {
				t.Errorf("expected method POST, got %q", decision.PrimaryAction.Method)
			}
			if !decision.PrimaryAction.Enabled {
				t.Error("expected pay action enabled while the payment window is open")
			}
			foundOrderID := false
			if decision.PrimaryAction.InputSchema != nil {
				for _, f := range decision.PrimaryAction.InputSchema.Fields {
					if f.Key == "order_id" {
						foundOrderID = true
					}
				}
			}
			if !foundOrderID {
				t.Error("expected order_id in primary action input schema")
			}
		})
	}
}

// TestBuildDecisionV2ForOrder_PendingBuyer_ExpiredWindow_NoPayAction is the
// mandatory canonical-window regression: a pending order whose
// PaymentExpiresAt has passed must expose NO pay action (CreatePayment would
// reject every request with 410). The cancel action stays available.
func TestBuildDecisionV2ForOrder_PendingBuyer_ExpiredWindow_NoPayAction(t *testing.T) {
	order := &entity.Order{
		ID:               uuid.New(),
		BuyerID:          uuid.New(),
		SellerID:         uuid.New(),
		Status:           entity.StatusPending,
		PaymentExpiresAt: time.Now().Add(-time.Second), // window closed
		CreatedAt:        time.Now().Add(-1 * time.Hour),
		UpdatedAt:        time.Now().Add(-1 * time.Hour),
	}
	pending := strPtr("pending")

	decision := buildDecisionV2ForOrder(order, "buyer", false, nil, pending, false)

	if decision.PrimaryAction != nil && decision.PrimaryAction.Type == ActionPay {
		t.Fatalf("pending order past PaymentExpiresAt must not expose a pay action, got %q",
			decision.PrimaryAction.Type)
	}
	for _, a := range decision.SecondaryActions {
		if a.Type == ActionPay {
			t.Fatal("pending order past PaymentExpiresAt must not expose pay in secondary actions")
		}
	}

	// Display hint must not promise pay either.
	if decision.Display != nil && decision.Display.NextAction != nil {
		if decision.Display.NextAction.Type == ActionPay {
			t.Fatal("display hint must not promise pay past the payment window")
		}
	}

	// Cancel remains available for the pending lifecycle.
	foundCancel := false
	for _, a := range decision.SecondaryActions {
		if a.Type == ActionCancel {
			foundCancel = true
		}
	}
	if !foundCancel {
		t.Error("cancel action must remain available on a pending order past the payment window")
	}
}

// TestBuildDecisionV2ForOrder_ZeroPaymentExpiresAt_NoPayAction locks the
// fail-closed edge: a pending order without a payment window (zero
// time.Time — corrupt/legacy row) is never payable.
func TestBuildDecisionV2ForOrder_ZeroPaymentExpiresAt_NoPayAction(t *testing.T) {
	order := &entity.Order{
		ID:        uuid.New(),
		BuyerID:   uuid.New(),
		SellerID:  uuid.New(),
		Status:    entity.StatusPending,
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
		// PaymentExpiresAt left as zero value
	}

	decision := buildDecisionV2ForOrder(order, "buyer", false, nil, nil, false)

	if decision.PrimaryAction != nil && decision.PrimaryAction.Type == ActionPay {
		t.Fatal("pending order without a payment window must not expose a pay action (fail closed)")
	}
}

// TestBuildDecisionV2ForOrder_TerminalStates_NoPayAction verifies terminal
// and post-payment statuses never expose a pay action, regardless of any
// stale payment row data.
func TestBuildDecisionV2ForOrder_TerminalStates_NoPayAction(t *testing.T) {
	statuses := []entity.Status{
		entity.StatusPaid,
		entity.StatusShipped,
		entity.StatusCompleted,
		entity.StatusCancelled,
		entity.StatusExpired,
		entity.StatusCancelledTimeout,
		entity.StatusRefunded,
		entity.StatusPartiallyRefunded,
		entity.StatusDisputeOpen,
	}

	settled := "settlement"
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			order := &entity.Order{
				ID:               uuid.New(),
				BuyerID:          uuid.New(),
				SellerID:         uuid.New(),
				Status:           status,
				PaymentExpiresAt: time.Now().Add(30 * time.Minute),
				CreatedAt:        time.Now().Add(-1 * time.Hour),
				UpdatedAt:        time.Now().Add(-1 * time.Hour),
			}
			decision := buildDecisionV2ForOrder(order, "buyer", false, nil, &settled, false)

			if decision.PrimaryAction != nil && decision.PrimaryAction.Type == ActionPay {
				t.Errorf("status %q must not expose a pay action, got primary action type %q",
					status, decision.PrimaryAction.Type)
			}
			for _, a := range decision.SecondaryActions {
				if a.Type == ActionPay {
					t.Errorf("status %q must not expose a pay action in secondary actions", status)
				}
			}
		})
	}
}

// TestBuildDecisionV2ForOrder_PendingSeller_NoPayAction verifies authorization:
// the seller perspective never receives the buyer's pay action.
func TestBuildDecisionV2ForOrder_PendingSeller_NoPayAction(t *testing.T) {
	order := pendingBuyerOrder()
	decision := buildDecisionV2ForOrder(order, "seller", false, nil, nil, false)

	if decision.PrimaryAction != nil && decision.PrimaryAction.Type == ActionPay {
		t.Fatal("seller perspective must not expose the buyer pay action")
	}
}
