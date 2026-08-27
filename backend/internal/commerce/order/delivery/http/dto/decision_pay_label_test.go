package dto

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
)

func strPtr(s string) *string { return &s }

func timePtr(t time.Time) *time.Time { return &t }

// TestSelectPayActionLabelKey_AllPaymentStates locks the canonical label_key
// selection for every payment state the pending-buyer pay CTA can encounter.
// Phase 2B-1: CTA wording must vary by payment state instead of always
// saying "Bayar Sekarang" (action.pay_now).
func TestSelectPayActionLabelKey_AllPaymentStates(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	past := time.Now().Add(-24 * time.Hour)

	cases := []struct {
		name             string
		paymentStatus    *string
		paymentExpiredAt *time.Time
		want             string
	}{
		{"no payment row", nil, nil, "action.pay_now"},
		{"active pending payment", strPtr("pending"), timePtr(future), "action.payment_continue"},
		{"pending payment expired", strPtr("pending"), timePtr(past), "action.pay_again"},
		{"pending with no expiry known", strPtr("pending"), nil, "action.payment_continue"},
		{"challenge", strPtr("challenge"), timePtr(future), "action.payment_check_status"},
		{"settlement while order pending", strPtr("settlement"), timePtr(future), "action.payment_check_status"},
		{"capture while order pending", strPtr("capture"), timePtr(future), "action.payment_check_status"},
		{"deny", strPtr("deny"), timePtr(future), "action.pay_again"},
		{"cancel", strPtr("cancel"), timePtr(future), "action.pay_again"},
		{"expire", strPtr("expire"), timePtr(future), "action.pay_again"},
		{"unrecognized status falls back safely", strPtr("unknown_status"), timePtr(future), "action.pay_now"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectPayActionLabelKey(tc.paymentStatus, tc.paymentExpiredAt)
			if got != tc.want {
				t.Errorf("selectPayActionLabelKey(%v, %v) = %q, want %q",
					tc.paymentStatus, tc.paymentExpiredAt, got, tc.want)
			}
		})
	}
}

func pendingBuyerOrder() *entity.Order {
	return &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusPending,
		EscrowStatus: entity.EscrowStatusHolding,
		CreatedAt:    time.Now().Add(-1 * time.Hour),
		UpdatedAt:    time.Now().Add(-1 * time.Hour),
	}
}

// TestBuildDecisionV2ForOrder_PendingBuyer_LabelVariesByPaymentState verifies
// the primary "pay" action's label_key reflects payment state end-to-end,
// while the action type, endpoint, and order_id input stay constant.
func TestBuildDecisionV2ForOrder_PendingBuyer_LabelVariesByPaymentState(t *testing.T) {
	order := pendingBuyerOrder()
	future := time.Now().Add(24 * time.Hour)

	cases := []struct {
		name          string
		paymentStatus *string
		expiredAt     *time.Time
		wantLabel     string
	}{
		{"no payment", nil, nil, "action.pay_now"},
		{"active pending", strPtr("pending"), timePtr(future), "action.payment_continue"},
		{"settlement lag", strPtr("settlement"), timePtr(future), "action.payment_check_status"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := buildDecisionV2ForOrder(order, "buyer", false, nil, tc.paymentStatus, tc.expiredAt)

			if decision.PrimaryAction == nil {
				t.Fatal("expected a primary action for pending buyer order")
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
				ID:           uuid.New(),
				BuyerID:      uuid.New(),
				SellerID:     uuid.New(),
				Status:       status,
				EscrowStatus: entity.EscrowStatusHolding,
				CreatedAt:    time.Now().Add(-1 * time.Hour),
				UpdatedAt:    time.Now().Add(-1 * time.Hour),
			}
			decision := buildDecisionV2ForOrder(order, "buyer", false, nil, &settled, nil)

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
