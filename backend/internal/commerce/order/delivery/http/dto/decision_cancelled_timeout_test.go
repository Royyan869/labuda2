package dto

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
)

func cancelledTimeoutOrder() *entity.Order {
	return &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusCancelledTimeout,
		EscrowStatus: entity.EscrowStatusReleased,
		CreatedAt:    time.Now().Add(-72 * time.Hour),
		UpdatedAt:    time.Now().Add(-1 * time.Hour),
	}
}

// TestCancelledTimeout_DisplayHints verifies badge and hint are populated.
func TestCancelledTimeout_DisplayHints_HasBadge(t *testing.T) {
	order := cancelledTimeoutOrder()
	hints := buildDisplayHintsForOrder(order, "buyer", nil, nil)
	if hints.Badge == nil || *hints.Badge != "Dibatalkan (Timeout)" {
		t.Errorf("expected badge 'Dibatalkan (Timeout)', got %+v", hints.Badge)
	}
	if hints.BadgeVariant == nil || *hints.BadgeVariant != "error" {
		t.Errorf("expected variant 'error', got %v", hints.BadgeVariant)
	}
}

// TestCancelledTimeout_DisplayHints_HasInfo verifies the seller-timeout info text.
func TestCancelledTimeout_DisplayHints_HasInfo(t *testing.T) {
	order := cancelledTimeoutOrder()
	hints := buildDisplayHintsForOrder(order, "buyer", nil, nil)
	if hints.Info == nil || *hints.Info != "Penjual tidak mengirim dalam batas waktu" {
		t.Errorf("expected info text about seller timeout, got %v", hints.Info)
	}
}

// TestCancelledTimeout_DecisionV2_NoPrimaryAction verifies terminal status has no action.
func TestCancelledTimeout_DecisionV2_NoPrimaryAction(t *testing.T) {
	order := cancelledTimeoutOrder()
	decision := buildDecisionV2ForOrder(order, "buyer", false, nil, nil, nil)
	if decision == nil {
		t.Fatal("expected non-nil decision")
	}
	if decision.PrimaryAction != nil {
		t.Errorf("expected nil primary action for cancelled_timeout, got %+v", decision.PrimaryAction)
	}
}

// TestCancelledTimeout_DecisionV2_NoSecondaryActions
func TestCancelledTimeout_DecisionV2_NoSecondaryActions(t *testing.T) {
	order := cancelledTimeoutOrder()
	decision := buildDecisionV2ForOrder(order, "buyer", false, nil, nil, nil)
	if len(decision.SecondaryActions) != 0 {
		t.Errorf("expected no secondary actions for cancelled_timeout, got %d", len(decision.SecondaryActions))
	}
}

// TestCancelledTimeout_DetailResponse_PaymentStatusWired verifies PaymentStatus flows through.
func TestCancelledTimeout_DetailResponse_PaymentStatusWired(t *testing.T) {
	order := cancelledTimeoutOrder()
	ps := "settlement"
	resp := OrderToDetailResponseWithIdentity(
		order,
		order.BuyerID,
		"", "", "",
		"", "", "", "",
		nil,
		nil,
		false,
		nil,
		&ps,
		nil, // no payment ID in test
		nil, // no payment expiry in test
	)
	if resp.PaymentStatus == nil || *resp.PaymentStatus != "settlement" {
		t.Errorf("expected PaymentStatus='settlement', got %v", resp.PaymentStatus)
	}
}

// TestDetailResponse_PaymentStatusNil_WhenNotProvided verifies nil is safe.
func TestDetailResponse_PaymentStatusNil_WhenNotProvided(t *testing.T) {
	order := cancelledTimeoutOrder()
	resp := OrderToDetailResponseWithIdentity(
		order,
		order.BuyerID,
		"", "", "",
		"", "", "", "",
		nil,
		nil,
		false,
		nil,
		nil, // no payment status in test
		nil, // no payment ID in test
		nil, // no payment expiry in test
	)
	if resp.PaymentStatus != nil {
		t.Errorf("expected nil PaymentStatus, got %v", resp.PaymentStatus)
	}
	if resp.PaymentID != nil {
		t.Errorf("expected nil PaymentID, got %v", resp.PaymentID)
	}
}

// TestTerminalOrders_NoPayNowAction verifies that terminal and post-payment
// states never expose the pay_now CTA.
func TestTerminalOrders_NoPayNowAction(t *testing.T) {
	cases := []struct {
		name   string
		status entity.Status
	}{
		{name: "completed", status: entity.StatusCompleted},
		{name: "cancelled", status: entity.StatusCancelled},
		{name: "cancelled_timeout", status: entity.StatusCancelledTimeout},
		{name: "shipped", status: entity.StatusShipped},
		{name: "delivered", status: entity.StatusDelivered},
		{name: "expired", status: entity.StatusExpired},
		{name: "refunded", status: entity.StatusRefunded},
		{name: "partially_refunded", status: entity.StatusPartiallyRefunded},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			order := cancelledTimeoutOrder()
			order.Status = tc.status
			decision := buildDecisionV2ForOrder(order, "buyer", false, nil, nil, nil)
			if decision == nil {
				t.Fatal("expected non-nil decision")
			}
			if decision.PrimaryAction != nil && decision.PrimaryAction.LabelKey == "action.pay_now" {
				t.Fatalf("unexpected pay_now primary action for %s", tc.name)
			}
			for _, action := range decision.SecondaryActions {
				if action.LabelKey == "action.pay_now" {
					t.Fatalf("unexpected pay_now secondary action for %s", tc.name)
				}
				if action.Endpoint == "/api/v1/payments" && action.Method == "POST" {
					t.Fatalf("unexpected payments endpoint for %s", tc.name)
				}
			}
		})
	}
}
