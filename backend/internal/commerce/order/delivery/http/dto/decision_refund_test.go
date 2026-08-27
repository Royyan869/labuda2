package dto

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TEST: C1B - Decision Builder hasActiveRefund
// ============================================================================

func TestDecisionBuilder_HasActiveRefund_HidesRefundCTA(t *testing.T) {
	order := &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusShipped,
		EscrowStatus: entity.EscrowStatusHolding,
	}

	pendingStatus := "pending_seller_review"
	decision := buildDecisionV2ForOrder(order, "buyer", true, &pendingStatus, nil, nil)

	for _, action := range decision.SecondaryActions {
		assert.NotEqual(t, ActionRequestRefund, action.Type,
			"Refund CTA should be hidden when active refund exists")
	}
}

func TestDecisionBuilder_NoActiveRefund_ShowsRefundCTA(t *testing.T) {
	order := &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusShipped,
		EscrowStatus: entity.EscrowStatusHolding,
	}

	decision := buildDecisionV2ForOrder(order, "buyer", false, nil, nil, nil)

	hasRefundAction := false
	for _, action := range decision.SecondaryActions {
		if action.Type == ActionRequestRefund {
			hasRefundAction = true
		}
	}
	assert.True(t, hasRefundAction,
		"Refund CTA should be visible when no active refund exists")
}

func TestDecisionBuilder_ActiveRefundPending_HidesDisputeCTA(t *testing.T) {
	order := &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusShipped,
		EscrowStatus: entity.EscrowStatusHolding,
	}

	pendingStatus := "pending_seller_review"
	decision := buildDecisionV2ForOrder(order, "buyer", true, &pendingStatus, nil, nil)

	for _, action := range decision.SecondaryActions {
		assert.NotEqual(t, ActionOpenDispute, action.Type,
			"Dispute CTA should be hidden when refund is pending seller review")
	}
}

func TestDecisionBuilder_ActiveRefundRejected_ShowsDisputeCTA(t *testing.T) {
	order := &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusShipped,
		EscrowStatus: entity.EscrowStatusHolding,
	}

	rejectedStatus := "seller_rejected"
	decision := buildDecisionV2ForOrder(order, "buyer", true, &rejectedStatus, nil, nil)

	hasDisputeAction := false
	for _, action := range decision.SecondaryActions {
		if action.Type == ActionOpenDispute {
			hasDisputeAction = true
		}
	}
	assert.True(t, hasDisputeAction,
		"Dispute CTA should be visible when refund is rejected (escalation path)")
}

func TestDecisionBuilder_ActiveRefundEscalated_HidesDisputeCTA(t *testing.T) {
	order := &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusShipped,
		EscrowStatus: entity.EscrowStatusHolding,
	}

	escalatedStatus := "escalated_to_admin"
	decision := buildDecisionV2ForOrder(order, "buyer", true, &escalatedStatus, nil, nil)

	for _, action := range decision.SecondaryActions {
		assert.NotEqual(t, ActionOpenDispute, action.Type,
			"Dispute CTA should be hidden when refund is escalated to admin")
	}
}

func TestDecisionBuilder_NoActiveRefund_ShowsDisputeCTA(t *testing.T) {
	order := &entity.Order{
		ID:           uuid.New(),
		BuyerID:      uuid.New(),
		SellerID:     uuid.New(),
		Status:       entity.StatusShipped,
		EscrowStatus: entity.EscrowStatusHolding,
	}

	decision := buildDecisionV2ForOrder(order, "buyer", false, nil, nil, nil)

	hasDisputeAction := false
	for _, action := range decision.SecondaryActions {
		if action.Type == ActionOpenDispute {
			hasDisputeAction = true
		}
	}
	assert.True(t, hasDisputeAction,
		"Dispute CTA should be visible when no active refund")
}


