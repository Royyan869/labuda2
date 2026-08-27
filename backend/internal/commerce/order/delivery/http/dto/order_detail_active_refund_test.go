package dto

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/stretchr/testify/assert"
)

func TestOrderDetailResponse_WithActiveRefund(t *testing.T) {
	order := &entity.Order{
		ID:                        uuid.New(),
		BuyerID:                   uuid.New(),
		SellerID:                  uuid.New(),
		Status:                    entity.StatusShipped,
		EscrowStatus:              entity.EscrowStatusHolding,
		CreatedAt:                 time.Now().Add(-2 * time.Hour),
		UpdatedAt:                 time.Now().Add(-1 * time.Hour),
		ConfirmationExtensionUsed: false,
	}

	activeRefund := &ActiveRefundSummary{
		ID:              uuid.New(),
		OrderID:         order.ID,
		BuyerID:         order.BuyerID,
		SellerID:        order.SellerID,
		Status:          "pending_seller_review",
		Reason:          "item_not_received",
		RequestedAmount: 120000,
		CreatedAt:       time.Now().Add(-30 * time.Minute).Unix(),
		UpdatedAt:       time.Now().Add(-10 * time.Minute).Unix(),
	}

	refundStatus := "pending_seller_review"
	paymentID := uuid.New()
	paymentStatus := "settlement"
	resp := OrderToDetailResponseWithIdentity(
		order,
		order.BuyerID,
		"", "", "",
		"", "", "", "",
		nil,
		activeRefund,
		true,
		&refundStatus,
		&paymentStatus,
		&paymentID,
		nil, // no payment expiry in test
	)

	assert.True(t, resp.HasActiveRefund)
	if assert.NotNil(t, resp.ActiveRefund) {
		assert.Equal(t, activeRefund.ID, resp.ActiveRefund.ID)
		assert.Equal(t, activeRefund.OrderID, resp.ActiveRefund.OrderID)
		assert.Equal(t, activeRefund.BuyerID, resp.ActiveRefund.BuyerID)
		assert.Equal(t, activeRefund.SellerID, resp.ActiveRefund.SellerID)
		assert.Equal(t, activeRefund.Status, resp.ActiveRefund.Status)
		assert.Equal(t, activeRefund.RequestedAmount, resp.ActiveRefund.RequestedAmount)
	}
	assert.NotNil(t, resp.Decision)
	assert.NotNil(t, resp.PaymentID)
	assert.Equal(t, paymentID, *resp.PaymentID)
	if assert.NotNil(t, resp.PaymentStatus) {
		assert.Equal(t, paymentStatus, *resp.PaymentStatus)
	}
}

func TestOrderDetailResponse_WithoutActiveRefund(t *testing.T) {
	order := &entity.Order{
		ID:                        uuid.New(),
		BuyerID:                   uuid.New(),
		SellerID:                  uuid.New(),
		Status:                    entity.StatusShipped,
		EscrowStatus:              entity.EscrowStatusHolding,
		CreatedAt:                 time.Now().Add(-2 * time.Hour),
		UpdatedAt:                 time.Now().Add(-1 * time.Hour),
		ConfirmationExtensionUsed: false,
	}

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

	assert.False(t, resp.HasActiveRefund)
	assert.Nil(t, resp.ActiveRefund)
	assert.NotNil(t, resp.Decision)
	assert.Nil(t, resp.PaymentStatus)
	assert.Nil(t, resp.PaymentID)
}
